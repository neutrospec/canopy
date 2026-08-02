// Package tasks is the wiki's delegated-work queue: judgment-required
// requests filed by any door (web UI button, CLI) and consumed by agent
// loops (docs/agent-tasks.md). One file per task under _meta/tasks/ so
// closing different tasks on different machines can never merge-conflict;
// files are self-versioned and travel with the wiki via git (AGENTS.md
// 규칙 1b) — the queue itself is the interface, no gateway required.
//
// Closing as done is the code's check, not the agent's claim: each task
// type registers a Verifier that confirms the outcome actually exists in
// the wiki (invariant T2, philosophy 원칙 6).
package tasks

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
	"github.com/neutrospec/canopy/internal/wiki"
)

const (
	FormatVersion = 1

	StatusPending   = "pending"
	StatusDone      = "done"
	StatusDismissed = "dismissed"

	TypeConnect = "connect"
	TypeEdit    = "edit"
)

// Task is one delegated request. Payload fields are type-specific and
// only ever added (규칙 1b: additive evolution behind the version field).
type Task struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Door    string `json:"door,omitempty"`   // web | cli | agent
	Created string `json:"created"`          // RFC3339
	Closed  string `json:"closed,omitempty"` // RFC3339, set on done/dismiss
	Note    string `json:"note,omitempty"`   // closer's one-line result/reason

	Pages   []string `json:"pages,omitempty"`   // connect: the pair (sorted); edit: the page
	Request string   `json:"request,omitempty"` // edit: what the user asked for
	Sim     float64  `json:"sim,omitempty"`     // connect: similarity when filed
	Base    string   `json:"base,omitempty"`    // edit: sha256 of the file when filed
	Body    string   `json:"body,omitempty"`    // edit: proposed full body (web editor files
	// the submitted text here instead of writing the page — the agent
	// judges and integrates it; docs/agent-tasks.md "웹 편집 = 제안")
}

func Dir(w *config.Wiki) string { return filepath.Join(w.Root, "_meta", "tasks") }

func pathOf(w *config.Wiki, id string) string { return filepath.Join(Dir(w), id+".json") }

