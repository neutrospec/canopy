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
