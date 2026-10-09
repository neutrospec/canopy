//go:build darwin || linux

package embed

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO has no writer. Readiness must reject it before an open can block.
// Use a child process so the timeout also bounds regressions in this test.
func TestModelReadinessRejectsSpecialFiles(t *testing.T) {
	if dir := os.Getenv("CANOPY_SPECIAL_MODEL_FIXTURE"); dir != "" {
		if s, err := CheckModel(dir); err == nil {
			t.Fatalf("special file accepted: %+v", s)
		}
		return
	}
	for _, name := range []string{"model.onnx", ManifestName} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "model")
			bundle(t, dir, true)
			if s, err := CheckModel(dir); err != nil || !s.Ready {
				t.Fatalf("healthy control: %+v %v", s, err)
			}
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestModelReadinessRejectsSpecialFiles$")
			cmd.Env = append(os.Environ(), "CANOPY_SPECIAL_MODEL_FIXTURE="+dir)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("readiness blocked on %s: %v", name, ctx.Err())
			}
			if err != nil {
				t.Fatalf("child: %v\n%s", err, out)
			}
		})
	}
}
