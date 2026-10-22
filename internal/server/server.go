package server

import (
	"billbook/internal/accounting"
	"billbook/internal/gst"
	"billbook/internal/web"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	// Set by the launcher for keyed installations; never exposed by an API.
	OpenDatabase   func(string) (*sql.DB, error)
	PrepareRestore func(string) error

	DB        *sql.DB
	DataDir   string
	requestMu sync.RWMutex
	authMu    sync.Mutex
	sessions  map[string]userSession
	attempts  map[string]loginAttempt
	Host      string
	Token     string
	Exit      func()
	assets    http.Handler
}

func New(db *sql.DB, host string, exit func()) (*Server, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	f, e := fs.Sub(web.Files, "dist")
	if e != nil {
		return nil, e
	}
	return &Server{DB: db, Host: host, Token: hex.EncodeToString(b), Exit: exit, assets: http.FileServer(http.FS(f))}, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if r.Host != s.Host {
		http.Error(w, "invalid host", 403)
		return
	}
	if r.URL.Path == "/api/session" && r.Method == "POST" {
		if r.Header.Get("Origin") != "http://"+s.Host {
			http.Error(w, "invalid origin", 403)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		if decode(w, r, &body) != nil || !equal(body.Token, s.Token) {
			http.Error(w, "invalid launch token", 401)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "ledgerleaf", Value: s.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
		cookie, e := r.Cookie("ledgerleaf")
		if e != nil || !equal(cookie.Value, s.Token) {
			http.Error(w, "open the application using its launch link", 401)
			return
		}
		if r.Method != "GET" && (r.Header.Get("Origin") != "http://"+s.Host || r.Header.Get("X-Ledgerleaf") != "1") {
			http.Error(w, "invalid request origin", 403)
			return
		}
		if r.URL.Path == "/api/restore" || r.URL.Path == "/api/diagnostics" {
			s.requestMu.Lock()
			defer s.requestMu.Unlock()
		} else {
			s.requestMu.RLock()
			defer s.requestMu.RUnlock()
		}
		switch r.URL.Path {
		case "/api/auth/status":
			s.authStatus(w, r)
			return
		case "/api/auth/login":
			s.authLogin(w, r)
			return
		}
		var allowed bool
		r, allowed = s.authorize(w, r)
		if !allowed {
			return
		}
		switch r.URL.Path {
		case "/api/parties", "/api/products", "/api/warehouses", "/api/stock", "/api/movements", "/api/trial-balance", "/api/journals":
			s.masters(w, r)
		case "/api/opening-stock":
			s.openingStock(w, r)
		case "/api/document-pdf", "/api/credit-note-pdf":
			s.pdf(w, r)
		case "/api/documents":
			s.documents(w, r)
		case "/api/finalize":
			s.finalize(w, r)
		case "/api/accounts":
			s.accounts(w, r)
		case "/api/manual-journal":
			s.manualJournal(w, r)
		case "/api/journal-reversal":
			s.reverseJournal(w, r)
		case "/api/auth/users":
			s.authUsers(w, r)
		case "/api/auth/logout":
			s.authLogout(w, r)
		case "/api/auth/password":
			s.authPassword(w, r)
		case "/api/stock-operation":
			s.stockOperation(w, r)
		case "/api/expense-attachments":
			s.expenseAttachments(w, r)
		case "/api/recurring-expenses", "/api/recurring-expense-post":
			s.recurringExpenses(w, r)
		case "/api/expenses":
			s.expenses(w, r)
		case "/api/report-summary":
			s.reportSummary(w, r)
		case "/api/receipt-reversal":
			s.reverseReceipt(w, r)
		case "/api/year-close":
			s.closeYear(w, r)
		case "/api/dispatches":
			s.dispatches(w, r)
		case "/api/dispatch-return":
			s.returnDispatch(w, r)
		case "/api/business-settings":
			s.businessSettings(w, r)
		case "/api/variants":
			s.variants(w, r)
		case "/api/receipts":
			s.receipts(w, r)
		case "/api/media":
			s.media(w, r)
		case "/api/units":
			s.units(w, r)
		case "/api/prices":
			s.prices(w, r)
		case "/api/period-lock":
			s.periodLock(w, r)
		case "/api/banks":
			s.banks(w, r)
		case "/api/bank-reconciliation":
			s.reconcile(w, r)
		case "/api/party-finance":
			s.partyFinance(w, r)
		case "/api/import-documents":
			s.importDocuments(w, r)
		case "/api/import-journals":
			s.importJournals(w, r)
		case "/api/database-excel":
			s.databaseExcel(w, r)
		case "/api/import-template":
			s.importTemplate(w, r)
		case "/api/import-masters":
			s.importMasters(w, r)
		case "/api/dashboard":
			s.dashboard(w, r)
		case "/api/reports":
			s.reports(w, r)
		case "/api/commercial-documents":
			s.commercial(w, r)
		case "/api/commercial-issue":
			s.issueCommercial(w, r)
		case "/api/commercial-pdf":
			s.commercialPDF(w, r)
		case "/api/taxes":
			s.taxes(w, r)
		case "/api/diagnostics":
			s.diagnostics(w, r)
		case "/api/backup-encrypt", "/api/backup-decrypt":
			s.encryptedBackup(w, r)
		case "/api/backups":
			s.backups(w, r)
		case "/api/backup-policy":
			s.backupSettings(w, r)
		case "/api/backup-import":
			s.importBackup(w, r)
		case "/api/backup-verify":
			s.verifyBackup(w, r)
		case "/api/restore":
			s.restoreBackup(w, r)
		case "/api/reversals":
			s.reverseDocument(w, r)
		case "/api/split-payments":
			s.splitPayments(w, r)
		case "/api/payments":
			s.payments(w, r)
		case "/api/businesses":
			s.businesses(w, r)
		case "/api/exit":
			if r.Method != "POST" {
				http.Error(w, "method not allowed", 405)
				return
			}
			respond(w, 200, map[string]bool{"ok": true})
			go func() { time.Sleep(100 * time.Millisecond); s.Exit() }()
		default:
			http.NotFound(w, r)
		}
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "method not allowed", 405)
		return
	}
	s.assets.ServeHTTP(w, r)
}
func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func decode(w http.ResponseWriter, r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(new(interface{})); !errors.Is(e, io.EOF) {
		return errors.New("one JSON object required")
	}
	return nil
}
func respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) { respond(w, 400, map[string]string{"error": e.Error()}) }

