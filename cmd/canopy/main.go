// canopy — LLM wiki manager. Enforces the wiki schema in code so agents
// (and humans) don't have to follow prose checklists.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nobocop/canopy/internal/config"
	"github.com/nobocop/canopy/internal/gitops"
	"github.com/nobocop/canopy/internal/lint"
	"github.com/nobocop/canopy/internal/store"
	"github.com/nobocop/canopy/internal/wiki"
)

var (
	flagWiki string
	flagJSON bool
)

func main() {
	root := &cobra.Command{
		Use:           "canopy",
		Short:         "Manage an LLM wiki: schema-enforced writes, hybrid search, sync",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagWiki, "wiki", "", "wiki root (default: $CANOPY_WIKI or canopy.toml discovery)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")

	root.AddCommand(cmdInit(), cmdStatus(), cmdReindex(), cmdSearch(), cmdBacklinks(), cmdLint(), cmdShow())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func loadWiki() (*config.Wiki, error) {
	return config.Resolve(flagWiki)
}

// banner prints the unsynced-state warning to stderr on every command,
// so a forgotten `canopy sync` is impossible to miss.
func banner(w *config.Wiki) {
	if flagJSON {
		return
	}
	st, err := gitops.GetStatus(w.Root)
	if err != nil {
		return
	}
	if b := st.Banner(); b != "" {
		fmt.Fprintln(os.Stderr, b)
	}
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// refreshIndex rescans the wiki and rebuilds page metadata + FTS.
// Cheap at current scale (~250 pages), so read commands always run it
// and can never serve stale keyword results.
func refreshIndex(w *config.Wiki) (*store.Store, *wiki.ScanResult, error) {
	scan, err := wiki.Scan(w)
	if err != nil {
		return nil, nil, err
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return nil, nil, err
	}
	if err := st.RebuildPages(scan.Pages); err != nil {
		st.Close()
		return nil, nil, err
	}
	return st, scan, nil
}

func cmdInit() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Adopt a wiki: write canopy.toml, prepare .canopy/, build the index",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			if w.HasTOML && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", w.TOMLPath())
			}
			if err := w.WriteTOML(); err != nil {
				return err
			}
			if err := ensureGitignore(w); err != nil {
				return err
			}
			st, scan, err := refreshIndex(w)
			if err != nil {
				return err
			}
			defer st.Close()
			if flagJSON {
				return emitJSON(map[string]any{"root": w.Root, "pages": len(scan.Pages)})
			}
			fmt.Printf("✓ initialized %s\n", w.Root)
			fmt.Printf("  canopy.toml written, .canopy/ gitignored, %d pages indexed\n", len(scan.Pages))
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite existing canopy.toml")
	return c
}

// ensureGitignore keeps the derived .canopy/ cache out of the wiki repo.
func ensureGitignore(w *config.Wiki) error {
	path := filepath.Join(w.Root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".canopy/" {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + ".canopy/\n")
	return err
}

func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Wiki health at a glance: pages, git state, index freshness",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			git, err := gitops.GetStatus(w.Root)
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{
					"root":       w.Root,
					"pages":      len(scan.Pages),
					"stray_root": scan.StrayRoot,
					"git":        git,
					"initialized": w.HasTOML,
				})
			}
			fmt.Printf("wiki:  %s\n", w.Root)
			fmt.Printf("pages: %d", len(scan.Pages))
			byDir := map[string]int{}
			for _, p := range scan.Pages {
				byDir[p.Dir]++
			}
			var parts []string
			for _, d := range w.Cfg.Schema.PageDirs {
				parts = append(parts, fmt.Sprintf("%s %d", d, byDir[d]))
			}
			fmt.Printf(" (%s)\n", strings.Join(parts, ", "))
			if !w.HasTOML {
				fmt.Println("init:  not adopted yet — run `canopy init`")
			}
			if git.IsRepo {
				fmt.Printf("git:   branch %s, %d dirty, %d ahead, %d behind\n", git.Branch, git.Dirty, git.Ahead, git.Behind)
				if b := git.Banner(); b != "" {
					fmt.Println(b)
				} else {
					fmt.Println("✓ fully synced")
				}
			}
			return nil
		},
	}
}

