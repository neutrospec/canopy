package webui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/config"
	cembed "github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/indexer"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/resurface"
	"github.com/neutrospec/canopy/internal/search"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/neutrospec/canopy/internal/wiki"
)

//go:embed templates/* static/*
var assets embed.FS

// Server renders the wiki read-only over HTTP. The engine may be nil,
// in which case search degrades to keyword-only (same as the CLI).
type Server struct {
	w   *config.Wiki
	eng cembed.Engine
	mu  sync.Mutex // serializes this server's explicit/automatic read updates
	// tmpl is keyed by locale then template name: each locale gets its own
	// parsed set with `t` bound to that locale's localizer (html/template
	// binds funcs at parse time). See docs/web-ui-i18n.md.
	tmpl map[string]map[string]*template.Template
	i18n *i18nBundle

	auth           *authStore
	authRequired   bool
	tokenDigest    *[32]byte
	publicOrigin   string
	hosts          *hostPolicy
	searchSlots    chan struct{}
	indexSlot      chan struct{}
	staticTags     map[string]string
	cacheMu        sync.Mutex // serializes cache initialization, never inference
	workMu         sync.Mutex
	workCtx        context.Context
	workCancel     context.CancelFunc
	workWG         sync.WaitGroup
	closed         bool
	refreshing     bool
	refreshPending bool
	searches       int

	dailyMu sync.Mutex
	daily   daily
}

var pageTemplates = []string{"home.html", "page.html", "search.html", "browse.html", "recent.html", "attention.html", "edit.html", "login.html", "setup.html", "discover.html", "gaps.html", "graph.html", "history.html", "tasks.html"}

func NewServer(w *config.Wiki, eng cembed.Engine) (*Server, error) {
	ib, err := loadBundle()
	if err != nil {
		return nil, err
	}
	s := &Server{w: w, eng: eng, i18n: ib, tmpl: map[string]map[string]*template.Template{}, searchSlots: make(chan struct{}, 8), indexSlot: make(chan struct{}, 1)}
	s.staticTags = map[string]string{}
	if err := fs.WalkDir(assets, "static", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		s.staticTags["/"+path] = `"` + contentHash(data) + `"`
		return nil
	}); err != nil {
		return nil, err
	}
	s.staticTags["/static/vendor/mermaid.min.js"] = `"` + contentHash(mermaid.Bundle) + `"`
	for _, lang := range ib.langs {
		loc := ib.localizer(lang)
		funcs := template.FuncMap{"short": short, "pathref": url.PathEscape, "assetURL": assetURL, "pageURL": func(ref string) string { return "/page/" + url.PathEscape(ref) }}
		for k, v := range i18nFuncMap(loc) {
			funcs[k] = v
		}
		set := map[string]*template.Template{}
		for _, name := range pageTemplates {
			t, err := template.New("base.html").Funcs(funcs).ParseFS(assets, "templates/base.html", "templates/"+name)
			if err != nil {
				return nil, err
			}
			set[name] = t
		}
		s.tmpl[lang] = set
	}
	s.workCtx, s.workCancel = context.WithCancel(context.Background())
	return s, nil
}

// Close cancels server-owned inference at its next boundary and joins all
// engine consumers. The caller may close the borrowed engine afterwards.
func (s *Server) Close() {
	s.workMu.Lock()
	s.closed = true
	s.workCancel()
	s.workMu.Unlock()
	s.workWG.Wait()
}

