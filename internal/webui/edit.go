package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/neutrospec/canopy/internal/wiki"
)

// The web editor is a proposal door, not a write door (docs/agent-tasks.md
// "웹 편집 = 제안"; the 2026-07-24 위상 재평가 in web-ui-write-design.md,
// executed): saving files an edit task carrying the submitted body in
// full, and the agent judges and integrates it through the CLI pipeline.
// The page file is never touched here (invariant I2/T6) — which also
// dissolves the old optimistic-lock conflict handling: two proposals
// are just two tasks.

func contentHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, ok := scan.BySlug[wiki.NormalizeLink(r.PathValue("slug"))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, http.StatusOK, "edit.html", map[string]any{
		"Title": "edit: " + p.Title,
		"Page":  p,
		"Body":  p.Body,
	})
}

func (s *Server) handleEditSave(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, ok := scan.BySlug[wiki.NormalizeLink(r.PathValue("slug"))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Browsers submit textarea content with CRLF line endings.
	body := strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n")
	note := strings.TrimSpace(r.FormValue("note"))
	// An unchanged body with no note is a no-op, not a proposal.
	if strings.TrimSpace(body) == strings.TrimSpace(p.Body) && note == "" {
		http.Redirect(w, r, "/page/"+p.Slug, http.StatusSeeOther)
		return
	}
	raw, err := os.ReadFile(filepath.Join(s.w.Root, p.RelPath))
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, err := tasks.FileEdit(s.w, p.Slug, note, body, contentHash(raw), "web", time.Now()); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/page/"+p.Slug+"?tasked=1", http.StatusSeeOther)
}
