package server

import (
	"billbook/internal/accounting"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func (s *Server) splitPayments(w http.ResponseWriter, r *http.Request) {
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
		DocumentID int64  `json:"document_id"`
		Date       string `json:"date"`
		Key        string `json:"key"`
		Lines      []struct {
			Account string `json:"account"`
			Amount  int64  `json:"amount"`
			Mode    string `json:"mode"`
		} `json:"lines"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) || len(p.Lines) < 1 || len(p.Lines) > 10 {
		fail(w, errors.New("valid date, key and 1–10 payment parts required"))
		return
	}
	modes := map[string]bool{"CASH": true, "UPI": true, "BANK_TRANSFER": true, "CHEQUE": true, "CARD": true, "OTHER": true}
	var amount int64
	for _, l := range p.Lines {
		if !modes[l.Mode] || l.Amount <= 0 || l.Amount > 1000000000000 {
			fail(w, errors.New("invalid payment part"))
			return
		}
		amount += l.Amount
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "split:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	var kind, date, status string
	var outstanding int64
	if e = tx.QueryRow(`SELECT d.kind,d.date,d.status,d.total-x.paid-x.returned+x.refunded FROM documents d JOIN document_settlements x ON x.business_id=d.business_id AND x.id=d.id WHERE d.business_id=? AND d.id=?`, b, p.DocumentID).Scan(&kind, &date, &status, &outstanding); e != nil || status != "FINALIZED" || p.Date < date || amount > outstanding {
		fail(w, errors.New("invalid document, payment date or amount above outstanding"))
		return
	}
	if actorOf(r).Role == "BILLER" && kind != "SALE" {
		http.Error(w, "billers may record sales receipts only", 403)
		return
	}
	lines := []accounting.Line{}
	for i, l := range p.Lines {
		account, e := paymentAccount(tx, b, l.Mode, l.Account)
		if e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO payments(business_id,document_id,amount,date,mode,reference,account_code) VALUES(?,?,?,?,?,?,?)", b, p.DocumentID, l.Amount, p.Date, l.Mode, fmt.Sprintf("%s:%d", ref, i), account); e != nil {
			fail(w, e)
			return
		}
		if kind == "SALE" {
			lines = append(lines, accounting.Line{Account: account, Debit: l.Amount})
		} else {
			lines = append(lines, accounting.Line{Account: account, Credit: l.Amount})
		}
	}
	if kind == "SALE" {
		lines = append(lines, accounting.Line{Account: "1100", Credit: amount})
	} else {
		lines = append(lines, accounting.Line{Account: "2000", Debit: amount})
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Split document payment", lines); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "payment.split", p.DocumentID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
