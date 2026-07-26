package reads

import (
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

func TestMarkUpgradeAndUndo(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	s.Mark("Page-A", "auto", now)
	if r := s.Get("page-a"); r == nil || r.Source != "auto" || r.Count != 1 {
		t.Fatalf("auto mark broken: %+v", r)
	}
	// explicit upgrades auto
	s.Mark("page-a", "explicit", now.Add(time.Hour))
	if r := s.Get("page-a"); r.Source != "explicit" || r.Count != 2 || r.First == r.Last {
		t.Fatalf("explicit upgrade broken: %+v", r)
	}
	// auto never downgrades explicit
	s.Mark("page-a", "auto", now.Add(2*time.Hour))
	if r := s.Get("page-a"); r.Source != "explicit" {
		t.Fatalf("auto downgraded explicit: %+v", r)
	}

	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	if !s2.IsRead("PAGE-A") {
		t.Fatal("persisted read lost (or slug case-sensitive)")
	}

	s2.Rename("page-a", "page-b")
	if s2.IsRead("page-a") || !s2.IsRead("page-b") {
		t.Fatal("rename did not migrate history")
	}

	s2.Unmark("page-b")
	if s2.IsRead("page-b") {
		t.Fatal("unmark failed")
	}
}

func TestRecentSlugs(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, _ := Load(w)
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	for i, slug := range []string{"old", "mid", "new"} {
		s.Mark(slug, "explicit", base.Add(time.Duration(i)*time.Hour))
	}
	got := s.RecentSlugs(2)
	if len(got) != 2 || got[0] != "new" || got[1] != "mid" {
		t.Fatalf("recent order wrong: %v", got)
	}
}

// Agent marks are day-quantized (H7), never touch the human map (H4),
// persist in their own file, and feed LastAccess alongside human reads.
func TestMarkAgent(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)

	if !s.MarkAgent("Page-A", now) {
		t.Fatal("first agent touch should report a change")
	}
	if s.MarkAgent("page-a", now.Add(3*time.Hour)) {
		t.Fatal("same-day touch must be a no-op (H7)")
	}
	if !s.MarkAgent("page-a", now.Add(26*time.Hour)) {
		t.Fatal("next-day touch should report a change")
	}
	a := s.Agent["page-a"]
	if a == nil || a.Days != 2 {
		t.Fatalf("want 2 touch days, got %+v", a)
	}
	// H4: the human read state is untouched by agent marks.
	if s.IsRead("page-a") || len(s.Reads) != 0 {
		t.Fatalf("agent mark polluted human reads: %+v", s.Reads)
	}

	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	if a := s2.Agent["page-a"]; a == nil || a.Days != 2 {
		t.Fatalf("agent aggregate not persisted: %+v", s2.Agent)
	}

	// LastAccess is the max across doors.
	s2.Mark("page-a", "explicit", now.Add(48*time.Hour))
	if last, ok := s2.LastAccess("page-a"); !ok || !last.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("LastAccess should follow the newest door, got %v %v", last, ok)
	}
}

// Rename must migrate agent history along with human reads (H3).
func TestRenameMovesAgent(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, _ := Load(w)
	now := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	s.MarkAgent("old-name", now)
	s.Rename("old-name", "new-name")
	if s.Agent["old-name"] != nil || s.Agent["new-name"] == nil {
		t.Fatalf("agent record not renamed: %+v", s.Agent)
	}
}
