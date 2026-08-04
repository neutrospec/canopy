package lint

import (
	"sort"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

// TagUsage is one tag's measured usage across governed pages.
type TagUsage struct {
	Tag   string  `json:"tag"`
	Count int     `json:"count"`
	Pct   float64 `json:"pct"` // percentage of total pages, 1 decimal
}

// TagAuditReport measures the taxonomy against actual usage — the code
// half of the governance split (docs/taxonomy.md): unused topics are
// reclaim candidates (S3), overbroad topics are split candidates (S4).
// Forms are reported for information only, never pressured.
type TagAuditReport struct {
	TotalPages      int        `json:"total_pages"`
	Legacy          bool       `json:"legacy"` // single-list `tags` config, facets not declared
	Topics          []TagUsage `json:"topics"`
	Forms           []TagUsage `json:"forms"`
	UnusedTopics    []string   `json:"unused_topics"`
	OverbroadTopics []TagUsage `json:"overbroad_topics"`
	BroadTopicPct   int        `json:"broad_topic_pct"`
	// UnknownTags appear on pages but not in the taxonomy (lint A5 rejects
	// them too; listed here so the audit view is self-contained).
	UnknownTags []TagUsage `json:"unknown_tags"`
}

// TagAudit reports; it never mutates anything (invariant S5).
func TagAudit(w *config.Wiki, scan *wiki.ScanResult) *TagAuditReport {
	r := &TagAuditReport{
		TotalPages:    len(scan.Pages),
		Legacy:        w.LegacyTags,
		BroadTopicPct: w.Cfg.Schema.BroadTopicPct,
	}

	count := map[string]int{}
	for _, p := range scan.Pages {
		for _, t := range p.Tags {
			count[t]++
		}
	}
	pct := func(n int) float64 {
		if r.TotalPages == 0 {
			return 0
		}
		return float64(int(float64(n)/float64(r.TotalPages)*1000+0.5)) / 10
	}
	usage := func(tags []string) []TagUsage {
		out := make([]TagUsage, 0, len(tags))
		for _, t := range tags {
			out = append(out, TagUsage{Tag: t, Count: count[t], Pct: pct(count[t])})
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
		return out
	}

	// In legacy mode the whole list lives in Tags; treat it as topics so
	// the reclaim/split pressure still applies until the facets are split.
	topics := w.Cfg.Schema.Topics
	if r.Legacy {
		topics = w.Cfg.Schema.Tags
	}
	r.Topics = usage(topics)
	r.Forms = usage(w.Cfg.Schema.Forms)

	for _, u := range r.Topics {
		if u.Count == 0 {
			r.UnusedTopics = append(r.UnusedTopics, u.Tag)
		}
		if r.BroadTopicPct > 0 && u.Count*100 > r.TotalPages*r.BroadTopicPct {
			r.OverbroadTopics = append(r.OverbroadTopics, u)
		}
	}
	sort.Strings(r.UnusedTopics)

	known := map[string]bool{}
	for _, t := range w.Cfg.Schema.AllTags() {
		known[t] = true
	}
	var unknown []string
	for t := range count {
		if !known[t] {
			unknown = append(unknown, t)
		}
	}
	sort.Strings(unknown)
	r.UnknownTags = usage(unknown)
	return r
}
