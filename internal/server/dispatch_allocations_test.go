package server

import (
	"fmt"
	"testing"
)

func TestDispatchCumulativeCost(t *testing.T) {
	row := dispatchBalance{Quantity: 3000, Remaining: 3000, Cost: 100}
	for _, want := range []int64{33, 34, 33} {
		got, e := dispatchAllocatedCost(row, 1000)
		if e != nil || got != want {
			t.Fatal(got, want, e)
		}
		row.Used += 1000
		row.Remaining -= 1000
		row.UsedCost += got
	}
	if _, e := dispatchAllocatedCost(row, 1); e == nil {
		t.Fatal("accepted exhausted dispatch")
	}
}

func TestPartialDispatchInvoiceReturnAndStaleDraft(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":3000,"value":100,"date":"2026-10-15","key":"partial-dispatch-opening"}`, 200)
	call("POST", "dispatches?business_id=1", `{"party_id":1,"warehouse_id":1,"date":"2026-10-15","key":"partial-dispatch","lines":[{"product_id":1,"quantity":3000}]}`, 200)
	draft := func(q int64, key string) Bill {
		in := draftInput("SALE", "2026-10-15", key, BillLine{ProductID: 1, Quantity: q, Price: 10000})
		in.DispatchID = 1
		return saveBill(t, call, in)
	}
	stale := draft(2000, "stale-dispatch-draft")
	first := finish(t, call, draft(1000, "first-dispatch-part"), 200)
	payload := `{"id":1,"date":"2026-10-15","key":"partial-warehouse-return","lines":[{"product_id":1,"quantity":1000}]}`
	call("POST", "dispatch-return?business_id=1", payload, 200)
	call("POST", "dispatch-return?business_id=1", payload, 200)
	finish(t, call, stale, 400)
	finish(t, call, draft(1000, "last-dispatch-part"), 200)
	if scalar(t, s.DB, "SELECT SUM(value) FROM dispatch_allocations") != 100 || scalar(t, s.DB, "SELECT SUM(quantity) FROM dispatch_allocations") != 3000 {
		t.Fatal("allocation lost quantity or paise")
	}
	if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 34 || scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 {
		t.Fatal("invoice double-dispatched or return lost value")
	}
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1250'") != 0 {
		t.Fatal("dispatch asset did not clear")
	}
	call("POST", "dispatch-return?business_id=1", `{"id":1,"date":"2026-10-15","key":"exhausted-dispatch-return"}`, 400)
	call("POST", "reversals?business_id=1", fmt.Sprintf(`{"id":%d,"date":"2026-10-15","reason":"invoiced customer return","mode":"CASH","key":"partial-invoice-credit"}`, first.ID), 200)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM dispatch_allocations") != 3000 {
		t.Fatal("invoice return reopened allocations")
	}
	if _, e := s.DB.Exec("DELETE FROM dispatch_allocations"); e == nil {
		t.Fatal("allocations mutable")
	}
	call("GET", "dispatches?business_id=1&id=1&remaining=1", "", 200)
	call("GET", "dispatches?business_id=1&id=1&history=1", "", 200)
	assertIntegrity(t, s.DB)
}

func TestConsolidatedDispatchCostsAndStaleDraft(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":5000,"value":101,"date":"2026-10-15","key":"consolidated-opening"}`, 200)
	for _, key := range []string{"consolidated-first", "consolidated-second"} {
		call("POST", "dispatches?business_id=1", fmt.Sprintf(`{"party_id":1,"warehouse_id":1,"date":"2026-10-15","key":%q,"lines":[{"product_id":1,"quantity":2000}]}`, key), 200)
	}
	in := draftInput("SALE", "2026-10-15", "consolidated-draft", BillLine{ProductID: 1, Quantity: 3000, Price: 10000})
	in.DispatchMatches = []DispatchMatch{{DispatchID: 1, ProductID: 1, Quantity: 1000}, {DispatchID: 2, ProductID: 1, Quantity: 2000}}
	first := saveBill(t, call, in)
	in.Key = "consolidated-stale"
	stale := saveBill(t, call, in)
	done := finish(t, call, first, 200)
	if done.Lines[0].Cost != 61 {
		t.Fatalf("wrong aggregate cost %d", done.Lines[0].Cost)
	}
	finish(t, call, stale, 400)
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 || scalar(t, s.DB, "SELECT SUM(value) FROM dispatch_allocations") != 61 {
		t.Fatal("double stock issue or wrong allocations")
	}
	call("POST", "dispatch-return?business_id=1", `{"id":1,"date":"2026-10-15","key":"consolidated-return-rest"}`, 200)
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1250'") != 0 {
		t.Fatal("uncleared dispatch asset")
	}
	assertIntegrity(t, s.DB)
}
