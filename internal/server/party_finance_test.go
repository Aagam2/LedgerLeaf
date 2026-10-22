package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestPartyOpeningAdvanceAllocationRefundReconcile(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "party-finance?business_id=1", `{"action":"OPENING","party_id":1,"date":"2026-10-01","amount":10000,"mode":"CASH","key":"party-opening-balance"}`, 200)
	call("POST", "party-finance?business_id=1", `{"action":"SETTLE_OPENING","party_id":1,"date":"2026-10-01","amount":10000,"mode":"CASH","key":"party-settle-opening"}`, 200)
	call("POST", "party-finance?business_id=1", `{"action":"ADVANCE","party_id":1,"date":"2026-10-01","amount":15000,"mode":"CASH","key":"party-advance-receipt"}`, 200)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "advance-purchase-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 6000}))
	finish(t, call, purchase, 200)
	sale := saveBill(t, call, draftInput("SALE", "2026-10-15", "advance-sale-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, sale, 200)
	body := fmt.Sprintf(`{"action":"ALLOCATE","party_id":1,"advance_id":1,"document_id":%d,"date":"2026-10-15","amount":10000,"mode":"CASH","key":"party-advance-allocate"}`, sale.ID)
	call("POST", "party-finance?business_id=1", body, 200)
	call("POST", "party-finance?business_id=1", body, 200)
	call("POST", "party-finance?business_id=1", `{"action":"REFUND","party_id":1,"advance_id":1,"date":"2026-10-15","amount":5001,"mode":"CASH","key":"party-advance-refund"}`, 400)
	call("POST", "party-finance?business_id=1", `{"action":"REFUND","party_id":1,"advance_id":1,"date":"2026-10-15","amount":5000,"mode":"CASH","key":"party-advance-refund"}`, 200)
	for _, code := range []string{"1100", "2200"} {
		if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account=?", code) != 0 {
			t.Fatal("party balance", code)
		}
	}
	if !strings.Contains(call("GET", "reports?business_id=1&kind=party-statement&party_id=1", "", 200), "OPENING PAYMENT") {
		t.Fatal("opening not in statement")
	}
	call("GET", "party-finance?business_id=1", "", 200)
	call("GET", "party-finance?business_id=1&kind=openings", "", 200)
	assertIntegrity(t, s.DB)
}
