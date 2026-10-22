package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) periodLock(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		var date string
		e = s.DB.QueryRow("SELECT closed_through FROM business_controls WHERE business_id=?", b).Scan(&date)
		if e != nil && e != sql.ErrNoRows {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"closed_through": date})
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Date string `json:"closed_through"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if p.Date != "" && !dateValid(p.Date) {
		fail(w, errors.New("invalid closing date"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var lastClose string
	if e = tx.QueryRow("SELECT COALESCE(MAX(date),'') FROM year_closings WHERE business_id=?", b).Scan(&lastClose); e != nil {
		fail(w, e)
		return
	}
	if p.Date < lastClose {
		fail(w, errors.New("cannot reopen a finalized year closing"))
		return
	}
	if _, e = tx.Exec("INSERT INTO business_controls(business_id,closed_through) VALUES(?,?) ON CONFLICT(business_id) DO UPDATE SET closed_through=excluded.closed_through", b, p.Date); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "period.lock:"+p.Date, 0); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]string{"closed_through": p.Date})
}
func (s *Server) banks(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		s.query(w, "SELECT code,name,account_number,ifsc FROM bank_accounts WHERE business_id=? ORDER BY code", b)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Number string `json:"account_number"`
		IFSC   string `json:"ifsc"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !accountCode.MatchString(p.Code) || strings.TrimSpace(p.Name) == "" || len(p.Name) > 160 || len(p.Number) > 40 || len(p.IFSC) > 11 {
		fail(w, errors.New("bank requires unique 4–8 digit code starting 6–9 and name"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO accounts(business_id,code,name,kind) VALUES(?,?,?,'ASSET')", b, p.Code, p.Name); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO bank_accounts(business_id,code,name,account_number,ifsc) VALUES(?,?,?,?,?)", b, p.Code, p.Name, p.Number, p.IFSC); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "bank.created", 0); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, p)
}
func paymentAccount(tx *sql.Tx, b int64, mode, chosen string) (string, error) {
	if mode == "CASH" {
		if chosen != "" && chosen != "1000" {
			return "", errors.New("cash payment must use cash account")
		}
		return "1000", nil
	}
	if chosen == "" {
		chosen = "1010"
	}
	var count int
	if e := tx.QueryRow("SELECT COUNT(*) FROM bank_accounts WHERE business_id=? AND code=?", b, chosen).Scan(&count); e != nil {
		return "", e
	}
	if count != 1 {
		return "", errors.New("select a bank account in this business")
	}
	return chosen, nil
}
func (s *Server) reconcile(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		s.query(w, `SELECT l.id,j.date,j.description,l.account,l.debit,l.credit,r.statement_date,r.statement_reference FROM journal_lines l JOIN journals j ON j.business_id=l.business_id AND j.id=l.journal_id JOIN bank_accounts a ON a.business_id=l.business_id AND a.code=l.account LEFT JOIN bank_reconciliations r ON r.business_id=l.business_id AND r.journal_line_id=l.id WHERE l.business_id=? AND (?='' OR l.account=?) ORDER BY j.date DESC,l.id DESC LIMIT ? OFFSET ?`, b, r.URL.Query().Get("account"), r.URL.Query().Get("account"), limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		ID        int64  `json:"id"`
		Date      string `json:"date"`
		Reference string `json:"reference"`
		Clear     bool   `json:"clear"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !p.Clear && (!dateValid(p.Date) || strings.TrimSpace(p.Reference) == "" || len(p.Reference) > 160) {
		fail(w, errors.New("statement date and reference required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var date string
	if e = tx.QueryRow("SELECT j.date FROM journal_lines l JOIN journals j ON j.id=l.journal_id AND j.business_id=l.business_id JOIN bank_accounts a ON a.business_id=l.business_id AND a.code=l.account WHERE l.business_id=? AND l.id=?", b, p.ID).Scan(&date); e != nil || !p.Clear && p.Date < date {
		fail(w, errors.New("select bank entry and a statement date on or after posting"))
		return
	}
	if p.Clear {
		_, e = tx.Exec("DELETE FROM bank_reconciliations WHERE business_id=? AND journal_line_id=?", b, p.ID)
	} else {
		_, e = tx.Exec("INSERT INTO bank_reconciliations(business_id,journal_line_id,statement_date,statement_reference) VALUES(?,?,?,?)", b, p.ID, p.Date, p.Reference)
	}
	if e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, fmt.Sprintf("bank.reconcile.clear=%t", p.Clear), p.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
