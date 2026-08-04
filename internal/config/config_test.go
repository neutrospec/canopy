package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGOverrides(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-c")
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-d")
	if got := ConfigHome(); got != "/tmp/xdg-c/canopy" {
		t.Errorf("ConfigHome = %s", got)
	}
	if got := CacheHome(); got != "/tmp/xdg-cache/canopy" {
		t.Errorf("CacheHome = %s", got)
	}
	if got := DataHome(); got != "/tmp/xdg-d/canopy" {
		t.Errorf("DataHome = %s", got)
	}
}

func TestXDGDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	for name, got := range map[string]string{
		"config": ConfigHome(),
		"cache":  CacheHome(),
		"data":   DataHome(),
	} {
		if !strings.HasSuffix(got, filepath.Join("canopy")) || strings.Contains(got, "xdg-") {
			t.Errorf("%s home fallback wrong: %s", name, got)
		}
	}
	if !strings.Contains(DataHome(), filepath.Join(".local", "share")) {
		t.Errorf("data fallback should be ~/.local/share: %s", DataHome())
	}
}

func resolveTOML(t *testing.T, body string) *Wiki {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "canopy.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// A legacy single-list `tags` config is the whole taxonomy: the built-in
// topic/form defaults must not leak into validation (rule 1b tolerant read).
func TestResolveLegacyTags(t *testing.T) {
	w := resolveTOML(t, "[schema]\ntags = [\"alpha\", \"beta\"]\n")
	if !w.LegacyTags {
		t.Error("LegacyTags should be true for a tags-only config")
	}
	got := w.Cfg.Schema.AllTags()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("AllTags = %v, want exactly the legacy list", got)
	}
}

func TestResolveFacetedTags(t *testing.T) {
	w := resolveTOML(t, "[schema]\ntopics = [\"cooking\"]\nforms = [\"review\"]\n")
	if w.LegacyTags {
		t.Error("LegacyTags must be false when facets are declared")
	}
	got := w.Cfg.Schema.AllTags()
	if len(got) != 2 || got[0] != "cooking" || got[1] != "review" {
		t.Errorf("AllTags = %v, want topics ∪ forms", got)
	}
}

// No canopy.toml → built-in defaults, which declare both facets.
func TestDefaultsAreFaceted(t *testing.T) {
	cfg := Default()
	if len(cfg.Schema.Topics) == 0 || len(cfg.Schema.Forms) == 0 || len(cfg.Schema.Tags) != 0 {
		t.Errorf("defaults must declare topics+forms and no legacy tags: %+v", cfg.Schema)
	}
	if cfg.Schema.BroadTopicPct != 25 {
		t.Errorf("BroadTopicPct default = %d, want 25", cfg.Schema.BroadTopicPct)
	}
	all := map[string]bool{}
	for _, tag := range cfg.Schema.AllTags() {
		if all[tag] {
			t.Errorf("duplicate tag across facets: %s", tag)
		}
		all[tag] = true
	}
}

func TestDBPathStablePerWiki(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache")
	a := &Wiki{Root: "/Users/x/wiki-a"}
	b := &Wiki{Root: "/Users/x/wiki-b"}
	if a.DBPath() == b.DBPath() {
		t.Error("different wikis must get different cache DBs")
	}
	if a.DBPath() != (&Wiki{Root: "/Users/x/wiki-a"}).DBPath() {
		t.Error("DBPath must be deterministic")
	}
	if !strings.HasPrefix(a.DBPath(), "/tmp/xdg-cache/canopy/index/") {
		t.Errorf("DBPath not under cache home: %s", a.DBPath())
	}
}
