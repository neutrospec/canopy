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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/statefile"
	"github.com/neutrospec/canopy/internal/wikilock"
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
	readsBase  []byte
	agentBase  []byte
	readsErr   error
	agentErr   error
	humanRaw   []byte
	agentRaw   []byte
	renameErr  error
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

// HumanAvailable and AgentAvailable distinguish a known empty aggregate from an
// unreadable file. Load can return a healthy half together with an error.
func (s *State) HumanAvailable() bool { return s != nil && s.readsErr == nil }
func (s *State) AgentAvailable() bool { return s != nil && s.agentErr == nil }

// Load reads both attention aggregates (human reads + agent reads); a
// missing file is an empty state, and unknown future versions are read
// tolerantly (self-versioned wiki files, AGENTS.md 규칙 1b).
func Load(w *config.Wiki) (*State, error) {
	s := &State{Version: 1, Reads: map[string]*Read{}, Agent: map[string]*AgentRead{}}
	data, err := os.ReadFile(Path(w))
	if err == nil {
		if err := decodeHuman(data, s); err != nil {
			s.readsErr = fmt.Errorf("%s unavailable: %w", Path(w), err)
			s.Reads = map[string]*Read{}
		}
	} else if !os.IsNotExist(err) {
		s.readsErr = fmt.Errorf("%s unavailable: %w", Path(w), err)
	}
	if s.Reads == nil {
		s.Reads = map[string]*Read{}
	}
	af := agentFile{Version: 1}
	adata, err := os.ReadFile(AgentPath(w))
	if err == nil {
		if err := decodeAgent(adata, &af); err != nil {
			s.agentErr = fmt.Errorf("%s unavailable: %w", AgentPath(w), err)
			af.Agent = map[string]*AgentRead{}
		}
	} else if !os.IsNotExist(err) {
		s.agentErr = fmt.Errorf("%s unavailable: %w", AgentPath(w), err)
	}
	s.Agent = af.Agent
	if s.Agent == nil {
		s.Agent = map[string]*AgentRead{}
	}
	if len(data) > 0 && s.readsErr == nil {
		s.readsBase = statefile.Snapshot(s)
	}
	if len(adata) > 0 && s.agentErr == nil {
		s.agentBase = statefile.Snapshot(af)
	}
	s.humanRaw = data
	s.agentRaw = adata
	return s, errors.Join(s.readsErr, s.agentErr)
}

func decodeHuman(data []byte, s *State) error {
	if err := statefile.Decode(data, s); err != nil {
		return err
	}
	for slug, r := range s.Reads {
		if r == nil {
			return fmt.Errorf("null read entry %q", slug)
		}
	}
	return nil
}
func decodeAgent(data []byte, a *agentFile) error {
	if err := statefile.Decode(data, a); err != nil {
		return err
	}
	for slug, r := range a.Agent {
		if r == nil {
			return fmt.Errorf("null agent entry %q", slug)
		}
	}
	return nil
}

// Save writes only the file(s) whose data actually changed, so an
// agent-only touch never creates or rewrites reads.json (H4) and a plain
// web mark never touches the agent file.
func (s *State) Save(w *config.Wiki) error {
	g, err := wikilock.AcquireShared(context.Background(), w.Root)
	if err != nil {
		return err
	}
	defer g.Unlock()
	return s.SaveLocked(w)
}

// SaveLocked requires a wiki lease from the caller; each file patch takes its
// own exclusive guard, merges disjoint edits and preserves raw unknown fields.
func (s *State) SaveLocked(w *config.Wiki) error {
	if s.renameErr != nil {
		return s.renameErr
	}
	if s.readsDirty {
		if s.readsErr != nil {
			return s.readsErr
		}
		if err := statefile.Patch(Path(w), s.readsBase, s, func(b []byte) error { return decodeHuman(b, &State{}) }); err != nil {
			return err
		}
		s.readsDirty = false
		s.readsBase = statefile.Snapshot(s)
	}
	if s.agentDirty {
		if s.agentErr != nil {
			return s.agentErr
		}
		af := agentFile{Version: 1, Agent: s.Agent}
		// Preserve a future self-version as well as its unknown fields.
		if len(s.agentBase) > 0 {
			var prev agentFile
			if statefile.Decode(s.agentBase, &prev) == nil && prev.Version > 0 {
				af.Version = prev.Version
			}
		}
		if err := statefile.Patch(AgentPath(w), s.agentBase, af, func(b []byte) error { return decodeAgent(b, &agentFile{}) }); err != nil {
			return err
		}
		s.agentDirty = false
		s.agentBase = statefile.Snapshot(af)
	}
	return nil
}

func norm(slug string) string { return strings.ToLower(slug) }

