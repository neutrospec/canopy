// Package resurface generates "re-encounter" candidates: forgotten
// pages, stale hubs, and similar-but-unlinked page pairs. canopy only
// selects candidates deterministically; judging whether a candidate is
// interesting, phrasing it, and delivering it (e.g. Telegram) is the
// agent's job. See docs/second-brain.md.
package resurface

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nobocop/canopy/internal/config"
)

// State is the pick history and user feedback. It is NOT a derived
// cache — it cannot be rebuilt — so it lives inside the wiki repo at
// _meta/resurface/state.json and syncs across devices with the wiki.
type State struct {
	Version    int               `json:"version"`
	Shown      map[string]string `json:"shown"`       // slug -> RFC3339 last shown
	ShownPairs map[string]string `json:"shown_pairs"` // "a|b" (sorted) -> RFC3339
	Snoozed    map[string]string `json:"snoozed"`     // slug -> YYYY-MM-DD until
	Feedback   []Feedback        `json:"feedback"`
}

type Feedback struct {
	Slug string `json:"slug"`
	Vote string `json:"vote"` // up | down
	Time string `json:"time"`
}

// Cooldowns: a page shown recently, or downvoted, stays out of the pool.
const (
	shownCooldown = 45 * 24 * time.Hour
	pairCooldown  = 90 * 24 * time.Hour
	downCooldown  = 120 * 24 * time.Hour
)

func StatePath(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "resurface", "state.json")
}

func LoadState(w *config.Wiki) (*State, error) {
	s := &State{Version: 1, Shown: map[string]string{}, ShownPairs: map[string]string{}, Snoozed: map[string]string{}}
	data, err := os.ReadFile(StatePath(w))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Shown == nil {
		s.Shown = map[string]string{}
	}
	if s.ShownPairs == nil {
		s.ShownPairs = map[string]string{}
	}
	if s.Snoozed == nil {
		s.Snoozed = map[string]string{}
	}
	return s, nil
}

func (s *State) Save(w *config.Wiki) error {
	path := StatePath(w)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

// eligible reports whether a page may be picked now.
func (s *State) eligible(slug string, now time.Time) bool {
	if until, ok := s.Snoozed[slug]; ok {
		if t, err := time.Parse("2006-01-02", until); err == nil && now.Before(t) {
			return false
		}
	}
	if ts, ok := s.Shown[slug]; ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil && now.Sub(t) < shownCooldown {
			return false
		}
	}
	for _, f := range s.Feedback {
		if f.Slug == slug && f.Vote == "down" {
			if t, err := time.Parse(time.RFC3339, f.Time); err == nil && now.Sub(t) < downCooldown {
				return false
			}
		}
	}
	return true
}

func (s *State) pairEligible(a, b string, now time.Time) bool {
	if ts, ok := s.ShownPairs[pairKey(a, b)]; ok {
		if ts == "dismissed" {
			return false
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil && now.Sub(t) < pairCooldown {
			return false
		}
	}
	return true
}

func (s *State) MarkShown(slugs []string, now time.Time) {
	for _, slug := range slugs {
		s.Shown[slug] = now.Format(time.RFC3339)
	}
}

func (s *State) MarkPairShown(a, b string, now time.Time) {
	s.ShownPairs[pairKey(a, b)] = now.Format(time.RFC3339)
}

func (s *State) DismissPair(a, b string) {
	s.ShownPairs[pairKey(a, b)] = "dismissed"
}

func (s *State) AddFeedback(slug, vote string, now time.Time) {
	s.Feedback = append(s.Feedback, Feedback{Slug: slug, Vote: vote, Time: now.Format(time.RFC3339)})
	sort.SliceStable(s.Feedback, func(i, j int) bool { return s.Feedback[i].Time < s.Feedback[j].Time })
}

func (s *State) Snooze(slug string, days int, now time.Time) {
	s.Snoozed[slug] = now.Add(time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
}

// normalizeSlug matches state keys case-insensitively like wikilinks.
func normalizeSlug(s string) string { return strings.ToLower(s) }
