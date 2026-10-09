package embed

import (
	"strings"
	"testing"
)

func TestSplitPreservesSource(t *testing.T) {
	for _, body := range []string{
		"First paragraph.  \n\n    indented code\n\n\n\nLast paragraph.\n",
		"한글 e\u0301\r\n\r\n```go\r\n\tf()\r\n```\r\n\r\n| x | y |\n|---|---|\n$x^2$\n",
		strings.Repeat("가😀", 2000),
	} {
		for _, c := range Split("Title", body) {
			raw := strings.TrimPrefix(c.Text, "Title\n\n")
			if !strings.Contains(body, raw) {
				t.Fatalf("chunk not in source: %q", raw)
			}
		}
	}
}
