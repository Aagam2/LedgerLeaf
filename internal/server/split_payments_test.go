package server

import (
	"fmt"
	"testing"
)

func TestSplitPaymentsAtomicAndIdempotent(t *testing.T) {
	s, call := setupTrading(t)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "split-purchase-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, purchase, 200)
	body := fmt.Sprintf(`{"document_id":%d,"date":"2026-10-15","key":"split-payment-request","lines":[{"amount":6000,"mode":"CASH"},{"amount":4000,"mode":"UPI"}]}`, purchase.ID)
	if _, e := s.DB.Exec("CREATE TRIGGER reject_split BEFORE INSERT ON payments WHEN NEW.mode='UPI' BEGIN SELECT RAISE(ABORT,'injected payment failure'); END"); e != nil {
		t.Fatal(e)
	}
	call("POST", "split-payments?business_id=1", body, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM payments") != 0 {
		t.Fatal("split partially committed")
	}
	s.DB.Exec("DROP TRIGGER reject_split")
	call("POST", "split-payments?business_id=1", body, 200)
	call("POST", "split-payments?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT SUM(amount) FROM payments") != 10000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='2000'") != 0 {
		t.Fatal("split reconciliation")
	}
	call("POST", "split-payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"date":"2026-10-15","key":"overpaid-split-key","lines":[{"amount":1,"mode":"CASH"}]}`, purchase.ID), 400)
	assertIntegrity(t, s.DB)
}
func TestDraftRetryReturnsSavedDocument(t *testing.T) {
	_, call := setupTrading(t)
	b := draftInput("SALE", "2026-10-15", "retryable-draft-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
	one := saveBill(t, call, b)
	two := saveBill(t, call, b)
	if one.ID == 0 || one.ID != two.ID {
		t.Fatal("retry did not return document")
	}
}
