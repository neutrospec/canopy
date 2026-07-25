package migrate

import (
	"fmt"
	"os"
	"path/filepath"
)

// ladder is the append-only list of migrations, one per rung. Index i is the
// migration that lands the on-disk state on version i+1.
//
// RULES (enforced by convention — see AGENTS.md and docs/versioning.md):
//   - NEVER edit, remove, or reorder an entry once it has shipped. It is a
//     historical fact; correct a mistake with a new rung at the end.
//   - Add a rung whenever a change alters canopy's durable on-disk state in a
//     way old data would not satisfy: a renamed/added config field with a
//     non-trivial default, a moved directory, a changed _meta/* format.
//   - Do NOT add a rung for index-DB layout changes. That DB is a rebuildable
//     cache — bump internal/store.SchemaVersion and let `reindex` recreate it.
//   - Every Run must be idempotent and a no-op when already satisfied, because
//     a pre-versioning install replays the entire ladder from rung 0.
var ladder = []Migration{
	{To: 1, Name: "relocate legacy ~/.canopy into XDG directories", Run: migrate001LegacyXDG},
}

// migrate001LegacyXDG moves a pre-XDG installation (~/.canopy/...) into the
// XDG layout canopy now uses (config → $XDG_CONFIG_HOME/canopy, models & libs
// → $XDG_DATA_HOME/canopy). It is a no-op — the overwhelmingly common case —
// when no ~/.canopy directory exists.
func migrate001LegacyXDG(ctx *Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	legacy := filepath.Join(home, ".canopy")
	if fi, err := os.Stat(legacy); err != nil || !fi.IsDir() {
		return nil // nothing to migrate
	}

	moves := []struct{ from, to string }{
		{filepath.Join(legacy, "models"), filepath.Join(ctx.DataHome, "models")},
		{filepath.Join(legacy, "lib"), filepath.Join(ctx.DataHome, "lib")},
		{filepath.Join(legacy, "config.toml"), filepath.Join(ctx.ConfigHome, "config.toml")},
		{filepath.Join(legacy, "webauth.json"), filepath.Join(ctx.ConfigHome, "webauth.json")},
	}
	for _, mv := range moves {
		if err := relocate(ctx, mv.from, mv.to); err != nil {
			return err
		}
	}

	// Retire the legacy dir so this can never fire twice. Rename rather than
	// delete: any unrecognized leftovers stay visible in ~/.canopy.migrated
	// instead of being silently destroyed.
	retired := legacy + ".migrated"
	if err := os.Rename(legacy, retired); err != nil {
		ctx.logf("note: could not retire %s (%v) — safe to remove by hand", legacy, err)
	}
	return nil
}

// relocate moves from→to when the source exists and the destination does not,
// creating parent directories. It never overwrites an existing destination:
// if the XDG copy already exists it wins and the legacy source is left in
// place (visible, not destroyed).
func relocate(ctx *Context, from, to string) error {
	if _, err := os.Stat(from); err != nil {
		return nil // source absent — skip
	}
	if _, err := os.Stat(to); err == nil {
		ctx.logf("note: %s already exists — leaving legacy %s untouched", to, from)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("move %s → %s: %w", from, to, err)
	}
	ctx.logf("moved %s → %s", from, to)
	return nil
}
