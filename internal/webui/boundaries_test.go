package webui

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/reads"
)

func TestForeignHostCannotReadOrMutate(t *testing.T) {
	s, _ := taskTestServer(t)
	for _, method := range []string{"GET", "POST"} {
		path := "http://foreign.example/page/alpha"
		if method == "POST" {
			path = "http://foreign.example/task/edit/alpha"
		}
		r := httptest.NewRequest(method, path, strings.NewReader("request=change"))
		r.Header.Set("Origin", "http://foreign.example")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s hostile Host returned %d", method, w.Code)
		}
	}
}

func TestUnusedMultipartFileRefusedBeforeMutation(t *testing.T) {
	for _, withFile := range []bool{true, false} {
		s, _ := taskTestServer(t)
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("action", "read"); err != nil {
			t.Fatal(err)
		}
		if withFile {
			part, err := writer.CreateFormFile("attachment", "unused.txt")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write([]byte("unused source")); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "http://localhost/api/read/alpha", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		want := http.StatusOK
		if withFile {
			want = http.StatusBadRequest
		}
		rs, err := reads.Load(s.w)
		if w.Code != want || err != nil || rs.IsRead("alpha") == withFile {
			t.Fatalf("file=%v status=%d read=%v err=%v", withFile, w.Code, rs.IsRead("alpha"), err)
		}
	}
}

func TestLogoutWithoutAuthClearsStaleCookie(t *testing.T) {
	s, _ := taskTestServer(t)
	r := httptest.NewRequest("POST", "http://localhost/logout", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "old"})
	w := httptest.NewRecorder()
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("logout panicked: %v", p)
		}
	}()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout = %d %v", w.Code, w.Header())
	}
}

func TestRawHTMLCannotExecuteButGeneratedMarkupSurvives(t *testing.T) {
	h, err := RenderPage("<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n[[known-page]]\n\n| A | B |\n|---|---|\n| x | y |\n\n```mermaid\nflowchart LR\nA[[\"hello\"]] --> B\n```", exists)
	if err != nil {
		t.Fatal(err)
	}
	got := string(h)
	if strings.Contains(got, "<script>") || strings.Contains(got, "<img src=x") {
		t.Fatalf("raw executable HTML survived: %s", got)
	}
	if !strings.Contains(got, `class="wikilink"`) || !strings.Contains(got, "<table>") || !strings.Contains(got, `class="mermaid"`) {
		t.Fatalf("generated markup lost: %s", got)
	}
}

func TestListenerAuthoritiesAndFormBudgets(t *testing.T) {
	s, _ := taskTestServer(t)
	if err := s.ConfigureListener("127.0.0.1:8737", []string{"wiki.example:443"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		host string
		want int
	}{{"localhost:8737", 200}, {"[::1]:8737", 200}, {"wiki.example:443", 200}, {"localhost:8738", 403}, {"evil.example:8737", 403}} {
		r := httptest.NewRequest("GET", "http://localhost:8737/page/alpha", nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("Host %s = %d, want %d", tc.host, w.Code, tc.want)
		}
	}
	for _, tc := range []struct {
		body string
		want int
	}{{"body=%zz", 400}, {"body=" + strings.Repeat("x", (1<<20)+1), 413}, {"body=", 400}} {
		r := httptest.NewRequest("POST", "http://localhost:8737/edit/alpha", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("form = %d, want %d", w.Code, tc.want)
		}
	}
}

func TestAPISessionFailureIsJSON401(t *testing.T) {
	s, _ := taskTestServer(t)
	if _, err := s.EnableAuth(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://localhost/api/search?q=alpha", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), "authentication_required") {
		t.Fatalf("API auth = %d %s", w.Code, w.Body)
	}
}

func TestMalformedQueryAndInvalidAPIBoundsRefused(t *testing.T) {
	s, _ := taskTestServer(t)
	for _, path := range []string{
		"/api/search?q=%ZZ", "/search?q=%ZZ", "/api/search?q=alpha;bad=x",
		"/api/search?q=alpha&k=-1", "/api/search?q=alpha&k=oops", "/api/search?q=alpha&k=51",
		"/api/search?q=alpha&k=0", "/api/search?k=invalid", "/api/search?q=alpha&k=",
	} {
		r := httptest.NewRequest("GET", "http://localhost"+path, nil)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", path, w.Code)
		}
	}
	for _, path := range []string{"/api/search?q=alpha", "/api/search?q=alpha&k=1", "/api/search?q=alpha&k=50", "/api/search?q="} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("normal control %s = %d", path, w.Code)
		}
	}
}

func TestUnparsedBodiesRespectBudgetBeforeMutation(t *testing.T) {
	for _, knownLength := range []bool{false, true} {
		s, _ := taskTestServer(t)
		r := httptest.NewRequest("POST", "http://localhost/api/read/alpha", strings.NewReader(strings.Repeat("x", (16<<10)+1)))
		r.Header.Set("Content-Type", "application/json")
		if !knownLength {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("knownLength=%v oversized unparsed body = %d", knownLength, w.Code)
		}
		rs, err := reads.Load(s.w)
		if err != nil || rs.IsRead("alpha") {
			t.Fatalf("oversized request changed read state: %v", err)
		}
		w = httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/preview/alpha", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("preview control = %d", w.Code)
		}
		w = httptest.NewRecorder()
		r = httptest.NewRequest("POST", "http://localhost/api/read/alpha", nil)
		r.Header.Set("Content-Type", "application/json")
		s.Handler().ServeHTTP(w, r)
		rs, err = reads.Load(s.w)
		if w.Code != http.StatusOK || err != nil || !rs.IsRead("alpha") {
			t.Fatalf("empty-body read control = %d, %v", w.Code, err)
		}
	}
}
