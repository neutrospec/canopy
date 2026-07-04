// Package digest collects review material for a time window: what was
// created, what was updated, activity stats, tag distribution, and the
// decision timeline. It supplies raw material only — composing a
// readable retrospective is the agent's job (Express step).
package digest

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/wiki"
)

type PageRef struct {
	Slug    string `json:"slug"`
	RelPath string `json:"rel_path"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Created string `json:"created"`
	Updated string `json:"updated"`
}

type Result struct {
	Since        string         `json:"since"` // YYYY-MM-DD (inclusive)
	CreatedPages []PageRef      `json:"created_pages"`
	UpdatedPages []PageRef      `json:"updated_pages"` // updated in window but created earlier
	TagCounts    map[string]int `json:"tag_counts"`    // tags of created pages
	Decisions    []PageRef      `json:"decisions"`     // pages tagged `decision`, whole wiki, chronological
	Stats        Stats          `json:"stats"`
}

type Stats struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Total   int `json:"total_pages"`
}

var sinceRe = regexp.MustCompile(`^(\d+)([dwm])$`)

// ParseSince accepts "90d", "12w", "3m", or an absolute YYYY-MM-DD.
func ParseSince(s string, now time.Time) (time.Time, error) {
	if m := sinceRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "d":
			return now.AddDate(0, 0, -n), nil
		case "w":
			return now.AddDate(0, 0, -7*n), nil
		case "m":
			return now.AddDate(0, -n, 0), nil
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("bad --since %q (want 90d, 12w, 3m, or YYYY-MM-DD)", s)
}

func Collect(scan *wiki.ScanResult, since time.Time) *Result {
	// Slices start non-nil so --json emits [] instead of null —
	// agents iterate results without null checks (G5 hygiene).
	res := &Result{
		Since:        since.Format("2006-01-02"),
		TagCounts:    map[string]int{},
		CreatedPages: []PageRef{},
		UpdatedPages: []PageRef{},
		Decisions:    []PageRef{},
	}
	res.Stats.Total = len(scan.Pages)
	cutoff := since.Format("2006-01-02")

	ref := func(p *wiki.Page) PageRef {
		return PageRef{Slug: p.Slug, RelPath: p.RelPath, Title: p.Title, Type: p.Type, Created: p.Created, Updated: p.Updated}
	}
	for _, p := range scan.Pages {
		// String compare works: dates are normalized YYYY-MM-DD.
		createdIn := p.Created >= cutoff && p.Created != ""
		updatedIn := p.Updated >= cutoff && p.Updated != ""
		switch {
		case createdIn:
			res.CreatedPages = append(res.CreatedPages, ref(p))
			for _, t := range p.Tags {
				res.TagCounts[t]++
			}
		case updatedIn:
			res.UpdatedPages = append(res.UpdatedPages, ref(p))
		}
		for _, t := range p.Tags {
			if t == "decision" {
				res.Decisions = append(res.Decisions, ref(p))
				break
			}
		}
	}
	byCreated := func(list []PageRef) {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Created != list[j].Created {
				return list[i].Created < list[j].Created
			}
			return list[i].Slug < list[j].Slug
		})
	}
	byCreated(res.CreatedPages)
	byCreated(res.Decisions)
	sort.Slice(res.UpdatedPages, func(i, j int) bool {
		if res.UpdatedPages[i].Updated != res.UpdatedPages[j].Updated {
			return res.UpdatedPages[i].Updated < res.UpdatedPages[j].Updated
		}
		return res.UpdatedPages[i].Slug < res.UpdatedPages[j].Slug
	})
	res.Stats.Created = len(res.CreatedPages)
	res.Stats.Updated = len(res.UpdatedPages)
	return res
}

// Render prints a compact human-readable digest.
func (r *Result) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "digest since %s — created %d, updated %d (total %d pages)\n",
		r.Since, r.Stats.Created, r.Stats.Updated, r.Stats.Total)
	if len(r.CreatedPages) > 0 {
		b.WriteString("\nCREATED\n")
		for _, p := range r.CreatedPages {
			fmt.Fprintf(&b, "  %s  %-40s %s\n", p.Created, p.Slug, p.Title)
		}
	}
	if len(r.UpdatedPages) > 0 {
		b.WriteString("\nUPDATED\n")
		for _, p := range r.UpdatedPages {
			fmt.Fprintf(&b, "  %s  %-40s %s\n", p.Updated, p.Slug, p.Title)
		}
	}
	if len(r.TagCounts) > 0 {
		type tc struct {
			tag string
			n   int
		}
		var tags []tc
		for t, n := range r.TagCounts {
			tags = append(tags, tc{t, n})
		}
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].n != tags[j].n {
				return tags[i].n > tags[j].n
			}
			return tags[i].tag < tags[j].tag
		})
		b.WriteString("\nTAGS (new pages): ")
		var parts []string
		for _, t := range tags {
			parts = append(parts, fmt.Sprintf("%s×%d", t.tag, t.n))
		}
		b.WriteString(strings.Join(parts, ", ") + "\n")
	}
	if len(r.Decisions) > 0 {
		b.WriteString("\nDECISION TIMELINE (all time)\n")
		for _, p := range r.Decisions {
			fmt.Fprintf(&b, "  %s  %s\n", p.Created, p.Title)
		}
	}
	return b.String()
}
