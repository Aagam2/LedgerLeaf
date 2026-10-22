package server

import (
	"billbook/internal/accounting"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

func (s *Server) partyFinance(w http.ResponseWriter, r *http.Request) {
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.Method == "GET" {
		_, limit, offset := pageArgs(r)
		if r.URL.Query().Get("kind") == "openings" {
			s.query(w, `SELECT o.party_id,p.name,p.kind,o.date,o.amount,o.amount-COALESCE((SELECT SUM(amount) FROM opening_settlements x WHERE x.business_id=o.business_id AND x.party_id=o.party_id),0) AS remaining FROM party_openings o JOIN parties p ON p.business_id=o.business_id AND p.id=o.party_id WHERE o.business_id=? ORDER BY o.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		} else {
			s.query(w, `SELECT a.id,a.party_id,p.name,p.kind,a.date,a.amount,a.mode,a.amount-COALESCE((SELECT SUM(amount) FROM advance_usage u WHERE u.business_id=a.business_id AND u.advance_id=a.id),0) AS remaining FROM party_advances a JOIN parties p ON p.business_id=a.business_id AND p.id=a.party_id WHERE a.business_id=? ORDER BY a.id DESC LIMIT ? OFFSET ?`, b, limit, offset)
		}
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Action     string `json:"action"`
		PartyID    int64  `json:"party_id"`
		AdvanceID  int64  `json:"advance_id"`
		DocumentID int64  `json:"document_id"`
		Date       string `json:"date"`
		Amount     int64  `json:"amount"`
		Mode       string `json:"mode"`
		Key        string `json:"key"`
	}
	if e = decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	if !dateValid(p.Date) || !validKey(p.Key) || p.Amount <= 0 || p.Amount > 1000000000000 || (p.Mode != "CASH" && p.Mode != "BANK") {
		fail(w, errors.New("valid amount, date, account and key required"))
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	raw, _ := json.Marshal(p)
	ref := "party-finance:" + p.Key
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
	if e = tx.QueryRow("SELECT kind FROM parties WHERE business_id=? AND id=?", b, p.PartyID).Scan(&kind); e != nil {
		fail(w, e)
		return
	}
	cash, control, advance := "1000", "1100", "2200"
	if p.Mode == "BANK" {
		cash = "1010"
	}
	if kind == "SUPPLIER" {
		control = "2000"
		advance = "1400"
	}
	var lines []accounting.Line
	switch p.Action {
	case "OPENING":
		var count int
		if e = tx.QueryRow("SELECT COUNT(*) FROM documents WHERE business_id=? AND party_id=? AND status='FINALIZED'", b, p.PartyID).Scan(&count); e != nil {
			fail(w, e)
			return
		}
		if count > 0 {
			fail(w, errors.New("enter opening balance before finalized transactions"))
			return
		}
		if _, e = tx.Exec("INSERT INTO party_openings(business_id,party_id,date,amount,reference) VALUES(?,?,?,?,?)", b, p.PartyID, p.Date, p.Amount, ref); e != nil {
			fail(w, e)
			return
		}
		lines = []accounting.Line{{Account: control, Debit: p.Amount}, {Account: "3000", Credit: p.Amount}}
	case "SETTLE_OPENING":
		var outstanding int64
		var date string
		if e = tx.QueryRow("SELECT o.amount-COALESCE((SELECT SUM(amount) FROM opening_settlements x WHERE x.business_id=o.business_id AND x.party_id=o.party_id),0),o.date FROM party_openings o WHERE business_id=? AND party_id=?", b, p.PartyID).Scan(&outstanding, &date); e != nil || p.Amount > outstanding || p.Date < date {
			fail(w, errors.New("amount exceeds opening balance or date precedes it"))
			return
		}
		if _, e = tx.Exec("INSERT INTO opening_settlements(business_id,party_id,date,amount,reference) VALUES(?,?,?,?,?)", b, p.PartyID, p.Date, p.Amount, ref); e != nil {
			fail(w, e)
			return
		}
		lines = []accounting.Line{{Account: cash, Debit: p.Amount}, {Account: control, Credit: p.Amount}}
	case "ADVANCE":
		if _, e = tx.Exec("INSERT INTO party_advances(business_id,party_id,date,amount,mode,reference) VALUES(?,?,?,?,?,?)", b, p.PartyID, p.Date, p.Amount, p.Mode, ref); e != nil {
			fail(w, e)
			return
		}
		lines = []accounting.Line{{Account: cash, Debit: p.Amount}, {Account: advance, Credit: p.Amount}}
	case "ALLOCATE", "REFUND":
		var remaining int64
		var date string
		var latest sql.NullString
		if e = tx.QueryRow("SELECT a.amount-COALESCE((SELECT SUM(amount) FROM advance_usage u WHERE u.business_id=a.business_id AND u.advance_id=a.id),0),a.date,(SELECT MAX(date) FROM advance_usage u WHERE u.business_id=a.business_id AND u.advance_id=a.id) FROM party_advances a WHERE a.business_id=? AND a.id=? AND a.party_id=?", b, p.AdvanceID, p.PartyID).Scan(&remaining, &date, &latest); e != nil || p.Amount > remaining || p.Date < date || (latest.Valid && p.Date < latest.String) {
			fail(w, errors.New("invalid advance, amount or chronological date"))
			return
		}
		var document interface{}
		if p.Action == "ALLOCATE" {
			var documentParty, outstanding int64
			var status, documentDate string
			if e = tx.QueryRow("SELECT d.party_id,d.status,d.date,d.total-x.paid-x.returned+x.refunded FROM documents d JOIN document_settlements x ON x.business_id=d.business_id AND x.id=d.id WHERE d.business_id=? AND d.id=?", b, p.DocumentID).Scan(&documentParty, &status, &documentDate, &outstanding); e != nil || status != "FINALIZED" || documentParty != p.PartyID || p.Amount > outstanding || p.Date < documentDate {
				fail(w, errors.New("select an outstanding finalized invoice for the same party"))
				return
			}
			document = p.DocumentID
			if _, e = tx.Exec("INSERT INTO payments(business_id,document_id,amount,date,mode,reference) VALUES(?,?,?,?,'OTHER',?)", b, p.DocumentID, p.Amount, p.Date, ref); e != nil {
				fail(w, e)
				return
			}
			lines = []accounting.Line{{Account: advance, Debit: p.Amount}, {Account: control, Credit: p.Amount}}
		} else {
			lines = []accounting.Line{{Account: advance, Debit: p.Amount}, {Account: cash, Credit: p.Amount}}
		}
		if _, e = tx.Exec("INSERT INTO advance_usage(business_id,advance_id,document_id,date,amount,kind,reference) VALUES(?,?,?,?,?,?,?)", b, p.AdvanceID, document, p.Date, p.Amount, p.Action, ref); e != nil {
			fail(w, e)
			return
		}
	default:
		fail(w, errors.New("unknown party finance action"))
		return
	}
	if kind == "SUPPLIER" {
		for i := range lines {
			lines[i].Debit, lines[i].Credit = lines[i].Credit, lines[i].Debit
		}
	}
	if e = accounting.Seed(tx, b); e != nil {
		fail(w, e)
		return
	}
	if e = accounting.Post(tx, b, p.Date, ref, "Party "+p.Action, lines); e != nil {
		fail(w, e)
		return
	}
	if e = auditFor(r, tx, b, "party."+p.Action, p.PartyID); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
