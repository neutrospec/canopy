package statefile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type example struct {
	Version int            `json:"version"`
	Values  map[string]int `json:"values"`
}

func TestPatchPreservesUnknownAndConcurrentKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	raw := []byte(`{"version":1,"values":{"a":1},"future":{"deep":{"n":9007199254740993}}}`)
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	base := example{1, map[string]int{"a": 1}}
	a, b := Snapshot(base), Snapshot(base)
	if err := Patch(p, a, example{1, map[string]int{"a": 1, "b": 2}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := Patch(p, b, example{1, map[string]int{"a": 1, "c": 3}}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(got), "9007199254740993") {
		t.Fatalf("unknown changed: %s %v", got, err)
	}
	var v example
	if err := json.Unmarshal(got, &v); err != nil || len(v.Values) != 3 {
		t.Fatalf("lost concurrent keys: %s %v", got, err)
	}
	fi, _ := os.Stat(p)
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v", fi.Mode())
	}
}

func TestPatchRejectsConflictingLeafAndBrokenState(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	base := example{1, map[string]int{"a": 1}}
	b := Snapshot(base)
	if err := os.WriteFile(p, []byte(`{"version":1,"values":{"a":2}}`), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	if err := Patch(p, b, example{1, map[string]int{"a": 3}}, nil); err == nil {
		t.Fatal("stale replacement accepted")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("conflict changed original")
	}
	for _, body := range []string{"null", "[]", "{"} {
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if err := Patch(p, b, base, nil); err == nil {
			t.Fatalf("broken state accepted: %s", body)
		}
		got, _ := os.ReadFile(p)
		if string(got) != body {
			t.Fatalf("broken state rewritten: %s", got)
		}
	}
}
