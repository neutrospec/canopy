# Final consolidation challenge — 2026-10-10

This review reconciles the selected minimum improvements with the historical
adjudication and the evidence actually retained. All **14 PLAN mappings, 95
historical finding IDs and 12 final objections** are represented without missing,
duplicate or remapped historical IDs. This is traceability, **not 95 independent
proofs**. Several useful corrections were still missing at the start of this
review; the coordinator corrected them as recorded below. Final source-bound
integration, a cut release, release archives and a new Homebrew installation
remain separate completion claims.

The reviewer authored no production implementation and ran no product tests,
reproduction scripts, HTTP/security probes, downloads, browser sessions or user
wiki/service operations. This is a read-only consolidation of existing source,
contracts and records. Its only execution was a read-only consistency/hash
helper, whose output is [final-challenge-evidence.json](final-challenge-evidence.json)
and whose inert source is [final-challenge-checks.py.txt](final-challenge-checks.py.txt).
These files and this report are the reviewer's entire write scope.

The historical source is `ede67e302a8487aecbb4d59587a8686292018cf6`
(`v0.10.0`). The current worktree is dirty. The JSON records individual current
file hashes and the evidence inputs observed by this review; it does not turn
an earlier execution into an execution of the final tree. The dispatch is
`task_28406e6dd660` / `ctx_67e17ba68a64`, run `run_7eb93ed076b5`.

## Prioritized findings and disposition

Findings were sent immediately through the dispatch receipt before this report.
Message IDs and coordinator replies are preserved in the accompanying JSON.
“Source reviewed” means this reviewer inspected the resulting code and contract;
it does not mean the reviewer independently executed the regression.

| ID | Priority and selected scope | Concrete gap | Disposition and evidence class |
|---|---|---|---|
| FC-01 | P1, STATE-14 / PLAN-07 | `digest.Collect` compared date text lexically, so `z-not-a-date` could count as normal window activity. | Coordinator now parses created/updated independently, excludes missing/invalid fields from window counts, emits stable `invalid_dates` and human diagnostics, and retains total page existence. Valid future dates and all-time decisions retain the existing policy. Source and `internal/digest/dates_test.go` inspected; coordinator reports before-fail/after-pass, not independent forward execution. G4 and the state contract were updated. |
| FC-02 | P2, WEB-08 / PLAN-12 | Unused multipart files were cleaned but accepted, contrary to the selected refusal remedy. The independent initial HTTP record includes such a successful upload. | `requestLimits` now returns 400 before the handler for file parts and defers `RemoveAll`; the fields-only positive control remains. Source and `TestUnusedMultipartFileRefusedBeforeMutation` inspected. The final independent web record additionally verifies file refusal without task/page mutation or spool residue and exact fields-only payload/base preservation (W358–W362). This is a later independent HTTP control, not part of the original four web counterexamples. |
| FC-03 | P2, STATE-11 / PLAN-08 | Future **known-type** task completion had no explicit policy/control, while the `Get` comment claimed a version gate. Unknown-type future tests did not cover this case. | Explicit future-version `done` refusal precedes the verifier; list/dismiss remain tolerant and raw unknown exact values survive. `TestFutureKnownTaskDoneRefusedAndDismissPreservesUnknown`, T5 and the task contract now agree. Coordinator-only regression, source inspected. This is a local task policy, not a new global downgrade architecture. |
| FC-04 | P2, SEARCH-05 / PLAN-09 | G2's title-prefix exception contradicted the exact raw source-range contract. After its removal, the named test still asserted only substring membership, which did not enforce its advertised offsets/revision/profile clauses. | The obsolete exception was removed. After `msg_4ecf810dad13`, the coordinator strengthened `TestRecallSourceRoundTrip` with a Unicode/spacing/multiple-chunk fixture, nonempty valid byte ranges, exact `parsedBody[start:end]`, current page identity, revision and profile. Source inspected; `msg_78721d8df37c` reports a focused pass. This is coordinator regression evidence, not this reviewer's independent execution. |
| FC-05 | P2, evidence provenance across plans | Earlier author manifests and integration results predate late corrections. A report link or an exit-zero run cannot certify changed final source. | Historical outputs remain historical. `pre-final-checks.json` explicitly records `source_unchanged: false` despite seven successful commands, and is superseded rather than presented as a final-tree pass. Coordinator will bind the final source after review reconciliation. The review does not wait for or claim that future run. |
| FC-06 | P3, STATE-15 / PLAN-03 | Source comments still promised unconditional clean cross-device Git merging, contradicting the manual conflict boundary. | Task and activity comments now describe reduced overlap and ordinary Git conflicts/recovery. No distributed convergence or external-editor CAS is promised. Source inspected. |
| FC-07 | P2 question, OPS-08 / PLAN-11 | Native preparation and the Homebrew template used `linux-amd64` versus `linux-x86_64` with the same digest; fake-download tests could not establish that both URLs were real. | [Coordinator-captured official release asset metadata](native-pin-provenance.json) records both names with equal size 14,331,593 and SHA-256 `72556cdca798dd4ea7cdaba308e5f0d68a8cb93b67c96edf485b7a0edd7b07f4`. This resolves the alias question without a source edit. It is metadata provenance, not a Linux download/build/load/infer result. |

