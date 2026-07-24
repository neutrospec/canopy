---
name: canopy-wiki
description: LLM Wiki 관리 — 모든 위키 조작은 canopy CLI로. 검색(하이브리드), 페이지 생성/수정/이동, sync. 규칙은 CLI가 코드로 강제하므로 체크리스트 암기 불필요.
category: note-taking
---

# Canopy Wiki

위키의 모든 조작은 **`canopy` CLI 하나로** 한다. 과거의 수동 체크리스트(index.md 갱신,
JSONL 로그, 임베딩 동기화, 스키마 검증)는 전부 canopy가 자동 수행한다 — 손으로 하지 마라.

- Wiki 경로: `~/workspace/wiki` (`canopy.toml`이 있는 곳). 다르면 `--wiki <path>`.
- 모든 명령은 `--json` 지원 (파싱해서 쓸 것).
- `index.md`, `index/*.md`, `logs/*.jsonl`은 **canopy 생성물 — 절대 직접 편집 금지.**
- 이 SKILL.md도 canopy가 설치·갱신한다 (`canopy skills install`이 덮어쓴다).
  **직접 수정하거나 옆에 별도 명령 레퍼런스 노트를 만들지 마라** — 손 노트는
  CLI 업그레이드 순간 낡는다. 명령 표면이 궁금하면 `canopy --help`가 진실 소스.

## 검색 (페이지 생성 전 필수)

```bash
canopy search "질의"                  # hybrid: 키워드 + 시맨틱 (bge-m3 내장, 외부 서버 없음)
canopy search "질의" --mode keyword   # 빠름 (모델 로드 없음), 정확한 용어 매칭
canopy list [--type T] [--tag t]     # 전체 페이지 목록 (slug/type/title) — ls로 훑지 마라
canopy backlinks <page>               # 이 페이지를 참조하는 페이지들
canopy show <page>                    # 페이지 내용 확인 (view도 동작; 경로 헤더는 stderr, 본문만 stdout)
canopy tags                           # 유효 type·태그 taxonomy (new가 검증에 쓰는 목록 그대로)
```

시맨틱/hybrid 검색의 첫 호출은 모델 로드가 선행된다 (워밍업 후 ~0.5s, 콜드 캐시면
수 초). 로드가 부담스러운 단순 조회는 `--mode keyword`나 `list`를 써라.

한국어/영어 모두 잘 된다. **새 페이지를 만들기 전에 반드시 검색해서** (1) 중복 방지,
(2) 연결할 관련 페이지 파악을 하라.

## Recall — 위키를 네 메모리로 쓰기

```bash
canopy recall "질문" --json    # 청크 원문 + 출처 slug (search는 페이지, recall은 근거)
```

- **환경/인프라/과거 설정 질문**("어떻게 설정했더라?", 홈랩·proxy·서버 구성)은
  답하기 전에 recall 먼저. 답에는 출처를 `[[slug]]`로 인용.
- **재도출 비용이 큰 답**(비교 분석, 심층 종합)을 만들었으면 증발시키지 말고
  `canopy new`로 파일링하라 — 질문이 위키를 키우는 루프.

## 페이지 생성

```bash
echo "$BODY" | canopy new "제목" --type concept --tags ai-ml,tool \
  --body-file - --links related-page-1,related-page-2
```

- `--type`: entity(사람/조직/제품/하드웨어) | concept(개념/방법/가이드) | comparison(비교)
- `--tags`: taxonomy에 있는 것만 — 목록은 `canopy tags --json`으로 확인 (위반 시 명령이 거부한다; 새 태그가 정말 필요하면 canopy.toml 수정이 먼저)
- 제목이 한글뿐이면 `--slug english-slug` 필수 (파일명은 영문 강제)
- 본문에 `<`/`>`가 들어가면(`<password>`, `<path>` 등) `echo … | --body-file -`
  파이프가 셸 리다이렉션으로 오해석되어 본문이 조용히 잘릴 수 있다 —
  본문은 임시 파일에 쓰고 `--body-file <파일>`로 넘기고, 생성 후 `canopy show`로 본문을 검증하라
- `--links`: **실존 페이지만** 허용 (없는 페이지면 거부). 생성 후 출력되는
  "related pages" 제안(유사도 ≥0.8, 태그 겹치는 페이지 우선)에서 진짜 관련 있는
  것만 골라라 — 유사도가 높아도 주제가 다르면 버려라, 억지 연결 금지
- 성공 시 index/log/임베딩 자동 처리됨

## 수정 / 이동 / 삭제

