package store

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ChunkHashes returns each page's stored chunk hashes joined by "," —
// a cheap fingerprint to decide whether a page needs re-embedding.
func (s *Store) ChunkHashes() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT slug, hash FROM chunks ORDER BY slug, seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := map[string][]string{}
	for rows.Next() {
		var slug, hash string
		if err := rows.Scan(&slug, &hash); err != nil {
			return nil, err
		}
		parts[slug] = append(parts[slug], hash)
	}
	out := make(map[string]string, len(parts))
	for slug, hs := range parts {
		out[slug] = strings.Join(hs, ",")
	}
	return out, rows.Err()
}

// ReplaceChunks swaps a page's embedded chunks atomically.
func (s *Store) ReplaceChunks(slug string, seqs []int, hashes, texts []string, vectors [][]float32) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM chunks WHERE slug=?`, slug); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO chunks(slug, seq, hash, text, vector) VALUES(?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for i := range seqs {
		if _, err := ins.Exec(slug, seqs[i], hashes[i], texts[i], encodeVec(vectors[i])); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PruneChunks removes vectors for pages that no longer exist.
func (s *Store) PruneChunks(liveSlugs map[string]bool) (int, error) {
	rows, err := s.db.Query(`SELECT DISTINCT slug FROM chunks`)
	if err != nil {
		return 0, err
	}
	var stale []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			rows.Close()
			return 0, err
		}
		if !liveSlugs[slug] {
			stale = append(stale, slug)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, slug := range stale {
		if _, err := s.db.Exec(`DELETE FROM chunks WHERE slug=?`, slug); err != nil {
			return 0, err
		}
	}
	return len(stale), nil
}

func (s *Store) ChunkCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&n)
	return n, err
}

// SearchSemantic brute-force scans all chunk vectors (unit-normalized,
// so cosine = dot product) and returns the best-scoring pages. At the
// current scale (thousands of chunks) this runs in milliseconds.
func (s *Store) SearchSemantic(query []float32, k int) ([]Hit, error) {
	rows, err := s.db.Query(`
		SELECT c.slug, p.rel_path, p.title, c.vector, c.text
		FROM chunks c JOIN pages p ON p.slug = c.slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	best := map[string]*Hit{}
	for rows.Next() {
		var slug, relPath, title, text string
		var blob []byte
		if err := rows.Scan(&slug, &relPath, &title, &blob, &text); err != nil {
			return nil, err
		}
		vec, err := decodeVec(blob)
		if err != nil {
			return nil, fmt.Errorf("chunk %s: %w", slug, err)
		}
		score := dot(query, vec)
		if h, ok := best[slug]; !ok || score > h.Score {
			best[slug] = &Hit{Slug: slug, RelPath: relPath, Title: title, Score: score, Snippet: snippetOf(text)}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(best))
	for _, h := range best {
		hits = append(hits, *h)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

func snippetOf(text string) string {
	// Chunk text starts with the page title line; snip a bit of the body.
	if i := strings.Index(text, "\n\n"); i >= 0 {
		text = text[i+2:]
	}
	runes := []rune(strings.ReplaceAll(text, "\n", " "))
	if len(runes) > 120 {
		runes = append(runes[:120], '…')
	}
	return string(runes)
}

func encodeVec(v []float32) []byte {
	buf := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeVec(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("bad vector blob length %d", len(b))
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v, nil
}

func dot(a, b []float32) float64 {
	n := min(len(a), len(b))
	var sum float64
	for i := 0; i < n; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}
