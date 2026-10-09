package tasks

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestFutureKnownTaskDoneRefusedAndDismissPreservesUnknown(t *testing.T) {
	for _, version := range []int{FormatVersion, FormatVersion + 1} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			w := testWiki(t)
			write(t, w, "concepts/alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nbody")
			if err := os.MkdirAll(Dir(w), 0755); err != nil {
				t.Fatal(err)
			}
			raw := fmt.Sprintf(`{"version":%d,"id":"known","type":"edit","status":"pending","pages":["alpha"],"extra":{"n":9007199254740993}}`, version)
			if err := os.WriteFile(pathOf(w, "known"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Close(w, scan(t, w), "known", StatusDone, "", "agent", now)
			if version == FormatVersion {
				if err != nil {
					t.Fatalf("current version control: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("future known-type task was declared done by an older verifier")
			}
			got, err := os.ReadFile(pathOf(w, "known"))
			if err != nil || string(got) != raw {
				t.Fatalf("refusal modified future task: %v", err)
			}
			if _, err := Close(w, scan(t, w), "known", StatusDismissed, "", "agent", now); err != nil {
				t.Fatalf("explicit dismiss: %v", err)
			}
			got, err = os.ReadFile(pathOf(w, "known"))
			if err != nil || !strings.Contains(string(got), "9007199254740993") {
				t.Fatalf("dismiss lost unknown exact number: %v", err)
			}
		})
	}
}
