// Package checkout lends wiki pages to agents for surgical editing.
// canopy does not compete with an agent's native read/grep/edit tools —
// it materializes a working copy OUTSIDE the wiki tree (git and
// reconcile never see the edit in progress), and validates on the way
// back in: schema, mermaid, and a base-hash conflict check. Design:
// docs/checkout-design.md, invariants R.
package checkout

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Info describes one checkout, open or just-created.
type Info struct {
	Slug     string `json:"slug"`
	RelPath  string `json:"rel_path"`
	Path     string `json:"path"` // working copy (absolute)
	Base     string `json:"base"` // sha256 of the wiki file at checkout
	OpenedAt string `json:"opened_at"`
	Modified bool   `json:"modified"` // copy differs from base content
}

// Result is a successful checkin.
type Result struct {
	RelPath   string   `json:"rel_path"`
	Unchanged bool     `json:"unchanged"`          // copy == base: wiki untouched, copy reclaimed
	OldLines  int      `json:"old_lines"`          // before/after sizes stand in for a diffstat
	NewLines  int      `json:"new_lines"`          //
	Warnings  []string `json:"warnings,omitempty"` // fail-open notices (P4)
}

type meta struct {
	RelPath  string `json:"rel_path"`
	Base     string `json:"base"`
	OpenedAt string `json:"opened_at"`
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func paths(w *config.Wiki, slug string) (copyPath, metaPath string) {
	dir := w.CheckoutDir()
	return filepath.Join(dir, slug+".md"), filepath.Join(dir, slug+".json")
}

func readMeta(path string) (*meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("checkout meta corrupt (%s): %w", path, err)
	}
	return &m, nil
}

// Checkout materializes slug's page as a working copy and returns where
// to edit it. Idempotent: an existing unmodified copy is refreshed to
// the current wiki content; a modified copy is returned as-is (never
// clobbered) with Modified=true.
func Checkout(w *config.Wiki, scan *wiki.ScanResult, slug string) (*Info, error) {
	p, ok := scan.BySlug[wiki.NormalizeLink(slug)]
	if !ok {
		return nil, fmt.Errorf("page not found: %s", slug)
	}
	current, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
	if err != nil {
		return nil, err
	}
	copyPath, metaPath := paths(w, p.Slug)

	if m, err := readMeta(metaPath); err == nil {
		copyData, err := os.ReadFile(copyPath)
		if err != nil {
			return nil, fmt.Errorf("checkout meta exists but copy is gone — `canopy checkin %s --discard` to reset: %w", p.Slug, err)
		}
		if hashOf(copyData) != m.Base {
			// Edits in progress: hand back the same copy untouched.
			return &Info{Slug: p.Slug, RelPath: m.RelPath, Path: copyPath, Base: m.Base, OpenedAt: m.OpenedAt, Modified: true}, nil
		}
		// Unmodified: refresh to current wiki content (base moves with it).
	}

	if err := os.MkdirAll(w.CheckoutDir(), 0o755); err != nil {
		return nil, err
	}
	base := hashOf(current)
	m := meta{RelPath: p.RelPath, Base: base, OpenedAt: time.Now().Format(time.RFC3339)}
	mdata, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(copyPath, current, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, mdata, 0o644); err != nil {
		return nil, err
	}
	return &Info{Slug: p.Slug, RelPath: p.RelPath, Path: copyPath, Base: base, OpenedAt: m.OpenedAt}, nil
}

