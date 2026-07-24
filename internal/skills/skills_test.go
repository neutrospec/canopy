package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSkillsDirDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := DefaultSkillsDir(); err == nil {
		t.Fatal("expected error when no known skills directory exists")
	}

	claude := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := DefaultSkillsDir(); err != nil || got != claude {
		t.Fatalf("expected %s, got %s (%v)", claude, got, err)
	}

	// hermes takes priority when both exist.
	hermes := filepath.Join(home, ".hermes", "skills")
	if err := os.MkdirAll(hermes, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := DefaultSkillsDir(); err != nil || got != hermes {
		t.Fatalf("expected %s, got %s (%v)", hermes, got, err)
	}
}

func TestInstallLayoutPerAgent(t *testing.T) {
	home := t.TempDir()

	// hermes: category layout.
	hermes := filepath.Join(home, ".hermes", "skills")
	if _, err := Install(hermes); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(hermes, "note-taking", "canopy-wiki", "SKILL.md")); err != nil {
		t.Errorf("hermes install should use note-taking/ layout: %v", err)
	}

	// generic agent (Claude Code): flat layout.
	claude := filepath.Join(home, ".claude", "skills")
	if _, err := Install(claude); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(claude, "canopy-wiki", "SKILL.md")); err != nil {
		t.Errorf("generic install should be flat: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claude, "note-taking")); !os.IsNotExist(err) {
		t.Error("generic install must not create hermes category folders")
	}

	// Legacy-cleanup hints are hermes-only care.
	legacy := filepath.Join(hermes, "note-taking", "wiki-management")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SupersededPresent(hermes); len(got) != 1 {
		t.Errorf("expected 1 superseded hermes skill, got %v", got)
	}
	if got := SupersededPresent(claude); got != nil {
		t.Errorf("non-hermes dirs have no legacy skills, got %v", got)
	}
}
