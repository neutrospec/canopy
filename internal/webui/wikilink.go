package webui

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var wikiResolveKey = parser.NewContextKey()
var wikiLinkKind = ast.NewNodeKind("CanopyWikiLink")

type wikiLinkNode struct {
	ast.BaseInline
	slug, reference, anchor, display string
	exists                           bool
}

func (n *wikiLinkNode) Kind() ast.NodeKind            { return wikiLinkKind }
func (n *wikiLinkNode) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// Parse inside Goldmark's inline grammar. Fences, indented code and code spans
// remain byte-for-byte source, with no raw-HTML escape hatch for wiki content.
type wikiLinkParser struct{}

func (wikiLinkParser) Trigger() []byte { return []byte{'['} }
func (wikiLinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	loc := linkRe.FindIndex(line)
	if loc == nil || loc[0] != 0 {
		return nil
	}
	g := linkRe.FindStringSubmatch(string(line[:loc[1]]))
	target, anchor, alias := strings.TrimSpace(g[1]), g[2], strings.TrimPrefix(g[3], "|")
	slug := wiki.NormalizeLink(target)
	display := target + anchor
	if alias != "" {
		display = alias
	}
	n := &wikiLinkNode{slug: slug, reference: target, anchor: strings.TrimPrefix(anchor, "#"), display: display}
	if resolve, ok := pc.Get(wikiResolveKey).(WikiLinkResolver); ok && resolve != nil {
		n.slug, n.reference, n.exists = resolve(target)
	}
	block.Advance(loc[1])
	return n
}

type wikiLinkRenderer struct{}

func (wikiLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikiLinkKind, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		n := node.(*wikiLinkNode)
		class := "wikilink"
		if !n.exists {
			class += " missing"
		}
		href := "/page/" + url.PathEscape(n.reference)
		if n.anchor != "" {
			href += "#" + url.PathEscape(n.anchor)
		}
		_, err := fmt.Fprintf(w, `<a class="%s" href="%s" data-slug="%s">%s</a>`, class, html.EscapeString(href), html.EscapeString(n.slug), html.EscapeString(n.display))
		return ast.WalkSkipChildren, err
	})
}
