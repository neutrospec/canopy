package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddingServiceMachineLocal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(ConfigHome(), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ConfigHome(), "config.toml")
	original := "other = 42\n[embedding]\nprovider = 'openai'\nurl = 'http://localhost/v1/embeddings'\ntoken = 'test-secret'\nmodel = 'embeddinggemma-2'\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	w := resolveTOML(t, "[embedding]\nmodel = 'bge-m3-int8'\n")
	if w.EmbeddingService.Token != "test-secret" || w.Cfg.Embedding.Model != "bge-m3-int8" {
		t.Fatal("local override not loaded separately")
	}
	if err := w.WriteTOML(); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(w.TOMLPath())
	if strings.Contains(string(body), "test-secret") || strings.Contains(string(body), "localhost") {
		t.Fatal("credentials leaked into wiki")
	}
	if _, err := AdoptAsDefaultWiki(w.Root); err != nil {
		t.Fatal(err)
	}
	c, err := LoadEmbeddingService()
	if err != nil || c.Token != "test-secret" {
		t.Fatal("adopt lost embedding settings", err)
	}
}

func TestEmbeddingConfigParseRedaction(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(ConfigHome(), 0700)
	os.WriteFile(filepath.Join(ConfigHome(), "config.toml"), []byte("[embedding]\ntoken = test-secret"), 0600)
	_, err := LoadEmbeddingService()
	if err == nil || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("parse error leaks token: %v", err)
	}
}
