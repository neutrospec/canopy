package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestServeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := LoadServeConfig()
	if err != nil || cfg.Addr != "localhost:8737" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	if _, err := AdoptAsDefaultWiki("/first/wiki"); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadServeConfig()
	if err != nil || cfg.Addr != "localhost:8737" {
		t.Fatalf("old config: %+v %v", cfg, err)
	}
	path := filepath.Join(ConfigHome(), "config.toml")
	body := `[serve]
addr = "192.168.1.22:8737"
auth_token_file = "/private/token"
public_origin = "https://wiki.example"
allow_hosts = ["wiki.example"]
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := LoadServeConfig()
	if err != nil || first.AuthTokenFile != "/private/token" || first.Addr != "192.168.1.22:8737" {
		t.Fatalf("load: %+v %v", first, err)
	}
	if _, err := AdoptAsDefaultWiki("/second/wiki"); err != nil {
		t.Fatal(err)
	}
	again, err := LoadServeConfig()
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("serve settings lost after adoption/reload")
	}
	for _, bad := range []string{"[serve", "[serve]\naddr=123", "[serve]\nauth_tokn_file=\"/token\""} {
		_ = os.WriteFile(path, []byte(bad), 0600)
		if _, err := LoadServeConfig(); err == nil {
			t.Fatal("invalid config silently accepted")
		}
	}
}
