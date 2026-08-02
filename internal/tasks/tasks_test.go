package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

func testWiki(t *testing.T) *config.Wiki {
	t.Helper()
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.MkdirAll(filepath.Join(w.Root, "concepts"), 0o755); err != nil {
		t.Fatal(err)
	}
	return w
}

func write(t *testing.T, w *config.Wiki, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(w.Root, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scan(t *testing.T, w *config.Wiki) *wiki.ScanResult {
	t.Helper()
	s, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

var now = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)

// T2: done is the code's check — a connect closes only once the mutual
// links actually exist.
func TestConnectDoneRequiresMutualLinks(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/alpha.md", "no links here")
	write(t, w, "concepts/beta.md", "none here either")

	task, created, err := FileConnect(w, "Beta", "Alpha", 0.83, "web", now)
	if err != nil || !created {
		t.Fatalf("file: created=%v err=%v", created, err)
	}
	if task.ID != "connect-alpha--beta" {
		t.Fatalf("id not deterministic/sorted: %s", task.ID)
	}

	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "", now); err == nil {
		t.Fatal("done must be rejected while links are missing")
	}
	if got, _ := Get(w, task.ID); got.Status != StatusPending {
		t.Fatalf("rejected done must leave task pending, got %s", got.Status)
	}

	// one-directional link is not enough
	write(t, w, "concepts/alpha.md", "see [[beta]]")
	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "", now); err == nil {
		t.Fatal("done must require BOTH directions")
	}

	write(t, w, "concepts/beta.md", "see [[alpha]]")
	closed, err := Close(w, scan(t, w), task.ID, StatusDone, "linked", now)
	if err != nil {
		t.Fatalf("done with mutual links: %v", err)
	}
	if closed.Status != StatusDone || closed.Closed == "" || closed.Note != "linked" {
		t.Fatalf("close fields: %+v", closed)
	}

	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "", now); err == nil {
		t.Fatal("closing twice must fail")
	}
}

// T4: refiling the same pair (either order) is a no-op, including after
// a dismiss — the judgment is respected.
func TestConnectFilingIsIdempotent(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "x")
	write(t, w, "concepts/b.md", "y")

	if _, created, err := FileConnect(w, "a", "b", 0.9, "web", now); err != nil || !created {
		t.Fatalf("first filing: created=%v err=%v", created, err)
	}
	if _, created, _ := FileConnect(w, "b", "a", 0.9, "web", now.Add(time.Hour)); created {
		t.Fatal("refiling must not create a second task")
	}
	entries, _ := os.ReadDir(Dir(w))
	if len(entries) != 1 {
		t.Fatalf("want 1 task file, got %d", len(entries))
	}

	if _, err := Close(w, scan(t, w), ConnectID("a", "b"), StatusDismissed, "unrelated", now); err != nil {
		t.Fatal(err)
	}
	got, _, err := FileConnect(w, "a", "b", 0.9, "web", now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusDismissed {
		t.Fatalf("refiling after dismiss must not resurrect the task, got %s", got.Status)
	}
	if n, _ := PendingCount(w); n != 0 {
		t.Fatalf("pending count after dismiss = %d", n)
	}
}

// T2 for edit: done requires the page to have actually changed since filing.
func TestEditDoneRequiresChange(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/topic.md", "original")
	base := hashBytes([]byte("original"))

	task, err := FileEdit(w, "topic", "add a comparison table", "", base, "web", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "", now); err == nil {
		t.Fatal("done must be rejected while the page is unchanged")
	}
	write(t, w, "concepts/topic.md", "original + the table")
	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "done", now); err != nil {
		t.Fatalf("done after change: %v", err)
	}
}

func TestEditFilingNeedsRequest(t *testing.T) {
	w := testWiki(t)
	if _, err := FileEdit(w, "topic", "  ", "", "", "web", now); err == nil {
		t.Fatal("blank request must be rejected")
	}
}

