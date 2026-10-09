package digest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/wiki"
)

func TestCollectDiagnosesInvalidDatesWithoutLosingPages(t *testing.T) {
	scan := &wiki.ScanResult{}
	for _, row := range []struct{ slug, created, updated string }{
		{"bad", "z-not-a-date", "2026-02-30"}, {"empty", "", ""},
		{"quoted-valid", "2026-10-10", "2026-10-10"}, {"before", "2026-10-09", "2026-10-09"},
		{"updated", "2026-01-01", "2026-10-10"}, {"future", "2026-10-11", "2026-10-11"},
	} {
		raw := fmt.Sprintf("---\ntitle: %s\ncreated: %q\nupdated: %q\ntype: concept\ntags: []\n---\nbody", row.slug, row.created, row.updated)
		scan.Pages = append(scan.Pages, wiki.Parse("concepts/"+row.slug+".md", []byte(raw)))
	}
	r := Collect(scan, time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	if r.Stats.Total != 6 || r.Stats.Created != 2 || r.Stats.Updated != 1 || r.CreatedPages[0].Slug != "quoted-valid" || r.UpdatedPages[0].Slug != "updated" {
		t.Fatalf("invalid date counted or valid page lost: %+v", r)
	}
	if got := r.Render(); !strings.Contains(got, "bad.created") || !strings.Contains(got, "z-not-a-date") || !strings.Contains(got, "empty.updated") {
		t.Fatalf("invalid/missing dates hidden: %s", got)
	}
}
