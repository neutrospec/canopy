package webui

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/neutrospec/canopy/internal/wiki"
)

// The web door files delegated tasks; agents perform them; `canopy tasks
// done` verifies the outcome (docs/agent-tasks.md). Filing never edits
// pages (invariant T6), so these handlers only write _meta/tasks/.

// taskView is the page-attached todo-list entry.
type taskView struct {
	ID        string
	TypeLabel string // localized chip text
	Other     string // connect: the other page's slug (linkified)
	Request   string // edit: the user's request
	Created   string // date only
}

// handleTaskConnect files a connect task for a suggestion the user
// endorsed (the click IS the human judgment; wording/placement stays
// with the agent).
func (s *Server) handleTaskConnect(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	a := wiki.NormalizeLink(r.FormValue("a"))
	b := wiki.NormalizeLink(r.FormValue("b"))
	pa, okA := scan.BySlug[a]
	_, okB := scan.BySlug[b]
	if !okA || !okB || a == b {
		http.Error(w, "bad pair", http.StatusBadRequest)
		return
	}
	sim, _ := strconv.ParseFloat(r.FormValue("sim"), 64)
	if _, _, err := tasks.FileConnect(s.w, a, b, sim, "web", time.Now()); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/page/"+pa.Slug+"?tasked=1", http.StatusSeeOther)
}

// handleTaskEdit files an edit request against a page, capturing the
// current content hash so `done` can require the page to have moved.
func (s *Server) handleTaskEdit(w http.ResponseWriter, r *http.Request) {
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
	request := strings.TrimSpace(r.FormValue("request"))
	if request == "" {
		http.Redirect(w, r, "/page/"+p.Slug, http.StatusSeeOther)
		return
	}
	raw, err := os.ReadFile(filepath.Join(s.w.Root, p.RelPath))
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, err := tasks.FileEdit(s.w, p.Slug, request, "", contentHash(raw), "web", time.Now()); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/page/"+p.Slug+"?tasked=1", http.StatusSeeOther)
}

// taskRow is one entry on the /special/tasks screen.
type taskRow struct {
	ID          string
	TypeLabel   string
	StatusLabel string
	Pages       []string
	Request     string
	Body        string // edit proposal text (web editor submissions)
	Note        string
	Door        string
	Created     string // date only
	Closed      string // date only
	// CanCancel: the requester may withdraw a pending edit request from
	// the web. Connect tasks are not cancellable here — a web dismiss
	// would permanently suppress the pair's button (T4), which is the
	// agent's judgment to make.
	CanCancel bool
}

func dateOnly(ts string) string {
	if i := strings.Index(ts, "T"); i > 0 {
		return ts[:i]
	}
	return ts
}

func (s *Server) typeLabel(lc *i18n.Localizer, typ string) string {
	switch typ {
	case tasks.TypeConnect:
		return localizeString(lc, "task_type_connect")
	case tasks.TypeEdit:
		return localizeString(lc, "task_type_edit")
	default: // future type from a newer canopy: show it raw, don't hide it
		return typ
	}
}

// handleTasksPage renders the queue: pending first (oldest first, the
// loop's processing order), then recently closed for a sense of motion.
func (s *Server) handleTasksPage(w http.ResponseWriter, r *http.Request) {
	list, err := tasks.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	lc := s.loc(r)
	var pending, closed []taskRow
	for _, t := range list {
		row := taskRow{
			ID: t.ID, TypeLabel: s.typeLabel(lc, t.Type), Pages: t.Pages,
			Request: t.Request, Body: t.Body, Note: t.Note, Door: t.Door,
			Created: dateOnly(t.Created), Closed: dateOnly(t.Closed),
		}
		if t.Status == tasks.StatusPending {
			row.CanCancel = t.Type == tasks.TypeEdit
			pending = append(pending, row)
			continue
		}
		row.StatusLabel = localizeString(lc, "tasks_status_"+t.Status)
		closed = append(closed, row)
	}
	// closed: newest first, capped — this is a pulse, not an archive
	sort.SliceStable(closed, func(i, j int) bool { return closed[i].Closed > closed[j].Closed })
	if len(closed) > 30 {
		closed = closed[:30]
	}
	s.render(w, r, http.StatusOK, "tasks.html", map[string]any{
		"Title":   localizeString(lc, "tasks_title"),
		"Pending": pending,
		"Closed":  closed,
	})
}

// handleTaskCancel withdraws a pending edit request from the web (the
// requester changing their mind). Everything else closes through the
// CLI where done is verified.
func (s *Server) handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	t, err := tasks.Get(s.w, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if t.Type != tasks.TypeEdit || t.Status != tasks.StatusPending {
		http.Error(w, "only pending edit requests can be withdrawn here", http.StatusBadRequest)
		return
	}
	if _, err := tasks.Close(s.w, nil, t.ID, tasks.StatusDismissed, "웹에서 철회", "web", time.Now()); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/special/tasks", http.StatusSeeOther)
}

// pageTasks returns the page's pending todo list plus the set of slugs
// with an already-filed connect task (any status — dismissed pairs must
// not re-offer the button). Best-effort: a broken queue never breaks
// page serving.
func (s *Server) pageTasks(slug string, lc *i18n.Localizer) (views []taskView, filed map[string]bool) {
	filed = map[string]bool{}
	list, err := tasks.Load(s.w)
	if err != nil {
		return nil, filed
	}
	slug = strings.ToLower(slug)
	for _, t := range list {
		if !t.Involves(slug) {
			continue
		}
		if t.Type == tasks.TypeConnect {
			for _, p := range t.Pages {
				if strings.ToLower(p) != slug {
					filed[strings.ToLower(p)] = true
				}
			}
		}
		if t.Status != tasks.StatusPending {
			continue
		}
		v := taskView{ID: t.ID, TypeLabel: s.typeLabel(lc, t.Type), Created: dateOnly(t.Created)}
		switch t.Type {
		case tasks.TypeConnect:
			for _, p := range t.Pages {
				if strings.ToLower(p) != slug {
					v.Other = p
				}
			}
		default:
			v.Request = t.Request
			if v.Request == "" && t.Body != "" {
				v.Request = localizeString(lc, "task_proposal_marker")
			}
		}
		views = append(views, v)
	}
	return views, filed
}
