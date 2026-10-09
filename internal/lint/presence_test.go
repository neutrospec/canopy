package lint

import (
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

func TestFrontmatterKeyPresence(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	base := "---\ntitle: page\ntype: concept\ncreated: 2026-10-10\nupdated: 2026-10-10\ntags: []\n---\n\nBody\n"
	for _, key := range []string{"title", "type", "created", "updated", "tags"} {
		lines := strings.Split(base, "\n")
		var kept []string
		for _, line := range lines {
			if !strings.HasPrefix(line, key+":") {
				kept = append(kept, line)
			}
		}
		p := wiki.Parse("concepts/page.md", []byte(strings.Join(kept, "\n")))
		r := Run(w, &wiki.ScanResult{Pages: []*wiki.Page{p}, BySlug: map[string]*wiki.Page{"page": p}}, nil)
		if r.Counts["frontmatter-fields"] != 1 {
			t.Fatalf("missing %s not diagnosed: %+v", key, r.Counts)
		}
	}
	for _, value := range []string{"[]", "null", "tool"} {
		p := wiki.Parse("concepts/page.md", []byte(strings.Replace(base, "tags: []", "tags: "+value, 1)))
		r := Run(w, &wiki.ScanResult{Pages: []*wiki.Page{p}, BySlug: map[string]*wiki.Page{"page": p}}, nil)
		if r.Counts["frontmatter-fields"] != 0 || r.Counts["bad-frontmatter"] != 0 {
			t.Fatalf("tolerated tags %s rejected: %+v", value, r.Counts)
		}
	}
}

func TestTagAuditCountsPagesOnce(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	p := wiki.Parse("concepts/page.md", []byte("---\ntags: [tool, tool, tool]\n---\n"))
	r := TagAudit(w, &wiki.ScanResult{Pages: []*wiki.Page{p}})
	for _, u := range r.Topics {
		if u.Tag == "tool" && (u.Count != 1 || u.Pct != 100) {
			t.Fatalf("usage = %+v", u)
		}
	}
}
