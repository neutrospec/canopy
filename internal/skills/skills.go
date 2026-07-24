// Package skills carries the thin agent skills that teach an LLM agent
// to drive the wiki through canopy commands. Content is embedded so
// `canopy skills install` works from a bare binary.
//
// Any agent with a skills directory is supported generically (flat
// <skill>/SKILL.md layout). hermes, the original integration, gets
// extra care: first priority in auto-detection, its category layout
// (note-taking/<skill>/SKILL.md), and cleanup hints for the legacy
// prose-checklist skills that canopy superseded.
package skills

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed canopy_wiki.md
var canopyWiki string

//go:embed canopy_ingest.md
var canopyIngest string

// Legacy hermes-era skills that the two canopy skills supersede.
var Superseded = []string{
	"note-taking/wiki-management",
	"note-taking/wiki-semantic-search",
	"note-taking/wiki-search-lint",
	"note-taking/wiki-embedding-workflow",
	"note-taking/wiki-log-structure",
	"research/llm-wiki",
}

// KnownSkillsDirs returns the agent skills directories canopy can
// auto-detect, in priority order.
func KnownSkillsDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(home, ".hermes", "skills"), // hermes
		filepath.Join(home, ".claude", "skills"), // Claude Code
	}, nil
}

// DefaultSkillsDir returns the first known agent skills directory that
// exists on this machine.
func DefaultSkillsDir() (string, error) {
	candidates, err := KnownSkillsDirs()
	if err != nil {
		return "", err
	}
	for _, dir := range candidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no agent skills directory found (looked for %s) — pass --dir",
		strings.Join(candidates, ", "))
}

// isHermesDir reports whether dir is a hermes skills tree, which uses
// category folders (note-taking/…) instead of the flat layout.
func isHermesDir(dir string) bool {
	return strings.Contains(filepath.ToSlash(dir), "/.hermes/")
}

// Install writes the canopy skills into an agent skills directory.
// Existing files are overwritten — the binary is the source of truth
// for these two skills.
func Install(skillsDir string) ([]string, error) {
	prefix := ""
	if isHermesDir(skillsDir) {
		prefix = "note-taking/"
	}
	targets := map[string]string{
		prefix + "canopy-wiki/SKILL.md":   canopyWiki,
		prefix + "canopy-ingest/SKILL.md": canopyIngest,
	}
	var written []string
	for rel, content := range targets {
		path := filepath.Join(skillsDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

// SupersededPresent lists which legacy skills still exist under dir.
// The legacy skills are hermes-era; other agents never had them.
func SupersededPresent(dir string) []string {
	if !isHermesDir(dir) {
		return nil
	}
	var present []string
	for _, s := range Superseded {
		if _, err := os.Stat(filepath.Join(dir, s, "SKILL.md")); err == nil {
			present = append(present, s)
		}
	}
	return present
}

// RemovalHint renders the manual cleanup guidance printed after install.
func RemovalHint(dir string, present []string) string {
	if len(present) == 0 {
		return ""
	}
	out := "superseded legacy skills still present (back up, then remove when ready):\n"
	for _, s := range present {
		out += fmt.Sprintf("  %s\n", filepath.Join(dir, s))
	}
	return out
}
