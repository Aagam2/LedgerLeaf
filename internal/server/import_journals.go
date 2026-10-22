package server

import (
	"billbook/internal/accounting"
	"billbook/internal/export"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var importReference = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

func (s *Server) importJournals(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	key := r.URL.Query().Get("key")
	if !validKey(key) {
		fail(w, errors.New("request key required"))
		return
	}
	data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 20<<20))
	if e != nil {
		fail(w, e)
		return
	}
	rows, e := export.ReadTable(data, r.URL.Query().Get("format"))
	if e != nil {
		fail(w, e)
		return
	}
	entries, e := parseJournalRows(rows)
	if e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	hash := sha256.Sum256(data)
	repeat, e := claim(tx, b, "journal-import:"+key, hex.EncodeToString(hash[:]))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]interface{}{"already_imported": true, "journals": len(entries), "rows": len(rows) - 1})
		return
	}
	for _, j := range entries {
		if e = accounting.Post(tx, b, j.Date, "manual:import:"+j.Reference, j.Description, j.Lines); e != nil {
			fail(w, fmt.Errorf("reference %s: %w; no journals imported", j.Reference, e))
			return
		}
	}
	preview := r.URL.Query().Get("preview") == "1"
	if !preview {
		if e = auditFor(r, tx, b, "import.journals", int64(len(entries))); e != nil {
			fail(w, e)
			return
		}
		if e = tx.Commit(); e != nil {
			fail(w, e)
			return
		}
	}
	result := map[string]interface{}{"journals": len(entries), "rows": len(rows) - 1, "preview": preview}
	if preview {
		summaries := make([]map[string]interface{}, 0, len(entries))
		for _, j := range entries {
			var total int64
			for _, line := range j.Lines {
				total += line.Debit
			}
			summaries = append(summaries, map[string]interface{}{"reference": j.Reference, "date": j.Date, "description": j.Description, "lines": len(j.Lines), "debit": total, "credit": total})
		}
		result["entries"] = summaries
	}
	respond(w, 200, result)
}

// parseJournalRows validates the complete file before a database transaction is opened.
// Account existence, period locks and duplicate references remain transactional checks.
type importJournal struct {
	Reference, Date, Description string
	Lines                        []accounting.Line
}

func parseJournalRows(rows [][]string) ([]importJournal, error) {
	if len(rows) < 3 {
		return nil, errors.New("header and at least two journal lines required")
	}
	required := []string{"reference", "date", "description", "account", "debit", "credit"}
	header := map[string]int{}
	for i, c := range rows[0] {
		c = strings.TrimSpace(strings.TrimPrefix(c, "\ufeff"))
		if _, ok := header[c]; ok {
			return nil, errors.New("duplicate header")
		}
		header[c] = i
	}
	if len(header) != len(required) {
		return nil, errors.New("use exactly reference,date,description,account,debit,credit headers")
	}
	for _, c := range required {
		if _, ok := header[c]; !ok {
			return nil, fmt.Errorf("missing column %s", c)
		}
	}
	entries := map[string]*importJournal{}
	order := []string{}
	for i, row := range rows[1:] {
		if len(row) != len(required) {
			return nil, fmt.Errorf("row %d: wrong column count", i+2)
		}
		get := func(c string) string { return strings.TrimSpace(row[header[c]]) }
		ref, date, description, account := get("reference"), get("date"), get("description"), get("account")
		debit, e1 := strconv.ParseInt(get("debit"), 10, 64)
		credit, e2 := strconv.ParseInt(get("credit"), 10, 64)
		if !importReference.MatchString(ref) || !dateValid(date) || description == "" || len(description) > 1000 || e1 != nil || e2 != nil || debit < 0 || credit < 0 || debit > 1000000000000 || credit > 1000000000000 || (debit == 0) == (credit == 0) {
			return nil, fmt.Errorf("row %d: invalid fields; amounts must be integer paise with one positive side", i+2)
		}
		if account != "1000" && account != "1010" && account != "3000" && account != "5200" && !accountCode.MatchString(account) {
			return nil, fmt.Errorf("row %d: use a cash, bank, opening-equity, expense or custom account; operational control accounts require their workflows", i+2)
		}
		j, ok := entries[ref]
		if !ok {
			j = &importJournal{Reference: ref, Date: date, Description: description}
			entries[ref] = j
			order = append(order, ref)
		}
		if j.Date != date || j.Description != description {
			return nil, fmt.Errorf("row %d: date and description differ within reference", i+2)
		}
		j.Lines = append(j.Lines, accounting.Line{Account: account, Debit: debit, Credit: credit})
		if len(j.Lines) > 100 || len(order) > 1000 {
			return nil, errors.New("maximum 100 lines per journal and 1,000 journals per file")
		}
	}
	result := make([]importJournal, 0, len(order))
	for _, ref := range order {
		j := entries[ref]
		if len(j.Lines) < 2 {
			return nil, fmt.Errorf("reference %s requires at least two lines", ref)
		}
		var debit, credit int64
		for _, line := range j.Lines {
			debit += line.Debit
			credit += line.Credit
		}
		if debit > 1000000000000 || credit > 1000000000000 {
			return nil, fmt.Errorf("reference %s: journal total exceeds limit", ref)
		}
		if debit != credit {
			return nil, fmt.Errorf("reference %s: unbalanced journal: debit %d credit %d", ref, debit, credit)
		}
		result = append(result, *j)
	}
	return result, nil
}
