package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type BillLine struct {
	CessPerUnit      int64  `json:"cess_per_unit,omitempty"`
	InvoiceDiscount  int64  `json:"invoice_discount,omitempty"`
	InvoiceCharge    int64  `json:"invoice_charge,omitempty"`
	RoundOff         int64  `json:"round_off,omitempty"`
	SaleUnit         string `json:"sale_unit,omitempty"`
	BaseUnit         string `json:"base_unit,omitempty"`
	UnitFactor       int64  `json:"unit_factor,omitempty"`
	ReceivedQuantity int64  `json:"received_quantity,omitempty"`
	InvoicedQuantity int64  `json:"invoiced_quantity,omitempty"`
	ReturnedQuantity int64  `json:"returned_quantity,omitempty"`
	HSN              string `json:"hsn"`
	Treatment        string `json:"treatment"`
	Rate             int64  `json:"rate"`
	CessRate         int64  `json:"cess_rate"`
	CGST             int64  `json:"cgst"`
	SGST             int64  `json:"sgst"`
	IGST             int64  `json:"igst"`
	Cess             int64  `json:"cess"`
	ProductID        int64  `json:"product_id"`
	Name             string `json:"name"`
	SKU              string `json:"sku"`
	Unit             string `json:"unit"`
	Kind             string `json:"kind"`
	Quantity         int64  `json:"quantity"`
	Price            int64  `json:"price"`
	Discount         int64  `json:"discount"`
	Amount           int64  `json:"amount"`
	Cost             int64  `json:"cost"`
}
type Bill struct {
	DispatchMatches []DispatchMatch `json:"dispatch_matches,omitempty"`
	ImportReference string          `json:"import_reference,omitempty"`
	DispatchID      int64           `json:"dispatch_id,omitempty"`
	ReceiptMatches  []ReceiptMatch  `json:"receipt_matches,omitempty"`
	InvoiceDiscount int64           `json:"invoice_discount,omitempty"`
	InvoiceCharge   int64           `json:"invoice_charge,omitempty"`
	RoundToRupee    bool            `json:"round_to_rupee,omitempty"`
	RoundOff        int64           `json:"round_off,omitempty"`
	LogoID          int64           `json:"logo_id,omitempty"`
	ReceiptID       int64           `json:"receipt_id,omitempty"`
	PriceList       string          `json:"price_list,omitempty"`
	DocumentType    string          `json:"document_type,omitempty"`
	SourceID        int64           `json:"source_id,omitempty"`
	BusinessGSTIN   string          `json:"business_gstin"`
	PartyGSTIN      string          `json:"party_gstin"`
	SupplierState   string          `json:"supplier_state"`
	TaxScheme       string          `json:"tax_scheme"`
	TaxEnabled      bool            `json:"tax_enabled"`
	TaxMode         string          `json:"tax_mode"`
	PlaceOfSupply   string          `json:"place_of_supply"`
	ShippingAddress string          `json:"shipping_address"`
	Inclusive       bool            `json:"inclusive"`
	InputCredit     bool            `json:"input_credit"`
	TaxTotal        int64           `json:"tax_total"`
	ID              int64           `json:"id"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	PartyID         int64           `json:"party_id"`
	WarehouseID     int64           `json:"warehouse_id"`
	Date            string          `json:"date"`
	DueDate         string          `json:"due_date"`
	Number          string          `json:"number"`
	Total           int64           `json:"total"`
	BusinessName    string          `json:"business_name"`
	BusinessAddress string          `json:"business_address"`
	PartyName       string          `json:"party_name"`
	PartyAddress    string          `json:"party_address"`
	Notes           string          `json:"notes"`
	Lines           []BillLine      `json:"lines"`
	Key             string          `json:"key"`
	BusinessPhone   string          `json:"business_phone"`
	BusinessEmail   string          `json:"business_email"`
	BankDetails     string          `json:"bank_details"`
	Terms           string          `json:"terms"`
	Footer          string          `json:"footer"`
}

func validKey(s string) bool { return len(s) >= 16 && len(s) <= 80 }
func claim(tx *sql.Tx, b int64, key, payload string) (bool, error) {
	var old string
	e := tx.QueryRow("SELECT payload FROM requests WHERE business_id=? AND key=?", b, key).Scan(&old)
	if e == nil {
		if old != payload {
			return false, errors.New("idempotency key was reused with different details")
		}
		return true, nil
	}
	if e != sql.ErrNoRows {
		return false, e
	}
	_, e = tx.Exec("INSERT INTO requests(business_id,key,payload) VALUES(?,?,?)", b, key, payload)
	return false, e
}
func (s *Server) documents(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		if r.URL.Query().Get("id") != "" {
			var snapshot string
			var reversed bool
			e = s.DB.QueryRow("SELECT snapshot,EXISTS(SELECT 1 FROM document_reversals v WHERE v.business_id=d.business_id AND v.document_id=d.id) FROM documents d WHERE business_id=? AND id=?", b, r.URL.Query().Get("id")).Scan(&snapshot, &reversed)
			if e != nil {
				fail(w, errors.New("document not found"))
				return
			}
			var bill Bill
			if e = json.Unmarshal([]byte(snapshot), &bill); e != nil {
				fail(w, e)
				return
			}
			for i := range bill.Lines {
				if e = s.DB.QueryRow(`SELECT COALESCE(SUM(json_extract(l.value,'$.quantity')),0) FROM credit_notes c,json_each(c.snapshot,'$.lines') l WHERE c.business_id=? AND c.document_id=? AND json_extract(l.value,'$.product_id')=?`, b, bill.ID, bill.Lines[i].ProductID).Scan(&bill.Lines[i].ReturnedQuantity); e != nil {
					fail(w, e)
					return
				}
			}
			if reversed {
				bill.Status = "REVERSED"
			}
			respond(w, 200, bill)
			return
		}
		q, limit, offset := pageArgs(r)
		s.query(w, `SELECT d.id,d.kind,CASE WHEN EXISTS(SELECT 1 FROM document_reversals v WHERE v.business_id=d.business_id AND v.document_id=d.id) THEN 'REVERSED' ELSE d.status END AS status,d.date,d.due_date,COALESCE(d.number,'Draft') AS number,p.name AS party,d.total,x.paid-x.refunded AS paid,x.returned FROM documents d JOIN document_settlements x ON x.business_id=d.business_id AND x.id=d.id JOIN parties p ON p.id=d.party_id AND p.business_id=d.business_id WHERE d.business_id=? AND (p.name LIKE ? OR d.number LIKE ?) AND (?='' OR d.status=?) AND (?='' OR d.kind=?) AND (?='' OR (d.status='FINALIZED' AND NOT EXISTS(SELECT 1 FROM document_reversals v WHERE v.business_id=d.business_id AND v.document_id=d.id) AND d.total-x.paid-x.returned+x.refunded>0)) ORDER BY d.id DESC LIMIT ? OFFSET ?`, b, q, q, r.URL.Query().Get("status"), r.URL.Query().Get("status"), r.URL.Query().Get("kind"), r.URL.Query().Get("kind"), r.URL.Query().Get("outstanding"), limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var bill Bill
	if e = decode(w, r, &bill); e != nil {
		fail(w, e)
		return
	}
	if (bill.Kind != "SALE" && bill.Kind != "PURCHASE") || !dateValid(bill.Date) || !dateValid(bill.DueDate) || bill.DueDate < bill.Date || len(bill.Lines) < 1 || len(bill.Lines) > 100 || len(bill.Notes) > 1000 || !validKey(bill.Key) {
		fail(w, errors.New("invalid document details"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(bill)
	requestPrefix := "draft:"
	commercial := r.URL.Path == "/api/commercial-documents"
	if commercial {
		requestPrefix = "commercial:"
	}
	repeated, e := claim(tx, b, requestPrefix+bill.Key, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeated {
		table := "documents"
		if commercial {
			table = "commercial_documents"
		}
		var saved string
		if e = tx.QueryRow("SELECT snapshot FROM "+table+" WHERE business_id=? AND json_extract(snapshot,'$.key')=? ORDER BY id DESC LIMIT 1", b, bill.Key).Scan(&saved); e != nil {
			fail(w, errors.New("saved draft has since changed; reload it before retrying"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, saved)
		return
	}
	if bill.SourceID > 0 && !commercial {
		var kind, status string
		var converted sql.NullInt64
		if e = tx.QueryRow("SELECT kind,status,converted_document_id FROM commercial_documents WHERE business_id=? AND id=? AND document_type!='DELIVERY_CHALLAN'", b, bill.SourceID).Scan(&kind, &status, &converted); e != nil || status != "ISSUED" || kind != bill.Kind || (bill.ID != 0 && (!converted.Valid || converted.Int64 != bill.ID)) {
			fail(w, errors.New("select an issued source document of the correct type"))
			return
		}
		var receipts int
		if e = tx.QueryRow("SELECT COUNT(*) FROM goods_receipts g WHERE business_id=? AND order_id=? AND NOT EXISTS(SELECT 1 FROM receipt_reversals v WHERE v.business_id=g.business_id AND v.receipt_id=g.id)", b, bill.SourceID).Scan(&receipts); e != nil {
			fail(w, e)
			return
		}
		if receipts > 0 {
			fail(w, errors.New("purchase order has receipts; invoice each goods receipt instead"))
			return
		}
		if converted.Valid && converted.Int64 != bill.ID {
			fail(w, errors.New("source document has already been converted"))
			return
		}
	}
	bill.ImportReference = ""
	if bill.ID > 0 && !commercial {
		var oldRaw string
		if e = tx.QueryRow("SELECT snapshot FROM documents WHERE business_id=? AND id=?", b, bill.ID).Scan(&oldRaw); e != nil {
			fail(w, e)
			return
		}
		var old Bill
		if e = json.Unmarshal([]byte(oldRaw), &old); e != nil {
			fail(w, e)
			return
		}
		bill.ImportReference = old.ImportReference
		if old.DispatchID != bill.DispatchID || old.ReceiptID != bill.ReceiptID || old.SourceID != bill.SourceID || old.hasReceipt() != bill.hasReceipt() || old.hasDispatch() != bill.hasDispatch() {
			fail(w, errors.New("source links on existing drafts cannot be changed"))
			return
		}
	}
	if status, err := prepareDraft(tx, b, &bill, r); err != nil {
		if status == 403 {
			http.Error(w, err.Error(), status)
		} else {
			fail(w, err)
		}
		return
	}
	if commercial {
		if e = saveCommercial(tx, b, &bill); e != nil {
			fail(w, e)
			return
		}
		if e = auditFor(r, tx, b, "commercial.draft_saved", bill.ID); e != nil {
			fail(w, e)
			return
		}
		if e = tx.Commit(); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, bill)
		return
	}
	if bill.ID == 0 {
		res, e := tx.Exec("INSERT INTO documents(business_id,kind,party_id,warehouse_id,date,due_date,total,snapshot) VALUES(?,?,?,?,?,?,?,'{}')", b, bill.Kind, bill.PartyID, bill.WarehouseID, bill.Date, bill.DueDate, bill.Total)
		if e != nil {
			fail(w, e)
			return
		}
		bill.ID, _ = res.LastInsertId()
	}
	snapshot, _ := json.Marshal(bill)
	res, e := tx.Exec("UPDATE documents SET kind=?,party_id=?,warehouse_id=?,date=?,due_date=?,total=?,snapshot=? WHERE business_id=? AND id=? AND status='DRAFT'", bill.Kind, bill.PartyID, bill.WarehouseID, bill.Date, bill.DueDate, bill.Total, string(snapshot), b, bill.ID)
	if e != nil {
		fail(w, e)
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		fail(w, errors.New("only existing drafts can be edited"))
		return
	}
	if bill.SourceID > 0 {
		if _, e = tx.Exec("UPDATE commercial_documents SET converted_document_id=? WHERE business_id=? AND id=? AND converted_document_id IS NULL", bill.ID, b, bill.SourceID); e != nil {
			fail(w, e)
			return
		}
	}
	if e = auditFor(r, tx, b, "document.draft_saved", bill.ID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, bill)
}
func (s *Server) finalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	var request struct {
		ID int64 `json:"id"`
	}
	if e = decode(w, r, &request); e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var snapshot, status string
	if e = tx.QueryRow("SELECT snapshot,status FROM documents WHERE business_id=? AND id=?", b, request.ID).Scan(&snapshot, &status); e != nil {
		fail(w, errors.New("document not found"))
		return
	}
	var bill Bill
	if e = json.Unmarshal([]byte(snapshot), &bill); e != nil {
		fail(w, e)
		return
	}
	if actorOf(r).Role == "BILLER" && bill.Kind != "SALE" {
		http.Error(w, "billers may finalize sales only", 403)
		return
	}
	if status == "FINALIZED" {
		respond(w, 200, bill)
		return
	}
	if status != "DRAFT" {
		fail(w, errors.New("document cannot be finalized"))
		return
	}
	bill, e = finalizeDraft(tx, b, bill, r)
	if e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, bill)
}
func (s *Server) payments(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		documentID := int64(0)
		if raw := r.URL.Query().Get("document_id"); raw != "" {
			documentID, e = strconv.ParseInt(raw, 10, 64)
			if e != nil || documentID <= 0 {
				fail(w, errors.New("invalid document ID"))
				return
			}
		}
		s.query(w, `SELECT p.id,p.date,p.amount,p.mode,p.account_code,d.number,d.kind FROM payments p JOIN documents d ON d.id=p.document_id AND d.business_id=p.business_id WHERE p.business_id=? AND (?=0 OR p.document_id=?) ORDER BY p.id DESC LIMIT ? OFFSET ?`, b, documentID, documentID, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Account    string `json:"account"`
		DocumentID int64  `json:"document_id"`
		Amount     int64  `json:"amount"`
		Date       string `json:"date"`
		Mode       string `json:"mode"`
		Key        string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	modes := map[string]bool{"CASH": true, "UPI": true, "BANK_TRANSFER": true, "CHEQUE": true, "CARD": true, "OTHER": true}
	if p.Amount <= 0 || p.Amount > 1000000000000 || !dateValid(p.Date) || !modes[p.Mode] || !validKey(p.Key) {
		fail(w, errors.New("invalid payment details"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "payment:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	var kind, status, date string
	var total, paid int64
	if e = tx.QueryRow("SELECT kind,status,date,total FROM documents WHERE business_id=? AND id=?", b, p.DocumentID).Scan(&kind, &status, &date, &total); e != nil || status != "FINALIZED" {
		fail(w, errors.New("select a finalized document"))
		return
	}
	if actorOf(r).Role == "BILLER" && kind != "SALE" {
		http.Error(w, "billers may record customer payments only", 403)
		return
	}
	var reversalCount int
	if e = tx.QueryRow("SELECT COUNT(*) FROM document_reversals WHERE business_id=? AND document_id=?", b, p.DocumentID).Scan(&reversalCount); e != nil {
		fail(w, e)
		return
	}
	if reversalCount > 0 {
		fail(w, errors.New("returned or cancelled documents cannot receive payments"))
		return
	}
	if p.Date < date {
		fail(w, errors.New("payment date cannot predate the document"))
		return
	}
	if e = tx.QueryRow("SELECT paid+returned-refunded FROM document_settlements WHERE business_id=? AND id=?", b, p.DocumentID).Scan(&paid); e != nil {
		fail(w, e)
		return
	}
	if p.Amount > total-paid {
		fail(w, errors.New("payment exceeds outstanding amount"))
		return
	}
	account, e := paymentAccount(tx, b, p.Mode, p.Account)
	if e != nil {
		fail(w, e)
		return
	}
	res, e := tx.Exec("INSERT INTO payments(business_id,document_id,amount,date,mode,reference,account_code) VALUES(?,?,?,?,?,?,?)", b, p.DocumentID, p.Amount, p.Date, p.Mode, ref, account)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := res.LastInsertId()

	lines := []accounting.Line{{Account: account, Debit: p.Amount}, {Account: "1100", Credit: p.Amount}}
	if kind == "PURCHASE" {
		lines = []accounting.Line{{Account: "2000", Debit: p.Amount}, {Account: account, Credit: p.Amount}}
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Document payment", lines); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "payment.recorded", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}

// prepareDraft is shared by interactive entry and atomic file imports.
func prepareDraft(tx *sql.Tx, b int64, bill *Bill, r *http.Request) (int, error) {
	var e error
	var gstin string
	if e = tx.QueryRow("SELECT name,address,gstin FROM businesses WHERE id=?", b).Scan(&bill.BusinessName, &bill.BusinessAddress, &gstin); e != nil {
		return 400, e
	}
	if e = tx.QueryRow("SELECT COALESCE(logo_id,0) FROM businesses WHERE id=?", b).Scan(&bill.LogoID); e != nil {
		return 400, e
	}
	var bankName, bankAccount, bankIFSC string
	if e = tx.QueryRow("SELECT phone,email,bank_name,bank_account,bank_ifsc,invoice_terms,invoice_footer FROM businesses WHERE id=?", b).Scan(&bill.BusinessPhone, &bill.BusinessEmail, &bankName, &bankAccount, &bankIFSC, &bill.Terms, &bill.Footer); e != nil {
		return 400, e
	}
	bill.BankDetails = bankName + " " + bankAccount + " " + bankIFSC

	if actorOf(r).Role == "BILLER" && bill.Kind != "SALE" {
		return 403, errors.New("billers may create sales only")
	}
	partyKind := "CUSTOMER"
	if bill.Kind == "PURCHASE" {
		partyKind = "SUPPLIER"
	}
	if e = tx.QueryRow("SELECT name,address FROM parties WHERE business_id=? AND id=? AND kind=? AND active=1", b, bill.PartyID, partyKind).Scan(&bill.PartyName, &bill.PartyAddress); e != nil {
		return 400, errors.New("select an active party of the correct type")
	}
	if e = prepareTax(tx, b, bill); e != nil {
		return 400, e
	}
	if actorOf(r).Role == "BILLER" && (bill.InvoiceDiscount != 0 || bill.InvoiceCharge != 0) {
		return 403, errors.New("invoice adjustments require accountant permission")
	}
	bill.Total = 0
	bill.Status = "DRAFT"
	bill.Number = ""
	seen := map[int64]bool{}
	for i := range bill.Lines {
		l := &bill.Lines[i]
		if seen[l.ProductID] || l.Quantity <= 0 || l.Quantity > 1000000000 || l.Price < 0 || l.Price > 100000000000 || l.Discount < 0 {
			return 400, errors.New("invalid or duplicate document line")
		}
		seen[l.ProductID] = true
		var configuredPrice int64
		if e = tx.QueryRow("SELECT name,sku,unit,kind,sale_price FROM products WHERE business_id=? AND id=? AND active=1", b, l.ProductID).Scan(&l.Name, &l.SKU, &l.Unit, &l.Kind, &configuredPrice); e != nil {
			return 400, errors.New("select active products in this business")
		}
		if e = applyUnit(tx, b, l); e != nil {
			return 400, e
		}
		if bill.PriceList != "" && bill.Kind == "SALE" {
			if len(bill.PriceList) > 100 {
				return 400, errors.New("invalid price list")
			}
			if e = tx.QueryRow("SELECT price FROM price_lists WHERE business_id=? AND product_id=? AND name=?", b, l.ProductID, bill.PriceList).Scan(&configuredPrice); e != nil {
				return 400, errors.New("price list has no price for this item")
			}
			l.Price, e = money.MulDiv(configuredPrice, l.UnitFactor, 1000)
			if e != nil || l.Price > 100000000000 {
				return 400, errors.New("converted price exceeds limit")
			}
		}
		configuredPrice, e = money.MulDiv(configuredPrice, l.UnitFactor, 1000)
		if e != nil {
			return 400, e
		}
		if actorOf(r).Role == "BILLER" && (l.Price != configuredPrice || l.Discount != 0) {
			return 403, errors.New("price overrides and discounts require accountant permission")
		}
		if bill.Kind == "PURCHASE" && l.Kind == "SERVICE" {
			return 400, errors.New("service purchases are not supported yet")
		}
		l.Amount, e = money.MulDiv(l.Price, l.Quantity, 1000)
		if e != nil || l.Discount > l.Amount {
			return 400, errors.New("invalid line amount or discount")
		}
		l.Amount -= l.Discount
	}
	if e = allocateInvoiceAdjustments(bill); e != nil {
		return 400, e
	}
	for i := range bill.Lines {
		l := &bill.Lines[i]
		if e = calculateLineTax(tx, b, bill, l); e != nil {
			return 400, e
		}
		l.Cost = 0
		bill.Total += l.Amount
		if bill.TaxMode != "REVERSE_CHARGE" {
			bill.Total += l.taxTotal()
		}
		if bill.Total > 1000000000000 {
			return 400, errors.New("document total exceeds supported limit")
		}
	}
	applyInvoiceRounding(bill)
	if bill.Total <= 0 {
		return 400, errors.New("document total must be positive")
	}
	if bill.hasReceipt() {
		if _, e = validateReceipt(tx, b, *bill); e != nil {
			return 400, e
		}
	}
	if bill.hasDispatch() {
		if e = validateDispatch(tx, b, bill); e != nil {
			return 400, e
		}
	}
	for i := range bill.ReceiptMatches {
		bill.ReceiptMatches[i].Value = 0
	}

	return 200, nil
}

func finalizeDraft(tx *sql.Tx, b int64, bill Bill, r *http.Request) (Bill, error) {
	var e error
	if bill.Kind == "SALE" {
		var limit, outstanding int64
		if e = tx.QueryRow("SELECT credit_limit FROM parties WHERE business_id=? AND id=?", b, bill.PartyID).Scan(&limit); e != nil {
			return bill, e
		}
		if limit > 0 {
			if e = tx.QueryRow(`SELECT COALESCE((SELECT SUM(d.total-x.paid-x.returned+x.refunded) FROM documents d JOIN document_settlements x ON x.business_id=d.business_id AND x.id=d.id WHERE d.business_id=? AND d.party_id=? AND d.status='FINALIZED'),0)+COALESCE((SELECT amount FROM party_openings WHERE business_id=? AND party_id=?),0)-COALESCE((SELECT SUM(amount) FROM opening_settlements WHERE business_id=? AND party_id=?),0)`, b, bill.PartyID, b, bill.PartyID, b, bill.PartyID).Scan(&outstanding); e != nil {
				return bill, e
			}
			if outstanding+bill.Total > limit {
				return bill, errors.New("customer credit limit exceeded; settle outstanding or revise the limit")
			}
		}
	}
	check := bill
	if e = prepareTax(tx, b, &check); e != nil {
		return bill, e
	}
	if check.BusinessGSTIN != bill.BusinessGSTIN || check.PartyGSTIN != bill.PartyGSTIN || (bill.TaxScheme != "" && check.TaxScheme != bill.TaxScheme) || (bill.SupplierState != "" && check.SupplierState != bill.SupplierState) {
		return bill, errors.New("tax registration changed; review and save draft again")
	}
	for _, line := range bill.Lines {
		if bill.TaxEnabled {
			var h, treatment string
			var rate, cess, fixed int64
			e = tx.QueryRow("SELECT hsn,treatment,rate,cess,cess_per_unit FROM product_taxes WHERE business_id=? AND product_id=? AND effective_date<=? ORDER BY effective_date DESC LIMIT 1", b, line.ProductID, bill.Date).Scan(&h, &treatment, &rate, &cess, &fixed)
			if e != nil || h != line.HSN || treatment != line.Treatment || rate != line.Rate || cess != line.CessRate || fixed != line.CessPerUnit {
				return bill, errors.New("tax configuration changed; review and save draft again")
			}
		}
	}
	year, e := financialYear(tx, b, bill.Date)
	if e != nil {
		return bill, e
	}
	series := bill.Kind + "/" + year
	if _, e = tx.Exec("INSERT OR IGNORE INTO counters(business_id,series,next) VALUES(?,?,1)", b, series); e != nil {
		return bill, e
	}
	var seq int64
	if e = tx.QueryRow("SELECT next FROM counters WHERE business_id=? AND series=?", b, series).Scan(&seq); e != nil {
		return bill, e
	}
	bill.Number = fmt.Sprintf("%s/%05d", series, seq)
	if bill.TaxEnabled || bill.TaxScheme == "COMPOSITION" {
		prefix := "S"
		if bill.Kind == "PURCHASE" {
			prefix = "P"
		}
		yearNumber, _ := strconv.Atoi(year[:4])
		bill.Number = fmt.Sprintf("%s%02d/%08d", prefix, yearNumber%100, seq)
		if len(bill.Number) > 16 {
			return bill, errors.New("invoice sequence exhausted")
		}
	}
	if _, e = tx.Exec("UPDATE counters SET next=next+1 WHERE business_id=? AND series=?", b, series); e != nil {
		return bill, e
	}
	var receiptValue int64
	if bill.hasReceipt() {
		receiptValue, e = validateReceipt(tx, b, bill)
		if e != nil {
			return bill, e
		}
	}
	if bill.hasDispatch() {
		if e = validateDispatch(tx, b, &bill); e != nil {
			return bill, e
		}
	}
	ref := "document:" + strconv.FormatInt(bill.ID, 10)
	var cost int64
	for i := range bill.Lines {
		l := &bill.Lines[i]
		var active int
		var currentUnit, currentKind string
		if e = tx.QueryRow("SELECT active,unit,kind FROM products WHERE business_id=? AND id=?", b, l.ProductID).Scan(&active, &currentUnit, &currentKind); e != nil || active != 1 {
			return bill, errors.New("a product has been archived")
		}
		if currentUnit != l.baseUnit() || currentKind != l.Kind {
			return bill, errors.New("a product's unit or type changed; review and save the draft again before finalizing")
		}
		if bill.hasDispatch() {
			cost += l.Cost
			continue
		}
		if l.Kind == "SERVICE" || bill.hasReceipt() {
			continue
		}
		var qty, value int64
		var lastDate sql.NullString
		if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0),MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, l.ProductID, bill.WarehouseID).Scan(&qty, &value, &lastDate); e != nil {
			return bill, e
		}
		if lastDate.Valid && bill.Date < lastDate.String {
			return bill, errors.New("stock transactions cannot predate existing movements")
		}
		stockQty, err := l.stockQuantity()
		if err != nil {
			return bill, err
		}
		delta := stockQty
		change := l.Amount
		if !bill.InputCredit {
			change += l.taxTotal()
		}
		if bill.Kind == "SALE" {
			if qty < stockQty {
				return bill, fmt.Errorf("insufficient stock for %s", l.Name)
			}
			l.Cost, e = money.MulDiv(value, stockQty, qty)
			if e != nil {
				return bill, e
			}
			cost += l.Cost
			delta = -stockQty
			change = -l.Cost
		}
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date) VALUES(?,?,?,?,?,?,?,?)", b, l.ProductID, bill.WarehouseID, delta, change, bill.Kind, ref, bill.Date); e != nil {
			return bill, e
		}
	}
	stockAccount := "1200"
	if bill.hasDispatch() {
		stockAccount = "1250"
		if e = recordDispatchInvoice(tx, b, bill, ref); e != nil {
			return bill, e
		}
	}
	lines := []accounting.Line{{Account: "1100", Debit: bill.Total}, {Account: "4000", Credit: bill.baseTotal()}, {Account: "5000", Debit: cost}, {Account: stockAccount, Credit: cost}}
	if bill.Kind == "PURCHASE" {
		lines = []accounting.Line{{Account: "1200", Debit: bill.inventoryTotal()}, {Account: "2000", Credit: bill.Total}}
	}
	if bill.hasReceipt() {
		lines = []accounting.Line{{Account: "2050", Debit: receiptValue}, {Account: "2000", Credit: bill.Total}}
		difference := bill.inventoryTotal() - receiptValue
		if difference > 0 {
			lines = append(lines, accounting.Line{Account: "5100", Debit: difference})
		} else if difference < 0 {
			lines = append(lines, accounting.Line{Account: "5100", Credit: -difference})
		}
		if e = postReceiptMatches(tx, b, &bill); e != nil {
			return bill, e
		}
		if bill.ReceiptID > 0 {
			if _, e = tx.Exec("UPDATE goods_receipts SET document_id=? WHERE business_id=? AND id=? AND document_id IS NULL", bill.ID, b, bill.ReceiptID); e != nil {
				return bill, e
			}
		}
	}
	lines = append(lines, taxJournal(bill, false)...)
	lines = append(lines, roundingJournal(bill, false)...)
	if e = accounting.Seed(tx, b); e != nil {
		return bill, e
	}
	if e = accounting.Post(tx, b, bill.Date, ref, bill.Number, lines); e != nil {
		return bill, e
	}
	bill.Status = "FINALIZED"
	snapshotBytes, _ := json.Marshal(bill)
	if _, e = tx.Exec("UPDATE documents SET status='FINALIZED',number=?,snapshot=? WHERE business_id=? AND id=?", bill.Number, string(snapshotBytes), b, bill.ID); e != nil {
		return bill, e
	}
	if e = auditFor(r, tx, b, "document.finalized", bill.ID); e != nil {
		return bill, e
	}

	return bill, nil
}
