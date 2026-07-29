# Homebrew formula for canopy (full build: schema-enforced writes, hybrid
# search, web UI). It builds from source so it links against Homebrew's own
# onnxruntime and the platform tokenizer lib — no fragile prebuilt CGO bottle.
#
# This file is the source of truth; copy it into the tap repo
# (github.com/neutrospec/homebrew-tap, path Formula/canopy.rb) at release time.
#
#   Stable:  brew install neutrospec/tap/canopy
#   HEAD:    brew install --HEAD neutrospec/tap/canopy   # builds main, no tag needed
#
# Release step: after `make release-tag V=0.1.0`, fill `url`/`sha256` below with
#   scripts/brew-sha256.sh v0.1.0
class Canopy < Formula
  desc "Local knowledge manager for markdown wikis: schema, hybrid search, web UI"
  homepage "https://github.com/neutrospec/canopy"
  url "https://github.com/neutrospec/canopy/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "fb9a1a58a79c4607183f342a7968b65496401a2c33fb466f2dc8670e3f7389d5"
  license "MIT"
  head "https://github.com/neutrospec/canopy.git", branch: "main"

  depends_on "go" => :build
  depends_on "onnxruntime"

  # Prebuilt static tokenizer lib (daulet/tokenizers), pinned and arch-specific.
  # Keep TOKENIZERS_VERSION in step with the Makefile.
  on_macos do
    on_arm do
      resource "tokenizers" do
        url "https://github.com/daulet/tokenizers/releases/download/v1.27.0/libtokenizers.darwin-arm64.tar.gz"
        sha256 "fb84b8b2e349a5952767ffe80ccd862fc44084de47f3b0cc3f0b7c9d4e649cf7"
      end
    end
    on_intel do
      resource "tokenizers" do
        url "https://github.com/daulet/tokenizers/releases/download/v1.27.0/libtokenizers.darwin-x86_64.tar.gz"
        sha256 "6239efe5a81fde8089ef2df8ae710366542b4e5deab6d8ecb74d7d1862db2ddb"
      end
    end
  end
  on_linux do
    on_arm do
      resource "tokenizers" do
        url "https://github.com/daulet/tokenizers/releases/download/v1.27.0/libtokenizers.linux-arm64.tar.gz"
        sha256 "e96545ad05930c26f51f63d932ee6d3bbd32bbed149e102c5290d587a2293067"
      end
    end
    on_intel do
      resource "tokenizers" do
        url "https://github.com/daulet/tokenizers/releases/download/v1.27.0/libtokenizers.linux-x86_64.tar.gz"
        sha256 "72556cdca798dd4ea7cdaba308e5f0d68a8cb93b67c96edf485b7a0edd7b07f4"
      end
    end
  end

  def install
    libdir = buildpath/"tokenizers-lib"
    resource("tokenizers").stage { libdir.install "libtokenizers.a" }

    ENV["CGO_ENABLED"] = "1"
    ENV.append "CGO_LDFLAGS", "-L#{libdir}"

    ldflags = %W[
      -X github.com/neutrospec/canopy/internal/buildinfo.version=#{version}
      -X github.com/neutrospec/canopy/internal/buildinfo.commit=homebrew
      -X github.com/neutrospec/canopy/internal/buildinfo.date=homebrew
    ].join(" ")
    system "go", "build", "-tags", "ORT", "-ldflags", ldflags, "-o", bin/"canopy", "./cmd/canopy"
  end

  # `brew services start canopy` runs the web UI on http://localhost:8737.
  # It resolves the wiki from default_wiki in ~/.config/canopy/config.toml —
  # there is no cwd or --wiki here, so that config must be set (see caveats).
  service do
    run [opt_bin/"canopy", "serve"]
    keep_alive true
    log_path var/"log/canopy.log"
    error_log_path var/"log/canopy.log"
    # Under launchd there is no login shell/PATH. canopy already falls back to
    # /opt/homebrew/lib for libonnxruntime, but pass the prefix so Linuxbrew and
    # custom prefixes resolve semantic search too.
    environment_variables HOMEBREW_PREFIX: HOMEBREW_PREFIX
  end

  def caveats
    <<~EOS
      Semantic search uses a local bge-m3 ONNX model (~2.3GB). Download it once:
        canopy model pull
      Keyword search works without it. If the ONNX Runtime library is not found
      automatically (e.g. Linuxbrew or a non-standard prefix), point canopy at it:
        export CANOPY_ONNXRUNTIME_DIR="#{formula_opt_lib("onnxruntime")}"

      To run the web UI as a background service (http://localhost:8737):
        brew services start canopy
      It serves the wiki named by default_wiki — set that first if you haven't:
        echo 'default_wiki = "/path/to/wiki"' > ~/.config/canopy/config.toml
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/canopy version")
  end
end
