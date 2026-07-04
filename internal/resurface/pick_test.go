package resurface

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/wiki"
)

var now = time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)

// makeScan builds a synthetic wiki: one fresh page, one forgotten page,
// and one stale hub referenced by many fresh pages.
func makeScan(t *testing.T) *wiki.ScanResult {
	t.Helper()
	res := &wiki.ScanResult{BySlug: map[string]*wiki.Page{}}
	add := func(slug, updated, body string) *wiki.Page {
		raw := fmt.Sprintf("---\ntitle: \"%s\"\ncreated: %s\nupdated: %s\ntype: concept\ntags: [tool]\n---\n\n%s\n", slug, updated, updated, body)
		p := wiki.Parse("concepts/"+slug+".md", []byte(raw))
		res.Pages = append(res.Pages, p)
		res.BySlug[slug] = p
		return p
	}
	add("fresh", "2026-07-01", "recent page, links [[hub]]")
	add("forgotten", "2026-04-01", "old page nobody links to. 내용 문단이다.")
	add("hub", "2026-03-01", "central page. 허브 문단.")
	for i := 0; i < 4; i++ {
		add(fmt.Sprintf("ref%d", i), "2026-07-01", fmt.Sprintf("links [[hub]] page %d", i))
	}
	return res
}

func newState() *State {
	return &State{Version: 1, Shown: map[string]string{}, ShownPairs: map[string]string{}, Snoozed: map[string]string{}}
}

func TestPickHub(t *testing.T) {
	scan := makeScan(t)
	picks, err := PickPages(scan, newState(), "hub", 1, now, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) != 1 || picks[0].Slug != "hub" {
		t.Fatalf("want hub pick, got %+v", picks)
	}
	if len(picks[0].Backlinks) < 4 {
		t.Errorf("hub should report its backlinks, got %v", picks[0].Backlinks)
	}
}

func TestPickRandomExcludesFreshAndCooldown(t *testing.T) {
	scan := makeScan(t)
	st := newState()
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 20; i++ {
		picks, err := PickPages(scan, st, "random", 1, now, rng)
		if err != nil {
			t.Fatal(err)
		}
		if len(picks) == 0 {
			break // pool exhausted by cooldown — expected eventually
		}
		if picks[0].Slug == "fresh" || picks[0].RelPath == "concepts/ref0.md" {
			t.Fatalf("fresh page picked: %+v", picks[0])
		}
		st.MarkShown([]string{picks[0].Slug}, now)
	}
	// After both old pages are shown, the pool must be empty.
	picks, _ := PickPages(scan, st, "random", 1, now, rng)
	if len(picks) != 0 {
		t.Errorf("cooldown not respected: %+v", picks)
	}
}

func TestSnoozeAndDownvote(t *testing.T) {
	scan := makeScan(t)
	st := newState()
	st.Snooze("forgotten", 7, now)
	st.AddFeedback("hub", "down", now)
	picks, err := PickPages(scan, st, "auto", 5, now, rand.New(rand.NewSource(3)))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range picks {
		if p.Slug == "forgotten" || p.Slug == "hub" {
			t.Errorf("snoozed/downvoted page picked: %s", p.Slug)
		}
	}
}

func TestPickBridges(t *testing.T) {
	scan := makeScan(t)
	// forgotten↔hub are similar but unlinked; fresh↔hub are linked.
	vectors := map[string][]float32{
		"forgotten": {1, 0},
		"hub":       {0.95, 0.312},
		"fresh":     {0.9, 0.436},
	}
	bridges := PickBridges(scan, vectors, newState(), 0.9, 10, false, now)
	found := map[string]bool{}
	for _, b := range bridges {
		found[b.A.Slug+"|"+b.B.Slug] = true
		if b.Linked {
			t.Errorf("default mode returned linked pair: %+v", b)
		}
		if (b.A.Slug == "fresh" && b.B.Slug == "hub") || (b.A.Slug == "hub" && b.B.Slug == "fresh") {
			t.Error("linked pair fresh↔hub must be excluded")
		}
	}
	if !found["forgotten|hub"] {
		t.Errorf("expected forgotten|hub bridge, got %+v", bridges)
	}

	// --include-linked surfaces fresh↔hub, flagged as linked (G6).
	withLinked := PickBridges(scan, vectors, newState(), 0.9, 10, true, now)
	sawLinked := false
	for _, b := range withLinked {
		if (b.A.Slug == "fresh" && b.B.Slug == "hub") || (b.A.Slug == "hub" && b.B.Slug == "fresh") {
			sawLinked = true
			if !b.Linked {
				t.Error("linked pair not flagged Linked=true")
			}
		}
	}
	if !sawLinked {
		t.Errorf("include-linked did not surface the linked pair: %+v", withLinked)
	}

	// Dismissed pairs never come back.
	st := newState()
	st.DismissPair("forgotten", "hub")
	if bs := PickBridges(scan, vectors, st, 0.9, 10, false, now); len(bs) != len(bridges)-1 {
		t.Errorf("dismiss not honored: %+v", bs)
	}
}
