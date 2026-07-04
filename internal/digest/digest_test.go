package digest

import (
	"fmt"
	"testing"
	"time"

	"github.com/nobocop/canopy/internal/wiki"
)

var now = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

func makeScan() *wiki.ScanResult {
	res := &wiki.ScanResult{BySlug: map[string]*wiki.Page{}}
	add := func(slug, created, updated, tags string) {
		raw := fmt.Sprintf("---\ntitle: \"%s\"\ncreated: %s\nupdated: %s\ntype: concept\ntags: [%s]\n---\n\nbody\n", slug, created, updated, tags)
		p := wiki.Parse("concepts/"+slug+".md", []byte(raw))
		res.Pages = append(res.Pages, p)
		res.BySlug[slug] = p
	}
	add("new-page", "2026-06-20", "2026-06-20", "tool")
	add("refreshed", "2026-01-01", "2026-06-25", "ai-ml")
	add("old-decision", "2026-02-01", "2026-02-01", "decision, business")
	add("untouched", "2026-01-01", "2026-01-01", "tool")
	return res
}

func TestParseSince(t *testing.T) {
	for in, want := range map[string]string{
		"30d":        "2026-06-04",
		"2w":         "2026-06-20",
		"3m":         "2026-04-04",
		"2026-05-01": "2026-05-01",
	} {
		got, err := ParseSince(in, now)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got.Format("2006-01-02") != want {
			t.Errorf("ParseSince(%s) = %s, want %s", in, got.Format("2006-01-02"), want)
		}
	}
	if _, err := ParseSince("nonsense", now); err == nil {
		t.Error("bad input accepted")
	}
}

func TestCollect(t *testing.T) {
	since, _ := ParseSince("30d", now) // 2026-06-04
	r := Collect(makeScan(), since)

	if r.Stats.Created != 1 || r.CreatedPages[0].Slug != "new-page" {
		t.Errorf("created wrong: %+v", r.CreatedPages)
	}
	if r.Stats.Updated != 1 || r.UpdatedPages[0].Slug != "refreshed" {
		t.Errorf("updated wrong: %+v", r.UpdatedPages)
	}
	// G4: everything in the window.
	for _, p := range r.UpdatedPages {
		if p.Updated < r.Since {
			t.Errorf("out-of-window page: %+v", p)
		}
	}
	// G5: internal consistency.
	if r.Stats.Created != len(r.CreatedPages) || r.Stats.Updated != len(r.UpdatedPages) {
		t.Error("stats disagree with lists")
	}
	if len(r.Decisions) != 1 || r.Decisions[0].Slug != "old-decision" {
		t.Errorf("decision timeline wrong: %+v", r.Decisions)
	}
	if r.TagCounts["tool"] != 1 || r.TagCounts["ai-ml"] != 0 {
		t.Errorf("tag counts should cover created pages only: %v", r.TagCounts)
	}
}
