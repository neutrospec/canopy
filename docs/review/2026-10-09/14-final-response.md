# 최종 반론에 대한 조정자 응답

대상은 `v0.10.0`, `ede67e302a8487aecbb4d59587a8686292018cf6`이다.
[최종 검토자](13-final-challenge.md)의 12개 요청을 코드·원자료·기존 토론과 대조했다.
아래는 조정자의 판단이며 다수결이나 최종 검토자의 자동 승인이 아니다. 수정한 것은
감사의 종합 문서와 원장이다. 제품 구현·사용자 위키·릴리스를 고친 것은 아니다.

`accept/narrow/reject/defer`는 [판정 원장](decisions.md)의 뜻을 따른다. 각 항목의
주장과 처방을 따로 판정한다. 원 90개+RED 4개+DEF 1개의 추적은 유지하며, 아래
12개는 종합안의 교정·결정 요청으로 별도 보존한다. 둘을 결함 수로 합산하지 않는다.
최종 검토자가 읽은 문서는 [고정 manifest](evidence/root-pre-final-manifest.json)에,
수정 이유는 이 응답에 남긴다. 원/RED/DEF/최종 반박 보고서는 그대로 보존했다.

## FINAL-01 — 1일 snooze의 즉시 만료를 한정 채택

**주장 accept / 처방 accept.** 7일 E4 점검이 통과한다는 반론은 옳다. 이것으로
합법적인 1일 옵션이 접수 직후 효력을 잃는 현상을 정상으로 볼 수는 없다.
최종 검토자의 고정 시각·실제 CLI 결과에 더해 조정자도 새 fixture에서 확인했다.
`2026-10-09T06:20:35Z`, `TZ=America/Los_Angeles`에서 1일 접수는 `recorded:true`,
저장 날짜는 `2026-10-09`, 직후 alpha가 다시 나왔다. 같은 페이지의 7일 접수는
`2026-10-15`로 저장되고 즉시 제외됐다.