type Business struct {
	TaxScheme     string `json:"tax_scheme"`
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LegalName     string `json:"legal_name"`
	GSTIN         string `json:"gstin"`
	StateCode     string `json:"state_code"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	PAN           string `json:"pan"`
	BankName      string `json:"bank_name"`
	BankAccount   string `json:"bank_account"`
	BankIFSC      string `json:"bank_ifsc"`
	InvoiceTerms  string `json:"invoice_terms"`
	InvoiceFooter string `json:"invoice_footer"`
}

func (s *Server) businesses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		actor := actorOf(r)
		q, limit, offset := pageArgs(r)
		if r.URL.Query().Get("paged") != "1" {
			limit = 200
			offset = 0
		}
		rows, e := s.DB.Query("SELECT id,name,legal_name,gstin,state_code,address,phone,email,pan,bank_name,bank_account,bank_ifsc,invoice_terms,invoice_footer,tax_scheme FROM businesses WHERE (?=0 OR id IN (SELECT business_id FROM user_businesses WHERE user_id=?)) AND (name LIKE ? OR legal_name LIKE ?) ORDER BY id LIMIT ? OFFSET ?", actor.Restricted && actor.Role != "ADMIN", actor.ID, q, q, limit, offset)
		if e != nil {
			fail(w, e)
			return
		}
		defer rows.Close()
		out := []Business{}
		for rows.Next() {
			var b Business
			if e = rows.Scan(&b.ID, &b.Name, &b.LegalName, &b.GSTIN, &b.StateCode, &b.Address, &b.Phone, &b.Email, &b.PAN, &b.BankName, &b.BankAccount, &b.BankIFSC, &b.InvoiceTerms, &b.InvoiceFooter, &b.TaxScheme); e != nil {
				fail(w, e)
				return
			}
			out = append(out, b)
		}
		if e = rows.Err(); e != nil {
			fail(w, e)
			return
		}
		respond(w, 200, out)
	case "POST":
		var b Business
		if e := decode(w, r, &b); e != nil {
			fail(w, e)
			return
		}
		b.Name = strings.TrimSpace(b.Name)
		state, stateErr := strconv.Atoi(b.StateCode)
		if stateErr != nil || state < 1 || state > 38 || b.Name == "" || len(b.Name) > 160 || len(b.StateCode) != 2 || b.StateCode < "01" || b.StateCode > "38" || len(b.LegalName) > 160 || len(b.Address) > 1000 {
			fail(w, errors.New("business name and a valid two-digit state code are required"))
			return
		}
		if len(b.Phone) > 40 || len(b.Email) > 254 || len(b.PAN) > 10 || len(b.BankName) > 160 || len(b.BankAccount) > 40 || len(b.BankIFSC) > 11 || len(b.InvoiceTerms) > 4000 || len(b.InvoiceFooter) > 1000 {
			fail(w, errors.New("business profile field is too long"))
			return
		}
		b.GSTIN = strings.ToUpper(strings.TrimSpace(b.GSTIN))
		b.PAN = strings.ToUpper(strings.TrimSpace(b.PAN))
		if b.TaxScheme == "" {
			b.TaxScheme = "UNREGISTERED"
		}
		if (b.PAN != "" && !gst.PAN(b.PAN)) || (b.GSTIN != "" && (!gst.GSTIN(b.GSTIN) || b.GSTIN[:2] != b.StateCode || b.PAN != "" && b.GSTIN[2:12] != b.PAN)) || (b.TaxScheme != "UNREGISTERED" && b.TaxScheme != "REGULAR" && b.TaxScheme != "COMPOSITION") || ((b.TaxScheme == "UNREGISTERED") != (b.GSTIN == "")) {
			fail(w, errors.New("valid matching GSTIN, PAN, state and registration scheme required"))
			return
		}
		tx, e := s.DB.Begin()
		if e != nil {
			fail(w, e)
			return
		}
		defer tx.Rollback()
		if b.ID != 0 {
			res, e := tx.Exec("UPDATE businesses SET name=?,legal_name=?,gstin=?,tax_scheme=?,state_code=?,address=?,phone=?,email=?,pan=?,bank_name=?,bank_account=?,bank_ifsc=?,invoice_terms=?,invoice_footer=? WHERE id=?", b.Name, b.LegalName, b.GSTIN, b.TaxScheme, b.StateCode, b.Address, b.Phone, b.Email, b.PAN, b.BankName, b.BankAccount, b.BankIFSC, b.InvoiceTerms, b.InvoiceFooter, b.ID)
			if e != nil {
				fail(w, e)
				return
			}
			n, e := res.RowsAffected()
			if e != nil || n != 1 {
				fail(w, errors.New("business not found"))
				return
			}
			if e = auditFor(r, tx, b.ID, "business.updated", b.ID); e != nil {
				fail(w, e)
				return
			}
			if e = tx.Commit(); e != nil {
				fail(w, e)
				return
			}
			respond(w, 200, b)
			return
		}
		res, e := tx.Exec("INSERT INTO businesses(name,legal_name,gstin,tax_scheme,state_code,address) VALUES(?,?,?,?,?,?)", b.Name, b.LegalName, b.GSTIN, b.TaxScheme, b.StateCode, b.Address)
		if e != nil {
			fail(w, e)
			return
		}
		b.ID, e = res.LastInsertId()
		if e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("UPDATE businesses SET phone=?,email=?,pan=?,bank_name=?,bank_account=?,bank_ifsc=?,invoice_terms=?,invoice_footer=? WHERE id=?", b.Phone, b.Email, b.PAN, b.BankName, b.BankAccount, b.BankIFSC, b.InvoiceTerms, b.InvoiceFooter, b.ID); e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO audit_log(business_id,action,entity_id,actor_id) VALUES(?,'business.created',?,?)", b.ID, b.ID, nullableActor(actorOf(r).ID)); e != nil {
			fail(w, e)
			return
		}
		if _, e = tx.Exec("INSERT INTO warehouses(business_id,name) VALUES(?,'Main warehouse')", b.ID); e != nil {
			fail(w, e)
			return
		}
		if e = accounting.Seed(tx, b.ID); e != nil {
			fail(w, e)
			return
		}
		if e = tx.Commit(); e != nil {
			fail(w, e)
			return
		}
		respond(w, 201, b)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
