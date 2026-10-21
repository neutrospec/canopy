<!-- i18n-source: docs/getting-started.md sha:975d85a08f5522914e91cd4ed8381ae925a2ee9f -->
> [한국어](../getting-started.md) · **English**

# Getting Started

> If "managing a wiki with an LLM" is new to you, start here.
> The order is concept → vocabulary → 15-minute walkthrough → daily use.
> First check page creation and keyword search without a model or Git, then add what you need.

## 1. What is this?

If you've used a notes app, you may know this experience too: notes you wrote
carefully but then **never look up again, that never connect to each other,
and that go stale over time** — a graveyard of notes.

The "LLM wiki" proposed by Andrej Karpathy is an answer to this. The idea is
simple:

- Accumulate knowledge as **plain markdown files** (no special app or DB)
- Let the **LLM (agent) do the tedious maintenance** — organizing, linking,
  keeping things consistent
- The human only **picks sources, asks questions, and judges**

The difference from asking a chatbot from scratch every time (RAG) is
"accumulation." In a wiki, knowledge synthesized once stays as a file, and as
new information arrives **existing pages get updated and connections grow.**
The more you ask and read, the richer the wiki becomes.

canopy is the tool that makes this actually work. Originally the rules —
"regenerate the index, write the log, don't break links" — were kept as long
prose instructions for the agent to follow, but agents forget, like people do.
canopy puts those rules **in code, inside the commands**. Validation differs by
command; people and agents still check factual accuracy and whether the request is satisfied.

## 2. Eight terms to know up front

| Term | Meaning |
|------|-----|
| **page** | One markdown file = one piece of knowledge. It lives in one of three folders: `concepts/`, `entities/` (people, products, orgs), `comparisons/` |
| **slug** | The filename (without extension). The slug of `opc-ua.md` is `opc-ua`. Lowercase ASCII letters, digits, and hyphens |
| **frontmatter** | The `---`-wrapped metadata at the top of a page (title, dates, tags). canopy creates it for you |
| **wikilink** | `[[another-page]]` in the body — a connection between pages. The more links, the more knowledge becomes a web |
| **backlink** | The reverse direction: "who references this page?" Check with `canopy backlinks <page>` |
| **semantic search** | Search by **meaning**, not words. It can find evidence with different wording, such as "vector cache" and KV-cache. Built-in inference code uses a separately installed local bge-m3 model; after preparation, no internet or external server is needed |
| **facet** | The web UI's way of classifying. Instead of a fixed folder tree, it combines folder × type × tag as **cross filters** to browse |
| **island** | A cluster of pages linked to each other but cut off from the wiki's mainland (largest connected component). It passes the orphan check, so `canopy lint` catches it separately |

## 3. What you need

