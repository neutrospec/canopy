// Package store maintains the derived SQLite index at <wiki>/.canopy/index.db.
// The DB is a cache: it can be deleted and rebuilt from the markdown files
// at any time with `canopy reindex`.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/nobocop/canopy/internal/wiki"
)

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS pages (
	slug     TEXT PRIMARY KEY,
	rel_path TEXT NOT NULL,
	title    TEXT,
	type     TEXT,
	created  TEXT,
	updated  TEXT,
	tags     TEXT
);
CREATE VIRTUAL TABLE IF NOT EXISTS fts USING fts5(
	slug, title, body, tokenize='unicode61 remove_diacritics 2'
);
CREATE TABLE IF NOT EXISTS chunks (
	id       INTEGER PRIMARY KEY,
	slug     TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	text     TEXT NOT NULL,
	vector   BLOB
);
CREATE INDEX IF NOT EXISTS idx_chunks_slug ON chunks(slug);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT);
`

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// RebuildPages replaces the pages and fts tables from a fresh scan.
// Embedding chunks are managed separately (M2) so a metadata rebuild
// does not throw away vectors.
func (s *Store) RebuildPages(pages []*wiki.Page) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM pages`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM fts`); err != nil {
		return err
	}
	insPage, err := tx.Prepare(`INSERT INTO pages(slug, rel_path, title, type, created, updated, tags) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	insFTS, err := tx.Prepare(`INSERT INTO fts(slug, title, body) VALUES(?,?,?)`)
	if err != nil {
		return err
	}
	for _, p := range pages {
		if _, err := insPage.Exec(p.Slug, p.RelPath, p.Title, p.Type, p.Created, p.Updated, strings.Join(p.Tags, ",")); err != nil {
			return fmt.Errorf("%s: %w", p.RelPath, err)
		}
		if _, err := insFTS.Exec(p.Slug, p.Title, p.Body); err != nil {
			return fmt.Errorf("%s: %w", p.RelPath, err)
		}
	}
	return tx.Commit()
}

type Hit struct {
	Slug    string  `json:"slug"`
	RelPath string  `json:"rel_path"`
	Title   string  `json:"title"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"`
}

// SearchKeyword runs BM25-ranked FTS5 search. Query terms are converted
// to prefix matches so Korean particles (조사) and English suffixes still
// match: "프로토콜" finds "프로토콜은".
func (s *Store) SearchKeyword(query string, k int) ([]Hit, error) {
	match := buildMatch(query)
	if match == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT f.slug, p.rel_path, p.title,
		       bm25(fts, 10.0, 5.0, 1.0) AS rank,
		       snippet(fts, 2, '»', '«', '…', 12)
		FROM fts f JOIN pages p ON p.slug = f.slug
		WHERE fts MATCH ?
		ORDER BY rank LIMIT ?`, match, k)
	if err != nil {
		return nil, fmt.Errorf("fts query %q: %w", match, err)
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		var rank float64
		if err := rows.Scan(&h.Slug, &h.RelPath, &h.Title, &rank, &h.Snippet); err != nil {
			return nil, err
		}
		// bm25() returns negative-is-better; flip sign so higher is better.
		h.Score = -rank
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// buildMatch converts free text into an FTS5 MATCH expression:
// each term quoted (protects operators) with a prefix star.
func buildMatch(query string) string {
	var terms []string
	for _, t := range strings.Fields(query) {
		t = strings.ReplaceAll(t, `"`, "")
		if t == "" {
			continue
		}
		terms = append(terms, `"`+t+`"*`)
	}
	return strings.Join(terms, " ")
}

func (s *Store) PageCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&n)
	return n, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) GetMeta(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}
