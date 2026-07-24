package webui

import (
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/wiki"
)

// --- faceted browsing (dir × type × tags) ---

type facet struct {
	Value  string
	Count  int
	Active bool
	Href   string
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	q := r.URL.Query()
	dir, typ := q.Get("dir"), q.Get("type")
	tags := q["tag"]

	match := func(p *wiki.Page) bool {
		if dir != "" && p.Dir != dir {
			return false
		}
		if typ != "" && p.Type != typ {
			return false
		}
		for _, t := range tags {
			if !containsStr(p.Tags, t) {
				return false
			}
		}
		return true
	}
	var pages []*wiki.Page
	for _, p := range scan.Pages {
		if match(p) {
			pages = append(pages, p)
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Updated > pages[j].Updated })

	// Facet chips: counts within the current result set, so every chip
	// is a meaningful refinement (faceted classification, not a tree).
	link := func(newDir, newType string, newTags []string) string {
		v := url.Values{}
		if newDir != "" {
			v.Set("dir", newDir)
		}
		if newType != "" {
			v.Set("type", newType)
		}
		for _, t := range newTags {
			v.Add("tag", t)
		}
		if enc := v.Encode(); enc != "" {
			return "/browse?" + enc
		}
		return "/browse"
	}
	dirCount, typeCount, tagCount := map[string]int{}, map[string]int{}, map[string]int{}
	for _, p := range pages {
		dirCount[p.Dir]++
		typeCount[p.Type]++
		for _, t := range p.Tags {
			if !containsStr(tags, t) {
				tagCount[t]++
			}
		}
	}
	var dirFacets, typeFacets, tagFacets []facet
	for _, d := range s.w.Cfg.Schema.PageDirs {
		if d == dir {
			dirFacets = append(dirFacets, facet{d, dirCount[d], true, link("", typ, tags)})
		} else if dirCount[d] > 0 {
			dirFacets = append(dirFacets, facet{d, dirCount[d], false, link(d, typ, tags)})
		}
	}
	for _, t := range s.w.Cfg.Schema.Types {
		if t == typ {
			typeFacets = append(typeFacets, facet{t, typeCount[t], true, link(dir, "", tags)})
		} else if typeCount[t] > 0 {
			typeFacets = append(typeFacets, facet{t, typeCount[t], false, link(dir, t, tags)})
		}
	}
	for _, t := range tags {
		var rest []string
		for _, o := range tags {
			if o != t {
				rest = append(rest, o)
			}
		}
		tagFacets = append(tagFacets, facet{t, len(pages), true, link(dir, typ, rest)})
	}
	var coTags []string
	for t := range tagCount {
		coTags = append(coTags, t)
	}
	sort.Slice(coTags, func(i, j int) bool {
		if tagCount[coTags[i]] != tagCount[coTags[j]] {
			return tagCount[coTags[i]] > tagCount[coTags[j]]
		}
		return coTags[i] < coTags[j]
	})
	for _, t := range coTags {
		tagFacets = append(tagFacets, facet{t, tagCount[t], false, link(dir, typ, append(append([]string{}, tags...), t))})
	}

	s.render(w, http.StatusOK, "browse.html", map[string]any{
		"Title":    "browse",
		"Pages":    pages,
		"Dirs":     dirFacets,
		"Types":    typeFacets,
		"Tags":     tagFacets,
		"Filtered": dir != "" || typ != "" || len(tags) > 0,
	})
}

// handleTag is the wiki-style category page: an alias into /browse.
func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/browse?tag="+url.QueryEscape(r.PathValue("tag")), http.StatusFound)
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// --- special pages ---

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	entries, err := logops.ReadRecent(s.w, 100)
	if err != nil {
		s.fail(w, err)
		return
	}
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	type row struct {
		Time, Action, Slug, Note string
		Exists                   bool
	}
	rows := make([]row, 0, len(entries))
	for _, e := range entries {
		slug := wiki.NormalizeLink(e.File)
		_, exists := scan.BySlug[slug]
		if e.File == "" {
			slug, exists = "", false
		}
		rows = append(rows, row{
			Time:   strings.Replace(e.Timestamp, "T", " ", 1),
			Action: e.Action,
			Slug:   slug,
			Note:   e.Note,
			Exists: exists,
		})
	}
	s.render(w, http.StatusOK, "recent.html", map[string]any{"Title": "최근 변경", "Rows": rows})
}

// handleAttention lists orphans and stale pages — the same signals
// `canopy backlinks --orphans` and resurface use, as a browsable page.
func (s *Server) handleAttention(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	in := scan.Backlinks()
	var orphans []*wiki.Page
	for _, p := range scan.Pages {
		if len(in[strings.ToLower(p.Slug)]) == 0 {
			orphans = append(orphans, p)
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Slug < orphans[j].Slug })

	type stalePage struct {
		Page *wiki.Page
		Days int
	}
	staleDays := s.w.Cfg.Schema.StaleDays
	var stale []stalePage
	now := time.Now()
	for _, p := range scan.Pages {
		touched, err := time.Parse("2006-01-02", firstNonEmpty(p.Updated, p.Created))
		if err != nil {
			continue
		}
		if d := int(now.Sub(touched).Hours() / 24); d >= staleDays {
			stale = append(stale, stalePage{p, d})
		}
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].Days > stale[j].Days })

	s.render(w, http.StatusOK, "attention.html", map[string]any{
		"Title":     "점검",
		"Orphans":   orphans,
		"Stale":     stale,
		"StaleDays": staleDays,
	})
}

func (s *Server) handleRandom(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if len(scan.Pages) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	p := scan.Pages[rand.Intn(len(scan.Pages))]
	http.Redirect(w, r, "/page/"+p.Slug, http.StatusFound)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// --- local graph (page + direct neighbors, server-rendered SVG) ---

type graphNode struct {
	Slug    string
	X, Y    float64
	Missing bool
}

type graphEdge struct {
	X1, Y1, X2, Y2 float64
}

// localGraph lays the page's neighbors on a circle around it. No
// physics: at one hop, a radial layout is always readable.
func localGraph(scan *wiki.ScanResult, p *wiki.Page, inbound []string) ([]graphNode, []graphEdge) {
	seen := map[string]bool{strings.ToLower(p.Slug): true}
	var neighbors []graphNode
	add := func(slug string) {
		key := strings.ToLower(slug)
		if seen[key] || len(neighbors) >= 14 {
			return
		}
		seen[key] = true
		_, exists := scan.BySlug[key]
		neighbors = append(neighbors, graphNode{Slug: slug, Missing: !exists})
	}
	for _, t := range inbound {
		add(t)
	}
	for _, t := range p.Links {
		add(t)
	}
	if len(neighbors) == 0 {
		return nil, nil
	}
	const cx, cy, rx, ry = 320.0, 130.0, 250.0, 95.0
	nodes := []graphNode{{Slug: p.Slug, X: cx, Y: cy}}
	var edges []graphEdge
	for i := range neighbors {
		ang := 2 * math.Pi * float64(i) / float64(len(neighbors))
		neighbors[i].X = cx + rx*math.Cos(ang)
		neighbors[i].Y = cy + ry*math.Sin(ang)
		edges = append(edges, graphEdge{cx, cy, neighbors[i].X, neighbors[i].Y})
	}
	return append(nodes, neighbors...), edges
}

// short trims a slug for graph labels.
func short(s string) string {
	if r := []rune(s); len(r) > 22 {
		return string(r[:20]) + "…"
	}
	return s
}
