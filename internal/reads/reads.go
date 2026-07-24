// Package reads tracks which pages the user has actually read. Like
// resurface state, this is NOT a derived cache — it cannot be rebuilt —
// so it lives inside the wiki repo at _meta/webui/reads.json and syncs
// across devices with the wiki (docs/web-ui-plan-2.md D3).
//
// "Read" means either an explicit mark (the ✓ button, first-class) or
// a conservative dwell+scroll detection (source "auto", undoable).
// Agents may read this file; writes go through this package.
package reads

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

type Read struct {
	First  string `json:"first"` // RFC3339, first time marked
	Last   string `json:"last"`  // RFC3339, latest mark
	Count  int    `json:"count"`
	Source string `json:"source"` // explicit | auto (explicit wins)
}

type State struct {
	Version int              `json:"version"`
	Reads   map[string]*Read `json:"reads"` // lowercased slug -> read
}

func Path(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "webui", "reads.json")
}

func Load(w *config.Wiki) (*State, error) {
	s := &State{Version: 1, Reads: map[string]*Read{}}
	data, err := os.ReadFile(Path(w))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Reads == nil {
		s.Reads = map[string]*Read{}
	}
	return s, nil
}

func (s *State) Save(w *config.Wiki) error {
	path := Path(w)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func norm(slug string) string { return strings.ToLower(slug) }

// Mark records a read. Explicit marks upgrade auto ones; auto marks
// never downgrade an explicit one.
func (s *State) Mark(slug, source string, now time.Time) {
	k := norm(slug)
	ts := now.Format(time.RFC3339)
	r, ok := s.Reads[k]
	if !ok {
		s.Reads[k] = &Read{First: ts, Last: ts, Count: 1, Source: source}
		return
	}
	r.Last = ts
	r.Count++
	if source == "explicit" {
		r.Source = "explicit"
	}
}

// Unmark forgets a read entirely (the undo affordance).
func (s *State) Unmark(slug string) {
	delete(s.Reads, norm(slug))
}

func (s *State) Get(slug string) *Read {
	return s.Reads[norm(slug)]
}

func (s *State) IsRead(slug string) bool {
	return s.Reads[norm(slug)] != nil
}

// Rename migrates history when a page moves (canopy mv).
func (s *State) Rename(oldSlug, newSlug string) {
	k := norm(oldSlug)
	if r, ok := s.Reads[k]; ok {
		delete(s.Reads, k)
		s.Reads[norm(newSlug)] = r
	}
}

// RecentSlugs returns up to n read slugs, most recently read first —
// the "current interest" signal for discovery ranking.
func (s *State) RecentSlugs(n int) []string {
	type item struct {
		slug, last string
	}
	items := make([]item, 0, len(s.Reads))
	for slug, r := range s.Reads {
		items = append(items, item{slug, r.Last})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].last > items[j].last })
	if len(items) > n {
		items = items[:n]
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.slug
	}
	return out
}
