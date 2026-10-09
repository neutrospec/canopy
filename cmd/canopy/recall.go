// recall & digest: the agent-memory read path and the Express material
// collector. Both return raw material; judgment and prose are the
// agent's job (docs/second-brain.md).
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/digest"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

func cmdRecall() *cobra.Command {
	var topK, perPage int
	var noMark bool
	c := &cobra.Command{
		Use:   "recall <question>",
		Short: "Chunk-level evidence for a question (agent memory: verbatim chunks + source slugs)",
		Long: `Unlike search (which ranks pages), recall returns the actual chunks
so an agent can inject them into context and cite [[slug]] sources.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.ValidateK(topK); err != nil {
				return err
			}
			if perPage < 0 || perPage > store.MaxK {
				return fmt.Errorf("per-page must be between 0 and %d", store.MaxK)
			}
			query := strings.Join(args, " ")
			if len([]rune(query)) > embed.MaxTextRunes {
				return fmt.Errorf("query exceeds %d runes", embed.MaxTextRunes)
			}
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			eng, err := newEngine()
			if err != nil {
				return fmt.Errorf("recall needs the embedding stack: %w", err)
			}
			defer eng.Close()
			st, _, err := refreshIndexContext(cmd.Context(), w, eng)
			if err != nil {
				return err
			}
			defer st.Close()
			qv, err := embed.EmbedContext(cmd.Context(), eng, []string{query})
			if err != nil {
				return err
			}
			chunks, err := st.SearchChunks(qv[0], topK, perPage)
			if err != nil {
				return err
			}
			// Returned chunks are consumption, not mere exposure: their
			// text enters the agent's context and gets cited as [[slug]].
			if !noMark && len(chunks) > 0 {
				seen := map[string]bool{}
				var slugs []string
				for _, h := range chunks {
					k := strings.ToLower(h.Slug)
					if !seen[k] {
						seen[k] = true
						slugs = append(slugs, h.Slug)
					}
				}
				touchAttention(w, slugs, attention.KindRecall, query)
			}
			if flagJSON {
				return emitJSON(map[string]any{"query": query, "mode": "semantic", "chunks": chunks})
			}
			if len(chunks) == 0 {
				fmt.Println("no indexed chunks — run `canopy reindex`")
				return nil
			}
			for i, h := range chunks {
				fmt.Printf("── %d. [%.3f] [[%s]] (%s #%d)\n%s\n\n", i+1, h.Score, h.Slug, h.RelPath, h.Seq, h.Text)
			}
			return nil
		},
	}
	c.Flags().IntVarP(&topK, "top-k", "k", 6, "number of chunks")
	c.Flags().IntVar(&perPage, "per-page", 2, "max chunks per page (0 = unlimited)")
	c.Flags().BoolVar(&noMark, "no-mark", false, "read without recording an access (like --peek)")
	return c
}

func cmdDigest() *cobra.Command {
	var since string
	c := &cobra.Command{
		Use:   "digest",
		Short: "Collect review material for a window: created/updated pages, tags, decision timeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			cutoff, err := digest.ParseSince(since, time.Now())
			if err != nil {
				return err
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			res := digest.Collect(scan, cutoff)
			// Attach consumption stats from the local event log (G8),
			// best-effort: no DB yet just means no section. Slugs whose
			// page no longer exists are dropped — 실존 페이지만.
			if ev, err := attention.Open(w.AttentionDBPath()); err == nil {
				if top, err := ev.TopConsumed(cutoff, 10); err == nil {
					for _, t := range top {
						p, ok := scan.BySlug[t.Slug]
						if !ok {
							continue
						}
						res.TopConsumed = append(res.TopConsumed, digest.Consumed{
							Slug: t.Slug, Title: p.Title,
							Events: t.Total, Web: t.Web, Agent: t.Agent,
						})
					}
				}
				ev.Close()
			}
			if flagJSON {
				return emitJSON(res)
			}
			fmt.Print(res.Render())
			return nil
		},
	}
	c.Flags().StringVar(&since, "since", "90d", "window start: 90d, 12w, 3m, or YYYY-MM-DD")
	return c
}
