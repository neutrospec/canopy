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

// F4: init adopts the wiki as the default so later commands work from
// any cwd — without it agents `cd` into the wiki and treat it as their
// workspace. An existing default belongs to the first wiki and stays.
func TestAdoptAsDefaultWiki(t *testing.T) {
	cfgHome := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfgHome)

	adopted, err := AdoptAsDefaultWiki("/Users/x/wiki-a")
	if err != nil || !adopted {
		t.Fatalf("first adopt: got (%v, %v), want (true, nil)", adopted, err)
	}
	// Resolution from an unrelated cwd now finds it.
	t.Setenv("CANOPY_WIKI", "")
	root, err := findRoot("")
	if err != nil || root != "/Users/x/wiki-a" {
		t.Fatalf("findRoot = %q (%v), want the adopted wiki", root, err)
	}

	adopted, err = AdoptAsDefaultWiki("/Users/x/wiki-b")
	if err != nil || adopted {
		t.Errorf("second adopt: got (%v, %v), want (false, nil) — must not steal the default", adopted, err)
	}
	if root, _ := findRoot(""); root != "/Users/x/wiki-a" {
		t.Errorf("default changed to %q", root)
	}
}

// A config file that exists but doesn't parse is the user's to fix —
// never silently rewritten (it may hold settings we don't model).
func TestAdoptLeavesUnparsableConfigAlone(t *testing.T) {
	cfgHome := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	path := filepath.Join(cfgHome, "canopy", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	junk := []byte("this is not = valid toml [[[\n")
	if err := os.WriteFile(path, junk, 0o644); err != nil {
		t.Fatal(err)
	}
	if adopted, err := AdoptAsDefaultWiki("/Users/x/wiki-a"); adopted || err != nil {
		t.Errorf("got (%v, %v), want (false, nil)", adopted, err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(junk) {
		t.Errorf("unparsable config was rewritten:\n%s", got)
	}
}
