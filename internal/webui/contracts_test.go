package webui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/resurface"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/tasks"
)

func putWebFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHomeHEADDoesNotConsumeDailyExposure(t *testing.T) {
	s, w := workspaceServer(t)
	putWebFile(t, filepath.Join(w.Root, "concepts/alpha.md"), "---\ntitle: alpha\ntype: concept\nupdated: 2000-01-01\n---\nOld page to re-encounter.\n")
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD: %d %s", rec.Code, rec.Body)
	}
	if s.daily.date != "" {
		t.Fatal("HEAD populated the daily exposure cache")
	}
	if _, err := os.Stat(resurface.StatePath(w)); !os.IsNotExist(err) {
		t.Fatalf("HEAD changed exposure history: %v", err)
	}
	if got := workspaceGet(t, s, "/", "en"); got.Code != http.StatusOK {
		t.Fatalf("GET: %d", got.Code)
	}
	before, err := os.ReadFile(resurface.StatePath(w))
	if err != nil {
		t.Fatal(err)
	}
	state, err := resurface.LoadState(w)
	if err != nil || len(state.Shown) != 1 {
		t.Fatalf("GET must consume one card: %+v %v", state, err)
	}
	workspaceGet(t, s, "/", "en")
	after, _ := os.ReadFile(resurface.StatePath(w))
	if !bytes.Equal(before, after) {
		t.Fatal("same-day reload consumed another card")
	}
}

func TestHomeUnreadWithoutRecommendation(t *testing.T) {
	s, w := workspaceServer(t)
	for _, name := range []string{"alpha", "beta"} {
		putWebFile(t, filepath.Join(w.Root, "concepts", name+".md"), "---\ntitle: "+name+"\ntype: concept\ncreated: 2000-01-01\nupdated: 2000-01-01\n---\nIsolated old page.\n")
	}
	for _, path := range []string{"/", "/special/discover"} {
		rec := workspaceGet(t, s, path, "en")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Unread pages remain") || strings.Contains(rec.Body.String(), "You've read everything") {
			t.Fatalf("false all-read on %s: %d %s", path, rec.Code, rec.Body)
		}
	}
	assertSummary(t, s, "2", "0", "2", "0")
}

