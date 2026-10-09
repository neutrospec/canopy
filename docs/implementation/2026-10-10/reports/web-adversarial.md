# Independent HTTP, rendering and UI review — 2026-10-10

Four independent counterexamples were found in the initial lite binary: malformed GET queries/invalid `k` returned successful fallbacks, an unused JSON body bypassed its route budget, hover previews used an ambiguous reference, and a math error caption exposed a message identifier. All four pass their same repros after coordinator corrections; the final added multipart-file rejection also passed an independent file-versus-fields control. The coordinator owns the production corrections; this worker owns only this report and evidence, and distinguishes actual HTTP, Node adapter logic, TeX compiler execution and unverified browser behavior.

## Evidence and review boundary

| Artifact | Scope |
|---|---|
| [web-adversarial-before.json](web-adversarial-before.json) | Frozen initial binary, policies, fixture setup, exact commands and HTTP responses, file contents/hashes, independent findings and controls; W1–W266. |
| [web-adversarial-evidence.json](web-adversarial-evidence.json) | Complete append-only event sequence including the same repros after coordinator changes, source snapshots, executed script text and cleanup receipt. |
| [web-adversarial-replay.py.txt](web-adversarial-replay.py.txt) | Inert opt-in focused replay against current source; exact historical scripts are also preserved in the evidence JSON. |

Initial evidence SHA-256: `80b79c2a1a2a7d61fd6d61c3305e513a967fc6d9f397ee8c57b434ae0951f107`. Initial binary SHA-256: `13b69019e8352af0ec9e02ab3908196decec756c560254ac4e3be4645ff666b6`; final binary SHA-256: `a934ca8c1395a1e35dae373557f02cc438f5c7396a4e8293093eda24791a4db2` (W356). The initial source was the dirty worktree at revision `ede67e302a8487aecbb4d59587a8686292018cf6`; that revision alone does not identify uncommitted production changes.

I read AGENTS, PLAN-12/13/14 selected policies, `web-contract.md` and the relevant invariants, then executed my own small CLI/HTTP/HTML flows and inspected current implementation. The first targeted read of `reports/web.md` was after the core independent flows (W173); the full report was captured at W254. Its conclusions were not the fixture recipe or acceptance oracle. No other agent was used and no production source, skill, previous report or actual wiki was edited.

All wiki, HOME, XDG, TMPDIR and working files were under `/private/tmp/canopy-web-adversarial-20261010-gts7m0qy`. Go started only after setting `GOTELEMETRY=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0`, the requested shared `GOCACHE`, and existing `GOMODCACHE`. All three builds ran through `sandbox-exec` denying writes below `/Users/nobocop/go/pkg/mod`; denied Go stat-cache writes were visible on stderr with build exit 0 (W7/W272/W355). No model, external network, download, push, release archive, Homebrew installation or existing service was used. The external-auth control bound a disposable server to `0.0.0.0` and connected to it through loopback only.

There were two explicit setup corrections, not product regressions: `init` correctly rejected the not-yet-created fixture directory (W10–W12), and the deliberately duplicated basename correctly rejected reindex before normal flows (W20). One duplicate fixture file was then moved outside the governed directories. An HTTP client request that incorrectly included a literal `#task-…` was corrected to `/special/tasks`, because browsers do not transmit fragments (W116/W137–W138). None was presented as a UI defect.

## Counterexamples and same-repro verification

### WA-01 — P2: malformed query and invalid result count masqueraded as success

**Initial failure:** `GET /api/search?q=%ZZ` returned 200 `{"results":[]}`; `/search?q=%ZZ` redirected to `/`; an unescaped semicolon query also returned empty success (W65–W71). `k=-1`, `k=oops` and `k=51` returned 200 using the default count. Valid `k=1` returned current evidence. Malformed POST encoding correctly returned 400, providing a contrast rather than a blanket parsing failure (W73).

**Location:** initial `internal/webui/boundaries.go:112` and `server.go`'s `handleAPISearch`. `URL.Query()` discards `ParseQuery` errors, and invalid supplied `k` fell back to eight.

**Consequence and correction:** a client could interpret malformed input as a successful empty search or a successfully honored count. Parse the raw query explicitly and reject malformed encoding; when supplied, require exactly one integer `k` in `1..50` before even the empty-query response. Omitted `k` retains its documented default.

**Final disposition: PASS after correction.** The actual corrected binary returned 400 for all nine malformed/count cases, including empty `q` with invalid `k` and repeated `k`; valid, omitted and empty-query controls returned 200 (W275–W286). The final rebuild again returned 400 for `%ZZ` and 200 with a current evidence hit (W368–W369).

### WA-02 — P2: the route body cap did not cover unread content types

**Initial failure:** `POST /api/read/anchor`, JSON content type, 17 KiB of `x` returned 200 `{"read":true}` against the 16 KiB default cap (W177). The empty-body control returned 200 (W178). A real chunked request with the same 17 KiB body also returned 200; a one-byte chunked control succeeded (W263–W264).

