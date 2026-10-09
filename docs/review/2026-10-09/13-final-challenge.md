# 최종 독립 반박 — v0.10.0 종합안

대상은 `ede67e302a8487aecbb4d59587a8686292018cf6`이다. AGENTS.md, 원래 철학과 불변식을 먼저 읽고 기준선, 여섯 원 리뷰, 세 RED, 세 DEF, README·계획·결정표·기계 원장을 모두 대조했다. 이 문서는 제품 수정이나 종합안의 묵시적 승인이 아니다. 조정자 소유 문서는 고정된 상태로 읽었고, 아래 FINAL 항목은 조정자가 별도로 응답해야 한다.

**판정:** Markdown 원본·재구축 캐시·사람/agent 주의 분리·제안 큐를 유지하는 방향과 우선 보존 수리는 지지한다. 95개 추적 항목에 통째로 빠진 원 ID는 없지만, **설정 부속 제안의 최종 처분, JSON 수락 행렬, 저장 실패의 게시 경계, 보류된 UI 기능의 승격**은 보완해야 한다. DEF-A-01은 새 fixture에서 재현됐으며, 이를 7일 E4 실패로 설명하면 안 된다. 모든 계획의 구현이나 브라우저·검색 품질을 합격 처리할 근거는 없다.

## 1. 목록과 토론의 완결성

[직접 추출한 목록](evidence/final-inventory.json)은 원 리뷰 DATA 15, SEARCH 13, WEB 13, STATE 18, DOC 20, OPS 11의 **90개**, RED-B 1·RED-C 3의 **4개**, DEF-A 1의 **1개**를 확인한다. 기계 원장은 중복 ID 없이 95개이며 PLAN은 14개다. 95는 결함 개수가 아니다. 하단 표에는 각 exact ID를 개별 기재했다.

| 토론 대상 | 반박/응답을 읽고 확인한 내용 | 별도 최종 주장/처방 판정 | 남은 예외 |
|---|---|---|---|
| DATA·STATE 33개 | [07](07-red-data.md)의 정상 대조·도달 조건·정책 대안에 [10](10-defense-data.md)이 실제 CLI/내부 호출, 원문 존속, 직접 편집 한계를 구별해 답함 | 33개 모두 두 판정·범위·처방·PLAN 연결이 있음 | DATA-02의 CLI checkin 증거 설명이 원장에 뒤처짐; DATA-04의 P1/P2 이견은 남아 있음 |
| SEARCH·OPS 24개 | [08](08-red-search.md)의 pooling/품질, 합성 벡터, RSS, top K 반례, native/자산 한정에 [11](11-defense-search.md)이 직접 답함 | 24개 모두 두 판정이 있음 | SEARCH-03의 사용하지 않는 설정 부속 제안은 DEF에서 답했지만 최종 계획/원장에서 명시 처분이 사라짐 |
| WEB·DOC 33개 | [09](09-red-web-docs.md)의 브라우저 부재, 의도된 제안, 권한/가독성 비용에 [12](12-defense-web-docs.md)이 양성 대조와 작은 대안을 제시함 | 33개 모두 두 판정이 있음 | A3 presence/optional의 B·C 이견, WEB-11·DOC-18의 선택 효용, 일부 UI 확대는 결정/평가가 남음 |
| RED-B-01 | DEF가 전송 실패와 유효 tar를 구별하고 pipeline/portable shell 반론에 응답 | 최종 accept/narrow 있음 | 실제 native 손상 사건을 입증한 것은 아님 |
| RED-C-01 | DEF가 호출자의 외부 env 가능성을 인정하고 단독 replay 보증만 좁힘 | 최종 narrow/narrow 있음 | 과거 Go 시작 환경은 재구성 불가; 실제 HOME 오염을 확정하거나 청소할 근거 없음 |
| RED-C-02 | DEF가 빈 제안 거부 정책과 잘못된 500을 분리 | 최종 narrow/narrow 있음 | 전체 삭제 제안 지원 여부는 제품 선택 |
| RED-C-03 | DEF가 들여쓴 두 값은 JSONL이 아님을 인정하고 stdout 소유권 대안을 답함 | 최종 narrow/narrow 있음 | 공개 framing/schema와 실패 조합 수락 조건이 미결정 |
| DEF-A-01 | 이 보고서 FINAL-01에서 새 고정 시각·실제 CLI·7일 대조로 반증 시도 | **pre-final은 defer/defer** | 조정자의 최종 응답·두 판정이 아직 없음. `14-final-response.md` 링크는 예정 경로이며 완료 증거가 아님 |

각 그룹의 ID가 표에 등장하는지만 보지 않았다. 원 주장에 대한 가장 강한 반론, 직접 질문의 답, 원문/부작용/대안, 미실행 조건을 읽어 위 결론을 냈다. 원/RED/DEF 사이에서 반박을 모두 받아들인 것은 아니지만, 상대의 핵심 질문을 답 없이 accept로 덮은 원 ID는 찾지 못했다. **부속 제안의 소실과 수락 조건의 빈틈은 별개**이며 아래에서 지적한다.

중복 묶기는 대체로 정당하다. DATA-10·STATE-03·WEB-03은 제안 게시 경쟁, DATA-07·STATE-07은 orphan 사본 보호, DATA-11·STATE-08은 agent 개명 이력, STATE-18·DOC-16은 R1 검사, DOC-03·RED-C-03은 stdout 합성이다. DATA-01·STATE-15도 같은 충돌 발행을 반복 발견했다. 그렇다고 원자적 visibility, RMW 소실, 기기간 ID, 회수 실패까지 한 시험으로 해결됐다고 보아서는 안 된다.

## 2. 독립 증거와 실험 한계

새 소스는 [API probe](evidence/final-probe.go.txt), [HTTP probe](evidence/final-http.go.txt), [CLI runner](evidence/final-runner.py.txt), [Git/HTTP 보강 runner](evidence/final-supplement.py.txt)에 보존했다. [첫 결과](evidence/final-results.json)와 [보강 결과](evidence/final-supplement-results.json)는 argv·exit·stdout·stderr와 실행 환경을 포함한다. 모두 자기 `/private/tmp/canopy-audit-20261009/final` 안에서 실행했다. Go는 임시 GOCACHE, `GOPROXY=off`, `GOTELEMETRY=off`; canopy와 Go 시작 전에 HOME/XDG/TMPDIR를 격리했다. Git identity는 fixture repo에만 설정하고 remote는 자기 로컬 bare repo만 사용했다.