func TestPartialReadsKeepHealthyAggregateAndRefuseWrites(t *testing.T) {
	for _, corrupt := range []string{"human", "agent"} {
		t.Run(corrupt, func(t *testing.T) {
			s, w := workspaceServer(t)
			rs, _ := reads.Load(w)
			rs.Mark("alpha", "explicit", time.Now())
			rs.MarkAgent("beta", time.Now())
			if err := rs.Save(w); err != nil {
				t.Fatal(err)
			}
			path := reads.Path(w)
			if corrupt == "agent" {
				path = reads.AgentPath(w)
			}
			putWebFile(t, path, "{broken aggregate")
			human, _ := os.ReadFile(reads.Path(w))
			agent, _ := os.ReadFile(reads.AgentPath(w))
			home := workspaceGet(t, s, "/", "en")
			page := workspaceGet(t, s, "/page/beta", "en")
			graph := workspaceGet(t, s, "/api/graph", "en")
			if home.Code != 200 || page.Code != 200 || graph.Code != 200 {
				t.Fatalf("healthy page unavailable: %d/%d/%d", home.Code, page.Code, graph.Code)
			}
			var data struct {
				Nodes   []graphAPINode `json:"nodes"`
				Partial bool           `json:"reads_partial"`
				Human   bool           `json:"human_available"`
				Agent   bool           `json:"agent_available"`
			}
			if err := json.Unmarshal(graph.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if !data.Partial || data.Human != (corrupt == "agent") || data.Agent != (corrupt == "human") {
				t.Fatalf("availability lost: %+v", data)
			}
			for _, node := range data.Nodes {
				if corrupt == "human" && node.Read != nil {
					t.Fatal("unknown human state became unread")
				}
				if corrupt == "human" && node.ID == "beta" && node.Touches != 1 {
					t.Fatal("healthy agent history disappeared")
				}
				if corrupt == "agent" && node.ID == "alpha" && (node.Read == nil || !*node.Read || node.Touches != 1) {
					t.Fatal("healthy human history disappeared")
				}
			}
			if corrupt == "human" && strings.Contains(page.Body.String(), "data-read-secs=") {
				t.Fatal("auto-read enabled against unavailable human state")
			}
			if got := postForm(t, s.Handler(), "/read/beta", url.Values{"action": {"read"}}); got.Code != 500 {
				t.Fatalf("corrupt aggregate mutation status %d", got.Code)
			}
			humanAfter, _ := os.ReadFile(reads.Path(w))
			agentAfter, _ := os.ReadFile(reads.AgentPath(w))
			if !bytes.Equal(human, humanAfter) || !bytes.Equal(agent, agentAfter) {
				t.Fatal("failed mark changed aggregate bytes")
			}
		})
	}
}

func TestUnknownReadFeedbackRejectedAndHistoricalUndoRetained(t *testing.T) {
	s, w := workspaceServer(t)
	rs, _ := reads.Load(w)
	rs.Mark("old-deleted-page", "explicit", time.Now())
	if err := rs.Save(w); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(reads.Path(w))
	for _, tc := range []struct {
		path string
		form url.Values
	}{
		{"/read/does-not-exist", url.Values{"action": {"read"}}},
		{"/api/read/does-not-exist", url.Values{}},
		{"/resurface/feedback", url.Values{"slug": {"does-not-exist"}, "action": {"up"}}},
	} {
		if rec := postForm(t, s.Handler(), tc.path, tc.form); rec.Code != 404 {
			t.Fatalf("%s status %d", tc.path, rec.Code)
		}
	}
	after, _ := os.ReadFile(reads.Path(w))
	if !bytes.Equal(before, after) {
		t.Fatal("rejected input changed read history")
	}
	if _, err := os.Stat(resurface.StatePath(w)); !os.IsNotExist(err) {
		t.Fatal("rejected feedback created history")
	}
	if rec := postForm(t, s.Handler(), "/read/old-deleted-page", url.Values{"action": {"unread"}}); rec.Code != 303 {
		t.Fatalf("historical undo: %d", rec.Code)
	}
	rs, err := reads.Load(w)
	if err != nil || rs.IsRead("old-deleted-page") {
		t.Fatalf("historical undo failed: %v", err)
	}
}

func TestProposalReceiptIsPOSTSnapshotAndNotApplication(t *testing.T) {
	s, w := workspaceServer(t)
	pagePath := filepath.Join(w.Root, "concepts/alpha.md")
	workspaceGet(t, s, "/edit/alpha", "en")
	current := "---\ntitle: alpha\ntype: concept\n---\nA change after opening the editor.\n"
	putWebFile(t, pagePath, current)
	proposal := "# Proposal\n\nUnique <literal> & quoted content.\n"
	rec := postForm(t, s.Handler(), "/edit/alpha", url.Values{"body": {strings.ReplaceAll(proposal, "\n", "\r\n")}, "note": {"Keep this request intact."}})
	if rec.Code != 303 {
		t.Fatalf("proposal: %d %s", rec.Code, rec.Body)
	}
	list, err := tasks.Load(w)
	if err != nil || len(list) != 1 {
		t.Fatalf("queue: %v %v", list, err)
	}
	task := list[0]
	if task.Body != proposal || task.Request != "Keep this request intact." || task.Base != contentHash([]byte(current)) || task.Status != tasks.StatusPending {
		t.Fatalf("receipt/payload mismatch: %+v", task)
	}
	stored, _ := os.ReadFile(pagePath)
	if string(stored) != current {
		t.Fatal("proposal applied before review")
	}
	if !strings.Contains(rec.Header().Get("Location"), "tasked="+task.ID) {
		t.Fatal("receipt does not identify task")
	}
	page := workspaceGet(t, s, rec.Header().Get("Location"), "en")
	queue := workspaceGet(t, s, "/special/tasks", "en")
	if !strings.Contains(page.Body.String(), "Request received.") || !strings.Contains(queue.Body.String(), task.Base) || !strings.Contains(html.UnescapeString(queue.Body.String()), task.Created) || !strings.Contains(queue.Body.String(), html.EscapeString(proposal)) {
		t.Fatalf("receipt screen: page status=%d queue status=%d notice=%t base=%t time=%t body=%t", page.Code, queue.Code, strings.Contains(page.Body.String(), "Request received."), strings.Contains(queue.Body.String(), task.Base), strings.Contains(html.UnescapeString(queue.Body.String()), task.Created), strings.Contains(queue.Body.String(), html.EscapeString(proposal)))
	}
	for _, value := range []string{"1", "nonexistent-task"} {
		if strings.Contains(workspaceGet(t, s, "/page/alpha?tasked="+value, "en").Body.String(), "Request received.") {
			t.Fatal("unverified query parameter claimed a receipt")
		}
	}
	if got := postForm(t, s.Handler(), "/edit/alpha", url.Values{"body": {"  "}, "note": {" "}}); got.Code != 400 {
		t.Fatalf("empty proposal status %d", got.Code)
	}
	if got := postForm(t, s.Handler(), "/edit/alpha", url.Values{"note": {"Request only."}}); got.Code != 303 {
		t.Fatalf("request-only proposal status %d", got.Code)
	}
	list, err = tasks.Load(w)
	if err != nil || len(list) != 2 {
		t.Fatalf("request-only queue: %v %v", list, err)
	}
}

type webEvidenceEngine struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (e *webEvidenceEngine) ProfileID() string { return "web-evidence-fixture" }
func (e *webEvidenceEngine) Close() error      { return nil }
func (e *webEvidenceEngine) Embed(texts []string) ([][]float32, error) {
	e.calls.Add(1)
	if e.started != nil {
		e.once.Do(func() { close(e.started); <-e.release })
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		if strings.Contains(strings.ToLower(text), "separate") {
			out[i] = []float32{0, 1}
		} else {
			out[i] = []float32{1, 0}
		}
	}
	return out, nil
}

func TestColdSearchAndEveryVectorConsumerRefreshCurrentEvidence(t *testing.T) {
	s, w := workspaceServer(t)
	cold := workspaceGet(t, s, "/api/search?q=body", "en")
	var response struct {
		Mode    string   `json:"mode"`
		Results []result `json:"results"`
		Index   struct {
			Needs int `json:"needs_reindex"`
		} `json:"index"`
	}
	if err := json.Unmarshal(cold.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if cold.Code != 200 || response.Mode != "keyword" || len(response.Results) != 2 || response.Index.Needs != 2 {
		t.Fatalf("cold keyword search dishonest: %+v", response)
	}
	engine := &webEvidenceEngine{}
	s.eng = engine
	if _, err := s.runSearch(context.Background(), "body", 8); err != nil {
		t.Fatal(err)
	}
	for i, path := range []string{"/api/search?q=body", "/search?q=alpha", "/page/alpha", "/", "/special/discover", "/special/attention"} {
		putWebFile(t, filepath.Join(w.Root, "concepts/beta.md"), "---\ntitle: beta\ntype: concept\n---\nseparate evidence revision "+strings.Repeat("x", i+1))
		before := engine.calls.Load()
		rec := workspaceGet(t, s, path, "en")
		if rec.Code != 200 && rec.Code != 302 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		// HTML validates immediately, then refreshes missing vectors in the
		// background. Search may refresh synchronously when the slot is idle.
		s.workWG.Wait()
		if engine.calls.Load() <= before {
			t.Fatalf("%s consumed an unrefreshed cache", path)
		}
		st, err := store.Open(w.DBPath())
		if err != nil {
			t.Fatal(err)
		}
		vectors, err := st.PageVectors()
		st.Close()
		if err != nil || len(vectors) != 2 || store.Cosine(vectors["alpha"], vectors["beta"]) != 0 {
			t.Fatalf("stale vector after %s: %v %v", path, vectors, err)
		}
	}
	// No-model refresh must remove deleted evidence and return keyword mode.
	s.eng = nil
	if err := os.Remove(filepath.Join(w.Root, "concepts/beta.md")); err != nil {
		t.Fatal(err)
	}
	out, err := s.runSearch(context.Background(), "beta", 8)
	if err != nil || out.Mode != "keyword" || len(out.Results) != 0 {
		t.Fatalf("deleted evidence leaked: %+v %v", out, err)
	}
}

func TestQueuedCanceledSearchNeverStartsInference(t *testing.T) {
	s, w := workspaceServer(t)
	engine := &webEvidenceEngine{started: make(chan struct{}), release: make(chan struct{})}
	s.eng = engine
	finished := make(chan error, 1)
	go func() { _, err := s.runSearch(context.Background(), "body", 8); finished <- err }()
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("first inference did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := s.runSearch(ctx, "canceled", 8)
	if !errors.Is(err, context.DeadlineExceeded) || engine.calls.Load() != 1 {
		t.Fatalf("queued cancellation started native work: %v calls=%d", err, engine.calls.Load())
	}
	// Independent read state writes are not held behind the engine gate.
	if rec := postForm(t, s.Handler(), "/read/alpha", url.Values{"action": {"read"}}); rec.Code != 303 {
		t.Fatalf("read during inference: %d", rec.Code)
	}
	rs, _ := reads.Load(w)
	if !rs.IsRead("alpha") {
		t.Fatal("read update did not persist")
	}
	close(engine.release)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("released inference did not finish")
	}
	if engine.calls.Load() != 3 {
		t.Fatalf("canceled request ran later: calls=%d", engine.calls.Load())
	}
}

func TestMarkdownCodeBoundariesPreserveLiteralSource(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"four-backticks", "````text\n[[inside]]\n```\n````\n", "[[inside]]\n```\n"},
		{"tilde", "~~~text\n[[inside]] <literal> &\n~~~\n", "[[inside]] <literal> &\n"},
		{"indented-fence", "   ```text\n   [[inside]]\n   ```\n", "[[inside]]\n"},
		{"blockquote", "> ```text\n> [[inside]]\n> ```\n", "[[inside]]\n"},
		{"list-container", "- item\n\n  ```text\n  [[inside]]\n  ```\n", "[[inside]]\n"},
		{"indented-code", "    [[inside]]\n    <literal> &\n", "[[inside]]\n<literal> &\n"},
	}
	pre := regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
	tags := regexp.MustCompile(`<[^>]*>`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.body + "\nOutside [[known-page]].\n"
			saved := source
			got, err := RenderPage(source, exists)
			if err != nil {
				t.Fatal(err)
			}
			matches := pre.FindStringSubmatch(string(got))
			if len(matches) != 2 {
				t.Fatalf("code block missing: %s", got)
			}
			literal := html.UnescapeString(tags.ReplaceAllString(matches[1], ""))
			if literal != tc.want || strings.Count(string(got), `class="wikilink`) != 1 || source != saved {
				t.Fatalf("source/link boundary changed: %q want %q; %s", literal, tc.want, got)
			}
		})
	}
	for _, source := range []string{"`[[inside]]`", "``[[inside]] ` literal``", "`line\n[[inside]]`"} {
		got, err := RenderPage(source+" Outside [[known-page]].", exists)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "[[inside]]") || strings.Count(string(got), `class="wikilink`) != 1 {
			t.Fatalf("inline boundary: %s", got)
		}
	}
}

