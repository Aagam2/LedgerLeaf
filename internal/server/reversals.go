package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// reverseDocument preserves the original snapshot and posts an immutable full
// credit/return and settlement reversal in one transaction.
func (s *Server) reverseDocument(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		s.query(w, `SELECT v.id,v.number,d.number AS invoice,d.kind,v.date,v.reason,v.total,v.refund,v.refund_mode FROM credit_notes v JOIN documents d ON d.business_id=v.business_id AND d.id=v.document_id WHERE v.business_id=? ORDER BY v.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Preview bool `json:"preview"`
		Lines   []struct {
			ProductID int64 `json:"product_id"`
			Quantity  int64 `json:"quantity"`
		} `json:"lines"`
		ID     int64  `json:"id"`
		Date   string `json:"date"`
		Reason string `json:"reason"`
		Mode   string `json:"mode"`
		Key    string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	p.Reason = strings.TrimSpace(p.Reason)
	if !dateValid(p.Date) || p.Reason == "" || len(p.Reason) > 1000 || !validKey(p.Key) || (p.Mode != "CASH" && p.Mode != "BANK") {
		fail(w, errors.New("provide a date, reason, refund account and request key"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "reversal:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	var snapshot, status string
	if e = tx.QueryRow("SELECT snapshot,status FROM documents WHERE business_id=? AND id=?", b, p.ID).Scan(&snapshot, &status); e != nil || status != "FINALIZED" {
		fail(w, errors.New("select a finalized document"))
		return
	}
	var bill Bill
	if e = json.Unmarshal([]byte(snapshot), &bill); e != nil {
		fail(w, e)
		return
	}
	if p.Date < bill.Date {
		fail(w, errors.New("return cannot predate its document"))
		return
	}
	var paid int64
	var lastPayment sql.NullString
	if e = tx.QueryRow("SELECT COALESCE(SUM(amount),0),MAX(date) FROM payments WHERE business_id=? AND document_id=?", b, p.ID).Scan(&paid, &lastPayment); e != nil {
		fail(w, e)
		return
	}
	if lastPayment.Valid && p.Date < lastPayment.String {
		fail(w, errors.New("return cannot predate its payments"))
		return
	}
	var priorTotal, priorRefund int64
	if e = tx.QueryRow("SELECT returned,refunded FROM document_settlements WHERE business_id=? AND id=?", b, p.ID).Scan(&priorTotal, &priorRefund); e != nil {
		fail(w, e)
		return
	}
	var lastReturn sql.NullString
	if e = tx.QueryRow("SELECT MAX(date) FROM credit_notes WHERE business_id=? AND document_id=?", b, p.ID).Scan(&lastReturn); e != nil {
		fail(w, e)
		return
	}
	if lastReturn.Valid && p.Date < lastReturn.String {
		fail(w, errors.New("return cannot predate earlier credit notes"))
		return
	}
	requested := map[int64]int64{}
	for _, l := range p.Lines {
		if l.Quantity <= 0 || requested[l.ProductID] != 0 {
			fail(w, errors.New("invalid or duplicate return line"))
			return
		}
		requested[l.ProductID] = l.Quantity
	}
	original := bill
	bill.Lines = nil
	bill.Total = 0
	bill.TaxTotal = 0
	bill.InvoiceDiscount = 0
	bill.InvoiceCharge = 0
	bill.RoundOff = 0
	complete := true
	for _, l := range original.Lines {
		var returned int64
		if e = tx.QueryRow(`SELECT COALESCE(SUM(json_extract(l.value,'$.quantity')),0) FROM credit_notes c,json_each(c.snapshot,'$.lines') l WHERE c.business_id=? AND c.document_id=? AND json_extract(l.value,'$.product_id')=?`, b, p.ID, l.ProductID).Scan(&returned); e != nil {
			fail(w, e)
			return
		}
		quantity := requested[l.ProductID]
		if p.Lines == nil {
			quantity = l.Quantity - returned
		}
		delete(requested, l.ProductID)
		if quantity < 0 || quantity > l.Quantity-returned {
			fail(w, errors.New("return quantity exceeds remaining invoiced quantity"))
			return
		}
		if returned+quantity < l.Quantity {
			complete = false
		}
		if quantity == 0 {
			continue
		}
		part := l
		part.Quantity = quantity
		if _, e = part.stockQuantity(); e != nil {
			fail(w, e)
			return
		}
		allocation := func(value int64) int64 {
			before, _ := money.MulDiv(value, returned, l.Quantity)
			after, _ := money.MulDiv(value, returned+quantity, l.Quantity)
			return after - before
		}
		part.Amount = allocation(l.Amount)
		part.Cost = allocation(l.Cost)
		part.Discount = allocation(l.Discount)
		part.CGST = allocation(l.CGST)
		part.SGST = allocation(l.SGST)
		part.IGST = allocation(l.IGST)
		part.Cess = allocation(l.Cess)
		part.InvoiceDiscount = allocation(l.InvoiceDiscount)
		part.InvoiceCharge = allocation(l.InvoiceCharge)
		part.RoundOff = 0
		if original.RoundToRupee {
			originalPayable := l.Amount + l.RoundOff
			partPayable := part.Amount
			if bill.TaxMode != "REVERSE_CHARGE" {
				originalPayable += l.taxTotal()
				partPayable += part.taxTotal()
			}
			part.RoundOff = allocation(originalPayable) - partPayable
		}
		bill.InvoiceDiscount += part.InvoiceDiscount
		bill.InvoiceCharge += part.InvoiceCharge
		bill.RoundOff += part.RoundOff
		bill.Lines = append(bill.Lines, part)
		bill.Total += part.Amount + part.RoundOff
		bill.TaxTotal += part.taxTotal()
		if bill.TaxMode != "REVERSE_CHARGE" {
			bill.Total += part.taxTotal()
		}
	}
	if len(requested) > 0 || len(bill.Lines) == 0 {
		fail(w, errors.New("no remaining invoice lines selected"))
		return
	}
	refund := paid - (original.Total - priorTotal - bill.Total) - priorRefund
	if refund < 0 {
		refund = 0
	}
	originalNumber := bill.Number
	// Independent immutable credit-note sequence; internal number stays <=16 characters.
	series := "CN/" + p.Date[:4]
	if _, e = tx.Exec("INSERT OR IGNORE INTO counters(business_id,series,next) VALUES(?,?,1)", b, series); e != nil {
		fail(w, e)
		return
	}
	var seq int64
	if e = tx.QueryRow("SELECT next FROM counters WHERE business_id=? AND series=?", b, series).Scan(&seq); e != nil {
		fail(w, e)
		return
	}
	bill.Number = fmt.Sprintf("CN%s/%08d", p.Date[2:4], seq)
	bill.Date = p.Date
	bill.Status = "FINALIZED"
	bill.Notes = "Against " + originalNumber + " dated " + original.Date + ". " + p.Reason
	if len(bill.Number) > 16 {
		fail(w, errors.New("credit note sequence exhausted"))
		return
	}
	if _, e = tx.Exec("UPDATE counters SET next=next+1 WHERE business_id=? AND series=?", b, series); e != nil {
		fail(w, e)
		return
	}
	snapshotBytes, _ := json.Marshal(bill)
	res, e := tx.Exec("INSERT INTO credit_notes(business_id,document_id,number,date,reason,total,refund,refund_mode,reference,snapshot) VALUES(?,?,?,?,?,?,?,?,?,?)", b, p.ID, bill.Number, p.Date, p.Reason, bill.Total, refund, p.Mode, ref, string(snapshotBytes))
	if e != nil {
		fail(w, e)
		return
	}
	id, e := res.LastInsertId()
	if e != nil {
		fail(w, e)
		return
	}
	if complete {
		if _, e = tx.Exec("INSERT INTO document_reversals(business_id,document_id,date,reason,refund,refund_mode,reference) VALUES(?,?,?,?,?,?,?)", b, p.ID, p.Date, p.Reason, priorRefund+refund, p.Mode, ref); e != nil {
			fail(w, errors.New("document has already been returned or cancelled"))
			return
		}
	}
	paid = refund
	var stockCost int64
	for _, l := range bill.Lines {
		if l.Kind == "SERVICE" {
			continue
		}
		var qty, value int64
		var last sql.NullString
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0),MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, l.ProductID, bill.WarehouseID).Scan(&qty, &value, &last); e != nil {
			fail(w, e)
			return
		}
		if last.Valid && p.Date < last.String {
			fail(w, errors.New("return cannot predate later stock movements"))
			return
		}
		stockQty, err := l.stockQuantity()
		if err != nil {
			fail(w, err)
			return
		}
		delta, cost := stockQty, l.Cost
		if bill.Kind == "PURCHASE" {
			if qty < stockQty {
				fail(w, fmt.Errorf("insufficient stock to return %s", l.Name))
				return
			}
			cost, e = money.MulDiv(value, stockQty, qty)
			if e != nil {
				fail(w, e)
				return
			}
			delta = -stockQty
		}
		stockCost += cost
		valueChange := cost
		if bill.Kind == "PURCHASE" {
			valueChange = -cost
		}
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date,remarks) VALUES(?,?,?,?,?,?,?,?,?)", b, l.ProductID, bill.WarehouseID, delta, valueChange, bill.Kind+"_RETURN", ref, p.Date, p.Reason); e != nil {
			fail(w, e)
			return
		}
	}
	account := "1000"
	if p.Mode == "BANK" {
		account = "1010"
	}
	lines := []accounting.Line{{Account: "4000", Debit: bill.baseTotal()}, {Account: "1100", Credit: bill.Total}, {Account: "1200", Debit: stockCost}, {Account: "5000", Credit: stockCost}, {Account: "1100", Debit: paid}, {Account: account, Credit: paid}}
	if bill.Kind == "PURCHASE" {
		lines = []accounting.Line{{Account: "2000", Debit: bill.Total}, {Account: "1200", Credit: stockCost}, {Account: account, Debit: paid}, {Account: "2000", Credit: paid}}
		if stockCost > bill.inventoryTotal() {
			lines = append(lines, accounting.Line{Account: "5100", Debit: stockCost - bill.inventoryTotal()})
		} else if stockCost < bill.inventoryTotal() {
			lines = append(lines, accounting.Line{Account: "5100", Credit: bill.inventoryTotal() - stockCost})
		}
	}
	lines = append(lines, taxJournal(bill, true)...)
	lines = append(lines, roundingJournal(bill, true)...)
	if e = accounting.Post(tx, b, p.Date, ref, "Return credit note: "+bill.Number, lines); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "document.reversed", id); e != nil {
		fail(w, e)
		return
	}
	if p.Preview {
		respond(w, 200, map[string]interface{}{"total": bill.Total, "refund": paid, "number": bill.Number})
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]interface{}{"id": id, "refund": paid, "number": bill.Number, "total": bill.Total})
}
