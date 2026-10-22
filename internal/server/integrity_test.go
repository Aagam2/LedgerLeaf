package server

import (
	"billbook/internal/database"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testAPI func(string, string, string, int) string

func setupTrading(t *testing.T) (*Server, testAPI) {
	t.Helper()
	s, call := fixture(t)
	call("POST", "parties?business_id=1", `{"name":"Buyer","kind":"CUSTOMER"}`, 200)
	call("POST", "parties?business_id=1", `{"name":"Vendor","kind":"SUPPLIER"}`, 200)
	for i := 1; i <= 2; i++ {
		call("POST", "products?business_id=1", fmt.Sprintf(`{"name":"Item %d","sku":"SKU%d","kind":"PRODUCT","unit":"PCS","sale_price":10000}`, i, i), 200)
	}
	return s, call
}
func jsonText(t *testing.T, v interface{}) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func draftInput(kind, date, key string, lines ...BillLine) Bill {
	party := int64(1)
	if kind == "PURCHASE" {
		party = 2
	}
	return Bill{Kind: kind, PartyID: party, WarehouseID: 1, Date: date, DueDate: date, Key: key, Lines: lines}
}
func saveBill(t *testing.T, call testAPI, b Bill) Bill {
	t.Helper()
	var got Bill
	if e := json.Unmarshal([]byte(call("POST", "documents?business_id=1", jsonText(t, b), 200)), &got); e != nil {
		t.Fatal(e)
	}
	return got
}
func finish(t *testing.T, call testAPI, b Bill, status int) Bill {
	t.Helper()
	raw := call("POST", "finalize?business_id=1", fmt.Sprintf(`{"id":%d}`, b.ID), status)
	var got Bill
	if status == 200 {
		if e := json.Unmarshal([]byte(raw), &got); e != nil {
			t.Fatal(e)
		}
	}
	return got
}
func scalar(t *testing.T, db *sql.DB, q string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if e := db.QueryRow(q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func execSQL(t *testing.T, db *sql.DB, q string, args ...interface{}) {
	t.Helper()
	if _, e := db.Exec(q, args...); e != nil {
		t.Fatal(e)
	}
}
func assertIntegrity(t *testing.T, db *sql.DB) {
	t.Helper()
	var check string
	if e := db.QueryRow("PRAGMA integrity_check").Scan(&check); e != nil || check != "ok" {
		t.Fatalf("integrity: %s %v", check, e)
	}
	rows, e := db.Query("PRAGMA foreign_key_check")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	rows.Close()
	if n := scalar(t, db, `SELECT COUNT(*) FROM (SELECT journal_id FROM journal_lines GROUP BY journal_id HAVING SUM(debit)!=SUM(credit))`); n != 0 {
		t.Fatalf("%d unbalanced journals", n)
	}
}

func TestInvalidDraftsDoNotWrite(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Bill)
	}{
		{"zero quantity", func(b *Bill) { b.Lines[0].Quantity = 0 }}, {"negative quantity", func(b *Bill) { b.Lines[0].Quantity = -1 }},
		{"negative price", func(b *Bill) { b.Lines[0].Price = -1 }}, {"excess discount", func(b *Bill) { b.Lines[0].Discount = 10001 }},
		{"negative discount", func(b *Bill) { b.Lines[0].Discount = -1 }}, {"duplicate line", func(b *Bill) { b.Lines = append(b.Lines, b.Lines[0]) }},
		{"missing product", func(b *Bill) { b.Lines[0].ProductID = 999 }}, {"wrong party type", func(b *Bill) { b.PartyID = 2 }},
		{"foreign warehouse", func(b *Bill) { b.WarehouseID = 2 }}, {"invalid date", func(b *Bill) { b.Date = "2026-02-30" }},
		{"early due date", func(b *Bill) { b.DueDate = "2026-01-01" }}, {"unsupported kind", func(b *Bill) { b.Kind = "GST" }},
		{"missing lines", func(b *Bill) { b.Lines = nil }}, {"short key", func(b *Bill) { b.Key = "short" }},
		{"over limit", func(b *Bill) { b.Lines[0].Quantity = 1000000000; b.Lines[0].Price = 100000000000 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, call := setupTrading(t)
			b := draftInput("SALE", "2026-10-15", "validation-key-1234", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
			tc.change(&b)
			call("POST", "documents?business_id=1", jsonText(t, b), 400)
			for _, table := range []string{"documents", "requests", "stock_movements", "journals"} {
				if scalar(t, s.DB, "SELECT COUNT(*) FROM "+table) != 0 {
					t.Fatal("invalid request wrote", table)
				}
			}
			assertIntegrity(t, s.DB)
		})
	}
}

func TestFinalizationRollsBackAfterPartialWork(t *testing.T) {
	for _, failure := range []string{"second-line-stock", "audit-insert"} {
		t.Run(failure, func(t *testing.T) {
			s, call := setupTrading(t)
			call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":1000,"value":6000,"date":"2026-10-15","key":"opening-test-12345"}`, 200)
			lines := []BillLine{{ProductID: 1, Quantity: 1000, Price: 10000}}
			if failure == "second-line-stock" {
				lines = append(lines, BillLine{ProductID: 2, Quantity: 1000, Price: 10000})
			}
			b := saveBill(t, call, draftInput("SALE", "2026-10-15", "rollback-draft-key", lines...))
			if failure == "audit-insert" {
				execSQL(t, s.DB, `CREATE TRIGGER injected_failure BEFORE INSERT ON audit_log WHEN NEW.action='document.finalized' BEGIN SELECT RAISE(ABORT,'injected disk-write failure'); END`)
			}
			finish(t, call, b, 400)
			if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 || scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 1 || scalar(t, s.DB, "SELECT COUNT(*) FROM counters") != 0 {
				t.Fatal("partial finalization persisted")
			}
			var state string
			if e := s.DB.QueryRow("SELECT status FROM documents WHERE id=?", b.ID).Scan(&state); e != nil || state != "DRAFT" {
				t.Fatal(state, e)
			}
			if failure == "audit-insert" {
				execSQL(t, s.DB, "DROP TRIGGER injected_failure")
				done := finish(t, call, b, 200)
				if done.Number != "SALE/2026-27/00001" {
					t.Fatal(done.Number)
				}
			}
			assertIntegrity(t, s.DB)
		})
	}
}

func TestWeightedAverageRoundingAndFinancialYears(t *testing.T) {
	s, call := setupTrading(t)
	for i, p := range []int64{100, 101} {
		b := saveBill(t, call, draftInput("PURCHASE", "2026-03-31", fmt.Sprintf("purchase-round-%04d", i), BillLine{ProductID: 1, Quantity: 1000, Price: p}))
		finish(t, call, b, 200)
	}
	for i, want := range []int64{101, 100} {
		date := "2026-03-31"
		number := "SALE/2025-26/00001"
		if i == 1 {
			date = "2026-04-01"
			number = "SALE/2026-27/00001"
		}
		b := saveBill(t, call, draftInput("SALE", date, fmt.Sprintf("sale-round-key-%04d", i), BillLine{ProductID: 1, Quantity: 1000, Price: 200, Discount: 1}))
		done := finish(t, call, b, 200)
		if done.Lines[0].Cost != want || done.Total != 199 || done.Number != number {
			t.Fatalf("unexpected snapshot: %+v", done)
		}
	}
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 0 || scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 0 {
		t.Fatal("rounding left residual inventory")
	}
	assertIntegrity(t, s.DB)
}

func TestPaymentFailureRollbackAndSupplierPosting(t *testing.T) {
	s, call := setupTrading(t)
	b := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "supplier-payment-test", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, b, 200)
	body := fmt.Sprintf(`{"document_id":%d,"amount":10000,"date":"2026-10-15","mode":"BANK_TRANSFER","key":"payment-failure-test"}`, b.ID)
	execSQL(t, s.DB, `CREATE TRIGGER fail_payment BEFORE INSERT ON audit_log WHEN NEW.action='payment.recorded' BEGIN SELECT RAISE(ABORT,'injected failure'); END`)
	call("POST", "payments?business_id=1", body, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM payments") != 0 || scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 1 || scalar(t, s.DB, "SELECT COUNT(*) FROM requests WHERE key LIKE 'payment:%'") != 0 {
		t.Fatal("partial payment persisted")
	}
	execSQL(t, s.DB, "DROP TRIGGER fail_payment")
	call("POST", "payments?business_id=1", body, 200)
	if scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='2000'") != 0 || scalar(t, s.DB, "SELECT SUM(credit-debit) FROM journal_lines WHERE account='1010'") != 10000 {
		t.Fatal("supplier payment posted incorrectly")
	}
	assertIntegrity(t, s.DB)
}

