#!/usr/bin/env bash
# Print the url + sha256 lines for the Homebrew formula's source tarball at a
# given tag. Run after the tag is pushed to GitHub.
#
#   scripts/brew-sha256.sh v0.1.0
#
# Paste the output over the `url`/`sha256` lines in packaging/homebrew/canopy.rb.
set -euo pipefail

tag="${1:?usage: brew-sha256.sh vX.Y.Z}"
url="https://github.com/neutrospec/canopy/archive/refs/tags/${tag}.tar.gz"

sha=$(curl -fsSL "$url" | shasum -a 256 | awk '{print $1}')

printf '  url "%s"\n  sha256 "%s"\n' "$url" "$sha"
