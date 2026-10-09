# 개선 계획 — 적대적 검토 반영안

대상은 `v0.10.0`, `ede67e302a8487aecbb4d59587a8686292018cf6`이다. 이 문서는
현재 제품의 보장이 아니라 **제안**이다. 영역별 반박·재반박과 종합안의
[최종 독립 반박](13-final-challenge.md), [조정자 응답](14-final-response.md)을 반영했다.
구현·릴리스 완료를 뜻하지 않는다. 아래 PLAN ID는 작은 변경 묶음의 식별자이며,
원 리뷰 ID의 주장과 처방 판정은 [판정 원장](decisions.md)에 따로 남긴다.

## 순서와 범위

사용자가 먼저 지적한 스킬 문서의 품질은 첫 변경 묶음으로 다룬다. 문서 정정과
보존 결함의 실패 테스트를 함께 시작하고, 다음 기능 릴리스의 기준에는 데이터 보존과
검색 근거 정합성을 먼저 둔다. 웹은 사용자가 선택한 “정보와 조작을 한눈에 보는
밀도 높은 작업 도구”로 다듬되, 상태의 진실성과 작업 완료 경로를 기준으로 삼는다.

```mermaid
flowchart LR
  Scope["한 변경의 범위와 필수·선택 결정"] --> Contract["불변식과 실행 점검"]
  Contract --> Proof["실패 재현과 정상 대조"]
  Proof --> Change["선택한 최소 구현"]
  Change --> Check["해당 수용 검사와 기본 품질 검사"]
  Check --> Release["검증한 소스와 배포물 연결"]
```

그림의 완료 단위는 **선택한 한 변경**이며 PLAN 14개 전부가 아니다. 각 묶음은
한 번에 전부 구현해야 하는 대형 PR을 뜻하지 않는다. 먼저 선택할 정책을
정하고, `docs/invariants.md`에 **실제로 실행할 점검**을 적고, 실패 사례를 고정한
뒤 필요한 구현만 바꾼다. 기존 테스트·포맷·lite 강등·i18n 규칙도 유지한다.

| 우선 묶음 | 처음 만들 변경 묶음 | 뒤로 미룰 결정 |
|---|---|---|
| 1 | 현재 동작과 다른 스킬·사용자 문장, 실패 시 원본 보존 예제, 실행 가능한 점검식 | 새 handbook, 스킬 전면 분할 |
| 2 | sync 충돌 발행, 단일 파일 중간 실패, archive/목적지 덮기, 제안 소실의 실패 시험과 최소 수리 | 전체 transaction·DB 이전·CRDT |
| 3 | orphan checkout, 상태 오류 표시, 정확한 시간/GC, 현재 검색 근거와 cold cache | 전역 page UUID, 자동 의미 병합 |
| 4 | 웹의 거짓 완료/읽음 성공, 제안과 반영의 구분, 실제 작업 흐름 확인 | 그래프 전면 재구현, 검증 없는 UI 기능 확대 |
| 5 | 모델/배포 자산의 준비·실패 계약, 작은 평가 corpus와 측정 가능한 성능 기록 | ANN·reranker·pooling 즉시 변경 |

문서 정정과 보존 실패 시험은 함께 시작한다. 저렴한 model-ready 검사나 입력 bounds는
다른 보존 수리와 병행할 수 있다. 이 표는 우선순위 제안이며 직렬 일정이나 전부의
release gate가 아니다. 다음 기능
릴리스에서 우선 막을 것은 확인된 원문·제안 소실과 미해결 충돌 발행이다. 더 큰 평가
체계나 모든 브라우저·native 조합을 한 번에 의무화하지 않는다. 조건부 호환성·보안
정책과 사용성 확장은 영향·실행 환경·비용을 정한 뒤 별도로 수락한다. origin 보존,
개별 미래 DB guard, 전면 평가·화면 기능은 선택하지 않은 채 작은 수리를 배포할 수 있다.

## PLAN-01 — 스킬과 현재 사용자 계약을 먼저 바로잡는다

**근거:** DOC-01~12, DOC-17~20, STATE-11/15, DATA-08/15.

- 두 내장 스킬에 짧은 공통 시작·종료 계약을 둔다. `status.root`로 대상 위키를
  확정하고, 변경 명령은 그 경로를 유지한다. 읽기·편집·수집·태스크 처리에는 각각
  필요한 검증과 완료 근거만 붙인다.
