package accounting

import (
	"billbook/internal/database"
	"path/filepath"
	"testing"
)

func TestBalancedPosting(t *testing.T) {
	db, e := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.Exec("INSERT INTO businesses(name,state_code) VALUES('Test','27')")
	tx, e := db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if e = Seed(tx, 1); e != nil {
		t.Fatal(e)
	}
	if e = Post(tx, 1, "2026-10-15", "bad", "Bad", []Line{{Account: "1200", Debit: 100}, {Account: "3000", Credit: 99}}); e == nil {
		t.Fatal("unbalanced entry accepted")
	}
	var count int
	tx.QueryRow("SELECT COUNT(*) FROM journals").Scan(&count)
	if count != 0 {
		t.Fatal("invalid entry persisted")
	}
	if e = Post(tx, 1, "2026-10-15", "good", "Good", []Line{{Account: "1200", Debit: 100}, {Account: "3000", Credit: 100}}); e != nil {
		t.Fatal(e)
	}
	if e = Post(tx, 1, "2026-10-15", "good", "Duplicate", []Line{{Account: "1200", Debit: 100}, {Account: "3000", Credit: 100}}); e == nil {
		t.Fatal("duplicate accepted")
	}
}
