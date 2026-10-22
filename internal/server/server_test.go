package server

import (
	"billbook/internal/database"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecurityAndBusiness(t *testing.T) {
	db, e := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s, e := New(db, "127.0.0.1:1234", func() {})
	if e != nil {
		t.Fatal(e)
	}
	request := func(method, path, body, host, origin, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
		r.Host = host
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Ledgerleaf", "1")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: token})
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	host := s.Host
	origin := "http://" + host
	if w := request("GET", "/api/businesses", "", host, "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/api/businesses", "", "evil.test", "", s.Token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/businesses", `{"name":"X","state_code":"27"}`, host, "http://evil.test", s.Token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/session", `{"token":"`+s.Token+`"}`, host, origin, ""); w.Code != 200 || len(w.Result().Cookies()) != 1 {
		t.Fatal(w.Code)
	}
	if w := request("POST", "/api/businesses", `{"name":"Greenfield","state_code":"27"}`, host, origin, s.Token); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/api/businesses", "", host, "", s.Token); w.Code != 200 || !strings.Contains(w.Body.String(), "Greenfield") {
		t.Fatal(w.Body.String())
	}
	if w := request("POST", "/api/businesses", `{"name":"","state_code":"27"}`, host, origin, s.Token); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/", "", host, "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Ledgerleaf") {
		t.Fatal(w.Code, w.Body.String())
	}
}
