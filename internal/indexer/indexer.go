// Package indexer rebuilds the derived index (page metadata, FTS,
// and — when an embedding engine is supplied — chunk vectors).
package indexer

import (
	"fmt"
	"strings"

	"github.com/nobocop/canopy/internal/config"
	"github.com/nobocop/canopy/internal/embed"
	"github.com/nobocop/canopy/internal/store"
	"github.com/nobocop/canopy/internal/wiki"
)

type Result struct {
	Pages       int `json:"pages"`
	Embedded    int `json:"embedded_pages"` // pages whose vectors were (re)computed
	Pruned      int `json:"pruned_pages"`   // deleted pages whose vectors were removed
	TotalChunks int `json:"total_chunks"`
}

// Reindex scans the wiki and refreshes the store. eng may be nil for a
// keyword-only refresh. progress (optional) receives human status lines.
func Reindex(w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine, progress func(string)) (*Result, error) {
	say := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	var err error
	if scan == nil {
		scan, err = wiki.Scan(w)
		if err != nil {
			return nil, err
		}
	}
	if err := st.RebuildPages(scan.Pages); err != nil {
		return nil, err
	}
	res := &Result{Pages: len(scan.Pages)}
	if eng == nil {
		return res, nil
	}

	stored, err := st.ChunkHashes()
	if err != nil {
		return nil, err
	}
	live := map[string]bool{}
	for _, p := range scan.Pages {
		live[p.Slug] = true
		title := p.Title
		if title == "" {
			title = p.Slug
		}
		chunks := embed.Split(title, p.Body)
		if len(chunks) == 0 {
			continue
		}
		var hashes []string
		for _, c := range chunks {
			hashes = append(hashes, c.Hash)
		}
		if stored[p.Slug] == strings.Join(hashes, ",") {
			continue // unchanged since last embedding run
		}
		texts := make([]string, len(chunks))
		seqs := make([]int, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
			seqs[i] = c.Seq
		}
		say(fmt.Sprintf("embedding %s (%d chunks)", p.RelPath, len(chunks)))
		vecs, err := eng.Embed(texts)
		if err != nil {
			return nil, fmt.Errorf("embed %s: %w", p.RelPath, err)
		}
		if err := st.ReplaceChunks(p.Slug, seqs, hashes, texts, vecs); err != nil {
			return nil, err
		}
		res.Embedded++
	}
	if res.Pruned, err = st.PruneChunks(live); err != nil {
		return nil, err
	}
	if res.TotalChunks, err = st.ChunkCount(); err != nil {
		return nil, err
	}
	return res, nil
}