func TestPinnedMathAssetsAreServedLocally(t *testing.T) {
	s, _ := workspaceServer(t)
	for _, name := range []string{"tex-svg.js", "mathjax-color.js", "mathjax-physics.js", "LICENSE-mathjax.txt", "NOTICE-mathjax.txt", "LICENSE-force-graph.txt", "NOTICE-force-graph.txt", "assets.json"} {
		rec := workspaceGet(t, s, "/static/vendor/"+name, "en")
		expected, err := assets.ReadFile("static/vendor/" + name)
		if err != nil || rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), expected) {
			t.Fatalf("asset route %s: %d %v", name, rec.Code, err)
		}
	}
}

func TestWebAddressingDoesNotConfuseDottedSlugsOrPaths(t *testing.T) {
	s, w := workspaceServer(t)
	short := "---\ntitle: short\ntype: concept\n---\nSHORT-CANARY\n"
	long := "---\ntitle: long\ntype: concept\n---\nLONG-CANARY [[concepts/foo.md.md|explicit]] [[foo.md|ambiguous]]\n"
	putWebFile(t, filepath.Join(w.Root, "concepts/foo.md"), short)
	putWebFile(t, filepath.Join(w.Root, "concepts/foo.md.md"), long)
	putWebFile(t, filepath.Join(w.Root, "concepts/spaced name.md"), "---\ntitle: spaced\ntype: concept\n---\nSPACE-CANARY\n")
	for _, path := range []string{"/page/concepts%2Ffoo.md.md", "/page/concepts%2Fspaced%20name.md"} {
		rec := workspaceGet(t, s, path, "en")
		if rec.Code != 200 {
			t.Fatalf("explicit path %s: %d", path, rec.Code)
		}
	}
	page := workspaceGet(t, s, "/page/concepts%2Ffoo.md.md", "en")
	if !strings.Contains(page.Body.String(), "LONG-CANARY") || strings.Contains(page.Body.String(), "SHORT-CANARY") || !strings.Contains(page.Body.String(), `href="/page/concepts%2Ffoo.md.md"`) || !strings.Contains(page.Body.String(), `action="/read/concepts%2Ffoo.md.md"`) {
		t.Fatal("explicit dotted path was remapped")
	}
	if rec := postForm(t, s.Handler(), "/edit/foo.md", url.Values{"note": {"ambiguous must refuse"}}); rec.Code != 404 {
		t.Fatalf("ambiguous edit: %d", rec.Code)
	}
	if rec := postForm(t, s.Handler(), "/api/read/concepts%2Ffoo.md.md", url.Values{}); rec.Code != 200 {
		t.Fatalf("dotted read: %d", rec.Code)
	}
	rs, err := reads.Load(w)
	if err != nil || !rs.IsRead("foo.md") || rs.IsRead("foo") {
		t.Fatalf("read silently targeted another slug: %v", err)
	}
	if rec := postForm(t, s.Handler(), "/edit/concepts%2Ffoo.md.md", url.Values{"note": {"explicit request"}}); rec.Code != 303 {
		t.Fatalf("explicit edit: %d", rec.Code)
	}
	list, err := tasks.Load(w)
	if err != nil || len(list) != 1 || len(list[0].Pages) != 1 || list[0].Pages[0] != "foo.md" {
		t.Fatalf("proposal target mismatch: %+v %v", list, err)
	}
	after, _ := os.ReadFile(filepath.Join(w.Root, "concepts/foo.md"))
	if string(after) != short {
		t.Fatal("non-target source changed")
	}
}

