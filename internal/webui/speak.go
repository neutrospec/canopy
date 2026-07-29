package webui

import (
	"bufio"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/reads"
	"github.com/neutrospec/canopy/internal/resurface"
	"github.com/neutrospec/canopy/internal/store"
	"github.com/neutrospec/canopy/internal/wiki"
)

// "위키가 먼저 말을 거는 홈" (docs/web-ui-plan-2.md D6): a daily
// resurface card with feedback buttons that write to the SAME state
// file the CLI/agent loop uses, plus a search-gap log — queries the
// wiki couldn't answer are page-creation demand.

// daily caches today's pick so reloading the home page doesn't burn
// through the resurface pool (each pick is marked shown, 45d cooldown).
type daily struct {
	date   string
	pick   *resurface.Pick
	bridge *resurface.Bridge
}

func (s *Server) todaysCard() (*resurface.Pick, *resurface.Bridge) {
	s.dailyMu.Lock()
	defer s.dailyMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if s.daily.date == today {
		return s.daily.pick, s.daily.bridge
	}
	s.daily = daily{date: today}
	scan, err := wiki.Scan(s.w)
	if err != nil {
		return nil, nil
	}
	st, err := resurface.LoadState(s.w)
	if err != nil {
		return nil, nil
	}
	now := time.Now()
	rng := rand.New(rand.NewSource(now.UnixNano()))
	changed := false
	rd, err := reads.Load(s.w)
	if err != nil {
		rd = nil
	}
	if picks, err := resurface.PickPages(scan, st, rd, "auto", 1, now, rng); err == nil && len(picks) > 0 {
		s.daily.pick = &picks[0]
		st.MarkShown([]string{picks[0].Slug}, now)
		changed = true
	}
	if sdb, err := store.Open(s.w.DBPath()); err == nil {
		if vectors, err := sdb.PageVectors(); err == nil {
			if bridges := resurface.PickBridges(scan, vectors, st, 0.7, 1, false, now); len(bridges) > 0 {
				s.daily.bridge = &bridges[0]
				st.MarkPairShown(strings.ToLower(bridges[0].A.Slug), strings.ToLower(bridges[0].B.Slug), now)
				changed = true
			}
		}
		sdb.Close()
	}
	if changed {
		if err := st.Save(s.w); err != nil {
			log.Printf("resurface state save: %v", err)
		}
	}
	return s.daily.pick, s.daily.bridge
}

// handleResurfaceFeedback wires 👍/👎/😴 to the shared resurface state
// so the web and the agent loop share cooldowns and votes.
func (s *Server) handleResurfaceFeedback(w http.ResponseWriter, r *http.Request) {
	slug := wiki.NormalizeLink(r.FormValue("slug"))
	action := r.FormValue("action")
	st, err := resurface.LoadState(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	now := time.Now()
	switch action {
	case "up", "down":
		st.AddFeedback(slug, action, now)
	case "snooze":
		st.Snooze(slug, 7, now)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err := st.Save(s.w); err != nil {
		s.fail(w, err)
		return
	}
	// Card is consumed for today; a new one comes tomorrow.
	s.dailyMu.Lock()
	s.daily.pick = nil
	s.dailyMu.Unlock()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- search gap log ---

// gapEntry mirrors attention.Gap for reading the jsonl; kept local so
// the 점검 page tolerates any line shape it has historically written.
type gapEntry struct {
	Time    string  `json:"time"`
	Query   string  `json:"query"`
	Results int     `json:"results"`
	Top     float64 `json:"top_score"`
}

// logSearchGap records deliberate searches the wiki couldn't answer:
// zero results, or zero keyword hits (the term doesn't exist in the
// wiki — semantic neighbors notwithstanding). The write goes through
// the shared door-agnostic writer (invariant H10).
func (s *Server) logSearchGap(query string, results []result, kwEmpty bool) {
	if len(results) > 0 && !kwEmpty {
		return
	}
	top := 0.0
	if len(results) > 0 {
		top = results[0].Score
	}
	attention.LogGap(s.w, attention.DoorWeb, query, len(results), top)
}

func (s *Server) handleGaps(w http.ResponseWriter, r *http.Request) {
	type agg struct {
		Query   string
		Count   int
		Last    string
		Results int
	}
	byQuery := map[string]*agg{}
	if f, err := os.Open(attention.GapsPath(s.w)); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var e gapEntry
			if json.Unmarshal(sc.Bytes(), &e) != nil {
				continue
			}
			q := strings.ToLower(strings.TrimSpace(e.Query))
			a, ok := byQuery[q]
			if !ok {
				a = &agg{Query: e.Query}
				byQuery[q] = a
			}
			a.Count++
			if e.Time > a.Last {
				a.Last = e.Time
				a.Results = e.Results
			}
		}
		f.Close()
	}
	rows := make([]*agg, 0, len(byQuery))
	for _, a := range byQuery {
		rows = append(rows, a)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Last > rows[j].Last
	})
	if len(rows) > 100 {
		rows = rows[:100]
	}
	for _, a := range rows {
		a.Last = strings.Replace(strings.SplitN(a.Last, "+", 2)[0], "T", " ", 1)
	}
	s.render(w, r, http.StatusOK, "gaps.html", map[string]any{"Title": localizeString(s.loc(r), "title_gaps"), "Rows": rows})
}
