// Package logops appends to the wiki activity log (logs/YYYY-MM.jsonl),
// the single source of truth for wiki actions. The format matches the
// entries the legacy append_log.py produced.
package logops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutrospec/canopy/internal/config"
	"github.com/neutrospec/canopy/internal/safefile"
	"github.com/neutrospec/canopy/internal/wikilock"
)

type Entry struct {
	Timestamp   string   `json:"timestamp"`
	Action      string   `json:"action"` // create|update|move|delete|archive|sync|lint
	File        string   `json:"file"`
	Related     []string `json:"related,omitempty"`
	Note        string   `json:"note,omitempty"`
	Auto        bool     `json:"auto"`
	OperationID string   `json:"operation_id,omitempty"`
}

func Append(w *config.Wiki, action, file string, related []string, note string) error {
	lease, err := wikilock.AcquireShared(context.Background(), w.Root)
	if err != nil {
		return err
	}
	defer lease.Unlock()
	return AppendLocked(w, action, file, related, note)
}

// AppendLocked is used while a wiki lease is held. The file lock spans the
// complete RMW and atomic replacement, so neither partial lines nor lost appends
// are exposed to cooperating readers/writers.
func AppendLocked(w *config.Wiki, action, file string, related []string, note string) error {
	return AppendOnceLocked(w, action, file, related, note, "")
}

// AppendOnceLocked gives retained checkins a stable, idempotent activity entry.
// Other operations pass an empty ID and keep their ordinary append semantics.
func AppendOnceLocked(w *config.Wiki, action, file string, related []string, note, id string) error {
	if err := os.MkdirAll(w.LogsDir(), 0o755); err != nil {
		return err
	}
	now := time.Now()
	e := Entry{
		Timestamp:   now.Format("2006-01-02T15:04:05"),
		Action:      action,
		File:        file,
		Related:     related,
		Note:        note,
		Auto:        true,
		OperationID: id,
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(w.LogsDir(), now.Format("2006-01")+".jsonl")
	guard, err := wikilock.AcquireFile(context.Background(), path)
	if err != nil {
		return err
	}
	defer guard.Unlock()
	if err := safefile.Regular(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if id != "" {
		months, err := filepath.Glob(filepath.Join(w.LogsDir(), "*.jsonl"))
		if err != nil {
			return err
		}
		for _, month := range months {
			priorData, err := os.ReadFile(month)
			if err != nil {
				return err
			}
			for _, raw := range strings.Split(string(priorData), "\n") {
				var prior Entry
				if json.Unmarshal([]byte(raw), &prior) == nil && prior.OperationID == id {
					return nil
				}
			}
		}
	}
	data = append(data, line...)
	data = append(data, '\n')
	return safefile.WriteFile(path, data, 0o644)
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
