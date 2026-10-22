package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"billbook/internal/printing"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func (s *Server) dispatches(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		if id := r.URL.Query().Get("id"); id != "" {
			if r.URL.Query().Get("remaining") == "1" {
				n, err := strconv.ParseInt(id, 10, 64)
				if err != nil {
					fail(w, err)
					return
				}
				tx, err := s.DB.Begin()
				if err != nil {
					fail(w, err)
					return
				}
				defer tx.Rollback()
				_, balances, err := dispatchBalances(tx, b, n)
				if err != nil {
					fail(w, err)
					return
				}
				respond(w, 200, balances)
				return
			}
			if r.URL.Query().Get("history") == "1" {
				_, limit, offset := pageArgs(r)
				s.query(w, "SELECT reference,date,document_id,product_id,quantity,value FROM dispatch_allocations WHERE business_id=? AND dispatch_id=? ORDER BY date,reference,product_id LIMIT ? OFFSET ?", b, id, limit, offset)
				return
			}

			var raw string
			if e = s.DB.QueryRow("SELECT snapshot FROM dispatches WHERE business_id=? AND id=?", b, id).Scan(&raw); e != nil {
				fail(w, e)
				return
			}
			if r.URL.Query().Get("pdf") == "1" {
				var d Bill
				if e = json.Unmarshal([]byte(raw), &d); e != nil {
					fail(w, e)
					return
				}
				p := printing.Invoice{Title: "DELIVERY CHALLAN - NOT A TAX INVOICE", Number: d.Number, Business: d.BusinessName, Address: d.BusinessAddress, Party: d.PartyName, PartyAddress: d.PartyAddress, Date: d.Date, Notes: d.Notes, Total: "Not an invoice"}
				for _, l := range d.Lines {
					p.Lines = append(p.Lines, printing.Line{Name: l.Name, Quantity: decimal(l.Quantity, 3) + " " + l.Unit, Price: "—", Discount: "—", Amount: "—"})
				}
				if imageExportData(w, r, p) {
					return
				}
				data, e := printing.PDF(p)
				if e != nil {
					fail(w, e)
					return
				}
				w.Header().Set("Content-Type", "application/pdf")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Ledgerleaf-dispatch-%d.pdf"`, d.ID))
				w.Write(data)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, raw)
			return
		}
		_, limit, offset := pageArgs(r)
		s.query(w, "SELECT d.id,d.number,d.date,p.name AS customer,d.value,d.document_id,d.returned_date,(SELECT COALESCE(SUM(json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000),0) FROM json_each(d.snapshot,'$.lines') l)-COALESCE((SELECT SUM(a.quantity) FROM dispatch_allocations a WHERE a.business_id=d.business_id AND a.dispatch_id=d.id),0) AS remaining_quantity FROM dispatches d JOIN parties p ON p.business_id=d.business_id AND p.id=d.party_id WHERE d.business_id=? ORDER BY d.id DESC LIMIT ? OFFSET ?", b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var d Bill
	if e = decode(w, r, &d); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(d.Date) || !validKey(d.Key) || len(d.Lines) == 0 || len(d.Lines) > 100 || len(d.Notes) > 1000 {
		fail(w, errors.New("valid dispatch date, key and 1–100 lines required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	payload, _ := json.Marshal(d)
	ref := "dispatch:" + d.Key
	repeat, e := claim(tx, b, ref, string(payload))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if e = tx.QueryRow("SELECT name,address FROM businesses WHERE id=?", b).Scan(&d.BusinessName, &d.BusinessAddress); e != nil {
		fail(w, e)
		return
	}
	if e = tx.QueryRow("SELECT name,address FROM parties WHERE business_id=? AND id=? AND kind='CUSTOMER' AND active=1", b, d.PartyID).Scan(&d.PartyName, &d.PartyAddress); e != nil {
		fail(w, errors.New("select an active customer"))
		return
	}
	d.Kind = "SALE"
	d.ReceiptID = 0
	d.ReceiptMatches = nil
	d.SourceID = 0
	d.DispatchID = 0
	d.DispatchMatches = nil
	d.TaxEnabled = false
	d.InvoiceDiscount = 0
	d.InvoiceCharge = 0
	d.RoundOff = 0
	d.RoundToRupee = false
	d.Total = 0
	d.Status = "DISPATCHED"
	seen := map[int64]bool{}
	for i := range d.Lines {
		l := &d.Lines[i]
		if seen[l.ProductID] || l.Quantity <= 0 || l.Quantity > 1000000000 {
			fail(w, errors.New("invalid dispatch quantity or duplicate item"))
			return
		}
		seen[l.ProductID] = true
		if e = tx.QueryRow("SELECT name,sku,unit,kind,sale_price FROM products WHERE business_id=? AND id=? AND active=1 AND kind='PRODUCT'", b, l.ProductID).Scan(&l.Name, &l.SKU, &l.Unit, &l.Kind, &l.Price); e != nil {
			fail(w, errors.New("select active stock products"))
			return
		}
		if e = applyUnit(tx, b, l); e != nil {
			fail(w, e)
			return
		}
		l.Price, e = money.MulDiv(l.Price, l.UnitFactor, 1000)
		if e != nil {
			fail(w, e)
			return
		}
		q, e := l.stockQuantity()
		if e != nil {
			fail(w, e)
			return
		}
		var qty, value int64
		var last sql.NullString
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0),MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, l.ProductID, d.WarehouseID).Scan(&qty, &value, &last); e != nil {
			fail(w, e)
			return
		}
		if qty < q || (last.Valid && d.Date < last.String) {
			fail(w, errors.New("insufficient stock or dispatch predates existing movements"))
			return
		}
		l.Cost, e = money.MulDiv(value, q, qty)
		if e != nil {
			fail(w, e)
			return
		}
		l.Amount = 0
		l.Discount = 0
		l.CGST = 0
		l.SGST = 0
		l.IGST = 0
		l.Cess = 0
		d.Total += l.Cost
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(?,?,?,?,?,'DISPATCH',?,?)", b, l.ProductID, d.WarehouseID, -q, -l.Cost, ref, d.Date); e != nil {
			fail(w, e)
			return
		}
	}
	if d.Total > 1000000000000 {
		fail(w, errors.New("dispatch value exceeds limit"))
		return
	}
	if e = accounting.Seed(tx, b); e != nil {
		fail(w, e)
		return
	}
	year, e := financialYear(tx, b, d.Date)
	if e != nil {
		fail(w, e)
		return
	}
	series := "DC/" + year
	if _, e = tx.Exec("INSERT OR IGNORE INTO counters(business_id,series,next) VALUES(?,?,1)", b, series); e != nil {
		fail(w, e)
		return
	}
	var seq int64
	if e = tx.QueryRow("SELECT next FROM counters WHERE business_id=? AND series=?", b, series).Scan(&seq); e != nil {
		fail(w, e)
		return
	}
	d.Number = fmt.Sprintf("%s/%05d", series, seq)
	if _, e = tx.Exec("UPDATE counters SET next=next+1 WHERE business_id=? AND series=?", b, series); e != nil {
		fail(w, e)
		return
	}
	if e = accounting.Post(tx, b, d.Date, ref, "Goods dispatched: "+d.Number, []accounting.Line{{Account: "1250", Debit: d.Total}, {Account: "1200", Credit: d.Total}}); e != nil {
		fail(w, e)
		return
	}
	raw, _ := json.Marshal(d)
	res, e := tx.Exec("INSERT INTO dispatches(business_id,party_id,warehouse_id,date,number,reference,snapshot,value) VALUES(?,?,?,?,?,?,?,?)", b, d.PartyID, d.WarehouseID, d.Date, d.Number, ref, string(raw), d.Total)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := res.LastInsertId()
	if e = auditFor(r, tx, b, "goods.dispatched", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}
func validateDispatch(tx *sql.Tx, b int64, bill *Bill) error {
	if len(bill.DispatchMatches) > 0 {
		return validateDispatchMatches(tx, b, bill)
	}
	if bill.Kind != "SALE" || bill.hasReceipt() || bill.SourceID != 0 || bill.DocumentType != "" {
		return errors.New("dispatch requires a sales invoice without other source links")
	}
	d, balances, e := dispatchBalances(tx, b, bill.DispatchID)
	if e != nil {
		return e
	}
	if bill.Date < d.Date || bill.PartyID != d.PartyID || bill.WarehouseID != d.WarehouseID {
		return errors.New("invoice date, customer or warehouse differs from dispatch")
	}
	if e = dispatchAllocationDate(tx, b, bill.DispatchID, bill.Date); e != nil {
		return e
	}
	available := map[int64]dispatchBalance{}
	for _, row := range balances {
		available[row.ProductID] = row
	}
	for i := range bill.Lines {
		l := &bill.Lines[i]
		row, ok := available[l.ProductID]
		if !ok || l.baseUnit() != row.Unit {
			return errors.New("invoice item or base unit differs from dispatch")
		}
		q, e := l.stockQuantity()
		if e != nil {
			return e
		}
		l.Cost, e = dispatchAllocatedCost(row, q)
		if e != nil {
			return e
		}
		delete(available, l.ProductID)
	}
	return nil
}
func (s *Server) returnDispatch(w http.ResponseWriter, r *http.Request) {
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
		ID    int64  `json:"id"`
		Date  string `json:"date"`
		Key   string `json:"key"`
		Lines []struct {
			ProductID int64 `json:"product_id"`
			Quantity  int64 `json:"quantity"`
		} `json:"lines"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) {
		fail(w, errors.New("valid return date and key required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "dispatch-return:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	d, balances, e := dispatchBalances(tx, b, p.ID)
	if e != nil {
		fail(w, e)
		return
	}
	if p.Date < d.Date {
		fail(w, errors.New("return predates dispatch"))
		return
	}
	if e = dispatchAllocationDate(tx, b, p.ID, p.Date); e != nil {
		fail(w, e)
		return
	}
	chosen := map[int64]int64{}
	if len(p.Lines) > 100 {
		fail(w, errors.New("maximum 100 return lines"))
		return
	}
	for _, l := range p.Lines {
		if l.Quantity <= 0 || chosen[l.ProductID] != 0 {
			fail(w, errors.New("positive quantities and unique items required"))
			return
		}
		chosen[l.ProductID] = l.Quantity
	}
	var value int64
	var count int
	for _, row := range balances {
		q := chosen[row.ProductID]
		if len(p.Lines) == 0 {
			q = row.Remaining
		}
		if q == 0 {
			continue
		}
		cost, err := dispatchAllocatedCost(row, q)
		if err != nil {
			fail(w, err)
			return
		}
		count++
		value += cost
		delete(chosen, row.ProductID)
		l := BillLine{ProductID: row.ProductID, Cost: cost}

		var last sql.NullString
		if e = tx.QueryRow("SELECT MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, l.ProductID, d.WarehouseID).Scan(&last); e != nil {
			fail(w, e)
			return
		}
		if last.Valid && p.Date < last.String {
			fail(w, errors.New("return predates stock movements"))
			return
		}
		if _, e = tx.Exec("INSERT INTO dispatch_allocations(business_id,dispatch_id,product_id,reference,date,quantity,value) VALUES(?,?,?,?,?,?,?)", b, p.ID, row.ProductID, ref, p.Date, q, cost); e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(?,?,?,?,?,'DISPATCH_RETURN',?,?)", b, l.ProductID, d.WarehouseID, q, l.Cost, ref, p.Date); e != nil {
			fail(w, e)
			return
		}
	}
	if count == 0 || len(chosen) > 0 {
		fail(w, errors.New("select remaining dispatched items to return"))
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Dispatch returned", []accounting.Line{{Account: "1200", Debit: value}, {Account: "1250", Credit: value}}); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "dispatch.returned", p.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
