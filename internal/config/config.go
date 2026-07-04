// Package config locates the wiki root and loads canopy.toml.
//
// Resolution order for the wiki root:
//  1. --wiki flag (explicit path)
//  2. CANOPY_WIKI environment variable
//  3. walk up from cwd looking for canopy.toml
//  4. default_wiki in ~/.canopy/config.toml
//
// A wiki without canopy.toml is usable read-only with built-in defaults;
// `canopy init` materializes the defaults into <wiki>/canopy.toml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Schema    Schema    `toml:"schema"`
	Embedding Embedding `toml:"embedding"`
}

type Schema struct {
	// Types allowed in frontmatter `type:`.
	Types []string `toml:"types"`
	// Tags is the allowed taxonomy for frontmatter `tags:`.
	Tags []string `toml:"tags"`
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
			Tags: []string{
				"person", "company", "community",
				"ai-ml", "science", "philosophy", "history", "language", "math", "politics",
				"health", "psychology", "fitness", "nutrition", "sleep",
				"business", "finance", "career", "productivity", "startup",
				"book", "movie", "music", "travel", "cooking", "hobby",
				"programming", "tool", "hardware", "infrastructure", "hacking",
				"comparison", "timeline", "controversy", "prediction", "method",
				"definition", "decision", "review", "debugging",
			},
			MinWikilinks: 2,
			MaxLines:     1000,
			StaleDays:    90,
		},
		Embedding: Embedding{
			Model:       "bge-m3-int8",
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
}

func (w *Wiki) DBPath() string      { return filepath.Join(w.Root, ".canopy", "index.db") }
func (w *Wiki) TOMLPath() string    { return filepath.Join(w.Root, "canopy.toml") }
func (w *Wiki) CanopyDir() string   { return filepath.Join(w.Root, ".canopy") }
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
		if _, err := toml.DecodeFile(tomlPath, w.Cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", tomlPath, err)
		}
		w.HasTOML = true
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
	if home, err := os.UserHomeDir(); err == nil {
		gc := globalConfig{}
		p := filepath.Join(home, ".canopy", "config.toml")
		if _, err := toml.DecodeFile(p, &gc); err == nil && gc.DefaultWiki != "" {
			return gc.DefaultWiki, nil
		}
	}
	return "", fmt.Errorf("no wiki found: pass --wiki, set CANOPY_WIKI, or set default_wiki in ~/.canopy/config.toml")
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
	fmt.Fprintln(f)
	return toml.NewEncoder(f).Encode(w.Cfg)
}
