// Resurface commands: candidate engines for the second-brain loop.
// canopy filters, scores, or samples candidates and records state; the agent judges,
// phrases, and delivers (docs/second-brain.md).
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/resurface"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

func cmdResurface() *cobra.Command {
	var n int
	var strategy string
	var peek bool
	c := &cobra.Command{
		Use:   "resurface",
		Short: "Pick forgotten/stale-hub pages to re-encounter (records pick history)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 1 || n > store.MaxK {
				return fmt.Errorf("count must be between 1 and %d", store.MaxK)
			}
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			st, err := resurface.LoadState(w)
			if err != nil {
				return err
			}
			now := time.Now()
			rng := rand.New(rand.NewSource(now.UnixNano()))
			// Attention aggregates keep recently-accessed pages out of the
			// pool (H6). Preserve a healthy half if the other file is unavailable.
			rd, err := reads.Load(w)
			if err != nil {
				fmt.Fprintln(os.Stderr, "warning: partial attention state:", err)
			}
			picks, err := resurface.PickPages(scan, st, rd, strategy, n, now, rng)
			if err != nil {
				return err
			}
			if !peek && len(picks) > 0 {
				var slugs []string
				for _, p := range picks {
					slugs = append(slugs, strings.ToLower(p.Slug))
				}
				st.MarkShown(slugs, now)
				if err := st.Save(w); err != nil {
					return err
				}
			}
			if flagJSON {
				return emitJSON(map[string]any{"picks": picks, "state_updated": !peek && len(picks) > 0})
			}
			if len(picks) == 0 {
				fmt.Println("no eligible pages (all recent, snoozed, or cooling down)")
				return nil
			}
			for _, p := range picks {
				fmt.Printf("[%s] %s — %s\n  %s\n", p.Strategy, p.Slug, p.Title, p.Explanation)
				if p.Excerpt != "" {
					fmt.Printf("  %s\n", p.Excerpt)
				}
			}
			if !peek {
				fmt.Println("(state updated — include _meta/resurface in your next canopy sync)")
			}
			return nil
		},
	}
	c.Flags().IntVarP(&n, "count", "n", 1, "number of picks")
	c.Flags().StringVar(&strategy, "strategy", "auto", "random|hub|auto (auto = 70% random-forgotten, 30% stale-hub)")
	c.Flags().BoolVar(&peek, "peek", false, "don't record picks in state (preview only)")
	c.AddCommand(cmdResurfaceFeedback())
	return c
}

func cmdResurfaceFeedback() *cobra.Command {
	var up, down bool
	var snooze int
	c := &cobra.Command{
		Use:   "feedback <page>",
		Short: "Record 👍/👎/snooze for a resurfaced page (tunes future picks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			choices := 0
			if up {
				choices++
			}
			if down {
				choices++
			}
			if snooze > 0 {
				choices++
			}
			if choices != 1 || snooze < 0 || int64(snooze) > int64(math.MaxInt64)/int64(24*time.Hour) {
				return fmt.Errorf("pass exactly one of --up, --down, --snooze <positive days>; snooze must fit a time.Duration")
			}
			w, err := loadWiki()
			if err != nil {
				return err
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			p, err := scan.Find(args[0])
			if err != nil {
				return err
			}
			slug := strings.ToLower(p.Slug)
			st, err := resurface.LoadState(w)
			if err != nil {
				return err
			}
			now := time.Now()
			switch {
			case up:
				st.AddFeedback(slug, "up", now)
			case down:
				st.AddFeedback(slug, "down", now)
			case snooze > 0:
				st.Snooze(slug, snooze, now)
			default:
				return fmt.Errorf("pass one of --up, --down, --snooze <days>")
			}
			if err := st.Save(w); err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"slug": slug, "recorded": true})
			}
			fmt.Println("✓ recorded")
			return nil
		},
	}
	c.Flags().BoolVar(&up, "up", false, "this pick was valuable")
	c.Flags().BoolVar(&down, "down", false, "this pick was noise (120d cooldown)")
	c.Flags().IntVar(&snooze, "snooze", 0, "hide this page for N days")
	return c
}

