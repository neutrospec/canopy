// Package indexer refreshes derived keyword and revision/profile-bound vectors.
package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
)

type Result struct {
	Pages         int    `json:"pages"`
	Embedded      int    `json:"embedded_pages"`
	Pruned        int    `json:"pruned_pages"`
	TotalChunks   int    `json:"total_chunks"`
	EmbeddedPages int    `json:"covered_pages"`
	NeedsReindex  int    `json:"needs_reindex"`
	SemanticReady bool   `json:"semantic_ready"`
	Profile       string `json:"profile"`
}

var ephemeral atomic.Uint64

// Reindex is retained for existing CLI callers.
func Reindex(w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine, progress func(string)) (*Result, error) {
	return EnsureCurrent(w, st, scan, eng, progress)
}

// EnsureCurrent bootstraps cold keyword caches and invalidates stale vectors
// even when eng is nil. Cached-vector consumers must use this or
// InspectCurrentContext before searching or computing related candidates.
func EnsureCurrent(w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine, progress func(string)) (*Result, error) {
	return EnsureCurrentContext(context.Background(), w, st, scan, eng, progress)
}

func EnsureCurrentContext(ctx context.Context, w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine, progress func(string)) (*Result, error) {
	return refresh(ctx, w, st, scan, eng, progress, true)
}

// InspectCurrentContext validates the same source/profile boundary without
// running inference. Incomplete coverage is a normal result: callers can render
// non-semantic information while another job fills the missing embeddings.
func InspectCurrentContext(ctx context.Context, w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine) (*Result, error) {
	return refresh(ctx, w, st, scan, eng, nil, false)
}

func refresh(ctx context.Context, w *config.Wiki, st *store.Store, scan *wiki.ScanResult, eng embed.Engine, progress func(string), inferMissing bool) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	guard, err := wikilock.AcquireShared(ctx, w.Root)
	if err != nil {
		return nil, err
	}
	defer guard.Unlock()
	// A supplied snapshot may have waited behind a cooperating mutation.
	// Refresh under the shared lease and update its caller's view as well.
	fresh, err := wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	if scan == nil {
		scan = fresh
	} else {
		*scan = *fresh
	}
	profile := embed.ProfileIDFor(w.Cfg.Embedding.Model)
	if eng != nil {
		if e, ok := eng.(interface{ ProfileID() string }); ok && e.ProfileID() != "" {
			profile = e.ProfileID()
		} else {
			// Engines without an explicit identity cannot safely reuse vectors.
			profile = fmt.Sprintf("unidentified/%T/%d/%d", eng, os.Getpid(), ephemeral.Add(1))
		}
	}
	before, err := st.ChunkHashes()
	if err != nil {
		return nil, err
	}
	if err = st.RebuildPages(scan.Pages); err != nil {
		return nil, err
	}
	if err = st.SetProfile(profile); err != nil {
		return nil, err
	}
	if current, ok := eng.(interface{ ProfileCurrent() bool }); ok && !current.ProfileCurrent() {
		if err = st.SetProfile(embed.ProfileIDFor(w.Cfg.Embedding.Model)); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("embedding model files changed; reload engine and reindex")
	}
	stored, err := st.ChunkHashes()
	if err != nil {
		return nil, err
	}
	if err = guard.Unlock(); err != nil {
		return nil, err
	}
	res := &Result{Pages: len(scan.Pages), Profile: profile}
	for slug := range before {
		if _, ok := stored[slug]; !ok {
			res.Pruned++
		}
	}
	expected := 0
	for _, p := range scan.Pages {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if eng == nil || !inferMissing {
			continue
		}
		title := p.Title
		if title == "" {
			title = p.Slug
		}
		chunks := embed.Split(title, p.Body)
		if len(chunks) == 0 {
			continue
		}
		expected++
		hashes := make([]string, len(chunks))
		for i, c := range chunks {
			hashes[i] = c.Hash
		}
		if stored[p.Slug] == strings.Join(hashes, ",") {
			continue
		}
		inputs, texts := make([]string, len(chunks)), make([]string, len(chunks))
		seqs, starts, ends := make([]int, len(chunks)), make([]int, len(chunks)), make([]int, len(chunks))
		for i, c := range chunks {
			inputs[i], texts[i], seqs[i], starts[i], ends[i] = c.Input, c.Text, c.Seq, c.Start, c.End
		}
		if progress != nil {
			progress(fmt.Sprintf("embedding %s (%d chunks)", p.RelPath, len(chunks)))
		}
		vectors, err := embed.EmbedContext(ctx, eng, inputs)
		if err != nil {
			return nil, fmt.Errorf("embed %s: %w", p.RelPath, err)
		}
		revision := store.SourceRevision(p)
		err = func() error {
			publishGuard, err := wikilock.AcquireShared(ctx, w.Root)
			if err != nil {
				return err
			}
			defer publishGuard.Unlock()
			if current, ok := eng.(interface{ ProfileCurrent() bool }); ok && !current.ProfileCurrent() {
				if err = st.SetProfile(embed.ProfileIDFor(w.Cfg.Embedding.Model)); err != nil {
					return err
				}
				return fmt.Errorf("embedding model changed during inference; reload engine and reindex")
			}
			raw, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
			if err != nil {
				return fmt.Errorf("source changed while embedding %s: %w; retry reindex", p.RelPath, err)
			}
			if store.SourceRevision(wiki.Parse(p.RelPath, raw)) != revision {
				return fmt.Errorf("source changed while embedding %s; retry reindex", p.RelPath)
			}
			return st.ReplaceCurrentChunks(p.Slug, revision, profile, seqs, hashes, texts, starts, ends, vectors)
		}()
		if err != nil {
			return nil, err
		}
		res.Embedded++
	}
	// Recheck earlier pages after later batches: a cooperating write during
	// inference must not leave the initial FTS snapshot or earlier evidence.
	finalGuard, err := wikilock.AcquireShared(ctx, w.Root)
	if err != nil {
		return nil, err
	}
	defer finalGuard.Unlock()
	finalScan, err := wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	*scan = *finalScan
	if err = st.RebuildPages(finalScan.Pages); err != nil {
		return nil, err
	}
	if current, ok := eng.(interface{ ProfileCurrent() bool }); ok && !current.ProfileCurrent() {
		if err = st.SetProfile(embed.ProfileIDFor(w.Cfg.Embedding.Model)); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("model changed during refresh; reload engine and reindex")
	}
	if current, err := st.GetMeta("embedding_profile"); err != nil {
		return nil, err
	} else if current != profile {
		return nil, fmt.Errorf("embedding profile changed during refresh; retry reindex")
	}
	expected = 0
	for _, p := range finalScan.Pages {
		if strings.TrimSpace(p.Body) != "" {
			expected++
		}
	}
	res.Pages = len(finalScan.Pages)
	res.TotalChunks, err = st.ChunkCount()
	if err != nil {
		return nil, err
	}
	res.EmbeddedPages, err = st.EmbeddedPageCount()
	if err != nil {
		return nil, err
	}
	res.NeedsReindex = expected - res.EmbeddedPages
	res.SemanticReady = res.NeedsReindex == 0
	if inferMissing && eng != nil && !res.SemanticReady {
		return nil, fmt.Errorf("source changed during index refresh; retry reindex (%d pages need embedding)", res.NeedsReindex)
	}
	return res, nil
}
