package resurface

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Thresholds from the user's 2026-06 Distill & Express design.
const (
	forgottenAfter = 30 * 24 * time.Hour // strategy 1: untouched this long
	staleHubAfter  = 60 * 24 * time.Hour // strategy 2: hubs untouched this long
	minHubLinks    = 4                   // strategy 2: inbound links to count as a hub
)

// Pick is one resurface candidate with the evidence the agent needs to
// phrase a message without re-reading the whole wiki.
type Pick struct {
	Slug        string   `json:"slug"`
	RelPath     string   `json:"rel_path"`
	Title       string   `json:"title"`
	Strategy    string   `json:"strategy"` // random-forgotten | stale-hub
	DaysStale   int      `json:"days_stale"`
	Backlinks   []string `json:"backlinks,omitempty"`
	Outbound    []string `json:"outbound,omitempty"`
	Excerpt     string   `json:"excerpt"`
	Explanation string   `json:"explanation"`
}

// PickPages selects up to n resurface candidates.
// strategy: "random" (forgotten), "hub" (stale-but-important), or
// "auto" (70% random / 30% hub, per the original design).
func PickPages(scan *wiki.ScanResult, st *State, strategy string, n int, now time.Time, rng *rand.Rand) ([]Pick, error) {
	backlinks := scan.Backlinks()

	type cand struct {
		page  *wiki.Page
		stale time.Duration
	}
	var forgotten, hubs []cand
	for _, p := range scan.Pages {
		if !st.eligible(normalizeSlug(p.Slug), now) {
			continue
		}
		touched, err := time.Parse("2006-01-02", firstNonEmpty(p.Updated, p.Created))
		if err != nil {
			continue // unparseable dates never enter the pool
		}
		stale := now.Sub(touched)
		if stale >= forgottenAfter {
			forgotten = append(forgotten, cand{p, stale})
		}
		if stale >= staleHubAfter && len(backlinks[normalizeSlug(p.Slug)]) >= minHubLinks {
			hubs = append(hubs, cand{p, stale})
		}
	}
	// Hubs: most-referenced first, older breaks ties.
	sort.Slice(hubs, func(i, j int) bool {
		bi, bj := len(backlinks[normalizeSlug(hubs[i].page.Slug)]), len(backlinks[normalizeSlug(hubs[j].page.Slug)])
		if bi != bj {
			return bi > bj
		}
		return hubs[i].stale > hubs[j].stale
	})

	picks := []Pick{}
	used := map[string]bool{}
	take := func(c cand, strat string) {
		if used[c.page.Slug] {
			return
		}
		used[c.page.Slug] = true
		days := int(c.stale.Hours() / 24)
		p := Pick{
			Slug:      c.page.Slug,
			RelPath:   c.page.RelPath,
			Title:     c.page.Title,
			Strategy:  strat,
			DaysStale: days,
			Backlinks: backlinks[normalizeSlug(c.page.Slug)],
			Outbound:  c.page.Links,
			Excerpt:   excerpt(c.page.Body),
		}
		if strat == "stale-hub" {
			p.Explanation = fmt.Sprintf("%d개 페이지가 여전히 참조하는데 %d일째 갱신이 없음", len(p.Backlinks), days)
		} else {
			p.Explanation = fmt.Sprintf("%d일간 다시 본 적 없는 페이지", days)
		}
		picks = append(picks, p)
	}
	takeRandomForgotten := func() bool {
		// Age-weighted random: older pages surface more often.
		var pool []cand
		var weights []float64
		for _, c := range forgotten {
			if used[c.page.Slug] {
				continue
			}
			pool = append(pool, c)
			weights = append(weights, c.stale.Hours()/24)
		}
		if len(pool) == 0 {
			return false
		}
		var total float64
		for _, w := range weights {
			total += w
		}
		r := rng.Float64() * total
		for i, w := range weights {
			if r -= w; r <= 0 {
				take(pool[i], "random-forgotten")
				return true
			}
		}
		take(pool[len(pool)-1], "random-forgotten")
		return true
	}
	takeHub := func() bool {
		for _, c := range hubs {
			if !used[c.page.Slug] {
				take(c, "stale-hub")
				return true
			}
		}
		return false
	}

	for len(picks) < n {
		var ok bool
		switch strategy {
		case "random":
			ok = takeRandomForgotten()
		case "hub":
			ok = takeHub()
		case "auto":
			if rng.Float64() < 0.7 {
				ok = takeRandomForgotten() || takeHub()
			} else {
				ok = takeHub() || takeRandomForgotten()
			}
		default:
			return nil, fmt.Errorf("unknown strategy %q (random|hub|auto)", strategy)
		}
		if !ok {
			break
		}
	}
	return picks, nil
}

// Bridge is a similar page pair. In default mode only unlinked pairs
// are returned (connection candidates). With includeLinked, linked
// pairs at high similarity also surface — those are merge/contradiction
// candidates for the agent's semantic lint pass.
type Bridge struct {
	A          BridgeSide `json:"a"`
	B          BridgeSide `json:"b"`
	Similarity float64    `json:"similarity"`
	Linked     bool       `json:"linked"`
}

type BridgeSide struct {
	Slug    string `json:"slug"`
	RelPath string `json:"rel_path"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

// PickBridges finds page pairs with cosine similarity >= minSim.
// Linked pairs are excluded unless includeLinked is set.
func PickBridges(scan *wiki.ScanResult, vectors map[string][]float32, st *State, minSim float64, n int, includeLinked bool, now time.Time) []Bridge {
	linked := map[string]bool{}
	for _, p := range scan.Pages {
		from := normalizeSlug(p.Slug)
		for _, t := range p.Links {
			linked[pairKey(from, t)] = true
		}
	}
	var slugs []string
	for _, p := range scan.Pages {
		if _, ok := vectors[p.Slug]; ok {
			slugs = append(slugs, p.Slug)
		}
	}
	sort.Strings(slugs)

	bridges := []Bridge{}
	for i := 0; i < len(slugs); i++ {
		for j := i + 1; j < len(slugs); j++ {
			a, b := slugs[i], slugs[j]
			na, nb := normalizeSlug(a), normalizeSlug(b)
			isLinked := linked[pairKey(na, nb)]
			if (isLinked && !includeLinked) || !st.pairEligible(na, nb, now) {
				continue
			}
			sim := store.Cosine(vectors[a], vectors[b])
			if sim < minSim {
				continue
			}
			pa, pb := scan.BySlug[na], scan.BySlug[nb]
			bridges = append(bridges, Bridge{
				A:          BridgeSide{Slug: pa.Slug, RelPath: pa.RelPath, Title: pa.Title, Excerpt: excerpt(pa.Body)},
				B:          BridgeSide{Slug: pb.Slug, RelPath: pb.RelPath, Title: pb.Title, Excerpt: excerpt(pb.Body)},
				Similarity: sim,
				Linked:     isLinked,
			})
		}
	}
	sort.Slice(bridges, func(i, j int) bool { return bridges[i].Similarity > bridges[j].Similarity })
	if len(bridges) > n {
		bridges = bridges[:n]
	}
	return bridges
}

// excerpt returns the first substantive paragraph (skips headings and
// link-list lines), capped at 200 runes.
func excerpt(body string) string {
	for _, para := range strings.Split(body, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "- [[") || strings.HasPrefix(p, "|") || strings.HasPrefix(p, ">") || strings.HasPrefix(p, "---") {
			continue
		}
		runes := []rune(strings.ReplaceAll(p, "\n", " "))
		if len(runes) > 200 {
			runes = append(runes[:200], '…')
		}
		return string(runes)
	}
	return ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
