// Package config locates the wiki root and loads canopy.toml.
//
// Resolution order for the wiki root:
//  1. --wiki flag (explicit path)
//  2. CANOPY_WIKI environment variable
//  3. walk up from cwd looking for canopy.toml
//  4. default_wiki in $XDG_CONFIG_HOME/canopy/config.toml
//
// A wiki without canopy.toml is usable read-only with built-in defaults;
// `canopy init` materializes the defaults into <wiki>/canopy.toml.
//
// Path layout follows XDG Base Directory:
//   - config (global settings)      $XDG_CONFIG_HOME/canopy  (~/.config/canopy)
//   - cache (derived, rebuildable)  $XDG_CACHE_HOME/canopy   (~/.cache/canopy)
//   - data (models, static libs)    $XDG_DATA_HOME/canopy    (~/.local/share/canopy)
//
// Only two things live inside the wiki itself, both deliberately:
// canopy.toml (the wiki's schema travels with its data) and
// _meta/resurface/ (non-derivable state that must sync across devices).
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func xdgDir(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return filepath.Join(v, "canopy")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "canopy")
	}
	return filepath.Join(home, fallback, "canopy")
}

// ConfigHome is $XDG_CONFIG_HOME/canopy (default ~/.config/canopy).
func ConfigHome() string { return xdgDir("XDG_CONFIG_HOME", ".config") }

// CacheHome is $XDG_CACHE_HOME/canopy (default ~/.cache/canopy).
func CacheHome() string { return xdgDir("XDG_CACHE_HOME", ".cache") }

