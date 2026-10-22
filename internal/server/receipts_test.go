package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPartialGoodsReceiptsAndInvoiceMatching(t *testing.T) {
	s, call := setupTrading(t)
	order := draftInput("PURCHASE", "2026-10-15", "order-partial-receipts", BillLine{ProductID: 1, Quantity: 3000, Price: 6000})
	order.DocumentType = "PURCHASE_ORDER"
	var saved Bill
	if e := json.Unmarshal([]byte(call("POST", "commercial-documents?business_id=1", jsonText(t, order), 200)), &saved); e != nil {
		t.Fatal(e)
	}
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, saved.ID), 200)
	for i, q := range []int64{1000, 2000} {
		body := fmt.Sprintf(`{"order_id":%d,"warehouse_id":1,"date":"2026-10-15","key":"partial-goods-receipt-%d","lines":[{"product_id":1,"quantity":%d}]}`, saved.ID, i, q)
		call("POST", "receipts?business_id=1", body, 200)
		call("POST", "receipts?business_id=1", body, 200)
	}
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 18000 {
		t.Fatal("receipt valuation")
	}
	call("POST", "receipts?business_id=1", fmt.Sprintf(`{"order_id":%d,"warehouse_id":1,"date":"2026-10-15","key":"excess-goods-receipt","lines":[{"product_id":1,"quantity":1}]}`, saved.ID), 400)
	b := draftInput("PURCHASE", "2026-10-16", "receipt-invoice-key", BillLine{ProductID: 1, Quantity: 1000, Price: 6500})
	b.ReceiptID = 1
	invoice := saveBill(t, call, b)
	b.Key = "duplicate-receipt-invoice"
	call("POST", "documents?business_id=1", jsonText(t, b), 400)
	edited := invoice
	edited.ReceiptID = 0
	edited.Key = "detach-receipt-key"
	call("POST", "documents?business_id=1", jsonText(t, edited), 400)
	finish(t, call, invoice, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 {
		t.Fatal("invoice received stock twice")
	}
	if scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='2050'") != 12000 {
		t.Fatal("receipt liability mismatch")
	}
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='5100'") != 500 {
		t.Fatal("purchase variance missing")
	}
	call("GET", "receipts?business_id=1", "", 200)
	call("GET", fmt.Sprintf("commercial-documents?business_id=1&id=%d", saved.ID), "", 200)
	assertIntegrity(t, s.DB)
}
func TestCreditLimitAndPriceLists(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "parties?business_id=1", `{"id":1,"name":"Buyer","kind":"CUSTOMER","active":1,"credit_limit":5000,"credit_days":30}`, 200)
	call("POST", "prices?business_id=1", `{"product_id":1,"name":"Wholesale","price":6000}`, 200)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":1000,"value":4000,"date":"2026-10-15","key":"price-list-opening"}`, 200)
	b := draftInput("SALE", "2026-10-15", "price-list-draft-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
	b.PriceList = "Wholesale"
	bill := saveBill(t, call, b)
	if bill.Total != 6000 {
		t.Fatal("price list ignored")
	}
	finish(t, call, bill, 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 {
		t.Fatal("credit limit failure changed stock")
	}
	call("POST", "parties?business_id=1", `{"id":1,"name":"Buyer","kind":"CUSTOMER","active":1,"credit_limit":6000}`, 200)
	finish(t, call, bill, 200)
	assertIntegrity(t, s.DB)
}
