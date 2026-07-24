package webui

import (
	"sort"
	"strings"

	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Loose link suggestions (docs/web-ui-plan-2.md D5): pages that are
// semantically close but not yet linked in either direction — the
// page-level version of `canopy bridge`. Rendered visually distinct
// from explicit links: the wiki asserting a connection vs guessing one.

const (
	suggestMinSim = 0.7 // matches bridge --min-sim default
	suggestMax    = 5
)

type suggestion struct {
	Slug  string
	Title string
	Sim   float64
	Tags  int // shared-tag count (ranking boost, shown as context)
}

func (s *Server) suggestLinks(scan *wiki.ScanResult, p *wiki.Page, backlinks []string) []suggestion {
	st, err := store.Open(s.w.DBPath())
	if err != nil {
		return nil
	}
	vectors, err := st.PageVectors()
	st.Close()
	if err != nil {
		return nil
	}
	self, ok := vectors[strings.ToLower(p.Slug)]
	if !ok {
		return nil
	}
	linked := map[string]bool{strings.ToLower(p.Slug): true}
	for _, l := range p.Links {
		linked[l] = true
	}
	for _, b := range backlinks {
		linked[strings.ToLower(b)] = true
	}
	selfTags := map[string]bool{}
	for _, t := range p.Tags {
		selfTags[t] = true
	}

	var out []suggestion
	for slug, v := range vectors {
		if linked[slug] {
			continue
		}
		other, exists := scan.BySlug[slug]
		if !exists { // stale vector for a deleted page
			continue
		}
		sim := store.Cosine(self, v)
		if sim < suggestMinSim {
			continue
		}
		shared := 0
		for _, t := range other.Tags {
			if selfTags[t] {
				shared++
			}
		}
		out = append(out, suggestion{Slug: other.Slug, Title: other.Title, Sim: sim, Tags: shared})
	}
	// Cosine leads; shared tags break near-ties (same ranking idea as
	// rankRelated in `canopy new`).
	sort.Slice(out, func(i, j int) bool {
		si, sj := out[i].Sim+0.01*float64(out[i].Tags), out[j].Sim+0.01*float64(out[j].Tags)
		if si != sj {
			return si > sj
		}
		return out[i].Slug < out[j].Slug
	})
	if len(out) > suggestMax {
		out = out[:suggestMax]
	}
	return out
}