| 독립 검사 | 관측 | 입증하지 않는 것 |
|---|---|---|
| 고정 시각 snooze | `2026-10-09T23:30:00-04:00`, 1일 → 저장 `2026-10-10`, 해석 deadline `00:00Z`, 잔여 **−3.5시간**, 즉시 후보 재등장. 같은 시각 7일 → 140.5시간, 즉시 제외 | 모든 timezone/시간에서 실패, 정확히 7일 E4의 즉시 제외 실패 |
| 실제 CLI snooze | `2026-10-09T06:28:50Z`, `TZ=Etc/GMT+12`: `--snooze 1` exit 0/recorded true 뒤 alpha 재등장. `--snooze 7` 뒤 picks `[]` | 시스템 시계 변경, 브라우저 노출, 일반 사용자 발생 빈도 |
| 실제 CLI JSON+로컬 push | new+sync exit 0, pretty JSON **2값**, 단일 parser 실패, line별 JSONL도 아님. update+sync는 sync object **1값**, 기본 update는 **0값** | 모든 명령의 계약, JSONL이 이미 공개 계약이었다는 주장 |
| 후처리 fault | 생성 index 경로를 디렉터리로 둠: update/checkin exit 1·stdout 빈 값이나 의도한 새 본문은 위키에 남음. checkin 사본 없음, 사전에 만든 별도 backup 있음 | 이 사례의 추가 복구 불능 원문 소실, 모든 I/O 실패가 안전하다는 보장 |
| 실제 HTTP 2개씩 16회 | 서로 다른 제안 32개 모두 303, 보존된 성공 본문 **26개**, transport 오류 0, 페이지 원본 해시 동일 | 소실 확률/사용자 빈도, 두 clone 병합의 실제 소실, 페이지 본문 파괴 |
| 자기 두 clone·bare sync | dirty same-line autostash 충돌 뒤 sync exit 0, committed/pushed true, 원격 marker 본문, status clean. stash와 이전 remote commit에서 양쪽 원문 복구 가능 | Git 양쪽 원문의 복구 불능 소실, literal marker 자체가 항상 충돌이라는 주장 |
| A3 API | missing tags, `tags: []`, scalar tags 모두 허용. bad YAML은 `bad-frontmatter=1`인데 현재 A3의 두-count 식은 참 | lint가 parse 오류를 숨겼다는 주장, tags presence가 의미적으로 더 좋은 제품이라는 증명 |
| 같은 version unknown 숫자 | task dismiss 뒤 nested `9007199254740993` → `9007199254740992`, text는 보존 | 실제 신버전 payload의 보편 손실, 모든 task unknown 필드가 사라짐 |
| fake engine 캐시 | A 한 번 호출 후 B 호출 0; 비기본 dimension/chunk_tokens도 효과 없음. 빈 본문 재색인 뒤 옛 text hit 잔류 | 실물 int8 품질 저하, 공식 pooling 적합성, ORT 추론 결과 |
| HTTP Host | foreign Host preview 200, fixture 본문 포함 | 브라우저 DNS rebinding·스크립트 실행·유출의 성립 |

lite를 새로 빌드했고 위 probe들이 정상 종료했다. 원 기준선의 전체 테스트/race/JS/i18n, 타 플랫폼 실행, ORT, 실제 브라우저, 실제 릴리스 archive는 **이번 최종 단계에서 재실행하지 않았다**. 기존 기록의 결과와 내 실행을 구별한다. 현재 시스템에 브라우저 표면이 없으므로 설치·우회하지 않았다. 합성 probe의 통과나 HTTP 응답을 모바일 품질/공격 성공으로 승격하지 않는다.

재실행은 absent인 자기 `review_probes/final`에 두 `.go.txt`를 `.go`로 복원하고 새 `FINAL_REPLAY_ROOT`를 지정해 runner를 실행한다. `FINAL_SOURCE_REPO`로 source repo를 명시한다. 보강 runner에는 첫 run 경로인 `FINAL_FIRST_RUN`과 새 `FINAL_SUPPLEMENT_ROOT`를 준다. 각 runner는 기존 run 디렉터리를 덮지 않는다. 현재 wall-clock의 UTC hour가 12 이상이면 그 CLI snooze 조건은 재현되지 않을 수 있으며, 고정 시각 결과를 별도로 사용한다. 실제 user wiki/remote나 installed model은 필요 없다.

## 3. FINAL 반론과 필요한 결정

### FINAL-01 — 즉시 만료는 확인됐지만 7일 E4 위반은 아니다