// The web editor files its submitted text as a proposal: body alone is
// a valid edit task and is preserved verbatim (invariant T9).
func TestEditProposalCarriesBody(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/topic.md", "original")
	proposed := "rewritten body\n\nwith [[links]] and\nmultiple lines"

	task, err := FileEdit(w, "topic", "", proposed, hashBytes([]byte("original")), "web", now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Get(w, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != proposed {
		t.Fatalf("proposed body not preserved verbatim:\n%q", got.Body)
	}
	// same verification as any edit: done needs the page to have moved
	if _, err := Close(w, scan(t, w), task.ID, StatusDone, "", now); err == nil {
		t.Fatal("done must be rejected while the page is unchanged")
	}
}

// T5 + 규칙 1b: unknown types can be listed and dismissed but not done,
// and closing preserves fields this binary doesn't know.
func TestUnknownTypeMixedVersionSafety(t *testing.T) {
	w := testWiki(t)
	if err := os.MkdirAll(Dir(w), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":9,"id":"future-1","type":"summarize","status":"pending","created":"2026-08-01T00:00:00Z","future_field":"keep me"}`
	if err := os.WriteFile(filepath.Join(Dir(w), "future-1.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := Load(w)
	if err != nil || len(list) != 1 {
		t.Fatalf("unknown type must still list: %v %d", err, len(list))
	}
	if _, err := Close(w, scan(t, w), "future-1", StatusDone, "", now); err == nil {
		t.Fatal("done on unknown type must be refused")
	}
	if _, err := Close(w, scan(t, w), "future-1", StatusDismissed, "can't judge here", now); err != nil {
		t.Fatalf("dismiss on unknown type must work: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(Dir(w), "future-1.json"))
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["future_field"] != "keep me" {
		t.Fatal("closing must preserve unknown fields (규칙 1b)")
	}
	if m["status"] != StatusDismissed {
		t.Fatalf("status not patched: %v", m["status"])
	}
}

// T8: gc removes only closed tasks past the keep window.
func TestGCNeverRemovesPending(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "see [[b]]")
	write(t, w, "concepts/b.md", "see [[a]]")

	if _, _, err := FileConnect(w, "a", "b", 0.9, "cli", now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := Close(w, scan(t, w), ConnectID("a", "b"), StatusDone, "", now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := FileEdit(w, "a", "still open", "", "", "cli", now.Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}

	removed, err := GC(w, 0, now)
	if err != nil || removed != 1 {
		t.Fatalf("gc: removed=%d err=%v", removed, err)
	}
	list, _ := Load(w)
	if len(list) != 1 || list[0].Status != StatusPending {
		t.Fatalf("pending must survive gc, got %+v", list)
	}
}

// Filing never touches pages (T6).
func TestFilingDoesNotEditPages(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "body a")
	write(t, w, "concepts/b.md", "body b")
	before := map[string]string{}
	for _, rel := range []string{"concepts/a.md", "concepts/b.md"} {
		data, _ := os.ReadFile(filepath.Join(w.Root, rel))
		before[rel] = string(data)
	}
	if _, _, err := FileConnect(w, "a", "b", 0.8, "web", now); err != nil {
		t.Fatal(err)
	}
	if _, err := FileEdit(w, "a", "fix the intro", "", hashBytes([]byte("body a")), "web", now); err != nil {
		t.Fatal(err)
	}
	for rel, want := range before {
		data, _ := os.ReadFile(filepath.Join(w.Root, rel))
		if string(data) != want {
			t.Fatalf("%s modified by filing", rel)
		}
	}
	// and every filed task is self-versioned (T1)
	list, _ := Load(w)
	for _, task := range list {
		if task.Version != FormatVersion {
			t.Fatalf("task %s missing version", task.ID)
		}
		if !strings.HasPrefix(task.Created, "2026-") {
			t.Fatalf("task %s missing created", task.ID)
		}
	}
}