- 문장은 행동·대상·완료 조건이 드러나게 쓴다. 추상적인 철학을 작업 단계마다
  되풀이하기보다, 공통 원칙은 한 번 설명하고 각 예제에 필요한 전제와 실패 뒤
  다음 행동을 붙인다. 명령의 실제 출력과 에이전트의 내용 판단을 구별한다.
- 읽기 대체 경로, 원본을 남기는 실패 처리, 부분 반영 뒤 재시도, 전체 변경을 포함하는
  sync, done의 실제 기계 검사 범위를 명시한다. 조회 원문은 권한의 독립 출처가
  아니며, task request는 이미 위임된 위키 작업의 의도로 해석한다.
- README·입문·문제 해결의 현재 웹 제안 큐, 명시적 slug, JSON 응답, 스킬 설치·태그
  안내를 맞춘다. 로그와 캐시, 머신 이행과 위키 self-version을 정확히 구분한다.
  역사 문서는 날짜와 현재 계약 링크로 연결하고 당시 판단을 보존한다.

문체의 기준 예시는 다음과 같다. 이는 완성된 스킬 본문이나 새 코드 보장이 아니라
앞으로 개정할 때 적용할 서술 방식이다.

> 요청에서 대상 위키를 지정했다면 그 경로를 `--wiki`로 넣어 `canopy status --json`의
> `root`를 확인한다. 이후 변경에도 확인한 경로를 유지한다. 쓰기 뒤 회수되는 작업
> 사본은 실행 전에 별도로 보존한다. 명령이 실패하면 보존본을 유지하고, 의도한 내용이
> 현재 페이지에 반영됐는지 반환 오류와 함께 확인한다. 후처리 실패는 페이지가 이미
> 바뀐 상태일 수 있으므로 같은 쓰기를 바로 반복하지 않는다.

**수용:** 두 위키·모델 없는 환경에서 읽기/신규 작성/부분 수정/수집/위임의 예제를
실행하고 대상 밖 위키의 해시도 확인한다. 생성 거부·기존 slug·후처리 실패에서
nonzero, 의도한 본문 반영 여부, 복구용 원본을 각각 확인한다. `show` 성공만으로
내용 만족을 판정하거나 이미 회수된 사본의 존속을 약속하지 않는다.
요청 만족과 자동 검사의 하한을 따로 보고한다. 한국어 변경과 영어
미러·해시를 같은 변경에서 갱신한다. 에이전트 권한 경계의 효용은 정상 태스크를
처리하는 양성 사례와 범위 밖 지시가 섞인 사례로 별도 평가한다.

**범위:** 새 대형 handbook이나 모든 작업에 전체 lint·sync를 요구하는 절차를
추가하지 않는다. 내용 판단을 문자열 검사로 완전히 대체하는 안은 채택하지 않는다.

## PLAN-02 — 불변식 점검을 실제 실행과 실패 판정에 연결한다

**근거:** DOC-13~17, OPS-01/04/06, 기존 테스트의 기준선.

- 감사 변수는 하나의 quoted 경로로 통일한다. Bash/zsh·공백 경로·임시 HOME/XDG를
  명시하고 CLI flags, Git `-C`, 파일 경로를 혼용하지 않는다.
- A3의 파싱 실패·필수 키, P1의 미검증 상태, R1의 허용된 attention 변경, 배너가
  없는 JSON/show의 관측 경로를 정확히 판정한다. 신규 tests의 존재도 확인해서
  존재하지 않는 `-run` 이름이 통과했다고 기록하지 않는다.
- 실제 lite binary의 기본 작업·F2 강등·입력 거부·파일 보존과 핵심 실패 경로를
  CI에 넣는다. 테스트 helper의 상태·캐시는 임시 영역에 두고 검사한 소스/아티팩트를
  publish 단계와 연결한다. ORT compile/load/inference, 실제 브라우저 검증은 서로
  다른 결과로 기록한다.

A3는 기존 다섯 키 계약을 유지하고 **lint의 키 존재 진단만** 보완하는 안을 권고한다.
구조를 일정하게 진단할 수 있다는 이익을 택하되, 외부 Markdown의 경고가 늘 수 있는
비용을 인정한다. `tags: []`와 기존 scalar/null의 관용 읽기는 유지하고 모든 쓰기의 새
hard gate로 확대하지 않는다. optional 대안과 B·C의 이견은 원장에 보존한다.
이 presence 진단에서 null은 키가 있는 빈 집합이며 tags 키 누락과 구별한다.

