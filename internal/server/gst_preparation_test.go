package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGSTPreparationClassificationReturnsAndExports(t *testing.T) {
	s, call := gstTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":10000,"date":"2026-10-15","key":"gst-prep-opening"}`, 200)
	in := draftInput("SALE", "2026-10-15", "gst-prep-large-invoice", BillLine{ProductID: 1, Quantity: 1000, Price: 10000001})
	in.PlaceOfSupply = "29"
	bill := finish(t, call, saveBill(t, call, in), 200)
	r := call("GET", "reports?business_id=1&kind=gstr-1-preparation&from=2026-10-01&to=2026-10-31", "", 200)
	if !strings.Contains(r, "5 - B2CL") {
		t.Fatal(r)
	}
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-11-01","reason":"Return","mode":"CASH","key":"gst-preparation-return"}`, bill.ID), 200)
	r = call("GET", "reports?business_id=1&kind=gstr-1-preparation&from=2026-11-01&to=2026-11-30", "", 200)
	if !strings.Contains(r, "9B - CDNUR") || !strings.Contains(r, `"taxable_value":-10000001`) {
		t.Fatal(r)
	}
	r = call("GET", "reports?business_id=1&kind=gstr-3b-preparation&from=2026-11-01&to=2026-11-30", "", 200)
	if !strings.Contains(r, "3.1(a)") || !strings.Contains(r, "3.2 - Interstate unregistered") || !strings.Contains(r, `"taxable_value":-10000001`) {
		t.Fatal(r)
	}
	for _, kind := range []string{"gstr-1-preparation", "gstr-3b-preparation"} {
		for _, format := range []string{"csv", "xlsx", "pdf"} {
			r = call("GET", "reports?business_id=1&kind="+kind+"&format="+format, "", 200)
			if len(r) < 20 {
				t.Fatal("empty export")
			}
		}
	}
	call("GET", "reports?business_id=2&kind=gstr-1-preparation", "", 400)
	assertIntegrity(t, s.DB)
}
func TestGSTPreparationITCCandidates(t *testing.T) {
	_, call := gstTrading(t)
	call("POST", "parties?business_id=1", `{"id":2,"kind":"SUPPLIER","name":"Supplier","active":1,"state_code":"27"}`, 200)
	in := draftInput("PURCHASE", "2026-10-15", "gst-prep-rcm-purchase", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
	in.PlaceOfSupply = "27"
	in.TaxMode = "REVERSE_CHARGE"
	in.InputCredit = true
	finish(t, call, saveBill(t, call, in), 200)
	var rows []map[string]interface{}
	raw := call("GET", "reports?business_id=1&kind=gstr-3b-preparation", "", 200)
	if e := json.Unmarshal([]byte(raw), &rows); e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, r := range rows {
		found[r["section"].(string)] = true
		if r["cgst"].(float64) != 900 {
			t.Fatal(r)
		}
	}
	if !found["3.1(d) - Inward reverse charge"] || !found["4(A)(3) - Booked RCM ITC candidate"] {
		t.Fatal(raw)
	}
}
