package embed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func field(n byte, b []byte) []byte { return append([]byte{n<<3 | 2, byte(len(b))}, b...) }
func externalGraph(name string) []byte {
	entry := append(field(1, []byte("location")), field(2, []byte(name))...)
	tensor := append(field(13, entry), 14<<3, 1)
	return field(7, field(5, tensor))
}
func bundle(t *testing.T, dir string, external bool) map[string][]byte {
	t.Helper()
	files := map[string][]byte{"model.onnx": {58, 0}, "config.json": []byte(`{"hidden_size":1024}`), "tokenizer.json": []byte(`{"version":"1.0"}`)}
	if external {
		files["model.onnx"] = externalGraph("weights.bin")
		files["weights.bin"] = []byte("small weights")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for n, b := range files {
		if err := os.WriteFile(filepath.Join(dir, n), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}
func testManifest(files map[string][]byte, external bool) *Manifest {
	m := &Manifest{Version: 1, Repository: "fixture", Revision: "pinned-test-revision", Kind: "onnx-inline", Pooling: MeanPooling, Dimension: Dimension, Files: map[string]Artifact{}}
	if external {
		m.Kind = "onnx-external"
	}
	for n, b := range files {
		s := sha256.Sum256(b)
		m.Files[n] = Artifact{SHA256: hex.EncodeToString(s[:]), Size: int64(len(b))}
	}
	return m
}
func TestModelReadyKindsAndLegacyAdoption(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline-custom", true: "external-int8"}[external], func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "custom")
			bundle(t, dir, external)
			s, err := CheckModel(dir)
			if err != nil || !s.Ready || !s.Legacy {
				t.Fatalf("legacy not ready: %+v %v", s, err)
			}
			if _, err = VerifyModel(dir, false); err == nil {
				t.Fatal("manifest-less explicit verification succeeded")
			}
			m, err := VerifyModel(dir, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = VerifyModel(dir, false); err != nil {
				t.Fatal(err)
			}
			// Valid symlinked files remain supported and adoption does not replace them.
			original := filepath.Join(dir, "config.json")
			target := filepath.Join(t.TempDir(), "config.json")
			if err = os.Rename(original, target); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(target, original); err != nil {
				t.Fatal(err)
			}
			if _, err = CheckModel(dir); err != nil {
				t.Fatal(err)
			}
			if _, err = VerifyModel(dir, true); err != nil {
				t.Fatal(err)
			}
			if info, _ := os.Lstat(original); info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("adoption replaced symlink")
			}
			_ = m
		})
	}
}
func TestModelReadinessRejectsIncomplete(t *testing.T) {
	for _, change := range []string{"zero", "directory", "missing-sidecar", "invalid-json", "null-json", "manifest-kind"} {
		t.Run(change, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "model")
			bundle(t, dir, true)
			switch change {
			case "zero":
				os.WriteFile(filepath.Join(dir, "model.onnx"), nil, 0644)
			case "directory":
				os.Remove(filepath.Join(dir, "tokenizer.json"))
				os.Mkdir(filepath.Join(dir, "tokenizer.json"), 0755)
			case "missing-sidecar":
				os.Remove(filepath.Join(dir, "weights.bin"))
			case "invalid-json":
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("{"), 0644)
			case "null-json":
				os.WriteFile(filepath.Join(dir, "config.json"), []byte("null"), 0644)
			case "manifest-kind":
				m := testManifest(bundle(t, dir, true), false)
				writeManifest(dir, m)
			}
			if s, err := CheckModel(dir); err == nil {
				t.Fatalf("incomplete ready: %+v", s)
			}
		})
	}
}
func TestModelProfileFileReplacementAndVerify(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "model")
	bundle(t, dir, true)
	if _, err := VerifyModel(dir, true); err != nil {
		t.Fatal(err)
	}
	before := profileAt(dir)
	path := filepath.Join(dir, "weights.bin")
	info, _ := os.Stat(path)
	if err := os.WriteFile(path, []byte("wrong weights"), 0644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, info.ModTime(), info.ModTime())
	if after := profileAt(dir); after == before {
		t.Fatal("same-size same-name same-mtime replacement did not change profile")
	}
	if _, err := VerifyModel(dir, false); err == nil {
		t.Fatal("full verification missed corrupted weight")
	}
}
func TestModelPullFailureRetryAndConcurrency(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "model")
	old := bundle(t, dir, false)
	if _, err := VerifyModel(dir, true); err != nil {
		t.Fatal(err)
	}
	newFiles := map[string][]byte{"model.onnx": externalGraph("weights.bin"), "weights.bin": []byte("new small weights"), "config.json": []byte(`{"hidden_size":1024}`), "tokenizer.json": []byte(`{"version":"2.0"}`)}
	m := testManifest(newFiles, true)
	bad := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := newFiles[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if bad && strings.HasSuffix(r.URL.Path, "weights.bin") {
			b = []byte("bad")
		}
		w.Write(b)
	}))
	defer server.Close()
	if _, err := pullBundle(context.Background(), dir, true, m, server.URL+"/", server.Client(), nil); err == nil {
		t.Fatal("bad hash pull succeeded")
	}
	for n, b := range old {
		got, _ := os.ReadFile(filepath.Join(dir, n))
		if string(got) != string(b) {
			t.Fatalf("failed pull changed old %s", n)
		}
	}
	bad = false
	if _, err := pullBundle(context.Background(), dir, true, m, server.URL+"/", server.Client(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyModel(dir, false); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), ".model.previous-*"))
	if len(backups) != 1 {
		t.Fatalf("previous bundle not retained: %v", backups)
	}
	unlock, err := lockBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err = pullBundle(context.Background(), dir, true, m, server.URL+"/", server.Client(), nil); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("second installer not rejected: %v", err)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failedReader struct{ sent bool }

func (r *failedReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "abc"), nil
	}
	return 0, errors.New("late transfer failure")
}
func TestDownloadDeadlineCancellationAndLateFailure(t *testing.T) {
	sum := sha256.Sum256([]byte("abc"))
	want := Artifact{Size: 3, SHA256: hex.EncodeToString(sum[:])}
	client := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("download context has no deadline")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(&failedReader{})}, nil
	})}
	ctx, cancel := context.WithTimeout(context.Background(), 1000000000)
	defer cancel()
	dst := filepath.Join(t.TempDir(), "model.onnx")
	if err := downloadArtifact(ctx, client, "https://fixture.invalid/model", dst, want); err == nil {
		t.Fatal("late failed transfer accepted")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("failed transfer published artifact")
	}
	ctx, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := Pull(ctx, filepath.Join(t.TempDir(), "model"), true, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestModelXDGAndExplicitPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	if got := ModelPath("custom-int8"); got != filepath.Join(root, "canopy/models/custom-int8") {
		t.Fatal(got)
	}
	if got := ModelPath(root); got != root {
		t.Fatal(got)
	}
	// Manifest remains ordinary JSON, usable by the offline converter.
	b, err := json.Marshal(defaultManifest())
	if err != nil || !json.Valid(b) {
		t.Fatal(err)
	}
}
