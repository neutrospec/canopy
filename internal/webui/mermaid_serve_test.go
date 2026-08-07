package webui

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/neutrospec/canopy/internal/mermaid"
)

// P3: the served mermaid bundle IS the validator's bundle — byte for byte.
// If these ever diverge, lint could pass a diagram the renderer breaks on.
func TestMermaidBundleServedFromValidator(t *testing.T) {
	s, _ := taskTestServer(t)
	h := s.Handler()
	req := httptest.NewRequest("GET", "/static/vendor/mermaid.min.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("bundle route status = %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), mermaid.Bundle) {
		t.Errorf("served bundle differs from mermaid.Bundle (%d vs %d bytes)", rec.Body.Len(), len(mermaid.Bundle))
	}
}