// loc returns the localizer for the request's resolved locale — for Go
// handlers that build dynamic display strings (invariant M: dynamic strings
// are localized where the data lives, static labels in templates via {{t}}).
func (s *Server) loc(r *http.Request) *i18n.Localizer {
	return s.i18n.localizer(s.i18n.resolveLang(r))
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	staticFiles := http.FileServerFS(assets)
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if tag := s.staticTags[r.URL.Path]; tag != "" {
			w.Header().Set("ETag", tag)
		}
		staticFiles.ServeHTTP(w, r)
	}))
	// The mermaid bundle lives in internal/mermaid (which validates wiki
	// diagrams with it); serving that same copy keeps parser and renderer
	// in lockstep (invariant P3). More specific pattern wins over /static/.
	mux.HandleFunc("GET /static/vendor/mermaid.min.js", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("ETag", s.staticTags["/static/vendor/mermaid.min.js"])
		http.ServeContent(rw, r, "mermaid.min.js", time.Time{}, bytes.NewReader(mermaid.Bundle))
	})
	mux.HandleFunc("GET /page/{slug}", s.handlePage)
	mux.HandleFunc("GET /search", s.handleSearch)
	mux.HandleFunc("GET /api/search", s.handleAPISearch)
	mux.HandleFunc("GET /api/preview/{slug}", s.handleAPIPreview)
	mux.HandleFunc("GET /edit/{slug}", s.handleEditForm)
	mux.HandleFunc("POST /edit/{slug}", s.handleEditSave)
	mux.HandleFunc("GET /browse", s.handleBrowse)
	mux.HandleFunc("GET /tag/{tag}", s.handleTag)
	mux.HandleFunc("GET /special/recent", s.handleRecent)
	mux.HandleFunc("GET /special/attention", s.handleAttention)
	mux.HandleFunc("GET /special/random", s.handleRandom)
	mux.HandleFunc("GET /graph", s.handleGraphPage)
	mux.HandleFunc("GET /api/graph", s.handleAPIGraph)
	mux.HandleFunc("GET /history", s.handleHistory)
	mux.HandleFunc("GET /special/discover", s.handleDiscover)
	mux.HandleFunc("GET /special/gaps", s.handleGaps)
	mux.HandleFunc("POST /read/{slug}", s.handleReadMark)
	mux.HandleFunc("POST /api/read/{slug}", s.handleReadAuto)
	mux.HandleFunc("POST /resurface/feedback", s.handleResurfaceFeedback)
	mux.HandleFunc("POST /task/connect", s.handleTaskConnect)
	mux.HandleFunc("POST /task/edit/{slug}", s.handleTaskEdit)
	mux.HandleFunc("POST /task/cancel/{id}", s.handleTaskCancel)
	mux.HandleFunc("GET /special/tasks", s.handleTasksPage)
	mux.HandleFunc("GET /setup", s.handleSetupForm)
	mux.HandleFunc("POST /setup", s.handleSetupSave)
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /setlang", s.handleSetLang)
	mux.HandleFunc("GET /{$}", s.handleHome)
	return logRequests(s.hostGuard(s.guard(s.requestLimits(mux))))
}

// assetURL changes with the exact embedded bytes, so a same-origin upgrade
// cannot reuse stale application CSS/JS under an unchanged URL.
func assetURL(path string) string {
	data, err := assets.ReadFile(strings.TrimPrefix(path, "/"))
	if err != nil {
		return path
	}
	return path + "?v=" + contentHash(data)[:12]
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("%s %s", r.Method, r.URL)
	})
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, data any) {
	lang := s.i18n.resolveLang(r)
	if m, ok := data.(map[string]any); ok {
		m["AuthOn"] = s.authRequired && !s.bearerOK(r)
		// Language-selector data for base.html (invariant M4: the loaded
		// locale list drives the menu, so a new file just appears).
		m["Lang"] = lang
		m["LangName"] = endonym(lang)
		m["Langs"] = s.i18n.options()
		m["Path"] = r.URL.RequestURI()
		m["View"] = strings.TrimSuffix(name, ".html")
		m["ActiveNav"] = activeNav(name)
		messages := map[string]string{}
		lc := s.i18n.localizer(lang)
		for _, id := range clientMessageIDs {
			messages[id] = localizeString(lc, id)
		}
		// html/template serializes this map in the JSON script context,
		// including escaping any HTML embedded in translations.
		m["ClientMessages"] = messages
	}
	set := s.tmpl[lang]
	if set == nil {
		set = s.tmpl[defaultLang]
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	// Template-only links can disambiguate lawful slugs ending in .md by
	// using the governed path. Load the map lazily, only for such names.
	tmpl, err := set[name].Clone()
	if err != nil {
		log.Printf("template clone: %v", err)
		return
	}
	var refs *wiki.ScanResult
	var refsOnce sync.Once
	tmpl = tmpl.Funcs(template.FuncMap{"pageURL": func(ref string) string {
		if strings.HasSuffix(strings.ToLower(ref), ".md") {
			refsOnce.Do(func() { refs, _ = wiki.Scan(s.w) })
			if refs != nil {
				if p := refs.BySlug[strings.ToLower(ref)]; p != nil {
					return pagePath(p)
				}
			}
		}
		return "/page/" + url.PathEscape(ref)
	}})
	if err := tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("template %s: %v", name, err)
	}
}