**Location:** initial `internal/webui/boundaries.go`, `requestLimits`. `MaxBytesReader` only constrains bytes actually read; `ParseForm` leaves unsupported content types unread.

**Consequence and correction:** the declared budget did not hold before mutation. This is not a claim of unbounded allocation or native inference. Reject a declared length above the cap and consume the remaining body through the same cumulative cap before dispatch, including unknown-length bodies and multipart epilogues. Keep valid form fields available and clean spilled files on both success and parse failure.

**Final disposition: PASS after correction.** Both the original declared-length body and actual chunked 17 KiB body returned 413; a one-byte chunked and empty JSON-typed body returned 200 (W287–W292). The final rebuild repeated both oversize paths and proved unchanged read-state hashes before the empty-body success control (W364–W367). This includes behavior before mutation, not merely an error status after a write.

### WA-03 — P2: preview construction discarded a canonical page address

**Initial failure:** lawful files `concepts/exact.md` and `concepts/exact.md.md` make the bare reference `exact.md` ambiguous. The home correctly generated `/page/concepts%2Fexact.md.md` for the latter, and following it returned 200 (W105–W107). The same anchor's `data-slug="exact.md"` made the shipped script request `/api/preview/exact.md`, which returned 404; the canonical preview path returned 200 (W109/W140/W256).

**Location:** initial `internal/webui/static/app.js:139–144`, `showPop`.

**Consequence and correction:** navigation worked but its preview disappeared for a supported address. Resolve the reference from the canonical `/page/` href, excluding fragment/query, and encode that reference for preview. The Node adapter executed the actual shipped script and recorded the requested URL; it did not simulate a browser's hover geometry or paint.

**Final disposition: PASS after correction.** The corrected shipped script requested `/api/preview/concepts%2Fexact.md.md` in the same Node adapter (W320), and the live route returned 200 (W314). The final rebuild repeated the route and script checks (W370/W378). Actual browser hover/placement remains unverified.

### WA-04 — P3: math failure source caption used a missing client message

**Initial failure:** the actual page's `ui-messages` omitted `math_source` (W200). Calling the shipped script's `formatError` through a small Node DOM adapter preserved the exact TeX but produced the literal summary `math_source` (W257). Both server locales already contained the intended label.

**Location:** `internal/webui/server.go`, `clientMessageIDs`, and `static/app.js`'s MathJax `formatError` callback.

**Consequence and correction:** the failure UI exposed an implementation identifier instead of its localized source label. Include the existing key in the client message payload. This adapter checks error UI construction; actual compiler coverage is separately recorded at W259.

**Final disposition: PASS after correction.** The fresh served message payload contained `math_source`; the same shipped-script adapter produced `TeX 원문` and preserved the exact source (W321). The final rebuild repeated it (W372/W379). This remains a construction/translation check, not browser paint.

## Per-contract disposition

PASS means the stated bounded observation passed; it is not certification of every input or environment. An initial FAIL remains in the frozen evidence after a correction.

