package server

import (
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
)

// Quantities are thousandths of the frozen base unit, independent of invoice units.
type ReceiptMatch struct {
	ReceiptID int64 `json:"receipt_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
	Value     int64 `json:"value,omitempty"`
}

func (b Bill) hasReceipt() bool { return b.ReceiptID > 0 || len(b.ReceiptMatches) > 0 }

func validateReceipt(tx *sql.Tx, b int64, bill Bill) (int64, error) {
	matches, e := prepareReceiptMatches(tx, b, bill)
	var value int64
	for _, m := range matches {
		value += m.Value
	}
	return value, e
}

func prepareReceiptMatches(tx *sql.Tx, b int64, bill Bill) ([]ReceiptMatch, error) {
	if bill.Kind != "PURCHASE" || bill.SourceID != 0 || bill.DocumentType != "" || bill.ReceiptID < 0 || (bill.ReceiptID > 0 && len(bill.ReceiptMatches) > 0) {
		return nil, errors.New("receipt matching requires a purchase invoice without another source link")
	}
	matches := append([]ReceiptMatch(nil), bill.ReceiptMatches...)
	if bill.ReceiptID > 0 {
		// Preserve historical whole-receipt draft behavior.
		var used int
		if e := tx.QueryRow("SELECT COUNT(*) FROM documents WHERE business_id=? AND id!=? AND json_extract(snapshot,'$.receipt_id')=?", b, bill.ID, bill.ReceiptID).Scan(&used); e != nil {
			return nil, e
		}
		if used > 0 {
			return nil, errors.New("a billing draft already exists for this receipt")
		}
		for _, l := range bill.Lines {
			q, e := l.stockQuantity()
			if e != nil {
				return nil, e
			}
			matches = append(matches, ReceiptMatch{ReceiptID: bill.ReceiptID, ProductID: l.ProductID, Quantity: q})
		}
	}
	if len(matches) == 0 || len(matches) > 200 {
		return nil, errors.New("select 1–200 receipt allocations")
	}
	invoiceQty := map[int64]int64{}
	for _, l := range bill.Lines {
		q, e := l.stockQuantity()
		if e != nil {
			return nil, e
		}
		invoiceQty[l.ProductID] = q
	}
	seen := map[[2]int64]bool{}
	var total int64
	for i := range matches {
		m := &matches[i]
		pair := [2]int64{m.ReceiptID, m.ProductID}
		if seen[pair] || m.Quantity <= 0 || m.Quantity > 1000000000 || m.ReceiptID <= 0 {
			return nil, errors.New("invalid or duplicate receipt allocation")
		}
		seen[pair] = true
		var raw, date string
		var party, warehouse int64
		var reversed int
		if e := tx.QueryRow(`SELECT snapshot,date,party_id,warehouse_id,EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id) FROM goods_receipts g WHERE business_id=? AND id=?`, b, m.ReceiptID).Scan(&raw, &date, &party, &warehouse, &reversed); e != nil {
			return nil, errors.New("receipt not found in this business")
		}
		if reversed != 0 || party != bill.PartyID || warehouse != bill.WarehouseID || bill.Date < date {
			return nil, errors.New("receipt is reversed or supplier, warehouse or invoice date differs")
		}
		var receipt Bill
		if e := json.Unmarshal([]byte(raw), &receipt); e != nil {
			return nil, e
		}
		var line *BillLine
		for j := range receipt.Lines {
			if receipt.Lines[j].ProductID == m.ProductID {
				line = &receipt.Lines[j]
				break
			}
		}
		if line == nil {
			return nil, errors.New("product is not on the selected receipt")
		}
		qty, e := line.stockQuantity()
		if e != nil {
			return nil, e
		}
		var used int64
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0) FROM receipt_matches WHERE business_id=? AND receipt_id=? AND product_id=?", b, m.ReceiptID, m.ProductID).Scan(&used); e != nil {
			return nil, e
		}
		if m.Quantity > qty-used {
			return nil, errors.New("allocation exceeds remaining uninvoiced receipt quantity; reload receipts")
		}
		if bill.ReceiptID > 0 && (used != 0 || m.Quantity != qty || len(receipt.Lines) != len(bill.Lines)) {
			return nil, errors.New("legacy invoice must match the entire uninvoiced receipt")
		}
		before, _ := money.MulDiv(line.Amount, used, qty)
		after, _ := money.MulDiv(line.Amount, used+m.Quantity, qty)
		m.Value = after - before
		total += m.Value
		if total > 1000000000000 {
			return nil, errors.New("matched receipt value exceeds limit")
		}
		invoiceQty[m.ProductID] -= m.Quantity
	}
	for _, q := range invoiceQty {
		if q != 0 {
			return nil, errors.New("receipt allocations must match every invoice quantity in base units")
		}
	}
	return matches, nil
}
func postReceiptMatches(tx *sql.Tx, b int64, bill *Bill) error {
	matches, e := prepareReceiptMatches(tx, b, *bill)
	if e != nil {
		return e
	}
	if len(bill.ReceiptMatches) > 0 {
		bill.ReceiptMatches = matches
	}
	for _, m := range matches {
		if _, e = tx.Exec("INSERT INTO receipt_matches(business_id,receipt_id,document_id,product_id,quantity,value) VALUES(?,?,?,?,?,?)", b, m.ReceiptID, bill.ID, m.ProductID, m.Quantity, m.Value); e != nil {
			return e
		}
	}
	return nil
}
