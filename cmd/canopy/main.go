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
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/buildinfo"
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/gitops"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/lint"
	"github.com/neutrospec/canopy/internal/migrate"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/search"
	"github.com/neutrospec/canopy/internal/skills"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/neutrospec/canopy/internal/wiki"
)

var (
	flagWiki string
	flagJSON bool
)

func main() {
	root := &cobra.Command{
		Use:           "canopy",
		Short:         "Manage an LLM wiki: schema-enforced writes, hybrid search, sync",
		Version:       buildinfo.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flagWiki, "wiki", "", "wiki root (default: $CANOPY_WIKI or canopy.toml discovery)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")

	// Bring durable on-disk state up to the binary's schema before any command
	// that touches it — "migrate before doing anything" on a fresh upgrade.
	// version/migrate/help/completion manage or inspect the schema themselves,
	// so they are exempt and stay usable to recover from a failed migration.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		switch p := cmd.CommandPath(); {
		case p == "canopy", p == "canopy version", p == "canopy help",
			strings.HasPrefix(p, "canopy migrate"), strings.HasPrefix(p, "canopy completion"):
			return nil
		}
		_, err := migrate.Ensure(migrateCtx(), buildinfo.Version())
		return err
	}

	root.AddCommand(cmdInit(), cmdStatus(), cmdReindex(), cmdSearch(), cmdBacklinks(), cmdLint(), cmdShow(), cmdList(), cmdTags(), cmdModel(),
		cmdNew(), cmdUpdate(), cmdMv(), cmdRm(), cmdArchive(), cmdSync(), cmdSkills(),
		cmdResurface(), cmdBridge(), cmdRecall(), cmdDigest(), cmdServe(),
		cmdVersion(), cmdMigrate(), cmdReconcile(), cmdTasks(), cmdEvents())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func loadWiki() (*config.Wiki, error) {
	w, err := config.Resolve(flagWiki)
	if err != nil {
		return nil, err
	}
	// Apply the wiki's model choice here, before any embed.* call:
	// ModelAvailable() gates and New() must agree on the directory.
	embed.SetModelDir(w.Cfg.Embedding.Model)
	return w, nil
}

