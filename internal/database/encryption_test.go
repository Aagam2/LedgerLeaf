package database

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingDatabaseEncryption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "encrypted.db")
	password := "database test &=?#' passphrase"
	if !CipherAvailable() {
		if db, e := OpenEncrypted(path, password); e == nil {
			db.Close()
			t.Fatal("ordinary build accepted encryption")
		}
		if _, e := os.Stat(path); !os.IsNotExist(e) {
			t.Fatal("unsupported encryption created a database")
		}
		return
	}
	db, e := OpenEncrypted(path, password)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO businesses(name,state_code) VALUES('Encrypted test marker','27')"); e != nil {
		t.Fatal(e)
	}
	plain := filepath.Join(t.TempDir(), "plain.db")
	if e = Export(db, plain, ""); e != nil {
		t.Fatal(e)
	}
	db.Close()
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.HasPrefix(raw, []byte("SQLite format 3")) || bytes.Contains(raw, []byte("Encrypted test marker")) {
		t.Fatal("plaintext working database")
	}
	if bad, e := OpenEncrypted(path, "incorrect-passphrase"); e == nil {
		bad.Close()
		t.Fatal("wrong key accepted")
	}
	db, e = OpenEncrypted(path, password)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var name string
	if e = db.QueryRow("SELECT name FROM businesses").Scan(&name); e != nil || name != "Encrypted test marker" {
		t.Fatal(name, e)
	}
	copied, e := Open(plain)
	if e != nil {
		t.Fatal(e)
	}
	defer copied.Close()
	if e = copied.QueryRow("SELECT name FROM businesses").Scan(&name); e != nil {
		t.Fatal(e)
	}
}
