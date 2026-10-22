package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) reverseReceipt(w http.ResponseWriter, r *http.Request) {
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
		Key    string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) || strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 1000 {
		fail(w, errors.New("date, reason and idempotency key required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "receipt-reversal:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	var snapshot, date string
	var matched, reversed int
	if e = tx.QueryRow(`SELECT snapshot,date,(SELECT COUNT(*) FROM receipt_matches m WHERE m.business_id=g.business_id AND m.receipt_id=g.id),EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id) FROM goods_receipts g WHERE business_id=? AND id=?`, b, p.ID).Scan(&snapshot, &date, &matched, &reversed); e != nil {
		fail(w, e)
		return
	}
	if matched > 0 || reversed != 0 || p.Date < date {
		fail(w, errors.New("only uninvoiced, unreversed receipts can be reversed on or after their receipt date"))
		return
	}
	var receipt Bill
	if e = json.Unmarshal([]byte(snapshot), &receipt); e != nil {
		fail(w, e)
		return
	}
	var stockCost int64
	for _, l := range receipt.Lines {
		var qty, value int64
		var last sql.NullString
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0),MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, l.ProductID, receipt.WarehouseID).Scan(&qty, &value, &last); e != nil {
			fail(w, e)
			return
		}
		q, err := l.stockQuantity()
		if err != nil {
			fail(w, err)
			return
		}
		if qty < q || (last.Valid && p.Date < last.String) {
			fail(w, errors.New("insufficient stock or reversal predates a stock movement"))
			return
		}
		cost, err := money.MulDiv(value, q, qty)
		if err != nil {
			fail(w, err)
			return
		}
		stockCost += cost
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(?,?,?,?,?,'RECEIPT_REVERSAL',?,?)", b, l.ProductID, receipt.WarehouseID, -q, -cost, ref, p.Date); e != nil {
			fail(w, e)
			return
		}
	}
	lines := []accounting.Line{{Account: "2050", Debit: receipt.Total}, {Account: "1200", Credit: stockCost}}
	if stockCost > receipt.Total {
		lines = append(lines, accounting.Line{Account: "5100", Debit: stockCost - receipt.Total})
	} else if stockCost < receipt.Total {
		lines = append(lines, accounting.Line{Account: "5100", Credit: receipt.Total - stockCost})
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Receipt correction: "+p.Reason, lines); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO receipt_reversals(business_id,receipt_id,date,reason,reference) VALUES(?,?,?,?,?)", b, p.ID, p.Date, p.Reason, ref); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "receipt.reversed", p.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
