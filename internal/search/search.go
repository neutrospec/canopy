// Package search fuses keyword (BM25) and semantic (cosine) rankings.
package search

import (
	"sort"
	"strings"

	"github.com/neutrospec/canopy/internal/store"
)

// rrfK is the standard Reciprocal Rank Fusion damping constant.
const rrfK = 60.0

// Fuse merges ranked lists with RRF: score = Σ 1/(60+rank). Snippets
// prefer the keyword list (it highlights matched terms).
func Fuse(k int, lists ...[]store.Hit) []store.Hit {
	if store.ValidateK(k) != nil {
		return nil
	}
	type acc struct {
		hit   store.Hit
		score float64
	}
	merged := map[string]*acc{}
	for _, list := range lists {
		for rank, h := range list {
			a, ok := merged[h.Slug]
			if !ok {
				a = &acc{hit: h}
				merged[h.Slug] = a
			} else if a.hit.Snippet == "" {
				a.hit.Snippet = h.Snippet
			}
			a.score += 1.0 / (rrfK + float64(rank+1))
		}
	}
	out := make([]store.Hit, 0, len(merged))
	for _, a := range merged {
		a.hit.Score = a.score
		out = append(out, a.hit)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if a, b := strings.ToLower(out[i].Slug), strings.ToLower(out[j].Slug); a != b {
			return a < b
		}
		return out[i].Slug < out[j].Slug
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}
