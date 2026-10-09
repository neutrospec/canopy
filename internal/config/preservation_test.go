package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTOMLRefusesUnknownWithoutTruncation(t *testing.T) {
	w := &Wiki{Root: t.TempDir(), Cfg: Default()}
	body := "[schema]\ntypes = [\"concept\"]\nfuture_rule = 9007199254740993\n"
	if err := os.WriteFile(w.TOMLPath(), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteTOML(); err == nil {
		t.Fatal("unknown config rewritten")
	}
	got, _ := os.ReadFile(w.TOMLPath())
	if string(got) != body {
		t.Fatalf("original changed: %s", got)
	}
}

func TestAdoptDefaultPreservesUnknownTOMLSemantics(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := filepath.Join(ConfigHome(), "config.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("future_counter = 9007199254740993\n[custom]\nvalue = \"keep\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if wrote, err := AdoptAsDefaultWiki("/tmp/wiki with spaces"); err != nil || !wrote {
		t.Fatalf("adopt: %v %v", wrote, err)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), "9007199254740993") || !strings.Contains(string(got), "keep") {
		t.Fatalf("unknown lost: %s", got)
	}
}
