package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func directReceipt(t *testing.T, call testAPI, q, price int64, key string) int64 {
	t.Helper()
	var out struct {
		ID int64 `json:"id"`
	}
	raw := call("POST", "receipts?business_id=1", fmt.Sprintf(`{"party_id":2,"warehouse_id":1,"date":"2026-10-15","key":%q,"lines":[{"product_id":1,"quantity":%d,"price":%d}]}`, key, q, price), 200)
	if e := json.Unmarshal([]byte(raw), &out); e != nil {
		t.Fatal(e)
	}
	return out.ID
}
func TestReceiptSplitCombinedInvoicesAndStaleDraft(t *testing.T) {
	s, call := setupTrading(t)
	a := directReceipt(t, call, 3000, 3333, "direct-receipt-first")
	b := directReceipt(t, call, 2000, 4500, "direct-receipt-second")
	if directReceipt(t, call, 3000, 3333, "direct-receipt-first") != a {
		t.Fatal("retry changed receipt")
	}
	input := draftInput("PURCHASE", "2026-10-16", "split-receipt-invoice", BillLine{ProductID: 1, Quantity: 2000, Price: 5000})
	input.ReceiptMatches = []ReceiptMatch{{ReceiptID: a, ProductID: 1, Quantity: 1000, Value: 999999}, {ReceiptID: b, ProductID: 1, Quantity: 1000}}
	first := saveBill(t, call, input)
	input.Key = "stale-receipt-invoice"
	stale := saveBill(t, call, input)
	execSQL(t, s.DB, `CREATE TRIGGER fail_receipt_invoice BEFORE INSERT ON journals BEGIN SELECT RAISE(ABORT,'injected journal failure'); END`)
	finish(t, call, first, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM receipt_matches") != 0 {
		t.Fatal("failed journal left receipt allocations")
	}
	execSQL(t, s.DB, "DROP TRIGGER fail_receipt_invoice")
	finish(t, call, first, 200)
	// A further invoice consumes all remaining quantities, making the stale draft invalid.
	input.Key = "remaining-receipt-invoice"
	input.Lines[0].Quantity = 3000
	input.ReceiptMatches[0].Quantity = 2000
	rest := saveBill(t, call, input)
	finish(t, call, rest, 200)
	finish(t, call, stale, 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 5000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 18999 {
		t.Fatal("matching changed received stock")
	}
	if scalar(t, s.DB, "SELECT SUM(value) FROM receipt_matches") != 18999 || scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='2050'") != 0 {
		t.Fatal("receipt value not fully cleared")
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM receipt_matches") != 4 {
		t.Fatal("failed finalization left matches")
	}
	call("POST", "receipt-reversal?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reason":"already matched","key":"matched-correction"}`, a), 400)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reason":"supplier return","mode":"CASH","key":"matched-purchase-return"}`, first.ID), 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 {
		t.Fatal("matched purchase return stock")
	}
	if !strings.Contains(call("GET", "receipts?business_id=1", "", 200), "unmatched_quantity") {
		t.Fatal("missing availability")
	}
	assertIntegrity(t, s.DB)
}
func TestReceiptCorrectionsStockChronologyAndLocks(t *testing.T) {
	s, call := setupTrading(t)
	a := directReceipt(t, call, 2000, 3000, "correct-receipt-first")
	directReceipt(t, call, 2000, 5000, "correct-receipt-second")
	body := fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"wrong receipt","key":"correct-receipt-request"}`, a)
	call("POST", "period-lock?business_id=1", `{"closed_through":"2026-10-15"}`, 200)
	call("POST", "receipt-reversal?business_id=1", body, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM receipt_reversals") != 0 {
		t.Fatal("locked correction persisted")
	}
	call("POST", "period-lock?business_id=1", `{"closed_through":"2026-10-14"}`, 200)
	call("POST", "receipt-reversal?business_id=1", body, 200)
	call("POST", "receipt-reversal?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 2000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 8000 {
		t.Fatal("correction average cost")
	}
	if scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='2050'") != 10000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='5100'") != 2000 {
		t.Fatal("correction journal")
	}
	input := draftInput("PURCHASE", "2026-10-16", "reversed-receipt-invoice", BillLine{ProductID: 1, Quantity: 2000, Price: 3000})
	input.ReceiptMatches = []ReceiptMatch{{ReceiptID: a, ProductID: 1, Quantity: 2000}}
	call("POST", "documents?business_id=1", jsonText(t, input), 400)
	if _, e := s.DB.Exec("DELETE FROM receipt_reversals"); e == nil {
		t.Fatal("reversal editable")
	}
	call("POST", "receipt-reversal?business_id=2", body, 400)
	assertIntegrity(t, s.DB)
}
func TestReceiptMatchingValidationAndAlternateUnits(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	call("POST", "receipts?business_id=1", `{"party_id":2,"warehouse_id":1,"date":"2026-10-15","key":"direct-box-receipt","lines":[{"product_id":1,"quantity":1000,"price":10001,"sale_unit":"BOX"}]}`, 200)
	for i, q := range []int64{5000, 5000, 2000} {
		in := draftInput("PURCHASE", "2026-10-15", fmt.Sprintf("unit-matching-invoice-%d", i), BillLine{ProductID: 1, Quantity: q, Price: 900})
		in.ReceiptMatches = []ReceiptMatch{{ReceiptID: 1, ProductID: 1, Quantity: q}}
		if i == 0 {
			bad := in
			bad.ReceiptMatches = []ReceiptMatch{{ReceiptID: 1, ProductID: 1, Quantity: q + 1}}
			call("POST", "documents?business_id=1", jsonText(t, bad), 400)
		}
		bill := saveBill(t, call, in)
		finish(t, call, bill, 200)
	}
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 12000 || scalar(t, s.DB, "SELECT SUM(value) FROM receipt_matches") != 10001 {
		t.Fatal("unit allocation rounding")
	}
	assertIntegrity(t, s.DB)
}
func TestReceiptCorrectionReopensOrderAndRejectsShortStock(t *testing.T) {
	s, call := setupTrading(t)
	in := draftInput("PURCHASE", "2026-10-15", "correction-source-order", BillLine{ProductID: 1, Quantity: 2000, Price: 6000})
	in.DocumentType = "PURCHASE_ORDER"
	var order Bill
	json.Unmarshal([]byte(call("POST", "commercial-documents?business_id=1", jsonText(t, in), 200)), &order)
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, order.ID), 200)
	call("POST", "receipts?business_id=1", fmt.Sprintf(`{"order_id":%d,"warehouse_id":1,"date":"2026-10-15","key":"correction-order-receipt","lines":[{"product_id":1,"quantity":2000}]}`, order.ID), 200)
	body := `{"id":1,"date":"2026-10-15","reason":"wrong receipt","key":"correction-order-reverse"}`
	call("POST", "receipt-reversal?business_id=1", body, 200)
	var remaining Bill
	json.Unmarshal([]byte(call("GET", fmt.Sprintf("commercial-documents?business_id=1&id=%d", order.ID), "", 200)), &remaining)
	if remaining.Lines[0].ReceivedQuantity != 0 {
		t.Fatal("order quantity not reopened")
	}
	r := directReceipt(t, call, 2000, 6000, "short-stock-receipt")
	sale := saveBill(t, call, draftInput("SALE", "2026-10-16", "short-stock-sale", BillLine{ProductID: 1, Quantity: 1000, Price: 8000}))
	finish(t, call, sale, 200)
	call("POST", "receipt-reversal?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reason":"cannot remove sold stock","key":"short-stock-correction"}`, r), 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 {
		t.Fatal("failed correction changed stock")
	}
	assertIntegrity(t, s.DB)
}
