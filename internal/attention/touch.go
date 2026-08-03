package attention

import (
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/reads"
)

// Touch records agent-door consumption of the given pages: every call
// appends events to the machine-local log, and the wiki aggregate
// (_meta/attention/agent-reads.json) advances at most once per page per
// day (invariant H7). The human read state is never touched (H4).
//
// Touch is best-effort by design — it returns the first error for the
// caller to mention on stderr, but a failed touch must never break the
// read that triggered it.
// LogLifecycle appends one lifecycle event (task/sync/reconcile —
// docs/events.md §2), best-effort: it never returns an error because a
// failed observation must never break the operation it observes (N2).
func LogLifecycle(w *config.Wiki, now time.Time, slug, door, kind, meta string) {
	ev, err := Open(w.AttentionDBPath())
	if err != nil {
		return
	}
	defer ev.Close()
	ev.Log(now, slug, door, kind, meta)
}

func Touch(w *config.Wiki, slugs []string, kind, meta string) error {
	if len(slugs) == 0 {
		return nil
	}
	now := time.Now()
	var firstErr error

	if ev, err := Open(w.AttentionDBPath()); err != nil {
		firstErr = err
	} else {
		for _, s := range slugs {
			if err := ev.Log(now, s, DoorAgent, kind, meta); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		ev.Close()
	}

	rs, err := reads.Load(w)
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
		return firstErr
	}
	changed := false
	for _, s := range slugs {
		if rs.MarkAgent(s, now) {
			changed = true
		}
	}
	if changed {
		if err := rs.Save(w); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
