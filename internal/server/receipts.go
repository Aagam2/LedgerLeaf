package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

func (s *Server) receipts(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		if id := r.URL.Query().Get("id"); id != "" {
			var raw string
			if e = s.DB.QueryRow("SELECT snapshot FROM goods_receipts WHERE business_id=? AND id=?", b, id).Scan(&raw); e != nil {
				fail(w, e)
				return
			}
			var receipt Bill
			if e = json.Unmarshal([]byte(raw), &receipt); e != nil {
				fail(w, e)
				return
			}
			for i := range receipt.Lines {
				if e = s.DB.QueryRow("SELECT COALESCE(SUM(quantity),0) FROM receipt_matches WHERE business_id=? AND receipt_id=? AND product_id=?", b, id, receipt.Lines[i].ProductID).Scan(&receipt.Lines[i].InvoicedQuantity); e != nil {
					fail(w, e)
					return
				}
			}
			respond(w, 200, receipt)
			return
		}
		_, limit, offset := pageArgs(r)
		s.query(w, `SELECT g.id,c.number AS purchase_order,p.name AS supplier,g.date,g.value,g.document_id,(SELECT GROUP_CONCAT(DISTINCT document_id) FROM receipt_matches m WHERE m.business_id=g.business_id AND m.receipt_id=g.id) AS invoice_ids,(SELECT SUM(json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000) FROM json_each(g.snapshot,'$.lines') l)-COALESCE((SELECT SUM(quantity) FROM receipt_matches m WHERE m.business_id=g.business_id AND m.receipt_id=g.id),0) AS unmatched_quantity,(SELECT COUNT(*) FROM receipt_matches m WHERE m.business_id=g.business_id AND m.receipt_id=g.id) AS match_count,EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id) AS reversed FROM goods_receipts g LEFT JOIN commercial_documents c ON c.business_id=g.business_id AND c.id=g.order_id JOIN parties p ON p.business_id=g.business_id AND p.id=g.party_id WHERE g.business_id=? ORDER BY g.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		PartyID     int64  `json:"party_id"`
		OrderID     int64  `json:"order_id"`
		WarehouseID int64  `json:"warehouse_id"`
		Date        string `json:"date"`
		Key         string `json:"key"`
		Lines       []struct {
			Price     int64  `json:"price"`
			SaleUnit  string `json:"sale_unit"`
			ProductID int64  `json:"product_id"`
			Quantity  int64  `json:"quantity"`
		} `json:"lines"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) || len(p.Lines) == 0 || len(p.Lines) > 100 {
		fail(w, errors.New("valid date, key and receipt lines required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "receipt:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		var id int64
		if e = tx.QueryRow("SELECT id FROM goods_receipts WHERE business_id=? AND reference=?", b, ref).Scan(&id); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, map[string]int64{"id": id})
		return
	}
	var order Bill
	if p.OrderID == 0 {
		order = Bill{Kind: "PURCHASE", PartyID: p.PartyID, Date: p.Date}
		if e = tx.QueryRow("SELECT name FROM parties WHERE business_id=? AND id=? AND kind='SUPPLIER' AND active=1", b, p.PartyID).Scan(&order.PartyName); e != nil {
			fail(w, errors.New("select an active supplier"))
			return
		}
		for _, line := range p.Lines {
			l := BillLine{ProductID: line.ProductID, Quantity: line.Quantity, Price: line.Price, SaleUnit: line.SaleUnit}
			if line.Quantity <= 0 || line.Quantity > 1000000000 || line.Price < 0 || line.Price > 100000000000 {
				fail(w, errors.New("invalid receipt quantity or estimated price"))
				return
			}
			if e = tx.QueryRow("SELECT name,sku,unit,kind FROM products WHERE business_id=? AND id=? AND active=1 AND kind='PRODUCT'", b, l.ProductID).Scan(&l.Name, &l.SKU, &l.Unit, &l.Kind); e != nil {
				fail(w, errors.New("select an active stock product"))
				return
			}
			if e = applyUnit(tx, b, &l); e != nil {
				fail(w, e)
				return
			}
			l.Amount, e = money.MulDiv(l.Price, l.Quantity, 1000)
			if e != nil || l.Amount > 1000000000000 {
				fail(w, errors.New("receipt line value exceeds limit"))
				return
			}
			order.Lines = append(order.Lines, l)
		}
	} else {
		var snapshot string
		var converted sql.NullInt64
		if e = tx.QueryRow("SELECT snapshot,converted_document_id FROM commercial_documents WHERE business_id=? AND id=? AND document_type='PURCHASE_ORDER' AND status='ISSUED'", b, p.OrderID).Scan(&snapshot, &converted); e != nil || converted.Valid {
			fail(w, errors.New("select an issued purchase order that has not been converted directly to a bill"))
			return
		}
		if e = json.Unmarshal([]byte(snapshot), &order); e != nil {
			fail(w, e)
			return
		}
	}
	if p.Date < order.Date {
		fail(w, errors.New("receipt cannot predate order"))
		return
	}
	remaining := map[int64]BillLine{}
	for _, l := range order.Lines {
		if _, exists := remaining[l.ProductID]; exists {
			fail(w, errors.New("duplicate receipt item"))
			return
		}
		remaining[l.ProductID] = l
	}
	receipt := order
	receipt.Lines = nil
	receipt.Date = p.Date
	receipt.WarehouseID = p.WarehouseID
	receipt.InvoiceDiscount = 0
	receipt.InvoiceCharge = 0
	receipt.RoundOff = 0
	receipt.RoundToRupee = false
	receipt.Total = 0
	receipt.TaxTotal = 0
	receipt.TaxEnabled = false
	receipt.Inclusive = false
	receipt.InputCredit = false
	receipt.SourceID = 0
	receipt.DocumentType = ""
	receipt.Number = ""
	receipt.Status = "DRAFT"
	for _, line := range p.Lines {
		original, ok := remaining[line.ProductID]
		if !ok || line.Quantity <= 0 || line.Quantity > 1000000000 {
			fail(w, errors.New("invalid or duplicate receipt item"))
			return
		}
		delete(remaining, line.ProductID)
		var received int64
		if e = tx.QueryRow(`SELECT COALESCE(SUM(json_extract(l.value,'$.quantity')),0) FROM goods_receipts g,json_each(g.snapshot,'$.lines') l WHERE NOT EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id) AND g.business_id=? AND g.order_id=? AND json_extract(l.value,'$.product_id')=?`, b, p.OrderID, line.ProductID).Scan(&received); e != nil {
			fail(w, e)
			return
		}
		if line.Quantity > original.Quantity-received {
			fail(w, errors.New("receipt exceeds remaining purchase order quantity"))
			return
		}
		var last sql.NullString
		var unit string
		if e = tx.QueryRow("SELECT unit FROM products WHERE business_id=? AND id=? AND kind='PRODUCT'", b, line.ProductID).Scan(&unit); e != nil || unit != original.baseUnit() {
			fail(w, errors.New("order item unit changed or is not a stock product"))
			return
		}
		if e = tx.QueryRow("SELECT MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, line.ProductID, p.WarehouseID).Scan(&last); e != nil {
			fail(w, e)
			return
		}
		if last.Valid && p.Date < last.String {
			fail(w, errors.New("receipt cannot predate stock movements"))
			return
		}
		before, _ := money.MulDiv(original.Amount, received, original.Quantity)
		after, _ := money.MulDiv(original.Amount, received+line.Quantity, original.Quantity)
		value := after - before
		l := original
		l.Quantity = line.Quantity
		l.Amount = value
		l.Price, e = money.MulDiv(value, 1000, line.Quantity)
		if e != nil {
			fail(w, e)
			return
		}
		l.InvoiceDiscount = 0
		l.InvoiceCharge = 0
		l.RoundOff = 0
		l.Discount = 0
		l.Cost = value
		l.CGST = 0
		l.SGST = 0
		l.IGST = 0
		l.Cess = 0
		stockQty, err := l.stockQuantity()
		if err != nil {
			fail(w, err)
			return
		}
		receipt.Lines = append(receipt.Lines, l)
		receipt.Total += value
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(?,?,?,?,?,'RECEIPT',?,?)", b, line.ProductID, p.WarehouseID, stockQty, value, ref, p.Date); e != nil {
			fail(w, e)
			return
		}
	}
	if receipt.Total > 1000000000000 {
		fail(w, errors.New("receipt exceeds value limit"))
		return
	}
	if e = accounting.Seed(tx, b); e != nil {
		fail(w, e)
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Goods received, invoice pending", []accounting.Line{{Account: "1200", Debit: receipt.Total}, {Account: "2050", Credit: receipt.Total}}); e != nil {
		fail(w, e)
		return
	}
	receipt.ID = 0
	receipt.Key = ""
	receipt.DueDate = p.Date
	rawReceipt, _ := json.Marshal(receipt)
	res, e := tx.Exec("INSERT INTO goods_receipts(business_id,order_id,party_id,warehouse_id,date,reference,snapshot,value) VALUES(?,NULLIF(?,0),?,?,?,?,?,?)", b, p.OrderID, order.PartyID, p.WarehouseID, p.Date, ref, string(rawReceipt), receipt.Total)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := res.LastInsertId()
	if e = auditFor(r, tx, b, "goods.received", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}
