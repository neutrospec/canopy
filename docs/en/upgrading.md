<!-- i18n-source: docs/upgrading.md sha:0bae33e295d0c302657089bda2f7b24e7e73e47d -->
> [한국어](../upgrading.md) · **English**

# Upgrade & Migration Guide

> A step-by-step guide for **moving canopy to a different install or run
> method**. There are three kinds of "version change" and this document covers
> only one of them — don't lump them together:
>
> | Kind | Who does it | Where |
> |---|---|---|
> | **Machine-local data-schema migration** | canopy, automatically (a new version's first run climbs it via `migrate.Ensure()`) | design [versioning.md](../versioning.md), checks [invariants.md](../invariants.md) J |
> | **Install / operational migration** | a person, manually (changing how it's installed or run) | **this document** |
> | Release changelog | GoReleaser, generated | GitHub Releases (from the commit convention) |
>
> Each item carries runnable check commands ([philosophy.md](../philosophy.md) principle 8).
> Items are stacked newest-first.

The ladder handles non-reproducible state on this machine, such as global settings
and XDG layouts. Traveling `_meta/*` files evolve through their own version fields;
completing a machine migration does not update an old wiki fetched later. Search
databases are derived caches, discarded and rebuilt on a cache-version mismatch.
Preserve `logs/` and wiki state; they are not caches.

## First: how am I installed right now

```bash
which -a canopy                 # 어느 바이너리가 PATH에 잡히는지 (여러 개면 두 벌 설치)
brew list --versions canopy     # brew 설치 여부·버전
canopy version                  # 앱·데이터 스키마·캐시 스키마 버전
```

- Only `/opt/homebrew/bin/canopy` (Apple Silicon) or `/usr/local/bin/canopy` (Intel) → **single brew install** — the recommended state.
- If `~/.local/bin/canopy` also shows up, a source/`make install` copy remains — clean it up below.

> **Key fact — a binary's location is independent of XDG data.** canopy's
> config, model, cache, and state live under `$XDG_*_HOME` (or HOME-based
> defaults). Changing only the binary location under the same HOME/XDG reuses
> those paths. The checks below confirm paths, counts, and service startup; they
> do not prove complete data preservation. For content preservation, compare
> actual files with a separate backup and account for intended migrations.

---

## Source / `make install` → Homebrew + `brew services` (v0.2.0+, 2026-07)

**Who** — anyone who has been using `~/.local/bin/canopy` from `make install`
(or a hand-copied `/opt/homebrew/bin`) built from source.

**Why** — since v0.2.0 the [Homebrew tap](../homebrew-guide.md) is the official
distribution channel. brew manages the onnxruntime/tokenizer dependencies and
upgrades, and `brew services` can run the web UI as a persistent service
(launchd). Two installs cause confusion about which one runs and let versions
drift apart.

**Prerequisites** — brew has installed it (`brew install neutrospec/tap/canopy`),
and the wiki is named by `default_wiki` in `~/.config/canopy/config.toml`. The
service runs with no cwd and no `--wiki`, so that config is the only way it
finds the wiki.

### Steps

```bash
# 1. brew로 설치 (아직 안 했다면)
brew install neutrospec/tap/canopy

# 2. 모델은 이전할 필요 없음 — XDG 데이터라 그대로 재사용된다. 확인만:
canopy model status --json | jq .model_path
#   → ~/.local/share/canopy/models/... (brew 바이너리도 같은 경로를 본다)

# 3. 옛 바이너리 제거
rm ~/.local/bin/canopy
hash -r                          # 셸의 명령 경로 캐시 비우기 (또는 새 셸)

# 4. 이제 bare `canopy`가 brew로 해석되는지 확인
which canopy                     # → /opt/homebrew/bin/canopy

# 5. (선택) 웹 UI를 상시 서비스로
brew services start canopy       # localhost:8737, 로그인 시 자동 시작도 등록
```

### Checks

```bash
# 한 벌만 남았나
which -a canopy | sort -u                 # brew 경로 하나만
test ! -e ~/.local/bin/canopy && echo ok  # 옛 사본 없음

# 데이터 경로·페이지 수 확인 (전체 내용 보존을 증명하지 않음)
canopy model status --json | jq .model_path
canopy status --json | jq .pages          # 페이지 수 그대로

# 서비스가 실제로 서빙하나
brew services list | grep canopy          # started
curl -s -o /dev/null -w '%{http_code}\n' localhost:8737/   # 200
```

### Rolling back

The same HOME/XDG reuses existing data and configuration paths. Before returning
to an older version, check file and schema compatibility separately; restore a
backup for that version if needed.

```bash
brew services stop canopy         # 서비스 내리기 (필요 시)
# 소스 설치로 복귀하려면 canopy 저장소에서:
make install                      # ~/.local/bin/canopy 재생성
```

### Pitfalls

- **Re-running `make install` recreates `~/.local/bin/canopy`.** After going
  brew-only, upgrade the daily binary with `brew upgrade canopy && brew services
  restart canopy`. To test a local dev build without polluting PATH, use
  `make build` in the repo and run `./canopy …`, or test main via brew with
  `brew install --HEAD neutrospec/tap/canopy`.
- **Minimal-PATH environments (system cron, launchd, etc.) don't read
  `.zshenv`.** Calling `canopy` there may fail because `/opt/homebrew/bin` isn't
  on PATH — use an absolute path or add the brew prefix to that job's PATH.
  (An agent like hermes that inherits your login environment rides `.zshenv`'s
  PATH, so bare `canopy` is enough.)
- **Automation that hardcodes the old path** (cron prompts, scripts) should
  switch `~/.local/bin/canopy` to bare `canopy` — after removal the stale
  absolute path breaks.

---

## Going forward: routine upgrades

With a single brew install, picking up a new release is two lines.

```bash
brew upgrade canopy
brew services restart canopy      # 서비스로 웹 UI를 돌리는 경우 (에셋이 바이너리에
                                  # 내장되므로 재시작해야 새 UI가 반영된다)
```

For a release that needs a machine-local data-schema migration, the **first `canopy` run
after the upgrade climbs it automatically** (confirm with `canopy migrate
status`). There is nothing for you to do by hand — that's the boundary between
automatic migration and this document (manual migration).

After upgrading, refresh embedded guidance with `canopy skills install`. For
skills in an arbitrary directory, pass the same `--dir <path>`. Use root in
`canopy status --json` to check that the service and agent use the same HOME/XDG
settings and wiki.