func cmdBridge() *cobra.Command {
	var n int
	var minSim float64
	var peek, includeLinked bool
	var dismiss string
	c := &cobra.Command{
		Use:   "bridge",
		Short: "Find similar page pairs: unlinked = connection candidates; --include-linked adds merge/contradiction candidates",
		RunE: func(cmd *cobra.Command, args []string) error {
			if n < 1 || n > store.MaxK {
				return fmt.Errorf("count must be between 1 and %d", store.MaxK)
			}
			if math.IsNaN(minSim) || math.IsInf(minSim, 0) || minSim < -1 || minSim > 1 {
				return fmt.Errorf("min-sim must be finite and between -1 and 1")
			}
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			st, err := resurface.LoadState(w)
			if err != nil {
				return err
			}
			if dismiss != "" {
				a, b, ok := strings.Cut(dismiss, ":")
				if !ok {
					return fmt.Errorf("--dismiss wants slug-a:slug-b")
				}
				scan, err := wiki.Scan(w)
				if err != nil {
					return err
				}
				pa, err := scan.Find(a)
				if err != nil {
					return err
				}
				pb, err := scan.Find(b)
				if err != nil {
					return err
				}
				st.DismissPair(strings.ToLower(pa.Slug), strings.ToLower(pb.Slug))
				if err := st.Save(w); err != nil {
					return err
				}
				if !flagJSON {
					fmt.Println("✓ pair dismissed permanently")
				}
				return nil
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			db, err := store.Open(w.DBPath())
			if err != nil {
				return err
			}
			defer db.Close()
			coverage, err := indexer.EnsureCurrentContext(cmd.Context(), w, db, scan, nil, nil)
			if err != nil {
				return err
			}
			if coverage.NeedsReindex > 0 {
				fmt.Fprintf(os.Stderr, "warning: %d pages need embeddings; bridge uses only current indexed evidence\n", coverage.NeedsReindex)
			}
			vectors, err := db.PageVectors()
			if err != nil {
				return err
			}
			if len(vectors) == 0 {
				return fmt.Errorf("no embeddings in the index — run `canopy reindex` first")
			}
			now := time.Now()
			bridges := resurface.PickBridges(scan, vectors, st, minSim, n, includeLinked, now)
			if !peek && len(bridges) > 0 {
				for _, b := range bridges {
					st.MarkPairShown(strings.ToLower(b.A.Slug), strings.ToLower(b.B.Slug), now)
				}
				if err := st.Save(w); err != nil {
					return err
				}
			}
			if flagJSON {
				return emitJSON(map[string]any{"bridges": bridges, "state_updated": !peek && len(bridges) > 0, "covered_pages": coverage.EmbeddedPages, "needs_reindex": coverage.NeedsReindex, "profile": coverage.Profile})
			}
			if len(bridges) == 0 {
				fmt.Println("no unlinked similar pairs above the threshold — the graph is well connected")
				return nil
			}
			for _, b := range bridges {
				mark := ""
				if b.Linked {
					mark = " (already linked — merge/contradiction candidate)"
				}
				fmt.Printf("[%.3f] %s ↔ %s%s\n", b.Similarity, b.A.Slug, b.B.Slug, mark)
				fmt.Printf("  A: %s\n  B: %s\n", b.A.Title, b.B.Title)
			}
			if !peek {
				fmt.Println("(pairs recorded — dismiss false positives with `canopy bridge --dismiss a:b`)")
			}
			return nil
		},
	}
	c.Flags().IntVarP(&n, "count", "n", 5, "max pairs")
	c.Flags().Float64Var(&minSim, "min-sim", 0.70, "cosine similarity threshold")
	c.Flags().BoolVar(&peek, "peek", false, "don't record pairs in state (preview only)")
	c.Flags().BoolVar(&includeLinked, "include-linked", false, "also return linked pairs (semantic-lint candidates; raise --min-sim to ~0.85)")
	c.Flags().StringVar(&dismiss, "dismiss", "", "permanently dismiss a pair: slug-a:slug-b")
	return c
}
