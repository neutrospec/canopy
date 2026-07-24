package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAndInstallAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := DetectSkillsDirs(); err == nil {
		t.Fatal("expected error when no known skills directory exists")
	}

	claude := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := DetectSkillsDirs(); err != nil || len(got) != 1 || got[0] != claude {
		t.Fatalf("expected [%s], got %v (%v)", claude, got, err)
	}

	// hermes joins (and leads) when both exist.
	hermes := filepath.Join(home, ".hermes", "skills")
	if err := os.MkdirAll(hermes, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := DetectSkillsDirs()
	if err != nil || len(got) != 2 || got[0] != hermes || got[1] != claude {
		t.Fatalf("expected [hermes claude], got %v (%v)", got, err)
	}

	// InstallAll writes to both, each with its own layout.
	byDir, err := InstallAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(byDir) != 2 {
		t.Fatalf("expected 2 dirs installed, got %v", byDir)
	}
	if _, err := os.Stat(filepath.Join(hermes, "note-taking", "canopy-wiki", "SKILL.md")); err != nil {
		t.Errorf("hermes layout missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claude, "canopy-wiki", "SKILL.md")); err != nil {
		t.Errorf("claude flat layout missing: %v", err)
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
