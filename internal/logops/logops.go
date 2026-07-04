// Package logops appends to the wiki activity log (logs/YYYY-MM.jsonl),
// the single source of truth for wiki actions. The format matches the
// entries the legacy append_log.py produced.
package logops

import (
	"encoding/json"
	"os"
	"path/filepath"
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
