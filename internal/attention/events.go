// Package attention owns the machine-local SQLite event log under
// $XDG_STATE_HOME/canopy/attention/ — the wiki's observation timeline
// (docs/events.md). Two event domains share one table:
//
//   - attention (undotted kinds): page-access precision for rich UI,
//     while the wiki-synced aggregate lives in internal/reads
//     (_meta/webui/reads.json + _meta/attention/agent-reads.json).
//   - lifecycle (dotted kinds, `domain.action`): task/sync/reconcile
//     activity that has no wiki-side timeline. Excluded from attention
//     instruments (invariant N3) by the dot discriminator.
//
// Events are observations, never truth (N1): the DB is state, not cache —
// it cannot be rebuilt, but losing it only loses local history; every
// judgment and queue lives in the wiki. Logging is best-effort (N2).
package attention

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Doors and kinds recorded in events. Exposure (search hits, resurface
// cards) is deliberately NOT an event kind: 노출은 읽음이 아니다 (원칙 12, H5).
const (
	DoorAgent = "agent" // canopy show / recall
	DoorWeb   = "web"   // web UI

	KindShow   = "show"   // agent: canopy show
	KindRecall = "recall" // agent: canopy recall chunk source
	KindView   = "view"   // web: page opened (weaker than a read)
	KindRead   = "read"   // web: explicit/auto read mark
	KindReread = "reread" // web: explicit re-read of an already-read page
	KindSearch = "search" // either door: a query was asked (slug empty —
	// the query itself is recorded, never the exposed hits, per H5)

	// Lifecycle kinds (docs/events.md §2). Dotted `domain.action` names;
	// attention kinds stay undotted (legacy rows are history — no rename).
	// Wiki truth is never copied into meta, only pointed at (N6).
	KindTaskFiled     = "task.filed"           // meta: task id
	KindTaskDone      = "task.done"            // meta: task id
	KindTaskDismissed = "task.dismissed"       // meta: task id
	KindTaskRejected  = "task.verify_rejected" // meta: "id: reason" — the one signal task files don't keep
	KindTaskGC        = "task.gc"              // meta: removed count
	KindSyncDone      = "sync.done"            // meta: "committed=<bool> pushed=<bool>"
	KindBless         = "reconcile.bless"      // meta: rel path, or "all:N" for a baseline bless
)

// IsLifecycle reports whether a kind belongs to the lifecycle domain —
// the dot is the discriminator (events.md §2). Attention instruments
// filter these out so a filed task never reads as page consumption (N3).
func IsLifecycle(kind string) bool { return strings.Contains(kind, ".") }

// schemaVersion is the event DB's PRAGMA user_version. The DB is
// machine-local non-reproducible state: evolve additively; a breaking
// change would ride the migrate ladder (AGENTS.md 규칙 1), never a
// drop-and-rebuild (that's for caches).
const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	ts   TEXT NOT NULL,
	slug TEXT NOT NULL,
	door TEXT NOT NULL,
	kind TEXT NOT NULL,
	meta TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_events_slug ON events(slug, ts);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
`

// Events is an open handle on the machine-local event log.
type Events struct {
	db *sql.DB
}

func Open(path string) (*Events, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("attention schema: %w", err)
	}
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err == nil && v == 0 {
		db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
	}
	return &Events{db: db}, nil
}

func (e *Events) Close() error { return e.db.Close() }

// Log appends one access event. Slugs are stored lowercased so event
// queries join cleanly with the aggregates.
func (e *Events) Log(now time.Time, slug, door, kind, meta string) error {
	_, err := e.db.Exec(`INSERT INTO events(ts, slug, door, kind, meta) VALUES(?,?,?,?,?)`,
		now.Format(time.RFC3339), strings.ToLower(slug), door, kind, meta)
	return err
}

// Event is one recorded access, for instruments and tests.
type Event struct {
	TS   string `json:"ts"`
	Slug string `json:"slug"`
	Door string `json:"door"`
	Kind string `json:"kind"`
	Meta string `json:"meta,omitempty"`
}

// Recent returns the latest n events, newest first (both domains —
// the raw surface `canopy events` also uses Query directly).
func (e *Events) Recent(n int) ([]Event, error) {
	return e.Query(Filter{Limit: n})
}

// Filter selects events for Query. Zero values mean "no constraint".
type Filter struct {
	Kind     string    // exact kind, or a prefix ending in '*' (e.g. "task.*")
	Slug     string    // lowercased match
	Since    time.Time // inclusive lower bound
	Limit    int       // 0 = a generous default
	AttnOnly bool      // exclude lifecycle (dotted) kinds — instrument mode (N3)
}

// Query returns matching events, newest first.
func (e *Events) Query(f Filter) ([]Event, error) {
	out, err := e.queryAll(f)
	if err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Legacy rows retain their original offset. Parse instants in Go rather than
// comparing timestamp text (or losing sub-millisecond precision in SQLite).
func (e *Events) queryAll(f Filter) ([]Event, error) {
	where := []string{"1=1"}
	var args []any
	switch {
	case strings.HasSuffix(f.Kind, "*"):
		where = append(where, "kind LIKE ?")
		args = append(args, strings.TrimSuffix(f.Kind, "*")+"%")
	case f.Kind != "":
		where = append(where, "kind = ?")
		args = append(args, f.Kind)
	}
	if f.Slug != "" {
		where = append(where, "slug = ?")
		args = append(args, strings.ToLower(f.Slug))
	}
	if f.AttnOnly {
		where = append(where, "instr(kind, '.') = 0")
	}
	rows, err := e.db.Query(`SELECT ts, slug, door, kind, meta FROM events WHERE `+
		strings.Join(where, " AND ")+` ORDER BY id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	valid := out[:0]
	for _, ev := range out {
		t, err := time.Parse(time.RFC3339, ev.TS)
		if !f.Since.IsZero() && (err != nil || t.Before(f.Since)) {
			continue
		}
		valid = append(valid, ev)
	}
	sort.SliceStable(valid, func(i, j int) bool {
		a, ea := time.Parse(time.RFC3339, valid[i].TS)
		b, eb := time.Parse(time.RFC3339, valid[j].TS)
		if ea != nil {
			return false
		}
		if eb != nil {
			return true
		}
		return a.After(b)
	})
	return valid, nil
}