var clientMessageIDs = []string{
	"copy_button", "copy_done", "copy_failed", "heading_link", "page_toc_title",
	"page_read_label", "page_auto", "page_reread", "page_reread_title",
	"page_cancel", "page_unread_title", "graph_links", "graph_touches",
	"graph_legend_unread", "graph_legend_island", "graph_load_failed", "graph_status_unknown", "graph_available_touches", "reads_partial",
	"page_read_failed", "page_read_auth", "diagram_failed", "math_failed", "math_source",
}

func activeNav(name string) string {
	switch name {
	case "page.html", "edit.html":
		return "browse"
	case "gaps.html":
		return "attention"
	default:
		return strings.TrimSuffix(name, ".html")
	}
}

// handleSetLang is the language selector: set the lang cookie and return to
// the page the user was on (the one new setting, docs/web-ui-i18n.md).
func (s *Server) handleSetLang(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if !s.i18n.has(lang) {
		lang = defaultLang
	}
	http.SetCookie(w, &http.Cookie{Name: langCookie, Value: lang, Path: "/", MaxAge: 86400 * 365, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	next := r.URL.Query().Get("next")
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// --- home ---

type dirStat struct {
	Dir   string
	Count int
	Read  int // read pages in this dir — fills its canopy-band segment
	Pct   int // read percentage within the dir
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	vectors, current, indexErr := s.currentVectors(r.Context(), scan)
	if indexErr != nil {
		log.Printf("home index: %v", indexErr)
	}
	rs, readErr := reads.Load(s.w)
	if readErr != nil {
		log.Printf("home reads: %v", readErr)
	}
	now := time.Now()
	byDir := map[string]int{}
	readByDir := map[string]int{}
	readTotal := 0
	for _, p := range scan.Pages {
		byDir[p.Dir]++
		if rs.IsRead(p.Slug) {
			readByDir[p.Dir]++
			readTotal++
		}
	}
	var dirs []dirStat
	for _, d := range s.w.Cfg.Schema.PageDirs {
		st := dirStat{Dir: d, Count: byDir[d], Read: readByDir[d]}
		if st.Count > 0 {
			st.Pct = st.Read * 100 / st.Count
		}
		dirs = append(dirs, st)
	}

	// 오늘의 주의: distinct pages touched and searches asked today,
	// from the local event log (best-effort).
	todayPages, todaySearches := 0, 0
	if ev, err := attention.Open(s.w.AttentionDBPath()); err == nil {
		// AttnOnly: lifecycle events (task/sync) are not attention (N3).
		if events, err := ev.Query(attention.Filter{Limit: 400, AttnOnly: true}); err == nil {
			day := now.Format("2006-01-02")
			seen := map[string]bool{}
			for _, e := range events {
				t, err := time.Parse(time.RFC3339, e.TS)
				if err != nil || t.Local().Format("2006-01-02") != day {
					continue
				}
				if e.Kind == attention.KindSearch {
					todaySearches++
				} else if !seen[e.Slug] {
					seen[e.Slug] = true
					todayPages++
				}
			}
		}
		ev.Close()
	}

	recent := append([]*wiki.Page(nil), scan.Pages...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].Updated > recent[j].Updated })
	if len(recent) > 10 {
		recent = recent[:10]
	}
	// Locale-dependent display strings are built here, where the data is,
	// and passed in finished (docs/web-ui-i18n.md); static labels use {{t}}.
	lc := s.loc(r)
	discover := s.discoverRanked(scan, rs, vectors, now, lc)
	if len(discover) > 4 {
		discover = discover[:4]
	}
	var pick *resurface.Pick
	var bridge *resurface.Bridge
	if r.Method != http.MethodHead && readErr == nil {
		pick, bridge = s.todaysCard(scan, rs, vectors)
	}
	pendingCount, tasksErr := tasks.PendingCount(s.w)
	if tasksErr != nil {
		log.Printf("home task summary: %v", tasksErr)
	}

	date := now.Format("2006-01-02") + " (" + localizeString(lc, "wd_"+strconv.Itoa(int(now.Weekday()))) + ")"
	readProgress := localizeString(lc, "home_read_progress", "Read", readTotal, "Total", len(scan.Pages))
	todayLine := ""
	if todayPages > 0 || todaySearches > 0 {
		var parts []string
		if todayPages > 0 {
			parts = append(parts, localizeString(lc, "home_today_pages", "Count", todayPages))
		}
		if todaySearches > 0 {
			parts = append(parts, localizeString(lc, "home_today_searches", "Count", todaySearches))
		}
		todayLine = localizeString(lc, "home_today", "Detail", strings.Join(parts, " · "))
	}

	s.render(w, r, http.StatusOK, "home.html", map[string]any{
		"Title":             localizeString(lc, "nav_home"),
		"Date":              date,
		"Total":             len(scan.Pages),
		"ReadTotal":         readTotal,
		"UnreadTotal":       len(scan.Pages) - readTotal,
		"ReadAvailable":     rs.HumanAvailable(),
		"ReadsPartial":      readErr != nil,
		"SemanticAvailable": indexErr == nil && current != nil && current.SemanticReady,
		"PendingCount":      pendingCount,
		"TasksAvailable":    tasksErr == nil,
		"Dirs":              dirs,
		"ReadProgress":      readProgress,
		"TodayLine":         todayLine,
		"Recent":            recent,
		"Discover":          discover,
		"Pick":              pick,
		"Bridge":            bridge,
	})
}

// --- page view ---

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	vectors, current, indexErr := s.currentVectors(r.Context(), scan)
	if indexErr != nil {
		log.Printf("page suggestions: %v", indexErr)
	}
	p, err := scan.Find(r.PathValue("slug"))
	if err != nil {
		// Wikipedia pattern: a missing page is a search, not a dead end.
		s.searchFallback(w, r, r.PathValue("slug"))
		return
	}
	body, err := RenderPageWithResolver(p.Body, func(target string) (string, string, bool) {
		linked, err := scan.Find(target)
		if err != nil {
			return wiki.NormalizeLink(target), target, false
		}
		reference := linked.Slug
		if strings.HasSuffix(strings.ToLower(reference), ".md") {
			reference = linked.RelPath
		}
		return strings.ToLower(linked.Slug), reference, true
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	slug := strings.ToLower(p.Slug)
	backlinks := scan.Backlinks()[slug]
	nodes, edges := localGraph(scan, p, backlinks)
	rs, readErr := reads.Load(s.w)
	if readErr != nil {
		log.Printf("page reads: %v", readErr)
	}
	// A page open is exposure-plus, not a read (원칙 12): it goes to the
	// machine-local event log only, never into the read aggregates.
	s.logEvent(slug, attention.KindView, "")
	var agentDays int
	if a := rs.Agent[slug]; a != nil {
		agentDays = a.Days
	}
	// Per-page attention panel (M12): recent trail + 12-week sparkline
	// from the local event log, best-effort.
	lc := s.loc(r)
	var attnRecent []histEntry
	var spark template.HTML
	if ev, err := attention.Open(s.w.AttentionDBPath()); err == nil {
		if events, err := ev.BySlug(slug, 6); err == nil {
			for _, e := range events {
				attnRecent = append(attnRecent, histEntryOf(e, "2006-01-02", lc))
			}
		}
		if counts, err := ev.WeeklyCounts(slug, 12, time.Now()); err == nil {
			spark = sparkSVG(counts, localizeString(lc, "spark_aria"))
		}
		ev.Close()
	}
	// Delegated-task wiring (docs/agent-tasks.md): pending tasks show as
	// the page's todo list, and suggestions with a filed connect task
	// swap their button for a "requested" marker.
	pageTasks, filed, tasksErr := s.pageTasks(slug, lc)
	if tasksErr != nil {
		log.Printf("page task summary: %v", tasksErr)
	}
	suggested := s.suggestLinks(scan, p, backlinks, vectors)
	for i := range suggested {
		suggested[i].Requested = filed[strings.ToLower(suggested[i].Slug)]
	}
	var receipt *tasks.Task
	if id := r.URL.Query().Get("tasked"); id != "" {
		if task, err := tasks.Get(s.w, id); err == nil && task.Involves(slug) {
			receipt = task
		}
	}
	receiptStatus := ""
	if receipt != nil {
		receiptStatus = taskStatusLabel(lc, receipt.Status)
	}
	s.render(w, r, http.StatusOK, "page.html", map[string]any{
		"Title":             p.Title,
		"Page":              p,
		"Body":              body,
		"Backlinks":         backlinks,
		"GraphNodes":        nodes,
		"GraphEdges":        edges,
		"Read":              rs.Get(slug),
		"ReadAvailable":     rs.HumanAvailable(),
		"AgentAvailable":    rs.AgentAvailable(),
		"ReadsPartial":      readErr != nil,
		"SemanticAvailable": indexErr == nil && current != nil && current.SemanticReady,
		"ReadAgo":           readAgo(rs.Get(slug), time.Now(), lc),
		"AgentDays":         agentDays,
		"AttnRecent":        attnRecent,
		"Spark":             spark,
		"ReadSecs":          readThresholdSecs(p),
		"Suggested":         suggested,
		"Tasks":             pageTasks,
		"TasksAvailable":    tasksErr == nil,
		"TaskReceipt":       receipt,
		"TaskReceiptStatus": receiptStatus,
	})
}

// readAgo renders "when did I last read this" for the page header:
// 오늘 / 어제 / N일 전.
func readAgo(r *reads.Read, now time.Time, loc *i18n.Localizer) string {
	if r == nil {
		return ""
	}
	t, err := time.Parse(time.RFC3339, r.Last)
	if err != nil {
		return ""
	}
	days := int(now.Sub(t).Hours() / 24)
	switch {
	case days <= 0:
		return localizeString(loc, "readago_today")
	case days == 1:
		return localizeString(loc, "readago_yesterday")
	default:
		return localizeString(loc, "readago_days", "Days", days)
	}
}

// logEvent appends a web-door access event to the machine-local log,
// best-effort: the event DB enriches instruments (web-ui-plan-4.md) but
// must never break page serving.
func (s *Server) logEvent(slug, kind, meta string) {
	ev, err := attention.Open(s.w.AttentionDBPath())
	if err != nil {
		return
	}
	defer ev.Close()
	ev.Log(time.Now(), slug, attention.DoorWeb, kind, meta)
}

// readThresholdSecs scales the auto-read dwell requirement with page
// length: floor 30s, +1s per 8 lines, capped at 150s.
func readThresholdSecs(p *wiki.Page) int {
	secs := 30 + p.Lines/8
	if secs > 150 {
		secs = 150
	}
	return secs
}

// --- search ---

type result struct {
	Slug    string
	Title   string
	Score   float64
	Snippet string
	URL     string
}

// withCurrentIndex serializes access without making canceled requests wait on
// a mutex and then start inference. The queue has a separate bounded budget;
// native work already running can outlive cancellation until its next boundary.
func (s *Server) withCurrentIndex(ctx context.Context, scan *wiki.ScanResult, use func(context.Context, *store.Store, *indexer.Result, bool) error) error {
	s.workMu.Lock()
	if s.closed {
		s.workMu.Unlock()
		return context.Canceled
	}
	s.workWG.Add(1)
	s.searches++
	background := s.refreshing
	acquired := false
	if !background {
		select {
		case s.indexSlot <- struct{}{}:
			acquired = true
		default:
		}
	}
	s.workMu.Unlock()
	defer func() {
		s.workMu.Lock()
		s.searches--
		pending := s.refreshPending && s.searches == 0
		s.workMu.Unlock()
		if pending {
			s.refreshIndex()
		}
		s.workWG.Done()
	}()
	ctx, cancelWork := context.WithCancel(ctx)
	stopOnClose := context.AfterFunc(s.workCtx, cancelWork)
	defer stopOnClose()
	defer cancelWork()
	if background {
		return s.withCachedIndex(ctx, scan, func(work context.Context, st *store.Store, current *indexer.Result) error {
			return use(work, st, current, false)
		})
	}
	queued, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !acquired {
		select {
		case s.indexSlot <- struct{}{}:
		case <-queued.Done():
			return queued.Err()
		}
	}
	defer func() { <-s.indexSlot }()
	if err := queued.Err(); err != nil {
		return err
	}
	work, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	st, err := s.openIndex()
	if err != nil {
		return err
	}
	defer st.Close()
	current, err := indexer.EnsureCurrentContext(work, s.w, st, scan, s.eng, nil)
	if err != nil {
		return err
	}
	if err := work.Err(); err != nil {
		return err
	}
	return use(work, st, current, true)
}

func (s *Server) openIndex() (*store.Store, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	return store.Open(s.w.DBPath())
}

func (s *Server) withCachedIndex(ctx context.Context, scan *wiki.ScanResult, use func(context.Context, *store.Store, *indexer.Result) error) error {
	st, err := s.openIndex()
	if err != nil {
		return err
	}
	defer st.Close()
	current, err := indexer.InspectCurrentContext(ctx, s.w, st, scan, s.eng)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return use(ctx, st, current)
}

// A refresh borrows the inference slot only when it is idle. Navigation never
// queues behind it. Remember demand while a search holds the slot, since that
// request may leave or hit its deadline before it finishes filling the cache.
func (s *Server) refreshIndex() {
	s.workMu.Lock()
	if s.closed || s.refreshing || s.eng == nil {
		s.workMu.Unlock()
		return
	}
	if s.searches != 0 {
		s.refreshPending = true
		s.workMu.Unlock()
		return
	}
	select {
	case s.indexSlot <- struct{}{}:
	default:
		s.workMu.Unlock()
		return
	}
	s.refreshing = true
	s.refreshPending = false
	s.workWG.Add(1)
	s.workMu.Unlock()
	go func() {
		defer s.workWG.Done()
		defer func() {
			s.workMu.Lock()
			s.refreshing = false
			<-s.indexSlot
			s.workMu.Unlock()
		}()
		st, err := s.openIndex()
		if err == nil {
			defer st.Close()
			_, err = indexer.EnsureCurrentContext(s.workCtx, s.w, st, nil, s.eng, nil)
		}
		if err != nil && s.workCtx.Err() == nil {
			log.Printf("background index: %v", err)
		}
	}()
}

// currentVectors is the shared evidence boundary for discovery, bridges, and
// suggestions. An incomplete/profile-stale corpus must not masquerade as a
// complete semantic ranking; non-vector discovery signals still work.
func (s *Server) currentVectors(ctx context.Context, scan *wiki.ScanResult) (map[string][]float32, *indexer.Result, error) {
	var vectors map[string][]float32
	var current *indexer.Result
	err := s.withCachedIndex(ctx, scan, func(_ context.Context, st *store.Store, res *indexer.Result) error {
		current = res
		if !res.SemanticReady {
			return nil
		}
		var err error
		vectors, err = st.PageVectorsFor(scan.Pages)
		if err == nil {
			expected := 0
			for _, p := range scan.Pages {
				if strings.TrimSpace(p.Body) != "" {
					expected++
				}
			}
			// A concurrent metadata refresh can invalidate a page after the
			// inspection. Never label the resulting partial map complete.
			current.EmbeddedPages = len(vectors)
			current.NeedsReindex = expected - len(vectors)
			current.SemanticReady = current.NeedsReindex == 0
			if !current.SemanticReady {
				vectors = nil
			}
		}
		return err
	})
	if err == nil && current != nil && !current.SemanticReady {
		s.refreshIndex()
	}
	return vectors, current, err
}

type searchOutcome struct {
	Results      []result
	Mode         string
	KeywordEmpty bool
	Index        *indexer.Result
}

func (s *Server) runSearch(ctx context.Context, query string, k int) (searchOutcome, error) {
	out := searchOutcome{Results: []result{}, Mode: "keyword"}
	scan := &wiki.ScanResult{}
	err := s.withCurrentIndex(ctx, scan, func(work context.Context, st *store.Store, current *indexer.Result, semanticQuery bool) error {
		out.Index = current
		kw, err := st.SearchKeyword(query, k)
		if err != nil {
			return err
		}
		out.KeywordEmpty = len(kw) == 0
		hits := kw
		chunkText := map[string]string{}
		if semanticQuery && s.eng != nil && current.SemanticReady {
			qv, err := cembed.EmbedContext(work, s.eng, []string{query})
			if err != nil {
				return err
			}
			sem, err := st.SearchSemantic(qv[0], k)
			if err != nil {
				return err
			}
			chunks, err := st.SearchChunks(qv[0], k*2, 1)
			if err != nil {
				return err
			}
			for _, c := range chunks {
				if _, seen := chunkText[c.Slug]; !seen {
					chunkText[c.Slug] = c.Text
				}
			}
			hits = search.Fuse(k, kw, sem)
			out.Mode = "hybrid"
		}
		for _, h := range hits {
			snippet := strings.Join(strings.Fields(h.Snippet), " ")
			if text, ok := chunkText[h.Slug]; ok {
				snippet = excerptText(text, 200)
			}
			if p, ok := scan.BySlug[strings.ToLower(h.Slug)]; ok && snippet == "" {
				snippet = FirstParagraph(p.Body, 160)
			}
			out.Results = append(out.Results, result{Slug: h.Slug, Title: h.Title, Score: h.Score, Snippet: snippet, URL: pagePath(scan.BySlug[strings.ToLower(h.Slug)])})
		}
		return work.Err()
	})
	return out, err
}

// excerptText flattens whitespace and truncates to maxRunes.
func excerptText(t string, maxRunes int) string {
	r := []rune(strings.Join(strings.Fields(t), " "))
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return string(r)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	out, err := s.runSearch(r.Context(), query, 20)
	if err != nil {
		s.fail(w, err)
		return
	}
	// An exact slug is still a search entry point: refresh before redirecting
	// so a cold visit cannot leave cached consumers empty or stale.
	if scan, err := wiki.Scan(s.w); err == nil {
		if p, err := scan.Find(query); err == nil {
			http.Redirect(w, r, pagePath(p), http.StatusFound)
			return
		}
	}
	// The query itself is an event (search→read trail on /history);
	// the exposed hits are never marked (H5).
	s.logEvent("", attention.KindSearch, query)
	s.logSearchGap(query, out.Results, out.KeywordEmpty)
	s.render(w, r, http.StatusOK, "search.html", map[string]any{
		"Title":   localizeString(s.loc(r), "search_results_title", "Query", query),
		"Query":   query,
		"Mode":    out.Mode,
		"Results": out.Results,
		"Index":   out.Index,
	})
}

// --- JSON API (instant search + popover previews) ---

func (s *Server) emit(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json: %v", err)
	}
}

func (s *Server) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	k := 8
	if values, present := params["k"]; present {
		if len(values) != 1 {
			http.Error(w, "k must be one integer from 1 to 50", http.StatusBadRequest)
			return
		}
		n, err := strconv.Atoi(values[0])
		if err != nil || n < 1 || n > 50 {
			http.Error(w, "k must be one integer from 1 to 50", http.StatusBadRequest)
			return
		}
		k = n
	}
	query := strings.TrimSpace(params.Get("q"))
	if query == "" {
		s.emit(w, map[string]any{"results": []result{}})
		return
	}
	out, err := s.runSearch(r.Context(), query, k)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.emit(w, map[string]any{"query": query, "mode": out.Mode, "results": out.Results, "index": out.Index})
}

