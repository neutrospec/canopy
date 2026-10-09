package embed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelManifestSizeLimitAndNormalControl(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "model")
	bundle(t, dir, true)
	if _, err := VerifyModel(dir, true); err != nil {
		t.Fatal(err)
	}
	if s, err := CheckModel(dir); err != nil || !s.Ready {
		t.Fatalf("normal manifest: %+v %v", s, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(strings.Repeat(" ", (1<<20)+1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckModel(dir); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

func BenchmarkRejectOversizedModelManifest(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(strings.Repeat(" ", 8<<20)), 0644); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CheckModel(dir); err == nil {
			b.Fatal("oversized manifest accepted")
		}
	}
}

func TestLegacyDirectoryAliasesInvalidateProfileAndReadiness(t *testing.T) {
	for _, kind := range []string{"real-control", "root-alias", "sidecar-directory-alias"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "model")
			bundle(t, dir, true)
			location := "weights.bin"
			if kind == "root-alias" {
				alias := filepath.Join(root, "alias")
				if err := os.Symlink(dir, alias); err != nil {
					t.Fatal(err)
				}
				dir = alias
			}
			if kind == "sidecar-directory-alias" {
				external := filepath.Join(root, "external")
				if err := os.Mkdir(external, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(dir, "weights.bin"), filepath.Join(external, "weights.bin")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(dir, "sidecar")); err != nil {
					t.Fatal(err)
				}
				location = "sidecar/weights.bin"
				if err := os.WriteFile(filepath.Join(dir, "model.onnx"), externalGraph(location), 0644); err != nil {
					t.Fatal(err)
				}
			}
			before, err := CheckModel(dir)
			if err != nil || !before.Ready {
				t.Fatalf("healthy control: %+v %v", before, err)
			}
			if err := os.WriteFile(filepath.Join(dir, location), []byte("other weights"), 0644); err != nil {
				t.Fatal(err)
			}
			if profileAt(dir) == before.Profile {
				t.Fatal("ordinary sidecar overwrite retained the previous model profile")
			}
			if err := os.Truncate(filepath.Join(dir, location), 0); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckModel(dir); err == nil {
				t.Fatal("empty required sidecar reused cached readiness")
			}
		})
	}
}

func TestExplicitAdoptRepairsStaleManifestAfterBundleValidation(t *testing.T) {
	for _, brokenBundle := range []bool{false, true} {
		t.Run(map[bool]string{false: "healthy-control", true: "missing-sidecar"}[brokenBundle], func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "model")
			bundle(t, dir, true)
			raw := []byte(`{"version":1,"revision":"old","pooling":"previous-pooling","dimension":1024,"kind":"onnx-external","files":{}}`)
			if err := os.WriteFile(filepath.Join(dir, ManifestName), raw, 0644); err != nil {
				t.Fatal(err)
			}
			if brokenBundle {
				if err := os.Remove(filepath.Join(dir, "weights.bin")); err != nil {
					t.Fatal(err)
				}
			}
			_, err := VerifyModel(dir, true)
			if brokenBundle {
				if err == nil {
					t.Fatal("adopt accepted incomplete bundle")
				}
				after, _ := os.ReadFile(filepath.Join(dir, ManifestName))
				if string(after) != string(raw) {
					t.Fatal("failed adoption modified manifest")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := VerifyModel(dir, false); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
