// Package webui serves the wiki over HTTP for human browsing.
// The editor queues proposals; agents apply them through the CLI pipeline.
package webui

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/mermaid"

	"github.com/neutrospec/canopy/internal/wiki"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Footnote,
		// Server-side syntax highlighting: chroma with CSS classes so
		// static/chroma.css can restyle tokens for light and dark.
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
		),
		// ```mermaid blocks become <pre class="mermaid">; app.js lazy-
		// loads the vendored script only when a page contains one.
		&mermaid.Extender{RenderMode: mermaid.RenderModeClient, NoScript: true},
	),
	goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithInlineParsers(util.Prioritized(wikiLinkParser{}, 100))),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wikiLinkRenderer{}, 100))),
)

// Wikilink with optional #anchor and |alias. Same target charset as
// wiki.ExtractLinks, but capturing all three parts for display.
var linkRe = regexp.MustCompile(`\[\[([^\]\[|#]+)(#[^\]\[|]*)?(\|[^\]\[]*)?\]\]`)

// RenderPage converts a page body to HTML. exists reports whether a
// normalized slug resolves to a page, so missing targets render as
// red links (Wikipedia pattern).
func RenderPage(body string, exists func(slug string) bool) (template.HTML, error) {
	return RenderPageWithResolver(body, func(target string) (string, string, bool) {
		slug := wiki.NormalizeLink(target)
		return slug, slug, exists != nil && exists(slug)
	})
}

// WikiLinkResolver resolves the original target with the same namespace as
// page lookup. reference can be a governed path to disambiguate dotted slugs.
type WikiLinkResolver func(target string) (slug, reference string, found bool)

func RenderPageWithResolver(body string, resolve WikiLinkResolver) (template.HTML, error) {
	ctx := parser.NewContext()
	ctx.Set(wikiResolveKey, resolve)
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf, parser.WithContext(ctx)); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// replaceWikilinks rewrites [[target#anchor|alias]] to inline anchor
// tags, skipping fenced and inline code regions.
func replaceWikilinks(src string, exists func(string) bool) string {
	return outsideCode(src, func(seg string) string {
		return linkRe.ReplaceAllStringFunc(seg, func(m string) string {
			g := linkRe.FindStringSubmatch(m)
			target, anchor, alias := strings.TrimSpace(g[1]), g[2], strings.TrimPrefix(g[3], "|")
			slug := wiki.NormalizeLink(target)
			display := target
			if anchor != "" {
				display += anchor
			}
			if alias != "" {
				display = alias
			}
			class := "wikilink"
			if !exists(slug) {
				class = "wikilink missing"
			}
			return fmt.Sprintf(`<a class=%q href="/page/%s%s" data-slug=%q>%s</a>`,
				class, html.EscapeString(slug), html.EscapeString(anchor), html.EscapeString(slug), html.EscapeString(display))
		})
	})
}

// outsideCode applies f to the parts of src not inside fenced blocks
// or inline code spans.
func outsideCode(src string, f func(string) string) string {
	return wiki.OutsideCode(src, f)
}

// FirstParagraph returns a plain-text excerpt of a page body for
// previews and search fallbacks.
func FirstParagraph(body string, maxRunes int) string {
	for _, para := range strings.Split(body, "\n\n") {
		t := strings.TrimSpace(para)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") ||
			strings.HasPrefix(t, ">") || strings.HasPrefix(t, "---") {
			continue
		}
		t = linkRe.ReplaceAllString(t, "$1")
		t = strings.NewReplacer("**", "", "*", "", "`", "").Replace(t)
		r := []rune(strings.Join(strings.Fields(t), " "))
		if len(r) > maxRunes {
			return string(r[:maxRunes]) + "…"
		}
		return string(r)
	}
	return ""
}
