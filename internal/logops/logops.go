// Package logops appends to the wiki activity log (logs/YYYY-MM.jsonl),
// the single source of truth for wiki actions. The format matches the
// entries the legacy append_log.py produced.
package logops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

type Entry struct {
	Timestamp string   `json:"timestamp"`
	Action    string   `json:"action"` // create|update|move|delete|archive|sync|lint
	File      string   `json:"file"`
	Related   []string `json:"related,omitempty"`
	Note      string   `json:"note,omitempty"`
	Auto      bool     `json:"auto"`
}

func Append(w *config.Wiki, action, file string, related []string, note string) error {
	if err := os.MkdirAll(w.LogsDir(), 0o755); err != nil {
		return err
	}
	now := time.Now()
	e := Entry{
		Timestamp: now.Format("2006-01-02T15:04:05"),
		Action:    action,
		File:      file,
		Related:   related,
		Note:      note,
		Auto:      true,
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(w.LogsDir(), now.Format("2006-01")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// ReadRecent returns up to n log entries, newest first. Malformed
// lines (hand-edited logs) are skipped, not fatal.
func ReadRecent(w *config.Wiki, n int) ([]Entry, error) {
	months, err := filepath.Glob(filepath.Join(w.LogsDir(), "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months))) // YYYY-MM sorts lexically
	var out []Entry
	for _, path := range months {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var file []Entry
		for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			var e Entry
			if json.Unmarshal([]byte(l), &e) == nil && e.Timestamp != "" {
				file = append(file, e)
			}
		}
		// Entries within a file are chronological; newest last.
		for i := len(file) - 1; i >= 0 && len(out) < n; i-- {
			out = append(out, file[i])
		}
		if len(out) >= n {
			break
		}
	}
	return out, nil
}
