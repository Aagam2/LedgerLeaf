package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

func checkIntegrity(db *sql.DB) error {
	var result string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("database integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("database integrity check failed; preserve the data directory and restore a verified backup")
	}
	return nil
}

// Preserve the last usable schema before an upgrade. VACUUM INTO (or keyed
// sqlcipher_export) includes committed WAL records; copying the live DB alone
// would lose them. These snapshots deliberately bypass scheduled-backup pruning.
func preserveBeforeUpgrade(db *sql.DB, path, password string) (string, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return "", err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return "", err
	}
	if version > len(files) {
		return "", fmt.Errorf("database schema %d is newer than this application supports", version)
	}
	if version == 0 || version == len(files) {
		return "", nil
	}
	dir := filepath.Join(filepath.Dir(path), "pre-upgrade")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot preserve database before upgrade: %w", err)
	}
	stage, err := os.MkdirTemp(dir, fmt.Sprintf("schema-%02d-to-%02d-", version, len(files)))
	if err != nil {
		return "", fmt.Errorf("cannot preserve database before upgrade: %w", err)
	}
	snapshot := filepath.Join(stage, filepath.Base(path))
	if err = Export(db, snapshot, password); err != nil {
		os.RemoveAll(stage)
		return "", fmt.Errorf("upgrade cancelled because the safety snapshot failed: %w", err)
	}
	for _, parent := range []string{stage, dir, filepath.Dir(dir)} {
		if err = syncDirectory(parent); err != nil {
			return "", fmt.Errorf("upgrade cancelled because the safety snapshot directory could not be synced: %w", err)
		}
	}
	return snapshot, nil
}
