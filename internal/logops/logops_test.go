package logops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func TestAppendCreatesLogFile(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	if err := Append(w, "create", "concepts/test.md", []string{"tag1"}, "note"); err != nil {
		t.Fatal(err)
	}

	// Log file should exist in logs/YYYY-MM.jsonl.
	logsDir := filepath.Join(root, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".jsonl") {
		t.Errorf("expected .jsonl file, got %s", entries[0].Name())
	}
}

func TestAppendContent(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	if err := Append(w, "update", "entities/foo.md", nil, ""); err != nil {
		t.Fatal(err)
	}

	// Read the log file.
	logsDir := filepath.Join(root, "logs")
	entries, _ := os.ReadDir(logsDir)
	data, err := os.ReadFile(filepath.Join(logsDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(data))

	if !strings.Contains(line, `"action":"update"`) {
		t.Errorf("missing action: %s", line)
	}
	if !strings.Contains(line, `"file":"entities/foo.md"`) {
		t.Errorf("missing file: %s", line)
	}
	if !strings.Contains(line, `"auto":true`) {
		t.Errorf("missing auto flag: %s", line)
	}
	if !strings.Contains(line, `"timestamp"`) {
		t.Errorf("missing timestamp: %s", line)
	}
}

func TestAppendAppends(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	// Two appends in the same month should go to the same file.
	if err := Append(w, "create", "a.md", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := Append(w, "create", "b.md", nil, ""); err != nil {
		t.Fatal(err)
	}

	logsDir := filepath.Join(root, "logs")
	entries, _ := os.ReadDir(logsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	data, _ := os.ReadFile(filepath.Join(logsDir, entries[0].Name()))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), data)
	}
}

func TestAppendWithRelated(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	if err := Append(w, "create", "p.md", []string{"related-a", "related-b"}, "test note"); err != nil {
		t.Fatal(err)
	}

	logsDir := filepath.Join(root, "logs")
	entries, _ := os.ReadDir(logsDir)
	data, _ := os.ReadFile(filepath.Join(logsDir, entries[0].Name()))
	line := string(data)

	if !strings.Contains(line, `"related":["related-a","related-b"]`) {
		t.Errorf("missing related: %s", line)
	}
	if !strings.Contains(line, `"note":"test note"`) {
		t.Errorf("missing note: %s", line)
	}
}

func TestAppendCreatesLogsDir(t *testing.T) {
	root := t.TempDir()
	w := &config.Wiki{Root: root, Cfg: config.Default()}

	// LogsDir should not exist yet.
	if _, err := os.Stat(w.LogsDir()); !os.IsNotExist(err) {
		t.Fatal("expected logs dir to not exist")
	}

	if err := Append(w, "sync", "", nil, "test"); err != nil {
		t.Fatal(err)
	}

	// LogsDir should now exist.
	if _, err := os.Stat(w.LogsDir()); os.IsNotExist(err) {
		t.Error("logs dir was not created")
	}
}