**수용:** 의도적 실패를 넣었을 때 해당 검사가 빨강이며 publish가 진행되지 않는다.
각 필수 키 누락과 tags 누락/빈 목록/scalar/null, bad YAML을 나눠 최종 predicate를
확인한다. R1은 첫 접근의 전체 위키 snapshot에서 허용한 agent 파일만 달라지고
당일 반복은 no-op인지 확인한다. 모든 명령의 실행 전제·실제 실행 여부·판정 범위를 남긴다. coverage 퍼센트는
합격 기준을 대신하지 않는다. SDK/toolchain 차이와 플랫폼별 미실행도 표시한다.

## PLAN-03 — sync가 해결하지 않은 Git 상태를 발행하지 못하게 한다

**근거:** DATA-01, STATE-15, DOC-20.

commit 전에 unmerged entries와 진행 중 Git 작업을 검사하고 pull/autostash 뒤에도
같은 검사를 한다. 충돌 시 원문·stash·복구 정보를 남기고 commit/push를 멈춘다.
sync가 위키 전체 변경을 포함한다는 계약은 유지한다. metadata의 수동 충돌 해결과
자동 수렴을 구별한다.

**수용:** 로컬 bare remote와 두 clone에서 dirty same-line 충돌을 만들면 sync가
nonzero이며 새 conflict-marker commit이 원격에 생기지 않는다. 양쪽 원문이 복구
가능하고, 정상 nonconflicting sync는 완료된다. sync를 read-only fetch처럼 쓰는
예제가 없어야 한다. 모든 conflict-marker 문자열을 금지하는 검사는 사용하지 않는다.

## PLAN-04 — 승인된 단일 파일 저장과 동시 접수를 보존한다

**근거:** DATA-02/03/05/10, STATE-01/03, WEB-03/04.

같은 디렉토리의 고유 임시 파일에 완전히 기록한 뒤 원자적으로 게시한다. create와
archive는 기존 목적지를 덮어쓰지 않는다. read–modify–write는 cooperating canopy
프로세스 사이에서 같은 위키의 공유 갱신 구간을 보호한다. 고유 task ID와 배타적
완료 게시를 함께 사용하고, nonce만으로 덮어쓰기 방지를 주장하지 않는다.

**수용:** 해당 단일 파일의 **게시 전** I/O 실패에는 이전 완전한 원문·JSON의 해시가
같다. **게시 후** directory sync·정리·후처리 실패에는 새 값의 존속 여부, 현재 해시,
미완료 단계와 repair 경로가 응답과 일치한다. 게시 뒤에도 old hash가 같다고 요구하지
않는다. 협력 writer가 쓰는 동안 독자는 완전한 이전/새 값만 본다.

두 독립 프로세스의 서로 다른 key 갱신은 합집합을 보존한다. 같은 base의 충돌하는
전체 본문 교체는 충돌 반환과 입력 보존, 또는 명시한 직렬 적용 결과로 검사한다.
현재 파일에 두 교체 본문이 동시에 남아야 한다고 요구하지 않는다. 태스크는 동시
N개 접수의 성공 수와 보존된 서로 다른 전문 수가 일치해야 한다.

alias 두 경로가 같은 협력 lock에 참여하는지, 완성 전 reader 노출, 오래된 base로
lock 대기, sync와 writer 겹침, 프로세스 종료와 stale lock 회복을 검사한다. 협력
sync와 겹치는 쓰기는 보호하거나 명시적 busy로 거부하고, 긴 모델·네트워크 작업에
무조건 전체 잠금을 씌우는 해법은 피한다. 기존 archive, file mode, symlink, temp/lock의
Git 포함 여부도 검사한다. 비협력 직접 editor·외부 Git/hook까지 잠금이 보호한다는
보장은 하지 않는다. 로컬 checkout namespace 변경이나 distributed lock은 전제하지
않는다. 전원 장애 durability를 약속할지는 별도로 정하고 검증한다.

## PLAN-05 — 부분 반영과 checkout 복구를 가시적으로 만든다

