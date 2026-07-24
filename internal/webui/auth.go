package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/neutrospec/canopy/internal/config"
)

// Auth protects non-loopback serving (docs/web-ui-plan-2.md D1/D2).
// One account, bootstrapped by a one-time setup code that is printed
// to the terminal only — whoever operates the server registers the
// credentials, so a public bind can never be claimed by a stranger.

const (
	sessionCookie = "canopy_session"
	sessionTTL    = 30 * 24 * time.Hour
	maxLoginDelay = 16 * time.Second
)

type account struct {
	ID   string `json:"id"`
	Hash string `json:"hash"` // bcrypt
}

type authStore struct {
	path string

	mu        sync.Mutex
	acct      *account
	setupCode string
	sessions  map[string]time.Time // token -> expiry
	failures  int
}

// webAuthPath is ~/.config/canopy/webauth.json — outside the wiki so
// credentials never travel through git.
func webAuthPath() string {
	return filepath.Join(config.ConfigHome(), "webauth.json")
}

func loadAuthStore(path string) (*authStore, error) {
	a := &authStore{path: path, sessions: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return a, nil
	}
	if err != nil {
		return nil, err
	}
	var acct account
	if err := json.Unmarshal(data, &acct); err != nil {
		return nil, fmt.Errorf("%s is corrupt (delete it to re-run setup): %w", path, err)
	}
	if acct.ID != "" && acct.Hash != "" {
		a.acct = &acct
	}
	return a, nil
}

func randomToken(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is not recoverable
	}
	return hex.EncodeToString(b)
}

func (a *authStore) hasAccount() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.acct != nil
}

// issueSetupCode creates (once) and returns the one-time setup code.
func (a *authStore) issueSetupCode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.setupCode == "" {
		a.setupCode = randomToken(4) // 8 hex chars, single-use, terminal-only
	}
	return a.setupCode
}

// register creates the single account. Requires the setup code.
func (a *authStore) register(code, id, pw string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.acct != nil {
		return fmt.Errorf("account already exists")
	}
	if a.setupCode == "" || subtle.ConstantTimeCompare([]byte(code), []byte(a.setupCode)) != 1 {
		return fmt.Errorf("설정 코드가 올바르지 않습니다 (serve를 실행한 터미널에 출력됩니다)")
	}
	if len(id) < 1 || len(pw) < 8 {
		return fmt.Errorf("아이디는 1자 이상, 비밀번호는 8자 이상이어야 합니다")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	acct := &account{ID: id, Hash: string(hash)}
	data, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(a.path, data, 0o600); err != nil {
		return err
	}
	a.acct = acct
	a.setupCode = "" // single use
	return nil
}

// login verifies credentials with an escalating delay on failures.
func (a *authStore) login(id, pw string) bool {
	a.mu.Lock()
	acct, failures := a.acct, a.failures
	a.mu.Unlock()
	if failures > 0 {
		d := time.Second << (failures - 1)
		if d > maxLoginDelay {
			d = maxLoginDelay
		}
		time.Sleep(d)
	}
	ok := acct != nil &&
		subtle.ConstantTimeCompare([]byte(id), []byte(acct.ID)) == 1 &&
		bcrypt.CompareHashAndPassword([]byte(acct.Hash), []byte(pw)) == nil
	a.mu.Lock()
	defer a.mu.Unlock()
	if ok {
		a.failures = 0
	} else if a.failures < 10 {
		a.failures++
	}
	return ok
}

func (a *authStore) newSession() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	tok := randomToken(32)
	a.sessions[tok] = time.Now().Add(sessionTTL)
	return tok
}

func (a *authStore) validSession(tok string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(a.sessions, tok)
		return false
	}
	return true
}

func (a *authStore) dropSession(tok string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, tok)
}

// --- server integration ---

// EnableAuth turns on the auth requirement (non-loopback bind). When
// no account exists yet it returns the one-time setup code to print.
func (s *Server) EnableAuth() (setupCode string, err error) {
	a, err := loadAuthStore(webAuthPath())
	if err != nil {
		return "", err
	}
	s.auth = a
	s.authRequired = true
	if !a.hasAccount() {
		return a.issueSetupCode(), nil
	}
	return "", nil
}

// IsLoopbackAddr reports whether a listen address binds only loopback.
// An empty host ("":8737") binds every interface — not loopback.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) sessionOK(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && s.auth.validSession(c.Value)
}

// guard wraps the mux with the auth wall and a same-origin check on
// every mutating request (CSRF/DNS-rebinding defence — applies even on
// localhost, where a hostile web page could otherwise POST blindly).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				if u, err := url.Parse(o); err != nil || u.Host != r.Host {
					http.Error(w, "cross-origin request rejected", http.StatusForbidden)
					return
				}
			}
		}
		if !s.authRequired {
			next.ServeHTTP(w, r)
			return
		}
		p := r.URL.Path
		if strings.HasPrefix(p, "/static/") || p == "/login" || p == "/setup" {
			next.ServeHTTP(w, r)
			return
		}
		if s.sessionOK(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !s.auth.hasAccount() {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

func (s *Server) setSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.auth.newSession(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
}

// --- handlers ---

func (s *Server) handleSetupForm(w http.ResponseWriter, r *http.Request) {
	if !s.authRequired || s.auth.hasAccount() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.render(w, http.StatusOK, "setup.html", map[string]any{"Title": "초기 설정"})
}

func (s *Server) handleSetupSave(w http.ResponseWriter, r *http.Request) {
	if !s.authRequired || s.auth.hasAccount() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	err := s.auth.register(
		strings.TrimSpace(r.FormValue("code")),
		strings.TrimSpace(r.FormValue("id")),
		r.FormValue("password"),
	)
	if err != nil {
		s.render(w, http.StatusBadRequest, "setup.html", map[string]any{"Title": "초기 설정", "Error": err.Error()})
		return
	}
	s.setSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if !s.authRequired || s.sessionOK(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !s.auth.hasAccount() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	s.render(w, http.StatusOK, "login.html", map[string]any{"Title": "로그인"})
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.authRequired {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if s.auth.login(strings.TrimSpace(r.FormValue("id")), r.FormValue("password")) {
		s.setSession(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusUnauthorized, "login.html", map[string]any{
		"Title": "로그인", "Error": "아이디 또는 비밀번호가 올바르지 않습니다",
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.auth.dropSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
