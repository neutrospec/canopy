package embed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const ManifestName = "canopy-model.json"
const MeanPooling = "first-output/mean-if-3d/l2"
const ModelRevision = "b85c5597da3ff553300a9b41e781429ec75af15e"
const ModelRepository = "EmbeddedLLM/bge-m3-onnx-o2-cpu"

type Artifact struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Manifest struct {
	Version    int                 `json:"version"`
	Repository string              `json:"repository"`
	Revision   string              `json:"revision"`
	Kind       string              `json:"kind"` // onnx-inline or onnx-external
	Pooling    string              `json:"pooling"`
	Dimension  int                 `json:"dimension"`
	Files      map[string]Artifact `json:"files"`
}
type ModelStatus struct {
	Ready    bool      `json:"ready"`
	Path     string    `json:"path"`
	Kind     string    `json:"kind"`
	Legacy   bool      `json:"legacy"`
	Profile  string    `json:"profile"`
	Manifest *Manifest `json:"manifest,omitempty"`
	Repair   string    `json:"repair"`
}

func requiredFiles(dir string) ([]string, string, error) {
	refs, err := externalFiles(filepath.Join(dir, "model.onnx"))
	if err != nil {
		return nil, "", err
	}
	names := []string{"model.onnx", "config.json", "tokenizer.json"}
	names = append(names, refs...)
	for _, name := range []string{"tokenizer_config.json", "special_tokens_map.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			names = append(names, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
	}
	kind := "onnx-inline"
	if len(refs) > 0 {
		kind = "onnx-external"
	}
	sort.Strings(names)
	out := names[:0]
	for _, n := range names {
		if len(out) == 0 || n != out[len(out)-1] {
			out = append(out, n)
		}
	}
	return out, kind, nil
}

func checkFile(dir, name string) (os.FileInfo, error) {
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("%s must be a nonempty regular file (symlink to a file is allowed)", name)
	}
	if strings.HasSuffix(name, ".json") {
		if info.Size() > 64<<20 {
			return nil, fmt.Errorf("%s exceeds JSON size limit", name)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var value map[string]json.RawMessage
		if err = json.Unmarshal(b, &value); err != nil || value == nil {
			return nil, fmt.Errorf("%s must contain a JSON object", name)
		}
	}
	return info, nil
}

func readManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, ManifestName)
	b, err := readSmallManifest(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Version != 1 || m.Revision == "" || m.Dimension != Dimension || m.Pooling != MeanPooling || (m.Kind != "onnx-inline" && m.Kind != "onnx-external") {
		return nil, fmt.Errorf("unsupported model manifest profile; run model verify --adopt for a local compatible bundle")
	}
	for name, a := range m.Files {
		if filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || strings.HasPrefix(name, "../") || name == "." || len(a.SHA256) != 64 || a.Size <= 0 {
			return nil, fmt.Errorf("invalid manifest artifact %q", name)
		}
		if _, err := hex.DecodeString(a.SHA256); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

var statusCache struct {
	sync.Mutex
	entries map[string]*ModelStatus
}

// CheckModel performs cheap readiness, not runtime inference or full checksum
// verification. Cached parsing is invalidated by file inode/size/mtime/ctime.
// It accepts structurally valid legacy bundles without silently writing them.
func CheckModel(dir string) (*ModelStatus, error) {
	profile := profileAt(dir)
	statusCache.Lock()
	if s := statusCache.entries[profile]; s != nil {
		copy := *s
		statusCache.Unlock()
		return &copy, nil
	}
	statusCache.Unlock()
	names, kind, err := requiredFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("model incomplete: %w; run canopy model pull --repair or model verify --adopt", err)
	}
	m, err := readManifest(dir)
	legacy := errors.Is(err, os.ErrNotExist)
	if err != nil && !legacy {
		return nil, err
	}
	if m != nil {
		if m.Kind != kind {
			return nil, fmt.Errorf("manifest model kind differs from ONNX; run model verify")
		}
		for name := range m.Files {
			if !contains(names, name) {
				names = append(names, name)
			}
		}
	}
	for _, name := range names {
		info, err := checkFile(dir, name)
		if err != nil {
			return nil, fmt.Errorf("model incomplete: %w; repair with model pull --repair", err)
		}
		if m != nil {
			a, ok := m.Files[name]
			if !ok || a.Size != info.Size() {
				return nil, fmt.Errorf("manifest artifact missing or size changed: %s; run model verify", name)
			}
		}
	}
	s := &ModelStatus{Ready: true, Path: dir, Kind: kind, Legacy: legacy, Manifest: m, Profile: profile, Repair: "canopy model pull --repair; custom/legacy bundle: canopy model verify --adopt"}
	statusCache.Lock()
	if statusCache.entries == nil {
		statusCache.entries = map[string]*ModelStatus{}
	}
	if len(statusCache.entries) > 32 {
		clear(statusCache.entries)
	}
	statusCache.entries[profile] = s
	statusCache.Unlock()
	copy := *s
	return &copy, nil
}
func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
func hashFile(path string) (Artifact, error) {
	f, err := os.Open(path)
	if err != nil {
		return Artifact{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, nil
}

// VerifyModel hashes a bundle explicitly. adopt records a compatible local
// inline/external/int8 bundle, preserving its files and symlink targets.
func VerifyModel(dir string, adopt bool) (*Manifest, error) {
	if adopt {
		unlock, err := lockBundle(dir)
		if err != nil {
			return nil, err
		}
		defer unlock()
	}
	names, kind, err := requiredFiles(dir)
	if err != nil {
		return nil, err
	}
	existing, err := readManifest(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		if !adopt {
			return nil, err
		}
		// Explicit adoption validates and hashes the current local bundle,
		// replacing stale metadata only after that validation succeeds.
		existing = nil
	}
	if existing == nil && !adopt {
		return nil, fmt.Errorf("legacy model has no manifest; use model verify --adopt")
	}
	if existing != nil {
		for name := range existing.Files {
			if !contains(names, name) {
				names = append(names, name)
			}
		}
	}
	m := &Manifest{Version: 1, Repository: "local", Kind: kind, Pooling: MeanPooling, Dimension: Dimension, Files: map[string]Artifact{}}
	for _, name := range names {
		if _, err = checkFile(dir, name); err != nil {
			return nil, err
		}
		a, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		m.Files[name] = a
		if existing != nil && !adopt {
			if want, ok := existing.Files[name]; !ok || want != a {
				return nil, fmt.Errorf("model digest mismatch: %s; repair or explicitly adopt a reviewed local bundle", name)
			}
		}
	}
	if existing != nil && !adopt {
		return existing, nil
	}
	b, _ := json.Marshal(m.Files)
	sum := sha256.Sum256(b)
	m.Revision = "local-" + hex.EncodeToString(sum[:])
	if err = writeManifest(dir, m); err != nil {
		return nil, err
	}
	return m, nil
}
func writeManifest(dir string, m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".manifest-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, ManifestName))
}

func lockBundle(dir string) (func(), error) {
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return nil, err
	}
	// Canonicalize aliases of the containing directory before choosing a lock.
	parent, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil {
		return nil, err
	}
	lock := filepath.Join(parent, "."+filepath.Base(dir)+".install-lock")
	if err = os.Mkdir(lock, 0700); err != nil {
		return nil, fmt.Errorf("model installer busy (lock %s): %w", lock, err)
	}
	return func() { os.Remove(lock) }, nil
}

// Pull downloads a pinned default bundle. It stages the complete bundle in a
// unique sibling directory and preserves the previous directory as a backup
// until the new bundle is published. Interrupted staging never becomes ready.
func Pull(ctx context.Context, dir string, repair bool, progress func(string)) (*Manifest, error) {
	return pullBundle(ctx, dir, repair, defaultManifest(), "https://huggingface.co/"+ModelRepository+"/resolve/"+ModelRevision+"/", http.DefaultClient, progress)
}
func pullBundle(ctx context.Context, dir string, repair bool, m *Manifest, baseURL string, client *http.Client, progress func(string)) (*Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("model destination is a symlink; use the resolved explicit path for repair")
	}
	unlock, err := lockBundle(dir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if s, err := CheckModel(dir); err == nil && !repair {
		if s.Manifest != nil {
			return s.Manifest, nil
		}
		return nil, fmt.Errorf("legacy model is ready; use model verify --adopt instead of replacing it")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	stage, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".stage-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	names := make([]string, 0, len(m.Files))
	for name := range m.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if progress != nil {
			progress("download " + name)
		}
		if err = downloadArtifact(ctx, client, baseURL+name, filepath.Join(stage, name), m.Files[name]); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	if err = writeManifest(stage, m); err != nil {
		return nil, err
	}
	if _, err = CheckModel(stage); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	// Two directory renames cannot provide a single atomic replacement on all
	// supported OSes. Readers may briefly see unavailable, never a mixed bundle.
	backup := ""
	if _, err = os.Lstat(dir); err == nil {
		backup, err = os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".previous-*")
		if err != nil {
			return nil, err
		}
		if err = os.Remove(backup); err != nil {
			return nil, err
		}
		if err = os.Rename(dir, backup); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err = os.Rename(stage, dir); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, dir); restoreErr != nil {
				return nil, fmt.Errorf("publish failed: %v; restore previous bundle at %s: %v", err, backup, restoreErr)
			}
		}
		return nil, err
	}
	if backup != "" {
		if progress != nil {
			progress("previous bundle retained at " + backup)
		}
	}
	return m, nil
}
func downloadArtifact(ctx context.Context, client *http.Client, url, dst string, want Artifact) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), ".download-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, want.Size+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("download transfer: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if n != want.Size || got != want.SHA256 {
		return fmt.Errorf("download digest/size mismatch (%d bytes)", n)
	}
	return os.Rename(tmp, dst)
}
