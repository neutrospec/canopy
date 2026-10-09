// Package checkout preserves working copies outside the wiki and validates
// snapshots again under the cooperative wiki lease before publication.
package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/safefile"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
)

type Info struct {
	Slug       string `json:"slug"`
	RelPath    string `json:"rel_path"`
	Path       string `json:"path"`
	Base       string `json:"base"`
	OpenedAt   string `json:"opened_at"`
	Modified   bool   `json:"modified"`
	Status     string `json:"status"`
	Diagnostic string `json:"diagnostic,omitempty"`
	Rescue     string `json:"rescue,omitempty"`
}

type Result struct {
	RelPath        string   `json:"rel_path"`
	Unchanged      bool     `json:"unchanged"`
	OldLines       int      `json:"old_lines"`
	NewLines       int      `json:"new_lines"`
	Warnings       []string `json:"warnings,omitempty"`
	Applied        bool     `json:"applied"`
	AppliedHash    string   `json:"applied_hash,omitempty"`
	CleanupPending bool     `json:"cleanup_pending"`
	Resumed        bool     `json:"resumed"`
	OperationID    string   `json:"operation_id,omitempty"`
}

type meta struct {
	Version        int    `json:"version,omitempty"`
	RelPath        string `json:"rel_path"`
	Base           string `json:"base"`
	OpenedAt       string `json:"opened_at"`
	PendingHash    string `json:"pending_hash,omitempty"`
	CopyHash       string `json:"copy_hash,omitempty"`
	AppliedHash    string `json:"applied_hash,omitempty"`
	CleanupPending bool   `json:"cleanup_pending,omitempty"`
	OperationID    string `json:"operation_id,omitempty"`
}

type StageError struct {
	Stage string
	Err   error
}

func (e *StageError) Error() string { return e.Stage + ": " + e.Err.Error() }
func (e *StageError) Unwrap() error { return e.Err }

func hashOf(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func paths(w *config.Wiki, slug string) (string, string) {
	return filepath.Join(w.CheckoutDir(), slug+".md"), filepath.Join(w.CheckoutDir(), slug+".json")
}
func readMeta(path string) (*meta, error) {
	if err := safefile.Regular(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m meta
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("checkout meta corrupt or unsupported (%s): %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("checkout meta has trailing data (%s)", path)
	}
	if m.Version > 1 || m.RelPath == "" || len(m.Base) != 64 || m.OpenedAt == "" || filepath.IsAbs(m.RelPath) || strings.HasPrefix(filepath.Clean(m.RelPath), "..") {
		return nil, fmt.Errorf("checkout metadata unsupported or incomplete (%s); preserve the working copy for rescue", path)
	}
	return &m, nil
}
func saveMeta(path string, m *meta) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return safefile.WriteFile(path, append(data, '\n'), 0o600)
}

// Checkout never overwrites a copy whose metadata cannot be read. Existing
// valid unmodified copies may refresh; modified copies are handed back as-is.
func Checkout(w *config.Wiki, scan *wiki.ScanResult, slug string) (*Info, error) {
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return nil, err
	}
	defer g.Unlock()
	scan, err = wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	p, err := scan.Find(slug)
	if err != nil {
		return nil, err
	}
	pagePath := filepath.Join(w.Root, p.RelPath)
	if err := safefile.Regular(pagePath); err != nil {
		return nil, err
	}
	current, err := os.ReadFile(pagePath)
	if err != nil {
		return nil, err
	}
	copyPath, metaPath := paths(w, p.Slug)
	if err := safefile.Regular(copyPath); err != nil {
		return nil, err
	}
	exists := false
	if m, err := readMeta(metaPath); err == nil {
		if m.PendingHash != "" || m.AppliedHash != "" || m.CleanupPending {
			return nil, fmt.Errorf("checkout has application/cleanup pending; canopy checkin %s inspects/resumes it; rescue the copy before --discard", p.Slug)
		}
		copyData, err := os.ReadFile(copyPath)
		if err != nil {
			return nil, fmt.Errorf("checkout meta exists but copy is gone; inspect --list before --discard: %w", err)
		}
		exists = true
		if hashOf(copyData) != m.Base {
			return &Info{Slug: p.Slug, RelPath: m.RelPath, Path: copyPath, Base: m.Base, OpenedAt: m.OpenedAt, Modified: true, Status: "open"}, nil
		}
	} else {
		_, copyErr := os.Lstat(copyPath)
		if !os.IsNotExist(err) || !os.IsNotExist(copyErr) {
			return nil, fmt.Errorf("orphan checkout %s: %v; copy preserved at %s; copy it to a safe location, inspect wiki, then canopy checkin %s --discard", p.Slug, err, copyPath, p.Slug)
		}
	}
	if err := os.MkdirAll(w.CheckoutDir(), 0o700); err != nil {
		return nil, err
	}
	m := meta{Version: 1, RelPath: p.RelPath, Base: hashOf(current), OpenedAt: time.Now().Format(time.RFC3339Nano)}
	write := safefile.CreateFile
	if exists {
		write = safefile.WriteFile
	}
	if err := write(copyPath, current, 0o600); err != nil {
		return nil, err
	}
	if err := saveMeta(metaPath, &m); err != nil {
		return nil, err
	}
	return &Info{Slug: p.Slug, RelPath: p.RelPath, Path: copyPath, Base: m.Base, OpenedAt: m.OpenedAt, Status: "open"}, nil
}

