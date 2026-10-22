package instance

import (
	"path/filepath"
	"testing"
)

func TestExclusive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "instance.lock")
	f, e := Lock(p)
	if e != nil {
		t.Fatal(e)
	}
	if g, e := Lock(p); e == nil {
		g.Close()
		t.Fatal("second instance acquired lock")
	}
	f.Close()
	g, e := Lock(p)
	if e != nil {
		t.Fatal(e)
	}
	g.Close()
}
