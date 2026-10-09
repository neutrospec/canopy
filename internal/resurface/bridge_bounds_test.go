package resurface

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

func TestBridgeTopKMatchesExhaustiveWithFiltersAndTies(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	scan := &wiki.ScanResult{BySlug: map[string]*wiki.Page{}}
	vectors := map[string][]float32{}
	for i, angle := range []float64{0, 0, .2, .4, .7, 1.2, 1.8, 2.6} {
		slug := fmt.Sprintf("p%d", i)
		p := &wiki.Page{Slug: slug, Title: slug, Body: "evidence-" + slug}
		if i == 0 {
			p.Links = []string{"p1"}
		}
		scan.Pages = append(scan.Pages, p)
		scan.BySlug[slug] = p
		vectors[slug] = []float32{float32(math.Cos(angle)), float32(math.Sin(angle))}
	}
	s := newState()
	s.MarkPairShown("p2", "p3", now)
	type expected struct {
		a, b  string
		score float64
	}
	for _, includeLinked := range []bool{false, true} {
		all := []expected{}
		for i := 0; i < len(scan.Pages); i++ {
			for j := i + 1; j < len(scan.Pages); j++ {
				a, b := scan.Pages[i].Slug, scan.Pages[j].Slug
				if (a == "p0" && b == "p1" && !includeLinked) || (a == "p2" && b == "p3") {
					continue
				}
				sim := store.Cosine(vectors[a], vectors[b])
				if sim >= .1 {
					all = append(all, expected{a, b, sim})
				}
			}
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].score != all[j].score {
				return all[i].score > all[j].score
			}
			if all[i].a != all[j].a {
				return all[i].a < all[j].a
			}
			return all[i].b < all[j].b
		})
		for _, k := range []int{0, 1, 3, 8, 50} {
			got := PickBridges(scan, vectors, s, .1, k, includeLinked, now)
			want := min(k, len(all))
			if len(got) != want {
				t.Fatalf("k=%d linked=%v got=%d want=%d", k, includeLinked, len(got), want)
			}
			for i, b := range got {
				if b.A.Slug != all[i].a || b.B.Slug != all[i].b || b.Similarity != all[i].score || b.A.Excerpt != "evidence-"+b.A.Slug {
					t.Fatalf("rank %d k=%d: %+v want %+v", i, k, b, all[i])
				}
			}
		}
	}
}

func BenchmarkPickBridgesDense(b *testing.B) {
	scan := &wiki.ScanResult{BySlug: map[string]*wiki.Page{}}
	vectors := map[string][]float32{}
	for i := 0; i < 200; i++ {
		slug := fmt.Sprintf("p%04d", i)
		p := &wiki.Page{Slug: slug, Title: slug, Body: strings.Repeat("Current source evidence. ", 20)}
		scan.Pages = append(scan.Pages, p)
		scan.BySlug[slug] = p
		v := make([]float32, 1024)
		v[0] = 1
		vectors[slug] = v
	}
	state := newState()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := PickBridges(scan, vectors, state, .7, 1, false, time.Unix(1791580000, 0)); len(got) != 1 {
			b.Fatalf("dense result count = %d", len(got))
		}
	}
}
