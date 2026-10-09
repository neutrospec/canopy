#!/usr/bin/env bash
set -euo pipefail

# Isolate even Go's startup and child commands; TestMain also protects direct
# `go test` test processes. Reuse the caller's module/build caches deliberately.
canopy_gopath="${GOPATH:-$HOME/go}"
canopy_modcache="${GOMODCACHE:-${canopy_gopath%%:*}/pkg/mod}"
canopy_test_root="$(mktemp -d "${TMPDIR:-/tmp}/canopy-go.XXXXXX")"
trap 'rm -rf "$canopy_test_root"' EXIT
mkdir -p "$canopy_test_root"/{home,config,cache,data,state,tmp}
canopy_buildcache="${GOCACHE:-$canopy_test_root/go-build}"
env HOME="$canopy_test_root/home" \
  XDG_CONFIG_HOME="$canopy_test_root/config" XDG_CACHE_HOME="$canopy_test_root/cache" \
  XDG_DATA_HOME="$canopy_test_root/data" XDG_STATE_HOME="$canopy_test_root/state" \
  TMPDIR="$canopy_test_root/tmp" GOMODCACHE="$canopy_modcache" GOCACHE="$canopy_buildcache" \
  GOTELEMETRY=off go test -buildvcs=false "$@"
