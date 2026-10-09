# PLAN-01 and PLAN-02 documentation implementation

Completed 2026-10-10 KST in the shared checkout. Historical audit files were not edited.
The root-owned new V section was preserved; edits in invariants are confined to the existing
A–U sections and audit procedure. No Go source, actual user wiki, installed agent directory,
repository commit, or remote publication was changed by this worker.

## Delivered

- Both embedded skills now confirm the requested `--wiki` against `status.root`, preserve
  that target, distinguish quoted source content from delegated `task.request`, and reuse
  existing authorization. Read-only work ends with evidence; no-model recall falls back to
  keyword search and show. Writing keeps body inputs and backs up consumed checkout copies
  before checkin; errors require inspecting current content and application state before retry.
- Task completion guidance describes the actual mechanical floor: connect checks mutual
  links; edit checks the whole-file hash against base, while legacy tasks without base check
  existence. The agent must separately check the requested content. Sync includes all dirty
  wiki changes, including pre-existing changes, and is not a read-only queue refresh.
- Current README, getting-started, troubleshooting, upgrading, and English mirrors explain
  proposal acceptance versus application, slug versus title, installation versus activation,
  legacy/bootstrap taxonomy, JSON/show banner exceptions, append-only history versus cache,
  and local migrations versus traveling files. They link the new mutation result contract.
- A3 includes parse failures and the five-key diagnostic, with null/empty/scalar tags handled
  explicitly and no new universal write gate. P1 distinguishes invalid from unchecked.
  R1 permits the first agent-read aggregate and requires same-day repeat no-op. Quoted W
  examples and full-file snapshot procedures work with spaces in Bash and zsh.

## Validation

Raw commands, stdout, stderr, exit statuses, content checks, source hashes, and non-target
hashes are in [docs-evidence.json](docs-evidence.json). Test helpers are preserved as inert
text: [runner](docs-runner.py.txt), [workflows](docs-workflows.py.txt), and
[document QA](docs-qa.py.txt). These are scripted workflow tests, not an independent LLM
behavioral evaluation or proof of prompt-injection resistance.

