package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAndInstallAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "") // 빈 값 = 미설정: 개발 머신의 실제 pi 홈이 새면 안 된다

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

func TestPiDetection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")

	claude := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}

	// agent 디렉토리가 없으면 pi는 없는 것 — 설치 대상에서 빠진다.
	if got, err := DetectSkillsDirs(); err != nil || len(got) != 1 {
		t.Fatalf("pi absent: expected [claude] only, got %v (%v)", got, err)
	}

	// 기본 위치: ~/.pi/agent 가 있으면 skills/ 가 아직 없어도 감지된다.
	piDefault := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(piDefault, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := DetectSkillsDirs()
	if err != nil || len(got) != 2 || got[1] != filepath.Join(piDefault, "skills") {
		t.Fatalf("expected [claude, pi-default/skills], got %v (%v)", got, err)
	}

	// env 오버라이드는 기본값을 대체한다 — 둘 다 존재해도 env 쪽만.
	piXDG := filepath.Join(home, ".config", "pi", "agent")
	if err := os.MkdirAll(piXDG, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", piXDG)
	got, err = DetectSkillsDirs()
	if err != nil || len(got) != 2 || got[1] != filepath.Join(piXDG, "skills") {
		t.Fatalf("env override: expected [claude, xdg/skills], got %v (%v)", got, err)
	}

	// env가 없는 곳을 가리키면 pi는 감지되지 않는다 (기본값으로 새지 않음).
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "nope"))
	if got, err := DetectSkillsDirs(); err != nil || len(got) != 1 {
		t.Fatalf("dangling env: expected [claude] only, got %v (%v)", got, err)
	}

	// ~ 확장은 pi의 config 로더와 같은 규칙.
	t.Setenv("PI_CODING_AGENT_DIR", "~/.config/pi/agent")
	got, err = DetectSkillsDirs()
	if err != nil || len(got) != 2 || got[1] != filepath.Join(piXDG, "skills") {
		t.Fatalf("tilde: expected xdg/skills, got %v (%v)", got, err)
	}

	// InstallAll이 skills/ 를 만들며 flat 레이아웃으로 설치한다.
	t.Setenv("PI_CODING_AGENT_DIR", piXDG)
	if _, err := InstallAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(piXDG, "skills", "canopy-wiki", "SKILL.md")); err != nil {
		t.Errorf("pi flat layout missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(piDefault, "skills")); !os.IsNotExist(err) {
		t.Error("env override must not leak an install into the default ~/.pi/agent")
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
