# 검색·회수·의미 후보 독립 감사 — v0.10.0

기준 커밋: `ede67e302a8487aecbb4d59587a8686292018cf6` (2026-10-09). 개인 지식 시스템에서 중요한 것은 가까운 문서를 찾는 능력과 **현재 원문에서 답의 근거를 회수하는 능력**을 구분하는 것이다. 이 구현은 CLI keyword 자가 갱신, 로컬 임베딩, 페이지당 recall 상한, 명시 링크와 의미 후보의 분리를 이미 갖췄다. 그러나 삭제한 본문을 다시 근거로 반환하는 경로와 현재 원문에 과거 벡터 점수를 붙이는 경로가 실제로 존재한다. 이 두 종류의 정합성부터 고친 뒤 검색 품질을 평가하는 순서가 타당하다.

이 문서는 검토와 제안만 담는다. 기존 추적 파일 수정, 구현 수정, 커밋, 네트워크 모델 다운로드·양자화, 서비스 재시작, 실제 위키 접근은 하지 않았다. `AGENTS.md`, `docs/philosophy.md`, `docs/invariants.md`를 먼저 읽었고, 다른 영역 감사 보고서를 읽지 않았다. 추가 evidence 파일은 조정자의 `msg_d8acae4ddb4e` 지시에 따라 허용된 범위에 보존했다.

## 판단 기준과 재현 범위

- **재현 결함**: 현재 코드를 격리 fixture와 공개 내부 API/HTTP handler로 실행하여 기대와 실제의 차이를 관측했다.
- **정적 계약 불일치**: 고정 의존성 코드·실제 모델 메타데이터·공식 계약이 다르다. 실제 추론 품질 감소량까지 측정했다는 뜻은 아니다.
- **조건부 방어 부족**: 잘못된 벡터/인자가 들어왔을 때의 실패를 재현했다. 정상 ORT가 그 입력을 실제로 만든다는 주장은 하지 않는다.
- **평가 공백/제품 선택**: 정책이 없거나 품질을 뒷받침하는 평가가 없다. 개인 위키에서 반드시 나쁜 결과가 난다는 판정과 구분한다.

심각도 P1은 현재 내용과 근거의 연결이 끊기거나 임베딩 공간을 잘못 쓰는 경우, P2는 특정 입력·규모·실패에서 검색/회수의 가용성·상한·품질 판단이 흔들리는 경우, P3는 재현성/주장 근거 보강이다. 아래 ID는 후속 수정·반박에서도 유지한다.

| ID | 등급·판정 | 핵심과 전제 |
|---|---|---|
| SEARCH-01 | P1 · 재현 | 이미 임베딩한 페이지의 본문을 비우면 성공한 reindex/recall 경로에도 옛 청크가 남음 |
| SEARCH-02 | P1 · 재현 및 일부 정적 추적 | cold API, 외부 편집/삭제, nil-engine/실패 갱신 뒤 캐시 소비자가 신선도를 보증하지 않음 |
| SEARCH-03 | P1 · fake engine 재현 | 모델/임베딩 프로필 변경 시 텍스트가 같으면 기존 벡터를 재사용 |
| SEARCH-04 | P1 · 정적 계약 불일치 | 확인한 3차원 `last_hidden_state` 모델은 hugot mean pooling을 거침; BGE-M3 공식 dense 계약은 CLS |
| SEARCH-05 | P2 · 재현 | 청크 생성이 문단별 공백·들여쓰기를 바꿔 G2의 원문 부분 문자열 조건을 깨뜨림 |
| SEARCH-06 | P2 · 조건부 방어 부족 재현 | 차원 차이를 조용히 자르고, 비정상 norm/NaN을 수용하며, 혼합 차원에서 panic |
| SEARCH-07 | P2 · 인자 경계 재현 | `top-k <= 0`의 모드별 동작이 달라 recall 상한 해제·semantic panic 가능 |
| SEARCH-08 | P2 · HTTP 재현, CLI 정적 추적 | 초기화 실패만 hybrid 강등; 런타임 추론 실패는 keyword 결과까지 실패시킴 |
| SEARCH-09 | P2 · 취소 재현/입력 상한 정적 추적 | 취소된 검색도 모델을 실행하며 mutex를 점유; 배치·질의·제목 예산이 없음 |
| SEARCH-10 | P2 · 제품 선택/평가 공백 | prefix FTS의 다국어 한계, 무응답/근거 충족 정책, RRF/청크/후보 임계값의 평가가 부족 |
| SEARCH-11 | P3 · 재현 | semantic/RRF 동점은 map 순회 순서에 따라 결과·경계 항목이 변함 |
| SEARCH-12 | P2 · 조건부 자원 사용 측정 | top-k가 작아도 전체 청크/bridge 후보를 모음; 고유사도 1,000페이지 bridge는 약 450 MiB 할당 |
| SEARCH-13 | P3 · 평가 근거 공백 | README 속도·양자화 품질 수치의 재현 조건이 부족하며 벡터 cosine은 검색 품질 보증이 아님 |

### 읽은 파일과 사각지대

| 영역 | 읽은 파일/범위 | 검증 방법·제한 |
|---|---|---|
| 저장소 | `internal/store/store.go`, `vectors.go`, `store_test.go` 전부 | FTS, 벡터, schema reset, cap, 비정상 입력 직접 실행; SQLite 손상 복구·동시 다중 프로세스 race는 미실행 |
| 인덱싱·청크 | `internal/indexer/indexer.go`, `internal/embed/{engine.go,engine_stub.go,engine_ort.go,chunk.go}` 전부 | deterministic unit engine으로 변경/무변경/빈본문/모델교체/실패 확인; 실제 ORT 추론은 하지 않음 |
| 검색 융합 | `internal/search/search.go`, `search_test.go` 전부 | RRF 산술 기존 테스트와 동점 반복 재현; 자연 질의 ranking benchmark 없음 |
| CLI | `cmd/canopy/main.go`의 resolve/refresh/search/reindex/model/download, `recall.go`, `resurface.go`의 bridge, `write.go`의 관련 추천/afterWrite/sync, `reconcile.go`의 enrichForeign | lite CLI와 fake-vector bridge를 실제 실행; ORT CLI 런타임 추론 실패는 코드 추적만 함 |
| 원문·설정·쓰기 | `internal/wiki/page.go`, `internal/config/config.go`, `internal/writeops/writeops.go` | governed page scan, slug 조인, DB 키, 프로필 설정 소비 확인; git sync는 실행하지 않음 |
| 의미 발견 통합 | `internal/webui/server.go`의 server/search/API, `suggest.go`, `discover.go`, `browse.go`의 islandReport, `speak.go`의 daily bridge; `internal/resurface/{pick.go,state.go,pick_test.go}` | 실제 HTTP handler를 httptest로 호출; 새발견 affinity·섬·daily 후보의 신선도는 소비 경로 정적 추적, 각 화면의 ranking 변경을 따로 실측하지 않음 |
| 웹 검증 | `internal/webui/static/app.js` 자동완성, `workspace_test.go`, `tasks_test.go` 관련 테스트 | client AbortController와 server 취소 경계 비교; 브라우저 end-to-end/네트워크 연결 해제는 미실행 |
| 설계·주장 | `AGENTS.md`, `docs/philosophy.md`, `docs/invariants.md`, README 성능 절, `scripts/quantize-model.py` | C4/C5/F2/G1–G3/E2/G6와 대조; 기존 벤치 원자료를 발견하지 못함 |
| 모델·의존성 | 로컬 bge-m3·fp32 backup의 작은 ONNX graph/config/tokenizer config; `hugot@v0.7.5/pipelines/featureExtraction.go`, `backends/tokenizer_rust.go` | 읽기 전용 메타데이터 검사; 외부 weight 파일 로드·hash·추론·tokenizer 실행 없음; BAAI/FlagOpen 공식 pooling 자료만 웹 확인 |

