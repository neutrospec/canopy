package store

import (
	"path/filepath"
	"testing"

	"github.com/nobocop/canopy/internal/config"
	"github.com/nobocop/canopy/internal/wiki"
)

func TestKeywordSearch(t *testing.T) {
	root, err := filepath.Abs("../../testdata/fixture-wiki")
	if err != nil {
		t.Fatal(err)
	}
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.RebuildPages(scan.Pages); err != nil {
		t.Fatal(err)
	}

	// Korean query with a particle-free prefix should match body text.
	hits, err := st.SearchKeyword("산업용 프로토콜", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits for Korean query")
	}
	if hits[0].Slug != "industrial-protocols" {
		t.Errorf("top hit = %s, want industrial-protocols (all: %+v)", hits[0].Slug, hits)
	}

	// English query.
	hits, err = st.SearchKeyword("MQTT", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits for MQTT")
	}
}

func TestSearchChunks(t *testing.T) {
	root, _ := filepath.Abs("../../testdata/fixture-wiki")
	w := &config.Wiki{Root: root, Cfg: config.Default()}
	scan, err := wiki.Scan(w)
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.RebuildPages(scan.Pages); err != nil {
		t.Fatal(err)
	}
	// Synthetic unit vectors: opc-ua has two chunks near the query axis.
	if err := st.ReplaceChunks("opc-ua", []int{0, 1}, []string{"h0", "h1"},
		[]string{"산업용 통신 프로토콜 본문", "보안 모델 본문"},
		[][]float32{{1, 0}, {0.9, 0.436}}); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceChunks("industrial-protocols", []int{0}, []string{"h2"},
		[]string{"프로토콜 개요 본문"}, [][]float32{{0.8, 0.6}}); err != nil {
		t.Fatal(err)
	}

	chunks, err := st.SearchChunks([]float32{1, 0}, 3, 1) // per-page cap 1
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("per-page cap ignored: %+v", chunks)
	}
	if chunks[0].Slug != "opc-ua" || chunks[0].Text != "산업용 통신 프로토콜 본문" {
		t.Errorf("top chunk wrong: %+v", chunks[0])
	}
	// G3: scores descend.
	if chunks[0].Score < chunks[1].Score {
		t.Error("scores not descending")
	}
}
