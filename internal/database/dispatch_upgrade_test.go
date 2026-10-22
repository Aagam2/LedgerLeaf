package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestDispatchUpgradePreservesHistoricalAllocations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-dispatches.db")
	db, e := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if e != nil {
		t.Fatal(e)
	}
	files, e := migrations.ReadDir("migrations")
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range files[:25] {
		body, e := migrations.ReadFile("migrations/" + f.Name())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(body)); e != nil {
			t.Fatal(e)
		}
	}
	_, e = db.Exec(`INSERT INTO businesses(id,name,state_code) VALUES(1,'Old shop','27');
 INSERT INTO parties(id,business_id,kind,name) VALUES(1,1,'SUPPLIER','Supplier');
 INSERT INTO products(id,business_id,name,sku,kind,sale_price) VALUES(1,1,'Old item','OLD','PRODUCT',10000);
 INSERT INTO warehouses(id,business_id,name) VALUES(1,1,'Main');
 INSERT INTO documents(id,business_id,kind,status,party_id,warehouse_id,date,due_date,total,snapshot) VALUES(1,1,'SALE','FINALIZED',1,1,'2026-10-15','2026-10-15',10001,'{}');
 INSERT INTO dispatches(id,business_id,party_id,warehouse_id,date,number,reference,snapshot,value,document_id) VALUES(1,1,1,1,'2026-10-15','DC/OLD/1','old-dispatch','{"lines":[{"product_id":1,"quantity":1000,"unit_factor":12000,"cost":10001}]}',10001,1);
 INSERT INTO dispatches(id,business_id,party_id,warehouse_id,date,number,reference,snapshot,value,returned_date) VALUES(2,1,1,1,'2026-10-15','DC/OLD/2','old-return','{"lines":[{"product_id":1,"quantity":2000,"cost":501}]}',501,'2026-10-16');
 PRAGMA user_version=25;`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var q, v, linked int64
	if e = db.QueryRow("SELECT quantity,value,document_id FROM dispatch_allocations WHERE dispatch_id=1").Scan(&q, &v, &linked); e != nil || q != 12000 || v != 10001 || linked != 1 {
		t.Fatal(q, v, linked, e)
	}
	if _, e = db.Exec("UPDATE dispatch_allocations SET quantity=1"); e == nil {
		t.Fatal("match mutable")
	}
	if _, e = db.Exec("DELETE FROM dispatches"); e == nil {
		t.Fatal("historical receipt deletable")
	}
	var returnedQuantity, returnedValue int64
	if e = db.QueryRow("SELECT quantity,value FROM dispatch_allocations WHERE dispatch_id=2 AND document_id IS NULL").Scan(&returnedQuantity, &returnedValue); e != nil || returnedQuantity != 2000 || returnedValue != 501 {
		t.Fatal(returnedQuantity, returnedValue, e)
	}
	rows, e := db.Query("PRAGMA foreign_key_check")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("broken foreign keys after rebuild")
	}
}
