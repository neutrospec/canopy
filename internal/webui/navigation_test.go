package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/resurface"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Keep inference blocked until after every response has been checked. A fast
// fake engine would hide the cold-cache navigation regression.
func TestNavigationDoesNotWaitForEmbedding(t *testing.T) {
	s, _ := workspaceServer(t)
	engine := &webEvidenceEngine{started: make(chan struct{}), release: make(chan struct{})}
	s.eng = engine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(engine.release) }) }
	first := make(chan *httptest.ResponseRecorder, 1)
	firstConsumed := false
	go func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		r.Host = "localhost"
		r.Header.Set("Accept-Language", "en")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		first <- w
	}()
	defer func() {
		release()
		// Join the request even on the original implementation's failure path.
		if !firstConsumed {
			select {
			case <-first:
			case <-time.After(time.Second):
			}
		}
	}()
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("cold navigation did not start an index refresh")
	}
	select {
	case rec := <-first:
		firstConsumed = true
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Semantic suggestions are unavailable") {
			t.Fatalf("cold overview: status=%d", rec.Code)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Overview waited for native inference before returning HTML")
	}
	// Leaving the initiating page must not cancel the server's refresh.
	cancel()
	for _, path := range []string{"/special/discover", "/page/alpha", "/special/attention", "/"} {
		finished := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Host = "localhost"
			r.Header.Set("Accept-Language", "en")
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			finished <- w
		}()
		select {
		case rec := <-finished:
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Semantic suggestions are unavailable") {
				t.Fatalf("cold %s: status=%d", path, rec.Code)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("%s waited for native inference", path)
		}
	}
	if engine.calls.Load() != 1 {
		t.Fatalf("repeated navigation duplicated inference: %d", engine.calls.Load())
	}
	// Background indexing must not turn ordinary keyword search into a queue
	// timeout, and the API must describe the coverage it actually used.
	search := workspaceGet(t, s, "/api/search?q=body", "en")
	var out struct {
		Mode    string   `json:"mode"`
		Results []result `json:"results"`
		Index   struct {
			Ready bool `json:"semantic_ready"`
			Needs int  `json:"needs_reindex"`
		} `json:"index"`
	}
	if err := json.Unmarshal(search.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if search.Code != http.StatusOK || out.Mode != "keyword" || len(out.Results) != 2 || out.Index.Ready || out.Index.Needs != 2 {
		t.Fatalf("search during refresh: status=%d %+v", search.Code, out)
	}
	release()
	s.workWG.Wait()
	warm := workspaceGet(t, s, "/special/discover", "en")
	if warm.Code != http.StatusOK || strings.Contains(warm.Body.String(), "Semantic suggestions are unavailable") || engine.calls.Load() != 2 {
		t.Fatalf("refresh did not survive request cancellation: status=%d calls=%d", warm.Code, engine.calls.Load())
	}
	before, err := resurface.LoadState(s.w)
	if err != nil {
		t.Fatal(err)
	}
	workspaceGet(t, s, "/", "en")
	after, err := resurface.LoadState(s.w)
	if err != nil || s.daily.bridge == nil || len(after.ShownPairs) != 1 || len(before.Shown) != len(after.Shown) {
		t.Fatalf("warm overview did not fill only its bridge: %+v %v", after, err)
	}
	for slug, stamp := range before.Shown {
		if after.Shown[slug] != stamp {
			t.Fatal("warming the bridge consumed another reading card")
		}
	}
	stateBefore, _ := os.ReadFile(resurface.StatePath(s.w))
	workspaceGet(t, s, "/", "en")
	stateAfter, _ := os.ReadFile(resurface.StatePath(s.w))
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("reloading the warm overview consumed another exposure")
	}
}

