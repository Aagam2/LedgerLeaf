package server

import (
	"billbook/internal/accounting"
	"billbook/internal/gst"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func businessID(r *http.Request) (int64, error) {
	b, e := strconv.ParseInt(r.URL.Query().Get("business_id"), 10, 64)
	if e != nil || b <= 0 {
		return 0, errors.New("select a business")
	}
	return b, nil
}
func pageArgs(r *http.Request) (string, int, int) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 160 {
		q = q[:160]
	}
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		p = 1
	}
	if p > 1000000 {
		p = 1000000
	}
	return "%" + q + "%", 50, (p - 1) * 50
}
func (s *Server) query(w http.ResponseWriter, query string, args ...interface{}) {
	rows, e := s.DB.Query(query, args...)
	if e != nil {
		fail(w, e)
		return
	}
	defer rows.Close()
	cols, e := rows.Columns()
	if e != nil {
		fail(w, e)
		return
	}
	out := []map[string]interface{}{}
	for rows.Next() {
		v := make([]interface{}, len(cols))
		ptr := make([]interface{}, len(cols))
		for i := range v {
			ptr[i] = &v[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			fail(w, e)
			return
		}
		obj := map[string]interface{}{}
		for i, c := range cols {
			if b, ok := v[i].([]byte); ok {
				obj[c] = string(b)
			} else if n, ok := v[i].(int64); ok && (n > 9007199254740991 || n < -9007199254740991) {
				obj[c] = strconv.FormatInt(n, 10)
			} else {
				obj[c] = v[i]
			}
		}
		out = append(out, obj)
	}
	if e = rows.Err(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, out)
}
func audit(tx *sql.Tx, b int64, action string, id int64) error {
	_, e := tx.Exec("INSERT INTO audit_log(business_id,action,entity_id) VALUES(?,?,?)", b, action, id)
	return e
}
func dateValid(s string) bool { _, e := time.Parse("2006-01-02", s); return e == nil }
func (s *Server) masters(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	q, limit, offset := pageArgs(r)
	entity := strings.TrimPrefix(r.URL.Path, "/api/")
	if r.Method == "GET" {
		if rawID := r.URL.Query().Get("id"); rawID != "" {
			fields := map[string]string{
				"parties":    "id,kind,name,active,credit_days",
				"products":   "id,name,sku,barcode,unit,kind,sale_price,purchase_price,active",
				"warehouses": "id,name",
			}
			if columns, ok := fields[entity]; ok {
				id, err := strconv.ParseInt(rawID, 10, 64)
				if err != nil || id <= 0 {
					fail(w, errors.New("valid record ID required"))
					return
				}
				s.query(w, "SELECT "+columns+" FROM "+entity+" WHERE business_id=? AND id=?", b, id)
				return
			}
		}
		switch entity {
		case "parties":
			kind := r.URL.Query().Get("kind")
			if kind != "" && kind != "CUSTOMER" && kind != "SUPPLIER" {
				fail(w, errors.New("invalid party kind"))
				return
			}
			s.query(w, "SELECT id,kind,name,phone,email,address,active,gstin,state_code,credit_limit,credit_days FROM parties WHERE business_id=? AND (name LIKE ? OR phone LIKE ?) AND (?='' OR kind=?) AND (?=0 OR active=1) ORDER BY name,id LIMIT ? OFFSET ?", b, q, q, kind, kind, r.URL.Query().Get("active") == "1", limit, offset)
		case "products":
			kind := r.URL.Query().Get("kind")
			if kind != "" && kind != "PRODUCT" && kind != "SERVICE" {
				fail(w, errors.New("invalid product kind"))
				return
			}
			if r.URL.Query().Get("exact") == "1" {
				code := strings.TrimSpace(r.URL.Query().Get("q"))
				if code == "" || len(code) > 160 {
					fail(w, errors.New("barcode or SKU required, up to 160 characters"))
					return
				}
				s.query(w, "SELECT id,name,sku,barcode,unit,kind,sale_price,active FROM products WHERE business_id=? AND active=1 AND (sku=? OR barcode=?) ORDER BY id LIMIT 2", b, code, code)
				return
			}
			s.query(w, "SELECT id,name,sku,barcode,unit,kind,sale_price,minimum_stock,active,category,brand,variant,purchase_price,mrp,COALESCE(image_id,0) AS image_id FROM products WHERE business_id=? AND (name LIKE ? OR sku LIKE ? OR barcode LIKE ?) AND (?=0 OR active=1) AND (?='' OR kind=?) ORDER BY name,id LIMIT ? OFFSET ?", b, q, q, q, r.URL.Query().Get("active") == "1", kind, kind, limit, offset)
		case "warehouses":
			if r.URL.Query().Get("paged") == "1" {
				s.query(w, "SELECT id,name FROM warehouses WHERE business_id=? AND name LIKE ? ORDER BY id LIMIT ? OFFSET ?", b, q, limit, offset)
				return
			}
			s.query(w, "SELECT id,name FROM warehouses WHERE business_id=? ORDER BY id LIMIT 200", b)
		case "stock":
			s.query(w, `SELECT p.id,p.name,p.sku,w.name AS warehouse,w.id AS warehouse_id,COALESCE(SUM(m.quantity),0) AS quantity,COALESCE(SUM(m.value),0) AS value,p.minimum_stock FROM products p JOIN warehouses w ON w.business_id=p.business_id LEFT JOIN stock_movements m ON m.business_id=p.business_id AND m.product_id=p.id AND m.warehouse_id=w.id WHERE p.business_id=? AND p.kind='PRODUCT' AND (p.name LIKE ? OR p.sku LIKE ?) GROUP BY p.id,w.id ORDER BY p.name,p.id,w.id LIMIT ? OFFSET ?`, b, q, q, limit, offset)
		case "movements":
			s.query(w, `SELECT m.id,p.name,w.name AS warehouse,m.quantity,m.value,m.kind,m.date,m.remarks FROM stock_movements m JOIN products p ON p.id=m.product_id AND p.business_id=m.business_id JOIN warehouses w ON w.id=m.warehouse_id AND w.business_id=m.business_id WHERE m.business_id=? ORDER BY m.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		case "trial-balance":
			s.query(w, `SELECT a.code,a.name,a.kind,COALESCE(SUM(l.debit),0) AS debit,COALESCE(SUM(l.credit),0) AS credit FROM accounts a LEFT JOIN journal_lines l ON l.business_id=a.business_id AND l.account=a.code WHERE a.business_id=? GROUP BY a.code ORDER BY a.code`, b)
		case "journals":
			s.query(w, `SELECT j.id,j.date,j.reference,j.description,l.account,l.debit,l.credit FROM journals j JOIN journal_lines l ON l.journal_id=j.id AND l.business_id=j.business_id WHERE j.business_id=? ORDER BY j.id DESC,l.id LIMIT ? OFFSET ?`, b, limit, offset)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var id int64
	var result sql.Result
	switch entity {
	case "parties":
		var p struct {
			CreditLimit int64  `json:"credit_limit"`
			CreditDays  int    `json:"credit_days"`
			GSTIN       string `json:"gstin"`
			StateCode   string `json:"state_code"`
			ID          int64  `json:"id"`
			Kind        string `json:"kind"`
			Name        string `json:"name"`
			Phone       string `json:"phone"`
			Email       string `json:"email"`
			Address     string `json:"address"`
			Active      int    `json:"active"`
		}
		if e = decode(w, r, &p); e != nil {
			break
		}
		p.GSTIN = strings.ToUpper(strings.TrimSpace(p.GSTIN))
		if p.GSTIN != "" && !gst.GSTIN(p.GSTIN) || p.StateCode != "" && !gst.State(p.StateCode) || p.GSTIN != "" && p.StateCode != p.GSTIN[:2] {
			e = errors.New("invalid GSTIN or state code")
			break
		}
		p.Name = strings.TrimSpace(p.Name)
		if p.CreditLimit < 0 || p.CreditLimit > 1000000000000 || p.CreditDays < 0 || p.CreditDays > 3650 || p.Name == "" || len(p.Name) > 160 || (p.Kind != "CUSTOMER" && p.Kind != "SUPPLIER") || len(p.Address) > 1000 || len(p.Phone) > 40 || len(p.Email) > 254 {
			e = errors.New("invalid party details")
			break
		}
		if p.ID == 0 {
			result, e = tx.Exec("INSERT INTO parties(business_id,kind,name,phone,email,address,gstin,state_code,credit_limit,credit_days) VALUES(?,?,?,?,?,?,?,?,?,?)", b, p.Kind, p.Name, p.Phone, p.Email, p.Address, p.GSTIN, p.StateCode, p.CreditLimit, p.CreditDays)
		} else {
			id = p.ID
			result, e = tx.Exec("UPDATE parties SET name=?,phone=?,email=?,address=?,gstin=?,state_code=?,credit_limit=?,credit_days=?,active=? WHERE business_id=? AND id=? AND kind=?", p.Name, p.Phone, p.Email, p.Address, p.GSTIN, p.StateCode, p.CreditLimit, p.CreditDays, p.Active, b, p.ID, p.Kind)
		}
	case "products":
		var p struct {
			Category      string `json:"category"`
			Brand         string `json:"brand"`
			Variant       string `json:"variant"`
			PurchasePrice int64  `json:"purchase_price"`
			MRP           int64  `json:"mrp"`
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			SKU           string `json:"sku"`
			Barcode       string `json:"barcode"`
			Unit          string `json:"unit"`
			Kind          string `json:"kind"`
			SalePrice     int64  `json:"sale_price"`
			MinimumStock  int64  `json:"minimum_stock"`
			Active        int    `json:"active"`
		}
		if e = decode(w, r, &p); e != nil {
			break
		}
		p.Name = strings.TrimSpace(p.Name)
		p.SKU = strings.TrimSpace(p.SKU)
		if len(p.Category) > 100 || len(p.Brand) > 100 || len(p.Variant) > 100 || p.PurchasePrice < 0 || p.PurchasePrice > 100000000000 || p.MRP < 0 || p.MRP > 100000000000 || p.Name == "" || len(p.Name) > 160 || p.SKU == "" || len(p.SKU) > 80 || len(p.Barcode) > 80 || p.Unit == "" || len(p.Unit) > 20 || (p.Kind != "PRODUCT" && p.Kind != "SERVICE") || p.SalePrice < 0 || p.SalePrice > 100000000000 || p.MinimumStock < 0 || p.MinimumStock > 1000000000 {
			e = errors.New("invalid product details")
			break
		}
		if p.ID == 0 {
			result, e = tx.Exec("INSERT INTO products(business_id,name,sku,barcode,unit,kind,sale_price,minimum_stock,category,brand,variant,purchase_price,mrp) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", b, p.Name, p.SKU, p.Barcode, p.Unit, p.Kind, p.SalePrice, p.MinimumStock, p.Category, p.Brand, p.Variant, p.PurchasePrice, p.MRP)
		} else {
			id = p.ID
			var priorUnit string
			var movementCount int
			if e = tx.QueryRow("SELECT unit FROM products WHERE business_id=? AND id=?", b, p.ID).Scan(&priorUnit); e != nil {
				break
			}
			if e = tx.QueryRow("SELECT COUNT(*) FROM stock_movements WHERE business_id=? AND product_id=?", b, p.ID).Scan(&movementCount); e != nil {
				break
			}
			var unitCount int
			if e = tx.QueryRow("SELECT COUNT(*) FROM product_units WHERE business_id=? AND product_id=?", b, p.ID).Scan(&unitCount); e != nil {
				break
			}
			if (movementCount > 0 || unitCount > 0) && priorUnit != p.Unit {
				e = errors.New("unit cannot change after stock movements or unit conversions")
				break
			}
			result, e = tx.Exec("UPDATE products SET name=?,sku=?,barcode=?,unit=?,sale_price=?,minimum_stock=?,category=?,brand=?,variant=?,purchase_price=?,mrp=?,active=? WHERE business_id=? AND id=? AND kind=?", p.Name, p.SKU, p.Barcode, p.Unit, p.SalePrice, p.MinimumStock, p.Category, p.Brand, p.Variant, p.PurchasePrice, p.MRP, p.Active, b, p.ID, p.Kind)
		}
	case "warehouses":
		var p struct {
			Name string `json:"name"`
		}
		if e = decode(w, r, &p); e != nil {
			break
		}
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" || len(p.Name) > 160 {
			e = errors.New("warehouse name required")
			break
		}
		result, e = tx.Exec("INSERT INTO warehouses(business_id,name) VALUES(?,?)", b, p.Name)
	default:
		e = errors.New("this operation is unavailable")
	}
	if e != nil {
		fail(w, e)
		return
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		fail(w, errors.New("record not found or not editable"))
		return
	}
	if id == 0 {
		id, e = result.LastInsertId()
		if e != nil {
			fail(w, e)
			return
		}
	}
	if e = auditFor(r, tx, b, entity+".saved", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}
func (s *Server) openingStock(w http.ResponseWriter, r *http.Request) {
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
		ProductID   int64  `json:"product_id"`
		WarehouseID int64  `json:"warehouse_id"`
		Quantity    int64  `json:"quantity"`
		Value       int64  `json:"value"`
		Date        string `json:"date"`
		Remarks     string `json:"remarks"`
		Key         string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if p.Quantity <= 0 || p.Quantity > 1000000000 || p.Value < 0 || p.Value > 1000000000000 || !dateValid(p.Date) || len(p.Key) < 16 || len(p.Key) > 80 || len(p.Remarks) > 1000 {
		fail(w, errors.New("invalid opening stock; use positive quantity, nonnegative total value and a valid date"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var kind string
	if e = tx.QueryRow("SELECT kind FROM products WHERE business_id=? AND id=? AND active=1", b, p.ProductID).Scan(&kind); e != nil || kind != "PRODUCT" {
		fail(w, errors.New("select an active stock product in this business"))
		return
	}
	ref := "opening:" + p.Key
	payload := fmt.Sprintf("%d/%d/%d/%d/%s/%s", p.ProductID, p.WarehouseID, p.Quantity, p.Value, p.Date, p.Remarks)
	var old string
	e = tx.QueryRow("SELECT payload FROM requests WHERE business_id=? AND key=?", b, ref).Scan(&old)
	if e == nil {
		if old != payload {
			fail(w, errors.New("idempotency key reused with different details"))
			return
		}
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if e != sql.ErrNoRows {
		fail(w, e)
		return
	}
	var count int
	if e = tx.QueryRow("SELECT COUNT(*) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, p.ProductID, p.WarehouseID).Scan(&count); e != nil {
		fail(w, e)
		return
	}
	if count > 0 {
		fail(w, errors.New("opening stock is only allowed before other stock movements"))
		return
	}
	if _, e = tx.Exec("INSERT INTO requests(business_id,key,payload) VALUES(?,?,?)", b, ref, payload); e != nil {
		fail(w, e)
		return
	}
	res, e := tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date,remarks) VALUES(?,?,?,?,?,'OPENING',?,?,?)", b, p.ProductID, p.WarehouseID, p.Quantity, p.Value, ref, p.Date, p.Remarks)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := res.LastInsertId()
	if e = accounting.Seed(tx, b); e != nil {
		fail(w, e)
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Opening inventory", []accounting.Line{{Account: "1200", Debit: p.Value}, {Account: "3000", Credit: p.Value}}); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "stock.opened", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func auditFor(r *http.Request, tx *sql.Tx, b int64, action string, id int64) error {
	_, e := tx.Exec("INSERT INTO audit_log(business_id,action,entity_id,actor_id) VALUES(?,?,?,?)", b, action, id, nullableActor(actorOf(r).ID))
	return e
}
