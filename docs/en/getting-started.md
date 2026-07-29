<!-- i18n-source: docs/getting-started.md sha:bbfbdf4d28cac06061dc725514aa15b8d13936c6 -->
> [한국어](../getting-started.md) · **English**

# Getting Started

> If "managing a wiki with an LLM" is new to you, start here.
> The order is concept → vocabulary → 15-minute walkthrough → daily use.
> Every command here was actually run in an empty folder and verified.

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
canopy puts those rules **in code, inside the commands**, so forgetting becomes
impossible.

## 2. Eight terms to know up front

| Term | Meaning |
|------|-----|
| **page** | One markdown file = one piece of knowledge. It lives in one of three folders: `concepts/`, `entities/` (people, products, orgs), `comparisons/` |
| **slug** | The filename (without extension). The slug of `opc-ua.md` is `opc-ua`. Lowercase ASCII letters and hyphens only |
| **frontmatter** | The `---`-wrapped metadata at the top of a page (title, dates, tags). canopy creates it for you |
| **wikilink** | `[[another-page]]` in the body — a connection between pages. The more links, the more knowledge becomes a web |
| **backlink** | The reverse direction: "who references this page?" Check with `canopy backlinks <page>` |
| **semantic search** | Search by **meaning**, not words. Searching "vector cache" surfaces the KV-cache page even without that exact term. The AI model embedded in canopy (bge-m3) handles it, needing no internet or external server |
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
which canopy                      # nothing? see the README's "PATH setup"
```

### 4-2. Create a wiki

```bash
mkdir ~/wiki && cd ~/wiki && git init -b main    # the wiki is a git repo (history = the record of knowledge growing)
mkdir -p ~/.config/canopy
echo 'default_wiki = "'$HOME'/wiki"' > ~/.config/canopy/config.toml   # so canopy finds the wiki from anywhere
canopy init
```

A file called `canopy.toml` appears — it holds this wiki's rules (allowed
tags, page kinds). To add tags later, edit it there.

### 4-3. Your first page

```bash
echo "OPC-UA is a standard protocol industrial devices use to talk to each other." | \
  canopy new "What is OPC-UA" --slug opc-ua --type concept --tags infrastructure --body-file -
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
> The title can be Korean, but the filename must be English for search and sync
> to be safe.

### 4-4. Save (sync)

```bash
canopy sync -m "first page"
```

sync = git pull → commit → push in one step. It's fine if you forget —
**every canopy command shows a ⚠ warning when there are unsaved changes.**

### 4-5. Search

```bash
canopy search "industrial device communication"   # by meaning — finds it without the exact words
canopy search "OPC" --mode keyword                 # by word — instant, no model load
```

Once, before your first semantic search: `canopy model pull` (downloads the
2.3GB AI model, then works fully offline; for the int8 shrink see
`scripts/quantize-model.py`).

### 4-6. View in the browser

```bash
canopy serve      # → http://localhost:8737
```

The wiki opens in your browser as a search-first website: type in the search
box for instant results, pages render with syntax highlighting, math, and
diagrams, and the `Graph` menu at the top lets you explore the whole web of
knowledge by zoom/drag. Read a page and press `✓ read` (shortcut `r`), and the
home "Discover" panel recommends pages you haven't read yet.

### 4-7. View in Obsidian (optional)

Obsidian → "Open folder as vault" → pick `~/wiki`. That's it.
Wikilinks become clickable and the Graph View shows the web of knowledge.
canopy and Obsidian work on the same files, so you can edit from either — but
if you edited in Obsidian, tell canopy with `canopy update <page>` (it refreshes
the modified date and the search index).

## 5. Daily use

### Solo

```bash
canopy search "topic"          # always, before writing anything: does it exist? what connects?
canopy list                    # skim all pages (filter with --type, --tag)
canopy new ... --links related-page   # add while linking to existing pages
canopy sync                    # end of day, or per unit of work
canopy lint                    # occasionally: check broken links and orphans
```

### With an agent (this is where the real fun starts)

Teaching an agent how to use canopy is one command:

```bash
canopy skills install    # install the two agent how-to guides into the skills folder
```

After that the division of labor is:

| Human | Agent | canopy |
|------|----------|--------|
| Throws sources ("organize this article") | Reads, summarizes, writes the page | Checks rules, index, log, save |
| Asks questions | Finds evidence in the wiki with `canopy recall` and answers | Provides evidence chunks with sources |
| Judges ("link it" / "no") | Executes | Records |

Remember one core rule: **don't let the agent edit the wiki directly — always
go through canopy commands.** That's what blocks rule violations at the source.

### Reading in a browser

```bash
canopy serve               # localhost:8737 — this machine only, no auth
canopy serve --addr :8737  # reachable from any device — auth required (see below)
```

A web UI that starts from the search box and moves to pages, backlinks, tags
(facets), and the full graph. Going to a missing page shows search results
instead, like Wikipedia, and a page's "✎ edit" lets you fix the body — web
editing goes through exactly the same pipeline as CLI `update` (index, log,
embeddings), so it's safe to use alongside the CLI and agents. Frontmatter and
page create/move/delete are CLI-only.

One principle: use web editing (and direct Obsidian edits) only for **small
fixes like typos or a one-line correction.** Serious writing and restructuring
belongs to the agent — content rules like "search before writing, update
instead of duplicating, record contradictions side by side" live in the agent's
skills, so a person writing directly walks right past those checkpoints.

To view from a phone or another computer, open the second form — and then
**authentication is required automatically**: the first time you run serve, a
setup code prints to the terminal, and at the browser's `/setup` you create the
id/password you want along with that code. Using it with a private network like
tailscale is recommended (if something breaks, see [troubleshooting.md](troubleshooting.md)).

## 6. Let the wiki speak first

A wiki that only accumulates is a warehouse. canopy has commands that **revive
forgotten knowledge**:

```bash
canopy resurface     # "you haven't seen this page in 59 days" — rediscover forgotten pages
canopy bridge        # "these two pages are similar but not linked" — find hidden relations
canopy digest --since 30d   # "what did I learn this month" — retrospective material
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
If you genuinely need it, add it to the `tags` list in `canopy.toml` first.

**Q. Can I use it from several computers?**
Yes, that's why it uses git. Just `canopy sync` well on each machine; since sync
pulls first, conflicts are rare.

**Q. I think my wiki is broken.**
`canopy lint` tells you what's wrong. If the search index is off,
`rm -rf ~/.cache/canopy && canopy reindex` — the index is a cache you can
always rebuild, so deleting it is safe. **All the real knowledge is in the
markdown files and git.**

## 8. What to read next

- [troubleshooting.md](troubleshooting.md) — when something won't work (PATH, semantic search, web auth)
- [philosophy.md](../philosophy.md) — why it's designed this way (each principle includes how to verify it)
- [invariants.md](../invariants.md) — a checklist to verify the system's health yourself
- [second-brain.md](../second-brain.md) — the rediscovery loop's design and agent-operation recipes
- Curious why the web UI looks the way it does? Follow the README's "design record" doc map
