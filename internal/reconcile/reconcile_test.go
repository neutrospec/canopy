package reconcile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
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

// The gate is opt-in: before a baseline, Foreign/Record stay silent.
func TestUninitializedGateIsSilent(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "content")

	if _, ok, err := Foreign(w); err != nil || ok {
		t.Fatalf("uninitialized gate should report ok=false, got ok=%v err=%v", ok, err)
	}
	if err := Record(w, Effects{Written: []string{"concepts/a.md"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path(w)); !os.IsNotExist(err) {
		t.Fatal("Record on an uninitialized gate must not create the ledger")
	}
}

// bless --all baselines; then edits/new/deleted surface as foreign (K2/K4).
func TestForeignDetection(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "original a")
	write(t, w, "concepts/b.md", "original b")

	n, err := BlessAll(w)
	if err != nil || n != 2 {
		t.Fatalf("baseline: n=%d err=%v", n, err)
	}
	if cands, ok, _ := Foreign(w); !ok || len(cands) != 0 {
		t.Fatalf("clean after baseline, got %+v", cands)
	}

	write(t, w, "concepts/a.md", "back-door edit")    // edited
	write(t, w, "concepts/c.md", "brand new page")    // new
	os.Remove(filepath.Join(w.Root, "concepts/b.md")) // deleted

	cands, _, err := Foreign(w)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range cands {
		got[c.RelPath] = c.Kind
	}
	want := map[string]string{
		"concepts/a.md": "edited",
		"concepts/b.md": "deleted",
		"concepts/c.md": "new",
	}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for rel, kind := range want {
		if got[rel] != kind {
			t.Fatalf("%s: want %s, got %s", rel, kind, got[rel])
		}
	}

	// Determinism (K3): repeated reads identical, and no trace (K5).
	before, _ := os.ReadFile(Path(w))
	again, _, _ := Foreign(w)
	if len(again) != len(cands) {
		t.Fatal("repeated Foreign differs")
	}
	after, _ := os.ReadFile(Path(w))
	if string(before) != string(after) {
		t.Fatal("Foreign mutated the ledger (K5)")
	}
}

// BlessPaths records current state including absence (K4).
func TestBlessPaths(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "v1")
	write(t, w, "concepts/b.md", "v1")
	if _, err := BlessAll(w); err != nil {
		t.Fatal(err)
	}

	write(t, w, "concepts/a.md", "v2 (back door)")
	os.Remove(filepath.Join(w.Root, "concepts/b.md"))

	if err := BlessPaths(w, []string{"concepts/a.md", "concepts/b.md"}); err != nil {
		t.Fatal(err)
	}
	cands, _, _ := Foreign(w)
	if len(cands) != 0 {
		t.Fatalf("blessed changes must not resurface, got %+v", cands)
	}
	if err := BlessPaths(w, []string{"concepts/no-such.md"}); err == nil {
		t.Fatal("blessing a path with no file and no ledger entry must error")
	}
}

// Record auto-blesses pipeline writes and clears removals (K2 후단).
func TestRecordEffects(t *testing.T) {
	w := testWiki(t)
	write(t, w, "concepts/a.md", "v1")
	if _, err := BlessAll(w); err != nil {
		t.Fatal(err)
	}

	// A pipeline write lands new content and declares it.
	write(t, w, "concepts/a.md", "v2 via pipeline")
	write(t, w, "concepts/d.md", "created via pipeline")
	if err := Record(w, Effects{Written: []string{"concepts/a.md", "concepts/d.md"}}); err != nil {
		t.Fatal(err)
	}
	if cands, _, _ := Foreign(w); len(cands) != 0 {
		t.Fatalf("pipeline writes must be auto-blessed, got %+v", cands)
	}

	// A pipeline removal clears the entry.
	os.Remove(filepath.Join(w.Root, "concepts/d.md"))
	if err := Record(w, Effects{Removed: []string{"concepts/d.md"}}); err != nil {
		t.Fatal(err)
	}
	if cands, _, _ := Foreign(w); len(cands) != 0 {
		t.Fatalf("pipeline removals must not surface as deleted, got %+v", cands)
	}
}
