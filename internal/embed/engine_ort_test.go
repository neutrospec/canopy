//go:build ORT

package embed

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestORTLoadAndInfer(t *testing.T) {
	model := os.Getenv("CANOPY_TEST_MODEL")
	if model == "" {
		t.Skip("native load/infer unverified: CANOPY_TEST_MODEL unset")
	}
	SetModelDir(model)
	if !Available() {
		t.Fatal("requested runtime library unavailable")
	}
	status, err := CheckModel(model)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	eng, err := New()
	if err != nil {
		t.Fatalf("native load failed: %v", err)
	}
	defer eng.Close()
	load := time.Since(start)
	texts := []string{"프로토콜은 산업용 통신 규칙이다.", "A cache stores derived results.", "대기 중 canceled request는 Embed를 시작하지 않는다."}
	start = time.Now()
	vs, err := EmbedContext(context.Background(), eng, texts)
	if err != nil {
		t.Fatalf("native inference failed: %v", err)
	}
	if err = ValidateVectors(vs, len(texts), Dimension); err != nil {
		t.Fatal(err)
	}
	t.Logf("native_load=pass native_infer=pass profile=%s kind=%s legacy=%v load=%s infer=%s texts=%d dimension=%d", status.Profile, status.Kind, status.Legacy, load, time.Since(start), len(texts), Dimension)
	t.Log("official CLS reference parity=unverified; existing first-output/mean-if-3d/l2 retained")
}
