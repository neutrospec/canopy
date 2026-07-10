// Write-path commands: new, update, mv, rm, archive, sync.
//
// Every write runs the same pipeline afterwards — regenerate indexes,
// append the JSONL log, refresh the derived DB — so the invariants the
// old prose checklists tried to enforce hold by construction. Commits
// are deliberately NOT automatic: batching related writes into one
// commit is the user's choice, so sync stays a separate command (or
// --sync on the write itself for one-shot work).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/genindex"
	"github.com/neutrospec/canopy/internal/gitops"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

var typeDirs = map[string]string{
	"entity":     "entities",
	"concept":    "concepts",
	"comparison": "comparisons",
}

func dirForType(w *config.Wiki, typ string) (string, error) {
	dir, ok := typeDirs[typ]
	if !ok {
		return "", fmt.Errorf("unknown type %q (valid: %s)", typ, strings.Join(w.Cfg.Schema.Types, "|"))
	}
	return dir, nil
}

func validateTags(w *config.Wiki, tags []string) error {
	allowed := map[string]bool{}
	for _, t := range w.Cfg.Schema.Tags {
		allowed[t] = true
	}
	var bad []string
	for _, t := range tags {
		if !allowed[t] {
			bad = append(bad, t)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("tags not in taxonomy: %s (see `canopy tags` for the valid list; extend canopy.toml first if genuinely new)", strings.Join(bad, ", "))
	}
	return nil
}

// relatedThreshold is the minimum page-level cosine for a "related
// pages" suggestion after `canopy new`. bge-m3 page vectors have a high
// similarity floor, so 0.80 trims only the clearly-unrelated tail;
// tag-overlap ordering does the real de-noising.
const relatedThreshold = 0.80

// rankRelated filters semantic hits for the related-pages suggestion:
// drop the new page itself and sub-threshold hits, then order by shared
// frontmatter tags first (score decides ties — hits arrive score-desc).
func rankRelated(hits []store.Hit, selfSlug string, newTags []string, scan *wiki.ScanResult) []store.Hit {
	tagSet := map[string]bool{}
	for _, t := range newTags {
		tagSet[t] = true
	}
	overlap := func(slug string) int {
		p, ok := scan.BySlug[strings.ToLower(slug)]
		if !ok {
			return 0
		}
		n := 0
		for _, t := range p.Tags {
			if tagSet[t] {
				n++
			}
		}
		return n
	}
	related := []store.Hit{}
	for _, h := range hits {
		if h.Slug == selfSlug || h.Score < relatedThreshold {
			continue
		}
		related = append(related, h)
	}
	sort.SliceStable(related, func(i, j int) bool {
		return overlap(related[i].Slug) > overlap(related[j].Slug)
	})
	if len(related) > 5 {
		related = related[:5]
	}
	return related
}

// afterWrite is the invariant pipeline every mutation runs.
func afterWrite(w *config.Wiki, action, relPath string, related []string, note string, syncNow bool, syncMsg string) error {
	scan, err := wiki.Scan(w)
	if err != nil {
		return err
	}
	if err := genindex.Regenerate(w, scan); err != nil {
		return err
	}
	if err := logops.Append(w, action, relPath, related, note); err != nil {
		return err
	}
	var eng embed.Engine
	if embed.Available && embed.ModelAvailable() {
		if e, err := embed.New(); err == nil {
			eng = e
			defer eng.Close()
		}
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return err
	}
	defer st.Close()
	// Rescan: Regenerate rewrote index files (harmless), and the page
	// set may have changed (mv/rm).
	scan, err = wiki.Scan(w)
	if err != nil {
		return err
	}
	if _, err := indexer.Reindex(w, st, scan, eng, nil); err != nil {
		return err
	}
	if syncNow {
		return runSync(w, syncMsg)
	}
	if !flagJSON {
		fmt.Println("NEXT: canopy sync   (commit & push this change, alone or batched with more writes)")
	}
	return nil
}

func runSync(w *config.Wiki, message string) error {
	st, err := gitops.GetStatus(w.Root)
	if err != nil {
		return err
	}
	if !st.IsRepo {
		return fmt.Errorf("%s is not a git repository", w.Root)
	}
	out, err := gitops.Pull(w.Root)
	if err != nil {
		return fmt.Errorf("pull failed (resolve manually in %s): %w", w.Root, err)
	}
	if out != "" && !flagJSON {
		fmt.Println("pull:", firstLine(out))
	}

	st, err = gitops.GetStatus(w.Root)
	if err != nil {
		return err
	}
	committed := false
	dirtyCount := st.Dirty
	if st.Dirty > 0 {
		if message == "" {
			message = autoMessage(st.Changed)
		}
		// Log the sync itself before committing so the entry rides along.
		_ = logops.Append(w, "sync", "", nil, message)
		if _, err := gitops.CommitAll(w.Root, message); err != nil {
			return err
		}
		committed = true
	}
	st, err = gitops.GetStatus(w.Root)
	if err != nil {
		return err
	}
	pushed := false
	if st.Ahead > 0 {
		if _, err := gitops.Push(w.Root); err != nil {
			return fmt.Errorf("push failed — commits are safe locally, rerun `canopy sync`: %w", err)
		}
		pushed = true
	}
	if flagJSON {
		return emitJSON(map[string]any{"committed": committed, "pushed": pushed, "message": message})
	}
	switch {
	case committed && pushed:
		fmt.Printf("✓ synced: committed %d change(s) and pushed\n", dirtyCount)
	case pushed:
		fmt.Println("✓ synced: pushed pending commit(s)")
	case committed:
		fmt.Println("✓ committed (no remote push configured or nothing to push)")
	default:
		fmt.Println("✓ already in sync — nothing to commit or push")
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func autoMessage(changed []string) string {
	var pages []string
	for _, c := range changed {
		if strings.HasSuffix(c, ".md") && !strings.HasPrefix(c, "index") && !strings.HasPrefix(c, "logs/") {
			pages = append(pages, strings.TrimSuffix(filepath.Base(c), ".md"))
		}
	}
	if len(pages) == 0 {
		return fmt.Sprintf("canopy: update %d file(s)", len(changed))
	}
	list := strings.Join(pages, ", ")
	if len(list) > 60 {
		list = strings.Join(pages[:3], ", ") + fmt.Sprintf(", … (%d pages)", len(pages))
	}
	return "canopy: " + list
}

func cmdNew() *cobra.Command {
	var typ, slug, bodyFile string
	var tags, links []string
	var syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a schema-validated page (index/log/embedding handled automatically)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.Join(args, " ")
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			dir, err := dirForType(w, typ)
			if err != nil {
				return err
			}
			if err := validateTags(w, tags); err != nil {
				return err
			}
			if slug == "" {
				slug = wiki.Slugify(title)
				if slug == "" {
					return fmt.Errorf("title has no ASCII letters to derive a filename from — pass an explicit English --slug (filenames must be English)")
				}
			}
			if !wiki.ValidFilename(slug + ".md") {
				return fmt.Errorf("invalid slug %q: lowercase ASCII letters, digits, hyphens only", slug)
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			if p, exists := scan.BySlug[strings.ToLower(slug)]; exists {
				return fmt.Errorf("page already exists: %s — update it instead of duplicating", p.RelPath)
			}
			for _, l := range links {
				if _, ok := scan.BySlug[wiki.NormalizeLink(l)]; !ok {
					return fmt.Errorf("--links target %q does not exist (run `canopy search` to find real pages; links to nonexistent pages are the top lint issue)", l)
				}
			}
			body := ""
			if bodyFile != "" {
				var data []byte
				if bodyFile == "-" {
					data, err = io.ReadAll(os.Stdin)
				} else {
					data, err = os.ReadFile(bodyFile)
				}
				if err != nil {
					return err
				}
				body = string(data)
			}
			today := time.Now().Format("2006-01-02")
			content := wiki.NewPageContent(title, typ, today, tags, links, body)
			relPath := filepath.Join(dir, slug+".md")
			// Fresh wikis start without category directories.
			if err := os.MkdirAll(filepath.Join(w.Root, dir), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(w.Root, relPath), []byte(content), 0o644); err != nil {
				return err
			}
			if !flagJSON {
				fmt.Printf("✓ created %s\n", relPath)
			}

			// Surface related pages so the agent can wire wikilinks
			// while the context is fresh.
			related := []store.Hit{}
			if embed.Available && embed.ModelAvailable() {
				if eng, err := embed.New(); err == nil {
					if qv, err := eng.Embed([]string{title + "\n" + body}); err == nil {
						if st, err := store.Open(w.DBPath()); err == nil {
							if hits, err := st.SearchSemantic(qv[0], 12); err == nil {
								related = rankRelated(hits, slug, tags, scan)
							}
							st.Close()
						}
					}
					eng.Close()
				}
			}
			if len(related) > 0 && !flagJSON {
				fmt.Println("related pages (add [[wikilinks]] where genuinely relevant):")
				for _, h := range related {
					fmt.Printf("  [%.2f] %s — %s\n", h.Score, h.Slug, h.Title)
				}
			}
			if err := afterWrite(w, "create", relPath, tags, title, syncNow, syncMsg); err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"created": relPath, "related": related})
			}
			return nil
		},
	}
	c.Flags().StringVar(&typ, "type", "", "entity|concept|comparison (required)")
	c.Flags().StringSliceVar(&tags, "tags", nil, "tags from the taxonomy")
	c.Flags().StringVar(&slug, "slug", "", "filename slug (required when the title has no English words)")
	c.Flags().StringVar(&bodyFile, "body-file", "", "markdown body file, or - for stdin")
	c.Flags().StringSliceVar(&links, "links", nil, "wikilink targets for the 관련 페이지 section (must exist)")
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	c.MarkFlagRequired("type")
	return c
}

