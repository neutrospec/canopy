package wiki

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SetFrontmatterField rewrites (or inserts) a scalar field in the YAML
// frontmatter without touching any other line, preserving field order
// and unknown fields.
func SetFrontmatterField(content, key, value string) (string, error) {
	fm, body, ok := splitFrontmatter(content)
	if !ok {
		return "", fmt.Errorf("frontmatter missing or unterminated; refusing to normalize %s", key)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return "", fmt.Errorf("frontmatter: %w", err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("frontmatter must be a mapping")
	}
	var replacement yaml.Node
	if err := yaml.Unmarshal([]byte(value), &replacement); err != nil {
		return "", fmt.Errorf("invalid replacement for %s: %w", key, err)
	}
	if len(replacement.Content) != 1 || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("replacement for %s must fit one YAML line", key)
	}
	lines := strings.Split(fm, "\n")
	root := doc.Content[0]
	found := false
	for i := 0; i < len(root.Content); i += 2 {
		k, v := root.Content[i], root.Content[i+1]
		if k.Value != key {
			continue
		}
		if found {
			return "", fmt.Errorf("duplicate frontmatter key %s", key)
		}
		found = true
		if v.Kind != yaml.ScalarNode || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 || v.Anchor != "" || v.Line != k.Line || strings.ContainsAny(v.Value, "\r\n") {
			return "", fmt.Errorf("unsupported multiline, anchored, or complex frontmatter field %s; preserve it and edit explicitly", key)
		}
		line := lines[k.Line-1]
		prefix := key + ":"
		if !strings.HasPrefix(line, prefix) {
			return "", fmt.Errorf("unsupported frontmatter key syntax for %s", key)
		}
		comment := ""
		if v.LineComment != "" {
			comment = " " + v.LineComment
		}
		cr := ""
		if strings.HasSuffix(line, "\r") {
			cr = "\r"
		}
		lines[k.Line-1] = prefix + " " + value + comment + cr
	}
	if !found {
		fm = strings.TrimRight(fm, "\r\n") + "\n" + key + ": " + value + "\n"
	} else {
		fm = strings.Join(lines, "\n")
	}
	// Reuse the original delimiters/body, including CRLF and blank lines.
	start := strings.Index(content, "\n") + 1
	stop := len(content) - len(body)
	oldFM, _, _ := splitFrontmatter(content)
	return content[:start] + fm + content[start+len(oldFM):stop] + body, nil
}

// ReplaceBody swaps everything after the frontmatter block.
func ReplaceBody(content, newBody string) string {
	fm, _, ok := splitFrontmatter(content)
	if !ok {
		return newBody
	}
	return "---\n" + strings.TrimRight(fm, "\n") + "\n---\n\n" + strings.TrimLeft(newBody, "\n")
}

// linkPattern matches [[target]] and [[target|alias]] (with optional
// #anchor), case-insensitively on the target.
// RewriteLinks retargets wikilinks from oldSlug to newSlug, keeping
// anchors and aliases.
func RewriteLinks(content, oldSlug, newSlug string) string {
	oldKey := strings.ToLower(strings.TrimSpace(oldSlug))
	if strings.Contains(oldSlug, "/") {
		oldKey = NormalizeLink(oldSlug)
	}
	return OutsideCode(content, func(prose string) string {
		return wikilinkRe.ReplaceAllStringFunc(prose, func(link string) string {
			m := wikilinkRe.FindStringSubmatch(link)
			if NormalizeLink(m[1]) != oldKey {
				return link
			}
			target := strings.TrimSpace(m[1])
			prefix := ""
			if i := strings.LastIndex(target, "/"); i >= 0 {
				prefix = target[:i+1]
			}
			ext := ""
			if strings.HasSuffix(strings.ToLower(target), ".md") {
				ext = target[len(target)-3:]
			}
			if ext == "" && strings.HasSuffix(strings.ToLower(newSlug), ".md") {
				ext = ".md"
			}
			return "[[" + prefix + newSlug + ext + m[2] + m[3] + "]]"
		})
	})
}

// RewritePageLinks also moves explicit directory targets when a page changes
// type. Bare links remain bare; .md suffixes, aliases and anchors are retained.
func RewritePageLinks(content, oldRel, newRel string) string {
	return rewritePageLinks(content, oldRel, newRel, nil)
}

