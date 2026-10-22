package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBillingWorkflow(t *testing.T) {
	s, call := fixture(t)
	call("POST", "parties?business_id=1", `{"name":"Buyer","kind":"CUSTOMER"}`, 200)
	call("POST", "parties?business_id=1", `{"name":"Supplier","kind":"SUPPLIER"}`, 200)
	call("POST", "products?business_id=1", `{"name":"Tea","sku":"T","kind":"PRODUCT","unit":"PCS","sale_price":100000}`, 200)
	draft := func(kind string, party, quantity, price int64, key string) Bill {
		raw := call("POST", "documents?business_id=1", fmt.Sprintf(`{"kind":%q,"party_id":%d,"warehouse_id":1,"date":"2026-10-15","due_date":"2026-10-20","key":%q,"lines":[{"product_id":1,"quantity":%d,"price":%d}]}`, kind, party, key, quantity, price), 200)
		var b Bill
		if e := json.Unmarshal([]byte(raw), &b); e != nil {
			t.Fatal(e)
		}
		return b
	}
	sale := draft("SALE", 1, 2000, 100000, "sale-key-12345678")
	call("POST", "finalize?business_id=1", fmt.Sprintf(`{"id":%d}`, sale.ID), 400)
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM journals").Scan(&n)
	if n != 0 {
		t.Fatal("failed sale left journal")
	}
	s.DB.QueryRow("SELECT COUNT(*) FROM counters").Scan(&n)
	if n != 0 {
		t.Fatal("failed sale left number reservation")
	}
	purchase := draft("PURCHASE", 2, 10000, 60000, "purchase-key-1234")
	call("POST", "finalize?business_id=1", fmt.Sprintf(`{"id":%d}`, purchase.ID), 200)
	call("POST", "finalize?business_id=1", fmt.Sprintf(`{"id":%d}`, sale.ID), 200)
	call("POST", "finalize?business_id=1", fmt.Sprintf(`{"id":%d}`, sale.ID), 200)
	var qty, value, debit, credit int64
	s.DB.QueryRow("SELECT SUM(quantity),SUM(value) FROM stock_movements").Scan(&qty, &value)
	if qty != 8000 || value != 480000 {
		t.Fatal(qty, value)
	}
	s.DB.QueryRow("SELECT SUM(debit),SUM(credit) FROM journal_lines").Scan(&debit, &credit)
	if debit != credit || debit != 920000 {
		t.Fatal(debit, credit)
	}
	pay := fmt.Sprintf(`{"document_id":%d,"amount":50000,"date":"2026-10-15","mode":"CASH","key":"payment-key-1234"}`, sale.ID)
	call("POST", "payments?business_id=1", pay, 200)
	call("POST", "payments?business_id=1", pay, 200)
	call("POST", "payments?business_id=1", strings.Replace(strings.Replace(pay, "50000", "200000", 1), "payment-key-1234", "payment-key-9999", 1), 400)
	call("POST", "payments?business_id=1", strings.Replace(strings.Replace(pay, "50000", "150000", 1), "payment-key-1234", "payment-key-5678", 1), 200)
	s.DB.QueryRow("SELECT COUNT(*) FROM payments").Scan(&n)
	if n != 2 {
		t.Fatal(n)
	}
	call("GET", fmt.Sprintf("documents?business_id=2&id=%d", sale.ID), "", 400)
	pdf := call("GET", fmt.Sprintf("document-pdf?business_id=1&id=%d", sale.ID), "", 200)
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Fatal("not a PDF")
	}
	if _, e := s.DB.Exec("UPDATE documents SET total=1 WHERE id=?", sale.ID); e == nil {
		t.Fatal("finalized invoice changed")
	}
	if _, e := s.DB.Exec("DELETE FROM stock_movements"); e == nil {
		t.Fatal("stock history deleted")
	}
}

func TestFinalizeConcurrent(t *testing.T) {
	s, call := fixture(t)
	call("POST", "parties?business_id=1", `{"name":"Buyer","kind":"CUSTOMER"}`, 200)
	call("POST", "products?business_id=1", `{"name":"Consulting","sku":"C","kind":"SERVICE","unit":"HOUR","sale_price":10000}`, 200)
	raw := call("POST", "documents?business_id=1", `{"kind":"SALE","party_id":1,"warehouse_id":1,"date":"2026-10-15","due_date":"2026-10-15","key":"concurrent-draft-key","lines":[{"product_id":1,"quantity":1000,"price":10000}]}`, 200)
	var bill Bill
	if e := json.Unmarshal([]byte(raw), &bill); e != nil {
		t.Fatal(e)
	}
	results := make(chan int, 8)
	for i := 0; i < 8; i++ {
		go func() {
			r := httptest.NewRequest("POST", "http://"+s.Host+"/api/finalize?business_id=1", strings.NewReader(fmt.Sprintf(`{"id":%d}`, bill.ID)))
			r.Host = s.Host
			r.Header.Set("Origin", "http://"+s.Host)
			r.Header.Set("X-Ledgerleaf", "1")
			r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	for i := 0; i < 8; i++ {
		if code := <-results; code != 200 {
			t.Fatal(code)
		}
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM journals").Scan(&n)
	if n != 1 {
		t.Fatal("duplicate journals", n)
	}
}
