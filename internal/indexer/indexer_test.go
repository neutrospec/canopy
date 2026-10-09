package indexer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
)

type testEngine struct {
	profile string
	calls   int
	fail    bool
	run     func()
}

func (e *testEngine) ProfileID() string { return e.profile }
func (e *testEngine) Embed(texts []string) ([][]float32, error) {
	e.calls++
	if e.run != nil {
		e.run()
	}
	if e.fail {
		return nil, errors.New("injected inference failure")
	}
	vs := make([][]float32, len(texts))
	for i := range vs {
		vs[i] = []float32{1, 0}
	}
	return vs, nil
}
func (*testEngine) Close() error { return nil }

func fixture(t *testing.T) (*config.Wiki, *store.Store, func(string)) {
	t.Helper()
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.MkdirAll(filepath.Join(w.Root, "concepts"), 0755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(w.Root, "concepts/a.md"), []byte("---\ntitle: Alpha\n---\n"+body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return w, st, write
}

func TestReindexClearsEmptyBody(t *testing.T) {
	for _, body := range []string{"", "  \n\n\t"} {
		t.Run(body, func(t *testing.T) {
			w, st, write := fixture(t)
			e := &testEngine{profile: "test-a"}
			write("Old factual claim.")
			if _, err := Reindex(w, st, nil, e, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := Reindex(w, st, nil, e, nil); err != nil || e.calls != 1 {
				t.Fatalf("unchanged control calls=%d err=%v", e.calls, err)
			}
			write(body)
			if _, err := Reindex(w, st, nil, e, nil); err != nil {
				t.Fatal(err)
			}
			if n, _ := st.ChunkCount(); n != 0 {
				t.Fatalf("blank page still has %d chunks", n)
			}
		})
	}
}

func TestDegradedIndexDoesNotServeStaleChunks(t *testing.T) {
	w, st, write := fixture(t)
	e := &testEngine{profile: "test-a"}
	write("Old factual claim.")
	if _, err := Reindex(w, st, nil, e, nil); err != nil {
		t.Fatal(err)
	}
	write("Current corrected claim.")
	if _, err := Reindex(w, st, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if hits, err := st.SearchChunks([]float32{1, 0}, 5, 1); err != nil || len(hits) != 0 {
		t.Fatalf("stale evidence: %+v %v", hits, err)
	}
	if _, err := Reindex(w, st, nil, e, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(w.Root, "concepts/a.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Reindex(w, st, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if vs, err := st.PageVectors(); err != nil || len(vs) != 0 {
		t.Fatalf("deleted vectors: %+v %v", vs, err)
	}
}

func TestReindexEmbeddingProfile(t *testing.T) {
	w, st, write := fixture(t)
	write("Stable text.")
	a := &testEngine{profile: "model-a/tokenizer-a/mean"}
	b := &testEngine{profile: "model-b/tokenizer-a/mean"}
	if _, err := Reindex(w, st, nil, a, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Reindex(w, st, nil, b, nil); err != nil {
		t.Fatal(err)
	}
	if b.calls != 1 {
		t.Fatalf("same-dimensional profile change reused old vectors: calls=%d", b.calls)
	}
	b.profile = "model-b/tokenizer-b/mean"
	if _, err := Reindex(w, st, nil, b, nil); err != nil || b.calls != 2 {
		t.Fatalf("tokenizer change: calls=%d err=%v", b.calls, err)
	}
}

func TestRecallSourceRoundTrip(t *testing.T) {
	w, st, write := fixture(t)
	body := "첫 문단 — 원문의 공백과 바이트 범위를 보존한다.  \n\n    indented code\n\n" +
		strings.Repeat("두 번째 문단. Raw evidence keeps Unicode and spacing.  \n\n", 80) +
		"\n\nLast paragraph.\n"
	write(body)
	if _, err := Reindex(w, st, nil, &testEngine{profile: "test"}, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, "concepts/a.md"))
	if err != nil {
		t.Fatal(err)
	}
	page := wiki.Parse("concepts/a.md", raw)
	revision := store.SourceRevision(page)
	hits, err := st.SearchChunks([]float32{1, 0}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 {
		t.Fatalf("multi-chunk fixture returned only %d evidence ranges", len(hits))
	}
	for _, h := range hits {
		if h.RelPath != page.RelPath || h.Slug != page.Slug || h.SourceRevision != revision || h.Profile != "test" {
			t.Fatalf("evidence provenance does not identify the current page/profile: %+v", h)
		}
		if h.StartByte < 0 || h.EndByte <= h.StartByte || h.EndByte > len(page.Body) {
			t.Fatalf("invalid nonempty source range [%d:%d] for %d bytes", h.StartByte, h.EndByte, len(page.Body))
		}
		if want := page.Body[h.StartByte:h.EndByte]; h.Text != want {
			t.Fatalf("evidence differs from its exact source range [%d:%d]: got %q, want %q", h.StartByte, h.EndByte, h.Text, want)
		}
	}
}

func TestFailedInferenceExcludesPreviousRevision(t *testing.T) {
	w, st, write := fixture(t)
	e := &testEngine{profile: "test"}
	write("Previous claim.")
	if _, err := Reindex(w, st, nil, e, nil); err != nil {
		t.Fatal(err)
	}
	write("New claim.")
	e.fail = true
	if _, err := Reindex(w, st, nil, e, nil); err == nil {
		t.Fatal("injected failure succeeded")
	}
	if hits, err := st.SearchChunks([]float32{1, 0}, 5, 1); err != nil || len(hits) != 0 {
		t.Fatalf("failed inference exposed stale evidence: %+v %v", hits, err)
	}
}

func TestInferenceSourceChangeIsExcluded(t *testing.T) {
	w, st, write := fixture(t)
	write("Observed claim.")
	e := &testEngine{profile: "test", run: func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		lease, err := wikilock.Acquire(ctx, w.Root)
		if err != nil {
			t.Fatalf("inference retained a wiki lease: %v", err)
		}
		write("Changed during inference.")
		if err = lease.Unlock(); err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := Reindex(w, st, nil, e, nil); err == nil || !strings.Contains(err.Error(), "source changed") {
		t.Fatalf("changed snapshot published: %v", err)
	}
	if n, _ := st.ChunkCount(); n != 0 {
		t.Fatalf("changed snapshot left %d chunks", n)
	}
	e.run = nil
	if _, err := Reindex(w, st, nil, e, nil); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}

func TestPassedSnapshotRescannedBeforePublication(t *testing.T) {
	w, st, write := fixture(t)
	write("oldneedle original")
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	write("newneedle current")
	if _, err = EnsureCurrent(w, st, scan, nil, nil); err != nil {
		t.Fatal(err)
	}
	if hits, err := st.SearchKeyword("oldneedle", 5); err != nil || len(hits) != 0 {
		t.Fatalf("stale passed snapshot published: %+v %v", hits, err)
	}
	if hits, err := st.SearchKeyword("newneedle", 5); err != nil || len(hits) != 1 {
		t.Fatalf("fresh snapshot unavailable: %+v %v", hits, err)
	}
	if !strings.Contains(scan.Pages[0].Body, "newneedle") {
		t.Fatal("caller snapshot was not refreshed")
	}
}
