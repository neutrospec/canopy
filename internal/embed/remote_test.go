package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neutrospec/canopy/internal/config"
)

func remoteWiki(url string) *config.Wiki {
	return &config.Wiki{Cfg: config.Default(), EmbeddingService: config.EmbeddingService{Provider: "openai", URL: url, Token: "test-secret", Model: "embeddinggemma-2", Dimension: 2}}
}

func TestRemoteProviderQueryDocument(t *testing.T) {
	var inputs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing bearer")
		}
		var body struct {
			Model    string
			Input    []string
			Encoding string `json:"encoding_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "embeddinggemma-2" || body.Encoding != "float" {
			t.Errorf("wrong request: %+v", body)
		}
		inputs = append(inputs, body.Input...)
		fmt.Fprint(w, `{"data":[{"index":1,"embedding":[0,4]},{"index":0,"embedding":[3,0]}]}`)
	}))
	defer srv.Close()
	w := remoteWiki(srv.URL)
	w.Cfg.Embedding.Model = "missing-native-model"
	if !ReadyFor(w) {
		t.Fatal("external provider gated on ORT")
	}
	e, err := NewFor(w)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if _, ok := e.(*remoteEngine); !ok {
		t.Fatal("native engine loaded")
	}
	v, err := EmbedContext(context.Background(), e, []string{"doc a", "doc b"})
	if err != nil {
		t.Fatal(err)
	}
	if v[0][0] != 1 || v[1][1] != 1 {
		t.Fatalf("ordering/normalization: %v", v)
	}
	_, err = EmbedQueryContext(context.Background(), e, []string{"q a", "q b"})
	if err != nil {
		t.Fatal(err)
	}
	if inputs[0] != "title: none | text: doc a" || inputs[2] != "task: search result | query: q a" {
		t.Fatalf("roles: %q", inputs)
	}
	if e.(interface{ ProfileID() string }).ProfileID() != ProfileFor(w) {
		t.Fatal("engine and nil-engine profiles differ")
	}
}

func TestRemoteProfileContract(t *testing.T) {
	w := remoteWiki("http://localhost/v1/embeddings")
	before := ProfileFor(w)
	w.EmbeddingService.Token = "rotated"
	if ProfileFor(w) != before {
		t.Fatal("token rotation invalidates vectors")
	}
	base := w.EmbeddingService
	for name, change := range map[string]func(*config.EmbeddingService){
		"model":     func(c *config.EmbeddingService) { c.Model = "bge-m3" },
		"url":       func(c *config.EmbeddingService) { c.URL += "/new" },
		"dimension": func(c *config.EmbeddingService) { c.Dimension++ },
		"revision":  func(c *config.EmbeddingService) { c.Revision = "v2" },
		"format":    func(c *config.EmbeddingService) { c.InputFormat = "plain" },
		"provider":  func(c *config.EmbeddingService) { c.Provider = "builtin" },
	} {
		t.Run(name, func(t *testing.T) {
			w.EmbeddingService = base
			change(&w.EmbeddingService)
			if ProfileFor(w) == before {
				t.Fatal("stale profile")
			}
		})
	}
}

func TestRemoteRejectsInvalidResponses(t *testing.T) {
	for name, body := range map[string]string{
		"null":    `{"data":[{"index":0,"embedding":[1,null]}]}`,
		"missing": `{"data":[]}`, "json": "test-secret", "index": `{"data":[{"index":1,"embedding":[1,0]}]}`,
		"missing index": `{"data":[{"embedding":[1,0]}]}`, "dimension": `{"data":[{"index":0,"embedding":[1]}]}`,
		"zero": `{"data":[{"index":0,"embedding":[0,0]}]}`, "overflow": `{"data":[{"index":0,"embedding":[1e99,0]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer srv.Close()
			e, err := NewFor(remoteWiki(srv.URL))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			_, err = EmbedContext(context.Background(), e, []string{"a"})
			if err == nil || strings.Contains(err.Error(), "test-secret") {
				t.Fatalf("unsafe acceptance/error: %v", err)
			}
		})
	}
}

func TestRemoteHTTPFailuresAndCancellation(t *testing.T) {
	for _, status := range []int{302, 401, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/redirect")
				w.WriteHeader(status)
				fmt.Fprint(w, "test-secret")
			}))
			defer srv.Close()
			e, _ := NewFor(remoteWiki(srv.URL))
			defer e.Close()
			_, err := EmbedContext(context.Background(), e, []string{"a"})
			if err == nil || strings.Contains(err.Error(), "test-secret") || calls != 1 {
				t.Fatalf("unsafe failure: %v calls=%d", err, calls)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = EmbedContext(ctx, e, []string{"a"})
			if err != context.Canceled || calls != 1 {
				t.Fatalf("canceled request: %v", err)
			}
		})
	}
}

func TestRemoteSettings(t *testing.T) {
	for _, name := range []string{"bge-m3", "google/embeddinggemma-2"} {
		w := remoteWiki("http://localhost/v1/embeddings")
		w.EmbeddingService.Model = name
		w.EmbeddingService.Dimension = 0
		e, err := NewFor(w)
		if err != nil {
			t.Fatal(err)
		}
		defer e.Close()
		dim := e.(*remoteEngine).cfg.Dimension
		if (name == "bge-m3" && dim != 1024) || (name != "bge-m3" && dim != 768) {
			t.Fatal(dim)
		}
	}
	for _, endpoint := range []string{"", ":bad", "file:///tmp/foo", "https://user:secret@example.com/embed", "https://example.com/embed?token=secret"} {
		if _, err := NewFor(remoteWiki(endpoint)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("endpoint validation: %v", err)
		}
	}
}

// Waiting for an HTTP response must honor both caller cancellation and the
// client's deadline, without exposing the URL/token through url.Error.
func TestRemoteInFlightCancellationAndTimeout(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			eng, err := NewFor(remoteWiki("http://localhost/v1/embeddings"))
			if err != nil {
				t.Fatal(err)
			}
			e := eng.(*remoteEngine)
			defer e.Close()
			started := make(chan struct{})
			e.client.Transport = remoteTransportFunc(func(r *http.Request) (*http.Response, error) {
				close(started)
				<-r.Context().Done()
				return nil, r.Context().Err()
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if timeout {
				e.client.Timeout = 20 * time.Millisecond
			}
			done := make(chan error, 1)
			go func() { _, err := EmbedContext(ctx, e, []string{"a"}); done <- err }()
			<-started
			if !timeout {
				cancel()
			}
			select {
			case err := <-done:
				if err == nil || strings.Contains(err.Error(), "localhost") || strings.Contains(err.Error(), "test-secret") {
					t.Fatalf("unsafe failure: %v", err)
				}
				if !timeout && err != context.Canceled {
					t.Fatalf("lost cancellation: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("request did not stop")
			}
		})
	}
}

type remoteTransportFunc func(*http.Request) (*http.Response, error)

func (f remoteTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
