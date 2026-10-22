package server

import (
	"billbook/internal/printing"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func testImage(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			im.SetRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	if e := png.Encode(&out, im); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestImagesFrozenOnInvoicesAndIncludedInBackup(t *testing.T) {
	s, call := setupTrading(t)
	t.Cleanup(func() { s.DB.Close() })
	first := testImage(t, color.RGBA{R: 180, A: 255})
	second := testImage(t, color.RGBA{G: 180, A: 255})
	call("POST", "media?business_id=1&kind=logo", string(first), 200)
	call("POST", "media?business_id=1&kind=product&product_id=1", string(first), 200)
	call("POST", "media?business_id=2&kind=product&product_id=1", string(first), 400)
	if got := call("GET", "media?business_id=1&kind=product&product_id=1", "", 200); got != string(first) {
		t.Fatal("product image mismatch")
	}
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":2000,"value":10000,"date":"2026-10-15","key":"media-opening-test"}`, 200)
	bill := saveBill(t, call, draftInput("SALE", "2026-10-15", "media-sale-test-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000}))
	finish(t, call, bill, 200)
	if bill.LogoID == 0 {
		t.Fatal("logo not frozen")
	}
	call("POST", "media?business_id=1&kind=logo", string(second), 200)
	var invoice printing.Invoice
	if e := s.invoiceLogo(1, bill, &invoice); e != nil || !bytes.Equal(invoice.Logo, first) {
		t.Fatal("saved invoice logo changed", e)
	}
	if e := s.invoiceLogo(2, bill, &invoice); e == nil {
		t.Fatal("cross-business logo leak")
	}
	for _, paper := range []string{"", "58", "80"} {
		pdf := call("GET", fmt.Sprintf("document-pdf?business_id=1&id=%d&paper=%s", bill.ID, paper), "", 200)
		if !strings.Contains(pdf, "/Subtype /Image") {
			t.Fatal("logo absent from PDF", paper)
		}
	}
	var snapshot map[string]string
	if e := json.Unmarshal([]byte(call("POST", "backups", `{}`, 200)), &snapshot); e != nil {
		t.Fatal(e)
	}
	call("DELETE", "media?business_id=1&kind=logo", "", 200)
	call("GET", "media?business_id=1&kind=logo", "", 404)
	call("POST", "restore", jsonText(t, map[string]string{"name": snapshot["name"], "confirm": "RESTORE ALL BUSINESSES"}), 200)
	if got := call("GET", "media?business_id=1&kind=logo", "", 200); got != string(second) {
		t.Fatal("backup lost logo")
	}
	if e := s.invoiceLogo(1, bill, &invoice); e != nil || !bytes.Equal(invoice.Logo, first) {
		t.Fatal("backup lost historical logo", e)
	}
	if _, e := s.DB.Exec("UPDATE businesses SET logo_id=? WHERE id=2", bill.LogoID); e == nil {
		t.Fatal("cross-business pointer accepted")
	}
	assertIntegrity(t, s.DB)
}
func TestInvalidImageAndRoleWrites(t *testing.T) {
	s, call := setupTrading(t)
	call("POST", "media?business_id=1&kind=logo", "<svg></svg>", 400)
	call("POST", "media?business_id=1&kind=logo", strings.Repeat("x", 512*1024+1), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM media_assets") != 0 {
		t.Fatal("invalid images persisted")
	}
	owner := authClient(t, s)
	owner("POST", "auth/users", `{"username":"owner","password":"Owner-Password-123!","role":"ADMIN","active":1}`, 200)
	owner("POST", "auth/users", `{"username":"accountant","password":"Accountant-Password-123!","role":"ACCOUNTANT","active":1}`, 200)
	clerk := authClient(t, s)
	clerk("POST", "auth/login", `{"username":"accountant","password":"Accountant-Password-123!"}`, 200)
	png := string(testImage(t, color.RGBA{B: 180, A: 255}))
	clerk("POST", "media?business_id=1&kind=logo", png, 403)
	clerk("POST", "media?business_id=1&kind=product&product_id=1", png, 200)
	owner("POST", "auth/users", `{"username":"viewer","password":"Viewer-Password-123!","role":"VIEWER","active":1}`, 200)
	viewer := authClient(t, s)
	viewer("POST", "auth/login", `{"username":"viewer","password":"Viewer-Password-123!"}`, 200)
	viewer("DELETE", "media?business_id=1&kind=product&product_id=1", "", 403)
	viewer("GET", "media?business_id=1&kind=product&product_id=1", "", 200)
}
