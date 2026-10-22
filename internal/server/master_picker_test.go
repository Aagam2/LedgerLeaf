package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMasterPickerFiltersPaginationAndIsolation(t *testing.T) {
	s, call := fixture(t)
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for i := 1; i <= 55; i++ {
		for _, kind := range []string{"CUSTOMER", "SUPPLIER"} {
			if _, e = tx.Exec("INSERT INTO parties(business_id,name,kind) VALUES(1,?,?)", fmt.Sprintf("%s %03d", kind, i), kind); e != nil {
				t.Fatal(e)
			}
		}
		for _, kind := range []string{"PRODUCT", "SERVICE"} {
			if _, e = tx.Exec("INSERT INTO products(business_id,name,sku,kind,unit,sale_price) VALUES(1,?,?,?,'PCS',100)", fmt.Sprintf("%s %03d", kind, i), fmt.Sprintf("%s-%03d", kind, i), kind); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i := 1; i <= 201; i++ {
		if _, e = tx.Exec("INSERT INTO warehouses(business_id,name) VALUES(1,?)", fmt.Sprintf("Warehouse %03d", i)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	read := func(path string) []map[string]interface{} {
		t.Helper()
		var rows []map[string]interface{}
		if e := json.Unmarshal([]byte(call("GET", path, "", 200)), &rows); e != nil {
			t.Fatal(e)
		}
		return rows
	}
	for _, entity := range []string{"parties", "products"} {
		kind := "CUSTOMER"
		if entity == "products" {
			kind = "PRODUCT"
		}
		rows := read(entity + "?business_id=1&active=1&kind=" + kind + "&page=2")
		if len(rows) != 5 || rows[0]["kind"] != kind {
			t.Fatal(entity, rows)
		}
		id := int64(rows[0]["id"].(float64))
		if got := read(fmt.Sprintf("%s?business_id=1&id=%d", entity, id)); len(got) != 1 {
			t.Fatal(got)
		}
		if got := read(fmt.Sprintf("%s?business_id=2&id=%d", entity, id)); len(got) != 0 {
			t.Fatal("cross-business selected record", got)
		}
		call("GET", entity+"?business_id=1&id=bad", "", 400)
		call("GET", entity+"?business_id=1&kind=INVALID", "", 400)
	}
	rows := read("warehouses?business_id=1&paged=1&page=5")
	if len(rows) != 2 {
		t.Fatal("warehouse pagination stopped at 200", len(rows))
	}
	rows = read("warehouses?business_id=1&paged=1&q=Warehouse%20201")
	if len(rows) != 1 || rows[0]["name"] != "Warehouse 201" {
		t.Fatal(rows)
	}
	id := int64(rows[0]["id"].(float64))
	if len(read(fmt.Sprintf("warehouses?business_id=2&id=%d", id))) != 0 {
		t.Fatal("cross-business warehouse")
	}
}

func TestBusinessAndVariantGroupPagination(t *testing.T) {
	s, call := fixture(t)
	tx, e := s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for i := 3; i <= 201; i++ {
		if _, e = tx.Exec("INSERT INTO businesses(name,state_code) VALUES(?,'27')", fmt.Sprintf("Business %03d", i)); e != nil {
			t.Fatal(e)
		}
	}
	for i := 1; i <= 501; i++ {
		if _, e = tx.Exec("INSERT INTO product_groups(business_id,name) VALUES(1,?)", fmt.Sprintf("Group %03d", i)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var rows []map[string]interface{}
	for _, path := range []string{"businesses?paged=1&page=5", "variants?business_id=1&groups=1&paged=1&page=11"} {
		if e = json.Unmarshal([]byte(call("GET", path, "", 200)), &rows); e != nil || len(rows) != 1 {
			t.Fatal(path, rows, e)
		}
	}
	raw := call("GET", "businesses?paged=1&q=Business%20201", "", 200)
	if e = json.Unmarshal([]byte(raw), &rows); e != nil || len(rows) != 1 || rows[0]["id"].(float64) != 201 {
		t.Fatal(raw, e)
	}
	call("GET", "variants?business_id=1&groups=1&id=501", "", 200)
	if raw = call("GET", "variants?business_id=2&groups=1&id=501", "", 200); raw != "[]\n" {
		t.Fatal("group isolation", raw)
	}
}
