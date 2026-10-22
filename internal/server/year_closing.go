package server

import (
	"billbook/internal/accounting"
	"errors"
	"net/http"
	"time"
)

func (s *Server) closeYear(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		s.query(w, "SELECT date,net_profit,reference FROM year_closings WHERE business_id=? ORDER BY date DESC", b)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if actorOf(r).Role != "ADMIN" {
		http.Error(w, "administrator permission required", 403)
		return
	}
	var p struct {
		Date    string `json:"date"`
		Confirm string `json:"confirm"`
		Preview bool   `json:"preview"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || (!p.Preview && p.Confirm != "CLOSE FINANCIAL YEAR") {
		fail(w, errors.New("valid year-end date and confirmation required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var month int
	if e = tx.QueryRow("SELECT COALESCE((SELECT year_start_month FROM business_settings WHERE business_id=?),4)", b).Scan(&month); e != nil {
		fail(w, e)
		return
	}
	date, _ := time.Parse("2006-01-02", p.Date)
	next := date.AddDate(0, 0, 1)
	if next.Day() != 1 || int(next.Month()) != month {
		fail(w, errors.New("date must be the last day of the configured financial year"))
		return
	}
	var count int
	if e = tx.QueryRow("SELECT COUNT(*) FROM year_closings WHERE business_id=? AND date>=?", b, p.Date).Scan(&count); e != nil {
		fail(w, e)
		return
	}
	if count > 0 {
		fail(w, errors.New("this or a later year is already closed"))
		return
	}
	rows, e := tx.Query("SELECT a.code,COALESCE(SUM(l.debit-l.credit),0) FROM accounts a LEFT JOIN (SELECT l.* FROM journal_lines l JOIN journals j ON j.business_id=l.business_id AND j.id=l.journal_id WHERE j.date<=?) l ON l.business_id=a.business_id AND l.account=a.code WHERE a.business_id=? AND a.kind IN ('INCOME','EXPENSE') GROUP BY a.code", p.Date, b)
	if e != nil {
		fail(w, e)
		return
	}
	lines := []accounting.Line{}
	var net int64
	for rows.Next() {
		var code string
		var balance int64
		if e = rows.Scan(&code, &balance); e != nil {
			rows.Close()
			fail(w, e)
			return
		}
		net -= balance
		if balance > 0 {
			lines = append(lines, accounting.Line{Account: code, Credit: balance})
		} else if balance < 0 {
			lines = append(lines, accounting.Line{Account: code, Debit: -balance})
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		fail(w, e)
		return
	}
	if net > 0 {
		lines = append(lines, accounting.Line{Account: "3100", Credit: net})
	} else if net < 0 {
		lines = append(lines, accounting.Line{Account: "3100", Debit: -net})
	}
	if p.Preview {
		respond(w, 200, map[string]interface{}{"date": p.Date, "net_profit": net, "lines": lines, "preview": true})
		return
	}
	if e = accounting.Seed(tx, b); e != nil {
		fail(w, e)
		return
	}
	ref := "year-close:" + p.Date
	if e = accounting.Post(tx, b, p.Date, ref, "Financial-year closing", lines); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO year_closings(business_id,date,reference,net_profit) VALUES(?,?,?,?)", b, p.Date, ref, net); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO business_controls(business_id,closed_through) VALUES(?,?) ON CONFLICT(business_id) DO UPDATE SET closed_through=MAX(closed_through,excluded.closed_through)", b, p.Date); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "year.closed", b); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"net_profit": net})
}
