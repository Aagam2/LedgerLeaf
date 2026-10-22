package server

import (
	"billbook/internal/accounting"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type recurringExpense struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Amount      int64  `json:"amount"`
	Account     string `json:"account"`
	NextDate    string `json:"next_date"`
	Months      int    `json:"interval_months"`
	Active      int    `json:"active"`
	Key         string `json:"key"`
}

func (s *Server) recurringExpenses(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		s.query(w, "SELECT id,description,category,amount,account,next_date,interval_months,active FROM recurring_expenses WHERE business_id=? ORDER BY active DESC,next_date,id LIMIT ? OFFSET ?", b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p recurringExpense
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	if r.URL.Path == "/api/recurring-expense-post" {
		if !validKey(p.Key) || !dateValid(p.NextDate) {
			fail(w, errors.New("expected due date and request key required"))
			return
		}
		payload, _ := json.Marshal(p)
		ref := "expense:recurring:" + p.Key
		repeat, e := claim(tx, b, ref, string(payload))
		if e != nil {
			fail(w, e)
			return
		}
		if repeat {
			respond(w, 200, map[string]bool{"ok": true})
			return
		}
		var saved recurringExpense
		var day int
		if e = tx.QueryRow("SELECT description,category,amount,account,next_date,interval_months,anchor_day,active FROM recurring_expenses WHERE business_id=? AND id=?", b, p.ID).Scan(&saved.Description, &saved.Category, &saved.Amount, &saved.Account, &saved.NextDate, &saved.Months, &day, &saved.Active); e != nil {
			fail(w, e)
			return
		}
		if saved.Active != 1 || p.NextDate != saved.NextDate {
			fail(w, errors.New("schedule changed or is inactive; reload it"))
			return
		}
		if e = accounting.Post(tx, b, saved.NextDate, ref, saved.Description, []accounting.Line{{Account: "5200", Debit: saved.Amount}, {Account: saved.Account, Credit: saved.Amount}}); e != nil {
			fail(w, e)
			return
		}
		var id int64
		if e = tx.QueryRow("SELECT id FROM journals WHERE business_id=? AND reference=?", b, ref).Scan(&id); e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO expense_details(business_id,journal_id,category) VALUES(?,?,?)", b, id, saved.Category); e != nil {
			fail(w, e)
			return
		}
		d, _ := time.Parse("2006-01-02", saved.NextDate)
		first := time.Date(d.Year(), d.Month()+time.Month(saved.Months), 1, 0, 0, 0, 0, time.UTC)
		last := first.AddDate(0, 1, -1).Day()
		if day > last {
			day = last
		}
		next := first.AddDate(0, 0, day-1).Format("2006-01-02")
		if _, e = tx.Exec("UPDATE recurring_expenses SET next_date=? WHERE business_id=? AND id=?", next, b, p.ID); e != nil {
			fail(w, e)
			return
		}
		if e = auditFor(r, tx, b, "expense.recurring.posted", id); e != nil {
			fail(w, e)
			return
		}
	} else {
		if strings.TrimSpace(p.Description) == "" || len(p.Description) > 1000 || len(p.Category) > 100 || p.Amount <= 0 || p.Amount > 1000000000000 || !dateValid(p.NextDate) || p.Months < 1 || p.Months > 12 || (p.Active != 0 && p.Active != 1) || p.Account == "" {
			fail(w, errors.New("valid description, amount, monthly interval, next date and cash/bank required"))
			return
		}
		mode := "BANK_TRANSFER"
		if p.Account == "1000" {
			mode = "CASH"
		}
		if _, e = paymentAccount(tx, b, mode, p.Account); e != nil {
			fail(w, e)
			return
		}
		d, _ := time.Parse("2006-01-02", p.NextDate)
		if p.ID == 0 {
			_, e = tx.Exec("INSERT INTO recurring_expenses(business_id,description,category,amount,account,next_date,interval_months,anchor_day,active) VALUES(?,?,?,?,?,?,?,?,?)", b, p.Description, p.Category, p.Amount, p.Account, p.NextDate, p.Months, d.Day(), p.Active)
		} else {
			var n int
			if e = tx.QueryRow("SELECT COUNT(*) FROM recurring_expenses WHERE business_id=? AND id=?", b, p.ID).Scan(&n); e != nil || n != 1 {
				fail(w, errors.New("schedule not found"))
				return
			}
			_, e = tx.Exec("UPDATE recurring_expenses SET description=?,category=?,amount=?,account=?,next_date=?,interval_months=?,anchor_day=?,active=? WHERE business_id=? AND id=?", p.Description, p.Category, p.Amount, p.Account, p.NextDate, p.Months, d.Day(), p.Active, b, p.ID)
		}
		if e != nil {
			fail(w, e)
			return
		}
		if e = auditFor(r, tx, b, "expense.recurring.saved", p.ID); e != nil {
			fail(w, e)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) expenseAttachments(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		if id := r.URL.Query().Get("id"); id != "" {
			var name, mime string
			var content []byte
			if e = s.DB.QueryRow("SELECT name,mime,content FROM expense_attachments WHERE business_id=? AND id=?", b, id).Scan(&name, &mime, &content); e != nil {
				fail(w, e)
				return
			}
			w.Header().Set("Content-Type", mime)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
			w.Write(content)
			return
		}
		s.query(w, "SELECT id,name,mime,length(content) AS bytes FROM expense_attachments WHERE business_id=? AND journal_id=? ORDER BY id", b, r.URL.Query().Get("journal_id"))
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	content, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 5<<20))
	if e != nil || len(content) == 0 {
		fail(w, errors.New("attachment must be between 1 byte and 5 MB"))
		return
	}
	mime := http.DetectContentType(content)
	if mime != "application/pdf" && mime != "image/png" && mime != "image/jpeg" {
		fail(w, errors.New("upload a PDF, PNG or JPEG"))
		return
	}
	name := filepath.Base(r.URL.Query().Get("name"))
	if name == "" || len(name) > 160 || strings.ContainsAny(name, "\r\n\x00") {
		fail(w, errors.New("invalid attachment filename"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var id int64
	if e = tx.QueryRow("SELECT id FROM journals WHERE business_id=? AND id=? AND reference LIKE 'expense:%'", b, r.URL.Query().Get("journal_id")).Scan(&id); e != nil {
		fail(w, errors.New("expense not found"))
		return
	}
	if _, e = tx.Exec("INSERT INTO expense_attachments(business_id,journal_id,name,mime,content) VALUES(?,?,?,?,?)", b, id, name, mime, content); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "expense.attachment", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
