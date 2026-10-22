package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestStockTransferDamageAndAddReconcile(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "warehouses?business_id=1", `{"name":"Branch"}`, 200)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":10000,"value":60000,"date":"2026-10-15","key":"operation-opening"}`, 200)
	transfer := `{"product_id":1,"warehouse_id":1,"target_id":3,"kind":"TRANSFER","quantity":3000,"date":"2026-10-16","reason":"Branch replenishment","key":"stock-transfer-1234"}`
	call("POST", "stock-operation?business_id=1", strings.Replace(transfer, `"target_id":3`, `"target_id":2`, 1), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != 1 {
		t.Fatal("foreign transfer partially committed")
	}
	call("POST", "stock-operation?business_id=1", transfer, 200)
	call("POST", "stock-operation?business_id=1", transfer, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements WHERE warehouse_id=3") != 3000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 60000 {
		t.Fatal("transfer lost stock value")
	}
	call("POST", "stock-operation?business_id=1", `{"product_id":1,"warehouse_id":3,"kind":"DAMAGE","quantity":1000,"date":"2026-10-16","reason":"Damaged packaging","key":"damage-operation-1234"}`, 200)
	call("POST", "stock-operation?business_id=1", `{"product_id":1,"warehouse_id":3,"kind":"ADD","quantity":1000,"value":8000,"date":"2026-10-16","reason":"Count correction","key":"added-operation-1234"}`, 200)
	if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 62000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1200'") != 62000 {
		t.Fatal("stock ledger and inventory account diverged")
	}
	assertIntegrity(t, s.DB)
}
func TestExpensesAndReportsReconcile(t *testing.T) {
	s, call := setupTrading(t)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "reports-purchase-key", BillLine{ProductID: 1, Quantity: 1000, Price: 6000}))
	finish(t, call, purchase, 200)
	sale := saveBill(t, call, draftInput("SALE", "2026-10-15", "reports-sale-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, sale, 200)
	body := `{"date":"2026-10-15","description":"Delivery cost","amount":1000,"account":"1000","key":"expense-report-key"}`
	call("POST", "expenses?business_id=1", body, 200)
	call("POST", "expenses?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals WHERE reference LIKE 'expense:%'") != 1 {
		t.Fatal("duplicate expense")
	}
	var rows []struct {
		Kind    string `json:"kind"`
		Balance int64  `json:"balance"`
	}
	if e := json.Unmarshal([]byte(call("GET", "report-summary?business_id=1&kind=profit-loss&from=2026-10-01&to=2026-10-31", "", 200)), &rows); e != nil {
		t.Fatal(e)
	}
	var profit int64
	for _, r := range rows {
		profit -= r.Balance
	}
	if profit != 3000 {
		t.Fatal("profit", profit)
	}
	if got := call("GET", "reports?business_id=2&kind=ledger", "", 200); strings.TrimSpace(got) != "[]" {
		t.Fatal(got)
	}
	csv := call("GET", "reports?business_id=1&kind=ledger&format=csv", "", 200)
	if !strings.Contains(csv, "Delivery cost") || !strings.Contains(csv, sale.Number) {
		t.Fatal("CSV missing postings")
	}
	call("GET", "reports?business_id=1&kind=ledger&from=2026-10-20&to=2026-10-01", "", 400)
	assertIntegrity(t, s.DB)
}
func TestReportCSVProtectsFormulaCells(t *testing.T) {
	_, call := setupTrading(t)
	call("POST", "expenses?business_id=1", `{"date":"2026-10-15","description":"=HYPERLINK(\"malicious\")","amount":100,"account":"1000","key":"formula-expense-key"}`, 200)
	csv := call("GET", "reports?business_id=1&kind=ledger&format=csv", "", 200)
	if !strings.Contains(csv, "'=HYPERLINK") {
		t.Fatal(fmt.Sprintf("unsafe CSV: %s", csv))
	}
}

func TestReportExportsAndPartyReconciliation(t *testing.T) {
	s, call := setupTrading(t)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "report-export-purchase", BillLine{ProductID: 1, Quantity: 1000, Price: 6000}))
	finish(t, call, purchase, 200)
	sale := saveBill(t, call, draftInput("SALE", "2026-10-15", "report-export-sale", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, sale, 200)
	call("POST", "payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"date":"2026-10-15","amount":2500,"mode":"CASH","key":"report-export-payment"}`, sale.ID), 200)
	for _, kind := range []string{"party-statement", "ageing", "gst", "trial-balance"} {
		call("GET", "reports?business_id=1&kind="+kind, "", 200)
	}
	if !strings.Contains(call("GET", "reports?business_id=1&kind=party-statement&party_id=1", "", 200), "7500") {
		t.Fatal("party balance mismatch")
	}
	xlsx := call("GET", "reports?business_id=1&kind=ageing&format=xlsx", "", 200)
	if !strings.HasPrefix(xlsx, "PK") {
		t.Fatal("invalid XLSX")
	}
	pdf := call("GET", "reports?business_id=1&kind=ageing&format=pdf", "", 200)
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Fatal("invalid report PDF")
	}
	call("GET", "dashboard?business_id=1", "", 200)
	call("GET", "dashboard?business_id=1&series=monthly", "", 200)
	assertIntegrity(t, s.DB)
}
