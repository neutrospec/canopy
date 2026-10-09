# Preservation and state: independent adversarial review

2026-10-10 · task `task_e4f2c148f34a` · dispatch `ctx_0e91735d6881` · baseline
`ede67e302a8487aecbb4d59587a8686292018cf6`.

**Five implementation defects and one contract mismatch were reproduced and sent
to root. Root fixed the five defects and clarified the cutoff policy; the same
counterexamples now pass. No reported issue remains unresolved in this bounded
review.** I did not implement these packages or edit production code, repository
tests, other reports, or the historical audit. This report and its inert
reproduction/evidence attachments are my only repository changes.

## Reproduced findings and retests

The [evidence JSON](preservation-state-adversarial-evidence.json) contains exact
commands, exit codes, complete stdout/stderr, inputs, before/after file hashes,
initial/final binary hashes and the final source snapshot. Names below are
keys under its `probes`/`commands`. Roots in commands are fresh private fixtures,
not the user's wiki.

| Finding | Original failure and practical impact | Retest after root's change |
| --- | --- | --- |
| PSA-01: ambiguous checkout discard | With both `concepts/foo.md` and `concepts/foo.md.md` checked out and modified, `checkin foo.md --discard --json` returned exit 0 / `noop` and deleted the short page's working copy. Other commands rejected that reference as ambiguous. A listed dotted slug could therefore discard a different copy. | The same request returns exit 1 / `error`, `applied:none`, and preserves all three copy hashes. Explicit `checkin concepts/foo.md.md` still applies only the long page and reclaims its own copy. `first-probes` → `first-probes-after1`. |
| PSA-02: generated links cannot be resolved | Successful explicit long checkin regenerated category/dashboard links as `[[foo.md]]`, ambiguous in this legal two-page namespace. Normal `[[foo]]` worked, but the generated long-page link could not select its target. | Generated links use `[[concepts/foo.md.md]]`. Every category link was subsequently passed to the real CLI `show`; each resolved to its intended governed file. `first-probes`, `first-probes-after1`, `final-integration-probes.generated_links`. |
| PSA-03: task order compares timestamp text | `tasks list --json` placed `2026-10-10T02:00:00Z` before `2026-10-10T10:00:00+09:00`, although the latter is 01:00Z and is earlier. This could change which queued request an agent sees first. | Old and final binaries were run against the **same two unchanged task files**: old order `later, earlier`; final order `earlier, later`. Both exit 0 and all file hashes remain identical. `time-order-retest`. |
| PSA-04: rename rewrites unresolved references | `mv concepts/foo.md --slug bar --json` returned exit 0 / applied and rewrote ambiguous `[[foo.md]]`, wrong-directory `[[entities/foo.md]]` and nested `[[concepts/nested/foo.md]]` as if they were actual inbound links. Valid path, long-page and code-span controls distinguished this from a blanket Markdown problem. | Only the resolved `[[concepts/foo.md\|short]]` changes. Ambiguous, wrong-directory, nested, long-page and code text remain byte-for-byte as supplied. `address-probes` → `address-probes-after1`. |
| PSA-05: alias root skips abandoned scratch cleanup | A live holder excluded alias acquisition; SIGKILL released the lease, but `CleanTemps(aliasRoot)` returned nil while retaining its interrupted scratch. Real CLI `update alpha --json` through an alias also exited 0 while leaving identical reserved scratch. Later alias operations therefore failed to reclaim it. Staging already excluded scratch; I did **not** reproduce its publication. | Root resolves the root symlink before walking. The same killed-holder overlay passes. Old/final CLI probes both publish the same page hash, but only the final binary removes the identical scratch. `review-overlay-live-helper`, `review-overlay-final`, `final-integration-probes`. |
| PSA-06: strict-older prose versus inclusive GC | At fixed `now=2026-10-10T00:00:00Z`, keep=90 days, GC removed both a task one second before cutoff and a task exactly at `2026-07-12T00:00:00Z`. The contract said strictly older. This was a boundary-description mismatch, not evidence of arbitrary or future-task deletion. | Root deliberately retained inclusive policy and corrected `docs/state-contract.md` and the GC comment: `Closed <= cutoff` is eligible. The final probe expects two deletions and separately checks after-cutoff, future-version, unknown-status and pending files survive. |

Representative exact hash transitions (full inputs and remaining hashes are in
the evidence):

- PSA-01 short working copy
  `cef8ed01edaa6848aad27b1eb9d0c102e93fc4c7cc41155928e7ee2e8fedf3cf`
  became absent originally; after the fix that hash survives the refusal. Long
  and normal non-target copy hashes also survive.
- PSA-02 category bytes changed from
  `0d13ce969dc3f888039e61ddb888eccea3dfae042332ef26087fc583491c58d6`
  to `294693e03ac4c4ea2355d93e57fa4969d57120eee5b3950384076d7d5a9d2e73`.
  Resolution is verified by CLI content/path output, not this hash alone.
