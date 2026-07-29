#!/usr/bin/env bash
# i18n staleness check (invariants L1-L3, docs/i18n.md). Every translated doc
# must record the source file + version it was made from; that version must
# still be current (else the translation is stale and needs re-translating);
# and code fences must not have been translated away. Exit non-zero on any
# problem. Zero translations = pass (nothing to check yet).
#
# Portable to macOS bash 3.2 (no mapfile): translations are gathered by glob.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

translations=()
[ -f README.en.md ] && translations+=(README.en.md)
shopt -s nullglob
translations+=(docs/en/*.md)
shopt -u nullglob

n=${#translations[@]}
if [ "$n" -eq 0 ]; then
	echo "✓ i18n: 0 translation(s) — nothing to check yet"
	exit 0
fi

problems=0
for t in "${translations[@]}"; do
	marker=$(grep -oE '<!-- i18n-source: [^ ]+ sha:[0-9a-f]{40} -->' "$t" | head -1 || true)
	if [ -z "$marker" ]; then
		echo "MISSING marker  $t  (add: <!-- i18n-source: <path> sha:<git hash-object> -->)"
		problems=$((problems + 1))
		continue
	fi
	src=$(printf '%s' "$marker" | sed -E 's/<!-- i18n-source: ([^ ]+) sha:[0-9a-f]{40} -->/\1/')
	recorded=$(printf '%s' "$marker" | sed -E 's/.* sha:([0-9a-f]{40}) -->/\1/')
	if [ ! -f "$src" ]; then
		echo "SOURCE GONE     $t -> $src"
		problems=$((problems + 1))
		continue
	fi
	current=$(git hash-object "$src")
	if [ "$current" != "$recorded" ]; then
		echo "STALE           $t  (source $src changed - re-translate, then set the marker sha to $current)"
		problems=$((problems + 1))
		continue
	fi
	# L3 partial check: code isn't translated, so fence counts must match.
	src_fences=$(grep -c '^```' "$src" || true)
	t_fences=$(grep -c '^```' "$t" || true)
	if [ "$src_fences" != "$t_fences" ]; then
		echo "FENCE MISMATCH  $t  (code blocks: source $src_fences vs translation $t_fences - keep code/commands byte-identical)"
		problems=$((problems + 1))
	fi
done

if [ "$problems" -ne 0 ]; then
	echo "✗ i18n: $problems problem(s) across $n translation(s)"
	exit 1
fi
echo "✓ i18n: $n translation(s) current"