// Checkin is the standalone compatibility entrypoint. Command callers use
// CheckinPending and Complete around their postprocessing pipeline.
func Checkin(w *config.Wiki, scan *wiki.ScanResult, slug string, mv *mermaid.Validator) (*Result, error) {
	res, err := CheckinPending(w, scan, slug, mv)
	if err != nil {
		return res, err
	}
	if err := Complete(w, slug, res); err != nil {
		return res, err
	}
	return res, nil
}

// CheckinPending validates outside the lease, then rechecks page, copy and
// metadata under an exclusive lease. Publication retains recovery metadata.
func CheckinPending(w *config.Wiki, scan *wiki.ScanResult, slug string, mv *mermaid.Validator) (*Result, error) {
	return checkinPending(w, scan, slug, mv, nil)
}

func checkinPending(w *config.Wiki, scan *wiki.ScanResult, slug string, mv *mermaid.Validator, afterValidation func() error) (*Result, error) {
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return nil, err
	}
	defer func() {
		if g != nil {
			g.Unlock()
		}
	}()
	scan, err = wiki.Scan(w)
	if err != nil {
		return nil, err
	}
	p, err := scan.Find(slug)
	if err != nil {
		return nil, err
	}
	copyPath, metaPath := paths(w, p.Slug)
	if err := safefile.Regular(copyPath); err != nil {
		return nil, err
	}
	m, err := readMeta(metaPath)
	if err != nil {
		return nil, fmt.Errorf("checkout metadata unavailable for %s: %w; preserve copy and inspect canopy checkout --list", slug, err)
	}
	if p.RelPath != m.RelPath {
		return nil, fmt.Errorf("page moved or removed since checkout (%s); rescue copy before --discard", m.RelPath)
	}
	wikiPath := filepath.Join(w.Root, p.RelPath)
	if err := safefile.Regular(wikiPath); err != nil {
		return nil, err
	}
	current, err := os.ReadFile(wikiPath)
	if err != nil {
		return nil, err
	}
	res := &Result{RelPath: p.RelPath, CleanupPending: true, OperationID: m.OperationID}
	if m.CleanupPending && m.PendingHash == "" && m.AppliedHash == "" {
		res.Unchanged = true
		res.Resumed = true
		if hashOf(current) != m.Base {
			return res, fmt.Errorf("page changed after unchanged checkin; preserve remaining metadata/copy and inspect")
		}
		if data, err := os.ReadFile(copyPath); err == nil && hashOf(data) != m.Base {
			return res, fmt.Errorf("working copy changed after unchanged checkin; preserve edits and inspect")
		} else if err != nil && !os.IsNotExist(err) {
			return res, err
		}
		return res, nil
	}
	if m.PendingHash != "" || m.AppliedHash != "" {
		expected := m.AppliedHash
		if expected == "" {
			expected = m.PendingHash
		}
		if hashOf(current) == expected {
			res.Applied = true
			res.AppliedHash = expected
			res.Resumed = true
			data, err := os.ReadFile(copyPath)
			if err != nil && !os.IsNotExist(err) {
				return res, err
			}
			if err == nil && m.CopyHash != "" && hashOf(data) != m.CopyHash {
				return res, fmt.Errorf("working copy changed after application; preserve it and inspect before cleanup")
			}
			m.AppliedHash = expected
			m.CleanupPending = true
			if err := saveMeta(metaPath, m); err != nil {
				return res, &StageError{"state", err}
			}
			return res, nil
		}
		if hashOf(current) != m.Base || m.AppliedHash != "" {
			res.Applied = m.AppliedHash != ""
			res.AppliedHash = m.AppliedHash
			return res, fmt.Errorf("page changed after recorded checkin; refusing to repeat application or cleanup; inspect page and retained copy")
		}
	}
	copyData, err := os.ReadFile(copyPath)
	if err != nil {
		return nil, fmt.Errorf("working copy missing; inspect --list before --discard: %w", err)
	}
	if hashOf(current) != m.Base {
		return nil, fmt.Errorf("page changed since checkout; re-checkout %s and re-apply after preserving edits; --discard drops this copy", slug)
	}
	if hashOf(copyData) == m.Base {
		res.Unchanged = true
		m.CleanupPending = true
		if err := saveMeta(metaPath, m); err != nil {
			return res, &StageError{"state", err}
		}
		return res, nil
	}
	res.OldLines = strings.Count(string(current), "\n") + 1
	res.NewLines = strings.Count(string(copyData), "\n") + 1
	g.Unlock()
	g = nil
	if err := validate(w, p, copyData, mv, res); err != nil {
		return nil, err
	}
	content, err := wiki.SetFrontmatterField(string(copyData), "updated", time.Now().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	if afterValidation != nil {
		if err := afterValidation(); err != nil {
			return nil, err
		}
	}
	g, err = wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return nil, err
	}
	latest, err := os.ReadFile(wikiPath)
	if err != nil {
		return nil, err
	}
	latestCopy, err := os.ReadFile(copyPath)
	if err != nil {
		return nil, err
	}
	latestMeta, err := readMeta(metaPath)
	if err != nil {
		return nil, err
	}
	if hashOf(latest) != m.Base || hashOf(latestCopy) != hashOf(copyData) || *latestMeta != *m {
		return nil, fmt.Errorf("page or working copy changed during validation; re-checkout and inspect preserved copy")
	}
	m.PendingHash = hashOf([]byte(content))
	m.CopyHash = hashOf(copyData)
	m.OperationID = hashOf([]byte(m.RelPath + "\x00" + m.Base + "\x00" + m.OpenedAt + "\x00" + m.PendingHash))
	res.OperationID = m.OperationID
	if err := saveMeta(metaPath, m); err != nil {
		return res, &StageError{"state", err}
	}
	if err := safefile.WriteFile(wikiPath, []byte(content), 0o644); err != nil {
		res.Applied = safefile.Published(err)
		if res.Applied {
			res.AppliedHash = m.PendingHash
		}
		return res, &StageError{"publish", err}
	}
	res.Applied = true
	res.AppliedHash = m.PendingHash
	m.AppliedHash = m.PendingHash
	m.CleanupPending = true
	if err := saveMeta(metaPath, m); err != nil {
		return res, &StageError{"state", err}
	}
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

