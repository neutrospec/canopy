package webui

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
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

func (s *Server) discoverRanked(scan *wiki.ScanResult, rs *reads.State, vectors map[string][]float32, now time.Time, loc *i18n.Localizer) []discovery {
	if !rs.HumanAvailable() {
		return nil // Unknown read state is not the same as an unread page.
	}
	var recentVecs [][]float32
	// Affinity blends both doors: recall citations are interest too.
	for _, slug := range rs.RecentAccessSlugs(affinityRecentK) {
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
		reasonID := "reason_new"
		switch {
		case affinity >= newness && 0.3*affinity >= 0.3*hub && affinity > 0:
			reasonID = "reason_affinity"
		case 0.3*hub > 0.4*newness && hub > 0:
			reasonID = "reason_hub"
		case newness == 0:
			reasonID = "reason_unread"
		}
		reason := localizeString(loc, reasonID)
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
	lease, err := wikilock.AcquireShared(r.Context(), s.w.Root)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer lease.Unlock()
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("slug")))
	action := r.FormValue("action")
	if action != "read" && action != "unread" {
		http.Error(w, "unknown read action", http.StatusBadRequest)
		return
	}
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, findErr := scan.Find(r.PathValue("slug"))
	if findErr == nil {
		slug = strings.ToLower(p.Slug)
	}
	if findErr != nil && !(action == "unread" && rs.IsRead(slug)) {
		http.NotFound(w, r)
		return
	}
	kind := attention.KindRead
	if action == "unread" {
		rs.Unmark(slug)
	} else {
		if rs.IsRead(slug) {
			kind = attention.KindReread
		}
		rs.Mark(slug, "explicit", time.Now())
	}
	if err := rs.SaveLocked(s.w); err != nil {
		s.fail(w, err)
		return
	}
	if action == "read" {
		s.logEvent(slug, kind, "")
	}
	if p != nil {
		http.Redirect(w, r, pagePath(p), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/browse", http.StatusSeeOther)
	}
}

// handleReadAuto records a dwell+scroll detected read. It never
// downgrades and never re-marks an already-read page.
func (s *Server) handleReadAuto(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, err := wikilock.AcquireShared(r.Context(), s.w.Root)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer lease.Unlock()
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("slug")))
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, findErr := scan.Find(r.PathValue("slug"))
	if findErr != nil {
		http.NotFound(w, r)
		return
	}
	slug = strings.ToLower(p.Slug)
	rs, err := reads.Load(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !rs.IsRead(slug) {
		rs.Mark(slug, "auto", time.Now())
		if err := rs.SaveLocked(s.w); err != nil {
			s.fail(w, err)
			return
		}
		s.logEvent(slug, attention.KindRead, "")
	}
	s.emit(w, map[string]any{"read": true})
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	vectors, current, indexErr := s.currentVectors(r.Context(), scan)
	if indexErr != nil {
		log.Printf("discovery index: %v", indexErr)
	}
	rs, readErr := reads.Load(s.w)
	if readErr != nil {
		log.Printf("discovery reads: %v", readErr)
	}
	loc := s.loc(r)
	ranked := s.discoverRanked(scan, rs, vectors, time.Now(), loc)
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
	s.render(w, r, http.StatusOK, "discover.html", map[string]any{
		"Title":             localizeString(loc, "title_discover"),
		"Ranked":            ranked,
		"ReadCount":         readCount,
		"UnreadCount":       len(scan.Pages) - readCount,
		"ReadAvailable":     rs.HumanAvailable(),
		"ReadsPartial":      readErr != nil,
		"SemanticAvailable": indexErr == nil && current != nil && current.SemanticReady,
		"Total":             len(scan.Pages),
		"Pct":               pct,
	})
}
