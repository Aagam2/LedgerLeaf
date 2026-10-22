// Package backup creates standalone SQLite snapshots and validates them before use.
package backup

import (
	"billbook/internal/database"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func Verify(path string) error {
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return errors.New("backup must be a regular file")
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	db, e := sql.Open("sqlite3", u.String()+"?mode=ro&immutable=1")
	if e != nil {
		return e
	}
	defer db.Close()
	var result string
	if e = db.QueryRow("PRAGMA integrity_check").Scan(&result); e != nil {
		return e
	}
	if result != "ok" {
		return errors.New("backup failed SQLite integrity check")
	}
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		return e
	}
	if version < 5 || version > 26 {
		return fmt.Errorf("backup schema %d is not supported by this build", version)
	}
	rows, e := db.Query("PRAGMA foreign_key_check")
	if e != nil {
		return e
	}
	if rows.Next() {
		rows.Close()
		return errors.New("backup contains broken references")
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	var count int
	if e = db.QueryRow("SELECT COUNT(*) FROM (SELECT journal_id FROM journal_lines GROUP BY journal_id HAVING SUM(debit)!=SUM(credit))").Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return errors.New("backup contains unbalanced journals")
	}
	for _, table := range []string{"businesses", "parties", "products", "documents", "document_reversals", "payments", "stock_movements", "requests", "audit_log"} {
		if e = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); e != nil {
			return e
		}
	}
	if version >= 6 {
		if e = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); e != nil {
			return e
		}
	}
	checks := []string{
		"SELECT id,name,legal_name,gstin,state_code,address FROM businesses LIMIT 0",
		"SELECT id,business_id,kind,name,phone,email,address,active FROM parties LIMIT 0",
		"SELECT id,business_id,name,sku,barcode,unit,kind,sale_price,minimum_stock,active FROM products LIMIT 0",
		"SELECT id,business_id,kind,status,party_id,warehouse_id,date,due_date,number,total,snapshot FROM documents LIMIT 0",
		"SELECT id,business_id,date,reference,description FROM journals LIMIT 0",
		"SELECT business_id,code,name,kind FROM accounts LIMIT 0",
		"SELECT id,business_id,product_id,warehouse_id,quantity,value,kind,reference,date,remarks FROM stock_movements LIMIT 0",
	}
	if version >= 6 {
		checks = append(checks, "SELECT id,username,password_hash,role,active FROM users LIMIT 0")
	}
	if version >= 7 {
		checks = append(checks, "SELECT phone,email,pan,bank_name,bank_account,bank_ifsc,invoice_terms,invoice_footer FROM businesses LIMIT 0")
	}
	if version >= 8 {
		checks = append(checks, "SELECT tax_scheme FROM businesses LIMIT 0", "SELECT gstin,state_code FROM parties LIMIT 0", "SELECT business_id,product_id,effective_date,hsn,treatment,rate,cess FROM product_taxes LIMIT 0")
	}
	if version >= 9 {
		checks = append(checks, "SELECT business_id,document_id,number,date,total,refund,snapshot FROM credit_notes LIMIT 0", "SELECT business_id,id,paid,returned,refunded FROM document_settlements LIMIT 0")
	}
	if version >= 10 {
		checks = append(checks, "SELECT business_id,id,document_type,kind,status,number,date,party_id,total,snapshot,converted_document_id FROM commercial_documents LIMIT 0")
	}
	if version >= 11 {
		checks = append(checks, "SELECT business_id,party_id,date,amount,reference FROM party_openings LIMIT 0", "SELECT business_id,party_id,date,amount,reference FROM party_advances LIMIT 0", "SELECT business_id,advance_id,document_id,date,amount,kind,reference FROM advance_usage LIMIT 0", "SELECT business_id,party_id,date,amount,reference FROM opening_settlements LIMIT 0")
	}
	if version >= 12 {
		checks = append(checks, "SELECT business_id,closed_through FROM business_controls LIMIT 0", "SELECT business_id,code,name,account_number,ifsc FROM bank_accounts LIMIT 0", "SELECT business_id,journal_line_id,statement_date,statement_reference FROM bank_reconciliations LIMIT 0", "SELECT account_code FROM payments LIMIT 0")
	}
	if version >= 13 {
		checks = append(checks, "SELECT category,brand,variant,purchase_price,mrp FROM products LIMIT 0", "SELECT credit_limit,credit_days FROM parties LIMIT 0", "SELECT business_id,product_id,name,price FROM price_lists LIMIT 0")
	}
	if version >= 14 {
		checks = append(checks, "SELECT business_id,order_id,party_id,warehouse_id,date,reference,snapshot,value,document_id FROM goods_receipts LIMIT 0")
	}
	if version >= 15 {
		checks = append(checks, "SELECT enabled,interval_hours,retain_count,last_success,last_error FROM backup_policy WHERE id=1")
	}
	if version >= 16 {
		checks = append(checks, "SELECT restricted FROM users LIMIT 0", "SELECT user_id,business_id FROM user_businesses LIMIT 0")
	}
	if version >= 17 {
		checks = append(checks, "SELECT business_id,product_id,name,factor FROM product_units LIMIT 0")
	}
	if version >= 18 {
		checks = append(checks, "SELECT business_id,format,content FROM media_assets LIMIT 0", "SELECT logo_id FROM businesses LIMIT 0", "SELECT image_id FROM products LIMIT 0")
	}
	if version >= 20 {
		checks = append(checks, "SELECT business_id,receipt_id,document_id,product_id,quantity,value FROM receipt_matches LIMIT 0", "SELECT business_id,receipt_id,date,reason,reference FROM receipt_reversals LIMIT 0")
	}
	if version >= 21 {
		checks = append(checks, "SELECT business_id,year_start_month FROM business_settings LIMIT 0", "SELECT id,business_id,name FROM product_groups LIMIT 0", "SELECT business_id,product_id,group_id,attributes FROM product_variants LIMIT 0")
	}
	if version >= 22 {
		checks = append(checks, "SELECT business_id,journal_id,category FROM expense_details LIMIT 0", "SELECT id,business_id,description,category,amount,account,next_date,interval_months,anchor_day,active FROM recurring_expenses LIMIT 0", "SELECT business_id,journal_id,name,mime,content FROM expense_attachments LIMIT 0")
	}
	if version >= 23 {
		checks = append(checks, "SELECT business_id,date,reference,net_profit FROM year_closings LIMIT 0")
	}
	if version >= 24 {
		checks = append(checks, "SELECT cess_per_unit FROM product_taxes LIMIT 0")
	}
	if version >= 25 {
		checks = append(checks, "SELECT business_id,party_id,warehouse_id,date,number,reference,snapshot,value,document_id,returned_date FROM dispatches LIMIT 0")
	}
	if version >= 26 {
		checks = append(checks, "SELECT business_id,dispatch_id,product_id,reference,date,document_id,quantity,value FROM dispatch_allocations LIMIT 0")
	}
	for _, query := range checks {
		rows, e := db.Query(query)
		if e != nil {
			return fmt.Errorf("backup schema validation: %w", e)
		}
		rows.Close()
	}
	if e = db.QueryRow("SELECT COUNT(*) FROM (SELECT business_id,product_id,warehouse_id FROM stock_movements GROUP BY business_id,product_id,warehouse_id HAVING SUM(quantity)<0 OR SUM(value)<0)").Scan(&count); e != nil {
		return e
	}
	if count != 0 {
		return errors.New("backup has negative stock or valuation")
	}
	return nil
}
func Create(db *sql.DB, dir string) (string, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	token := make([]byte, 8)
	if _, e := rand.Read(token); e != nil {
		return "", e
	}
	name := "Ledgerleaf-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(token) + ".db"
	path := filepath.Join(dir, name)
	if e := database.Export(db, path, ""); e != nil {
		os.Remove(path)
		return "", e
	}
	if e := Verify(path); e != nil {
		os.Remove(path)
		return "", e
	}
	f, e := os.OpenFile(path, os.O_RDWR, 0600)
	if e != nil {
		return "", e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return "", e
	}
	return name, nil
}

// Import stages an uploaded standalone database and publishes it only after validation.
func Import(input io.Reader, dir string) (string, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(dir, ".upload-*.db")
	if e != nil {
		return "", e
	}
	stage := f.Name()
	defer os.Remove(stage)
	_, e = io.Copy(f, input)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if e = Verify(stage); e != nil {
		return "", e
	}
	token := make([]byte, 8)
	if _, e = rand.Read(token); e != nil {
		return "", e
	}
	name := "Ledgerleaf-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(token) + ".db"
	if e = os.Rename(stage, filepath.Join(dir, name)); e != nil {
		return "", e
	}
	return name, nil
}
