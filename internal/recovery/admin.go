// Package recovery implements local, offline owner recovery. The caller must
// hold the installation's exclusive instance lock throughout the operation.
package recovery

import (
	"billbook/internal/backup"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

func ResetAdmin(db *sql.DB, backupDir, username string) (password, snapshot string, err error) {
	var id int64
	if err = db.QueryRow("SELECT id FROM users WHERE username=? AND role='ADMIN'", strings.ToLower(strings.TrimSpace(username))).Scan(&id); err != nil {
		return "", "", errors.New("administrator not found; recovery cannot promote another role")
	}
	secret := make([]byte, 24)
	if _, err = rand.Read(secret); err != nil {
		return "", "", err
	}
	password = base64.RawURLEncoding.EncodeToString(secret)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", "", err
	}
	if snapshot, err = backup.Create(db, backupDir); err != nil {
		return "", "", err
	}
	tx, err := db.Begin()
	if err != nil {
		return "", snapshot, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE users SET password_hash=?,active=1 WHERE id=? AND role='ADMIN'", string(hash), id); err != nil {
		return "", snapshot, err
	}
	if _, err = tx.Exec("INSERT INTO audit_log(action,entity_id,actor_id) VALUES('admin.password.recovered',?,?)", id, id); err != nil {
		return "", snapshot, err
	}
	if err = tx.Commit(); err != nil {
		return "", snapshot, err
	}
	return password, snapshot, nil
}
