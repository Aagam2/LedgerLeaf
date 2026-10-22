package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestFinancialSettingsVariantsAndClosing(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "business-settings?business_id=1", `{"year_start_month":1}`, 200)
	call("POST", "variants?business_id=1", `{"name":"Shirts"}`, 200)
	call("POST", "variants?business_id=1", `{"product_id":1,"group_id":1,"attributes":{"Color":" Blue ","size":"L"}}`, 200)
	call("POST", "variants?business_id=1", `{"product_id":2,"group_id":1,"attributes":{"color":"blue","size":"l"}}`, 400)
	call("POST", "variants?business_id=2", `{"product_id":1,"group_id":1,"attributes":{"color":"red"}}`, 400)
	call("POST", "expenses?business_id=1", `{"date":"2025-12-15","description":"Rent","category":"Office","amount":10000,"account":"1000","key":"year-close-expense"}`, 200)
	call("POST", "business-settings?business_id=1", `{"year_start_month":4}`, 400)
	call("POST", "year-close?business_id=1", `{"date":"2025-12-31","confirm":"CLOSE FINANCIAL YEAR"}`, 200)
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='5200'") != 0 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='3100'") != 10000 {
		t.Fatal("year closing did not transfer loss")
	}
	call("POST", "period-lock?business_id=1", `{"closed_through":""}`, 400)
	var summary []struct {
		Kind    string
		Balance int64
	}
	json.Unmarshal([]byte(call("GET", "report-summary?business_id=1&kind=profit-loss&from=2025-01-01&to=2025-12-31", "", 200)), &summary)
	if len(summary) != 1 || summary[0].Balance != 10000 {
		t.Fatal("closing changed historical profit", summary)
	}
	call("POST", "year-close?business_id=1", `{"date":"2025-12-31","confirm":"CLOSE FINANCIAL YEAR"}`, 400)
	in := draftInput("PURCHASE", "2026-01-01", "calendar-year-invoice", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
	bill := finish(t, call, saveBill(t, call, in), 200)
	if !strings.HasPrefix(bill.Number, "PURCHASE/2026/") {
		t.Fatal(bill.Number)
	}
	assertIntegrity(t, s.DB)
}
func TestRecurringExpenseCategoryIdempotencyAndMonthEnds(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "recurring-expenses?business_id=1", `{"description":"Monthly rent","category":"Office","amount":12345,"account":"1000","next_date":"2026-01-31","interval_months":1,"active":1}`, 200)
	p := `{"id":1,"next_date":"2026-01-31","key":"recurring-first-post"}`
	call("POST", "recurring-expense-post?business_id=1", p, 200)
	call("POST", "recurring-expense-post?business_id=1", p, 200)
	var date string
	s.DB.QueryRow("SELECT next_date FROM recurring_expenses").Scan(&date)
	if date != "2026-02-28" {
		t.Fatal(date)
	}
	call("POST", "recurring-expense-post?business_id=1", `{"id":1,"next_date":"2026-02-28","key":"recurring-second-post"}`, 200)
	s.DB.QueryRow("SELECT next_date FROM recurring_expenses").Scan(&date)
	if date != "2026-03-31" {
		t.Fatal("anchor day lost", date)
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM expense_details WHERE category='Office'") != 2 {
		t.Fatal("duplicate or missing expenses")
	}
	call("POST", "recurring-expense-post?business_id=1", `{"id":1,"next_date":"2026-02-28","key":"recurring-stale-post"}`, 400)
	assertIntegrity(t, s.DB)
}
func TestQuantityCessInclusiveUnitsAndReturns(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "taxes?business_id=1", `{"product_id":1,"effective_date":"2026-10-01","hsn":"1001","treatment":"TAXABLE","rate":1800,"cess":0,"cess_per_unit":250}`, 201)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":12000,"value":60000,"date":"2026-10-15","key":"quantity-cess-stock"}`, 200)
	in := draftInput("SALE", "2026-10-15", "quantity-cess-invoice", BillLine{ProductID: 1, Quantity: 1000, Price: 144600, SaleUnit: "BOX"})
	in.Inclusive = true
	in.PlaceOfSupply = "27"
	bill := saveBill(t, call, in)
	if bill.Total != 144600 || bill.Lines[0].Amount != 120000 || bill.Lines[0].Cess != 3000 {
		t.Fatal(bill)
	}
	finish(t, call, bill, 200)
	for i := 0; i < 2; i++ {
		call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"half box","mode":"CASH","key":"quantity-cess-return-%d","lines":[{"product_id":1,"quantity":500}]}`, bill.ID, i), 200)
	}
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='2130'") != 0 {
		t.Fatal("cess return balance")
	}
	assertIntegrity(t, s.DB)
}

func TestDispatchInvoiceReturnAndRollback(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":3000,"value":18000,"date":"2026-10-15","key":"dispatch-opening-stock"}`, 200)
	call("POST", "dispatches?business_id=1", `{"party_id":1,"warehouse_id":1,"date":"2026-10-15","key":"independent-dispatch","lines":[{"product_id":1,"quantity":2000}]}`, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1250'") != 12000 {
		t.Fatal("dispatch did not move stock")
	}
	call("GET", "dispatches?business_id=1&id=1&pdf=1", "", 200)
	in := draftInput("SALE", "2026-10-15", "dispatch-invoice-draft", BillLine{ProductID: 1, Quantity: 2000, Price: 10000})
	in.DispatchID = 1
	bill := saveBill(t, call, in)
	finish(t, call, bill, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1250'") != 0 {
		t.Fatal("invoice double-dispatched")
	}
	call("POST", "dispatch-return?business_id=1", `{"id":1,"date":"2026-10-15","key":"invoiced-dispatch-return"}`, 400)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"customer return","mode":"CASH","key":"dispatch-invoice-return"}`, bill.ID), 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 18000 {
		t.Fatal("invoiced return lost original cost")
	}
	call("POST", "dispatches?business_id=1", `{"party_id":1,"warehouse_id":1,"date":"2026-10-15","key":"second-dispatch-test","lines":[{"product_id":1,"quantity":1000}]}`, 200)
	call("POST", "dispatch-return?business_id=1", `{"id":2,"date":"2026-10-15","key":"second-dispatch-return"}`, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 {
		t.Fatal("unbilled return")
	}
	assertIntegrity(t, s.DB)
}
func TestFinancialJournalImportPreviewAndAtomicity(t *testing.T) {
	s, call := setupTrading(t)
	csv := "reference,date,description,account,debit,credit\nrent01,2026-10-15,Rent,5200,10000,0\nrent01,2026-10-15,Rent,1000,0,10000\n"
	path := "import-journals?business_id=1&format=csv&key=financial-import-key"
	call("POST", path+"&preview=1", csv, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 0 {
		t.Fatal("preview wrote journals")
	}
	call("POST", path, csv, 200)
	call("POST", path, csv, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 1 {
		t.Fatal("import not idempotent")
	}
	bad := strings.ReplaceAll(csv, "rent01", "rent02") + "bad,2026-10-15,Unbalanced,5200,200,0\nbad,2026-10-15,Unbalanced,1000,0,100\n"
	call("POST", path+"-invalid", bad, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 1 {
		t.Fatal("partial financial import")
	}
	assertIntegrity(t, s.DB)
}
