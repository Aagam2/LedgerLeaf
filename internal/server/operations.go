package server

import (
	"billbook/internal/accounting"
	"billbook/internal/money"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) stockOperation(w http.ResponseWriter, r *http.Request) {
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
		TargetID    int64  `json:"target_id"`
		Kind        string `json:"kind"`
		Quantity    int64  `json:"quantity"`
		Value       int64  `json:"value"`
		Date        string `json:"date"`
		Reason      string `json:"reason"`
		Key         string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if p.Quantity <= 0 || p.Quantity > 1000000000 || p.Value < 0 || p.Value > 1000000000000 || !dateValid(p.Date) || !validKey(p.Key) || strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 1000 || (p.Kind != "ADD" && p.Kind != "REMOVE" && p.Kind != "DAMAGE" && p.Kind != "TRANSFER") {
		fail(w, errors.New("invalid stock operation"))
		return
	}
	if p.Kind == "TRANSFER" && p.TargetID == p.WarehouseID {
		fail(w, errors.New("select a different destination warehouse"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "stock:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	var kind string
	if e = tx.QueryRow("SELECT kind FROM products WHERE business_id=? AND id=? AND active=1", b, p.ProductID).Scan(&kind); e != nil || kind != "PRODUCT" {
		fail(w, errors.New("select an active stock product"))
		return
	}
	var qty, value int64
	var last sql.NullString
	if e = tx.QueryRow("SELECT COALESCE(SUM(quantity),0),COALESCE(SUM(value),0),MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, p.ProductID, p.WarehouseID).Scan(&qty, &value, &last); e != nil {
		fail(w, e)
		return
	}
	if last.Valid && p.Date < last.String {
		fail(w, errors.New("stock operation cannot predate existing movements"))
		return
	}
	delta, cost := p.Quantity, p.Value
	if p.Kind != "ADD" {
		if qty < p.Quantity {
			fail(w, errors.New("insufficient stock"))
			return
		}
		cost, e = money.MulDiv(value, p.Quantity, qty)
		if e != nil {
			fail(w, e)
			return
		}
		delta = -p.Quantity
	}
	valueChange := cost
	if delta < 0 {
		valueChange = -cost
	}
	res, e := tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date,remarks) VALUES(?,?,?,?,?,?,?,?,?)", b, p.ProductID, p.WarehouseID, delta, valueChange, p.Kind, ref, p.Date, p.Reason)
	if e != nil {
		fail(w, e)
		return
	}
	id, _ := res.LastInsertId()
	if p.Kind == "TRANSFER" {
		if e = tx.QueryRow("SELECT MAX(date) FROM stock_movements WHERE business_id=? AND product_id=? AND warehouse_id=?", b, p.ProductID, p.TargetID).Scan(&last); e != nil {
			fail(w, e)
			return
		}
		if last.Valid && p.Date < last.String {
			fail(w, errors.New("transfer cannot predate destination movements"))
			return
		}
		if _, e = tx.Exec("INSERT INTO stock_movements(business_id,product_id,warehouse_id,quantity,value,kind,reference,date,remarks) VALUES(?,?,?,?,?,'TRANSFER',?,?,?)", b, p.ProductID, p.TargetID, p.Quantity, cost, ref, p.Date, p.Reason); e != nil {
			fail(w, e)
			return
		}
	} else {
		lines := []accounting.Line{{Account: "5100", Debit: cost}, {Account: "1200", Credit: cost}}
		if p.Kind == "ADD" {
			lines = []accounting.Line{{Account: "1200", Debit: cost}, {Account: "5100", Credit: cost}}
		}
		if e = accounting.Post(tx, b, p.Date, ref, "Stock "+p.Kind+": "+p.Reason, lines); e != nil {
			fail(w, e)
			return
		}
	}
	if e = auditFor(r, tx, b, "stock."+strings.ToLower(p.Kind), id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id, "cost": cost})
}

func (s *Server) expenses(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		s.query(w, `SELECT j.id,j.date,j.description,l.debit AS amount,COALESCE((SELECT category FROM expense_details d WHERE d.business_id=j.business_id AND d.journal_id=j.id),'') AS category,CASE WHEN EXISTS(SELECT 1 FROM journals r WHERE r.business_id=j.business_id AND r.reference='manual-reversal:'||j.id) THEN 'REVERSED' ELSE 'POSTED' END AS status FROM journals j JOIN journal_lines l ON l.business_id=j.business_id AND l.journal_id=j.id WHERE j.business_id=? AND j.reference LIKE 'expense:%' AND l.account='5200' ORDER BY j.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Category    string `json:"category"`
		Date        string `json:"date"`
		Description string `json:"description"`
		Amount      int64  `json:"amount"`
		Account     string `json:"account"`
		Key         string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if len(p.Category) > 100 || !dateValid(p.Date) || !validKey(p.Key) || strings.TrimSpace(p.Description) == "" || len(p.Description) > 1000 || p.Amount <= 0 || p.Amount > 1000000000000 || p.Account == "" {
		fail(w, errors.New("provide date, description, positive amount and cash/bank account"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	mode := "BANK_TRANSFER"
	if p.Account == "1000" {
		mode = "CASH"
	}
	if _, e = paymentAccount(tx, b, mode, p.Account); e != nil {
		fail(w, e)
		return
	}

	raw, _ := json.Marshal(p)
	ref := "expense:" + p.Key
	repeat, e := claim(tx, b, ref, string(raw))
	if e != nil {
		fail(w, e)
		return
	}
	if repeat {
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, p.Description, []accounting.Line{{Account: "5200", Debit: p.Amount}, {Account: p.Account, Credit: p.Amount}}); e != nil {
		fail(w, e)
		return
	}
	var id int64
	if e = tx.QueryRow("SELECT id FROM journals WHERE business_id=? AND reference=?", b, ref).Scan(&id); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO expense_details(business_id,journal_id,category) VALUES(?,?,?)", b, id, strings.TrimSpace(p.Category)); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "expense.posted", id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]int64{"id": id})
}
