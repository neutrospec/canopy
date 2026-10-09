package store

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/neutrospec/canopy/internal/embed"
)

// SetProfile invalidates incompatible vectors before any inference. The
// profile is an effective identity, including model files and chunker values.
func (s *Store) SetProfile(profile string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old := ""
	err = tx.QueryRow(`SELECT value FROM meta WHERE key='embedding_profile'`).Scan(&old)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if old != profile {
		if _, err = tx.Exec(`DELETE FROM chunks`); err != nil {
			return err
		}
		if _, err = tx.Exec(`DELETE FROM meta WHERE key='embedding_dimension'`); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO meta(key,value) VALUES('embedding_profile',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, profile); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.profileMu.Lock()
	s.profile = &profile
	s.profileMu.Unlock()
	return nil
}

func (s *Store) expectedProfile() (string, error) {
	s.profileMu.RLock()
	p := s.profile
	s.profileMu.RUnlock()
	current, err := s.GetMeta("embedding_profile")
	if err != nil {
		return "", err
	}
	if p != nil && *p != current {
		return "", fmt.Errorf("embedding profile changed; refresh before querying")
	}
	return current, nil
}

func (s *Store) ChunkHashes() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT c.slug,c.hash FROM chunks c JOIN pages p ON p.slug=c.slug AND p.revision=c.revision WHERE c.profile=COALESCE((SELECT value FROM meta WHERE key='embedding_profile'),'') ORDER BY c.slug,c.seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := map[string][]string{}
	for rows.Next() {
		var slug, hash string
		if err = rows.Scan(&slug, &hash); err != nil {
			return nil, err
		}
		parts[slug] = append(parts[slug], hash)
	}
	out := map[string]string{}
	for slug, hs := range parts {
		out[slug] = strings.Join(hs, ",")
	}
	return out, rows.Err()
}

// ReplaceChunks is the legacy direct-store helper; indexer uses the checked
// revision/profile variant with raw byte offsets.
func (s *Store) ReplaceChunks(slug string, seqs []int, hashes, texts []string, vectors [][]float32) error {
	if len(seqs) != len(hashes) || len(seqs) != len(texts) {
		return fmt.Errorf("chunk field counts differ")
	}
	if err := embed.ValidateVectors(vectors, len(seqs), 0); err != nil {
		return err
	}
	var revision string
	if err := s.db.QueryRow(`SELECT revision FROM pages WHERE slug=?`, slug).Scan(&revision); err != nil {
		return err
	}
	profile, err := s.GetMeta("embedding_profile")
	if err != nil {
		return err
	}
	starts, ends := make([]int, len(seqs)), make([]int, len(seqs))
	for i, t := range texts {
		ends[i] = len(t)
	}
	return s.ReplaceCurrentChunks(slug, revision, profile, seqs, hashes, texts, starts, ends, vectors)
}

