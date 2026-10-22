package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type userSession struct {
	ID      int64
	Expires time.Time
}
type loginAttempt struct {
	Failures int
	Until    time.Time
}
type actorKey struct{}
type Actor struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	Restricted bool   `json:"restricted"`
}

func actorOf(r *http.Request) Actor { actor, _ := r.Context().Value(actorKey{}).(Actor); return actor }
func (s *Server) currentActor(r *http.Request) (Actor, bool) {
	cookie, e := r.Cookie("ledgerleaf_user")
	if e != nil {
		return Actor{}, false
	}
	s.authMu.Lock()
	session, ok := s.sessions[cookie.Value]
	if ok && time.Now().After(session.Expires) {
		delete(s.sessions, cookie.Value)
		ok = false
	}
	s.authMu.Unlock()
	if !ok {
		return Actor{}, false
	}
	var actor Actor
	e = s.DB.QueryRow("SELECT id,username,role,restricted FROM users WHERE id=? AND active=1", session.ID).Scan(&actor.ID, &actor.Username, &actor.Role, &actor.Restricted)
	return actor, e == nil
}
func (s *Server) loginCookie(w http.ResponseWriter, id int64) error {
	data := make([]byte, 32)
	if _, e := rand.Read(data); e != nil {
		return e
	}
	token := hex.EncodeToString(data)
	s.authMu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]userSession{}
	}
	for k, v := range s.sessions {
		if time.Now().After(v.Expires) {
			delete(s.sessions, k)
		}
	}
	s.sessions[token] = userSession{id, time.Now().Add(12 * time.Hour)}
	s.authMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ledgerleaf_user", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	return nil
}
func (s *Server) invalidateSessions(id int64) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	for token, session := range s.sessions {
		if id == 0 || id == session.ID {
			delete(s.sessions, token)
		}
	}
}