func cmdReindex() *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the derived index (pages + FTS; embeddings in M2)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			st, scan, err := refreshIndex(w)
			if err != nil {
				return err
			}
			defer st.Close()
			if flagJSON {
				return emitJSON(map[string]any{"pages": len(scan.Pages)})
			}
			fmt.Printf("✓ indexed %d pages → %s\n", len(scan.Pages), w.DBPath())
			return nil
		},
	}
}

func cmdSearch() *cobra.Command {
	var mode string
	var topK int
	c := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the wiki (keyword now; semantic/hybrid arrive with M2)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			switch mode {
			case "keyword":
			case "semantic", "hybrid":
				return fmt.Errorf("mode %q requires the embedding index (M2); use --mode keyword for now", mode)
			default:
				return fmt.Errorf("unknown mode %q", mode)
			}
			st, _, err := refreshIndex(w)
			if err != nil {
				return err
			}
			defer st.Close()
			hits, err := st.SearchKeyword(query, topK)
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"query": query, "mode": mode, "hits": hits})
			}
			if len(hits) == 0 {
				fmt.Println("no results")
				return nil
			}
			for i, h := range hits {
				fmt.Printf("%2d. [%.2f] %s — %s\n", i+1, h.Score, h.Slug, h.Title)
				if h.Snippet != "" {
					fmt.Printf("      %s\n", strings.ReplaceAll(h.Snippet, "\n", " "))
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&mode, "mode", "keyword", "keyword|semantic|hybrid")
	c.Flags().IntVarP(&topK, "top-k", "k", 10, "number of results")
	return c
}

func cmdBacklinks() *cobra.Command {
	var orphans bool
	c := &cobra.Command{
		Use:   "backlinks [page]",
		Short: "Show pages linking to a page, or list orphans with --orphans",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			in := scan.Backlinks()
			if orphans {
				var list []string
				for _, p := range scan.Pages {
					if len(in[strings.ToLower(p.Slug)]) == 0 {
						list = append(list, p.RelPath)
					}
				}
				if flagJSON {
					return emitJSON(map[string]any{"orphans": list})
				}
				for _, p := range list {
					fmt.Println(p)
				}
				fmt.Fprintf(os.Stderr, "%d orphan(s)\n", len(list))
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("usage: canopy backlinks <page> (or --orphans)")
			}
			slug := wiki.NormalizeLink(args[0])
			p, ok := scan.BySlug[slug]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			sources := in[slug]
			if flagJSON {
				return emitJSON(map[string]any{"page": p.RelPath, "backlinks": sources})
			}
			fmt.Printf("%s ← %d backlink(s)\n", p.RelPath, len(sources))
			for _, s := range sources {
				fmt.Println("  " + s)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&orphans, "orphans", false, "list pages with no inbound links")
	return c
}

func cmdLint() *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Check schema compliance, links, staleness (report only for now)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			rep := lint.Run(w, scan)
			if flagJSON {
				return emitJSON(rep)
			}
			fmt.Printf("lint: %d pages checked\n", rep.TotalPages)
			if len(rep.Findings) == 0 {
				fmt.Println("✓ clean")
				return nil
			}
			cur := lint.Severity("")
			for _, f := range rep.Findings {
				if f.Severity != cur {
					cur = f.Severity
					fmt.Printf("\n%s\n", strings.ToUpper(string(cur)))
				}
				fmt.Printf("  [%s] %s: %s\n", f.Kind, f.Page, f.Message)
			}
			fmt.Println()
			for kind, n := range rep.Counts {
				fmt.Printf("  %-20s %d\n", kind, n)
			}
			return nil
		},
	}
}

func cmdShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <page>",
		Short: "Print a page (path + content)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			data, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"rel_path": p.RelPath, "content": string(data)})
			}
			fmt.Fprintf(os.Stderr, "— %s —\n", p.RelPath)
			fmt.Print(string(data))
			return nil
		},
	}
}
