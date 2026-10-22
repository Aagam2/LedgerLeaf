package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupRestoreAndRecovery(t *testing.T) {
	s, call := fixture(t)
	t.Cleanup(func() { s.DB.Close() })
	var result map[string]string
	if e := json.Unmarshal([]byte(call("POST", "backups", `{}`, 200)), &result); e != nil {
		t.Fatal(e)
	}
	name := result["name"]
	call("POST", "backup-verify", jsonText(t, map[string]string{"name": name}), 200)
	call("POST", "businesses", `{"name":"After backup","state_code":"27"}`, 201)
	call("POST", "restore", jsonText(t, map[string]string{"name": name, "confirm": "wrong"}), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 3 {
		t.Fatal("unconfirmed restore mutated database")
	}
	var restored map[string]string
	if e := json.Unmarshal([]byte(call("POST", "restore", jsonText(t, map[string]string{"name": name, "confirm": "RESTORE ALL BUSINESSES"}), 200)), &restored); e != nil {
		t.Fatal(e)
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 2 {
		t.Fatal("restore did not replace data")
	}
	call("POST", "restore", jsonText(t, map[string]string{"name": restored["recovery_backup"], "confirm": "RESTORE ALL BUSINESSES"}), 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 3 {
		t.Fatal("recovery snapshot did not retain prior data")
	}
	assertIntegrity(t, s.DB)
}
func TestCorruptBackupRejectedWithoutChangingData(t *testing.T) {
	s, call := fixture(t)
	dir, e := s.backupDir()
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	name := "Ledgerleaf-20261015T000000Z-1234567890abcdef.db"
	if e = os.WriteFile(filepath.Join(dir, name), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	call("POST", "restore", jsonText(t, map[string]string{"name": name, "confirm": "RESTORE ALL BUSINESSES"}), 400)
	call("POST", "backup-verify", `{"name":"../../outside.db"}`, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 2 {
		t.Fatal("corrupt restore mutated data")
	}
	assertIntegrity(t, s.DB)
}

func TestBackupImportValidatesAndPreservesLiveData(t *testing.T) {
	s, call := fixture(t)
	var created map[string]string
	if e := json.Unmarshal([]byte(call("POST", "backups", `{}`, 200)), &created); e != nil {
		t.Fatal(e)
	}
	data := call("GET", "backups?name="+created["name"], "", 200)
	call("POST", "businesses", `{"name":"Keep until restore","state_code":"27"}`, 201)
	var imported map[string]string
	if e := json.Unmarshal([]byte(call("POST", "backup-import", data, 201)), &imported); e != nil {
		t.Fatal(e)
	}
	if imported["name"] == created["name"] || !backupName.MatchString(imported["name"]) {
		t.Fatal("import did not get a new safe filename")
	}
	call("POST", "backup-verify", jsonText(t, imported), 200)
	call("POST", "backup-import", "not a database", 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM businesses") != 3 {
		t.Fatal("import changed live data")
	}
	dir, e := s.backupDir()
	if e != nil {
		t.Fatal(e)
	}
	staged, e := filepath.Glob(filepath.Join(dir, ".upload-*"))
	if e != nil || len(staged) != 0 {
		t.Fatal("staged upload leaked", staged, e)
	}
	assertIntegrity(t, s.DB)
}