실제 위키 `/Users/nobocop/workspace/wiki`는 열지 않았다. 모든 fixture와 binary는 `/private/tmp/canopy-audit-20261009/search`, Go cache는 `/private/tmp/canopy-review/go-cache`, `GOPROXY=off`를 사용했다. CLI subprocess의 HOME/XDG를 fixture 경로로 격리했다. 실제 HOME/XDG 내용은 수정하지 않았다. 기존 모델은 허용된 읽기 전용 검사만 했다. 시간 측정 환경은 macOS 27.0.1 arm64, Apple M5 Max (`Mac17,7`), RAM 36 GiB, Go 1.27.1이며 모듈 선언은 Go 1.26.4다. 다른 감사 작업이 같은 머신에서 진행되어 완전히 통제된 성능 환경은 아니다.

### 재현 명령과 원자료

전체 소스는 비빌드 텍스트로 보존했다: [main probe](evidence/search-main.go.txt), [synthetic benchmark](evidence/search-bench.go.txt), [실행기](evidence/search-run.py.txt), [모델 metadata 검사기](evidence/search-model-inspect.py.txt). 실행기는 실제 위키 없이 fixture를 새로 쓰고, 잠시 생성한 `review_probes/search`를 `finally`에서 제거한다. 같은 경로의 fixture는 덮어쓰므로 이 감사 전용 경로만 사용해야 한다. 기존 `review_probes/search`가 있으면 덮어쓰지 않고 중단한다.

```bash
cd /Users/nobocop/play/canopy
python3 docs/review/2026-10-09/evidence/search-run.py.txt
# 각 argv/exit/stdout/stderr: /private/tmp/canopy-audit-20261009/search/<label>.json
# 메타데이터만 읽기; 아래는 해당 로컬 모델이 있을 때만 실행
python3 docs/review/2026-10-09/evidence/search-model-inspect.py.txt
```

probe exit 0은 **관측을 정상 수집했다**는 뜻이다. 발견 결함이 해결됐다는 테스트 통과가 아니다. 원래 관측은 [probe stdout](evidence/search-probe-output.txt), [handler stderr](evidence/search-probe-stderr.txt), [benchmark 측정](evidence/search-benchmark-output.txt)에 있다. 보존한 실행기를 실제로 재실행해 전체 예상 exit와 cleanup을 확인한 [argv/exit/stdout/stderr 기록](evidence/search-replay.jsonl.txt)도 있다. 재실행의 주요 probe stdout은 원래 기록과 일치했다. 각 항목의 `empty`, `api_cold` 등 이름은 stdout의 레코드 이름이다. 시간·동점 순서 집합은 달라질 수 있지만, 원문 잔존·미무효화·HTTP 500·취소 무시는 동일하게 검증할 수 있다.

기존 테스트를 cache 없이 다음 명령으로 실행해 전부 통과했다.

```bash
GOCACHE=/private/tmp/canopy-review/go-cache GOPROXY=off go test -count=1 \
  ./internal/store ./internal/indexer ./internal/embed ./internal/search \
  ./internal/resurface ./internal/wiki ./internal/reconcile ./internal/webui ./cmd/canopy
```

`store`, `search`, `resurface`, `wiki`, `reconcile`, `webui`는 `ok`였다. `indexer`, `embed`, `cmd/canopy`는 `[no test files]`였다. 이 감사 자체는 `-race`나 ORT tagged test를 실행하지 않았다. 다음 제안 테스트 이름은 **아직 없는 후속 수용 테스트**이며, 현재 통과한 테스트 목록과 혼동하면 안 된다.

## SEARCH-01 — 빈 본문이 과거 근거를 계속 반환한다

