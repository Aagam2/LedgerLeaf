package server

import (
	"encoding/json"
	"testing"
)

func TestAssignedBusinessesEnforcedAndChangesRevokeSessions(t *testing.T) {
	s, _ := fixture(t)
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Owner-Password-123!","role":"ADMIN","active":1}`, 200)
	owner("POST", "auth/users", `{"username":"limited","password":"Limited-Password-123!","role":"ACCOUNTANT","active":1,"restricted":true,"business_ids":[1]}`, 200)
	limited := authClient(t, s)
	limited("POST", "auth/login", `{"username":"limited","password":"Limited-Password-123!"}`, 200)
	var businesses []Business
	if e := json.Unmarshal([]byte(limited("GET", "businesses", "", 200)), &businesses); e != nil || len(businesses) != 1 || businesses[0].ID != 1 {
		t.Fatal("business list leaked", businesses, e)
	}
	limited("GET", "products?business_id=1", "", 200)
	limited("GET", "products?business_id=2", "", 403)
	limited("GET", "reports?business_id=2&type=sales", "", 403)
	limited("GET", "document-pdf?business_id=2&id=1", "", 403)
	limited("POST", "parties?business_id=2", `{"name":"Denied","kind":"CUSTOMER"}`, 403)
	owner("POST", "auth/users", `{"id":2,"username":"limited","role":"ACCOUNTANT","active":1,"restricted":true,"business_ids":[2,999]}`, 400)
	// Invalid reassignment must roll back the deletion of the original membership.
	limited("GET", "products?business_id=1", "", 200)
	limited("GET", "products?business_id=2", "", 403)
	owner("POST", "auth/users", `{"id":2,"username":"limited","role":"ACCOUNTANT","active":1,"restricted":true,"business_ids":[2]}`, 200)
	limited("GET", "products?business_id=1", "", 401)
	limited("POST", "auth/login", `{"username":"limited","password":"Limited-Password-123!"}`, 200)
	limited("GET", "products?business_id=1", "", 403)
	limited("GET", "products?business_id=2", "", 200)
	owner("POST", "auth/users", `{"id":1,"username":"owner","role":"ADMIN","active":1,"restricted":true,"business_ids":[1]}`, 400)
	owner("POST", "auth/users", `{"id":2,"username":"limited","role":"VIEWER","active":1,"restricted":true,"business_ids":[]}`, 200)
	limited("POST", "auth/login", `{"username":"limited","password":"Limited-Password-123!"}`, 200)
	if e := json.Unmarshal([]byte(limited("GET", "businesses", "", 200)), &businesses); e != nil || len(businesses) != 0 {
		t.Fatal("empty assignment leaked", businesses, e)
	}
	limited("GET", "auth/status", "", 200)
	assertIntegrity(t, s.DB)
}
