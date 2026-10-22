package server

import (
	"context"
	"net/http"
	"runtime"
	"time"
)

type diagnosticCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Issues int64  `json:"issues"`
	Detail string `json:"detail"`
}

// Run on explicit request under the exclusive request lock. The report contains
// counts only, never customer records, credentials, filesystem paths or tokens.
func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	checks := []diagnosticCheck{}
	overall := "pass"
	add := func(name string, n int64, err error) {
		c := diagnosticCheck{Name: name, Issues: n, Status: "pass", Detail: "No issues found"}
		if err != nil {
			c.Status = "error"
			c.Detail = "Check could not complete; retry when the application is idle. If it persists, preserve the data directory and use a verified backup."
		} else if n != 0 {
			c.Status = "fail"
			c.Detail = "Issues require investigation; preserve the data directory before recovery."
		}
		if c.Status != "pass" {
			overall = "attention"
		}
		checks = append(checks, c)
	}
	var schema int
	err := s.DB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schema)
	add("Read schema version", 0, err)
	rows, err := s.DB.QueryContext(ctx, "PRAGMA integrity_check(100)")
	var issues int64
	if err == nil {
		for rows.Next() {
			var result string
			if err = rows.Scan(&result); err != nil {
				break
			}
			if result != "ok" {
				issues++
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	add("SQLite structural integrity", issues, err)
	rows, err = s.DB.QueryContext(ctx, "PRAGMA foreign_key_check")
	issues = 0
	if err == nil {
		for rows.Next() {
			issues++
		}
		err = rows.Err()
		rows.Close()
	}
	add("Foreign key references", issues, err)
	for _, q := range [][2]string{
		{"Balanced journal entries", `SELECT COUNT(*) FROM journals j LEFT JOIN (SELECT business_id,journal_id,SUM(debit) AS debit,SUM(credit) AS credit FROM journal_lines GROUP BY business_id,journal_id) l ON l.business_id=j.business_id AND l.journal_id=j.id WHERE COALESCE(l.debit,0)!=COALESCE(l.credit,0) OR COALESCE(l.debit,0)=0`},
		{"Nonnegative stock balances", `SELECT COUNT(*) FROM (SELECT business_id,product_id,warehouse_id FROM stock_movements GROUP BY business_id,product_id,warehouse_id HAVING SUM(quantity)<0 OR SUM(value)<0 OR (SUM(quantity)=0 AND SUM(value)!=0))`},
		{"Inventory ledger reconciliation", `SELECT COUNT(*) FROM businesses b WHERE COALESCE((SELECT SUM(value) FROM stock_movements WHERE business_id=b.id),0)!=COALESCE((SELECT SUM(debit-credit) FROM journal_lines WHERE business_id=b.id AND account='1200'),0)`},
		{"Dispatch ledger reconciliation", `SELECT COUNT(*) FROM businesses b WHERE COALESCE((SELECT SUM(value) FROM dispatches WHERE business_id=b.id),0)-COALESCE((SELECT SUM(value) FROM dispatch_allocations WHERE business_id=b.id),0)!=COALESCE((SELECT SUM(debit-credit) FROM journal_lines WHERE business_id=b.id AND account='1250'),0)`},
		{"Dispatch allocation limits", `SELECT COUNT(*) FROM (SELECT d.business_id,d.id,json_extract(l.value,'$.product_id') AS product_id,json_extract(l.value,'$.quantity')*COALESCE(NULLIF(json_extract(l.value,'$.unit_factor'),0),1000)/1000 AS quantity,json_extract(l.value,'$.cost') AS value,COALESCE((SELECT SUM(a.quantity) FROM dispatch_allocations a WHERE a.business_id=d.business_id AND a.dispatch_id=d.id AND a.product_id=json_extract(l.value,'$.product_id')),0) AS used,COALESCE((SELECT SUM(a.value) FROM dispatch_allocations a WHERE a.business_id=d.business_id AND a.dispatch_id=d.id AND a.product_id=json_extract(l.value,'$.product_id')),0) AS used_value FROM dispatches d,json_each(d.snapshot,'$.lines') l) WHERE used>quantity OR used_value>value OR (used=quantity AND used_value!=value)`},
		{"Invoice snapshot totals", `SELECT COUNT(*) FROM documents WHERE CASE WHEN json_valid(snapshot) THEN COALESCE(json_extract(snapshot,'$.total'),-1)!=total ELSE 1 END`},
		{"Credit note snapshot totals", `SELECT COUNT(*) FROM credit_notes WHERE CASE WHEN json_valid(snapshot) THEN COALESCE(json_extract(snapshot,'$.total'),-1)!=total ELSE 1 END`},
		{"Receipt snapshot values", `SELECT COUNT(*) FROM goods_receipts WHERE CASE WHEN json_valid(snapshot) THEN COALESCE(json_extract(snapshot,'$.total'),-1)!=value ELSE 1 END`},
	} {
		issues = 0
		err = s.DB.QueryRowContext(ctx, q[1]).Scan(&issues)
		add(q[0], issues, err)
	}
	respond(w, 200, map[string]interface{}{"status": overall, "checked_at": time.Now().UTC().Format(time.RFC3339), "schema": schema, "platform": runtime.GOOS + "/" + runtime.GOARCH, "checks": checks})
}
