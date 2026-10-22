package printing

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFContentAndPagination(t *testing.T) {
	b := Invoice{Title: "SALE", Number: "SALE/2026-27/00001", Business: "Café Traders", Party: "Customer One", Date: "2026-10-15", DueDate: "2026-10-20", Total: "12345.67", Notes: "Final invoice notes"}
	for i := 1; i <= 100; i++ {
		b.Lines = append(b.Lines, Line{Name: fmt.Sprintf("Invoice item %03d", i), Quantity: "1.000 PCS", Price: "123.45", Discount: "0.00", Amount: "123.45"})
	}
	data, e := PDF(b)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("invalid PDF envelope")
	}
	converter, e := exec.LookPath("pdftotext")
	if e != nil {
		t.Skip("PDF generated; semantic parsing needs optional pdftotext")
	}
	path := filepath.Join(t.TempDir(), "invoice.pdf")
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	text, e := exec.Command(converter, "-layout", path, "-").CombinedOutput()
	if e != nil {
		t.Fatalf("PDF parse: %v %s", e, text)
	}
	for _, want := range []string{"Café Traders", "Customer One", "Invoice item 001", "Invoice item 100", "12345.67", "Final invoice notes", "Page 2"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("missing PDF content %q", want)
		}
	}
}

func TestPDFUserTextDoesNotPanic(t *testing.T) {
	for _, value := range []string{"Tea 🍵", "नमस्ते", "<script>alert(1)</script>", strings.Repeat("Long name ", 200)} {
		t.Run(value[:minLen(len(value), 30)], func(t *testing.T) {
			data, e := PDF(Invoice{Business: value, Party: value, Notes: value, Lines: []Line{{Name: value}}, Total: "1.00"})
			if e == nil && !bytes.HasPrefix(data, []byte("%PDF-")) {
				t.Fatal("invalid PDF")
			}
		})
	}
}
func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}