// ReplaceCurrentChunks is a short atomic publish. Compare-and-swap rejects
// work computed for a snapshot/profile replaced by another refresh.
func (s *Store) ReplaceCurrentChunks(slug, revision, profile string, seqs []int, hashes, texts []string, starts, ends []int, vectors [][]float32) error {
	n := len(seqs)
	if len(hashes) != n || len(texts) != n || len(starts) != n || len(ends) != n {
		return fmt.Errorf("chunk field counts differ")
	}
	if err := embed.ValidateVectors(vectors, n, 0); err != nil {
		return err
	}
	seen := map[int]bool{}
	for i, seq := range seqs {
		if seq < 0 || seen[seq] || starts[i] < 0 || ends[i] < starts[i] || ends[i]-starts[i] != len(texts[i]) {
			return fmt.Errorf("invalid chunk sequence or byte range")
		}
		seen[seq] = true
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current string
	if err = tx.QueryRow(`SELECT revision FROM pages WHERE slug=?`, slug).Scan(&current); err != nil {
		return err
	}
	if current != revision {
		return fmt.Errorf("source changed while embedding %s; retry reindex", slug)
	}
	effective := ""
	err = tx.QueryRow(`SELECT value FROM meta WHERE key='embedding_profile'`).Scan(&effective)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if effective != profile {
		return fmt.Errorf("embedding profile changed; retry reindex")
	}
	dimension := 0
	var d string
	err = tx.QueryRow(`SELECT value FROM meta WHERE key='embedding_dimension'`).Scan(&d)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if d != "" {
		dimension, err = strconv.Atoi(d)
		if err != nil {
			return err
		}
	}
	if err = embed.ValidateVectors(vectors, n, dimension); err != nil {
		return err
	}
	if n > 0 && dimension == 0 {
		if _, err = tx.Exec(`INSERT INTO meta(key,value) VALUES('embedding_dimension',?)`, strconv.Itoa(len(vectors[0]))); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM chunks WHERE slug=?`, slug); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO chunks(slug,seq,hash,text,vector,revision,profile,start_byte,end_byte) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for i := range seqs {
		if _, err = ins.Exec(slug, seqs[i], hashes[i], texts[i], encodeVec(vectors[i]), revision, profile, starts[i], ends[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PruneChunks(live map[string]bool) (int, error) {
	rows, err := s.db.Query(`SELECT DISTINCT slug FROM chunks`)
	if err != nil {
		return 0, err
	}
	var stale []string
	for rows.Next() {
		var slug string
		if err = rows.Scan(&slug); err != nil {
			rows.Close()
			return 0, err
		}
		if !live[slug] {
			stale = append(stale, slug)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, slug := range stale {
		if _, err = s.db.Exec(`DELETE FROM chunks WHERE slug=?`, slug); err != nil {
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
func (s *Store) EmbeddedPageCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT c.slug) FROM chunks c JOIN pages p ON p.slug=c.slug AND p.revision=c.revision WHERE c.profile=COALESCE((SELECT value FROM meta WHERE key='embedding_profile'),'')`).Scan(&n)
	return n, err
}

// ChunkHit is raw evidence and enough provenance to recheck its source range.
type ChunkHit struct {
	Slug           string  `json:"slug"`
	RelPath        string  `json:"rel_path"`
	Title          string  `json:"title"`
	Seq            int     `json:"seq"`
	Score          float64 `json:"score"`
	Text           string  `json:"text"`
	SourceRevision string  `json:"source_revision"`
	Profile        string  `json:"profile"`
	StartByte      int     `json:"start_byte"`
	EndByte        int     `json:"end_byte"`
}

func chunkLess(a, b ChunkHit) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	al, bl := strings.ToLower(a.Slug), strings.ToLower(b.Slug)
	if al != bl {
		return al < bl
	}
	if a.Slug != b.Slug {
		return a.Slug < b.Slug
	}
	return a.Seq < b.Seq
}

// offer retains at most k candidates while enforcing per-page caps. A better
// chunk replaces that page's worst selected chunk before global eviction.
func offer(out []ChunkHit, h ChunkHit, k, cap int) []ChunkHit {
	if cap > 0 {
		count, worst := 0, -1
		for i := range out {
			if out[i].Slug == h.Slug {
				count++
				worst = i
			}
		}
		if count >= cap {
			if !chunkLess(h, out[worst]) {
				return out
			}
			out = append(out[:worst], out[worst+1:]...)
		}
	}
	if len(out) == k {
		if !chunkLess(h, out[len(out)-1]) {
			return out
		}
		out = out[:len(out)-1]
	}
	i := sort.Search(len(out), func(i int) bool { return chunkLess(h, out[i]) })
	out = append(out, ChunkHit{})
	copy(out[i+1:], out[i:])
	out[i] = h
	return out
}

func (s *Store) queryDimension(query []float32) error {
	d, err := s.GetMeta("embedding_dimension")
	if err != nil {
		return err
	}
	dim := 0
	if d != "" {
		dim, err = strconv.Atoi(d)
		if err != nil || dim <= 0 {
			return fmt.Errorf("invalid cached vector dimension")
		}
	}
	return embed.ValidateVector(query, dim)
}

