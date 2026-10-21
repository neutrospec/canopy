package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// EmbeddingService is machine-local: credentials must never travel with a wiki.
// Empty provider preserves the existing built-in ONNX model selection.
type EmbeddingService struct {
	Provider    string `toml:"provider"`
	URL         string `toml:"url"`
	Token       string `toml:"token" json:"-"`
	Model       string `toml:"model"`
	Dimension   int    `toml:"dimension"`
	Revision    string `toml:"revision"`
	InputFormat string `toml:"input_format"`
}

func LoadEmbeddingService() (EmbeddingService, error) {
	var cfg struct {
		Embedding EmbeddingService `toml:"embedding"`
	}
	_, err := toml.DecodeFile(filepath.Join(ConfigHome(), "config.toml"), &cfg)
	if os.IsNotExist(err) {
		return cfg.Embedding, nil
	}
	if err != nil {
		return EmbeddingService{}, fmt.Errorf("cannot parse global config.toml (check TOML syntax)")
	}
	return cfg.Embedding, nil
}
