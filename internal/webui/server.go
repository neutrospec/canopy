package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
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
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/search"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

//go:embed templates/* static/*
var assets embed.FS

// Server renders the wiki read-only over HTTP. The engine may be nil,
// in which case search degrades to keyword-only (same as the CLI).
type Server struct {
	w   *config.Wiki
	eng cembed.Engine
	mu  sync.Mutex // serializes engine + store access
	// tmpl is keyed by locale then template name: each locale gets its own
	// parsed set with `t` bound to that locale's localizer (html/template
	// binds funcs at parse time). See docs/web-ui-i18n.md.
	tmpl map[string]map[string]*template.Template
	i18n *i18nBundle

	auth         *authStore
	authRequired bool

	dailyMu sync.Mutex
	daily   daily
}

var pageTemplates = []string{"home.html", "page.html", "search.html", "browse.html", "recent.html", "attention.html", "edit.html", "login.html", "setup.html", "discover.html", "gaps.html", "graph.html", "history.html"}

func NewServer(w *config.Wiki, eng cembed.Engine) (*Server, error) {
	ib, err := loadBundle()
	if err != nil {
		return nil, err
	}
	s := &Server{w: w, eng: eng, i18n: ib, tmpl: map[string]map[string]*template.Template{}}
	for _, lang := range ib.langs {
		loc := ib.localizer(lang)
		funcs := template.FuncMap{"short": short}
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
	return s, nil
}

// loc returns the localizer for the request's resolved locale — for Go
// handlers that build dynamic display strings (invariant M: dynamic strings
// are localized where the data lives, static labels in templates via {{t}}).
func (s *Server) loc(r *http.Request) *i18n.Localizer {
	return s.i18n.localizer(s.i18n.resolveLang(r))
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServerFS(assets))
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
	mux.HandleFunc("GET /setup", s.handleSetupForm)
	mux.HandleFunc("POST /setup", s.handleSetupSave)
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /setlang", s.handleSetLang)
	mux.HandleFunc("GET /{$}", s.handleHome)
	return logRequests(s.guard(mux))
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
		m["AuthOn"] = s.authRequired
		// Language-selector data for base.html (invariant M4: the loaded
		// locale list drives the menu, so a new file just appears).
		m["Lang"] = lang
		m["LangName"] = endonym(lang)
		m["Langs"] = s.i18n.options()
		m["Path"] = r.URL.Path
	}
	set := s.tmpl[lang]
	if set == nil {
		set = s.tmpl[defaultLang]
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := set[name].ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("template %s: %v", name, err)
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
	if next == "" || !strings.HasPrefix(next, "/") {
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
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
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
		if events, err := ev.Recent(400); err == nil {
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
	discover := s.discoverRanked(scan, rs, now, lc)
	if len(discover) > 4 {
		discover = discover[:4]
	}
	pick, bridge := s.todaysCard()

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
		"Title":        "wiki",
		"Date":         date,
		"Total":        len(scan.Pages),
		"Dirs":         dirs,
		"ReadProgress": readProgress,
		"TodayLine":    todayLine,
		"Recent":       recent,
		"Discover":     discover,
		"Pick":         pick,
		"Bridge":       bridge,
	})
}

