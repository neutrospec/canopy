package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func TestExternalEmbeddingCommands(t *testing.T) {
	oldWiki, oldJSON := flagWiki, flagJSON
	t.Cleanup(func() { flagWiki, flagJSON = oldWiki, oldJSON })
	flagWiki = t.TempDir()
	flagJSON = true
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	os.MkdirAll(filepath.Join(flagWiki, "concepts"), 0755)
	os.WriteFile(filepath.Join(flagWiki, "concepts/a.md"), []byte("---\ntitle: Alpha\ntype: concept\n---\nAlpha evidence."), 0644)
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
	os.MkdirAll(config.ConfigHome(), 0700)
	writeCfg := func(endpoint string) {
		t.Helper()
		err := os.WriteFile(filepath.Join(config.ConfigHome(), "config.toml"), []byte(fmt.Sprintf("[embedding]\nprovider='openai'\nmodel='embeddinggemma-2'\ndimension=2\nurl=%q\n", endpoint)), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}
	writeCfg(srv.URL)
	c := cmdReindex()
	c.SetArgs([]string{"--check"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("check called API")
	}
	c = cmdReindex()
	c.SetArgs(nil)
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("reindex not using remote", calls)
	}
	c = cmdSearch()
	c.SetArgs([]string{"Alpha", "--mode", "semantic"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("query missing or documents re-embedded", calls)
	}
	// A blank external URL must not open the native model, even in an ORT build.
	writeCfg("")
	w, err := loadWiki()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newEngine(w); err == nil {
		t.Fatal("blank URL accepted")
	}
	c = cmdSearch()
	c.SetArgs([]string{"Alpha", "--mode", "hybrid"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("invalid setup called API")
	}
}
