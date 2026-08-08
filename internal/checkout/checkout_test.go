package checkout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/wiki"
)

var mv = mermaid.NewValidator()

func testWiki(t *testing.T) (*config.Wiki, *wiki.ScanResult) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "concepts"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: Alpha\ntype: concept\ncreated: 2026-01-01\nupdated: 2026-01-01\ntags: []\n---\n\n# Alpha\n\n본문 첫 판.\n"
	if err := os.WriteFile(filepath.Join(root, "concepts", "alpha.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	return w, scan
}

func rescan(t *testing.T, w *config.Wiki) *wiki.ScanResult {
	t.Helper()
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	return scan
}

func edit(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("edit target %q not in copy", old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The happy path covers R1 (checkout leaves the wiki alone), R5
// (checkin reclaims the copy), and the updated-date bump.
func TestCheckoutEditCheckin(t *testing.T) {
	w, scan := testWiki(t)
	wikiPath := filepath.Join(w.Root, "concepts", "alpha.md")
	before, _ := os.ReadFile(wikiPath)

	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(info.Path, w.Root) {
		t.Errorf("working copy inside the wiki tree: %s", info.Path) // R2
	}
	after, _ := os.ReadFile(wikiPath)
	if string(before) != string(after) {
		t.Error("checkout modified the wiki") // R1
	}

	edit(t, info.Path, "본문 첫 판.", "본문 둘째 판 — 수술적 편집.")
	res, err := Checkin(w, scan, "alpha", mv)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unchanged {
		t.Error("edit reported as unchanged")
	}
	got, _ := os.ReadFile(wikiPath)
	if !strings.Contains(string(got), "둘째 판") {
		t.Error("edit did not land in the wiki")
	}
	if !strings.Contains(string(got), "updated: "+timeNowDate()) {
		t.Errorf("updated not bumped:\n%s", got)
	}
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Error("working copy not reclaimed") // R5
	}
	if open, _ := Open(w); len(open) != 0 {
		t.Errorf("open list not empty: %+v", open)
	}
}

func timeNowDate() string { return time.Now().Format("2006-01-02") }

// R4: the wiki moved underneath the checkout — refuse, guide, keep the copy.
func TestCheckinBaseConflict(t *testing.T) {
	w, scan := testWiki(t)
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, info.Path, "본문 첫 판.", "에이전트의 편집.")

	wikiPath := filepath.Join(w.Root, "concepts", "alpha.md")
	data, _ := os.ReadFile(wikiPath)
	os.WriteFile(wikiPath, []byte(strings.Replace(string(data), "첫 판", "딴 머신의 편집", 1)), 0o644)

	_, err = Checkin(w, rescan(t, w), "alpha", mv)
	if err == nil || !strings.Contains(err.Error(), "re-checkout") {
		t.Fatalf("expected base-conflict refusal, got %v", err)
	}
	if _, statErr := os.Stat(info.Path); statErr != nil {
		t.Error("refusal must keep the working copy for re-apply")
	}
}

// R3: write gates run on the way in — broken mermaid and taxonomy
// violations never reach the wiki.
func TestCheckinGates(t *testing.T) {
	w, scan := testWiki(t)
	wikiPath := filepath.Join(w.Root, "concepts", "alpha.md")

	cases := map[string][2]string{
		"mermaid": {"본문 첫 판.", "```mermaid\nflowchart LR\n  A --> end\n```"},
		"tag":     {"tags: []", "tags: [nosuchtag]"},
		"type":    {"type: concept", "type: entity"},              // R7
		"created": {"created: 2026-01-01", "created: 2030-01-01"}, // R7
	}
	for label, c := range cases {
		info, err := Checkout(w, scan, "alpha")
		if err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(wikiPath)
		edit(t, info.Path, c[0], c[1])
		if _, err := Checkin(w, scan, "alpha", mv); err == nil {
			t.Errorf("%s: gate did not refuse", label)
		}
		after, _ := os.ReadFile(wikiPath)
		if string(before) != string(after) {
			t.Errorf("%s: refused checkin still touched the wiki", label)
		}
		if err := Discard(w, "alpha"); err != nil {
			t.Fatal(err)
		}
	}
}

// R8: an untouched copy checks in as a no-op and is reclaimed.
func TestCheckinUnchanged(t *testing.T) {
	w, scan := testWiki(t)
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	wikiPath := filepath.Join(w.Root, "concepts", "alpha.md")
	before, _ := os.ReadFile(wikiPath)
	res, err := Checkin(w, scan, "alpha", mv)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Unchanged {
		t.Error("expected unchanged result")
	}
	after, _ := os.ReadFile(wikiPath)
	if string(before) != string(after) {
		t.Error("no-op checkin modified the wiki")
	}
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Error("no-op checkin must reclaim the copy")
	}
}

// Checkout is idempotent: a modified copy is never clobbered; an
// unmodified one refreshes to the current wiki content.
func TestCheckoutIdempotent(t *testing.T) {
	w, scan := testWiki(t)
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	edit(t, info.Path, "본문 첫 판.", "진행 중인 편집.")

	again, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !again.Modified {
		t.Error("second checkout must report modified")
	}
	data, _ := os.ReadFile(again.Path)
	if !strings.Contains(string(data), "진행 중인 편집.") {
		t.Error("second checkout clobbered in-progress edits")
	}

	// After discard, wiki changes flow into a fresh checkout.
	if err := Discard(w, "alpha"); err != nil {
		t.Fatal(err)
	}
	wikiPath := filepath.Join(w.Root, "concepts", "alpha.md")
	data, _ = os.ReadFile(wikiPath)
	os.WriteFile(wikiPath, []byte(strings.Replace(string(data), "첫 판", "새 판", 1)), 0o644)
	fresh, err := Checkout(w, rescan(t, w), "alpha")
	if err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(fresh.Path)
	if !strings.Contains(string(data), "새 판") {
		t.Error("fresh checkout did not pick up current wiki content")
	}
}

// R6: open checkouts are enumerable with their modified state.
func TestOpenList(t *testing.T) {
	w, scan := testWiki(t)
	if open, err := Open(w); err != nil || len(open) != 0 {
		t.Fatalf("expected empty list, got %v (%v)", open, err)
	}
	info, err := Checkout(w, scan, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	open, err := Open(w)
	if err != nil || len(open) != 1 || open[0].Slug != "alpha" || open[0].Modified {
		t.Fatalf("expected one unmodified checkout, got %+v (%v)", open, err)
	}
	edit(t, info.Path, "본문 첫 판.", "수정.")
	open, _ = Open(w)
	if !open[0].Modified {
		t.Error("modified flag not set after edit")
	}
}