Go started with HOME, all four XDG homes, TMPDIR, and GOCACHE under
`/private/tmp/canopy-docs-20261010-zje6hdi9`, before the Go process existed. It used
`GOMODCACHE=/Users/nobocop/go/pkg/mod`, `GOTELEMETRY=off`, `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, and `CGO_ENABLED=0`. macOS sandbox-exec denied writes to that shared
module cache. Global/system Git config was disabled. Final CLI fixtures used a fresh isolated
environment under `/private/tmp/canopy-docs-final-20261010-1lhny1xs`, with no model files.

The final tested binary SHA-256 is
`07eabd3ecbf7006c65e5057d64ae5e33d97076574213638a13424c8808d7ec2e`.
The checkout continued changing in other workers, so this is a precise artifact result,
not certification of subsequent shared-checkout changes.

| Check | Result | Evidence and limit |
|---|---|---|
| `make i18n-check` | PASS | Four mirrors current; all 22 fenced blocks also compared byte-for-byte between source and mirror |
| `go test -count=1 -v ./internal/skills/` | PASS | `TestInstallRefreshesEmbeddedSkills`, `TestDetectAndInstallAll`, `TestPiDetection`, `TestInstallLayoutPerAgent`; refreshed after the final skill edit |
| `go test -count=1 -v ./internal/lint/ ./internal/mermaid/` | PASS | Six lint and five Mermaid tests, including actual key-presence and tag page-dedup tests; actual test names/output retained |
| `go test -count=1 -v ./internal/webui/ -run 'TestRawHTMLCannotExecuteButGeneratedMarkupSurvives\|TestWebEditFilesProposalNotFile'` | PASS | Both named tests actually ran; generated markup survives and web edit preserves the submitted proposal without changing the page |
| `go test -count=1 -v ./internal/resurface/ -run 'TestSnoozeDurationAcrossZones\|TestLegacySnoozeDateStillLoads'` | PASS | Both named tests actually ran; duration deadlines and readable legacy dates |
| `go build -buildvcs=false -o <isolated>/canopy ./cmd/canopy` | PASS | Actual cgo-free lite binary; model/native/ORT execution not claimed |
| Two-wiki/no-model workflows | PASS, 17/17 | See the detailed cases below; actual content and input preservation, not skill wording matching |
| Local document links and Mermaid | PASS | 114 relative file links exist; two actual Mermaid blocks passed the embedded parser with invalid=0 and unchecked=0; anchors/external URLs and browser layout unverified |
| Quoted W path positions | PASS | Bash and zsh ran CLI `--wiki "$W"`, Git `-C "$W"`, and jq file paths with spaces |
| `git diff --check` | PASS | No whitespace errors at completion |
| skill-creator `quick_validate.py` for both skills | FAILED TO START / UNVERIFIED | Both exit 1 with `ModuleNotFoundError: No module named 'yaml'`; no validator pass claimed and no global package install attempted |

The generic validator's source also excludes the existing `category` property. That property
is retained as purposeful Hermes metadata; changing the target installation format merely to
satisfy a different validator is outside this correction. Naming/frontmatter were reviewed and
the actual embedded installation tests passed, but this does not substitute for its unavailable run.

Two earlier builds failed during concurrent API integration: the first had three
one-result uses of the new two-result `wiki.SetFrontmatterField`; the second referenced
undefined `Schema.RequiredFrontmatter`. The first also attempted a main-module stat-cache
write, which the read-only sandbox denied. These were reported to the coordinator, followed
by successful `-buildvcs=false` builds; no source correction outside ownership was made here.

## Workflow observations

All commands kept the requested wiki explicit while cwd, CANOPY_WIKI, and the pre-existing
default pointed at the other wiki. Initializing the requested wiki preserved the existing global
config bytes. The other wiki's entire non-Git file manifest remained identical throughout.

| Case | Actual outcome |
|---|---|
| New page | Stored body equals intended input; show content equals on-disk bytes; input retained |
| Read without model | Recall nonzero; keyword and hybrid both returned the intended slug in actual keyword mode; show supplied the cited content; no model files appeared |
| Partial edit | Requested sentence changed, unrelated quotation remained, pre-checkin backup retained, successful checkin reclaimed the working copy |
| Ingest | Attachment original and raw bytes match; existing raw collision refused without overwrite; sources and cited body match the intended edit |
| Delegated edit | Premature done nonzero; legitimate request applied and marked done; quoted scope-expanding text preserved as data; non-target wiki unchanged |
| Checkin log failure | Nonzero, `applied:applied`, `stage:log`; intended body present; input, backup, and retained working copy survived; no blind retry |
| Published new recipe, Bash and zsh | Eight cases: success, invalid-type refusal, duplicate slug, and postpublication log failure in each shell; failures nonzero with input preserved; duplicate/refusal snapshots unchanged; partial failure stored the intended body |
| A3 actual lint predicate | Ten cases: healthy, no frontmatter, malformed YAML, each of five missing keys, tags null, and scalar tool; only declared healthy/tolerated cases green |
| P1 predicate | Healthy green; invalid and unchecked red separately. The unchecked input was synthetic predicate data, not runtime lint fault injection |
| C4/R1 snapshots | Cache deletion/reindex preserved all wiki files and returned the expected hit; first checkout changed only agent-reads.json with days=1; same-day repeat changed no wiki file; copy outside wiki |

## Remaining limits and reconciliation

Fresh independent skill forward-testing was requested through the coordinator when the drafts
were ready. It remains UNVERIFIED here: scripted positive/quoted-data cases do not measure
activation, agent judgment, or injection resistance. The coordinator will provide the fresh agent;
this worker spawned none.

The six-verb JSON/sync/repair matrix, concurrent writes, task GC retention, old-clone migration,
actual remote sync, complete test suite, native ORT, and real browser/focus/accessibility checks
were not run by this worker. Their implementation owners and root own final integration.
The raw-HTML and snooze wording was reconciled with the chosen implementation and targeted
tests; no outstanding known contradiction remains there. The mutation contract is linked and
selected failure fields were observed, but full matrix acceptance is not inferred from these cases.

The W examples assume the declared disposable environment, a prepared binary/module cache,
and a new scratch wiki for destructive cache/foreign-file fixtures. The final report does not claim
those examples can be run unchanged against a live user wiki. Historical audit assertions and
the root's V contracts remain separate from these measured results.