// Discard is an explicit destructive choice, including an orphan with no meta.
func Discard(w *config.Wiki, slug string) error {
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return err
	}
	defer g.Unlock()
	// Include known orphan copies in the same address namespace. A removed
	// page can still be discarded, but an ambiguous filename cannot choose
	// which unique draft to destroy.
	scan, err := wiki.Scan(w)
	if err != nil {
		return err
	}
	open, err := Open(w)
	if err != nil {
		return err
	}
	for _, info := range open {
		key := strings.ToLower(info.Slug)
		if _, ok := scan.BySlug[key]; !ok {
			p := &wiki.Page{Slug: info.Slug, RelPath: info.RelPath}
			scan.Pages = append(scan.Pages, p)
			scan.BySlug[key] = p
		}
	}
	p, err := scan.Find(slug)
	if err != nil {
		return err
	}
	copyPath, metaPath := paths(w, p.Slug)
	_, a := os.Lstat(metaPath)
	_, b := os.Lstat(copyPath)
	if os.IsNotExist(a) && os.IsNotExist(b) {
		return fmt.Errorf("not checked out: %s", slug)
	}
	return reclaim(copyPath, metaPath, os.Remove)
}
func reclaim(copyPath, metaPath string, remove func(string) error) error {
	if err := remove(copyPath); err != nil && !os.IsNotExist(err) {
		return &StageError{"cleanup", err}
	}
	if err := remove(metaPath); err != nil && !os.IsNotExist(err) {
		return &StageError{"cleanup", err}
	}
	return nil
}