No selected source-remedy gap remains open in this bounded consolidation after
the above corrections. This does not mean every environmental acceptance was
executed. The initial missing independent report paths were also reported. Preservation/state
and web reports now exist; search has an explicitly **coordinator-authored**
synthesis. The implementation README and coverage ledger still mark validation
pending at this review's snapshot. Those pending states are not unsupported
completion claims, and should be updated only with the observed final outcomes.

## Every selected PLAN mapping

The [historical plan](../../../review/2026-10-09/plan.md),
[adjudication](../../../review/2026-10-09/adjudication.json), current
[coverage ledger](../coverage.json), contracts and implementation were compared.
Every mapped implementation/evidence path currently exists. The JSON retains an
individual decision/review note for each of the 95 historical IDs, including
rejected or narrowed prescriptions; linking a finding to selected policy does
not mean accepting its original diagnosis, its proposed architecture or every
environmental acceptance case.

| PLAN | Selected remedy inspected | Existing evidence and practical bound |
|---|---|---|
| 01 | Short skill entry/exit contracts; explicit wiki; read fallback; preserved input; partial-result/content judgment; queue versus applied page; whole-wiki sync authority. | [Author workflows](docs.md) plus [one independent blind named-wiki execution and prose review](docs-adversarial.md). Two-wiki/no-model author cases are scripted; one independent execution preserves default A and reports B's hook failure honestly. Neither establishes general LLM success or injection resistance. |
| 02 | Executable invariant predicates, lint presence, first/repeat R1 snapshots, isolated tests/CLI and CI's actual lite path. | Author predicates and CLI records, independent use, current i18n marker consistency, coordinator pre-final checks. Initial module-cache `-mod=readonly` is explicitly weaker than OS write denial. Native, browser and future deployment evidence remain distinct. |
| 03 | Unmerged/in-progress Git checks before commit and after pull/autostash; source/stash retained; no remote publication of unresolved conflict. | [Preservation author matrix](preservation.md) and [independent local bare-remote/two-clone flows](preservation-state-adversarial.md) include dirty conflict, normal sync and rejected push. Literal conflict-marker text is not universally banned. External Git/hook races and distributed convergence are outside the guarantee. |
| 04 | Complete same-directory publication; no-clobber create/archive; canonical cooperating leases; per-file RMW; queue nonce **and** exclusive publication. | Author fault/CLI matrix plus independent complete-reader, disjoint-key, eight-process payload, alias/process-lock and killed-holder controls. Prepublication old bytes and postpublication applied bytes have different oracles. Ordinary mode bits are covered; physical power loss, ACL/xattr/ownership/hardlink identity and global multi-file transactions are not certified. |
| 05 | Applied/partial/unknown stage and repair; orphan visibility; cleanup pending; same-operation checkin resume without reapplying. | Author six-verb outcomes and independent retained-checkin failure/resumption, orphan and changed-input controls. Repair does not imply rollback. The independent reviewer did not independently repeat every author's error/matrix cell. |
| 06 | Governed/flat addressing and ambiguity; resolved rename links versus code; scalar/YAML preservation or prewrite refusal; agent-only rename and taxonomy/page-count semantics. | Author parser/mutation fixtures and independent PSA-02/04/05 address, rename and cleanup counterexamples followed by retests. Dotted/nested scope is explicit. Targeted complex YAML refuses rather than silently rewrites; unrelated fields/comments survive. Historical task/cooldown references and path-keyed checkout namespace remain documented. |
| 07 | Raw unknown/exact-number preservation or refusal; corrupt/partial availability; time instants, duration snooze, known-closed GC; digest date diagnostics. | [State author evidence](state.md) and independent state/process/time/GC controls; FC-01 digest and FC-03 future-known policy are later coordinator regressions. Existing migration rungs are not rewritten. Timestamp snooze requires compatible writers; this change cannot repair old binaries. Event matching history still loads in memory. |
| 08 | Legacy connect-pair identity, accepted payload preservation, POST-time base, mechanical done floor, future-known policy, single outer mutation JSON/stderr/exit. | Author actual-binary mutation/sync matrix and independent collision, full-payload, stale form, repair and hook-failure controls. Empty body plus empty request refuses; request-only works. Viewed-origin snapshots/retention and full deletion proposals were not selected. Bootstrap/argument failures can have empty stdout and nonzero; they do not imply zero side effects. |
| 09 | Current source/revision/profile coverage; empty/deleted/failed-inference exclusion; raw body offsets; exposed actual mode/effective config. | Author indexer/CLI/corpus cases, independent cold keyword/external edits and SA-01 alias post-fix pass, coordinator native rerun. Later SA-02–07 fixes and strengthened G2 check have coordinator regression provenance. Filesystem observation and cooperating locks do not make a global external-editor transaction. |
| 10 | Bounds before expensive work; cancellation gate/batches; exact deterministic ties; bounded retained candidates/excerpts; fixed labeled local evaluation. | Author fake-engine/CLI/tie/corpus and benchmark records; independent HTTP queue/cancellation observations and SA-07 memory counterexample. Coordinator heap regression/exhaustive oracle follows. Pair CPU remains quadratic. B/op is cumulative allocation, not RSS; six pages/ten queries and three benchmark repetitions cannot establish general quality or an SLO. |
| 11 | Legacy/custom/inline/external/symlink readiness; bounded manifests; checked download/preparation; quantizer complete-bundle staging; preserved previous assets. | Author local native load/infer plus the actual post-fix coordinator native log copied into this review JSON. Independent SA-01 passes after fix; SA-02–07 root regression provenance is explicit. Four-tuple pins and metadata are not four-platform execution. Official CLS parity, other native platforms, actual large quantization and fp32/int8 ranking parity remain unverified. |
| 12 | Listener-bound/explicit Host, auth and Origin distinctions, cumulative route caps/parse 4xx, bounded search queue, unused upload refusal, raw HTML and logout. | [Web author](web.md) and [independent actual loopback HTTP review](web-adversarial.md). WA-01/02 uncovered real malformed-query/unread-body gaps; their initial failures and passing same-reproduction retests remain preserved. FC-02 also has a later independent HTTP control. Host allowlisting does not turn a public loopback reverse proxy into authenticated TLS. No browser security execution is claimed. |
| 13 | Truthful unread/recommendation/partial-state distinctions, full queue payload and receipt, POST-time base, exact read success, nearby normal-form actions and fallback structure. | Independent HTTP responses and shipped-script Node adapters, including `read:true` versus false/HTML/redirect/401/network failure. These prove response and logic behavior within the fixture. Real paint, focus, keyboard order, IME, U5 usability and comparative layout effectiveness remain unverified. |
| 14 | Parser-aligned code/Mermaid/raw ranges; declared local math packages; local routes, content-hashed app assets, pinned notices and packaging declarations. | Author and independent HTML/range checks, actual Node TeX compilation with valid/invalid controls, hash/notice fixtures and synthetic archive checks. WA-03/04 concern preview addressing/message fallback, with independent dispositions in its report. No real offline browser render, actual release archive or new Homebrew install is established by these checks. |

