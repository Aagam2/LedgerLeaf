package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateRetainsRecentLogsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application.log")
	w, e := Open(path, 8, 2)
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range []string{"first\n", "second\n", "third\n"} {
		if _, e = w.Write([]byte(line)); e != nil {
			t.Fatal(e)
		}
	}
	w.Close()
	w, e = Open(path, 8, 2)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	if _, e = w.Write([]byte("fourth\n")); e != nil {
		t.Fatal(e)
	}
	for suffix, want := range map[string]string{"": "fourth\n", ".1": "third\n", ".2": "second\n"} {
		got, e := os.ReadFile(path + suffix)
		if e != nil || string(got) != want {
			t.Fatal(suffix, string(got), e)
		}
	}
	if _, e = os.Stat(path + ".3"); !os.IsNotExist(e) {
		t.Fatal("retention exceeded")
	}
	w.Close()
	if _, e = w.Write([]byte("closed")); e == nil {
		t.Fatal("write to closed log succeeded")
	}
}
