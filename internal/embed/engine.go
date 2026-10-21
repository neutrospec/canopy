// Package embed computes embeddings using a built-in ONNX model or an optional
// OpenAI-compatible external API. External engines also work in lite builds.
package embed

import (
	"path/filepath"
	"sync"

	"github.com/neutrospec/canopy/internal/config"
)

const Dimension = 1024

// modelDirName is the default model subdirectory under $XDG_DATA_HOME/canopy/models/.
// It can be overridden via SetModelDir, typically from the wiki's Embedding.Model config.
var modelDirName = "bge-m3"
var modelDirMu sync.RWMutex

// Engine turns texts into unit-normalized vectors.
type Engine interface {
	Embed(texts []string) ([][]float32, error)
	Close() error
}

// SetModelDir overrides the default model directory name.
// Call before New() to select a different model (e.g. "bge-m3-int8").
// An empty name is ignored.
func SetModelDir(name string) {
	if name != "" {
		modelDirMu.Lock()
		defer modelDirMu.Unlock()
		modelDirName = name
	}
}

// DefaultModelPath returns $XDG_DATA_HOME/canopy/models/<modelDirName>.
func DefaultModelPath() string {
	modelDirMu.RLock()
	defer modelDirMu.RUnlock()
	return ModelPath(modelDirName)
}

// ModelPath uses the same XDG layout as pull and the offline quantizer.
func ModelPath(name string) string {
	if name == "" {
		name = "bge-m3"
	}
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(config.DataHome(), "models", name)
}

// ModelAvailable reports whether the model files are downloaded.
func ModelAvailable() bool {
	_, err := CheckModel(DefaultModelPath())
	return err == nil
}
