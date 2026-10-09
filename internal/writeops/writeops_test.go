package writeops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
)

func pipelineWiki(t *testing.T) *config.Wiki {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "concepts"), 0o755)
	os.WriteFile(filepath.Join(root, "concepts", "alpha.md"), []byte(wiki.NewPageContent("Alpha", "concept", "2026-10-10", nil, nil, "already published")), 0o644)
	return &config.Wiki{Root: root, Cfg: config.Default()}
}
func TestPostpublicationFailuresNameStageAndPreserveAppliedHash(t *testing.T) {
	for _, stage := range []string{"ledger", "index", "log", "reindex"} {
		t.Run(stage, func(t *testing.T) {
			w := pipelineWiki(t)
			page := filepath.Join(w.Root, "concepts", "alpha.md")
			before, _ := os.ReadFile(page)
			switch stage {
			case "ledger":
				os.MkdirAll(filepath.Dir(reconcile.Path(w)), 0o755)
				os.WriteFile(reconcile.Path(w), []byte("{broken"), 0o644)
			case "index":
				os.Mkdir(w.IndexMDPath(), 0o755)
			case "log":
				os.WriteFile(w.LogsDir(), []byte("obstruction"), 0o644)
			case "reindex":
				os.MkdirAll(w.DBPath(), 0o755)
			}
			_, err := Run(w, "update", "concepts/alpha.md", nil, "", reconcile.Effects{Written: []string{"concepts/alpha.md"}})
			var failure *StageError
			if !errors.As(err, &failure) || failure.Stage != stage {
				t.Fatalf("want %s, got %v", stage, err)
			}
			after, _ := os.ReadFile(page)
			if reconcile.HashBytes(before) != reconcile.HashBytes(after) {
				t.Fatal("postprocessing rewrote source")
			}
		})
	}
}
func TestResumedCheckinActivityIsIdempotent(t *testing.T) {
	w := pipelineWiki(t)
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Unlock()
	for range 2 {
		if _, err := RunLockedWithID(w, "update", "concepts/alpha.md", nil, "", reconcile.Effects{Written: []string{"concepts/alpha.md"}}, "stable-checkin-id"); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := logops.ReadRecent(w, 10)
	if err != nil || len(entries) != 1 || entries[0].OperationID != "stable-checkin-id" {
		t.Fatalf("repeated activity: %+v %v", entries, err)
	}
}