[소스](../../../internal/resurface/state.go#L143)는 현지 날짜를 저장하고 조회는 UTC
자정으로 해석한다. [조정자 재현 소스](evidence/root-snooze.py)와
[원자료](evidence/root-snooze-result.json), [최종 독립 결과](evidence/final-results.json)를
대조했다. 조정자 helper의 초기 두 실패는 wiki/concepts 디렉터리 준비 누락이었다.
각 기록도 보존했고, 조건을 갖춘 **다른 새 fixture**에서 CLI 성공과 7일 대조를 확인했다.
각 fixture는 사용 뒤 제거했다. CLI 재현은 wall-clock/TZ 조건부이며 항상 실패하는
회귀 테스트로 쓰지 않는다. 미래 회귀 검사는 고정 시각·양/음 offset·날짜 경계를 쓴다.

**수정:** DEF-A-01은 `accept/narrow`, 시간대 조건부 중간으로 종결했다. PLAN-07의
최소 수용은 새 성공 접수의 `deadline > now`와 직후 후보 제외다. 문자 그대로의
7일 E4 실패·모든 시간대/기간 실패·빈도 추정은 기각한다. duration/calendar와 legacy
날짜의 의미, 새 형식 선택은 별도 결정이다. 형식을 바꾸면 위키 self-version·옛 writer
보호를 검토하며 머신-로컬 사다리로 대신하지 않는다.

## FINAL-02 — 저장 실패를 게시 경계로 나눔

**주장 accept / 처방 accept.** DATA-04를 높음으로 되돌릴 추가 근거는 없었다.
명시적으로 요청한 삭제/이동이나 새 본문이 남은 후처리 오류를 별도 복구 불능 소실로
세지 않는다. 반면 일반 오류가 재시도를 유도하는 문제와 정확한 적용 상태는 남는다.
최종 검토자의 update/checkin fault에서도 exit 1, 빈 stdout, 새 본문 존속이 함께
관측됐다. checkin 사본은 회수됐고 별도로 준비한 backup은 남았다.

**수정:** PLAN-04는 해당 파일의 게시 전 실패에 old hash 보존을, 게시 후 실패에
적용 결과·현재 hash·미완료 단계·repair 일치를 요구한다. directory sync 실패 뒤
old hash가 같아야 한다는 잘못된 기준을 제거했다. PLAN-05는 new/update까지 포함해
대상·백링크·상태·index·로그·DB·cleanup 중 해당 단계의 실제 효과를 검사한다.
확인 불가 상태는 unknown으로 남기며 변경 없음으로 표시하지 않는다. DATA-04는
중간을 유지하되 이 결과 계약을 다음 보존 수리 범위에서 무기한 미루지 않는다.
전면 transaction·일괄 rollback·영속 operation framework를 선행 조건으로 삼지 않는다.

## FINAL-03 — A3의 키 진단을 선택하고 이견을 유지

**주장 narrow / 처방 accept.** presence 권고 자체는 종전 원장에 이미 공개돼 있었다.
누락된 것은 선택 이유와 호환 비용의 설명이다. 현재 parser에서 누락과 빈 목록이
같은 tag 집합이라는 사실은 어느 정책이 더 좋은지 증명하지 않는다.

**수정:** PLAN-02/06은 현 A3의 다섯 키 계약을 유지하고 lint 키 존재 진단만
보완하는 안을 권고한다. 구조를 일관되게 진단할 수 있다는 이익을 택하되 외부
Markdown에 경고가 늘 수 있는 비용을 인정한다. `tags: []`, scalar, null은 관용 읽기를
유지한다. 선택한 presence 진단에서 null은 **키가 존재하는 빈 tag 집합**으로 다루며,
tags 누락과 구별한다. 다른 네 필드의 기존 비어 있음 진단과 taxonomy 검사는 유지한다.
전체 write의 새 hard gate로 확대하지 않는다. bad YAML과 각 키 누락/값 대조는 별도
수락이다. B의 presence와 C의 optional 대안이 합의됐다고 쓰지 않는다. optional을
택하는 후속 결정은 문구·predicate·예제를 함께 바꾸어야 한다.

## FINAL-04 — JSON 권고와 실패 조합을 복원

**주장 accept / 처방 narrow.** streaming JSONL도 가능한 설계다. 다만 현재 new의
pretty object 두 값은 JSONL이 아니며 update+sync의 한 값·기본 빈 stdout과도 다르다.
[최종 CLI 원자료](evidence/final-results.json)를 확인했고 RED-C-03의 관측을 `accept`로
정정했다. DOC-03의 모든 조합이 비어 있다는 일반화는 계속 축소한다.

**수정:** PLAN-08의 최소 권고는 영향받는 mutation의 단일 outer object 합성이다.
stdout은 결과, stderr는 진단, exit는 요청 단계의 완료/실패를 구별한다. 알려진 실행
단계의 실패도 실제 적용/부분 적용/unknown과 commit/push 결과를 표현한다.
인자 파싱·bootstrap처럼 구조화 전 실패의 빈 stdout은 명시적 예외로 남기고
stderr+nonzero를 요구한다. 빈 출력만으로 모든 상태가 같다고 판단하지 않는다.
공개 schema·호환 필드·기존 sync consumer의 전환은 구현 전에 정한다. 이는 현재
응답 보장이나 모든 command의 전면 schema 통일 선언이 아니다.

`new/update/mv/archive/rm/checkin` × 기본/`--sync` × 성공/no-op/검증 거부/게시 후
실패/commit 또는 push 실패의 검토표를 복원했다. 불가능한 조합은 N/A와 이유를
남기고, 선택한 변경이 영향을 주는 경우를 actual binary·parser·stderr·exit·페이지/
사본/후처리/commit/remote bytes로 확인한다. 이 검토표를 매 작은 수리에 60개의
새 테스트를 강제하는 gate로 확대하는 해석은 채택하지 않는다. JSONL을 택하려면
별도 framing·호환 계약으로 다시 결정하고 결과 emit만 덧붙이지 않는다.

## FINAL-05 — 동시 쓰기의 서로 다른 수용 기준

**주장 accept / 처방 narrow.** 현재 파일 하나에 서로 충돌하는 두 전체 본문을
동시에 보존하라는 oracle은 성립하지 않는다. 반대로 성공한 독립 제안의 전문은
각각 남아야 한다. 원자적 게시·RMW·ID·기기간 합류는 서로 다른 보장이다.

**수정:** PLAN-04는 서로 다른 key의 RMW 합집합, 같은 base 교체의 충돌+입력 보존
또는 명시한 직렬 결과, task 성공 수와 전문 수 일치를 구별한다. 두 프로세스·alias·
reader·오래된 base 대기·sync 겹침·종료/stale 회복을 수락에 넣었다. 협력 Canopy
sync와 겹치는 쓰기는 보호하거나 busy로 거부한다. 구체적인 lock 프로토콜과 비용은
구현 전에 정하며 모델/네트워크 전체를 무조건 긴 잠금으로 감싸지 않는다.
직접 editor·외부 Git/hook에 대한 CAS, distributed lock, checkout namespace 통합은
요구하지 않는다. 이 범위 밖의 변경까지 보존한다고 약속하는 해석은 기각한다.

## FINAL-06 — 같은 버전의 unknown과 정확한 수 포함

**주장 accept / 처방 accept.** 종전 PLAN-07의 “지원한 미래 필드만”은 원장의
same-version additive 사례보다 약했다. 최종 결과에서 version 1 task의 nested
`9007199254740993`이 `9007199254740992`로 바뀌고 text는 보존된 것을 확인했다.
모든 unknown 필드나 일반 현재 데이터가 사라진다는 주장은 하지 않는다.

**수정:** old/current/future의 top/nested unknown 의미와 정확한 수 보존, 또는
해당 상태 쓰기 전 거부+old hash 보존을 PLAN-07에 요구한다. version guard만으로
current 확장을 보호했다고 보지 않는다. TOML 값과 formatting 보존은 구별하고,
손상 파일과 건강한 human/agent 기록은 unavailable·부분 활용을 따로 표시한다.
빈 새 파일로 재저장하지 않는다. 원문 적용 뒤 상태 실패를 발견한 경우에는
PLAN-05의 적용 상태도 함께 보고한다. 과거 바이너리의 동작을 현재 수정으로
소급 보증하지 않으며, 필요한 경우 별도 traveling 파일·보존 fallback을 선택한다.

## FINAL-07 — 미지원 embedding 설정의 처분 복원

**주장 accept / 처방 accept.** profile identity만 설명하면 공개 설정의 효과를
설명한 것이 아니다. [설정](../../../internal/config/config.go#L106),
[chunker](../../../internal/embed/chunk.go#L16), indexer와 CLI의 사용 위치를 직접
읽었다. dimension/chunk_tokens는 직렬화되지만 계산에는 쓰이지 않는다. 이 부속
제안은 원/RED/DEF에서 논의됐으나 조정자의 합성에서 빠졌다.

**수정:** SEARCH-03과 PLAN-09에 별도 처분·수락을 복원했다. 최소 권고는 기본
config를 계속 읽고 미지원 비기본 값에 경고와 effective 값·단위를 표시하는 것이다.
기존 설정을 갑자기 거부하는 호환 비용을 줄이려는 선택이며, 실제 지원/명시 거부는
후속 대안이다. vector 절단이나 token/rune의 묵시적 치환은 하지 않는다. 경고는 JSON
stdout을 오염시키지 않는다. 기본/비기본을 각각 검사하고 profile은 선언값만이 아니라
실제 엔진·chunker를 식별한다. 이 관측은 새로운 제품 ID나 실물 모델 품질 실패가 아니다.

## FINAL-08 — 기존 상태 수정과 선택 UI 기능 분리

**주장 narrow / 처방 accept.** 종전 표는 이미 넓은/좁은 화면을 후보로 불렀고
우수성은 미검증이라고 적었다. 모든 후보를 필수 기능으로 약속했다는 해석은
채택하지 않는다. 다만 “미읽음 목록으로 바로 이동”과 “비교 근거”가 보류된 기능을
다시 요구하는 듯 읽힐 여지는 있었다.

**수정:** PLAN-13 최소안은 count/문구·실제 read payload·접수/반영 상태·기존
페이지/태스크 경로·접수 시점 base의 의미 표시다. 새 unread filter, 복원 가능한
origin/이력, 새 화면 기능은 선택안이다. origin은 PLAN-08을 택할 때만 복원/retention을
약속한다. 사용자의 밀도 높은 도구 방향과 세 과제 비교는 유지하며 열 수·TOC·graph·
theme가 우수한 해법이라는 판정은 보류한다. WEB-11/DOC-18을 현재 불변식 위반으로
승격하지 않는다. 실제 browser/focus/IME/접근성의 합격도 선언하지 않는다.

## FINAL-09 — 실행 전에 소모 사본을 보존

**주장 accept / 처방 accept.** 문체 샘플은 미래 개정의 예시였지만, 실패 뒤에
이미 회수된 copy를 남기라는 지침으로 옮기면 실행할 수 없다. 별도 backup을 준비한
최종 fixture와 원 DOC-08의 duplicate→show→삭제 사례가 그 차이를 보여준다.

**수정:** PLAN-01 샘플은 요청의 위키를 지정해 root를 확인하고, 쓰기 뒤 회수되는
작업 사본은 **실행 전에 별도로 보존**하도록 바꿨다. 실패 뒤에는 backup을 유지하며
의도한 내용과 현재 페이지·반환 오류를 비교한다. 두 위키·생성 거부·중복·후처리 실패의
수락은 nonzero, intended body, 원본, 비대상 hash를 구별한다. `show` 성공이나
`set -e` 하나로 내용 만족과 모든 실패 처리를 대체하지 않는다. 긴 공통 절차를 각
작업에 복제하지 않으며 이 문장의 LLM 효용·권한 방어 효과는 아직 평가하지 않았다.

## FINAL-10 — 실제 CLI checkin 증거로 정정

**주장 accept / 처방 accept.** 조정자 원장의 “내부 호출” 설명이 최신 DEF 증거를
반영하지 못했다. [보존된 CLI window 출력](evidence/def_data-cli-window-output.txt)은
호출 전 direct 변경의 13.9ms 거부·사본 보존과, 검증 중 변경의 2.309초 성공·sentinel
소실·사본 회수를 구별한다. 해당 probe source의 자식 CLI 호출도 확인했다.

**수정:** DATA-02의 설명을 **timing 조건을 가진 실제 CLI checkin 증거**로 고쳤다.
이 시험을 이번 조정자 단계에서 새로 반복했다고 쓰지 않는다. 원래 정상 두 CLI
경쟁과 순차 거부 대조는 유지하며, 비교/쓰기 틈이 직접 editor에 대한 CAS를
제공하지 않는다는 한계도 유지한다. 심각도를 올리거나 신규 결함으로 재계수하지 않는다.

## FINAL-11 — 평가의 종료와 미검증 구분

**주장 accept / 처방 narrow.** 작은 corpus와 실제 browser라는 말만으로 합격선을
알 수 없다는 지적을 수용한다. 임의 표본 수·전체 플랫폼 행렬을 모든 릴리스에
강제하는 확대는 채택하지 않는다.

**수정:** PLAN-10은 선택한 평가의 query별 기대 근거/관련성 라벨·판정자/규칙·baseline·
변경 전 합격 규칙·실제 결과 수를 요구한다. 작은 집합의 결과를 일반 오류율·속도·
검색 우위로 통계적 일반화하지 않는다. parity에는 revision/tokenizer/input·output·
normalization·오차를, perf에는 크기·밀도·분포·반복·warm/cold·측정 종류를 남긴다.
PLAN-11/13/14는 native compile/load/infer, tuple, browser/U5, archive/고지를 각각
pass/fail/unverified로 기록한다. 기존 3문장 ORT smoke·브라우저 부재·합성 TotalAlloc을
품질·다른 플랫폼 실행·RSS/SLO 합격으로 재사용하지 않는다. 실제 평가와 화면 검사는
미래 선택한 변경의 수용이며 이번 감사가 통과한 새 검사로 쓰지 않는다.

## FINAL-12 — 한 변경의 검증 흐름과 병행 우선순위

**주장 narrow / 처방 accept.** 종전 산문은 이미 일정/대형 PR/전체 matrix가
아니라고 설명했으므로 모든 정책을 실제로 직렬 gate로 채택했다는 해석은 좁힌다.
반면 DAG가 PLAN 묶음 전부의 선행 완료처럼 읽히는 문제는 인정한다.

**수정:** 그림을 **선택한 한 변경**의 범위→불변식/점검→실패/정상 대조→최소 구현→
수용→source/artifact 확인 흐름으로 바꿨다. 다섯 묶음 표는 병행 가능한 우선순위다.
첫 문서 정정과 보존 실패 시험을 함께 시작하고 cheap ready/bounds도 병행할 수 있다.
다음 기능 릴리스의 확인된 원문/제안 소실과 충돌 발행 방지는 유지한다. origin·미래
DB guard·전면 평가·화면 확대는 선택하지 않은 채 작은 수리를 배포할 수 있다.
선택한 기능은 실제 수용 없이 구현 완료라 부를 수 없다.

## 전달과 남은 선택

원 95개와 14개 PLAN의 실질 토론, 위 12개에 대한 응답·문서 정정으로 **이번 범위의
검토와 개선안 작성**을 마친다. 구현 완료나 포괄적 안전 보증은 아니다. 최종 원장의
DEF-A-01은 한정 채택이고, 부속 embedding 설정과 JSON framing도 더 이상 ID 뒤에
묻히지 않는다. 현재 형식의 literal 7일 E4, 정상 read 직렬화, 순차 checkout 거부 등
양성 대조는 삭제하지 않았다.

duration/calendar·옛 date, dismissed 만료, blank 본문 지원, 실제 JSON schema/consumer
전환, origin retention, executable HTML/proxy·예산, 추가 평가·화면 정책은 각 계획의
선택 또는 명시 보류로 남는다. 실제 browser·타 플랫폼 native·실제 archive/Homebrew
설치·실물 quantization·LLM 효과는 이번 검증 결과로 합격 처리하지 않는다.

감사 산출물 검사 명령은 다음과 같다. 원 제품의 회귀 테스트를 대신하지 않는다.

```bash
python3 docs/review/2026-10-09/evidence/check-review.py
python3 docs/review/2026-10-09/evidence/root-verify.py
```

첫 검사는 ID·원장·반론/응답 참조를, 둘째는 파일 링크·fence·저장소 Mermaid parser를
검사한다. 둘 다 실질 토론·화면·내용 품질의 증명이 아니다. 원 source/index·토론
입력 보존과 최종 산출물의 해시·cleanup은 [최종 보존 검사](evidence/root-final-integrity.json)에,
실행 결과는 [문서 QA](evidence/root-report-qa.json)와 [ID 검사](evidence/root-coverage.json)에
기록한다. 기존 Go/race/JS/i18n/lite 결과는 [기준선](00-baseline.md)에서 범위를 확인한다.
