// Package lint checks the wiki against the schema in canopy.toml.
// It reports; fixing is a separate concern (M4).
package lint

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/wiki"
)

type Severity string

const (
	Critical Severity = "critical"
	Warning  Severity = "warning"
	Info     Severity = "info"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Kind     string   `json:"kind"`
	Page     string   `json:"page,omitempty"`
	Message  string   `json:"message"`
}

type Report struct {
	TotalPages int            `json:"total_pages"`
	Findings   []Finding      `json:"findings"`
	Counts     map[string]int `json:"counts"`
}

func Run(w *config.Wiki, scan *wiki.ScanResult) *Report {
	r := &Report{TotalPages: len(scan.Pages), Counts: map[string]int{}}
	add := func(sev Severity, kind, page, msg string) {
		r.Findings = append(r.Findings, Finding{sev, kind, page, msg})
		r.Counts[kind]++
	}

	allowedTags := map[string]bool{}
	for _, t := range w.Cfg.Schema.Tags {
		allowedTags[t] = true
	}
	allowedTypes := map[string]bool{}
	for _, t := range w.Cfg.Schema.Types {
		allowedTypes[t] = true
	}

	for _, f := range scan.StrayRoot {
		add(Critical, "stray-root", f, "page in wiki root; move it under a page directory")
	}

	backlinks := scan.Backlinks()
	now := time.Now()

	for _, p := range scan.Pages {
		name := p.RelPath[strings.LastIndex(p.RelPath, "/")+1:]
		if !wiki.ValidFilename(name) {
			add(Critical, "bad-filename", p.RelPath, "filename must be lowercase ASCII with hyphens")
		}

		if !p.HasFrontmatter {
			add(Critical, "no-frontmatter", p.RelPath, "missing YAML frontmatter")
		} else if p.FMErr != "" {
			add(Critical, "bad-frontmatter", p.RelPath, "frontmatter parse error: "+p.FMErr)
		} else {
			var missing []string
			for k, v := range map[string]string{"title": p.Title, "type": p.Type, "created": p.Created, "updated": p.Updated} {
				if v == "" {
					missing = append(missing, k)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				add(Critical, "frontmatter-fields", p.RelPath, "missing fields: "+strings.Join(missing, ", "))
			}
			if p.Type != "" && !allowedTypes[p.Type] {
				add(Critical, "invalid-type", p.RelPath, fmt.Sprintf("type %q not in schema (%s)", p.Type, strings.Join(w.Cfg.Schema.Types, "|")))
			}
			for _, t := range p.Tags {
				if !allowedTags[t] {
					add(Warning, "invalid-tag", p.RelPath, fmt.Sprintf("tag %q not in taxonomy", t))
				}
			}
			if p.Updated != "" && w.Cfg.Schema.StaleDays > 0 {
				if ts, err := time.Parse("2006-01-02", p.Updated); err == nil {
					if now.Sub(ts) > time.Duration(w.Cfg.Schema.StaleDays)*24*time.Hour {
						add(Info, "stale", p.RelPath, fmt.Sprintf("not updated since %s", p.Updated))
					}
				}
			}
		}

		for _, target := range p.Links {
			if _, ok := scan.BySlug[target]; !ok {
				add(Critical, "broken-link", p.RelPath, "broken wikilink [["+target+"]]")
			}
		}

		if len(backlinks[strings.ToLower(p.Slug)]) == 0 {
			add(Warning, "orphan", p.RelPath, "no inbound links from other pages")
		}

		if w.Cfg.Schema.MaxLines > 0 && p.Lines > w.Cfg.Schema.MaxLines {
			add(Warning, "large-page", p.RelPath, fmt.Sprintf("%d lines (max %d); consider splitting", p.Lines, w.Cfg.Schema.MaxLines))
		}
	}

	// Island clusters pass the per-page orphan check (they backlink each
	// other) but no path connects them to the main knowledge network.
	if comps := scan.Components(); len(comps) > 1 {
		for _, island := range comps[1:] {
			preview := island
			if len(preview) > 6 {
				preview = preview[:6]
			}
			msg := fmt.Sprintf("%d-page island, unreachable from the main graph (%d pages): %s",
				len(island), len(comps[0]), strings.Join(preview, ", "))
			if len(island) > len(preview) {
				msg += fmt.Sprintf(", … (+%d)", len(island)-len(preview))
			}
			add(Warning, "island", island[0], msg)
		}
	}

	sort.SliceStable(r.Findings, func(i, j int) bool {
		order := map[Severity]int{Critical: 0, Warning: 1, Info: 2}
		if order[r.Findings[i].Severity] != order[r.Findings[j].Severity] {
			return order[r.Findings[i].Severity] < order[r.Findings[j].Severity]
		}
		return r.Findings[i].Kind < r.Findings[j].Kind
	})
	return r
}
