package webui

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func workspaceServer(t *testing.T) (*Server, *config.Wiki) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return taskTestServer(t)
}

func workspaceGet(t *testing.T, s *Server, path, lang string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	r.Host = "localhost"
	r.Header.Set("Accept-Language", lang)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestClientMessagesLocalized(t *testing.T) {
	s, _ := workspaceServer(t)
	bootstrap := regexp.MustCompile(`<script type="application/json" id="ui-messages">(.*?)</script>`)
	for _, lang := range s.i18n.langs {
		t.Run(lang, func(t *testing.T) {
			rec := workspaceGet(t, s, "/page/alpha", lang)
			match := bootstrap.FindStringSubmatch(rec.Body.String())
			if rec.Code != http.StatusOK || len(match) != 2 {
				t.Fatalf("client messages missing: status %d", rec.Code)
			}
			var messages map[string]string
			if err := json.Unmarshal([]byte(match[1]), &messages); err != nil {
				t.Fatal(err)
			}
			lc := s.i18n.localizer(lang)
			for _, id := range clientMessageIDs {
				if messages[id] == "" || messages[id] == id || messages[id] != localizeString(lc, id) {
					t.Errorf("%s: client message %q = %q", lang, id, messages[id])
				}
			}
			// Verify messages consumed by the shipped clients, independently of
			// the server's own allowlist; a missing allowlist entry must fail.
			for _, file := range []string{"static/app.js", "static/graph.js"} {
				source, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				for _, call := range regexp.MustCompile(`\bt\(["']([a-z][a-z0-9_]*)["']\)`).FindAllStringSubmatch(string(source), -1) {
					if messages[call[1]] == "" || messages[call[1]] == call[1] {
						t.Errorf("%s requires undelivered translation %s", file, call[1])
					}
				}
			}
		})
	}

	// Translation strings are JSON data, even if a future catalog contains
	// HTML. A script terminator must never become executable markup.
	payload := `</script><img src=x onerror="alert(1)">`
	s.i18n.bundle.AddMessages(language.English, &i18n.Message{ID: "copy_button", Other: payload})
	rec := workspaceGet(t, s, "/", "en")
	if strings.Contains(rec.Body.String(), payload) {
		t.Fatal("translation escaped the JSON script context")
	}
	match := bootstrap.FindStringSubmatch(rec.Body.String())
	if len(match) != 2 {
		t.Fatal("safe JSON bootstrap missing")
	}
	var messages map[string]string
	if err := json.Unmarshal([]byte(match[1]), &messages); err != nil || messages["copy_button"] != payload {
		t.Fatalf("JSON must preserve the translated text safely: %v, %q", err, messages["copy_button"])
	}
}

func TestWorkspaceNavigation(t *testing.T) {
	s, _ := workspaceServer(t)
	current := regexp.MustCompile(`<a href="([^"]+)" aria-current="page"`)
	for _, tc := range []struct{ path, section string }{
		{"/", "/"}, {"/browse?type=concept", "/browse"},
		{"/page/alpha", "/browse"}, {"/edit/alpha", "/browse"},
		{"/graph", "/graph"}, {"/special/discover", "/special/discover"},
		{"/special/tasks", "/special/tasks"}, {"/history", "/history"},
		{"/special/recent", "/special/recent"},
		{"/special/attention", "/special/attention"}, {"/special/gaps", "/special/attention"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rec := workspaceGet(t, s, tc.path, "en")
			matches := current.FindAllStringSubmatch(rec.Body.String(), -1)
			if rec.Code != http.StatusOK || len(matches) != 1 || matches[0][1] != tc.section {
				t.Fatalf("current section: status=%d links=%v want=%s", rec.Code, matches, tc.section)
			}
			if !strings.Contains(rec.Body.String(), `href="#main-content"`) || !strings.Contains(rec.Body.String(), `id="main-content"`) {
				t.Fatal("keyboard skip link has no destination")
			}
		})
	}
}

