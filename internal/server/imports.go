package server

import (
	"billbook/internal/export"
	"billbook/internal/gst"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) importMasters(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	entity := r.URL.Query().Get("entity")
	if entity != "products" && entity != "parties" {
		fail(w, errors.New("import products or parties"))
		return
	}
	key := r.URL.Query().Get("key")
	if !validKey(key) {
		fail(w, errors.New("request key required"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	defer r.Body.Close()
	data, e := io.ReadAll(r.Body)
	if e != nil {
		fail(w, e)
		return
	}
	rows, e := export.ReadTable(data, r.URL.Query().Get("format"))
	if e != nil {
		fail(w, e)
		return
	}
	if len(rows) < 2 {
		fail(w, errors.New("header and at least one data row required"))
		return
	}
	allowed := map[string]bool{"name": true, "kind": true}
	required := []string{"name", "kind"}
	if entity == "products" {
		for _, c := range []string{"sku", "barcode", "unit", "sale_price", "minimum_stock"} {
			allowed[c] = true
		}
		required = append(required, "sku", "unit", "sale_price")
	} else {
		for _, c := range []string{"phone", "email", "address", "gstin", "state_code"} {
			allowed[c] = true
		}
	}
	header := map[string]int{}
	for i, c := range rows[0] {
		c = strings.TrimSpace(strings.TrimPrefix(c, "\ufeff"))
		if _, ok := header[c]; ok || !allowed[c] {
			fail(w, fmt.Errorf("duplicate or unknown header %q", c))
			return
		}
		header[c] = i
	}
	for _, c := range required {
		if _, ok := header[c]; !ok {
			fail(w, fmt.Errorf("required header %s missing", c))
			return
		}
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	hash := sha256.Sum256(data)
	repeat, e := claim(tx, b, "import:"+key, entity+":"+hex.EncodeToString(hash[:]))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]interface{}{"already_imported": true, "rows": len(rows) - 1})
		return
	}
	for index, row := range rows[1:] {
		get := func(name string) string {
			i, ok := header[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		name, kind := get("name"), get("kind")
		var rowErr error
		if len(row) > len(header) || name == "" || len(name) > 160 {
			rowErr = errors.New("invalid name or column count")
		} else if entity == "products" {
			price, err := strconv.ParseInt(get("sale_price"), 10, 64)
			minimum := int64(0)
			if get("minimum_stock") != "" {
				minimum, rowErr = strconv.ParseInt(get("minimum_stock"), 10, 64)
			}
			if err != nil || rowErr != nil || price < 0 || price > 100000000000 || minimum < 0 || minimum > 1000000000 || get("sku") == "" || len(get("sku")) > 80 || len(get("barcode")) > 80 || get("unit") == "" || len(get("unit")) > 20 || (kind != "PRODUCT" && kind != "SERVICE") {
				rowErr = errors.New("invalid product fields; prices are integer paise and quantities integer thousandths")
			} else {
				_, rowErr = tx.Exec("INSERT INTO products(business_id,name,kind,sku,barcode,unit,sale_price,minimum_stock) VALUES(?,?,?,?,?,?,?,?)", b, name, kind, get("sku"), get("barcode"), get("unit"), price, minimum)
			}
		} else {
			gstin := strings.ToUpper(get("gstin"))
			state := get("state_code")
			if kind != "CUSTOMER" && kind != "SUPPLIER" || len(get("phone")) > 40 || len(get("email")) > 254 || len(get("address")) > 1000 || state != "" && !gst.State(state) || gstin != "" && (!gst.GSTIN(gstin) || gstin[:2] != state) {
				rowErr = errors.New("invalid party details, GSTIN or state")
			} else {
				_, rowErr = tx.Exec("INSERT INTO parties(business_id,name,kind,phone,email,address,gstin,state_code) VALUES(?,?,?,?,?,?,?,?)", b, name, kind, get("phone"), get("email"), get("address"), gstin, state)
			}
		}
		if rowErr != nil {
			fail(w, fmt.Errorf("row %d: %w; no rows were imported", index+2, rowErr))
			return
		}
	}
	if e = auditFor(r, tx, b, "import."+entity, int64(len(rows)-1)); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int{"rows": len(rows) - 1})
}
