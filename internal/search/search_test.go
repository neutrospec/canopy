package search

import (
	"math"
	"testing"

	"github.com/neutrospec/canopy/internal/store"
)

func TestFuseEmpty(t *testing.T) {
	hits := Fuse(10)
	if len(hits) != 0 {
		t.Errorf("expected empty, got %d", len(hits))
	}
}

func TestFuseSingleList(t *testing.T) {
	list := []store.Hit{
		{Slug: "a", Title: "A", Score: 5.0},
		{Slug: "b", Title: "B", Score: 3.0},
	}
	hits := Fuse(10, list)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].Slug != "a" || hits[1].Slug != "b" {
		t.Errorf("order wrong: %+v", hits)
	}
	// RRF score for rank 0 = 1/(60+1) ≈ 0.01639
	expected := 1.0 / (rrfK + 1)
	if math.Abs(hits[0].Score-expected) > 1e-6 {
		t.Errorf("score %f, want %f", hits[0].Score, expected)
	}
}

func TestFuseTwoLists(t *testing.T) {
	kw := []store.Hit{
		{Slug: "a", Title: "A", Score: 10.0, Snippet: "kw snippet a"},
		{Slug: "b", Title: "B", Score: 5.0},
	}
	sem := []store.Hit{
		{Slug: "b", Title: "B", Score: 8.0},
		{Slug: "c", Title: "C", Score: 6.0},
	}
	hits := Fuse(10, kw, sem)
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d: %+v", len(hits), hits)
	}
	// a: rank 0 in kw only → 1/(60+1) ≈ 0.01639
	// b: rank 1 in kw (1/62) + rank 0 in sem (1/61) ≈ 0.01613 + 0.01639 = 0.03252
	// c: rank 1 in sem only → 1/62 ≈ 0.01613
	if hits[0].Slug != "b" {
		t.Errorf("expected b first (highest RRF), got %s", hits[0].Slug)
	}
	// Snippet should come from keyword list (slug 'a' has it in kw, not in sem).
	for _, h := range hits {
		if h.Slug == "a" && h.Snippet != "kw snippet a" {
			t.Errorf("a's snippet wrong: %q", h.Snippet)
		}
	}
}

func TestFuseTopK(t *testing.T) {
	list := []store.Hit{
		{Slug: "a"}, {Slug: "b"}, {Slug: "c"}, {Slug: "d"},
	}
	hits := Fuse(2, list)
	if len(hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(hits))
	}
}

func TestFuseDeduplicates(t *testing.T) {
	kw := []store.Hit{
		{Slug: "a", Title: "A"},
		{Slug: "b", Title: "B"},
	}
	sem := []store.Hit{
		{Slug: "a", Title: "A"},
		{Slug: "c", Title: "C"},
	}
	hits := Fuse(10, kw, sem)
	if len(hits) != 3 {
		t.Errorf("expected 3 unique slugs, got %d", len(hits))
	}
}