| Contract | Observed disposition | Final disposition / evidence |
|---|---|---|
| V12: actual listener / explicit Host / forwarded headers | PASS | Actual loopback port and explicit `wiki.fixture:8443` accepted; hostile Host, wrong/missing ports and forwarded-Host spoof rejected with 403 (W53–W60). |
| I1/I3/I4, V12: external auth, Origin, logout | PASS | API 401 JSON versus HTML 302; wrong bootstrap code 400; valid registration/session works; hostile Origin logout 403 retains session, same-Origin logout 303 invalidates it; local bogus-cookie logout has no panic (W61–W64/W90–W100). |
| V12: query decoding / supplied count | FAIL → PASS | WA-01; same malformed/count cases now 400, valid controls 200 (W275–W286/W368–W369). A 40,000-byte request target returned 431 (W72). |
| V12: malformed and oversized forms | PASS | Malformed percent/multipart 400; edit, task and login parsed-body caps 413; rejected forms left task hashes unchanged (W73–W79). |
| V12: content-independent body budget | FAIL → PASS | WA-02; declared-length/chunked 413, read state unchanged, small/empty controls 200 (W287–W292/W364–W367). |
| V12: multipart policy / temporary files | PASS for cleanup; file policy clarified later | Initially an accepted 100 KiB file part and a truncated multipart left no temporary files (W80–W81/W175–W176). The coordinator then explicitly selected unsupported file parts → 400: the final same file request rejected with no task/page mutation or spool residue (W358–W359), while fields-only multipart returned 303 with exact body/request/base and no page mutation (W360–W362). This late policy correction is distinct from the four independently discovered defects. |
| Search bounds / cancellation / truthful errors | PASS within lite HTTP scope | Under a private exclusive SQLite lock, eleven concurrent requests produced three immediate 503s with `Retry-After: 1`, six queue-deadline 500s, one `SQLITE_BUSY` 500 and one valid response after release; all workers finished and a normal follow-up returned 200 (W179–W193). A queued socket was disconnected, and server cancellation was observable; no engine call-count claim. |
| V9 / search → actual evidence | PASS, keyword only | API exposed actual `mode: keyword`, `semantic_ready: false`, profile and coverage; an external fixture edit removed the old marker from results and exposed the new one (W33/W123–W126). Semantic retrieval quality remains unverified. |
| I2/V13: page → proposal → receipt | PASS | After opening the editor, the fixture page changed; proposal `base` matched the file at POST time, body matched after CRLF→LF normalization, request remained intact and POST did not modify the page (W110–W115). Queue HTML retained full escaped body/base/status/time; empty proposal 400 created no task; request-only submission and pending-edit withdrawal worked (W117–W122/W137). |
| U2/V13: unread versus recommendations | PASS | Two old, unlinked pages had unread count 2 and no recommendation, without an “all read” claim (W148–W149). |
| V13: healthy / partial corrupt reading state | PASS | Healthy reads remained when agent JSON was corrupt; human corruption gave dash counters and graph `read:null` with explicit availability flags. Corrupt bytes and the healthy counterpart were preserved; new writes returned 500 rather than success (W151–W166). |
| V13: HEAD home exposure and unknown references | PASS | First HEAD `/` returned no body and left resurface state absent; GET created exposure state (W146–W150). Unknown read/feedback/edit/connect requests returned 4xx; historical read undo after page removal remained possible (W82–W87/W168–W171). |
| V14: Markdown/code/raw HTML | PASS at HTTP/HTML layer | No generated anchors inside inline, blockquote/list fence or explicit indented code; raw executable tags/handlers omitted, javascript href removed, alias text escaped; ordinary wikilinks, table, math source and Mermaid source remained (W197–W198). No actual browser exploit was run. |
| V4/V14: dotted, nested and ambiguous addresses | PASS for declared namespace; preview FAIL → PASS | Dotted page works; nested page works after its directory is explicitly declared; undeclared nested file is not implicitly governed. Duplicate basenames refuse the scan, and ambiguous extension-like bare references refuse resolution while explicit governed paths work (W20/W23–W24/W38–W41/W129–W132). WA-03 now requests the canonical preview and returns 200 (W370/W378). |
| V13/V14: auto-read UI truthfulness | PASS for shipped-script Node adapter | Only JSON `read:true` changed the form; `false`, HTML, redirect, invalid JSON, 401 and network error preserved it and showed a failure; canonical read reference retained (W258). This does not prove scroll/dwell, native focus or browser session behavior. |
| App content URL / ETag revalidation | PASS for live HTTP, including upgrade | App CSS/JS query versions matched the first 12 SHA-256 hex characters; ETags matched full bytes, `no-cache` was present, matching validators returned 304 with no body, and HEAD returned no asset body (W207–W221). Replacing the old binary with the corrected one at the same loopback origin changed app URL/bytes/ETag; the old URL plus old validator returned 200 with new bytes, and the new matching validator returned empty 304 (W324–W333). Final JS bytes match that upgrade probe (W376). This is HTTP revalidation, not a browser reload/offline test. |
| V14: local asset routes and manifest hashes | PASS for binary/source scope | Mermaid, force-graph, MathJax and local color/physics components, manifest and licenses/notices returned 200; pinned assets and notices matched their source hashes (W219–W241). |
| V14: declared MathJax corpus | PASS for actual TeX compiler | All five manifest expressions compiled to MathML without `merror`; undeclared extension and malformed TeX produced `merror`, preserving source (W259). Loader, SVG paint, fonts and accessibility remain unverified. |
| V14: notice/manifest packaging declarations | PASS for declarations/fixtures | Asset checker passed four assets, six notices and eight positive/negative controls (W260). These are not actual release archives or a Homebrew installation. |
| U1: navigation, labels, fallback structure | PASS only for static/HTTP structure | Served graph/browse/task/page/editor links have the expected active state; search has none. Main/skip targets, localized navigation, associated proposal labels and native form actions are present (W334–W339). Graph exposes page/link lists. The source contains the responsive navigation rule; actual visibility, tab order and JS-off use remain unverified. U6 browser interaction was not independently exercised. |
| U5 / actual-browser / JS-off / offline-browser | UNVERIFIED | No browser tool available; the dispatch supplied an earlier Orca remote-runtime browser rejection. No alternative browser or runtime change attempted (W261). |
| Native inference interruption / actual archives / Homebrew | UNVERIFIED here | Lite has no native engine; actual release and Homebrew execution belong to the coordinator after release. |

## Limits and cleanup

No broad Go, browser or platform suite was repeated for this review. Existing tests were not counted as independent fixture coverage. Node adapters execute shipped logic with controlled events/responses; their stub focus/DOM is not native focus, IME, responsive layout, contrast, screen-reader use or paint. TeX compilation is stronger than a stub macro check but does not establish local browser loading or SVG rendering. The HTTP queue experiment confirms finite observed behavior under this fixture; it does not promise immediate native inference interruption or a strict total response deadline.

The final evidence preserves private fixture text, hashes, commands, responses and script sources, followed by deletion of all disposable wiki/server/binary fixtures. The shared requested Go build cache is retained. No blanket product or release certification is made.
