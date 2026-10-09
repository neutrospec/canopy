<!-- i18n-source: README.md sha:771c7fe77683c10cdb895c8e33a103084d4cc9e1 -->
# 🌳 canopy

> [한국어](README.md) · **English**

**A local knowledge manager for markdown wikis.** Schema validation, search, a
web UI, and a rediscovery loop in a single Go binary. Designed so people and LLM
agents can tend the same wiki together, on the principle "judgment to the LLM,
invariants to code."

A wiki is a plain markdown + git repository. canopy handles command-specific
validation, index regeneration, an activity log, embedding sync, and git sync
while coexisting with existing tools like Obsidian.

## Features

- **Validated writes with follow-up processing** — `new` checks type, tags,
  slug, and `--links`; `checkin` checks the edited copy's schema and base.
  `update` preserves existing frontmatter. Writes refresh the index, log, and
  embeddings; moves rewrite inbound links. Use `lint` to check body links and
  structural problems in external edits.
- **Hybrid search** — fuses BM25 keyword search with semantic vectors (bge-m3,
  fully local ONNX inference). Works on mixed Korean/English documents, with no
  external API calls.
- **Web UI** (`canopy serve`) — search-first wiki browsing: instant search,
  backlinks, hover previews, dir×type×tag facet exploration, a local link graph,
  and body-edit proposals.
- **Second-brain loop** — rediscover forgotten pages (`resurface`), find
  similar-but-unlinked pages (`bridge`), recall evidence chunks (`recall`), and
  review a time window (`digest`).
- **Agent integration** — `--json` for machine output, with agent skill
  docs embedded in the binary and installable with one command.
- **A light footprint** — a single binary, XDG Base Directory compliant. The wiki
  holds its schema, generated markdown indexes, activity history, and shared
  state; search databases and models live outside it.

## Install

**Homebrew** (recommended — builds from source and links onnxruntime automatically):

```bash
brew install neutrospec/tap/canopy
canopy model pull          # 시맨틱 검색 모델 bge-m3 ONNX ~2.3GB (1회)
```

Before a tag exists, build main with `brew install --HEAD neutrospec/tap/canopy`.

**From source** (development):

```bash
brew install onnxruntime   # 시맨틱 검색용 (libonnxruntime)
make build                 # -tags ORT; 버전 각인 + 최초 1회 libtokenizers.a 자동 다운로드
make install               # ~/.local/bin (없으면 /opt/homebrew/bin)
```

To use it without embeddings: `make build-lite` — no cgo needed, keyword search
only (the `*_lite` archives on GitHub Releases are the same keyword-only binary).

## Quick start

```bash
# 새 폴더를 지정한다. 모델·Git·서비스 없이 첫 페이지와 검색을 확인할 수 있다.
wiki="$HOME/canopy-demo"
mkdir -p "$wiki"
canopy init --wiki "$wiki"
canopy status --wiki "$wiki" --json   # root가 지정한 위키인지 확인
canopy new "첫 페이지" --slug first-page --type concept --tags tool --wiki "$wiki"
canopy search "첫 페이지" --mode keyword --wiki "$wiki"
canopy show first-page --wiki "$wiki"
```

`init` fills an empty `default_wiki` and preserves existing global settings.
Semantic search is optional: with an ORT build, run `canopy model pull`, then
`canopy reindex --wiki "$wiki"`. Open the web UI with `canopy serve --wiki "$wiki"`.
Git history and a remote are separate steps in the getting-started guide.

New here? The 15-minute tutorial in [docs/en/getting-started.md](docs/en/getting-started.md) is recommended.

## Web UI

`canopy serve` brings the wiki up as a search-first website.

- **Search is the entrance** — typing in the home search box gives instant
  results (↑↓/Enter keyboard navigation), and an exact slug match jumps
  straight to that page. Results show the **actual matching paragraph**, not
  just the page title.
- **A modern markdown viewer** — code-block syntax highlighting (server-side,
  light/dark theme), **mermaid diagrams**, LaTeX math ($…$/$$…$$, MathJax SVG),
  copy buttons, heading anchors, and footnotes. Every asset is embedded in the
  binary so it works offline, loaded only on pages that need it.
- **Read it like a wiki** — wikilinks are clickable (missing pages show as red
  links), hover pops a preview of the target page, and the page footer has
  backlinks ("pages that link here") and a collapsible local link graph.
  Reaching a missing page shows search results instead of a 404.