// --- page view ---

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	slug := wiki.NormalizeLink(r.PathValue("slug"))
	p, ok := scan.BySlug[slug]
	if !ok {
		// Wikipedia pattern: a missing page is a search, not a dead end.
		s.searchFallback(w, r, r.PathValue("slug"))
		return
	}
	body, err := RenderPage(p.Body, func(t string) bool { _, ok := scan.BySlug[t]; return ok })
	if err != nil {
		s.fail(w, err)
		return
	}
	backlinks := scan.Backlinks()[slug]
	nodes, edges := localGraph(scan, p, backlinks)
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
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
	s.render(w, r, http.StatusOK, "page.html", map[string]any{
		"Title":      p.Title,
		"Page":       p,
		"Body":       body,
		"Backlinks":  backlinks,
		"GraphNodes": nodes,
		"GraphEdges": edges,
		"Read":       rs.Get(slug),
		"ReadAgo":    readAgo(rs.Get(slug), time.Now(), lc),
		"AgentDays":  agentDays,
		"AttnRecent": attnRecent,
		"Spark":      spark,
		"ReadSecs":   readThresholdSecs(p),
		"Suggested":  s.suggestLinks(scan, p, backlinks),
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
}

// runSearch mirrors cmdSearch: hybrid unless the engine is missing
// (keyword-only fallback). refresh controls whether the index is
// rebuilt first — full page loads do, per-keystroke API calls skip it.
func (s *Server) runSearch(query string, k int, refresh bool) ([]result, string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, err := wiki.Scan(s.w)
	if err != nil {
		return nil, "", false, err
	}
	st, err := store.Open(s.w.DBPath())
	if err != nil {
		return nil, "", false, err
	}
	defer st.Close()
	if refresh {
		if _, err := indexer.Reindex(s.w, st, scan, s.eng, nil); err != nil {
			return nil, "", false, err
		}
	}
	kw, err := st.SearchKeyword(query, k)
	if err != nil {
		return nil, "", false, err
	}
	mode := "keyword"
	hits := kw
	// Best matching chunk per page: shows WHICH paragraph matched,
	// not just that the page did.
	chunkText := map[string]string{}
	if s.eng != nil {
		qv, err := s.eng.Embed([]string{query})
		if err != nil {
			return nil, "", false, err
		}
		sem, err := st.SearchSemantic(qv[0], k)
		if err != nil {
			return nil, "", false, err
		}
		if chunks, err := st.SearchChunks(qv[0], k*2, 1); err == nil {
			for _, c := range chunks {
				if _, seen := chunkText[c.Slug]; !seen {
					chunkText[c.Slug] = c.Text
				}
			}
		}
		hits = search.Fuse(k, kw, sem)
		mode = "hybrid"
	}
	res := make([]result, 0, len(hits))
	for _, h := range hits {
		snippet := strings.Join(strings.Fields(h.Snippet), " ")
		if t, ok := chunkText[h.Slug]; ok {
			snippet = excerptText(t, 200)
		}
		if p, ok := scan.BySlug[wiki.NormalizeLink(h.Slug)]; ok && snippet == "" {
			snippet = FirstParagraph(p.Body, 160)
		}
		res = append(res, result{h.Slug, h.Title, h.Score, snippet})
	}
	return res, mode, len(kw) == 0, nil
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
	// Wikipedia "Go" behavior: an exact title jumps straight to the page.
	if scan, err := wiki.Scan(s.w); err == nil {
		if p, ok := scan.BySlug[wiki.NormalizeLink(query)]; ok {
			http.Redirect(w, r, "/page/"+p.Slug, http.StatusFound)
			return
		}
	}
	res, mode, kwEmpty, err := s.runSearch(query, 20, true)
	if err != nil {
		s.fail(w, err)
		return
	}
	// The query itself is an event (search→read trail on /history);
	// the exposed hits are never marked (H5).
	s.logEvent("", attention.KindSearch, query)
	s.logSearchGap(query, res, kwEmpty)
	s.render(w, r, http.StatusOK, "search.html", map[string]any{
		"Title":   fmt.Sprintf("search: %s", query),
		"Query":   query,
		"Mode":    mode,
		"Results": res,
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		s.emit(w, map[string]any{"results": []result{}})
		return
	}
	k := 8
	if n, err := strconv.Atoi(r.URL.Query().Get("k")); err == nil && n > 0 && n <= 50 {
		k = n
	}
	res, mode, _, err := s.runSearch(query, k, false)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.emit(w, map[string]any{"query": query, "mode": mode, "results": res})
}

func (s *Server) handleAPIPreview(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	slug := wiki.NormalizeLink(r.PathValue("slug"))
	p, ok := scan.BySlug[slug]
	if !ok {
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
	res, mode, _, err := s.runSearch(query, 20, true)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, http.StatusNotFound, "search.html", map[string]any{
		"Title":   "page not found",
		"Query":   query,
		"Mode":    mode,
		"Results": res,
		"Missing": raw,
	})
}
