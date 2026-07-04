// Package wiki models pages: frontmatter, wikilinks, and filesystem scanning.
package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nobocop/canopy/internal/config"
)

// Page is one markdown page inside a schema-governed directory.
type Page struct {
	RelPath string // path relative to wiki root, e.g. "concepts/foo.md"
	Slug    string // filename without .md, the wikilink target
	Dir     string // top-level directory ("concepts", ...)
	Title   string
	Type    string
	Created string
	Updated string
	Tags    []string
	Sources []string
	Body    string   // content with frontmatter stripped
	Lines   int      // total line count of the file
	Links   []string // outbound wikilink targets, normalized (lowercased, no alias/anchor)

	HasFrontmatter bool
	FMErr          string // non-empty if frontmatter failed to parse
}

var (
	wikilinkRe   = regexp.MustCompile(`\[\[([^\]\[|#]+)(?:#[^\]\[|]*)?(?:\|[^\]\[]*)?\]\]`)
	fencedCodeRe = regexp.MustCompile("(?ms)^\\s*```.*?^\\s*```\\s*$")
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
)

// ExtractLinks returns normalized wikilink targets found in text.
// Fenced code blocks and inline code are ignored so bash conditionals
// like [[ "$x" == y ]] inside examples don't register as links.
func ExtractLinks(text string) []string {
	text = fencedCodeRe.ReplaceAllString(text, "")
	text = inlineCodeRe.ReplaceAllString(text, "")
	seen := map[string]bool{}
	var out []string
	for _, m := range wikilinkRe.FindAllStringSubmatch(text, -1) {
		t := NormalizeLink(m[1])
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// NormalizeLink lowercases and trims a wikilink target so it can be
// compared against page slugs (Obsidian resolves links case-insensitively).
func NormalizeLink(s string) string {
	s = strings.TrimSpace(s)
	// Links may carry a path (e.g. concepts/foo); resolve by basename like Obsidian.
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".md")
	return strings.ToLower(s)
}

// Parse builds a Page from raw file bytes.
func Parse(relPath string, raw []byte) *Page {
	p := &Page{
		RelPath: filepath.ToSlash(relPath),
		Slug:    strings.TrimSuffix(filepath.Base(relPath), ".md"),
	}
	if i := strings.Index(p.RelPath, "/"); i >= 0 {
		p.Dir = p.RelPath[:i]
	}
	content := string(raw)
	p.Lines = strings.Count(content, "\n") + 1
	p.Body = content

	fm, body, ok := splitFrontmatter(content)
	if ok {
		p.HasFrontmatter = true
		p.Body = body
		var m map[string]any
		if err := yaml.Unmarshal([]byte(fm), &m); err != nil {
			p.FMErr = err.Error()
		} else {
			p.Title = coerceString(m["title"])
			p.Type = coerceString(m["type"])
			p.Created = coerceString(m["created"])
			p.Updated = coerceString(m["updated"])
			p.Tags = coerceStringList(m["tags"])
			p.Sources = coerceStringList(m["sources"])
		}
	}
	p.Links = ExtractLinks(p.Body)
	return p
}

func splitFrontmatter(content string) (fm, body string, ok bool) {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", content, false
	}
	rest := content[strings.Index(content, "\n")+1:]
	// Closing delimiter: a line that is exactly "---".
	closeRe := regexp.MustCompile(`(?m)^---\s*$`)
	loc := closeRe.FindStringIndex(rest)
	if loc == nil {
		return "", content, false
	}
	fm = rest[:loc[0]]
	body = rest[loc[1]:]
	body = strings.TrimPrefix(body, "\n")
	return fm, body, true
}

func coerceString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case time.Time:
		// yaml.v3 resolves unquoted YYYY-MM-DD to time.Time.
		return t.Format("2006-01-02")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func coerceStringList(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			s := coerceString(e)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}

// ScanResult is a full snapshot of the wiki's governed pages plus
// any stray .md files found where they don't belong.
type ScanResult struct {
	Pages      []*Page
	BySlug     map[string]*Page // lowercased slug -> page
	StrayRoot  []string         // .md files in root not in RootFiles allowlist
	IndexPages []string         // files under index/ (generated, not governed)
}

// Scan walks the wiki and parses all governed pages.
func Scan(w *config.Wiki) (*ScanResult, error) {
	res := &ScanResult{BySlug: map[string]*Page{}}

	entries, err := os.ReadDir(w.Root)
	if err != nil {
		return nil, err
	}
	rootAllowed := map[string]bool{}
	for _, f := range w.Cfg.Schema.RootFiles {
		rootAllowed[f] = true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !rootAllowed[e.Name()] {
			res.StrayRoot = append(res.StrayRoot, e.Name())
		}
	}

	for _, dir := range w.Cfg.Schema.PageDirs {
		dirPath := filepath.Join(w.Root, dir)
		files, err := os.ReadDir(dirPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dirPath, f.Name()))
			if err != nil {
				return nil, err
			}
			p := Parse(filepath.Join(dir, f.Name()), raw)
			res.Pages = append(res.Pages, p)
			res.BySlug[strings.ToLower(p.Slug)] = p
		}
	}

	if files, err := os.ReadDir(filepath.Join(w.Root, "index")); err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				res.IndexPages = append(res.IndexPages, "index/"+f.Name())
			}
		}
	}

	sort.Slice(res.Pages, func(i, j int) bool { return res.Pages[i].RelPath < res.Pages[j].RelPath })
	return res, nil
}

// Backlinks returns inbound link sources (slugs) for every page,
// counting only links that originate from governed pages.
func (r *ScanResult) Backlinks() map[string][]string {
	in := map[string][]string{}
	for _, p := range r.Pages {
		from := strings.ToLower(p.Slug)
		for _, target := range p.Links {
			if _, exists := r.BySlug[target]; exists && target != from {
				in[target] = append(in[target], p.Slug)
			}
		}
	}
	for k := range in {
		sort.Strings(in[k])
	}
	return in
}
