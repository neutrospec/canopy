<!-- i18n-source: docs/troubleshooting.md sha:30a87d56d6f59c8a377ea120a80746609b1ec5d5 -->
> [한국어](../troubleshooting.md) · **English**

# Troubleshooting

Common problems and the commands to diagnose them.

## `canopy: command not found`

`make install` installs to `~/.local/bin` first, falling back to
`/opt/homebrew/bin` if that directory does not exist. Check these cases.

1. **`~/.local/bin` is not on PATH** — adding it only to an interactive rc
   (.zshrc) means non-interactive shells like cron or agents won't find it.
   For zsh, put it in `~/.zshenv` so every shell picks it up:
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```
2. **Automation may not read your rc at all** — for cron jobs or agent
   prompts, an absolute path (`~/.local/bin/canopy`) is the most reliable.
Failure to find a wiki is separate from PATH. Specify the requested path and
check `root` in `canopy status --wiki "/path/to/wiki" --json`. Target precedence
is `--wiki` → `CANOPY_WIKI` → cwd ancestors → global `default_wiki`.
`init --wiki <path>` only fills an empty default. For automation with several
wikis, keep the same explicit `--wiki` instead of changing global settings.

Check `which canopy` and `canopy status --wiki <path> --json` in the same
environment as the automation. Its PATH, HOME, and XDG settings may differ from
an interactive shell.

## Semantic search finds nothing

`canopy model status` diagnoses it. For the built-in provider, check in order:

1. Did you download the model with `canopy model pull`?
2. Is this binary an ORT build? (`make build-lite` has no native inference.)
3. Is `libonnxruntime` installed? (`brew install onnxruntime`)
4. Were embeddings built with `canopy reindex`?

External providers need neither ORT nor a model download. Check the URL, token,
server model name, and dimension in `[embedding]` in the machine-local
`config.toml`. If `canopy reindex --check --json` reports `needs_reindex` above
zero, run `canopy reindex`. Update `revision` when replacing server weights under
the same model name. HTTP failures report the status code and do not switch to
the built-in model. [Setup instructions](getting-started.md#4-5-search).

To read without a model, find pages with `search --mode keyword` and inspect
their bodies with `show`. Hybrid search degrades to keyword, but `recall` fails
without embeddings. `embedding.model` in
`canopy.toml` must match the model directory name (under
`~/.local/share/canopy/models/`, default `bge-m3`).

## Web UI (serve)

- **Login required for external access** — this is by design. The localhost
  binding (default) is unauthenticated, but a binding reachable from outside
  like `--addr :8737` requires authentication. Create an account at `/setup`
  with the setup code printed once to the serve terminal.
- **Resetting the account** — delete `~/.config/canopy/webauth.json` and
  restart serve; it re-bootstraps with a fresh setup code.
- **HTTPS** — canopy does not terminate TLS itself. Put it behind tailscale
  (recommended) or a Caddy/nginx reverse proxy. If the proxy preserves the original
  Host, explicitly allow that authority, for example with
  `canopy serve --allow-host wiki.example`. Include a nonstandard port when it is
  forwarded, such as `wiki.example:8443`.
  `--allow-host` configures Host acceptance; it enables neither authentication nor TLS.
  When exposing a localhost backend through a public proxy, provide authentication
  at the proxy or configure a backend using canopy's authenticated external binding.
  Forwarded headers do not determine Host acceptance or authentication.

- **Port already in use** — identify the PID and command holding that port, then
  stop only the intended process you started, or use another port such as
  `--addr localhost:8738`.
- **The body is unchanged after saving** — a web save files an edit proposal.
  Check `Tasks` or `canopy tasks list --json --wiki <path>`. An agent must apply
  it; automatic execution and processing time are not guaranteed. `base` is
  the file hash at submission, not a saved copy of the body when the form opened
  or a 409 conflict check.
- **Search only works in keyword mode** — the serve startup log prints the
  embedding stack status. Same cause as "Semantic search" above.

## Graph, math, or diagrams don't render

- Graphs, math, and mermaid render in the browser via JS — with JS disabled
  you see body text only (reading, search, and editing still work without JS).
- All assets are embedded in the binary, so they're independent of any
  internet connection. If something looks off after an upgrade, hard-refresh
  the browser cache (⌘⇧R).
- Source raw HTML is disabled by default. Generated wikilinks, math, and Mermaid
  use supported rendering paths. In lint, `invalid-mermaid` means a syntax error;
  `mermaid-unchecked` means an environment problem prevented validation.

## Write failures and copy recovery

Keep body inputs until the actual content is verified, and back up an edited
working copy outside the wiki before checkin. Checkin reclaims the copy; do not
assume it survives a failure. A failed command may already have changed the page.
Compare the original or backup with current `show` output, stderr, and application
state; identify unfinished steps before recovery. Do not immediately repeat the
write. Exit 0 from `show` does not prove the content satisfies the request.
See [mutation-results.md](../mutation-results.md) for `applied`, `stage`, `repair`,
and `sync` fields and recovery of retained checkouts.

JSON mode and `show` have no status banner. Check the target, dirty files, open
copies, and pending tasks with `canopy status --json --wiki <path>`. Sync commits
and pushes all dirty changes, including existing ones; a failed push can leave
a local commit.

## Installed agent skills are stale

Run `canopy skills install` again. If you hand-edited an installed copy, the
next install overwrites it — the source of truth for skills is the binary
(`internal/skills/*.md`).
If installed in an arbitrary directory, pass the same `--dir <path>` again.
Installation does not register that path for automatic detection or guarantee
activation in the agent. Check the agent's loading rules.
