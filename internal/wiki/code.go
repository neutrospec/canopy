package wiki

import (
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type codeRange struct{ start, stop int }

// OutsideCode applies f to prose, preserving frontmatter and CommonMark code
// blocks/spans byte for byte. Extraction, link edits, and HTML preprocessing use
// the same parser boundaries (nested containers, tilde/long fences included).
func OutsideCode(src string, f func(string) string) string {
	body := src
	offset := 0
	if _, b, ok := splitFrontmatter(src); ok {
		body = b
		offset = len(src) - len(b)
	}
	data := []byte(body)
	root := goldmark.New().Parser().Parse(text.NewReader(data))
	var ranges []codeRange
	ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.FencedCodeBlock:
			if v.Info != nil {
				s := v.Info.Segment
				ranges = append(ranges, codeRange{s.Start, s.Stop})
			}
			for i := 0; i < v.Lines().Len(); i++ {
				s := v.Lines().At(i)
				ranges = append(ranges, codeRange{s.Start, s.Stop})
			}
		case *ast.CodeBlock:
			for i := 0; i < v.Lines().Len(); i++ {
				s := v.Lines().At(i)
				ranges = append(ranges, codeRange{s.Start, s.Stop})
			}
		case *ast.CodeSpan:
			for c := v.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					s := t.Segment
					ranges = append(ranges, codeRange{s.Start, s.Stop})
				}
			}
		}
		return ast.WalkContinue, nil
	})
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	var out strings.Builder
	out.WriteString(src[:offset])
	pos := 0
	for _, r := range ranges {
		if r.start < pos {
			if r.stop > pos {
				out.WriteString(body[pos:r.stop])
				pos = r.stop
			}
			continue
		}
		out.WriteString(f(body[pos:r.start]))
		out.WriteString(body[r.start:r.stop])
		pos = r.stop
	}
	out.WriteString(f(body[pos:]))
	return out.String()
}
