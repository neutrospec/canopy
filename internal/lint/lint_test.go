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

// Fixture usage: infrastructure ×3, tool ×1, comparison ×1,
// notataxonomytag ×1, across 4 governed pages (orphan-note has no tags).
func TestTagAudit(t *testing.T) {
	root, err := filepath.Abs("../../testdata/fixture-wiki")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Schema.Topics = []string{"infrastructure", "tool", "unused-topic"}
	cfg.Schema.Forms = []string{"comparison"}
	w := &config.Wiki{Root: root, Cfg: cfg}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	rep := TagAudit(w, scan)

	if rep.TotalPages != 4 {
		t.Fatalf("total_pages = %d, want 4", rep.TotalPages)
	}
	if len(rep.UnusedTopics) != 1 || rep.UnusedTopics[0] != "unused-topic" {
		t.Errorf("unused_topics = %v", rep.UnusedTopics)
	}
	// infrastructure 3/4 = 75% > 25%; tool 1/4 = 25% is not strictly over.
	if len(rep.OverbroadTopics) != 1 || rep.OverbroadTopics[0].Tag != "infrastructure" {
		t.Errorf("overbroad_topics = %+v", rep.OverbroadTopics)
	}
	// comparison is a form at 25% — never pressured, only reported.
	if len(rep.Forms) != 1 || rep.Forms[0].Count != 1 {
		t.Errorf("forms = %+v", rep.Forms)
	}
	if len(rep.UnknownTags) != 1 || rep.UnknownTags[0].Tag != "notataxonomytag" {
		t.Errorf("unknown_tags = %+v", rep.UnknownTags)
	}
	if rep.Legacy {
		t.Error("legacy must be false with faceted config")
	}
}

// Legacy single-list taxonomy: the list is treated as topics so reclaim
// and split pressure still apply before the facets are split.
func TestTagAuditLegacy(t *testing.T) {
	root, err := filepath.Abs("../../testdata/fixture-wiki")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Schema.Topics = nil
	cfg.Schema.Forms = nil
	cfg.Schema.Tags = []string{"infrastructure", "ghost"}
	w := &config.Wiki{Root: root, Cfg: cfg, LegacyTags: true}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	rep := TagAudit(w, scan)
	if !rep.Legacy {
		t.Error("legacy must be reported")
	}
	if len(rep.UnusedTopics) != 1 || rep.UnusedTopics[0] != "ghost" {
		t.Errorf("unused_topics = %v", rep.UnusedTopics)
	}
	if len(rep.OverbroadTopics) != 1 || rep.OverbroadTopics[0].Tag != "infrastructure" {
		t.Errorf("overbroad_topics = %+v", rep.OverbroadTopics)
	}
}
