package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authClient(t *testing.T, s *Server) func(string, string, string, int) string {
	t.Helper()
	cookies := map[string]*http.Cookie{}
	return func(method, path, body string, status int) string {
		t.Helper()
		r := httptest.NewRequest(method, "http://"+s.Host+"/api/"+path, strings.NewReader(body))
		r.Header.Set("Origin", "http://"+s.Host)
		r.Header.Set("X-Ledgerleaf", "1")
		r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
		for _, c := range cookies {
			r.AddCookie(c)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: expected %d got %d: %s", method, path, status, w.Code, w.Body.String())
		}
		for _, c := range w.Result().Cookies() {
			if c.MaxAge < 0 {
				delete(cookies, c.Name)
			} else {
				cookies[c.Name] = c
			}
		}
		return w.Body.String()
	}
}
func TestPasswordAuthenticationAndRoles(t *testing.T) {
	s, _ := fixture(t)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Test-Password-123!","role":"ADMIN","active":1}`, 200)
	anonymous := authClient(t, s)
	anonymous("GET", "businesses", "", 401)
	owner("POST", "auth/users", `{"username":"viewer","password":"Viewer-Password-123!","role":"VIEWER","active":1}`, 200)
	viewer := authClient(t, s)
	viewer("POST", "auth/login", `{"username":"VIEWER","password":"Viewer-Password-123!"}`, 200)
	viewer("GET", "products?business_id=1", "", 200)
	viewer("POST", "parties?business_id=1", `{"name":"Denied","kind":"CUSTOMER"}`, 403)
	viewer("GET", "backups", "", 403)
	viewer("GET", "backup-policy", "", 403)
	viewer("POST", "backup-policy", `{ "enabled":true,"interval_hours":1,"retain_count":1}`, 403)
	viewer("GET", "auth/users", "", 403)
	owner("POST", "auth/users", `{"id":1,"username":"owner","role":"VIEWER","active":1}`, 400)
	owner("POST", "parties?business_id=1", `{"name":"Allowed","kind":"CUSTOMER"}`, 200)
	if scalar(t, s.DB, "SELECT actor_id FROM audit_log WHERE action='parties.saved' ORDER BY id DESC LIMIT 1") != 1 {
		t.Fatal("audit actor missing")
	}
	owner("POST", "auth/users", `{"id":2,"username":"viewer","role":"VIEWER","active":0}`, 200)
	viewer("GET", "products?business_id=1", "", 401)
	assertIntegrity(t, s.DB)
}
func TestPasswordChangeInvalidatesOtherSessions(t *testing.T) {
	s, _ := fixture(t)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Test-Password-123!","role":"ADMIN","active":1}`, 200)
	other := authClient(t, s)
	other("POST", "auth/login", `{"username":"owner","password":"Test-Password-123!"}`, 200)
	owner("POST", "auth/password", `{"current":"wrong","password":"Updated-Password-123!"}`, 403)
	owner("POST", "auth/password", `{"current":"Test-Password-123!","password":"Updated-Password-123!"}`, 200)
	other("GET", "businesses", "", 401)
	owner("GET", "businesses", "", 200)
	owner("POST", "auth/logout", `{}`, 200)
	owner("GET", "businesses", "", 401)
	owner("POST", "auth/login", `{"username":"owner","password":"Test-Password-123!"}`, 401)
	owner("POST", "auth/login", `{"username":"owner","password":"Updated-Password-123!"}`, 200)
}
func TestLoginThrottlesRepeatedFailures(t *testing.T) {
	s, _ := fixture(t)
	call := authClient(t, s)
	call("POST", "auth/users", `{"username":"owner","password":"Test-Password-123!","role":"ADMIN","active":1}`, 200)
	for i := 0; i < 5; i++ {
		call("POST", "auth/login", `{"username":"missing","password":"wrong"}`, 401)
	}
	call("POST", "auth/login", `{"username":"missing","password":"wrong"}`, 429)
}
func TestBillerCannotOverridePricesOrAccessPurchases(t *testing.T) {
	s, seed := setupTrading(t)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Test-Password-123!","role":"ADMIN","active":1}`, 200)
	owner("POST", "auth/users", `{"username":"biller","password":"Biller-Password-123!","role":"BILLER","active":1}`, 200)
	biller := authClient(t, s)
	biller("POST", "auth/login", `{"username":"biller","password":"Biller-Password-123!"}`, 200)
	bill := draftInput("SALE", "2026-10-16", "biller-price-request", BillLine{ProductID: 1, Quantity: 1000, Price: 9000})
	biller("POST", "documents?business_id=1", jsonText(t, bill), 403)
	bill.Lines[0].Price = 10000
	biller("POST", "documents?business_id=1", jsonText(t, bill), 200)
	bill.Kind = "PURCHASE"
	bill.PartyID = 2
	bill.Key = "biller-purchase-denied"
	biller("POST", "documents?business_id=1", jsonText(t, bill), 403)
	biller("POST", "expenses?business_id=1", `{}`, 403)
	_ = seed
}
