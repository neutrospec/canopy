// recall & digest: the agent-memory read path and the Express material
// collector. Both return raw material; judgment and prose are the
// agent's job (docs/second-brain.md).
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nobocop/canopy/internal/digest"
	"github.com/nobocop/canopy/internal/wiki"
)

func cmdRecall() *cobra.Command {
	var topK, perPage int
	c := &cobra.Command{
		Use:   "recall <question>",
		Short: "Chunk-level evidence for a question (agent memory: verbatim chunks + source slugs)",
		Long: `Unlike search (which ranks pages), recall returns the actual chunks
so an agent can inject them into context and cite [[slug]] sources.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
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
			st, _, err := refreshIndex(w, eng)
			if err != nil {
				return err
			}
			defer st.Close()
			qv, err := eng.Embed([]string{query})
			if err != nil {
				return err
			}
			chunks, err := st.SearchChunks(qv[0], topK, perPage)
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"query": query, "chunks": chunks})
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
