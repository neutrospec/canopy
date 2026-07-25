//go:build ORT

package embed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/options"
	"github.com/knights-analytics/hugot/pipelines"
)

// Available reports whether the ORT backend is compiled in AND the
// ONNX Runtime shared library can be found at runtime.
func Available() bool {
	return findOnnxRuntime() != ""
}

type ortEngine struct {
	session *hugot.Session
	pipe    *pipelines.FeatureExtractionPipeline
}

// onnxRuntimeDirs are the fixed fallback locations searched for
// libonnxruntime.dylib/.so (Apple Silicon brew, Intel brew, system).
var onnxRuntimeDirs = []string{
	"/opt/homebrew/lib",
	"/usr/local/lib",
	"/usr/lib",
}

// findOnnxRuntime returns the directory containing libonnxruntime, or "".
// It checks, in order: an explicit override, the active Homebrew prefix
// (covers Linuxbrew and non-standard prefixes the Homebrew formula relies on),
// then the fixed fallbacks.
func findOnnxRuntime() string {
	var dirs []string
	if d := os.Getenv("CANOPY_ONNXRUNTIME_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	if p := os.Getenv("HOMEBREW_PREFIX"); p != "" {
		dirs = append(dirs, filepath.Join(p, "lib"))
	}
	dirs = append(dirs, onnxRuntimeDirs...)
	for _, d := range dirs {
		for _, name := range []string{"libonnxruntime.dylib", "libonnxruntime.so"} {
			if _, err := os.Stat(filepath.Join(d, name)); err == nil {
				return d
			}
		}
	}
	return ""
}

func New() (Engine, error) {
	if !ModelAvailable() {
		return nil, fmt.Errorf("embedding model not found at %s — run `canopy model pull`", DefaultModelPath())
	}
	dir := findOnnxRuntime()
	if dir == "" {
		return nil, fmt.Errorf("libonnxruntime not found (try `brew install onnxruntime`)")
	}
	ctx := context.Background()
	session, err := hugot.NewORTSession(ctx, options.WithOnnxLibraryPath(dir))
	if err != nil {
		return nil, err
	}
	pipe, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath: DefaultModelPath(),
		Name:      "canopy-embed",
		Options:   []hugot.FeatureExtractionOption{pipelines.WithNormalization()},
	})
	if err != nil {
		session.Destroy()
		return nil, err
	}
	return &ortEngine{session: session, pipe: pipe}, nil
}

func (e *ortEngine) Embed(texts []string) ([][]float32, error) {
	out, err := e.pipe.RunPipeline(context.Background(), texts)
	if err != nil {
		return nil, err
	}
	return out.Embeddings, nil
}

func (e *ortEngine) Close() error { return e.session.Destroy() }