**근거:** DATA-04/07, STATE-07, DOC-08.

페이지 적용과 후처리 실패를 구별해 적용 여부·실패 단계·재개 경로를 보여준다.
복구가 필요한 원문 적용 단계에는 보존본이나 재적용 근거를 정의한다. 후처리만
실패하고 새 본문이 위키에 남은 경우를 원문 소실로 표시하지 않는다. metadata가
없거나 깨진 checkout은 orphan으로 노출하고 기존 사본을 덮지 않는다. 적용 뒤
회수 실패는 cleanup 미완료로 표시하며 같은 본문을 다시 적용하게 만들지 않는다.

**수용:** new/update/mv/rm/archive/checkin의 대상·백링크·상태·index·로그·DB·cleanup
중 해당 단계마다 오류를 주입해 실제 파일 효과와 반환 상태가 일치하는지 확인한다.
게시 경계는 PLAN-04, 기계 출력은 PLAN-08과 연결한다. 확인할 수 없는 적용 상태는
unknown으로 남기고 “변경 없음”이나 성공으로 숨기지 않는다. 선택한 repair의 반복은
같은 작업·성공 기록을 중복 적용하거나 기존 원문·새 외부 변경을 덮어쓰지 않는다.
corrupt/missing meta에서 사본 해시가 같고 목록에
진단이 보이며, 회수 성공일 때만 사본과 meta가 없다. 완전한 다중 파일 트랜잭션이나
자동 rollback은 이 실패들을 해결하는 유일한 방법으로 전제하지 않는다.

## PLAN-06 — 페이지 식별·링크·frontmatter의 문법을 맞춘다

**근거:** DATA-06/08/09/11/13, STATE-08/16, OPS-03/05, DOC-02/09/14.

- scanner·조회·원장에 관리 영역과 slug 유일성 기준을 맞추고 ambiguity를 드러낸다.
  rename은 실제 링크와 code를 같은 문법으로 구별하고 자기 링크·경로 링크를 다룬다.
  YAML scalar/value 교체는 원문의 의미와 관련 없는 필드·주석을 보존한다.
- agent-only 읽기 이력은 새 slug로 이관한다. task·cooldown 참조는 별도 정책을
  명시해 사용자 제안 본문/과거 판단을 무조건 재작성하지 않는다. 로컬 상태의
  경로별 namespace는 안내하거나 보존 가능한 alias 정책으로 바꾼다.
- 쓰기 gate와 사후 lint 범위를 명시하고, tag 사용률은 페이지별 중복을 제거해 센다.
  초기 taxonomy 추천 목록과 운영 중 수요 기준을 구분한다.

**수용:** duplicate/nested 페이지, 목적지 존재, path/.md/alias/self/code 링크,
역슬래시 제목·block scalar·unknown frontmatter의 round-trip을 검사한다. 동일 agent
기록이 human 유무와 관계없이 이관되며, 목적지에 남은 과거 이력도 보존하거나
명확한 충돌로 남긴다. 중복 태그의 사용률은 100%를 넘지 않는다.
지원하지 않는 입력은 쓰기 전에 명확히 거부한다. 기존 합법 slug나 legacy taxonomy를
즉시 금지하거나 recursive addressing을 무조건 확대하지 않는다.

## PLAN-07 — 재현 불가 상태의 형식·오류·시간·GC를 보호한다

**근거:** STATE-04~06/09/10/12~18, DATA-14, DOC-11/12,
DEF-A-01(최종 한정 채택; FINAL-01).

unknown top/nested 값·정확한 JSON 숫자를 보존하거나 미지원 미래 쓰기를 거부한다.
손상 파일의 unavailable을 건강한 0건과 구분하고 원본을 남긴다. GC는 실제로 선택한
closed 파일만 지우며 미지 상태·pending을 보존한다. event 조회/보존은 실제 instant로
계산하고, snooze 기간·dismissed 판단의 만료 정책·동률 정렬을 명시한다.
새 snooze의 접수 직후 억제는 기간/달력 선택과 별도로 검사한다. DEF-A-01을
통해 확인한 과거 deadline 생성은 수정 대상으로 채택한다. 성공한 새 접수는
`deadline > now`이며 접수 직후 해당 후보를 제외해야 한다. 모든 시간대나 7일 E4가
실패한 것으로 일반화하지 않는다. duration/calendar와 legacy date 해석은 별도 선택이다.

