package embed

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanceledGateDoesNotStartInference(t *testing.T) {
	g := NewGate()
	g <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var calls atomic.Int32
	go func() {
		_, err := g.Run(ctx, func() ([][]float32, error) { calls.Add(1); return nil, nil })
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued cancellation did not return")
	}
	<-g
	if calls.Load() != 0 {
		t.Fatalf("canceled queued request started %d native calls", calls.Load())
	}
	// After-gate cancellation check is exercised when both select cases can win.
	for range 200 {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := g.Run(ctx, func() ([][]float32, error) { calls.Add(1); return nil, nil }); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("pre-canceled gate entered native code")
	}
	if _, err := g.Run(context.Background(), func() ([][]float32, error) { calls.Add(1); return [][]float32{{1, 0}}, nil }); err != nil || calls.Load() != 1 {
		t.Fatalf("normal request failed: %d %v", calls.Load(), err)
	}
}

type budgetEngine struct {
	calls, maxTexts, maxRunes int
	cancel                    context.CancelFunc
	badCount                  bool
}

func (e *budgetEngine) Close() error { return nil }
func (e *budgetEngine) Embed(texts []string) ([][]float32, error) {
	e.calls++
	if len(texts) > e.maxTexts {
		e.maxTexts = len(texts)
	}
	total := 0
	for _, s := range texts {
		total += len([]rune(s))
	}
	if total > e.maxRunes {
		e.maxRunes = total
	}
	if e.cancel != nil {
		e.cancel()
	}
	if e.badCount {
		return nil, nil
	}
	v := make([][]float32, len(texts))
	for i := range v {
		v[i] = []float32{1, 0}
	}
	return v, nil
}
func TestEmbeddingInputBudget(t *testing.T) {
	e := &budgetEngine{}
	texts := make([]string, 41)
	for i := range texts {
		texts[i] = strings.Repeat("가", 3000)
	}
	if out, err := EmbedContext(context.Background(), e, texts); err != nil || len(out) != len(texts) {
		t.Fatalf("batch result count: %d %v", len(out), err)
	}
	if e.calls < 2 || e.maxTexts > MaxBatchTexts || e.maxRunes > MaxBatchRunes {
		t.Fatalf("unbounded inference batches: %+v", e)
	}
	before := e.calls
	if _, err := EmbedContext(context.Background(), e, []string{strings.Repeat("x", MaxTextRunes+1)}); err == nil || e.calls != before {
		t.Fatal("oversized input reached engine")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e = &budgetEngine{cancel: cancel}
	if _, err := EmbedContext(ctx, e, texts); !errors.Is(err, context.Canceled) || e.calls != 1 {
		t.Fatalf("canceled batch continued: %d %v", e.calls, err)
	}
	e = &budgetEngine{badCount: true}
	if _, err := EmbedContext(context.Background(), e, []string{"x"}); err == nil {
		t.Fatal("bad output count accepted")
	}
}
