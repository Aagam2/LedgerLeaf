package server

import (
	"billbook/internal/export"
	"billbook/internal/printing"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" {
		from = "0001-01-01"
	}
	if to == "" {
		to = "9999-12-31"
	}
	if !dateValid(from) || !dateValid(to) || from > to {
		fail(w, errors.New("invalid report date range"))
		return
	}
	kind := r.URL.Query().Get("kind")
	query := ""
	var args []interface{}
	switch kind {
	case "gstr-1-preparation", "gstr-3b-preparation":
		var scheme string
		if e = s.DB.QueryRow("SELECT tax_scheme FROM businesses WHERE id=?", b).Scan(&scheme); e != nil {
			fail(w, e)
			return
		}
		if scheme != "REGULAR" {
			fail(w, errors.New("GST return preparation requires a regular registered business"))
			return
		}
		query = gstPreparationQuery(kind)
		args = []interface{}{b, from, to, b, from, to}

	case "customer-sales", "monthly-sales":
		query = salesActivityQuery(kind)
		args = []interface{}{b, from, to, b, from, to}
		if kind == "customer-sales" {
			args = append(args, b)
		}
	case "low-stock":
		query = `WITH balances AS (SELECT p.id AS product_id,p.sku,p.name AS product,p.unit,p.minimum_stock,COALESCE(SUM(m.quantity),0) AS quantity FROM products p LEFT JOIN stock_movements m ON m.business_id=p.business_id AND m.product_id=p.id AND m.date<=? WHERE p.business_id=? AND p.kind='PRODUCT' AND p.active=1 AND p.minimum_stock>0 GROUP BY p.id) SELECT *,minimum_stock-quantity AS shortage FROM balances WHERE quantity<minimum_stock ORDER BY shortage DESC,product_id`
		args = []interface{}{to, b}
	case "expense-categories":
		query = `SELECT COALESCE(NULLIF(d.category,''),'Uncategorized') AS category,SUM(l.debit-l.credit) AS amount FROM journals j JOIN journal_lines l ON l.business_id=j.business_id AND l.journal_id=j.id LEFT JOIN journals original ON original.business_id=j.business_id AND j.reference='manual-reversal:'||original.id LEFT JOIN expense_details d ON d.business_id=j.business_id AND d.journal_id=COALESCE(original.id,j.id) WHERE j.business_id=? AND j.date BETWEEN ? AND ? AND l.account='5200' AND (j.reference LIKE 'expense:%' OR original.reference LIKE 'expense:%') GROUP BY category ORDER BY amount DESC`
		args = []interface{}{b, from, to}
	case "hsn-summary", "product-sales":
		query = analyticsQuery(kind)
		args = []interface{}{b, from, to, b, from, to}
	case "trial-balance", "profit-loss", "balance-sheet":
		if kind == "balance-sheet" {
			from = "0001-01-01"
		}
		query = `SELECT a.code,a.name,a.kind,COALESCE(SUM(x.debit),0) AS debit,COALESCE(SUM(x.credit),0) AS credit,COALESCE(SUM(x.debit-x.credit),0) AS balance FROM accounts a LEFT JOIN (SELECT l.* FROM journal_lines l JOIN journals j ON j.id=l.journal_id AND j.business_id=l.business_id WHERE j.business_id=? AND j.date BETWEEN ? AND ?) x ON x.business_id=a.business_id AND x.account=a.code WHERE a.business_id=?`
		args = []interface{}{b, from, to, b}
		if kind == "profit-loss" {
			query = strings.Replace(query, "AND j.date BETWEEN ? AND ?)", "AND j.date BETWEEN ? AND ? AND j.reference NOT LIKE 'year-close:%')", 1)
			query += ` AND a.kind IN ('INCOME','EXPENSE')`
		}
		query += " GROUP BY a.code ORDER BY a.code"
	case "ledger", "cash-book", "bank-book":
		account := r.URL.Query().Get("account")
		if kind == "cash-book" {
			account = "1000"
		}
		if kind == "bank-book" && account == "" {
			account = "1010"
		}
		query = `SELECT j.date,j.reference,j.description,l.account,a.name,l.debit,l.credit FROM journal_lines l JOIN journals j ON j.business_id=l.business_id AND j.id=l.journal_id JOIN accounts a ON a.business_id=l.business_id AND a.code=l.account WHERE l.business_id=? AND j.date BETWEEN ? AND ? AND (?='' OR l.account=?) ORDER BY j.date,j.id,l.id`
		args = []interface{}{b, from, to, account, account}
	case "sales", "purchases":
		documentKind := "SALE"
		if kind == "purchases" {
			documentKind = "PURCHASE"
		}
		query = `SELECT d.date,d.number,p.name AS party,d.total,COALESCE((SELECT SUM(amount) FROM payments WHERE business_id=d.business_id AND document_id=d.id AND date<=?),0) AS paid,COALESCE((SELECT SUM(total) FROM credit_notes c WHERE c.business_id=d.business_id AND c.document_id=d.id AND c.date<=?),0) AS returned FROM documents d JOIN parties p ON p.business_id=d.business_id AND p.id=d.party_id WHERE d.business_id=? AND d.kind=? AND d.status='FINALIZED' AND d.date BETWEEN ? AND ? ORDER BY d.date,d.id`
		args = []interface{}{to, to, b, documentKind, from, to}
	case "products":
		query = "SELECT name,kind,sku,barcode,unit,sale_price,minimum_stock FROM products WHERE business_id=? AND active=1 ORDER BY id"
		args = []interface{}{b}
	case "parties":
		query = "SELECT name,kind,phone,email,address,gstin,state_code FROM parties WHERE business_id=? AND active=1 ORDER BY id"
		args = []interface{}{b}
	case "party-statement":
		query = `WITH events AS (
 SELECT d.party_id,d.date,d.number AS reference,'INVOICE' AS entry,CASE WHEN d.kind='SALE' THEN d.total ELSE 0 END AS debit,CASE WHEN d.kind='PURCHASE' THEN d.total ELSE 0 END AS credit FROM documents d WHERE d.business_id=? AND d.status='FINALIZED'
 UNION ALL SELECT d.party_id,p.date,p.reference,'PAYMENT',CASE WHEN d.kind='PURCHASE' THEN p.amount ELSE 0 END,CASE WHEN d.kind='SALE' THEN p.amount ELSE 0 END FROM payments p JOIN documents d ON d.business_id=p.business_id AND d.id=p.document_id WHERE p.business_id=?
 UNION ALL SELECT d.party_id,c.date,c.number,'CREDIT / REFUND',CASE WHEN d.kind='PURCHASE' THEN c.total ELSE c.refund END,CASE WHEN d.kind='SALE' THEN c.total ELSE c.refund END FROM credit_notes c JOIN documents d ON d.business_id=c.business_id AND d.id=c.document_id WHERE c.business_id=?
 UNION ALL SELECT o.party_id,o.date,o.reference,'OPENING',CASE WHEN p.kind='CUSTOMER' THEN o.amount ELSE 0 END,CASE WHEN p.kind='SUPPLIER' THEN o.amount ELSE 0 END FROM party_openings o JOIN parties p ON p.business_id=o.business_id AND p.id=o.party_id WHERE o.business_id=?
 UNION ALL SELECT o.party_id,o.date,o.reference,'OPENING PAYMENT',CASE WHEN p.kind='SUPPLIER' THEN o.amount ELSE 0 END,CASE WHEN p.kind='CUSTOMER' THEN o.amount ELSE 0 END FROM opening_settlements o JOIN parties p ON p.business_id=o.business_id AND p.id=o.party_id WHERE o.business_id=?
 ), running AS (SELECT p.id AS party_id,p.name,p.kind,e.date,e.reference,e.entry,e.debit,e.credit,SUM(e.debit-e.credit) OVER(PARTITION BY e.party_id ORDER BY e.date,e.reference,e.entry ROWS UNBOUNDED PRECEDING) AS balance FROM events e JOIN parties p ON p.id=e.party_id AND p.business_id=? WHERE e.date<=?)
 SELECT * FROM running WHERE date>=? AND (?='' OR party_id=?) ORDER BY name,party_id,date,reference,entry`
		party := r.URL.Query().Get("party_id")
		args = []interface{}{b, b, b, b, b, b, to, from, party, party}
	case "ageing":
		query = `WITH balances AS (SELECT d.id,d.number,p.name AS party,d.kind,d.date,d.due_date,d.total-COALESCE((SELECT SUM(amount) FROM payments WHERE business_id=d.business_id AND document_id=d.id AND date<=?),0)-COALESCE((SELECT SUM(total-refund) FROM credit_notes WHERE business_id=d.business_id AND document_id=d.id AND date<=?),0) AS outstanding FROM documents d JOIN parties p ON p.business_id=d.business_id AND p.id=d.party_id WHERE d.business_id=? AND d.status='FINALIZED' AND d.date<=?) SELECT *,CASE WHEN due_date>=? THEN 'NOT DUE' WHEN julianday(?)-julianday(due_date)<=30 THEN '1-30 DAYS' WHEN julianday(?)-julianday(due_date)<=60 THEN '31-60 DAYS' WHEN julianday(?)-julianday(due_date)<=90 THEN '61-90 DAYS' ELSE '90+ DAYS' END AS age_bucket FROM balances WHERE outstanding>0 ORDER BY due_date,id`
		args = []interface{}{to, to, b, to, to, to, to, to}
	case "gst":
		query = `WITH tax_rows AS (
 SELECT d.date,d.number,d.kind,'INVOICE' AS entry,json_extract(d.snapshot,'$.party_gstin') AS gstin,json_extract(d.snapshot,'$.place_of_supply') AS place_of_supply,json_extract(d.snapshot,'$.tax_mode') AS treatment,json_extract(l.value,'$.hsn') AS hsn,json_extract(l.value,'$.rate') AS rate,json_extract(l.value,'$.amount') AS taxable_value,COALESCE(json_extract(l.value,'$.cgst'),0) AS cgst,COALESCE(json_extract(l.value,'$.sgst'),0) AS sgst,COALESCE(json_extract(l.value,'$.igst'),0) AS igst,COALESCE(json_extract(l.value,'$.cess'),0) AS cess FROM documents d,json_each(d.snapshot,'$.lines') l WHERE d.business_id=? AND d.status='FINALIZED' AND json_extract(d.snapshot,'$.tax_enabled')=1
 UNION ALL SELECT v.date,d.number,d.kind,'RETURN',json_extract(d.snapshot,'$.party_gstin'),json_extract(d.snapshot,'$.place_of_supply'),json_extract(d.snapshot,'$.tax_mode'),json_extract(l.value,'$.hsn'),json_extract(l.value,'$.rate'),-json_extract(l.value,'$.amount'),-COALESCE(json_extract(l.value,'$.cgst'),0),-COALESCE(json_extract(l.value,'$.sgst'),0),-COALESCE(json_extract(l.value,'$.igst'),0),-COALESCE(json_extract(l.value,'$.cess'),0) FROM credit_notes v JOIN documents d ON d.business_id=v.business_id AND d.id=v.document_id,json_each(v.snapshot,'$.lines') l WHERE d.business_id=? AND json_extract(d.snapshot,'$.tax_enabled')=1)
 SELECT * FROM tax_rows WHERE date BETWEEN ? AND ? ORDER BY date,number,entry,hsn`
		args = []interface{}{b, b, from, to}
	case "stock":
		query = `SELECT p.sku,p.name,w.name AS warehouse,COALESCE(SUM(m.quantity),0) AS quantity,COALESCE(SUM(m.value),0) AS value FROM products p JOIN warehouses w ON w.business_id=p.business_id LEFT JOIN stock_movements m ON m.business_id=p.business_id AND m.product_id=p.id AND m.warehouse_id=w.id AND m.date<=? WHERE p.business_id=? AND p.kind='PRODUCT' GROUP BY p.id,w.id ORDER BY p.name,p.id,w.id`
		args = []interface{}{to, b}
	case "audit":
		query = `SELECT id,action,entity_id,created_at FROM audit_log WHERE business_id=? AND substr(created_at,1,10) BETWEEN ? AND ? ORDER BY id DESC`
		args = []interface{}{b, from, to}
	default:
		fail(w, errors.New("unknown report"))
		return
	}
	format := r.URL.Query().Get("format")
	if format != "" && format != "csv" && format != "xlsx" && format != "pdf" {
		fail(w, errors.New("supported export formats are CSV, XLSX and PDF"))
		return
	}
	if format == "" {
		_, limit, offset := pageArgs(r)
		s.query(w, query+" LIMIT ? OFFSET ?", append(args, limit, offset)...)
		return
	}
	rows, e := s.DB.QueryContext(r.Context(), query, args...)
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
	next := func() ([]interface{}, error) {
		if !rows.Next() {
			if e := rows.Err(); e != nil {
				return nil, e
			}
			return nil, io.EOF
		}
		values := make([]interface{}, len(cols))
		ptr := make([]interface{}, len(cols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if e := rows.Scan(ptr...); e != nil {
			return nil, e
		}
		return values, nil
	}
	if format == "pdf" {
		data, e := printing.Report("Ledgerleaf · "+kind+" · "+from+" to "+to, cols, next)
		if e != nil {
			fail(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-`+kind+`.pdf"`)
		w.Write(data)
		return
	}
	if format == "xlsx" {
		file, e := os.CreateTemp("", "ledgerleaf-export-*.xlsx")
		if e != nil {
			fail(w, e)
			return
		}
		defer os.Remove(file.Name())
		defer file.Close()
		if e = export.Spreadsheet(file, cols, next); e != nil {
			fail(w, e)
			return
		}
		if _, e = file.Seek(0, 0); e != nil {
			fail(w, e)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-`+kind+`.xlsx"`)
		io.Copy(w, file)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Ledgerleaf-`+kind+`.csv"`)
	writer := csv.NewWriter(w)
	writer.Write(cols)
	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptr := make([]interface{}, len(cols))
		for i := range values {
			ptr[i] = &values[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			return
		}
		record := make([]string, len(cols))
		for i, v := range values {
			if v == nil {
				continue
			}
			text := fmt.Sprint(v)
			if bytes, ok := v.([]byte); ok {
				text = string(bytes)
			}
			if _, numeric := v.(int64); !numeric {
				trim := strings.TrimLeft(text, " \t\r\n")
				if strings.HasPrefix(trim, "=") || strings.HasPrefix(trim, "+") || strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "@") {
					text = "'" + text
				}
			}
			record[i] = text
		}
		if e = writer.Write(record); e != nil {
			return
		}
	}
	if rows.Err() != nil {
		panic(http.ErrAbortHandler)
	}
	writer.Flush()
}

func (s *Server) reportSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" || r.URL.Query().Get("kind") == "balance-sheet" {
		from = "0001-01-01"
	}
	if to == "" {
		to = "9999-12-31"
	}
	if !dateValid(from) || !dateValid(to) || from > to {
		fail(w, errors.New("invalid date range"))
		return
	}
	query := `SELECT a.kind,COALESCE(SUM(l.debit-l.credit),0) AS balance FROM accounts a JOIN journal_lines l ON l.business_id=a.business_id AND l.account=a.code JOIN journals j ON j.id=l.journal_id AND j.business_id=l.business_id WHERE a.business_id=? AND j.date BETWEEN ? AND ?`
	if r.URL.Query().Get("kind") == "profit-loss" {
		query += " AND j.reference NOT LIKE 'year-close:%'"
		query += ` AND a.kind IN ('INCOME','EXPENSE')`
	}
	s.query(w, query+" GROUP BY a.kind", b, from, to)
}