## Every final historical objection

| ID | Current adjudication preserved |
|---|---|
| FINAL-01 | One-day immediate expiry is repaired with `deadline > now`; duration versus calendar and legacy interpretation are explicit. The historical seven-day positive control is not rewritten as a failure. |
| FINAL-02 | Prepublication old-hash preservation differs from postpublication applied/current-hash/stage/repair truth. A partial write is not mislabeled unchanged. |
| FINAL-03 | Five-key presence is lint policy, with scalar/null/empty tags tolerated. It is not an unselected universal hard gate or retrospective consensus. |
| FINAL-04 | One outer object, stderr/exit, bootstrap empty-stdout exception and consumer transition are documented. Six-verb author cells and impossible N/A cases are not replaced by independent spot checks. |
| FINAL-05 | Disjoint-key union, conflicting whole-body replacements and accepted-task payloads have distinct oracles. Canonical local cooperating locks do not protect direct editors, external Git/hooks or other machines. |
| FINAL-06 | Same-version and future top/nested unknown and exact numbers are protected or refused. Partial healthy read data, post-apply state failure and TOML formatting are separate claims. |
| FINAL-07 | Nondefault unsupported embedding settings warn on stderr and expose actual effective units/profile. No vector slicing or token-to-rune relabeling was introduced. |
| FINAL-08 | Existing paths, truthful counts/payload/receipt and POST base are the minimum. New filters, historical viewed origin and broad browser/layout effectiveness claims remain deferred or unverified. |
| FINAL-09 | Backup precedes the consuming checkin. Independent B use retains input and distinguishes content applied from failed commit/push; `show` existence is not intent satisfaction. |
| FINAL-10 | The historical checkin timing record remains actual CLI evidence, with its direct-editor timing limit. This review did not repeat it or recast it as an internal-only call. |
| FINAL-11 | Fixed labels/rules/baseline and actual local results are kept separate from smoke, parity, performance and environmental claims. No universal statistical/platform/browser release gate is invented. |
| FINAL-12 | The approved minimum remedies are retained without turning all 14 into a serial gate or adding UUID/DB/CRDT/ANN/rollback/origin/full-platform architecture. |

