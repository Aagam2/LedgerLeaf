package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func gstTrading(t *testing.T) (*Server, func(string, string, string, int) string) {
	s, call := setupTrading(t)
	call("POST", "businesses", `{"id":1,"name":"GST Shop","state_code":"27","gstin":"27AAPFU0939F1ZV","tax_scheme":"REGULAR"}`, 200)
	call("POST", "taxes?business_id=1", `{"product_id":1,"effective_date":"2026-01-01","hsn":"1001","treatment":"TAXABLE","rate":1800,"cess":0}`, 201)
	return s, call
}
func TestGSTSalePostingAndReturn(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":120000,"date":"2026-10-15","key":"gst-opening-stock"}`, 200)
	b := draftInput("SALE", "2026-10-15", "gst-invoice-draft", BillLine{ProductID: 1, Quantity: 2000, Price: 100000})
	b.PlaceOfSupply = "27"
	bill := saveBill(t, call, b)
	if bill.Total != 236000 || bill.TaxTotal != 36000 || bill.Lines[0].CGST != 18000 {
		t.Fatal(bill)
	}
	finish(t, call, bill, 200)
	for code, want := range map[string]int64{"1100": 236000, "4000": -200000, "2100": -18000, "2110": -18000, "5000": 120000} {
		if got := scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account=?", code); got != want {
			t.Fatal(code, got, want)
		}
	}
	if !strings.Contains(call("GET", "reports?business_id=1&kind=gst", "", 200), "18000") {
		t.Fatal("GST report missing tax")
	}
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Return GST goods","mode":"CASH","key":"gst-sale-return-key"}`, bill.ID), 200)
	for _, code := range []string{"1100", "4000", "2100", "2110", "5000"} {
		if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account=?", code) != 0 {
			t.Fatal("tax reversal", code)
		}
	}
	assertIntegrity(t, s.DB)
}
func TestGSTPurchaseCreditAndRCM(t *testing.T) {
	for _, rcm := range []bool{false, true} {
		for _, credit := range []bool{false, true} {
			t.Run(fmt.Sprint(rcm, credit), func(t *testing.T) {
				s, call := gstTrading(t)
				call("POST", "parties?business_id=1", `{"id":2,"kind":"SUPPLIER","name":"Supplier","active":1,"gstin":"27AAPFU0939F1ZV","state_code":"27"}`, 200)
				b := draftInput("PURCHASE", "2026-10-15", "gst-purchase-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 100000})
				b.PlaceOfSupply = "27"
				b.InputCredit = credit
				if rcm {
					b.TaxMode = "REVERSE_CHARGE"
				}
				bill := saveBill(t, call, b)
				finish(t, call, bill, 200)
				want := int64(118000)
				if credit {
					want = 100000
				}
				if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != want {
					t.Fatal("purchase valuation")
				}
				call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Supplier GST return","mode":"BANK","key":"gst-purchase-return"}`, bill.ID), 200)
				if scalar(t, s.DB, "SELECT COUNT(*) FROM (SELECT account FROM journal_lines GROUP BY account HAVING SUM(debit-credit)!=0)") != 0 {
					t.Fatal("purchase tax reversal")
				}
				assertIntegrity(t, s.DB)
			})
		}
	}
}
func TestTaxDraftFreezeAndInclusiveExport(t *testing.T) {
	s, call := gstTrading(t)
	b := draftInput("SALE", "2026-10-15", "gst-inclusive-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 11800})
	b.PlaceOfSupply = "29"
	b.Inclusive = true
	bill := saveBill(t, call, b)
	if bill.Total != 11800 || bill.Lines[0].IGST != 1800 || bill.Lines[0].Amount != 10000 {
		t.Fatal(bill)
	}
	call("POST", "taxes?business_id=1", `{"product_id":1,"effective_date":"2026-10-01","hsn":"1001","treatment":"TAXABLE","rate":1200,"cess":0}`, 201)
	finish(t, call, bill, 400)
	b.ID = 0
	b.Key = "gst-export-lut-key"
	b.TaxMode = "EXPORT_LUT"
	b.ShippingAddress = "Export destination"
	bill = saveBill(t, call, b)
	if bill.TaxTotal != 0 || bill.Total != 11800 {
		t.Fatal("LUT", bill)
	}
	call("POST", "taxes?business_id=1", `{"product_id":1,"effective_date":"2026-11-01","hsn":"1001","treatment":"EXEMPT","rate":1800,"cess":0}`, 400)
	var rows []map[string]interface{}
	if e := json.Unmarshal([]byte(call("GET", "taxes?business_id=2", "", 200)), &rows); e != nil || len(rows) != 0 {
		t.Fatal("tax isolation")
	}
	assertIntegrity(t, s.DB)
}

func TestPartialGSTReturnsRoundAndRefundExactly(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":3000,"value":10001,"date":"2026-10-15","key":"partial-gst-opening"}`, 200)
	b := draftInput("SALE", "2026-10-15", "partial-gst-invoice", BillLine{ProductID: 1, Quantity: 3000, Price: 3333})
	b.PlaceOfSupply = "27"
	b.Inclusive = true
	bill := saveBill(t, call, b)
	finish(t, call, bill, 200)
	call("POST", "payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"date":"2026-10-15","amount":%d,"mode":"CASH","key":"partial-gst-payment"}`, bill.ID, bill.Total), 200)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"Empty","mode":"CASH","key":"partial-empty-lines","lines":[]}`, bill.ID), 400)
	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"One unit","mode":"CASH","key":"partial-gst-return-%d","lines":[{"product_id":1,"quantity":1000}]}`, bill.ID, i)
		preview := strings.TrimSuffix(body, "}") + `,"preview":true}`
		before := scalar(t, s.DB, "SELECT COUNT(*) FROM journals")
		call("POST", "reversals?business_id=1", preview, 200)
		if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != before {
			t.Fatal("preview committed journal")
		}
		call("POST", "reversals?business_id=1", body, 200)
		call("POST", "reversals?business_id=1", body, 200)
		if i < 2 && scalar(t, s.DB, "SELECT COUNT(*) FROM document_reversals") != 0 {
			t.Fatal("partial return marked full")
		}
	}
	if scalar(t, s.DB, "SELECT SUM(total) FROM credit_notes") != bill.Total || scalar(t, s.DB, "SELECT SUM(refund) FROM credit_notes") != bill.Total {
		t.Fatal("credit/refund rounding mismatch")
	}
	if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 10001 {
		t.Fatal("cost not restored exactly")
	}
	for _, code := range []string{"1100", "1000", "4000", "2100", "2110", "5000"} {
		if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account=?", code) != 0 {
			t.Fatal("residual balance", code)
		}
	}
	call("GET", "credit-note-pdf?business_id=1&id=1", "", 200)
	call("GET", "reports?business_id=1&kind=gst", "", 200)
	assertIntegrity(t, s.DB)
}
