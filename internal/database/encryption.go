package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

func CipherAvailable() bool {
	db, e := sql.Open("sqlite3", ":memory:")
	if e != nil {
		return false
	}
	defer db.Close()
	var version string
	return db.QueryRow("PRAGMA cipher_version").Scan(&version) == nil && version != ""
}

func ValidDatabasePassword(password string) bool {
	return len(password) >= 12 && len(password) <= 256 && !strings.ContainsAny(password, "\x00\r\n")
}

// Export writes a new standalone database. SQLCipher exports are transactional;
// the original connection remains keyed and the destination is explicitly keyed.
func Export(db *sql.DB, path, password string) error {
	if _, e := os.Lstat(path); e == nil {
		return errors.New("export destination already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	if password != "" && !ValidDatabasePassword(password) {
		return errors.New("database password must be 12–256 bytes without line breaks or NUL")
	}
	if !CipherAvailable() {
		if password != "" {
			return errors.New("this build does not support SQLCipher encryption")
		}
		if _, e := db.Exec("VACUUM INTO ?", path); e != nil {
			return e
		}
		return syncExport(path)
	}
	ctx := context.Background()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	// Parameters prevent both paths and passwords from becoming SQL syntax.
	if _, e = conn.ExecContext(ctx, "ATTACH DATABASE ? AS ledgerleaf_export KEY ?", path, password); e != nil {
		return errors.New("cannot attach export destination")
	}
	defer conn.ExecContext(ctx, "DETACH DATABASE ledgerleaf_export")
	var version int
	if e = conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); e != nil {
		return e
	}
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("SELECT sqlcipher_export('ledgerleaf_export')"); e != nil {
		return e
	}
	if _, e = tx.Exec(fmt.Sprintf("PRAGMA ledgerleaf_export.user_version=%d", version)); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	var integrity string
	if e = conn.QueryRowContext(ctx, "PRAGMA ledgerleaf_export.integrity_check").Scan(&integrity); e != nil || integrity != "ok" {
		return errors.New("export integrity check failed")
	}
	if password != "" {
		var status int
		if e = conn.QueryRowContext(ctx, "PRAGMA ledgerleaf_export.cipher_status").Scan(&status); e != nil || status != 1 {
			return errors.New("export encryption was not activated")
		}
	}
	return syncExport(path)
}

func syncExport(path string) error {
	if e := os.Chmod(path, 0600); e != nil {
		return e
	}
	file, e := os.OpenFile(path, os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer file.Close()
	return file.Sync()
}
