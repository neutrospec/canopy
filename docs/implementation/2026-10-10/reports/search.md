# PLAN-09/10/11 구현 보고

V9~V11을 `docs/invariants.md`에 먼저 등록한 상태에서 현재 원문·profile 정합성,
검색 입력/비용 경계, 모델/native 준비와 repair를 구현했다. 역사 감사 파일은 수정하지
않았고 commit/publish는 하지 않았다. 일반·race 회귀, 작은 holdout, toy download/converter,
현재 host의 실제 native compile/load/infer를 실행했다.

## 구현한 계약

- 캐시 schema 3은 parsed body/path/title의 source revision, embedding profile,
  원문 byte range를 저장한다. blank/deleted/edited 페이지의 과거 chunk는 engine 없이도
  제거된다. keyword transaction과 inference는 분리하며 실패한 inference가 옛 근거를
  현재 제목에 붙이지 않는다.
- `EnsureCurrent`/`EnsureCurrentContext`는 shared wiki lease 아래 fresh scan과 FTS를
  게시하고 lease를 풀어 inference한 뒤, 짧은 shared lease에서 파일 revision/loaded
  model profile/CAS를 검증한다. 오래된 supplied scan도 재스캔하며 caller view를 갱신한다.
  마지막 재스캔으로 이전 batch 후에 바뀐 페이지도 무효화한다. 변경 중인 source는
  재색인 오류 또는 engine 없는 partial coverage이며 stale evidence를 반환하지 않는다.
- raw body slice와 title/trim을 포함한 embedding input을 분리했다. 반환 `text`는
  UTF-8 parsed-body의 `[start_byte:end_byte]`와 같고 `source_revision`/`profile`을 포함한다.
  whitespace-only 변경도 옛 offset을 재사용하지 않는다.
- profile은 실제 file metadata/symlink target/작은 manifest, hugot/ORT/output/pooling/L2,
  dimension, chunker/normalization을 식별한다. 같은 이름·차원의 model/tokenizer 교체도
  무효화하며 대형 weight를 query마다 hash하지 않는다. 현재 mean-if-3d를 유지했다.
- k=1..1000, per-page=0..1000(0=cap 해제), vector 개수/차원/finite/nonzero/unit norm
  오차 0.01을 검사한다. 동점은 normalized slug/original slug/seq 순이다. engine gate는
  획득 전후 취소를 확인하고 bounded batch 사이 취소도 검사한다. 실행 중 native의
  즉시 중단, token 기준 truncation 측정 또는 모든 runtime 고장의 fallback은 약속하지 않는다.
- SearchChunks는 최대 k 후보를 page cap과 함께 유지하고 최종 선택 후 raw excerpt를 읽는다.
  shuffled exhaustive oracle로 cap 적용 전 global top-k 절단의 누락을 방지했다.
- cheap model readiness는 nonempty regular/symlink 파일, JSON object, ONNX external-data
  sidecar와 manifest kind/size를 검사한다. inline/custom/int8/legacy를 읽으며 explicit
  verify/adopt가 full digest를 처리한다. pinned pull은 30분 context deadline과 고유 stage를
  사용하고 이전 정상 bundle을 `.previous-*`에 보존한다. 동시 installer는 busy로 거부한다.
- native 준비는 darwin/linux × arm64/amd64의 pinned URL/digest를 선택하고 미지원 host,
  cross-native 또는 지원 밖 Linux libc는 download 전에 거부한다. curl/hash/extract를
  별도로 판정한다. quantizer는 같은 XDG/explicit path와 staging/backup/installer lock을 쓴다.

root 소유 integration은 `main.go`의 search k/EmbedContext/model pull/status/verify,
`config.go`의 unsupported nondefault 경고/effective 값과 web/bridge/recommendation
소비자다. 본 worker는 그 파일을 수정하지 않았다. Makefile은 허가받은 native deps
block만 바꿨다. 세부 계약은 [search-profile](../../../search-profile.md)에 있다.

## 실패 재현과 정상 대조

구현 전 실제 테스트는 blank body 2가지, nil-engine stale evidence, same-dimension
profile 변경의 inference skip, whitespace로 변형된 raw text, inference 실패 뒤 옛 근거,
NaN/Inf/zero/nonunit vector와 k=-1/0/large에서 실패했다. unchanged 재색인 control과
일반 keyword/청크 순위 control은 유지했다. 초기 cmd build는 다른 worker가 바꾸던
SetFrontmatterField 시그니처 때문에 일시 실패했고 그 owner가 고친 뒤 재실행했다.

