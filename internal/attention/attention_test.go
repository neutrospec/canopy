package attention

import (
	"encoding/json"
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

// M12 instrument queries: per-slug trails, weekly buckets, consumption
// ranking (searches excluded — they carry no slug).
func TestInstrumentQueries(t *testing.T) {
	ev, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	ev.Log(now.Add(-8*24*time.Hour), "page-a", DoorAgent, KindShow, "") // last week
	ev.Log(now.Add(-time.Hour), "page-a", DoorWeb, KindRead, "")        // this week
	ev.Log(now.Add(-time.Hour), "page-b", DoorAgent, KindRecall, "질문")  // this week
	ev.Log(now.Add(-30*time.Minute), "", DoorWeb, KindSearch, "쿼리")     // no slug
	ev.Log(now.Add(-90*24*time.Hour), "page-a", DoorWeb, KindView, "")  // out of window

	top, err := ev.TopConsumed(now.Add(-30*24*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 || top[0].Slug != "page-a" || top[0].Total != 2 || top[0].Web != 1 || top[0].Agent != 1 {
		t.Fatalf("TopConsumed wrong: %+v", top)
	}

	counts, err := ev.WeeklyCounts("page-a", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 4 || counts[3] != 1 || counts[2] != 1 {
		t.Fatalf("weekly buckets wrong: %v", counts)
	}

	trail, err := ev.BySlug("page-a", 2)
	if err != nil || len(trail) != 2 || trail[0].Kind != KindRead {
		t.Fatalf("BySlug wrong: %+v (%v)", trail, err)
	}
}

// Gaps append door-tagged jsonl lines to the shared wiki file (H10).
func TestLogGap(t *testing.T) {
	w := &config.Wiki{Root: t.TempDir()}
	if err := LogGap(w, DoorAgent, "없는 주제", 0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(GapsPath(w))
	if err != nil {
		t.Fatal(err)
	}
	var g Gap
	if err := json.Unmarshal(b[:len(b)-1], &g); err != nil {
		t.Fatal(err)
	}
	if g.Query != "없는 주제" || g.Door != DoorAgent || g.Results != 0 {
		t.Fatalf("gap line wrong: %+v", g)
	}
}

// Lifecycle events share the table but never pollute attention
// instruments (N3); Query filters and Prune (N5) drive `canopy events`.
func TestLifecycleDomainSeparation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attention", "x.db")
	ev, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	ev.Log(now, "page-a", DoorWeb, KindView, "")
	ev.Log(now.Add(time.Minute), "", DoorWeb, KindTaskFiled, "connect-a--b")
	ev.Log(now.Add(2*time.Minute), "", DoorAgent, KindTaskRejected, "connect-a--b: no link yet")
	ev.Log(now.Add(3*time.Minute), "page-a", DoorAgent, KindBless, "concepts/page-a.md")
	ev.Log(now.Add(4*time.Minute), "", DoorAgent, KindSyncDone, "committed=true pushed=true")

	if !IsLifecycle(KindTaskFiled) || IsLifecycle(KindView) {
		t.Fatal("dot discriminator broken")
	}

	// Instrument mode sees only the attention event.
	attn, err := ev.Query(Filter{AttnOnly: true})
	if err != nil || len(attn) != 1 || attn[0].Kind != KindView {
		t.Fatalf("AttnOnly leak: %+v (%v)", attn, err)
	}
	if top, _ := ev.TopConsumed(now.Add(-time.Hour), 10); len(top) != 1 || top[0].Total != 1 {
		t.Fatalf("TopConsumed counts lifecycle as consumption: %+v", top)
	}
	if counts, _ := ev.WeeklyCounts("page-a", 2, now.Add(time.Hour)); counts[1] != 1 {
		t.Fatalf("WeeklyCounts counts lifecycle: %v", counts)
	}
	if trail, _ := ev.BySlug("page-a", 10); len(trail) != 1 {
		t.Fatalf("BySlug shows lifecycle: %+v", trail)
	}

	// Prefix and exact kind filters (the `canopy events --kind` surface).
	if got, _ := ev.Query(Filter{Kind: "task.*"}); len(got) != 2 {
		t.Fatalf("prefix filter: %+v", got)
	}
	if got, _ := ev.Query(Filter{Kind: KindSyncDone}); len(got) != 1 || got[0].Meta != "committed=true pushed=true" {
		t.Fatalf("exact filter: %+v", got)
	}
	if got, _ := ev.Query(Filter{Since: now.Add(3 * time.Minute)}); len(got) != 2 {
		t.Fatalf("since filter: %+v", got)
	}

	// Retention: prune old, keep new.
	if n, err := ev.Prune(now.Add(2 * time.Minute)); err != nil || n != 2 {
		t.Fatalf("prune: n=%d err=%v", n, err)
	}
	if rest, _ := ev.Query(Filter{Limit: 10}); len(rest) != 3 {
		t.Fatalf("prune kept wrong rows: %+v", rest)
	}
}
