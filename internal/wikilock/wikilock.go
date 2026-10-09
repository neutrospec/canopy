// Package wikilock coordinates cooperating canopy writers across processes and
// symlink aliases. Locks live outside the wiki, so Git cannot commit them.
// Lock order is wiki (exclusive or shared), then file. Locks are not reentrant;
// already leased callers must use explicit Locked variants of state helpers.
package wikilock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

type Guard struct{ lock *flock.Flock }

func (g *Guard) Unlock() error {
	if g == nil || g.lock == nil {
		return nil
	}
	err := g.lock.Unlock()
	if err == nil {
		g.lock = nil
	}
	return err
}

// Canonical resolves symlinks through the nearest existing ancestor, retaining
// the suffix of a not-yet-created file. It does not change state namespaces.
func Canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cur := filepath.Clean(abs)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", err
		}
		suffix = append(suffix, filepath.Base(cur))
		cur = parent
	}
}

func Acquire(ctx context.Context, root string) (*Guard, error) {
	return acquire(ctx, "wiki", root, false)
}
func AcquireShared(ctx context.Context, root string) (*Guard, error) {
	return acquire(ctx, "wiki", root, true)
}
func AcquireFile(ctx context.Context, path string) (*Guard, error) {
	return acquire(ctx, "file", path, false)
}

func acquire(ctx context.Context, kind, path string, shared bool) (*Guard, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := Canonical(path)
	if err != nil {
		return nil, err
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cache = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(cache, "canopy", "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(kind + "\x00" + key))
	l := flock.New(filepath.Join(dir, hex.EncodeToString(sum[:])+".lock"))
	var ok bool
	if shared {
		ok, err = l.TryRLockContext(ctx, 10*time.Millisecond)
	} else {
		ok, err = l.TryLockContext(ctx, 10*time.Millisecond)
	}
	if err != nil || !ok {
		l.Close()
		if err == nil {
			err = fmt.Errorf("lock unavailable: %s", path)
		}
		return nil, err
	}
	return &Guard{lock: l}, nil
}
