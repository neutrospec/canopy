package wiki

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func fixtureWiki(t *testing.T) *config.Wiki {
	t.Helper()
	root, err := filepath.Abs("../../testdata/fixture-wiki")
	if err != nil {
		t.Fatal(err)
	}
	return &config.Wiki{Root: root, Cfg: config.Default()}
}

func TestExtractLinks(t *testing.T) {
	body := "See [[opc-ua]], [[Concepts/Foo Bar|display]], [[page#section]], [[opc-ua]] again."
	got := ExtractLinks(body)
	want := []string{"opc-ua", "foo bar", "page"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("link %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseFrontmatter(t *testing.T) {
	raw := []byte("---\ntitle: \"Test\"\ncreated: 2026-01-01\nupdated: 2026-01-02\ntype: concept\ntags: [tool, ai-ml]\n---\n\nbody [[link-a]]\n")
	p := Parse("concepts/test.md", raw)
	if !p.HasFrontmatter || p.FMErr != "" {
		t.Fatalf("frontmatter not parsed: %+v", p)
	}
	if p.Title != "Test" || p.Type != "concept" || p.Created != "2026-01-01" {
		t.Errorf("fields wrong: %+v", p)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "tool" {
		t.Errorf("tags wrong: %v", p.Tags)
	}
	if len(p.Links) != 1 || p.Links[0] != "link-a" {
		t.Errorf("links wrong: %v", p.Links)
	}
}

func TestScanAndBacklinks(t *testing.T) {
	scan, err := Scan(fixtureWiki(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Pages) != 4 {
		t.Fatalf("want 4 pages, got %d", len(scan.Pages))
	}
	if len(scan.StrayRoot) != 1 || scan.StrayRoot[0] != "stray-page.md" {
		t.Errorf("stray root detection failed: %v", scan.StrayRoot)
	}
	in := scan.Backlinks()
	if n := len(in["opc-ua"]); n != 2 {
		t.Errorf("opc-ua should have 2 backlinks, got %d", n)
	}
	if n := len(in["orphan-note"]); n != 0 {
		t.Errorf("orphan-note should have 0 backlinks, got %d", n)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Windows RDP Remote Access Guide": "windows-rdp-remote-access-guide",
		"태국 입국 정보":                        "", // pure Korean → caller must ask for explicit slug
		"OPC-UA란? (개요)":                   "opc-ua",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComponents(t *testing.T) {
	mk := func(slug string, links ...string) *Page {
		body := ""
		for _, l := range links {
			body += "[[" + l + "]] "
		}
		return Parse("concepts/"+slug+".md", []byte("---\ntitle: \""+slug+"\"\ntype: concept\n---\n"+body))
	}
	res := &ScanResult{BySlug: map[string]*Page{}}
	for _, p := range []*Page{
		mk("a", "b"), mk("b", "c"), mk("c"), // mainland: a-b-c
		mk("x", "y"), mk("y"), // island: x-y
		mk("solo"), // orphan island of one
	} {
		res.Pages = append(res.Pages, p)
		res.BySlug[strings.ToLower(p.Slug)] = p
	}
	comps := res.Components()
	if len(comps) != 3 {
		t.Fatalf("expected 3 components, got %d: %v", len(comps), comps)
	}
	if len(comps[0]) != 3 || comps[0][0] != "a" {
		t.Errorf("mainland wrong: %v", comps[0])
	}
	if len(comps[1]) != 2 || comps[1][0] != "x" {
		t.Errorf("island wrong: %v", comps[1])
	}
	if len(comps[2]) != 1 || comps[2][0] != "solo" {
		t.Errorf("solo island wrong: %v", comps[2])
	}
}
