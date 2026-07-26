// Package attention records page-access events. Storage is two-tier
// (docs/web-ui-plan-4.md): this package owns the machine-local SQLite
// event log under $XDG_STATE_HOME/canopy/attention/ — per-call precision
// for rich UI (timelines, stats) — while the wiki-synced aggregate lives
// in internal/reads (_meta/webui/reads.json + _meta/attention/agent-reads.json).
//
// The event DB is state, not cache: it cannot be rebuilt, but losing it
// only loses local detail — the synced aggregates keep the resurface loop
// whole (invariant H8).
package attention

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
)

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

// Recent returns the latest n events, newest first.
func (e *Events) Recent(n int) ([]Event, error) {
	rows, err := e.db.Query(`SELECT ts, slug, door, kind, meta FROM events ORDER BY ts DESC, id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

// CountBySlug returns how many events a page has accumulated — the
// precise per-call number the day-quantized aggregate does not keep.
func (e *Events) CountBySlug(slug string) (int, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM events WHERE slug = ?`, strings.ToLower(slug)).Scan(&n)
	return n, err
}

// BySlug returns one page's latest events, newest first — the per-page
// attention panel's raw material.
func (e *Events) BySlug(slug string, n int) ([]Event, error) {
	rows, err := e.db.Query(`SELECT ts, slug, door, kind, meta FROM events WHERE slug = ? ORDER BY ts DESC, id DESC LIMIT ?`,
		strings.ToLower(slug), n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
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
// consumption.
func (e *Events) TopConsumed(since time.Time, n int) ([]Consumed, error) {
	rows, err := e.db.Query(`
		SELECT slug, COUNT(*),
		       SUM(door = 'web'), SUM(door = 'agent')
		FROM events
		WHERE ts >= ? AND slug <> ''
		GROUP BY slug
		ORDER BY COUNT(*) DESC, slug
		LIMIT ?`, since.Format(time.RFC3339), n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Consumed
	for rows.Next() {
		var c Consumed
		if err := rows.Scan(&c.Slug, &c.Total, &c.Web, &c.Agent); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// WeeklyCounts buckets one page's events into per-week counts covering
// the last `weeks` weeks, oldest bucket first — the sparkline's data.
func (e *Events) WeeklyCounts(slug string, weeks int, now time.Time) ([]int, error) {
	cutoff := now.Add(-time.Duration(weeks) * 7 * 24 * time.Hour)
	rows, err := e.db.Query(`SELECT ts FROM events WHERE slug = ? AND ts >= ?`,
		strings.ToLower(slug), cutoff.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make([]int, weeks)
	for rows.Next() {
		var ts string
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		idx := weeks - 1 - int(now.Sub(t).Hours()/(24*7))
		if idx >= 0 && idx < weeks {
			counts[idx]++
		}
	}
	return counts, rows.Err()
}