**수용:** null/malformed/future JSON/TOML, 큰 정수, filename/id mismatch, 미지 status,
동률 top K, mixed UTC offset과 cutoff 직전/동일/이후 이벤트를 검사한다. 이후 이벤트와
파싱 불가 원본을 무심코 삭제하지 않는다. old/current/future version의 top/nested
unknown 의미와 정확한 수는 보존하거나 해당 상태 쓰기 전에 거부하고 old hash를
유지한다. future version guard만으로 same-version 확장을 보호했다고 보지 않는다.
TOML 값의 보존과 formatting 보존은 구별한다. 손상 한 파일과 건강한 human/agent
기록을 나눠 unavailable·부분 활용을 표시하며, 빈 새 파일로 재저장하지 않는다.
원문 적용 뒤 상태 실패를 발견하면 PLAN-05의 적용 상태를 함께 보고한다.
새 snooze는 1/7일, 양/음 offset과 날짜 경계 전후를 고정 시각으로 검사한다.
기존 이행 rung는 바꾸지 않는다.
머신-로컬 비파생 형식/키 변경에는 새 rung 또는 보존 fallback, 위키 동반 형식에는
self-version, 캐시에는 재구축을 사용한다. 중앙 downgrade 가드와 개별 event DB
가드의 실제 보장 범위를 먼저 정한다.

## PLAN-08 — 태스크 정체성과 제안의 비교 근거를 정확히 한다

**근거:** STATE-02/03/11/12, WEB-11, DOC-01/10/20, RED-C-02/03.

connect ID가 서로 다른 합법 페어를 같은 요청으로 반환하지 않게 한다. 기존 payload와
기각 판단을 존중한다. 현재 edit base는 접수 시점임을 설명한다. 사용자가 본 origin의
추가 보존은 별도 제품 선택이며, 과거 원문을 비교할 수 있다고 약속할 경우 실제
snapshot/조회 가능한 revision을 남긴다. 클라이언트가 보낸 hash만으로 과거 원문의
출처나 내용을 입증했다고 보지 않는다.
done의 기계 하한과 내용 판단을 UI·스킬에서 정확히 설명한다.
빈 본문 제안의 지원 여부와 입력 오류 응답, `--json --sync`의 출력 단위는 별도
계약으로 결정한다. 이번 권고는 영향받는 mutation의 `--json`을 **단일 outer object**로
합성하는 안이다. stdout은 결과, stderr는 진단, exit는 요청한 단계의 완료/실패를
구별한다. 성공/no-op은 문서화한 의미로 exit 0, 검증·적용·후처리·요청한 sync 실패는
nonzero다. 알려진 실행 단계의 실패도 적용/부분 적용/미확인과 commit/push의 실제
결과를 한 object로 표현하도록 권고한다. 초기 인자 파싱·bootstrap처럼 구조화 전
실패의 빈 stdout 예외는 stderr+nonzero와 함께 명시하며 상태 무변경을 추정하지 않는다.
공개 schema와 기존 sync consumer의 전환은 구현 전에 정한다. 모든 command를 한
schema로 바꾸는 안은 필수가 아니며, JSONL을 택하려면 별도의 호환·framing 계약으로
다시 결정한다. 현재 pretty object 두 개를 JSONL이라고 설명하지 않는다.

**수용:** `(a,b--c)`와 `(a--b,c)`, 역순 재접수, 기존 dismissed·GC, 같은 시각 여러
기기의 접수, stale form, metadata-only 변경, base 없는 구형/미래 known-type
태스크를 검사한다. origin 보강을 선택하면 task의 근거로 실제 과거 본문을 비교할 수
있어야 한다. 접수는 제안이므로 stale form을 직접 저장 충돌처럼 무조건 거부하지 않는다.
형식 추가의 비용과 구버전 저장 안전성은 PLAN-07과 함께 판단한다.

JSON 변경의 검토표에는 `new/update/mv/archive/rm/checkin` × 기본/`--sync` ×
성공/no-op/검증 거부/게시 후 실패/commit 또는 push 실패를 복원한다. 불가능한
조합은 N/A와 이유를 남긴다. 선택한 변경이 영향을 주는 조합을 실제 binary로 실행하고
stdout 전체의 단일 parser, stderr, exit, 페이지·사본·후처리·commit·remote bytes를
함께 확인한다. 이 검토표가 모든 작은 수리에 60개 새 테스트를 강제하는 것은 아니다.

