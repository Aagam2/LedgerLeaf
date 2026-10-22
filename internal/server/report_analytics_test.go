package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestProductAndHSNSummariesReconcileConvertedPartialReturns(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "units?business_id=1", `{"product_id":1,"name":"BOX","factor":12000}`, 200)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":24000,"value":120000,"date":"2026-10-15","key":"analytics-opening"}`, 200)
	in := draftInput("SALE", "2026-10-15", "analytics-sale-key", BillLine{ProductID: 1, Quantity: 2000, Price: 120000, SaleUnit: "BOX"})
	in.PlaceOfSupply = "27"
	bill := saveBill(t, call, in)
	finish(t, call, bill, 200)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reason":"half box","mode":"CASH","key":"analytics-return-key","lines":[{"product_id":1,"quantity":500}]}`, bill.ID), 200)
	check := func(kind, from, to string, want map[string]int64) {
		t.Helper()
		var rows []map[string]interface{}
		if e := json.Unmarshal([]byte(call("GET", "reports?business_id=1&kind="+kind+"&from="+from+"&to="+to, "", 200)), &rows); e != nil || len(rows) != 1 {
			t.Fatal(rows, e)
		}
		if rows[0]["unit"] != "PCS" {
			t.Fatal("summary not normalized to base unit", rows)
		}
		for key, n := range want {
			if rows[0][key] != float64(n) {
				t.Fatal(kind, key, rows[0][key], n)
			}
		}
	}
	check("product-sales", "2026-10-15", "2026-10-16", map[string]int64{"quantity": 18000, "net_sales": 180000, "cost_of_sales": 90000, "gross_profit": 90000})
	check("hsn-summary", "2026-10-15", "2026-10-16", map[string]int64{"quantity": 18000, "taxable_value": 180000, "cgst": 16200, "sgst": 16200})
	check("product-sales", "2026-10-16", "2026-10-16", map[string]int64{"quantity": -6000, "net_sales": -60000, "cost_of_sales": -30000, "gross_profit": -30000})
	check("hsn-summary", "2026-10-15", "2026-10-15", map[string]int64{"quantity": 24000, "taxable_value": 240000, "cgst": 21600})
	if got := call("GET", "reports?business_id=2&kind=product-sales", "", 200); strings.TrimSpace(got) != "[]" {
		t.Fatal("cross-business data", got)
	}
	for _, kind := range []string{"hsn-summary", "product-sales"} {
		for _, format := range []string{"csv", "xlsx", "pdf"} {
			got := call("GET", "reports?business_id=1&kind="+kind+"&format="+format, "", 200)
			if len(got) < 20 {
				t.Fatal("empty export", kind, format)
			}
		}
	}
	assertIntegrity(t, s.DB)
}

func TestCustomerAndMonthlySalesReturnPeriods(t *testing.T) {
	_, call := gstTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":10000,"date":"2026-09-30","key":"customer-report-opening"}`, 200)
	in := draftInput("SALE", "2026-09-30", "customer-report-sale", BillLine{ProductID: 1, Quantity: 2000, Price: 10000})
	in.PlaceOfSupply = "27"
	bill := finish(t, call, saveBill(t, call, in), 200)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-01","reason":"half returned","mode":"CASH","key":"customer-report-return","lines":[{"product_id":1,"quantity":1000}]}`, bill.ID), 200)
	for _, kind := range []string{"customer-sales", "monthly-sales"} {
		var rows []map[string]interface{}
		if e := json.Unmarshal([]byte(call("GET", "reports?business_id=1&kind="+kind+"&from=2026-10-01&to=2026-10-31", "", 200)), &rows); e != nil || len(rows) != 1 {
			t.Fatal(rows, e)
		}
		for k, want := range map[string]float64{"invoices": 0, "credit_notes": 1, "net_sales": -10000, "cost_of_sales": -5000, "gross_profit": -5000} {
			if rows[0][k] != want {
				t.Fatal(kind, k, rows[0][k], want)
			}
		}
		if got := call("GET", "reports?business_id=2&kind="+kind, "", 200); strings.TrimSpace(got) != "[]" {
			t.Fatal("cross-business report", got)
		}
		for _, format := range []string{"csv", "xlsx", "pdf"} {
			if got := call("GET", "reports?business_id=1&kind="+kind+"&format="+format, "", 200); len(got) < 20 {
				t.Fatal("empty export", kind, format)
			}
		}
	}
}

func TestLowStockUsesClosingDateAndAllWarehouses(t *testing.T) {
	s, call := setupTrading(t)
	if _, e := s.DB.Exec("UPDATE products SET minimum_stock=3000 WHERE business_id=1 AND id=1"); e != nil {
		t.Fatal(e)
	}
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":10000,"date":"2026-10-01","key":"low-stock-opening"}`, 200)
	for _, c := range []struct {
		date               string
		quantity, shortage float64
	}{{"2026-09-30", 0, 3000}, {"2026-10-01", 2000, 1000}} {
		var rows []map[string]interface{}
		if e := json.Unmarshal([]byte(call("GET", "reports?business_id=1&kind=low-stock&to="+c.date, "", 200)), &rows); e != nil || len(rows) != 1 {
			t.Fatal(rows, e)
		}
		if rows[0]["quantity"] != c.quantity || rows[0]["shortage"] != c.shortage {
			t.Fatal(rows)
		}
	}
	call("POST", "warehouses?business_id=1", `{"name":"Second warehouse"}`, 200)
	warehouse := scalar(t, s.DB, "SELECT MAX(id) FROM warehouses WHERE business_id=1")
	call("POST", "opening-stock?business_id=1", fmt.Sprintf(`{"product_id":1,"warehouse_id":%d,"quantity":1000,"value":5000,"date":"2026-10-01","key":"low-stock-second"}`, warehouse), 200)
	if got := call("GET", "reports?business_id=1&kind=low-stock&to=2026-10-01", "", 200); strings.TrimSpace(got) != "[]" {
		t.Fatal("threshold should combine warehouses", got)
	}
}
