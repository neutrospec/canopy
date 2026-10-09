package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/wiki"
)

func BenchmarkRetrievalScale(b *testing.B) {
	for _, pages := range []int{200, 1000} {
		b.Run(fmt.Sprintf("pages%d_chunks%d_k8_cap2", pages, pages*5), func(b *testing.B) {
			st, err := Open(filepath.Join(b.TempDir(), "db"))
			if err != nil {
				b.Fatal(err)
			}
			defer st.Close()
			ps := make([]*wiki.Page, pages)
			for i := range ps {
				slug := fmt.Sprintf("p%04d", i)
				ps[i] = &wiki.Page{Slug: slug, RelPath: "concepts/" + slug + ".md", Title: slug, Body: strings.Repeat("raw evidence ", 80)}
			}
			if err = st.RebuildPages(ps); err != nil {
				b.Fatal(err)
			}
			v := make([]float32, 1024)
			v[0] = 1
			for _, p := range ps {
				if err = st.ReplaceChunks(p.Slug, []int{0, 1, 2, 3, 4}, []string{"a", "b", "c", "d", "e"}, []string{p.Body, p.Body, p.Body, p.Body, p.Body}, [][]float32{v, v, v, v, v}); err != nil {
					b.Fatal(err)
				}
			}
			start := time.Now()
			if _, err = st.SearchChunks(v, 8, 2); err != nil {
				b.Fatal(err)
			}
			firstQuery := time.Since(start)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				hits, err := st.SearchChunks(v, 8, 2)
				if err != nil || len(hits) != 8 {
					b.Fatalf("hits=%d err=%v", len(hits), err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(firstQuery.Nanoseconds()), "first-query-ns")
		})
	}
}