- **An interactive knowledge graph** — explore the whole wiki's connections on
  one screen, like Obsidian: force layout, zoom/pan/drag, neighbor highlighting
  on hover, node finding, click to open a page. Unread pages look different, so
  you can see where to read.
- **Classification by facet, not tree** — the `Browse` menu cross-filters by
  directory × type × tag. Recent changes / orphan · stale pages / a random page
  are also offered.
- **Read history and discovery** — the `✓ read` button (shortcut `r`) or a
  conservative dwell+scroll auto-detection records reads (undoable), and unread
  pages are ranked and recommended by newness, hubness, and similarity to recent
  interest (`Discover` menu, with read progress). History is stored in the wiki
  repo (`_meta/webui/`) and syncs across devices.
- **The wiki speaks first** — the home shows a "today's rediscovery" card
  (a forgotten page + 👍/👎/😴 feedback, sharing state with CLI resurface) and a
  "suggested connection" card (similar but unlinked pairs), and each page footer
  shows still-unlinked similar pages as **suggested links**. Searches that found
  no answer accumulate as a gap log, becoming page-creation candidates
  (Checks → search gaps).
- **Body-edit proposals** — saving in a page's `✎ edit` files the full body as an
  edit task. **Acceptance into the queue does not apply the edit to the page.**
  Check its state in `Tasks`, then check the current page after an agent reviews
  and applies it. canopy does not start the agent automatically. Frontmatter and
  page create/move/delete are handled through the CLI
  ([task contract](docs/agent-tasks.md)).
- **Safe defaults** — it binds to localhost only by default. Opening it on an
  externally reachable address makes authentication mandatory, with an account
  (id/pw) created from a setup code printed once to the terminal. Dark mode and
  mobile supported. The server never commits — git changes are always an
  explicit `canopy sync`.

```bash
canopy serve                # 기본 localhost:8737 (무인증)
canopy serve --addr :8737   # 모든 인터페이스 — 인증 필수 (최초 실행 시 설정 코드 안내)
```

## Command reference

**Read**

| Command | Description |
|------|------|
| `canopy search "query" [--mode hybrid\|keyword\|semantic] [-k N]` | Hybrid search (default). Degrades to keyword when the embedding stack is missing |
| `canopy serve [--addr :8737]` | Web UI (see above) |
| `canopy backlinks <page>` / `--orphans` | Backlinks / orphan pages |
| `canopy list [--type T] [--tag t]` | List all pages (slug·type·title) |
| `canopy tags` | View the valid type/tag taxonomy (same source as `new` validation) |
| `canopy show <page>` (alias `view`) | Print a page (path header on stderr, body on stdout) |
| `canopy grep "패턴" [page]` | Regular-expression search with line numbers |
| `canopy status` · `canopy lint` | Wiki state · schema/link/freshness checks |

**Write** — index/log/embeddings refresh automatically afterward

| Command | Description |
|------|------|
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | Validated creation + related-page suggestions |
| `canopy update <page> [--body-file f]` | Bump updated (+replace body) |
| `canopy checkout <page>` / `canopy checkin <page>` | Edit an external working copy → validate, apply, reclaim the copy |
| `canopy mv <page> [--type T] [--slug s]` | Move/rename — inbound links rewritten automatically |
| `canopy rm <page> [--force]` / `canopy archive <page>` | Delete (refused with backlinks) / archive |
| `canopy sync [-m msg]` | pull --rebase → regenerate Markdown indexes → commit → push → refresh search cache |

**Second brain** — canopy filters and scores candidates; an agent or a person judges and delivers them. `resurface auto/random` samples eligible candidates. ([details](docs/second-brain.md))

| Command | Description |
|------|------|
| `canopy recall "question" [-k N] [--per-page N]` | Raw evidence chunks + source slugs. Requires embeddings; otherwise use keyword search + show |
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | Forgotten pages / stale-hub rediscovery candidates |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | Record a reaction → factored into later candidate selection |
| `canopy bridge [-n N] [--min-sim 0.7] [--include-linked]` | Find similar-but-unlinked page pairs |
| `canopy digest [--since 90d]` | Review material for a window: created/updated pages, tag distribution |

