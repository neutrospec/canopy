package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

// remoteEngine owns no ONNX session and is available in CGO-free builds.
type remoteEngine struct {
	cfg    config.EmbeddingService
	client *http.Client
	gate   Gate
}

func newRemote(c config.EmbeddingService) (Engine, error) {
	c = remoteSettings(c)
	if err := validateRemote(c); err != nil {
		return nil, err
	}
	return &remoteEngine{cfg: c, gate: NewGate(), client: &http.Client{
		Timeout:       60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}
func (e *remoteEngine) Close() error      { e.client.CloseIdleConnections(); return nil }
func (e *remoteEngine) ProfileID() string { return remoteProfile(e.cfg) }
func (e *remoteEngine) Embed(texts []string) ([][]float32, error) {
	return e.EmbedContext(context.Background(), texts)
}
func (e *remoteEngine) EmbedContext(ctx context.Context, texts []string) ([][]float32, error) {
	return e.request(ctx, texts, false)
}
func (e *remoteEngine) EmbedQueryContext(ctx context.Context, texts []string) ([][]float32, error) {
	return e.request(ctx, texts, true)
}

func (e *remoteEngine) request(ctx context.Context, texts []string, query bool) ([][]float32, error) {
	return e.gate.Run(ctx, func() ([][]float32, error) {
		if len(texts) == 0 {
			return nil, nil
		}
		if len(texts) > MaxBatchTexts {
			return nil, fmt.Errorf("embedding batch too large")
		}
		inputs := make([]string, len(texts))
		total := 0
		for i, s := range texts {
			n := len([]rune(s))
			total += n
			if n > MaxTextRunes || total > MaxBatchRunes {
				return nil, fmt.Errorf("embedding input too large")
			}
			if e.cfg.InputFormat == "embeddinggemma" {
				if query {
					s = "task: search result | query: " + s
				} else {
					s = "title: none | text: " + s
				}
			}
			inputs[i] = s
		}
		body, err := json.Marshal(struct {
			Model    string   `json:"model"`
			Input    []string `json:"input"`
			Encoding string   `json:"encoding_format"`
		}{e.cfg.Model, inputs, "float"})
		if err != nil {
			return nil, fmt.Errorf("cannot encode embedding request")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.URL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("cannot create embedding request")
		}
		req.Header.Set("Content-Type", "application/json")
		if e.cfg.Token != "" {
			req.Header.Set("Authorization", "Bearer "+e.cfg.Token)
		}
		resp, err := e.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("embedding API request failed (connection or timeout); check endpoint and service")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("embedding API returned HTTP %d", resp.StatusCode)
		}
		const maxResponse = 32 << 20
		raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
		if err != nil || len(raw) > maxResponse {
			return nil, fmt.Errorf("embedding API response unreadable or too large")
		}
		var out struct {
			Data []struct {
				Index     *int       `json:"index"`
				Embedding []*float32 `json:"embedding"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &out) != nil {
			return nil, fmt.Errorf("embedding API returned invalid JSON")
		}
		if len(out.Data) != len(texts) {
			return nil, fmt.Errorf("embedding API returned wrong vector count")
		}
		vectors := make([][]float32, len(texts))
		for _, item := range out.Data {
			if item.Index == nil || *item.Index < 0 || *item.Index >= len(texts) || vectors[*item.Index] != nil {
				return nil, fmt.Errorf("embedding API returned invalid or duplicate index")
			}
			v := make([]float32, len(item.Embedding))
			for i, value := range item.Embedding {
				if value == nil {
					return nil, fmt.Errorf("embedding API returned null vector element")
				}
				v[i] = *value
			}
			if len(v) != e.cfg.Dimension {
				return nil, fmt.Errorf("embedding API dimension %d, configured %d; check model and reindex", len(v), e.cfg.Dimension)
			}
			norm := 0.0
			for _, x := range v {
				if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
					return nil, fmt.Errorf("embedding API returned nonfinite vector")
				}
				norm += float64(x) * float64(x)
			}
			if norm == 0 {
				return nil, fmt.Errorf("embedding API returned zero vector")
			}
			norm = math.Sqrt(norm)
			for i := range v {
				v[i] = float32(float64(v[i]) / norm)
			}
			vectors[*item.Index] = v
		}
		return vectors, ValidateVectors(vectors, len(texts), e.cfg.Dimension)
	})
}

// EmbedQueryContext preserves document/query distinctions while keeping older
// symmetric engines compatible and applying the same batching/validation.
func EmbedQueryContext(ctx context.Context, eng Engine, texts []string) ([][]float32, error) {
	if q, ok := eng.(interface {
		EmbedQueryContext(context.Context, []string) ([][]float32, error)
	}); ok {
		return EmbedContext(ctx, queryAdapter{Engine: eng, query: q.EmbedQueryContext}, texts)
	}
	return EmbedContext(ctx, eng, texts)
}

type queryAdapter struct {
	Engine
	query func(context.Context, []string) ([][]float32, error)
}

func (e queryAdapter) EmbedContext(ctx context.Context, texts []string) ([][]float32, error) {
	return e.query(ctx, texts)
}
