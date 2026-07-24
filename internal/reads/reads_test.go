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