func TestFinalizedSnapshotAndRestart(t *testing.T) {
	s, call := setupTrading(t)
	b := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "restart-snapshot-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	done := finish(t, call, b, 200)
	call("POST", "products?business_id=1", `{"id":1,"name":"New name","sku":"SKU1","kind":"PRODUCT","unit":"PCS","sale_price":20000,"active":1}`, 200)
	var seq int
	var name, path string
	if e := s.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := database.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s.DB = db
	var restored Bill
	if e = json.Unmarshal([]byte(call("GET", fmt.Sprintf("documents?business_id=1&id=%d", done.ID), "", 200)), &restored); e != nil {
		t.Fatal(e)
	}
	if restored.Lines[0].Name != "Item 1" || restored.Number != done.Number || restored.Total != 10000 {
		t.Fatal(restored)
	}
	assertIntegrity(t, db)
}

func TestConcurrentSalesCannotOversell(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":1000,"value":6000,"date":"2026-10-15","key":"opening-race-12345"}`, 200)
	var bills []Bill
	for i := 0; i < 2; i++ {
		bills = append(bills, saveBill(t, call, draftInput("SALE", "2026-10-15", fmt.Sprintf("race-draft-key-%04d", i), BillLine{ProductID: 1, Quantity: 1000, Price: 10000})))
	}
	codes := make(chan int, 2)
	for _, b := range bills {
		go func(id int64) {
			r := httptest.NewRequest("POST", "http://"+s.Host+"/api/finalize?business_id=1", strings.NewReader(fmt.Sprintf(`{"id":%d}`, id)))
			r.Header.Set("Origin", "http://"+s.Host)
			r.Header.Set("X-Ledgerleaf", "1")
			r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			codes <- w.Code
		}(b.ID)
	}
	a, c := <-codes, <-codes
	if !((a == 200 && c == 400) || (a == 400 && c == 200)) {
		t.Fatal(a, c)
	}
	if scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 0 {
		t.Fatal("oversold")
	}
	assertIntegrity(t, s.DB)
}

