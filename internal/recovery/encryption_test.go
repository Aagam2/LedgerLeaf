package recovery

import (
	"billbook/internal/backup"
	"billbook/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineEncryptionAndRestoreStage(t *testing.T) {
	if !database.CipherAvailable() {
		t.Skip("requires the SQLCipher build")
	}
	dir := t.TempDir()
	for _, part := range []string{"data", "backups"} {
		if e := os.MkdirAll(filepath.Join(dir, part), 0700); e != nil {
			t.Fatal(e)
		}
	}
	path := filepath.Join(dir, "data", "billbook.db")
	db, e := database.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("INSERT INTO businesses(name,state_code) VALUES('Preserved business','27')"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	password := "offline test &=?#' passphrase"
	preserved, e := EncryptOffline(dir, password)
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.Verify(filepath.Join(preserved, "billbook.db")); e != nil {
		t.Fatal(e)
	}
	db, e = database.OpenEncrypted(path, password)
	if e != nil {
		t.Fatal(e)
	}
	snapshot, e := backup.Create(db, filepath.Join(dir, "backups"))
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	stage := filepath.Join(dir, "backups", snapshot)
	if e = EncryptRestoreStage(stage, password); e != nil {
		t.Fatal(e)
	}
	restored, e := database.OpenEncrypted(stage, password)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	var name string
	if e = restored.QueryRow("SELECT name FROM businesses").Scan(&name); e != nil || name != "Preserved business" {
		t.Fatal(name, e)
	}
}