func TestBackgroundSearchStaysKeywordWhenCorpusBecomesEmpty(t *testing.T) {
	s, w := workspaceServer(t)
	engine := &webEvidenceEngine{started: make(chan struct{}), release: make(chan struct{})}
	s.eng = engine
	defer close(engine.release)
	workspaceGet(t, s, "/", "en")
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("background inference did not start")
	}
	for _, slug := range []string{"alpha", "beta"} {
		if err := os.Remove(filepath.Join(w.Root, "concepts", slug+".md")); err != nil {
			t.Fatal(err)
		}
	}
	finished := make(chan searchOutcome, 1)
	errors := make(chan error, 1)
	go func() {
		out, err := s.runSearch(context.Background(), "body", 8)
		finished <- out
		errors <- err
	}()
	select {
	case out := <-finished:
		if err := <-errors; err != nil || out.Mode != "keyword" || len(out.Results) != 0 || out.Index == nil || !out.Index.SemanticReady || engine.calls.Load() != 1 {
			t.Fatalf("empty current corpus queued query inference: %+v %v calls=%d", out, err, engine.calls.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("keyword search waited on background native inference")
	}
}

func TestNavigationRejectsChangedCachedEvidence(t *testing.T) {
	s, w := workspaceServer(t)
	engine := &webEvidenceEngine{}
	s.eng = engine
	if _, err := s.runSearch(context.Background(), "body", 8); err != nil {
		t.Fatal(err)
	}
	engine.started, engine.release = make(chan struct{}), make(chan struct{})
	defer close(engine.release)
	putWebFile(t, filepath.Join(w.Root, "concepts/beta.md"), "---\ntitle: beta\ntype: concept\n---\nseparate current evidence")
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	vectors, coverage, err := s.currentVectors(context.Background(), scan)
	if err != nil || vectors != nil || coverage == nil || coverage.SemanticReady || coverage.NeedsReindex != 1 {
		t.Fatalf("changed source reused stale ranking: %v %+v %v", vectors, coverage, err)
	}
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("changed source was not scheduled for embedding")
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cached, err := st.PageVectors()
	if err != nil || len(cached) != 1 || cached["beta"] != nil {
		t.Fatalf("changed page's old vector survived inspection: %v %v", cached, err)
	}
}

func TestBackgroundCloseWaitsForEngineConsumers(t *testing.T) {
	for _, background := range []bool{true, false} {
		t.Run(map[bool]string{true: "background", false: "search"}[background], func(t *testing.T) {
			s, _ := workspaceServer(t)
			engine := &webEvidenceEngine{started: make(chan struct{}), release: make(chan struct{})}
			s.eng = engine
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(engine.release) }) }
			defer release()
			searchDone := make(chan error, 1)
			if background {
				workspaceGet(t, s, "/", "en")
			} else {
				go func() { _, err := s.runSearch(context.Background(), "body", 8); searchDone <- err }()
			}
			select {
			case <-engine.started:
			case <-time.After(time.Second):
				t.Fatal("inference did not start")
			}
			closed := make(chan struct{})
			go func() { s.Close(); close(closed) }()
			<-s.workCtx.Done()
			select {
			case <-closed:
				t.Fatal("server returned before native inference ended")
			case <-time.After(10 * time.Millisecond):
			}
			release()
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("server did not join inference after release")
			}
			if !background {
				if err := <-searchDone; err == nil {
					t.Fatal("search ignored server shutdown")
				}
			}
			if _, err := s.runSearch(context.Background(), "body", 8); err != context.Canceled || engine.calls.Load() != 1 {
				t.Fatalf("closed server started another inference: %v calls=%d", err, engine.calls.Load())
			}
		})
	}
}

func TestBackgroundRefreshSurvivesCanceledActiveSearch(t *testing.T) {
	s, _ := workspaceServer(t)
	engine := &webEvidenceEngine{started: make(chan struct{}), release: make(chan struct{})}
	s.eng = engine
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(engine.release) }) }
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	searchDone := make(chan error, 1)
	go func() { _, err := s.runSearch(ctx, "body", 8); searchDone <- err }()
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("search inference did not start")
	}
	if rec := workspaceGet(t, s, "/", "en"); rec.Code != http.StatusOK {
		t.Fatalf("navigation during search: %d", rec.Code)
	}
	cancel()
	release()
	select {
	case err := <-searchDone:
		if err == nil {
			t.Fatal("active search ignored cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled search did not finish")
	}
	s.workWG.Wait()
	if rec := workspaceGet(t, s, "/special/discover", "en"); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Semantic suggestions are unavailable") {
		t.Fatal("canceled search lost navigation's background refresh request")
	}
}
