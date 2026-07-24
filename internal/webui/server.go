package webui

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/neutrospec/canopy/internal/config"
	cembed "github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/indexer"
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
}

func NewServer(w *config.Wiki, eng cembed.Engine) (*Server, error) {
	s := &Server{w: w, eng: eng, tmpl: map[string]*template.Template{}}
	for _, name := range []string{"home.html", "page.html", "search.html"} {
		t, err := template.ParseFS(assets, "templates/base.html", "templates/"+name)
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
	mux.HandleFunc("GET /{$}", s.handleHome)
	return logRequests(mux)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Printf("%s %s", r.Method, r.URL)
	})
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data any) {
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
	s.render(w, http.StatusOK, "home.html", map[string]any{
		"Title":  "wiki",
		"Total":  len(scan.Pages),
		"Dirs":   dirs,
		"Recent": recent,
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
	s.render(w, http.StatusOK, "page.html", map[string]any{
		"Title":     p.Title,
		"Page":      p,
		"Body":      body,
		"Backlinks": backlinks,
	})
}

// --- search ---

type result struct {
	Slug    string
	Title   string
	Score   float64
	Snippet string
}

// runSearch mirrors cmdSearch: refresh the index, then hybrid unless
// the engine is missing (keyword-only fallback).
func (s *Server) runSearch(query string, k int) ([]result, string, error) {
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
	if _, err := indexer.Reindex(s.w, st, scan, s.eng, nil); err != nil {
		return nil, "", err
	}
	kw, err := st.SearchKeyword(query, k)
	if err != nil {
		return nil, "", err
	}
	mode := "keyword"
	hits := kw
	if s.eng != nil {
		qv, err := s.eng.Embed([]string{query})
		if err != nil {
			return nil, "", err
		}
		sem, err := st.SearchSemantic(qv[0], k)
		if err != nil {
			return nil, "", err
		}
		hits = search.Fuse(k, kw, sem)
		mode = "hybrid"
	}
	res := make([]result, 0, len(hits))
	for _, h := range hits {
		snippet := strings.Join(strings.Fields(h.Snippet), " ")
		if p, ok := scan.BySlug[wiki.NormalizeLink(h.Slug)]; ok && snippet == "" {
			snippet = FirstParagraph(p.Body, 160)
		}
		res = append(res, result{h.Slug, h.Title, h.Score, snippet})
	}
	return res, mode, nil
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	res, mode, err := s.runSearch(query, 20)
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

// searchFallback renders search results for a slug that has no page.
func (s *Server) searchFallback(w http.ResponseWriter, raw string) {
	query := strings.ReplaceAll(wiki.NormalizeLink(raw), "-", " ")
	res, mode, err := s.runSearch(query, 20)
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
