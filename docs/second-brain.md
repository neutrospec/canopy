# Second Brain 루프

> Karpathy: "The wiki is a persistent, **compounding** artifact."
> compounding은 양방향이다 — 지식을 넣는 것만으로는 창고이고,
> **위키가 먼저 말을 걸어야** second brain이다.
>
> 이 문서는 그 "되돌려주는 절반"의 설계와 운영법. 원 설계는 위키의
> [[wiki-distill-express]] / [[wiki-resurface-distill-express]] (2026-06-01,
> Tiago Forte + Andy Matuschak 기반)이며, canopy가 당시 L1(데이터 소스)을 전부
> 내장하면서 대폭 단순해졌다.

## 역할 분담 (philosophy.md 원칙 6)

```
canopy  (결정론, 코드)          hermes  (판단, LLM)              사용자
─────────────────────          ─────────────────────           ──────────
resurface: 잊힌/허브 후보   →   후보 중 오늘 보낼 것 판단    →   Telegram 수신
bridge:   유사-미연결 페어  →   "왜 관련 있는지" 문장화       →   👍/👎/링크 결정
state:    노출/피드백 기록  ←   feedback 명령으로 기록        ←   반응
```

canopy는 절대 메시지를 만들지 않고, hermes는 절대 후보를 임의로 고르지 않는다.

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
커밋되어 기기 간 동기화된다(`.canopy/` 캐시와 다른 이유가 이것).

## hermes 운영 레시피

원 설계의 v1 → v2 진행을 그대로 따른다. **v1부터 시작하고 반응을 보고 늘린다.**

**v1 — 주간 저널 (토 10:00 cron)**
```bash
canopy resurface -n 5 --strategy auto --json   # 후보 5
canopy bridge -n 2 --json                      # 연결 제안 2
# hermes: 각 pick의 excerpt/explanation으로 30-50줄 저널 작성 → Telegram
# hermes: 마지막에 canopy sync -m "resurface: weekly journal state"
```

**v1.5 — Daily Highlight (평일 09:00)**: `canopy resurface -n 1 --json` → 5-10줄.

**v2 — 반응 처리**: 사용자의 👍/👎/스누즈를 받으면
`canopy resurface feedback <slug> --up|--down|--snooze 7`.
bridge에 "연결해줘"라고 답하면 hermes가 실제로 두 페이지에 [[링크]]를 추가하고
(`canopy update` 로 마무리), "아니야"면 `canopy bridge --dismiss a:b`.

**금지**: 후보를 무시하고 hermes가 임의 페이지를 고르는 것, state 파일 직접 편집.

## 로드맵 (P2–P4) — 아직 점검 명령이 없으므로 invariants가 아니라 여기 있다

- **P2 — Query 파일링 루프**: `canopy recall "질문" --json`이 청크 단위 근거+출처
  slug를 반환(에이전트 컨텍스트 주입용). 스킬 규칙: 재도출 비용이 큰 답은
  `canopy new`로 파일링. Karpathy의 "질문할수록 위키가 부자가 된다" 복원.
- **P3 — Semantic Lint (야간 증류)**: 주 1회 hermes가 `canopy bridge` + 고유사
  페어를 받아 모순 탐지·통합 제안·커버리지 갭 판단. canopy 쪽 후보 명령 추가 검토:
  `canopy lint --semantic-candidates`.
- **P4 — Express**: `canopy digest --since 90d` (분기 회고 소재), decision 태그
  시계열. 위키 → 외부 산출물.

각 항목은 구현 시 invariants.md에 점검 항목을 먼저 추가한다.
