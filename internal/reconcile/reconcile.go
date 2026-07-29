// Package reconcile is the canonicalization gate (docs/reconcile-design.md):
// a wiki-committed ledger of content hashes that have passed judgment.
// Pipeline writes (writeops) land in the ledger automatically — the main
// road and the web door are validated by construction — while back-door
// changes (Obsidian, other clones, raw edits) differ from the ledger and
// surface as foreign candidates for the agent to judge before they count
// as canonical. Detection, not prevention: nothing here blocks a write.
//
// The gate is opt-in per wiki: until `reconcile bless --all` establishes a
// baseline, Record and Count stay silent so existing wikis see no noise.
package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neutrospec/canopy/internal/config"
)

// State is the blessed-content ledger, committed to the wiki so every
// device agrees on what has been judged (invariant K1). Self-versioned
// wiki file (AGENTS.md 규칙 1b).
type State struct {
	Version int               `json:"version"`
	Blessed map[string]string `json:"blessed"` // rel path -> sha256 hex of reviewed content
}

func Path(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "reconcile", "state.json")
}

// Load reads the ledger. initialized=false means the gate has not been
// opted into on this wiki (no state file).
func Load(w *config.Wiki) (*State, bool, error) {
	s := &State{Version: 1, Blessed: map[string]string{}}
	data, err := os.ReadFile(Path(w))
	if os.IsNotExist(err) {
		return s, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", Path(w), err)
	}
	if s.Blessed == nil {
		s.Blessed = map[string]string{}
	}
	return s, true, nil
}

func (s *State) Save(w *config.Wiki) error {
	path := Path(w)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ") // map keys sort — stable git diffs
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return HashBytes(b), nil
}

// Effects declares what a pipeline mutation did to the filesystem, so the
// ledger can absorb it (자동 축복). Every writeops caller must be complete
// here — the single hallway (원칙 9) makes that a local property: mv lists
// the rewritten inbound pages, rm/archive list the link-stripped sources.
type Effects struct {
	Written []string // rel paths whose current content is pipeline product
	Removed []string // rel paths the mutation deleted or moved away
}

// Record folds a mutation's effects into the ledger. A no-op until the
// gate is initialized (opt-in), and for empty effects.
func Record(w *config.Wiki, eff Effects) error {
	if len(eff.Written) == 0 && len(eff.Removed) == 0 {
		return nil
	}
	st, ok, err := Load(w)
	if err != nil || !ok {
		return err
	}
	for _, rel := range eff.Written {
		h, err := hashFile(filepath.Join(w.Root, rel))
		if err != nil {
			return fmt.Errorf("bless %s: %w", rel, err)
		}
		st.Blessed[rel] = h
	}
	for _, rel := range eff.Removed {
		delete(st.Blessed, rel)
	}
	return st.Save(w)
}

// Candidate is one foreign change: page content the ledger has not judged.
type Candidate struct {
	RelPath string `json:"rel_path"`
	Kind    string `json:"kind"` // edited | new | deleted
}

// pageFiles lists the schema-governed page files without parsing them —
// cheap enough for the every-command banner.
func pageFiles(w *config.Wiki) []string {
	cfg := w.Cfg
	if cfg == nil {
		cfg = config.Default()
	}
	var out []string
	for _, d := range cfg.Schema.PageDirs {
		filepath.WalkDir(filepath.Join(w.Root, d), func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return nil // dir may not exist yet
			}
			if !e.IsDir() && strings.HasSuffix(p, ".md") {
				if rel, err := filepath.Rel(w.Root, p); err == nil {
					out = append(out, rel)
				}
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// Foreign compares the working tree against the ledger and returns the
// unjudged changes, sorted by path (deterministic — invariant K3). The
// read never touches the ledger (K5).
func Foreign(w *config.Wiki) ([]Candidate, bool, error) {
	st, ok, err := Load(w)
	if err != nil || !ok {
		return nil, ok, err
	}
	var cands []Candidate
	present := map[string]bool{}
	for _, rel := range pageFiles(w) {
		present[rel] = true
		h, err := hashFile(filepath.Join(w.Root, rel))
		if err != nil {
			return nil, true, err
		}
		prev, has := st.Blessed[rel]
		if has && prev == h {
			continue
		}
		kind := "edited"
		if !has {
			kind = "new"
		}
		cands = append(cands, Candidate{RelPath: rel, Kind: kind})
	}
	for rel := range st.Blessed {
		if !present[rel] {
			cands = append(cands, Candidate{RelPath: rel, Kind: "deleted"})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].RelPath < cands[j].RelPath })
	return cands, true, nil
}

// Count is the banner's cheap question: how many foreign changes exist?
// 0 with initialized=false when the gate is off.
func Count(w *config.Wiki) (int, bool, error) {
	cands, ok, err := Foreign(w)
	return len(cands), ok, err
}

// BlessPaths marks the given pages' CURRENT state — including absence —
// as reviewed (invariant K4). A path present on disk records its hash; a
// path only in the ledger accepts the deletion (entry removed); a path in
// neither is an error.
func BlessPaths(w *config.Wiki, rels []string) error {
	st, _, err := Load(w)
	if err != nil {
		return err
	}
	for _, rel := range rels {
		full := filepath.Join(w.Root, rel)
		if _, statErr := os.Stat(full); statErr == nil {
			h, err := hashFile(full)
			if err != nil {
				return err
			}
			st.Blessed[rel] = h
			continue
		}
		if _, has := st.Blessed[rel]; has {
			delete(st.Blessed, rel) // reviewed: the deletion stands
			continue
		}
		return fmt.Errorf("nothing to bless at %s (no such file, no ledger entry)", rel)
	}
	return st.Save(w)
}

// BlessAll baselines the gate: every current page is recorded as reviewed
// and stale ledger entries drop. First run initializes (opts in) the wiki.
func BlessAll(w *config.Wiki) (int, error) {
	st, _, err := Load(w)
	if err != nil {
		return 0, err
	}
	st.Blessed = map[string]string{}
	files := pageFiles(w)
	for _, rel := range files {
		h, err := hashFile(filepath.Join(w.Root, rel))
		if err != nil {
			return 0, err
		}
		st.Blessed[rel] = h
	}
	return len(files), st.Save(w)
}
