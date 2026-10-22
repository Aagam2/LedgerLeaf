package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCommercialIssueAndConversion(t *testing.T) {
	s, call := setupTrading(t)
	b := draftInput("SALE", "2026-10-15", "quotation-save-key", BillLine{ProductID: 1, Quantity: 1000, Price: 10000})
	b.DocumentType = "QUOTATION"
	var q Bill
	if e := json.Unmarshal([]byte(call("POST", "commercial-documents?business_id=1", jsonText(t, b), 200)), &q); e != nil {
		t.Fatal(e)
	}
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, q.ID), 200)
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"id":%d}`, q.ID), 200)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM journals") != 0 || scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != 0 {
		t.Fatal("quote posted financials")
	}
	q.Key = "quotation-edit-key"
	call("POST", "commercial-documents?business_id=1", jsonText(t, q), 400)
	call("GET", fmt.Sprintf("commercial-pdf?business_id=1&id=%d", q.ID), "", 200)
	b.SourceID = q.ID
	b.ID = 0
	b.DocumentType = ""
	b.Key = "quotation-convert-key"
	invoice := saveBill(t, call, b)
	if scalar(t, s.DB, "SELECT converted_document_id FROM commercial_documents WHERE id=?", q.ID) != invoice.ID {
		t.Fatal("conversion link missing")
	}
	b.Key = "quotation-duplicate-key"
	call("POST", "documents?business_id=1", jsonText(t, b), 400)
	call("POST", "opening-stock?business_id=1", `{"product_id":1,"warehouse_id":1,"quantity":1000,"value":5000,"date":"2026-10-15","key":"challan-opening-stock"}`, 200)
	finish(t, call, invoice, 200)
	movements := scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements")
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"invoice_id":%d}`, invoice.ID), 200)
	call("POST", "commercial-issue?business_id=1", fmt.Sprintf(`{"invoice_id":%d}`, invoice.ID), 400)
	if scalar(t, s.DB, "SELECT COUNT(*) FROM stock_movements") != movements {
		t.Fatal("challan double posted stock")
	}
	assertIntegrity(t, s.DB)
}
