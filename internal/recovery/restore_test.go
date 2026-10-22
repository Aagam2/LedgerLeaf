package recovery

import (
	"billbook/internal/backup"
	"billbook/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineRestorePreservesCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "data"), 0700)
	os.MkdirAll(filepath.Join(dir, "backups"), 0700)
	source := filepath.Join(t.TempDir(), "source.db")
	db, e := database.Open(source)
	if e != nil {
		t.Fatal(e)
	}
	name, e := backup.Create(db, filepath.Join(dir, "backups"))
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	target := filepath.Join(dir, "data", "billbook.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if e = os.WriteFile(target+suffix, []byte("original corrupt "+suffix), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = RestoreOffline(dir, target); e == nil {
		t.Fatal("accepted corrupt backup")
	}
	preserved, e := RestoreOffline(dir, filepath.Join(dir, "backups", name))
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.Verify(target); e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		got, e := os.ReadFile(filepath.Join(preserved, "billbook.db"+suffix))
		if e != nil || string(got) != "original corrupt "+suffix {
			t.Fatal("forensic original lost", suffix, e)
		}
	}
}

func TestEncryptedOfflineRestore(t *testing.T) {
	dir := t.TempDir()
	if e := os.MkdirAll(filepath.Join(dir, "data"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(dir, "backups"), 0700); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(t.TempDir(), "source.db")
	db, e := database.Open(source)
	if e != nil {
		t.Fatal(e)
	}
	name, e := backup.Create(db, filepath.Join(dir, "backups"))
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	plain, e := os.Open(filepath.Join(dir, "backups", name))
	if e != nil {
		t.Fatal(e)
	}
	defer plain.Close()
	info, e := plain.Stat()
	if e != nil {
		t.Fatal(e)
	}
	encrypted := filepath.Join(t.TempDir(), "snapshot.llbenc")
	out, e := os.Create(encrypted)
	if e != nil {
		t.Fatal(e)
	}
	password := "offline-restore-test-password"
	e = backup.Encrypt(out, plain, info.Size(), password)
	out.Close()
	if e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(dir, "data", "billbook.db")
	original := []byte("original corrupt installation")
	if e = os.WriteFile(target, original, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = RestoreEncryptedOffline(dir, encrypted, "incorrect-password"); e == nil {
		t.Fatal("accepted incorrect password")
	}
	got, e := os.ReadFile(target)
	if e != nil || string(got) != string(original) {
		t.Fatal("original changed after decryption failure", e)
	}
	preserved, e := RestoreEncryptedOffline(dir, encrypted, password)
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.Verify(target); e != nil {
		t.Fatal(e)
	}
	got, e = os.ReadFile(filepath.Join(preserved, "billbook.db"))
	if e != nil || string(got) != string(original) {
		t.Fatal("original not retained", e)
	}
	stages, e := filepath.Glob(filepath.Join(dir, "data", ".decrypt-restore-*"))
	if e != nil || len(stages) != 0 {
		t.Fatal("decrypted staging file not cleaned", stages, e)
	}
}
