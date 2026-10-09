# PLAN-13 and bounded PLAN-14 implementation

Dispatch `task_b1b965146412` / `ctx_961220566011`, 2026-10-10. The selected
implementation is ready for independent review. The historical audit was not
edited, and no user wiki, commit, deployment, or publication was made.

## Result and scope

The web workspace now distinguishes receiving a proposal from applying it. Page
receipts and task rows show the real ID, target, request, proposed body, status,
full timestamps, and the meaning of `base`: the file hash at POST reception.
They do not promise an editor-open snapshot. Empty body plus empty request is
400; a request alone is accepted. Proposals preserve the submitted content
(with the existing CRLF-to-LF normalization) and leave page files unchanged.

Automatic read completion requires a successful, non-redirected JSON response
with boolean `read: true`. Login HTML, bad JSON, wrong content type, `read: false`,
missing fields, and request failure preserve the native controls and show a
failure. Home unread totals, recommendation candidates, and queued work are
independent. No recommendation is no longer reported as all-read. HEAD does not
consume the daily recommendation or its persisted exposure history.

Read-history failures retain the healthy aggregate, expose unavailable/partial
information, and refuse aggregate writes without resetting files. Graph read
state is nullable when unknown; touch counts describe the available aggregates.
New unknown read/feedback targets receive 4xx, while historical keys and undo
remain usable. User references resolve through `Scan.Find`; explicit governed
paths distinguish `foo.md` from the legal slug of `foo.md.md`, including forms,
rendered wikilinks, search results, graph links, and task targets.

Cached search, page suggestions, home/discovery bridges, and attention vectors
now pass through `indexer.EnsureCurrentContext`. Cold API searches and exact-slug
redirects also refresh evidence. API mode and semantic coverage describe the
actual result; incomplete vectors do not supply semantic ranking. Queued work
has a five-second context budget and active index/search work a thirty-second
context budget. Queued cancellation is checked before starting work, and query
embedding uses `embed.EmbedContext`. These are cooperative bounds: inference
that has already entered native code can finish after cancellation.

The layout keeps page actions near the body, with a contextual outline,
related pages, backlinks, and task proposals alongside it on wide screens and
native expandable panels on narrow screens. Graph has a server-rendered page
and connection list, a native search fallback, and a link for the selected page.
Code palettes were adjusted for both themes; code and tables scroll within
their own regions. Mermaid and math failures are visible and retain source.
These changes do not establish a usability or accessibility improvement by
themselves; actual browser results are listed separately below.

App CSS/JS URLs contain the embedded bytes' SHA-256 prefix. Static responses use
full-byte ETags and `Cache-Control: no-cache`; HTML also requires revalidation.
A refreshed page therefore requests the new application assets after a server
upgrade. Already-open tabs still require refresh.

The current contract is [docs/web-contract.md](../../../web-contract.md). This
worker changed the transferred web handlers, templates, CSS/JS/locales and
tests, added the asset checker/notices/manifest and this report. Root-owned
HTTP boundaries, auth, Markdown renderer/parser, CLI, core search/state, and
release/Homebrew files were integrated without editing them. Root's V12–V14
and U invariants cover the selected contracts; no new invariant section was
written by this worker.

## Verification

All commands ran from `/Users/nobocop/play/canopy`. Before starting Go, the
[runner](web-runner.py.txt) sets temporary HOME, all XDG directories, TMPDIR and
GOCACHE, with `GOPROXY=off`, `GOTELEMETRY=off`, `CGO_ENABLED=0`, and
`GOMODCACHE=/Users/nobocop/go/pkg/mod`. macOS `sandbox-exec` denies writes below
that module cache. Git global configuration and ambient wiki overrides are
isolated. TestMain isolation is additional protection, not the first isolation
boundary. The source hashes and command results are in
[web-evidence.json](web-evidence.json).

| Command/check | Final result | Evidence and limit |
| --- | --- | --- |
| `go test -buildvcs=false -json ./internal/webui/ ./internal/wiki/` | PASS, exit 0: web 41 top-level / 63 including subtests; wiki 15 / 20 | [Full JSON events](web-go-tests.jsonl). Includes root-owned boundary/address tests; no ORT inference claim. |
| `make test-ui` | PASS, exit 0: 36 tests, 0 skipped or failed | [Node results](web-ui-tests.txt). DOM adapters, real Node fetch redirect, and actual pinned TeX processor compilation; not browser paint. |
| `make test-assets` | PASS, exit 0: 6 native-download fixtures; 4 vendor assets, 6 notices, packaging declarations and 8 asset controls | [Results](web-assets-tests.txt). Native fixtures are not a native inference test. Actual release archive explicitly remains unverified. |
| Asset checker `--archive` on generated tar.gz and zip | PASS, exit 0 for both | [CLI results](web-archive-tests.txt). These fixtures contain canonical notices and manifest, not a release binary. |
| Asset checker `--archive` on fixture missing manifest | Expected rejection, exit 1 | Same log: missing canonical `assets.json` is rejected. Self-tests also mutate vendor/notice/manifest bytes and omit a license. |
| `make i18n-check` | Latest run FAIL, exit 2: two stale markers after shared docs edits; earlier run passed | [Both results](web-i18n-tests.txt). README.en.md and docs/en/getting-started.md require coordinator reconciliation. This worker did not edit mirrored user docs in this dispatch. Locales load in the Go suite. |
| `gofmt -l` on ten owned Go files; `git diff --check` | PASS, exit 0, no output | [Result](web-format-tests.txt). Source manifest records the checked files. |

