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
	session   *hugot.Session
	pipe      *pipelines.FeatureExtractionPipeline
	gate      Gate
	profile   string
	modelPath string
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
	modelPath := DefaultModelPath()
	status, err := CheckModel(modelPath)
	if err != nil {
		return nil, fmt.Errorf("embedding model unavailable at %s: %w", modelPath, err)
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
		ModelPath: modelPath,
		Name:      "canopy-embed",
		Options:   []hugot.FeatureExtractionOption{pipelines.WithNormalization()},
	})
	if err != nil {
		session.Destroy()
		return nil, err
	}
	if profileAt(modelPath) != status.Profile {
		session.Destroy()
		return nil, fmt.Errorf("model changed during load; retry after installation completes")
	}
	return &ortEngine{session: session, pipe: pipe, gate: NewGate(), profile: status.Profile, modelPath: modelPath}, nil
}

func (e *ortEngine) Embed(texts []string) ([][]float32, error) {
	return e.EmbedContext(context.Background(), texts)
}

func (e *ortEngine) ProfileID() string    { return e.profile }
func (e *ortEngine) ProfileCurrent() bool { return profileAt(e.modelPath) == e.profile }

func (e *ortEngine) EmbedContext(ctx context.Context, texts []string) ([][]float32, error) {
	return e.gate.Run(ctx, func() ([][]float32, error) {
		if !e.ProfileCurrent() {
			return nil, fmt.Errorf("model files changed; reload embedding engine")
		}
		if len(texts) > MaxBatchTexts {
			return nil, fmt.Errorf("embedding batch exceeds %d texts; use embed.EmbedContext", MaxBatchTexts)
		}
		total := 0
		for _, text := range texts {
			n := len([]rune(text))
			if n > MaxTextRunes {
				return nil, fmt.Errorf("embedding text exceeds %d runes", MaxTextRunes)
			}
			total += n
		}
		if total > MaxBatchRunes {
			return nil, fmt.Errorf("embedding batch exceeds %d runes", MaxBatchRunes)
		}
		out, err := e.pipe.RunPipeline(ctx, texts)
		if err != nil {
			return nil, err
		}
		if err = ValidateVectors(out.Embeddings, len(texts), Dimension); err != nil {
			return nil, err
		}
		if !e.ProfileCurrent() {
			return nil, fmt.Errorf("model files changed during inference; reload embedding engine")
		}
		return out.Embeddings, nil
	})
}

func (e *ortEngine) Close() error { return e.session.Destroy() }
