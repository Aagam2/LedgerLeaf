package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAlternateUnitsStockReturnsAndExactMoney(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"box","factor":24000}`, 400)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "unit-purchase-key", BillLine{ProductID: 1, Quantity: 2000, Price: 10001, SaleUnit: "BOX", UnitFactor: 999, BaseUnit: "FAKE"}))
	if purchase.Total != 20002 || purchase.Lines[0].UnitFactor != 12000 || purchase.Lines[0].BaseUnit != "PCS" {
		t.Fatal("incorrect unit pricing or trusted client factor", purchase)
	}
	finish(t, call, purchase, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 24000 {
		t.Fatal("purchase stock not converted")
	}
	sale := saveBill(t, call, draftInput("SALE", "2026-10-15", "unit-sale-test-key", BillLine{ProductID: 1, Quantity: 1000, Price: 15001, SaleUnit: "BOX"}))
	finish(t, call, sale, 200)
	for i := 0; i < 2; i++ {
		call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"half box","mode":"CASH","key":"unit-return-test-%d","lines":[{"product_id":1,"quantity":500}]}`, sale.ID, i), 200)
	}
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 24000 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 20002 {
		t.Fatal("partial unit returns lost stock/value")
	}
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"supplier return","mode":"CASH","key":"unit-supplier-return"}`, purchase.ID), 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 0 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 0 {
		t.Fatal("purchase return not converted")
	}
	call("GET", fmt.Sprintf("document-pdf?business_id=1&id=%d", sale.ID), "", 200)
	assertIntegrity(t, s.DB)
}
func TestUnitPrecisionAndConfigurationProtection(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"SMALL","factor":1}`, 200)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"PCS","factor":1000}`, 400)
	call("POST", "units?business_id=2", `{"product_id":1,"name":"BOX","factor":12000}`, 400)
	in := draftInput("PURCHASE", "2026-10-15", "unit-precision-key", BillLine{ProductID: 1, Quantity: 500, Price: 100, SaleUnit: "SMALL"})
	call("POST", "documents?business_id=1", jsonText(t, in), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents") != 0 {
		t.Fatal("invalid conversion persisted")
	}
	in.Lines[0].Quantity = 1000
	bill := saveBill(t, call, in)
	finish(t, call, bill, 200)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"fraction","mode":"CASH","key":"unit-precision-return","preview":true,"lines":[{"product_id":1,"quantity":500}]}`, bill.ID), 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1 {
		t.Fatal("precision failure mutated stock")
	}
	if _, e := s.DB.Exec("UPDATE product_units SET factor=2"); e == nil {
		t.Fatal("conversion changed")
	}
	call("POST", "products?business_id=1", `{"id":1,"name":"Changed","sku":"ITEM","kind":"PRODUCT","unit":"KG","sale_price":10000,"active":1}`, 400)
	assertIntegrity(t, s.DB)
}
func TestAlternateUnitReceiptMatchesBaseUnitInvoice(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	in := draftInput("PURCHASE", "2026-10-15", "unit-purchase-order", BillLine{ProductID: 1, Quantity: 2000, Price: 12000, SaleUnit: "BOX"})
	in.DocumentType = "PURCHASE_ORDER"
	var order Bill
	if e := json.Unmarshal([]byte(call("POST", "commercial-documents?business_id=1", jsonText(t, in), 200)), &order); e != nil {
		t.Fatal(e)
	}
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, order.ID), 200)
	call("POST", "receipts?business_id=1", fmt.Sprintf(`{"order_id":%d,"warehouse_id":1,"date":"2026-10-15","key":"unit-receipt-test","lines":[{"product_id":1,"quantity":1000}]}`, order.ID), 200)
	in = draftInput("PURCHASE", "2026-10-15", "unit-receipt-invoice", BillLine{ProductID: 1, Quantity: 12000, Price: 1000})
	in.ReceiptID = 1
	bill := saveBill(t, call, in)
	finish(t, call, bill, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 12000 {
		t.Fatal("matched invoice duplicated stock")
	}
	if scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='2050'") != 0 {
		t.Fatal("receipt liability not cleared")
	}
	assertIntegrity(t, s.DB)
}

func TestAlternateUnitGSTAndBillerPrices(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":12000,"value":60000,"date":"2026-10-15","key":"unit-gst-opening"}`, 200)
	b := draftInput("SALE", "2026-10-15", "unit-gst-invoice", BillLine{ProductID: 1, Quantity: 1000, Price: 141600, SaleUnit: "BOX"})
	b.Inclusive = true
	b.PlaceOfSupply = "27"
	bill := saveBill(t, call, b)
	if bill.Total != 141600 || bill.TaxTotal != 21600 {
		t.Fatal("unit tax incorrect", bill)
	}
	finish(t, call, bill, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 0 {
		t.Fatal("converted sale stock")
	}
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Owner-Password-123!","role":"ADMIN","active":1}`, 200)
	owner("POST", "auth/users", `{"username":"biller","password":"Biller-Password-123!","role":"BILLER","active":1}`, 200)
	biller := authClient(t, s)
	biller("POST", "auth/login", `{"username":"biller","password":"Biller-Password-123!"}`, 200)
	b.Key = "unit-biller-price-key"
	b.Inclusive = false
	b.Lines[0].Price = 120000
	biller("POST", "documents?business_id=1", jsonText(t, b), 200)
	b.Key = "unit-biller-price-bad"
	b.Lines[0].Price = 10000
	biller("POST", "documents?business_id=1", jsonText(t, b), 403)
	biller("POST", "units?business_id=1", `{"product_id":1,"name":"CASE","factor":24000}`, 403)
	assertIntegrity(t, s.DB)
}
