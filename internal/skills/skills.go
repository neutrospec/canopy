// Package skills carries the thin hermes skills that replace the old
// prose-checklist wiki skills. Content is embedded so `canopy skills
// install` works from a bare binary.
package skills

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed canopy_wiki.md
var canopyWiki string

//go:embed canopy_ingest.md
var canopyIngest string

// Legacy skills that the two canopy skills supersede.
var Superseded = []string{
	"note-taking/wiki-management",
	"note-taking/wiki-semantic-search",
	"note-taking/wiki-search-lint",
	"note-taking/wiki-embedding-workflow",
	"note-taking/wiki-log-structure",
	"research/llm-wiki",
}

// Install writes the canopy skills into a hermes skills directory
// (e.g. ~/.hermes/skills). Existing files are overwritten — the binary
// is the source of truth for these two skills.
func Install(skillsDir string) ([]string, error) {
	targets := map[string]string{
		"note-taking/canopy-wiki/SKILL.md":   canopyWiki,
		"note-taking/canopy-ingest/SKILL.md": canopyIngest,
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

// DefaultSkillsDir is ~/.hermes/skills.
func DefaultSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermes", "skills"), nil
}

// SupersededPresent lists which legacy skills still exist under dir.
func SupersededPresent(dir string) []string {
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
