package webui

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/tasks"
)

const proxyTestToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func proxyTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, _ := taskTestServer(t)
	if err := s.EnableTokenAuth(proxyTestToken); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigurePublicOrigin("https://wiki.example"); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureListener("127.0.0.1:8737", nil); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProxyAuthRoutes(t *testing.T) {
	s := proxyTestServer(t)
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(old) })
	before, _ := os.ReadFile(filepath.Join(s.w.Root, "concepts/alpha.md"))
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/page/alpha", "", 200}, {"HEAD", "/page/alpha", "", 200},
		{"GET", "/api/search?q=alpha", "", 200}, {"GET", "/api/graph", "", 200},
		{"GET", "/static/app.js", "", 200}, {"POST", "/task/edit/alpha", "request=add+diagram", 303},
		{"POST", "/edit/alpha", "body=proposed+body", 303},
		{"GET", "/setup", "", 303}, {"POST", "/login", "", 303}, {"POST", "/logout", "", 303},
	} {
		r := httptest.NewRequest(tc.method, "http://localhost:8737"+tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+proxyTestToken)
		r.Header.Set("Origin", "https://wiki.example")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
		if w.Header().Get("Set-Cookie") != "" {
			t.Fatalf("token request set cookie: %s", tc.path)
		}
		if strings.Contains(w.Body.String()+w.Header().Get("Location"), proxyTestToken) {
			t.Fatal("token in response")
		}
		if loc := w.Header().Get("Location"); loc != "" && !strings.HasPrefix(loc, "/") {
			t.Fatalf("non-relative redirect: %s", loc)
		}
	}
	after, _ := os.ReadFile(filepath.Join(s.w.Root, "concepts/alpha.md"))
	list, err := tasks.Load(s.w)
	if !bytes.Equal(before, after) || err != nil || len(list) != 2 {
		t.Fatalf("proposal invariant: tasks=%d err=%v", len(list), err)
	}
	if s.auth.hasAccount() || s.auth.setupCode != "" || len(s.auth.sessions) != 0 {
		t.Fatal("token bootstrapped account/session")
	}
	if strings.Contains(logs.String(), proxyTestToken) {
		t.Fatal("token logged")
	}
}

