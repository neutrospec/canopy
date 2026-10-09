package wiki

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutrospec/canopy/internal/config"
)

func TestScannedGraphResolvesOriginalLinkAddress(t *testing.T) {
	for _, tc := range []struct{ target, want string }{
		{"concepts/foo.md", "foo"}, {"concepts/foo.md.md", "foo.md"},
		{"foo.md", ""}, {"concepts/nested/foo.md", ""}, {"entities/foo.md", ""},
	} {
		t.Run(tc.target, func(t *testing.T) {
			w := &config.Wiki{Root: t.TempDir(), Cfg: config.Default()}
			if err := os.Mkdir(filepath.Join(w.Root, "concepts"), 0755); err != nil {
				t.Fatal(err)
			}
			for name, body := range map[string]string{"foo": "ordinary", "foo.md": "dotted", "ref": "[[" + tc.target + "#heading|label]]\n\n`[[foo]]`"} {
				if err := os.WriteFile(filepath.Join(w.Root, "concepts", name+".md"), []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			scan, err := Scan(w)
			if err != nil {
				t.Fatal(err)
			}
			ref := scan.BySlug["ref"]
			if tc.want == "" && len(ref.Links) != 0 {
				t.Fatalf("invalid address became a real graph edge: %v", ref.Links)
			}
			if tc.want != "" && (len(ref.Links) != 1 || ref.Links[0] != tc.want) {
				t.Fatalf("wrong graph target: %v", ref.Links)
			}
			for target, sources := range scan.Backlinks() {
				for _, source := range sources {
					if source == "ref" && target != tc.want {
						t.Fatalf("false backlink %s <- ref", target)
					}
				}
			}
		})
	}
}
