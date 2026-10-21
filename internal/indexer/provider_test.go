package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
)

func TestExternalProfileReindex(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var in struct{ Input []string }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"data":[`)
		for i := range in.Input {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"index":%d,"embedding":[1,0]}`, i)
		}
		fmt.Fprint(w, `]}`)
	}))
	defer srv.Close()
	w, st, write := fixture(t)
	write("Current factual claim.")
	w.EmbeddingService = config.EmbeddingService{Provider: "openai", URL: srv.URL, Model: "bge-m3", Dimension: 2}
	run := func() {
		t.Helper()
		e, err := embed.NewFor(w)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		if _, err := Reindex(w, st, nil, e, nil); err != nil {
			t.Fatal(err)
		}
	}
	run()
	if calls != 1 {
		t.Fatal(calls)
	}
	w.EmbeddingService.Token = "rotated"
	run()
	if calls != 1 {
		t.Fatal("token caused inference")
	}
	// Same-dimensional model changes still invalidate, even with no engine/API.
	w.EmbeddingService.Model = "embeddinggemma-2"
	report, err := InspectCurrentContext(context.Background(), w, st, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.NeedsReindex != 1 || report.TotalChunks != 0 || calls != 1 {
		t.Fatalf("stale coverage: %+v calls=%d", report, calls)
	}
	if vs, err := st.PageVectors(); err != nil || len(vs) != 0 {
		t.Fatalf("stale suggestions: %v %v", vs, err)
	}
	run()
	if calls != 2 {
		t.Fatal("new model reused vectors")
	}
	report, err = InspectCurrentContext(context.Background(), w, st, nil, nil)
	if err != nil || report.NeedsReindex != 0 {
		t.Fatalf("new coverage: %+v %v", report, err)
	}
	w.EmbeddingService.Provider = "builtin"
	report, err = InspectCurrentContext(context.Background(), w, st, nil, nil)
	if err != nil || report.NeedsReindex != 1 {
		t.Fatalf("switch back: %+v %v", report, err)
	}
}
