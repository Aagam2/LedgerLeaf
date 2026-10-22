package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestInvoiceAdjustmentsMixedTaxAndPartialReturns(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "taxes?business_id=1", `{"product_id":2,"effective_date":"2026-01-01","hsn":"1002","treatment":"TAXABLE","rate":500,"cess":0}`, 201)
	for i := 1; i <= 2; i++ {
		call("POST", "opening-stock?business_id=1", fmt.Sprintf(`{"product_id":%d,"warehouse_id":1,"quantity":3000,"value":9000,"date":"2026-10-15","key":"adjust-opening-%d"}`, i, i), 200)
	}
	b := draftInput("SALE", "2026-10-15", "adjust-sale-draft", BillLine{ProductID: 1, Quantity: 3000, Price: 10000}, BillLine{ProductID: 2, Quantity: 3000, Price: 5000})
	b.PlaceOfSupply = "27"
	b.InvoiceDiscount = 4500
	b.InvoiceCharge = 901
	b.RoundToRupee = true
	bill := saveBill(t, call, b)
	// 30,000:15,000 discount ratio, then 27,000:13,500 charges.
	if bill.Lines[0].InvoiceDiscount != 3000 || bill.Lines[1].InvoiceDiscount != 1500 || bill.Lines[0].InvoiceCharge != 601 || bill.Lines[1].InvoiceCharge != 300 || bill.Total != 47100 || bill.RoundOff != 41 {
		t.Fatal("incorrect mixed-rate adjustments", bill)
	}
	bill = finish(t, call, bill, 200)
	call("POST", "payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"amount":%d,"date":"2026-10-15","mode":"CASH","key":"adjust-sale-payment"}`, bill.ID, bill.Total), 200)
	for i := 0; i < 3; i++ {
		call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"one each","mode":"CASH","key":"adjust-sale-return-%d","lines":[{"product_id":1,"quantity":1000},{"product_id":2,"quantity":1000}]}`, bill.ID, i), 200)
	}
	if scalar(t, s.DB, "SELECT SUM(total) FROM credit_notes") != bill.Total || scalar(t, s.DB, "SELECT SUM(refund) FROM credit_notes") != bill.Total {
		t.Fatal("partial returns did not restore exact payable")
	}
	for _, code := range []string{"1100", "4000", "5300", "2100", "2110", "1000", "5000"} {
		if scalar(t, s.DB, "SELECT COALESCE(SUM(debit-credit),0) FROM journal_lines WHERE account=?", code) != 0 {
			t.Fatal("return left balance", code)
		}
	}
	if scalar(t, s.DB, `SELECT SUM(json_extract(snapshot,'$.invoice_discount')) FROM credit_notes`) != 4500 || scalar(t, s.DB, `SELECT SUM(json_extract(snapshot,'$.invoice_charge')) FROM credit_notes`) != 901 {
		t.Fatal("return adjustment allocation")
	}
	for _, paper := range []string{"", "58", "80"} {
		call("GET", fmt.Sprintf("document-pdf?business_id=1&id=%d&paper=%s", bill.ID, paper), "", 200)
	}
	assertIntegrity(t, s.DB)
}

func TestInvoiceAdjustmentPurchasesInclusiveAndRCM(t *testing.T) {
	for _, mode := range []string{"NORMAL", "REVERSE_CHARGE"} {
		for _, inclusive := range []bool{false, true} {
			for _, credit := range []bool{false, true} {
				t.Run(fmt.Sprint(mode, inclusive, credit), func(t *testing.T) {
					s, call := gstTrading(t)
					call("POST", "parties?business_id=1", `{"id":2,"kind":"SUPPLIER","name":"Supplier","active":1,"gstin":"27AAPFU0939F1ZV","state_code":"27"}`, 200)
					b := draftInput("PURCHASE", "2026-10-15", "adjust-purchase-draft", BillLine{ProductID: 1, Quantity: 3000, Price: 10001})
					b.PlaceOfSupply = "27"
					b.TaxMode = mode
					b.Inclusive = inclusive
					b.InputCredit = credit
					b.InvoiceDiscount = 100
					b.InvoiceCharge = 48
					b.RoundToRupee = true
					bill := saveBill(t, call, b)
					finish(t, call, bill, 200)
					if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != bill.inventoryTotal() {
						t.Fatal("rounding leaked into inventory")
					}
					for i := 0; i < 3; i++ {
						call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"return","mode":"CASH","key":"adjust-purchase-return-%d","lines":[{"product_id":1,"quantity":1000}]}`, bill.ID, i), 200)
					}
					if scalar(t, s.DB, "SELECT COUNT(*) FROM (SELECT account FROM journal_lines GROUP BY account HAVING SUM(debit-credit)!=0)") != 0 {
						t.Fatal("purchase return left balances")
					}
					assertIntegrity(t, s.DB)
				})
			}
		}
	}
}