func TestLanguageSwitchPreservesQuery(t *testing.T) {
	s, _ := workspaceServer(t)
	links := regexp.MustCompile(`href="(/setlang\?[^"]+)"`)
	for _, path := range []string{"/browse?dir=concepts&tag=ai-ml&tag=tool", "/search?q=body+alpha"} {
		rec := workspaceGet(t, s, path, "ko")
		match := links.FindStringSubmatch(rec.Body.String())
		if rec.Code != http.StatusOK || len(match) != 2 {
			t.Fatalf("language link missing on %s (status %d)", path, rec.Code)
		}
		link, err := url.Parse(html.UnescapeString(match[1]))
		if err != nil || link.Query().Get("next") != path {
			t.Fatalf("language link lost query: %v, %s", err, match[1])
		}
		switched := workspaceGet(t, s, link.String(), "ko")
		if switched.Code != http.StatusSeeOther || switched.Header().Get("Location") != path {
			t.Fatalf("language change lost query: status=%d location=%s", switched.Code, switched.Header().Get("Location"))
		}
		cookies := switched.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != langCookie || cookies[0].Value != "en" {
			t.Fatalf("language cookie missing: %v", cookies)
		}
	}
}

func TestLanguageSwitchRejectsExternalRedirect(t *testing.T) {
	s, _ := workspaceServer(t)
	for _, next := range []string{"https://example.test/", "//example.test/", "/\\example.test/", "\\example.test/", "relative", "/page/alpha\r\nLocation: https://example.test", ""} {
		query := url.Values{"lang": {"en"}, "next": {next}}
		rec := workspaceGet(t, s, "/setlang?"+query.Encode(), "ko")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
			t.Errorf("unsafe next %q: status=%d location=%q", next, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func assertSummary(t *testing.T, s *Server, total, read, unread, pending string) {
	t.Helper()
	rec := workspaceGet(t, s, "/", "en")
	if rec.Code != http.StatusOK {
		t.Fatalf("home status %d: %s", rec.Code, rec.Body.String())
	}
	for id, want := range map[string]string{"total": total, "read": read, "unread": unread, "pending": pending} {
		re := regexp.MustCompile(`<strong id="stat-` + id + `">([^<]+)</strong>`)
		got := re.FindStringSubmatch(rec.Body.String())
		if len(got) != 2 || got[1] != want {
			t.Errorf("%s summary: got %v want %s", id, got, want)
		}
	}
}

func TestWorkspaceSummary(t *testing.T) {
	s, w := workspaceServer(t)
	assertSummary(t, s, "2", "0", "2", "0")
	rs, err := reads.Load(w)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	rs.Mark("alpha", "explicit", now)
	rs.Mark("no-longer-in-wiki", "explicit", now)
	rs.MarkAgent("beta", now)
	if err := rs.Save(w); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(reads.Path(w))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.FileEdit(w, "alpha", "add a reference", "", "base", "web", now); err != nil {
		t.Fatal(err)
	}
	closed, err := tasks.FileEdit(w, "beta", "already handled", "", "base", "web", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Close(w, nil, closed.ID, tasks.StatusDismissed, "already handled", "web", now); err != nil {
		t.Fatal(err)
	}
	assertSummary(t, s, "2", "1", "1", "1")
	after, err := os.ReadFile(reads.Path(w))
	if err != nil || string(after) != string(before) {
		t.Fatal("viewing the summary changed human read state")
	}

	// Corrupt task data must be shown as unavailable, never as zero work.
	if err := os.WriteFile(filepath.Join(w.Root, "_meta", "tasks", "bad.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertSummary(t, s, "2", "1", "1", "—")

	t.Run("empty wiki", func(t *testing.T) {
		empty := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
		server, err := NewServer(empty, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertSummary(t, server, "0", "0", "0", "0")
	})
}
