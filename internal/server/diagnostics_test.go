package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticsDetectsAccountingDriftWithoutWriting(t *testing.T) {
	s, call := setupTrading(t)
	purchase := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "diagnostic-purchase", BillLine{ProductID: 1, Quantity: 3000, Price: 10000}))
	finish(t, call, purchase, 200)
	read := func() map[string]diagnosticCheck {
		t.Helper()
		var report struct {
			Checks []diagnosticCheck `json:"checks"`
		}
		raw := call("POST", "diagnostics", `{}`, 200)
		if strings.Contains(raw, "Buyer") || strings.Contains(raw, s.Token) || strings.Contains(raw, s.DataDir) && s.DataDir != "" {
			t.Fatal("diagnostics leaked private data")
		}
		if e := json.Unmarshal([]byte(raw), &report); e != nil {
			t.Fatal(e)
		}
		out := map[string]diagnosticCheck{}
		for _, c := range report.Checks {
			out[c.Name] = c
		}
		return out
	}
	for _, c := range read() {
		if c.Status != "pass" {
			t.Fatal(c)
		}
	}
	// Simulate damage in a disposable DB: insert an unbalanced ledger entry.
	execSQL(t, s.DB, `INSERT INTO journals(business_id,date,reference,description) VALUES(1,'2026-10-15','corrupt-test','test fixture')`)
	execSQL(t, s.DB, `INSERT INTO journal_lines(business_id,journal_id,account,debit,credit) VALUES(1,(SELECT MAX(id) FROM journals),'1200',1,0)`)
	before := scalar(t, s.DB, "SELECT total_changes()")
	report := read()
	for _, name := range []string{"Balanced journal entries", "Inventory ledger reconciliation"} {
		if report[name].Status != "fail" || report[name].Issues != 1 {
			t.Fatal("missed damage", name, report[name])
		}
	}
	if scalar(t, s.DB, "SELECT total_changes()") != before {
		t.Fatal("diagnostics modified database")
	}
	call("GET", "diagnostics", "", 405)
}

func TestDiagnosticsAdministratorOnly(t *testing.T) {
	s, _ := setupTrading(t)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"admin","password":"Admin-Password-123!","role":"ADMIN","active":1}`, 200)
	for _, role := range []string{"ACCOUNTANT", "VIEWER", "BILLER"} {
		username := strings.ToLower(role)
		owner("POST", "auth/users", `{"username":"`+username+`","password":"Other-Password-123!","role":"`+role+`","active":1}`, 200)
		client := authClient(t, s)
		client("POST", "auth/login", `{"username":"`+username+`","password":"Other-Password-123!"}`, 200)
		client("POST", "diagnostics", `{}`, 403)
	}
	owner("POST", "diagnostics", `{}`, 200)
}
