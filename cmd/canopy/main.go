// canopy — LLM wiki manager. Enforces the wiki schema in code so agents
// (and humans) don't have to follow prose checklists.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nobocop/canopy/internal/config"
	"github.com/nobocop/canopy/internal/embed"
	"github.com/nobocop/canopy/internal/gitops"
	"github.com/nobocop/canopy/internal/indexer"
	"github.com/nobocop/canopy/internal/lint"
	"github.com/nobocop/canopy/internal/search"
	"github.com/nobocop/canopy/internal/skills"
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

	root.AddCommand(cmdInit(), cmdStatus(), cmdReindex(), cmdSearch(), cmdBacklinks(), cmdLint(), cmdShow(), cmdModel(),
		cmdNew(), cmdUpdate(), cmdMv(), cmdRm(), cmdArchive(), cmdSync(), cmdSkills())

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
// and can never serve stale keyword results. Vector chunks are only
// refreshed when an engine is passed (they cost model inference).
func refreshIndex(w *config.Wiki, eng embed.Engine) (*store.Store, *wiki.ScanResult, error) {
	scan, err := wiki.Scan(w)
	if err != nil {
		return nil, nil, err
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return nil, nil, err
	}
	progress := func(s string) {
		if !flagJSON {
			fmt.Fprintln(os.Stderr, "  "+s)
		}
	}
	if _, err := indexer.Reindex(w, st, scan, eng, progress); err != nil {
		st.Close()
		return nil, nil, err
	}
	return st, scan, nil
}

// newEngine loads the in-process embedding model, with a heads-up on
// stderr because the fp32 model takes ~10s to load.
func newEngine() (embed.Engine, error) {
	if !flagJSON {
		fmt.Fprintln(os.Stderr, "loading embedding model (~10s)…")
	}
	return embed.New()
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
			st, scan, err := refreshIndex(w, nil)
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
					"root":        w.Root,
					"pages":       len(scan.Pages),
					"stray_root":  scan.StrayRoot,
					"git":         git,
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
	var noEmbed bool
	c := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the derived index (pages, FTS, and embeddings)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			var eng embed.Engine
			if !noEmbed && embed.Available && embed.ModelAvailable() {
				if eng, err = newEngine(); err != nil {
					return err
				}
				defer eng.Close()
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			st, err := store.Open(w.DBPath())
			if err != nil {
				return err
			}
			defer st.Close()
			progress := func(s string) {
				if !flagJSON {
					fmt.Fprintln(os.Stderr, "  "+s)
				}
			}
			res, err := indexer.Reindex(w, st, scan, eng, progress)
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(res)
			}
			fmt.Printf("✓ indexed %d pages → %s\n", res.Pages, w.DBPath())
			if eng != nil {
				fmt.Printf("  embeddings: %d page(s) refreshed, %d pruned, %d chunks total\n", res.Embedded, res.Pruned, res.TotalChunks)
			} else if !noEmbed {
				fmt.Println("  embeddings skipped (model or ORT build missing — see `canopy model pull`)")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&noEmbed, "no-embed", false, "skip embedding refresh")
	return c
}

func cmdSearch() *cobra.Command {
	var mode string
	var topK int
	c := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the wiki (hybrid = BM25 keyword + semantic vectors)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			switch mode {
			case "keyword", "semantic", "hybrid":
			default:
				return fmt.Errorf("unknown mode %q", mode)
			}

			// Hybrid degrades to keyword when the embedding stack is
			// missing; explicit --mode semantic fails loudly instead.
			var eng embed.Engine
			if mode != "keyword" {
				eng, err = newEngine()
				if err != nil {
					if mode == "semantic" {
						return err
					}
					fmt.Fprintf(os.Stderr, "hybrid → keyword only (%v)\n", err)
					mode = "keyword"
				} else {
					defer eng.Close()
				}
			}

			st, _, err := refreshIndex(w, eng)
			if err != nil {
				return err
			}
			defer st.Close()

			var hits []store.Hit
			var kw, sem []store.Hit
			if mode == "keyword" || mode == "hybrid" {
				if kw, err = st.SearchKeyword(query, topK); err != nil {
					return err
				}
			}
			if mode == "semantic" || mode == "hybrid" {
				qv, err := eng.Embed([]string{query})
				if err != nil {
					return err
				}
				if sem, err = st.SearchSemantic(qv[0], topK); err != nil {
					return err
				}
			}
			switch mode {
			case "keyword":
				hits = kw
			case "semantic":
				hits = sem
			case "hybrid":
				hits = search.Fuse(topK, kw, sem)
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
	c.Flags().StringVar(&mode, "mode", "hybrid", "keyword|semantic|hybrid")
	c.Flags().IntVarP(&topK, "top-k", "k", 10, "number of results")
	return c
}

// modelFiles are fetched from Hugging Face by `canopy model pull`.
// EmbeddedLLM/bge-m3-onnx-o2-cpu: fp32 CPU-optimized bge-m3 whose fused
// ops require the ONNX Runtime backend (hence the ORT build tag).
const modelRepo = "EmbeddedLLM/bge-m3-onnx-o2-cpu"

var modelFiles = []string{
	"model.onnx", "model.onnx.data", "config.json",
	"tokenizer.json", "tokenizer_config.json", "special_tokens_map.json",
}

func cmdModel() *cobra.Command {
	c := &cobra.Command{Use: "model", Short: "Manage the local embedding model"}
	c.AddCommand(&cobra.Command{
		Use:   "pull",
		Short: "Download bge-m3 ONNX (~2.3GB) to ~/.canopy/models",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := embed.DefaultModelPath()
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, f := range modelFiles {
				dst := filepath.Join(dir, f)
				if _, err := os.Stat(dst); err == nil {
					fmt.Printf("  %s (cached)\n", f)
					continue
				}
				fmt.Printf("  %s …\n", f)
				if err := download("https://huggingface.co/"+modelRepo+"/resolve/main/"+f, dst); err != nil {
					return fmt.Errorf("%s: %w", f, err)
				}
			}
			fmt.Println("✓ model ready:", dir)
			if !embed.Available {
				fmt.Println("note: this binary lacks the ORT backend — rebuild with `make build` (-tags ORT)")
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show embedding stack status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagJSON {
				return emitJSON(map[string]any{
					"ort_build":       embed.Available,
					"model_available": embed.ModelAvailable(),
					"model_path":      embed.DefaultModelPath(),
				})
			}
			fmt.Printf("ORT backend in binary: %v\n", embed.Available)
			fmt.Printf("model downloaded:      %v (%s)\n", embed.ModelAvailable(), embed.DefaultModelPath())
			return nil
		},
	})
	return c
}

func cmdSkills() *cobra.Command {
	var dir string
	c := &cobra.Command{Use: "skills", Short: "Manage the hermes skill set for this wiki"}
	install := &cobra.Command{
		Use:   "install",
		Short: "Write the canopy-wiki / canopy-ingest skills into the hermes skills directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if dir == "" {
				if dir, err = skills.DefaultSkillsDir(); err != nil {
					return err
				}
			}
			written, err := skills.Install(dir)
			if err != nil {
				return err
			}
			present := skills.SupersededPresent(dir)
			if flagJSON {
				return emitJSON(map[string]any{"written": written, "superseded_present": present})
			}
			for _, p := range written {
				fmt.Println("✓", p)
			}
			if hint := skills.RemovalHint(dir, present); hint != "" {
				fmt.Print(hint)
			}
			return nil
		},
	}
	install.Flags().StringVar(&dir, "dir", "", "skills directory (default ~/.hermes/skills)")
	c.AddCommand(install)
	return c
}

func download(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
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