- macOS (Apple Silicon recommended) + [Homebrew](https://brew.sh)
- The Go toolchain: `brew install go onnxruntime`
- (optional) [Obsidian](https://obsidian.md) — a nice viewer for the wiki. Just
  open the wiki folder as a vault
- (optional) an LLM agent — canopy works solo without one. The agent comes in §5

## 4. 15-minute walkthrough

### 4-1. Install

```bash
git clone https://github.com/neutrospec/canopy && cd canopy
make build && make install        # canopy → ~/.local/bin
which canopy                      # 안 나오면 troubleshooting.md의 PATH 항목 참조
```

### 4-2. Create a wiki

```bash
wiki="$HOME/canopy-demo"           # 아직 canopy.toml이 없는 새 폴더를 선택
mkdir -p "$wiki"
canopy init --wiki "$wiki"
canopy status --wiki "$wiki" --json # root가 이 경로인지 확인
```

A file called `canopy.toml` appears — it holds this wiki's rules (allowed
tags and page kinds). `init` only fills an empty global `default_wiki`; it keeps
existing settings. This guide pins every wiki command with `--wiki "$wiki"`.

### 4-3. Your first page

```bash
echo "OPC-UA는 산업 장비들이 서로 대화할 때 쓰는 표준 프로토콜이다." | \
  canopy new "OPC-UA란 무엇인가" --slug opc-ua --type concept --tags infrastructure --body-file - --wiki "$wiki"
canopy show opc-ua --wiki "$wiki"
```

```
✓ created concepts/opc-ua.md
NEXT: canopy sync   (commit & push this change, ...)
```

What canopy did with that one line: created the file + checked the rules
(rejecting an invalid tag) + regenerated the table of contents (index.md) +
recorded an activity log. These are all things a person/agent used to have to
remember.

> If a title is Korean-only, be sure to give an English filename with `--slug`.
> A title with no ASCII characters cannot produce an automatic filename.
> A successful `show` does not prove the requested body is correct; inspect the content.

### 4-4. Save in Git (optional)

The page is already a local file. Git history requires a repository and an author
identity. Replace the example name and email with your own. Local commits work without a remote.

```bash
git -C "$wiki" init -b main
git -C "$wiki" config user.name "Your Name"
git -C "$wiki" config user.email "you@example.com"
canopy sync -m "첫 페이지" --wiki "$wiki"
```

With an upstream, sync pulls, commits, and pushes; without one, it skips remote
steps. It includes **all dirty files in the wiki**, including existing changes.
For several devices, connect a remote and set the upstream on the first push.
On conflicts or failure, inspect and resolve the current Git state. Ordinary text
commands have a status banner; JSON and `show` do not, so check
`canopy status --json --wiki "$wiki"` directly.

### 4-5. Search

```bash
canopy search "OPC" --mode keyword --wiki "$wiki"   # 모델 없이 동작
canopy search "산업 장비 통신" --wiki "$wiki"        # hybrid, 미준비면 keyword 강등
```

Before your first semantic search, download the model with `canopy model pull --wiki "$wiki"`
(about 2.3GB), then create embeddings with `canopy reindex --wiki "$wiki"`.
For optional int8 conversion, check dependencies and paths with
`scripts/quantize-model.py --help` in the repository. To use its default output,
set `model` under `[embedding]` in the wiki's `canopy.toml` to `bge-m3-int8`, then
check `model status` and run `reindex` with the same `--wiki`. Lite builds support keyword search and external embedding APIs without native inference. `recall` requires embeddings; when unavailable, use keyword search and
show to check evidence.

To use an external service, add the following to `~/.config/canopy/config.toml`
(or `canopy/config.toml` under `XDG_CONFIG_HOME`), rather than the wiki file.

```toml
[embedding]
provider = "openai"
url = ""
token = ""
model = "embeddinggemma-2"
dimension = 768
input_format = "auto"
revision = ""
```

Set `url` to the complete OpenAI-compatible endpoint (for example,
`https://host/v1/embeddings`) and `token` to the Bearer token. Match `model` to the
name accepted by your server. For a BGE-M3 service, use `model = "bge-m3"` and
`dimension = 1024`. The external provider does not load the built-in model; it
sends documents and queries to the configured server. An empty URL fails setup,
and hybrid search falls back to keyword search. If your server already adds
EmbeddingGemma prefixes, use `input_format = "plain"`.

```bash
canopy model status --wiki "$wiki" --json
canopy reindex --check --wiki "$wiki" --json
canopy reindex --wiki "$wiki"
```

`--check` reports `needs_reindex` without calling the API and invalidates
incompatible cached vectors. Changes to the model, endpoint, dimension, input
format, or `revision` require reindexing; token rotation preserves vectors.
Change `revision` when replacing server weights under the same model name.
Restart a running `serve` process to apply settings.
Details: [Embedding provider design (Korean)](../embedding-providers.md).

### 4-6. View in the browser

```bash
canopy serve --wiki "$wiki"      # → http://localhost:8737
```

The wiki opens in your browser as a search-first website: type in the search
box for instant results, pages render with syntax highlighting, math, and
diagrams, and the `Graph` menu at the top lets you explore the whole web of
knowledge by zoom/drag. Read a page and press `✓ read` (shortcut `r`), and the
home "Discover" panel recommends pages you haven't read yet.

### 4-7. View in Obsidian (optional)

Obsidian → "Open folder as vault" → select `~/canopy-demo`, created in this tutorial.
If you used an existing wiki, select the folder that `$wiki` points to.
Wikilinks become clickable and the Graph View shows the web of knowledge.
canopy and Obsidian work on the same files. After editing in Obsidian, review
external changes with `canopy reconcile --wiki "$wiki"`, normalize the content
as needed, then accept it with `update` or `reconcile bless`
([reconciliation contract](../reconcile-design.md)). For partial edits you can
also borrow an external working copy with `checkout`/`checkin`.

## 5. Daily use

### Solo

```bash
canopy search "주제" --wiki "$wiki"   # 이미 있나? 뭐랑 연결되나?
canopy list --wiki "$wiki"            # --type, --tag로 필터
canopy checkout opc-ua --wiki "$wiki" # 반환된 사본에서 부분 편집
# 편집 사본을 별도 백업한 뒤 checkin, 현재 내용 확인
canopy checkin opc-ua --wiki "$wiki"
canopy lint --wiki "$wiki"            # 관련 구조·링크 finding 검토
canopy sync --wiki "$wiki"            # 전체 변경을 저장할 범위일 때
```

### With an agent (this is where the real fun starts)

Teaching an agent how to use canopy is one command:

```bash
canopy skills install    # 에이전트용 사용 설명서 2개를 스킬 폴더에 설치
```

Automatic detection covers supported agent paths. Specify an arbitrary path
with `canopy skills install --dir <path>` on every update, and check the agent's
skill activation and loading instructions after installation. The division of labor is:

| Human | Agent | canopy |
|------|----------|--------|
| Throws sources ("organize this article") | Reads, summarizes, writes the page | Checks rules, index, log, save |
| Asks questions | Uses recall or keyword search + show to find evidence and answer | Provides evidence and sources |
| Judges ("link it" / "no") | Executes | Records |

Create, update, move, and delete pages **through canopy commands**. Make partial
edits in a `checkout` copy and apply them with `checkin`. Add source material to
`raw/` and edit `canopy.toml` settings directly while preserving existing content.
Use command validation together with content checks.
A reading question ends with evidence and limitations; it does not automatically
create a page or start a sync.

### Reading in a browser

```bash
canopy serve --wiki "$wiki"               # localhost:8737 — 인증 없음
canopy serve --addr :8737 --wiki "$wiki"   # 외부 접근 — 인증 필수
```

A web UI that starts from the search box and moves to pages, backlinks, tags
(facets), and the full graph. Going to a missing page shows search results
instead, like Wikipedia. A page's "✎ edit" lets you submit a revised body.
Saving files the full body as an edit task and **leaves the current page unchanged**.
Check the proposal in `Tasks`, then check the actual page after an agent handles it.
canopy does not start an agent automatically or guarantee a processing time.
Frontmatter and page create/move/delete are CLI-only.

Web requests enter the agent's review path. The agent searches existing content
and considers duplicates, contradictions, and sources before applying changes.
See [agent-tasks.md](../agent-tasks.md) for queue handling and the limits of the
done check.

To view from a phone or another computer, open the second form — and then
**authentication is required automatically**: the first time you run serve, a
setup code prints to the terminal, and at the browser's `/setup` you create the
id/password you want along with that code. Using it with a private network like
tailscale is recommended (if something breaks, see [troubleshooting.md](troubleshooting.md)).

## 6. Let the wiki speak first

A wiki that only accumulates is a warehouse. canopy has commands that **revive
forgotten knowledge**:

```bash
canopy resurface --wiki "$wiki"         # 잊힌 페이지 재발견
canopy bridge --wiki "$wiki"            # 유사하지만 연결 안 된 페이지
canopy digest --since 30d --wiki "$wiki" # 회고 소재
```

This loop works through two touchpoints:

- **The web home** — the home screen of `canopy serve` shows a daily "today's
  rediscovery" card (one forgotten page + 👍/👎/😴 buttons) and a "suggested
  connection" card. Button reactions write to the same state file as the CLI, so
  reacting from either side means no duplicate surfacing.
- **The agent** — ask it to run the commands above weekly and send a summary
  (e.g. a Telegram morning briefing), and the wiki comes to you and speaks.

Searches that found no answer aren't discarded either — they pile up in the web
UI's `Checks → search gaps`, becoming candidates for "knowledge not in the wiki
= the next page to create." For the full design and cron recipes, see
[second-brain.md](../second-brain.md).

## 7. Frequently asked questions

**Q. Semantic search finds nothing.**
Check that you ran `canopy model pull` and that embeddings were built with
`canopy reindex`. `canopy model status` diagnoses it. In a pinch, `--mode
keyword` always works.

**Q. I'm trying to delete a page but it's refused.**
Another page references it (deleting would break the link). It shows you the
list, so use `canopy archive`, or `--force` if you really mean to delete (the
references become plain text).

**Q. I want to use a new tag but it's refused.**
That's intended — if tags grow arbitrarily, classification becomes meaningless.
First check existing tags with `canopy tags --wiki "$wiki"`. Add a new topic to
`topics` in `canopy.toml` when at least three pages need it, and retag existing pages.
`forms` is a closed set; legacy `tags` is for compatibility. A new init's suggested
list is a bootstrap, so you need not use every tag immediately. Use `tags --audit`
to refine it as actual demand develops.

**Q. Can I use it from several computers?**
Yes, that's why it uses git. Use `canopy sync` on each machine. It pulls first,
but conflicts can still happen. On failure, preserve both versions and inspect
and resolve the Git state before continuing.

**Q. I think my wiki is broken.**
`canopy lint` tells you what's wrong. If the search index is off, rebuild with
`canopy reindex --wiki "$wiki"`. Search databases are derived caches; `logs/`
and `_meta/` contain history and state that cannot be reconstructed and must be
preserved. **Keep pages, raw sources, history, and state together in Git.**

## 8. What to read next

- [proxy-auth.md](../proxy-auth.md) — private Gateway authentication and persistent Homebrew service settings (Korean)

- [troubleshooting.md](troubleshooting.md) — when something won't work (PATH, semantic search, web auth)
- [philosophy.md](../philosophy.md) — why it's designed this way (each principle includes how to verify it)
- [invariants.md](../invariants.md) — a checklist to verify the system's health yourself
- [second-brain.md](../second-brain.md) — the rediscovery loop's design and agent-operation recipes
- Curious why the web UI looks the way it does? Follow the README's "design record" doc map
