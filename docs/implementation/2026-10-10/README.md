# 감사 후 개선 구현

사용자가 2026-10-10에 [감사 개선안](../../review/2026-10-09/plan.md)의 진행을
승인했다. 기준 소스는 `ede67e302a8487aecbb4d59587a8686292018cf6` (`v0.10.0`)이다.
감사 기록은 당시 증거로 보존하며, 이 문서는 이번 구현의 선택과 결과를 기록한다.
14개 계획의 선택한 최소 정책을 구현하고 독립 반박을 반영했다. 원 감사의 95개
항목과 마지막 12개 이의는 [coverage.json](coverage.json)에서 원 판정 그대로
추적한다. 매핑의 존재를 개별 항목의 검증 통과로 세지 않는다.

| 묶음 | PLAN | 구현과 확인 범위 | 상태 |
|---|---|---|---|
| 문서 | 01, 02 | 두 내장 스킬, 사용자 계약·번역, 실행 가능한 불변식 | 구현·독립 검토 반영 |
| 보존 | 03–06, 08 | Git 충돌, 원자적 게시·협력 잠금, checkout 복구, 식별·링크·YAML, mutation JSON | 구현·독립 재현 재검사 |
| 상태 | 04, 06–08 | unknown 보존, 오류·시간·GC, 제안 ID와 동시 접수 | 구현·독립 검토 반영 |
| 검색 | 09–11 | 현재 근거·profile·입력·취소·평가, 모델·native 준비 | 구현·독립 반박 및 조정자 회귀 검사 |
| 웹 | 12–14 | HTTP 경계, 상태가 정확한 밀도 높은 UI, 렌더링·배포 고지 | 구현·독립 HTTP 재검사; 브라우저 미검증 |
| 배포 | 02, 11, 14 | 격리된 CLI 검사·CI, 실제 빌드·웹 배포·릴리스·Homebrew | 실제 배포 결과는 별도 실행 기록 |

## 이번에 선택한 정책

- frontmatter 다섯 키는 lint에서 존재를 진단한다. `tags: []`, scalar, null의
  관용 읽기는 유지한다. 모든 쓰기의 새 필수 gate로 확대하지 않는다.
- mutation `--json`은 `new/update/mv/archive/rm/checkin`의 단일 결과 object다.
  요청한 sync의 commit/push 결과와 적용·후처리 실패를 합성한다. 초기 인자/bootstrap
  실패의 stderr와 nonzero 예외는 명시한다. 공개 필드는 코드 변경 전에 문서화한다.
- 새 snooze는 접수 시각부터 정해진 일수의 duration이다. legacy 날짜는 기존
  저장값을 읽되, 새 값과 의미를 구별한다. dismiss/GC는 기존 유한 보존 기간을
  설명하며 영구 보존을 약속하지 않는다.
- 비어 있는 본문과 요청이 모두 없는 제안은 4xx로 거부한다. 요청만 있는 제안은
  허용한다. 과거 본문 snapshot/origin과 본문 전체 삭제 제안은 이번 범위에서 추가하지 않는다.
- 실행 가능한 raw HTML은 기본 렌더링에서 허용하지 않는다. 생성 wikilink·표·수식·
  Mermaid는 유지한다. 허용 Host는 실제 listener와 명시한 추가 host에 한정하고,
  forwarded header를 권한의 근거로 사용하지 않는다.
- 기존 pooling은 유지한다. 무시되던 비기본 embedding 설정은 stderr 경고와 실제
  effective 값으로 드러낸다. 작은 retrieval 평가는 정해 둔 corpus 안에서만 판정한다.
- alias가 가리키는 같은 위키의 협력 쓰기와 sync를 보호한다. 직접 editor·외부 Git,
  전원 장애, 분산 잠금에 대한 보장은 추가하지 않는다. 모델·네트워크 비용과 잠금
  구간을 구분한다. 구체 프로토콜은 보존 구현 보고서에 기록한다.

선택하지 않은 대안은 원 감사의 판정을 유지한다. 전체 transaction/rollback 체계,
DB·CRDT·ANN 도입, 전역 page UUID, pooling 즉시 교체, 전체 플랫폼/브라우저 행렬,
origin retention을 이번 완료 조건에 섞지 않는다.

## 검증과 독립 검토

각 묶음은 불변식 → 실패 재현과 정상 대조 → 구현 → 해당 검사 → 다른 작업자의
적대적 검토 순으로 확인했다. 수정과 검토 작업자의 파일 소유권을 분리했다.
Go/CLI 실행은 임시 HOME/XDG/wiki를 사용하고 실제 사용자 위키는 개발 fixture로 쓰지 않는다.
역사 감사의 source-unchanged 검사를 구현 검사로 재사용하지 않는다.

```mermaid
flowchart LR
  Contract["불변식과 정책"] --> Repro["실패·정상 대조"]
  Repro --> Change["구현"] --> Independent["독립 반박과 수정"]
  Independent --> Checks["소스·배포물 검사"] --> Publish["배포와 릴리스"]
```

브라우저 paint/focus/IME, native compile/load/infer, archive 고지, Homebrew 설치는
각각 실제 실행 여부를 보고한다. 대체 논리 시험을 해당 환경의 합격으로 표시하지 않는다.

[조정자 판정](adjudication.md)은 독립 반박의 처리와 현재 한계를 정리한다.
[최종 대조](reports/final-challenge.md)는 누락된 digest 날짜 처리, 업로드 거부,
미래 task 완료 정책과 G2 검사 강도를 추가로 지적했고, 이를 수정한 소스를 확인했다.
검색 검토자는 원시 결과를 남긴 뒤 provider 응답 중단으로 최종 보고를 제출하지 못했다.
[검색 종합 기록](reports/search-adversarial.md)은 조정자 작성이며, SA-01의 독립
재검사와 SA-02–07의 조정자 회귀 검사를 구분한다.

변경 중 실행한 `reports/pre-final-*`의 일곱 명령은 모두 성공했지만 소스가 바뀌어
최종 통과로 인정하지 않았다. 고정 소스 전체 검사의 결과와 각 파일 해시는
`reports/final-checks.json`을 기준으로 읽는다. 실제 릴리스·아카이브·Homebrew·기존
서비스 검증은 그 뒤의 별도 배포 기록으로 확인한다.

현재 host의 실제 ORT compile/load/infer와 고정 6페이지·10질의 평가는 실행했다.
공식 CLS parity, 다른 native 플랫폼, 실제 대형 양자화는 미검증이다. Orca의 원격
runtime은 브라우저 탭 생성을 거부했으므로 실제 paint/focus/IME/U5/offline-browser도
미검증이다. 협력 잠금은 직접 editor·외부 Git·다른 머신의 쓰기를 보호하지 않으며,
전원 장애나 전체 위키 rollback을 보장하지 않는다.
