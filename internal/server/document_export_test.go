package server

import (
	"billbook/internal/printing"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestImageExportData(t *testing.T) {
	// Pure response formatting: no server listener or database is opened.
	document := printing.Invoice{Title: "CREDIT NOTE / RETURN - SALE", Number: "CN/001", Business: "Café Traders", Party: "Customer", TaxSummary: "GST 18.00", Adjustments: "Rounding -0.01", Notes: "Frozen notes and bank details", Total: "117.99", Logo: []byte{1, 2, 3}, LogoFormat: "png", Lines: []printing.Line{{Name: "Saved item", Quantity: "1 PCS", Price: "100.00", Discount: "0.00", Amount: "100.00", Tax: "CGST 9.00 / SGST 9.00"}}}
	for _, path := range []string{"/api/document-pdf?id=1", "/api/credit-note-pdf?id=1", "/api/commercial-pdf?id=1", "/api/dispatches?id=1&pdf=1"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			if imageExportData(response, httptest.NewRequest("GET", path, nil), document) || response.Body.Len() != 0 {
				t.Fatal("default PDF path was intercepted")
			}
			response = httptest.NewRecorder()
			if !imageExportData(response, httptest.NewRequest("GET", path+"&format=image-data", nil), document) {
				t.Fatal("image request not handled")
			}
			if response.Code != 200 || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Header().Get("Content-Type"))
			}
			var actual printing.Invoice
			if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, document) {
				t.Fatal("image data differs from the PDF print model")
			}
		})
	}
}
