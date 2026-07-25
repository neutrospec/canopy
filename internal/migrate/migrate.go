// Package migrate applies ordered, one-way migrations to canopy's durable
// on-disk state: the global config files, the model / XDG directory layout,
// and any wiki-side state formats. Everything it touches is state that
// CANNOT be regenerated from the markdown.
//
// It deliberately does NOT touch the search-index database. That DB is a
// derived cache (internal/store); its "migration" is to be dropped and
// rebuilt with `canopy reindex`, so bumping store.SchemaVersion — not a
// migration here — is the right tool for a cache-layout change. Migrating a
// throwaway would be wasted work, so the two concerns are kept apart
// (see docs/versioning.md).
//
// The applied rung is recorded machine-locally in
//
//	$XDG_CONFIG_HOME/canopy/state.json
//
// so it survives cache wipes and binary replacement. Ensure() runs on every
// startup and walks from the recorded rung up to the binary's Target(),
// applying each pending step in order and stamping the new rung after each
// one — so an interrupted upgrade resumes from the last good rung.
//
// The ladder (migrations.go) is APPEND-ONLY: a shipped migration is an
// immutable historical fact and must never be edited, removed, or reordered.
package migrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Context gives a migration the resolved machine-local directories and a
// progress logger. Migrations operate on these paths; one that needs the
// active wiki resolves it itself (most do not, which is why the wiki is not
// a field here — startup migrations must run before any wiki is known).
type Context struct {
	ConfigHome string // $XDG_CONFIG_HOME/canopy
	CacheHome  string // $XDG_CACHE_HOME/canopy
	DataHome   string // $XDG_DATA_HOME/canopy
	Log        func(format string, args ...any)
}

func (c *Context) logf(format string, args ...any) {
	if c.Log != nil {
		c.Log(format, args...)
	}
}

// Migration is one rung of the ladder. To is the version it lands on; rungs
// are dense and start at 1. Run MUST be idempotent — safe to re-run and a
// no-op when its precondition is already satisfied — because a pre-versioning
// install replays the whole ladder from rung 0.
type Migration struct {
	To   int
	Name string
	Run  func(*Context) error
}

// Target is the schema version this binary knows how to produce: exactly the
// number of registered migrations.
func Target() int { return len(ladder) }

// state is the machine-local record of how far the on-disk state has climbed.
// It is neither the app version (buildinfo) nor the cache-schema version
// (store.SchemaVersion); AppVersion is stored only as a human breadcrumb.
type state struct {
	SchemaVersion int    `json:"schema_version"`
	AppVersion    string `json:"app_version,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

func statePath(configHome string) string { return filepath.Join(configHome, "state.json") }

func loadState(configHome string) (state, bool, error) {
	b, err := os.ReadFile(statePath(configHome))
	if os.IsNotExist(err) {
		return state{}, false, nil
	}
	if err != nil {
		return state{}, false, err
	}
	var s state
	if err := json.Unmarshal(b, &s); err != nil {
		return state{}, false, fmt.Errorf("parse %s: %w", statePath(configHome), err)
	}
	return s, true, nil
}

// saveState writes the version atomically (temp + rename) so a crash mid-write
// cannot leave a truncated state.json.
func saveState(configHome, appVersion string, version int) error {
	if err := os.MkdirAll(configHome, 0o755); err != nil {
		return err
	}
	s := state{
		SchemaVersion: version,
		AppVersion:    appVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := statePath(configHome) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(configHome))
}

// Result reports what Ensure did.
type Result struct {
	From    int      `json:"from"`
	To      int      `json:"to"`
	Applied []string `json:"applied"`
}

// Ensure brings the on-disk state up to the binary's Target(), applying any
// pending migrations in order. It is safe and cheap to call on every startup:
// when already current it only reads one small file. Migrations self-report
// concrete actions via ctx.Log, so a no-op run is silent.
func Ensure(ctx *Context, appVersion string) (Result, error) {
	cur, fromFile, err := resolveCurrent(ctx)
	if err != nil {
		return Result{}, err
	}
	target := Target()

	// Refuse to run against state written by a newer canopy: applying old
	// logic to newer data risks corruption. Upgrading the binary fixes it.
	if cur > target {
		return Result{From: cur, To: cur}, fmt.Errorf(
			"canopy is older (schema %d) than your data (schema %d) — upgrade canopy", target, cur)
	}

	res := Result{From: cur, To: cur}
	if cur == target {
		// Already current. Stamp the baseline once so the version is on
		// record and `canopy version` can show it.
		if !fromFile {
			if err := saveState(ctx.ConfigHome, appVersion, target); err != nil {
				return res, err
			}
		}
		return res, nil
	}

	for _, m := range ladder {
		if m.To <= cur {
			continue
		}
		if err := m.Run(ctx); err != nil {
			return res, fmt.Errorf("migration to schema %d (%s): %w", m.To, m.Name, err)
		}
		if err := saveState(ctx.ConfigHome, appVersion, m.To); err != nil {
			return res, err
		}
		res.To = m.To
		res.Applied = append(res.Applied, m.Name)
	}
	return res, nil
}

// Report is a read-only view of migration state for `version` / `migrate status`.
type Report struct {
	Current   int
	Target    int
	Persisted bool // a state.json exists (vs. an inferred baseline)
	Pending   []Migration
}

// Status inspects the current state without changing anything.
func Status(ctx *Context) (Report, error) {
	cur, fromFile, err := resolveCurrent(ctx)
	if err != nil {
		return Report{}, err
	}
	r := Report{Current: cur, Target: Target(), Persisted: fromFile}
	for _, m := range ladder {
		if m.To > cur {
			r.Pending = append(r.Pending, m)
		}
	}
	return r, nil
}

// resolveCurrent reads the recorded rung, or infers a baseline when no state
// file exists yet.
func resolveCurrent(ctx *Context) (version int, fromFile bool, err error) {
	s, ok, err := loadState(ctx.ConfigHome)
	if err != nil {
		return 0, false, err
	}
	if ok {
		return s.SchemaVersion, true, nil
	}
	return detectBaseline(ctx), false, nil
}

// detectBaseline decides the starting rung when no state.json exists:
//
//   - a brand-new install (no trace of canopy on disk) starts at Target():
//     there is no legacy state to climb from, so no migration should run;
//   - an install whose data predates versioning starts at 0, so the whole
//     idempotent ladder replays and any old layout is brought forward.
func detectBaseline(ctx *Context) int {
	if hasExistingData(ctx) {
		return 0
	}
	return Target()
}

// hasExistingData reports whether canopy has run on this machine before the
// state file existed — the signal that a pre-versioning layout may be present.
func hasExistingData(ctx *Context) bool {
	if fileExists(filepath.Join(ctx.ConfigHome, "config.toml")) ||
		fileExists(filepath.Join(ctx.ConfigHome, "webauth.json")) ||
		dirNonEmpty(filepath.Join(ctx.DataHome, "models")) ||
		dirNonEmpty(filepath.Join(ctx.CacheHome, "index")) {
		return true
	}
	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".canopy")); err == nil {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirNonEmpty(p string) bool {
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}
