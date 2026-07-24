package webui

import (
	"net/http"
	"strings"

	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Global knowledge graph (Obsidian-style): every page is a node, every
// resolved wikilink an edge. The client (static/graph.js + vendored
// force-graph) does force layout, zoom/pan, drag, hover highlighting.

type graphAPINode struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Dir    string `json:"dir"`
	Deg    int    `json:"deg"`    // total degree — drives node size
	Read   bool   `json:"read"`   // ties into the read-history loop
	Island bool   `json:"island"` // outside the largest connected component
}

type graphAPILink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func (s *Server) handleAPIGraph(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	deg := map[string]int{}
	var links []graphAPILink
	seen := map[string]bool{}
	for _, p := range scan.Pages {
		from := strings.ToLower(p.Slug)
		for _, target := range p.Links {
			if _, ok := scan.BySlug[target]; !ok || target == from {
				continue
			}
			key := from + "→" + target
			if seen[key] {
				continue
			}
			seen[key] = true
			links = append(links, graphAPILink{Source: from, Target: target})
			deg[from]++
			deg[target]++
		}
	}
	islands := map[string]bool{}
	if comps := scan.Components(); len(comps) > 1 {
		for _, comp := range comps[1:] {
			for _, slug := range comp {
				islands[slug] = true
			}
		}
	}
	nodes := make([]graphAPINode, 0, len(scan.Pages))
	for _, p := range scan.Pages {
		slug := strings.ToLower(p.Slug)
		nodes = append(nodes, graphAPINode{
			ID:     slug,
			Title:  p.Title,
			Dir:    p.Dir,
			Deg:    deg[slug],
			Read:   rs.IsRead(slug),
			Island: islands[slug],
		})
	}
	s.emit(w, map[string]any{"nodes": nodes, "links": links})
}

func (s *Server) handleGraphPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "graph.html", map[string]any{"Title": "그래프"})
}
