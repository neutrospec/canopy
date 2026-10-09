# 판정 원장

대상은 v0.10.0, `ede67e302a8487aecbb4d59587a8686292018cf6`이다. 이 표는
조정자가 원 보고서·독립 반박·재반박의 증거와 제한을 대조해 내린 판정이다.
다수결이나 최초 심각도의 평균이 아니다. 원문과 조건은 각 토론 보고서에서 확인한다.
구현은 변경하지 않았으며 `accept`는 수정안의 구현·수용 시험 통과를 뜻하지 않는다.

`accept`는 해당 범위 채택, `narrow`는 조건·주장·처방 축소, `reject`는 제시된
결함/의무 기각, `defer`는 필요한 근거나 제품 선택이 남음을 뜻한다. **주장과 처방을
따로 판정**한다. 낮음/중간/높음은 개인 위키의 보존·정합성·사용 영향을 가리키며
취약점 점수나 자동 릴리스 차단 목록이 아니다. 같은 원인의 여러 ID를 결함 여러 개로
세지 않는다. 조건부·선택 제안은 정상 사용의 확정 결함과 구별한다.

기계 원장은 [adjudication.json](adjudication.json), 실행 순서와 미래 수용 조건은
[개선 계획](plan.md)에 있다. 아래 표의 근거 ID는 해당 영역 토론 표에서도 같다.
원 보고서 제목이 더 강한 표현이어도 최종 범위는 여기의 한정 문장이다.

종합안에 대한 [최종 반박](13-final-challenge.md)과 [조정자 응답](14-final-response.md)도 함께 보존한다.

## 원문·Git·쓰기

