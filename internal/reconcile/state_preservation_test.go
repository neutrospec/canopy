package reconcile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerPreservesExtensionsAndRefusesNull(t *testing.T) {
	w := testWiki(t)
	if err := os.MkdirAll(filepath.Dir(Path(w)), 0755); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":9,"blessed":{},"future":{"n":9007199254740993}}`
	if err := os.WriteFile(Path(w), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	s, _, err := Load(w)
	if err != nil {
		t.Fatal(err)
	}
	s.Blessed["concepts/page.md"] = "hash"
	if err := s.Save(w); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(Path(w))
	if !strings.Contains(string(got), "9007199254740993") {
		t.Fatalf("unknown lost: %s", got)
	}
	if err := os.WriteFile(Path(w), []byte("null"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(w); err == nil {
		t.Fatal("null ledger accepted")
	}
	if err := s.Save(w); err == nil {
		t.Fatal("null ledger overwritten")
	}
	got, _ = os.ReadFile(Path(w))
	if string(got) != "null" {
		t.Fatal("null ledger reset")
	}
}
