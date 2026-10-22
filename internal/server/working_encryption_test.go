package server

import (
	"billbook/internal/database"
	"billbook/internal/recovery"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestEncryptedWorkingDatabaseRestore(t *testing.T) {
	if !database.CipherAvailable() {
		t.Skip("requires SQLCipher build")
	}
	s, call := fixture(t)
	path := filepath.Join(t.TempDir(), "working.db")
	password := "server encryption &=?#' passphrase"
	if e := database.Export(s.DB, path, password); e != nil {
		t.Fatal(e)
	}
	s.DB.Close()
	var e error
	s.DB, e = database.OpenEncrypted(path, password)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	s.OpenDatabase = func(p string) (*sql.DB, error) { return database.OpenEncrypted(p, password) }
	s.PrepareRestore = func(p string) error { return recovery.EncryptRestoreStage(p, password) }
	var result map[string]string
	if e = json.Unmarshal([]byte(call("POST", "backups", `{}`, 200)), &result); e != nil {
		t.Fatal(e)
	}
	call("POST", "businesses", `{"name":"After encrypted backup","state_code":"27"}`, 201)
	call("POST", "restore", jsonText(t, map[string]string{"name": result["name"], "confirm": "RESTORE ALL BUSINESSES"}), 200)
	if scalar(t, s.DB, "PRAGMA cipher_status") != 1 || scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 2 {
		t.Fatal("restore lost encryption or business data")
	}
	assertIntegrity(t, s.DB)
}
