package attention

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEventInstantBoundaries(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	cut := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for i, ts := range []string{
		"2026-10-10T08:59:59+09:00", // before, despite the later wall-clock text
		"2026-10-09T17:00:00-07:00", // equal
		"2026-10-09T17:00:01-07:00", // after
	} {
		v, _ := time.Parse(time.RFC3339, ts)
		if err := e.Log(v, []string{"before", "equal", "after"}[i], DoorWeb, KindRead, ""); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := e.Query(Filter{Since: cut})
	if err != nil || len(rows) != 2 || rows[0].Slug != "after" || rows[1].Slug != "equal" {
		t.Fatalf("instant query = %+v, %v", rows, err)
	}
	top, err := e.TopConsumed(cut, 10)
	if err != nil || len(top) != 2 {
		t.Fatalf("instant top = %+v, %v", top, err)
	}
	if _, err := e.db.Exec(`INSERT INTO events(ts,slug,door,kind) VALUES('invalid','keep','web','read')`); err != nil {
		t.Fatal(err)
	}
	n, err := e.Prune(cut)
	if err != nil || n != 1 {
		t.Fatalf("prune = %d, %v", n, err)
	}
	if n, err := e.CountBySlug("keep"); err != nil || n != 1 {
		t.Fatalf("invalid timestamp deleted: %d %v", n, err)
	}
}

func TestWeeklyCountsExcludeFutureAndRespectOffsets(t *testing.T) {
	e, err := Open(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for _, ts := range []string{"2026-10-09T17:00:00-07:00", "2026-10-10T09:00:01+09:00", "2026-10-03T09:00:00+09:00"} {
		v, _ := time.Parse(time.RFC3339, ts)
		if err := e.Log(v, "page", DoorWeb, KindRead, ""); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := e.WeeklyCounts("page", 1, now)
	if err != nil || len(counts) != 1 || counts[0] != 2 {
		t.Fatalf("weekly = %v, %v", counts, err)
	}
}
