package webui

// /history — the attention timeline (M12): what passed through both
// doors, straight from the machine-local event log (invariant H9).
// Searches appear inline, so the search→read trail shows itself without
// any fuzzy attribution logic.

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/wiki"
)

type histEntry struct {
	Time  string // HH:MM (or a date in the per-page panel)
	Slug  string
	Title string // empty when the page no longer resolves (renamed/deleted)
	Label string // human label for the event kind
	Query string // search entries only
	Agent bool   // agent door
	Count int    // same-day repeats folded into one row

	kind string // raw event kind, for fold-strength comparison
}

type histDay struct {
	Date    string
	Entries []*histEntry
}

// kindLabelID maps event kinds to message ids; kindRank orders them so a
// day's strongest interaction with a page wins the fold. Labels are
// localized per request (invariant M).
var kindLabelID = map[string]string{
	attention.KindReread: "kind_reread",
	attention.KindRead:   "kind_read",
	attention.KindRecall: "kind_recall",
	attention.KindShow:   "kind_view",
	attention.KindView:   "kind_view",
	attention.KindSearch: "kind_search",
}

var kindRank = map[string]int{
	attention.KindReread: 5,
	attention.KindRead:   4,
	attention.KindRecall: 3,
	attention.KindShow:   2,
	attention.KindView:   1,
}

// histEntryOf shapes one raw event for display; timeFmt picks how much
// of the timestamp to show (clock on the timeline, date in the panel).
func histEntryOf(e attention.Event, timeFmt string, loc *i18n.Localizer) histEntry {
	h := histEntry{Slug: e.Slug, Label: localizeString(loc, kindLabelID[e.Kind]), Agent: e.Door == attention.DoorAgent, Count: 1, kind: e.Kind}
	if kindLabelID[e.Kind] == "" {
		h.Label = e.Kind
	}
	if e.Kind == attention.KindSearch {
		h.Query = e.Meta
	}
	if t, err := time.Parse(time.RFC3339, e.TS); err == nil {
		h.Time = t.Local().Format(timeFmt)
	}
	return h
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	loc := s.loc(r)
	scan, err := wiki.Scan(s.w)
	if err != nil {
		s.fail(w, err)
		return
	}
	var events []attention.Event
	if ev, err := attention.Open(s.w.AttentionDBPath()); err == nil {
		// The attention timeline: lifecycle events live in `canopy events`,
		// not here (N3) — a filed task is not a page access.
		events, _ = ev.Query(attention.Filter{Limit: 500, AttnOnly: true})
		ev.Close()
	}

	// Fold per day: one row per page (strongest kind wins, repeats
	// counted) and one row per distinct query, newest day first.
	var days []*histDay
	dayIdx := map[string]*histDay{}
	type foldKey struct{ date, slug, query string }
	folded := map[foldKey]*histEntry{}

	for _, e := range events {
		t, err := time.Parse(time.RFC3339, e.TS)
		if err != nil {
			continue
		}
		lt := t.Local()
		date := lt.Format("2006-01-02")
		d, ok := dayIdx[date]
		if !ok {
			d = &histDay{Date: date + " (" + localizeString(loc, "wd_"+strconv.Itoa(int(lt.Weekday()))) + ")"}
			dayIdx[date] = d
			days = append(days, d)
		}
		key := foldKey{date: date, slug: e.Slug}
		if e.Kind == attention.KindSearch {
			key.query = strings.ToLower(strings.TrimSpace(e.Meta))
		}
		if f, ok := folded[key]; ok {
			f.Count++
			if kindRank[e.Kind] > kindRank[f.kind] {
				f.kind, f.Label = e.Kind, localizeString(loc, kindLabelID[e.Kind])
				f.Agent = e.Door == attention.DoorAgent
			}
			continue
		}
		entry := histEntryOf(e, "15:04", loc)
		if p, ok := scan.BySlug[e.Slug]; ok {
			entry.Title = p.Title
		}
		folded[key] = &entry
		d.Entries = append(d.Entries, &entry)
	}

	s.render(w, r, http.StatusOK, "history.html", map[string]any{
		"Title": localizeString(loc, "title_history"),
		"Days":  days,
	})
}

// sparkSVG renders weekly access counts as a tiny inline SVG bar row —
// server-side, no JS, vendored-assets rule trivially satisfied.
func sparkSVG(counts []int, aria string) template.HTML {
	max := 1
	for _, c := range counts {
		if c > max {
			max = c
		}
	}
	const bw, gap, h = 7, 2, 18
	total := len(counts)*(bw+gap) - gap
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="spark" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`, total, h, total, h, template.HTMLEscapeString(aria))
	for i, c := range counts {
		bh := 2 // baseline tick so empty weeks stay visible
		if c > 0 {
			bh = 2 + (h-4)*c/max
		}
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="1"%s/>`,
			i*(bw+gap), h-bh, bw, bh, map[bool]string{true: ` class="on"`, false: ""}[c > 0])
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
