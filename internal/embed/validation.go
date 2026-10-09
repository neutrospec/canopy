package embed

import (
	"context"
	"fmt"
	"math"
)

// NormTolerance admits float32 rounding without accepting non-unit vectors.
const NormTolerance = 0.01
const MaxBatchTexts = 16
const MaxBatchRunes = 32000
const MaxTextRunes = 16000

func ValidateVector(v []float32, dimension int) error {
	if len(v) == 0 || (dimension > 0 && len(v) != dimension) {
		return fmt.Errorf("vector dimension %d, want %d", len(v), dimension)
	}
	var norm float64
	for _, f := range v {
		x := float64(f)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("vector contains nonfinite values")
		}
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 || math.Abs(norm-1) > NormTolerance {
		return fmt.Errorf("vector norm %.6g is not unit (tolerance %.3g)", norm, NormTolerance)
	}
	return nil
}

func ValidateVectors(vs [][]float32, count, dimension int) error {
	if len(vs) != count {
		return fmt.Errorf("embedding count %d, want %d", len(vs), count)
	}
	for i, v := range vs {
		if err := ValidateVector(v, dimension); err != nil {
			return fmt.Errorf("embedding %d: %w", i, err)
		}
		if dimension == 0 {
			dimension = len(v)
		}
	}
	return nil
}

// EmbedContext checks cancellation between bounded batches. Native work
// already running may finish; no subsequent batch starts after cancellation.
func EmbedContext(ctx context.Context, eng Engine, texts []string) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if eng == nil {
		return nil, fmt.Errorf("embedding engine unavailable")
	}
	for _, s := range texts {
		if len([]rune(s)) > MaxTextRunes {
			return nil, fmt.Errorf("embedding input exceeds %d runes", MaxTextRunes)
		}
	}
	var out [][]float32
	for start := 0; start < len(texts); {
		end, total := start, 0
		for end < len(texts) && end-start < MaxBatchTexts {
			n := len([]rune(texts[end]))
			if end > start && total+n > MaxBatchRunes {
				break
			}
			total += n
			end++
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var vs [][]float32
		var err error
		if e, ok := eng.(interface {
			EmbedContext(context.Context, []string) ([][]float32, error)
		}); ok {
			vs, err = e.EmbedContext(ctx, texts[start:end])
		} else {
			vs, err = eng.Embed(texts[start:end])
		}
		if err != nil {
			return nil, err
		}
		if err = ValidateVectors(vs, end-start, 0); err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, vs...)
		start = end
	}
	return out, nil
}

// Gate serializes an engine instance and rechecks cancellation after entry.
type Gate chan struct{}

func NewGate() Gate { return make(chan struct{}, 1) }
func (g Gate) Run(ctx context.Context, fn func() ([][]float32, error)) ([][]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case g <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-g }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vs, err := fn()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return vs, nil
}