func TestInvoiceAdjustmentValidationAndClientFields(t *testing.T) {
	s, call := setupTrading(t)
	b := draftInput("SALE", "2026-10-15", "adjust-validation", BillLine{ProductID: 1, Quantity: 1000, Price: 10000, InvoiceDiscount: 999, InvoiceCharge: 999, RoundOff: 999})
	b.RoundOff = 999
	bill := saveBill(t, call, b)
	if bill.Total != 10000 || bill.RoundOff != 0 || bill.Lines[0].InvoiceCharge != 0 || bill.Lines[0].InvoiceDiscount != 0 || bill.Lines[0].RoundOff != 0 {
		t.Fatal("trusted computed fields", bill)
	}
	for i, change := range []func(*Bill){func(b *Bill) { b.InvoiceDiscount = -1 }, func(b *Bill) { b.InvoiceDiscount = 10001 }, func(b *Bill) { b.InvoiceCharge = -1 }, func(b *Bill) { b.InvoiceCharge = 1000000000000 }, func(b *Bill) { b.InvoiceDiscount = 10000; b.InvoiceCharge = 1 }, func(b *Bill) { b.Lines[0].Price = 49; b.RoundToRupee = true }} {
		candidate := draftInput("SALE", "2026-10-15", fmt.Sprintf("adjust-invalid-%02d", i), BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
		change(&candidate)
		call("POST", "documents?business_id=1", jsonText(t, candidate), 400)
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents") != 1 {
		t.Fatal("invalid adjustments persisted")
	}
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Owner-Password-123!","role":"ADMIN","active":1}`, 200)
	owner("POST", "auth/users", `{"username":"biller","password":"Biller-Password-123!","role":"BILLER","active":1}`, 200)
	clerk := authClient(t, s)
	clerk("POST", "auth/login", `{"username":"biller","password":"Biller-Password-123!"}`, 200)
	b.InvoiceDiscount = 1
	b.Key = "adjust-biller-denied"
	clerk("POST", "documents?business_id=1", jsonText(t, b), 403)
	assertIntegrity(t, s.DB)
}

func TestAdjustedOrderConversionAndReceipt(t *testing.T) {
	s, call := setupTrading(t)
	b := draftInput("PURCHASE", "2026-10-15", "adjust-order-draft", BillLine{ProductID: 1, Quantity: 3000, Price: 10001})
	b.DocumentType = "PURCHASE_ORDER"
	b.InvoiceDiscount = 3000
	b.InvoiceCharge = 301
	b.RoundToRupee = true
	var order Bill
	if e := json.Unmarshal([]byte(call("POST", "commercial-documents?business_id=1", jsonText(t, b), 200)), &order); e != nil {
		t.Fatal(e)
	}
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, order.ID), 200)
	call("GET", fmt.Sprintf("commercial-pdf?business_id=1&id=%d", order.ID), "", 200)
	call("POST", "receipts?business_id=1", fmt.Sprintf(`{"order_id":%d,"warehouse_id":1,"date":"2026-10-15","key":"adjust-order-receipt","lines":[{"product_id":1,"quantity":3000}]}`, order.ID), 200)
	b.ID = 0
	b.SourceID = 0
	b.DocumentType = ""
	b.ReceiptID = 1
	b.Key = "adjust-order-invoice"
	bill := saveBill(t, call, b)
	finish(t, call, bill, 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 3000 || scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='2050'") != 0 {
		t.Fatal("receipt matching")
	}
	assertIntegrity(t, s.DB)
}
