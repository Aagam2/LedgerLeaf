package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestManualJournalAndReversal(t *testing.T) {
	s, call := fixture(t)
	call("POST", "accounts?business_id=1", `{"code":"6100","name":"Rent expense","kind":"EXPENSE"}`, 200)
	body := `{"date":"2026-10-16","description":"Opening capital","key":"manual-capital-request","lines":[{"account":"1000","debit":100000},{"account":"3000","credit":100000}]}`
	var result map[string]int64
	if e := json.Unmarshal([]byte(call("POST", "manual-journal?business_id=1", body, 200)), &result); e != nil {
		t.Fatal(e)
	}
	call("POST", "manual-journal?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1000'") != 100000 {
		t.Fatal("duplicate or incorrect journal")
	}
	invalid := strings.Replace(strings.Replace(body, "manual-capital-request", "unbalanced-request-1", 1), `"credit":100000`, `"credit":99999`, 1)
	call("POST", "manual-journal?business_id=1", invalid, 400)
	invalid = strings.Replace(strings.Replace(body, "manual-capital-request", "control-account-test", 1), `"1000"`, `"1200"`, 1)
	call("POST", "manual-journal?business_id=1", invalid, 400)
	reversal := fmt.Sprintf(`{"id":%d,"date":"2026-10-16","reason":"Incorrect opening entry"}`, result["id"])
	call("POST", "journal-reversal?business_id=1", reversal, 200)
	call("POST", "journal-reversal?business_id=1", reversal, 400)
	if scalar(t, s.DB, "SELECT SUM(debit-credit) FROM journal_lines WHERE account='1000'") != 0 {
		t.Fatal("reversal failed")
	}
	assertIntegrity(t, s.DB)
}
func TestBusinessProfilePersistsAndInvoiceSnapshotIsFrozen(t *testing.T) {
	s, call := setupTrading(t)
	body := `{"id":1,"name":"Updated Business","legal_name":"Legal Ltd","state_code":"27","phone":"1234567890","email":"shop@example.test","pan":"ABCDE1234F","bank_name":"Test Bank","bank_account":"000123456789","bank_ifsc":"TEST0123456","invoice_terms":"Pay within seven days","invoice_footer":"Thank you"}`
	call("POST", "businesses", body, 200)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-16", "profile-snapshot-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	done := finish(t, call, purchase, 200)
	call("POST", "businesses", strings.Replace(body, "Test Bank", "Changed Bank", 1), 200)
	var snapshot Bill
	if e := json.Unmarshal([]byte(call("GET", fmt.Sprintf("documents?business_id=1&id=%d", done.ID), "", 200)), &snapshot); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(snapshot.BankDetails, "Test Bank") || snapshot.Terms != "Pay within seven days" || snapshot.BusinessName != "Updated Business" {
		t.Fatal(snapshot)
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 2 || scalar(t, s.DB, "SELECT COUNT(*) FROM warehouses WHERE business_id=1") != 1 {
		t.Fatal("business edit created duplicate defaults")
	}
	assertIntegrity(t, s.DB)
}
