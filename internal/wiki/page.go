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

	"github.com/neutrospec/canopy/internal/config"
)

// Page is one markdown page inside a schema-governed directory.
type Page struct {
	RelPath     string // path relative to wiki root, e.g. "concepts/foo.md"
	Slug        string // filename without .md, the wikilink target
	Dir         string // top-level directory ("concepts", ...)
	Title       string
	Type        string
	Created     string
	Updated     string
	Tags        []string
	Sources     []string
	Body        string   // content with frontmatter stripped
	Lines       int      // total line count of the file
	Links       []string // after Scan: resolved outbound slugs (lowercased)
	BrokenLinks []string // after Scan: original unresolved/ambiguous targets
	linkTargets []string

	HasFrontmatter bool
	FMErr          string          // non-empty if frontmatter failed to parse
	FMKeys         map[string]bool // presence differs from an empty value
}

func (p *Page) HasField(key string) bool { return p.FMKeys[key] }

var wikilinkRe = regexp.MustCompile(`\[\[([^\]\[|#]+)(#[^\]\[|]*)?(\|[^\]\[]*)?\]\]`)

// ExtractLinks returns normalized wikilink targets found in text.
// Fenced code blocks and inline code are ignored so bash conditionals
// like [[ "$x" == y ]] inside examples don't register as links.
func ExtractLinks(text string) []string {
	return normalizeTargets(extractLinkTargets(text))
}

func normalizeTargets(targets []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, target := range targets {
		t := NormalizeLink(target)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func extractLinkTargets(text string) []string {
	seen := map[string]bool{}
	var out []string
	OutsideCode(text, func(segment string) string {
		for _, m := range wikilinkRe.FindAllStringSubmatch(segment, -1) {
			t := strings.TrimSpace(m[1])
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			out = append(out, t)
		}
		return segment
	})
	return out
}

// NormalizeLink lowercases and trims a wikilink target so it can be
// compared against page slugs (Obsidian resolves links case-insensitively).
func NormalizeLink(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "#|"); i >= 0 {
		s = s[:i]
	}
	// Links may carry a path (e.g. concepts/foo); resolve by basename like Obsidian.
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimSuffix(s, ".md")
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
			p.FMKeys = map[string]bool{}
			for key := range m {
				p.FMKeys[key] = true
			}
			p.Title = coerceString(m["title"])
			p.Type = coerceString(m["type"])
			p.Created = coerceString(m["created"])
			p.Updated = coerceString(m["updated"])
			p.Tags = coerceStringList(m["tags"])
			p.Sources = coerceStringList(m["sources"])
		}
	}
	p.linkTargets = extractLinkTargets(p.Body)
	p.Links = normalizeTargets(p.linkTargets)
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
			if previous, exists := res.BySlug[strings.ToLower(p.Slug)]; exists {
				return nil, fmt.Errorf("ambiguous slug %q: %s and %s; resolve duplicate filenames before using canopy", p.Slug, previous.RelPath, p.RelPath)
			}
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

	res.ResolveLinks()
	sort.Slice(res.Pages, func(i, j int) bool { return res.Pages[i].RelPath < res.Pages[j].RelPath })
	return res, nil
}

// ResolveLinks refreshes the graph after every page in a snapshot is parsed.
// Callers that replace parsed pages must resolve again in that same namespace.
func (res *ScanResult) ResolveLinks() {
	// Resolve links only after the complete unique namespace is known. A
	// path-qualified or ambiguous link must not become an edge to an unrelated
	// basename merely because standalone normalization removed its path.
	for _, p := range res.Pages {
		p.Links = nil
		p.BrokenLinks = nil
		seen := map[string]bool{}
		for _, target := range p.linkTargets {
			to, err := res.Find(target)
			if err != nil {
				p.BrokenLinks = append(p.BrokenLinks, target)
				continue
			}
			slug := strings.ToLower(to.Slug)
			if !seen[slug] {
				seen[slug] = true
				p.Links = append(p.Links, slug)
			}
		}
	}
}

// Find uses the scanner's flat, unique-slug namespace. Explicit paths must name
// the actual governed file; nested pages are not implicitly made addressable.
func (r *ScanResult) Find(input string) (*Page, error) {
	name := strings.TrimSpace(input)
	if strings.HasPrefix(name, "[[") && strings.HasSuffix(name, "]]") {
		name = name[2 : len(name)-2]
	}
	if i := strings.IndexAny(name, "#|"); i >= 0 {
		name = name[:i]
	}
	if strings.Contains(name, "/") {
		expect := strings.ToLower(filepath.ToSlash(name))
		// An explicit filename wins over an extensionless path spelling.
		for _, p := range r.Pages {
			if strings.EqualFold(p.RelPath, expect) {
				return p, nil
			}
		}
		for _, p := range r.Pages {
			if strings.EqualFold(p.RelPath, expect+".md") {
				return p, nil
			}
		}
		return nil, fmt.Errorf("page path is not a flat governed page: %s", input)
	}
	exact, hasExact := r.BySlug[strings.ToLower(strings.TrimSpace(name))]
	p, ok := r.BySlug[NormalizeLink(name)]
	if hasExact && ok && exact != p {
		return nil, fmt.Errorf("ambiguous page reference %q: use explicit %s or %s", input, exact.RelPath, p.RelPath)
	}
	if hasExact {
		return exact, nil
	}
	if !ok {
		return nil, fmt.Errorf("page not found: %s", input)
	}
	return p, nil
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

// Components returns the connected components of the undirected
// wikilink graph, largest first (members sorted). The first component
// is the "mainland"; every other one is an island — internally linked
// pages that no path connects to the main knowledge network, which
// per-page orphan checks cannot see.
func (r *ScanResult) Components() [][]string {
	adj := map[string][]string{}
	for _, p := range r.Pages {
		from := strings.ToLower(p.Slug)
		if _, ok := adj[from]; !ok {
			adj[from] = nil
		}
		for _, target := range p.Links {
			if _, exists := r.BySlug[target]; exists && target != from {
				adj[from] = append(adj[from], target)
				adj[target] = append(adj[target], from)
			}
		}
	}
	slugs := make([]string, 0, len(adj))
	for s := range adj {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs) // deterministic traversal order
	seen := map[string]bool{}
	var comps [][]string
	for _, start := range slugs {
		if seen[start] {
			continue
		}
		var comp []string
		stack := []string{start}
		seen[start] = true
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			comp = append(comp, v)
			for _, w := range adj[v] {
				if !seen[w] {
					seen[w] = true
					stack = append(stack, w)
				}
			}
		}
		sort.Strings(comp)
		comps = append(comps, comp)
	}
	sort.SliceStable(comps, func(i, j int) bool { return len(comps[i]) > len(comps[j]) })
	return comps
}
