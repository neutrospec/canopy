package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func TestServeFailsClosedAndFlagPrecedence(t *testing.T) {
	oldWiki := flagWiki
	t.Cleanup(func() { flagWiki = oldWiki })
	flagWiki = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(config.ConfigHome(), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config.ConfigHome(), "config.toml")
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte(strings.Repeat("a", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		args       []string
		want       string
	}{
		{"missing token", "[serve]\naddr='192.168.1.22:8737'\nauth_token_file='/nonexistent/canopy-token'", nil, "auth token file"},
		{"malformed config", "[serve]\nauth_token_file=123", nil, "invalid global configuration"},
		{"relative flag", "", []string{"--auth-token-file", "relative"}, "absolute path"},
		{"bad origin", "", []string{"--public-origin", "https://*.example"}, "public origin"},
		{"CLI token override", "[serve]\nauth_token_file='/nonexistent/canopy-token'", []string{"--auth-token-file", tokenPath, "--public-origin", "http://insecure.example"}, "public origin"},
		{"config origin", "[serve]\nauth_token_file='" + tokenPath + "'\npublic_origin='http://insecure.example'", nil, "public origin"},
		{"CLI origin override", "[serve]\npublic_origin='http://insecure.example'", []string{"--public-origin", "https://wiki.example", "--addr", "invalid-address"}, "missing port"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			c := cmdServe()
			c.SilenceUsage = true
			c.SilenceErrors = true
			c.SetArgs(tc.args)
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %s, got %v", tc.want, err)
			}
		})
	}
}
