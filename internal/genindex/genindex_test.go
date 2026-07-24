package genindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

// TestRegenerate checks invariants C1 and C2:
//
//	C1: index.md Total == 실제 파일 수
//	C2: 카테고리 인덱스는 전량 나열
func TestRegenerate(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	// Create a minimal wiki with 3 pages.
	dirs := map[string]string{
		"concepts":    "concept",
		"entities":    "entity",
		"comparisons": "comparison",
	}
	for dir, typ := range dirs {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
		raw := "---\ntitle: \"Test " + dir + "\"\ncreated: 2026-01-01\nupdated: 2026-01-15\ntype: " + typ + "\ntags: [tool]\n---\n\nbody\n"
		os.WriteFile(filepath.Join(root, dir, "test-"+dir+".md"), []byte(raw), 0o644)
	}

	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(scan.Pages))
	}

	if err := Regenerate(w, scan); err != nil {
		t.Fatal(err)
	}

	// C1: index.md should have "Total pages: 3".
	data, err := os.ReadFile(w.IndexMDPath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "**Total pages**: 3") {
		t.Errorf("index.md missing total count: %s", content)
	}
	// Generated mark present.
	if !strings.Contains(content, generatedMark) {
		t.Error("index.md missing generated mark")
	}
	// Each page appears in the recent list.
	for _, dir := range []string{"concepts", "entities", "comparisons"} {
		if !strings.Contains(content, "[[test-"+dir+"]]") {
			t.Errorf("index.md missing [[test-%s]]", dir)
		}
	}

	// C2: each category index lists its page.
	for _, dir := range []string{"concepts", "entities", "comparisons"} {
		idxPath := filepath.Join(root, "index", dir+".md")
		data, err := os.ReadFile(idxPath)
		if err != nil {
			t.Fatal(err)
		}
		idx := string(data)
		if !strings.Contains(idx, "[[test-"+dir+"]]") {
			t.Errorf("index/%s.md missing [[test-%s]]", dir, dir)
		}
		if !strings.Contains(idx, generatedMark) {
			t.Errorf("index/%s.md missing generated mark", dir)
		}
	}
}

func TestRegenerateEmptyWiki(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Pages) != 0 {
		t.Fatalf("expected 0 pages, got %d", len(scan.Pages))
	}

	if err := Regenerate(w, scan); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(w.IndexMDPath())
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "**Total pages**: 0") {
		t.Errorf("empty wiki total wrong: %s", content)
	}
}

func TestRegenerateIdempotent(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	os.MkdirAll(filepath.Join(root, "concepts"), 0o755)
	raw := "---\ntitle: \"Idempotent\"\ncreated: 2026-01-01\nupdated: 2026-01-01\ntype: concept\ntags: [tool]\n---\n\nbody\n"
	os.WriteFile(filepath.Join(root, "concepts", "idempotent.md"), []byte(raw), 0o644)

	scan, _ := wiki.Scan(w)

	// Run twice.
	if err := Regenerate(w, scan); err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(w, scan); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(w.IndexMDPath())
	if !strings.Contains(string(data), "**Total pages**: 1") {
		t.Error("count drifted after second regenerate")
	}
}

func TestCountArchived(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	// No archive directory.
	if n := countArchived(w); n != 0 {
		t.Errorf("expected 0 archived, got %d", n)
	}

	// With archived pages.
	os.MkdirAll(filepath.Join(root, "_archive", "concepts"), 0o755)
	os.WriteFile(filepath.Join(root, "_archive", "concepts", "old.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(root, "_archive", "concepts", "older.md"), []byte("older"), 0o644)

	if n := countArchived(w); n != 2 {
		t.Errorf("expected 2 archived, got %d", n)
	}
}
