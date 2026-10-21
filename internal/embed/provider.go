package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/neutrospec/canopy/internal/config"
)

func external(w *config.Wiki) bool { return w.EmbeddingService.Provider == "openai" }

// ReadyFor never probes the native stack when an external provider is selected.
// Invalid external settings are handled by NewFor, not silently replaced by ORT.
func ReadyFor(w *config.Wiki) bool {
	if p := w.EmbeddingService.Provider; p != "" && p != "builtin" {
		return true
	}
	SetModelDir(w.Cfg.Embedding.Model)
	return Available() && ModelAvailable()
}

func NewFor(w *config.Wiki) (Engine, error) {
	p := w.EmbeddingService.Provider
	if external(w) {
		return newRemote(w.EmbeddingService)
	}
	if p != "" && p != "builtin" {
		return nil, fmt.Errorf("unsupported embedding provider; use builtin or openai")
	}
	SetModelDir(w.Cfg.Embedding.Model)
	return New()
}

func remoteSettings(c config.EmbeddingService) config.EmbeddingService {
	if c.InputFormat == "" || c.InputFormat == "auto" {
		c.InputFormat = "plain"
		if strings.Contains(strings.ToLower(c.Model), "embeddinggemma") {
			c.InputFormat = "embeddinggemma"
		}
	}
	if c.Dimension == 0 {
		if strings.Contains(strings.ToLower(c.Model), "embeddinggemma") {
			c.Dimension = 768
		} else if strings.Contains(strings.ToLower(c.Model), "bge-m3") {
			c.Dimension = 1024
		}
	}
	return c
}

func validateRemote(c config.EmbeddingService) error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("embedding.url must be a full http(s) embeddings endpoint without credentials, query or fragment")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("embedding.model is required for openai provider")
	}
	if c.Dimension < 1 || c.Dimension > 65536 {
		return fmt.Errorf("embedding.dimension must be between 1 and 65536 (required for unknown models)")
	}
	if c.InputFormat != "plain" && c.InputFormat != "embeddinggemma" {
		return fmt.Errorf("embedding.input_format must be auto, plain or embeddinggemma")
	}
	if strings.ContainsAny(c.Token, "\r\n") {
		return fmt.Errorf("embedding.token contains an invalid newline")
	}
	return nil
}

// ProfileFor includes the vector-space contract but excludes credentials. A
// server-side model replacement under the same alias requires a revision bump.
func ProfileFor(w *config.Wiki) string {
	if p := w.EmbeddingService.Provider; p == "" || p == "builtin" {
		return ProfileIDFor(w.Cfg.Embedding.Model)
	}
	return remoteProfile(remoteSettings(w.EmbeddingService))
}

func remoteProfile(c config.EmbeddingService) string {
	c.Token = ""
	b, _ := json.Marshal(struct {
		Service                          config.EmbeddingService
		Chunker, Normalization, Protocol string
	}{c, ChunkerVersion, NormalizationVersion, "openai-float/l2/input-v1"})
	sum := sha256.Sum256(b)
	return "remote/" + hex.EncodeToString(sum[:])
}