// Mark records a read. Explicit marks upgrade auto ones; auto marks
// never downgrade an explicit one.
func (s *State) Mark(slug, source string, now time.Time) {
	k := norm(slug)
	ts := now.Format(time.RFC3339)
	s.readsDirty = true
	r, ok := s.Reads[k]
	if !ok || r == nil {
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
	if !ok || a == nil {
		s.Agent[k] = &AgentRead{First: ts, Last: ts, Days: 1}
		s.agentDirty = true
		return true
	}
	if prev, err := time.Parse(time.RFC3339, a.Last); err == nil && prev.In(now.Location()).Format("2006-01-02") == day {
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
	if r, ok := s.Reads[k]; ok && r != nil {
		if t, err := time.Parse(time.RFC3339, r.Last); err == nil {
			best = t
		}
	}
	if a, ok := s.Agent[k]; ok && a != nil {
		if t, err := time.Parse(time.RFC3339, a.Last); err == nil && t.After(best) {
			best = t
		}
	}
	return best, !best.IsZero()
}

// CanRename refuses an ambiguous agent-day merge: aggregates contain no day
// set from which a distinct union could be reconstructed. Human event counts
// can be added. Call this under the mutation's wiki lease before moving files.
func (s *State) CanRename(oldSlug, newSlug string) error {
	k := norm(oldSlug)
	n := norm(newSlug)
	if k == n {
		return nil
	}
	if s.readsErr != nil || s.agentErr != nil {
		return errors.Join(s.readsErr, s.agentErr)
	}
	if s.Reads[k] == nil && s.Agent[k] == nil {
		return nil
	}
	if s.Agent[k] != nil && s.Agent[n] != nil {
		return fmt.Errorf("read history already exists for %q; preserve both agent aggregates and choose a different slug", newSlug)
	}
	// Relocating an entry with fields this binary cannot interpret would drop
	// those fields. Refuse that state write instead, leaving recovery possible.
	for _, input := range []struct {
		raw []byte
		dst any
	}{{s.humanRaw, &State{}}, {s.agentRaw, &agentFile{}}} {
		if len(input.raw) == 0 {
			continue
		}
		d := json.NewDecoder(bytes.NewReader(input.raw))
		d.DisallowUnknownFields()
		if err := d.Decode(input.dst); err != nil {
			return fmt.Errorf("cannot rename extended read state; preserve it and upgrade: %w", err)
		}
	}
	return nil
}

// CanRenameInWiki requires the caller's wiki lease, just like RenameInWiki.
func CanRenameInWiki(w *config.Wiki, oldSlug, newSlug string) error {
	s, err := Load(w)
	if err != nil {
		return err
	}
	return s.CanRename(oldSlug, newSlug)
}

// Rename migrates history when a page moves (canopy mv).
func (s *State) Rename(oldSlug, newSlug string) {
	if err := s.CanRename(oldSlug, newSlug); err != nil {
		s.renameErr = err
		return
	}
	k, n := norm(oldSlug), norm(newSlug)
	if k == n {
		return
	}
	if r, ok := s.Reads[k]; ok && r != nil {
		delete(s.Reads, k)
		if dst := s.Reads[n]; dst != nil {
			r = &Read{First: earlier(r.First, dst.First), Last: later(r.Last, dst.Last), Count: r.Count + dst.Count, Source: r.Source}
			if dst.Source == "explicit" {
				r.Source = "explicit"
			}
		}
		s.Reads[n] = r
		s.readsDirty = true
	}
	if a, ok := s.Agent[k]; ok && a != nil {
		delete(s.Agent, k)
		s.Agent[n] = a
		s.agentDirty = true
	}
}

// RenameInWikiLocked is called by mv under its exclusive wiki lease.
func RenameInWikiLocked(w *config.Wiki, oldSlug, newSlug string) error {
	s, err := Load(w)
	if err != nil {
		return err
	}
	s.Rename(oldSlug, newSlug)
	return s.SaveLocked(w)
}

// RenameInWiki is the mutation-pipeline alias; caller must hold a wiki lease.
func RenameInWiki(w *config.Wiki, oldSlug, newSlug string) error {
	return RenameInWikiLocked(w, oldSlug, newSlug)
}

func earlier(a, b string) string {
	x, ex := time.Parse(time.RFC3339, a)
	y, ey := time.Parse(time.RFC3339, b)
	if ex != nil {
		return b
	}
	if ey != nil || x.Before(y) {
		return a
	}
	return b
}
func later(a, b string) string {
	x, ex := time.Parse(time.RFC3339, a)
	y, ey := time.Parse(time.RFC3339, b)
	if ex != nil {
		return b
	}
	if ey != nil || x.After(y) {
		return a
	}
	return b
}

// RecentAccessSlugs returns up to n slugs by most recent access through
// ANY door — the "current interest" signal for discovery affinity. The
// agent door counts because the user's questions drive it: what recall
// keeps citing is what the user is thinking about (web-ui-plan-4 M12).
func (s *State) RecentAccessSlugs(n int) []string {
	last := map[string]string{}
	for slug, r := range s.Reads {
		if r == nil {
			continue
		}
		last[slug] = r.Last
	}
	for slug, a := range s.Agent {
		if a == nil {
			continue
		}
		if later(a.Last, last[slug]) == a.Last {
			last[slug] = a.Last
		}
	}
	type item struct{ slug, ts string }
	items := make([]item, 0, len(last))
	for slug, ts := range last {
		items = append(items, item{slug, ts})
	}
	sort.Slice(items, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, items[i].ts)
		b, _ := time.Parse(time.RFC3339, items[j].ts)
		if a.Equal(b) {
			return items[i].slug < items[j].slug
		}
		return a.After(b)
	})
	if n < 0 {
		n = 0
	}
	if len(items) > n {
		items = items[:n]
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.slug
	}
	return out
}

// RecentSlugs returns up to n read slugs, most recently read first —
// the "current interest" signal for discovery ranking.
func (s *State) RecentSlugs(n int) []string {
	type item struct {
		slug, last string
	}
	items := make([]item, 0, len(s.Reads))
	for slug, r := range s.Reads {
		if r == nil {
			continue
		}
		items = append(items, item{slug, r.Last})
	}
	sort.Slice(items, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, items[i].last)
		b, _ := time.Parse(time.RFC3339, items[j].last)
		if a.Equal(b) {
			return items[i].slug < items[j].slug
		}
		return a.After(b)
	})
	if n < 0 {
		n = 0
	}
	if len(items) > n {
		items = items[:n]
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.slug
	}
	return out
}
