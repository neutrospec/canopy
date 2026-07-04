package config

import (
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
