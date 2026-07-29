// Package writeops is the invariant pipeline every wiki mutation runs:
// one filesystem scan, index regeneration, activity log, reindex (with
// embeddings when the stack is available). The CLI write commands and
// the web UI editor both call Run — there is exactly one write path,
// so the invariants cannot drift between entry points.
package writeops

import (
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/genindex"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
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
	if err := reconcile.Record(w, eff); err != nil {
		return nil, err
	}
	scan, err := wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	if err := genindex.Regenerate(w, scan); err != nil {
		return nil, err
	}
	if err := logops.Append(w, action, relPath, related, note); err != nil {
		return nil, err
	}
	var eng embed.Engine
	if embed.Available() && embed.ModelAvailable() {
		if e, err := embed.New(); err == nil {
			eng = e
			defer eng.Close()
		}
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return nil, err
	}
	defer st.Close()
	if _, err := indexer.Reindex(w, st, scan, eng, nil); err != nil {
		return nil, err
	}
	return scan, nil
}
