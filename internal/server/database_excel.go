package server

import (
	"billbook/internal/export"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"
)

// databaseExcel uses one read transaction, so all sheets describe the same snapshot.
// Large/binary values are spooled separately rather than truncated by Excel.
func (s *Server) databaseExcel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback()
	tables, err := tx.QueryContext(r.Context(), `SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name`)
	if err != nil {
		fail(w, err)
		return
	}
	var names []string
	for tables.Next() {
		var name string
		if err = tables.Scan(&name); err != nil {
			break
		}
		names = append(names, name)
	}
	if err == nil {
		err = tables.Err()
	}
	tables.Close()
	if err != nil {
		fail(w, err)
		return
	}
	overflow, err := os.CreateTemp("", "ledgerleaf-excel-values-*")
	if err != nil {
		fail(w, err)
		return
	}
	defer os.Remove(overflow.Name())
	defer overflow.Close()
	enc := json.NewEncoder(overflow)
	info := [][]interface{}{
		{"Scope", "All database tables and all businesses, including archived records, settings, audit history and user password hashes. Treat this unencrypted workbook as sensitive."},
		{"Restore", "For inspection only. Use a SQLite backup to restore the application. Import screens accept their own predefined templates, not this workbook."},
		{"Units", "Raw stored values: money is integer paise; quantities are integer thousandths. IDs link records across tables."},
		{"Values", "NULL is a blank cell. Binary, long or XML-unsafe values use @base64: references. Join matching chunks from _LargeValues in part order, then base64-decode. Row numbers include the header."},
	}
	sheets := []export.Sheet{{Name: "_Guide", Columns: []string{"Item", "Description"}, Next: func() ([]interface{}, error) {
		if len(info) == 0 {
			return nil, io.EOF
		}
		row := info[0]
		info = info[1:]
		return row, nil
	}}}
	var active *sql.Rows
	defer func() {
		if active != nil {
			active.Close()
		}
	}()
	for index, name := range names {
		quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		rows, e := tx.QueryContext(r.Context(), "SELECT * FROM "+quoted+" LIMIT 0")
		if e != nil {
			fail(w, e)
			return
		}
		cols, e := rows.Columns()
		rows.Close()
		if e != nil {
			fail(w, e)
			return
		}
		// Prefix ensures uniqueness even if a future table name needs truncation.
		sheetName := fmt.Sprintf("T%d_%s", index+1, name)
		if len(sheetName) > 31 {
			sheetName = sheetName[:31]
		}
		info = append(info, []interface{}{sheetName, name})
		rowNumber := 1
		started := false
		sheets = append(sheets, export.Sheet{Name: sheetName, Columns: cols, Next: func() ([]interface{}, error) {
			if !started {
				var e error
				active, e = tx.QueryContext(r.Context(), "SELECT * FROM "+quoted)
				if e != nil {
					return nil, e
				}
				started = true
			}
			if !active.Next() {
				e := active.Err()
				active.Close()
				active = nil
				if e != nil {
					return nil, e
				}
				return nil, io.EOF
			}
			rowNumber++
			values := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if e := active.Scan(ptrs...); e != nil {
				return nil, e
			}
			for i, v := range values {
				var raw []byte
				encode := false
				switch v := v.(type) {
				case []byte:
					raw = v
					encode = true
				case string:
					raw = []byte(v)
					encode = len(raw) > 30000 || strings.Contains(v, "_x") || strings.HasPrefix(v, "@base64:") || strings.IndexFunc(v, func(c rune) bool {
						return (unicode.IsControl(c) && c != '\n' && c != '\r' && c != '\t') || c == 0xfffe || c == 0xffff
					}) >= 0
				}
				if encode {
					ref := fmt.Sprintf("%s!%d:%s", sheetName, rowNumber, cols[i])
					values[i] = "@base64:" + ref
					data := base64.StdEncoding.EncodeToString(raw)
					for part := 1; len(data) > 0 || part == 1; part++ {
						n := len(data)
						if n > 30000 {
							n = 30000
						}
						if e := enc.Encode([]interface{}{ref, part, data[:n]}); e != nil {
							return nil, e
						}
						data = data[n:]
						if len(data) == 0 {
							break
						}
					}
				}
			}
			return values, nil
		}})
	}
	var dec *json.Decoder
	sheets = append(sheets, export.Sheet{Name: "_LargeValues", Columns: []string{"Reference", "Part", "Base64"}, Next: func() ([]interface{}, error) {
		if dec == nil {
			if _, e := overflow.Seek(0, 0); e != nil {
				return nil, e
			}
			dec = json.NewDecoder(overflow)
		}
		var row []interface{}
		e := dec.Decode(&row)
		return row, e
	}})
	out, err := os.CreateTemp("", "ledgerleaf-database-*.xlsx")
	if err != nil {
		fail(w, err)
		return
	}
	defer os.Remove(out.Name())
	defer out.Close()
	if err = export.Workbook(out, sheets); err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, err)
		return
	}
	if _, err = out.Seek(0, 0); err != nil {
		fail(w, err)
		return
	}
	stat, err := out.Stat()
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-entire-database.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "Ledgerleaf-entire-database.xlsx", stat.ModTime(), out)
}

func (s *Server) importTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	entity := r.URL.Query().Get("entity")
	var cols []string
	switch entity {
	case "products":
		cols = []string{"name", "kind", "sku", "unit", "sale_price", "barcode", "minimum_stock"}
	case "parties":
		cols = []string{"name", "kind", "phone", "email", "address", "gstin", "state_code"}
	default:
		http.Error(w, "choose products or parties", 400)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-`+entity+`-template.xlsx"`)
	export.Spreadsheet(w, cols, func() ([]interface{}, error) { return nil, io.EOF })
}