func (s *Server) handleAPIPreview(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := scan.Find(r.PathValue("slug"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		s.emit(w, map[string]any{"exists": false})
		return
	}
	s.emit(w, map[string]any{
		"exists":  true,
		"slug":    p.Slug,
		"title":   p.Title,
		"type":    p.Type,
		"excerpt": FirstParagraph(p.Body, 240),
	})
}

// searchFallback renders search results for a slug that has no page.
func (s *Server) searchFallback(w http.ResponseWriter, r *http.Request, raw string) {
	query := strings.ReplaceAll(wiki.NormalizeLink(raw), "-", " ")
	out, err := s.runSearch(r.Context(), query, 20)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, http.StatusNotFound, "search.html", map[string]any{
		"Title":   localizeString(s.loc(r), "search_results_title", "Query", query),
		"Query":   query,
		"Mode":    out.Mode,
		"Results": out.Results,
		"Index":   out.Index,
		"Missing": raw,
	})
}

// pagePath keeps ordinary slug URLs compact and uses an encoded governed path
// when a .md suffix would otherwise be ambiguous with a filename spelling.
func pagePath(p *wiki.Page) string {
	if p == nil {
		return "/browse"
	}
	ref := p.Slug
	if strings.HasSuffix(strings.ToLower(ref), ".md") {
		ref = p.RelPath
	}
	return "/page/" + url.PathEscape(ref)
}