func TestGraphHasServerRenderedKeyboardLinks(t *testing.T) {
	s, w := workspaceServer(t)
	putWebFile(t, filepath.Join(w.Root, "concepts/alpha.md"), "---\ntitle: alpha\ntype: concept\n---\n[[beta]]\n")
	rec := workspaceGet(t, s, "/graph", "en")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `class="graphneighbors"`) || !strings.Contains(rec.Body.String(), `href="/page/alpha"`) || !strings.Contains(rec.Body.String(), `href="/page/beta"`) {
		t.Fatal("graph's ordinary link path is absent")
	}
}

func TestApplicationAssetURLsMatchBytesAndRevalidate(t *testing.T) {
	s, _ := workspaceServer(t)
	paths := map[string]bool{}
	refs := regexp.MustCompile(`(?:href|src)="(/static/(?:style\.css|chroma\.css|app\.js|graph\.js)\?v=[a-f0-9]{12})"`)
	for _, page := range []string{"/", "/graph"} {
		for _, match := range refs.FindAllStringSubmatch(workspaceGet(t, s, page, "en").Body.String(), -1) {
			paths[html.UnescapeString(match[1])] = true
		}
	}
	if len(paths) != 4 {
		t.Fatalf("application URLs are not content-versioned: %v", paths)
	}
	for path := range paths {
		u, err := url.Parse(path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := assets.ReadFile(strings.TrimPrefix(u.Path, "/"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		full := hex.EncodeToString(sum[:])
		if u.Query().Get("v") != full[:12] {
			t.Fatalf("wrong asset hash for %s", path)
		}
		rec := workspaceGet(t, s, path, "en")
		if rec.Code != 200 || rec.Header().Get("ETag") != `"`+full+`"` || rec.Header().Get("Cache-Control") != "no-cache" || !bytes.Equal(rec.Body.Bytes(), data) {
			t.Fatalf("asset response does not describe exact bytes: %s", path)
		}
		req := httptest.NewRequest("GET", path, nil)
		req.Host = "localhost"
		req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
		cached := httptest.NewRecorder()
		s.Handler().ServeHTTP(cached, req)
		if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 || cached.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("conditional request %s: %d", path, cached.Code)
		}
	}
}
