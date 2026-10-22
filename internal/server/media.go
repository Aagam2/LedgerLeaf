package server

import (
	"billbook/internal/printing"
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
)

// Image bytes live in SQLite so backups include them. Replacement only moves
// the current pointer; immutable invoices retain their original logo asset.
func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	kind := r.URL.Query().Get("kind")
	product, _ := strconv.ParseInt(r.URL.Query().Get("product_id"), 10, 64)
	if kind != "logo" && (kind != "product" || product <= 0) {
		fail(w, errors.New("select a logo or product image"))
		return
	}
	if r.Method == "GET" {
		var content []byte
		var format string
		query := "SELECT m.content,m.format FROM media_assets m JOIN businesses b ON b.logo_id=m.id AND b.id=m.business_id WHERE b.id=?"
		args := []interface{}{b}
		if kind == "product" {
			query = "SELECT m.content,m.format FROM media_assets m JOIN products p ON p.image_id=m.id AND p.business_id=m.business_id WHERE p.business_id=? AND p.id=?"
			args = append(args, product)
		}
		if e = s.DB.QueryRow(query, args...).Scan(&content, &format); e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/"+format)
		w.Write(content)
		return
	}
	if r.Method != "POST" && r.Method != "DELETE" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if kind == "logo" && actorOf(r).Role != "ADMIN" {
		http.Error(w, "administrator permission required", 403)
		return
	}
	var content []byte
	var format string
	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
		defer r.Body.Close()
		if content, e = io.ReadAll(r.Body); e != nil {
			fail(w, e)
			return
		}
		cfg, f, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || (f != "png" && f != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
			fail(w, errors.New("use a PNG or JPEG up to 512 KB and 2048 × 2048 pixels"))
			return
		}
		if _, _, e = image.Decode(bytes.NewReader(content)); e != nil {
			fail(w, errors.New("image is damaged"))
			return
		}
		format = f
		if kind == "logo" {
			if _, e = printing.PDF(printing.Invoice{Logo: content, LogoFormat: format}); e != nil {
				fail(w, errors.New("this image encoding cannot be printed; export it as a standard PNG or JPEG"))
				return
			}
		}
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var exists int
	query := "SELECT COUNT(*) FROM businesses WHERE id=?"
	args := []interface{}{b}
	if kind == "product" {
		query = "SELECT COUNT(*) FROM products WHERE business_id=? AND id=?"
		args = append(args, product)
	}
	if e = tx.QueryRow(query, args...).Scan(&exists); e != nil || exists != 1 {
		fail(w, errors.New("business or product not found"))
		return
	}
	var asset interface{}
	if r.Method == "POST" {
		res, e := tx.Exec("INSERT INTO media_assets(business_id,format,content) VALUES(?,?,?)", b, format, content)
		if e != nil {
			fail(w, e)
			return
		}
		id, e := res.LastInsertId()
		if e != nil {
			fail(w, e)
			return
		}
		asset = id
	}
	if kind == "logo" {
		_, e = tx.Exec("UPDATE businesses SET logo_id=? WHERE id=?", asset, b)
	} else {
		_, e = tx.Exec("UPDATE products SET image_id=? WHERE business_id=? AND id=?", asset, b, product)
	}
	if e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "image."+kind+".changed", product); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) invoiceLogo(b int64, bill Bill, p *printing.Invoice) error {
	if bill.LogoID == 0 {
		return nil
	}
	return s.DB.QueryRow("SELECT content,format FROM media_assets WHERE business_id=? AND id=?", b, bill.LogoID).Scan(&p.Logo, &p.LogoFormat)
}