// SearchChunks scans vectors without loading all evidence text. Candidate
// storage is O(k); only selected raw excerpts are fetched after ranking.
func (s *Store) SearchChunks(query []float32, k, perPageCap int) ([]ChunkHit, error) {
	if err := ValidateK(k); err != nil {
		return nil, err
	}
	if perPageCap < 0 || perPageCap > MaxK {
		return nil, fmt.Errorf("per-page must be between 0 and %d", MaxK)
	}
	if err := s.queryDimension(query); err != nil {
		return nil, err
	}
	profile, err := s.expectedProfile()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT c.slug,p.rel_path,p.title,c.seq,c.vector,c.revision,c.profile,c.start_byte,c.end_byte FROM chunks c JOIN pages p ON p.slug=c.slug AND p.revision=c.revision WHERE c.profile=? AND c.profile=COALESCE((SELECT value FROM meta WHERE key='embedding_profile'),'')`, profile)
	if err != nil {
		return nil, err
	}
	out := make([]ChunkHit, 0, k)
	for rows.Next() {
		var h ChunkHit
		var blob []byte
		if err = rows.Scan(&h.Slug, &h.RelPath, &h.Title, &h.Seq, &blob, &h.SourceRevision, &h.Profile, &h.StartByte, &h.EndByte); err != nil {
			rows.Close()
			return nil, err
		}
		h.Score, err = dotBlob(query, blob)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("chunk %s #%d: %w", h.Slug, h.Seq, err)
		}
		out = offer(out, h, k, perPageCap)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Keep the identity predicate on late reads: another refresh can replace
	// chunks between the scan and excerpt selection, and must not mix evidence.
	selected := out[:0]
	for _, h := range out {
		err = s.db.QueryRow(`SELECT text FROM chunks WHERE slug=? AND seq=? AND revision=? AND profile=?`, h.Slug, h.Seq, h.SourceRevision, h.Profile).Scan(&h.Text)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		selected = append(selected, h)
	}
	return selected, nil
}

func (s *Store) SearchSemantic(query []float32, k int) ([]Hit, error) {
	chunks, err := s.SearchChunks(query, k, 1)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(chunks))
	for _, c := range chunks {
		hits = append(hits, Hit{Slug: c.Slug, RelPath: c.RelPath, Title: c.Title, Score: c.Score, Snippet: snippetOf(c.Text)})
	}
	return hits, nil
}

// PageVectors only combines vectors of the current revision and profile.
func (s *Store) PageVectors() (map[string][]float32, error) {
	profile, err := s.expectedProfile()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT c.slug,c.vector FROM chunks c JOIN pages p ON p.slug=c.slug AND p.revision=c.revision WHERE c.profile=? AND c.profile=COALESCE((SELECT value FROM meta WHERE key='embedding_profile'),'') ORDER BY c.slug,c.seq`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sums := map[string][]float64{}
	dimension := 0
	for rows.Next() {
		var slug string
		var blob []byte
		if err = rows.Scan(&slug, &blob); err != nil {
			return nil, err
		}
		vec, err := decodeVec(blob)
		if err != nil {
			return nil, err
		}
		if err = embed.ValidateVector(vec, dimension); err != nil {
			return nil, err
		}
		if dimension == 0 {
			dimension = len(vec)
		}
		sum := sums[slug]
		if sum == nil {
			sum = make([]float64, dimension)
			sums[slug] = sum
		}
		for i, f := range vec {
			sum[i] += float64(f)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := map[string][]float32{}
	for slug, sum := range sums {
		var norm float64
		for _, f := range sum {
			norm += f * f
		}
		norm = math.Sqrt(norm)
		if norm == 0 {
			continue
		}
		v := make([]float32, len(sum))
		for i, f := range sum {
			v[i] = float32(f / norm)
		}
		out[slug] = v
	}
	return out, nil
}

// Cosine rejects invalid or mismatched vectors with NaN, which fails ordinary
// threshold comparisons. Retrieval methods return an explicit validation error.
func Cosine(a, b []float32) float64 {
	if embed.ValidateVector(a, 0) != nil || embed.ValidateVector(b, len(a)) != nil {
		return math.NaN()
	}
	return dot(a, b)
}
func snippetOf(text string) string {
	var b strings.Builder
	n := 0
	for _, r := range text {
		if n == 120 {
			b.WriteRune('…')
			break
		}
		if r == '\n' || r == '\r' {
			r = ' '
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}
func decodeVec(b []byte) ([]float32, error) {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil, fmt.Errorf("bad vector blob length %d", len(b))
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v, nil
}
func dotBlob(q []float32, b []byte) (float64, error) {
	if len(b) != 4*len(q) {
		return 0, fmt.Errorf("cached dimension %d differs from query %d", len(b)/4, len(q))
	}
	var score, norm float64
	for i, f := range q {
		x := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:])))
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, fmt.Errorf("nonfinite cached vector")
		}
		score += float64(f) * x
		norm += x * x
	}
	if norm == 0 || math.Abs(math.Sqrt(norm)-1) > embed.NormTolerance {
		return 0, fmt.Errorf("non-unit cached vector")
	}
	return score, nil
}
func dot(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.NaN()
	}
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}