최종 `go test -race -count=1 ./internal/embed ./internal/indexer ./internal/store
./internal/search ./cmd/canopy`는 다섯 package 모두 통과했다.
[실행 결과](search-tests.txt)에 기록했다. 주요 신규 시험은 다음과 같다.

- `TestReindexClearsEmptyBody`, `TestDegradedIndexDoesNotServeStaleChunks`,
  `TestReindexEmbeddingProfile`, `TestRecallSourceRoundTrip`,
  `TestFailedInferenceExcludesPreviousRevision`, `TestInferenceSourceChangeIsExcluded`,
  `TestPassedSnapshotRescannedBeforePublication`.
- `TestCanceledGateDoesNotStartInference`, `TestEmbeddingInputBudget`,
  `TestModelReadyKindsAndLegacyAdoption`, `TestModelReadinessRejectsIncomplete`,
  `TestModelProfileFileReplacementAndVerify`, `TestModelPullFailureRetryAndConcurrency`,
  `TestDownloadDeadlineCancellationAndLateFailure`.
- `TestVectorValidation`, `TestRetrievalBounds`, `TestVectorCountsAndNearUnitControl`,
  `TestBoundedCandidatesMatchExhaustive`, `TestConcurrentStoreProfileSwapRejectsOldQuery`,
  `TestDeterministicRankingTies`,
  `TestSearchRecallGrepBoundsBeforeWiki`, `TestReconcileExcludesDeletedAndEditedEvidence`.

`python3 scripts/test-native-download.py`의 5개 작은 시험도 통과했다.
[결과](search-download-results.txt)는 complete tar + curl exit 18, hash/extract 실패의
old-library 보존, retry/cache/concurrency, unsupported tuple, custom XDG/explicit path와
실패하는 fake converter 뒤 previous bundle 보존을 포함한다. 큰 모델 변환은 실행하지 않았다.

실제 CGO_ENABLED=0 binary로 disposable wiki에서 hybrid MQTT 검색의 mode=keyword/hit/
exit 0, nondefault dimension/chunk_tokens의 stderr 경고와 single JSON stdout, invalid
search/recall k의 exit 1/빈 stdout, 모델 없는 status의 available=false를 확인했다.
[CLI argv/stdout/stderr/exit](search-cli-results.json)에 있다. owned source의 gofmt 출력은 비었다.

모든 Go/canopy 실행 전에 HOME/XDG_CONFIG/CACHE/DATA/STATE/TMPDIR를
`/private/tmp/canopy-search-20261010`으로 격리했다. GOTELEMETRY=off, GOPROXY=off,
GOTOOLCHAIN=local, GOMODCACHE=/Users/nobocop/go/pkg/mod를 사용했다. 모듈 cache와
기존 model/native 파일은 읽기 전용 재사용했다. actualwiki는 읽거나 쓰지 않았다.

## 고정 holdout 결과

첫 측정 전에 [평가 계약](../../../search-evaluation.md)과 fixture label/acceptance를
작성했다. corpus SHA256은
`8e0e9e907269e5cb87a5a6f9920f59c32c5fca9fb6daacc9c2bba6d11a86d5c0`이다.
6페이지·10질의(8 answerable/2 unsupported), 30 query-mode 결과를 실행했고
[원자료](search-evaluation-results.json)에 모든 hit/Recall@3/nDCG@3/span 수를 공개했다.

| 사전 수용 또는 설명 metric | 실제 결과 |
|---|---|
| literal 5질의 keyword Recall@3=1 | 5/5 pass |
| unsupported 2질의 keyword hit=0 | 2/2 pass |
| semantic 반환 근거의 source byte range 일치 | 30/30 returned range pass |
| 8 answerable semantic/hybrid expected page Recall@3 | 각 8/8, 평균 1.0 |
| 9 supporting span의 semantic/hybrid 반환 coverage | 각 9/9 |
| English paraphrase의 semantic/hybrid nDCG@3 | 0.6309297535714574 (관련 page 2위) |
| unsupported semantic/hybrid 후보 수 | 각 질의 3개, support 0 |

baseline은 기존 unicode61 prefix-AND, 현재 mean-if-3d/L2와 RRF 60이다. 한국어 조사
변형과 두 paraphrase의 keyword baseline은 0 hit였다. 새 언어 엔진/pooling/threshold
튜닝을 하지 않았으며 이 작은 집합을 일반 품질 또는 abstention 보장으로 일반화하지 않는다.

