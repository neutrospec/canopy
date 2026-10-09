package search_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/indexer"
	productsearch "github.com/neutrospec/canopy/internal/search"
	"github.com/neutrospec/canopy/internal/store"
)

type evalPage struct{ Slug, Title, Body string }
type evalQuery struct {
	ID              string         `json:"id"`
	Language        string         `json:"language"`
	Query           string         `json:"query"`
	Answerable      bool           `json:"answerable"`
	ExpectedPages   []string       `json:"expected_pages"`
	SupportingSpans []string       `json:"supporting_spans"`
	Relevance       map[string]int `json:"relevance"`
	Gate            string         `json:"gate"`
}
type evalCorpus struct {
	Pages   []evalPage
	Queries []evalQuery
}

func TestGroundedRetrievalEval(t *testing.T) {
	b, err := os.ReadFile("testdata/evaluation/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus evalCorpus
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(b)
	t.Logf("corpus_sha256=%s pages=%d queries=%d", hex.EncodeToString(digest[:]), len(corpus.Pages), len(corpus.Queries))
	root := t.TempDir()
	if err = os.Mkdir(filepath.Join(root, "concepts"), 0755); err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]evalPage{}
	for _, p := range corpus.Pages {
		bySlug[p.Slug] = p
		raw := "---\ntitle: " + p.Title + "\n---\n" + p.Body
		if err = os.WriteFile(filepath.Join(root, "concepts", p.Slug+".md"), []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
	}
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	st, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var eng embed.Engine
	if model := os.Getenv("CANOPY_TEST_MODEL"); model != "" {
		w.Cfg.Embedding.Model = model
		embed.SetModelDir(model)
		eng, err = embed.New()
		if err != nil {
			t.Fatalf("requested semantic model failed: %v", err)
		}
		defer eng.Close()
	} else {
		t.Log("semantic/hybrid=unverified (CANOPY_TEST_MODEL unset); keyword baseline evaluated")
	}
	res, err := indexer.EnsureCurrent(w, st, nil, eng, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("profile=%s covered=%d needs_reindex=%d", res.Profile, res.EmbeddedPages, res.NeedsReindex)
	for _, q := range corpus.Queries {
		t.Run(q.ID, func(t *testing.T) {
			kw, err := st.SearchKeyword(q.Query, 3)
			if err != nil {
				t.Fatal(err)
			}
			scoreEval(t, q, "keyword", kw, nil)
			if q.Gate == "literal" && !anyExpected(kw, q.ExpectedPages) {
				t.Fatalf("declared literal acceptance failed: %+v", kw)
			}
			if q.Gate == "unsupported" && len(kw) != 0 {
				t.Fatalf("declared unsupported keyword acceptance failed: %+v", kw)
			}
			if eng == nil {
				return
			}
			qv, err := embed.EmbedContext(context.Background(), eng, []string{q.Query})
			if err != nil {
				t.Fatal(err)
			}
			sem, err := st.SearchSemantic(qv[0], 3)
			if err != nil {
				t.Fatal(err)
			}
			chunks, err := st.SearchChunks(qv[0], 3, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range chunks {
				p := bySlug[c.Slug]
				if c.StartByte < 0 || c.EndByte > len(p.Body) || p.Body[c.StartByte:c.EndByte] != c.Text {
					t.Fatalf("raw-range acceptance failed: %+v", c)
				}
			}
			scoreEval(t, q, "semantic", sem, chunks)
			scoreEval(t, q, "hybrid", productsearch.Fuse(3, kw, sem), chunks)
		})
	}
}
func anyExpected(hits []store.Hit, expected []string) bool {
	for _, h := range hits {
		for _, e := range expected {
			if h.Slug == e {
				return true
			}
		}
	}
	return false
}
func scoreEval(t *testing.T, q evalQuery, mode string, hits []store.Hit, chunks []store.ChunkHit) {
	t.Helper()
	recall := 0.0
	matched := map[string]bool{}
	slugs := []string{}
	dcg := 0.0
	for i, h := range hits {
		slugs = append(slugs, h.Slug)
		matched[h.Slug] = true
		dcg += (math.Pow(2, float64(q.Relevance[h.Slug])) - 1) / math.Log2(float64(i+2))
	}
	for _, s := range q.ExpectedPages {
		if matched[s] {
			recall++
		}
	}
	if len(q.ExpectedPages) > 0 {
		recall /= float64(len(q.ExpectedPages))
	}
	labels := []int{}
	for _, r := range q.Relevance {
		labels = append(labels, r)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(labels)))
	ideal := 0.0
	for i, r := range labels {
		if i == 3 {
			break
		}
		ideal += (math.Pow(2, float64(r)) - 1) / math.Log2(float64(i+2))
	}
	ndcg := 0.0
	if ideal > 0 {
		ndcg = dcg / ideal
	}
	covered := 0
	for _, span := range q.SupportingSpans {
		for _, c := range chunks {
			if matched[c.Slug] && strings.Contains(c.Text, span) {
				covered++
				break
			}
		}
	}
	record := map[string]any{"id": q.ID, "language": q.Language, "mode": mode, "answerable": q.Answerable, "hits": slugs, "recall_at_3": recall, "ndcg_at_3": ndcg, "supporting_spans": len(q.SupportingSpans), "retrieved_supporting_spans": covered, "candidate_count": len(hits)}
	if mode == "keyword" {
		record["retrieved_supporting_spans"] = nil
		record["span_basis"] = "unmeasured"
	} else if mode == "hybrid" {
		record["span_basis"] = "semantic_top3_intersect_hybrid_pages"
	} else {
		record["span_basis"] = "semantic_top3"
	}
	b, _ := json.Marshal(record)
	t.Log(string(b))
}