## PLAN-09 — 검색 근거와 캐시의 revision·profile을 일치시킨다

**근거:** SEARCH-01~03/05/06/08, WEB-09, DATA-12.

빈/삭제/변경 페이지의 과거 청크를 현재 근거처럼 소비하지 않게 한다. 페이지 revision,
임베딩 profile, 청크화/정규화 설정을 캐시 유효성에 포함하고 모델 변경 시 파생 벡터를
재구축한다. 생성한 근거 원문과 embedding용 정규화를 구별한다. cold cache·실패·강등
경로는 소비자에게 준비 상태/실제 mode를 정확히 알려준다.

SEARCH-03의 부속 제안인 `embedding.dimension/chunk_tokens`도 별도로 처분한다.
현재 직렬화되는 두 값은 계산에 사용되지 않는다. 최소 권고는 기존 기본값을 계속
읽고 **미지원 비기본 값에는 경고와 실제 effective 설정을 표시**하는 것이다. 실제
지원이나 명시적 거부는 별도 선택이다. vector 절단으로 dimension을 구현하거나
chunk_tokens를 rune 수로 몰래 바꿔 부르지 않는다. 경고는 JSON stdout을 오염시키지 않는다.

**수용:** 빈 본문·삭제·외부 편집·실패한 추론·모델/profile 변경 뒤 recall/API/bridge/
reconcile/추천을 검사한다. 반환 근거는 관측한 원문 revision의 실제 범위이며 stale
청크가 섞이지 않는다. 추론은 긴 SQL transaction 안에 넣지 않고 관측 snapshot이
바뀌었을 때의 재시도/제외 정책을 정한다. sync 뒤 Markdown index도 최종 tree와
일치하거나 명시된 repair 상태다. 캐시 형식 변경은 cache schema/rebuild로 처리한다.
기본/비기본 dimension·chunk_tokens를 각각 실행해 경고·표시 또는 선택한 지원/거부를
검사하고 기존 기본 config의 읽기를 유지한다. profile에는 선언값만이 아니라 실제
엔진·chunker의 값과 단위를 기록한다.

## PLAN-10 — 검색 품질·입력·비용을 측정 가능한 범위로 다룬다

**근거:** SEARCH-04/06~13, OPS-11, WEB-09.

음수/0/큰 k, 차원·NaN·norm, 취소/queue·batch 예산을 경계에서 다룬다. pooling과
reference 적합성 검증을 retrieval 평가와 분리한다. 작은 한국어·영어·혼합·기호·코드·
근거 없는 질의 corpus를 만들어 랭킹·근거 충족을 평가하고, ties는 안정적으로 정렬한다.
실제 규모·쿼리 분포를 정한 후 scan/bridge의 후보 저장·excerpt 비용을 제한한다.

**수용:** 선택한 계약의 잘못된 입력은 model/index 작업 전에 정상 오류이며 panic이
없다. 상태 무변경은 정상 startup migration을 마친 기준선과 비교한다. 사전 취소는
Embed를 시작하지 않고, 대기 중 취소된 요청도 gate를 얻은 뒤 새 Embed를 시작하지
않는다. 대기 즉시 반환·실행 중 native 중단은 별도 계약이다. 공식 비교는 동일
revision/tokenizer/input의 parity로, 제품 평가는 holdout 랭킹과 실제 근거로 확인한다.
성능은 corpus·toolchain·hardware·warm/cold·반복 수와 함께 기록하고 누적 allocation을
RSS로 부르지 않는다. ANN·reranker·새 tokenizer나 임계값은 결과가 필요성을 입증한
뒤 선택한다. 이번 smoke를 품질 합격으로 쓰지 않는다.

평가를 선택한 변경은 query별 기대 근거·관련성 라벨, 판정자/규칙, baseline 비교와
**변경 전에 정한 합격 규칙**, 실제 결과 수를 공개한다. 작은 corpus의 결과는 그
집합에 한정하고 일반 품질·오류율·속도 우위로 통계적 일반화를 하지 않는다.
reference parity에는 revision/tokenizer/input, output 선택·normalization·허용 오차를
남긴다. 성능 기록에는 corpus 크기·pair 밀도·query 분포와 측정 종류도 구별한다.