## Evidence attribution and stale-source limits

The consistency helper compares recorded author source manifests against current
bytes. At its first run, **10/20 search, 4/17 state, 15/32 preservation and 4/40
web entries** differed. The JSON preserves actual paths and both digests; the
ratios describe drift, not defects or test failure counts. Later corrections are
not retroactively attributed to the authors' earlier execution. A partial author
manifest also cannot bind every shared dependency or toolchain.

The independently authored preservation/state report records PSA-01–06 failures
and same-reproduction post-fix controls. Its actual git conflict/push, lock,
retained-checkin, queue/state/address/GC observations are stronger than source
inspection for those cases, but are not the author's entire six-verb fault
matrix. Its final binary/hash remains that review's binary, not a future release.

The docs blind execution is one real named-B task. It was frozen separately from
the later prose corrections and positive controls. Its initial build did not
apply OS write denial to the module cache; a later sandboxed rebuild cannot
retroactively give that initial execution stronger isolation. The report states
this limit. The author's 17 scripted workflows and this single independent task
are useful different evidence, not a general effectiveness study.

The search independent worker retained raw JSON and inert Go sources but ended
after a provider interruption without a final report or `worker_done`. The
[search synthesis](search-adversarial.md) is written by the **coordinator** and
says so. SA-01 has an independent post-fix pass in the third raw record.
SA-02–07 retain their raw independent failures and have subsequent **coordinator
regressions**, not independent acceptance. `[no tests to run]` is not a package
pass. No refused action or reproduction script was rerun in this consolidation.

