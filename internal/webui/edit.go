package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/neutrospec/canopy/internal/lint"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/writeops"
)

// Web editing mirrors `canopy update --body-file` exactly: body replace
// + updated bump, then the shared writeops.Run pipeline. Frontmatter
// stays CLI-only (docs/web-ui-write-design.md).

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
	raw, err := os.ReadFile(filepath.Join(s.w.Root, p.RelPath))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "edit.html", map[string]any{
		"Title": "edit: " + p.Title,
		"Page":  p,
		"Body":  p.Body,
		"Hash":  contentHash(raw),
	})
}

func (s *Server) handleEditSave(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	path := filepath.Join(s.w.Root, p.RelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		s.fail(w, err)
		return
	}
	newBody := r.FormValue("body")
	// Optimistic lock: reject if the file changed since the form loaded
	// (MediaWiki edit-conflict pattern; no auto-merge by design).
	if r.FormValue("hash") != contentHash(raw) {
		s.render(w, http.StatusConflict, "edit.html", map[string]any{
			"Title":    "edit: " + p.Title,
			"Page":     p,
			"Body":     newBody,
			"Hash":     contentHash(raw),
			"Conflict": true,
		})
		return
	}
	content := wiki.ReplaceBody(string(raw), newBody)
	content = wiki.SetFrontmatterField(content, "updated", time.Now().Format("2006-01-02"))
	// Defensive: body replace cannot break frontmatter, but verify
	// before writing rather than after.
	if parsed := wiki.Parse(p.RelPath, []byte(content)); parsed.FMErr != "" {
		s.fail(w, fmt.Errorf("frontmatter would break: %s", parsed.FMErr))
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		s.fail(w, err)
		return
	}
	// Same pipeline as every CLI mutation.
	postScan, err := writeops.Run(s.w, "update", p.RelPath, p.Tags, "web edit")
	if err != nil {
		s.fail(w, err)
		return
	}
	// Lint findings for this page only, shown once on the result view.
	var findings []string
	for _, f := range lint.Run(s.w, postScan).Findings {
		if f.Page == p.RelPath {
			findings = append(findings, fmt.Sprintf("[%s] %s", f.Kind, f.Message))
		}
	}
	dest := "/page/" + p.Slug
	if len(findings) > 0 {
		s.renderSaved(w, postScan, p, findings)
		return
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

// renderSaved shows the updated page with lint notices attached.
func (s *Server) renderSaved(w http.ResponseWriter, scan *wiki.ScanResult, stale *wiki.Page, findings []string) {
	p, ok := scan.BySlug[wiki.NormalizeLink(stale.Slug)]
	if !ok {
		p = stale
	}
	body, err := RenderPage(p.Body, func(t string) bool { _, ok := scan.BySlug[t]; return ok })
	if err != nil {
		s.fail(w, err)
		return
	}
	backlinks := scan.Backlinks()[wiki.NormalizeLink(p.Slug)]
	nodes, edges := localGraph(scan, p, backlinks)
	s.render(w, http.StatusOK, "page.html", map[string]any{
		"Title":      p.Title,
		"Page":       p,
		"Body":       body,
		"Backlinks":  backlinks,
		"GraphNodes": nodes,
		"GraphEdges": edges,
		"Lint":       findings,
		"Saved":      true,
	})
}
