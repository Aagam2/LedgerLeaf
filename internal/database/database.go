package database

import (
	"database/sql"
	"embed"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"path/filepath"
	"strings"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(path string) (*sql.DB, error) { return OpenEncrypted(path, "") }

func OpenEncrypted(path, password string) (*sql.DB, error) {
	if password != "" {
		if !ValidDatabasePassword(password) {
			return nil, fmt.Errorf("database password must be 12–256 bytes without line breaks or NUL")
		}
		if !CipherAvailable() {
			return nil, fmt.Errorf("this build does not support SQLCipher encryption")
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	options := "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate&_synchronous=FULL"
	if password != "" {
		options += "&textkey=" + strings.ReplaceAll(url.QueryEscape(password), "+", "%20")
	}
	db, err := sql.Open("sqlite3", u.String()+options)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if password != "" {
		var status int
		if e := db.QueryRow("PRAGMA cipher_status").Scan(&status); e != nil || status != 1 {
			db.Close()
			return nil, fmt.Errorf("database encryption is not active")
		}
	}
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	if version > len(files) {
		return fmt.Errorf("database schema %d is newer than this application supports", version)
	}
	for i, f := range files {
		if i < version {
			continue
		}
		body, e := migrations.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		if _, e = tx.Exec(string(body)); e != nil {
			return fmt.Errorf("migration %s: %w", f.Name(), e)
		}
		if _, e = tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", i+1)); e != nil {
			return e
		}
	}
	return tx.Commit()
}