// migrateCtx builds the migration runner's view of the machine-local XDG
// directories. Migrations self-report concrete actions to stderr; in --json
// mode they stay silent so machine-readable output is never polluted.
func migrateCtx() *migrate.Context {
	logf := func(format string, args ...any) {
		if !flagJSON {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	return &migrate.Context{
		ConfigHome: config.ConfigHome(),
		CacheHome:  config.CacheHome(),
		DataHome:   config.DataHome(),
		Log:        logf,
	}
}

// banner prints the unsynced-state warning to stderr on every command,
// so a forgotten `canopy sync` is impossible to miss. When the reconcile
// gate is on, unjudged back-door changes surface the same way (K6) —
// noise-free hash walk, silent until the gate is opted into.
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
	if n, ok, err := reconcile.Count(w); err == nil && ok && n > 0 {
		fmt.Fprintf(os.Stderr, "⚠ 미정규화 외부 변경 %d건 — `canopy reconcile`로 검토\n", n)
	}
	// Pending delegated tasks surface on every command (invariant T7,
	// 원칙 5) — an agent session that sees the banner can run the loop.
	if n, err := tasks.PendingCount(w); err == nil && n > 0 {
		fmt.Fprintf(os.Stderr, "⚑ 위임 태스크 %d건 대기 — `canopy tasks list`로 확인\n", n)
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
// stderr. Warm loads of the int8 model take ~0.5s, but a cold OS page
// cache (first run after boot) can take several seconds, so report the
// measured time instead of promising one.
func newEngine() (embed.Engine, error) {
	if flagJSON {
		return embed.New()
	}
	fmt.Fprint(os.Stderr, "loading embedding model… ")
	start := time.Now()
	eng, err := embed.New()
	if err != nil {
		fmt.Fprintln(os.Stderr)
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "%.1fs\n", time.Since(start).Seconds())
	return eng, nil
}

func cmdInit() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Adopt a wiki: write canopy.toml (schema) and build the index cache",
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
			st, scan, err := refreshIndex(w, nil)
			if err != nil {
				return err
			}
			defer st.Close()
			if flagJSON {
				return emitJSON(map[string]any{"root": w.Root, "pages": len(scan.Pages), "db": w.DBPath()})
			}
			fmt.Printf("✓ initialized %s\n", w.Root)
			fmt.Printf("  canopy.toml written, %d pages indexed → %s\n", len(scan.Pages), w.DBPath())
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite existing canopy.toml")
	return c
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
			recN, recOn, _ := reconcile.Count(w)
			taskN, _ := tasks.PendingCount(w)
			if flagJSON {
				out := map[string]any{
					"root":          w.Root,
					"pages":         len(scan.Pages),
					"stray_root":    scan.StrayRoot,
					"git":           git,
					"initialized":   w.HasTOML,
					"tasks_pending": taskN,
				}
				if recOn {
					out["unreconciled"] = recN
				}
				return emitJSON(out)
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
			if recOn && recN > 0 {
				fmt.Printf("⚠ 미정규화 외부 변경 %d건 — `canopy reconcile`로 검토\n", recN)
			}
			if taskN > 0 {
				fmt.Printf("⚑ 위임 태스크 %d건 대기 — `canopy tasks list`로 확인\n", taskN)
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
			if !noEmbed && embed.Available() && embed.ModelAvailable() {
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
	var noMark bool
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
			if !noMark {
				// The query is an event (search→read trail); hits are
				// exposure and never marked (H5). Unanswered queries are
				// demand — same gap file as the web (H10).
				attention.LogSearchEvent(w, attention.DoorAgent, query)
				kwEmpty := mode != "semantic" && len(kw) == 0
				if len(hits) == 0 || kwEmpty {
					top := 0.0
					if len(hits) > 0 {
						top = hits[0].Score
					}
					attention.LogGap(w, attention.DoorAgent, query, len(hits), top)
				}
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
	c.Flags().BoolVar(&noMark, "no-mark", false, "search without recording the query event/gap")
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
		Short: "Download bge-m3 ONNX (~2.3GB) to $XDG_DATA_HOME/canopy/models",
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
			if !embed.Available() {
				fmt.Println("note: this binary lacks the ORT backend — rebuild with `make build` (-tags ORT)")
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show embedding stack status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Best-effort: inside a wiki, report the configured model dir
			// (loadWiki applies it); outside one, fall back to the default.
			_, _ = loadWiki()
			if flagJSON {
				return emitJSON(map[string]any{
					"ort_available":   embed.Available(),
					"model_available": embed.ModelAvailable(),
					"model_path":      embed.DefaultModelPath(),
				})
			}
			fmt.Printf("ORT backend available: %v\n", embed.Available())
			fmt.Printf("model downloaded:      %v (%s)\n", embed.ModelAvailable(), embed.DefaultModelPath())
			return nil
		},
	})
	return c
}

func cmdSkills() *cobra.Command {
	var dir string
	c := &cobra.Command{Use: "skills", Short: "Manage the agent skill set for this wiki (hermes, Claude Code, …)"}
	install := &cobra.Command{
		Use:   "install",
		Short: "Install/refresh the canopy-wiki / canopy-ingest skills for every detected agent",
		Long: `Installs the two embedded skills into every known agent skills
directory that exists (~/.hermes/skills, ~/.claude/skills), so one
command after a canopy upgrade refreshes every agent. Use --dir to
target a single directory (it is created if missing).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// --dir targets exactly one directory (and can bootstrap it);
			// the default sweeps every detected agent.
			byDir := map[string][]string{}
			if dir != "" {
				written, err := skills.Install(dir)
				if err != nil {
					return err
				}
				byDir[dir] = written
			} else {
				var err error
				if byDir, err = skills.InstallAll(); err != nil {
					return err
				}
			}
			supersededByDir := map[string][]string{}
			for d := range byDir {
				if present := skills.SupersededPresent(d); len(present) > 0 {
					supersededByDir[d] = present
				}
			}
			if flagJSON {
				return emitJSON(map[string]any{"written": byDir, "superseded_present": supersededByDir})
			}
			for d, written := range byDir {
				for _, p := range written {
					fmt.Println("✓", p)
				}
				if hint := skills.RemovalHint(d, supersededByDir[d]); hint != "" {
					fmt.Print(hint)
				}
			}
			return nil
		},
	}
	install.Flags().StringVar(&dir, "dir", "", "install into this directory only (default: every existing known dir)")
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
	var noMark bool
	c := &cobra.Command{
		Use: "show <page>",
		// "view" is what agents reach for first; make it just work.
		Aliases: []string{"view", "cat"},
		Short:   "Print a page (path header on stderr, content on stdout)",
		Args:    cobra.ExactArgs(1),
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
			if !noMark {
				touchAttention(w, []string{p.Slug}, attention.KindShow, "")
			}
			if flagJSON {
				return emitJSON(map[string]any{"rel_path": p.RelPath, "content": string(data)})
			}
			fmt.Fprintf(os.Stderr, "— %s —\n", p.RelPath)
			fmt.Print(string(data))
			return nil
		},
	}
	c.Flags().BoolVar(&noMark, "no-mark", false, "read without recording an access (like --peek)")
	return c
}

// touchAttention records agent-door consumption, best-effort: a failed
// mark warns on stderr but never breaks the read that triggered it.
func touchAttention(w *config.Wiki, slugs []string, kind, meta string) {
	if err := attention.Touch(w, slugs, kind, meta); err != nil && !flagJSON {
		fmt.Fprintf(os.Stderr, "attention: %v\n", err)
	}
}

func cmdList() *cobra.Command {
	var typ, tag string
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all pages (slug, type, title), filterable by --type / --tag",
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
			type row struct {
				Slug    string   `json:"slug"`
				Type    string   `json:"type"`
				Title   string   `json:"title"`
				Tags    []string `json:"tags"`
				Updated string   `json:"updated"`
				RelPath string   `json:"rel_path"`
			}
			rows := []row{}
			for _, p := range scan.Pages {
				if typ != "" && p.Type != typ {
					continue
				}
				if tag != "" && !contains(p.Tags, tag) {
					continue
				}
				rows = append(rows, row{p.Slug, p.Type, p.Title, p.Tags, p.Updated, p.RelPath})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].RelPath < rows[j].RelPath })
			if flagJSON {
				return emitJSON(map[string]any{"pages": rows, "count": len(rows)})
			}
			for _, r := range rows {
				fmt.Printf("%-32s %-11s %s\n", r.Slug, r.Type, r.Title)
			}
			fmt.Fprintf(os.Stderr, "%d page(s)\n", len(rows))
			return nil
		},
	}
	c.Flags().StringVar(&typ, "type", "", "filter by type (entity|concept|comparison)")
	c.Flags().StringVar(&tag, "tag", "", "filter by tag")
	return c
}

// cmdTags exposes the taxonomy that validation enforces, from the same
// source (canopy.toml / defaults) — no need to parse the TOML by hand.
// --audit measures the taxonomy against actual usage (invariants S3–S5):
// report only, no side effects.
func cmdTags() *cobra.Command {
	var audit bool
	c := &cobra.Command{
		Use:   "tags",
		Short: "Show the valid types and tag taxonomy; --audit measures usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			if audit {
				scan, err := wiki.Scan(w)
				if err != nil {
					return err
				}
				rep := lint.TagAudit(w, scan)
				if flagJSON {
					return emitJSON(rep)
				}
				printTagAudit(rep)
				return nil
			}
			if flagJSON {
				return emitJSON(map[string]any{
					"types":  w.Cfg.Schema.Types,
					"topics": w.Cfg.Schema.Topics,
					"forms":  w.Cfg.Schema.Forms,
					// union, same source validation enforces (A7/S2);
					// also where a legacy single-list taxonomy shows up
					"tags": w.Cfg.Schema.AllTags(),
				})
			}
			fmt.Printf("types:  %s\n", strings.Join(w.Cfg.Schema.Types, ", "))
			if w.LegacyTags {
				fmt.Printf("tags:   %s\n", strings.Join(w.Cfg.Schema.Tags, ", "))
				fmt.Fprintln(os.Stderr, "(legacy single-list taxonomy — split into topics/forms in canopy.toml; see `canopy tags --audit`)")
			} else {
				fmt.Printf("topics: %s\n", strings.Join(w.Cfg.Schema.Topics, ", "))
				fmt.Printf("forms:  %s\n", strings.Join(w.Cfg.Schema.Forms, ", "))
			}
			fmt.Fprintln(os.Stderr, "(source: canopy.toml — a new topic needs ≥3 pages demanding it; forms are frozen)")
			return nil
		},
	}
	c.Flags().BoolVar(&audit, "audit", false, "measure taxonomy against usage: unused/overbroad topics, unknown tags")
	return c
}

func printTagAudit(rep *lint.TagAuditReport) {
	fmt.Printf("pages: %d\n", rep.TotalPages)
	if rep.Legacy {
		fmt.Println("⚠ legacy single-list taxonomy — split canopy.toml `tags` into `topics`/`forms` (S1)")
	}
	section := func(title string, us []lint.TagUsage) {
		fmt.Printf("\n%s:\n", title)
		for _, u := range us {
			fmt.Printf("  %4d  %4.1f%%  %s\n", u.Count, u.Pct, u.Tag)
		}
	}
	section("topics", rep.Topics)
	section("forms", rep.Forms)
	if len(rep.UnusedTopics) > 0 {
		fmt.Printf("\n⚠ unused topics (reclaim candidates, S3): %s\n", strings.Join(rep.UnusedTopics, ", "))
	}
	for _, u := range rep.OverbroadTopics {
		fmt.Printf("⚠ overbroad topic (>%d%% of pages — consider splitting, S4): %s (%d pages, %.1f%%)\n",
			rep.BroadTopicPct, u.Tag, u.Count, u.Pct)
	}
	if len(rep.UnknownTags) > 0 {
		var names []string
		for _, u := range rep.UnknownTags {
			names = append(names, u.Tag)
		}
		fmt.Printf("⚠ tags on pages but not in taxonomy (lint A5): %s\n", strings.Join(names, ", "))
	}
	if len(rep.UnusedTopics) == 0 && len(rep.OverbroadTopics) == 0 && len(rep.UnknownTags) == 0 && !rep.Legacy {
		fmt.Println("\n✓ taxonomy matches usage")
	}
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, build metadata, and the data-schema version",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := migrate.Status(migrateCtx())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{
					"version":        buildinfo.Version(),
					"commit":         buildinfo.Commit(),
					"date":           buildinfo.Date(),
					"go":             buildinfo.GoVersion(),
					"schema_version": rep.Current,
					"schema_target":  rep.Target,
					"cache_schema":   store.SchemaVersion,
					"semantic":       embed.Available(),
				})
			}
			fmt.Printf("canopy %s\n", buildinfo.Version())
			fmt.Printf("  commit    %s\n", buildinfo.Commit())
			fmt.Printf("  built     %s\n", buildinfo.Date())
			fmt.Printf("  go        %s\n", buildinfo.GoVersion())
			fmt.Printf("  schema    %d/%d\n", rep.Current, rep.Target)
			fmt.Printf("  semantic  %v\n", embed.Available())
			return nil
		},
	}
}

func cmdMigrate() *cobra.Command {
	c := &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending data-state migrations (also runs automatically on first use)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := migrateCtx()
			res, err := migrate.Ensure(ctx, buildinfo.Version())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(res)
			}
			if len(res.Applied) == 0 {
				fmt.Printf("✓ up to date (schema %d/%d)\n", res.To, migrate.Target())
				return nil
			}
			fmt.Printf("✓ migrated schema %d → %d\n", res.From, res.To)
			for _, name := range res.Applied {
				fmt.Printf("  • %s\n", name)
			}
			return nil
		},
	}
	c.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show the current and target data-schema version and any pending steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := migrate.Status(migrateCtx())
			if err != nil {
				return err
			}
			if flagJSON {
				pending := []map[string]any{}
				for _, m := range rep.Pending {
					pending = append(pending, map[string]any{"to": m.To, "name": m.Name})
				}
				return emitJSON(map[string]any{
					"current":   rep.Current,
					"target":    rep.Target,
					"persisted": rep.Persisted,
					"pending":   pending,
				})
			}
			fmt.Printf("schema %d/%d\n", rep.Current, rep.Target)
			if len(rep.Pending) == 0 {
				fmt.Println("✓ no pending migrations")
				return nil
			}
			fmt.Println("pending:")
			for _, m := range rep.Pending {
				fmt.Printf("  → %d  %s\n", m.To, m.Name)
			}
			return nil
		},
	})
	return c
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
