package server

import (
	"billbook/internal/accounting"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var accountCode = regexp.MustCompile(`^[6-9][0-9]{3,7}$`)

func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		s.query(w, "SELECT code,name,kind FROM accounts WHERE business_id=? ORDER BY code", b)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Code string `json:"code"`
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	kinds := map[string]bool{"ASSET": true, "LIABILITY": true, "EQUITY": true, "INCOME": true, "EXPENSE": true}
	if !accountCode.MatchString(p.Code) || strings.TrimSpace(p.Name) == "" || len(p.Name) > 160 || !kinds[p.Kind] {
		fail(w, errors.New("use a unique code starting 6–9 (4–8 digits), name and valid account type"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO accounts(business_id,code,name,kind) VALUES(?,?,?,?)", b, p.Code, p.Name, p.Kind); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "account.created", 0); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]string{"code": p.Code})
}
func (s *Server) manualJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var p struct {
		Date        string `json:"date"`
		Description string `json:"description"`
		Key         string `json:"key"`
		Lines       []struct {
			Account string `json:"account"`
			Debit   int64  `json:"debit"`
			Credit  int64  `json:"credit"`
		} `json:"lines"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) || strings.TrimSpace(p.Description) == "" || len(p.Description) > 1000 || len(p.Lines) < 2 || len(p.Lines) > 100 {
		fail(w, errors.New("provide date, description and 2–100 balanced lines"))
		return
	}
	lines := []accounting.Line{}
	var amount int64
	for _, l := range p.Lines {
		if l.Account != "1000" && l.Account != "1010" && l.Account != "3000" && l.Account != "5200" && !accountCode.MatchString(l.Account) {
			fail(w, errors.New("control accounts must be posted through their business workflows"))
			return
		}
		if l.Debit < 0 || l.Credit < 0 || l.Debit > 1000000000000 || l.Credit > 1000000000000 || (l.Debit == 0) == (l.Credit == 0) {
			fail(w, errors.New("each line needs one positive debit or credit"))
			return
		}
		amount += l.Debit
		lines = append(lines, accounting.Line{Account: l.Account, Debit: l.Debit, Credit: l.Credit})
	}
	if amount <= 0 {
		fail(w, errors.New("journal must have positive value"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "manual:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, p.Description, lines); e != nil {
		fail(w, e)
		return
	}
	var id int64
	if e = tx.QueryRow("SELECT id FROM journals WHERE business_id=? AND reference=?", b, ref).Scan(&id); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "journal.posted", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}
func (s *Server) reverseJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var p struct {
		ID     int64  `json:"id"`
		Date   string `json:"date"`
		Reason string `json:"reason"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 1000 {
		fail(w, errors.New("valid reversal date and reason required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var ref, date string
	if e = tx.QueryRow("SELECT reference,date FROM journals WHERE business_id=? AND id=?", b, p.ID).Scan(&ref, &date); e != nil {
		fail(w, e)
		return
	}
	if p.Date < date || (!strings.HasPrefix(ref, "manual:") && !strings.HasPrefix(ref, "expense:")) {
		fail(w, errors.New("only manual or expense journals may be reversed here, on or after their date"))
		return
	}
	rows, e := tx.Query("SELECT account,credit,debit FROM journal_lines WHERE business_id=? AND journal_id=?", b, p.ID)
	if e != nil {
		fail(w, e)
		return
	}
	lines := []accounting.Line{}
	for rows.Next() {
		var line accounting.Line
		if e = rows.Scan(&line.Account, &line.Debit, &line.Credit); e != nil {
			rows.Close()
			fail(w, e)
			return
		}
		lines = append(lines, line)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		fail(w, e)
		return
	}
	if e = accounting.Post(tx, b, p.Date, fmt.Sprintf("manual-reversal:%d", p.ID), p.Reason, lines); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "journal.reversed", p.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
