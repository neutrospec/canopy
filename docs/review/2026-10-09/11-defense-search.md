# Group B 방어·증거 중재 — 검색·운영

대상은 Canopy v0.10.0, `ede67e302a8487aecbb4d59587a8686292018cf6`이다. 검토자 `def_search`는 [원본 검색](02-search.md), [원본 운영](06-ops.md), [RED-B](08-red-search.md)를 읽고 별도 fixture에서 쟁점을 재실행했다. 다른 defense 보고서는 읽지 않았다. **빈 본문의 과거 근거, 소비 경로별 stale 결과, 거짓 model ready, Mermaid 전달문 변형은 방어되지 않는다.** 다만 비정상 내부 벡터, native 품질, 실제 브라우저 표시, 배포 법률 판단까지 같은 수준으로 입증된 것은 아니다.

`accept`는 한정한 주장이 증거로 성립함, `narrow`는 범위·전제·의무를 축소해야 함, `reject`는 해당 결함 또는 의무의 근거가 없음, `defer`는 필요한 실행·제품 결정이 남음이다. **주장 판정과 해결안 판정은 독립적**이다. 심각도는 로컬 위키에서의 중간/낮음으로 표시한다. 원본의 P1/P2를 원격 취약점 등급으로 전용하지 않는다. 아래 25행은 원본 **SEARCH-01–13, OPS-01–11 전부와 RED-B-01**을 포함하며, `SEARCH-xx-I`는 같은 ID의 제안 검사이지 별도 결함이 아니다.

AGENTS.md와 원래 [철학](../../philosophy.md), [불변식](../../invariants.md), [버전 설계](../../versioning.md)를 먼저 대조했다. G2는 원문 청크의 정확성 계약이고, 원칙 6·10은 후보 판단을 LLM에 맡기므로 검색 코드에 정답/무응답 판정을 의무화하지 않는다. cache revision/profile/원문 slice의 저장 레이아웃 변경은 `store.SchemaVersion`과 재구축 대상이다. 반대로 새 모델 manifest가 기존 머신 상태를 반드시 바꾸게 한다면 기존 설치의 adoption과 append-only migration 규칙을 별도로 검토해야 한다. 이번에는 invariants나 구현을 고치지 않았다.

## 독립 증거와 한계

