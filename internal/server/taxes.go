package server

import (
	"billbook/internal/gst"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
)

var hsnPattern = regexp.MustCompile(`^[0-9]{4}([0-9]{2}){0,2}$`)

func (s *Server) taxes(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		q, limit, offset := pageArgs(r)
		s.query(w, `SELECT t.*,p.name,p.sku FROM product_taxes t JOIN products p ON p.business_id=t.business_id AND p.id=t.product_id WHERE t.business_id=? AND (p.name LIKE ? OR p.sku LIKE ?) ORDER BY t.effective_date DESC,t.id DESC LIMIT ? OFFSET ?`, b, q, q, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		ProductID   int64  `json:"product_id"`
		Date        string `json:"effective_date"`
		HSN         string `json:"hsn"`
		Treatment   string `json:"treatment"`
		Rate        int64  `json:"rate"`
		Cess        int64  `json:"cess"`
		CessPerUnit int64  `json:"cess_per_unit"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if p.CessPerUnit < 0 || p.CessPerUnit > 100000000000 || !dateValid(p.Date) || !hsnPattern.MatchString(p.HSN) || (p.Treatment != "TAXABLE" && p.Treatment != "NIL" && p.Treatment != "EXEMPT" && p.Treatment != "NON_GST") {
		fail(w, errors.New("valid effective date, HSN/SAC and tax treatment required"))
		return
	}
	if _, e = gst.Calculate(0, p.Rate, p.Cess, false, false); e != nil {
		fail(w, e)
		return
	}
	if p.Treatment != "TAXABLE" && (p.Rate != 0 || p.Cess != 0 || p.CessPerUnit != 0) {
		fail(w, errors.New("non-taxable treatments require zero rates"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	// A backdated rate could change already-finalized tax classification. Require a
	// later effective date after any finalized document containing this product.
	var latest sql.NullString
	if e = tx.QueryRow(`SELECT MAX(d.date) FROM documents d,json_each(d.snapshot,'$.lines') l WHERE d.business_id=? AND d.status='FINALIZED' AND json_extract(l.value,'$.product_id')=?`, b, p.ProductID).Scan(&latest); e != nil {
		fail(w, e)
		return
	}
	if latest.Valid && p.Date <= latest.String {
		fail(w, errors.New("tax rate effective date must be after existing finalized invoices for this item"))
		return
	}
	result, e := tx.Exec(`INSERT INTO product_taxes(business_id,product_id,effective_date,hsn,treatment,rate,cess,cess_per_unit) VALUES(?,?,?,?,?,?,?,?)`, b, p.ProductID, p.Date, p.HSN, p.Treatment, p.Rate, p.Cess, p.CessPerUnit)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := result.LastInsertId()
	if e = auditFor(r, tx, b, "tax.configured", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, map[string]int64{"id": id})
}