**Management**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install` · `canopy version` · `canopy migrate [status]` · `canopy reconcile`

Start with `canopy tasks list --json` for the queue, `canopy reconcile --json`
for external edits, and `canopy events --json` for machine activity.
Automation should check each command's `--json` output, stderr, and exit status
together. Argument parsing or initialization can fail without JSON output.
JSON mode and `show` have no status banner; use `canopy status --json` to check
the target, dirty files, open copies, and pending tasks.
See [mutation-results.md](docs/mutation-results.md) for the six page mutation
commands' responses, partial application, and recovery. Exit 0 means the requested
steps completed; it does not verify content accuracy.
`--peek` (resurface/bridge) previews without recording exposure state.

## Agent integration

Two skills that teach an LLM agent how to use canopy (`canopy-wiki`: CLI usage
and content-judgment rules, `canopy-ingest`: an external-content ingestion
workflow) are embedded in the binary:

```bash
canopy skills install              # 감지된 모든 에이전트에 설치/갱신 (업그레이드 후 이 한 명령)
canopy skills install --dir <path> # 지정한 스킬 디렉토리만 설치/갱신 (없으면 생성)
```

- Auto-detected targets: existing `~/.hermes/skills` (hermes), `~/.claude/skills`
  (Claude Code), and pi's agent home (`PI_CODING_AGENT_DIR`, default `~/.pi/agent`).
  pi's `skills/` is created if its home exists. A general agent gets a
  `<skill>/SKILL.md` flat layout; hermes gets a category layout
  (`note-taking/…`). `--dir` does not register an arbitrary path for automatic
  detection; specify that path again when updating. Installation and activation
  are separate: check the agent's skill discovery and loading rules.
- The source of truth for the installed copy is the binary — don't hand-edit an
  installed SKILL.md (the next install reverts it). Re-run after a canopy
  upgrade to pick up new commands.
- Create, update, move, and delete pages **through canopy commands**. Make partial
  edits in a `checkout` copy and apply them with `checkin`. Add source material to
  `raw/` and edit `canopy.toml` settings directly while preserving existing content.
  Use command-specific validation and follow-up processing, then check the content
  and relevant lint findings.
- Recipes for scheduled runs (weekly journal, lint, etc.) are in [docs/second-brain.md](docs/second-brain.md).

## Data layout (XDG Base Directory compliant)

| Location | Nature |
|------|------|
| `<wiki>/canopy.toml` | The wiki's schema (type/tag taxonomy) — committed to the wiki since rules travel with the data |
| `<wiki>/index.md`, `<wiki>/index/` | Fully regenerated markdown indexes — do not edit manually |
| `<wiki>/logs/*.jsonl` | Append-only activity history — preserve it; current pages cannot reconstruct it |
| `<wiki>/_meta/tasks/`, `<wiki>/_meta/reconcile/` | Delegated queue and review baseline — committed with the wiki |
| `<wiki>/_meta/resurface/` | Non-reproducible state (rediscovery surfacing history, feedback) — committed to the wiki |
| `<wiki>/_meta/webui/` | Non-reproducible state (read history, search gaps) — committed to the wiki, synced across devices |
| `<wiki>/_meta/attention/` | Non-reproducible state (agent access aggregate, day-quantized) — committed to the wiki |
| `~/.local/state/canopy/attention/<hash>.db` | Access-event detail (machine-local) — for rich UI like the timeline |
| `~/.cache/canopy/index/<hash>.db` | Derived cache (FTS + vectors) — rebuildable anytime with `reindex` |
| `~/.config/canopy/config.toml` | Global settings (`default_wiki`) |
| `~/.config/canopy/state.json` | Data-schema version (migration progress) — machine-local, survives cache wipes and upgrades |
| `~/.config/canopy/webauth.json` | Web UI account (bcrypt hash) — a secret, so outside the wiki, machine-local |
| `~/.local/share/canopy/models/` | ONNX model, static build libraries |

It respects `XDG_CONFIG_HOME`/`XDG_CACHE_HOME`/`XDG_DATA_HOME`/`XDG_STATE_HOME`.
Deleting caches does not restore activity history or `_meta/` state. Machine-local
non-reproducible state evolves through the migration ladder, traveling `_meta/*`
through each file's version, and derived search databases through a cache version
and rebuild. Secrets are never put in the wiki.

## Performance

Earlier measurements on Apple Silicon with the int8-quantized model follow. They do not certify this release's overall performance or quality.

- Embedding ~11ms/chunk, model load ~0.5s (warm) — 2.4× faster than fp32 while
  keeping embedding quality at cosine similarity ≥0.988
- No-change `reindex` ≈ 1s, keyword search <100ms (no model load), web UI
  instant search ~40ms/query
- int8 quantization is optional. After `canopy model pull` (fp32), check conversion
  dependencies and paths with `scripts/quantize-model.py --help` in the repository,
  then run it. The default output is a separate `bge-m3-int8` directory. In the target
  wiki's `canopy.toml`, set `model` under `[embedding]` to `bge-m3-int8`, check
  `canopy model status` with the same `--wiki`, then run `canopy reindex`.

## Documentation

Read from top (concept) to bottom (verification) to trace the reason for every
implementation without reading code.

**Understand** — why it's built this way

| Doc | Contents |
|------|------|
| [docs/philosophy.md](docs/philosophy.md) | Design philosophy — for each principle, the enforcer ([code]/[detect]/[convention]) and a check command |
| [docs/second-brain.md](docs/second-brain.md) | Design and operation of the rediscovery loop (resurface/bridge/recall/digest) |

**Use** — how to use it

| Doc | Contents |
|------|------|
| [docs/en/getting-started.md](docs/en/getting-started.md) | For newcomers — concepts, vocabulary, a 15-minute tutorial, daily use |
| [docs/en/upgrading.md](docs/en/upgrading.md) | Install/operation migration guide (source/make install → Homebrew + brew services, routine upgrades) |
| [docs/en/troubleshooting.md](docs/en/troubleshooting.md) | Common problems (PATH, semantic search, web UI auth) |
| [docs/mutation-results.md](docs/mutation-results.md) | Mutation JSON, partial application, checkout recovery, and cooperative writes |

**Design record** — what decisions each feature was born from

| Doc | Contents |
|------|------|
| [docs/versioning.md](docs/versioning.md) | Three version numbers · migration ladder · release procedure · Homebrew distribution |
| [docs/i18n.md](docs/i18n.md) | Multilingual docs management — source/translation, staleness check (invariants L) |
| [docs/homebrew-guide.md](docs/homebrew-guide.md) | Homebrew distribution/release hands-on guide (for first-timers) |
| [docs/web-ui-plan.md](docs/web-ui-plan.md) | Round 1 (M1–M4): search-first viewer, facets, web editing |
| [docs/web-ui-plan-2.md](docs/web-ui-plan-2.md) | Round 2 (M5–M8): security, read history · discovery, suggested links, speaking home |
| [docs/web-ui-plan-3.md](docs/web-ui-plan-3.md) | Round 3 (M9–M10+): modern viewer, knowledge graph, island detection |
| [docs/web-ui-plan-4.md](docs/web-ui-plan-4.md) | Round 4 (M11–M13): two-tier attention storage, access tracking, instruments · visual refresh |
| [docs/web-ui-write-design.md](docs/web-ui-write-design.md) | Historical direct-save design and explicit sync decision — see agent-tasks for the current proposal contract |
| [docs/web-ui-i18n.md](docs/web-ui-i18n.md) | Web UI localization — go-i18n, locale negotiation, language selector (invariants M) |
| [docs/reconcile-design.md](docs/reconcile-design.md) | Canonicalization gate design — edit anywhere, canonicalize through one door (the main road re-judges every door's changes) |
| [docs/agent-tasks.md](docs/agent-tasks.md) | Agent task queue — web delegations (link/edit requests) as a file queue, closed only by per-type code verification (invariants T) |
| [docs/events.md](docs/events.md) | Event-log generalization — attention + lifecycle observation timeline in machine-local sqlite, non-authoritative & best-effort (invariants N) |
| [docs/web-ui-board.md](docs/web-ui-board.md) | Execution board — the full record of per-milestone tasks and exit criteria |

**Verify** — is it healthy now

| Doc | Contents |
|------|------|
| [docs/invariants.md](docs/invariants.md) | Checkable invariants, execution prerequisites, and audit procedure |

One contribution rule: **a claim without a check command is not a principle.** A
new feature records its invariant and check method in invariants.md first, then
gets implemented. Development rules (build, version, migration, commit
conventions) are collected in [AGENTS.md](AGENTS.md).

## On how this is developed

Most of this project's code was written with **AI-assisted coding** (Claude
Code). Direction and judgment from a human, implementation and verification from
the AI — the same setup as the "judgment to the LLM, invariants to code"
principle this tool applies to the wiki. What decisions each feature was born
from can be traced through the "design record" docs above. Since this disclosure
is the attribution, later commit messages don't carry an AI co-author trailer
(some early commits do — history isn't rewritten).

## License

[MIT](LICENSE)