실행은 새 `/private/tmp/canopy-def-search-*` 및 `/private/tmp/canopy-def-search-follow-*`에서 했다. 모든 Go·canopy 호출의 HOME/XDG/TMPDIR를 fixture로 고정하고 `GOTELEMETRY=off`, `GOCACHE=/private/tmp/canopy-review/go-cache`, `GOPROXY=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, 읽기 전용 재사용 모듈 경로 `/Users/nobocop/go/pkg/mod`를 명시했다. 실제 SDK는 `go1.27.1 darwin/arm64`이며 go.mod의 Go 1.26.4와 동일 SDK 검증은 아니다. 실제 위키, 설치 모델/native 자산을 열거나 변경하지 않았고, 모델 다운로드·양자화·서비스·커밋·원격 Git 작업을 하지 않았다. Orca 스킬/CLI 안내와 최초 Git 조회만 기존 shell 환경에서 실행했다. **Go나 canopy를 상속 HOME으로 실행한 적은 없다.** SDK telemetry 파일이 임시 HOME에 생겼으므로 `GOTELEMETRY=off`를 HOME 무쓰기 증명으로 사용하지 않는다.

원본의 ORT stack·ONNX metadata는 보존된 기록으로 읽었으며 내 native 재검증으로 표시하지 않는다. HTTP handler는 실제 라우팅을 `httptest`로 호출했고 semantic 동작은 결정론적 unit engine이다. 실제 ORT load/infer, 배포 export, CLS reference parity, 브라우저 SVG/typesetting, release archive, workflow failure injection, cross-platform native 실행, peak RSS, 프로세스 간 race는 미실행이다. 재현된 내부 vector panic을 CLI/browser 결함으로 승격하지 않는다.

재실행 명령은 다음과 같다. 첫 실행의 probe compile 오류(`Parse` 대신 실제 API `Validate` 필요)는 수정 전 로그에도 남겼다. 기존 internal/JS 검사는 그 첫 실행에서 통과했고, 수정한 **자체 probe**의 컴파일·실행은 새 fixture에서 다시 성공했다. 테스트 파일·소스 구현은 변경하지 않았다.

```sh
python3 docs/review/2026-10-09/evidence/def_search-run.py.txt
python3 docs/review/2026-10-09/evidence/def_search-followups.py.txt
```

증거 표의 `M:<record>`는 [독립 probe 출력](evidence/def_search-probe-output.txt), `R:<label>`은 [argv·환경·exit·stdout·stderr](evidence/def_search-run.jsonl.txt), `F:<label>`은 [추가 실행 기록](evidence/def_search-followups.jsonl.txt), `S:<file:line>`은 [번호·SHA256을 보존한 코드](evidence/def_search-static.txt)를 뜻한다. 소스는 [Go probe](evidence/def_search-main.go.txt), [실행기](evidence/def_search-run.py.txt), [추가 실행기](evidence/def_search-followups.py.txt), [MathJax 검사기](evidence/def_search-mathjax.cjs.txt)다. [첫 실행 기록](evidence/def_search-initial-attempt.jsonl.txt)의 `tests`, `ui-tests`에 전체 baseline 결과가 있다. probe exit 0은 **관측 수집 성공**이고 아래 미구현 인수 검사 통과가 아니다.

| 독립 검사 | 관측 | 범위 제한·반례 |
|---|---|---|
| `go test -count=1 ./internal/... ./cmd/canopy`; Node app tests | exit 0; JS 18 pass, 0 skip | embed/indexer/cmd에는 test 없음. ORT/race/browser 통과가 아님 |
| initial → unchanged → blank → nonempty → delete | embedded 1→0, blank chunks 1/옛 9 bars; nonempty embedded 1; delete pruned 1/chunks 0 | 일반 갱신·삭제가 모두 깨졌다는 주장 반박 |
| blank 이후 `/search?q=bars` | HTTP 200, `retired valve rating was 9 bars` HTML 잔존 | handler 도달 입증; 실제 ORT CLI recall 실행 아님 |
| nil-engine edit/delete | 새 title에 옛 text; 삭제 후 joined chunks 0/raw vectors 1 | G1 recall 삭제 누출과 raw PageVectors 잔존을 구별 |
| profile A→직교 B, 동일 text | B 호출 0, score 0; title 변경 시 B 호출 1 | 실제 fp32/int8 손실 실험 아님 |
| cold/exact/full/API edit/delete | cold와 exact redirect 뒤 빈 응답; full search로 회복; 이후 API는 옛 hit·삭제 slug | keyword API는 fake engine 없이 재현 |
| lite bridge/reconcile | 현재 maple excerpt+옛 similarity 1; 삭제 maple을 cedar의 dup 후보로 반환, 모두 exit 0 | cache seed만 synthetic. 소비 CLI 자체는 실제 실행 |
| k=-1/0/1, HTTP 경계 | chunks는 -1/0 모두 3; semantic -1 내부 panic; CLI bridge -1 exit 2; HTTP -1/0/51/overflow는 8, k=1은 1 | HTTP 음수 panic 주장 반박; ORT search stack은 원본 기록 |
| 취소 요청이 mutex 대기 | 첫 요청 release 전 미반환; 취소 요청도 Embed, 총 2회/200 | 30ms는 channel gate 관측 시간이며 native latency·DoS 측정 아님 |
| source slice·Mermaid | whitespace slice=false; normal/CRLF/긴 Unicode=true; 4-tick/tilde/quote는 source 변형·parse error, 3-tick 정상 | browser SVG 미실행. P4 boot fault는 diagram=nil/environment error |
| model 0B/동명 directory | status available=true, cached pull ready/exit 0 | 네트워크 사용 0. runtime loader까지 검증한 상태가 아님 |
| tags·task helpers | missing/empty/scalar 모두 missing-field 없음; 1p 중복3개 300%, 정상2p count2; tasks/web helper 모두 20,480B DB | 각 sentinel은 임시 HOME. 사용자 데이터 손실 아님 |
| MathJax | loader 3.2.2/color autoload; color/physics 동일-origin 경로 404, tex-svg 200 | 실제 수식 error node·외부 요청 미검증 |
| cap 개선안 반례 | A score1/.99, B .8, k2/cap1: 현 결과 A,B; global top2 뒤 cap은 A만 | 새로운 runtime 결함이 아니라 제안 최적화의 동등성 반례 |
| bridge dense/sparse | 1000p/3D/n5: dense 663,470,120B(약632.7MiB)/232.439ms; 후보0은 35,160B/22.278ms | TotalAlloc 누적치, RSS 아님. 동일 unit vectors·많은 미연결 후보 조건 |
| fake curl exit7+완전 toy tar | make exit0; 38B toy 파일 생성; 다음 deps `Nothing to be done` | 실제 upstream/native 손상 사건을 입증하지 않음 |

## 검색 ID 전수 중재 행렬

각 행의 마지막 열은 후속 검사에서 관측해야 할 최소 결과다. 자세한 질문 답변·대안 판정은 같은 ID의 다음 절에 있다.

| ID | 원 주장·제안 | RED의 반론·방어 결론 | 독립 근거 | 주장 판정 | 해결안 판정 | 최종 심각도·실제 전제 | 한정 해결·측정 인수 |
|---|---|---|---|---|---|---|---|
| SEARCH-01 | 빈본문 뒤 과거 청크. 0집합 교체/title-only/제외와 I | nonempty/delete 정상은 인정; blank 금지 계약은 없음. 삭제 실패를 성공 취급할 수 없음 | M:blank/blank_http/empty_replace_control, R:cli-empty-update; S:indexer:51–58 | **accept** | **narrow**: 0집합 교체 accept, 새 title-only defer | 중간; 이전 청크가 있고 페이지가 남은 채 body가 empty/whitespace | 빈집합 교체 완료 후 성공, 재실행0청크; clear 실패는 오류. 전체 G2는 SEARCH-05도 필요 |
| SEARCH-02 | cold/stale API·nil/failure·bridge/추천/reconcile. revision/view/bootstrap/coverage 또는 TTL | full 갱신·joined 삭제 필터 정상. 전 소비자 G1 위반으로 묶지 않음; API/bridge/reconcile는 남음 | M:api_*/nil_*; R:bridge-current-excerpt/reconcile-deleted-duplicate; S:suggest:32–77 | **narrow** | **narrow**: 최소 bootstrap·current revision gate; TTL 단독 reject | API 낮음, 현재 의미 후보 불일치 중간; cache 뒤 외부 변경/nil engine | 현재 출처 밖 결과0; cold=준비상태/FTS bootstrap; 부분 coverage 수치; 일반 정상cache 보존 |
| SEARCH-03 | text hash만으로 profile 변경 skip. profile/manifest/compatibility·설정 소비 | fake 직교는 int8 피해량 아님. 다만 같은 차원 profile 미식별은 실제다 | M:profile_switch/title_control; S:config:106–109,indexer:64 | **accept** 프로필 공백; 피해 일반화 narrow | **narrow**: effective profile gate, manifest 단계화·legacy 관용 | 중간·조건부; 실제 embedding 계약이 달라야 함 | same-dim A/B skip금지, 동일profile no-op; 수동 교체 감지 정책·기존 설치 adoption 검사 |
| SEARCH-04 | 3D BGE output→mean, 공식 dense CLS. wrapper/export 또는 독자 mean+parity | query/corpus 일관성은 인정. 품질 P1·모든 export 적용은 증명 부족 | S:engine_ort:71–75,hugot:222–247; 원본 graph metadata; 공식 pooling 설정 | **narrow** 계약 불일치; 품질량 defer | **narrow**: profile/contract 명시 accept; 즉시 CLS 재수출 defer | 낮음/계약 공백; 기록된 3D export 조건 | 실제 export/output 고정; CLS를 제공하면 tokenizer/padding/normalize parity와 profile 재색인 |
| SEARCH-05 | 문단 trim/rejoin이 G2 위반. raw slice·offset/revision 또는 G2완화 | normal/CRLF/Unicode 정상도 있음. normalization hash만으로 raw revision을 대표 못 함 | M:source_slice/normalized_hash_control; S:chunk:31–74 | **accept**; 변형되는 입력으로 narrow | **narrow** raw slice accept; 새 응답필드 optional, G2완화 reject(동등수정으로는) | 낮음~중간; 들여쓰기/hardbreak/빈줄 의미가 있는 본문 | Split→store 반환에서 prefix 제외 실제 연속slice; 원문만 변경해도 old offset 재사용0 |
| SEARCH-06 | dim/count/finite/norm 미검사·panic. 저장/조회 검증·integrity/rebuild | unit Engine 계약 인정. mixed cache/NaN의 정상 지원경로는 미입증; malformed blob은 오류 처리됨 | M:dim_mismatch/nonunit/mixed_dim_panic/nan_json/slice_count_panic/malformed_blob_control | **narrow** 방어 부족; 정상 ORT/원격 장애 reject | **narrow** finite/count/dim 검증 accept; exact norm/전query전수/keyword 차단 reject | 낮음; invalid engine/cache. 다른dim profile 비교는 SEARCH-03과 조건부 중간 | mismatch·NaN·Inf·zero·개수 부족을 panic없이 진단, float32 오차 허용, keyword 복구 유지 |
| SEARCH-07 | k≤0 unlimited/empty/panic. CLI/store 검사·max/unlimited | 양수 정상, HTTP 1..50 보정 정상. per-page 0 의미 보존 필요 | M:count_bound/api_k; R:cli-count-*/bridge-negative | **accept** local boundary; HTTP crash reject | **narrow** 음수거부·0정의 accept; 임의max/새옵션 defer | 낮음; local flag/internal invalid count | top-k CLI는 양수 정책 권고; API 기존 보정·per-page0=unlimited 유지; 작업 전 오류 |
| SEARCH-08 | initialization만 강등, infer실패500. fresh keyword fallback+reason 또는 docs한정 | F2는 stack 부재 계약. runtime fallback은 새 선택; broad catch는 오류를 숨김 | M:api_infer_failure; R:cli-count-hybrid-*; S:main:389–418 | **narrow** 정책 공백; F2위반 reject | **narrow** 문서 범위 명시 accept; typed fallback conditional/defer | 낮음; initialization 성공 후 inference 실패 | fail-closed 문서 또는 embedding-only fallback 계약 중 명시; stale/partial vectors 혼합0 |
| SEARCH-09 | 취소 불전달·대기열·예산 없음. context/gate/batch/budgets/keyword API | thread serialization·per-input tokenizer 제한 인정. native 중단/OOM/보안 주장은 없음 | M:queued_cancel; S:server:453–479,engine_ort:66/84,tokenizer:71–77 | **narrow** 불필요 작업; 자원장애 defer | **narrow** 전후 취소체크 accept; 즉시취소gate·hard budget는 결정/측정 후 | 낮음; 중첩 검색/느린 batch에서 사용자 지연 | 사전취소 Embed0, queued 취소 release후 새Embed0; native 한batch 종료 허용, 즉시대기취소는 별도 계약 |
| SEARCH-10 | prefix 다국어·nearest/RRF·평가 공백. 소규모eval·필요시 정규화/ngram/minscore | rank를 확률이라고 출력하는 코드 없음. 판단/abstention은 LLM 책임 | M:literal; S:store:131–175,recall 설명·출력 | **narrow** 계약/eval 공백; 실행 버그 reject | **narrow** eval·정확한 문서 accept; 형태소/ANN/reranker/minscore 의무 reject | 낮음/제품 선택; 지원 언어·literal/natural 계약을 먼저 정함 | query/page/span/answerable annotation과 언어별metric; rank≠truth; no-answer 판정 주체 명시 |
| SEARCH-11 | semantic/RRF exact tie·경계 비결정론. slug/seq/SQL tie-break | relevance tie 동일해도 source 선택 변동은 실재; 정상 bridge는 별개 | M:ties(300회 k1); S:vectors:121–127,Fuse:33–40 | **accept** semantic/RRF; SQL/recall 변동은 defer | **accept** exact-score total order; SQL/seq는 계약 보강 | 낮음; exact tie corpus/재현성 필요 | score 뒤 slug/seq, best chunk tie도 고정; 여러 reopen/process k1 동일; 근접점수 EPS변형0 |
| SEARCH-12 | 전체 chunks·중복scan·모든bridge 목록. heap/shared scan·예산 | result n은 CPU/RSS 계약 아님. dense candidate allocation만 재현; 평균장애 아님 | M:bridge_scale/cap_counterexample; S:pick:206–230,vectors:146–182 | **narrow** 조건부 비용; 실제 사용자OOM defer | **narrow** bounded bridge accept; cap-aware scan은 독립검증, 절반 목표 defer | 낮음~중간; 페이지수와 qualifying pair 밀도 둘 다 큼 | 최종 보관 O(n), late excerpt; tied/filter 결과 동일; k2/cap1에서 A,B 보존 |
| SEARCH-13 | README 속도/양자화 품질 재현조건 부족. bench/manifest/eval/영어 | 수치가 거짓이라는 증거 없음. cosine은 rank/threshold 보존 보증 아님 | S:README:198–205,quantize:10–22; 원본 native smoke | **narrow** 검증 공백; 성능실패 reject | **narrow** 조건·문구 범위 accept; 전체30회harness defer | 낮음/보증 공백; 공개 경험값을 일반 품질로 읽을 때 | profile/sample/환경/명령 공개 또는 미재현 경험치 표시; 한영+source hash 동시검사 |

### 검색 질문에 대한 직접 답변과 인수 기준 보정

**SEARCH-01.** 본문 비우기를 금지하는 실제 계약은 찾지 못했다. `update --body-file <empty>`가 exit 0이고 실제 파일 body가 비는 것도 확인했다. “빈 페이지가 비정상”이라는 방어는 성립하지 않는다. 기존 `ReplaceChunks(slug,nil,nil,nil,nil)` 호출은 fixture에서 청크를 0으로 바꾸므로 새 title-only embedding을 설계할 필요 없이 최소 경계가 존재한다. clear의 오류를 무시하고 refresh 성공을 보고하는 해결은 기각한다. title-only/제외는 별도 제품 선택이며, 선택해도 옛 body는 0이어야 한다. `TestReindexClearsEmptyBody`는 empty/whitespace/nonempty/delete/재실행 및 clear 실패를 포함해야 한다. 원본 I의 “성공 후 모든 청크가 현재 원문”은 0청크 처리만으로 통과하지 못하므로 SEARCH-05와 별도로 검사한다.

**SEARCH-02.** current evidence를 약속하는 것은 recall(G2), 현재 검색 제목·snippet, 현재 bridge/관련·중복 후보에 붙인 의미 점수다. 발견 화면의 **최근 읽은 slug를 interest 신호로 사용하는 것**은 역사적 주의를 보존하는 정당한 목적이다. 그렇다고 수정된 현재 페이지를 옛 vector와 동일 텍스트인 것처럼 비교해도 된다는 뜻은 아니다. 읽음 이력 자체를 지울 필요 없이 current candidate의 source/profile 유효성만 구별할 수 있다. 삭제 raw vector가 존재해도 joined recall은 0이고 bridge/suggest는 scan 존재필터가 있으므로 G1 누출로 일반화하지 않는다. 실패 Reindex는 오류를 반환하므로 그 호출이 거짓 성공을 반환했다고도 하지 않는다. 반면 reconcile의 삭제 dup는 실제 성공 CLI 응답으로 확인했다.

단일 `semantic_ready` bool은 부분 coverage를 설명하지 못한다. 최소한 현재 scan 총 페이지, 유효 vector 페이지/청크, stale/missing 수 또는 needs_reindex를 구별해야 한다. 첫 API는 FTS bootstrap 또는 준비상태, 삭제 출처는 제거, current-vector 소비자는 최소 content/profile gate를 적용하는 범위로 좁힌다. 매 키 입력마다 전체 재색인, keyword-only 읽기마다 모든 정상 vector 삭제, 모델 추론을 하나의 장기 SQLite transaction으로 감싸는 의무는 기각한다. TTL+stale 표시/bridge의 reindex 요구는 가능한 운영 대안이지만 TTL만으로 삭제 근거를 정상 현재 결과로 허용할 수 없다. `TestAPISearchColdAndChanged`, `TestDegradedIndexDoesNotServeStaleChunks`, `TestBridgeChecksRevision`는 cold/cache삭제/본문·제목편집/삭제/rename/부분실패를 나누고 정상 full-search 회복을 positive control로 남긴다. 두 프로세스 교차 commit은 **후속 race 검사**이며 이번에 관측한 race가 아니다.

**SEARCH-03.** 현재 기본 선택은 `EmbeddedLLM/bge-m3-onnx-o2-cpu`를 받는 `bge-m3`, 선택적 script의 `bge-m3-int8`, 위키 설정의 모델 directory다. 코드에 호환 profile registry는 없다. 지원/호환성을 선언할 주체는 배포 모델 계약과 이를 검사하는 프로젝트이며, 디렉토리 이름이나 내가 만든 직교 engine이 실물 호환성을 대신하지 못한다. 같은 이름 weight 교체는 이름만으로 검출 불가다. 설치·변환 때 만든 versioned manifest와 명시적 검증/수동교체 후 invalidation 정책을 묶어야 하며 매 query 2GB hash는 요구하지 않는다. manifest 없는 기존 정상/수동 모델은 unknown profile로 채택·검증한 뒤 **파생 캐시만** 재구축할 수 있어야 한다. 기존 모델을 삭제하거나 새 manifest 없음을 곧 unusable model로 판단하는 안은 기각한다.

unused `dimension`, `chunk_tokens`는 직렬화만 되고 계산에 쓰이지 않는다. 지원하지 않는 **비기본** 값의 경고/거부 또는 실제 지원을 명시하되, 기본 config까지 일괄 거부하거나 `dimension=2`를 vector 절단으로 구현해서는 안 된다. `TestReindexEmbeddingProfile`는 같은차원 A/B, tokenizer/pooling/dimension·동일이름 교체, 동일profile no-op을 검사하고, `TestEmbeddingSettingsHaveEffectOrReject`는 legacy default 관용도 포함해야 한다. acquisition revision/digest/repair 제안은 OPS-02와 합쳐 검토한다.

**SEARCH-04.** 제공 export의 **현재 전체 identity/output**은 이 방어에서 확인하지 못했다. 원본 보존 metadata의 두 설치 graph는 `last_hidden_state [batch,sequence,1024]`이며 external weight identity까지 증명하지 않는다. 고정 hugot v0.7.5는 3D를 mean, 2D를 그대로 받는다. [BAAI 공식 pooling 설정](https://huggingface.co/BAAI/bge-m3/blob/main/1_Pooling/config.json)은 CLS=true/mean=false다. 따라서 그 **기록된 3D graph 조건**의 구현 계약 차이는 남는다. mean으로 query/corpus를 함께 만드는 일관성은 그 차이를 없애지 않지만, relevance가 얼마나 나쁜지는 답하지 못한다.

CLS 전환의 위키 relevance 이득, CPU 비용, fp32/int8 parity 자료는 없다. 먼저 현재 mean effective profile과 제공 graph contract를 고정하고, CLS를 제공하기로 하면 작은 wrapper/지원 경로+새 profile+재색인을 한 묶음으로 검토한다. external weights/CPU fused ops/quantization을 확인하지 않은 즉시 재수출·전면 threshold 조정은 유보한다. `TestBGEM3ReferenceParity`의 20문장은 tokenizer/special tokens/padding 길이/CLS/L2와 오차허용치 검사의 smoke이며 검색 품질 전체 증거가 아니다. int8 허용치는 fp32와 분리하고 SEARCH-10 holdout을 함께 본다. ORT가 없어 skipped/unavailable인 결과는 pass로 계산하지 않는다. **미해결 이견:** 공식 dense를 즉시 복원할지, 명시한 mean profile을 평가 후 유지할지는 이 자료만으로 확정하지 않는다.

**SEARCH-05.** embedding 입력 hash와 evidence revision은 서로 다른 대상을 식별해야 한다. `First.  \n\n    indented statement`와 `First.\n\nindented statement`는 독립 probe에서 raw bytes가 다르지만 Split hash가 같았다. 이 관측은 기존 normalization 쟁점의 인수 반례이며 새로운 CLI 피해 주장으로 세지 않는다. normalized text는 모델용으로 유지해도 raw body revision/반환 slice 검증을 대신할 수 없다. offset을 쓰면 body 기준 UTF-8 byte start/end인지 명시하고 content revision과 묶어 재생성해야 한다. 앞 원문 변경 후 옛 offset을 그대로 재사용하지 않는 검사가 필요하다.

최소 G2 수정에 새 응답 offset/revision 필드나 별도 DB 필드가 반드시 필요한 것은 아니다. 최종 반환이 실제 원문 slice임을 보장하면 되고, 장기 citation 기능을 원할 때 필드를 추가한다. `TestSplitPreservesSource`와 `TestRecallSourceRoundTrip`은 CRLF/hardbreak/indented·fenced code/반복빈줄/표/수식/결합문자/oversized paragraph를 **Split→indexer→store 반환**까지 검사해야 한다. 저장한 수작업 문자열의 왕복만으로는 부족하다. “정규화 발췌” 표시만으로 G2를 완화하는 대안은 공백의 코드·수식 의미를 복원하지 못하므로 이번 버그의 동등 해결로 기각한다. 명시적 제품 계약 변경으로 논의할 수는 있다.

**SEARCH-06.** mixed-dim/NaN/개수 부족을 실제 만드는 정상 ORT 지원경로는 확인하지 못했다. same-page ReplaceChunks는 원자 교체라 정상 모델 교체가 같은 페이지 안 mixed dimension을 자동 생성한다고 주장하지 않는다. profile dimension이 바뀐 뒤 비교를 계속하면 조용히 잘린 dot이 되는 경계는 SEARCH-03과 함께 남는다. scalar nonunit/NaN/mixed/count 부족은 독립 내부 API 반례, 2B blob은 정상 오류라는 반례를 모두 유지한다.

정책은 저장 직전 count·finite·일관 dimension, query의 effective profile/dimension 검사다. zero vector는 의미 cosine 근거로 승격하지 않고 해당 vector 사용불가로 처리하며 keyword 결과는 유지하는 방향을 권고한다. norm 허용 오차는 float32 정규화/실제 backend에 맞춰 정해야 한다. exact norm==1, 매 query 전체 norm 재계산, invalid vector 하나로 keyword까지 중단하는 안은 기각한다. `TestVectorCompatibility`는 error/skip/rebuild 상태를 구분하고 panic·NaN JSON 부재, 정상 near-unit 허용, malformed blob 기존 진단, engine 없는 keyword 복구를 검사한다. 자동 rebuild는 engine이 없을 때 성공했다고 표시해서는 안 된다.

**SEARCH-07.** 내 권고는 사용자 `top-k`/`-n`을 양수로 정의하고 0·음수는 명시 오류로 만드는 것이다. 0=빈결과도 가능한 제품 선택이지만 검색·recall·bridge에서 일관되어야 한다. **`per-page 0=unlimited`는 별도 cap 의미라 그대로 보존**하고 HTTP의 잘못된 k→default8 기존 정책도 무심히 바꾸지 않는다. 내부 Fuse처럼 error return 없는 API는 안전 빈결과 또는 API 변화 중 명시한 정책을 시험한다. 양수 상한 유지·오버플로 문자열 거부는 필요하지만 근거 없는 작은 max나 새 unlimited 옵션 추가는 유보한다. `TestTopKBounds`/`TestSearchRecallBridgeLimits`에서 -1/0/1/large×mode×cap을 검사하고 model/index 호출 전 진단·panic 부재를 확인한다. state 기준선은 OPS-11대로 migration 이후다.

**SEARCH-08.** 이 리뷰는 runtime fallback을 기존 F2 의무로 승격하지 않는다. 현재의 fail-closed도 문서가 **초기화 실패에서의 강등**을 정확히 말하면 일관된 선택이다. runtime fallback을 지원 계약으로 정한다면 embedding-only typed error에서만 최신 FTS의 actual mode/reason을 반환하고 semantic/recall은 오류를 유지할 수 있다. SQL/scan/invalid input/cancellation까지 catch-all로 성공시키는 안은 기각한다.

corpus embedding 실패 뒤 FTS 신선도는 RebuildPages의 완료 revision을, partial vectors 배제는 그 실패 요청이 semantic/fusion을 전혀 소비하지 않는 것을 검사해야 한다. 기존 handler는 오류라 partial 성공을 반환하지 않지만 이후 다른 소비자의 유효성은 SEARCH-02 책임이다. `TestHybridInferenceFailureFallback`/`TestHybridRuntimeFallback`는 init/query/corpus 실패를 나누고 최신 키워드 hit, `mode=keyword`, 진단, semantic/recall 오류 및 stale-vector 혼합0을 assert해야 한다. 문서 한정 대안을 선택했다면 같은 테스트에서 runtime 명시 오류와 F2 성공을 검사해야 하며, 아직 없는 fallback 테스트 명칭만 실행해 `[no tests to run]`을 수용 통과로 세면 안 된다. **미해결 이견:** runtime fallback의 제품 채택 여부.

**SEARCH-09.** 이미 실행 중인 ORT **한 batch의 종료를 기다리는 계약은 수용**한다. 취소 후 다음 batch/새 Embed를 시작하지 않는 것과 native in-flight interrupt는 다른 보증이다. mutex 획득 전후 ctx 검사만으로 queued 요청을 즉시 반환할 수는 없다. 즉시 대기 취소를 약속하려면 취소 가능한 gate/queue 획득 또는 동등한 조정이 필요하다. 현재 주장의 최소 해결은 release 후 취소 요청의 새 Embed 0회이고, queue 시간 상한을 아직 측정·정의하지 않은 상태에서 “즉시 취소”라고 쓰지 않는다.

전체 Engine/scan/store interface 변경은 수단이지 필수 결과가 아니다. tokenizer는 per-input token 길이를 자르지만 encode 이전 raw input, 전체 batch, 반복 title, 반환 text 예산을 보증하지 않는다. 실제 truncation·호출별 token 수·긴 제목/query·다수 청크 비용을 측정한 뒤 batch/token/text budget과 표시를 정한다. 무조건 짧은 rune truncation·keyword-only autocomplete는 의미를 바꾸는 선택이라 유보한다. `TestSearchCancellation`은 사전취소/queued취소/진입후취소/뒤따르는 최신질의를 channel로 검사한다. `TestEmbeddingInputBudget`는 실제 tokenizer truncation 여부까지 분리하고 native 종료시간·DoS/OOM을 fake engine으로 입증하지 않는다. **미해결:** native 취소 지원과 지원 규모에 맞는 budget.

**SEARCH-10.** rank를 진실 확률이라고 말하는 UI/CLI 문구는 발견하지 못했다. 후보를 반환하거나 cutoff 없이 nearest를 주는 것 자체는 철학 위반도 답변 생성 버그도 아니다. 독립 literal 표는 Korean prefix 성공, 조사 변형/CJK 내부 miss, C++와 C 구분 한계를 보여줄 뿐이다. 형태소/정규화/ngram/heading·overlap/minscore/ANN/reranker는 baseline에서 필요가 확인된 뒤 비교할 후보이지 필수 처방이 아니다.

“misleading-evidence rate”를 쓰려면 인간/승인된 annotation의 **query–returned span–구체 주장** 단위를 먼저 정의하고, corpus가 그 질문에 답하는지와 span이 주장을 지지하는지를 별도로 표시해야 한다. 단순 무응답 질의에 후보가 나왔다는 이유만으로 misleading으로 세지 않는다. 판단 주체 없는 metric은 인수 기준에서 제외한다. `TestMultilingualLiteralRetrieval`는 언어/정규화Unicode/identifier/quote/operator의 현재 지원 정책을 고정한다. `TestGroundedRetrievalEval`의 literal40/paraphrase40/no-answer20/구조20/pair40/churn10×전이는 출발 예시이며 출시 최소량 의무가 아니다. query/page/exact span/answerable/corpus_revision을 기록하고 언어별 Recall@k/span recall/nDCG, pair precision과 calibration을 별도 보고한다. tuning/holdout 분리와 label 모호성 검토, churn의 old-evidence0 검사는 수용한다. LLM의 abstention을 검색 코드에 강제하는 것은 기각한다.

**SEARCH-11.** 페이지 best chunk의 exact tie도 seq 등 명시 순서로 고정해야 한다. SQL `ORDER BY` 하나로 map 순회 이후의 semantic/RRF tie가 해결되지 않는다. 독립 k1 반복에서 semantic 3slug/RRF 2slug가 모두 winner가 됐다. keyword/recall의 실제 순서 변동은 새로 관측하지 않아 보증 보강으로만 둔다. score 뒤 normalized slug, chunk seq를 total order로 사용하고 가까운 점수까지 EPS로 평탄화하지 않는다. `TestDeterministicRankingTies`는 200회/DB reopen/여러 process에서 결과·k 경계·best snippet을 확인한다. `TestBridgeUniqueDeterministic`의 self/대칭중복/reverse link/dismiss/cooldown은 정상 bridge 속성을 유지하는 검사이며 그곳의 nondeterminism을 주장하는 증거가 아니다.

**SEARCH-12.** heap에서도 tie/cooldown/dismiss/include-linked를 보존해야 한다. candidate eligibility를 먼저 적용하고 score·명시한 tie 순서로 n개만 보관한 뒤 excerpt를 만드는 한정안은 수용한다. heap은 pairwise O(P²D) dot이나 PageVectors 메모리를 없애지 않는다. n=0 정책도 SEARCH-07과 함께 고정해야 동등성 검사에 의미가 있다.

cap 전에 global top-k를 자르면 결과가 누락된다. 독립 fixture A1=1/A2=.99/B1=.8에서 k2/cap1의 현 결과는 A,B지만 먼저 top2를 자른 뒤 cap을 적용하면 A만 남았다. 따라서 SearchChunks bounded 구조/shared scan은 page best, per-page cap, chunk/snippet 선택을 모두 보존해야 한다. `TestBridgeBoundedCandidates`는 dense1000p의 최종 top5와 eligibility/tie를 기존 exhaustive 기준과 대조하고 보관 candidate≤n을 계측해야 한다. `BenchmarkRetrievalScale`의 1k/5k/10k·실제길이 text·k/cap 조합은 기록 가능한 제안이다. 기존 121MiB의 절반은 기계/fixture·단위가 고정된 후속 목표일 뿐 정확성 인수선으로 채택하지 않는다. 지원 규모/budget 문서 대안도 가능하나 초과 결과를 무관함의 빈결과로 숨기지 말고 partial/error를 명시한다. **미해결:** 실제 위키 분포에서의 우선순위와 latency/메모리 예산.

**SEARCH-13.** cosine≥0.988의 표본과 baseline profile은 읽은 기록만으로 확인할 수 없다. 이 숫자나 `~40ms`가 거짓이라고 결론내리지도 않는다. “품질 유지”는 ranking·bridge threshold 보존을 측정하지 않았다면 **조건·표본 미공개의 vector 일치 경험값** 범위로 줄이거나 검증 자료가 준비될 때까지 삭제하는 것이 타당하다. mean↔mean의 높은 cosine은 공식 CLS 계약 적합성을 입증하지 않는다.

작은 benchmark/eval 스크립트·corpus manifest·원자료 공개와 측정 경계 명시는 수용한다. chip/RAM/commit/모델graph+weight/tokenizer/pooling/thread/batch/corpus 크기·길이/warm·cold/CLI·HTTP·kernel을 구별해야 한다. `scripts/bench-search --fixture … --profile … --runs 30 --json`은 **존재하지 않는 후속 제안**이며 30이라는 횟수 자체를 의무화하지 않는다. fp32/int8·mean/CLS를 같은 relevance 집합에서 비교하는 안은 SEARCH-04/10과 묶어 유보된 실물 평가로 남긴다. README 한국어를 고치면 영어 mirror와 `git hash-object` 마커를 같은 변경에서 갱신하고 `make i18n-check`로 확인한다.

## 운영·새 RED ID 전수 중재 행렬

| ID | 원 주장·제안 | RED의 반론·방어 결론 | 독립 근거 | 주장 판정 | 해결안 판정 | 최종 심각도·실제 전제 | 한정 해결·측정 인수 |
|---|---|---|---|---|---|---|---|
| OPS-01 | CI/release F2·CLI/native/browser/P4 공백. smoke·root분리·artifact gate | F1/기존JS green, native 매번 비용 인정. release failure 주장 아님; P4 boot 정상 | initial:tests/ui-tests; M:p4_boot_control; S:ci/release/goreleaser | **narrow** 보증 공백 | **narrow** binary F2/flags/거부 smoke accept; root전면분리·모든gate defer | 낮음; 해당 연결 경계를 바꾸는 릴리스 | 실제publish commit/artifact와 smoke 매핑; mandatory smoke 실패가publish를 막음; native skip 구별 |
| OPS-02 | broken model available/ready·재시도skip. cheap validate/manifest/repair/deadline/temp | 존재≠추론준비 반론은 ready 문구·cached skip을 구하지 못함. 정상 단일파일 rename 보호 유지 | R:model-status-*/model-pull-cached-*; S:engine:43–49,main:489–498 | **accept** 거짓ready; revision혼합/hang/race는 defer | **narrow** cheap complete/repair accept; 정상custom/int8 거부·매status대형hash reject | 중간; 이미 incomplete/corrupt path 존재. keyword 정상 | bundle별 required 정의, valid symlink 허용; fake 중단/repair·정상old 보존; inferred readiness 분리 |
| OPS-03 | 지원 fence의 wikilink 전처리로 Mermaid파괴. 공통code 제외/AST/regex/canonical | 동일 bundle·3-tick 정상 인정. tilde/4-tick/quote 실제 변형은 남음 | M:mermaid_parity; R:cli-tilde-new/cli-tilde-lint; S:render:47–114/wiki:39–48 | **accept** | **narrow** code-region 제외 accept; AST roundtrip·canonical축소 reject | 중간; 지원 fence에서 `[[...]]` Mermaid node | block source==DOM textContent, 양쪽parse성공/code-link0; alias/rawHTML/Chroma controls |
| OPS-04 | A3 required tags 누락 lint미검출. presence/type/date 또는 docsoptional | scalar는 관용parser, date형식은 A3 key계약과 별개 | R:cli-missing-tags/cli-tags-*; S:lint:89/wiki:99 | **accept** key계약 불일치; type/date 확대 narrow | **narrow** presence 또는 명시optional accept; scalar/date즉시금지 reject | 낮음; 외부frontmatter tags key 생략 | 각required키 제거 진단, []허용, malformed YAML count 포함; 기존scalar 정책 보존 |
| OPS-05 | 중복tags로 page count/pct과대. audit dedup/optional lint/occurrence | 외부편집만 전제 아님: CLI new도 중복허용. 정상2p count2 보존 | R:cli-duplicate-new; F:one-page-three-tags/two-pages-control | **accept** | **accept** exact-tag per-page dedup; lint/별도occurrence optional | 낮음; 반복된 유효tag | count≤pages,pct≤100; form/unknown/overbroad도pagecount; audit전후파일불변 |
| OPS-06 | tasks/web helper가 HOME eventDB잔존. helper/Makefile격리·sentinel | hash분리/non-authoritative라 데이터손실 아님. Makefile만으론 direct test보호못함 | R:task-sentinel/sentinel-files; F:web-task-sentinel/web-sentinel-files | **accept** 격리결함 | **narrow** 공용helper state/cache 격리 accept; 모든HOME무쓰기 reject | 낮음; 상속HOME에서 해당test 수행 | direct go test sentinel canopy namespace파일증가0; t.Setenv/t.Parallel 정책 명시 |
| OPS-07 | MathJax autoload/require local404·offline과장. 작은확장/core/fullbundle | CDN의존·실제typeset실패는 미입증, 기본수식전부실패 아님 | M:asset_route; F:mathjax-loader; 공식3.2 docs | **narrow** support/route공백; browser실패 defer | **narrow** 지원corpus+필요extension accept; full/core제한은 선택 | 낮음; 선언한 지원식이 추가component 요구 | offline real browser에서404/외부요청/error node0; unavailable이면 route만 통과 |
| OPS-08 | Make native darwin-arm64 고정/digest없음. tuple/hash/stage/failfast | 현재arm64·formula정상. cross/native실제실패·공격은 미입증 | R:deps-dry-*; S:Make:9–12/53–57,formula:24–51 | **narrow** 선택·보증공백 | **narrow** 지원host+digest/stage 또는failfast accept; 자동cross보장 reject | 낮음; nonarm64 직접build/잘못된cached lib | tuple별URL/hash 또는download전오류; fake 실패/hash mismatch old보존; 실제native별도 |
| OPS-09 | JS upstream notice/provenance경로 부족. ledger/licenses/archive/bare/manifest/distro | 일부font/Mermaidnotice존재. archive실물·법률전수판정 없음; CLI조회 의무 근거없음 | S:asset hashes/string count/goreleaser:31–37; upstream tagged licenses | **narrow** 두자산 고지경로공백; 법률결론 defer | **narrow** exact license/필요notice동봉 accept; fullledger/SBOM/전면vendor교체 defer | 낮음~중간; 해당bytes재배포 | 실제archive 고지사본·bare전달경로 확인; hash/link만으로license대체금지 |
| OPS-10 | quantize HOME하드코드/XDG불일치·직접final write. resolver/args/temp | default에서는맞음. 실제양자화실패·quality실험 없음 | R:quantize-path; S:quantize:6–19/config:49–50 | **accept** 경로결함; activation사고 defer | **narrow** XDG/명시args accept; bundleactivation은정책후defer | 낮음; custom XDG에서 선택적script실행 | ONNX import없이default/custom/explicit paths; stub실패ready0/old보존; 수치실험별도 |
| OPS-11 | ORT search 음수panic. engine/index전검증·internal safe·state불변 | localinvalid/default정상, lite는 stack없음오류. J3 startup stamp허용필요 | 원본 ORT stack; M:count_bound; R:bridge-negative; S:main:56–63 | **accept** 원본localCLI; 독립ORT재현 defer | **narrow** 조기검사 accept; 무조건모든state불변 reject | 낮음; ORT지원/가용후negative count | migration완료baseline, invalid=-1에exit1/panic0/model/index호출0/wiki·cache불변 |
| RED-B-01 | curl오류exit를tar성공이숨김/다음depsskip. tmp검증 또는verifiedartifact정책 | 완전archive는오류exit여도유효할수있음. 현재는digest검사위치없음 | R:failed-curl-complete-tar/toy-artifact/next-deps-attempt; S:Make:57 | **accept** status소실; 실제손상사건 reject | **narrow** transport+digest+stage accept; blanketpipefail reject | 낮음; curl실패이나tar성공/파일존재 | 오류transfer/hash mismatch는ready0·old보존·재시도검증; 승인된digest예외정책만별도 |

### 운영 질문에 대한 직접 답변과 인수 기준 보정

**OPS-01.** main CI 통과가 태그의 정확한 commit에 연결됐다는 실행 증거는 없다. J7은 ancestor 계보를 확인하며 같은 commit의 CI success를 검사하지 않는다. 정상 `make release-tag`가 release-check를 먼저 호출하는 수동 보호는 있지만 직접 tag push도 workflow trigger이고 그 release job은 tests/F2를 재확인하지 않는다. 현재 필수 F2 smoke 하나를 깨뜨려도 publish를 막는 별도 job은 **없다**. 이를 실제 실패 릴리스의 증거로 확대하지 않는다.

최소 개선은 `CGO_ENABLED=0` 바이너리로 F2의 실제 mode/hit/warning/exit, F4의 기본 위키/cwd, 잘못된 플래그, 쓰기 거부 전후 파일 불변을 검사하고 publish가 소비하는 commit/artifact와 연결하는 것이다. root 생성 함수 분리는 테스트 지원 선택이며 CLI framework 변경은 필요 없다. GoReleaser가 다시 만든 bytes와 다른 바이너리의 smoke를 동일 artifact 검증으로 부르지 않는다. ORT compile/missing-model 강등·library load·inference는 별도 결과로 기록하고, 승인된 기존 모델로 수동/릴리스 smoke를 선택할 수 있다. 매 PR 모델 다운로드와 모든 browser/native gate는 요구하지 않는다. P4 boot fault는 독립 probe에서 정상 fail-open이므로 sandbox 전면 재설계 근거가 없고 timeout/unsettled-promise 주입은 후속 검사다. 실제 DOM/vendor browser 검사는 해당 경계 변경에 맞춘다. **인수:** 필수 smoke 실패 주입이 publish의 의존 job을 실패시키는지 검증한다. 이 작업에서 workflow를 실행했다는 뜻은 아니다.

**OPS-02.** 현재 `available` 구현은 존재만 뜻하지만 `model pull`의 `ready`는 최소 **다운로드 묶음 완료**로 정의하는 것이 타당하다. 그와 `inference-validated`는 구분해야 한다. custom self-contained graph까지 무조건 원격 기본 6파일을 요구하지 않는다. default downloader의 required 묶음, custom/quantized 모델의 graph가 실제 참조하는 파일과 tokenizer 계약을 각각 정한다. symlink가 가리키는 유효 regular 파일은 보존할 수 있으며 모든 symlink를 거부할 근거는 없다. 기존 수동 양자화 모델은 확인/adoption 후 계속 사용하고 repair 전에 지우지 않는다.

저비용 nonempty/파일 종류/JSON 검증과 명시적 repair/force는 거짓 ready를 고치는 최소안이다. 동일 revision/digest manifest를 마지막에 기록하는 것은 completeness 검사를 거친 뒤에만 의미가 있다. status에서 매번 큰 weight를 hash하거나 manifest 없는 정상 모델을 일괄 거부하는 안은 기각한다. fake transport로 missing sidecar/0B/손상 JSON/중간 실패/재시도/정상 old bundle 보존을 검사한다. mutable main 혼합, deadline 없는 hang, `.part` 충돌은 **보증 개선 제안**으로 수용하되 사건으로 확정하지 않는다. context/deadline·고유 temp를 이 downloader 경계에 한정하고 취소 시 ready 없음·old bundle 불변을 검사한다. 최종 ready가 실제 추론 가능까지 뜻한다면 별도 native validation 단계가 필요하며 이번 lite fixture는 그 단계의 증명이 아니다.

**OPS-03.** AST를 쓰더라도 Markdown을 roundtrip serialize해 원문을 바꾸는 수정은 권고하지 않는다. goldmark로 code block/span 범위를 알아낸 뒤 원본 범위 밖에서만 기존 wikilink 치환·추출을 수행하거나 현재 renderer 확장 경계를 쓰는 작은 구현을 검토한다. raw HTML, alias/anchor, Chroma, Mermaid extender 및 기존 newline은 현재 처리를 유지하는 control fixture가 필요하다. regex 길이 확대는 중간 완화일 수 있지만 container/indented/span까지 해결했다는 acceptance로는 부족하다. canonical 3-tick만 허용하는 대안은 이미 지원하는 fence를 줄이므로 작은 버그 수정으로 채택하지 않는다.

독립 source parity는 3/4/tilde/blockquote에서 element에 전달될 decoded text와 원문의 동일성 및 같은 parser의 parse 성공 여부까지 확인했다. **actual browser parse/SVG는 아직 안 했다.** 원본 인수의 “indented fence”는 CommonMark에서 mermaid fenced block으로 해석되는 경우와 단순 indented code를 구분해야 한다. 렌더러가 mermaid로 지원하는 모든 경우는 Blocks Source와 DOM textContent 동일, 일반 code 영역은 wikilink0을 검사한다. 외부 정상 `[[target#anchor|alias]]`, rawHTML, 강조/code/newline control을 보존하고 P3 bundle equality도 유지한다. browser가 없으면 parser-input까지 통과라고 기록한다.

**OPS-04.** tags key 존재만 A3 문제라면 presence bool 하나로 충분하다. 모든 frontmatter 타입을 보존하는 재설계는 필요 없다. scalar tag를 없앨 현재 계약 근거/호환 경로는 없으므로 즉시 sequence-only 거부는 기각한다. 날짜 nonsense는 별도 date validity 계약과 시간 기능의 영향 검사가 있어야 하며 A3 필수 키 결함에 끼워 넣지 않는다. 최소 권고는 tags presence 검사다. omitted=empty를 의도했다면 A3를 optional로 명시하는 대안도 있지만 lint를 그대로 두고 계약이 맞는다고 주장할 수는 없다. 각 required key를 하나씩 제거하는 table, 빈 목록 허용, 기존 scalar 관용, malformed YAML의 `bad-frontmatter` 진단을 포함한다. A3 점검식의 두 count만으로 bad-frontmatter를 놓치는 절차도 함께 보정해야 한다.

**OPS-05.** audit dedup는 원문을 손대지 않는다. exact tag membership 그대로 페이지 내부 seen set만 적용하고 alias/대소문자 통합까지 섞지 않는다. 중복은 CLI new에서도 들어갈 수 있어 외부 편집 전제는 제거한다. `F:one-page-three-tags`는 count=3/pct=300, `F:two-pages-control`은 count=2/pct=100이다. 인수는 topic/form/unknown 및 overbroad가 동일 page count를 사용하고 `0≤count≤pages`, `0≤pct≤100`인 것, 다른 두 페이지의 같은 tag는 2회인 것, 실행 전후 file hash 불변이다. duplicate-tag lint와 별도 occurrence metric은 선택이며 page count를 대체하거나 자동 retag/taxonomy split을 수행하지 않는다(S5/LLM 판단 보존).

**OPS-06.** t.Setenv는 process 전역이므로 이를 쓰는 helper는 t.Parallel과 조합할 수 없다. 현재처럼 비병렬 fixture helper에서 state/cache를 각 TempDir로 설정하고 향후 병렬화 시 의존 경로 주입/격리 subprocess 등 별도 해결을 택해야 한다. Makefile wrapper만으로 IDE/direct go test를 보호하지 못한다. tasks와 web tasks 모두 fresh sentinel에서 각각 20,480B DB가 생겼다. 인수 범위는 `.local/state/canopy`/canopy cache namespace이며 SDK telemetry/컴파일 파일은 별개다. helper 실행 전후 sentinel의 canopy 파일 증가 0, fixture 안에만 DB 존재를 검사한다. production logging 제거/실제 HOME 청소/사용자 데이터 손상 주장은 모두 범위 밖이다.

**OPS-07.** 선언된 지원 수식 corpus는 발견하지 못했다. 권고는 기본 수식과 실제 지원할 extension(예: color/physics)의 대표식을 명시하고 같은 3.2.2의 의존 component 집합을 vendor하는 것이다. 독립 loader 검사에서 color autoload 설정과 같은 origin URL, handler 404를 확인했으나 **실제 browser가 요청한 component나 보여준 error node는 확인하지 못했다.** 이 질문은 browser 실행까지 defer다. [MathJax 3.2 autoload 문서](https://docs.mathjax.org/en/v3.2/input/tex/extensions/autoload.html)도 추가 component 로딩 계약을 설명할 뿐 내 browser 결과를 대신하지 않는다.

core 제한/require·autoload 끄기는 지원 호환성 변경이며 선택하면 지원 밖 매크로에 명확한 진단을 줘야 한다. full bundle은 bytes/license/update 비용을 확인한 뒤 선택하고 arbitrary require 전부 지원 의무는 기각한다. CDN fallback은 로컬 목적과 맞지 않는다. 인수는 지원 corpus를 network 차단 browser에서 렌더하여 404/외부 요청/error node가 0이고 필요한 component manifest/route가 완결된 것이다. browser unavailable이면 manifest/route 검사만 pass로 표시하고 typeset은 skip/unverified로 남긴다. **미해결:** 실제 지원 corpus·브라우저 사용자 영향.

**OPS-08.** 선택은 native 빌드를 수행하는 host와 CGO target이 일치하는 지원 tuple부터 정해야 한다. 지금 Makefile의 GOOS/GOARCH를 바꿔도 URL은 darwin-arm64다. 자동 cross compile까지 보장하지 말고 host≠target/unsupported tuple이면 download 전에 명시적으로 거부하는 것이 작은 대안이다. 네 지원 host를 받는 정책이면 formula의 version/URL/SHA256과 같은 원자료를 쓰거나 정적 parity 검사로 drift를 막는다. Apple Silicon만 직접 build를 지원하는 fail-fast 대안도 문서에 명시하면 가능하다.

staging에서 archive digest·기대 파일을 검증한 뒤 기존 lib를 교체하고 failure/retry/hash mismatch에 old 정상 bytes를 보존해야 한다. 현재 XDG layout을 바꿀 필요는 없고 새 platform별 layout을 도입하면 machine-local migration 규칙을 별도 검토한다. Go sum이 static archive를 검증하지 않는다는 원본 한계는 유지한다. dry run/fake tar는 선택·실패 정책의 증거이며 actual host native link/inference와 타 플랫폼 런타임은 별도 결과다. 원본 digest 제안과 새 RED 실패 exit 제안은 동일한 작은 installer 경계에서 통합할 수 있다.

**OPS-09.** exact release archive에 다른 고지 파일이 동봉된다는 증거는 없다. 읽은 설정은 README.md+root LICENSE이며 source 내 font 고지와 Mermaid 주석이 존재해도 force-graph/MathJax 고지를 충족하는 경로라고 확인할 수 없다. bare binary를 전달할 때 license 사본을 주는 경로도 확인하지 못했다. [force-graph v1.51.4 MIT](https://github.com/vasturiano/force-graph/blob/v1.51.4/LICENSE)는 귀속·허가 고지를 요구하고 [MathJax 3.2.2 Apache-2.0](https://github.com/mathjax/MathJax/blob/3.2.2/LICENSE)는 재배포 시 license 사본 제공 조건을 둔다. 이는 정확한 두 자산의 고지 경로 점검이며 침해 사건/전체 전이 라이선스의 법률 판정은 아니다.

최소 해결은 해당 version의 full license와 필요한 notice를 source 및 실제 배포물에 동봉하고 bare 전달 정책을 정하는 것이다. embedded 조회 경로 또는 동봉 고지를 선택할 수 있으며 라이선스가 특정 CLI `--licenses`를 명령한다는 근거는 없다. source/version/hash/가공 방법 ledger는 업데이트 추적에 유용하지만 license 사본을 대체하지 못한다. 모든 asset manifest/hash/폰트·Go·Rust·model·native까지 전수 SBOM은 별도 범위로 유보한다. 원본의 “모든 vendor asset” 인수는 두 자산 고지 수정의 필수 조건으로는 과하다. 최소 인수는 실제 archive 추출/embedded 또는 동봉 조회에서 정확한 license·notice 확인, 누락 fixture 실패다. 표준 distro 교체는 크기·동작이 달라질 수 있어 고지 추가에 끼워 넣지 않는다. **미해결:** actual archive/bare 전달 확인과 transitive 고지 검토.

**OPS-10.** 이 작업은 외부 sidecar 상대 경로를 보존하는 실제 변환/활성화를 확인하지 못했다. graph 파일 하나만 rename하면 묶음의 atomic activation이 된다는 안은 기각한다. 기존 nonempty directory는 단순 rename 교체가 안 되므로 기존 묶음을 그대로 둔 새 완료 directory를 검증 후 선택하거나 명시적 force/backup 정책을 정해야 한다. 최소 XDG 경로 해결과 이 큰 activation 계약은 분리할 수 있다.

Go와 같은 XDG fallback/명시 src·dst 인수와 경로 출력은 수용한다. path resolver는 ONNX import 없이 default/custom/explicit 경로를 검사하고 destination이 runtime 설정으로 선택 가능해야 한다. stub converter 실패는 incomplete를 ready로 승격하지 않음/old 보존만 입증하며 int8 수치·속도·Python package 호환성을 증명하지 못한다. 개인 spike 한정+custom XDG 수동 안내 대안도 가능하지만 사용자 문서에서 일반 옵션처럼 안내하는 현재 상태는 설명해야 한다. 이 대안을 선택하면 한국어/영어 mirror·hash 동시 갱신을 포함한다. 자동 Python/모델 설치는 추가하지 않는다. **미해결:** 기존 output 교체 정책과 external data 묶음 검증.

**OPS-11.** invalid count의 불변 검사 baseline은 **migration 완료 뒤**로 정해야 한다. J3에 따르면 첫 명령의 schema stamp는 합법적이며 실제 PersistentPreRunE가 RunE count 검사보다 먼저 돈다. 그 write를 없애려고 startup 순서를 뒤집는 안은 기각한다. 0 정책은 SEARCH-07 권고(양수 CLI), HTTP 기존 default 보정/per-page 0 unlimited와 함께 명시한다. 원본 ORT CLI stack은 SearchSemantic:127/main:419에 도달한 독립 선행 기록이고 내가 새 ORT 실행을 한 것은 아니다. 내 lite bridge panic과 내부 semantic panic은 각각 그 자신의 도달 범위만 입증한다.

인수는 bootstrapped fixture에서 keyword/semantic/hybrid·recall/bridge의 -1/0/1/large 입력을 검사하고 invalid는 exit 1·명확한 stderr·panic 0·engine/index 호출 0·wiki/cache 불변이다. model unavailable 환경의 오류와 valid ORT 음수 검사를 구별하고 native 없는 내부 slice 검사를 별도로 둔다. source·실제 HOME를 건드리지 않는 CLI harness로 가능하며 무조건 모든 XDG 파일 불변은 아니다.

**RED-B-01.** 프로젝트의 현재 구현 성공 기준은 pipeline의 마지막 tar status와 최종 파일 존재다. transport status와 verified artifact를 구분하는 digest/완료 검사 위치는 **없다**. 내 완전 toy archive+curl exit 7에서도 make exit 0·다음 skip을 확인했으므로 status 소실 주장은 수용한다. 그렇다고 완전 archive가 실제로 손상되었다고 말하지 않는다.

권고 정책은 transport 성공과 정확한 digest 검증을 둘 다 요구하고 임시 archive에서 검증·추출·교체를 수행하는 것이다. `/bin/sh`에 무조건 `set -o pipefail`만 추가하는 해법은 기각한다. curl status를 직접 검사하면 이식성 문제와 의도하지 않은 정상 old artifact 삭제를 피할 수 있다. 인수는 첫 설치 실패 시 최종 ready/파일 없음·다음 시도 검증 재실행, hash mismatch/추출 실패 시 old 정상 bytes 보존, 정상 toy archive+승인 digest 성공이다. 기존 final이 있는 경우의 검증/명시 repair 정책도 정해 단순 존재만으로 영구 skip하지 않게 해야 한다. **미해결 이견:** 정확한 digest를 통과한 artifact라면 transport 오류를 허용할 정책도 논리적으로 가능하다. 이를 선택하려면 예외를 명시·검사해야 하며 현재 코드는 그 정책을 구현한 것이 아니다.

## 후속 인수 검사의 실행 경계

위에서 이름을 든 새 Test 함수와 benchmark는 **아직 구현하지 않았다**. 후속 변경은 먼저 해당 불변식·검사 명령을 invariants에 적고 아래 진입점에서 실제 케이스/주입/기대값이 등록됐는지 확인해야 한다. 이름만 존재하거나 `[no tests to run]`인 결과는 인수 통과가 아니다. 모든 실행은 이 보고서 실행기와 같은 격리 환경을 전제로 한다.

```sh
go test -count=1 ./internal/indexer -run '^(TestReindexClearsEmptyBody|TestDegradedIndexDoesNotServeStaleChunks|TestReindexEmbeddingProfile|TestRecallSourceRoundTrip)$'
go test -count=1 ./internal/embed -run '^(TestSplitPreservesSource|TestEmbeddingInputBudget)$'
go test -count=1 ./internal/config -run '^TestEmbeddingSettingsHaveEffectOrReject$'
go test -count=1 ./internal/store ./internal/search -run '^(TestVectorCompatibility|TestTopKBounds|TestMultilingualLiteralRetrieval|TestDeterministicRankingTies)$'
go test -count=1 ./internal/webui -run '^(TestAPISearchColdAndChanged|TestHybridInferenceFailureFallback|TestSearchCancellation)$'
go test -count=1 ./cmd/canopy -run '^(TestBridgeChecksRevision|TestSearchRecallBridgeLimits|TestHybridRuntimeFallback)$'
go test -count=1 ./internal/resurface -run '^(TestBridgeUniqueDeterministic|TestBridgeBoundedCandidates)$'
go test ./internal/store -bench '^BenchmarkRetrievalScale$' -benchmem
```

OPS의 태그·lint·helper·Mermaid 경계는 위에서 특정한 fixture/assert를 `go test -count=1 ./internal/lint ./internal/tasks ./internal/webui ./internal/mermaid ./cmd/canopy`에 실제 등록한 뒤 확인한다. model/downloader·Make deps·양자화 stub·license packaging은 위 절의 실패/복구 케이스를 추가한 CLI/가짜 transport harness로 검사하며 기존 suite green만으로 대체하지 않는다. OPS-01의 publish gate 실패, OPS-07의 offline browser, OPS-09의 actual archive는 각각 별도 실행 결과가 필요하다. ORT parity/eval을 채택할 때만 승인된 reference fixture로 `go test -count=1 -tags ORT ./internal/embed -run '^TestBGEM3ReferenceParity$'` 및 `go test -count=1 -tags ORT ./internal/search -run '^TestGroundedRetrievalEval$'`을 실행하며 compile/load/infer/quality와 unavailable을 따로 기록한다.

## 남는 이견·제안 상태와 종료 확인

우선 회복할 계약은 현재 원문과 반환 근거의 연결(SEARCH-01/02/05), 지원된 embedding profile의 식별(SEARCH-03/04), 정상 Mermaid source 전달(OPS-03), 사용 불가 bundle의 거짓 ready(OPS-02)다. 검색 품질 평가·runtime fallback·즉시 취소·대규모 budget·MathJax 지원 범위·배포 실물 고지는 각 절의 미해결 판정을 유지한다. 형태소 엔진/ANN/외부 DB/전체 모델 재수출/전면 CLI 재작성으로 확대할 근거는 없다.

새 독립 결함/제품 기능 제안은 만들지 않았다. normalized hash·cap 반례와 positive controls는 배정된 원 ID의 주장/수용 조건을 보정하는 증거다. 이 보고서의 해결안은 모두 **미구현·최종 독립 검토 전 제안**이며, 추후 별도 새 주장/제안이 생기면 `DEF-B-NN`으로 식별하고 최종 독립 검토 전에는 validated로 올리지 않는다. 전수 행렬의 accept는 현재 관측 주장의 판정이지 제안 구현의 검증 완료가 아니다.

주 실행의 before/after에서 HEAD·Git index SHA256·tracked diff·배정된 원본/RED 보고서 SHA256이 같았고 tracked diff는 빈 값이었다. 두 실행기의 fixture는 삭제했고 `review_probes/def_search`도 제거했다. 최종 파일 검사·25행 누락 검사·HEAD/index/원본 보존 확인은 [최종 확인](evidence/def_search-final-check.json.txt)에 남긴다. 기존 source/docs/원본/RED 수정, commit/release, 실제 wiki/HOME/XDG/native/model/service 변경이나 nested worker는 없다.
