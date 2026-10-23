//go:build sqlite_omit_load_extension

package database

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseDisablesNativeSQLiteExtensionLoading(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "release.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var disabled int
	if err = db.QueryRow("SELECT sqlite_compileoption_used('OMIT_LOAD_EXTENSION')").Scan(&disabled); err != nil || disabled != 1 {
		t.Fatal("release still includes native extension loading", disabled, err)
	}
	_, err = db.Exec("SELECT load_extension('unused-test-extension')")
	if err == nil || !strings.Contains(err.Error(), "no such function") {
		t.Fatal("load_extension function remains available", err)
	}
}
