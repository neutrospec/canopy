package resurface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

func TestStateUnknownVersionsAndBrokenFiles(t *testing.T) {
	for _, version := range []string{"1", "9"} {
		w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
		if err := os.MkdirAll(filepath.Dir(StatePath(w)), 0755); err != nil {
			t.Fatal(err)
		}
		raw := `{"version":` + version + `,"shown":{},"shown_pairs":{},"snoozed":{},"feedback":[{"slug":"a","vote":"up","time":"2026-10-09T00:00:00Z","extra":{"n":9007199254740993}}],"extra":{"n":9007199254740993}}`
		if err := os.WriteFile(StatePath(w), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		s, err := LoadState(w)
		if err != nil {
			t.Fatal(err)
		}
		s.MarkShown([]string{"new"}, time.Now())
		if err := s.Save(w); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(StatePath(w))
		if strings.Count(string(got), "9007199254740993") != 2 {
			t.Fatalf("unknown values lost: %s", got)
		}
	}
	for _, raw := range []string{"null", "[1]", "{"} {
		w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
		if err := os.MkdirAll(filepath.Dir(StatePath(w)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(StatePath(w), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadState(w); err == nil {
			t.Fatalf("broken state healthy: %s", raw)
		}
		got, _ := os.ReadFile(StatePath(w))
		if string(got) != raw {
			t.Fatal("broken state reset")
		}
	}
}
