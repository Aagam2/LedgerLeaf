package server

import (
	"net/http"
	"time"
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	b, e := businessID(r)
	if e != nil {
		fail(w, e)
		return
	}
	if r.URL.Query().Get("series") == "monthly" {
		from := time.Now().AddDate(0, -11, 0).Format("2006-01") + "-01"
		s.query(w, `SELECT substr(j.date,1,7) AS month,SUM(l.credit-l.debit) AS revenue FROM journals j JOIN journal_lines l ON l.business_id=j.business_id AND l.journal_id=j.id WHERE j.business_id=? AND l.account='4000' AND j.reference NOT LIKE 'year-close:%' AND j.date>=? AND j.date<=? GROUP BY substr(j.date,1,7) ORDER BY month`, b, from, time.Now().Format("2006-01-02"))
		return
	}
	s.query(w, `SELECT a.code,a.name,a.kind,COALESCE(SUM(l.debit-l.credit),0) AS balance FROM accounts a LEFT JOIN journal_lines l ON l.business_id=a.business_id AND l.account=a.code WHERE a.business_id=? AND a.code IN ('1000','1010','1100','1200','2000') GROUP BY a.code ORDER BY a.code`, b)
}