[원 리뷰](01-data.md) · [반박](07-red-data.md) · [재반박](10-defense-data.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| DATA-01 | accept / narrow | 높음 | 정상 두 clone의 겹친 dirty 변경에서 autostash 충돌이 exit 0으로 commit/push됨. Git 양쪽 원문은 복구 가능. **결정:** pull 전후 및 add 전에 미해결/진행 상태를 검사하고 발행을 중단한다. literal marker 금지·정상 dirty pull 전면 금지는 채택하지 않는다. | PLAN-03 |
| DATA-02 | accept / narrow | 높음 | 정상 두 CLI의 create/update 경쟁에서 성공한 본문·metadata를 덮음. 방어의 실제 CLI checkin도 timing 조건에서 직접 쓴 sentinel을 덮었고, 호출 전 변경은 정상 거부했다. **결정:** 협력 writer의 짧은 적용 구간, 최종 base 확인과 no-clobber를 결합한다. 직접 편집까지 CAS로 보호한다는 보장은 제외한다. | PLAN-04 |
| DATA-03 | accept / narrow | 높음 | 쓰기 중 파일 크기 제한 오류에서 원문이 일부로 잘림. 실제 disk full·전원 장애 빈도는 미측정. **결정:** 완전한 임시 파일 게시와 close 오류 확인을 채택한다. mode/링크 의미와 임시 파일 발행을 검사하며 fsync·backup 의무는 별도 결정한다. | PLAN-04 |
| DATA-04 | narrow / narrow | 중간 | 다중 원문/백링크 부분 적용과 파생물 후처리 실패가 다름. index 실패 사례에는 새 본문이 위키에 남음. **결정:** 적용 여부·실패 단계·repair 경로를 반환한다. 사본만 남기거나 같은 명령을 무조건 재시도하는 기준, 일괄 rollback 의무는 기각한다. | PLAN-05 |
| DATA-05 | accept / accept | 높음 | 정상 archive→같은 slug new→archive가 이전 보관본을 무경고 대체함. 미커밋 보관본이면 유일한 원문이 될 수 있음. **결정:** 최소안은 기존 목적지 보존과 충돌 거부다. 고유 suffix는 복원/예약 규칙을 정할 경우의 선택안이다. | PLAN-04 |
| DATA-06 | accept / narrow | 중간 | 링크 추출과 regex 수정의 문법 차이로 지원된 path 링크가 남고 코드 예제가 바뀜. **결정:** Body의 실제 target span과 code 영역을 공유해 수정한다. 정규화 slug만으로 원문 위치를 복원하거나 raw 전체를 재작성하지 않는다. | PLAN-06 |
| DATA-07 | accept / narrow | 높음 | 손상·누락·읽기 실패 meta와 수정 checkout 사본이 함께 있을 때 사본을 덮음. 정상 meta 보호는 동작함. **결정:** 기존 copy의 base를 입증 못 하면 보존하고 orphan을 노출한다. 명시 rescue/discard를 정의하며 meta atomic 저장만으로 해결됐다고 보지 않는다. | PLAN-05 |
| DATA-08 | narrow / narrow | 중간 | 공유 쓰기 후처리와 동일 입력 gate는 다름. 본문 broken link 허용은 현재 lint 계약과 구별해야 함. **결정:** 명령별 구조 gate/no-op/사후 lint를 정확히 설명하고 선택한 필수 구조만 공통화한다. 삭제·복구에 모든 협약을 hard gate로 붙이지 않는다. | PLAN-01, PLAN-06 |
| DATA-09 | accept / narrow | 중간 | 정상 역슬래시 제목과 loader가 수용하는 block scalar에서 값 의미가 변함. unknown/body 보존 양성 대조가 있음. **결정:** known scalar의 의미와 무관한 bytes를 따로 보존·검사한다. 전체 YAML Marshal이나 위치 하나만으로 후속 줄 삭제는 채택하지 않는다. | PLAN-06 |
| DATA-10 | accept / narrow | 높음/중간 | 동시 성공 edit 본문과 feedback 소실. 한 서버의 human read 및 서로 다른 human/agent 파일은 보존됨. **결정:** RMW 전체 보호와 exclusive 완전 task 게시를 각각 검사한다. Save-only mutex·nonce-only ID·웹 mutex의 전역 보증은 기각한다. | PLAN-04 |
| DATA-11 | narrow / narrow | 중간 | agent-only 개명 이력 누락은 확인. 모든 task·cooldown 자동 이관은 기존 보장이 아니며 목적지 이력 덮기 반례가 있음. **결정:** agent 이관에서 목적지 과거 기록을 보존/충돌 처리한다. task base·기각·slug 재사용 정책 없이 과거 참조를 일괄 재작성하지 않는다. | PLAN-06, PLAN-08 |
| DATA-12 | narrow / narrow | 중간 | 정상 sync 후 Markdown index가 최종 tree와 다를 수 있음. cache는 일반 검색으로 회복되므로 영구 검색 불능은 아님. **결정:** sync 완료 시 생성물 정합 또는 명시 repair 상태를 정한다. 무변경 sync의 날짜별 불필요한 commit을 막으며 모든 cache 소비자 refresh를 따로 확인한다. | PLAN-09 |
| DATA-13 | narrow / narrow | 높음/중간 | 기존 duplicate slug에서 type-only 이동이 원문을 대체함. nested 지원과 ledger 영역 차이는 정책 문제도 포함함. **결정:** 목적지 no-clobber와 ambiguity 진단을 먼저 둔다. recursive addressing·새 page identity·외부 원문 자동 이동/삭제는 필수 해법이 아니다. | PLAN-04, PLAN-06 |
| DATA-14 | narrow / narrow | 조건부 중간 | 현재 version의 additive unknown 및 미래 version을 실제 수정해 Save할 때 소실. 평범한 현재 데이터 전체 손실은 아님. **결정:** nested/exact unknown 보존 또는 명확한 쓰기 거부를 택한다. version guard만으로 same-version 확장을 보호한다는 주장은 기각한다. | PLAN-07 |
| DATA-15 | reject / narrow | 선택 제안 | 현재 활동 feed는 rollback·판단 복원 원장이 아님. hash와 이유 추가만으로 과거 bytes가 복원되지 않음. **결정:** 보장 범위 설명을 채택한다. operation ID·복구 원장·event sourcing·본문 DB 복제는 입증된 필수 수정으로 올리지 않는다. | PLAN-01, PLAN-05 |

## 재현 불가 상태

[원 리뷰](04-state.md) · [반박](07-red-data.md) · [재반박](10-defense-data.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| STATE-01 | accept / narrow | 중간 | 상태 RMW·중간 파일 노출을 재현함. 고정 migration tmp의 goroutine 오류는 실제 CLI 36회 성공과 구별함. **결정:** RMW 보호와 고유 완전 게시를 채택한다. Ensure API 경합과 실제 startup 경합을 별도로 검사하며 모든 이행 실패를 입증했다고 하지 않는다. | PLAN-04, PLAN-07 |
| STATE-02 | accept / narrow | 중간 | 정렬된 합법 -- slug 페어가 같은 connect ID로 다른 Pages/기각을 반환함. 작성 본문 손실 사례는 아님. **결정:** payload 동일성 확인과 충돌 진단이 최소안이다. encoding 변경 시 옛 payload·기각을 찾으며 기존 slug를 금지하지 않는다. | PLAN-08 |
| STATE-03 | accept / narrow | 높음 | 같은 page·second 동시 edit 성공 뒤 전문 소실. 기기간 같은 ID는 관측했으나 두 clone 최종 전문 소실까지는 미검증. **결정:** 기기간 충돌 가능성과 로컬 exclusive 완전 게시를 따로 검사한다. nonce만으로 no-clobber를 대체하거나 재시도 idempotency를 필수화하지 않는다. | PLAN-04, PLAN-08 |
| STATE-04 | accept / narrow | 조건부 중간 | 손상/수작업 identity mismatch에서 CLI GC가 pending을 지움. 위키 밖 임의 삭제나 정상 큐의 자연 발생은 미입증. **결정:** 실제 열거한 closed 파일만 지우고 unknown status·mismatch를 보존/진단한다. 한 bad task가 모든 정상 task를 0건으로 보이게 하지 않는다. | PLAN-07 |
| STATE-05 | narrow / narrow | 조건부 중간 | additive/future 필드 및 정확한 큰 정수가 재저장 중 소실·반올림됨. 일반적인 현재 데이터 전체 파괴는 아님. **결정:** nested unknown과 정확한 숫자를 patch하거나 쓰기를 거부한다. TOML 값 보존과 formatting 보존을 구별하며 version guard만으로 충분하다고 하지 않는다. | PLAN-07 |
| STATE-06 | accept / narrow | 조건부 중간 | null read entry의 CLI panic, 비정상 tasks 경로의 접수 hang, status 거짓 0 및 한 문 오류의 영향이 확인됨. **결정:** Stat 오류·null shape를 검사하고 unavailable과 0을 구분한다. 건강한 다른 기록의 부분 활용 여부를 명시하며 원문 열람 전체를 일괄 차단하지 않는다. | PLAN-07 |
| STATE-07 | accept / narrow | 높음/중간 | meta 오류 뒤 edited copy 덮기는 손실, 회수 실패 뒤 파일 잔류는 별도 문제. 정상 modified 보호는 작동함. **결정:** orphan 보존·노출 및 remove 결과를 전달한다. 이미 위키에 적용된 뒤 회수 실패한 상태는 단순 실패·blind retry와 구분한다. | PLAN-05 |
| STATE-08 | accept / narrow | 중간 | agent-only mv의 caller 조건 누락. 목적지 page 없이도 옛 목적지 이력이 남아 있을 수 있음. **결정:** agent-only 이관에서 destination 기록 병합/충돌 기준을 정한다. H3 literal 실패나 모든 과거 참조 자동 이관 의무로 확대하지 않는다. | PLAN-06 |
| STATE-09 | accept / narrow | 중간 | mixed offset의 event Query/TopConsumed/Prune 및 RecentAccess 정렬 오류. 실제 CLI GC가 cutoff 이후 기록도 지움. LastAccess는 정상. **결정:** 기존 행까지 instant 기준으로 조회·보존한다. parsed 실패를 오래된 행으로 지우지 않으며 필요한 머신 상태 변경은 새 rung로 보존 이행한다. | PLAN-07 |
| STATE-10 | narrow / narrow | 낮음 | 날짜-only snooze는 UTC에서도 24시간보다 짧을 수 있음. viewer마다 다른 instant로 읽는다는 주장은 기각. **결정:** 기간 또는 달력 정책과 실제 만료를 설명한다. timestamp 전환은 선택이며 구형 date에서 원래 duration을 복원할 수 있다고 하지 않는다. | PLAN-07 |
| STATE-11 | narrow / narrow | 낮음/선택 | 접수 시점 hash·파일 변화 verifier는 현재 T2/T9를 지킴. 의미 만족·GET 기준 복원을 보장하지는 않음. **결정:** 기계 하한과 내용 검토를 설명한다. 모든 edit body-only gate는 기각하며 origin/snapshot과 future known verifier 정책은 별도 결정한다. | PLAN-01, PLAN-08 |
| STATE-12 | narrow / narrow | 낮음 | 명시 GC 뒤 dismissed connect 재접수는 영구라는 설명과 정책이 충돌함. 자동 판단 무시는 아님. **결정:** 유한 보존 안내 또는 dismissed 보존 정책을 선택한다. tombstone bytes 보존만으로 구버전의 억제 행동까지 보장하지 않는다. | PLAN-07, PLAN-08 |
| STATE-13 | narrow / narrow | 낮음 | 동률 최근 접근 top K가 비결정적임. seed/now가 같은 auto 함수는 재현 가능하며 random 다양성은 의도된 기능. **결정:** instant+slug tie-break와 재현 입력을 명시한다. 기본 random 제거·고정 seed 강제는 기각한다. | PLAN-07, PLAN-10 |
| STATE-14 | accept / narrow | 조건부 중간 | invalid date가 digest 기간 집계에 들어감. 정상 생성 ISO 값과 전체 count/배열 정합성은 별개. **결정:** 파싱 실패를 날짜 집계에서 제외·진단하되 페이지 존재를 지우지 않는다. 미래 날짜와 decisions 기간 정책은 별도 선택한다. | PLAN-07 |
| STATE-15 | narrow / narrow | 낮음/중복 | 같은 JSONL Git 충돌은 수용된 절충. 미해결 marker 발행은 DATA-01과 같은 원인이므로 중복 결함으로 세지 않음. **결정:** 수동 해결과 성공 경계를 설명하고 DATA-01 guard를 둔다. 기기별 파일·자동 의미 병합·CRDT를 필수 해법으로 올리지 않는다. | PLAN-01, PLAN-03 |
| STATE-16 | reject / defer | 선택 제안 | local state는 명시적 path namespace다. alias에서 목록이 다르지만 원래 copy가 삭제된 것은 아님. **결정:** 현재 namespace 안내는 유효하다. canonical/UUID 전환은 clone 격리·옛 hash 발견·중복 처리와 보존 fallback을 정한 뒤 선택한다. | PLAN-06 |
| STATE-17 | defer / narrow | 조건부 보강 | global future version은 이미 차단됨. 높은 DB version의 현재 schema에 정상 쓰기는 실제 미래 schema 손상을 증명하지 않음. **결정:** null/음수 상태 진단을 보강할 수 있다. 개별 DB guard를 택하면 Open 전 쓰기까지 정의하고 best-effort 이벤트 실패가 접수를 막지 않게 한다. | PLAN-07 |
| STATE-18 | accept / accept | 낮음 | 첫 checkout의 허용 agent attention 변경과 R1 status 동일 점검이 충돌함. 페이지 본문은 보존됨. **결정:** 실제 파일 snapshot과 허용 경로를 비교한다. 사전 마킹으로 첫 접근 시험을 피하거나 ?? _meta/ 요약만으로 허용 여부를 판단하지 않는다. | PLAN-02 |

## 검색·근거

[원 리뷰](02-search.md) · [반박](08-red-search.md) · [재반박](11-defense-search.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| SEARCH-01 | accept / narrow | 중간 | 본문이 빈 페이지를 성공 refresh해도 과거 chunks가 남아 recall 근거로 소비됨. **결정:** 빈 chunk 결과에서 이전 벡터/근거를 지운다. title-only 임베딩은 별도 기능 선택이며 기존 근거 유지의 해법이 아니다. | PLAN-09 |
| SEARCH-02 | narrow / narrow | 중간/낮음 | API cold/stale, bridge/reconcile의 cached score와 현재 excerpt 불일치가 확인됨. 일반 full search는 refresh하는 양성 대조가 있음. **결정:** 소비자별 revision/readiness와 부분 실패 정책을 정한다. 모든 소비자를 무조건 매번 전체 재색인하거나 coverage bool 하나로 성공 처리하지 않는다. | PLAN-09 |
| SEARCH-03 | accept / narrow | 조건부 중간 | cache는 text hash만으로 모델/tokenizer/pooling/chunk profile 변경을 감지하지 못함. 품질 하락 자체는 미측정. **결정:** effective profile과 변경 시 재구축을 둔다. 기본 config는 유지하고 미지원 비기본 dimension/chunk_tokens에는 경고와 실제 값·단위를 표시하는 최소안을 권고한다. vector 절단·token/rune 치환은 하지 않는다. custom/legacy 채택을 남기고 대형 파일을 매 질의 hash하지 않는다. | PLAN-09, PLAN-11 |
| SEARCH-04 | narrow / narrow | 검증 보강 | 로컬 3D hidden-state mean pooling은 확인. 공식 적합성·검색 품질 하락을 측정하지 않아 P1 품질 결함은 기각. **결정:** 현재 profile을 기록하고 동일 입력/reference parity와 retrieval 평가를 분리한다. 즉시 CLS/re-export를 의무 수리로 올리지 않는다. | PLAN-10 |
| SEARCH-05 | accept / narrow | 중간 | 일부 whitespace 입력의 chunk text가 raw 원문 연속 범위와 다름. 모든 chunk/페이지가 잘못된 것은 아님. **결정:** 인용 근거는 raw slice로 보존하고 embedding 정규화와 구별한다. 새 offsets API·전면 DB 필드 추가를 최소 해법으로 전제하지 않는다. | PLAN-09 |
| SEARCH-06 | narrow / narrow | 조건부 낮음 | malformed/차원 불일치·NaN 벡터 방어 공백은 synthetic fixture로 확인. 정상 ORT 출력 결함은 아님. **결정:** count/dim/finite 및 허용 오차를 검사한다. 정확 norm 1 강제·keyword까지 차단·매 질의 rebuild는 기각한다. | PLAN-10 |
| SEARCH-07 | accept / narrow | 낮음 | 음수 k와 bridge n의 로컬 호출/CLI panic. HTTP k는 invalid/범위 밖을 default로 clamp하는 양성 대조가 있음. **결정:** 로컬 경계에서 음수·큰 값의 명시적 계약을 검사한다. 0=무제한 여부와 상한은 제품 선택이며 remote panic으로 분류하지 않는다. | PLAN-10 |
| SEARCH-08 | narrow / narrow | 가용성 선택 | 초기 stack 부재는 keyword 강등하지만 실행 중 embedding 실패는 hybrid 오류. F2가 현재 이 경로를 약속한 것은 아님. **결정:** typed embedding failure에 한정한 강등 또는 정확한 오류/문서를 선택한다. SQL/scan/invalid/cancel 오류를 모두 성공 강등으로 감추지 않는다. | PLAN-09 |
| SEARCH-09 | narrow / narrow | 조건부 낮음 | 취소된 대기 요청도 embedding 작업을 할 수 있음. tokenizer는 이미 입력을 truncate하며 native 즉시 중단은 미검증. **결정:** 대기/배치 경계의 취소와 실제 비용을 측정한다. interface 전면 변경·임의 hard budget·native preemption 보장을 필수화하지 않는다. | PLAN-10 |
| SEARCH-10 | narrow / narrow | 평가 보강 | FTS의 혼합어/기호 제한과 평가 부재는 확인. 후보가 truth confidence가 아니라는 점에서 runtime correctness 결함 주장은 기각. **결정:** 작은 재현 corpus로 후보·근거를 평가한다. 자동 abstention·ANN·형태소 분석·최소 score를 측정 전에 의무화하지 않는다. | PLAN-10 |
| SEARCH-11 | accept / narrow | 낮음 | 동점 map 순회가 semantic/RRF top K를 바꿈. bridge는 이미 안정 정렬함. **결정:** exact score에 slug/chunk sequence의 total order를 둔다. SQL 정렬만으로 map 이후 문제를 고쳤다거나 epsilon으로 근접 점수를 뭉개지 않는다. | PLAN-10 |
| SEARCH-12 | narrow / narrow | 조건부 낮음 | dense bridge 후보가 누적 allocation을 크게 만듦. sparse는 작으며 617MiB 수치는 RSS/평균 사용자 outage가 아님. **결정:** 늦은 excerpt 생성과 bounded candidate 저장을 실측한다. heap만으로 P² 계산 제거·페이지별 cap으로 정확한 global top K 유지 주장은 기각한다. | PLAN-10 |
| SEARCH-13 | narrow / narrow | 낮음 | 공개 속도/양자화 유사도 수치의 재현 manifest·retrieval 평가가 부족함. 숫자가 거짓이거나 느리다는 입증은 아님. **결정:** corpus/profile/hardware/toolchain/반복 조건을 공개하거나 수치를 한정한다. 임의 표본 수·평가 framework를 모든 릴리스의 선결 조건으로 강제하지 않는다. | PLAN-10, PLAN-11 |

## 검증·배포·자산

[원 리뷰](06-ops.md) · [반박](08-red-search.md) · [재반박](11-defense-search.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| OPS-01 | narrow / narrow | 낮음 | CI build/test와 실제 F2 CLI·ORT inference·브라우저·배포물 실행의 검증 범위가 다름. 실패한 릴리스를 입증한 것은 아님. **결정:** 작은 lite binary 작업/F2 gate를 우선하고 동일 아티팩트 검증을 연결한다. 모든 native/browser 조합을 모든 PR에 강제하지 않는다. | PLAN-02 |
| OPS-02 | accept / narrow | 중간 | zero-byte/디렉터리 형태의 모델 자산을 Ready로 보고 cached pull이 수리하지 않음. timeout/mixed download 실제 장애는 미검증. **결정:** 저렴한 준비 검사와 명시 repair를 둔다. custom inline ONNX·유효 symlink·quantized bundle까지 기본 6개 sidecar로 일괄 거부하지 않는다. | PLAN-11 |
| OPS-03 | accept / narrow | 중간 | 유효 tilde/4-backtick/blockquote Mermaid code의 wikilink가 전처리에서 HTML로 바뀜. 실제 browser SVG 실패는 미실행. **결정:** 지원 code 영역의 source equality를 검사하고 target span만 바꾼다. Markdown 전체 AST 직렬화·canonical fence만 지원하는 축소는 최소안이 아니다. | PLAN-06, PLAN-14 |
| OPS-04 | narrow / narrow | 낮음 | A3 파싱·tags 키 predicate가 문서 보장과 다름. 전체 lint는 bad-frontmatter를 보고하며 scalar legacy 수용은 별도 정책. **결정:** bad-frontmatter를 건강식에 포함하고 현재 A3의 tags 키 presence 진단을 권고한다. 빈 목록·scalar 관용 읽기는 유지하며 모든 mutation gate로 확대하지 않는다. | PLAN-02, PLAN-06 |
| OPS-05 | accept / accept | 낮음 | 같은 태그 중복이 3/300% 사용률을 만듦. 정상 CLI --tags tool,tool로도 도달함. **결정:** 페이지별 dedup으로 통계를 계산하고 readonly audit를 유지한다. 원문 태그 자동 수정·전면 taxonomy 변환은 필요하지 않다. | PLAN-06 |
| OPS-06 | accept / narrow | 낮음 | 기존 helper가 fake HOME의 canopy attention DB를 남김. SDK telemetry 등 다른 도구 쓰기와 구별해야 함. **결정:** 직접 Go test에도 적용되는 helper HOME/XDG 격리와 canopy namespace 검사를 둔다. 전체 HOME 0·Makefile wrapper만의 보장·t.Parallel 충돌은 피한다. | PLAN-02 |
| OPS-07 | narrow / narrow | 조건부 낮음 | 선언한 MathJax autoload extension의 local URL 404. 실제 typesetting 실패·외부 CDN 요청은 미검증. **결정:** 지원 extension 집합과 route를 맞추거나 범위를 선언한다. 필요한 작은 자산부터 확인하며 전체 bundle·CDN 유출 단정을 의무화하지 않는다. | PLAN-14 |
| OPS-08 | narrow / narrow | 낮음 | Make deps는 host와 무관한 darwin-arm64 native URL을 선택함. Homebrew 4 tuple은 pinned이며 다른 플랫폼 native 실행은 미검증. **결정:** 지원 host/CGO target을 구별해 선택하거나 fail-fast한다. layout 변경은 상태 종류에 맞게 다루고 모든 cross build 지원을 새로 약속하지 않는다. | PLAN-11 |
| OPS-09 | narrow / narrow | 낮음/검증 보강 | 내장 JS 2종의 expected notice/provenance 공백. 실제 릴리스 archive·transitive 법적 준수 전체는 미검증. **결정:** 작은 source/version/hash/license·notice 목록과 배포 고지를 확인한다. mandatory licenses CLI·SBOM·전체 법률 판정은 채택하지 않는다. | PLAN-14 |
| OPS-10 | accept / narrow | 낮음 | quantizer AST 경로는 HOME 고정으로 XDG와 다름. 실제 weight 변환·parity 실패는 실행하지 않음. **결정:** runtime과 같은 명시/XDG 경로를 사용한다. 단일 ONNX rename만으로 external-data bundle의 atomic activation을 보장하지 않는다. | PLAN-11 |
| OPS-11 | accept / narrow | 낮음 | 실제 ORT CLI search -k=-1 panic. 로컬 잘못된 입력이며 전역 startup migration은 RunE 전 실행될 수 있음. **결정:** model/index 작업 전 bounds를 확인한다. 모든 상태 무변경 기준은 초기 bootstrap 또는 허용 migration stamp를 구별해 정의한다. | PLAN-10 |

## 웹

[원 리뷰](03-web.md) · [반박](09-red-web-docs.md) · [재반박](12-defense-web-docs.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| WEB-01 | narrow / narrow | 조건부 중간 | 무인증 loopback이 foreign Host read와 같은 authority Origin POST를 허용함. 실제 브라우저 DNS rebinding·기밀 유출은 미입증. **결정:** listener/port/custom hostname/proxy 정책에 맞는 authority 검증을 설계한다. scheme 비교만의 해법·127.0.0.1 하드코딩·전면 인증 의무는 기각한다. | PLAN-12 |
| WEB-02 | narrow / narrow | 조건부 정책 | 응답에 실행 가능한 raw HTML이 남음. 웹 task 접수만으로 XSS가 생기지는 않으며 공격자 영향 페이지 반영이 전제임. **결정:** HTML 실행 신뢰를 명시하고 비실행 정책을 선택하면 정상 wikilink/수식/표를 보존한다. WithUnsafe 제거만의 해법·원격 이미지 전면 확인은 채택하지 않는다. | PLAN-12 |
| WEB-03 | accept / narrow | 높음 | 서로 다른 두 정상 HTTP 제안을 반복해도 성공 뒤 본문 소실. 페이지 본문 파괴와는 구별함. **결정:** 고유 ID와 exclusive 완전 게시를 함께 검사한다. 권위 JSON 빈 파일 선예약·일반 overwrite rename·웹 mutex의 전역 보증은 기각한다. | PLAN-04 |
| WEB-04 | accept / narrow | 중간 | 성공 feedback 소실과 갱신 중 JSON GET 오류가 재현됨. 한 서버 human read의 직렬화는 보존하는 양성 대조가 있음. **결정:** atomic replacement와 writer RMW 보호를 각각 구현·시험한다. rename-only·Save-only mutex·읽는 요청까지 전역 잠금은 최소 해법이 아니다. | PLAN-04 |
| WEB-05 | accept / accept | 중간 | 로그인 redirect 뒤 최종 HTML 200과 read:false도 shipped JS가 성공처럼 처리함. 실제 브라우저 탭 시퀀스는 미실행. **결정:** API 인증 오류와 read:true payload를 구별한다. 실패 시 컨트롤을 보존하며 page redirect와 저장 성공을 별도로 다룬다. | PLAN-13 |
| WEB-06 | narrow / narrow | 낮음 | 홈 HEAD가 body 0바이트로 daily shown을 소비함. 인간 읽음 기록이나 H5 위반은 아님. **결정:** 홈 HEAD의 영속 노출 기록을 건너뛴다. 모든 HEAD/GET의 모든 analytics 무변경·JS viewport 확인 의무는 별도 결정한다. | PLAN-13 |
| WEB-07 | narrow / narrow | 낮음 | explicit read/feedback은 nonexistent slug를 새로 저장하지만 auto는 404. 옛 기록을 남기는 자체는 정상 보존 정책. **결정:** 새 입력의 존재/stale-tab 정책을 정하고 취소는 허용한다. Load 때 과거 key를 자동 삭제하거나 page UUID를 필수화하지 않는다. | PLAN-13 |
| WEB-08 | narrow / narrow | 조건부 중간 | setup pre-auth file spool과 parse 오류의 잘못된 500은 확인. net/http 기본 제한·종료 cleanup이 있고 실용 DoS/누출은 미입증. **결정:** 작은 route별 body cap·명시 parse·unused multipart 거부·header/idle 예산부터 정한다. 긴 검색 WriteTimeout·세션/로그/전역 backoff 정책은 실측 후 선택한다. | PLAN-12 |
| WEB-09 | narrow / narrow | 중간/낮음 | cold autocomplete는 실제 slug search redirect 뒤에도 []이며 일반 본문 search가 준비함. preview O(N) allocation은 RSS/전체 UI 장애가 아님. **결정:** 소비자 readiness를 먼저 고친다. snapshot/targeted preview는 외부 편집 freshness를 유지해야 하며 임의 1MiB·watcher·pagination을 필수화하지 않는다. | PLAN-09, PLAN-10 |
| WEB-10 | accept / narrow | 낮음 | 미읽음 1·추천 0 상태를 전부 읽음으로 표시함. 추천이 모든 unread를 포함해야 하는 것은 아님. **결정:** count 기반 또는 추천 없음 문구로 정정한다. 최소 score·새 filter·모든 unread 추천은 별도 UX 선택이다. | PLAN-13 |
| WEB-11 | narrow / defer | 선택 제안 | POST 시점 base는 현재 T9를 지키고 stale 제안은 원문을 덮지 않음. GET 기준 비교 자료 부재는 추가 기능 제안. **결정:** viewed provenance를 선택할 때 복원 가능한 근거/retention/구버전 보존을 정한다. hash-only 복원·hidden input의 권위 증거·일괄 409 거부는 기각한다. | PLAN-08 |
| WEB-12 | narrow / narrow | 검증 보강 | 18개 fake DOM 시험은 실제 layout/접근성을 검증하지 않음. 일반적인 모바일·키보드 고장 선언은 미입증. **결정:** 작은 실제 browser smoke와 기능별 확인을 제안한다. graph 전 node DOM화·전체 matrix 의무·computed style 없는 준수 실패 선언은 채택하지 않는다. | PLAN-13, PLAN-14 |
| WEB-13 | accept / accept | 낮음 | auth off에서 옛 cookie가 있는 logout 요청이 panic/EOF. net/http는 회복하여 다음 요청은 성공함. **결정:** nil 모드 세션 drop만 건너뛰고 cookie를 정리한다. 프로세스 crash·auth 우회로 부풀리거나 always authStore를 만들지 않는다. | PLAN-12 |

## 문서·스킬

[원 리뷰](05-docs.md) · [반박](09-red-web-docs.md) · [재반박](12-defense-web-docs.md)

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| DOC-01 | accept / accept | 중간 | 현재 사용자 문서가 웹 제안 접수를 직접 반영/409 충돌처럼 설명함. 실제 원문은 보존되고 task가 접수됨. **결정:** 현재 3쌍의 설명과 영어/hash를 맞춘다. 에이전트 자동 처리 시간이나 모든 제안 반영을 새로 약속하지 않으며 역사 설계 본문은 보존한다. | PLAN-01 |
| DOC-02 | narrow / narrow | 중간 | 공유 후처리와 공통 입력 gate를 혼동한 문서 보장. checkin no-op 예외와 flag/body link 정책도 다름. **결정:** 명령별 gate·no-op·사후 lint를 설명한다. 모든 협약을 쓰기 hard gate로 강화하거나 삭제/복구를 차단하지 않는다. | PLAN-01, PLAN-06 |
| DOC-03 | narrow / narrow | 중간 | 기본 update/mv/archive/rm --json 성공은 빈 stdout. --sync 조합의 결과는 다르므로 항상 빈 출력이라는 일반화는 기각. **결정:** 단일 object 또는 명시 JSONL/예외 계약을 택한다. afterWrite 뒤 emit만 추가해 다중 JSON을 악화시키지 않는다. | PLAN-01, PLAN-08 |
| DOC-04 | narrow / narrow | 중간 | 한국어 bare title 첫 생성은 실패하고 explicit slug는 성공. Git identity 전제와 config 덮기 셸 예제도 설명이 필요함. **결정:** 작은 lite 첫 성공에 slug/wiki를 명시하고 기존 config를 보존한다. full 선택지를 지우거나 Git identity를 임의로 만들지 않는다. | PLAN-01 |
| DOC-05 | accept / accept | 낮음 | arbitrary --dir는 자동 후보로 등록되지 않음. 알려진 후보 경로와 pi 환경 override는 실제 동작함. **결정:** 설치와 activation을 구별하고 갱신 후보/explicit --dir를 설명한다. 새 local registry는 별도 기능이며 필수 수리가 아니다. | PLAN-01 |
| DOC-06 | narrow / accept | 조건부 중간 | wiki 선택 우선순위와 default_wiki 안내가 다름. resolver 자체 버그나 실제 잘못된 위키 변경은 미입증. **결정:** status.root 확인 후 explicit 경로를 유지한다. cwd 편의를 없애거나 명확한 요청마다 대상 확인 질문을 반복하지 않는다. | PLAN-01 |
| DOC-07 | defer / narrow | 지침 보강 | 검색 자료/제안 body의 권한 경계를 보강할 여지. 실제 LLM injection·방어 효과는 시험하지 않음. **결정:** 자료와 위임된 task.request를 구별하는 짧은 지침을 제안한다. 모든 task 무시·원문 scrub·반복 승인·보안 보장 선언은 기각한다. | PLAN-01 |
| DOC-08 | accept / accept | 높음 | 공개 new/show/rm 블록에서 duplicate new 실패 뒤 기존 show 성공·새 임시 본문 삭제·셸 0이 재현됨. **결정:** 쓰기 성공과 intended 내용 확인 뒤만 cleanup한다. set -e 단독·show 존재만·blind retry·사본 항상 보존 약속으로 대체하지 않는다. | PLAN-01, PLAN-05 |
| DOC-09 | narrow / narrow | 중간/낮음 | FAQ legacy tags, mixed audit 누락, bootstrap unused 기준이 운영 taxonomy 안내와 어긋남. legacy 관용 읽기는 정상. **결정:** legacy-only/mixed/bootstrap을 구별하고 사실을 고친다. 자동 제거·모든 warning 0·AllTags 단순 길이 합 검증은 기각한다. | PLAN-01, PLAN-06 |
| DOC-10 | accept / narrow | 중간 | 파일 hash 변화 verifier를 본문/요청 만족 검증처럼 설명함. title/tag-only edit도 합법일 수 있음. **결정:** 기계 하한과 agent의 요청/현재 내용 대조를 설명한다. 모든 edit body-only gate·note 한 줄의 의미 만족 증명은 기각한다. | PLAN-01, PLAN-08 |
| DOC-11 | accept / accept | 중간 | 자동 생성 로그를 재생성 가능한 파생 index/cache처럼 분류한 안내는 부정확함. **결정:** append-only history와 rebuildable cache를 구별한다. 로그에 marker 추가·현재 페이지로 과거 재구성·DB 이전은 필요하지 않다. | PLAN-01 |
| DOC-12 | accept / accept | 중간 | versioning 개요의 _meta ladder 설명이 뒤의 self-version 및 AGENTS 규칙과 충돌함. 실제 migration 실패는 미입증. **결정:** 현재형 개요만 3종 분류와 일치시킨다. 배포된 rung·사용자 state를 문서 오류 때문에 바꾸지 않는다. | PLAN-01 |
| DOC-13 | accept / accept | 중간 | W가 flags/path로 혼용되어 zsh와 Git -C에서 실행 불가. 제품 CLI 오류는 아님. **결정:** quoted path·셸·disposable fixture 전제를 명시한다. 파괴적/ORT/browser 행을 모두 자동 실행 가능한 감사로 과장하지 않는다. | PLAN-02 |
| DOC-14 | narrow / accept | 중간 | A3의 두 count는 bad YAML/tags 부재를 거짓 green으로 판정할 수 있음. 전체 lint는 bad-frontmatter를 보고함. **결정:** 파싱 실패를 predicate에 포함한다. tags는 현 A3의 키 presence 진단을 권고하되 optional 대안의 이견을 남긴다. lint exit 0만으로 건강을 판정하지 않는다. | PLAN-02 |
| DOC-15 | narrow / accept | 중간 | unchecked가 있어도 invalid=0은 참. 재현은 메모리 bundle 주입으로 배포 CLI 환경 failure 자체는 미입증. **결정:** 완전 검증은 invalid/unchecked 모두 0으로 구별하되 의도된 P4 fail-open은 유지한다. syntax와 browser render를 구별한다. | PLAN-02, PLAN-14 |
| DOC-16 | accept / narrow | 낮음 | 첫 checkout의 허용 attention 쓰기와 R1 Git status 동일이 충돌함. 본문 쓰기 버그는 아님. **결정:** page만이 아니라 허용 파일 밖 wiki snapshot도 비교한다. 첫 접근과 당일 no-op을 나누고 기록을 꺼 시험을 통과시키지 않는다. | PLAN-02 |
| DOC-17 | accept / narrow | 중간 | 모든 명령 배너 보장은 show/JSON 등 예외와 다름. mtime touch는 Git dirty 내용 변경이 아님. **결정:** 지원 예외와 explicit status를 안내하고 실제 내용 변경 fixture를 쓴다. 모든 JSON schema에 상태를 강제 추가하지 않는다. | PLAN-01, PLAN-02 |
| DOC-18 | narrow / narrow | 개선 제안 | lite recall fallback와 낡은 로드맵 사실 오류는 확인. 전면 구조 개편/skill 분할의 사용성 효용은 미측정. **결정:** 현재 문서의 작은 진입부·작업별 종료·fallback·원본 보존 예제를 우선한다. 새 canonical handbook·전체 lint/sync 확대는 기각하고 구조 효과는 평가한다. | PLAN-01 |
| DOC-19 | accept / narrow | 낮음 | 정확한 title 자동 이동 설명과 달리 실제 비교는 slug. 일반 검색은 정상이고 keyboard 선택은 별도 흐름. **결정:** 최소안은 정확한 slug로 정정한다. title branch는 중복/문자 비교/예상 밖 navigation 정책을 정한 뒤 선택한다. | PLAN-01, PLAN-13 |
| DOC-20 | narrow / accept | 조건부 중간 | queue 전 sync는 제한된 task 범위와 무관한 dirty도 commit/push함. 전체 wiki 관리 위임에서는 정상 계약. **결정:** 첫 sync의 포함 범위를 설명하고 기존 허가를 존중한다. 반복 승인·다른 작업 자동 되돌림은 기각하며 pull-only CLI는 별도 계약으로 보류한다. | PLAN-01, PLAN-03 |

## 반박에서 추가된 관측

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| RED-B-01 | accept / narrow | 조건부 낮음 | fake curl이 완전한 작은 tar를 출력한 뒤 exit 7이어도 tar/make는 성공함. 실제 native 파일 손상은 입증하지 않음. **결정:** 전송·digest·추출 결과를 분리해 게시 기준을 정하고 이전 정상 자산을 보존한다. portable /bin/sh에 pipefail만 추가하는 안은 채택하지 않는다. | PLAN-11 |
| RED-C-01 | narrow / narrow | 감사 절차 | 원 replay wrapper는 HOME/telemetry를 자체 지정하지 않아 단독 실행 격리를 입증 못함. 실제 사용자 HOME 쓰기는 확인되지 않음. **결정:** 실행 전 환경을 wrapper/기록에 명시하고 격리 보장을 좁힌다. 원 실행의 증거 없이 모든 도구 HOME 무변경·사용자 상태 cleanup을 주장하지 않는다. | PLAN-02 |
| RED-C-02 | narrow / narrow | 낮음 | 빈 본문 제안이 500이고 note 추가 시 body key 부재로 접수됨. 원문은 불변이며 blank 지원 여부는 제품 선택. **결정:** 최소안은 입력 오류 4xx/안내다. blank 지원을 택하면 body 미제공과 empty body를 구별하는 additive 표현을 정한다. | PLAN-08 |
| RED-C-03 | accept / narrow | 중간 | 실제 CLI new --json --sync는 pretty object 두 값이며 단일 JSON도 JSONL도 아니다. update 조합은 sync object 한 값, 기본 update는 빈 stdout이다. **결정:** 관측은 확정하고 단일 outer object 합성을 최소 권고로 선택한다. 정확한 schema·consumer 전환·초기 실패 예외는 구현 전에 명시하고 영향받는 verb/성공/부분 실패/sync 조합을 실제 parser·exit·bytes로 검사한다. | PLAN-01, PLAN-08 |

## 재반박에서 추가된 관측

| ID | 주장 / 처방 | 영향·전제 | 최종 범위와 결정 | 계획 |
|---|---|---|---|---|
| DEF-A-01 | accept / narrow | 시간대 조건부 중간 | 일부 음수 offset의 늦은 현지시각에서 합법 CLI 1일 snooze 성공 직후 후보가 재등장한다. FINAL의 고정 시각·CLI와 조정자의 새 CLI가 재현했고 각각 7일 대조는 억제됐다. **결정:** 새 성공 deadline>now와 접수 직후 제외를 최소 수정 계약으로 채택한다. literal 7일 E4 실패·모든 시간대/기간 실패는 기각하고 duration/calendar·legacy 형식은 별도 선택으로 남긴다. | PLAN-07 |

## 중복과 보류를 읽는 법

DATA-10/STATE-03/WEB-03은 제안 저장 경쟁, DATA-07/STATE-07은 checkout 사본, DATA-11/STATE-08은 agent-only 개명, DOC-13~16/STATE-18/OPS-04는 점검 계약이 겹친다. STATE-15의 충돌 발행은 DATA-01과 묶고, 일반 Git 수동 충돌은 별도 결함으로 세지 않는다.

공식 임베딩 적합성·실제 retrieval 품질, runtime fallback, origin snapshot, 경로 identity, snooze/기각 보존 정책, 실제 브라우저 품질·공격, native 타 플랫폼·배포 archive는 남은 선택 또는 실행 범위를 유지한다. 이 항목을 보류한 것은 검토를 생략했다는 뜻이 아니라, 현재 증거가 허용하는 결론을 제한한 것이다.

DATA-04는 원/RED의 높음·조건부 높음과 방어의 중간 판정이 갈렸다. 조정자는 별도 복구 불능 원문 소실이 입증되지 않은 이 항목을 중간으로 제한하고, DATA-03/05/07의 실증된 소실을 재계수하지 않는다.

태그 키는 B 측의 presence 진단과 C 측의 optional 문구 권고가 갈렸다. 조정자는 현재 A3에 선언된 필수 키 계약을 유지하고 lint의 키 존재 진단을 추가하는 최소안을 권고한다. 빈 목록·기존 scalar 관용 읽기는 유지하며, 이를 모든 mutation의 hard gate로 확장하지 않는다. 다른 계약을 택하려면 의미가 같은 누락/빈 목록을 의도적 선택으로 문서와 점검에 함께 반영해야 한다.

## 종합안 최종 반론의 판정

아래 12개는 종합안의 교정·결정 요청이며 위 95개 제품 추적 항목에 더해 결함 수로 세지 않는다. 항목별 반증·근거·수정은 [최종 응답](14-final-response.md)에 있다.

| ID | 주장 / 처방 | 조정자의 최종 결정 |
|---|---|---|
| FINAL-01 | accept / accept | 1일 즉시 만료를 한정 채택하고 deadline>now·직후 제외를 권고한다. 7일 E4 실패는 기각하며 기간/달력·legacy 선택은 분리한다. |
| FINAL-02 | accept / accept | 게시 전 old hash 보존과 게시 후 적용/현재 hash/단계/repair 일치를 나눈다. DATA-04 중간 판정은 유지하고 정확한 결과 계약을 보존 수리 범위에 둔다. |
| FINAL-03 | narrow / accept | presence 권고는 이미 공개돼 있었으나 비용 설명을 보완한다. lint 키 진단만 강화하고 empty/scalar/null 관용·optional 이견을 유지한다. |
| FINAL-04 | accept / narrow | 단일 outer object를 권고하고 stderr/exit·구조화 실패/초기 빈 stdout 예외·consumer 전환을 명시한다. 6verb 조합 검토는 영향을 받는 변경에 적용하며 매 작은 수리에 60개 새 테스트를 강제하지 않는다. |
| FINAL-05 | accept / narrow | RMW 합집합·동일 base 교체 충돌/직렬 결과·task payload 보존의 oracle을 나눈다. 협력 Canopy/alias/sync 범위만 검사하고 직접 editor·외부 Git/hook 보호는 약속하지 않는다. |
| FINAL-06 | accept / accept | old/current/future의 top/nested unknown 의미·정확 수 보존 또는 해당 상태 사전 거부+old hash를 수락으로 정정한다. 원문 적용 후 실패·healthy 부분 기록·formatting을 별도로 다룬다. |
| FINAL-07 | accept / accept | 누락된 설정 부속 처분을 복원한다. 기본 config는 읽고 미지원 비기본 값에는 경고+effective 값·단위를 권고하며 실제 지원/거부는 별도 선택이다. |
| FINAL-08 | narrow / accept | 화면 후보가 모두 필수였다는 해석은 좁히되 문구의 누수를 수정한다. 기존 경로·접수 base 설명을 최소안으로, 새 filter/origin/화면 기능을 선택안으로 분리한다. |
| FINAL-09 | accept / accept | 소모 사본은 실행 전 별도 보존하도록 문체 예시를 고친다. 실패 후 실제 반영 내용과 nonzero/backup을 구별하고 show 존재만으로 성공을 판단하지 않는다. |
| FINAL-10 | accept / accept | 조정자의 낡은 내부 호출 설명을 실제 CLI timing 증거로 정정한다. 순차 거부 양성 대조와 직접 editor의 틈을 유지하며 심각도·ID 수는 늘리지 않는다. |
| FINAL-11 | accept / narrow | 선택한 평가의 라벨/판정/기준선/사전 합격 규칙·실제 수와 제한을 명시한다. 모든 release의 통계/전체 platform gate는 만들지 않고 실행 종류별 pass/fail/unverified를 분리한다. |
| FINAL-12 | narrow / accept | 전체 14개 직렬 gate라는 실제 정책은 아니었으나 DAG의 오해 여지를 제거한다. 그림을 선택한 한 변경의 검증 흐름으로 바꾸고 우선 표의 병행과 선택 기능 보류를 명시한다. |