```bash
canopy update <page> --body-file -   # 본문 교체 + updated 갱신 (파일 직접 편집했다면 body-file 없이 실행)
canopy mv <page> --slug new-slug     # 개명 — 인바운드 위키링크 자동 재작성
canopy mv <page> --type comparison   # 카테고리 이동
canopy archive <page>                # 완전 대체된 페이지 → _archive/
canopy rm <page>                     # 백링크 있으면 거부됨 (archive를 먼저 고려)
```

파일을 에디터/스크립트로 직접 고쳤을 때도 마지막에 `canopy update <page>`를 실행해
updated 날짜·인덱스·임베딩을 갱신하라.

## Sync (작업 마무리 — 잊지 마라)

```bash
canopy sync                    # pull --rebase → commit(자동 메시지) → push → 인덱스 갱신
canopy sync -m "커밋 메시지"    # 메시지 지정
canopy new "..." --sync        # 단건 작업은 생성과 동시에 sync
```

- **여러 페이지를 만드는 배치 작업**: 전부 만든 뒤 `canopy sync` 한 번.
- **위키 작업이 포함된 모든 태스크의 마지막 단계는 `canopy sync`다.**
  모든 canopy 명령이 시작할 때 미동기 상태 배너(⚠)를 띄우니, 배너가 보이면 sync가 밀린 것.
- push 실패 시 커밋은 로컬에 안전하다. `canopy sync` 재실행.

## Second Brain 루프 (resurface / bridge)

위키가 축적한 지식을 사용자에게 되돌려주는 루프. **후보 선정은 canopy가, 판단·문장화·전달은 네가** 한다.
canopy가 주는 후보를 무시하고 임의 페이지를 고르지 마라. state 파일(`_meta/resurface/`)은 직접 편집 금지.

```bash
canopy resurface -n 1 --json                     # 잊힌 페이지/낡은 허브 후보 (노출 이력 자동 기록)
canopy bridge -n 3 --json                        # 유사한데 연결 안 된 페어
canopy resurface feedback <slug> --up|--down|--snooze 7   # 사용자 반응 기록
canopy bridge --dismiss a:b                      # 사용자가 "관련 없다"고 한 페어 영구 제외
```

- cron 저널/하이라이트: 후보의 excerpt/explanation을 바탕으로 짧게 문장화해 Telegram 전송,
  끝에 `canopy sync -m "resurface state"`.
- 사용자가 bridge에 "연결해"라고 하면: 두 페이지의 관련 섹션에 [[상호 링크]]를 추가하고
  각각 `canopy update <page>` 실행.
- 미리보기만 필요하면 `--peek` (state 무기록).

## 건강 검사

```bash
canopy status          # 페이지 수, git 상태
canopy lint            # broken link, orphan, island, 스키마 위반, stale (--json으로 파싱)
canopy backlinks --orphans
```

주기 점검(cron)은 `canopy lint --json` + `canopy status --json`을 쓰고 결과를 보고하라.

`island` finding = 서로는 링크돼 있지만 본토(최대 연결 성분)와 끊어진 클러스터.
고아 검사는 통과하므로 별도 처리 필요: **finding에 나열된 섬 구성원과 본토 페이지
사이에 진짜 관련이 있으면** 그 두 페이지에 [[상호 링크]]를 추가해 연결하고(bridge
수락과 같은 절차), 주제상 정말 무관한 클러스터(예: 요리 레시피만 모인 섬)면
사용자에게 "의도된 섬인지" 물어보고 그렇다면 그대로 둔다. 링크 개수 채우기용
억지 연결은 금지.

## 콘텐츠 판단 기준 (코드가 강제할 수 없는 것)

- **페이지 생성 기준**: 2개 이상 소스에서 등장하거나 한 소스의 중심 주제일 때만.
  스치듯 언급된 것은 기존 페이지에 한 줄 추가로 충분하다.
- **중복 대신 갱신**: 검색에서 같은 주제가 나오면 새 페이지 대신 기존 페이지를 보강.
- **모순 처리**: 새 정보가 기존 내용과 충돌하면 조용히 덮어쓰지 말고 두 주장을
  날짜·출처와 함께 병기.
- **위키링크는 진짜 관련만**: 개수 채우기 금지. 관련 페이지가 없으면 비워둔다.
- **본문 언어**: 한국어 기본, 고유명사/기술용어 영어 허용. 제목(title)은 한글 가능,
  파일명(slug)만 영문.
- **페이지 길이**: 1000줄 넘으면 분할 후보 (lint가 알려준다).
- **raw/ 는 불변**: 원본 소스는 수정하지 않는다. 정정은 위키 페이지에서.
