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

.PHONY: build build-lite test deps install

build: deps
	go build -tags ORT -o canopy ./cmd/canopy

# keyword-search-only binary, no cgo/native deps
build-lite:
	go build -o canopy ./cmd/canopy

test:
	go test ./internal/...

deps: $(LIBDIR)/libtokenizers.a

$(LIBDIR)/libtokenizers.a:
	mkdir -p $(LIBDIR)
	curl -sL https://github.com/daulet/tokenizers/releases/download/$(TOKENIZERS_VERSION)/libtokenizers.darwin-arm64.tar.gz | tar xz -C $(LIBDIR)

install: build
	install -m 0755 canopy $(HOME)/.local/bin/canopy 2>/dev/null || install -m 0755 canopy /opt/homebrew/bin/canopy
