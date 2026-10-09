package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/wiki"
)

func TestMutationRewritesOnlyResolvedInboundLinks(t *testing.T) {
	for _, verb := range []string{"mv", "rm", "archive"} {
		t.Run(verb, func(t *testing.T) {
			w := mutationWiki(t)
			longPath := filepath.Join(w.Root, "concepts/alpha.md.md")
			long := []byte(wiki.NewPageContent("Long", "concept", "2026-01-01", nil, nil, "long sentinel"))
			if err := os.WriteFile(longPath, long, 0644); err != nil {
				t.Fatal(err)
			}
			body := "VALID [[concepts/alpha.md#here|short]] AMBIGUOUS [[alpha.md|which]] WRONG [[entities/alpha.md|entity]] NESTED [[concepts/nested/alpha.md|nested]] LONG [[concepts/alpha.md.md|long]] CODE `[[concepts/alpha.md]]`."
			refPath := filepath.Join(w.Root, "concepts/omega.md")
			raw := wiki.NewPageContent("Ref", "concept", "2026-01-01", nil, nil, body)
			if err := os.WriteFile(refPath, []byte(raw), 0644); err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(raw, "[[concepts/alpha.md#here|short]]", "short", 1)
			var err error
			switch verb {
			case "mv":
				_, err = mutationRun(t, w, cmdMv(), "concepts/alpha.md", "--slug", "delta")
				want = strings.Replace(raw, "[[concepts/alpha.md#here|short]]", "[[concepts/delta.md#here|short]]", 1)
			case "rm":
				_, err = mutationRun(t, w, cmdRm(), "concepts/alpha.md", "--force")
			case "archive":
				_, err = mutationRun(t, w, cmdArchive(), "concepts/alpha.md")
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(refPath)
			if err != nil || string(got) != want {
				t.Errorf("rewrote a non-edge: %v\ngot %s\nwant %s", err, got, want)
			}
			if got, err := os.ReadFile(longPath); err != nil || string(got) != string(long) {
				t.Errorf("non-target page changed: %v", err)
			}
		})
	}
}

func TestUnresolvedLinksDoNotBlockRemoval(t *testing.T) {
	w := mutationWiki(t)
	path := filepath.Join(w.Root, "concepts/omega.md")
	raw := wiki.NewPageContent("Ref", "concept", "2026-01-01", nil, nil, "[[entities/alpha.md]] [[concepts/nested/alpha.md]]")
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := mutationRun(t, w, cmdRm(), "alpha"); err != nil {
		t.Fatalf("unresolved paths were treated as backlinks: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != raw {
		t.Fatalf("unresolved source changed: %v", err)
	}
}