// DataHome is $XDG_DATA_HOME/canopy (default ~/.local/share/canopy).
func DataHome() string { return xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share")) }

// StateHome is $XDG_STATE_HOME/canopy (default ~/.local/state/canopy) —
// machine-local, non-derivable history (the attention event log). Not cache
// (cannot be rebuilt) and not data (it is history, per the XDG state spec).
func StateHome() string { return xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state")) }

type Config struct {
	Schema    Schema    `toml:"schema"`
	Embedding Embedding `toml:"embedding"`
}

type Schema struct {
	// Types allowed in frontmatter `type:`.
	Types []string `toml:"types"`
	// Topics is the subject facet of the tag taxonomy — an open set that
	// grows on demand and shrinks on disuse (docs/taxonomy.md).
	Topics []string `toml:"topics"`
	// Forms is the page-kind facet (method, review, timeline, …) — a
	// closed set, exempt from the growth/pressure rules.
	Forms []string `toml:"forms"`
	// Tags is the legacy single-list taxonomy. Read tolerantly (treated
	// as the whole taxonomy when topics/forms are absent); new configs
	// declare topics/forms instead.
	Tags []string `toml:"tags,omitempty"`
	// BroadTopicPct flags a topic used by more than this percentage of
	// pages as having lost discriminating power (invariant S4). 0 disables.
	BroadTopicPct int `toml:"broad_topic_pct"`
	// PageDirs are directories whose *.md files are schema-governed pages.
	PageDirs []string `toml:"page_dirs"`
	// RootFiles are the only .md files allowed in the wiki root.
	RootFiles []string `toml:"root_files"`
	// MinWikilinks is the soft minimum of outbound links per page.
	MinWikilinks int `toml:"min_wikilinks"`
	// MaxLines flags pages longer than this for splitting.
	MaxLines int `toml:"max_lines"`
	// StaleDays flags pages not updated for this many days.
	StaleDays int `toml:"stale_days"`
}

// AllTags is the valid-tag set that validation (new/lint) enforces:
// topics ∪ forms ∪ legacy tags, deduplicated, declaration order kept.
func (s *Schema) AllTags() []string {
	seen := map[string]bool{}
	var all []string
	for _, group := range [][]string{s.Topics, s.Forms, s.Tags} {
		for _, t := range group {
			if !seen[t] {
				seen[t] = true
				all = append(all, t)
			}
		}
	}
	return all
}

type Embedding struct {
	Model       string `toml:"model"`
	Dimension   int    `toml:"dimension"`
	ChunkTokens int    `toml:"chunk_tokens"`
}

// Default mirrors the conventions already established in the existing
// wiki's SCHEMA.md (types, tag taxonomy, directory layout).
func Default() *Config {
	return &Config{
		Schema: Schema{
			Types:    []string{"entity", "concept", "comparison"},
			PageDirs: []string{"entities", "concepts", "comparisons"},
			RootFiles: []string{
				"index.md", "SCHEMA.md",
			},
			Topics: []string{
				"person", "company", "community",
				"ai-ml", "science", "philosophy", "history", "language", "math", "politics",
				"health", "psychology", "fitness", "nutrition", "sleep",
				"business", "finance", "career", "productivity", "startup",
				"book", "movie", "music", "travel", "cooking", "hobby",
				"programming", "tool", "hardware", "infrastructure", "hacking",
			},
			Forms: []string{
				"comparison", "timeline", "controversy", "prediction", "method",
				"definition", "decision", "review", "debugging",
			},
			BroadTopicPct: 25,
			MinWikilinks:  2,
			MaxLines:      1000,
			StaleDays:     90,
		},
		Embedding: Embedding{
			// Model defaults to the fp32 directory that `canopy model pull`
			// downloads. Users who run scripts/quantize-model.py set this
			// to "bge-m3-int8" in their canopy.toml.
			Model:       "bge-m3",
			Dimension:   1024,
			ChunkTokens: 400,
		},
	}
}

// Wiki bundles a resolved wiki root with its configuration.
type Wiki struct {
	Root string
	Cfg  *Config
	// HasTOML reports whether <root>/canopy.toml exists (i.e. init was run).
	HasTOML bool
	// LegacyTags reports that canopy.toml declares the old single `tags`
	// list without the topic/form facets — read tolerantly (the list is
	// the whole taxonomy); `canopy tags --audit` recommends migrating.
	LegacyTags bool
}

// DBPath keys the derived index cache by wiki path so multiple wikis
// coexist: $XDG_CACHE_HOME/canopy/index/<sha256[:12] of root>.db.
func (w *Wiki) DBPath() string {
	sum := sha256.Sum256([]byte(w.Root))
	return filepath.Join(CacheHome(), "index", hex.EncodeToString(sum[:])[:12]+".db")
}

// AttentionDBPath is the machine-local access-event log for this wiki,
// keyed like DBPath but under state (it is history, not a rebuildable
// cache — see docs/web-ui-plan-4.md).
func (w *Wiki) AttentionDBPath() string {
	sum := sha256.Sum256([]byte(w.Root))
	return filepath.Join(StateHome(), "attention", hex.EncodeToString(sum[:])[:12]+".db")
}

func (w *Wiki) TOMLPath() string    { return filepath.Join(w.Root, "canopy.toml") }
func (w *Wiki) LogsDir() string     { return filepath.Join(w.Root, "logs") }
func (w *Wiki) IndexMDPath() string { return filepath.Join(w.Root, "index.md") }

type globalConfig struct {
	DefaultWiki string `toml:"default_wiki"`
}

// Resolve finds the wiki root and loads its config.
func Resolve(explicit string) (*Wiki, error) {
	root, err := findRoot(explicit)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("wiki root is not a directory: %s", root)
	}
	w := &Wiki{Root: root, Cfg: Default()}
	tomlPath := w.TOMLPath()
	if _, err := os.Stat(tomlPath); err == nil {
		md, err := toml.DecodeFile(tomlPath, w.Cfg)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", tomlPath, err)
		}
		w.HasTOML = true
		// Legacy single-list taxonomy: the file's `tags` IS the whole
		// taxonomy — drop the built-in topic/form defaults so validation
		// enforces exactly what the wiki declared (rule 1b: tolerant read).
		if md.IsDefined("schema", "tags") &&
			!md.IsDefined("schema", "topics") && !md.IsDefined("schema", "forms") {
			w.LegacyTags = true
			w.Cfg.Schema.Topics = nil
			w.Cfg.Schema.Forms = nil
		}
	}
	return w, nil
}

func findRoot(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("CANOPY_WIKI"); env != "" {
		return env, nil
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "canopy.toml")); err == nil {
				return dir, nil
			}
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	gc := globalConfig{}
	if _, err := toml.DecodeFile(filepath.Join(ConfigHome(), "config.toml"), &gc); err == nil && gc.DefaultWiki != "" {
		return gc.DefaultWiki, nil
	}
	return "", fmt.Errorf("no wiki found: pass --wiki, set CANOPY_WIKI, or set default_wiki in %s", filepath.Join(ConfigHome(), "config.toml"))
}

// WriteTOML writes the current config to <root>/canopy.toml.
func (w *Wiki) WriteTOML() error {
	f, err := os.Create(w.TOMLPath())
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "# canopy configuration — machine-readable schema for this wiki.")
	fmt.Fprintln(f, "# Tag taxonomy source of truth lives here; SCHEMA.md is the human narrative.")
	fmt.Fprintln(f, "# topics = subject facet (open set, demand-driven); forms = page-kind facet")
	fmt.Fprintln(f, "# (closed set, frozen). Governance rules: docs/taxonomy.md in the canopy repo.")
	fmt.Fprintln(f)
	return toml.NewEncoder(f).Encode(w.Cfg)
}
