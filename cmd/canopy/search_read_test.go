package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/store"
)

func TestSearchRecallGrepBoundsBeforeWiki(t *testing.T) {
	oldWiki, oldJSON := flagWiki, flagJSON
	t.Cleanup(func() { flagWiki, flagJSON = oldWiki, oldJSON })
	flagWiki = filepath.Join(t.TempDir(), "missing-wiki")
	flagJSON = true
	cases := []struct {
		name   string
		newCmd func() *cobra.Command
		args   []string
		want   string
	}{
		{"search-negative", cmdSearch, []string{"needle", "-k=-1"}, "k must"},
		{"search-zero", cmdSearch, []string{"needle", "-k=0"}, "k must"},
		{"search-large", cmdSearch, []string{"needle", "-k=1001"}, "k must"},
		{"recall-negative", cmdRecall, []string{"needle", "-k=-1"}, "k must"},
		{"recall-zero", cmdRecall, []string{"needle", "-k=0"}, "k must"},
		{"recall-cap-negative", cmdRecall, []string{"needle", "--per-page=-1"}, "per-page must"},
		{"grep-negative", cmdGrep, []string{"needle", "-n=-1"}, "max must"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.newCmd()
			c.SilenceUsage = true
			c.SilenceErrors = true
			c.SetArgs(tc.args)
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validation did not precede wiki/model/cache work: %v", err)
			}
		})
	}
	if _, err := os.Stat(flagWiki); !os.IsNotExist(err) {
		t.Fatal("invalid counts created wiki")
	}
}

type reconcileEngine struct{ profile string }

func (e reconcileEngine) ProfileID() string { return e.profile }
func (e reconcileEngine) Close() error      { return nil }
func (e reconcileEngine) Embed(texts []string) ([][]float32, error) {
	vs := make([][]float32, len(texts))
	for i := range vs {
		vs[i] = []float32{1, 0}
	}
	return vs, nil
}
func TestReconcileExcludesDeletedAndEditedEvidence(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
	if err := os.Mkdir(filepath.Join(w.Root, "concepts"), 0755); err != nil {
		t.Fatal(err)
	}
	write := func(slug, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(w.Root, "concepts", slug+".md"), []byte("---\ntitle: "+slug+"\ntype: concept\ntags: []\ncreated: 2026-10-10\nupdated: 2026-10-10\n---\n"+body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", "Old same claim.")
	write("b", "Old same claim.")
	st, err := store.Open(w.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = indexer.Reindex(w, st, nil, reconcileEngine{profile: embed.ProfileIDFor(w.Cfg.Embedding.Model)}, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()
	write("a", "Current corrected claim.")
	if err = os.Remove(filepath.Join(w.Root, "concepts/b.md")); err != nil {
		t.Fatal(err)
	}
	reports := enrichForeign(w, []reconcile.Candidate{{RelPath: "concepts/a.md", Kind: "modified"}, {RelPath: "concepts/b.md", Kind: "deleted"}})
	if len(reports) != 2 {
		t.Fatalf("missing raw foreign candidates: %+v", reports)
	}
	for _, r := range reports {
		if len(r.DupCandidates) != 0 {
			t.Fatalf("stale duplicate evidence: %+v", r)
		}
	}
}
