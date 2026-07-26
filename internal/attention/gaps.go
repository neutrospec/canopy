package attention

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

// Search gaps — queries the wiki couldn't answer — are page-creation
// demand, and demand is door-agnostic: web searches and agent (CLI)
// searches append to the SAME wiki-synced file (invariant H10). The
// path keeps its historical _meta/webui/ location: the web shipped it
// first and the 점검 page reads it there.

// Gap is one recorded unanswered query. Pre-M12 lines lack the door
// field; readers treat an absent door as "web".
type Gap struct {
	Time    string  `json:"time"`
	Query   string  `json:"query"`
	Results int     `json:"results"`
	Top     float64 `json:"top_score"`
	Door    string  `json:"door,omitempty"`
}

func GapsPath(w *config.Wiki) string {
	return filepath.Join(w.Root, "_meta", "webui", "search-gaps.jsonl")
}

// LogGap appends one gap line. Append-only jsonl merges cleanly across
// devices, unlike the aggregate JSON files.
func LogGap(w *config.Wiki, door, query string, results int, top float64) error {
	g := Gap{Time: time.Now().Format(time.RFC3339), Query: query, Results: results, Top: top, Door: door}
	line, err := json.Marshal(g)
	if err != nil {
		return err
	}
	path := GapsPath(w)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// LogSearchEvent records that a query was asked (machine-local event
// log only — no page is marked, per H5). meta carries the query text
// so the /history timeline can show the search→read trail.
func LogSearchEvent(w *config.Wiki, door, query string) error {
	ev, err := Open(w.AttentionDBPath())
	if err != nil {
		return err
	}
	defer ev.Close()
	return ev.Log(time.Now(), "", door, KindSearch, query)
}