// RewritePageLinksInScan changes only links that resolved to oldRel before
// the move, and emits a target that remains unambiguous after that move.
func RewritePageLinksInScan(content, oldRel, newRel string, scan *ScanResult) string {
	return rewritePageLinks(content, oldRel, newRel, scan)
}

func rewritePageLinks(content, oldRel, newRel string, scan *ScanResult) string {
	oldSlug := NormalizeLink(oldRel)
	newSlug := NormalizeLink(newRel)
	lookupAfter := func(key string) string {
		if key == strings.ToLower(newSlug) {
			return newRel
		}
		if p := scan.BySlug[key]; p != nil && p.RelPath != oldRel {
			return p.RelPath
		}
		return ""
	}
	return OutsideCode(content, func(prose string) string {
		return wikilinkRe.ReplaceAllStringFunc(prose, func(link string) string {
			m := wikilinkRe.FindStringSubmatch(link)
			if scan != nil {
				p, err := scan.Find(m[1])
				if err != nil || p.RelPath != oldRel {
					return link
				}
			} else if NormalizeLink(m[1]) != oldSlug {
				return link
			}
			target := strings.TrimSpace(m[1])
			prefix := ""
			if strings.Contains(target, "/") {
				prefix = filepath.ToSlash(filepath.Dir(newRel)) + "/"
			}
			ext := ""
			if strings.HasSuffix(strings.ToLower(target), ".md") {
				ext = target[len(target)-3:]
			}
			if ext == "" && strings.HasSuffix(newSlug, ".md") {
				ext = ".md"
			}
			next := prefix + newSlug + ext
			if scan != nil && prefix == "" {
				// Evaluate Find's exact/extension aliases in the destination
				// namespace without rebuilding it for every inbound page.
				exact, normalized := lookupAfter(strings.ToLower(next)), lookupAfter(NormalizeLink(next))
				if (exact != "" && (exact != newRel || (normalized != "" && normalized != exact))) || (exact == "" && normalized != newRel) {
					next = filepath.ToSlash(newRel)
				}
			}
			return "[[" + next + m[2] + m[3] + "]]"
		})
	})
}

// StripLinks converts wikilinks to plain text: [[t]] → t, [[t|a]] → a.
// Used when the target page is archived or deleted.
func StripLinks(content, target string) string {
	return stripLinks(content, target, nil)
}

// StripPageLinksInScan preserves unresolved/ambiguous targets and removes only
// actual references to the page being removed.
func StripPageLinksInScan(content, targetRel string, scan *ScanResult) string {
	return stripLinks(content, targetRel, scan)
}

func stripLinks(content, target string, scan *ScanResult) string {
	targetKey := strings.ToLower(strings.TrimSpace(target))
	if strings.Contains(target, "/") {
		targetKey = NormalizeLink(target)
	}
	return OutsideCode(content, func(prose string) string {
		return wikilinkRe.ReplaceAllStringFunc(prose, func(m string) string {
			parts := wikilinkRe.FindStringSubmatch(m)
			if scan != nil {
				p, err := scan.Find(parts[1])
				if err != nil || p.RelPath != target {
					return m
				}
			} else if NormalizeLink(parts[1]) != targetKey {
				return m
			}
			inner := strings.TrimSuffix(strings.TrimPrefix(m, "[["), "]]")
			if i := strings.Index(inner, "|"); i >= 0 {
				return inner[i+1:]
			}
			if i := strings.Index(inner, "#"); i >= 0 {
				return inner[:i]
			}
			return inner
		})
	})
}

// NewPageContent renders a schema-compliant page.
func NewPageContent(title, typ, created string, tags, links []string, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: %s\ncreated: %s\nupdated: %s\ntype: %s\ntags: [%s]\nsources: []\n---\n\n",
		strconv.Quote(title), created, created, typ, strings.Join(tags, ", "))
	body = strings.TrimSpace(body)
	if body == "" {
		body = "# " + title
	} else if !strings.HasPrefix(body, "#") {
		body = "# " + title + "\n\n" + body
	}
	b.WriteString(body)
	b.WriteString("\n")
	if len(links) > 0 {
		b.WriteString("\n## 관련 페이지\n\n")
		for _, l := range links {
			fmt.Fprintf(&b, "- [[%s]]\n", l)
		}
	}
	return b.String()
}
