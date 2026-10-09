// Package safefile publishes completed files without truncating the previous
// version. It is a single-file boundary, not a multi-file transaction.
package safefile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// TempPrefix identifies uncommitted publication scratch. Git staging excludes
// these names, including scratch left behind by a killed writer.
const TempPrefix = ".canopy-tmp-"

// PublishError means the destination was published, but a later durability or
// cleanup step failed. Callers must report applied state instead of retrying a
// mutation as if nothing happened.
type PublishError struct{ Err error }

func (e *PublishError) Error() string { return "file published; follow-up failed: " + e.Err.Error() }
func (e *PublishError) Unwrap() error { return e.Err }
func Published(err error) bool {
	var e *PublishError
	return errors.As(err, &e)
}

// Regular rejects symlinks and special files. A missing path is allowed.
func Regular(path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("refusing nonregular file (including symlink): %s", path)
	}
	return nil
}

// WriteFile atomically replaces a regular file and preserves its permission
// bits. For a new path it uses perm. Parent directories must already exist.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	return publish(path, data, perm, false, nil)
}

// CreateFile atomically publishes a completed file only if path is absent.
// A losing concurrent creator never truncates or replaces the winner.
func CreateFile(path string, data []byte, perm fs.FileMode) error {
	return publish(path, data, perm, true, nil)
}

// hooks are per-call fault boundaries used by tests, never global writer state.
type hooks struct {
	write         func(*os.File, []byte) error
	beforePublish func() error
	afterPublish  func() error
}

func publish(path string, data []byte, perm fs.FileMode, exclusive bool, h *hooks) error {
	if err := Regular(path); err != nil {
		return err
	}
	if st, err := os.Lstat(path); err == nil {
		if exclusive {
			return &os.PathError{Op: "create", Path: path, Err: fs.ErrExist}
		}
		perm = st.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), TempPrefix)
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if h != nil && h.write != nil {
		err = h.write(f, data)
	} else {
		var n int
		n, err = f.Write(data)
		if err == nil && n != len(data) {
			err = fmt.Errorf("short write: %d of %d bytes", n, len(data))
		}
	}
	if err != nil {
		return err
	}
	if err := f.Chmod(perm.Perm()); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if h != nil && h.beforePublish != nil {
		if err := h.beforePublish(); err != nil {
			return err
		}
	}
	if err := Regular(path); err != nil {
		return err
	}
	if exclusive {
		// Hard-linking the completed temp is an atomic, no-replace publication.
		if err := os.Link(tmp, path); err != nil {
			return err
		}
		if err := os.Remove(tmp); err != nil {
			return &PublishError{err}
		}
	} else if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if h != nil && h.afterPublish != nil {
		if err := h.afterPublish(); err != nil {
			return &PublishError{err}
		}
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return &PublishError{err}
	}
	return nil
}

// MoveNoReplace moves a regular source without overwriting any destination.
// Same-filesystem hard-link publication leaves the source recoverable until
// the destination exists. If source removal fails, both complete copies remain.
func MoveNoReplace(src, dst string) error {
	if err := Regular(src); err != nil {
		return err
	}
	if err := Regular(dst); err != nil {
		return err
	}
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := syncDir(filepath.Dir(dst)); err != nil {
		return &PublishError{err}
	}
	if err := os.Remove(src); err != nil {
		return &PublishError{err}
	}
	if err := syncDir(filepath.Dir(src)); err != nil {
		return &PublishError{err}
	}
	return nil
}

// CleanTemps removes reserved scratch left by a dead cooperating writer. The
// caller MUST hold the exclusive wiki lease: shared metadata writers may have
// live temps. It never follows symlinks or traverses Git internals, and never
// treats a Markdown file as disposable scratch.
func CleanTemps(root string) error {
	// The wiki lock already treats aliases as the same root. WalkDir does
	// not follow a symlink root itself, so resolve that root before walking.
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root = real
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !strings.HasPrefix(d.Name(), TempPrefix) || strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		return os.Remove(path)
	})
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
