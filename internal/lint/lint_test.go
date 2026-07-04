package lint

import (
	"path/filepath"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

func TestRunFixture(t *testing.T) {
	root, err := filepath.Abs("../../testdata/fixture-wiki")
	if err != nil {
		t.Fatal(err)
	}
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	rep := Run(w, scan)

	want := map[string]int{
		"stray-root":     1, // stray-page.md
		"no-frontmatter": 1, // orphan-note.md
		"broken-link":    1, // [[does-not-exist]]
		"invalid-tag":    1, // notataxonomytag
		"orphan":         1, // orphan-note.md
	}
	for kind, n := range want {
		if rep.Counts[kind] != n {
			t.Errorf("%s: got %d, want %d\nfindings: %+v", kind, rep.Counts[kind], n, rep.Findings)
		}
	}
}
