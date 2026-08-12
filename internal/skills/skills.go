// Package skills carries the thin agent skills that teach an LLM agent
// to drive the wiki through canopy commands. Content is embedded so
// `canopy skills install` works from a bare binary.
//
// Any agent with a skills directory is supported generically (flat
// <skill>/SKILL.md layout). hermes, the original integration, gets
// extra care: first priority in auto-detection, its category layout
// (note-taking/<skill>/SKILL.md), and cleanup hints for the legacy
// prose-checklist skills that canopy superseded.
//
// pi (pi-coding-agent) is detected by mirroring pi's own home
// resolution: $PI_CODING_AGENT_DIR wins, else ~/.pi/agent. Its skills
// live in <agent dir>/skills, which may not exist yet on a working pi
// install — so presence of the agent dir (not the skills dir) is the
// detection signal, and install creates skills/ on demand. No agent
// dir → pi is absent → skipped.
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

// candidate is one auto-detectable agent skills directory. marker is
// the path whose existence proves the agent is installed — usually the
// skills dir itself, but pi's skills dir is created lazily so its
// marker is the agent home instead.
type candidate struct {
	skillsDir string
	marker    string
}

// expandTilde resolves a leading ~/ the way pi's own config loader
// does, so a PI_CODING_AGENT_DIR like "~/.config/pi/agent" matches.
func expandTilde(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

// piAgentDir mirrors pi's getAgentDir: $PI_CODING_AGENT_DIR if set,
// else ~/.pi/agent. Never both — an env override must not leak
// installs into a stale default dir.
func piAgentDir() (string, error) {
	if env := os.Getenv("PI_CODING_AGENT_DIR"); env != "" {
		return expandTilde(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pi", "agent"), nil
}

func knownCandidates() ([]candidate, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	hermes := filepath.Join(home, ".hermes", "skills")
	claude := filepath.Join(home, ".claude", "skills")
	cands := []candidate{
		{skillsDir: hermes, marker: hermes}, // hermes
		{skillsDir: claude, marker: claude}, // Claude Code
	}
	if agent, err := piAgentDir(); err == nil {
		cands = append(cands, candidate{skillsDir: filepath.Join(agent, "skills"), marker: agent}) // pi
	}
	return cands, nil
}

// KnownSkillsDirs returns the agent skills directories canopy can
// auto-detect, in priority order.
func KnownSkillsDirs() ([]string, error) {
	cands, err := knownCandidates()
	if err != nil {
		return nil, err
	}
	dirs := make([]string, len(cands))
	for i, c := range cands {
		dirs[i] = c.skillsDir
	}
	return dirs, nil
}

// DetectSkillsDirs returns every known agent skills directory whose
// agent exists on this machine, in priority order.
func DetectSkillsDirs() ([]string, error) {
	cands, err := knownCandidates()
	if err != nil {
		return nil, err
	}
	var found, looked []string
	for _, c := range cands {
		looked = append(looked, c.skillsDir)
		if fi, err := os.Stat(c.marker); err == nil && fi.IsDir() {
			found = append(found, c.skillsDir)
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no agent skills directory found (looked for %s) — pass --dir",
			strings.Join(looked, ", "))
	}
	return found, nil
}

// InstallAll installs the skills into every detected agent skills
// directory — one command keeps every agent fresh after an upgrade.
func InstallAll() (map[string][]string, error) {
	dirs, err := DetectSkillsDirs()
	if err != nil {
		return nil, err
	}
	written := map[string][]string{}
	for _, dir := range dirs {
		w, err := Install(dir)
		if err != nil {
			return written, err
		}
		written[dir] = w
	}
	return written, nil
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