Earlier test runs exposed old assertions for empty-request redirects and fake
`tasked=1` receipts, and the old JS test's non-JSON success stub. They were
updated to assert the new contract. Final web, wiki, UI and asset executions
have no unexpected failures. The shared-document i18n failure was reported to
root; the intentionally rejected archive is a negative control.

The HTTP and input-preservation tests exercise the requested flows:

| Flow | Executed checks |
| --- | --- |
| Search → evidence | Cold no-model API indexes current files; edits refresh every vector consumer; exact-slug redirect also refreshes; deleted pages leave keyword results; fake-profile vectors change with current content. Actual embedded asset URL hashes and ETag/304 bodies are checked. |
| Page → proposal → queue | File changed after opening editor, then proposal submitted; POST-time base, exact normalized body/request, real receipt, task state/time and unchanged page bytes are asserted. Invalid/ambiguous targets are rejected, and a dotted non-target file remains byte-identical. |
| Unread → read | HEAD preserves exposure history; GET consumes it once; unread pages remain visible without a candidate; partial aggregates retain healthy information and refuse writes byte-for-byte; unknown writes fail, historical undo succeeds. Seven actual JSON/HTML response failures and a real followed HTTP login redirect preserve client controls. |

Other regression checks compare decoded literal code source for four-backtick,
tilde, indented, list and blockquote fences, and single/double/multiline spans.
They assert only the outside wikilink is resolved. A blocked fake engine proves
that an expired queued request never starts inference, while a read-history
write remains possible during indexing. These are behavioral/content checks,
not just searches for implementation text.

## Rendering and redistribution choices

The selected vendors are force-graph **1.51.4** and MathJax **3.2.2**. The existing
force-graph distribution was compared byte-for-byte with its publisher's
[npm release archive](https://registry.npmjs.org/force-graph/-/force-graph-1.51.4.tgz),
and tex-svg with the pinned
[upstream bundle](https://raw.githubusercontent.com/mathjax/MathJax/3.2.2/es5/tex-svg.js).
The local color and physics components retain pinned upstream bytes under local
filenames. The manifest records asset and upstream hashes, archive hash,
processing, exact versions, licenses, and corpus.

The complete [MIT text](../../../../internal/webui/static/vendor/LICENSE-force-graph.txt)
retains Vasco Asturiano's attribution; the complete
[Apache 2.0 text](../../../../internal/webui/static/vendor/LICENSE-mathjax.txt)
and redistribution note retain MathJax Consortium attribution. The pinned
MathJax distribution has no top-level upstream NOTICE; canopy's `NOTICE-*`
files identify themselves as redistribution notes, rather than invented
upstream notices. Primary license sources are the pinned
[force-graph license](https://github.com/vasturiano/force-graph/blob/v1.51.4/LICENSE),
[MathJax license](https://github.com/mathjax/MathJax/blob/3.2.2/LICENSE) and
[corresponding source header](https://github.com/mathjax/MathJax-src/blob/3.2.2/ts/input/tex.ts).
The manifest is provenance, not a replacement for these texts.

Supported TeX packages are `base`, `ams`, `newcommand`, `color`, `physics`.
`autoload`, `require`, remote extensions and the optional menu are disabled.
Five corpus expressions compile with the shipped processor and extension
modules into MathML without `merror`; an undeclared `mhchem` request produces
an error and keeps source. This bounds support rather than promising full
LaTeX. Browser loader, SVG rendering and source presentation remain unverified.

The checker verifies source hashes, required notices, the root's GoReleaser and
canonical Homebrew installation declarations, and optionally archive contents
without extracting files. Archives must carry the exact notices, font license,
and manifest. There is no manifest self-hash. This is the selected two-vendor
check, not a full dependency SBOM or vulnerability audit. Root owns actual
release assembly and installation verification.

## Unverified execution and review risks

The coordinator rechecked Orca's browser on 2026-10-10: the tab list was empty
and creation returned “Browser tabs are unavailable while a remote runtime is
active.” No ad hoc browser/CDP/Playwright bypass was used.

| Actual execution | Status |
| --- | --- |
| 360px and 1280px views in light/dark themes | UNVERIFIED |
| Keyboard focus order, selected graph links, screen reader behavior | UNVERIFIED |
| Native IME composition and search/proposal input | UNVERIFIED |
| JavaScript-disabled reading and native form flows in a browser | UNVERIFIED; HTTP/native HTML paths are covered separately |
| Mermaid paint, MathJax local-component loading/SVG paint, canvas graph | UNVERIFIED |
| Full search → evidence / page → proposal → queue / unread → read browser flows | UNVERIFIED |
| Malicious-browser exploit against the HTTP/auth boundaries | UNVERIFIED; root HTTP tests are not that exploit |
| Real ORT execution and interruption | UNVERIFIED in this worker; only fake-engine cancellation and lite tests ran |
| Actual GoReleaser archive, Homebrew installation, deployed-server refresh | UNVERIFIED in this worker; source declarations and synthetic archives passed |

The [palette calculation](web-code-contrast.json) measures explicit token colors
against declared code surfaces only. It is not a browser contrast or
accessibility pass. Native inference interruption, actual-browser behavior,
release notice contents, and deployment cache transition must retain their
separate evidence. The worker notified root when the implementation was ready
for a fresh forward test; independent challenge and final artifact/release
checks remain with the coordinator.
