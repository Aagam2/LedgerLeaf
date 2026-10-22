package server

import (
	"billbook/internal/backup"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type backupPolicy struct {
	Enabled       bool   `json:"enabled"`
	IntervalHours int    `json:"interval_hours"`
	RetainCount   int    `json:"retain_count"`
	LastSuccess   string `json:"last_success"`
	LastError     string `json:"last_error"`
}

func (s *Server) readBackupPolicy() (p backupPolicy, err error) {
	err = s.DB.QueryRow("SELECT enabled,interval_hours,retain_count,last_success,last_error FROM backup_policy WHERE id=1").Scan(&p.Enabled, &p.IntervalHours, &p.RetainCount, &p.LastSuccess, &p.LastError)
	return
}

func (s *Server) backupSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var p struct {
			Enabled       bool `json:"enabled"`
			IntervalHours int  `json:"interval_hours"`
			RetainCount   int  `json:"retain_count"`
		}
		if e := decode(w, r, &p); e != nil {
			fail(w, e)
			return
		}
		if p.IntervalHours < 1 || p.IntervalHours > 720 || p.RetainCount < 1 || p.RetainCount > 100 {
			fail(w, errors.New("choose an interval of 1–720 hours and retain 1–100 scheduled backups"))
			return
		}
		tx, e := s.DB.Begin()
		if e != nil {
			fail(w, e)
			return
		}
		defer tx.Rollback()
		if _, e = tx.Exec("UPDATE backup_policy SET enabled=?,interval_hours=?,retain_count=? WHERE id=1", p.Enabled, p.IntervalHours, p.RetainCount); e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO audit_log(action,entity_id,actor_id) VALUES('backup.policy.updated',1,?)", nullableActor(actorOf(r).ID)); e != nil {
			fail(w, e)
			return
		}
		if e = tx.Commit(); e != nil {
			fail(w, e)
			return
		}
	} else if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	p, e := s.readBackupPolicy()
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, p)
}

// RunBackupSchedule shares the restore lock so snapshots never use a replaced
// database connection. Callers must wait for this worker before closing the DB.
func (s *Server) RunBackupSchedule(ctx context.Context, report func(error)) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if e := s.scheduledBackup(time.Now().UTC()); e != nil && report != nil {
			report(e)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (s *Server) scheduledBackup(now time.Time) (err error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	p, e := s.readBackupPolicy()
	if e != nil || !p.Enabled {
		return e
	}
	if p.LastSuccess != "" {
		last, e := time.Parse(time.RFC3339, p.LastSuccess)
		if e != nil {
			return e
		}
		if now.Before(last.Add(time.Duration(p.IntervalHours) * time.Hour)) {
			return nil
		}
	}
	defer func() {
		if err != nil {
			_, saveErr := s.DB.Exec("UPDATE backup_policy SET last_error=? WHERE id=1", err.Error())
			if saveErr != nil {
				err = fmt.Errorf("%v; recording backup failure: %w", err, saveErr)
			}
		}
	}()
	dir, e := s.backupDir()
	if e != nil {
		return e
	}
	name, e := backup.Create(s.DB, dir)
	if e != nil {
		return e
	}
	autoName := "Auto-" + name
	if e = os.Rename(filepath.Join(dir, name), filepath.Join(dir, autoName)); e != nil {
		return e
	}
	// Never prune unless a fresh verified snapshot has been saved successfully.
	if _, e = s.DB.Exec("UPDATE backup_policy SET last_success=?,last_error='' WHERE id=1", now.Format(time.RFC3339)); e != nil {
		return e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	type candidate struct {
		name     string
		modified time.Time
	}
	var names []candidate
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "Auto-") || !backupName.MatchString(entry.Name()) {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if info.Mode().IsRegular() {
			names = append(names, candidate{entry.Name(), info.ModTime()})
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i].name == autoName && names[j].name != autoName {
			return true
		}
		if names[j].name == autoName {
			return false
		}
		if names[i].modified.Equal(names[j].modified) {
			return names[i].name > names[j].name
		}
		return names[i].modified.After(names[j].modified)
	})
	for i := p.RetainCount; i < len(names); i++ {
		if e = os.Remove(filepath.Join(dir, names[i].name)); e != nil {
			return e
		}
	}
	return nil
}