**P1, 재현 결함.** 이전에 하나 이상 청크가 저장된 페이지를 남겨둔 채 본문을 빈 문자열/공백만으로 바꾸는 것이 전제다. [indexer.go](../../../internal/indexer/indexer.go#L49) 49–58행은 slug를 `live`에 넣고, Split 결과가 0이면 기존 청크 삭제 없이 `continue`한다. 83행 PruneChunks는 살아 있는 slug를 제거하지 않는다. [recall.go](../../../cmd/canopy/recall.go#L39) 39–48행은 refresh 뒤 이 청크를 조회한다.

`empty` 재현: `Old factual claim.`을 unit vector로 색인 → 무변경 재색인 → 같은 파일의 본문을 `""`로 저장 → engine을 주어 다시 Reindex → SearchChunks. 실제 현재 본문은 빈 문자열인데 아래를 얻었다.

```text
body="", embedded_pages=0, pruned_pages=0, total_chunks=1
text="Alpha\n\nOld factual claim.", score=1, rel_path="concepts/alpha.md"
```

기대는 청크 0개 또는 명시된 title-only 정책이다. 실제는 **성공한 semantic refresh 뒤에도 제거한 주장을 인용 가능 근거로 반환**한다. G1의 페이지 실존 확인만 통과하고 G2는 실패한다. 실제 ORT CLI recall 호출은 하지 않았지만, 동일 indexer→store 경로를 유효한 deterministic engine으로 실행했다.

가장 강한 반론은 빈본문 페이지가 lint 대상이고 정상 쓰기에서 드물다는 것이다. 그러나 검색은 외부 변경도 읽으며, blank page가 거부되는 조건과 관계없이 현재 내용 밖의 주장을 근거로 재사용하면 안 된다. **제안**: 0청크는 해당 slug의 청크를 빈 집합으로 원자 교체한다. 대안은 본문 없는 페이지를 검색에서 제외하되 상태를 명시하는 것이며, title-only embedding을 선택해도 옛 본문 보존은 허용하지 않는다.

**제안 불변식 SEARCH-01-I**: 성공한 refresh 후 모든 청크의 원문 부분이 그 snapshot의 페이지 본문에 존재하며, 본문이 비면 본문 청크가 0이다. 새 `TestReindexClearsEmptyBody`를 `go test ./internal/indexer -run '^TestReindexClearsEmptyBody$'`로 점검한다. 일반 빈 문자열/공백/삭제, G1과 G2를 함께 검사하고 재실행 no-op도 확인한다.

## SEARCH-02 — 소비 경로별 cold-start와 신선도 계약이 다르다

**P1, 재현 결함과 정적 통합 확인.** 다음 경로는 하나의 freshness 문제지만 전제를 따로 봐야 한다.

| 경로 | 근거 | 기대와 실제 |
|---|---|---|
| API cold cache | [server.go](../../../internal/webui/server.go#L452) 452–468, 559–574; [serve.go](../../../cmd/canopy/serve.go#L45) 45행; NewServer 54–76행 | 새 캐시의 즉시 검색이 현재 페이지를 찾거나 준비 중을 알려야 함. API는 `refresh=false`; startup도 초기 색인을 하지 않아 정상 200/빈 결과 |
| API 외부 편집·삭제 | 같은 runSearch 455행의 Scan, 498–506행 결과 조립 | Scan은 새 파일을 읽지만 기존 FTS 결과를 교체/실존 필터하지 않음. 옛 제목·snippet과 삭제한 페이지를 반환 |
| nil-engine refresh | [indexer.go](../../../internal/indexer/indexer.go#L37) 37–43행 | FTS는 새로 만들지만 청크 검증/삭제/prune 전에 반환. `noengine`에서 새 본문에 옛 text; `deleted`에서 JOIN 조회 0, PageVectors 1 |
| 중간 embedding 실패 | indexer 37, 74–79행; [store.go](../../../internal/store/store.go#L92) 92–120행 | FTS metadata transaction은 이미 commit. `partial_failure`는 새 title/옛 청크 title+본문 조합을 남김. CLI search/recall은 오류를 반환하므로 이 자체가 성공 응답은 아님 |
| CLI bridge | [resurface.go](../../../cmd/canopy/resurface.go#L165) 165–182행 | 현재 Scan에 기존 PageVectors를 붙이며 refresh/hash 검사 없음. 실제 lite CLI가 수정한 B의 현재 excerpt에 옛 `similarity=1`을 붙여 exit 0 |
| 관련/발견 후보 | [suggest.go](../../../internal/webui/suggest.go#L32) 32–77행; [discover.go](../../../internal/webui/discover.go#L31) 31–73행; [browse.go](../../../internal/webui/browse.go#L241) 241–275행; [speak.go](../../../internal/webui/speak.go#L62) 62–68행 | engine 없이도 캐시 vectors를 읽어 의미 후보/affinity를 생성. 현재내용 검증은 없고 각 화면별 영향량은 미실측 |
| new/reconcile | [write.go](../../../cmd/canopy/write.go#L317) 317–339행; [reconcile.go](../../../cmd/canopy/reconcile.go#L187) 187–238행 | new 추천은 afterWrite 전 cached search라 cold cache/외부 변경 영향을 받음. reconcile nil-engine 경로는 PageVectors에 남은 삭제 slug까지 duplicate 후보에 넣을 수 있음; 이 CLI 두 경우는 정적 추적만 함 |

`api_cold`는 `oldneedle present`가 있는 위키에 NewServer(nil)를 생성한 뒤 첫 API 호출에서 `status=200, results=[]`였다. `/search?q=oldneedle` 호출 후 API는 `Old title`을 찾았다. 파일을 `New title/newneedle present`로 직접 바꾸면 `oldneedle`은 여전히 옛 snippet으로 발견되고 `newneedle`은 0건이었다. 파일을 지워도 옛 hit가 남았다. 이 재현은 fake model도 필요 없다. cold-start 뒤 정확한 slug로 `/search`하면 528–530행의 페이지 redirect가 refresh를 우회하므로 모든 전체 검색이 초기화를 보장하는 것도 아니다.

`seed-bridge`로 같은 본문의 두 페이지를 `[1,0]`으로 색인하고 B 본문을 바꾼 뒤 `canopy-lite bridge --peek --json --wiki .../cli-bridge`를 실행했다. exit 0, `linked=false`, `similarity=1`, B excerpt는 `Changed independent current material.`이었다. 이것은 새 내용이 의미상 반드시 낮은 유사도여야 한다는 실험이 아니다. **점수 계산에 쓰인 텍스트가 현재 excerpt와 다름**을 보인 실험이다.

가장 강한 반론은 API는 속도를 위해 refresh를 생략하고 bridge는 `reindex`를 전제하는 설계라는 것이다. 그 선택 자체는 가능하다. 문제는 stale/missing coverage를 결과에 표시하지 않아 “찾지 못함”과 “준비 안 됨”, 현재 판단과 과거 판단이 구분되지 않는다는 점이다. nil-engine에서 기존 vectors를 보존하는 것도 유용하지만 유효한 evidence로 소비할 근거는 별도로 필요하다. 신선한 CLI keyword/search·engine recall의 일반 경로가 정상이라는 사실은 이 소비 경로의 보증으로 확장되지 않는다.

**제안**: page content revision과 embedding revision/profile을 저장해 현재 scan과 불일치한 청크를 싸게 무효화하고, API의 첫 사용은 최소 FTS bootstrap을 보장한다. 모든 cached consumer에 같은 valid-vector view를 제공하고 `semantic_ready`, coverage/needs_reindex를 드러낸다. FTS와 vectors를 전부 하나의 장기 transaction으로 묶어 모델 추론 동안 lock을 잡을 필요는 없다. 페이지별 revision gating 또는 새 generation을 준비하고 공개하는 경계로 정합성을 만들 수 있다. 대안은 API의 TTL+명시적인 stale 표시와 bridge의 강제 `reindex` 요구다. TTL만으로 즉시 삭제 근거 반환을 정당화할 수는 없다.

**제안 불변식 SEARCH-02-I**: 인덱스가 없거나 현재 원문과 다른 경우 정상 빈 검색/현재 의미 근거로 위장하지 않는다. 새 `TestAPISearchColdAndChanged`, `TestDegradedIndexDoesNotServeStaleChunks`, `TestBridgeChecksRevision`으로 cold/cache삭제/본문·제목편집/파일삭제/rename/engine실패/부분갱신을 점검한다. 명령은 각각 `go test ./internal/webui -run '^TestAPISearchColdAndChanged$'`, `go test ./internal/indexer -run '^TestDegradedIndexDoesNotServeStaleChunks$'`, `go test ./cmd/canopy -run '^TestBridgeChecksRevision$'`이다. 두 process의 새/옛 scan 교차 commit도 후속 테스트에 넣되, 이번 감사는 그 race를 관측했다고 주장하지 않는다.

## SEARCH-03 — 벡터 캐시가 임베딩 프로필을 식별하지 않는다

**P1, 유효한 fake engine으로 재현.** [indexer.go](../../../internal/indexer/indexer.go#L45) 45–65행의 재사용 조건은 title+본문 청크 hash 목록뿐이다. [store.go](../../../internal/store/store.go#L43) 43–50행 chunks는 hash/text/vector만 저장한다. [main.go](../../../cmd/canopy/main.go#L82) 82–84행은 wiki model directory를 engine에 적용하지만 indexer에 모델 identity를 전달하지 않는다. `SchemaVersion="2"`는 레이아웃 번호이지 임베딩 공간의 버전이 아니다.

`model` 재현은 engine A가 `[1,0]`, B가 `[0,1]`을 내는 같은 2차원 unit engine이다. A로 저장한 뒤 wiki의 model을 B로 바꾸고 B로 reindex했다. 실제 `new_engine_calls=0, embedded_pages=0`, B query `[0,1]`의 저장 청크 점수는 0이었다. 기대는 프로필 변경 시 B의 벡터로 재계산하여 점수 1이 되거나, 준비 전 비교를 거부하는 것이다. 임의의 차원 손상 없이도 발생한다. 실제 fp32→int8의 순위 변화량은 미측정이다.

`config.Embedding.Dimension`, `ChunkTokens`는 [config.go](../../../internal/config/config.go#L106) 106–109행에 선언·직렬화되지만 소비처가 없다 (`rg -n 'Dimension|ChunkTokens' internal cmd`). `chunk_tokens_config`에서 1로 바꿔도 청크/engine 호출은 바뀌지 않았다. 이 두 설정을 현재 지원하는 기능으로 오해할 여지가 있다. 반면 title 변경과 비어 있지 않은 body 변경은 실제 재임베딩했고 무변경은 skip했다. 해시 충돌을 발견했다는 주장도 하지 않는다.

model 관리 경로의 한계도 profile 식별과 관련된다. [engine.go](../../../internal/embed/engine.go#L42) 42–49행의 `ModelAvailable`은 `model.onnx` 한 경로의 존재만 확인한다. [main.go](../../../cmd/canopy/main.go#L487) 487–495행 pull은 파일이 있으면 skip하고 `/resolve/main/`에서 개별 파일을 가져오며 revision/checksum manifest가 없다. 580–602행 download는 `.part` 후 rename을 사용한다. 이 마지막 방식은 한 파일의 부분 다운로드를 최종 파일로 노출하지 않는 장점이 있다. 하지만 여러 파일의 동일 revision, 기존 파일의 무결성, tokenizer/weight 호환성을 보증하지는 않는다. 실제 다운로드·손상 모델 실패는 실행하지 않았으므로 이는 acquisition 검증 공백이며 재현된 model 변경 결함과 구분한다. `model status`의 available을 성공한 inference 검증으로 해석하지 않는 것이 맞다.

가장 강한 반론은 양자화 전후 공간이 근사적으로 같고 사용자가 새 모델 때 캐시를 지울 수 있다는 것이다. 근사 재사용을 명시적으로 승인한 프로필은 허용할 수 있지만 임의 모델·tokenizer·pooling·chunker 변경에도 같은 규칙을 적용하면 안 된다. 같은 디렉토리 이름으로 model 파일을 교체하는 경우도 이름만으로 해결되지 않는다.

**제안**: 모델 revision/weight manifest, tokenizer, output/pooling/normalization, dimension, chunker revision을 묶은 embedding profile ID를 캐시에 기록한다. 실행마다 2GB를 hash할 필요는 없다. 설치/변환 시 manifest를 만들고 불변 revision을 사용하며, 검증 실패 시 stale로 처리한다. 프로필 변경은 파생 캐시 무효화이므로 머신 상태 migration ladder에 넣지 않는다. 대안은 명시적인 “이 두 profile은 호환” 정책+품질 검증이며, 기록된 호환성만 허용한다. 사용하지 않는 두 설정은 구현하거나 지원하지 않음을 명확하게 거부/표시하는 것이 작은 해결이다.

**제안 불변식 SEARCH-03-I**: 검색 query와 비교되는 모든 vector의 effective profile이 같고, profile 변화에서 무변경 text skip을 하지 않는다. `go test ./internal/indexer -run '^TestReindexEmbeddingProfile$'`에 same-dim A/B, dimension 변경, tokenizer/pooling 변경, 파일 교체, 무변경 프로필 no-op을 넣는다. `go test ./internal/config -run '^TestEmbeddingSettingsHaveEffectOrReject$'`는 지원되지 않는 dimension/chunk_tokens 값의 명시된 정책을 검사한다.

## SEARCH-04 — BGE-M3 dense pooling 계약과 현 engine 경로가 다르다

**P1, 정적으로 확인한 계약 불일치; end-to-end 품질 손실 미측정.** [engine_ort.go](../../../internal/embed/engine_ort.go#L71) 71–75행은 `WithNormalization()`만 주고 FeatureExtractionPipeline을 만든다. 고정 의존성 `hugot@v0.7.5/pipelines/featureExtraction.go` 119–135행은 첫 model output을 선택하고, 223–237행은 3차원 token output을 mean pooling한다. L2 정규화(243–247행)는 pooling 선택을 CLS로 바꾸지 않는다. [hugot v0.7.5 원본](https://github.com/knights-analytics/hugot/blob/v0.7.5/pipelines/featureExtraction.go#L223)도 같은 경로다.

확인한 두 로컬 ONNX graph의 유일 output은 `last_hidden_state [batch_size, sequence_length, 1024]`이다. 작은 graph만 읽어 다음을 관측했다.

| 모델 directory | graph SHA-256 | metadata |
|---|---|---|
| `bge-m3` | `a58799bf3ef61932ee1523a9a9a1de8e84d13fc8dbd6aabb001bfaf31e5d90ab` | `DynamicQuantizeLinear=96`, `MatMulInteger=72`; 실제 int8 graph |
| `bge-m3-fp32.bak` | `ef046f21faf5aa86bcff05b6fb25c34ef9b163ee86053c147c6434160f13a914` | quantized op 없음; 같은 3차원 output |

두 config는 `XLMRobertaModel`, hidden size 1024, max positions 8194이고 tokenizer config의 model maximum은 8192다. [실제 metadata 출력](evidence/search-model-output.txt)을 보존했다. 검사기는 external weights를 읽지 않으므로 이 hash는 모델 전체 identity가 아니다. 디렉토리 이름 `bge-m3`만 보고 fp32라고 부르지 않은 이유도 metadata다.

BAAI가 배포한 [BGE-M3 pooling config](https://huggingface.co/BAAI/bge-m3/blob/main/1_Pooling/config.json)는 CLS 사용과 mean 미사용을 지정한다. [FlagOpen 공식 dense 설명](https://github.com/FlagOpen/FlagEmbedding/blob/master/docs/source/bge/bge_m3.rst)은 첫 특수 token hidden state를 정규화하는 계약을 설명한다. 따라서 **확인한 graph를 이 engine에 넣는 경우 mean-normalized embedding을 만든다는 코드 추론**은 명확하다. 평균을 사용하는 vector가 무조건 무용하다거나 특정 질의 recall이 몇 % 떨어진다고 말할 근거는 없다. 다운로드 원격 `main`의 현재 export를 가져오거나 ORT를 실행하지 않았으므로 모든 미확인 export에 적용되는 실측 판정도 아니다.

가장 강한 반론은 query와 corpus 모두 mean을 쓰므로 일관된 공간이고 자체 위키에서 충분할 수 있다는 것이다. 일관성은 맞지만 학습·공식 평가의 dense embedding 계약과 다르며, BGE-M3 이름만으로 성능/0.7·0.8 threshold를 가져올 수 없다. **제안**: 첫 token 선택+정규화의 2차원 output을 갖는 작은 ONNX wrapper/export 또는 명시적인 CLS 지원 경로를 사용하고 SEARCH-03 프로필을 바꿔 재색인한다. 라이브러리/검색 인프라 전체를 교체할 이유는 없다. 대안은 mean을 독자 프로필로 명시하고 아래 parity/검색 평가를 통해 유지 여부를 결정하는 것이다.

**제안 불변식 SEARCH-04-I**: `bge-m3/cls`라고 표시한 engine이 고정 reference encoder와 동일 tokenization·CLS·normalization을 사용한다. 새 ORT 조건부 `go test -tags ORT ./internal/embed -run '^TestBGEM3ReferenceParity$'`는 짧은/긴 한국어·영어·중국어 20개, padding 길이가 다른 batch, query/document 각각을 비교한다. fp32 reference의 element 오차/cosine 허용치를 먼저 정하고, int8은 별도 허용치와 SEARCH-10 rank 평가를 적용한다. 이 테스트는 이번 감사에서 실행하지 않았다.

## SEARCH-05 — 원문 청크가 whitespace 정규화로 바뀐다

**P2, G2 위반 재현.** [chunk.go](../../../internal/embed/chunk.go#L31) 31–40, 49–53, 70–74행은 문단 양끝을 TrimSpace하고 빈 문단을 버린 뒤 `\n\n`으로 다시 결합한다. `verbatim` 재현의 본문은 `First paragraph.  \n\n    indented code\n\n\n\nLast paragraph.\n`이다. title prefix를 제거한 text는 `First paragraph.\n\nindented code\n\nLast paragraph.`였고 `strings.Contains(body, text)==false`였다. Markdown hard break와 indented code 문법도 바뀐다.

기대는 G2 그대로 title prefix를 제외한 chunk text가 실제 본문의 연속 부분 문자열인 것이다. [store_test.go](../../../internal/store/store_test.go#L67) 67–86행은 수작업으로 넣은 문자열이 돌아오는지 검사하므로 **Split→indexer→recall의 원문 보존**을 증명하지 않는다.

가장 강한 반론은 모델 입력의 정규화가 품질에 유리하고 사람이 읽기 편하다는 것이다. 모델용 텍스트와 evidence용 원문은 별도로 둘 수 있다. **제안**: 원문의 byte offset 범위를 보존하고 원문 slice를 반환하며, 제목·embedding용 정규화는 별도 필드로 분리한다. content revision/hash와 offset을 응답에 포함하면 seq만으로는 부족한 장기 citation 검증도 가능하다. 대안은 G2를 완화해 정규화된 발췌라고 명시하는 것이지만, code/수식/공백의 의미를 바꾸는 증거 변형 비용이 있다.

**제안 불변식 SEARCH-05-I**: 모든 evidence text는 prefix 제외 후 원문 연속 slice이고, start/end와 source revision으로 재검증 가능하다. `go test ./internal/embed -run '^TestSplitPreservesSource$'`에 CRLF, hard break, indented/fenced code, 반복 빈줄, 표, 수식, Unicode 결합 문자, oversized paragraph를 포함한다. `go test ./internal/indexer -run '^TestRecallSourceRoundTrip$'`는 실제 저장 후 반환과 원문을 대조한다.

## SEARCH-06 — 벡터의 dimension·유효값을 경계에서 검사하지 않는다

**P2, 조건부 방어 부족 재현.** 전제는 잘못된 engine 출력, 다른 차원 모델로 query 변경, 또는 손상/혼합된 cache다. 정상 ORT가 혼합 차원이나 NaN을 냈다는 증거는 없다. [vectors.go](../../../internal/store/vectors.go#L35) 35–53행 ReplaceChunks는 slice 개수·dimension·norm·finite를 검사하지 않는다. 262–269행 decode는 blob 길이가 4의 배수인지만 검사한다. 273–279행 dot은 `min(len(a),len(b))`까지만 계산한다. 207–213행 PageVectors는 첫 chunk 길이에 맞춘 sum에 이후 vector를 더한다.

```text
dimension: stored_dim=3, query_dim=2, score=1, error=null
nonunit: stored [2,0], query [1,0] -> score=2
mixed_dimension_panic: index out of range [2] with length 2
nan: SearchChunks가 NaN score를 반환 -> json: unsupported value: NaN
```

기대는 서로 다른 공간의 비교를 거부하고 잘못된 cache를 재구축 대상으로 표시하는 것이다. 실제는 잘린 dot을 cosine으로 제시하거나 process/HTTP 응답을 깨뜨릴 수 있다. PageVectors는 일반 정상 vector 평균을 다시 정규화하며, zero-sum은 skip한다. 이 정상 경로의 normalization 자체를 결함이라고 보지 않는다.

가장 강한 반론은 Engine interface가 unit vectors를 약속하고 backend가 normalization하므로 검증은 중복이라는 것이다. 그것은 정상값 성능의 이유가 되지만 profile 변경·깨진 derived DB에 대해 조용한 가짜 점수를 내는 이유가 되지는 않는다. **제안**: 저장 시 output 개수·일관 dimension·finite/nonzero를 검사하고, 검색 시 effective profile/dimension 일치를 확인한다. 정상 engine normalization은 유지하되 norm 정책과 허용 오차를 명시한다. 대안은 별도 cache integrity check 후 신뢰하는 fast path이며, malformed cache는 clear/reindex 안내로 복구한다. 매 query마다 모든 norm을 다시 계산할 필요는 없다.

**제안 불변식 SEARCH-06-I**: 비교 대상은 같은 dimension/profile의 finite vectors이며 사용자 입력/cache 불일치에서 panic이나 NaN JSON이 없다. `go test ./internal/store -run '^TestVectorCompatibility$'`에 query 길이 mismatch, zero/NaN/Inf, per-page 혼합 dimension, malformed blob, slice 개수 부족을 넣고 error/rebuild 정책을 검사한다. 과거 레이아웃 version 변경 시 통째로 reset하는 현 설계는 `control_schema_reset`에서 pages/chunks 0으로 확인했다.

## SEARCH-07 — 요청한 k가 회수 상한을 보장하지 않는다

**P2, 인자 경계 재현.** [main.go](../../../cmd/canopy/main.go#L461) 461–463행, [recall.go](../../../cmd/canopy/recall.go#L79) 79–80행, [resurface.go](../../../cmd/canopy/resurface.go#L212) 212–216행은 숫자 인자의 유효 범위를 검사하지 않는다. [vectors.go](../../../internal/store/vectors.go#L174) 174–182행은 append 뒤 `len(out)==k`로만 중단한다. semantic 126–127행과 Fuse 39–40행/bridge 229–230행은 negative slice에 취약하다.

`chunk_k` 재현은 3개 페이지에서 `k=0`과 `k=-1` 모두 청크 3개를 반환했다. `SearchSemantic(...,-1)`은 slice bounds panic이었다. 실제 lite CLI `search --mode keyword --top-k -1`은 exit 0/hit 반환이며 SQLite 음수 LIMIT가 상한을 풀었다. ORT CLI negative k panic은 engine 없이 직접 store/fusion 코드로 확인한 범위를 넘어서 실행했다고 주장하지 않는다.

기대는 k의 일관된 정의다. 상한이 필요한 agent context injection에서 0을 무제한으로 해석하는 정책이라면 명시해야 하지만 `--per-page 0=unlimited`와 달리 top-k에는 그런 설명이 없다. 가장 강한 반론은 정상 사용자 default k가 양수라는 것이며 그 경로는 문제없다. **제안**: CLI/store 양쪽에서 k≥1과 합리적인 최대치를 검사하거나 k=0을 빈 결과로 통일한다. 무제한은 별도 명시 옵션으로 둔다. 대안은 호출자만 검사하는 작은 수정이지만 내부 소비자의 안전성은 남는다.

**제안 불변식 SEARCH-07-I**: 성공한 요청의 결과 수는 선언한 양수 k 이하이고, 잘못된 k는 빈 결과/명시 오류 중 정한 정책으로 처리하며 panic하지 않는다. `go test ./internal/store ./internal/search -run '^TestTopKBounds$'` 및 `go test ./cmd/canopy -run '^TestSearchRecallBridgeLimits$'`에서 -1/0/1/매우 큰 값과 per-page cap의 조합을 점검한다.

## SEARCH-08 — 우아한 강등의 적용 범위가 초기화 실패까지다

**P2, HTTP 재현/CLI 정적 추적.** 실제 lite CLI의 hybrid는 stderr `hybrid → keyword only (...)`, JSON `mode="keyword"`, exit 0이었다. explicit semantic과 recall은 exit 1로 명시적으로 거부했다. 이는 F2의 정상 보증이고 유지할 만하다.

그러나 [main.go](../../../cmd/canopy/main.go#L385) 385–418행은 `newEngine` 실패에만 fallback하며, 이후 indexer 또는 query Embed 실패는 반환한다. [server.go](../../../internal/webui/server.go#L478) 478–486행도 이미 구한 keyword hit가 있어도 query Embed 실패에 500을 반환한다. `api_inference_failure`에서 cached FTS에 `oldneedle` hit가 있는데 `Embed`가 오류를 반환하자 HTTP 500/`fixture inference failed`였다. CLI runtime 실패는 실제 ORT 대신 이 코드 경계로 확인했다.

가장 강한 반론은 stack 부재와 실행 중 오류는 다르며 내부 결함을 조용히 감추면 안 된다는 것이다. 맞다. 그래서 실패를 관측 가능하게 한 keyword fallback이 필요하거나 문서의 강등 범위를 더 좁혀야 한다. **제안**: hybrid의 vector refresh/query 실패는 현재 FTS 결과를 제공하며 actual mode·degraded reason·coverage를 명시한다. semantic/recall은 강한 evidence 보증을 위해 실패를 유지한다. SEARCH-02의 stale vector를 fallback에 섞지 않는다. 대안은 모두 fail-closed하되 사용자 문서가 “초기화 실패에 한한 강등”을 정확히 설명하는 것이다.

**제안 불변식 SEARCH-08-I**: fallback을 보장한 hybrid에서 embeddings의 단독 실패는 신선한 keyword의 성공을 막지 않고 실제 모드/원인을 보고한다. `go test ./internal/webui -run '^TestHybridInferenceFailureFallback$'`, `go test ./cmd/canopy -run '^TestHybridRuntimeFallback$'`에 initialization/query/corpus embedding 실패를 각각 주입하고 explicit semantic/recall 실패 정책도 검사한다.

## SEARCH-09 — 요청 취소와 입력 예산이 engine 경계까지 도달하지 않는다

**P2, 취소 재현 및 상한 정적 추적.** [engine.go](../../../internal/embed/engine.go#L22) 22–25행 interface에는 context가 없다. [engine_ort.go](../../../internal/embed/engine_ort.go#L66) 66행 New와 84행 RunPipeline은 `context.Background()`를 사용한다. [server.go](../../../internal/webui/server.go#L452) 452–479행은 mutex 안에서 scan/open/keyword/Embed를 수행하며 r.Context를 전달하지 않는다. client [app.js](../../../internal/webui/static/app.js#L64) 64–68, 95–100행은 오래된 fetch를 Abort하지만 server의 일을 중단시키지 못한다.

`cancellation`은 먼저 취소한 context의 request를 handler에 넣고 channel로 blocking하는 fake engine을 사용했다. 실제 `cancel_entered_engine=true`, `cancel_returned_before_release=false`였으며 50ms 기다려도 반환하지 않고 engine을 release한 뒤 완료했다. 이 50ms는 실제 모델 latency가 아니라 취소가 이미 된 요청도 engine을 호출한다는 결정적 경계 검사다. ORT native call의 실행 중 interrupt 가능 여부는 미검증이다.

추가로 [indexer.go](../../../internal/indexer/indexer.go#L67) 67–74행은 한 페이지의 모든 청크를 한 번에 Embed한다. [chunk.go](../../../internal/embed/chunk.go#L38) 38–39행은 제목을 budget 계산 밖에서 각 chunk에 prepend하며 body는 rune 기반이다. query 길이·title 길이·batch token/rune 수를 명시적으로 제한하지 않는다. 의존성 `backends/tokenizer_rust.go` 71–77행은 model 최대 token을 넘으면 잘라낸다. 따라서 “전체 chunk가 embedding에 쓰였다”는 보증도 tokenizer truncation 계측 없이 할 수 없다. 이번에는 실제 긴 입력 tokenization/OOM을 만들지 않았다.

가장 강한 반론은 엔진 thread safety 때문에 직렬화가 필요하고 개인 사용에서는 가벼운 입력이라는 것이다. serial engine 자체는 합리적이다. 그러나 lock 대기열에서 이미 취소된 질의를 제거하고 inference 사이에 취소를 확인하는 것은 별개다. **제안**: Engine/scan/store에 context를 전달하고 시작 전·lock/queue 획득 후·batch 사이에 취소를 확인한다. native inference가 중단 불가하면 그 한 batch만 끝내고 후속 일을 수행하지 않는 계약을 둔다. 배치를 작은 token 예산으로 나누고 title/query/총 반환 text 예산과 truncation 상태를 명시한다. 대안은 짧은 API 검색을 keyword-only로 운영하고 의미 검색을 submit 시점에 제한하는 제품 선택이다.

**제안 불변식 SEARCH-09-I**: 취소가 이미 된 요청은 engine 호출 0회이고, 취소 후 새 batch를 시작하지 않으며 queue 체류와 반환 evidence 크기에 상한이 있다. `go test ./internal/webui -run '^TestSearchCancellation$'`는 queued cancel/entered cancel/뒤따르는 최신 query를 channel fake engine으로 검사한다. `go test ./internal/indexer ./internal/embed -run '^TestEmbeddingInputBudget$'`는 매우 긴 제목·query·한 페이지 다수 청크에서 호출당 예산과 실제 tokenizer truncation 표시를 검사한다. 허용 native 작업 종료 시간은 구현 지원에 맞춰 먼저 정의해야 한다.

## SEARCH-10 — 현재 단어 처리와 점수를 근거 충족도로 읽을 수 없다

**P2, 제품 선택/평가 공백.** [store.go](../../../internal/store/store.go#L40) 40–42행은 `unicode61`, 166–175행은 whitespace term별 quoted prefix의 AND다. 이 전략이 잘 동작하는 한국어 prefix·영문 literal도 확인했다. 동시에 `keyword` 레코드에서 다음을 관측했다.

| 저장 본문 | 질의 | 결과 | 해석 |
|---|---|---|---|
| `프로토콜은 산업용 통신 규칙이다.` | `프로토콜` / `산업용 프로토콜` | `ko` | 현재 테스트가 보증하는 조사 없는 prefix |
| 같은 본문 | `프로토콜을` | 0 | 질의 쪽 조사 변형은 해결하지 않음 |
| `工业协议用于通信。` | `工业协议` / `协议` | `zh` / 0 | 공백 없는 텍스트의 내부 단어 retrieval 보증 없음 |
| `cache caching; C++ and C languages.` | `cache` / `cach` | `en` / `en` | prefix는 stemming 계약과 다름; 후자는 여러 형태를 포괄 |
| 같은 본문 | `C++` / `C` | 모두 `en` | punctuation 식별자의 구별을 보증하지 않음; C++만/C만 페이지의 precision은 별도 평가 필요 |

위 결과는 형태소 분석기를 당장 넣어야 한다는 결론이 아니다. 단어 수준 exact retrieval 선택의 경계를 보여준다. 자연 한국어 질문·조사·영문 활용·혼합 식별자·띄어쓰기·중국어의 품질은 현재 두 keyword assert만으로 판단할 수 없다.

[vectors.go](../../../internal/store/vectors.go#L94) 94–129, 146–184행은 similarity cutoff 없이 top-k를 반환한다. [search.go](../../../internal/search/search.go#L15) 15–30행 RRF는 raw BM25/cosine 크기와 무관하게 순위를 합친다. index에 무관한 문서만 있어도 semantic/recall은 가장 가까운 후보를 제공하는 정책이다. full web search는 487–501행에서 keyword snippet보다 best chunk excerpt를 우선하여 literal match가 보이지 않을 수도 있다. 이것들은 탐색 도구로 유효한 선택이며 **허위 답변을 생성했다는 관측이 아니다**. 다만 “nearest candidate”와 “질문에 답하는 evidence”를 분리할 정책/평가가 없다.

bridge 0.7, new related 0.8, reconcile duplicate 0.8, discovery 0.4/0.3/0.3 가중치도 서로 다른 목적이다. `new`는 whole-page query↔best chunk이고 bridge는 chunk mean↔chunk mean이라 동일 점수처럼 보아서는 안 된다. [write.go](../../../cmd/canopy/write.go#L91) 91–94행의 높은 similarity floor 설명도 SEARCH-04 pooling 경로를 포함한 corpus 평가가 필요하다. 청크는 문단/rune 기반이고 heading 계승·overlap이 없어 긴 문단의 조건/결론이 갈라질 수 있으나 실제 답변 실패율은 미측정이다.

가장 강한 반론은 판단·답변·abstention은 LLM의 책임이라는 철학이다. 맞다. 검색 코드가 정답을 판정할 필요는 없다. 대신 raw rank와 provenance·coverage·잘림 여부를 제공하고 “후보가 있으니 위키가 답한다”는 잘못된 신호를 피해야 한다. **제안**: 아래 소규모 평가를 먼저 만들고 profile/언어별 결과를 남긴다. literal과 natural query를 구분하며, 필요가 입증되면 낮은 비용의 query 정규화/FTS n-gram 보조/optional min-score·후속 원문 확인부터 비교한다. 한국어 형태소 엔진, reranker, 외부 vector DB의 도입을 전제하지 않는다. 대안은 CLI를 후보 탐색기로 명확히 문서화하고 recall의 근거 충족 판단을 LLM에 맡기되 low-match/raw score/미색인 상태를 함께 반환하는 것이다.

**제안 불변식 SEARCH-10-I**: retrieval score를 진실 확률로 표시하지 않고, 승인된 relevance 집합/언어/무응답 집합별 성능을 반복 평가할 수 있다. `go test ./internal/store -run '^TestMultilingualLiteralRetrieval$'`는 위 표와 punctuation-only/quote/operator escaping, 정규화 Unicode를 고정한다. `go test -tags ORT ./internal/search -run '^TestGroundedRetrievalEval$'`는 아래 데이터의 page Recall@k, evidence-span recall, nDCG@k, 무응답 질의에 대한 misleading-evidence rate를 profile별로 출력/회귀 검사한다. thresholds는 검증 집합에서 정하고 별도의 holdout으로 확인한다. 현재 구현에 보편적인 min cosine를 임의로 박는 것은 제안하지 않는다.

## SEARCH-11 — search 동점은 결정론적이지 않다; 정상 bridge는 다르다

**P3, 재현 결함.** [vectors.go](../../../internal/store/vectors.go#L102) 102, 121–125행과 [search.go](../../../internal/search/search.go#L20) 20, 33–38행은 map에서 결과를 모으고 score만 비교한다. 같은 DB에서 같은 unit vectors를 가진 a/b/c를 200번 검색한 `tie_orders`는 semantic 순서 `a,b,c`, `b,c,a`, `c,a,b`, RRF 순서 `a,b`, `b,a`를 관측했다. 일부 Go 실행에서 특정 순서만 나왔다고 결함이 해결된 것은 아니며 map 순서를 계약으로 삼을 수 없다.

기대는 같은 snapshot/profile/query에서 동점과 k 경계도 동일한 것이다. 실제 recall chunk sort도 score만 비교하고 SQL order가 없으므로 같은 policy를 명시하는 편이 낫지만, 그 chunk tie를 별도 반복 nondeterminism으로 관측했다고 주장하지 않는다. keyword SQL은 rank만 정렬한다. stable SQL order가 없는 것은 보증 공백이며, 이번에 keyword 순서 변동을 발견하지 않았다.

반면 [pick.go](../../../internal/resurface/pick.go#L198) 198–209행 bridge는 slugs를 정렬한 후 i<j 페어를 만들므로 유효한 고유 slug 입력에서는 self/대칭 중복 없이 반복 순서가 일정했다. `bridge_control`의 100회는 `a|b,a|c,b|c` 한 순서만, `unique=true`였다. B→A 역방향 링크를 추가하면 남는 두 페어만 반환했다. 이 경로에 map tie 결함을 잘못 전파하지 않는다. bridge cache 신선도 문제는 SEARCH-02다.

가장 강한 반론은 동점 문서가 같은 점수라 검색 품질에 영향이 작다는 것이다. UI만 보면 맞을 수 있지만 top-k 경계가 source 선택과 regression eval을 바꾼다. **제안**: score 뒤 normalized slug와 chunk seq의 명시적인 tie-break를 둔다. SQL도 동일 순서를 사용한다. 대안은 동점 그룹을 동등 relevance로 평가하면서 화면 순서만 고정하는 것이다.

**제안 불변식 SEARCH-11-I**: 같은 입력에서 결과·동점·k 경계가 동일하다. `go test ./internal/store ./internal/search -run '^TestDeterministicRankingTies$'`는 같은-score 페이지/청크와 RRF 동점을 200회/여러 DB reopen으로 검사한다. `go test ./internal/resurface -run '^TestBridgeUniqueDeterministic$'`는 현재 정상 속성을 고정하고 reverse link/linked flag/dismiss/cooldown을 함께 검사한다.

## SEARCH-12 — 작은 k에도 전체 청크와 전체 bridge 후보를 물질화한다

**P2, 조건부 자원 사용을 실제 측정.** 현 규모에서 brute-force는 단순하고 충분히 빠를 수 있다. [vectors.go](../../../internal/store/vectors.go#L146) 146–171행 SearchChunks는 모든 vector/text를 읽고 전체 청크를 sort한 다음 cap/k를 적용한다. [server.go](../../../internal/webui/server.go#L483) 483–487행은 같은 query에서 SearchSemantic과 SearchChunks를 연달아 호출하므로 vector DB를 두 번 훑는다. [pick.go](../../../internal/resurface/pick.go#L206) 206–230행은 모든 qualifying pair의 excerpt/metadata를 만들어 sort한 뒤 n=5로 자른다. 계산량은 vector retrieval O(C·D), chunks 정렬 O(C log C), bridge O(P²·D)와 qualifying pair 수만큼의 목록 메모리다.

synthetic benchmark는 같은 **유효한 1024차원 unit vector**를 모든 청크에 주고 페이지당 5청크를 저장했다. 서로 미연결이라 bridge 모든 페어가 threshold를 통과한다. 이는 고유사도 상한 사례이고 개인 위키의 평균 분포를 대표하지 않는다.

| 동작 | fixture 규모 | 관측 latency | 관측 할당 |
|---|---|---|---|
| SearchSemantic(k=8) + SearchChunks(k=16, cap=1) | 200페이지 / 1,000청크 | warm 10회 p50 7.866ms, nearest-rank p95 8.188ms | 평균 24.136 MiB/쌍 |
| 같은 paired scan | 1,000페이지 / 5,000청크 | p50 37.471ms, p95 38.732ms | 평균 121.215 MiB/쌍 |
| PickBridges(n=5) | 200페이지 / 19,900 후보 | 1회 23.420ms | 13.858 MiB |
| PickBridges(n=5) | 1,000페이지 / 499,500 후보 | 1회 596.851ms | 449.832 MiB |

할당은 `runtime.MemStats.TotalAlloc` 차이로 **누적 allocation**이며 RSS/peak live heap이 아니다. paired scan에는 model load/inference, wiki scan, DB open, FTS, HTTP, 렌더링이 없다. warm 10샘플 p95는 통계적으로 안정적인 서비스 SLO가 아니다. 현재 “수천 청크의 비교는 milliseconds” 주석을 이 결과가 반박하지 않는다. 입증한 문제는 n/k가 작은 호출에도 높은 후보 수에서 메모리를 크게 할당한다는 것이다.

가장 강한 반론은 1,000페이지가 현재 위키보다 크고 모든 vectors가 같은 극단 fixture라는 것이다. 그 전제는 그대로 남긴다. 그래도 동일/중복 내용이나 넓은 제목 공통성이 높은 corpus에서는 후보 수가 늘고 n=5라는 상한이 메모리 상한으로 작동하지 않는다. **제안**: bridge는 bounded top-n heap에 slug/score만 유지하고 채택 후 excerpt를 만든다. SearchChunks는 page cap을 반영하는 bounded candidate 구조와 한 번의 scan에서 page/chunk 결과 생성부터 검토한다. brute-force dot과 SQLite는 유지할 수 있다. 대안은 지원 페이지/청크 규모를 명시하고 budget 초과 시 결과를 제한/준비 상태로 보고하는 것이며, ANN/외부 검색 인프라로 교체할 근거는 아직 없다.

**제안 불변식 SEARCH-12-I**: 결과가 같은 한, top-n을 위해 보관하는 bridge metadata는 O(n)이고 query allocation/latency가 명시된 fixture budget을 넘지 않는다. 새 `go test ./internal/resurface -run '^TestBridgeBoundedCandidates$'`는 1,000페이지 equal vectors에서 top-5 정답과 보관 candidate 수를 검증한다. `go test ./internal/store -bench '^BenchmarkRetrievalScale$' -benchmem`은 1k/5k/10k chunk, actual-size text, k/cap 조합을 기록한다. 예시 수용 목표는 bridge 후보 저장 ≤n, paired vector scan의 5k fixture 할당을 현재 121 MiB보다 절반 이하로 낮추되 동일 결과를 유지하는 것이다. 목표치는 제안이며 이번에 달성했다는 주장이 아니다.

## SEARCH-13 — 성능 수치와 양자화 품질 주장의 평가 근거를 분리한다

**P3, 검증 근거 공백.** [README.md](../../../README.md#L198) 198–203행은 Apple Silicon/int8, 약 11ms/chunk, warm load 0.5s, fp32 대비 2.4배, cosine≥0.988, no-change reindex≈1s, keyword<100ms, web≈40ms를 제시한다. `docs/web-ui-board.md` 71–74, 138행에는 warm 149페이지의 페이지 로드·API 측정 기록이 있다. [quantize-model.py](../../../scripts/quantize-model.py#L10) 10–22행은 변환·파일 복사·크기 출력만 하며 benchmark/eval은 없다. 명시된 속도 수치가 거짓이라는 증거는 발견하지 못했다.

기대는 commit, chip/RAM, model export·weight/profile, ORT/tokenizer 버전·thread, corpus pages/chunks/token 길이, batch 크기, cache warm/cold, 전체 CLI vs kernel, 반복 횟수·분포가 함께 있어 재실행 가능한 것이다. 현재 기록만으로는 다른 사용자가 같은 결과를 검증할 수 없다. SEARCH-12의 fake kernel 측정은 실제 inference/CLI 숫자를 검증하거나 대체하지 않는다. **이번 감사는 실제 model load/embedding/int8-vs-fp32 검색 품질을 benchmark하지 않았다.**

또한 fp32와 int8의 같은 text 벡터 cosine≥0.988은 방향 유사성을 나타낼 뿐 ranking·top-k recall·bridge threshold 판정을 보증하지 않는다. 예를 들어 정답/오답 후보가 거의 동점이면 작은 방향 변화에도 순위가 뒤집힌다. SEARCH-04처럼 양쪽 모두 동일한 잘못된 pooling을 쓰는 비교라도 높은 cosine이 나올 수 있으므로 reference 계약 적합성 검사와 검색 품질 검사는 독립적이어야 한다.

가장 강한 반론은 README가 실측 경험의 대략적인 안내라는 것이다. `~`와 warm 표현은 유용하다. 그 범위를 유지하면서 “임베딩 품질 유지”라는 더 강한 표현을 측정한 vector 일치 범위로 좁히면 된다. **제안**: 재현 가능한 작은 benchmark/eval 스크립트와 고정 corpus manifest/원자료를 저장하고, speed와 quality를 별도 표로 기록한다. 실제 profiler 결과가 필요해질 때만 최적화를 넓힌다. 사용자 README의 한국어 수정을 후속으로 할 경우 영어 미러/hash도 같은 변경에서 갱신한다. 대안은 상세 benchmark가 준비될 때까지 수치를 조건부 경험값으로 표시하는 것이다.

**제안 불변식 SEARCH-13-I**: 공개 수치에는 재실행 명령/fixture manifest/profile이 있고, quantization 품질 문구는 실제 평가한 metric의 범위를 넘지 않는다. 새 `scripts/bench-search` 같은 경량 점검기를 만드는 경우 `scripts/bench-search --fixture <fixed-path> --profile <manifest> --runs 30 --json`으로 cold/warm load·no-change reindex·keyword process·HTTP·embedding batch를 분리 출력한다. 이 명령은 **후속 제안이고 현재 없는 파일**이다. 품질은 SEARCH-04 parity와 SEARCH-10 holdout eval을 함께 통과해야 한다.

## 기존 통과 테스트가 실제로 보증하는 것

| 기존 검사 | 증명한 범위 | 놓친 범위 |
|---|---|---|
| `store.TestKeywordSearch` | fixture의 한국어 prefix 질의에서 예상 첫 slug, MQTT hit 존재 | 질문형 조사/활용/CJK 내부 단어/식별자 precision, cold/stale API |
| `store.TestSearchChunks` | 수작업 2차원 vector의 순위, page cap, 삽입한 문자열 반환, score 내림차순 | real Split 원문 보존, empty-body/프로필 invalidation, real 1024차원/유효성, 0/음수 k |
| `search.TestFuse*` | RRF 산술, 두 목록의 합침/dedup/top-k, 일부 snippet 우선 | 동점 결정론, pool 깊이/threshold/최종 relevance, stale evidence |
| `resurface.TestPickBridges` | 지정된 유사 페어 존재, linked 배제/flag, dismissed 배제 | 현재 원문과 vector revision, unique/반복순서의 명시 assert, 대규모 allocation |
| web workspace/i18n/task tests | 렌더/탐색/언어 전환/제안 버튼, 기타 기존 웹 기능 | runtime fake engine·cold API·외부 수정·취소 queue; suggestion test는 직접 struct를 render하여 similarity 계산을 거치지 않음 |
| 이번 positive controls | 무변경 embedded=0, title/body 변경 embedded=1, engine 있는 deletion prune=1, cache schema mismatch reset | 실제 ONNX 추론 정확도·tokenization·동시 process 정합성 |
| 이번 lite CLI | F2 hybrid 강등/actual mode/exit0, explicit semantic·recall 오류, 실제 cache 사용 bridge | 실제 ORT cold/warm load·infer failure latency·모델 profile 변경 후 rank |

캐시 schema reset과 engine이 있는 deletion은 정상이다. title/body hash invalidation도 비어 있지 않은 일반 본문에서 정상이다. 이를 전체 freshness 결함으로 과장하면 안 된다. C5의 skip은 **페이지 단위**다. 페이지의 어느 청크 하나가 바뀌면 전체 페이지 batch를 다시 Embed하므로 청크 단위 최소 갱신을 테스트가 증명한다고 해석하지 않는다.

## 후속 평가 데이터와 수용 순서

다음은 SEARCH-10/SEARCH-13의 구체적인 bounded eval 제안이다. 실제 개인정보 위키를 공개할 필요는 없으며 hand-authored 작은 지식 corpus로 시작할 수 있다.

| 집합 | 크기 제안 | annotation·metric |
|---|---|---|
| literal/language | 40 query | 한국어 조사·띄어쓰기, 영어 활용/약어, 혼합 코드 식별자, 중국어/일본어; expected page slug와 허용 variant, language별 Recall@5/10 및 false positives |
| 의미 paraphrase | 40 query | 정답 page와 exact supporting span; 단어 겹침 0/일부/높음, 한국어→영어와 역방향, top-k page recall·span recall·nDCG |
| 근거 부족/무응답 | 20 query | corpus에 답이 없는 질문, 가까운 오답, 최신 사실이 삭제/정정된 질문; 후보 반환 자체와 support 판정을 분리, misleading-evidence rate |
| 긴 문서/구조 | 20 query | 표·code·수식·heading·경계에 걸친 조건/예외/부정·상충 주장; G2 exactness, source revision, token truncation, supporting span coverage |
| 연결/중복 후보 | 40 page pair | 관련 있음/공통 단어만/기존 한 방향 link/중복/모순/무관; bridge/new/reconcile 별도 precision@5와 임계값 calibration |
| churn | 고정 10페이지 × 전이 | initial/무변경/편집/비움/삭제/rename/type-dir move/profile 변경/cache삭제/부분 infer실패; 현재 원문 밖의 returned evidence 0, pending readiness 명시 |

query 레코드는 최소 `id, language, query, expected_pages, supporting_spans, answerable, corpus_revision`을 가져야 한다. corpus/profile을 고정하고 keyword/semantic/hybrid, fp32/int8, current mean/official CLS를 같은 query에서 비교한다. 튜닝용/holdout을 분리하고 우연한 한 질의 성공을 전체 품질 주장으로 쓰지 않는다. recall 후보는 모델이 가장 가까운 조각을 반환한 사실만 나타낸다. agent가 어떤 span으로 어떤 주장을 뒷받침했는지의 판정은 별도다.

구현을 진행한다면 이 보고서의 제안 불변식을 `docs/invariants.md`에 실행 검사와 함께 먼저 적는 프로젝트 규율을 따른다. 먼저 SEARCH-01/02의 현재 원문 정합성을 잠그고, SEARCH-03/04/06의 embedding 계약을 고정한 뒤 SEARCH-05의 원문 slice를 보장하는 순서가 바람직하다. SEARCH-07/08/09/11은 작은 경계 테스트로 병행 가능하다. SEARCH-10/13의 평가 없이 형태소 분석기·reranker·ANN으로 확대하지 않고, SEARCH-12는 bounded 보관부터 비교한다. 모든 제안은 기존 SQLite/로컬 단일 바이너리의 경계 안에서 출발할 수 있다.
