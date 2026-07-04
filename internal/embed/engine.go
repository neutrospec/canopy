// Package embed computes text embeddings fully in-process (no external
// API) using hugot with the ONNX Runtime backend and bge-m3.
//
// The engine is only available in binaries built with `-tags ORT`
// (see engine_ort.go / engine_stub.go). Model files live under
// $XDG_DATA_HOME/canopy/models/bge-m3.
package embed

import (
	"os"
	"path/filepath"

	"github.com/nobocop/canopy/internal/config"
)

const (
	ModelDirName = "bge-m3"
	Dimension    = 1024
)

// Engine turns texts into unit-normalized vectors.
type Engine interface {
	Embed(texts []string) ([][]float32, error)
	Close() error
}

// DefaultModelPath is $XDG_DATA_HOME/canopy/models/bge-m3.
func DefaultModelPath() string {
	return filepath.Join(config.DataHome(), "models", ModelDirName)
}

// ModelAvailable reports whether the model files are downloaded.
func ModelAvailable() bool {
	p := DefaultModelPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(p, "model.onnx"))
	return err == nil
}
