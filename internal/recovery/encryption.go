package recovery

import (
	"billbook/internal/database"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EncryptOffline requires the caller to hold the installation's instance lock.
// It preserves the complete plaintext original; it never deletes recovery data.
func EncryptOffline(dir, password string) (string, error) {
	if !database.ValidDatabasePassword(password) || !database.CipherAvailable() {
		return "", errors.New("SQLCipher build and a 12–256 byte database password are required")
	}
	path := filepath.Join(dir, "data", "billbook.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, e := os.Lstat(path + suffix)
		if suffix != "" && os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("refusing to convert a non-regular database file")
		}
	}
	db, e := database.Open(path)
	if e != nil {
		return "", e
	}
	defer db.Close()
	f, e := os.CreateTemp(filepath.Dir(path), ".encrypt-*.db")
	if e != nil {
		return "", e
	}
	stage := f.Name()
	f.Close()
	os.Remove(stage)
	defer os.Remove(stage)
	if e = database.Export(db, stage, password); e != nil {
		return "", e
	}
	checked, e := database.OpenEncrypted(stage, password)
	if e != nil {
		return "", e
	}
	if e = checked.Close(); e != nil {
		return "", e
	}
	var busy, frames, done int
	if e = db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &done); e != nil || busy != 0 {
		return "", errors.New("database busy; encryption not applied")
	}
	if e = db.Close(); e != nil {
		return "", e
	}
	preserved, e := os.MkdirTemp(filepath.Join(dir, "backups"), "pre-encryption-")
	if e != nil {
		return "", e
	}
	moved := []string{}
	rollback := func(cause error) error {
		for i := len(moved) - 1; i >= 0; i-- {
			suffix := moved[i]
			if err := os.Rename(filepath.Join(preserved, "billbook.db"+suffix), path+suffix); err != nil {
				cause = errors.Join(cause, fmt.Errorf("original retained in %s: %w", preserved, err))
			}
		}
		return cause
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, e = os.Stat(path + suffix); os.IsNotExist(e) {
			continue
		}
		if e = os.Rename(path+suffix, filepath.Join(preserved, "billbook.db"+suffix)); e != nil {
			return preserved, rollback(e)
		}
		moved = append(moved, suffix)
	}
	if e = os.Rename(stage, path); e != nil {
		return preserved, rollback(e)
	}
	return preserved, nil
}

// EncryptRestoreStage replaces a disposable verified plaintext restore stage.
func EncryptRestoreStage(path, password string) error {
	source, e := database.Open(path)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".keyed-restore-*.db")
	if e != nil {
		source.Close()
		return e
	}
	target := f.Name()
	f.Close()
	os.Remove(target)
	defer os.Remove(target)
	e = database.Export(source, target, password)
	closeErr := source.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	// Windows cannot replace an existing destination. The stage is disposable.
	if e = os.Remove(path); e != nil {
		return e
	}
	return os.Rename(target, path)
}