func cmdUpdate() *cobra.Command {
	var bodyFile string
	var syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "update <page>",
		Short: "Record an edit: bump updated date, optionally replace the body, reindex + log",
		Args:  cobra.ExactArgs(1),
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
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			path := filepath.Join(w.Root, p.RelPath)
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(raw)
			if bodyFile != "" {
				var data []byte
				if bodyFile == "-" {
					data, err = io.ReadAll(os.Stdin)
				} else {
					data, err = os.ReadFile(bodyFile)
				}
				if err != nil {
					return err
				}
				content = wiki.ReplaceBody(content, string(data))
			}
			content = wiki.SetFrontmatterField(content, "updated", time.Now().Format("2006-01-02"))
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}
			if !flagJSON {
				fmt.Printf("✓ updated %s\n", p.RelPath)
			}
			return afterWrite(w, "update", p.RelPath, p.Tags, "", syncNow, syncMsg)
		},
	}
	c.Flags().StringVar(&bodyFile, "body-file", "", "replace the page body from file, or - for stdin")
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	return c
}

func cmdMv() *cobra.Command {
	var newType, newSlug string
	var syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "mv <page>",
		Short: "Move a page between categories and/or rename it (rewrites inbound wikilinks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if newType == "" && newSlug == "" {
				return fmt.Errorf("nothing to do: pass --type and/or --slug")
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
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			typ, slug := p.Type, p.Slug
			if newType != "" {
				typ = newType
			}
			if newSlug != "" {
				if !wiki.ValidFilename(newSlug + ".md") {
					return fmt.Errorf("invalid slug %q", newSlug)
				}
				if _, exists := scan.BySlug[strings.ToLower(newSlug)]; exists && !strings.EqualFold(newSlug, p.Slug) {
					return fmt.Errorf("target slug already exists: %s", newSlug)
				}
				slug = newSlug
			}
			dir, err := dirForType(w, typ)
			if err != nil {
				return err
			}
			oldPath := filepath.Join(w.Root, p.RelPath)
			newRel := filepath.Join(dir, slug+".md")
			raw, err := os.ReadFile(oldPath)
			if err != nil {
				return err
			}
			content := string(raw)
			if typ != p.Type {
				content = wiki.SetFrontmatterField(content, "type", typ)
			}
			content = wiki.SetFrontmatterField(content, "updated", time.Now().Format("2006-01-02"))
			if err := os.MkdirAll(filepath.Join(w.Root, dir), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(w.Root, newRel), []byte(content), 0o644); err != nil {
				return err
			}
			if newRel != p.RelPath {
				if err := os.Remove(oldPath); err != nil {
					return err
				}
			}
			// Retarget inbound links on rename.
			if !strings.EqualFold(slug, p.Slug) {
				for _, other := range scan.Pages {
					if other.Slug == p.Slug {
						continue
					}
					op := filepath.Join(w.Root, other.RelPath)
					b, err := os.ReadFile(op)
					if err != nil {
						return err
					}
					rewritten := wiki.RewriteLinks(string(b), p.Slug, slug)
					if rewritten != string(b) {
						if err := os.WriteFile(op, []byte(rewritten), 0o644); err != nil {
							return err
						}
						if !flagJSON {
							fmt.Printf("  relinked %s\n", other.RelPath)
						}
					}
				}
			}
			if !flagJSON {
				fmt.Printf("✓ moved %s → %s\n", p.RelPath, newRel)
			}
			return afterWrite(w, "move", newRel, nil, "from "+p.RelPath, syncNow, syncMsg)
		},
	}
	c.Flags().StringVar(&newType, "type", "", "new type (changes category directory)")
	c.Flags().StringVar(&newSlug, "slug", "", "new filename slug (rewrites inbound wikilinks)")
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	return c
}

