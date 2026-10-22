package server

import (
	"billbook/internal/database"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (*Server, func(string, string, string, int) string) {
	t.Helper()
	db, e := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	s, e := New(db, "127.0.0.1:1234", func() {})
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, path, body string, status int) string {
		t.Helper()
		r := httptest.NewRequest(method, "http://"+s.Host+"/api/"+path, strings.NewReader(body))
		r.Host = s.Host
		r.Header.Set("Origin", "http://"+s.Host)
		r.Header.Set("X-Ledgerleaf", "1")
		r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d expected %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w.Body.String()
	}
	call("POST", "businesses", `{"name":"First","state_code":"27"}`, 201)
	call("POST", "businesses", `{"name":"Second","state_code":"27"}`, 201)
	return s, call
}
func TestStockIsolationIdempotencyAndAccounting(t *testing.T) {
	s, call := fixture(t)
	call("POST", "products?business_id=1", `{"name":"Tea","sku":"T","kind":"PRODUCT","unit":"PCS","sale_price":10000}`, 200)
	body := `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":120000,"date":"2026-10-15","key":"1234567890123456"}`
	call("POST", "opening-stock?business_id=2", body, 400)
	call("POST", "opening-stock?business_id=1", body, 200)
	call("POST", "opening-stock?business_id=1", body, 200)
	call("POST", "opening-stock?business_id=1", strings.Replace(body, `"value":120000`, `"value":130000`, 1), 400)
	call("POST", "opening-stock?business_id=1", strings.Replace(body, "1234567890123456", "another-key-12345", 1), 400)
	var movements, qty, value, deb, cred int64
	s.DB.QueryRow("SELECT COUNT(*),SUM(quantity),SUM(value) FROM stock_movements").Scan(&movements, &qty, &value)
	if movements != 1 || qty != 2000 || value != 120000 {
		t.Fatal(movements, qty, value)
	}
	s.DB.QueryRow("SELECT SUM(debit),SUM(credit) FROM journal_lines").Scan(&deb, &cred)
	if deb != 120000 || cred != deb {
		t.Fatal(deb, cred)
	}
	if got := call("GET", "stock?business_id=2", "", 200); strings.TrimSpace(got) != "[]" {
		t.Fatal(got)
	}
	var rows []map[string]interface{}
	json.Unmarshal([]byte(call("GET", "products?business_id=1", "", 200)), &rows)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	call("POST", "products?business_id=2", `{"id":1,"name":"Hijack","sku":"T","kind":"PRODUCT","unit":"PCS","sale_price":10000,"active":1}`, 400)
}