The web independent record preserves the initial HTTP/Node/TeX observations and
four counterexamples, all passing their same repros after correction, plus the
later multipart policy control. Its final binary is
`a934ca8c1395a1e35dae373557f02cc438f5c7396a4e8293093eda24791a4db2`;
the initial frozen JSON and final append-only sequence are separately hashed in
this review's evidence. These dispositions apply to the stated binary and
source snapshot. HTTP/HTML output, shipped-script Node
adapters and real Node TeX compilation are different checks. None establishes a
browser's paint, session interaction, focus, keyboard/IME or offline rendering.
The initial successful unused multipart upload remains historical cleanup
evidence; W358–W362 supplies the later independent refusal/fields-only control.
Same-origin binary replacement also exercised app URL/ETag revalidation at HTTP
level (W324–W333), without establishing a real browser upgrade or offline reload.

The latest actual native rerun supplied to this task is
`/private/tmp/canopy-improve-20261010/native-final.txt`; its complete text and
SHA-256 are copied durably into `native_existing_log` in this review's JSON.
It records actual ORT compilation, model load and inference, then the bounded
six-page/ten-query retrieval corpus. Load/infer pass with three texts and 1024
dimensions, profile
`3b45cea3fbf1b079a527f47a79ee3da1774fcb62334efa9750c4498fbe224be2`.
The task identifies isolated HOME/XDG and OS deny-write to model/module inputs;
the terminal transcript alone does not encode those sandbox settings.

That native run uses the installed legacy external bundle. It does not establish
official CLS reference parity or a freshly quantized model. The evaluation has
fixed local labels: unsupported semantic queries still return nearest candidates;
C++ and C are the same unicode61 keyword token; hybrid span support is the
semantic top-3 intersected with hybrid-selected pages. These are not independent
hybrid chunk ranking, punctuation recall, abstention or general quality proofs.
The separate 200/1000-page vector microbenchmarks exclude wiki scan/load/native
inference/HTTP; three repetitions and allocation totals do not establish RSS,
end-to-end latency or a speedup claim.

## Remaining bounded acceptance and delivery claims

The selected source remedies and their bounded evidence must remain distinct
from the following unobserved outcomes:

- **Final-source integration:** the current `pre-final-*` run passed commands
  while files changed and explicitly says so. An unchanged-source final ledger,
  then the committed release check, belongs to the coordinator's later record.
- **Browser:** paint, responsive layout visibility, focus/keyboard order, IME,
  U5 usability, real graph/Mermaid/math rendering and real offline behavior remain
  unverified. Static routes, a fallback's presence and Node math compilation
  cannot substitute for them.
- **Model/native:** official CLS reference parity, the other three native tuples,
  full large-model quantization and fp32/int8 ranking parity remain unverified.
  A local native pass and pinned alias metadata do not prove these outcomes.
- **Release/distribution:** no release was cut at this review. CI/workflow
  declarations, `goreleaser check`, Ruby syntax, license fixtures and synthetic
  archives do not establish the future tagged release, actual archive contents,
  published binary or a new Homebrew build/install/test. An already installed
  v0.10.0 formula is baseline evidence only.

These are actual limits on the completion claim, not a proposal for a new probe
wave or a new architecture program. The selected incremental work can be
reported with its bounded passes and explicit unverified outcomes; it should not
be summarized as “95 findings independently proven” or “all environments passed.”
