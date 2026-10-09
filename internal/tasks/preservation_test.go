package tasks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectDelimiterCollision(t *testing.T) {
	w := testWiki(t)
	a, _, err := FileConnect(w, "a", "b--c", .9, "web", now)
	if err != nil {
		t.Fatal(err)
	}
	b, created, err := FileConnect(w, "a--b", "c", .8, "web", now)
	if err != nil || !created || a.ID == b.ID {
		t.Fatalf("distinct pairs reused: %+v %+v created=%v err=%v", a, b, created, err)
	}
	legacy := &Task{Version: 1, ID: "connect-x--y", Type: TypeConnect, Status: StatusDismissed, Pages: []string{"x", "y"}}
	if err := writeTask(w, legacy.ID, legacy); err != nil {
		t.Fatal(err)
	}
	got, created, err := FileConnect(w, "y", "x", .9, "web", now)
	if err != nil || created || got.Status != StatusDismissed {
		t.Fatalf("legacy decision lost: %+v %v %v", got, created, err)
	}
}

func TestEditProcessHelper(t *testing.T) {
	root := os.Getenv("CANOPY_TASK_PROCESS_ROOT")
	if root == "" {
		t.Skip("subprocess helper")
	}
	w := testWiki(t)
	w.Root = root
	if _, err := FileEdit(w, "page", os.Getenv("CANOPY_TASK_PROCESS_VALUE"), "", "", "web", now); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentEditProcessesPreserveAllPayloads(t *testing.T) {
	w := testWiki(t)
	const count = 8
	commands := make([]*exec.Cmd, count)
	for i := range commands {
		cmd := exec.Command(os.Args[0], "-test.run=^TestEditProcessHelper$")
		cmd.Env = append(os.Environ(), "CANOPY_TASK_PROCESS_ROOT="+w.Root, fmt.Sprintf("CANOPY_TASK_PROCESS_VALUE=proposal-%d", i))
		commands[i] = cmd
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	list, err := Load(w)
	if err != nil || len(list) != count {
		t.Fatalf("stored=%d wanted=%d err=%v", len(list), count, err)
	}
	seen := map[string]bool{}
	for _, task := range list {
		seen[task.Request] = true
	}
	if len(seen) != count {
		t.Fatalf("proposal bodies overwritten: %+v", list)
	}
}

func TestClosePreservesExactUnknownNumbers(t *testing.T) {
	w := testWiki(t)
	if err := os.MkdirAll(Dir(w), 0755); err != nil {
		t.Fatal(err)
	}
	before := `{"version":1,"id":"old","type":"edit","status":"pending","pages":["a"],"extra":{"n":9007199254740993}}`
	if err := os.WriteFile(pathOf(w, "old"), []byte(before), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Close(w, scan(t, w), "old", StatusDismissed, "skip", "agent", now); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(pathOf(w, "old"))
	if err != nil || !strings.Contains(string(after), "9007199254740993") {
		t.Fatalf("number changed: %s %v", after, err)
	}
}

func TestGCUsesEnumeratedClosedFileOnly(t *testing.T) {
	w := testWiki(t)
	closed := now.Add(-365 * 24 * time.Hour).Format(time.RFC3339)
	for id, status := range map[string]string{"pending": StatusPending, "alien": "unrecognized"} {
		if err := writeTask(w, id, &Task{Version: 1, ID: id, Status: status, Closed: closed}); err != nil {
			t.Fatal(err)
		}
	}
	// A closed file claiming another task's ID must never delete that task.
	if err := writeTask(w, "mismatch", &Task{Version: 1, ID: "pending", Status: StatusDone, Closed: closed}); err != nil {
		t.Fatal(err)
	}
	_, _ = GC(w, 24*time.Hour, now)
	for _, id := range []string{"pending", "alien"} {
		if _, err := os.Stat(pathOf(w, id)); err != nil {
			t.Fatalf("GC deleted %s: %v", id, err)
		}
	}
}

func TestMalformedTaskIsUnavailable(t *testing.T) {
	for _, body := range []string{"null", "{", `{"id":"bad","status":"pending"}`} {
		w := testWiki(t)
		if err := os.MkdirAll(Dir(w), 0755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(Dir(w), "bad.json")
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(w); err == nil {
			t.Fatalf("corrupt task treated as healthy: %s", body)
		}
	}
}
