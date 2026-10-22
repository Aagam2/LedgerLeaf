package money

import (
	"math"
	"testing"
)

func TestMulDiv(t *testing.T) {
	for _, c := range []struct{ a, b, d, w int64 }{{100000, 2000, 1000, 200000}, {1, 5, 10, 1}, {118000, 10000, 11800, 100000}, {math.MaxInt64, 2, 2, math.MaxInt64}} {
		v, e := MulDiv(c.a, c.b, c.d)
		if e != nil || v != c.w {
			t.Fatalf("%+v: %d %v", c, v, e)
		}
	}
	if _, e := MulDiv(math.MaxInt64, 2, 1); e == nil {
		t.Fatal("overflow accepted")
	}
	if _, e := MulDiv(-1, 1, 1); e == nil {
		t.Fatal("negative accepted")
	}
}
