// Package webui serves the wiki over HTTP for human browsing.
// Reading is free-form; the only mutation is the body editor, which
// runs the same writeops.Run pipeline as the CLI so the write
// invariants cannot be bypassed (docs/web-ui-write-design.md).
package webui

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"

	"github.com/neutrospec/canopy/internal/wiki"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// The wiki is the user's own content and wikilink preprocessing
	// emits inline <a> tags, so raw HTML must pass through.
	goldmark.WithRendererOptions(ghtml.WithUnsafe()),
)

// Wikilink with optional #anchor and |alias. Same target charset as
// wiki.ExtractLinks, but capturing all three parts for display.
var linkRe = regexp.MustCompile(`\[\[([^\]\[|#]+)(#[^\]\[|]*)?(\|[^\]\[]*)?\]\]`)

// Code regions where wikilinks must be left verbatim (mirrors the
// exclusions in wiki.ExtractLinks).
var (
	fencedRe = regexp.MustCompile("(?ms)^\\s*```.*?^\\s*```\\s*$")
	inlineRe = regexp.MustCompile("`[^`\n]*`")
)

// RenderPage converts a page body to HTML. exists reports whether a
// normalized slug resolves to a page, so missing targets render as
// red links (Wikipedia pattern).
func RenderPage(body string, exists func(slug string) bool) (template.HTML, error) {
	src := replaceWikilinks(body, exists)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
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
	var out strings.Builder
	last := 0
	for _, loc := range fencedRe.FindAllStringIndex(src, -1) {
		out.WriteString(applyOutsideInline(src[last:loc[0]], f))
		out.WriteString(src[loc[0]:loc[1]])
		last = loc[1]
	}
	out.WriteString(applyOutsideInline(src[last:], f))
	return out.String()
}

func applyOutsideInline(seg string, f func(string) string) string {
	var out strings.Builder
	last := 0
	for _, loc := range inlineRe.FindAllStringIndex(seg, -1) {
		out.WriteString(f(seg[last:loc[0]]))
		out.WriteString(seg[loc[0]:loc[1]])
		last = loc[1]
	}
	out.WriteString(f(seg[last:]))
	return out.String()
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