func cmdRm() *cobra.Command {
	var force, syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "rm <page>",
		Short: "Delete a page (refuses when backlinks exist, unless --force)",
		Args:  cobra.ExactArgs(1),
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
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			sources := scan.Backlinks()[strings.ToLower(p.Slug)]
			if len(sources) > 0 && !force {
				return fmt.Errorf("%s has %d backlink(s) (%s) — they would break; use `canopy archive` or --force",
					p.Slug, len(sources), strings.Join(sources, ", "))
			}
			if err := os.Remove(filepath.Join(w.Root, p.RelPath)); err != nil {
				return err
			}
			// --force delete: strip now-dangling links.
			for _, src := range sources {
				sp := scan.BySlug[strings.ToLower(src)]
				path := filepath.Join(w.Root, sp.RelPath)
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(wiki.StripLinks(string(b), p.Slug)), 0o644); err != nil {
					return err
				}
			}
			if !flagJSON {
				fmt.Printf("✓ deleted %s\n", p.RelPath)
			}
			return afterWrite(w, "delete", p.RelPath, nil, "", syncNow, syncMsg)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "delete even with backlinks (they become plain text)")
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	return c
}

func cmdArchive() *cobra.Command {
	var syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "archive <page>",
		Short: "Move a superseded page to _archive/ (inbound wikilinks become plain text)",
		Args:  cobra.ExactArgs(1),
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
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			dstDir := filepath.Join(w.Root, "_archive", p.Dir)
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return err
			}
			if err := os.Rename(filepath.Join(w.Root, p.RelPath), filepath.Join(dstDir, filepath.Base(p.RelPath))); err != nil {
				return err
			}
			for _, src := range scan.Backlinks()[strings.ToLower(p.Slug)] {
				sp := scan.BySlug[strings.ToLower(src)]
				path := filepath.Join(w.Root, sp.RelPath)
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(wiki.StripLinks(string(b), p.Slug)), 0o644); err != nil {
					return err
				}
				if !flagJSON {
					fmt.Printf("  unlinked in %s\n", sp.RelPath)
				}
			}
			if !flagJSON {
				fmt.Printf("✓ archived %s → _archive/%s\n", p.RelPath, p.RelPath)
			}
			return afterWrite(w, "archive", p.RelPath, nil, "", syncNow, syncMsg)
		},
	}
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	return c
}

func cmdSync() *cobra.Command {
	var msg string
	c := &cobra.Command{
		Use:   "sync",
		Short: "pull --rebase → commit all wiki changes → push → refresh index",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			if err := runSync(w, msg); err != nil {
				return err
			}
			// Post-pull content may differ; keep the derived index fresh.
			var eng embed.Engine
			if embed.Available && embed.ModelAvailable() {
				if e, err := embed.New(); err == nil {
					eng = e
					defer eng.Close()
				}
			}
			st, err := store.Open(w.DBPath())
			if err != nil {
				return err
			}
			defer st.Close()
			_, err = indexer.Reindex(w, st, nil, eng, nil)
			return err
		},
	}
	c.Flags().StringVarP(&msg, "message", "m", "", "commit message (default: auto-generated from changed pages)")
	return c
}
