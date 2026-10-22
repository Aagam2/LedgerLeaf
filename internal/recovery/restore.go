package recovery

import (
	"billbook/internal/backup"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RestoreEncryptedOffline decrypts into a private staging file. The existing
// installation is untouched until authentication and database verification pass.
// Like RestoreOffline, the caller must hold the installation lock.
func RestoreEncryptedOffline(dataDir, source, password string) (string, error) {
	src, e := os.Open(source)
	if e != nil {
		return "", e
	}
	defer src.Close()
	stage, e := os.CreateTemp(filepath.Join(dataDir, "data"), ".decrypt-restore-*")
	if e != nil {
		return "", e
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	if e = backup.Decrypt(stage, src, password); e != nil {
		return "", fmt.Errorf("backup decryption failed; current files unchanged: %w", e)
	}
	if e = stage.Sync(); e != nil {
		return "", e
	}
	if e = stage.Close(); e != nil {
		return "", e
	}
	return RestoreOffline(dataDir, stage.Name())
}

// RestoreOffline requires the caller to hold the installation lock. It preserves
// the original DB/WAL/SHM bytes, including corrupt files, before replacing them.
func RestoreOffline(dataDir, source string) (string, error) {
	if e := backup.Verify(source); e != nil {
		return "", fmt.Errorf("backup verification failed; current files unchanged: %w", e)
	}
	folder := filepath.Join(dataDir, "data")
	target := filepath.Join(folder, "billbook.db")
	if _, e := os.Stat(target); e != nil {
		return "", errors.New("offline restore requires an existing installation")
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, e := os.Lstat(target + suffix)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("refusing to replace a non-regular database file")
		}
	}
	in, e := os.Open(source)
	if e != nil {
		return "", e
	}
	defer in.Close()
	stage, e := os.CreateTemp(folder, ".restore-*")
	if e != nil {
		return "", e
	}
	defer os.Remove(stage.Name())
	defer stage.Close()
	if _, e = io.Copy(stage, in); e != nil {
		return "", e
	}
	if e = stage.Sync(); e != nil {
		return "", e
	}
	if e = stage.Close(); e != nil {
		return "", e
	}
	if e = backup.Verify(stage.Name()); e != nil {
		return "", e
	}
	preserved, e := os.MkdirTemp(filepath.Join(dataDir, "backups"), "pre-offline-restore-")
	if e != nil {
		return "", e
	}
	moved := []string{}
	rollback := func(cause error) error {
		for i := len(moved) - 1; i >= 0; i-- {
			suffix := moved[i]
			if e := os.Rename(filepath.Join(preserved, "billbook.db"+suffix), target+suffix); e != nil {
				cause = errors.Join(cause, fmt.Errorf("original file retained in %s: %w", preserved, e))
			}
		}
		return cause
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, e = os.Stat(target + suffix); os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return preserved, rollback(e)
		}
		if e = os.Rename(target+suffix, filepath.Join(preserved, "billbook.db"+suffix)); e != nil {
			return preserved, rollback(e)
		}
		moved = append(moved, suffix)
	}
	if e = os.Rename(stage.Name(), target); e != nil {
		return preserved, rollback(e)
	}
	return preserved, nil
}