// Complete reclaims only the known application; it never writes the wiki page.
func Complete(w *config.Wiki, slug string, res *Result) error {
	g, err := wikilock.Acquire(context.Background(), w.Root)
	if err != nil {
		return err
	}
	defer g.Unlock()
	return completeLocked(w, slug, res, os.Remove)
}
func completeLocked(w *config.Wiki, slug string, res *Result, remove func(string) error) error {
	if res == nil || res.RelPath == "" {
		return &StageError{"cleanup", fmt.Errorf("missing validated checkin receipt for %s", slug)}
	}
	// Use the validated application identity, without stripping an extra .md
	// from a legitimate dotted slug.
	copyPath, metaPath := paths(w, strings.TrimSuffix(filepath.Base(res.RelPath), ".md"))
	m, err := readMeta(metaPath)
	if err != nil {
		return &StageError{"cleanup", err}
	}
	if m.RelPath != res.RelPath {
		return &StageError{"cleanup", fmt.Errorf("checkout identity changed; preserve copy and inspect")}
	}
	current, err := os.ReadFile(filepath.Join(w.Root, m.RelPath))
	if err != nil {
		return &StageError{"cleanup", err}
	}
	expected := m.AppliedHash
	if expected == "" {
		expected = m.PendingHash
	}
	if expected == "" {
		expected = m.Base
	}
	if hashOf(current) != expected {
		return &StageError{"cleanup", fmt.Errorf("page changed; preserve checkout and inspect before --discard")}
	}
	data, err := os.ReadFile(copyPath)
	if err != nil && !os.IsNotExist(err) {
		return &StageError{"cleanup", err}
	}
	copyHash := m.CopyHash
	if copyHash == "" {
		copyHash = m.Base
	}
	if err == nil && hashOf(data) != copyHash {
		return &StageError{"cleanup", fmt.Errorf("working copy changed; preserve edits before --discard")}
	}
	if err := reclaim(copyPath, metaPath, remove); err != nil {
		return err
	}
	res.CleanupPending = false
	return nil
}

// Open includes missing/corrupt/unreadable metadata and missing copies instead
// of hiding the only recovery material from the list.
func Open(w *config.Wiki) ([]Info, error) {
	entries, err := os.ReadDir(w.CheckoutDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	slugs := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			slugs[strings.TrimSuffix(e.Name(), ".json")] = true
		}
		if strings.HasSuffix(e.Name(), ".md") {
			slugs[strings.TrimSuffix(e.Name(), ".md")] = true
		}
	}
	var out []Info
	for slug := range slugs {
		copyPath, metaPath := paths(w, slug)
		m, err := readMeta(metaPath)
		if err != nil {
			out = append(out, Info{Slug: slug, Path: copyPath, Modified: true, Status: "orphan", Diagnostic: err.Error(), Rescue: "copy working file to a safe location, inspect wiki, then canopy checkin " + slug + " --discard"})
			continue
		}
		info := Info{Slug: slug, RelPath: m.RelPath, Path: copyPath, Base: m.Base, OpenedAt: m.OpenedAt, Status: "open"}
		if m.CleanupPending || m.PendingHash != "" || m.AppliedHash != "" {
			info.Status = "cleanup_pending"
			info.Diagnostic = "application/postprocessing or cleanup incomplete; checkin resumes only a matching recorded application"
		}
		if data, err := os.ReadFile(copyPath); err == nil {
			info.Modified = hashOf(data) != m.Base
		} else {
			info.Diagnostic = err.Error()
			if info.Status == "open" {
				info.Status = "orphan"
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OpenedAt == out[j].OpenedAt {
			return out[i].Slug < out[j].Slug
		}
		return out[i].OpenedAt < out[j].OpenedAt
	})
	return out, nil
}
