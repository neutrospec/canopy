# canopy build
#
# The ORT build embeds semantic search (hugot + ONNX Runtime). It needs:
#   brew install onnxruntime            → libonnxruntime.dylib
#   $XDG_DATA_HOME/canopy/lib/libtokenizers.a (~/.local/share/canopy/lib)
#                                       → prebuilt from daulet/tokenizers releases
# `make deps` fetches the static tokenizer lib.

TOKENIZERS_VERSION := v1.27.0
XDG_DATA := $(or $(XDG_DATA_HOME),$(HOME)/.local/share)
LIBDIR := $(XDG_DATA)/canopy/lib
export CGO_LDFLAGS := -L$(LIBDIR)

# Version metadata stamped into the binary (see internal/buildinfo). VERSION
# tracks `git describe`, so a tagged build reports e.g. v0.1.0 and an untagged
# one v0.1.0-3-gabc1234[-dirty]. Override with `make build VERSION=...`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/neutrospec/canopy/internal/buildinfo
LDFLAGS := -X $(PKG).version=$(VERSION) -X $(PKG).commit=$(COMMIT) -X $(PKG).date=$(DATE)

.PHONY: build build-lite test fmt deps install release-check release-tag

build: deps
	go build -tags ORT -ldflags "$(LDFLAGS)" -o canopy ./cmd/canopy

# keyword-search-only binary, no cgo/native deps
build-lite:
	go build -ldflags "$(LDFLAGS)" -o canopy ./cmd/canopy

test:
	go test ./internal/...

fmt:
	gofmt -w .

deps: $(LIBDIR)/libtokenizers.a

$(LIBDIR)/libtokenizers.a:
	mkdir -p $(LIBDIR)
	curl -sL https://github.com/daulet/tokenizers/releases/download/$(TOKENIZERS_VERSION)/libtokenizers.darwin-arm64.tar.gz | tar xz -C $(LIBDIR)

install: build
	install -m 0755 canopy $(HOME)/.local/bin/canopy 2>/dev/null || install -m 0755 canopy /opt/homebrew/bin/canopy

# --- release ---------------------------------------------------------------
# Pre-flight gate before tagging: clean tree, green tests, clean gofmt
# (invariants F1/F3). See docs/versioning.md for the full release runbook.
release-check:
	@test -z "$$(git status --porcelain)" || { echo "working tree dirty — commit first"; exit 1; }
	go test ./internal/...
	@test -z "$$(gofmt -l .)" || { echo "gofmt not clean: $$(gofmt -l .)"; exit 1; }
	@echo "✓ ready to release $(VERSION)"

# Tag the current commit and push it. GoReleaser (.goreleaser.yaml) turns the
# tag into a GitHub release with a generated changelog. Then refresh the
# Homebrew formula url+sha256 (scripts/brew-sha256.sh v$(V)) and push it to the
# neutrospec/homebrew-tap repo.
release-tag: release-check
	@test -n "$(V)" || { echo "usage: make release-tag V=0.1.0"; exit 1; }
	git tag -a v$(V) -m "canopy v$(V)"
	git push origin v$(V)
	@echo "✓ tagged v$(V) — next: goreleaser release, then bump the Homebrew formula"
