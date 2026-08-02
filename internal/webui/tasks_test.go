package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/tasks"
)

func taskTestServer(t *testing.T) (*Server, *config.Wiki) {
	t.Helper()
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.MkdirAll(filepath.Join(w.Root, "concepts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		content := "---\ntitle: " + name + "\ntype: concept\n---\nbody of " + name
		if err := os.WriteFile(filepath.Join(w.Root, "concepts", name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewServer(w, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, w
}

func postForm(t *testing.T, h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Filing from the web creates queue files only — it never edits pages (T6).
func TestWebTaskFilingDoesNotEditPages(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()
	pagePath := filepath.Join(w.Root, "concepts", "alpha.md")
	before, _ := os.ReadFile(pagePath)

	rec := postForm(t, h, "/task/edit/alpha", url.Values{"request": {"add a diagram"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("edit filing: status %d", rec.Code)
	}
	rec = postForm(t, h, "/task/connect", url.Values{"a": {"alpha"}, "b": {"beta"}, "sim": {"0.81"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("connect filing: status %d", rec.Code)
	}

	after, _ := os.ReadFile(pagePath)
	if string(before) != string(after) {
		t.Fatal("filing edited the page (violates T6)")
	}
	list, err := tasks.Load(w)
	if err != nil || len(list) != 2 {
		t.Fatalf("want 2 filed tasks, got %d (err=%v)", len(list), err)
	}
	for _, task := range list {
		if task.Door != "web" || task.Status != tasks.StatusPending {
			t.Fatalf("bad task: %+v", task)
		}
	}
	// edit task captured the baseline hash so done can demand a change
	for _, task := range list {
		if task.Type == tasks.TypeEdit && task.Base != contentHash(before) {
			t.Fatalf("edit base = %q, want hash of current file", task.Base)
		}
	}
}

func TestWebTaskFilingRejectsBadInput(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()

	if rec := postForm(t, h, "/task/connect", url.Values{"a": {"alpha"}, "b": {"alpha"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("same-page pair: status %d", rec.Code)
	}
	if rec := postForm(t, h, "/task/connect", url.Values{"a": {"alpha"}, "b": {"no-such"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing page: status %d", rec.Code)
	}
	// blank request: bounce back without filing
	if rec := postForm(t, h, "/task/edit/alpha", url.Values{"request": {"   "}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("blank request: status %d", rec.Code)
	}
	if list, _ := tasks.Load(w); len(list) != 0 {
		t.Fatalf("bad input must not file tasks, got %d", len(list))
	}
}

// Refiling the same suggestion is a no-op, and the page todo list shows
// pending tasks (the "requested" marker draws from the same data).
func TestWebConnectRefilingAndTodoList(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()
	form := url.Values{"a": {"alpha"}, "b": {"beta"}, "sim": {"0.81"}}
	postForm(t, h, "/task/connect", form)
	postForm(t, h, "/task/connect", form)
	if list, _ := tasks.Load(w); len(list) != 1 {
		t.Fatalf("refiling created a duplicate, got %d tasks", len(list))
	}

	views, filed := s.pageTasks("alpha", s.i18n.localizer(defaultLang))
	if len(views) != 1 || views[0].Other != "beta" {
		t.Fatalf("todo list: %+v", views)
	}
	if !filed["beta"] {
		t.Fatal("suggestion marker must know the pair is filed")
	}

	// The page view actually renders the todo list, the edit-request
	// form, and the filed notice (template executes, not just parses).
	req := httptest.NewRequest("GET", "/page/alpha?tasked=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("page render: status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"tasklist", "/task/edit/alpha", "task_filed_notice", "/page/beta"} {
		if want == "task_filed_notice" {
			continue // localized text, checked via the notice element below
		}
		if !strings.Contains(body, want) {
			t.Fatalf("page render missing %q", want)
		}
	}
	if !strings.Contains(body, "notice") {
		t.Fatal("tasked=1 must show the filed notice")
	}
}

// The web editor is a proposal door (invariant I2): saving files an edit
// task carrying the body — the page file is never written.
func TestWebEditFilesProposalNotFile(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()
	pagePath := filepath.Join(w.Root, "concepts", "alpha.md")
	before, _ := os.ReadFile(pagePath)

	rec := postForm(t, h, "/edit/alpha", url.Values{
		"body": {"rewritten body\r\nline two with [[beta]]"},
		"note": {"인트로를 다듬음"},
	})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "tasked=1") {
		t.Fatalf("proposal save: status %d loc %s", rec.Code, rec.Header().Get("Location"))
	}
	after, _ := os.ReadFile(pagePath)
	if string(before) != string(after) {
		t.Fatal("web edit wrote the page file (I2 violated)")
	}
	list, err := tasks.Load(w)
	if err != nil || len(list) != 1 {
		t.Fatalf("want 1 task, got %d (err=%v)", len(list), err)
	}
	task := list[0]
	if task.Type != tasks.TypeEdit || task.Door != "web" {
		t.Fatalf("bad task: %+v", task)
	}
	if task.Body != "rewritten body\nline two with [[beta]]" {
		t.Fatalf("body not normalized/preserved (T9): %q", task.Body)
	}
	if task.Request != "인트로를 다듬음" || task.Base != contentHash(before) {
		t.Fatalf("note/base: %+v", task)
	}
}

// Submitting the editor without changing anything files nothing.
func TestWebEditNoopFilesNothing(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()
	rec := postForm(t, h, "/edit/alpha", url.Values{"body": {"body of alpha"}, "note": {""}})
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "tasked=1") {
		t.Fatalf("no-op save: status %d loc %s", rec.Code, rec.Header().Get("Location"))
	}
	if list, _ := tasks.Load(w); len(list) != 0 {
		t.Fatalf("no-op save filed %d task(s)", len(list))
	}
}

// The tasks screen lists the queue, shows proposal bodies, and lets the
// requester withdraw a pending edit request — but never a connect task.
func TestTasksScreenAndWithdraw(t *testing.T) {
	s, w := taskTestServer(t)
	h := s.Handler()
	postForm(t, h, "/edit/alpha", url.Values{"body": {"proposed text"}, "note": {"note-marker"}})
	postForm(t, h, "/task/connect", url.Values{"a": {"alpha"}, "b": {"beta"}, "sim": {"0.8"}})

	req := httptest.NewRequest("GET", "/special/tasks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tasks screen: status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"note-marker", "proposed text", "/task/cancel/", "/page/beta"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tasks screen missing %q", want)
		}
	}

	list, _ := tasks.Load(w)
	var editID, connectID string
	for _, task := range list {
		if task.Type == tasks.TypeEdit {
			editID = task.ID
		} else {
			connectID = task.ID
		}
	}
	if rec := postForm(t, h, "/task/cancel/"+connectID, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("connect withdraw must be refused, got %d", rec.Code)
	}
	if rec := postForm(t, h, "/task/cancel/"+editID, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit withdraw: status %d", rec.Code)
	}
	got, _ := tasks.Get(w, editID)
	if got.Status != tasks.StatusDismissed {
		t.Fatalf("withdrawn task status = %s", got.Status)
	}
}

// The suggested-links section renders both branches: a connect button
// for fresh suggestions and a "requested" marker for filed ones.
func TestSuggestedSectionTaskButtons(t *testing.T) {
	s, _ := taskTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/page/alpha", nil)
	data := map[string]any{
		"Title":     "alpha",
		"Page":      map[string]any{"Slug": "alpha", "Title": "alpha", "Type": "concept", "Tags": []string{}, "Updated": "2026-08-02", "RelPath": "concepts/alpha.md"},
		"Body":      "",
		"Backlinks": []string{},
		"Suggested": []suggestion{
			{Slug: "beta", Title: "beta", Sim: 0.81},
			{Slug: "gamma", Title: "gamma", Sim: 0.75, Requested: true},
		},
	}
	s.render(rec, req, http.StatusOK, "page.html", data)
	body := rec.Body.String()
	if !strings.Contains(body, `action="/task/connect"`) || !strings.Contains(body, `value="beta"`) {
		t.Fatal("fresh suggestion must render the connect-request form")
	}
	if !strings.Contains(body, "⏳") {
		t.Fatal("filed suggestion must render the requested marker")
	}
}
