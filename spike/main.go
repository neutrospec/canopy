// Spike: verify that bge-m3 ONNX runs fully in-process (no external API)
// via hugot, and measure single/batch embedding latency on this machine.
//
// Run: go run ./spike [modelPath]
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

func main() {
	home, _ := os.UserHomeDir()
	modelPath := filepath.Join(home, ".canopy", "models", "bge-m3")
	if len(os.Args) > 1 {
		modelPath = os.Args[1]
	}
	ctx := context.Background()

	t0 := time.Now()
	session, err := newSession(ctx)
	must(err)
	defer session.Destroy()

	opts := []hugot.FeatureExtractionOption{pipelines.WithNormalization()}
	if os.Getenv("SPIKE_OUTPUT") != "" {
		opts = append(opts, pipelines.WithOutputName(os.Getenv("SPIKE_OUTPUT")))
	}
	pipe, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      "embed",
		Options:   opts,
	})
	must(err)
	fmt.Printf("model loaded in %s\n", time.Since(t0))

	texts := []string{
		"산업용 프로토콜 비교: OPC-UA와 MQTT의 차이",
		"Industrial protocol comparison between OPC-UA and MQTT",
		"오늘 점심 메뉴로 김치찌개를 먹었다",
	}

	t1 := time.Now()
	out, err := pipe.RunPipeline(ctx, texts[:1])
	must(err)
	fmt.Printf("single embed: %s, dim=%d\n", time.Since(t1), len(out.Embeddings[0]))

	t2 := time.Now()
	out, err = pipe.RunPipeline(ctx, texts)
	must(err)
	fmt.Printf("batch of %d: %s\n", len(texts), time.Since(t2))

	// Sanity: Korean/English paraphrases should be far more similar
	// than the unrelated Korean sentence.
	simKE := cosine(out.Embeddings[0], out.Embeddings[1])
	simKU := cosine(out.Embeddings[0], out.Embeddings[2])
	fmt.Printf("sim(ko-protocol, en-protocol) = %.4f\n", simKE)
	fmt.Printf("sim(ko-protocol, ko-lunch)    = %.4f\n", simKU)
	if simKE > simKU+0.1 {
		fmt.Println("SPIKE OK: semantic ordering is correct")
	} else {
		fmt.Println("SPIKE FAIL: embeddings do not separate topics")
		os.Exit(1)
	}
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na)*math.Sqrt(nb) + 1e-12)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "spike error:", err)
		os.Exit(1)
	}
}
