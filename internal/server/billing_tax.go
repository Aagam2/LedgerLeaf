package server

import (
	"billbook/internal/accounting"
	"billbook/internal/gst"
	"billbook/internal/money"
	"database/sql"
	"errors"
)

func prepareTax(tx *sql.Tx, b int64, bill *Bill) error {
	if bill.TaxMode == "" {
		bill.TaxMode = "NORMAL"
	}
	switch bill.TaxMode {
	case "NORMAL", "EXPORT_LUT", "EXPORT_TAX", "SEZ_LUT", "SEZ_TAX", "REVERSE_CHARGE":
	default:
		return errors.New("invalid supply treatment")
	}
	var businessState, partyState string
	if e := tx.QueryRow("SELECT gstin,state_code,tax_scheme FROM businesses WHERE id=?", b).Scan(&bill.BusinessGSTIN, &businessState, &bill.TaxScheme); e != nil {
		return e
	}
	if e := tx.QueryRow("SELECT gstin,state_code FROM parties WHERE business_id=? AND id=?", b, bill.PartyID).Scan(&bill.PartyGSTIN, &partyState); e != nil {
		return e
	}
	bill.SupplierState = businessState
	if bill.Kind == "PURCHASE" {
		bill.SupplierState = partyState
	}
	bill.TaxEnabled = bill.TaxScheme != "UNREGISTERED"
	if bill.Kind == "SALE" && bill.TaxScheme == "COMPOSITION" {
		bill.TaxEnabled = false
	}
	if bill.Kind == "PURCHASE" && bill.PartyGSTIN == "" && bill.TaxMode != "REVERSE_CHARGE" {
		bill.TaxEnabled = false
	}
	if !bill.TaxEnabled && bill.TaxMode != "NORMAL" {
		return errors.New("special supply treatment requires regular GST registration")
	}
	if bill.Kind == "PURCHASE" && bill.TaxMode != "NORMAL" && bill.TaxMode != "REVERSE_CHARGE" {
		return errors.New("export and SEZ treatments apply to sales")
	}
	if bill.InputCredit && (bill.Kind != "PURCHASE" || bill.TaxScheme != "REGULAR" || !bill.TaxEnabled) {
		return errors.New("input credit requires a taxable purchase by a regular registered business")
	}
	if bill.TaxMode == "EXPORT_LUT" || bill.TaxMode == "EXPORT_TAX" {
		bill.PlaceOfSupply = "96"
		if bill.ShippingAddress == "" {
			return errors.New("export destination address required")
		}
	} else if bill.TaxEnabled {
		if !gst.State(bill.PlaceOfSupply) || !gst.State(bill.SupplierState) {
			return errors.New("supplier state and explicit place of supply required for GST")
		}
	}
	if bill.TaxMode == "SEZ_LUT" || bill.TaxMode == "SEZ_TAX" {
		if bill.PartyGSTIN == "" {
			return errors.New("SEZ recipient GSTIN required")
		}
	}
	if len(bill.ShippingAddress) > 1000 {
		return errors.New("delivery address is too long")
	}
	bill.TaxTotal = 0
	return nil
}
func calculateLineTax(tx *sql.Tx, b int64, bill *Bill, l *BillLine) error {
	l.HSN = ""
	l.Treatment = "NON_GST"
	l.Rate = 0
	l.CessRate = 0
	l.CessPerUnit = 0
	l.CGST = 0
	l.SGST = 0
	l.IGST = 0
	l.Cess = 0
	if !bill.TaxEnabled {
		return nil
	}
	if e := tx.QueryRow(`SELECT hsn,treatment,rate,cess,cess_per_unit FROM product_taxes WHERE business_id=? AND product_id=? AND effective_date<=? ORDER BY effective_date DESC LIMIT 1`, b, l.ProductID, bill.Date).Scan(&l.HSN, &l.Treatment, &l.Rate, &l.CessRate, &l.CessPerUnit); e != nil {
		return errors.New("configure an effective GST rate and HSN/SAC for every item")
	}
	rate, cess := l.Rate, l.CessRate
	qty, e := l.stockQuantity()
	if e != nil {
		return e
	}
	fixed, e := money.MulDiv(l.CessPerUnit, qty, 1000)
	if e != nil || fixed > 1000000000000 {
		return errors.New("quantity cess exceeds limit")
	}
	if bill.TaxMode == "EXPORT_LUT" || bill.TaxMode == "SEZ_LUT" {
		rate = 0
		cess = 0
		fixed = 0
	}
	inter := bill.SupplierState != bill.PlaceOfSupply || bill.TaxMode == "SEZ_LUT" || bill.TaxMode == "SEZ_TAX"
	taxInput := l.Amount
	if bill.Inclusive {
		taxInput -= fixed
		if taxInput < 0 {
			return errors.New("tax-inclusive value is below quantity cess")
		}
	}
	a, e := gst.Calculate(taxInput, rate, cess, bill.Inclusive, inter)
	if e != nil {
		return e
	}
	l.Amount = a.Base
	l.CGST = a.CGST
	l.SGST = a.SGST
	l.IGST = a.IGST
	l.Cess = a.Cess + fixed
	bill.TaxTotal += a.Total - a.Base + fixed
	return nil
}
func (b Bill) baseTotal() int64 {
	var total int64
	for _, l := range b.Lines {
		total += l.Amount
	}
	return total
}
func (l BillLine) taxTotal() int64 { return l.CGST + l.SGST + l.IGST + l.Cess }
func (b Bill) inventoryTotal() int64 {
	n := b.baseTotal()
	if !b.InputCredit {
		n += b.TaxTotal
	}
	return n
}
func taxJournal(b Bill, reverse bool) []accounting.Line {
	var sums [4]int64
	for _, l := range b.Lines {
		sums[0] += l.CGST
		sums[1] += l.SGST
		sums[2] += l.IGST
		sums[3] += l.Cess
	}
	out := []accounting.Line{}
	for i, n := range sums {
		if b.Kind == "SALE" && b.TaxMode != "REVERSE_CHARGE" || b.Kind == "PURCHASE" && b.TaxMode == "REVERSE_CHARGE" {
			out = append(out, accounting.Line{Account: []string{"2100", "2110", "2120", "2130"}[i], Credit: n})
		}
		if b.Kind == "PURCHASE" && b.InputCredit {
			out = append(out, accounting.Line{Account: []string{"1300", "1310", "1320", "1330"}[i], Debit: n})
		}
	}
	if reverse {
		for i := range out {
			out[i].Debit, out[i].Credit = out[i].Credit, out[i].Debit
		}
	}
	return out
}
