package server

import (
	"billbook/internal/backup"
	"billbook/internal/database"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var backupName = regexp.MustCompile(`^(Auto-)?Ledgerleaf-[0-9]{8}T[0-9]{6}Z-[a-f0-9]{16}\.db$`)

func (s *Server) backupDir() (string, error) {
	if s.DataDir != "" {
		return filepath.Join(s.DataDir, "backups"), nil
	}
	var seq int
	var name, path string
	if e := s.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); e != nil {
		return "", e
	}
	return filepath.Join(filepath.Dir(path), "backups"), nil
}
func (s *Server) backups(w http.ResponseWriter, r *http.Request) {
	dir, e := s.backupDir()
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "POST" {
		name, e := backup.Create(s.DB, dir)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]string{"name": name})
		return
	}
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := r.URL.Query().Get("name")
	if name != "" {
		if !backupName.MatchString(name) {
			fail(w, errors.New("invalid backup name"))
			return
		}
		path := filepath.Join(dir, name)
		if e = backup.Verify(path); e != nil {
			fail(w, e)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Content-Type", "application/vnd.sqlite3")
		http.ServeFile(w, r, path)
		return
	}
	entries, e := os.ReadDir(dir)
	if e != nil && !os.IsNotExist(e) {
		fail(w, e)
		return
	}
	out := []map[string]interface{}{}
	for _, entry := range entries {
		if !backupName.MatchString(entry.Name()) || entry.IsDir() {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			fail(w, e)
			return
		}
		out = append(out, map[string]interface{}{"name": entry.Name(), "bytes": info.Size(), "modified": info.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) > out[j]["name"].(string) })
	respond(w, 200, out)
}
func (s *Server) verifyBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Name string `json:"name"`
	}
	if e := decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !backupName.MatchString(p.Name) {
		fail(w, errors.New("invalid backup name"))
		return
	}
	dir, e := s.backupDir()
	if e != nil {
		fail(w, e)
		return
	}
	path := filepath.Join(dir, p.Name)
	if e = backup.Verify(path); e != nil {
		fail(w, e)
		return
	}
	f, e := os.Open(path)
	if e != nil {
		fail(w, e)
		return
	}
	defer f.Close()
	hash := sha256.New()
	if _, e = io.Copy(hash, f); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]string{"name": p.Name, "sha256": hex.EncodeToString(hash.Sum(nil)), "integrity": "ok"})
}

// restoreBackup runs under the server-wide exclusive request lock. The previous
// database is retained as a verified recovery snapshot before replacement.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Name    string `json:"name"`
		Confirm string `json:"confirm"`
	}
	if e := decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !backupName.MatchString(p.Name) || p.Confirm != "RESTORE ALL BUSINESSES" {
		fail(w, errors.New("confirm restore of all businesses"))
		return
	}
	dir, e := s.backupDir()
	if e != nil {
		fail(w, e)
		return
	}
	source := filepath.Join(dir, p.Name)
	if e = backup.Verify(source); e != nil {
		fail(w, e)
		return
	}
	// Stage and fsync the validated source before touching the live database.
	var seq int
	var name, path string
	if e = s.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); e != nil {
		fail(w, e)
		return
	}
	stage, e := os.CreateTemp(filepath.Dir(path), ".restore-*.db")
	if e != nil {
		fail(w, e)
		return
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)
	input, e := os.Open(source)
	if e != nil {
		stage.Close()
		fail(w, e)
		return
	}
	_, e = io.Copy(stage, input)
	input.Close()
	if e == nil {
		e = stage.Sync()
	}
	stage.Close()
	if e != nil {
		fail(w, e)
		return
	}
	if e = backup.Verify(stagePath); e != nil {
		fail(w, e)
		return
	}
	if s.PrepareRestore != nil {
		if e = s.PrepareRestore(stagePath); e != nil {
			fail(w, e)
			return
		}
	}
	recovery, e := backup.Create(s.DB, dir)
	if e != nil {
		fail(w, e)
		return
	}
	var busy, logFrames, checkpointed int
	if e = s.DB.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); e != nil || busy != 0 {
		fail(w, errors.New("database is busy; restore not applied"))
		return
	}
	if e = s.DB.Close(); e != nil {
		fail(w, e)
		return
	}
	// After a successful checkpoint and close, no committed data resides in WAL.
	for _, suffix := range []string{"-wal", "-shm"} {
		if e = os.Remove(path + suffix); e != nil && !os.IsNotExist(e) {
			if reopened, openErr := s.openDatabase(path); openErr == nil {
				s.DB = reopened
			} else {
				s.Exit()
			}
			fail(w, e)
			return
		}
	}
	if e = os.Rename(stagePath, path); e != nil {
		if reopened, openErr := s.openDatabase(path); openErr == nil {
			s.DB = reopened
		} else {
			s.Exit()
		}
		fail(w, e)
		return
	}
	reopened, openErr := s.openDatabase(path)
	if openErr != nil { // Keep the verified recovery path visible; never delete it.
		http.Error(w, "restore open failed; recovery backup: "+recovery, 500)
		s.Exit()
		return
	}
	s.DB = reopened
	s.invalidateSessions(0)
	respond(w, 200, map[string]string{"restored": p.Name, "recovery_backup": recovery})
}

func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	dir, e := s.backupDir()
	if e != nil {
		fail(w, e)
		return
	}
	// The filename is never trusted. Limit the stream before staging any bytes.
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	defer r.Body.Close()
	name, e := backup.Import(r.Body, dir)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, map[string]string{"name": name})
}

func (s *Server) openDatabase(path string) (*sql.DB, error) {
	if s.OpenDatabase != nil {
		return s.OpenDatabase(path)
	}
	return database.Open(path)
}
