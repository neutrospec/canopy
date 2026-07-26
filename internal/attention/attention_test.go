package attention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/reads"
)

func TestEventLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attention", "x.db")
	ev, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	now := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)
	if err := ev.Log(now, "Page-A", DoorAgent, KindShow, ""); err != nil {
		t.Fatal(err)
	}
	if err := ev.Log(now.Add(time.Minute), "page-a", DoorAgent, KindRecall, "질문"); err != nil {
		t.Fatal(err)
	}
	// Slugs are stored lowercased so counts join with the aggregates.
	if n, err := ev.CountBySlug("PAGE-A"); err != nil || n != 2 {
		t.Fatalf("want 2 events, got %d (%v)", n, err)
	}
	recent, err := ev.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].Kind != KindRecall || recent[0].Meta != "질문" {
		t.Fatalf("recent order/meta wrong: %+v", recent)
	}
}

// Touch = per-call events (machine-local) + day-quantized wiki aggregate.
// The second same-day touch adds events but must not change the wiki file.
func TestTouch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	w := &config.Wiki{Root: filepath.Join(home, "wiki")}

	if err := Touch(w, []string{"page-a", "page-b"}, KindRecall, "질문"); err != nil {
		t.Fatal(err)
	}
	rs, err := reads.Load(w)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Agent["page-a"] == nil || rs.Agent["page-b"] == nil {
		t.Fatalf("aggregate missing: %+v", rs.Agent)
	}
	if len(rs.Reads) != 0 {
		t.Fatalf("human reads polluted (H4): %+v", rs.Reads)
	}

	before := fileBytes(t, reads.AgentPath(w))
	if err := Touch(w, []string{"page-a"}, KindShow, ""); err != nil {
		t.Fatal(err)
	}
	if after := fileBytes(t, reads.AgentPath(w)); string(after) != string(before) {
		t.Fatal("same-day touch rewrote the wiki aggregate (H7)")
	}
	ev, err := Open(w.AttentionDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	if n, _ := ev.CountBySlug("page-a"); n != 2 {
		t.Fatalf("events should record every call, got %d", n)
	}
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