- PSA-04 reference source hash
  `2d586a7a7c35f4063ba3b2339c5391615392a7a0c70d99de1f2926dc26d8d79b`
  originally became `83f1f0a7f1fa76073b01a002cbd6b9460a8eae405ec8acae5251c5d4331786f0`.
  The correct rewrite becomes
  `2e45f771ee32f29bce2f1a39552fb84df772d578c93cc34594ea0b5e2c80196c`.
- PSA-05 scratch input/old output hash is
  `02828b1b115c781eb8622ce5ca472aa73c354da323f837073eff29ab102aa8f5`;
  final output is absent. Both CLI page outputs have hash
  `4259450408d98fab12e1c267f8ccc3a943e18912517053f4ec7e41656ec4efe2`.
- PSA-06 exact-cutoff task input hash is
  `6aa7af3c79f56524334c782c4d630b76aa9e1fc6be4914b143776573dba3655e`;
  its output is absent under the now-explicit inclusive policy.

Findings were reported promptly through Orca (`msg_3f4a9b9bad18`,
`msg_56b60b94200a`, `msg_93a633e28110`), followed by actual retest results.
Root made the implementation/contract edits; the reviewer did not apply fixes.

## Independent failure probes and normal controls

These checks execute new small fixtures rather than using the presence of
implementation tests as proof. All recorded `passes` fields are true for the
final cases below.

| Area | Executed observation |
| --- | --- |
| Git pull/autostash conflict | Bare file remote and two clones; same-line local/upstream edits. `sync --json` exits 1 at pull with commit/push not attempted. Remote tip is unchanged and unmerged entries remain visible. Full blobs retrieved from autostash and upstream HEAD hash identically to the original local/peer inputs, respectively. `exact-preservation-retest.git`. |
| Normal sync and rejected push | Nonconflicting upstream page plus unrelated dirty local file. Supported `update --body-file INPUT --sync --json` emits one parseable object, exits 0, confirms commit/push, carries the unrelated file to the remote, and its final Markdown index includes the pulled page. A real file-remote pre-receive rejection then yields nonzero / applied / push failure, confirmed local commit, unchanged remote tip and retained local input. `git-probes`. |
| Atomic file publication | Eight concurrent exclusive creates yield one complete winner. Thirty replacements alternate 200003/300007-byte payloads while a reader sees only either complete value. Permission bits survive; an existing archive destination refuses with both complete inputs unchanged; a symlink target is refused and untouched. |
| Local process and alias locks | Independent helper holds the exclusive lease; alias acquisition times out. After SIGKILL and wait, the alias acquires the same lease within the test budget. Abandoned scratch is reclaimed; Markdown-shaped scratch and Git-internal data remain. This tests process death, not power loss. |
| Checkin partial apply and repair | A symlink at the generated index path injects a postpublication index failure. Result is nonzero / applied / index; `applied_hash` matches the actual page. An immediate postfailure hash check confirms the full checkout and external target bytes survive. Editing the retained copy causes refusal with its new input preserved. After rescuing it, restoring the matching proposal and removing the bad index target, checkin resumes without republishing, clears cleanup, and records exactly one operation ID. A blind subsequent checkin is refused. `recovery-queue-probes`, `exact-preservation-retest.checkout`. |
| Orphan checkout | Replace metadata with literal `null`, retaining an edited copy. Recheckout refuses; `checkout --list --json` exposes orphan status/diagnostic; copy hash is unchanged. `recovery-queue-probes`. |
| State preservation/concurrency | Two independent processes load the same version-91 read aggregate before a barrier, then add disjoint keys. Both succeed and the union preserves unknown top/nested values, null and exact `90071992547409931234567890`. A different stale update of the same leaf is refused with unchanged bytes. Same-version unknown TOML extension is refused with the original file unchanged. |
| Partial read history | Healthy human history survives corrupt agent `null`; updating the healthy half leaves the corrupt bytes unchanged. A human null entry is unavailable, and its attempted write refuses while both source files remain unchanged. |
| Task identities, content and GC | Eight simultaneous CLI processes return eight unique edit IDs, and all eight full multiline/Unicode requests survive. Legacy dismissed `(a,b--c)` is byte-identical; `(a--b,c)` gets a different ID, and reverse refiling reuses its own receipt. Filename/ID mismatch makes GC fail before deleting any file. Dismiss patches both current and future/unknown-type tasks while preserving proposed body, request, null, nested arrays, huge exact numbers and version. |
| Time and snooze | Event queries/prune use mixed-offset instants before/at/after a fixed cutoff; exact cutoff is retained and malformed timestamp remains. 1/7-day snoozes at +14/-11 offsets suppress immediately and until deadline; eligibility returns at the deadline. Legacy date expires at UTC midnight; future version and unknown number remain. |
| Rename state | Agent source/destination collision refuses before publication with page/history hashes unchanged. Normal control merges human counts to five with actual-instant extrema, moves agent-only history, retains future version numbers, and leaves historical task/cooldown references unchanged as documented. |

## Isolation, commands, and reproducibility

