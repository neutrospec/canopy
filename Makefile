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

.PHONY: build build-lite test test-ui test-cli test-assets fmt deps install release-check release-tag i18n-check release-lineage-check

build: deps
	go build -tags ORT -ldflags "$(LDFLAGS)" -o canopy ./cmd/canopy

# keyword-search-only binary, no cgo/native deps
build-lite:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o canopy ./cmd/canopy

test:
	bash scripts/test-go.sh ./internal/... ./cmd/canopy

test-cli:
	python3 scripts/test-cli.py

test-assets:
	python3 scripts/test-native-download.py
	python3 scripts/check-assets.py

# Client interaction checks use Node's built-in runner, without npm packages.
# Node is a development-only dependency; the binary still embeds all UI assets.
test-ui:
	node --test internal/webui/uitest/*.test.cjs

fmt:
	gofmt -w .

# Verify translated docs (README.en.md, docs/en/*) are current with their
# Korean sources — invariants L1-L3, see docs/i18n.md.
i18n-check:
	scripts/i18n-check.sh

# Every release tag must sit on origin/main (invariant J7). CI runs this on
# each release; run it by hand to audit lineage anytime.
release-lineage-check:
	scripts/release-lineage-check.sh

# Native preparation validates the host tuple and pinned archive digest even
# when a library already exists. Each stage has its own checked exit status.
deps:
	python3 scripts/prepare-native.py --libdir "$(LIBDIR)"

install: build
	install -m 0755 canopy $(HOME)/.local/bin/canopy 2>/dev/null || install -m 0755 canopy /opt/homebrew/bin/canopy

# --- release ---------------------------------------------------------------
# Pre-flight gate before tagging: clean tree, green tests, clean gofmt
# (invariants F1/F3). See docs/versioning.md for the full release runbook.
release-check:
	@test -z "$$(git status --porcelain)" || { echo "working tree dirty — commit first"; exit 1; }
	$(MAKE) test test-ui test-cli test-assets i18n-check
	go vet ./...
	@git diff --exit-code
	@test -z "$$(gofmt -l .)" || { echo "gofmt not clean: $$(gofmt -l .)"; exit 1; }
	@echo "✓ ready to release $(VERSION)"

# Tag the current commit and push it. GoReleaser (.goreleaser.yaml) turns the
# tag into a GitHub release with a generated changelog, then publishes the
# source formula to neutrospec/homebrew-tap with the release URL and digest.
#
# Releases are cut from main only (invariant J7). Guard order matters:
#   1. cheap guards first (V set, on main) — fail before the test suite runs;
#   2. fetch + require local main to CONTAIN origin/main — an outdated clone
#      would otherwise tag an old commit that still passes J7 (an old commit
#      is an ancestor of origin/main too) and ship stale code under a new
#      version number. The fetch is not optional: releasing offline is wrong
#      anyway since the pushes below would fail;
#   3. push the branch BEFORE the tag so origin/main can never fall behind
#      the tag: pushing a tag uploads its commit objects but leaves
#      origin/main pointing at the old commit, which is how v0.4.1 shipped
#      while origin/main still lagged.
release-tag:
	@test -n "$(V)" || { echo "usage: make release-tag V=0.1.0"; exit 1; }
	@test "$$(git rev-parse --abbrev-ref HEAD)" = "main" || \
		{ echo "릴리스는 main에서만 (현재: $$(git rev-parse --abbrev-ref HEAD))"; exit 1; }
	git fetch origin main
	@git merge-base --is-ancestor origin/main main || \
		{ echo "로컬 main이 origin/main을 포함하지 않음 (뒤처짐/분기) — git pull --rebase 먼저"; exit 1; }
	@$(MAKE) release-check
	git push origin main
	git tag -a v$(V) -m "canopy v$(V)"
	git push origin v$(V)
	@echo "✓ tagged v$(V) — GitHub Actions will publish the release and Homebrew formula"
