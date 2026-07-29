<!-- i18n-source: docs/troubleshooting.md sha:aa97e58d36a7c4e0c260df4d3c153cc5e3f965bd -->
> [한국어](../troubleshooting.md) · **English**

# Troubleshooting

Common problems and the commands to diagnose them.

## `canopy: command not found`

`make install` installs to `~/.local/bin` first, falling back to
`/opt/homebrew/bin` if that directory does not exist. Check these three cases.

1. **`~/.local/bin` is not on PATH** — adding it only to an interactive rc
   (.zshrc) means non-interactive shells like cron or agents won't find it.
   For zsh, put it in `~/.zshenv` so every shell picks it up:
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```
2. **Automation may not read your rc at all** — for cron jobs or agent
   prompts, an absolute path (`~/.local/bin/canopy`) is the most reliable.
3. **Running outside the wiki** — canopy walks up from the cwd looking for
   `canopy.toml`. In an environment that runs from an arbitrary directory
   (like cron) that search fails, so set `default_wiki` in
   `~/.config/canopy/config.toml`.

Check: `which canopy && cd /tmp && canopy status` — both succeeding means
you're ready for automation.

## Semantic search finds nothing

`canopy model status` diagnoses it. Check in order:

1. Did you download the model with `canopy model pull`?
2. Is this binary an ORT build? (A `make build-lite` build is keyword-only.)
3. Is `libonnxruntime` installed? (`brew install onnxruntime`)
4. Were embeddings built with `canopy reindex`?

When you're in a hurry, `--mode keyword` always works. `embedding.model` in
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
  (recommended) or a Caddy/nginx reverse proxy.

- **Port already in use** — a previous serve process may still be running:
  `pkill -f "canopy serve"`, then restart.
- **409 (conflict) when saving an edit** — the page was modified elsewhere
  (CLI, agent, another tab) after you opened the form. Reconcile the edit
  still in your browser against the current version, then save again.
- **Search only works in keyword mode** — the serve startup log prints the
  embedding stack status. Same cause as "Semantic search" above.

## Graph, math, or diagrams don't render

- Graphs, math, and mermaid render in the browser via JS — with JS disabled
  you see body text only (reading, search, and editing still work without JS).
- All assets are embedded in the binary, so they're independent of any
  internet connection. If something looks off after an upgrade, hard-refresh
  the browser cache (⌘⇧R).

## Installed agent skills are stale

Run `canopy skills install` again. If you hand-edited an installed copy, the
next install overwrites it — the source of truth for skills is the binary
(`internal/skills/*.md`).
