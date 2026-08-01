#!/usr/bin/env bash
# Release lineage check (invariant J7, docs/versioning.md). Every release tag
# vX.Y.Z must point at a commit that lives on origin/main's history — i.e. an
# ancestor of origin/main. `git merge-base --is-ancestor` counts a commit as
# its own ancestor, so a freshly cut tag whose commit == origin/main passes.
#
# This catches two failure modes:
#   1. tag pushed but branch not (`make release-tag` once pushed only the tag,
#      leaving the released code absent from main — origin/main fell behind);
#   2. a force-push to main that orphans a previously released commit.
#
# Exit non-zero if any release tag points off origin/main. Portable to macOS
# bash 3.2 (no mapfile/associative arrays). Zero release tags = pass.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if ! git fetch origin main --quiet 2>/dev/null; then
  echo "release-lineage: warning — origin fetch 실패, 로컬에 기록된 origin/main 기준으로 점검함 (낡았을 수 있음)" >&2
fi
if ! git rev-parse --verify --quiet origin/main >/dev/null; then
  echo "release-lineage: origin/main 을 확인할 수 없음 — 먼저 fetch 하라" >&2
  exit 1
fi

stale=0
for tag in $(git tag -l 'v*'); do
  commit="$(git rev-parse --verify --quiet "${tag}^{commit}")" || continue
  if ! git merge-base --is-ancestor "$commit" origin/main; then
    echo "STALE: $tag ($(git rev-parse --short "$commit")) 가 origin/main 히스토리에 없음"
    stale=1
  fi
done

if [ "$stale" -ne 0 ]; then
  echo "release-lineage: FAIL — 위 태그가 origin/main 밖을 가리킴 (불변식 J7)" >&2
  exit 1
fi
echo "release-lineage: OK — 모든 릴리스 태그가 origin/main 위에 있음"
