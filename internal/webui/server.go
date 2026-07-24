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
	w    *config.Wiki
	eng  cembed.Engine
	mu   sync.Mutex // serializes engine + store access
	tmpl map[string]*template.Template

	auth         *authStore
	authRequired bool
}

func NewServer(w *config.Wiki, eng cembed.Engine) (*Server, error) {
	s := &Server{w: w, eng: eng, tmpl: map[string]*template.Template{}}
	funcs := template.FuncMap{"short": short}
	for _, name := range []string{"home.html", "page.html", "search.html", "browse.html", "recent.html", "attention.html", "edit.html", "login.html", "setup.html", "discover.html"} {
		t, err := template.New("base.html").Funcs(funcs).ParseFS(assets, "templates/base.html", "templates/"+name)
		if err != nil {
			return nil, err
		}
		s.tmpl[name] = t
	}
	return s, nil
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
	mux.HandleFunc("GET /special/discover", s.handleDiscover)
	mux.HandleFunc("POST /read/{slug}", s.handleReadMark)
	mux.HandleFunc("POST /api/read/{slug}", s.handleReadAuto)
	mux.HandleFunc("GET /setup", s.handleSetupForm)
	mux.HandleFunc("POST /setup", s.handleSetupSave)
	mux.HandleFunc("GET /login", s.handleLoginForm)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /{$}", s.handleHome)
	return logRequests(s.guard(mux))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("%s %s", r.Method, r.URL)
	})
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
	if m, ok := data.(map[string]any); ok {
		m["AuthOn"] = s.authRequired
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := s.tmpl[name].ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("template %s: %v", name, err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// --- home ---

type dirStat struct {
	Dir   string
	Count int
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	byDir := map[string]int{}
	for _, p := range scan.Pages {
		byDir[p.Dir]++
	}
	var dirs []dirStat
	for _, d := range s.w.Cfg.Schema.PageDirs {
		dirs = append(dirs, dirStat{d, byDir[d]})
	}
	recent := append([]*wiki.Page(nil), scan.Pages...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].Updated > recent[j].Updated })
	if len(recent) > 10 {
		recent = recent[:10]
	}
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	discover := s.discoverRanked(scan, rs, time.Now())
	if len(discover) > 4 {
		discover = discover[:4]
	}
	s.render(w, http.StatusOK, "home.html", map[string]any{
		"Title":    "wiki",
		"Total":    len(scan.Pages),
		"Dirs":     dirs,
		"Recent":   recent,
		"Discover": discover,
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
		s.searchFallback(w, r.PathValue("slug"))
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
	s.render(w, http.StatusOK, "page.html", map[string]any{
		"Title":      p.Title,
		"Page":       p,
		"Body":       body,
		"Backlinks":  backlinks,
		"GraphNodes": nodes,
		"GraphEdges": edges,
		"Read":       rs.Get(slug),
		"ReadSecs":   readThresholdSecs(p),
		"Suggested":  s.suggestLinks(scan, p, backlinks),
	})
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
func (s *Server) runSearch(query string, k int, refresh bool) ([]result, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, err := wiki.Scan(s.w)
	if err != nil {
		return nil, "", err
	}
	st, err := store.Open(s.w.DBPath())
	if err != nil {
		return nil, "", err
	}
	defer st.Close()
	if refresh {
		if _, err := indexer.Reindex(s.w, st, scan, s.eng, nil); err != nil {
			return nil, "", err
		}
	}
	kw, err := st.SearchKeyword(query, k)
	if err != nil {
		return nil, "", err
	}
	mode := "keyword"
	hits := kw
	// Best matching chunk per page: shows WHICH paragraph matched,
	// not just that the page did.
	chunkText := map[string]string{}
	if s.eng != nil {
		qv, err := s.eng.Embed([]string{query})
		if err != nil {
			return nil, "", err
		}
		sem, err := st.SearchSemantic(qv[0], k)
		if err != nil {
			return nil, "", err
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
	return res, mode, nil
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
	res, mode, err := s.runSearch(query, 20, true)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusOK, "search.html", map[string]any{
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
	res, mode, err := s.runSearch(query, k, false)
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
func (s *Server) searchFallback(w http.ResponseWriter, raw string) {
	query := strings.ReplaceAll(wiki.NormalizeLink(raw), "-", " ")
	res, mode, err := s.runSearch(query, 20, true)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, http.StatusNotFound, "search.html", map[string]any{
		"Title":   "page not found",
		"Query":   query,
		"Mode":    mode,
		"Results": res,
		"Missing": raw,
	})
}
