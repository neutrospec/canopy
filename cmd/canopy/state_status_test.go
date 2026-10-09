package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/tasks"
)

func TestStatusReportsUnavailableState(t *testing.T) {
	w := mutationWiki(t)
	for _, path := range []string{reconcile.Path(w), filepath.Join(tasks.Dir(w), "broken.json")} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("null\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := mutationRun(t, w, cmdStatus())
	if err != nil {
		t.Fatal(err)
	}
	if result["tasks_pending"] != nil || result["tasks_available"] != false || result["reconcile_available"] != false || result["unreconciled"] != nil {
		t.Fatalf("unreadable state represented as healthy zero: %+v", result)
	}
	if warnings, ok := result["warnings"].([]any); !ok || len(warnings) < 2 {
		t.Fatalf("diagnostics missing: %+v", result)
	}
}

func TestResurfaceBridgeBoundsBeforeWiki(t *testing.T) {
	oldWiki := flagWiki
	flagWiki = filepath.Join(t.TempDir(), "missing")
	defer func() { flagWiki = oldWiki }()
	for _, factory := range []func() *cobra.Command{cmdResurface, cmdBridge} {
		for _, n := range []string{"-1", "0", "1001"} {
			c := factory()
			c.SetArgs([]string{"--count=" + n})
			c.SilenceUsage = true
			c.SilenceErrors = true
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "count must") {
				t.Fatalf("count validation did not precede wiki resolution: %v", err)
			}
		}
	}
}

func TestGCRetentionCannotWrapBeforeWiki(t *testing.T) {
	oldWiki := flagWiki
	flagWiki = filepath.Join(t.TempDir(), "missing")
	defer func() { flagWiki = oldWiki }()
	for _, factory := range []func() *cobra.Command{cmdTasksGC, cmdEventsGC} {
		for _, days := range []string{"-1", "213504", "1000000000"} {
			c := factory()
			c.SetArgs([]string{"--days=" + days})
			c.SilenceUsage = true
			c.SilenceErrors = true
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "days must") {
				t.Fatalf("retention overflow reached wiki: %v", err)
			}
		}
	}
}
