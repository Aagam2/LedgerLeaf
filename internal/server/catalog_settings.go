package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func financialYear(tx *sql.Tx, b int64, date string) (string, error) {
	var month int
	if e := tx.QueryRow("SELECT COALESCE((SELECT year_start_month FROM business_settings WHERE business_id=?),4)", b).Scan(&month); e != nil {
		return "", e
	}
	d, e := time.Parse("2006-01-02", date)
	if e != nil {
		return "", e
	}
	year := d.Year()
	if int(d.Month()) < month {
		year--
	}
	if month == 1 {
		return fmt.Sprint(year), nil
	}
	return fmt.Sprintf("%d-%02d", year, (year+1)%100), nil
}
func (s *Server) businessSettings(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		var month int
		e = s.DB.QueryRow("SELECT COALESCE((SELECT year_start_month FROM business_settings WHERE business_id=?),4)", b).Scan(&month)
		if e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]int{"year_start_month": month})
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if actorOf(r).Role != "ADMIN" {
		http.Error(w, "administrator permission required", 403)
		return
	}
	var p struct {
		Month int `json:"year_start_month"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if p.Month < 1 || p.Month > 12 {
		fail(w, errors.New("choose a month from 1 to 12"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var old, count int
	if e = tx.QueryRow("SELECT COALESCE((SELECT year_start_month FROM business_settings WHERE business_id=?),4)", b).Scan(&old); e != nil {
		fail(w, e)
		return
	}
	if e = tx.QueryRow("SELECT (SELECT COUNT(*) FROM counters WHERE business_id=?)+(SELECT COUNT(*) FROM journals WHERE business_id=?)+(SELECT COUNT(*) FROM year_closings WHERE business_id=?)", b, b, b).Scan(&count); e != nil {
		fail(w, e)
		return
	}
	if old != p.Month && count > 0 {
		fail(w, errors.New("financial-year start cannot change after document numbering or journal posting"))
		return
	}
	if _, e = tx.Exec("INSERT INTO business_settings(business_id,year_start_month) VALUES(?,?) ON CONFLICT(business_id) DO UPDATE SET year_start_month=excluded.year_start_month", b, p.Month); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "settings.year", b); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) variants(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		if r.URL.Query().Get("groups") == "1" {
			q, limit, offset := pageArgs(r)
			if id := r.URL.Query().Get("id"); id != "" {
				s.query(w, "SELECT id,name FROM product_groups WHERE business_id=? AND id=?", b, id)
				return
			}
			if r.URL.Query().Get("paged") != "1" {
				limit = 500
				offset = 0
			}
			s.query(w, "SELECT id,name FROM product_groups WHERE business_id=? AND name LIKE ? ORDER BY name,id LIMIT ? OFFSET ?", b, q, limit, offset)
			return
		}
		_, limit, offset := pageArgs(r)
		s.query(w, "SELECT v.product_id,p.name,p.sku,v.group_id,g.name AS group_name,v.attributes FROM product_variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id JOIN product_groups g ON g.business_id=v.business_id AND g.id=v.group_id WHERE v.business_id=? ORDER BY g.name,p.name LIMIT ? OFFSET ?", b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Name       string            `json:"name"`
		ProductID  int64             `json:"product_id"`
		GroupID    int64             `json:"group_id"`
		Attributes map[string]string `json:"attributes"`
		Remove     bool              `json:"remove"`
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
	if p.ProductID == 0 {
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" || len(p.Name) > 160 {
			fail(w, errors.New("group name required, up to 160 characters"))
			return
		}
		if _, e = tx.Exec("INSERT INTO product_groups(business_id,name) VALUES(?,?)", b, p.Name); e != nil {
			fail(w, e)
			return
		}
	} else {
		var found int
		if e = tx.QueryRow("SELECT COUNT(*) FROM products WHERE business_id=? AND id=?", b, p.ProductID).Scan(&found); e != nil || found != 1 {
			fail(w, errors.New("product not found in this business"))
			return
		}
		if p.Remove {
			_, e = tx.Exec("DELETE FROM product_variants WHERE business_id=? AND product_id=?", b, p.ProductID)
		} else {
			if len(p.Attributes) < 1 || len(p.Attributes) > 10 {
				fail(w, errors.New("provide 1–10 variant attributes"))
				return
			}
			clean := map[string]string{}
			for k, v := range p.Attributes {
				k = strings.ToLower(strings.TrimSpace(k))
				v = strings.ToLower(strings.TrimSpace(v))
				if k == "" || v == "" || len(k) > 40 || len(v) > 80 || clean[k] != "" {
					fail(w, errors.New("invalid or duplicate variant attribute"))
					return
				}
				clean[k] = v
			}
			raw, _ := json.Marshal(clean)
			_, e = tx.Exec("INSERT INTO product_variants(business_id,product_id,group_id,attributes) VALUES(?,?,?,?) ON CONFLICT(business_id,product_id) DO UPDATE SET group_id=excluded.group_id,attributes=excluded.attributes", b, p.ProductID, p.GroupID, string(raw))
		}
		if e != nil {
			fail(w, e)
			return
		}
	}
	if e = auditFor(r, tx, b, "catalog.variants", p.ProductID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
