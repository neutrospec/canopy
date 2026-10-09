# 작은 검색 holdout과 측정 계약

PLAN-10의 평가 fixture는 `internal/search/testdata/evaluation/corpus.json`이다.
첫 측정 전에 corpus·기대 page·정확한 supporting span·0/1/2 relevance label·판정 규칙과
아래 수용 조건을 고정했다. 6개의 수작업 페이지와 10개의 질의는 한국어·영어·혼합·
기호·코드·근거 없는 질의를 포함한다. 이 집합으로 튜닝하지 않는다. 일반 언어 품질,
오류율, 속도 우위 또는 실제 사용자 위키를 대표한다는 주장은 하지 않는다.

판정자는 구현 에이전트가 작성한 고정 label이고, 실행 중 답변을 생성하거나 LLM으로
relevance를 재판정하지 않는다. 관련성 2는 질의가 요청한 사실의 supporting span,
1은 관련 개념, 0은 그 외다. unsupported는 후보 자체를 허위 답변으로 세지 않는다.
검색기는 답변하지 않으며 nearest candidate 반환과 evidence support는 별개다.

baseline은 기존 unicode61 quoted-prefix AND, 기존 mean-if-3d/L2 semantic 공간,
기존 RRF(60)이다. 수정은 캐시 정합성·원문 범위·bounds·결정론·후보 메모리이며
새 형태소 분석기·CLS·reranker·ANN·threshold 튜닝을 넣지 않는다.

**측정 전 수용 조건:** literal 5질의의 keyword Recall@3=1.0, 모든 반환 raw text가
source revision/byte range와 정확히 일치, unsupported 2질의의 keyword hit=0이다.
조사 변형·paraphrase·semantic/hybrid의 Recall@3, exact supporting-span coverage,
nDCG@3는 공개하는 설명 수치이며 보편적 합격선을 만들지 않는다. 8개 answerable과
2개 unsupported를 분리한다. official CLS parity와 fp32/int8 ranking 비교는 별도 미검증이다.

```bash
go test ./internal/search -run TestGroundedRetrievalEval -count=1 -v
# 환경변수가 없거나 ORT가 없으면 semantic은 unverified/skip이다.
CANOPY_TEST_MODEL=/explicit/read-only/model/path go test -tags ORT ./internal/search -run TestGroundedRetrievalEval -count=1 -v
go test ./internal/store -run '^$' -bench BenchmarkRetrievalScale -benchtime=3x -benchmem
```

실행 시 fixture SHA256, profile, 실제 질의 수, 각 query/mode의 page Recall@3,
span coverage, nDCG@3와 unsupported 후보 수를 JSON log로 남긴다. label과 결과를
보고 수정하는 경우 새 fixture version과 새 holdout을 따로 만든다.

독립 검토에서 두 측정 한계를 확인했다. unicode61은 `C++ API`와 `C API`를
같은 token으로 취급하며 `++`만 있는 질의는 keyword hit가 없다. 현재 corpus의
기호 페이지 hit는 C와 C++의 구별을 증명하지 않는다. 구별이 필요한 literal 확인은
`canopy grep 'C\+\+' --wiki "$W"`처럼 원문 regexp 경로를 사용한다.

`retrieved_supporting_spans`의 `span_basis`는 semantic에서 `semantic_top3`, hybrid에서
`semantic_top3_intersect_hybrid_pages`다. hybrid의 page ranking에 남은 페이지 중
별도로 조회한 semantic top-3 chunk에 포함된 span만 센다. keyword-only로 들어온
hybrid 페이지의 전체 근거 coverage나 별도 hybrid chunk retrieval의 측정이 아니다.
keyword의 span coverage는 미측정이다. 기존 구현 보고서의 hybrid 9/9 수치는 이
제한된 교집합 집계로 읽어야 하며, 독립적인 hybrid 근거 회수율로 일반화하지 않는다.

성능은 synthetic 200/1000 페이지, 페이지당 5 chunk, chunk당 1024 float32,
약 1000 UTF-8 bytes의 raw text, query axis와 같은 unit vectors(동률이 높은 상한 사례)를
사용한다. k=8/cap=2로 bounded 후보와 late excerpt를 측정한다. benchmark의 누적
allocation은 `TotalAlloc`/B/op이고 RSS 또는 peak heap이 아니다. wiki scan/model load/
inference/HTTP를 포함하지 않는 kernel 측정이다. benchmark 반복 3회는 SLO의 통계적
추정이 아니다. 첫 DB query와 이어지는 query를 별도로 기록하며 OS page cache를
강제로 비우지 않은 첫 query를 진정한 cold disk라고 부르지 않는다.

결과 보고서 `docs/implementation/2026-10-10/reports/search.md`는 실제 hardware,
RAM, OS, Go/native toolchain, corpus 분포·pair 밀도, warm/first-query, 반복 수, 포함된
비용과 미실행 항목을 함께 기록한다. bridge pair 후보 메모리는 root 담당 구현의
별도 검증이며 이 vector retrieval benchmark의 결과로 대체하지 않는다.