// Checkin validates the working copy and, if clean, writes it into the
// wiki (bumping updated) and reclaims the copy. The write pipeline
// (index/log/embedding) is the caller's job — same as update.
func Checkin(w *config.Wiki, scan *wiki.ScanResult, slug string, mv *mermaid.Validator) (*Result, error) {
	copyPath, metaPath := paths(w, wiki.NormalizeLink(slug))
	m, err := readMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("not checked out: %s (run `canopy checkout %s` first)", slug, slug)
	}
	copyData, err := os.ReadFile(copyPath)
	if err != nil {
		return nil, fmt.Errorf("working copy missing — `canopy checkin %s --discard` to reset: %w", slug, err)
	}

	p, ok := scan.BySlug[wiki.NormalizeLink(slug)]
	if !ok || p.RelPath != m.RelPath {
		return nil, fmt.Errorf("page moved or removed since checkout (%s) — re-checkout and re-apply; `--discard` drops this copy", m.RelPath)
	}
	wikiPath := filepath.Join(w.Root, p.RelPath)
	current, err := os.ReadFile(wikiPath)
	if err != nil {
		return nil, err
	}

	// R4: the wiki moved underneath us — refuse, never merge silently.
	if hashOf(current) != m.Base {
		return nil, fmt.Errorf("page changed since checkout (another machine, web edit, or direct write) — re-checkout %s and re-apply your edits; `--discard` drops this copy", slug)
	}

	// R8: nothing changed — reclaim the copy, leave the wiki untouched.
	if hashOf(copyData) == m.Base {
		reclaim(copyPath, metaPath)
		return &Result{RelPath: p.RelPath, Unchanged: true}, nil
	}

	res := &Result{RelPath: p.RelPath,
		OldLines: strings.Count(string(current), "\n") + 1,
		NewLines: strings.Count(string(copyData), "\n") + 1}
	if err := validate(w, p, copyData, mv, res); err != nil {
		return nil, err
	}

	content := wiki.SetFrontmatterField(string(copyData), "updated", time.Now().Format("2006-01-02"))
	if err := os.WriteFile(wikiPath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	reclaim(copyPath, metaPath)
	return res, nil
}

// validate runs the write gates on the edited copy: frontmatter shape,
// immutable fields (R7), taxonomy (A5), mermaid (P2 — env faults fail
// open into res.Warnings, per P4).
func validate(w *config.Wiki, p *wiki.Page, copyData []byte, mv *mermaid.Validator, res *Result) error {
	edited := wiki.Parse(p.RelPath, copyData)
	if !edited.HasFrontmatter {
		return fmt.Errorf("frontmatter block missing — the copy must keep its `---` header")
	}
	if edited.FMErr != "" {
		return fmt.Errorf("frontmatter parse error: %s", edited.FMErr)
	}
	if edited.Title == "" {
		return fmt.Errorf("frontmatter title must not be empty")
	}
	if edited.Type != p.Type {
		return fmt.Errorf("type change (%s → %s) is not a checkin — `canopy mv %s --type %s` moves the page between category dirs", p.Type, edited.Type, p.Slug, edited.Type)
	}
	if edited.Created != p.Created {
		return fmt.Errorf("created (%s) is history — restore it (was changed to %s)", p.Created, edited.Created)
	}
	allowed := map[string]bool{}
	for _, t := range w.Cfg.Schema.AllTags() {
		allowed[t] = true
	}
	var bad []string
	for _, t := range edited.Tags {
		if !allowed[t] {
			bad = append(bad, t)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("tags not in taxonomy: %s (see `canopy tags`)", strings.Join(bad, ", "))
	}
	if mv != nil {
		for _, b := range mermaid.Blocks(string(copyData)) {
			diagErr, envErr := mv.Validate(b.Source)
			if envErr != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("mermaid block (line %d) not validated (environment, not the diagram): %v", b.Line, envErr))
				continue
			}
			if diagErr != nil {
				return fmt.Errorf("mermaid block at line %d fails the renderer's parser — fix it in the working copy and retry:\n%s", b.Line, diagErr.Message)
			}
		}
	}
	return nil
}

// Discard drops a checkout without touching the wiki.
func Discard(w *config.Wiki, slug string) error {
	copyPath, metaPath := paths(w, wiki.NormalizeLink(slug))
	if _, err := os.Stat(metaPath); err != nil {
		return fmt.Errorf("not checked out: %s", slug)
	}
	reclaim(copyPath, metaPath)
	return nil
}

func reclaim(copyPath, metaPath string) {
	os.Remove(copyPath)
	os.Remove(metaPath)
}

// Open lists checkouts, oldest first, with their modified state (R6).
func Open(w *config.Wiki) ([]Info, error) {
	entries, err := os.ReadDir(w.CheckoutDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".json")
		copyPath, metaPath := paths(w, slug)
		m, err := readMeta(metaPath)
		if err != nil {
			continue
		}
		info := Info{Slug: slug, RelPath: m.RelPath, Path: copyPath, Base: m.Base, OpenedAt: m.OpenedAt}
		if data, err := os.ReadFile(copyPath); err == nil {
			info.Modified = hashOf(data) != m.Base
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenedAt < out[j].OpenedAt })
	return out, nil
}
