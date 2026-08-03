<!-- i18n-source: README.md sha:4098d744e0e2d0a49f86470e220571e717b2f02b -->
# 🌳 canopy

> [한국어](README.md) · **English**

**A local knowledge manager for markdown wikis.** Schema validation, search, a
web UI, and a rediscovery loop in a single Go binary. Designed so people and LLM
agents can tend the same wiki together, on the principle "judgment to the LLM,
invariants to code."

A wiki is a plain markdown + git repository. canopy sits beside it and enforces
schema validation, index regeneration, an activity log, embedding sync, and git
sync — in code — while coexisting with existing tools like Obsidian.

## Features

- **Schema-enforced writes** — every change via `new/update/mv/rm/archive` goes
  through type/tag taxonomy validation, and the index, log, and embeddings
  follow automatically. On a move, inbound wikilinks are rewritten for you.
- **Hybrid search** — fuses BM25 keyword search with semantic vectors (bge-m3,
  fully local ONNX inference). Works on mixed Korean/English documents, with no
  external API calls.
- **Web UI** (`canopy serve`) — search-first wiki browsing: instant search,
  backlinks, hover previews, dir×type×tag facet exploration, a local link graph,
  and body editing.
- **Second-brain loop** — rediscover forgotten pages (`resurface`), find
  similar-but-unlinked pages (`bridge`), recall evidence chunks (`recall`), and
  review a time window (`digest`).
- **Agent integration** — every command supports `--json`, and the agent skill
  docs are embedded in the binary, installable with one command.
- **A light footprint** — a single binary, XDG Base Directory compliant. Inside
  the wiki repo it leaves nothing but the schema file (`canopy.toml`).

## Install

**Homebrew** (recommended — builds from source and links onnxruntime automatically):

```bash
brew install neutrospec/tap/canopy
canopy model pull          # semantic-search model, bge-m3 ONNX ~2.3GB (once)
```

Before a tag exists, build main with `brew install --HEAD neutrospec/tap/canopy`.

**From source** (development):

```bash
brew install onnxruntime   # for semantic search (libonnxruntime)
make build                 # -tags ORT; stamps the version + auto-downloads libtokenizers.a once
make install               # ~/.local/bin (or /opt/homebrew/bin if absent)
```

To use it without embeddings: `make build-lite` — no cgo needed, keyword search
only (the `*_lite` archives on GitHub Releases are the same keyword-only binary).

## Quick start

```bash
# 1. Point at a wiki (so canopy finds it from any directory)
mkdir -p ~/.config/canopy
echo 'default_wiki = "/path/to/wiki"' > ~/.config/canopy/config.toml

# 2. Adopt the wiki — create the schema (canopy.toml) + index
canopy init

# 3. (optional) Prepare semantic search — download the model, then full index (once)
canopy model pull          # bge-m3 ONNX ~2.3GB
canopy reindex

# 4. Start using it
canopy new "First page" --type concept --tags tool
canopy search "anything"
canopy serve               # → http://localhost:8737
```

New here? The 15-minute tutorial in [docs/en/getting-started.md](docs/en/getting-started.md) is recommended.

## Web UI

`canopy serve` brings the wiki up as a search-first website.

- **Search is the entrance** — typing in the home search box gives instant
  results (↑↓/Enter keyboard navigation), and an exact title match jumps
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
- **Body editing** — a page's `✎ edit` lets you fix the body. Web editing goes
  through exactly the same pipeline as CLI `update` (validation, index, log,
  embeddings), and edit conflicts are caught by optimistic locking. Frontmatter
  and page create/move/delete are CLI-only. But web editing is meant for
  **small fixes like typos or a one-line correction** — serious writing and
  restructuring should go through the agent (the only path where content
  judgment rules also hold, [philosophy principle 9](docs/philosophy.md)).
- **Safe defaults** — it binds to localhost only by default. Opening it on an
  externally reachable address makes authentication mandatory, with an account
  (id/pw) created from a setup code printed once to the terminal. Dark mode and
  mobile supported. The server never commits — git changes are always an
  explicit `canopy sync`.

```bash
canopy serve                # default localhost:8737 (no auth)
canopy serve --addr :8737   # all interfaces — auth required (setup code on first run)
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
| `canopy status` · `canopy lint` | Wiki state · schema/link/freshness checks |

**Write** — index/log/embeddings refresh automatically afterward

| Command | Description |
|------|------|
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | Validated creation + related-page suggestions |
| `canopy update <page> [--body-file f]` | Bump updated (+replace body) |
| `canopy mv <page> [--type T] [--slug s]` | Move/rename — inbound links rewritten automatically |
| `canopy rm <page> [--force]` / `canopy archive <page>` | Delete (refused with backlinks) / archive |
| `canopy sync [-m msg]` | pull --rebase → commit → push → index refresh |

**Second brain** — candidates from canopy (deterministic), judgment/delivery by an agent or a person ([details](docs/second-brain.md))

| Command | Description |
|------|------|
| `canopy recall "question" [-k N] [--per-page N]` | Chunk-level evidence + source slugs (for injecting into agent context) |
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | Forgotten pages / stale-hub rediscovery candidates |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | Record a reaction → factored into later candidate selection |
| `canopy bridge [-n N] [--min-sim 0.7] [--include-linked]` | Find similar-but-unlinked page pairs |
| `canopy digest [--since 90d]` | Review material for a window: created/updated pages, tag distribution |

**Management**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install` · `canopy version` · `canopy migrate [status]` · `canopy reconcile`