// Local-owner mode is available until an administrator enables password login.
// Once enabled, possession of the launch capability alone does not bypass login.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	var count int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); e != nil {
		fail(w, e)
		return r, false
	}
	actor := Actor{Username: "Local owner", Role: "ADMIN"}
	if count > 0 {
		var ok bool
		actor, ok = s.currentActor(r)
		if !ok {
			http.Error(w, "sign in to continue", 401)
			return r, false
		}
	}
	path := r.URL.Path
	if actor.Restricted && actor.Role != "ADMIN" && r.URL.Query().Has("business_id") {
		b, e := businessID(r)
		var allowed int
		if e != nil {
			fail(w, e)
			return r, false
		}
		if e = s.DB.QueryRow("SELECT COUNT(*) FROM user_businesses WHERE user_id=? AND business_id=?", actor.ID, b).Scan(&allowed); e != nil {
			fail(w, e)
			return r, false
		}
		if allowed != 1 {
			http.Error(w, "access to this business is not assigned", 403)
			return r, false
		}
	}
	if actor.Role != "ADMIN" {
		admin := path == "/api/database-excel" || path == "/api/backup-encrypt" || path == "/api/backup-decrypt" || path == "/api/diagnostics" || path == "/api/backup-policy" || path == "/api/period-lock" && r.Method != "GET" || path == "/api/auth/users" || path == "/api/backups" || path == "/api/backup-import" || path == "/api/backup-verify" || path == "/api/restore" || path == "/api/exit" || path == "/api/businesses" && r.Method != "GET"
		if admin {
			http.Error(w, "administrator permission required", 403)
			return r, false
		}
		if actor.Role == "VIEWER" && r.Method != "GET" && !strings.HasPrefix(path, "/api/auth/") {
			http.Error(w, "read-only account", 403)
			return r, false
		}
		if actor.Role == "BILLER" && !strings.HasPrefix(path, "/api/auth/") {
			allowed := map[string]bool{"/api/businesses": true, "/api/parties": true, "/api/media": true, "/api/units": true, "/api/products": true, "/api/warehouses": true, "/api/documents": true, "/api/finalize": true, "/api/banks": true, "/api/split-payments": true, "/api/payments": true, "/api/document-pdf": true}
			if !allowed[path] || (r.Method != "GET" && (path == "/api/media" || path == "/api/units" || path == "/api/banks" || path == "/api/products" || path == "/api/warehouses")) {
				http.Error(w, "billing permission does not allow this operation", 403)
				return r, false
			}
		}
	}
	return r.WithContext(context.WithValue(r.Context(), actorKey{}, actor)), true
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 72 }
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var n int
	if e := s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); e != nil {
		fail(w, e)
		return
	}
	actor, ok := s.currentActor(r)
	if n == 0 {
		actor = Actor{Username: "Local owner", Role: "ADMIN"}
		ok = true
	}
	respond(w, 200, map[string]interface{}{"enabled": n > 0, "authenticated": ok, "user": actor})
}
func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	name := strings.ToLower(strings.TrimSpace(p.Username))
	s.authMu.Lock()
	attempt := s.attempts[name]
	s.authMu.Unlock()
	if time.Now().Before(attempt.Until) {
		http.Error(w, "too many attempts; try again in one minute", 429)
		return
	}
	var id int64
	var hash string
	e := s.DB.QueryRow("SELECT id,password_hash FROM users WHERE username=? AND active=1", name).Scan(&id, &hash)
	if e != nil || len(p.Password) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(p.Password)) != nil {
		s.authMu.Lock()
		if s.attempts == nil {
			s.attempts = map[string]loginAttempt{}
		}
		attempt.Failures++
		if attempt.Failures >= 5 {
			attempt.Until = time.Now().Add(time.Minute)
			attempt.Failures = 0
		}
		s.attempts[name] = attempt
		s.authMu.Unlock()
		http.Error(w, "invalid username or password", 401)
		return
	}
	s.authMu.Lock()
	delete(s.attempts, name)
	s.authMu.Unlock()
	if e = s.loginCookie(w, id); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) authUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s.query(w, "SELECT id,username,role,active,restricted,COALESCE((SELECT json_group_array(business_id) FROM user_businesses WHERE user_id=users.id),'[]') AS business_ids FROM users ORDER BY username")
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		ID          int64   `json:"id"`
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		Role        string  `json:"role"`
		Active      int     `json:"active"`
		Restricted  bool    `json:"restricted"`
		BusinessIDs []int64 `json:"business_ids"`
	}
	if e := decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	p.Username = strings.ToLower(strings.TrimSpace(p.Username))
	if p.Restricted && p.Role == "ADMIN" || len(p.BusinessIDs) > 200 {
		fail(w, errors.New("administrators need installation-wide access; at most 200 business assignments are allowed"))
		return
	}
	roles := map[string]bool{"ADMIN": true, "ACCOUNTANT": true, "BILLER": true, "VIEWER": true}
	if !usernamePattern.MatchString(p.Username) || !roles[p.Role] || (p.Active != 0 && p.Active != 1) || (p.ID == 0 || p.Password != "") && !validPassword(p.Password) {
		fail(w, errors.New("use a valid username, role and a password of 12–72 bytes"))
		return
	}
	hash := ""
	if p.Password != "" {
		bytes, e := bcrypt.GenerateFromPassword([]byte(p.Password), 12)
		if e != nil {
			fail(w, e)
			return
		}
		hash = string(bytes)
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); e != nil {
		fail(w, e)
		return
	}
	if count == 0 && (p.ID != 0 || p.Role != "ADMIN" || p.Active != 1) {
		fail(w, errors.New("the first user must be an active administrator"))
		return
	}
	if count > 0 && actorOf(r).ID == 0 {
		http.Error(w, "sign in as administrator to manage users", 403)
		return
	}
	id := p.ID
	if id == 0 {
		res, e := tx.Exec("INSERT INTO users(username,password_hash,role,active) VALUES(?,?,?,?)", p.Username, hash, p.Role, p.Active)
		if e != nil {
			fail(w, e)
			return
		}
		id, _ = res.LastInsertId()
	} else {
		var oldRole string
		var active int
		if e = tx.QueryRow("SELECT role,active FROM users WHERE id=?", id).Scan(&oldRole, &active); e != nil {
			fail(w, e)
			return
		}
		if oldRole == "ADMIN" && active == 1 && (p.Role != "ADMIN" || p.Active == 0) {
			var admins int
			if e = tx.QueryRow("SELECT COUNT(*) FROM users WHERE role='ADMIN' AND active=1").Scan(&admins); e != nil {
				fail(w, e)
				return
			}
			if admins <= 1 {
				fail(w, errors.New("at least one active administrator is required"))
				return
			}
		}
		if _, e = tx.Exec("UPDATE users SET username=?,role=?,active=?,password_hash=CASE WHEN ?='' THEN password_hash ELSE ? END WHERE id=?", p.Username, p.Role, p.Active, hash, hash, id); e != nil {
			fail(w, e)
			return
		}
	}
	if _, e = tx.Exec("UPDATE users SET restricted=? WHERE id=?", p.Restricted, id); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("DELETE FROM user_businesses WHERE user_id=?", id); e != nil {
		fail(w, e)
		return
	}
	if p.Restricted {
		for _, b := range p.BusinessIDs {
			if _, e = tx.Exec("INSERT INTO user_businesses(user_id,business_id) VALUES(?,?)", id, b); e != nil {
				fail(w, e)
				return
			}
		}
	}
	if _, e = tx.Exec("INSERT INTO audit_log(action,entity_id,actor_id) VALUES('user.saved',?,?)", id, nullableActor(actorOf(r).ID)); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	s.invalidateSessions(id)
	if count == 0 {
		if e = s.loginCookie(w, id); e != nil {
			fail(w, e)
			return
		}
	}
	respond(w, 200, map[string]int64{"id": id})
}
func nullableActor(id int64) interface{} {
	if id == 0 {
		return nil
	}
	return id
}
func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	cookie, e := r.Cookie("ledgerleaf_user")
	if e == nil {
		s.authMu.Lock()
		delete(s.sessions, cookie.Value)
		s.authMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "ledgerleaf_user", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) authPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var p struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &p); e != nil {
		fail(w, e)
		return
	}
	id := actorOf(r).ID
	if id == 0 || !validPassword(p.Password) {
		fail(w, errors.New("sign in and use a password of 12–72 bytes"))
		return
	}
	var hash string
	if e := s.DB.QueryRow("SELECT password_hash FROM users WHERE id=?", id).Scan(&hash); e != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(p.Current)) != nil {
		http.Error(w, "current password is incorrect", 403)
		return
	}
	newHash, e := bcrypt.GenerateFromPassword([]byte(p.Password), 12)
	if e != nil {
		fail(w, e)
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, e)
		return
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", string(newHash), id); e != nil {
		fail(w, e)
		return
	}
	if _, e = tx.Exec("INSERT INTO audit_log(action,entity_id,actor_id) VALUES('password.changed',?,?)", id, id); e != nil {
		fail(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, e)
		return
	}
	s.invalidateSessions(id)
	if e = s.loginCookie(w, id); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
