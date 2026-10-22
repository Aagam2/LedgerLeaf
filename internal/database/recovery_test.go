package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeFromVersionOnePreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, e := sql.Open("sqlite3", path)
	if e != nil {
		t.Fatal(e)
	}
	first, e := migrations.ReadFile("migrations/001_foundation.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(first)); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO businesses(name,state_code) VALUES('Existing business','27'); PRAGMA user_version=1"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if e = db.QueryRow("SELECT name FROM businesses").Scan(&name); e != nil || name != "Existing business" {
		t.Fatal(name, e)
	}
	var count int
	if e = db.QueryRow("SELECT COUNT(*) FROM warehouses WHERE business_id=1").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 26 {
		t.Fatal(version, e)
	}
}

func TestFailedMigrationRollsBackWholeBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conflict.db")
	db, e := sql.Open("sqlite3", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec("CREATE TABLE products(sentinel TEXT); INSERT INTO products VALUES('keep')"); e != nil {
		t.Fatal(e)
	}
	if e = migrate(db); e == nil {
		t.Fatal("expected migration conflict")
	}
	var version, count int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil || version != 0 {
		t.Fatal(version, e)
	}
	if e = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name IN ('businesses','parties','audit_log')").Scan(&count); e != nil || count != 0 {
		t.Fatal("partial schema persisted", count, e)
	}
	var sentinel string
	if e = db.QueryRow("SELECT sentinel FROM products").Scan(&sentinel); e != nil || sentinel != "keep" {
		t.Fatal(sentinel, e)
	}
}

func TestRejectCorruptDatabaseWithoutOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	original := []byte("This is not a SQLite database.")
	if e := os.WriteFile(path, original, 0600); e != nil {
		t.Fatal(e)
	}
	if db, e := Open(path); e == nil {
		db.Close()
		t.Fatal("accepted corrupt database")
	}
	got, e := os.ReadFile(path)
	if e != nil || string(got) != string(original) {
		t.Fatal("overwrote corrupt input", e)
	}
}

func TestSpecialCharactersInDatabasePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "books ?# & space.db")
	db, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	if _, e = os.Stat(path); e != nil {
		t.Fatal(e)
	}
}
