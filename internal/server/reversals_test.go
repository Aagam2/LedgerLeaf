package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestPaidSaleReturnRestoresStockAndBalances(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":12000,"date":"2026-10-15","key":"return-opening-stock"}`, 200)
	bill := saveBill(t, call, draftInput("SALE", "2026-10-15", "return-sale-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, bill, 200)
	payment := fmt.Sprintf(`{"document_id":%d,"amount":5000,"date":"2026-10-15","mode":"CASH","key":"return-payment-key"}`, bill.ID)
	call("POST", "payments?business_id=1", payment, 200)
	body := fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Customer returned goods","mode":"CASH","key":"return-sale-request"}`, bill.ID)
	call("POST", "reversals?business_id=2", body, 400)
	call("POST", "reversals?business_id=1", body, 200)
	call("POST", "reversals?business_id=1", body, 200)
	call("POST", "reversals?business_id=1", strings.Replace(body, "return-sale-request", "another-return-key", 1), 400)
	call("POST", "payments?business_id=1", strings.Replace(payment, "return-payment-key", "after-return-payment", 1), 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 2000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 12000 {
		t.Fatal("inventory not restored")
	}
	for _, account := range []string{"1000", "1100", "4000", "5000"} {
		if scalar(t, s.DB, "SELECT COALESCE(SUM(debit-credit),0) FROM journal_lines WHERE account=?", account) != 0 {
			t.Fatal("unreversed balance", account)
		}
	}
	if !strings.Contains(call("GET", "documents?business_id=1", "", 200), "REVERSED") {
		t.Fatal("return not visible")
	}
	if !strings.Contains(call("GET", fmt.Sprintf("documents?business_id=1&id=%d", bill.ID), "", 200), "REVERSED") {
		t.Fatal("individual document hides return")
	}
	var snapshot string
	if err := s.DB.QueryRow("SELECT snapshot FROM documents WHERE id=?", bill.ID).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(snapshot, "REVERSED") {
		t.Fatal("original snapshot modified")
	}
	assertIntegrity(t, s.DB)
}
func TestPurchaseReturnInsufficientStockRollsBack(t *testing.T) {
	s, call := setupTrading(t)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "purchase-return-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 6000}))
	finish(t, call, purchase, 200)
	sale := saveBill(t, call, draftInput("SALE", "2026-10-15", "sale-return-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, sale, 200)
	body := fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Supplier return","mode":"CASH","key":"supplier-return-key"}`, purchase.ID)
	call("POST", "reversals?business_id=1", body, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM document_reversals") != 0 {
		t.Fatal("partial return persisted")
	}
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Sale cancellation","mode":"CASH","key":"sale-return-request"}`, sale.ID), 200)
	call("POST", "reversals?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 0 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 0 {
		t.Fatal("return left stock")
	}
	assertIntegrity(t, s.DB)
}
