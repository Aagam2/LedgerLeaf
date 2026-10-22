package server

import (
	"fmt"
	"testing"
)

func TestPeriodLockAndBankReconciliation(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "banks?business_id=1", `{"code":"6100","name":"Trading bank","account_number":"00123","ifsc":"TEST0123456"}`, 201)
	p := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "locked-purchase-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	call("POST", "period-lock?business_id=1", `{"closed_through":"2026-10-15"}`, 200)
	finish(t, call, p, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 0 || scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != 0 {
		t.Fatal("locked transaction partially posted")
	}
	call("POST", "period-lock?business_id=1", `{"closed_through":"2026-10-14"}`, 200)
	finish(t, call, p, 200)
	call("POST", "payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"amount":10000,"date":"2026-10-15","mode":"BANK_TRANSFER","account":"6100","key":"named-bank-payment"}`, p.ID), 200)
	id := scalar(t, s.DB, "SELECT id FROM journal_lines WHERE account='6100'")
	call("POST", "bank-reconciliation?business_id=2", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reference":"Statement 123"}`, id), 400)
	call("POST", "bank-reconciliation?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reference":"Statement 123"}`, id), 200)
	call("GET", "bank-reconciliation?business_id=1&account=6100", "", 200)
	call("POST", "bank-reconciliation?business_id=1", fmt.Sprintf(`{"id":%d,"clear":true}`, id), 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM bank_reconciliations") != 0 || scalar(t, s.DB, "SELECT SUM(credit) FROM journal_lines WHERE account='6100'") != 10000 {
		t.Fatal("reconciliation changed ledger")
	}
	assertIntegrity(t, s.DB)
}
