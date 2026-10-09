package store

import (
	"github.com/neutrospec/canopy/internal/wiki"
	"math"
	"math/rand"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestVectorValidation(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, v := range [][]float32{nil, {0, 0}, {2, 0}, {float32(math.NaN()), 0}, {float32(math.Inf(1)), 0}} {
		if err := st.ReplaceChunks("a", []int{0}, []string{"h"}, []string{"text"}, [][]float32{v}); err == nil {
			t.Errorf("invalid vector accepted: %v", v)
		}
	}
}

func TestBoundedCandidatesMatchExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, cap := range []int{0, 1, 2, 5} {
		for _, k := range []int{1, 2, 8, 15} {
			all := []ChunkHit{}
			for p := range 12 {
				for seq := range 7 {
					all = append(all, ChunkHit{Slug: string(rune('a' + p)), Seq: seq, Score: float64(rng.Intn(7))})
				}
			}
			sort.Slice(all, func(i, j int) bool { return chunkLess(all[i], all[j]) })
			want := []ChunkHit{}
			counts := map[string]int{}
			for _, h := range all {
				if cap > 0 && counts[h.Slug] >= cap {
					continue
				}
				counts[h.Slug]++
				want = append(want, h)
				if len(want) == k {
					break
				}
			}
			for range 30 {
				rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
				got := []ChunkHit{}
				for _, h := range all {
					got = offer(got, h, k, cap)
					if len(got) > k {
						t.Fatal("candidate storage exceeded k")
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("k=%d cap=%d\ngot=%+v\nwant=%+v", k, cap, got, want)
				}
			}
		}
	}
}

func TestVectorCountsAndNearUnitControl(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.ReplaceChunks("a", []int{0, 1}, []string{"h"}, []string{"t"}, [][]float32{{1, 0}}); err == nil {
		t.Fatal("field count mismatch accepted")
	}
	if err := st.ReplaceChunks("a", []int{0}, []string{"h"}, []string{"t"}, nil); err == nil {
		t.Fatal("embedding count mismatch accepted")
	}
	if _, err := st.SearchChunks([]float32{0.9, 0.436}, 1, 1); err != nil {
		t.Fatalf("near-unit vector rejected: %v", err)
	}
	if _, err := st.SearchChunks([]float32{1, 0}, 1, -1); err == nil {
		t.Fatal("negative per-page accepted")
	}
}

func TestConcurrentStoreProfileSwapRejectsOldQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	a, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err = a.RebuildPages([]*wiki.Page{{Slug: "a", RelPath: "concepts/a.md", Body: "raw"}}); err != nil {
		t.Fatal(err)
	}
	if err = a.SetProfile("space-a"); err != nil {
		t.Fatal(err)
	}
	if err = a.ReplaceChunks("a", []int{0}, []string{"h"}, []string{"raw"}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if _, err = a.SearchChunks([]float32{1, 0}, 1, 1); err != nil {
		t.Fatalf("normal current profile failed: %v", err)
	}
	if err = b.SetProfile("space-b"); err != nil {
		t.Fatal(err)
	}
	if err = b.ReplaceChunks("a", []int{0}, []string{"h"}, []string{"raw"}, [][]float32{{0, 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err = a.SearchChunks([]float32{1, 0}, 1, 1); err == nil || !strings.Contains(err.Error(), "profile changed") {
		t.Fatalf("old query compared to new same-dimensional space: %v", err)
	}
	if _, err = a.PageVectors(); err == nil {
		t.Fatal("old store handle silently consumed swapped profile")
	}
}

func TestRetrievalBounds(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, k := range []int{-1, 0, 10001} {
		if _, err := st.SearchKeyword("test", k); err == nil {
			t.Errorf("keyword accepted k=%d", k)
		}
		if _, err := st.SearchChunks([]float32{1, 0}, k, 1); err == nil {
			t.Errorf("chunks accepted k=%d", k)
		}
	}
}
