package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"errors"
	"fmt"
)

// Allocate adjustments before tax. Cumulative allocation preserves every paise
// without putting a rounding remainder onto a zero-value line.
func allocateInvoiceAdjustments(b *Bill) error {
	const limit int64 = 1000000000000
	if b.InvoiceDiscount < 0 || b.InvoiceDiscount > limit || b.InvoiceCharge < 0 || b.InvoiceCharge > limit {
		return errors.New("invoice discount and charges must be between zero and the document limit")
	}
	var subtotal int64
	for _, l := range b.Lines {
		if l.Amount < 0 || l.Amount > limit || subtotal > limit-l.Amount {
			return errors.New("document subtotal exceeds supported limit")
		}
		subtotal += l.Amount
	}
	if subtotal == 0 || b.InvoiceDiscount > subtotal {
		return errors.New("invoice discount exceeds the positive item subtotal")
	}
	net := subtotal - b.InvoiceDiscount
	if b.InvoiceCharge > limit-net || (net == 0 && b.InvoiceCharge != 0) {
		return errors.New("charges require a positive discounted subtotal within the document limit")
	}
	var weight, previousDiscount, chargeWeight, previousCharge int64
	for i := range b.Lines {
		l := &b.Lines[i]
		weight += l.Amount
		discount, _ := money.MulDiv(b.InvoiceDiscount, weight, subtotal)
		l.InvoiceDiscount = discount - previousDiscount
		previousDiscount = discount
		l.Amount -= l.InvoiceDiscount
		chargeWeight += l.Amount
		l.InvoiceCharge = 0
		if net > 0 {
			charge, _ := money.MulDiv(b.InvoiceCharge, chargeWeight, net)
			l.InvoiceCharge = charge - previousCharge
			previousCharge = charge
		}
		l.Amount += l.InvoiceCharge
		l.RoundOff = 0
	}
	b.RoundOff = 0
	return nil
}

func applyInvoiceRounding(b *Bill) {
	b.RoundOff = 0
	if !b.RoundToRupee || b.Total <= 0 {
		return
	}
	rounded := ((b.Total + 50) / 100) * 100
	b.RoundOff = rounded - b.Total
	var cumulative, previous int64
	for i := range b.Lines {
		l := &b.Lines[i]
		payable := l.Amount
		if b.TaxMode != "REVERSE_CHARGE" {
			payable += l.taxTotal()
		}
		cumulative += payable
		allocated, _ := money.MulDiv(rounded, cumulative, b.Total)
		l.RoundOff = allocated - previous - payable
		previous = allocated
	}
	b.Total = rounded
}

func roundingJournal(b Bill, reverse bool) []accounting.Line {
	n := b.RoundOff
	if b.Kind == "SALE" {
		n = -n
	}
	if reverse {
		n = -n
	}
	if n > 0 {
		return []accounting.Line{{Account: "5300", Debit: n}}
	}
	if n < 0 {
		return []accounting.Line{{Account: "5300", Credit: -n}}
	}
	return nil
}

func adjustmentSummary(b Bill) string {
	if b.InvoiceDiscount == 0 && b.InvoiceCharge == 0 && !b.RoundToRupee {
		return ""
	}
	return fmt.Sprintf("Invoice discount: INR %s\nAllocated charges: INR %s\nAdjustments are included in line amounts.\nRound-off: INR %s", decimal(b.InvoiceDiscount, 2), decimal(b.InvoiceCharge, 2), decimal(b.RoundOff, 2))
}
