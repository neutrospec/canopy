# Second Brain 루프

> Karpathy: "The wiki is a persistent, **compounding** artifact."
> compounding은 양방향이다 — 지식을 넣는 것만으로는 창고이고,
> **위키가 먼저 말을 걸어야** second brain이다.
>
> 이 문서는 그 "되돌려주는 절반"의 설계와 운영법. 원 설계는 Tiago Forte의
> Distill/Express와 Andy Matuschak의 노트 원칙에 기반한 Resurface 설계(2026-06)이며,
> canopy가 당시 L1(데이터 소스)을 전부
> 내장하면서 대폭 단순해졌다.

## 역할 분담 (philosophy.md 원칙 6)

```
canopy  (결정론, 코드)          에이전트 (판단, LLM)             사용자
─────────────────────          ─────────────────────           ──────────
resurface: 잊힌/허브 후보   →   후보 중 오늘 보낼 것 판단    →   Telegram 수신
bridge:   유사-미연결 페어  →   "왜 관련 있는지" 문장화       →   👍/👎/링크 결정
state:    노출/피드백 기록  ←   feedback 명령으로 기록        ←   반응
```

canopy는 절대 메시지를 만들지 않고, 에이전트는 절대 후보를 임의로 고르지 않는다.

## 명령

```bash
canopy resurface [-n 1] [--strategy auto|random|hub] [--peek] [--json]
canopy resurface feedback <slug> --up | --down | --snooze <days>
canopy bridge [-n 5] [--min-sim 0.70] [--peek] [--dismiss a:b] [--json]
```

- **random-forgotten**: 30일+ 미접촉 페이지를 나이 가중 무작위로. 오래될수록 잘 나온다.
- **stale-hub**: 백링크 4개 이상인데 60일+ 미갱신 — "여전히 많이 참조되는데 낡은" 페이지.
- **auto**: 70% random / 30% hub (원 설계의 Daily 배합).
- **bridge**: 페이지 벡터(청크 평균) 코사인 ≥ 0.70인데 상호 wikilink가 없는 페어.

쿨다운: 노출 45일, 페어 90일, 👎 120일. 스누즈는 지정일까지. `--peek`은 상태 무기록.

상태는 `<wiki>/_meta/resurface/state.json` — 파생 불가능한 데이터이므로 위키 repo에
커밋되어 기기 간 동기화된다 (XDG 캐시로 빼지 않는 이유: 캐시는 재구축 가능하지만
노출 이력·피드백은 잃으면 기기 간 중복 resurface가 발생한다).

## 에이전트 운영 레시피

원 설계의 v1 → v2 진행을 그대로 따른다. **v1부터 시작하고 반응을 보고 늘린다.**

**v1 — 주간 저널 (토 10:00 cron)**
```bash
canopy resurface -n 5 --strategy auto --json   # 후보 5
canopy bridge -n 2 --json                      # 연결 제안 2
# 에이전트: 각 pick의 excerpt/explanation으로 30-50줄 저널 작성 → Telegram
# 에이전트: 마지막에 canopy sync -m "resurface: weekly journal state"
```

**v1.5 — Daily Highlight (평일 09:00)**: `canopy resurface -n 1 --json` → 5-10줄.

**v2 — 반응 처리**: 사용자의 👍/👎/스누즈를 받으면
`canopy resurface feedback <slug> --up|--down|--snooze 7`.
bridge에 "연결해줘"라고 답하면 에이전트가 실제로 두 페이지에 [[링크]]를 추가하고
(`canopy update` 로 마무리), "아니야"면 `canopy bridge --dismiss a:b`.

**금지**: 후보를 무시하고 에이전트가 임의 페이지를 고르는 것, state 파일 직접 편집.

## P2 — Query 파일링 루프 (구현됨, invariants G1–G3)

```bash
canopy recall "질문" --json [-k 6] [--per-page 2]
```

search(페이지 랭킹)와 달리 **청크 원문 + 출처 slug**를 반환한다 — 에이전트가
컨텍스트에 주입하고 `[[slug]]`로 인용하는 용도. per-page 캡으로 긴 페이지가
결과를 독점하지 못한다. 나머지 절반은 스킬 규칙이다:
**재도출 비용이 큰 답(비교·심층 종합)은 `canopy new`로 파일링하라** —
Karpathy의 "질문할수록 위키가 부자가 된다"의 복원.

## P3 — Semantic Lint 후보 (canopy 측 구현됨, invariant G6)

```bash
canopy bridge --include-linked --min-sim 0.85 --peek --json
```

링크 여부와 무관하게 고유사 페어를 공급하고 `linked` 필드로 구분한다.
`linked: true`인 고유사 페어 = **통합/모순 후보** (유사도 0.98대의 사실상 중복
페이지가 이 방법으로 발견된다). 주기 실행에서 에이전트가
모순 탐지·통합 제안·커버리지 갭을 판단한다.

## P4 — Express 소재 수집 (canopy 측 구현됨, invariants G4–G5)

```bash
canopy digest --since 90d --json   # 90d | 12w | 3m | YYYY-MM-DD
```

기간 내 생성/갱신 페이지, 신규 페이지 태그 분포, decision 태그 시계열(전 기간)을
구조화 출력. 분기 회고("이 3개월 내가 뭘 알게 됐나")·주간 저널의 소재.
문장화는 에이전트가 한다.

## 남은 로드맵

- 커버리지 갭 후보: raw/ 최근 유입과 위키 페이지의 의미적 커버리지 비교 (원 설계 전략 5)
- 피드백 가중치 자동 조정 (원 설계 v3) — feedback 데이터가 쌓인 뒤

각 항목은 구현 시 invariants.md에 점검 항목을 먼저 추가한다.
