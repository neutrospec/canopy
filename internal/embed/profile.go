package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// ProfileIDFor is cheap: file identity/size/mtime/ctime plus the small
// manifest, engine output, normalization and chunker versions. It never
// hashes weight tensors during a query. An explicit VerifyModel does that.
func ProfileIDFor(model string) string {
	dir := ModelPath(model)
	return profileAt(dir)
}

func profileAt(dir string) string {
	h := sha256.New()
	fmt.Fprintf(h, "hugot-v0.7.5/ORT/first-output/mean-if-3d/l2/dimension=%d/%s/%s\x00%s", Dimension, ChunkerVersion, NormalizationVersion, dir)
	// WalkDir deliberately does not follow its root symlink. Resolve that
	// directory, then add declared dependencies through nested directory aliases.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	fmt.Fprintf(h, "/root=%s", dir)
	var names []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if rel, err := filepath.Rel(dir, path); err == nil {
				names = append(names, rel)
			}
		}
		return nil
	})
	names = append(names, "model.onnx", "config.json", "tokenizer.json", "tokenizer_config.json", "special_tokens_map.json")
	refs, err := profileDependencies(filepath.Join(dir, "model.onnx"))
	if err != nil {
		fmt.Fprintf(h, "/dependencies=%v", err)
	}
	names = append(names, refs...)
	// Manifest paths may traverse a symlinked directory; stat their targets
	// too, while hashing only the small manifest itself.
	manifestData, manifestErr := readSmallManifest(filepath.Join(dir, ManifestName))
	if manifestErr == nil {
		var m Manifest
		if json.Unmarshal(manifestData, &m) == nil {
			for name := range m.Files {
				if !filepath.IsAbs(name) && !strings.HasPrefix(filepath.ToSlash(filepath.Clean(name)), "../") {
					names = append(names, name)
				}
			}
		}
	}
	sort.Strings(names)
	for i, name := range names {
		if i > 0 && name == names[i-1] {
			continue
		}
		path := filepath.Join(dir, name)
		fmt.Fprintf(h, "/%s/%s", name, fileIdentity(path))
		if name == ManifestName {
			if manifestErr == nil {
				h.Write(manifestData)
			} else {
				fmt.Fprintf(h, "/manifest=%v", manifestErr)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readSmallManifest(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	const maxManifest = 1 << 20
	if !info.Mode().IsRegular() || info.Size() > maxManifest {
		return nil, fmt.Errorf("invalid model manifest size/kind")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxManifest {
		return nil, fmt.Errorf("invalid model manifest size/kind")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxManifest+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxManifest {
		return nil, fmt.Errorf("invalid model manifest size/kind")
	}
	return b, nil
}

func fileIdentity(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path + ":" + err.Error()
	}
	real, _ := filepath.EvalSymlinks(path)
	var b strings.Builder
	fmt.Fprintf(&b, "%s/%d/%d/%v", real, info.Size(), info.ModTime().UnixNano(), info.Mode())
	v := reflect.ValueOf(info.Sys())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		for _, key := range []string{"Dev", "Ino", "Ctim", "Ctimespec", "Ctime"} {
			if f := v.FieldByName(key); f.IsValid() {
				fmt.Fprintf(&b, "/%s=%v", key, f.Interface())
			}
		}
	}
	return b.String()
}

var dependencyCache struct {
	sync.Mutex
	entries map[string]dependencyEntry
}

type dependencyEntry struct {
	names []string
	err   error
}

// Graph declarations are small but parsing them on every ProfileCurrent check
// would add repeated file I/O. Cache only while the graph's own identity holds;
// every declared sidecar is still statted on every profile check.
func profileDependencies(graph string) ([]string, error) {
	key := fileIdentity(graph)
	dependencyCache.Lock()
	e, ok := dependencyCache.entries[key]
	dependencyCache.Unlock()
	if ok {
		return e.names, e.err
	}
	names, err := externalFiles(graph)
	dependencyCache.Lock()
	if dependencyCache.entries == nil {
		dependencyCache.entries = map[string]dependencyEntry{}
	}
	if len(dependencyCache.entries) >= 32 {
		clear(dependencyCache.entries)
	}
	dependencyCache.entries[key] = dependencyEntry{names, err}
	dependencyCache.Unlock()
	return names, err
}
