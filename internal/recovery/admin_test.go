package recovery

import (
	"billbook/internal/backup"
	"billbook/internal/database"
	"golang.org/x/crypto/bcrypt"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryPreservesBackupAndRejectsNonAdmin(t *testing.T) {
	dir := t.TempDir()
	db, e := database.Open(filepath.Join(dir, "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec("INSERT INTO users(username,password_hash,role,active) VALUES('owner','old-hash','ADMIN',0),('viewer','viewer-hash','VIEWER',1)")
	if e != nil {
		t.Fatal(e)
	}
	backups := filepath.Join(dir, "backups")
	if _, _, e = ResetAdmin(db, backups, "viewer"); e == nil {
		t.Fatal("promoted viewer")
	}
	if _, _, e = ResetAdmin(db, backups, "missing"); e == nil {
		t.Fatal("created unknown administrator")
	}
	blocked := filepath.Join(dir, "blocked")
	os.WriteFile(blocked, []byte("blocked"), 0600)
	if _, _, e = ResetAdmin(db, blocked, "owner"); e == nil {
		t.Fatal("reset without backup")
	}
	var hash string
	if e = db.QueryRow("SELECT password_hash FROM users WHERE username='owner'").Scan(&hash); e != nil || hash != "old-hash" {
		t.Fatal("failed recovery modified password", e)
	}
	password, snapshot, e := ResetAdmin(db, backups, "OWNER")
	if e != nil {
		t.Fatal(e)
	}
	if e = backup.Verify(filepath.Join(backups, snapshot)); e != nil {
		t.Fatal(e)
	}
	var active int
	if e = db.QueryRow("SELECT password_hash,active FROM users WHERE username='owner'").Scan(&hash, &active); e != nil {
		t.Fatal(e)
	}
	if active != 1 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		t.Fatal("generated password is unusable")
	}
	var n int
	if e = db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action='admin.password.recovered'").Scan(&n); e != nil || n != 1 {
		t.Fatal("missing recovery audit", e)
	}
	old, e := database.Open(filepath.Join(backups, snapshot))
	if e != nil {
		t.Fatal(e)
	}
	defer old.Close()
	if e = old.QueryRow("SELECT password_hash,active FROM users WHERE username='owner'").Scan(&hash, &active); e != nil || hash != "old-hash" || active != 0 {
		t.Fatal("recovery backup lost prior credentials", e)
	}
}
