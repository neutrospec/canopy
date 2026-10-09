// Package writeops is the invariant pipeline every wiki mutation runs:
// one filesystem scan, index regeneration, activity log, reindex (with
// embeddings when the stack is available). The CLI write commands and
// the web UI editor both call Run — there is exactly one write path,
// so the invariants cannot drift between entry points.
package writeops

import (
	"context"
	"fmt"
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/genindex"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/safefile"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
)

// Run executes the post-mutation pipeline. It performs ONE filesystem
// scan (after the mutation) and uses it for both index regeneration and
// reindexing — genindex only touches index/ files which are outside
// governed dirs, so the page set is unchanged.
//
// eff declares what the mutation wrote/removed so the reconcile ledger
// absorbs it (자동 축복, invariant K2): pipeline products never surface
// as foreign changes. Callers must be complete — mv lists the rewritten
// inbound pages, rm/archive the link-stripped sources.
func Run(w *config.Wiki, action, relPath string, related []string, note string, eff reconcile.Effects) (*wiki.ScanResult, error) {
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return nil, &StageError{"lock", err}
	}
	if err := safefile.CleanTemps(w.Root); err != nil {
		g.Unlock()
		return nil, &StageError{"cleanup", err}
	}
	scan, err := RunLocked(w, action, relPath, related, note, eff)
	g.Unlock()
	if err != nil {
		return scan, err
	}
	return scan, Refresh(w, scan)
}

type StageError struct {
	Stage string
	Err   error
}

func (e *StageError) Error() string { return fmt.Sprintf("%s: %v", e.Stage, e.Err) }
func (e *StageError) Unwrap() error { return e.Err }

// RunLocked executes only short filesystem stages under an existing exclusive
// wiki lease. Call Refresh after releasing that lease; inference is never part
// of the exclusive critical section. Errors retain the failed stage.
func RunLocked(w *config.Wiki, action, relPath string, related []string, note string, eff reconcile.Effects) (*wiki.ScanResult, error) {
	return RunLockedWithID(w, action, relPath, related, note, eff, "")
}

func RunLockedWithID(w *config.Wiki, action, relPath string, related []string, note string, eff reconcile.Effects, id string) (*wiki.ScanResult, error) {
	if err := reconcile.RecordLocked(w, eff); err != nil {
		return nil, &StageError{"ledger", err}
	}
	scan, err := wiki.Scan(w)
	if err != nil {
		return nil, &StageError{"scan", err}
	}
	if err := genindex.Regenerate(w, scan); err != nil {
		return scan, &StageError{"index", err}
	}
	if err := logops.AppendOnceLocked(w, action, relPath, related, note, id); err != nil {
		return scan, &StageError{"log", err}
	}
	return scan, nil
}

// Refresh updates the rebuildable cache outside the wiki lease. The indexer
// revalidates its source snapshot before publication.
func Refresh(w *config.Wiki, scan *wiki.ScanResult) error {
	var eng embed.Engine
	if embed.Available() && embed.ModelAvailable() {
		if e, err := embed.New(); err == nil {
			eng = e
			defer eng.Close()
		}
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return &StageError{"reindex", err}
	}
	defer st.Close()
	if _, err := indexer.Reindex(w, st, scan, eng, nil); err != nil {
		return &StageError{"reindex", err}
	}
	return nil
}
