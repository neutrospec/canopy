package webui

import (
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/wiki"
)

func TestRenderedWikilinksUseOriginalAddressAndExplicitDottedSlug(t *testing.T) {
	a := &wiki.Page{Slug: "foo", RelPath: "concepts/foo.md"}
	b := &wiki.Page{Slug: "foo.md", RelPath: "concepts/foo.md.md"}
	scan := &wiki.ScanResult{Pages: []*wiki.Page{a, b}, BySlug: map[string]*wiki.Page{"foo": a, "foo.md": b}}
	resolve := func(target string) (string, string, bool) {
		p, err := scan.Find(target)
		if err != nil {
			return wiki.NormalizeLink(target), target, false
		}
		ref := p.Slug
		if strings.HasSuffix(strings.ToLower(ref), ".md") {
			ref = p.RelPath
		}
		return strings.ToLower(p.Slug), ref, true
	}
	for _, tc := range []struct{ input, href, class string }{
		{"[[concepts/foo.md]]", "/page/foo", `class="wikilink"`},
		{"[[concepts/foo.md.md#section|Alias]]", "/page/concepts%2Ffoo.md.md#section", `class="wikilink"`},
		{"[[foo.md]]", "/page/foo.md", `class="wikilink missing"`},
		{"[[concepts/sub/foo.md]]", "/page/concepts%2Fsub%2Ffoo.md", `class="wikilink missing"`},
	} {
		got, err := RenderPageWithResolver(tc.input, resolve)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), `href="`+tc.href+`"`) || !strings.Contains(string(got), tc.class) {
			t.Fatalf("%s: %s", tc.input, got)
		}
	}
}
