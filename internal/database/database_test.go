package database

import (
	"path/filepath"
	"testing"
)

func TestPersistenceAndVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO businesses(name,state_code) VALUES('Test','27')"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if e = db.QueryRow("SELECT name FROM businesses").Scan(&name); e != nil || name != "Test" {
		t.Fatal(name, e)
	}
	var fk int
	db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatal("foreign keys disabled")
	}
	db.Exec("PRAGMA user_version=999")
	db.Close()
	if db, e = Open(p); e == nil {
		db.Close()
		t.Fatal("accepted future schema")
	}
}