func TestProxyAuthRejectionsAndSessions(t *testing.T) {
	s := proxyTestServer(t)
	for _, path := range []string{"/page/alpha", "/api/search?q=alpha", "/api/graph", "/setup", "/login"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost:8737"+path, nil))
		if w.Code != 401 {
			t.Fatalf("no credentials %s: %d", path, w.Code)
		}
	}
	code := s.auth.issueSetupCode()
	if err := s.auth.register(code, "me", "longenough"); err != nil {
		t.Fatal(err)
	}
	// The original session login is still usable with token auth enabled.
	r := httptest.NewRequest("POST", "http://localhost:8737/login", strings.NewReader("id=me&password=longenough"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 303 || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login: %d", w.Code)
	}
	cookie := w.Result().Cookies()[0]
	for _, headers := range [][]string{nil, {"Bearer wrong"}, {"Bearer"}, {"Basic anything"}, {""}, {"Bearer " + proxyTestToken, "Bearer " + proxyTestToken}, {"Bearer " + proxyTestToken + ",wrong"}} {
		r := httptest.NewRequest("GET", "http://localhost:8737/page/alpha", nil)
		r.AddCookie(cookie)
		for _, value := range headers {
			r.Header.Add("Authorization", value)
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		want := 401
		if headers == nil {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("credential combination: %d want %d", w.Code, want)
		}
	}
}

func TestProxyOriginAndHost(t *testing.T) {
	s := proxyTestServer(t)
	for _, tc := range []struct {
		host, origin string
		want         int
	}{
		{"localhost:8737", "https://wiki.example", 200}, {"localhost:8737", "http://localhost:8737", 200}, {"localhost:8737", "", 200},
		{"localhost:8737", "https://evil.example", 403}, {"localhost:8737", "https://wiki.example.evil", 403},
		{"localhost:8737", "https://wiki.example/", 403}, {"localhost:8737", "null", 403},
		{"localhost:8737", "https://localhost:8737", 403}, {"localhost:8737", "http://wiki.example", 403},
		{"evil.example", "https://wiki.example", 403}, {"wiki.example", "https://wiki.example", 403},
		{"localhost:8738", "https://wiki.example", 403},
	} {
		r := httptest.NewRequest("POST", "http://localhost:8737/api/read/alpha", nil)
		r.Host = tc.host
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		r.Header.Set("Authorization", "Bearer "+proxyTestToken)
		r.Header.Set("X-Forwarded-Host", "wiki.example")
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("host=%s origin=%s: %d", tc.host, tc.origin, w.Code)
		}
	}
	if err := s.ConfigureListener("127.0.0.1:8737", []string{"wiki.example"}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://wiki.example/api/read/alpha", nil)
	r.Header.Set("Origin", "https://wiki.example")
	r.Header.Set("Authorization", "Bearer "+proxyTestToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("explicit proxy Host: %d", w.Code)
	}
	for _, origin := range []string{"http://wiki.example", "https://*.example", "https://wiki.example/", "https://user@wiki.example", "https://wiki.example?", "https://wiki.example#", "https://wiki.example:0", "null"} {
		if s.ConfigurePublicOrigin(origin) == nil {
			t.Fatalf("accepted public origin %q", origin)
		}
	}
}

func TestTokenFile(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		mode          os.FileMode
		ok            bool
	}{
		{"valid", proxyTestToken, 0600, true}, {"newline", proxyTestToken + "\n", 0600, true}, {"crlf", proxyTestToken + "\r\n", 0600, true},
		{"empty", "", 0600, false}, {"short", "abcd", 0600, false}, {"nothex", strings.Repeat("z", 64), 0600, false},
		{"space", proxyTestToken + " ", 0600, false}, {"two", proxyTestToken + "\n" + proxyTestToken, 0600, false},
		{"public", proxyTestToken, 0644, false}, {"large", strings.Repeat("a", 10000), 0600, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			if err := os.WriteFile(path, []byte(tc.content), tc.mode); err != nil {
				t.Fatal(err)
			}
			token, err := ReadTokenFile(path, "")
			if (err == nil) != tc.ok {
				t.Fatalf("valid=%v err=%v", tc.ok, err)
			}
			if err != nil && strings.Contains(err.Error(), proxyTestToken) {
				t.Fatal("token in error")
			}
			if tc.ok && token != proxyTestToken {
				t.Fatal("incorrect token")
			}
		})
	}
	if _, err := ReadTokenFile("relative", ""); err == nil {
		t.Fatal("relative path")
	}
	if _, err := ReadTokenFile(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Fatal("missing file")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	_ = os.WriteFile(path, []byte(proxyTestToken), 0600)
	if _, err := ReadTokenFile(path, dir); err == nil {
		t.Fatal("token inside wiki")
	}
	_ = os.Mkdir(filepath.Join(dir, ".git"), 0700)
	if _, err := ReadTokenFile(path, ""); err == nil {
		t.Fatal("token inside git")
	}
}

func TestProxyAuthUnsetRegression(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	s, _ := taskTestServer(t)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/page/alpha", nil))
	if w.Code != 200 {
		t.Fatal("default loopback changed")
	}
	code, err := s.EnableAuth()
	if err != nil || code == "" {
		t.Fatal("account bootstrap changed")
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/page/alpha", nil))
	if w.Code != 302 || w.Header().Get("Location") != "/setup" {
		t.Fatal("legacy setup redirect changed")
	}
}

// Exercise real HTTP parsing and gateway forwarding, including duplicate headers.
func TestProxyAuthGatewayRoundTrip(t *testing.T) {
	s := proxyTestServer(t)
	upstream := httptest.NewServer(s.Handler())
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	if err := s.ConfigureListener(target.Host, nil); err != nil {
		t.Fatal(err)
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Header.Set("Authorization", "Bearer "+proxyTestToken)
	}}
	gateway := httptest.NewServer(proxy)
	defer gateway.Close()
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	for _, tc := range []struct {
		method, path, origin string
		want                 int
	}{
		{"GET", "/page/alpha", "", 200}, {"HEAD", "/page/alpha", "", 200},
		{"GET", "/static/app.js", "", 200}, {"GET", "/api/search?q=alpha", "", 200},
		{"GET", "/api/graph", "", 200}, {"POST", "/task/edit/alpha", "https://wiki.example", 303},
		{"POST", "/task/edit/alpha", "https://evil.example", 403},
	} {
		req, _ := http.NewRequest(tc.method, gateway.URL+tc.path, strings.NewReader("request=change"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != tc.want || res.Header.Get("Set-Cookie") != "" || strings.Contains(string(body), proxyTestToken) {
			t.Fatalf("gateway %s %s: status=%d", tc.method, tc.path, res.StatusCode)
		}
		if tc.method == "HEAD" && len(body) != 0 {
			t.Fatal("HEAD returned body")
		}
	}
	// Direct access without gateway credentials fails.
	res, err := client.Get(upstream.URL + "/page/alpha")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal("direct access allowed")
	}
}

func TestProxyAuthTokenRotation(t *testing.T) {
	s := proxyTestServer(t)
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(proxyTestToken), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := ReadTokenFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTokenAuth(first); err != nil {
		t.Fatal(err)
	}
	replacement := strings.Repeat("f", 64)
	if err := os.WriteFile(path, []byte(replacement), 0600); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://localhost:8737/page/alpha", nil)
	r.Header.Set("Authorization", "Bearer "+proxyTestToken)
	if !s.bearerOK(r) {
		t.Fatal("file replacement changed live credential before restart")
	}
	second, err := ReadTokenFile(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnableTokenAuth(second); err != nil {
		t.Fatal(err)
	}
	if s.bearerOK(r) {
		t.Fatal("old token survived restart")
	}
	r.Header.Set("Authorization", "Bearer "+replacement)
	if !s.bearerOK(r) {
		t.Fatal("replacement token rejected")
	}
}
