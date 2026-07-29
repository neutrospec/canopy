<!-- i18n-source: docs/upgrading.md sha:aab42b3d08286c5f46f679655bf4f212f0b8ad36 -->
> [한국어](../upgrading.md) · **English**

# Upgrade & Migration Guide

> A step-by-step guide for **moving canopy to a different install or run
> method**. There are three kinds of "version change" and this document covers
> only one of them — don't lump them together:
>
> | Kind | Who does it | Where |
> |---|---|---|
> | **Data-schema migration** | canopy, automatically (a new version's first run climbs it via `migrate.Ensure()`) | design [versioning.md](../versioning.md), checks [invariants.md](../invariants.md) J |
> | **Install / operational migration** | a person, manually (changing how it's installed or run) | **this document** |
> | Release changelog | GoReleaser, generated | GitHub Releases (from the commit convention) |
>
> Each item carries runnable check commands ([philosophy.md](../philosophy.md) principle 8).
> Items are stacked newest-first.

## First: how am I installed right now

```bash
which -a canopy                 # which binaries resolve on PATH (multiple = two installs)
brew list --versions canopy     # whether brew installed it, and the version
canopy version                  # app / data-schema / cache-schema versions
```

- Only `/opt/homebrew/bin/canopy` (Apple Silicon) or `/usr/local/bin/canopy` (Intel) → **single brew install** — the recommended state.
- If `~/.local/bin/canopy` also shows up, a source/`make install` copy remains — clean it up below.

> **Key fact — a binary's location is independent of XDG data.** canopy's
> config, model, cache, and state live under `$XDG_*_HOME` (or HOME-based
> defaults), so the same paths are used no matter where the binary is. Moving
> or deleting the binary leaves the data untouched — the checks below prove it.

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
# 1. Install via brew (if you haven't)
brew install neutrospec/tap/canopy

# 2. The model needs no migration — it's XDG data and is reused as-is. Just confirm:
canopy model status --json | jq .model_path
#   → ~/.local/share/canopy/models/... (the brew binary looks at the same path)

# 3. Remove the old binary
rm ~/.local/bin/canopy
hash -r                          # clear the shell's command-path cache (or open a new shell)

# 4. Confirm bare `canopy` now resolves to brew
which canopy                     # → /opt/homebrew/bin/canopy

# 5. (optional) Run the web UI as a persistent service
brew services start canopy       # localhost:8737, also registered to start at login
```

### Checks

```bash
# Only one install remains
which -a canopy | sort -u                 # a single brew path
test ! -e ~/.local/bin/canopy && echo ok  # the old copy is gone

# No data loss (identical before and after — binary location ≠ XDG)
canopy model status --json | jq .model_path
canopy status --json | jq .pages          # page count unchanged

# The service actually serves
brew services list | grep canopy          # started
curl -s -o /dev/null -w '%{http_code}\n' localhost:8737/   # 200
```

### Rolling back

Data and config live in XDG regardless, so either direction is lossless.

```bash
brew services stop canopy         # take the service down (if needed)
# To return to a source install, from the canopy repo:
make install                      # recreates ~/.local/bin/canopy
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
brew services restart canopy      # if you run the web UI as a service (assets are
                                  # embedded in the binary, so a restart is what
                                  # brings the new UI in)
```

For a release that needs a data-schema migration, the **first `canopy` run
after the upgrade climbs it automatically** (confirm with `canopy migrate
status`). There is nothing for you to do by hand — that's the boundary between
automatic migration and this document (manual migration).
