package server

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

// Quantities remain in the selected document unit. Only stock movements use
// the base unit. Factors are frozen in snapshots; pre-conversion documents
// implicitly have factor 1. Exact base quantities avoid cumulative stock drift.
func (l BillLine) stockQuantity() (int64, error) {
	factor := l.UnitFactor
	if factor == 0 {
		factor = 1000
	}
	if l.Quantity <= 0 || l.Quantity > 1000000000 || factor < 1 || factor > 1000000000 {
		return 0, errors.New("invalid unit conversion quantity")
	}
	n := l.Quantity * factor
	if n%1000 != 0 || n/1000 > 1000000000 {
		return 0, errors.New("converted quantity must fit the stock limit and be exact to 3 decimal places")
	}
	return n / 1000, nil
}
func (l BillLine) baseUnit() string {
	if l.BaseUnit != "" {
		return l.BaseUnit
	}
	return l.Unit
}
func applyUnit(tx *sql.Tx, b int64, l *BillLine) error {
	l.BaseUnit = l.Unit
	l.UnitFactor = 1000
	if l.SaleUnit != "" && !strings.EqualFold(l.SaleUnit, l.BaseUnit) {
		if e := tx.QueryRow("SELECT name,factor FROM product_units WHERE business_id=? AND product_id=? AND name=?", b, l.ProductID, l.SaleUnit).Scan(&l.Unit, &l.UnitFactor); e != nil {
			return errors.New("select a configured unit for this product")
		}
		l.SaleUnit = l.Unit
	} else {
		l.SaleUnit = ""
	}
	_, e := l.stockQuantity()
	return e
}
func (s *Server) units(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		q, limit, offset := pageArgs(r)
		s.query(w, `SELECT u.product_id,p.name AS product,p.unit AS base_unit,p.sale_price,p.purchase_price,u.name,u.factor FROM product_units u JOIN products p ON p.business_id=u.business_id AND p.id=u.product_id WHERE u.business_id=? AND (?='' OR u.product_id=?) AND (p.name LIKE ? OR p.sku LIKE ? OR u.name LIKE ?) ORDER BY p.name,u.name LIMIT ? OFFSET ?`, b, r.URL.Query().Get("product_id"), r.URL.Query().Get("product_id"), q, q, q, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		ProductID int64  `json:"product_id"`
		Name      string `json:"name"`
		Factor    int64  `json:"factor"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 30 || p.Factor < 1 || p.Factor > 1000000000 {
		fail(w, errors.New("unit name and a positive conversion factor required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRow("SELECT COUNT(*) FROM product_units WHERE business_id=? AND product_id=?", b, p.ProductID).Scan(&count); e != nil {
		fail(w, e)
		return
	}
	if count >= 50 {
		fail(w, errors.New("a product can have at most 50 alternate units"))
		return
	}
	var base string
	if e = tx.QueryRow("SELECT unit FROM products WHERE business_id=? AND id=? AND active=1", b, p.ProductID).Scan(&base); e != nil {
		fail(w, e)
		return
	}
	if strings.EqualFold(base, p.Name) {
		fail(w, errors.New("alternate unit must differ from the base unit"))
		return
	}
	if _, e = tx.Exec("INSERT INTO product_units(business_id,product_id,name,factor) VALUES(?,?,?,?)", b, p.ProductID, p.Name, p.Factor); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "unit.created", p.ProductID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