**대상:** DEF-A-01·STATE-10·PLAN-07. 가장 강한 반론은 E4의 문자 그대로인 `--snooze 7` 점검이 통과하고, 달력 기반 snooze가 24시간 배수일 필요도 없다는 것이다. 내 두 대조도 이를 지지한다. 그러나 [Snooze](../../../internal/resurface/state.go#L143)는 현지 날짜로 저장하고 [eligible](../../../internal/resurface/state.go#L87)는 UTC 자정으로 해석한다. 합법적인 1일 접수가 이미 과거인 deadline을 만들고, 별도 shown/읽음 cooldown 없는 실존 페이지를 즉시 반환했다. 이것은 단지 24시간보다 조금 짧은 사례와 구별된다.

**한정 판정:** 새 관측은 accept를 권고한다. 현재 7일 E4의 literal 실패 주장은 reject, 모든 snooze/timezone의 실패나 사용자 빈도 추정도 reject한다. **필요 결정:** 새 성공 접수는 `deadline > now`이고 접수 직후 해당 후보를 제외한다는 최소 조건을 PLAN-07에 명시할 것인지 답하라. duration/calendar와 legacy date 해석은 별도 선택으로 남기고, 경계 직전/직후·양/음 offset·1/7일 양성 대조를 수락한다. 새 format을 택하면 traveling file self-version과 옛 writer 보호를 별도로 다룬다. 이 권고는 조정자의 응답 전까지 최종 원장에서는 미확정이다.

### FINAL-02 — DATA-04 중간 판정은 유지할 수 있지만 게시 후 실패를 수락에서 지워서는 안 된다

**대상:** DATA-04·PLAN-04/05·다음 릴리스 기준. 원문의 가장 강한 반론은 uncommitted rm 뒤 backlink 실패가 돌이킬 수 없는 삭제일 수 있고, 일반 error가 agent의 잘못된 재시도를 유도한다는 것이다. 반대로 명시적으로 요청한 삭제/이동과 새 본문 존속만으로 **추가적인 의도 밖 복구 불능 소실**을 입증하지는 못한다. 내 update/checkin 결과도 새 본문은 남았다. 그러므로 별도 손실을 가정해 높음으로 되돌릴 근거는 없다. **중간/narrow 유지에 동의한다.**

다만 PLAN-04의 “저장 I/O 실패 뒤 이전 해시 동일”은 모든 단계에 적용할 수 없다. 완전한 rename 게시 뒤 directory sync·정리·index/log/DB 실패라면 새 값이 보이고 old hash가 달라질 수 있으며, PLAN-05는 그 상태를 의도적으로 허용한다. **한정 교정:** 게시 전 실패는 old hash 보존, 게시 후 실패는 적용 결과·현재 해시·미완료 단계·복구 경로 일치로 수락 조건을 나누라. 새 전면 transaction은 필요 없다. unknown write 실패를 “변경 없음”으로 잘못 내보내는 경로와 회수된 checkin 사본은 수락 시험에 반드시 남겨야 한다. 심각도 라벨만으로 이 정확한 결과 계약을 후순위 무기한 보류하지 않겠다는 결정을 요청한다.

### FINAL-03 — A3는 presence와 optional 중 하나를 공개적으로 택해야 한다

**대상:** OPS-04·DOC-14·PLAN-02/06. B의 최소 presence 진단은 현재 A3의 다섯 키 문구를 그대로 지킨다는 장점이 있다. C의 optional 대안은 누락과 빈 목록이 현재 parser/읽기/checkin에서 의미상 같고, 외부 Markdown과의 호환 비용이 더 작다는 장점이 있다. 내 probe는 둘의 **현재 동작 동등성**만 확인했고 어느 처방의 효용을 증명하지 않았다.

조정자가 presence를 권고한 것은 보이지 않는 누락이 아니라 명시적 선택이다. 그 선택을 veto하지 않는다. **한정 교정/결정:** `tags: []`와 scalar 관용은 유지하고 **A3 진단만** 강화하는 선택임을 확정하거나, optional 계약으로 문구를 바꾸는 대안을 채택하라. 전자이면 왜 단순 구조 일관성에 호환 비용을 쓰는지 한 문장 남기고, 이를 모든 write의 새 hard gate로 확대하지 말라. 어느 쪽도 bad YAML 음성 대조를 제외할 수 없으며 각 필수 키 누락·tags 누락/[]/scalar/null의 최종 predicate를 별도로 검사해야 한다. 두 기존 방어자의 의견이 합의됐다고 쓰면 안 된다.

### FINAL-04 — JSON framing 선택에 비해 PLAN-08의 수락 행렬이 빠졌다

**대상:** DOC-03·RED-C-03·PLAN-01/05/08. streaming JSONL도 타당한 공개 API일 수 있다는 반론은 인정한다. 그러나 현재 pretty object 두 개는 JSONL이 아니며, new의 sync emit 후 create emit과 update의 sync emit만 존재하는 차이는 [afterWrite/runSync](../../../cmd/canopy/write.go#L137) 및 new/update 분기와 내 실제 stdout에서 확인된다. 출력 관측은 확정 사실이고 최종 schema 선택만 미정이다.

**한정 교정:** DEF의 여섯 verb `new/update/mv/archive/rm/checkin` × 기본/`--sync` × 성공/no-op/검증 거부/게시 후 실패/commit 또는 push 실패를 PLAN-08 수락에 복원하라. 불가능한 조합은 N/A로 이유를 적는다. stdout 전체를 단일 parser로 읽을지, 각 line을 record로 읽을지 먼저 결정하고 stderr와 exit의 역할을 명시한다. 페이지 적용·후처리·commit·push의 성공이 서로 다른 경우 실제 상태와 응답을 대조한다. 단일 outer object는 작은 권고이며, 기존 sync consumer 전환 비용을 숨기거나 모든 command의 전면 schema 통일을 필수화하지 않는다. **빈 stdout의 실패를 의도된 예외로 둘 것인지도 명시**해야 PLAN-05의 기계 판독 상태를 검증할 수 있다.

### FINAL-05 — writer 잠금과 “승인된 갱신 보존”의 대상이 모호하다

**대상:** PLAN-03/04/05/06. per-process mutex로 충분하다는 반론은 HTTP 하나의 human reads에는 맞지만 여러 CLI/serve에는 맞지 않는다. 반대로 wiki 전체를 model inference·network pull 동안 잠그면 짧은 적용 구간이라는 처방과 충돌한다. 고유 temp/rename은 RMW 소실이나 create no-clobber를 해결하지 못하고, 마지막 hash 검사도 직접 편집기와 CAS가 아니다. 이 구별을 종합안이 대체로 잘 유지했다.

남은 빈틈은 “두 독립 프로세스의 승인된 서로 다른 갱신”이다. 서로 다른 task 전문은 모두 남아야 하지만, 한 페이지에 순차로 승인한 두 전체 본문 교체가 현재 파일에 동시에 존재할 수는 없다. **한정 교정:** 다른 key/파일의 RMW는 합집합 보존, 같은 base의 충돌하는 교체는 충돌 반환·입력 보존 또는 명시한 직렬 적용 결과, task 접수는 성공 payload 전부 보존으로 oracle을 나누라. alias 두 경로는 같은 협력 lock에 들어가되 로컬 checkout namespace 변경을 강제하지 않는다. create의 완성 전 reader 노출, 예전 base 준비 후 lock 대기, sync와 writer의 겹침, 프로세스 종료 후 회복을 검사한다. 지원하는 cooperating writer와 외부 git/editor/hook의 한계를 적는 것으로 충분하며 distributed lock·새 DB를 요구하지 않는다.

### FINAL-06 — unknown 보존 조건을 “지원한 미래 필드만”으로 축소하면 같은-version 손실을 통과시킨다

**대상:** DATA-14·STATE-05/06·PLAN-07. version guard와 별도 파일은 작은 현실적 대안이라는 반론을 인정하지만 same-version additive 필드에는 충분하지 않다. `tasks.Close`의 raw map patch도 `map[string]any` 숫자 경로 때문에 내 version 1 nested 정수를 반올림했다. unknown text가 보존된 양성 대조도 있으므로 모든 필드 소실이라고 확대하지 않는다.

**한정 교정:** PLAN-07 수락의 “지원한 미래 필드만 소실되지 않는다”를 **old/current/future version의 top/nested unknown 의미·정확한 수 보존 또는 사전 거부+old hash 동일**로 바꾸라. 거부 시 healthy 값을 0으로 표시하거나 이전 상태를 빈 새 파일로 재저장하지 않는다. TOML 의미 보존과 formatting 보존은 구별한다. corrupt 한 파일 때문에 건강한 human/agent 기록을 모두 버릴지 부분 활용할지도 결과에 드러낸다. 형식 보존 실패를 원문 수정 뒤에 발견하는 경우에는 FINAL-02의 applied 상태로 연결하고 silent success로 숨기지 않는다. 과거 writer의 동작을 현재 수정으로 소급 보증하지 않는다.

### FINAL-07 — 사용하지 않는 embedding 설정의 부속 판정이 최종 합성에서 사라졌다

**대상:** SEARCH-03·PLAN-09/10/11. profile에 effective chunk 설정을 기록하겠다는 문장만으로 `config.Embedding.Dimension/ChunkTokens`의 공개 의미가 정해지지는 않는다. 원문과 RED가 이 부속 제안을 지목했고 DEF 11의 73행은 비기본 값의 경고/거부 또는 실제 지원을 명시했다. [config](../../../internal/config/config.go#L106)는 직렬화하지만 [Split](../../../internal/embed/chunk.go#L16)과 indexer는 값을 소비하지 않는다. 내 probe에서도 비기본 값+engine B의 새 계산이 없었다. 실물 모델의 성능 저하 주장이 아니다.

**한정 교정:** 기존 기본값은 계속 읽고, 비기본 값은 실제 지원/명시 경고/거부 중 무엇인지 독립 disposition과 acceptance를 추가하라. 벡터 절단으로 dimension을 구현하거나 chunk_tokens를 rune 수와 같은 단위로 몰래 치환하지 않는다. 미지원임을 정확히 표시하는 작은 문서/진단도 충분한 선택이다. ID SEARCH-03의 존재 검사만으로 이 부속 제안을 처리 완료했다고 보지 말라.

### FINAL-08 — 보류된 WEB-11/10 기능이 PLAN-13의 필수 화면으로 돌아오지 않게 하라

**대상:** WEB-10/11/12·DOC-18·PLAN-01/08/13. 정보가 많은 작업 도구와 가까운 조작은 제품 방향에 맞지만, 새 필터·목차·origin 저장이 기존 문구 오류를 고치는 유일한 해법은 아니다. 현 POST base는 T9를 지키고 stale 제안도 원문을 덮지 않는다. DOC-18의 fallback/낡은 사실 정정은 유용하나 문서 재구조화의 우수성을 측정하지 않았다.

**한정 교정:** PLAN-13의 “미읽음/대기 목록으로 바로 이동”, 태스크 “비교 근거”를 **기존 경로 연결과 접수 base 표시까지의 최소안**과 **새 unread filter·복원 가능한 origin/이력·새 화면 기능의 선택안**으로 구별하라. 현재 동작 오류는 count/문구·payload·제안/반영 표시로 먼저 수리할 수 있다. origin은 PLAN-08 선택 때만 과거 본문 복원/retention을 약속한다. 넓은/좁은 화면 후보와 세 과제 비교는 유지하되 열 수·TOC·graph DOM·theme 구현을 이미 우수한 해법으로 확정하지 않는다. WEB-11/DOC-18을 현재 불변식 위반으로 승격하는 결정은 거부한다.

### FINAL-09 — 스킬 문체 예시는 현재 checkin의 실패 전 backup 경계를 표현하지 못한다

**대상:** PLAN-01의 66–69행 prose sample·DOC-08·PLAN-05. 예시는 완성 스킬이 아니라 문체 기준이라는 반론은 타당하다. 그래도 “실패하면 원본을 남긴다”를 checkin에도 그대로 옮기면 이미 회수된 copy를 남기라는 실행 불가능한 지침이 된다. 내 fixture는 copy 없이 새 본문만 위키에 남았다. `show` 성공만으로 의도한 본문 반영을 증명할 수 없다는 기존 반론도 유효하다.

**한정 교정:** 짧은 샘플에 요청한 wiki를 지정해 status.root를 확인하는 경로와 **소모되는 작업 사본은 쓰기 전에 보존**한다는 문장을 반영하거나, checkin 예제가 그 책임을 명시하게 하라. 공통 원칙을 긴 절차로 복제할 필요는 없다. 중복 생성·validate 거부·후처리 실패에서 nonzero와 intended body 비교·복구 원본을 각각 확인한다. 이 문체 샘플의 agent 효용은 미평가이며 문장 존재만으로 정상 태스크 처리/권한 방어 성공을 선언하지 않는다.

### FINAL-10 — 원장 증거 설명 하나가 최신 방어와 맞지 않는다

**대상:** DATA-02·decisions·adjudication. 원장/결정표는 “checkin timing 증거는 내부 호출”이라고 하지만 DEF 10의 24·47행은 자식 CLI의 24개 Mermaid 검증 중 150ms direct write, 2.309초 성공 덮기를 보존했다고 명시한다. 이는 새 CLI 무경합 확정 CAS가 아니라 **timing 조건을 가진 실제 CLI 증거**다. 내 이번 audit는 그 timing 시험을 재실행하지 않았고 내 Git/HTTP 반복으로 대체하지 않는다.

**한정 교정:** 원장 설명을 보존된 실제 실행 단계에 맞추거나, 해당 CLI 증거를 채택하지 않는 구체적 이유를 적으라. 두 경우 모두 순차 R4의 정상 거부와 직접 편집의 비교/쓰기 틈은 남긴다. 이 정정은 DATA-02 심각도를 올리는 근거가 아니며 신규 결함으로 세지 않는다. RED-C-03의 출력 관측 accept와 최종 contract narrow의 차이도 같은 방식으로 사실/영향을 구별하면 일관된다.

### FINAL-11 — 작은 corpus와 smoke가 품질/플랫폼 합격으로 변하지 않을 종료 조건이 필요하다

**대상:** PLAN-02/10/11/13/14. corpus·holdout·실제 browser를 계획에 넣은 것은 좋다. 그러나 “작다”, “랭킹과 근거로 확인”만으로 개선 합격선을 정할 수는 없다. 반대로 임의 통계 표본 수/전체 matrix를 모든 release의 gate로 정하면 로컬 단일 바이너리의 작은 수리를 막는다.

**한정 교정:** 평가에서 query별 기대 근거/관련성 라벨·판정자/규칙·baseline 비교·변경 전 합격 규칙·실제 결과 수를 공개하라. 작은 corpus의 결과는 그 집합의 결과로만 진술하고 일반 검색 품질/오류율/속도 우위를 통계적으로 일반화하지 않는다. reference parity에는 실제 같은 revision/tokenizer/input·output 선택·normalization·허용 오차를 기록하고 retrieval 판정과 분리한다. perf에는 크기·밀도·분포·반복·warm/cold·TotalAlloc/RSS의 측정 종류를 적는다. native compile/load/infer, host tuple, 실제 browser/U5, archive/고지는 pass/fail/unverified를 따로 둔다. 이 audit의 브라우저 부재나 3문장 ORT를 해당 gate의 통과로 재사용할 수 없다.

### FINAL-12 — 다섯 단계와 DAG를 전체 정책 확정의 직렬 release gate로 읽지 않게 하라

**대상:** 전체 계획의 15–44행·완료 정의. 문서 우선과 보존/근거 우선은 설득력 있고 “일정 아님/대형 PR 아님”도 이미 명시됐다. 그러나 그림은 PLAN-03~08 전부 → PLAN-09~11 전부 → 화면 → release로 보이며, 별도 선택인 미래 DB guard·origin·평가·모든 bundle 정책까지 끝나야 작은 수리를 낼 수 있다는 해석이 가능하다. cheap model-ready 검사와 입력 bounds는 화면 개선을 기다려야 할 근거가 없다.

**한정 교정:** 필수 최소 수리와 선택 정책/후속 평가를 표시하고, 단계 표는 병행 가능한 우선순위임을 그림에도 반영하거나 화살표의 완료 단위를 명시하라. 최초 문서 묶음과 보존 실패 시험 병행, 다음 기능 릴리스의 확인된 원문/제안 소실·충돌 발행 방지는 유지한다. 선택 기능을 미구현으로 남긴 것은 audit 미완료가 아니지만, 선택했다면 그 수락을 생략하고 구현 완료라 부를 수 없다. 조정자의 응답과 수정된 전달 문서가 없는 상태에서는 최종 리뷰 완료를 선언할 수 없다.

## 4. 모든 PLAN의 하위 제안·수락 조건 판정

`조건부 수용`은 작은 방향을 채택하되 아래 경계를 먼저 결정하라는 뜻이다. 이 표의 모든 구현·새 invariant는 아직 제안이다. 원 코드의 현재 보장으로 소급하지 않는다.

| PLAN | 하위 제안 전체에 대한 판단 | 가장 강한 반론·부작용과 제한 | 수락의 구체적 빈틈/최종 결정 |
|---|---|---|---|
| PLAN-01 | 공통 root/종료, 행동 문장, fallback·원본·partial retry·sync 범위·done 하한, source/task 권한, 현재 안내·설치·taxonomy·history/i18n 모두 **조건부 수용** | 매 read에 전체 lint/sync나 긴 공통 handbook을 붙이면 독자의 질문 해결을 방해. 이미 위임된 task.request를 모두 무권한 자료로 처리하면 정상 처리 손상 | prose sample도 FINAL-09 적용. 두 wiki·모델 없음에서 읽기/생성/수정/수집/위임을 각각 실행하고 비대상 hash·intended body·원본·종료 범위를 확인. 권한 효과/구조 효용은 LLM 과제 평가 전 보류. JSON은 FINAL-04 |
| PLAN-02 | quoted path·shell/격리, A3/P1/R1/배너, 실제 test 존재, lite/F2/실패 CI·artifact 연결, helper 격리 **수용** | `go test -run`의 0 tests·lint exit 0·i18n hash·다른 bytes의 smoke가 거짓 green을 만들 수 있음. t.Setenv와 parallel도 충돌 | FINAL-03/11. 의도적 실패 시 실제 publish 의존 job이 막히는지, 정확 commit와 게시 bytes가 같은지 확인. 첫 R1 전체 snapshot에서 agent 파일만 허용, 반복 no-op 분리. Go 시작 전 env·direct test helper도 검사. 모든 platform/native/browser 의무화는 기각 |
| PLAN-03 | pull 전후/add 전 unmerged·Git 진행 상태 거부, stash/원문/복구 정보 보존, whole-wiki sync 설명 **수용** | marker 문자열 금지는 정상 코드 예제를 막음. Git 원문은 복구 가능하므로 본문 소실로 과장 불가. 검사 직후 외부 Git 경쟁까지 자동 보호하지 않음 | 내 local bare 재현으로 중요도 확인. 정상 nonconflicting/이미 unmerged/진행 작업/dirty autostash를 나눠 nonzero·remote 새 발행 없음·복구 bytes 확인. FINAL-05의 cooperating sync 경계. fetch 전용 새 CLI/자동 metadata merge는 보류 |
| PLAN-04 | 완전 temp 게시·create/archive no-clobber·협력 process RMW·task 고유 ID+배타적 완료 게시 **조건부 수용** | rename-only·Save mutex·nonce-only 모두 충분하지 않음. mode/symlink의 현재 의미를 바꾸거나 temp/lock이 add-A에 들어갈 수 있음. 모든 교체 결과의 동시 존속은 불가능 | FINAL-02/05. pre/post publish 실패·독립 RMW/충돌 교체/task 보존을 별도 oracle로. 실제 두 process·alias·reader·crash/stale 회복 확인. lock 설계 선택은 비용에 맞게, 직접 editor 보호/전원 durability는 미약속 |
| PLAN-05 | applied/stage/repair, 필요한 source 보존, orphan 노출·미덮기, cleanup_pending **수용** | copy만 남겨도 base와 위키가 달라 재checkin이 거부되고, 무조건 rollback은 더 최신 편집을 되돌림. 로그/DB 실패를 원문 손실로 세면 과장 | FINAL-02/04/09. source/backlink/ledger/index/log/DB/cleanup 단계와 실제 bytes·현재 checkout·error 상태 대조; 명시 repair 수렴·중복 기록 없음·외부 최신 편집 보존. mv/rm/archive/checkin 외 new/update의 실제 post-write 실패도 남길 것. 별도 영속 operation framework는 선결 조건 아님 |
| PLAN-06 | 관리 영역/ambiguity·destination, 링크/code/self/path 문법, YAML known scalar/무관 bytes, agent 이관·옛 목적지 이력, task/cooldown/namespace 정책, gate·tag dedup·bootstrap **조건부 수용** | recursive/UUID/전체 YAML marshal은 최소 수리보다 큼. agent destination history를 합산할 의미 근거 없고 task Pages만 바꾸면 old Base/Body 판단 왜곡 | FINAL-03/05. duplicate/nested·same path·case/alias·실제 destination hash, 본문 밖 unknown/comment bytes, code fence/span·anchor·alias·self 대조. merge 못 정하면 destination 충돌 사전 거부가 작은 대안. tags는 페이지별 count·union; legacy 자동 제거/모든 warning 0은 기각 |
| PLAN-07 | unknown/숫자 보존 또는 거부·unavailable·선택 closed GC·instant/expiry/ties·3종 version 경계 **조건부 수용** | future guard만으로 additive current 필드를 못 보호. null/malformed를 0/fresh로 바꾸면 judged state를 지움. 새 UTC만 저장해도 옛 mixed offset은 남음 | FINAL-01/06. cutoff 직전/동일/이후·parse 불가 보존, 실제 열거 파일≠JSON id·unknown status, 같은 instant 동률, current nested·정확 수·no-op/cross-file. snooze와 dismissed 정책은 선택을 명시. DB guard/old-clone·startup race는 미실행을 남김; 기존 rung 수정/캐시를 이행하는 안은 기각 |
| PLAN-08 | connect pair equality·legacy 기각/GC, POST base 설명·선택 origin, done 하한, blank 입력 정책, stdout 단위 **조건부 수용** | ID encoding 변경이 옛 dismiss를 무효화할 수 있음. hash는 원문 snapshot이 아니며 metadata 요청도 합법. 모든 stale form 409는 proposal 제품을 손상 | FINAL-04/08. sorted/reverse collision과 old dismissed, blank/whitespace×note, no-base/future known type·unknown type, metadata-only 요청 대조. 실제 두 clone 합류는 아직 미검증이며 same timestamp fixture만으로 통과 아님. origin 채택 때만 복원/retention+old writer 검증 |
| PLAN-09 | blank/delete/change 근거 제거·revision/profile/정규화 구별·readiness/actual mode·최종 sync index **수용** | 매 소비자 full reindex/장기 SQL transaction은 비용/동시성 악화. FTS 최신이어도 chunks는 stale일 수 있으며 readiness bool 하나는 부분 실패를 못 표현 | FINAL-07. recall/API/bridge/reconcile/suggest 각각 empty/delete/external change/infer failure/profile swap. 원문 revision·raw 범위 확인, race 시 snapshot retry/제외 정책. model 없는 keyword 접근은 유지. 날짜만 바뀐 sync가 쓸데없는 commit을 만들지 않는 양성 대조 필요 |
| PLAN-10 | k/벡터/취소·queue/batch, parity와 retrieval 분리·작은 다국어 corpus·ties·후보/excerpt 비용 측정 **조건부 수용** | 정상 unit float32 exact norm 1 강제는 오탐. HTTP k는 이미 clamp. heap은 P² 계산을 없애지 않고 cap-after-global-K는 결과를 바꿈. 작은 합성 dense TotalAlloc을 RSS/SLO로 쓰면 안 됨 | FINAL-07/11. bounds 전 model/index 불호출+startup baseline, queued cancel 뒤 Embed 0, count/dim/NaN/Inf/zero/error와 keyword recovery, exact tie order, eligibility/cooldown/linked/dismissed를 유지한 top K 대조. 선택 예산의 값은 workload 후 결정. ANN/reranker/형태소/CLS 즉시 강제는 기각 |
| PLAN-11 | required kind별 저렴한 ready·manifest/adoption·repair, deadline/취소/unique temp, host pin/fail-fast·XDG quantizer·transfer/hash/extract 분리 **조건부 수용** | manifest도 검증 없으면 거짓이며 모든 모델에 default sidecar를 요구하면 custom/quantized 설치 손상. unique .part만으로 여러 파일 bundle의 원자 활성화나 두 installer 충돌을 보장 못 함 | 이전 정상 bundle hash와 incomplete 미게시·재시도 검증; fake transport와 converter의 mid-failure를 file/bundle 경계로 나눌 것. 기본값/legacy adoption은 FINAL-07, native/manifest/hash/실제 inference 분리는 FINAL-11. layout를 바꾸지 않는 최소안 가능, installed asset/실물 quantization은 이번 미실행 |
| PLAN-12 | Host/listener/합법 주소·proxy, executable HTML/resource trust 선택, route body/parse/timeout/concurrency/API 오류·logout nil **조건부 수용** | no-Origin 로컬 CLI·IPv6/custom proxy를 깨면 정상 사용 손상. raw HTML 신뢰 자체는 제품 선택. 짧은 WriteTimeout는 큰 wiki/ORT를 끊고 CSP 존재만으로 경계를 해결 못 함 | Host+port 정규화/허용집합·spoofed forwarded header·정상 LAN auth 대조, cap±1/잘못된 encoding/unused multipart→4xx+상태 불변·spool 상한/종료 cleanup. header/idle 최소안과 새 session/login/resource 정책을 구별. auth on/off×cookie nil 경로 확인. 실제 browser exploit은 계속 미검증 |
| PLAN-13 | counts/추천/queue 진실, API payload, 읽음·인용·제안·queue 경로, task 표시·title/slug, 선택 화면/graph/TOC/theme/code/vendor, HEAD/unknown 입력 **조건부 수용** | 추천0은 unread0이 아님. 미읽음 전부 추천·새 filter/column은 필수 아님. hash/base 설명을 복원 기능으로 바꾸면 범위/retention 확대. graph/TOC/theme의 전반적 고장·모바일 열등은 미입증 | FINAL-08/11/12. count·auth HTML/read:false·정상 true·실패 controls 보존, HEAD 첫 요청과 GET 반복, 기존 history undo/unknown 새 입력 분리. 넓은/좁은 후보·세 과제는 비교 제안으로 유지. U5 360/1280 실제 browser와 바꾼 IME/키보드/focus/theme 경계만 수락; 전체 matrix는 별도 비용 결정 |
| PLAN-14 | 지원 code 영역의 전달 source parity, 선언한 math/route 완결, 두 asset version/license/notice·작은 provenance 목록 **수용** | regex만 늘려 container/span까지 같다고 하면 안 됨. indented code와 Mermaid fence의 의미는 다름. 선언 안 된 모든 require를 vendor하는 것은 무한 범위. ledger는 license 사본 대체 불가 | 실제 지원 문법 corpus의 Blocks source·decoded element text equality+일반 code 불치환, rawHTML/alias/newline 양성 대조. declared math offline browser와 asset route, 두 자산의 해당 version notice·실제 archive 누락 검사. FINAL-11의 unverified 분리. 전체 SBOM/법적 준수/버전 upgrade는 선결 조건 아님 |

다섯 단계 표의 개별 판단도 분명히 한다. **1단계**의 사실 정정·실패 보존 예제는 즉시 유용하며 handbook/전면 분할은 보류한다. **2단계**의 충돌 발행·절단·destination/제안 소실은 내 재현/기존 근거와 부합하고 transaction/DB/CRDT는 필요 없다. **3단계**의 orphan·상태·시간/GC·current evidence는 보존 성격이 강하지만 canonical UUID와 자동 합류를 결합하지 않는다. **4단계**의 false success·proposal 구분은 작게 수리할 수 있고 실브라우저 전면 개선을 먼저 요구하지 않는다. **5단계**의 저렴한 ready/배포 실패 검사는 병행 가능하며 retrieval 통계 평가·pooling 변경과 한 묶음의 직렬 blocker로 만들지 않는다. 전체 의존성/우선순위 정정은 FINAL-12다.

## 5. 미해결·조건부 사항과 전달 기준

| 남은 사항 | 현재 허용되는 결론 | 종료에 필요한 것 |
|---|---|---|
| snooze·A3·blank·JSON·dismiss expiry·HTML/proxy·예산 | 관측과 정책 대안은 기록됨; 구현/공개 계약 확정은 아님 | 선택 또는 명시 defer, 영향·호환·검사 명령을 남김 |
| 직접 editor·외부 Git·clone 합류·전원 장애 | 협력 process와 실제 관측 구간의 보증만 주장 | 선택한 지원 범위의 실제 fixture; 선택 밖 한계를 표시 |
| model profile/official pooling/retrieval·성능 | smoke/코드 경로/합성 local 동작 확인 | FINAL-11의 비교 증거 전 품질·SLO·공식 적합성 합격 보류 |
| browser security/mobile/IME/접근성/MathJax 렌더 | HTTP/전달문/JS 논리까지만 확인 | 실제 browser 표면 확보 후 선택 경계 검사; 현재는 unverified |
| native 타 플랫폼·실제 release archive/notice·quantizer | source/fake 선택·실패 정책까지 제안 | 실행한 tuple/bytes/해시·실제 artifact 범위 명시; 미실행 분리 |
| LLM 권한 지침·문서/화면 구조 효용 | 작은 개선 제안으로 유효, 코드 보안/사용성 증명 아님 | 정상 task/범위 밖 지시/세 UI 과제 또는 문서 과제의 실제 결과 |
| 과거 wrapper env·오래된 증거 provenance | 원 canopy 격리와 Go tool 시작 환경의 보장 범위 분리 | 재구성 불가를 유지; 소급 무오염/오염 확정이나 사용자 상태 청소 금지 |

**리뷰 전달의 필수 조건:** 조정자는 FINAL-01, FINAL-02, FINAL-03, FINAL-04, FINAL-05, FINAL-06, FINAL-07, FINAL-08, FINAL-09, FINAL-10, FINAL-11, FINAL-12 각각에 수용/축소/기각/보류와 이유를 공개 응답한다. DEF-A-01은 주장과 처방을 따로 종결하고, 고정 문서 뒤에 이루어진 수정은 보이는 최종 응답 또는 후속 diff로 남긴다. 원 95개 추적을 유지하면서 새 FINAL 반론을 별도로 추가하고, 이 표를 결함 개수로 합산하지 않는다. 명시 보류된 기능은 다음 릴리스의 묵시적 gate로 만들지 않는다.

**미래 구현 완료의 필수 조건:** 선택한 범위의 invariant와 실행 점검을 먼저 문서화하고 실패 fixture→수리→양성/음성 수락으로 확인한다. 프로젝트의 F1/F2/F3 및 해당 i18n 규칙은 유지한다. 확인한 source와 publish artifact를 연결하고, 구현하지 않은 선택 기능과 실행하지 않은 browser/native/평가를 pass로 세지 않는다. 리뷰 완료와 구현/릴리스 완료는 별개다.

## 6. exact ID별 추적·독립 평가 부록

아래 `주장/처방`은 **고정 pre-final 원장의 값**이며 내 수락 시험 통과나 조정자의 FINAL 응답을 뜻하지 않는다. A=07→10, B=08→11, C=09→12의 실질 토론을 확인했다. `RED` 행은 DEF→decisions까지 확인했고, DEF-A-01만 최종 응답 대기다. 각 행의 마지막 열은 단순 ID 존재를 넘어 확인한 한정/반례 또는 남은 이견이다.

| exact ID | 토론 | pre-final 주장 / 처방 | 연결 계획 | 독립 확인·한정·예외 |
|---|---|---|---|---|
| DATA-01 | A | accept / narrow | PLAN-03 | 자기 Git 재현으로 wrong publication 확인; stash/이전 commit 복구 가능, STATE-15와 중복. |
| DATA-02 | A | accept / narrow | PLAN-04 | 두 CLI 정상 경쟁은 유지. 실제 CLI checkin timing을 내부 호출로만 쓴 원장은 FINAL-10; 직접 editor CAS 제외. |
| DATA-03 | A | accept / narrow | PLAN-04 | 조건부 RLIMIT 절단과 실제 disk-full 빈도 구별. 게시 전/후 오류 oracle은 FINAL-02. |
| DATA-04 | A | narrow / narrow | PLAN-05 | 내 post-index 실패도 새 본문 존속; 추가 irrecoverable loss 미확립. 중간 유지·단계 수락은 FINAL-02. |
| DATA-05 | A | accept / accept | PLAN-04 | archive→같은 slug 재생성→archive는 정상 도달; no-clobber 최소안, suffix 정책은 선택. |
| DATA-06 | A | accept / narrow | PLAN-06 | extract/rewrite 문법 차이·코드 변조 유지; target span과 Body 범위, 전체 raw 재직렬화 제외. |
| DATA-07 | A | accept / narrow | PLAN-05 | 정상 meta 보호 양성 대조 유효. corrupt/missing/unreadable+edited copy만 손실 조건; orphan rescue 필요. |
| DATA-08 | A | narrow / narrow | PLAN-01, PLAN-06 | 공통 후처리≠동일 gate. body broken-link lint와 --links gate 구별; blanket hard gate 제외. |
| DATA-09 | A | accept / narrow | PLAN-06 | 역슬래시/block scalar 값 반례와 unknown/body 양성 보존을 함께 유지; 전체 Marshal 제외. |
| DATA-10 | A | accept / narrow | PLAN-04 | proposal·feedback 소실과 human read 직렬화 분리. 내 32→26 제안 재현; FINAL-05의 별도 oracle. |
| DATA-11 | A | narrow / narrow | PLAN-06, PLAN-08 | agent-only caller 누락과 destination 과거 history 덮기 구별. 모든 task/body 자동 이관은 미채택. |
| DATA-12 | A | narrow / narrow | PLAN-09 | 최종 tree와 dashboard stale 유지, 일반 검색 치유 양성 대조로 영구 검색 불능 기각. |
| DATA-13 | A | narrow / narrow | PLAN-04, PLAN-06 | type-only destination 충돌은 보존 문제; flat/nested 영역 선택은 별도. UUID/recursive 확대 불필요. |
| DATA-14 | A | narrow / narrow | PLAN-07 | same-version additive 조건 포함; 미래 version guard만으로 부족. FINAL-06의 exact unknown/거부. |
| DATA-15 | A | reject / narrow | PLAN-01, PLAN-05 | 현재 feed를 rollback 원장으로 해석하는 결함 주장은 기각, 보장 설명과 필요한 복구만 유지. |
| STATE-01 | A | accept / narrow | PLAN-04, PLAN-07 | RMW/visibility와 migration 고정 tmp API race 분리; 실제 CLI startup 성공 대조를 무시하지 않음. |
| STATE-02 | A | accept / narrow | PLAN-08 | sorted 합법 pair collision 유지; 다른 body 손실과 별개, 옛 dismissed payload equality 필요. |
| STATE-03 | A | accept / narrow | PLAN-04, PLAN-08 | DATA-10/WEB-03 중복. 로컬 전문 소실 확인, 기기간 동일 ID≠실제 clone merge 소실 입증. |
| STATE-04 | A | accept / narrow | PLAN-07 | 수작업/malformed id alias가 조건; 임의 외부 삭제 주장 제외. 실제 선택 파일·known closed만 GC. |
| STATE-05 | A | narrow / narrow | PLAN-07 | 같은 version nested 큰 정수 반올림 독립 확인. 일반 현재 데이터 전부 파괴 아님; FINAL-06. |
| STATE-06 | A | accept / narrow | PLAN-07 | null panic/Stat 오류 hang/거짓 0은 조건부. 부분 healthy 기록 활용·unavailable 정책 필요. |
| STATE-07 | A | accept / narrow | PLAN-05 | DATA-07 중복의 orphan과 cleanup 실패는 서로 다른 상태; 내 checkin 회수 관측도 구별. |
| STATE-08 | A | accept / narrow | PLAN-06 | DATA-11 중복. agent-only 이관이 human-read H3 literal 전체 실패를 뜻하지 않음. |
| STATE-09 | A | accept / narrow | PLAN-07 | RFC3339 TEXT 조회/GC는 코드상 instant와 다름. LastAccess 양성 대조·옛 mixed 행 보존 필요. |
| STATE-10 | A | narrow / narrow | PLAN-07 | duration/calendar 선택과 조기 종료 분리. viewer별 instant 차이 기각; DEF-A-01은 별도 좁은 새 반례. |
| STATE-11 | A | narrow / narrow | PLAN-01, PLAN-08 | POST base·full-file verifier는 현 계약 충족. 의미 만족·GET 원문 복원·body-only gate는 별도. |
| STATE-12 | A | narrow / narrow | PLAN-07, PLAN-08 | 명시 GC와 영구 억제 설명의 충돌; tombstone byte 보존으로 old reader 행동까지 보증 못 함. |
| STATE-13 | A | narrow / narrow | PLAN-07, PLAN-10 | recent exact tie 불안정과 고정 seed auto 결정론 구별. random 다양성 제거 제외. |
| STATE-14 | A | accept / narrow | PLAN-07 | invalid date 집계는 조건부; 배열 count 정합·정상 생성 날짜 양성 대조로 범위 한정. |
| STATE-15 | A | narrow / narrow | PLAN-01, PLAN-03 | JSONL 수동 conflict는 수용된 절충, marker 발행만 DATA-01과 합침; CRDT 강제 제외. |
| STATE-16 | A | reject / defer | PLAN-06 | 명시 path namespace에서 다른 목록은 source loss 아님. lock alias와 state identity 이행 구별. |
| STATE-17 | A | defer / narrow | PLAN-07 | global downgrade guard 이미 있음. higher DB version/current schema 쓰기는 실제 미래 손상 증거 아님. |
| STATE-18 | A | accept / accept | PLAN-02 | 첫 checkout agent attention 허용 vs R1 검사 모순. DOC-16과 중복, 전체 manifest allowlist 필요. |
| DEF-A-01 | 최종 응답 대기 | defer / defer | PLAN-07 | 내 고정시각+실제 CLI 1일 즉시 재등장/7일 제외 대조. FINAL-01 답변·최종 두 판정 대기. |
| SEARCH-01 | B | accept / narrow | PLAN-09 | 빈 본문 success refresh 뒤 old text 반환을 fake engine 독립 확인; title-only 기능 강제 제외. |
| SEARCH-02 | B | narrow / narrow | PLAN-09 | cold/stale 소비 경로와 full search refresh 양성 구별. 현재 excerpt+old score 혼합 허용 불가. |
| SEARCH-03 | B | accept / narrow | PLAN-09, PLAN-11 | effective profile 유지; 실물 int8 피해량 미입증. unused 설정 부속 소실은 FINAL-07. |
| SEARCH-04 | B | narrow / narrow | PLAN-10 | local 3D mean 경로와 공식/reference·retrieval 적합성 미검증 분리. 즉시 CLS/re-export 의무 기각. |
| SEARCH-05 | B | accept / narrow | PLAN-09 | 일부 whitespace 청크만 raw 연속 범위 위반. 인용 raw와 embedding normalization 분리. |
| SEARCH-06 | B | narrow / narrow | PLAN-10 | synthetic invalid engine/cache 조건. near-unit float32/정상 malformed blob 오류 대조, keyword 회복 유지. |
| SEARCH-07 | B | accept / narrow | PLAN-10 | 로컬 음수 k/bridge n과 HTTP clamp 양성 구별. 0/상한 정책과 startup mutation 한정 필요. |
| SEARCH-08 | B | narrow / narrow | PLAN-09 | 초기 keyword 강등은 F2 준수. runtime embedding 실패의 fallback/정확 오류는 별도 선택. |
| SEARCH-09 | B | narrow / narrow | PLAN-10 | gate 대기 취소 뒤 Embed 경계만 확인; native 즉시 중단·모든 입력 미절단 주장은 제외. |
| SEARCH-10 | B | narrow / narrow | PLAN-10 | candidate score≠truth confidence. 다국어/기호 한계 유지, 자동 abstention/새 tokenizer 의무 제외. |
| SEARCH-11 | B | accept / narrow | PLAN-10 | map 뒤 exact tie 불안정과 안정 bridge 양성 구별. slug/seq total order, epsilon 묶음 제외. |
| SEARCH-12 | B | narrow / narrow | PLAN-10 | dense 후보 TotalAlloc이지 RSS 아님. sparse 대조, cap-after-K 반례와 P² compute 잔류 유지. |
| SEARCH-13 | B | narrow / narrow | PLAN-10, PLAN-11 | 공개 수치의 manifest/평가 근거 공백≠숫자 거짓. FINAL-11의 bounded 평가/표현. |
| OPS-01 | B | narrow / narrow | PLAN-02 | CLI F2·exact artifact/commit release gate 공백≠실패 release 입증; 모든 native/browser PR gate 제외. |
| OPS-02 | B | accept / narrow | PLAN-11 | 0B/directory ready 반례 유지. custom/self-contained/symlink/int8/legacy adoption의 정상 경로 필요. |
| OPS-03 | B | accept / narrow | PLAN-06, PLAN-14 | tilde/4-tick/container 전처리 source 변조와 실제 SVG 실패 미실행 구별; source grammar 공통화. |
| OPS-04 | B | narrow / narrow | PLAN-02, PLAN-06 | A3 presence 진단 B와 optional C 대안을 FINAL-03에서 공개 유지; scalar/empty 호환은 보존. |
| OPS-05 | B | accept / accept | PLAN-06 | 정상 CLI duplicate tags로 page pct>100 도달. readonly per-page dedup 수용, 자동 retag 제외. |
| OPS-06 | B | accept / narrow | PLAN-02 | fake HOME canopy event DB 쓰기와 SDK/tool telemetry 구별. direct test helper 격리, parallel 주의. |
| OPS-07 | B | narrow / narrow | PLAN-14 | local extension route 404 관측≠browser typeset/CDN 유출. declared corpus 후 실행 전 미검증. |
| OPS-08 | B | narrow / narrow | PLAN-11 | hardcoded darwin-arm64 선택과 formula pinning 구별; host/CGO target fail-fast 최소안. |
| OPS-09 | B | narrow / narrow | PLAN-14 | 두 asset의 bounded notice/provenance 제안. 실제 archive·전체 법적 준수 판정은 미검증. |
| OPS-10 | B | accept / narrow | PLAN-11 | quantizer XDG 불일치 정적 경로 확인, 실물 변환/parity 미실행. external-data bundle 게시 경계 필요. |
| OPS-11 | B | accept / narrow | PLAN-10 | ORT 음수 k local panic 기록 유지, 이번 재실행 아님. bounds 전 model/index 작업·startup baseline 구별. |
| RED-B-01 | RED→DEF→결정 | accept / narrow | PLAN-11 | 유효 toy tar 뒤 curl 7인데 pipeline 0; transport 상태 소실 확인, actual native corruption 기각. |
| WEB-01 | C | narrow / narrow | PLAN-12 | 내 foreign Host HTTP 200 재확인. actual browser rebinding/exfil 미검증, 정상 IPv6/proxy 보존. |
| WEB-02 | C | narrow / narrow | PLAN-12 | unsafe HTML 출력과 attacker-influenced 반영 전제 구별. task 접수만의 자동 XSS/remote image 동일위험 기각. |
| WEB-03 | C | accept / narrow | PLAN-04 | 내 실제 HTTP 32 success/26 preserved, page unchanged. 웹 mutex/nonce만의 전역 보증 제외. |
| WEB-04 | C | accept / narrow | PLAN-04 | feedback RMW 소실·reader 중간 JSON과 human read mutex 양성 구별; visibility/RMW 별도. |
| WEB-05 | C | accept / accept | PLAN-13 | redirect HTML200/read:false 거짓 성공은 JS 논리 관측. actual browser session sequence는 미실행. |
| WEB-06 | C | narrow / narrow | PLAN-13 | HEAD daily shown 소비는 인간 read H5 위반 아님. GET daily no-op 대조와 작은 HEAD 예외. |
| WEB-07 | C | narrow / narrow | PLAN-13 | unknown 신규 입력과 기존 history undo/보존 구별; alias/UUID는 필수 아님. |
| WEB-08 | C | narrow / narrow | PLAN-12 | pre-auth spool/잘못된500은 조건부, 기본 form cap·cleanup 있음. practical DoS/유출·새 session 정책 미입증. |
| WEB-09 | C | narrow / narrow | PLAN-09, PLAN-10 | cold API 오류와 preview O(N) allocation 구별. 실제 slug redirect는 warm 안 함; RSS/p95 주장 제외. |
| WEB-10 | C | accept / narrow | PLAN-13 | unread1/recommendation0의 false all-read만 수리. 모든 unread 추천/새 filter는 선택; FINAL-08. |
| WEB-11 | C | narrow / defer | PLAN-08 | 현재 T9 위반 기각, useful viewed-origin 제안은 보류로 유지. FINAL-08의 scope 누수 금지. |
| WEB-12 | C | narrow / narrow | PLAN-13, PLAN-14 | fake DOM18≠layout/focus/접근성. source CSS 계산≠computed style 실패; 실제 browser 미검증. |
| WEB-13 | C | accept / accept | PLAN-12 | cookie+auth off nil panic/EOF와 server 회복 양성 구별. nil guard/clear만 최소 수리. |
| DOC-01 | C | accept / accept | PLAN-01 | 현재 3쌍의 web proposal 안내 정정, 과거 설계 당시 판단 보존. 처리시간/반영 SLA 추가 제외. |
| DOC-02 | C | narrow / narrow | PLAN-01, PLAN-06 | 같은 pipeline≠같은 input gate, checkin no-op/--links/body 차이 유지; 문구부터 바로잡기. |
| DOC-03 | C | narrow / narrow | PLAN-01, PLAN-08 | 기본 빈 stdout을 sync에도 일반화하지 않음. RED-C-03과 합성 schema 문제 공유, FINAL-04. |
| DOC-04 | C | narrow / narrow | PLAN-01 | 한글 title→명시 slug 양성, empty identity sync 실패 전제 구별. 기존 config 보존·lite starter. |
| DOC-05 | C | accept / accept | PLAN-01 | arbitrary --dir 등록/activation 아님, known candidates/override는 동작. persistent registry 강제 제외. |
| DOC-06 | C | narrow / accept | PLAN-01 | resolver precedence 사실 정정. 실제 오위키 변경 미입증, 명확한 요청/기존 허가에 반복 질문 제외. |
| DOC-07 | C | defer / narrow | PLAN-01 | injection 실제 침해/방어효과 미시험. quoted body와 위임된 task.request를 구별하는 짧은 지침. |
| DOC-08 | C | accept / accept | PLAN-01, PLAN-05 | duplicate new→기존 show→rm의 원본 삭제 반례 유지. 내 partial checkin으로 사전 backup 필요; FINAL-09. |
| DOC-09 | C | narrow / narrow | PLAN-01, PLAN-06 | legacy read 관용 정상, mixed audit/FAQ/bootstrap 기준 정정. union을 단순 길이 합으로 검사 금지. |
| DOC-10 | C | accept / narrow | PLAN-01, PLAN-08 | hash change≠request 만족, metadata-only 합법 요청 유지. note는 증명 아님, universal body gate 제외. |
| DOC-11 | C | accept / accept | PLAN-01 | append history≠rebuildable cache. marker/DB 이전/현재 페이지로 과거 복원 의무 제외. |
| DOC-12 | C | accept / accept | PLAN-01 | overview _meta ladder 오류 정정, 실제 migration 실패는 미입증. 배포 rung 수정 제외. |
| DOC-13 | C | accept / accept | PLAN-02 | flags/path 변수 혼용 bash/zsh/Git 실패 유지. disposable quoted path 전제, 전체 감사 자동green 제외. |
| DOC-14 | C | narrow / accept | PLAN-02 | 내 bad YAML old A3 green 재확인; lint 자체는 오류 보고. tags 선택 이견은 FINAL-03. |
| DOC-15 | C | narrow / accept | PLAN-02, PLAN-14 | internal injected unchecked≠production CLI failure. invalid/unchecked predicate와 browser render 분리. |
| DOC-16 | C | accept / narrow | PLAN-02 | STATE-18 중복; page hash만 아닌 wiki allowlist snapshot·첫날/당일 반복으로 수락. |
| DOC-17 | C | accept / narrow | PLAN-01, PLAN-02 | JSON/show 예외와 touch≠content dirty 정정. 모든 command/banner/consumer 정상 보장 제외. |
| DOC-18 | C | narrow / narrow | PLAN-01 | useful 작은 fallback/현재 사실 정정 유지; 구조 효용 미측정, current invariant failure 기각; FINAL-08. |
| DOC-19 | C | accept / narrow | PLAN-01, PLAN-13 | Enter slug jump와 autocomplete 선택 구별. 문구 최소 정정, unique-title branch는 선택. |
| DOC-20 | C | narrow / accept | PLAN-01, PLAN-03 | whole-wiki sync의 실제 add-A 범위 유지. 기존 full 허가 존중, 좁은 위임의 범위 안내; pull-only 보류. |
| RED-C-01 | RED→DEF→결정 | narrow / narrow | PLAN-02 | 단독 replay 격리 공백만 최종 채택, 과거 외부 env 불명은 남김. 실제 사용자 HOME 오염 주장 제외. |
| RED-C-02 | RED→DEF→결정 | narrow / narrow | PLAN-08 | blank+no note500 vs note request-only 접수 구별. minimum4xx와 blank 지원 선택 분리. |
| RED-C-03 | RED→DEF→결정 | narrow / narrow | PLAN-01, PLAN-08 | 내 actual local CLI pretty2values/JSONL 아님 재확인. contract 영향 narrow와 사실 확정을 분리; FINAL-04. |

자기 source HEAD/index/tracked 내용과 고정 audit 문서의 종료 대조 및 scratch 제거는 [종료 검증](evidence/final-final-check.json)에 기록한다. 제품 코드·기존 문서·원장·다른 검토자 파일은 수정하지 않았다.
