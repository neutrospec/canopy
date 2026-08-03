// canopy reconcile — the canonicalization gate's CLI (docs/
// reconcile-design.md): report back-door changes the blessed-content
// ledger doesn't know (read-only, deterministic), and bless reviewed
// state into the ledger. Judgment stays with the agent (원칙 6): canopy
// only produces candidates with dup/lint evidence attached.
package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/lint"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// dupThreshold mirrors relatedThreshold: bge-m3 page vectors have a
// high similarity floor, so 0.80 keeps only plausible duplicates.
const dupThreshold = 0.80

type dupCandidate struct {
	Slug       string  `json:"slug"`
	Similarity float64 `json:"similarity"`
}

type foreignReport struct {
	RelPath       string         `json:"rel_path"`
	Kind          string         `json:"kind"`
	DupCandidates []dupCandidate `json:"dup_candidates"`
	Issues        []string       `json:"issues"`
}

func cmdReconcile() *cobra.Command {
	c := &cobra.Command{
		Use:   "reconcile",
		Short: "Report back-door changes the blessed ledger doesn't know (read-only)",
		Long: `Compares every page's content hash against the blessed ledger
(_meta/reconcile/state.json). Pipeline writes are blessed automatically;
what remains is foreign — edited/created/deleted outside canopy — and
needs judgment before it counts as canonical: merge duplicates into
existing pages, record contradictions side by side, fix links, then
bless what stands. See docs/reconcile-design.md.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			cands, initialized, err := reconcile.Foreign(w)
			if err != nil {
				return err
			}
			if !initialized {
				if flagJSON {
					return emitJSON(map[string]any{"initialized": false, "foreign": []foreignReport{}})
				}
				fmt.Println("정규화 게이트가 아직 꺼져 있습니다 (원장 없음).")
				fmt.Println("현재 상태 전체를 기준선으로 삼아 켜려면: canopy reconcile bless --all")
				return nil
			}
			reports := enrichForeign(w, cands)
			if flagJSON {
				return emitJSON(map[string]any{"initialized": true, "foreign": reports})
			}
			if len(reports) == 0 {
				fmt.Println("✓ 정규화 평면 깨끗 — 원장이 모르는 변경 없음")
				return nil
			}
			fmt.Printf("미정규화 외부 변경 %d건:\n\n", len(reports))
			for _, r := range reports {
				fmt.Printf("  [%s] %s\n", r.Kind, r.RelPath)
				for _, d := range r.DupCandidates {
					fmt.Printf("      ≈ %.2f  %s (중복이면 새 페이지 대신 이쪽을 갱신)\n", d.Similarity, d.Slug)
				}
				for _, is := range r.Issues {
					fmt.Printf("      ! %s\n", is)
				}
			}
			fmt.Println("\nNEXT: 판단 후 — 수정은 canopy update/mv (자동 축복),")
			fmt.Println("      이대로 수용은 canopy reconcile bless <path>")
			return nil
		},
	}
	c.AddCommand(cmdReconcileBless())
	return c
}

func cmdReconcileBless() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "bless [<page|path>…]",
		Short: "Mark reviewed state — current content, or its absence — as canonical",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			_, initialized, err := reconcile.Load(w)
			if err != nil {
				return err
			}
			if all {
				n, err := reconcile.BlessAll(w)
				if err != nil {
					return err
				}
				attention.LogLifecycle(w, time.Now(), "", attention.DoorAgent,
					attention.KindBless, fmt.Sprintf("all:%d", n))
				if flagJSON {
					return emitJSON(map[string]any{"blessed": n, "initialized_now": !initialized})
				}
				if !initialized {
					fmt.Printf("✓ 정규화 게이트 켜짐 — 페이지 %d개를 기준선으로 축복\n", n)
				} else {
					fmt.Printf("✓ 현재 상태 전체 축복 (%d페이지)\n", n)
				}
				fmt.Println("NEXT: canopy sync   (원장은 위키와 함께 여행합니다)")
				return nil
			}
			if len(args) == 0 {
				return fmt.Errorf("bless할 대상을 지정하세요: <page|path>… 또는 --all")
			}
			if !initialized {
				return fmt.Errorf("게이트가 꺼져 있습니다 — 먼저 `canopy reconcile bless --all`로 기준선을 잡으세요")
			}
			// Args may be slugs or rel paths; slugs resolve via the scan.
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			var rels []string
			for _, a := range args {
				if p, ok := scan.BySlug[wiki.NormalizeLink(a)]; ok {
					rels = append(rels, p.RelPath)
				} else {
					rels = append(rels, a)
				}
			}
			if err := reconcile.BlessPaths(w, rels); err != nil {
				return err
			}
			for _, rel := range rels {
				slug := wiki.NormalizeLink(rel)
				attention.LogLifecycle(w, time.Now(), slug, attention.DoorAgent, attention.KindBless, rel)
			}
			n, _, err := reconcile.Count(w)
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"blessed": rels, "remaining_foreign": n})
			}
			for _, rel := range rels {
				fmt.Printf("✓ blessed %s\n", rel)
			}
			if n > 0 {
				fmt.Printf("남은 미정규화 변경 %d건 — canopy reconcile\n", n)
			} else {
				fmt.Println("✓ 정규화 평면 깨끗")
			}
			fmt.Println("NEXT: canopy sync   (원장은 위키와 함께 여행합니다)")
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "bless the entire current state (initializes the gate on first use)")
	return c
}

// enrichForeign attaches judgment evidence to raw candidates: per-page
// lint findings and semantic duplicate candidates (graceful degradation
// when the embedding stack is missing — same policy as search).
func enrichForeign(w *config.Wiki, cands []reconcile.Candidate) []foreignReport {
	reports := make([]foreignReport, 0, len(cands))
	if len(cands) == 0 {
		return reports
	}

	var eng embed.Engine
	if embed.Available() && embed.ModelAvailable() {
		if e, err := embed.New(); err == nil {
			eng = e
			defer eng.Close()
		}
	}
	st, scan, err := refreshIndex(w, eng)
	if err != nil {
		// Evidence is best-effort; the raw candidate list still stands.
		for _, c := range cands {
			reports = append(reports, foreignReport{RelPath: c.RelPath, Kind: c.Kind,
				DupCandidates: []dupCandidate{}, Issues: []string{}})
		}
		return reports
	}
	defer st.Close()

	issuesByPage := map[string][]string{}
	for _, f := range lint.Run(w, scan).Findings {
		issuesByPage[f.Page] = append(issuesByPage[f.Page], f.Kind+": "+f.Message)
	}
	pageByRel := map[string]*wiki.Page{}
	for _, p := range scan.Pages {
		pageByRel[p.RelPath] = p
	}
	vectors, _ := st.PageVectors()

	for _, c := range cands {
		r := foreignReport{RelPath: c.RelPath, Kind: c.Kind,
			DupCandidates: []dupCandidate{}, Issues: issuesByPage[c.RelPath]}
		if r.Issues == nil {
			r.Issues = []string{}
		}
		if p, ok := pageByRel[c.RelPath]; ok && c.Kind != "deleted" {
			if v, ok := vectors[p.Slug]; ok {
				for slug, other := range vectors {
					if strings.EqualFold(slug, p.Slug) {
						continue
					}
					if sim := store.Cosine(v, other); sim >= dupThreshold {
						r.DupCandidates = append(r.DupCandidates, dupCandidate{Slug: slug, Similarity: sim})
					}
				}
				sort.Slice(r.DupCandidates, func(i, j int) bool {
					if r.DupCandidates[i].Similarity != r.DupCandidates[j].Similarity {
						return r.DupCandidates[i].Similarity > r.DupCandidates[j].Similarity
					}
					return r.DupCandidates[i].Slug < r.DupCandidates[j].Slug
				})
				if len(r.DupCandidates) > 3 {
					r.DupCandidates = r.DupCandidates[:3]
				}
			}
		}
		reports = append(reports, r)
	}
	return reports
}
