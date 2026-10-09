package reads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

func TestReadStateUnknownFieldsAndPartialAvailability(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.MkdirAll(filepath.Dir(Path(w)), 0755); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"reads":{"a":{"first":"2026-10-09T00:00:00Z","last":"2026-10-09T00:00:00Z","count":1,"source":"auto","extra":{"n":9007199254740993}}},"extra":{"keep":true}}`
	if err := os.WriteFile(Path(w), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	s.Mark("a", "explicit", time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(Path(w))
	if !strings.Contains(string(got), "9007199254740993") || !strings.Contains(string(got), "keep") {
		t.Fatalf("unknown lost: %s", got)
	}
	s.MarkAgent("healthy", time.Now())
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(w), []byte("null"), 0644); err != nil {
		t.Fatal(err)
	}
	s, err = Load(w)
	if err == nil || s == nil || s.Agent["healthy"] == nil {
		t.Fatalf("partial state unavailable: %+v %v", s, err)
	}
	s.MarkAgent("another", time.Now())
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(Path(w))
	if string(got) != "null" {
		t.Fatal("corrupt file reset")
	}
	s.Mark("a", "explicit", time.Now())
	if err := s.Save(w); err == nil {
		t.Fatal("corrupt human write allowed")
	}
}

func TestReadsMergeDifferentKeysAndRefuseSameLeaf(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	a, _ := Load(w)
	b, _ := Load(w)
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	a.Mark("a", "auto", now)
	b.Mark("b", "auto", now)
	if err := a.Save(w); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(w); err != nil {
		t.Fatal(err)
	}
	s, err := Load(w)
	if err != nil || !s.IsRead("a") || !s.IsRead("b") {
		t.Fatalf("lost key: %+v %v", s, err)
	}
	a, _ = Load(w)
	b, _ = Load(w)
	a.Mark("a", "explicit", now.Add(time.Hour))
	b.Mark("a", "explicit", now.Add(2*time.Hour))
	if err := a.Save(w); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(Path(w))
	if err := b.Save(w); err == nil {
		t.Fatal("stale same-key accepted")
	}
	after, _ := os.ReadFile(Path(w))
	if string(before) != string(after) {
		t.Fatal("conflict changed file")
	}
}

func TestRecentOrderUsesInstantsAndStableSlug(t *testing.T) {
	s := &State{Reads: map[string]*Read{}, Agent: map[string]*AgentRead{}}
	for slug, ts := range map[string]string{"a": "2026-10-10T08:00:00+09:00", "b": "2026-10-09T17:00:00-07:00", "c": "2026-10-10T00:00:00Z"} {
		v, _ := time.Parse(time.RFC3339, ts)
		s.Mark(slug, "explicit", v)
	}
	got := s.RecentAccessSlugs(3)
	if strings.Join(got, ",") != "b,c,a" {
		t.Fatalf("instant/tie order = %v", got)
	}
}

func TestRenamePreservesExistingDestinationHistory(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, _ := Load(w)
	now := time.Now()
	s.MarkAgent("old", now.Add(-time.Hour))
	s.Mark("old", "auto", now.Add(-time.Hour))
	s.Mark("new", "explicit", now)
	s.Rename("old", "new")
	if s.Agent["new"].Days != 1 || s.Reads["new"].Count != 2 || s.Reads["new"].Source != "explicit" {
		t.Fatalf("destination overwritten: %+v %+v", s.Agent, s.Reads)
	}
}

func TestRenameAgentDestinationRefusesWithoutChangingEitherHistory(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	s, _ := Load(w)
	now := time.Now()
	s.MarkAgent("old", now)
	s.MarkAgent("new", now)
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(AgentPath(w))
	if err := CanRenameInWiki(w, "old", "new"); err == nil {
		t.Fatal("ambiguous day union accepted")
	}
	s.Rename("old", "new")
	if err := s.Save(w); err == nil {
		t.Fatal("ambiguous rename saved")
	}
	after, _ := os.ReadFile(AgentPath(w))
	if string(before) != string(after) || s.Agent["old"] == nil || s.Agent["new"] == nil {
		t.Fatal("collision changed either history")
	}
}
