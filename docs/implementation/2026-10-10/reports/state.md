# 상태 보존과 통합 구현

PLAN-04/06/07/08의 상태 부분을 V4–V8 등록 뒤 구현했다. 구현자가 기록한 시험 결과이며
독립 검토의 대체물이 아니다. 공개 정책은 [state-contract](../../../state-contract.md)에 있다.
기준 감사와 그 manifest는 수정하지 않았다.

## 변경과 실패 동작

공통 `statefile.Patch`는 latest raw JSON에 known snapshot 이후의 변경만 적용한다.
unknown object 필드·큰 정수를 유지하고 disjoint key는 합친다. 같은 leaf의 stale
수정, 해석할 수 없는 배열 변경과 손상된 root는 게시 전에 거부한다. per-file guard는
위키 lease 안에서만 획득하고 파일은 완성 후 원자적으로 게시한다.

reads는 사람·agent 파일의 사용 가능성을 분리한다. 한쪽이 손상돼도 다른 healthy
기록은 조회·저장할 수 있다. rename은 agent-only 기록을 옮기고 사람 count는 합치며,
양쪽 agent count의 distinct-day union을 추측하지 않고 destination 충돌을 사전 거부한다.
corrupt/extended 기록도 mutation의 exclusive lease 아래 사전 점검한다.

tasks는 queue의 전체 RMW를 보호한다. connect의 기존 ID 표시가 다른 pair와 충돌하면
payload를 비교하고 SHA256 구분 ID를 사용한다. edit는 임의 nonce와 exclusive complete
publication으로 접수한다. close는 raw JSON의 종료 필드만 바꾼다. GC는 열거 filename과
내부 ID가 맞고 known version/status/closed instant인 파일만 지운다. future·pending은
보존하고 잘못된 ID/파일은 오류로 돌려준다.

resurface/reconcile도 raw patch를 사용한다. 새 snooze는 version 2의 RFC3339 duration이고
legacy 날짜를 읽는다. future version은 낮추지 않는다. 시간 조회·정렬·GC는 offset을
포함한 actual instant로 판단한다. attention의 invalid timestamp는 GC로 지우지 않는다.
현재 attention 조회는 일치하는 이벤트 전체를 읽어 시간순으로 정렬하므로 긴 이벤트
이력의 메모리가 출력 limit에 비례한다고 주장하지 않는다.

global TOML default 추가는 semantic unknown 값을 보존한다. 위키 TOML 재작성은
unknown key가 있으면 원본을 유지하며 거부한다. tags 존재는 lint 진단이며 scalar/null
관용 읽기를 유지한다. 태그 사용량은 페이지별 중복을 세지 않는다.

CLI status는 unavailable count를 null/availability/diagnostic으로 표시한다. resurface는
healthy attention half를 유지한다. bridge는 current-source/profile 갱신 뒤 벡터를 읽고
coverage를 공개한다. count와 GC duration overflow는 상태를 열기 전에 거부한다.
검색·recall·reindex의 command context는 indexer와 inference gate에 전달한다.

후속 통합에서 링크 graph의 주소 불일치도 재현했다. `concepts/nested/foo.md`, 잘못된
디렉토리, dotted slug의 모호한 bare spelling이 basename `foo`로 연결되던 세 경우가
실패했다. Scan은 전체 namespace가 정해진 뒤 original target을 Find로 해석하고,
해결된 slug만 graph edge로 쓰며 unresolved/ambiguous target은 BrokenLinks와 lint에 남긴다.
독립적인 정상 대조는 explicit `concepts/foo.md`와 `concepts/foo.md.md`였다.

## 실행 근거

임시 HOME/XDG config/cache/data/state/TMPDIR에서 실행했다. module cache만 읽기 전용으로
재사용하고 실제 사용자 위키는 열지 않았다. 모든 test process에 TestMain 격리를 넣었으며
`scripts/test-go.sh`는 Go 시작 전에도 같은 격리를 적용한다.

- `go test -race -count=1`의 statefile/reads/tasks/resurface/reconcile/attention/config/lint
  전체 8개 package 통과: [원출력](state-tests.txt).
- tasks의 동일 시각 8개 child process가 성공한 8개 요청을 모두 보존했다. reads는 서로
  다른 key를 합치고 same-leaf 충돌 뒤 원래 file hash를 보존했다.
- old/current/future JSON의 top/nested unknown·9007199254740993 보존, null/array/bad JSON
  원본 유지, agent/human partial availability, filename/ID mismatch GC 보존을 시험했다.
- 1/7일 snooze의 offset -12/-7/UTC/+9/+14와 자정 전후·직후/만료 경계, legacy 날짜,
  이벤트 Query/Top/Prune/Weekly의 cutoff·미래·invalid 대조를 시험했다.
- status의 corrupt state→healthy zero와 resurface/bridge의 bounds-before-wiki는 수정 전
  실패를 확인했다. 수정 뒤 해당 command tests가 통과했다. GC overflow·address graph·
  rendered original-target tests도 통과했다.

[이 보고서 작성 시 source hashes](state-source-manifest.json)는 이후 독립 검토 수정과
구별한다. 최종 전체 소스/배포물 검사는 별도 통합 기록으로 남긴다.

## 남은 경계

복수 state 파일의 transaction/rollback, 외부 editor/Git의 협력 없는 경쟁, 분산 파일
시스템, 전원 장애는 보장하지 않는다. resurface version 2 snooze 기간 중에는 추천기를
운영하는 구버전 client도 업그레이드해야 한다. tasks/cooldown의 과거 slug는 자동으로
재작성하지 않으며 현재 페이지를 보고 다시 판단한다. 이러한 정책을 은밀한 이행이나
데이터 삭제로 대신하지 않았다.
