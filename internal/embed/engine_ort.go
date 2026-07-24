//go:build ORT

package embed

import (
	"context"
	"fmt"
	"os"

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

// onnxRuntimeDirs are searched for libonnxruntime.dylib/.so.
var onnxRuntimeDirs = []string{
	"/opt/homebrew/lib",
	"/usr/local/lib",
	"/usr/lib",
}

// findOnnxRuntime returns the directory containing libonnxruntime, or "".
func findOnnxRuntime() string {
	for _, d := range onnxRuntimeDirs {
		for _, name := range []string{"libonnxruntime.dylib", "libonnxruntime.so"} {
			if _, err := os.Stat(d + "/" + name); err == nil {
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
