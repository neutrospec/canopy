package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/mermaid"
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
	rep := Run(w, scan, mermaid.NewValidator())

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

// P1: a mermaid block the renderer would choke on is a critical finding;
// valid blocks and non-mermaid fences stay silent. nil validator skips.
func TestMermaidFindings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "concepts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntitle: %s\ntype: concept\ncreated: 2026-01-01\nupdated: 2026-01-01\ntags: []\n---\n\n"
	pages := map[string]string{
		"good.md": "본문 [[bad]]\n\n```mermaid\nflowchart LR\n  A[\"시작\"] --> B[\"끝\"]\n```\n\n```bash\necho '[not mermaid'\n```\n",
		"bad.md":  "본문 [[good]]\n\n```mermaid\nflowchart LR\n  A[깨진 (괄호)] --> end\n```\n",
	}
	for name, body := range pages {
		content := strings.Replace(fm, "%s", strings.TrimSuffix(name, ".md"), 1) + body
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}

	rep := Run(w, scan, mermaid.NewValidator())
	if rep.Counts["invalid-mermaid"] != 1 {
		t.Errorf("invalid-mermaid = %d, want 1\nfindings: %+v", rep.Counts["invalid-mermaid"], rep.Findings)
	}
	for _, f := range rep.Findings {
		if f.Kind == "invalid-mermaid" {
			if f.Page != "concepts/bad.md" {
				t.Errorf("finding on %s, want concepts/bad.md", f.Page)
			}
			if !strings.Contains(f.Message, "Parse error") {
				t.Errorf("message lacks parser diagnostic: %s", f.Message)
			}
		}
	}

	if rep := Run(w, scan, nil); rep.Counts["invalid-mermaid"] != 0 {
		t.Errorf("nil validator must skip mermaid checks, got %d", rep.Counts["invalid-mermaid"])
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
