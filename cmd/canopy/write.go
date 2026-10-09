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
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/genindex"
	"github.com/neutrospec/canopy/internal/gitops"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/logops"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/safefile"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
	"github.com/neutrospec/canopy/internal/writeops"
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
	for _, t := range w.Cfg.Schema.AllTags() {
		allowed[t] = true
	}
	var bad []string
	for _, t := range tags {
		if !allowed[t] {
			bad = append(bad, t)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("tags not in taxonomy: %s (see `canopy tags` for the valid list; a topic is added to canopy.toml only when ≥3 pages demand it — docs/taxonomy.md)", strings.Join(bad, ", "))
	}
	return nil
}

// validateMermaid rejects content whose ```mermaid blocks fail the
// renderer's own parser (invariant P2) — a broken diagram would render
// as an error box, so it never lands on disk through canopy. Sandbox
// faults are not the diagram's fault and fail open with a warning (P4).
// Line numbers are relative to the final file content passed in.
func validateMermaid(content string) error {
	blocks := mermaid.Blocks(content)
	if len(blocks) == 0 {
		return nil
	}
	v := mermaid.NewValidator()
	for _, b := range blocks {
		bad, err := v.Validate(b.Source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠ mermaid block (line %d) could not be validated (environment, not the diagram): %v\n", b.Line, err)
			continue
		}
		if bad != nil {
			return fmt.Errorf("mermaid block at line %d fails the renderer's parser — fix the diagram and retry (syntax rules: canopy-wiki skill):\n%s", b.Line, bad.Message)
		}
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

// Mutation JSON is one public outer object, including known execution failures.
// Contract and compatibility policy: docs/mutation-results.md.
type mutationResult struct {
	SchemaVersion int            `json:"schema_version"`
	Command       string         `json:"command"`
	Status        string         `json:"status"`
	Applied       string         `json:"applied"`
	Stage         string         `json:"stage"`
	Result        map[string]any `json:"result"`
	Error         string         `json:"error,omitempty"`
	Repair        string         `json:"repair,omitempty"`
	Sync          *syncResult    `json:"sync,omitempty"`
	Committed     *bool          `json:"committed,omitempty"`
	Pushed        *bool          `json:"pushed,omitempty"`
	Message       *string        `json:"message,omitempty"`
	noop          bool
}
type syncResult struct {
	Requested bool   `json:"requested"`
	Committed bool   `json:"committed"`
	Pushed    bool   `json:"pushed"`
	Message   string `json:"message"`
	Pull      string `json:"pull"`
	Commit    string `json:"commit"`
	Push      string `json:"push"`
	Stage     string `json:"stage"`
}

func newMutationResult(verb string, syncNow bool) *mutationResult {
	r := &mutationResult{SchemaVersion: 1, Command: verb, Applied: "none", Stage: "validate", Result: map[string]any{}}
	if syncNow {
		r.Sync = &syncResult{Requested: true, Pull: "not_attempted", Commit: "not_attempted", Push: "not_attempted"}
	}
	return r
}
func (r *mutationResult) finish(err error) error {
	if err != nil {
		r.Status = "error"
		r.Error = err.Error()
		var stageErr *writeops.StageError
		if errors.As(err, &stageErr) {
			r.Stage = stageErr.Stage
		}
		if r.Repair == "" {
			if r.Applied == "none" {
				r.Repair = "preserve input, correct the reported error, inspect current wiki, then retry"
			} else {
				r.Repair = "inspect page, source/destination and backlinks; repair the named stage without repeating the mutation; canopy reindex repairs the search cache; safe canopy sync regenerates Markdown indexes and commits all wiki changes"
			}
		}
	} else {
		r.Status = "ok"
		if r.noop {
			r.Status = "noop"
		}
		r.Stage = "complete"
	}
	if r.Sync != nil {
		r.Committed = &r.Sync.Committed
		r.Pushed = &r.Sync.Pushed
		r.Message = &r.Sync.Message
	}
	if flagJSON {
		if emitErr := emitJSON(r); emitErr != nil {
			return emitErr
		}
	}
	return err
}

type fileStep struct {
	kind, path, from, stage string
	data                    []byte
	perm                    fs.FileMode
}
type mutationPlan struct {
	scan              *wiki.ScanResult
	raw               map[string][]byte
	steps             []fileStep
	action, rel, note string
	related           []string
	effects           reconcile.Effects
	before            func() error
	after             func() error
}

func newPlan(w *config.Wiki) (*mutationPlan, error) {
	scan, err := wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	plan := &mutationPlan{scan: scan, raw: map[string][]byte{}}
	for i, p := range scan.Pages {
		data, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
		if err != nil {
			return nil, err
		}
		plan.raw[p.RelPath] = data
		actual := wiki.Parse(p.RelPath, data)
		scan.Pages[i] = actual
		scan.BySlug[strings.ToLower(p.Slug)] = actual
	}
	scan.ResolveLinks()
	return plan, nil
}
func (p *mutationPlan) verify(w *config.Wiki) error {
	current, err := wiki.Scan(w)
	if err != nil {
		return err
	}
	if len(current.Pages) != len(p.raw) {
		return fmt.Errorf("wiki page set changed while preparing mutation; inspect and retry")
	}
	for _, page := range current.Pages {
		before, ok := p.raw[page.RelPath]
		if !ok {
			return fmt.Errorf("wiki page set changed while preparing mutation; inspect and retry")
		}
		now, err := os.ReadFile(filepath.Join(w.Root, page.RelPath))
		if err != nil {
			return err
		}
		if reconcile.HashBytes(now) != reconcile.HashBytes(before) {
			return fmt.Errorf("page %s changed while preparing mutation; inspect and retry", page.RelPath)
		}
	}
	return nil
}
func (p *mutationPlan) apply(w *config.Wiki, r *mutationResult) error {
	for i, step := range p.steps {
		r.Stage = step.stage
		path := filepath.Join(w.Root, step.path)
		perm := step.perm
		if perm == 0 {
			perm = 0o644
		}
		var err error
		checkPath := step.path
		if step.kind == "move" {
			checkPath = step.from
		}
		if before, ok := p.raw[checkPath]; ok && step.kind != "create" {
			now, readErr := os.ReadFile(filepath.Join(w.Root, checkPath))
			if readErr != nil {
				err = readErr
			} else if reconcile.HashBytes(now) != reconcile.HashBytes(before) {
				err = fmt.Errorf("page %s changed immediately before publication; preserve current bytes and inspect", checkPath)
			}
		}
		if err == nil {
			switch step.kind {
			case "create":
				if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
					err = safefile.CreateFile(path, step.data, perm)
				}
			case "write":
				err = safefile.WriteFile(path, step.data, 0o644)
			case "remove":
				if err = safefile.Regular(path); err == nil {
					err = os.Remove(path)
				}
			case "move":
				if err = os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
					err = safefile.MoveNoReplace(filepath.Join(w.Root, step.from), path)
				}
			}
		}
		if err != nil {
			if i > 0 || safefile.Published(err) {
				r.Applied = "partial"
			}
			if len(p.steps) == 1 && safefile.Published(err) && step.kind != "move" {
				r.Applied = "applied"
			}
			if i == 0 && safefile.Published(err) {
				if data, readErr := os.ReadFile(path); readErr == nil {
					r.Result["applied_hash"] = reconcile.HashBytes(data)
				} else {
					r.Applied = "unknown"
				}
			}
			return err
		}
		if i == 0 && step.kind != "remove" {
			if data, readErr := os.ReadFile(path); readErr == nil {
				r.Result["applied_hash"] = reconcile.HashBytes(data)
			} else {
				r.Applied = "unknown"
				return readErr
			}
		}
		r.Applied = "partial"
	}
	if len(p.steps) > 0 {
		r.Applied = "applied"
	} else {
		r.noop = true
	}
	if p.after != nil {
		r.Stage = "state"
		if err := p.after(); err != nil {
			return err
		}
	}
	return nil
}