func TestUnitChangedAfterDraftRequiresResave(t *testing.T) {
	_, call := setupTrading(t)
	b := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "unit-change-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	call("POST", "products?business_id=1", `{"id":1,"name":"Item 1","sku":"SKU1","kind":"PRODUCT","unit":"KG","sale_price":10000,"active":1}`, 200)
	finish(t, call, b, 400)
	b.Key = "unit-change-resaved"
	b = saveBill(t, call, b)
	done := finish(t, call, b, 200)
	if done.Lines[0].Unit != "KG" {
		t.Fatal("resave did not refresh unit")
	}
}

func TestPaymentValidationAndConcurrentAllocation(t *testing.T) {
	s, call := setupTrading(t)
	bill := saveBill(t, call, draftInput("PURCHASE", "2026-10-15", "payment-validation-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	base := map[string]interface{}{"document_id": bill.ID, "amount": int64(10000), "date": "2026-10-15", "mode": "CASH", "key": "payment-validation-request"}
	call("POST", "payments?business_id=1", jsonText(t, base), 400) // draft has no payable
	finish(t, call, bill, 200)
	cases := []struct {
		field string
		value interface{}
	}{{"amount", 0}, {"amount", -1}, {"amount", 10001}, {"date", "2026-02-30"}, {"date", "2026-10-14"}, {"mode", "ONLINE"}, {"document_id", 999}, {"key", "short"}}
	for _, c := range cases {
		body := map[string]interface{}{}
		for k, v := range base {
			body[k] = v
		}
		body[c.field] = c.value
		call("POST", "payments?business_id=1", jsonText(t, body), 400)
	}
	call("POST", "payments?business_id=2", jsonText(t, base), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM payments") != 0 {
		t.Fatal("invalid payment persisted")
	}
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		body := map[string]interface{}{}
		for k, v := range base {
			body[k] = v
		}
		body["key"] = fmt.Sprintf("concurrent-payment-%d", i)
		raw := jsonText(t, body)
		go func() {
			r := httptest.NewRequest("POST", "http://"+s.Host+"/api/payments?business_id=1", strings.NewReader(raw))
			r.Header.Set("Origin", "http://"+s.Host)
			r.Header.Set("X-Ledgerleaf", "1")
			r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	a, b := <-codes, <-codes
	if !((a == 200 && b == 400) || (a == 400 && b == 200)) {
		t.Fatal(a, b)
	}
	if scalar(t, s.DB, "SELECT SUM(amount) FROM payments") != 10000 {
		t.Fatal("concurrent payments exceeded outstanding")
	}
	assertIntegrity(t, s.DB)
}

func TestArchivedProductAndBackdatedStockCannotFinalize(t *testing.T) {
	for _, mode := range []string{"archived", "backdated"} {
		t.Run(mode, func(t *testing.T) {
			s, call := setupTrading(t)
			call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":1000,"value":6000,"date":"2026-10-15","key":"opening-validation"}`, 200)
			date := "2026-10-15"
			if mode == "backdated" {
				date = "2026-10-14"
			}
			bill := saveBill(t, call, draftInput("SALE", date, "archive-test-draft", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
			if mode == "archived" {
				call("POST", "products?business_id=1", `{"id":1,"name":"Item 1","sku":"SKU1","kind":"PRODUCT","unit":"PCS","sale_price":10000,"active":0}`, 200)
			}
			finish(t, call, bill, 400)
			if scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != 1 {
				t.Fatal("rejected finalization moved stock")
			}
			assertIntegrity(t, s.DB)
		})
	}
}
