package printing

import (
	"bytes"
	"testing"
)

func TestThermalPaperAndLongReceipt(t *testing.T) {
	b := Invoice{Business: "Receipt test", Total: "118.00", TaxSummary: "CGST 9.00 SGST 9.00"}
	for i := 0; i < 100; i++ {
		b.Lines = append(b.Lines, Line{Name: "Long item name for a narrow paper receipt", Quantity: "1 PCS", Price: "1.00", Amount: "1.00"})
	}
	for _, width := range []float64{58, 80} {
		pdf, e := Thermal(b, width)
		if e != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			t.Fatal(e)
		}
	}
	if _, e := Thermal(b, 70); e == nil {
		t.Fatal("invalid paper accepted")
	}
}