## PLAN-11 — 모델과 native 자산의 준비·복구 계약을 고친다

**근거:** OPS-02/08/10, SEARCH-03/13, RED-B-01.

모델 종류별 required 묶음의 nonempty/JSON 검사와 revision/digest manifest, 명시적
repair 경로를 둔다. download의 deadline·취소·고유 임시 경로를 정한다. source build는
지원하는 host tuple의 pinned native URL/digest를 선택하거나 범위를 좁혀 fail-fast한다.
선택적 quantizer는 runtime과 같은 XDG/명시 경로를 사용한다.
다운로드·해시 검사·추출의 실패를 각각 관측하고, 어떤 조건을 만족해야 아티팩트를
완성본으로 게시할지 정한다. 파이프라인의 마지막 명령 성공만으로 다운로드 성공을
추정하지 않는다.

**수용:** fake transport와 작은 bundle/tar/converter로 zero-byte·missing sidecar·bad
hash·중단·재시도·custom XDG·unsupported tuple을 검사한다. 이전 정상 자산이 남고
incomplete가 ready로 표시되지 않는다. full checksum은 최초 준비/명시 검사에 두고
매 검색에서 대형 파일을 다시 hash하지 않는다. 실제 ORT/platform 실행은 별도 gate다.
유효 파일을 가리키는 symlink, self-contained ONNX, custom/int8 묶음과 manifest 없는
기존 정상 설치는 각각 검증·adoption 경로를 둔다. 기본 download의 파일 수를 모든
모델에 강제하거나 repair 전에 정상 묶음을 지우지 않는다.
단일 파일과 여러 파일 bundle의 게시 경계를 나누고, 중간 실패·두 installer 겹침의
지원 정책을 검사한다. compile/load/infer는 host tuple별 pass/fail/unverified로 따로
기록하며 다른 tuple의 실행 합격으로 옮기지 않는다.

## PLAN-12 — HTTP의 실제 신뢰 경계와 자원 한계를 명시한다

**근거:** WEB-01/02/08/13.

listener와 허용 Host를 연결하고 합법 localhost/IPv6/사용자 지정 도메인·proxy 정책을
정한다. 위키 HTML/위험 URL/외부 자원의 신뢰 선택을 명시하고, 실행 가능한 원문에
대한 기본 정책을 결정한다. route별 body/parse/time/concurrency 예산과 API의 오류
계약을 정리하며 무인증 logout의 nil 경로를 처리한다.

**수용:** hostile Host는 내용/변경을 얻지 못하고 허용된 주소는 동작한다. HTML 정책은
wikilink·Mermaid·수식·필요한 합법 HTML을 보존하는 양성 fixture와 함께 검증한다.
과한/깨진 요청은 의미 있는 4xx이며 임시 spool은 요청 종료 뒤 정리된다. 선택한
header/body/idle/queue 경계의 예산과 취소를 검사하고, 실행 중 native 추론의 즉시
중단은 확인하기 전 약속하지 않는다. 실제 브라우저
공격의 성립은 별도 증거가 필요하다. CSP나 raw HTML 전체 제거를 단독 해법으로
전제하지 않고, LAN·loopback·인증 여부에 따른 조건을 유지한다.

## PLAN-13 — 웹을 상태가 정확하고 조작이 가까운 작업 도구로 다듬는다

**근거:** WEB-05~07/09~12, DOC-01/19, U 섹션의 현재 검증 한계.

- 미읽음 수, 추천 후보, 대기 작업을 독립 상태로 표시한다. 후보가 없다고 “전부
  읽음”을 표시하지 않고 기존 페이지·태스크 경로를 연결한다. 새 unread filter나
  전용 목록은 선택 기능이다. API는 세션 만료를
  로그인 HTML의 200 대신 명확한 실패로 알리고 클라이언트는 실제 payload의 성공을
  확인한 뒤 표시를 바꾼다.
- 페이지에서 읽음·인용·제안·큐 확인으로 이어지는 작업 경로를 좁힌다. 태스크에는
  현재 상태·대상·요청·시간과 **접수 시점 base의 의미**를 보여주고 제안 접수와 페이지
  반영을 구별한다. 과거 본문의 복원 가능한 origin/이력은 PLAN-08을 선택했을 때만
  약속하며 기본 화면 수정의 선결 조건으로 삼지 않는다.
  exact title/slug 이동 정책도 현재 동작 또는 선택한 개선과 맞춘다.
