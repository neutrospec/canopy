package wiki

import (
	"fmt"
	"regexp"
	"strings"
)

// SetFrontmatterField rewrites (or inserts) a scalar field in the YAML
// frontmatter without touching any other line, preserving field order
// and unknown fields.
func SetFrontmatterField(content, key, value string) string {
	fm, body, ok := splitFrontmatter(content)
	if !ok {
		return content
	}
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:.*$`)
	if re.MatchString(fm) {
		fm = re.ReplaceAllString(fm, key+": "+value)
	} else {
		fm = strings.TrimRight(fm, "\n") + "\n" + key + ": " + value + "\n"
	}
	return "---\n" + strings.TrimRight(fm, "\n") + "\n---\n" + body
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
func linkPattern(target string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\[\[` + regexp.QuoteMeta(target) + `(#[^\]\[|]*)?(\|[^\]\[]*)?\]\]`)
}

// RewriteLinks retargets wikilinks from oldSlug to newSlug, keeping
// anchors and aliases.
func RewriteLinks(content, oldSlug, newSlug string) string {
	return linkPattern(oldSlug).ReplaceAllString(content, "[["+newSlug+"$1$2]]")
}

// StripLinks converts wikilinks to plain text: [[t]] → t, [[t|a]] → a.
// Used when the target page is archived or deleted.
func StripLinks(content, target string) string {
	return linkPattern(target).ReplaceAllStringFunc(content, func(m string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(m, "[["), "]]")
		if i := strings.Index(inner, "|"); i >= 0 {
			return inner[i+1:]
		}
		if i := strings.Index(inner, "#"); i >= 0 {
			return inner[:i]
		}
		return inner
	})
}

// NewPageContent renders a schema-compliant page.
func NewPageContent(title, typ, created string, tags, links []string, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: \"%s\"\ncreated: %s\nupdated: %s\ntype: %s\ntags: [%s]\nsources: []\n---\n\n",
		strings.ReplaceAll(title, `"`, `\"`), created, created, typ, strings.Join(tags, ", "))
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
