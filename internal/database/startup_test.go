package database

import (
	"bytes"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEncryptedUpgradeSnapshotKeepsEncryption(t *testing.T) {
	if !CipherAvailable() {
		t.Skip("requires SQLCipher build")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	password := "Upgrade test private password"
	db, err := OpenEncrypted(path, password)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Snapshot creation must retain its input schema and key without attempting
	// to migrate the standalone copy.
	if _, err = db.Exec("PRAGMA user_version=25; INSERT INTO businesses(name,state_code) VALUES('Private marker','27')"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := preserveBeforeUpgrade(db, path, password)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(snapshot)
	if err != nil || bytes.HasPrefix(raw, []byte("SQLite format 3")) || bytes.Contains(raw, []byte("Private marker")) {
		t.Fatal("upgrade snapshot exposed plaintext", err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(snapshot)}
	copyDB, err := sql.Open("sqlite3", u.String()+"?mode=ro&textkey="+url.QueryEscape(password))
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var name string
	if err = copyDB.QueryRow("SELECT name FROM businesses").Scan(&name); err != nil || name != "Private marker" {
		t.Fatal("keyed snapshot is unreadable", name, err)
	}
}

func oldLedger(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	body, err := migrations.ReadFile("migrations/001_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(body)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA user_version=1; INSERT INTO businesses(name,state_code) VALUES('Committed in WAL','27')"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpgradeSnapshotIncludesCommittedWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	oldLedger(t, path) // Keep the writer open so committed pages remain in WAL.
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	paths, err := filepath.Glob(filepath.Join(dir, "pre-upgrade", "*", "ledger.db"))
	if err != nil || len(paths) != 1 {
		t.Fatal(paths, err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(paths[0])}
	snapshot, err := sql.Open("sqlite3", u.String()+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	var version int
	var name string
	if err = snapshot.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatal(version, err)
	}
	if err = snapshot.QueryRow("SELECT name FROM businesses").Scan(&name); err != nil || name != "Committed in WAL" {
		t.Fatal(name, err)
	}
	if err = checkIntegrity(snapshot); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(paths[0])
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("snapshot is not private", info, err)
		}
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	again, _ := filepath.Glob(filepath.Join(dir, "pre-upgrade", "*", "ledger.db"))
	if len(again) != 1 {
		t.Fatal("ordinary restart created another upgrade snapshot", again)
	}
}

func TestUpgradeStopsWhenSnapshotCannotBeSaved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	original := oldLedger(t, path)
	if err := os.WriteFile(filepath.Join(dir, "pre-upgrade"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if db, err := Open(path); err == nil {
		db.Close()
		t.Fatal("upgrade ran without a safety snapshot")
	}
	var version int
	if err := original.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatal("schema changed despite failed snapshot", version, err)
	}
}

func TestFailedUpgradeRetainsSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	original := oldLedger(t, path)
	// Conflict with a later migration: preserve the ledger, roll back the batch.
	if _, err := original.Exec("CREATE TABLE users(sentinel TEXT)"); err != nil {
		t.Fatal(err)
	}
	db, err := Open(path)
	if err == nil {
		db.Close()
		t.Fatal("expected migration failure")
	}
	if !strings.Contains(err.Error(), "pre-upgrade snapshot retained at") {
		t.Fatal("missing recovery location", err)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "pre-upgrade", "*", "ledger.db"))
	if len(paths) != 1 {
		t.Fatal("lost snapshot", paths)
	}
	var version int
	if err = original.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatal("failed migration changed schema", version, err)
	}
}