Every command supports `--json`. `--peek` (resurface/bridge) previews without recording state.

## Agent integration

Two skills that teach an LLM agent how to use canopy (`canopy-wiki`: CLI usage
and content-judgment rules, `canopy-ingest`: an external-content ingestion
workflow) are embedded in the binary:

```bash
canopy skills install              # install/refresh into every detected agent (this one command after an upgrade)
canopy skills install --dir <path> # a specific directory only (created if absent — for a new agent's first registration)
```

- Auto-detected targets: **every existing** directory among `~/.hermes/skills`
  (hermes) and `~/.claude/skills` (Claude Code). A general agent gets a
  `<skill>/SKILL.md` flat layout; hermes gets a category layout
  (`note-taking/…`). For a new agent, install once with `--dir` and it joins
  auto-detection afterward.
- The source of truth for the installed copy is the binary — don't hand-edit an
  installed SKILL.md (the next install reverts it). Re-run after a canopy
  upgrade to pick up new commands.
- The core rule: **keep the agent from editing wiki files directly — always go
  through canopy commands.** Schema violations are blocked at the source.
- Recipes for scheduled runs (weekly journal, lint, etc.) are in [docs/second-brain.md](docs/second-brain.md).

## Data layout (XDG Base Directory compliant)

| Location | Nature |
|------|------|
| `<wiki>/canopy.toml` | The wiki's schema (type/tag taxonomy) — committed to the wiki since rules travel with the data |
| `<wiki>/_meta/resurface/` | Non-reproducible state (rediscovery surfacing history, feedback) — committed to the wiki |
| `<wiki>/_meta/webui/` | Non-reproducible state (read history, search gaps) — committed to the wiki, synced across devices |
| `<wiki>/_meta/attention/` | Non-reproducible state (agent access aggregate, day-quantized) — committed to the wiki |
| `~/.local/state/canopy/attention/<hash>.db` | Access-event detail (machine-local) — for rich UI like the timeline |
| `~/.cache/canopy/index/<hash>.db` | Derived cache (FTS + vectors) — rebuildable anytime with `reindex` |
| `~/.config/canopy/config.toml` | Global settings (`default_wiki`) |
| `~/.config/canopy/state.json` | Data-schema version (migration progress) — machine-local, survives cache wipes and upgrades |
| `~/.config/canopy/webauth.json` | Web UI account (bcrypt hash) — a secret, so outside the wiki, machine-local |
| `~/.local/share/canopy/models/` | ONNX model, static build libraries |

It respects `XDG_CONFIG_HOME`/`XDG_CACHE_HOME`/`XDG_DATA_HOME`. Inside the wiki
repo canopy leaves only the schema and `_meta/` state above, all of which have
a reason to "travel with the wiki." Secrets are never put in the wiki.

## Performance

On Apple Silicon with the int8-quantized model:

- Embedding ~11ms/chunk, model load ~0.5s (warm) — 2.4× faster than fp32 while
  keeping embedding quality at cosine similarity ≥0.988
- No-change `reindex` ≈ 1s, keyword search <100ms (no model load), web UI
  instant search ~40ms/query
- int8 quantization is optional: after `canopy model pull` (fp32), run
  `scripts/quantize-model.py` once (Python needed only for the conversion, not
  at runtime)

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
| [docs/web-ui-write-design.md](docs/web-ui-write-design.md) | Web write concurrency/conflict design (single write pipeline) |
| [docs/web-ui-i18n.md](docs/web-ui-i18n.md) | Web UI localization — go-i18n, locale negotiation, language selector (invariants M) |
| [docs/reconcile-design.md](docs/reconcile-design.md) | Canonicalization gate design — edit anywhere, canonicalize through one door (the main road re-judges every door's changes) |
| [docs/agent-tasks.md](docs/agent-tasks.md) | Agent task queue — web delegations (link/edit requests) as a file queue, closed only by per-type code verification (invariants T) |
| [docs/events.md](docs/events.md) | Event-log generalization — attention + lifecycle observation timeline in machine-local sqlite, non-authoritative & best-effort (invariants N) |
| [docs/web-ui-board.md](docs/web-ui-board.md) | Execution board — the full record of per-milestone tasks and exit criteria |

**Verify** — is it healthy now

| Doc | Contents |
|------|------|
| [docs/invariants.md](docs/invariants.md) | The list of checkable invariants (A–L) + audit procedure |

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
