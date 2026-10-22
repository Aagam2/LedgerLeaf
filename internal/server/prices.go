package server

import (
	"errors"
	"net/http"
	"strings"
)

func (s *Server) prices(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		q, limit, offset := pageArgs(r)
		s.query(w, `SELECT l.id,l.product_id,p.name AS product,p.sku,l.name,l.price FROM price_lists l JOIN products p ON p.business_id=l.business_id AND p.id=l.product_id WHERE l.business_id=? AND (l.name LIKE ? OR p.name LIKE ? OR p.sku LIKE ?) ORDER BY l.name,p.name,l.id LIMIT ? OFFSET ?`, b, q, q, q, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		ProductID int64  `json:"product_id"`
		Name      string `json:"name"`
		Price     int64  `json:"price"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 100 || p.Price < 0 || p.Price > 100000000000 {
		fail(w, errors.New("valid price list name and price required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO price_lists(business_id,product_id,name,price) VALUES(?,?,?,?) ON CONFLICT(business_id,product_id,name) DO UPDATE SET price=excluded.price", b, p.ProductID, p.Name, p.Price); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "price.saved", p.ProductID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
