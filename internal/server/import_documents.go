package server

import (
	"billbook/internal/export"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type importedDocument struct {
	Reference string   `json:"reference"`
	Bill      Bill     `json:"bill"`
	SKUs      []string `json:"-"`
}

func parseDocumentRows(rows [][]string) ([]importedDocument, error) {
	required := []string{"reference", "kind", "date", "due_date", "party_id", "warehouse_id", "sku", "quantity", "price"}
	allowed := map[string]bool{}
	for _, c := range append(required, "discount", "sale_unit", "tax_mode", "place_of_supply", "inclusive", "input_credit", "notes", "invoice_discount", "invoice_charge", "round_to_rupee", "shipping_address") {
		allowed[c] = true
	}
	if len(rows) < 2 || len(rows) > 10001 {
		return nil, errors.New("provide a header and 1–10,000 lines")
	}
	header := map[string]int{}
	for i, c := range rows[0] {
		c = strings.TrimSpace(strings.TrimPrefix(c, "\ufeff"))
		if _, ok := header[c]; ok || !allowed[c] {
			return nil, fmt.Errorf("duplicate or unknown header %q", c)
		}
		header[c] = i
	}
	for _, c := range required {
		if _, ok := header[c]; !ok {
			return nil, fmt.Errorf("missing header %s", c)
		}
	}
	result := []importedDocument{}
	indexes := map[string]int{}
	metadata := map[string]string{}
	seen := map[string]map[string]bool{}
	for i, row := range rows[1:] {
		if len(row) != len(header) {
			return nil, fmt.Errorf("row %d: wrong column count", i+2)
		}
		get := func(c string) string {
			if n, ok := header[c]; ok {
				return strings.TrimSpace(row[n])
			}
			return ""
		}
		var fieldErr error
		number := func(c string, max int64, positive bool) int64 {
			v := get(c)
			if v == "" && !positive {
				return 0
			}
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n < 0 || n > max || positive && n == 0 {
				fieldErr = fmt.Errorf("invalid %s", c)
			}
			return n
		}
		boolean := func(c string) bool {
			v := get(c)
			if v != "" && v != "true" && v != "false" {
				fieldErr = fmt.Errorf("%s must be true or false", c)
			}
			return v == "true"
		}
		ref := get("reference")
		sku := get("sku")
		bill := Bill{Kind: get("kind"), Date: get("date"), DueDate: get("due_date"), PartyID: number("party_id", 1<<53-1, true), WarehouseID: number("warehouse_id", 1<<53-1, true), TaxMode: get("tax_mode"), PlaceOfSupply: get("place_of_supply"), Inclusive: boolean("inclusive"), InputCredit: boolean("input_credit"), Notes: get("notes"), ShippingAddress: get("shipping_address"), InvoiceDiscount: number("invoice_discount", 1000000000000, false), InvoiceCharge: number("invoice_charge", 1000000000000, false), RoundToRupee: boolean("round_to_rupee")}
		line := BillLine{Quantity: number("quantity", 1000000000, true), Price: number("price", 100000000000, false), Discount: number("discount", 1000000000000, false), SaleUnit: get("sale_unit")}
		if get("price") == "" {
			fieldErr = errors.New("price is required")
		}
		if fieldErr != nil {
			return nil, fmt.Errorf("row %d: %w; quantities use integer thousandths and money uses integer paise", i+2, fieldErr)
		}
		if !importReference.MatchString(ref) || (bill.Kind != "SALE" && bill.Kind != "PURCHASE") || !dateValid(bill.Date) || !dateValid(bill.DueDate) || bill.DueDate < bill.Date || sku == "" || len(sku) > 80 || len(line.SaleUnit) > 30 || len(bill.Notes) > 1000 {
			return nil, fmt.Errorf("row %d: invalid reference, type, dates, SKU, unit or notes", i+2)
		}
		raw, _ := json.Marshal(bill)
		index, ok := indexes[ref]
		if !ok {
			index = len(result)
			indexes[ref] = index
			metadata[ref] = string(raw)
			seen[ref] = map[string]bool{}
			result = append(result, importedDocument{Reference: ref, Bill: bill})
		}
		if metadata[ref] != string(raw) {
			return nil, fmt.Errorf("row %d: invoice fields differ within reference %s", i+2, ref)
		}
		if seen[ref][sku] {
			return nil, fmt.Errorf("row %d: duplicate SKU within invoice", i+2)
		}
		seen[ref][sku] = true
		result[index].Bill.Lines = append(result[index].Bill.Lines, line)
		result[index].SKUs = append(result[index].SKUs, sku)
		if len(result) > 1000 || len(result[index].Bill.Lines) > 100 {
			return nil, errors.New("maximum 1,000 invoices and 100 lines per invoice")
		}
	}
	return result, nil
}

func (s *Server) importDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	key := r.URL.Query().Get("key")
	mode := r.URL.Query().Get("mode")
	if !validKey(key) || (mode != "draft" && mode != "finalize") {
		fail(w, errors.New("request key and draft/finalize mode required"))
		return
	}
	data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 20<<20))
	if e != nil {
		fail(w, e)
		return
	}
	rows, e := export.ReadTable(data, r.URL.Query().Get("format"))
	if e != nil {
		fail(w, e)
		return
	}
	entries, e := parseDocumentRows(rows)
	if e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	hash := sha256.Sum256(data)
	repeat, e := claim(tx, b, "document-import:"+key, mode+":"+hex.EncodeToString(hash[:]))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]interface{}{"already_imported": true, "documents": len(entries)})
		return
	}
	for i := range entries {
		entry := &entries[i]
		bill := &entry.Bill
		// Reference claims survive draft editing and block a second import under a new upload key.
		ref := "document-import-reference:" + entry.Reference
		repeated, err := claim(tx, b, ref, mode+":"+hex.EncodeToString(hash[:]))
		if err != nil || repeated {
			fail(w, fmt.Errorf("reference %s was already imported; no invoices imported", entry.Reference))
			return
		}
		for j, sku := range entry.SKUs {
			if e = tx.QueryRow("SELECT id FROM products WHERE business_id=? AND sku=? AND active=1", b, sku).Scan(&bill.Lines[j].ProductID); e != nil {
				fail(w, fmt.Errorf("reference %s: active SKU %s not found; no invoices imported", entry.Reference, sku))
				return
			}
		}
		bill.ImportReference = entry.Reference
		bill.Key = "import:" + hex.EncodeToString(hash[:])[:32] + ":" + strconv.Itoa(i)
		var warehouse int64
		if e = tx.QueryRow("SELECT id FROM warehouses WHERE business_id=? AND id=?", b, bill.WarehouseID).Scan(&warehouse); e != nil {
			fail(w, fmt.Errorf("reference %s: warehouse not found", entry.Reference))
			return
		}
		if _, e = prepareDraft(tx, b, bill, r); e != nil {
			fail(w, fmt.Errorf("reference %s: %w; no invoices imported", entry.Reference, e))
			return
		}
		res, err := tx.Exec("INSERT INTO documents(business_id,kind,party_id,warehouse_id,date,due_date,total,snapshot) VALUES(?,?,?,?,?,?,?,'{}')", b, bill.Kind, bill.PartyID, bill.WarehouseID, bill.Date, bill.DueDate, bill.Total)
		if err != nil {
			fail(w, err)
			return
		}
		bill.ID, e = res.LastInsertId()
		if e != nil {
			fail(w, e)
			return
		}
		if mode == "finalize" {
			*bill, e = finalizeDraft(tx, b, *bill, r)
			if e != nil {
				fail(w, fmt.Errorf("reference %s: %w; no invoices imported", entry.Reference, e))
				return
			}
		} else {
			snapshot, _ := json.Marshal(bill)
			if _, e = tx.Exec("UPDATE documents SET snapshot=? WHERE business_id=? AND id=?", string(snapshot), b, bill.ID); e != nil {
				fail(w, e)
				return
			}
		}
		if e = auditFor(r, tx, b, "document.imported", bill.ID); e != nil {
			fail(w, e)
			return
		}
	}
	preview := r.URL.Query().Get("preview") == "1"
	if !preview {
		if e = tx.Commit(); e != nil {
			fail(w, e)
			return
		}
	}
	respond(w, 200, map[string]interface{}{"documents": len(entries), "preview": preview, "entries": entries})
}
