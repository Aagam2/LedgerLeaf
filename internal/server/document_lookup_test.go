package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDocumentOutstandingAndPaymentHistory(t *testing.T) {
	_, call := setupTrading(t)
	draft := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "lookup-draft-00001", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	posted := finish(t, call, draft, 200)
	saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "lookup-draft-00002", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	read := func(path string) []map[string]interface{} {
		t.Helper()
		var rows []map[string]interface{}
		if e := json.Unmarshal([]byte(call("GET", path, "", 200)), &rows); e != nil {
			t.Fatal(e)
		}
		return rows
	}
	if rows := read("documents?business_id=1&outstanding=1&kind=PURCHASE&q=Vendor"); len(rows) != 1 || int64(rows[0]["id"].(float64)) != posted.ID {
		t.Fatalf("outstanding: %v", rows)
	}
	for i, amount := range []int{4000, 6000} {
		call("POST", "payments?business_id=1", fmt.Sprintf(`{"document_id":%d,"date":"2026-10-15","amount":%d,"mode":"CASH","key":"lookup-payment-000%d"}`, posted.ID, amount, i), 200)
	}
	if rows := read("documents?business_id=1&outstanding=1"); len(rows) != 0 {
		t.Fatalf("settled included: %v", rows)
	}
	path := fmt.Sprintf("payments?business_id=1&document_id=%d", posted.ID)
	if rows := read(path); len(rows) != 2 || rows[0]["account_code"] != "1000" {
		t.Fatalf("payments: %v", rows)
	}
	if rows := read(fmt.Sprintf("payments?business_id=2&document_id=%d", posted.ID)); len(rows) != 0 {
		t.Fatal("cross-business payment disclosure")
	}
	if rows := read("payments?business_id=1&document_id=999999"); len(rows) != 0 {
		t.Fatal("unrelated payments")
	}
	call("GET", "payments?business_id=1&document_id=invalid", "", 400)
}
