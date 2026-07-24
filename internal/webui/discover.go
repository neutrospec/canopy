package webui

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// Discovery ranks UNREAD pages — "새발견" is meeting a page for the
// first time, complementary to resurface (re-meeting a forgotten one).
// Signals (docs/web-ui-plan-2.md D4): newness, hubness, and affinity
// to recently-read pages. All computed from data we already have.

type discovery struct {
	Page   *wiki.Page
	Score  float64
	Reason string
	Days   int // age since created, for display
}

const affinityRecentK = 10

func (s *Server) discoverRanked(scan *wiki.ScanResult, rs *reads.State, now time.Time) []discovery {
	// Vectors are best-effort: without the embedding index the
	// affinity signal is 0 and newness+hubness still rank.
	var vectors map[string][]float32
	if st, err := store.Open(s.w.DBPath()); err == nil {
		vectors, _ = st.PageVectors()
		st.Close()
	}
	var recentVecs [][]float32
	for _, slug := range rs.RecentSlugs(affinityRecentK) {
		if v, ok := vectors[slug]; ok {
			recentVecs = append(recentVecs, v)
		}
	}
	backlinks := scan.Backlinks()

	var out []discovery
	for _, p := range scan.Pages {
		if rs.IsRead(p.Slug) {
			continue
		}
		days := 9999
		if t, err := time.Parse("2006-01-02", p.Created); err == nil {
			days = int(now.Sub(t).Hours() / 24)
		}
		newness := 0.0
		if days <= 30 {
			newness = 1 - float64(days)/30
		}
		hub := float64(len(backlinks[strings.ToLower(p.Slug)])) / 5
		if hub > 1 {
			hub = 1
		}
		affinity := 0.0
		if v, ok := vectors[strings.ToLower(p.Slug)]; ok {
			for _, rv := range recentVecs {
				if c := store.Cosine(v, rv); c > affinity {
					affinity = c
				}
			}
		}
		score := 0.4*newness + 0.3*hub + 0.3*affinity
		if score <= 0 {
			continue
		}
		reason := "새 페이지"
		switch {
		case affinity >= newness && 0.3*affinity >= 0.3*hub && affinity > 0:
			reason = "최근 읽은 주제와 유사"
		case 0.3*hub > 0.4*newness && hub > 0:
			reason = "링크가 모이는 허브"
		case newness == 0:
			reason = "아직 안 읽음"
		}
		out = append(out, discovery{Page: p, Score: score, Reason: reason, Days: days})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// --- handlers ---

// handleReadMark is the explicit ✓ button (plain form POST — works
// without JS; the auto path is /api/read).
func (s *Server) handleReadMark(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slug := wiki.NormalizeLink(r.PathValue("slug"))
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if r.FormValue("action") == "unread" {
		rs.Unmark(slug)
	} else {
		rs.Mark(slug, "explicit", time.Now())
	}
	if err := rs.Save(s.w); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/page/"+slug, http.StatusSeeOther)
}

// handleReadAuto records a dwell+scroll detected read. It never
// downgrades and never re-marks an already-read page.
func (s *Server) handleReadAuto(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slug := wiki.NormalizeLink(r.PathValue("slug"))
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, ok := scan.BySlug[slug]; !ok {
		http.NotFound(w, r)
		return
	}
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !rs.IsRead(slug) {
		rs.Mark(slug, "auto", time.Now())
		if err := rs.Save(s.w); err != nil {
			s.fail(w, err)
			return
		}
	}
	s.emit(w, map[string]any{"read": true})
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
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
	ranked := s.discoverRanked(scan, rs, time.Now())
	readCount := 0
	for _, p := range scan.Pages {
		if rs.IsRead(p.Slug) {
			readCount++
		}
	}
	pct := 0
	if len(scan.Pages) > 0 {
		pct = readCount * 100 / len(scan.Pages)
	}
	s.render(w, http.StatusOK, "discover.html", map[string]any{
		"Title":     "새발견",
		"Ranked":    ranked,
		"ReadCount": readCount,
		"Total":     len(scan.Pages),
		"Pct":       pct,
	})
}