## 성능·native manifest

측정 host는 Apple M5 Max, RAM 36 GiB, macOS 27.0.1(26A434), darwin/arm64,
Go 1.27.1, hugot v0.7.5, tokenizers Go module v1.27.0, onnxruntime_go v1.31.0이다.
실제 로드한 shared library는 설치된 ORT 1.30.0이고 tokenizer archive의 로컬 SHA256은
`c91ae814afb8fe4f000099972208f60c3aa2d13899a7cd5a31fee2e9e2efbac7`이다.
모델은 설치된 read-only legacy external/int8 bundle(193354-byte graph,
567735312-byte data), profile
`b43a213e4e503ce2ad5ce576f5b47cf7b7121a43638b56d2cc156f8ccf1f4c84`이다.
그 설치물의 upstream revision을 이번 실행에서 새로 인증한 것은 아니다.
작은 graph의 SHA256은 `a58799bf3ef61932ee1523a9a9a1de8e84d13fc8dbd6aabb001bfaf31e5d90ab`이며
DynamicQuantizeLinear 96회/MatMulInteger 72회가 있어 int8 graph임을 별도로 확인했다.

| 경계/tuple | 결과 |
|---|---|
| darwin/arm64 ORT test compile | pass (duplicate -ldl linker warning만 있음) |
| 실제 shared library + legacy model load | pass, 마지막 실행 479.78ms |
| 3문장 실제 inference | pass, 마지막 실행 15.73ms, 1024차원·finite·unit |
| official CLS reference parity | unverified; 현재 mean profile 유지 |
| 다른 3개 host tuple native runtime | unverified; pinned preparation만 구현 |
| real quantization / fp32-vs-int8 ranking | unverified; fake converter만 실행 |

[최종 compile/load/infer와 eval 로그](search-native-results.txt)는 작은 inference smoke와
retrieval evaluation을 분리한다. load/infer 시간은 두 번의 짧은 local smoke 경험값이며
warm/cold 우위를 추정하지 않는다. OS cache flush는 하지 않았다.

`BenchmarkRetrievalScale -benchtime=3x -benchmem`은 200/1000페이지 × 5 chunks,
1024차원 unit vector, chunk text 1040 bytes, k=8/cap=2를 사용했다. 모든 vector가 query와
같아 동점이 높은 synthetic 상한 사례다. pair 비교는 실행하지 않았으며 같은 corpus를
bridge에 사용하면 threshold 0.7을 모두 통과하는 pair 밀도 1이다.

| pages/chunks | first query, OS cache 미통제 | 이후 3회 평균 | cumulative B/op |
|---|---:|---:|---:|
| 200 / 1000 | 4.429ms | 4.158ms | 8,571,362 |
| 1000 / 5000 | 30.515ms | 30.090ms | 42,781,109 |

[benchmark 원자료](search-benchmark-results.txt). vector DB kernel과 late excerpt 비용만
포함하며 wiki scan/model load/inference/HTTP는 제외했다. B/op는 누적 allocation으로
RSS/peak heap이 아니다. candidate 저장만 O(k)이고 SQLite의 blob scan allocation까지
O(k)라고 주장하지 않는다. 동시에 작업하는 IDE 환경의 3회 결과를 SLO·speedup이나
과거 paired-scan 자료와 동일 실험의 before/after 비교로 쓰지 않는다.

## 핀 출처와 인계

기본 model repository는 [고정 revision](https://huggingface.co/EmbeddedLLM/bge-m3-onnx-o2-cpu/tree/b85c5597da3ff553300a9b41e781429ec75af15e)이다.
LFS graph/weight/tokenizer SHA256은 upstream model API에서, 작은 JSON의 SHA256은
그 revision의 파일 bytes에서 얻어 `model_pins.go`에 고정했다. 기본 2.3GB bundle은
다운로드하지 않았다. native pin은
[tokenizers v1.27.0 release](https://github.com/daulet/tokenizers/releases/tag/v1.27.0)의
GitHub asset digest를 사용했다.

[테스트한 owned 구현 source SHA256](search-source-manifest.json)을 남겼다.
root는 web API cold/current/helper integration, bridge/recommendation 소비자,
config/effective 표시와 release/test-assets gate의 최종 통합을 별도로 검증한다.
본 worker의 구현과 수용 검사는 완료했으며 ANN/CLS export/전체 플랫폼 runtime/
real quantization은 요구 범위의 완료로 가장하지 않고 명시된 미검증 경계로 남긴다.