- graph의 키보드 목록/이동, 작은 화면 TOC, theme 변경, code 가독성, Mermaid/MathJax
  실패 표시를 실제 브라우저로 확인한 뒤 필요한 부분을 바꾼다. HEAD 요청은 노출
  기억을 소비하지 않게 하고 존재하지 않는 입력에는 명확한 오류를 준다.

화면 후보는 정보의 역할을 다음처럼 배치한다. 이는 사용자가 선택한 도구 방향의
구체안이며 기존 화면보다 우수하다는 검증 결과는 아니다.

| 화면 | 정보와 조작의 배치 후보 | 실제 과제로 확인할 점 |
|---|---|---|
| 넓은 화면 | 탐색·검색, 본문·인접한 읽음/인용/제안 조작, 목차·관련 문서·해당 페이지 태스크를 함께 배치 | 본문 폭을 유지하면서 출처와 대기 상태를 찾고 조작할 수 있는가 |
| 좁은 화면 | 본문을 중심으로 검색·핵심 조작을 유지하고 목차·관련 태스크를 펼쳐 접근 | 긴 문서 이동, focus·뒤로가기, 제안 후 상태 확인이 끊기지 않는가 |

검색→근거 확인, 페이지→제안→큐 확인, 미읽음→열람→읽음 기록의 세 과제로 비교한다.
열 수나 새 필터를 필수 기능으로 고정하기 전에 기존 탐색 경로와의 차이를 확인한다.
목차·graph·theme를 포함한 화면 후보의 우수성도 이 비교 전에는 확정하지 않는다.

**수용:** 선택한 핵심 경로를 실제 브라우저에서 먼저 실행하고, 현재 U5의
360px/1280px 점검과 연결한다. 한국어 IME·키보드·밝음/어두움·세션 실패·긴 제목과
표/코드는 변경한 경계에 맞춰 확인한다. JS off는 지원하는 기본 열람/폼과 대체 표시의
범위를 먼저 정한다. 전체 조합을 새 릴리스 필수 gate로 추가하려면 실행 환경과 비용을
별도로 수락한다. overflow·focus·접근 가능한 이름·상태 메시지를 확인하고 fake DOM
통과와 구별한다. 아직 실행하지 않은 화면의 품질 합격을 선언하지 않는다.
실제 browser/U5의 pass/fail/unverified는 native smoke나 JS 논리 시험과 별도로 기록한다.

## PLAN-14 — 내장 렌더링의 완결성과 배포 고지를 확인한다

**근거:** OPS-03/07/09, DOC-15, WEB-12.

모든 지원 code fence에서 검사 원문과 Mermaid element text를 같게 전달한다.
MathJax의 지원 extension 집합과 자산 route를 맞추거나 지원 범위를 명시적으로 좁힌다.
고지 경로가 확인되지 않은 force-graph/MathJax 두 자산부터 해당 version의 license와
필요 notice를 source와 배포물에 동봉한다. upstream/version/hash/가공 방법의 작은
목록은 업데이트 추적용이며 license 사본을 대신하지 않는다.

**수용:** 3/4-backtick·tilde·nested/code span에서 실제 source equality를 검사한다.
선언한 수식 corpus는 offline browser에서 필요한 component가 모두 내장되어 있고
404·외부 요청·error node가 없다. 선택한 자산 manifest의 불일치와 해당 두 자산의
archive 고지 누락을 검출한다. 실제 release archive/대상 플랫폼을 실행하지 않은 검사는 그 범위를
명시한다. 전체 dependency 법적 판정·SBOM 체계 도입·무차별 버전 업그레이드는
이 작은 보완의 전제로 삼지 않는다.
archive/고지, asset route, offline browser의 결과도 각각 pass/fail/unverified로 남긴다.

## 완료의 의미

리뷰 완료는 발견과 제안의 반박·재반박·최종 판정이 모두 남은 상태다. 위 계획의
구현 완료는 각 선택한 정책·불변식·실패 재현·실제 수용 검사까지 통과한 상태다.
두 완료를 혼동하지 않는다. 미래 구현에서 새로 드러난 문제는 새로운 ID와 근거를
추가해 다시 검토한다.
