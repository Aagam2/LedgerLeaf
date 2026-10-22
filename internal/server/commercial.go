package server

import (
	"billbook/internal/printing"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

func commercialType(t string) bool {
	return t == "ESTIMATE" || t == "QUOTATION" || t == "PROFORMA" || t == "PURCHASE_ORDER" || t == "DELIVERY_CHALLAN"
}
func (s *Server) commercial(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.documents(w, r)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if id := r.URL.Query().Get("id"); id != "" {
		var raw string
		if e = s.DB.QueryRow("SELECT snapshot FROM commercial_documents WHERE business_id=? AND id=?", b, id).Scan(&raw); e != nil {
			fail(w, e)
			return
		}
		var bill Bill
		if e = json.Unmarshal([]byte(raw), &bill); e != nil {
			fail(w, e)
			return
		}
		if bill.DocumentType == "PURCHASE_ORDER" {
			for i := range bill.Lines {
				if e = s.DB.QueryRow(`SELECT COALESCE(SUM(json_extract(l.value,'$.quantity')),0) FROM goods_receipts g,json_each(g.snapshot,'$.lines') l WHERE NOT EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id) AND g.business_id=? AND g.order_id=? AND json_extract(l.value,'$.product_id')=?`, b, bill.ID, bill.Lines[i].ProductID).Scan(&bill.Lines[i].ReceivedQuantity); e != nil {
					fail(w, e)
					return
				}
			}
		}
		respond(w, 200, bill)
		return
	}
	q, limit, offset := pageArgs(r)
	s.query(w, `SELECT c.id,c.document_type,c.kind,c.status,c.number,c.date,c.total,c.converted_document_id,p.name AS party FROM commercial_documents c JOIN parties p ON p.business_id=c.business_id AND p.id=c.party_id WHERE c.business_id=? AND (p.name LIKE ? OR c.number LIKE ?) ORDER BY c.id DESC LIMIT ? OFFSET ?`, b, q, q, limit, offset)
}
func saveCommercial(tx *sql.Tx, b int64, bill *Bill) error {
	if !commercialType(bill.DocumentType) || (bill.DocumentType == "PURCHASE_ORDER") != (bill.Kind == "PURCHASE") {
		return errors.New("invalid commercial document type")
	}
	if bill.DocumentType == "DELIVERY_CHALLAN" {
		return errors.New("create delivery challan from a finalized invoice")
	}
	if bill.ID == 0 {
		res, e := tx.Exec("INSERT INTO commercial_documents(business_id,document_type,kind,date,party_id,total,snapshot) VALUES(?,?,?,?,?,?,'{}')", b, bill.DocumentType, bill.Kind, bill.Date, bill.PartyID, bill.Total)
		if e != nil {
			return e
		}
		bill.ID, _ = res.LastInsertId()
	}
	raw, _ := json.Marshal(bill)
	res, e := tx.Exec("UPDATE commercial_documents SET document_type=?,kind=?,date=?,party_id=?,total=?,snapshot=? WHERE business_id=? AND id=? AND status='DRAFT'", bill.DocumentType, bill.Kind, bill.Date, bill.PartyID, bill.Total, string(raw), b, bill.ID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("only drafts can be edited")
	}
	return nil
}
func (s *Server) issueCommercial(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var p struct {
		ID        int64 `json:"id"`
		InvoiceID int64 `json:"invoice_id"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var raw, status string
	var bill Bill
	if p.InvoiceID > 0 {
		var count int
		if e = tx.QueryRow("SELECT COUNT(*) FROM commercial_documents WHERE business_id=? AND document_type='DELIVERY_CHALLAN' AND converted_document_id=?", b, p.InvoiceID).Scan(&count); e != nil {
			fail(w, e)
			return
		}
		if count > 0 {
			fail(w, errors.New("challan already exists for this invoice"))
			return
		}
		if e = tx.QueryRow("SELECT snapshot FROM documents d WHERE business_id=? AND id=? AND status='FINALIZED' AND kind='SALE' AND NOT EXISTS(SELECT 1 FROM credit_notes c WHERE c.business_id=d.business_id AND c.document_id=d.id)", b, p.InvoiceID).Scan(&raw); e != nil {
			fail(w, errors.New("select an unreturned finalized sale"))
			return
		}
		if e = json.Unmarshal([]byte(raw), &bill); e != nil {
			fail(w, e)
			return
		}
		bill.DocumentType = "DELIVERY_CHALLAN"
		bill.SourceID = p.InvoiceID
		res, e := tx.Exec("INSERT INTO commercial_documents(business_id,document_type,kind,date,party_id,total,snapshot,converted_document_id) VALUES(?,'DELIVERY_CHALLAN','SALE',?,?,?,'{}',?)", b, bill.Date, bill.PartyID, bill.Total, p.InvoiceID)
		if e != nil {
			fail(w, e)
			return
		}
		p.ID, _ = res.LastInsertId()
		bill.ID = p.ID
	} else {
		if e = tx.QueryRow("SELECT snapshot,status FROM commercial_documents WHERE business_id=? AND id=?", b, p.ID).Scan(&raw, &status); e != nil {
			fail(w, e)
			return
		}
		if status == "ISSUED" {
			respond(w, 200, map[string]bool{"ok": true})
			return
		}
		if e = json.Unmarshal([]byte(raw), &bill); e != nil {
			fail(w, e)
			return
		}
	}
	prefix := map[string]string{"ESTIMATE": "ES", "QUOTATION": "QT", "PROFORMA": "PF", "PURCHASE_ORDER": "PO", "DELIVERY_CHALLAN": "DC"}[bill.DocumentType]
	series := prefix + bill.Date[:4]
	if _, e = tx.Exec("INSERT OR IGNORE INTO counters(business_id,series,next) VALUES(?,?,1)", b, series); e != nil {
		fail(w, e)
		return
	}
	var seq int64
	if e = tx.QueryRow("SELECT next FROM counters WHERE business_id=? AND series=?", b, series).Scan(&seq); e != nil {
		fail(w, e)
		return
	}
	bill.Number = fmt.Sprintf("%s%s/%08d", prefix, bill.Date[2:4], seq)
	bill.Status = "ISSUED"
	if len(bill.Number) > 16 {
		fail(w, errors.New("sequence exhausted"))
		return
	}
	rawBytes, _ := json.Marshal(bill)
	if _, e = tx.Exec("UPDATE commercial_documents SET status='ISSUED',number=?,snapshot=? WHERE business_id=? AND id=?", bill.Number, string(rawBytes), b, p.ID); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("UPDATE counters SET next=next+1 WHERE business_id=? AND series=?", b, series); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "commercial.issued", p.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, bill)
}
func (s *Server) commercialPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var raw string
	if e = s.DB.QueryRow("SELECT snapshot FROM commercial_documents WHERE business_id=? AND id=? AND status='ISSUED'", b, r.URL.Query().Get("id")).Scan(&raw); e != nil {
		fail(w, e)
		return
	}
	var bill Bill
	if e = json.Unmarshal([]byte(raw), &bill); e != nil {
		fail(w, e)
		return
	}
	p := printing.Invoice{Title: bill.DocumentType + " - NOT A TAX INVOICE", Number: bill.Number, Business: bill.BusinessName, Address: bill.BusinessAddress + "\nGSTIN: " + bill.BusinessGSTIN, Party: bill.PartyName, PartyAddress: bill.PartyAddress + "\nDelivery: " + bill.ShippingAddress, Date: bill.Date, DueDate: bill.DueDate, Total: decimal(bill.Total, 2), Notes: bill.Notes + "\n" + bill.Terms}
	p.Adjustments = adjustmentSummary(bill)
	if e = s.invoiceLogo(b, bill, &p); e != nil {
		fail(w, e)
		return
	}
	for _, l := range bill.Lines {
		p.Lines = append(p.Lines, printing.Line{Name: l.Name, Quantity: decimal(l.Quantity, 3) + " " + l.Unit, Price: decimal(l.Price, 2), Discount: decimal(l.Discount, 2), Amount: decimal(l.Amount, 2), Tax: taxLineText(bill, l)})
	}
	if bill.DocumentType == "DELIVERY_CHALLAN" {
		p.Notes += fmt.Sprintf("\nAccompanies finalized invoice ID %d; stock was posted by that invoice.", bill.SourceID)
	}
	data, e := printing.PDF(p)
	if e != nil {
		fail(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-`+bill.Number[:2]+`.pdf"`)
	w.Write(data)
}