// runMutation prepares input and expensive validation outside the wiki lease,
// then verifies the observed page tree and protects publication + filesystem
// postprocessing. Inference and optional sync run after that lease is released.
func runMutation(cmd *cobra.Command, verb string, syncNow bool, syncMsg string, build func(*config.Wiki, *mutationResult) (*mutationPlan, error)) (retErr error) {
	r := newMutationResult(verb, syncNow)
	defer func() { retErr = r.finish(retErr) }()
	w, err := loadWiki()
	if err != nil {
		return err
	}
	banner(w)
	plan, err := build(w, r)
	if err != nil {
		return err
	}
	var scan *wiki.ScanResult
	if plan != nil && !r.noop {
		r.Stage = "lock"
		g, err := wikilock.Acquire(cmd.Context(), w.Root)
		if err != nil {
			return err
		}
		err = func() error {
			defer g.Unlock()
			r.Stage = "cleanup"
			if err := safefile.CleanTemps(w.Root); err != nil {
				return err
			}
			r.Stage = "validate"
			if err := plan.verify(w); err != nil {
				return err
			}
			if plan.before != nil {
				r.Stage = "state"
				if err := plan.before(); err != nil {
					return err
				}
			}
			if err := plan.apply(w, r); err != nil {
				return err
			}
			var err error
			scan, err = writeops.RunLocked(w, plan.action, plan.rel, plan.related, plan.note, plan.effects)
			return err
		}()
		if err != nil {
			return err
		}
		r.Stage = "reindex"
		if err := writeops.Refresh(w, scan); err != nil {
			return err
		}
	}
	if verb == "new" && plan != nil {
		r.Result["related"] = relatedAfterCreate(cmd.Context(), w, plan, r)
	}
	if syncNow {
		r.Sync, err = performSync(w, syncMsg)
		r.Stage = r.Sync.Stage
		if err != nil {
			r.Repair = "inspect Git status and preserve any autostash/conflicted files; repair the requested sync stage and run canopy sync; do not repeat the page mutation"
			return err
		}
	}
	if !flagJSON {
		if r.noop {
			fmt.Printf("no changes (%s)\n", verb)
		} else {
			fmt.Printf("✓ %s complete\n", verb)
		}
		if !syncNow {
			fmt.Println("NEXT: canopy sync   (commit & push this change, alone or batched with more writes)")
		}
	}
	return nil
}