// Load reads every task, oldest first. A missing directory is an empty
// queue; a corrupt file is an error (the queue is judged state, not a
// rebuildable cache — silently skipping would hide lost work).
func Load(w *config.Wiki) ([]*Task, error) {
	entries, err := os.ReadDir(Dir(w))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		t, err := Get(w, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Created != out[j].Created {
			return out[i].Created < out[j].Created
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Get reads one task tolerantly: unknown fields and future versions
// still load — only closing as done is version/type-gated.
func Get(w *config.Wiki, id string) (*Task, error) {
	data, err := os.ReadFile(pathOf(w, id))
	if err != nil {
		return nil, err
	}
	t := &Task{}
	if err := json.Unmarshal(data, t); err != nil {
		return nil, err
	}
	return t, nil
}

// PendingCount is the status/banner signal (invariant T7).
func PendingCount(w *config.Wiki) (int, error) {
	list, err := Load(w)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range list {
		if t.Status == StatusPending {
			n++
		}
	}
	return n, nil
}

// Involves reports whether the task references the page (lowercased slug).
func (t *Task) Involves(slug string) bool {
	slug = strings.ToLower(slug)
	for _, p := range t.Pages {
		if strings.ToLower(p) == slug {
			return true
		}
	}
	return false
}

func writeTask(w *config.Wiki, id string, v any) error {
	if err := os.MkdirAll(Dir(w), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pathOf(w, id), append(data, '\n'), 0o644)
}

// ConnectID is deterministic (sorted pair) so refiling the same pair is
// a no-op — including after a dismiss, which is a judgment to respect
// (invariant T4).
func ConnectID(a, b string) string {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if b < a {
		a, b = b, a
	}
	return "connect-" + a + "--" + b
}

// FileConnect files a connect task. Filing never edits pages (T6).
// created=false means a task for this pair already exists (any status).
func FileConnect(w *config.Wiki, a, b string, sim float64, door string, now time.Time) (t *Task, created bool, err error) {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return nil, false, fmt.Errorf("connect needs two different pages")
	}
	if b < a {
		a, b = b, a
	}
	id := ConnectID(a, b)
	if existing, err := Get(w, id); err == nil {
		return existing, false, nil
	} else if !os.IsNotExist(err) {
		return nil, false, err
	}
	t = &Task{
		Version: FormatVersion, ID: id, Type: TypeConnect, Status: StatusPending,
		Door: door, Created: now.Format(time.RFC3339), Pages: []string{a, b}, Sim: sim,
	}
	return t, true, writeTask(w, id, t)
}

// FileEdit files an edit request: an instruction (request), a proposed
// full body (body — the web editor's "저장"), or both. base is the page
// file's sha256 at filing time — verifyEdit later requires the content
// to have moved.
func FileEdit(w *config.Wiki, slug, request, body, base, door string, now time.Time) (*Task, error) {
	slug = strings.ToLower(slug)
	if strings.TrimSpace(request) == "" && strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("edit task needs a request or a proposed body")
	}
	stamp := now.Format("20060102-150405")
	id := fmt.Sprintf("edit-%s-%s", slug, stamp)
	for i := 2; ; i++ {
		if _, err := os.Stat(pathOf(w, id)); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("edit-%s-%s-%d", slug, stamp, i)
	}
	t := &Task{
		Version: FormatVersion, ID: id, Type: TypeEdit, Status: StatusPending,
		Door: door, Created: now.Format(time.RFC3339), Pages: []string{slug},
		Request: request, Base: base, Body: body,
	}
	return t, writeTask(w, id, t)
}

// Close marks a task done or dismissed. done must pass the type's
// Verifier (T2); an unknown type refuses done but allows dismiss —
// dismiss records a judgment, done claims an outcome (T5). The write
// patches the raw JSON so fields this binary doesn't know survive a
// mixed-version fleet (규칙 1b).
func Close(w *config.Wiki, scan *wiki.ScanResult, id, status, note string, now time.Time) (*Task, error) {
	if status != StatusDone && status != StatusDismissed {
		return nil, fmt.Errorf("invalid status %q", status)
	}
	data, err := os.ReadFile(pathOf(w, id))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	t := &Task{}
	if err := json.Unmarshal(data, t); err != nil {
		return nil, err
	}
	if t.Status != StatusPending {
		return nil, fmt.Errorf("task %s is already %s", id, t.Status)
	}
	if status == StatusDone {
		v, ok := verifiers[t.Type]
		if !ok {
			return nil, fmt.Errorf("unknown task type %q — filed by a newer canopy; upgrade before closing as done (dismiss is allowed)", t.Type)
		}
		if err := v(w, scan, t); err != nil {
			return nil, fmt.Errorf("done rejected (%s): %w", t.Type, err)
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	t.Status, t.Closed = status, now.Format(time.RFC3339)
	raw["status"], raw["closed"] = t.Status, t.Closed
	if note != "" {
		t.Note = note
		raw["note"] = note
	}
	return t, writeTask(w, id, raw)
}

// GC removes closed tasks whose Closed time is older than keep.
// Pending tasks are never removed (invariant T8).
func GC(w *config.Wiki, keep time.Duration, now time.Time) (int, error) {
	list, err := Load(w)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, t := range list {
		if t.Status == StatusPending || t.Closed == "" {
			continue
		}
		closed, err := time.Parse(time.RFC3339, t.Closed)
		if err != nil || now.Sub(closed) < keep {
			continue
		}
		if err := os.Remove(pathOf(w, t.ID)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// --- per-type done verification (the registry new types extend) ---

// Verifier confirms, in code, that a task's outcome is present in the
// wiki. Adding a task type = a payload shape + one entry here + a row
// in invariants T.
type Verifier func(w *config.Wiki, scan *wiki.ScanResult, t *Task) error

var verifiers = map[string]Verifier{
	TypeConnect: verifyConnect,
	TypeEdit:    verifyEdit,
}

func verifyConnect(w *config.Wiki, scan *wiki.ScanResult, t *Task) error {
	if len(t.Pages) != 2 {
		return fmt.Errorf("connect task needs 2 pages, has %d", len(t.Pages))
	}
	a, b := strings.ToLower(t.Pages[0]), strings.ToLower(t.Pages[1])
	pa, ok := scan.BySlug[a]
	if !ok {
		return fmt.Errorf("page not found: %s", a)
	}
	pb, ok := scan.BySlug[b]
	if !ok {
		return fmt.Errorf("page not found: %s", b)
	}
	if !hasLink(pa, b) {
		return fmt.Errorf("%s has no [[%s]] link yet", pa.Slug, pb.Slug)
	}
	if !hasLink(pb, a) {
		return fmt.Errorf("%s has no [[%s]] link yet", pb.Slug, pa.Slug)
	}
	return nil
}

func hasLink(p *wiki.Page, target string) bool {
	for _, l := range p.Links {
		if l == target {
			return true
		}
	}
	return false
}

func verifyEdit(w *config.Wiki, scan *wiki.ScanResult, t *Task) error {
	if len(t.Pages) != 1 {
		return fmt.Errorf("edit task needs 1 page, has %d", len(t.Pages))
	}
	p, ok := scan.BySlug[strings.ToLower(t.Pages[0])]
	if !ok {
		return fmt.Errorf("page not found: %s (dismiss if removing it was the judgment)", t.Pages[0])
	}
	if t.Base == "" {
		return nil // old/handmade task without a baseline: existence is all we can check
	}
	raw, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
	if err != nil {
		return err
	}
	if hashBytes(raw) == t.Base {
		return fmt.Errorf("%s is unchanged since the request was filed", p.Slug)
	}
	return nil
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