The fresh root is
`/private/tmp/canopy-preservation-state-review-k2k8h9ze`. The
[runner](preservation-state-adversarial-runner.py.txt) sets HOME, all XDG
directories, TMPDIR and GOCACHE **before Go starts**, with `GOTELEMETRY=off`,
`GOPROXY=off`, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`,
`GOMODCACHE=/Users/nobocop/go/pkg/mod`, and `CGO_ENABLED=0`. macOS
`sandbox-exec` denies writes below that module cache and denies both reads and
writes below `/Users/nobocop/workspace/wiki`. Git identity/config are disposable;
every remote is a file path inside the fresh root. No actual wiki or live user
service was used. Isolation succeeded without an escalation or cache-write
exception.

Four isolated `go build -buildvcs=false` executions succeeded. Initial binary
SHA-256 is `3987c82a526f3c77de130aad31fcc7bad6dcb5d8fc305f9b21b6a3f141bfa194`;
the CLI-retest binary is
`65eb78fa8b0c0cc5e6acb675a970ccfde373f60500bc2acd12fd452566bd234c`;
the final handoff rebuild is
`259f653986507db23c2ee085bff4d469af7314e7d6e5cdb8c0885724173d026e`.
These identify the dirty implementation snapshots tested, not new commits.
The final handoff check detected later task/gap source changes, contract prose
and an unrelated bridge test. Current contract prose was read again, and the
already-started selected overlay was rerun against current source (eight pass).
The evidence retains both source snapshots rather than treating an earlier
CLI binary as proof of the later source. The coordinator requested no further
probe wave and asked for this bounded report to settle.

Final independent Go command:

```sh
go test -buildvcs=false -overlay=/private/tmp/canopy-preservation-state-review-k2k8h9ze/overlay.json -run '^TestReview' -v ./internal/statefile
```

**PASS, exit 0: eight substantive tests; one helper entry intentionally skips in
the parent process and is executed by the subprocess cases.**
[Full final/earlier output](preservation-state-adversarial-tests.txt) distinguishes
the failing counterexamples from retests. Private overlay sources and map were
removed after preserving the [final inert source](preservation-state-adversarial-overlay.go.txt)
and [original strict-cutoff source](preservation-state-adversarial-strict-overlay.go.txt).
No physical `internal/statefile/adversarial_review_test.go` was created.

CLI reproductions are preserved as inert text:
[initial addressing](preservation-state-adversarial-first-before.py.txt),
[addressing retest](preservation-state-adversarial-first-after.py.txt),
[initial rename](preservation-state-adversarial-address-before.py.txt),
[rename retest](preservation-state-adversarial-address-after.py.txt),
[Git](preservation-state-adversarial-git.py.txt),
[checkout/queue/rename state](preservation-state-adversarial-recovery-queue.py.txt),
[alias and generated-link integration](preservation-state-adversarial-final-integration.py.txt),
[task raw-patch controls](preservation-state-adversarial-task-close.py.txt),
[same-input task ordering](preservation-state-adversarial-time-order.py.txt),
[full-blob and immediate copy-hash checks](preservation-state-adversarial-exact-preservation.py.txt).
Copy the needed `.txt` source to a new private harness before executing; the
attached runner's root is its own directory. Never point these at a user wiki.

Two harness mistakes are recorded rather than attributed to the product. The
first driver tried unsupported `update --title`; that command exited before
mutation, and the initial index evidence actually came from the successful
explicit long checkin. Retest uses supported normalization. The first lock
helper used `select{}` and died as a runtime deadlock; that assertion was
discarded, then a live timer-sleep helper reproduced the genuine alias cleanup
failure. The final cutoff assertion changed only after the coordinator chose
and documented inclusive retention.

## Contract assessment and remaining limits

For the exercised cases, invariants V1–V8, `docs/mutation-results.md` and
`docs/state-contract.md` now agree with observable behavior. In particular,
single mutation JSON does not claim rollback, checkin receipts refer to actual
published bytes, healthy partial information is not a zero/reset, and orphan
rescue does not authorize a blind overwrite. The task-order and inclusive
retention wording was inspected after root's changes. Existing author tests were
not counted as independent evidence and were not exhaustively rerun here.

This is a bounded review, not the full six-verb/error-stage matrix. Direct
noncooperating editors/Git, distributed filesystems, different XDG cache homes,
hard-link aliases, power-loss durability and global multi-file rollback are
outside the chosen cooperation contract or remain untested. Process-death
coverage is the held-lease/scratch case; it is not proof of interrupting every
native/subprocess operation. Flat resolution does not add recursive addressing
or global page identity. Historical task/cooldown references deliberately remain
historical after rename.

No actual browser, native ORT, full UI, release archive, Homebrew install,
deployment or real user-service restart was tested in this dispatch. The
coordinator owns those separate reviews and the final full artifact checks.

During final consolidation, root also added conservative future-version
known-type task `done` refusal before verification, while retaining explicit
raw-preserving dismiss, and corrected digest invalid-date handling. Root reports
current-version done / future refusal / dismiss controls and focused digest
tests pass. Those two late changes are **root-reported verification**, not
independent passes from this reviewer; `late_root_verification` records the
messages and boundary explicitly.