// Prune deletes events older than the cutoff — the retention half of
// `canopy events gc`. Machine-local only; the wiki is never touched (N5).
func (e *Events) Prune(before time.Time) (int, error) {
	rows, err := e.db.Query(`SELECT id, ts FROM events`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var ts string
		if err := rows.Scan(&id, &ts); err != nil {
			rows.Close()
			return 0, err
		}
		if instant, err := time.Parse(time.RFC3339, ts); err == nil && instant.Before(before) {
			ids = append(ids, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	tx, err := e.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	removed := 0
	for _, id := range ids {
		res, err := tx.Exec(`DELETE FROM events WHERE id = ?`, id)
		if err != nil {
			return 0, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		removed += int(n)
	}
	return removed, tx.Commit()
}

// CountBySlug returns how many events a page has accumulated — the
// precise per-call number the day-quantized aggregate does not keep.
func (e *Events) CountBySlug(slug string) (int, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM events WHERE slug = ?`, strings.ToLower(slug)).Scan(&n)
	return n, err
}

// BySlug returns one page's latest attention events, newest first — the
// per-page attention panel's raw material (lifecycle excluded, N3).
func (e *Events) BySlug(slug string, n int) ([]Event, error) {
	return e.Query(Filter{Slug: slug, Limit: n, AttnOnly: true})
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.TS, &ev.Slug, &ev.Door, &ev.Kind, &ev.Meta); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// Consumed aggregates one page's events for a window — digest's
// "most consumed" retrospective material (invariant G8).
type Consumed struct {
	Slug  string `json:"slug"`
	Total int    `json:"events"`
	Web   int    `json:"web_events"`
	Agent int    `json:"agent_events"`
}

// TopConsumed ranks pages by access events since the cutoff. Search
// events carry no slug and are excluded — queries are demand, not
// consumption. Lifecycle events are excluded the same way (N3): a filed
// task is not a read.
func (e *Events) TopConsumed(since time.Time, n int) ([]Consumed, error) {
	if n < 0 {
		return nil, fmt.Errorf("limit must not be negative")
	}
	rows, err := e.queryAll(Filter{Since: since, AttnOnly: true})
	if err != nil {
		return nil, err
	}
	counts := map[string]*Consumed{}
	for _, ev := range rows {
		if ev.Slug == "" {
			continue
		}
		c := counts[ev.Slug]
		if c == nil {
			c = &Consumed{Slug: ev.Slug}
			counts[ev.Slug] = c
		}
		c.Total++
		if ev.Door == DoorWeb {
			c.Web++
		}
		if ev.Door == DoorAgent {
			c.Agent++
		}
	}
	var out []Consumed
	for _, c := range counts {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Slug < out[j].Slug
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// WeeklyCounts buckets one page's events into per-week counts covering
// the last `weeks` weeks, oldest bucket first — the sparkline's data.
func (e *Events) WeeklyCounts(slug string, weeks int, now time.Time) ([]int, error) {
	if weeks < 0 || weeks > 5200 {
		return nil, fmt.Errorf("weeks must be between 0 and 5200")
	}
	if weeks == 0 {
		return []int{}, nil
	}
	cutoff := now.Add(-time.Duration(weeks) * 7 * 24 * time.Hour)
	rows, err := e.queryAll(Filter{Slug: slug, Since: cutoff, AttnOnly: true})
	if err != nil {
		return nil, err
	}
	counts := make([]int, weeks)
	for _, ev := range rows {
		t, err := time.Parse(time.RFC3339, ev.TS)
		if err != nil || t.After(now) || t.Before(cutoff) {
			continue
		}
		idx := weeks - 1 - int(now.Sub(t).Hours()/(24*7))
		if idx == -1 && t.Equal(cutoff) {
			idx = 0
		}
		if idx >= 0 && idx < weeks {
			counts[idx]++
		}
	}
	return counts, nil
}
