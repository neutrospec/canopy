package resurface

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

func TestSnoozeDurationAcrossZones(t *testing.T) {
	for _, offset := range []int{-12, -7, 0, 9, 14} {
		for _, hour := range []int{0, 23} {
			for _, days := range []int{1, 7} {
				now := time.Date(2026, 10, 10, hour, 59, 59, 0, time.FixedZone("test", offset*3600))
				s := newState()
				s.Snooze("page", days, now)
				deadline, err := time.Parse(time.RFC3339, s.Snoozed["page"])
				if err != nil || !deadline.After(now) || deadline.Sub(now) != time.Duration(days)*24*time.Hour {
					t.Fatalf("offset=%d hour=%d days=%d deadline=%q (%v)", offset, hour, days, s.Snoozed["page"], err)
				}
				if s.eligible("page", now) || s.eligible("page", deadline.Add(-time.Second)) || !s.eligible("page", deadline) {
					t.Fatalf("duration exclusion wrong: %s", deadline)
				}
			}
		}
	}
}

func TestLegacySnoozeDateStillLoads(t *testing.T) {
	s := newState()
	s.Snoozed["page"] = "2026-10-11"
	if s.eligible("page", time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("legacy date lost")
	}
}

func TestSnoozeSelfVersionAndSubsecondDeadline(t *testing.T) {
	for _, version := range []int{1, 9} {
		w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
		if err := os.MkdirAll(filepath.Dir(StatePath(w)), 0755); err != nil {
			t.Fatal(err)
		}
		raw := fmt.Sprintf(`{"version":%d,"shown":{},"shown_pairs":{},"snoozed":{"legacy":"2026-10-11"},"feedback":[],"future":{"n":9007199254740993}}`, version)
		if err := os.WriteFile(StatePath(w), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		s, err := LoadState(w)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 10, 10, 12, 0, 0, 123456789, time.FixedZone("test", 9*3600))
		s.Snooze("page", 1, now)
		if err := s.Save(w); err != nil {
			t.Fatal(err)
		}
		s, err = LoadState(w)
		if err != nil {
			t.Fatal(err)
		}
		deadline, err := time.Parse(time.RFC3339, s.Snoozed["page"])
		wantVersion := version
		if wantVersion < 2 {
			wantVersion = 2
		}
		if err != nil || !deadline.Equal(now.Add(24*time.Hour)) || s.Version != wantVersion || s.Snoozed["legacy"] != "2026-10-11" {
			t.Fatalf("self-version/deadline lost: %+v %v", s, err)
		}
	}
}
