// Package buildinfo carries the binary's version metadata. The values are
// stamped at build time via -ldflags (see the Makefile and .goreleaser.yaml);
// an unstamped local build reports "dev".
//
// This is the *application* version (semver, e.g. v0.1.0). It is distinct
// from two other version numbers in the codebase, and the three must not be
// conflated:
//
//   - the data-schema version (internal/migrate) — how far the durable
//     on-disk state has been migrated;
//   - the cache-schema version (internal/store.SchemaVersion) — the throwaway
//     index DB layout, bumped to force a rebuild.
package buildinfo

import "runtime"

// Overridden by the linker in release builds, e.g.:
//
//	-X github.com/neutrospec/canopy/internal/buildinfo.version=v0.1.0
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version is the semantic version of this binary ("dev" if unstamped).
func Version() string { return version }

// Commit is the short git SHA this binary was built from.
func Commit() string { return commit }

// Date is the build timestamp (RFC3339, or "unknown").
func Date() string { return date }

// GoVersion is the Go toolchain the binary was compiled with.
func GoVersion() string { return runtime.Version() }