func readBody(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	return string(b), err
}
func gatePage(w *config.Wiki, rel, content string) error {
	p := wiki.Parse(rel, []byte(content))
	if !p.HasFrontmatter || p.FMErr != "" {
		return fmt.Errorf("frontmatter missing or invalid: %s", p.FMErr)
	}
	// Existing legacy pages retain their tolerated schema. Five-key presence
	// is a lint diagnostic, not a new universal mutation gate. Explicit new
	// type/tags are checked by new/mv; checkin keeps its established gates.
	return validateMermaid(content)
}
func addSyncFlags(c *cobra.Command, syncNow *bool, syncMsg *string) {
	c.Flags().BoolVar(syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(syncMsg, "message", "m", "", "commit message when using --sync")
}
func absent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("destination already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func cmdNew() *cobra.Command {
	var typ, slug, bodyFile, syncMsg string
	var tags, links []string
	var syncNow bool
	c := &cobra.Command{Use: "new <title>", Short: "Create a page, validate and refresh indexes", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runMutation(cmd, "new", syncNow, syncMsg, func(w *config.Wiki, r *mutationResult) (*mutationPlan, error) {
			body, err := readBody(bodyFile)
			if err != nil {
				return nil, err
			}
			dir, err := dirForType(w, typ)
			if err != nil {
				return nil, err
			}
			if err := validateTags(w, tags); err != nil {
				return nil, err
			}
			title := args[0]
			name := slug
			if name == "" {
				name = wiki.Slugify(title)
				if name == "" {
					return nil, fmt.Errorf("title has no ASCII letters to derive a filename from; pass an explicit English --slug")
				}
			}
			if !wiki.ValidFilename(name + ".md") {
				return nil, fmt.Errorf("invalid slug %q", name)
			}
			plan, err := newPlan(w)
			if err != nil {
				return nil, err
			}
			if p, ok := plan.scan.BySlug[strings.ToLower(name)]; ok {
				return nil, fmt.Errorf("page already exists: %s; update it instead", p.RelPath)
			}
			for _, link := range links {
				if _, err := plan.scan.Find(link); err != nil {
					return nil, fmt.Errorf("--links target %q does not exist: %w", link, err)
				}
			}
			rel := filepath.Join(dir, name+".md")
			content := wiki.NewPageContent(title, typ, time.Now().Format("2006-01-02"), tags, links, body)
			if err := gatePage(w, rel, content); err != nil {
				return nil, err
			}
			plan.steps = []fileStep{{kind: "create", path: rel, data: []byte(content), stage: "publish"}}
			plan.action = "create"
			plan.rel = rel
			plan.related = tags
			plan.note = title
			plan.effects = reconcile.Effects{Written: []string{rel}}
			r.Result["created"] = rel
			r.Result["related"] = []store.Hit{}
			return plan, nil
		})
	}
	c.Flags().StringVar(&typ, "type", "", "entity|concept|comparison (required)")
	c.Flags().StringVar(&slug, "slug", "", "filename slug (required when title has no English words)")
	c.Flags().StringSliceVar(&tags, "tags", nil, "tags from taxonomy")
	c.Flags().StringSliceVar(&links, "links", nil, "existing wikilink targets")
	c.Flags().StringVar(&bodyFile, "body-file", "", "markdown body file, or - for stdin")
	addSyncFlags(c, &syncNow, &syncMsg)
	c.MarkFlagRequired("type")
	return c
}

func cmdUpdate() *cobra.Command {
	var bodyFile, syncMsg string
	var syncNow bool
	c := &cobra.Command{Use: "update <page>", Short: "Validate an edit, bump updated, refresh indexes and log", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runMutation(cmd, "update", syncNow, syncMsg, func(w *config.Wiki, r *mutationResult) (*mutationPlan, error) {
			body, err := readBody(bodyFile)
			if err != nil {
				return nil, err
			}
			plan, err := newPlan(w)
			if err != nil {
				return nil, err
			}
			p, err := plan.scan.Find(args[0])
			if err != nil {
				return nil, err
			}
			content := string(plan.raw[p.RelPath])
			if bodyFile != "" {
				content = wiki.ReplaceBody(content, body)
			}
			content, err = wiki.SetFrontmatterField(content, "updated", time.Now().Format("2006-01-02"))
			if err != nil {
				return nil, err
			}
			if err := gatePage(w, p.RelPath, content); err != nil {
				return nil, err
			}
			plan.steps = []fileStep{{kind: "write", path: p.RelPath, data: []byte(content), stage: "publish"}}
			plan.action = "update"
			plan.rel = p.RelPath
			plan.related = p.Tags
			plan.effects = reconcile.Effects{Written: []string{p.RelPath}}
			r.Result["updated"] = p.RelPath
			return plan, nil
		})
	}
	c.Flags().StringVar(&bodyFile, "body-file", "", "replace body from file, or - for stdin")
	addSyncFlags(c, &syncNow, &syncMsg)
	return c
}

func cmdMv() *cobra.Command {
	var newType, newSlug, syncMsg string
	var syncNow bool
	c := &cobra.Command{Use: "mv <page>", Short: "Move or rename a page and rewrite actual inbound wikilinks", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runMutation(cmd, "mv", syncNow, syncMsg, func(w *config.Wiki, r *mutationResult) (*mutationPlan, error) {
			if newType == "" && newSlug == "" {
				return nil, fmt.Errorf("nothing to do: pass --type and/or --slug")
			}
			plan, err := newPlan(w)
			if err != nil {
				return nil, err
			}
			p, err := plan.scan.Find(args[0])
			if err != nil {
				return nil, err
			}
			typ, name := p.Type, p.Slug
			if newType != "" {
				typ = newType
			}
			if newSlug != "" {
				name = newSlug
			}
			if !wiki.ValidFilename(name + ".md") {
				return nil, fmt.Errorf("invalid slug %q", name)
			}
			if target, ok := plan.scan.BySlug[strings.ToLower(name)]; ok && target.RelPath != p.RelPath {
				return nil, fmt.Errorf("target slug already exists: %s", target.RelPath)
			}
			dir, err := dirForType(w, typ)
			if err != nil {
				return nil, err
			}
			rel := filepath.Join(dir, name+".md")
			r.Result["from"] = p.RelPath
			r.Result["moved"] = rel
			if rel == p.RelPath && typ == p.Type {
				r.noop = true
				return plan, nil
			}
			if rel != p.RelPath {
				if err := absent(filepath.Join(w.Root, rel)); err != nil {
					return nil, err
				}
			}
			if err := safefile.Regular(filepath.Join(w.Root, p.RelPath)); err != nil {
				return nil, err
			}
			info, err := os.Stat(filepath.Join(w.Root, p.RelPath))
			if err != nil {
				return nil, err
			}
			content := string(plan.raw[p.RelPath])
			if typ != p.Type {
				content, err = wiki.SetFrontmatterField(content, "type", typ)
				if err != nil {
					return nil, err
				}
			}
			content, err = wiki.SetFrontmatterField(content, "updated", time.Now().Format("2006-01-02"))
			if err != nil {
				return nil, err
			}
			content = wiki.RewritePageLinksInScan(content, p.RelPath, rel, plan.scan)
			if err := gatePage(w, rel, content); err != nil {
				return nil, err
			}
			kind := "write"
			if rel != p.RelPath {
				kind = "create"
			}
			plan.steps = append(plan.steps, fileStep{kind: kind, path: rel, data: []byte(content), stage: "publish", perm: info.Mode().Perm()})
			plan.effects.Written = append(plan.effects.Written, rel)
			if rel != p.RelPath {
				plan.steps = append(plan.steps, fileStep{kind: "remove", path: p.RelPath, stage: "publish"})
				plan.effects.Removed = []string{p.RelPath}
			}
			for _, other := range plan.scan.Pages {
				if other.RelPath == p.RelPath {
					continue
				}
				raw := string(plan.raw[other.RelPath])
				next := wiki.RewritePageLinksInScan(raw, p.RelPath, rel, plan.scan)
				if next != raw {
					plan.steps = append(plan.steps, fileStep{kind: "write", path: other.RelPath, data: []byte(next), stage: "backlinks"})
					plan.effects.Written = append(plan.effects.Written, other.RelPath)
				}
			}
			if !strings.EqualFold(name, p.Slug) {
				plan.before = func() error { return reads.CanRenameInWiki(w, p.Slug, name) }
				plan.after = func() error { return reads.RenameInWiki(w, p.Slug, name) }
			}
			plan.action = "move"
			plan.rel = rel
			plan.note = "from " + p.RelPath
			return plan, nil
		})
	}
	c.Flags().StringVar(&newType, "type", "", "new type/category")
	c.Flags().StringVar(&newSlug, "slug", "", "new slug (rewrites inbound wikilinks)")
	addSyncFlags(c, &syncNow, &syncMsg)
	return c
}

func removalPlan(w *config.Wiki, r *mutationResult, input string, archive, force bool) (*mutationPlan, error) {
	plan, err := newPlan(w)
	if err != nil {
		return nil, err
	}
	p, err := plan.scan.Find(input)
	if err != nil {
		return nil, err
	}
	sources := plan.scan.Backlinks()[strings.ToLower(p.Slug)]
	if !archive && len(sources) > 0 && !force {
		return nil, fmt.Errorf("%s has %d backlink(s) (%s); use archive or --force", p.Slug, len(sources), strings.Join(sources, ", "))
	}
	if archive {
		rel := filepath.Join("_archive", p.RelPath)
		if err := absent(filepath.Join(w.Root, rel)); err != nil {
			return nil, err
		}
		plan.steps = append(plan.steps, fileStep{kind: "move", path: rel, from: p.RelPath, stage: "publish"})
		plan.action = "archive"
		r.Result["archived"] = rel
	} else {
		plan.steps = append(plan.steps, fileStep{kind: "remove", path: p.RelPath, stage: "publish"})
		plan.action = "delete"
		r.Result["deleted"] = p.RelPath
	}
	plan.rel = p.RelPath
	plan.effects.Removed = []string{p.RelPath}
	for _, src := range sources {
		source := plan.scan.BySlug[strings.ToLower(src)]
		raw := string(plan.raw[source.RelPath])
		next := wiki.StripPageLinksInScan(raw, p.RelPath, plan.scan)
		if next != raw {
			plan.steps = append(plan.steps, fileStep{kind: "write", path: source.RelPath, data: []byte(next), stage: "backlinks"})
			plan.effects.Written = append(plan.effects.Written, source.RelPath)
		}
	}
	return plan, nil
}
func cmdRm() *cobra.Command {
	var force, syncNow bool
	var syncMsg string
	c := &cobra.Command{Use: "rm <page>", Short: "Delete a page, requiring --force when backlinks exist", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runMutation(cmd, "rm", syncNow, syncMsg, func(w *config.Wiki, r *mutationResult) (*mutationPlan, error) {
			return removalPlan(w, r, args[0], false, force)
		})
	}
	c.Flags().BoolVar(&force, "force", false, "strip backlinks and delete page")
	addSyncFlags(c, &syncNow, &syncMsg)
	return c
}
func cmdArchive() *cobra.Command {
	var syncNow bool
	var syncMsg string
	c := &cobra.Command{Use: "archive <page>", Short: "Archive a page without overwriting existing archive content", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return runMutation(cmd, "archive", syncNow, syncMsg, func(w *config.Wiki, r *mutationResult) (*mutationPlan, error) {
			return removalPlan(w, r, args[0], true, false)
		})
	}
	addSyncFlags(c, &syncNow, &syncMsg)
	return c
}

func relatedAfterCreate(ctx context.Context, w *config.Wiki, plan *mutationPlan, r *mutationResult) []store.Hit {
	related := []store.Hit{}
	if !embed.Available() || !embed.ModelAvailable() {
		return related
	}
	eng, err := embed.New()
	if err != nil {
		return related
	}
	defer eng.Close()
	raw, err := os.ReadFile(filepath.Join(w.Root, plan.rel))
	if err != nil {
		return related
	}
	page := wiki.Parse(plan.rel, raw)
	qv, err := embed.EmbedContext(ctx, eng, []string{page.Title + "\n" + page.Body})
	if err != nil || len(qv) == 0 {
		return related
	}
	st, err := store.Open(w.DBPath())
	if err != nil {
		return related
	}
	defer st.Close()
	if _, err := indexer.EnsureCurrentContext(ctx, w, st, plan.scan, nil, nil); err != nil {
		return related
	}
	hits, err := st.SearchSemantic(qv[0], 12)
	if err != nil {
		return related
	}
	related = rankRelated(hits, page.Slug, page.Tags, plan.scan)
	if !flagJSON && len(related) > 0 {
		fmt.Println("related pages:")
		for _, hit := range related {
			fmt.Printf("  [%.2f] %s — %s\n", hit.Score, hit.Slug, hit.Title)
		}
	}
	return related
}

// performSync serializes Git and short filesystem stages with all cooperating
// mutations. It never holds the lease across model loading/inference.
func performSync(w *config.Wiki, message string) (*syncResult, error) {
	res := &syncResult{Requested: true, Message: message, Pull: "not_attempted", Commit: "not_attempted", Push: "not_attempted", Stage: "lock"}
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return res, err
	}
	var scan *wiki.ScanResult
	err = func() error {
		defer g.Unlock()
		res.Stage = "pull"
		st, err := gitops.GetStatus(w.Root)
		if err != nil {
			return err
		}
		if !st.IsRepo {
			return fmt.Errorf("%s is not a git repository", w.Root)
		}
		if err := gitops.RequireSafe(w.Root); err != nil {
			res.Pull = "failed"
			return err
		}
		res.Stage = "cleanup"
		if err := safefile.CleanTemps(w.Root); err != nil {
			return err
		}
		res.Stage = "pull"
		out, err := gitops.Pull(w.Root)
		if err != nil {
			res.Pull = "failed"
			return fmt.Errorf("pull failed; resolve manually in %s, preserve local files and autostash: %w", w.Root, err)
		}
		res.Pull = "succeeded"
		if strings.HasPrefix(out, "no upstream") {
			res.Pull = "skipped"
		}
		if out != "" && !flagJSON {
			fmt.Println("pull:", firstLine(out))
		}
		if err := gitops.RequireSafe(w.Root); err != nil {
			return err
		}
		// Markdown indexes must reflect the final pulled tree and ride in its commit.
		res.Stage = "scan"
		scan, err = wiki.Scan(w)
		if err != nil {
			return err
		}
		res.Stage = "index"
		if err := genindex.Regenerate(w, scan); err != nil {
			return err
		}
		st, err = gitops.GetStatus(w.Root)
		if err != nil {
			return err
		}
		res.Commit = "skipped"
		if st.Dirty > 0 {
			if message == "" {
				message = autoMessage(st.Changed)
				res.Message = message
			}
			if n, ok, err := reconcile.Count(w); err == nil && ok && n > 0 && !flagJSON {
				fmt.Fprintf(os.Stderr, "⚠ 미정규화 외부 변경 %d건이 함께 커밋됩니다 — 검토는 canopy reconcile\n", n)
			}
			if err := gitops.RequireSafe(w.Root); err != nil {
				return err
			}
			res.Stage = "log"
			if err := logops.AppendLocked(w, "sync", "", nil, message); err != nil {
				return err
			}
			before, _ := gitops.Head(w.Root)
			res.Stage = "commit"
			_, commitErr := gitops.CommitAll(w.Root, message)
			after, headErr := gitops.Head(w.Root)
			res.Committed = headErr == nil && after != "" && after != before
			if commitErr != nil {
				res.Commit = "failed"
				if headErr != nil {
					res.Commit = "unknown"
				}
				return commitErr
			}
			res.Commit = "succeeded"
		}
		st, err = gitops.GetStatus(w.Root)
		if err != nil {
			return err
		}
		res.Push = "skipped"
		if st.Ahead > 0 {
			res.Stage = "push"
			out, err := gitops.Push(w.Root)
			if err != nil {
				res.Push = "unknown"
				if remote, probeErr := gitops.RemoteTip(w.Root); probeErr == nil {
					if head, headErr := gitops.Head(w.Root); headErr == nil {
						res.Pushed = remote == head
						res.Push = "failed"
						if res.Pushed {
							res.Push = "succeeded"
						}
					}
				}
				return fmt.Errorf("push failed; commits remain local; inspect and rerun canopy sync: %w", err)
			}
			if !strings.HasPrefix(out, "no remote") {
				res.Pushed = true
				res.Push = "succeeded"
			}
		}
		return nil
	}()
	if err != nil {
		return res, err
	}
	res.Stage = "reindex"
	if err := writeops.Refresh(w, scan); err != nil {
		return res, err
	}
	attention.LogLifecycle(w, time.Now(), "", attention.DoorAgent, attention.KindSyncDone, fmt.Sprintf("committed=%v pushed=%v", res.Committed, res.Pushed))
	res.Stage = "complete"
	return res, nil
}
func runSync(w *config.Wiki, message string) error {
	res, err := performSync(w, message)
	if flagJSON {
		payload := map[string]any{"committed": res.Committed, "pushed": res.Pushed, "message": res.Message, "sync": res}
		if err != nil {
			payload["error"] = err.Error()
			payload["stage"] = res.Stage
		}
		if emitErr := emitJSON(payload); emitErr != nil {
			return emitErr
		}
	}
	if err == nil && !flagJSON {
		fmt.Printf("✓ sync complete (committed=%v pushed=%v)\n", res.Committed, res.Pushed)
	}
	return err
}
func cmdSync() *cobra.Command {
	var message string
	c := &cobra.Command{Use: "sync", Short: "Pull safely, refresh Markdown indexes, commit wiki changes, push and refresh cache"}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		w, err := loadWiki()
		if err != nil {
			return err
		}
		return runSync(w, message)
	}
	c.Flags().StringVarP(&message, "message", "m", "", "commit message (default: from changed pages)")
	return c
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
