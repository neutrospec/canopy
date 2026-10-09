package genindex

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

func TestGeneratedLinksResolveDottedNamespace(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.MkdirAll(filepath.Join(w.Root, "concepts"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"foo.md", "foo.md.md"} {
		if err := os.WriteFile(filepath.Join(w.Root, "concepts", file), []byte("---\ntitle: Foo\ntype: concept\nupdated: 2026-10-10\n---\nbody"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(w, scan); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"index/concepts.md", "index.md"} {
		raw, err := os.ReadFile(filepath.Join(w.Root, file))
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, m := range regexp.MustCompile(`\[\[([^]|]+)\]\]`).FindAllStringSubmatch(string(raw), -1) {
			if len(m[1]) >= 6 && m[1][:6] == "index/" {
				continue
			}
			p, err := scan.Find(m[1])
			if err != nil {
				t.Errorf("%s generated invalid target %q: %v", file, m[1], err)
				continue
			}
			seen[p.RelPath] = true
		}
		if len(seen) != 2 {
			t.Errorf("%s reaches %d distinct pages, want 2", file, len(seen))
		}
	}
}
