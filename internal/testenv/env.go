// Package testenv keeps test-process state and child commands outside the
// developer's real HOME. It is used only by TestMain, before any tests run.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func Run(m *testing.M) int {
	// Helper processes inherit the same lock/cache identity as their parent.
	if os.Getenv("CANOPY_TEST_ENV_ACTIVE") == "1" {
		return m.Run()
	}
	root, err := os.MkdirTemp("", "canopy-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(root)
	for key, sub := range map[string]string{
		"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache",
		"XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state", "TMPDIR": "tmp",
	} {
		path := filepath.Join(root, sub)
		if err := os.MkdirAll(path, 0700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := os.Setenv(key, path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	os.Setenv("CANOPY_TEST_ENV_ACTIVE", "1")
	return m.Run()
}
