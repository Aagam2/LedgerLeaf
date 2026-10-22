package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMalformedJSONRejected(t *testing.T) {
	s, call := fixture(t)
	for _, body := range []string{`{`, `{"name":"X","state_code":"27","unexpected":1}`, `{"name":"X","state_code":"27"} {}`, `{"name":123,"state_code":"27"}`, `{"name":"` + strings.Repeat("x", (1<<20)+1) + `","state_code":"27"}`} {
		call("POST", "businesses", body, 400)
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 2 {
		t.Fatal("malformed input created records")
	}
}
func TestProtectedRoutesRequireSession(t *testing.T) {
	s, _ := fixture(t)
	for _, path := range []string{"businesses", "products", "parties", "warehouses", "stock", "movements", "trial-balance", "journals", "documents", "payments", "document-pdf", "exit"} {
		t.Run(path, func(t *testing.T) {
			for _, token := range []string{"", "incorrect-token"} {
				r := httptest.NewRequest("GET", "http://"+s.Host+"/api/"+path+"?business_id=1", nil)
				if token != "" {
					r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: token})
				}
				w := httptest.NewRecorder()
				s.ServeHTTP(w, r)
				if w.Code != 401 {
					t.Fatal(w.Code)
				}
			}
		})
	}
}
func TestMissingCSRFHeaderAndSessionCookieFlags(t *testing.T) {
	s, _ := fixture(t)
	r := httptest.NewRequest("POST", "http://"+s.Host+"/api/businesses", strings.NewReader(`{"name":"X","state_code":"27"}`))
	r.Header.Set("Origin", "http://"+s.Host)
	r.AddCookie(&http.Cookie{Name: "ledgerleaf", Value: s.Token})
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "http://"+s.Host+"/api/session", strings.NewReader(`{"token":"`+s.Token+`"}`))
	r.Header.Set("Origin", "http://"+s.Host)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	cookies := w.Result().Cookies()
	if w.Code != 200 || len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		t.Fatal(w.Code, cookies)
	}
}
func TestSearchPaginationAndSQLInput(t *testing.T) {
	s, call := fixture(t)
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for i := 0; i < 51; i++ {
		if _, e = tx.Exec("INSERT INTO parties(business_id,kind,name) VALUES(1,'CUSTOMER',?)", fmt.Sprintf("Customer %03d", i)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		query string
		count int
	}{{"page=1", 50}, {"page=2", 1}, {"page=3", 0}, {"page=-1", 50}, {"q=Customer%20050", 1}, {"q=%27%20OR%201%3D1--", 0}} {
		var rows []map[string]interface{}
		if e = json.Unmarshal([]byte(call("GET", "parties?business_id=1&"+tc.query, "", 200)), &rows); e != nil || len(rows) != tc.count {
			t.Fatal(tc, len(rows), e)
		}
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM parties") != 51 {
		t.Fatal("search changed records")
	}
}
