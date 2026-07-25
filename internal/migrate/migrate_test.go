package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

// testCtx builds a Context rooted in a throwaway HOME so migrations that read
// ~/.canopy (migrate001) act on the sandbox, not the developer's machine.
func testCtx(t *testing.T) *Context {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return &Context{
		ConfigHome: filepath.Join(home, ".config", "canopy"),
		CacheHome:  filepath.Join(home, ".cache", "canopy"),
		DataHome:   filepath.Join(home, ".local", "share", "canopy"),
		Log:        func(string, ...any) {},
	}
}

// A brand-new install has nothing to migrate: it jumps straight to Target and
// records the version, without running any migration.
func TestEnsureFreshInstall(t *testing.T) {
	ctx := testCtx(t)
	res, err := Ensure(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.From != Target() || res.To != Target() {
		t.Fatalf("fresh install should start at target %d, got %d→%d", Target(), res.From, res.To)
	}
	if len(res.Applied) != 0 {
		t.Fatalf("fresh install should apply no migrations, got %v", res.Applied)
	}
	if _, err := os.Stat(statePath(ctx.ConfigHome)); err != nil {
		t.Fatalf("state.json not written on fresh install: %v", err)
	}
	rep, err := Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Current != Target() || len(rep.Pending) != 0 {
		t.Fatalf("after fresh install: current=%d pending=%d", rep.Current, len(rep.Pending))
	}
}

// A pre-versioning install (legacy ~/.canopy present) replays the ladder from
// rung 0, relocating data into the XDG layout, and is idempotent on re-run.
func TestLegacyRelocation(t *testing.T) {
	ctx := testCtx(t)
	home, _ := os.UserHomeDir()

	legacyModel := filepath.Join(home, ".canopy", "models", "bge-m3")
	if err := os.MkdirAll(legacyModel, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyModel, "model.onnx"), []byte("weights"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".canopy", "config.toml"), []byte("default_wiki='/w'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Ensure(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.From != 0 || res.To != Target() {
		t.Fatalf("expected climb 0→%d, got %d→%d", Target(), res.From, res.To)
	}
	if got := filepath.Join(ctx.DataHome, "models", "bge-m3", "model.onnx"); !exists(got) {
		t.Fatalf("model not relocated to %s", got)
	}
	if got := filepath.Join(ctx.ConfigHome, "config.toml"); !exists(got) {
		t.Fatalf("config not relocated to %s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".canopy")); !os.IsNotExist(err) {
		t.Fatalf("legacy dir should be gone (stat err=%v)", err)
	}
	if !exists(filepath.Join(home, ".canopy.migrated")) {
		t.Fatal("legacy dir should be retired to ~/.canopy.migrated")
	}

	// Idempotent: a second Ensure changes nothing.
	res2, err := Ensure(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if res2.From != Target() || len(res2.Applied) != 0 {
		t.Fatalf("second run should be a no-op, got from=%d applied=%v", res2.From, res2.Applied)
	}
}

// relocate must not clobber an existing XDG destination.
func TestRelocateDoesNotClobber(t *testing.T) {
	ctx := testCtx(t)
	home, _ := os.UserHomeDir()
	if err := os.MkdirAll(filepath.Join(home, ".canopy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".canopy", "config.toml"), []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ctx.ConfigHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctx.ConfigHome, "config.toml"), []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(ctx.ConfigHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "current\n" {
		t.Fatalf("existing XDG config was overwritten: %q", b)
	}
}

// A binary older than the data it finds must refuse rather than corrupt it.
func TestDowngradeGuard(t *testing.T) {
	ctx := testCtx(t)
	if err := saveState(ctx.ConfigHome, "future", Target()+5); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(ctx, "test"); err == nil {
		t.Fatal("expected refusal when on-disk data is newer than the binary")
	}
}

// The ladder must be dense and start at 1 (rung i lands on version i+1), the
// invariant the whole runner relies on.
func TestLadderIsDense(t *testing.T) {
	for i, m := range ladder {
		if m.To != i+1 {
			t.Fatalf("ladder[%d].To = %d, want %d (rungs must be dense from 1)", i, m.To, i+1)
		}
		if m.Run == nil || m.Name == "" {
			t.Fatalf("ladder[%d] (%q) missing Run or Name", i, m.Name)
		}
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
