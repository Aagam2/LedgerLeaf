package server

import (
	"strings"
	"testing"
)

func TestInvoiceImportAtomicPreviewAndPosting(t *testing.T) {
	s, call := fixture(t)
	call("POST", "products?business_id=1", `{"name":"Tea","sku":"T","kind":"PRODUCT","unit":"PCS","sale_price":10000}`, 200)
	call("POST", "parties?business_id=1", `{"name":"Supplier","kind":"SUPPLIER"}`, 200)
	call("POST", "parties?business_id=1", `{"name":"Buyer","kind":"CUSTOMER"}`, 200)
	csv := "reference,kind,date,due_date,party_id,warehouse_id,sku,quantity,price\nP1,PURCHASE,2026-10-15,2026-10-15,1,1,T,2000,6000\nS1,SALE,2026-10-15,2026-10-15,2,1,T,1000,10000\n"
	path := "import-documents?business_id=1&format=csv&mode=finalize&key=invoice-import-first"
	preview := call("POST", path+"&preview=1", csv, 200)
	if !strings.Contains(preview, `"total":10000`) {
		t.Fatal(preview)
	}
	for _, table := range []string{"documents", "stock_movements", "journals", "requests", "counters"} {
		if scalar(t, s.DB, "SELECT COUNT(*) FROM "+table) != 0 {
			t.Fatalf("preview wrote %s", table)
		}
	}
	invalid := strings.Replace(csv, "T,1000,10000", "T,3000,10000", 1)
	call("POST", path, invalid, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents") != 0 || scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != 0 {
		t.Fatal("partial posting")
	}
	call("POST", path, csv, 200)
	call("POST", path, csv, 200)
	call("POST", strings.Replace(path, "invoice-import-first", "invoice-import-again", 1), csv, 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents WHERE status='FINALIZED'") != 2 || scalar(t, s.DB, "SELECT SUM(quantity) FROM stock_movements") != 1000 {
		t.Fatal("wrong posting or duplicate")
	}
	if scalar(t, s.DB, "SELECT SUM(value) FROM stock_movements") != 6000 {
		t.Fatal("wrong stock value")
	}
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents WHERE json_extract(snapshot,'$.import_reference') IN ('P1','S1')") != 2 {
		t.Fatal("missing reference")
	}
	assertIntegrity(t, s.DB)
}

func TestInvoiceImportDraftAndIsolation(t *testing.T) {
	s, call := fixture(t)
	call("POST", "products?business_id=1", `{"name":"Tea","sku":"T","kind":"PRODUCT","unit":"PCS","sale_price":10000}`, 200)
	call("POST", "parties?business_id=1", `{"name":"Buyer","kind":"CUSTOMER"}`, 200)
	csv := "reference,kind,date,due_date,party_id,warehouse_id,sku,quantity,price\nD1,SALE,2026-10-15,2026-10-15,1,1,T,1000,10000\n"
	path := "import-documents?business_id=1&format=csv&mode=draft&key=draft-import-request"
	call("POST", strings.Replace(path, "business_id=1", "business_id=2", 1), csv, 400)
	call("POST", path, csv, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents WHERE status='DRAFT'") != 1 || scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 0 {
		t.Fatal("draft posted")
	}
	call("POST", "documents?business_id=1", `{"id":1,"kind":"SALE","date":"2026-10-15","due_date":"2026-10-15","party_id":1,"warehouse_id":1,"lines":[{"product_id":1,"quantity":1000,"price":11000}],"key":"edited-import-draft"}`, 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM documents WHERE json_extract(snapshot,'$.import_reference')='D1'") != 1 {
		t.Fatal("editing lost reference")
	}
}

func TestInvoiceImportParser(t *testing.T) {
	header := []string{"reference", "kind", "date", "due_date", "party_id", "warehouse_id", "sku", "quantity", "price"}
	row := []string{"A", "SALE", "2026-10-15", "2026-10-15", "1", "1", "T", "1000", "100"}
	if _, e := parseDocumentRows([][]string{header, row}); e != nil {
		t.Fatal(e)
	}
	for _, column := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8} {
		r := append([]string{}, row...)
		r[column] = ""
		if _, e := parseDocumentRows([][]string{header, r}); e == nil {
			t.Fatalf("accepted empty column %d", column)
		}
	}
	if _, e := parseDocumentRows([][]string{header, row, row}); e == nil {
		t.Fatal("duplicate SKU accepted")
	}
	changed := append([]string{}, row...)
	changed[6] = "T2"
	changed[3] = "2026-10-16"
	if _, e := parseDocumentRows([][]string{header, row, changed}); e == nil {
		t.Fatal("inconsistent metadata accepted")
	}
}
