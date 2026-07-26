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

// AgentRead aggregates agent-door consumption (canopy show / recall) for
// one page. Day-quantized: Days counts distinct touch days, not calls —
// per-call precision lives in the machine-local event log (internal/
// attention), so a busy recall session dirties the wiki at most once a day.
type AgentRead struct {
	First string `json:"first"` // RFC3339, first agent touch
	Last  string `json:"last"`  // RFC3339, latest agent touch
	Days  int    `json:"days"`  // distinct days touched
}

type State struct {
	Version int              `json:"version"`
	Reads   map[string]*Read `json:"reads"` // lowercased slug -> human read
	// Agent lives in its own file (_meta/attention/agent-reads.json), not
	// in reads.json v2: an older canopy re-saving reads.json would silently
	// drop fields it doesn't know (AGENTS.md 규칙 1b — mixed-version safety).
	Agent map[string]*AgentRead `json:"-"`

	readsDirty bool
	agentDirty bool
}

// agentFile is the on-disk shape of the agent aggregate.
type agentFile struct {
	Version int                   `json:"version"`
	Agent   map[string]*AgentRead `json:"agent"`
}

func Path(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "webui", "reads.json")
}

// AgentPath is the agent-door aggregate, committed to the wiki like
// reads.json (docs/web-ui-plan-4.md).
func AgentPath(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "attention", "agent-reads.json")
}

// Load reads both attention aggregates (human reads + agent reads); a
// missing file is an empty state, and unknown future versions are read
// tolerantly (self-versioned wiki files, AGENTS.md 규칙 1b).
func Load(w *config.Wiki) (*State, error) {
	s := &State{Version: 1, Reads: map[string]*Read{}, Agent: map[string]*AgentRead{}}
	data, err := os.ReadFile(Path(w))
	if err == nil {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if s.Reads == nil {
		s.Reads = map[string]*Read{}
	}
	af := agentFile{}
	adata, err := os.ReadFile(AgentPath(w))
	if err == nil {
		if err := json.Unmarshal(adata, &af); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	s.Agent = af.Agent
	if s.Agent == nil {
		s.Agent = map[string]*AgentRead{}
	}
	return s, nil
}

// Save writes only the file(s) whose data actually changed, so an
// agent-only touch never creates or rewrites reads.json (H4) and a plain
// web mark never touches the agent file.
func (s *State) Save(w *config.Wiki) error {
	if s.readsDirty {
		if err := writeJSON(Path(w), s); err != nil {
			return err
		}
		s.readsDirty = false
	}
	if len(s.Agent) > 0 && s.agentDirty {
		if err := writeJSON(AgentPath(w), agentFile{Version: 1, Agent: s.Agent}); err != nil {
			return err
		}
		s.agentDirty = false
	}
	return nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
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
	s.readsDirty = true
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
	k := norm(slug)
	if _, ok := s.Reads[k]; ok {
		delete(s.Reads, k)
		s.readsDirty = true
	}
}

func (s *State) Get(slug string) *Read {
	return s.Reads[norm(slug)]
}

func (s *State) IsRead(slug string) bool {
	return s.Reads[norm(slug)] != nil
}

// MarkAgent records an agent-door consumption (show/recall), quantized to
// one wiki-file change per day: it reports true — data changed, caller
// should Save — only on the first touch of a calendar day (invariant H7).
// It never touches the human Reads map (invariant H4).
func (s *State) MarkAgent(slug string, now time.Time) bool {
	k := norm(slug)
	ts := now.Format(time.RFC3339)
	day := now.Format("2006-01-02")
	a, ok := s.Agent[k]
	if !ok {
		s.Agent[k] = &AgentRead{First: ts, Last: ts, Days: 1}
		s.agentDirty = true
		return true
	}
	if prev, err := time.Parse(time.RFC3339, a.Last); err == nil && prev.Local().Format("2006-01-02") == day {
		return false // already recorded today
	}
	a.Last = ts
	a.Days++
	s.agentDirty = true
	return true
}

// LastAccess returns the most recent touch through ANY door (human read or
// agent consumption). This is the "forgotten?" signal resurface uses —
// a page cited by recall yesterday is not forgotten (invariant H6).
func (s *State) LastAccess(slug string) (time.Time, bool) {
	k := norm(slug)
	var best time.Time
	if r, ok := s.Reads[k]; ok {
		if t, err := time.Parse(time.RFC3339, r.Last); err == nil {
			best = t
		}
	}
	if a, ok := s.Agent[k]; ok {
		if t, err := time.Parse(time.RFC3339, a.Last); err == nil && t.After(best) {
			best = t
		}
	}
	return best, !best.IsZero()
}

// Rename migrates history when a page moves (canopy mv).
func (s *State) Rename(oldSlug, newSlug string) {
	k := norm(oldSlug)
	if r, ok := s.Reads[k]; ok {
		delete(s.Reads, k)
		s.Reads[norm(newSlug)] = r
		s.readsDirty = true
	}
	if a, ok := s.Agent[k]; ok {
		delete(s.Agent, k)
		s.Agent[norm(newSlug)] = a
		s.agentDirty = true
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
