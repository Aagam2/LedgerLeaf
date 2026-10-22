package gst

import "testing"

func TestGSTCalculation(t *testing.T) {
	for _, tc := range []struct {
		name             string
		net, rate, cess  int64
		inclusive, inter bool
		want             Amounts
	}{
		{"intrastate", 200000, 1800, 0, false, false, Amounts{200000, 18000, 18000, 0, 0, 236000}},
		{"interstate inclusive", 118000, 1800, 0, true, true, Amounts{100000, 0, 0, 18000, 0, 118000}},
		{"cess inclusive", 120000, 1800, 200, true, false, Amounts{100000, 9000, 9000, 0, 2000, 120000}},
		{"zero", 100, 0, 0, true, false, Amounts{100, 0, 0, 0, 0, 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := Calculate(tc.net, tc.rate, tc.cess, tc.inclusive, tc.inter)
			if e != nil || got != tc.want {
				t.Fatal(got, e)
			}
		})
	}
	for n := int64(0); n < 10000; n++ {
		a, e := Calculate(n, 1800, 100, true, false)
		if e != nil || a.Total != n || a.Base+a.CGST+a.SGST+a.Cess != n {
			t.Fatal("inclusive rounding", n, a, e)
		}
	}
	for _, rate := range []int64{-1, 10001, 3} {
		if _, e := Calculate(100, rate, 0, false, false); e == nil {
			t.Fatal("invalid rate accepted", rate)
		}
	}
}
func TestRegistrationValidation(t *testing.T) {
	if !GSTIN("27AAPFU0939F1ZV") {
		t.Fatal("valid GSTIN rejected")
	}
	for _, v := range []string{"", "27AAPFU0939F1ZA", "99AAPFU0939F1ZV", "junk"} {
		if GSTIN(v) {
			t.Fatal("invalid GSTIN accepted", v)
		}
	}
	if !PAN("AAPFU0939F") || PAN("123") {
		t.Fatal("PAN validation")
	}
}
