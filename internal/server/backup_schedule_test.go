package server

import (
	"billbook/internal/backup"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScheduledBackupDueRetentionAndFailure(t *testing.T) {
	s, call := fixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	dir, e := s.backupDir()
	if e != nil {
		t.Fatal(e)
	}
	manual, e := backup.Create(s.DB, dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.scheduledBackup(now); e != nil {
		t.Fatal(e)
	}
	count := func() int {
		t.Helper()
		entries, e := os.ReadDir(dir)
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "Auto-") {
				n++
			}
		}
		return n
	}
	if count() != 0 {
		t.Fatal("disabled schedule ran")
	}
	call("POST", "backup-policy", `{"enabled":true,"interval_hours":0,"retain_count":2}`, 400)
	call("POST", "backup-policy", `{"enabled":true,"interval_hours":1,"retain_count":2}`, 200)
	for i := 0; i < 4; i++ {
		if e = s.scheduledBackup(now.Add(time.Duration(i) * time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	if count() != 2 {
		t.Fatal("retention did not keep two automatic snapshots")
	}
	if e = backup.Verify(filepath.Join(dir, manual)); e != nil {
		t.Fatal("manual backup removed", e)
	}
	if e = s.scheduledBackup(now.Add(3*time.Hour + 30*time.Minute)); e != nil {
		t.Fatal(e)
	}
	p, e := s.readBackupPolicy()
	if e != nil {
		t.Fatal(e)
	}
	if p.LastSuccess != now.Add(3*time.Hour).Format(time.RFC3339) {
		t.Fatal("schedule ran before due", p)
	}
	// An invalid destination must leave all existing snapshots intact and report failure.
	s.DataDir = t.TempDir()
	if e = os.WriteFile(filepath.Join(s.DataDir, "backups"), []byte("blocked"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.scheduledBackup(now.Add(4 * time.Hour)); e == nil {
		t.Fatal("expected backup failure")
	}
	p, e = s.readBackupPolicy()
	if e != nil || p.LastError == "" || p.LastSuccess != now.Add(3*time.Hour).Format(time.RFC3339) {
		t.Fatal("failure state", p, e)
	}
	if count() != 2 {
		t.Fatal("failed backup pruned old copies")
	}
	s.DataDir = ""
	if e = s.scheduledBackup(now.Add(4 * time.Hour)); e != nil {
		t.Fatal(e)
	}
	p, e = s.readBackupPolicy()
	if e != nil || p.LastError != "" {
		t.Fatal("failure did not clear", p, e)
	}
	call("GET", "backups", "", 200)
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "Auto-") {
			call("POST", "backup-verify", jsonText(t, map[string]string{"name": entry.Name()}), 200)
		}
	}
}
