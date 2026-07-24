# 🌳 canopy

**마크다운 위키를 위한 로컬 지식 관리 도구.** 스키마 검증·검색·웹 UI·재발견 루프를
단일 Go 바이너리로 제공합니다. 사람과 LLM 에이전트가 같은 위키를 함께 가꿀 수 있도록,
"판단은 LLM이, 불변식은 코드가" 원칙으로 설계되었습니다.

위키는 평범한 마크다운 + git 저장소입니다. canopy는 그 옆에서 스키마 검증, 인덱스 갱신,
활동 로그, 임베딩 동기화, git sync를 코드로 강제하고 — Obsidian 등 기존 도구와
그대로 공존합니다.

## 주요 기능

- **스키마가 강제되는 쓰기** — `new/update/mv/rm/archive` 모든 변경이 타입·태그
  taxonomy 검증을 거치고, 인덱스·로그·임베딩이 자동으로 따라 갱신됩니다.
  이동 시 인바운드 위키링크까지 자동 재작성됩니다.
- **하이브리드 검색** — BM25 키워드 + 시맨틱 벡터(bge-m3, 완전 로컬 ONNX 추론)를
  융합합니다. 한국어·영어 혼용 문서에서 동작하며, 외부 API 호출이 없습니다.
- **웹 UI** (`canopy serve`) — 검색에서 시작하는 위키 브라우징: 인스턴트 서치,
  백링크, hover 미리보기, dir×type×tag facet 탐색, 로컬 링크 그래프, 본문 편집까지.
- **Second brain 루프** — 잊힌 페이지 재발견(`resurface`), 유사하지만 연결 안 된
  페이지 발견(`bridge`), 근거 청크 회수(`recall`), 기간 회고(`digest`).
- **에이전트 연동** — 모든 명령이 `--json`을 지원하고, 에이전트용 스킬 문서가
  바이너리에 내장되어 한 명령으로 설치됩니다.
- **가벼운 설치 공간** — 단일 바이너리, XDG Base Directory 준수. 위키 저장소 안에는
  스키마 파일(`canopy.toml`) 외에 아무것도 남기지 않습니다.

## 설치

```bash
brew install onnxruntime   # 시맨틱 검색용 (libonnxruntime)
make build                 # -tags ORT; 최초 1회 libtokenizers.a 자동 다운로드
make install               # ~/.local/bin (없으면 /opt/homebrew/bin)
```

임베딩 없이 쓰려면: `make build-lite` — cgo 불필요, keyword 검색만 제공됩니다.

## 빠른 시작

```bash
# 1. 위키 지정 (어느 디렉토리에서든 canopy가 위키를 찾도록)
mkdir -p ~/.config/canopy
echo 'default_wiki = "/path/to/wiki"' > ~/.config/canopy/config.toml

# 2. 위키 채택 — 스키마(canopy.toml) 생성 + 인덱싱
canopy init

# 3. (선택) 시맨틱 검색 준비 — 임베딩 모델 다운로드 후 전체 인덱싱 (1회)
canopy model pull          # bge-m3 ONNX ~2.3GB
canopy reindex

# 4. 사용 시작
canopy new "첫 페이지" --type concept --tags tool
canopy search "아무거나"
canopy serve               # → http://localhost:8737
```

처음이라면 [docs/getting-started.md](docs/getting-started.md)의 15분 튜토리얼을 권합니다.

## 웹 UI

`canopy serve`는 위키를 검색-우선(search-first) 웹사이트로 띄웁니다.

- **검색이 입구** — 홈의 검색박스에서 타이핑하면 즉시 결과가 뜨고(↑↓/Enter 키보드
  탐색), 제목이 정확히 일치하면 바로 그 페이지로 이동합니다. 결과에는 페이지 제목이
  아니라 **실제로 매칭된 문단**이 표시됩니다.
- **현대적인 마크다운 뷰어** — 코드 블록 문법 하이라이팅(서버사이드, 라이트/다크
  테마), **mermaid 다이어그램**, LaTeX 수식($…$/$$…$$, MathJax SVG), 코드 복사
  버튼, 헤딩 앵커, 각주까지. 모든 에셋이 바이너리에 내장되어 오프라인에서도
  동작하고, 필요한 페이지에서만 로드됩니다.
- **위키답게 읽기** — 위키링크는 클릭 가능한 링크로(없는 페이지는 붉은 링크),
  hover 시 대상 페이지 미리보기가 뜨고, 페이지 하단에 백링크("여기를 링크한 페이지")와
  접이식 로컬 링크 그래프가 있습니다. 없는 페이지에 접근하면 404 대신 검색 결과를
  보여줍니다.
- **인터랙티브 지식 그래프** — Obsidian처럼 위키 전체의 연결을 한 화면에서
  탐험합니다: force 레이아웃, 줌/팬/드래그, hover 시 이웃 하이라이트, 노드 찾기,
  클릭으로 페이지 이동. 안 읽은 페이지는 표시가 달라 읽을 곳이 보입니다.
- **분류는 트리가 아니라 facet** — `탐색` 메뉴에서 디렉토리×타입×태그를 교차 필터링
  합니다. 최근 변경 / 고아·오래된 페이지 / 랜덤 페이지도 제공됩니다.
- **읽기 히스토리와 새발견** — `✓ 읽음` 버튼(단축키 `r`)이나 충분한 체류+스크롤
  자동 감지로 열람을 기록하고(취소 가능), 아직 읽지 않은 페이지를 새 페이지·허브·
  최근 관심과의 유사도로 랭킹해 추천합니다(`발견` 메뉴, 읽기 진행률 포함).
  이력은 위키 저장소(`_meta/webui/`)에 저장되어 기기 간 동기화됩니다.
- **위키가 먼저 말을 겁니다** — 홈에 "오늘의 재발견" 카드(잊힌 페이지 +
  👍/👎/😴 피드백, CLI resurface와 상태 공유)와 "연결 제안" 카드(유사한데 링크
  없는 페어)가 뜨고, 각 페이지 하단에는 아직 연결 안 된 유사 페이지가
  **제안 링크**로 표시됩니다. 답을 못 찾은 검색은 갭 로그로 쌓여 페이지 생성
  후보가 됩니다(점검 → 검색 갭).
- **본문 편집** — 페이지의 `✎ 편집`으로 본문을 수정할 수 있습니다. 웹 편집은 CLI
  `update`와 완전히 같은 파이프라인(검증·인덱스·로그·임베딩)을 지나며, 편집 충돌은
  낙관적 잠금으로 감지합니다. frontmatter와 페이지 생성/이동/삭제는 CLI 전용입니다.
- **안전한 기본값** — 기본은 localhost에만 바인딩됩니다. 외부에서 접근 가능한
  주소로 열면 인증이 필수가 되며, 최초 1회 터미널에 출력되는 설정 코드로
  계정(id/pw)을 만듭니다. 다크 모드·모바일 대응. 서버는 커밋하지 않습니다 —
  git 반영은 언제나 명시적 `canopy sync`로.

```bash
canopy serve                # 기본 localhost:8737 (무인증)
canopy serve --addr :8737   # 모든 인터페이스 — 인증 필수 (최초 실행 시 설정 코드 안내)
```

## 명령 레퍼런스

**읽기**

| 명령 | 설명 |
|------|------|
| `canopy search "질의" [--mode hybrid\|keyword\|semantic] [-k N]` | 하이브리드 검색 (기본). 임베딩 스택이 없으면 keyword로 자동 강등 |
| `canopy serve [--addr :8737]` | 웹 UI (위 참조) |
| `canopy backlinks <page>` / `--orphans` | 역링크 / 고아 페이지 |
| `canopy list [--type T] [--tag t]` | 전체 페이지 목록 (slug·type·title) |
| `canopy tags` | 유효 type·태그 taxonomy 조회 (`new` 검증과 동일 소스) |
| `canopy show <page>` (alias `view`) | 페이지 출력 (경로 헤더는 stderr, 본문은 stdout) |
| `canopy status` · `canopy lint` | 위키 상태 · 스키마/링크/신선도 검사 |

**쓰기** — 실행 후 index/log/임베딩 자동 갱신

| 명령 | 설명 |
|------|------|
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | 검증된 생성 + 관련 페이지 제안 |
| `canopy update <page> [--body-file f]` | updated 갱신 (+본문 교체) |
| `canopy mv <page> [--type T] [--slug s]` | 이동/개명 — 인바운드 링크 자동 재작성 |
| `canopy rm <page> [--force]` / `canopy archive <page>` | 삭제(백링크 있으면 거부) / 아카이브 |
| `canopy sync [-m msg]` | pull --rebase → commit → push → 인덱스 갱신 |

**Second brain** — 후보 선정은 canopy(결정론)가, 판단·전달은 에이전트나 사람이 ([상세](docs/second-brain.md))

| 명령 | 설명 |
|------|------|
| `canopy recall "질문" [-k N] [--per-page N]` | 청크 단위 근거 + 출처 slug (에이전트 컨텍스트 주입용) |
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | 잊힌 페이지 / 낡은 허브 재발견 후보 |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | 반응 기록 → 이후 후보 선정에 반영 |
| `canopy bridge [-n N] [--min-sim 0.7] [--include-linked]` | 유사하지만 연결 안 된 페이지 페어 발견 |
| `canopy digest [--since 90d]` | 기간 회고 소재: 생성/갱신 페이지, 태그 분포 |

**관리**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install`

모든 명령이 `--json`을 지원합니다. `--peek`(resurface/bridge)은 상태 기록 없이 미리보기합니다.

## 에이전트 연동

LLM 에이전트에게 canopy 사용법을 가르치는 스킬 2종(`canopy-wiki`: CLI 사용법과
콘텐츠 판단 규칙, `canopy-ingest`: 외부 콘텐츠 수집 워크플로우)이 바이너리에
내장되어 있습니다:

```bash
canopy skills install              # 감지된 모든 에이전트에 설치/갱신 (업그레이드 후 이 한 명령)
canopy skills install --dir <path> # 특정 디렉토리만 (없으면 생성 — 새 에이전트 최초 등록에 사용)
```

- 자동 감지 대상: `~/.hermes/skills`(hermes), `~/.claude/skills`(Claude Code) 중
  **존재하는 모든 디렉토리**. 일반 에이전트에는 `<skill>/SKILL.md` flat
  레이아웃으로, hermes에는 카테고리 레이아웃(`note-taking/…`)으로 설치됩니다.
  처음 쓰는 에이전트는 `--dir`로 한 번 설치하면 이후 자동 감지에 포함됩니다.
- 설치본의 진실 소스는 바이너리입니다 — 설치된 SKILL.md를 직접 수정하지 마세요
  (다음 install이 되돌립니다). canopy 업그레이드 후 재실행하면 새 명령이 반영됩니다.
- 핵심 규칙: **에이전트가 위키 파일을 직접 편집하지 않고 항상 canopy 명령을
  거치게** 하세요. 스키마 위반이 원천 차단됩니다.
- 주기 실행(주간 저널, lint 등) 레시피는 [docs/second-brain.md](docs/second-brain.md) 참조.

## 데이터 배치 (XDG Base Directory 준수)

| 위치 | 성격 |
|------|------|
| `<wiki>/canopy.toml` | 위키의 스키마 (타입·태그 taxonomy) — 데이터와 함께 여행하므로 위키에 커밋 |
| `<wiki>/_meta/resurface/` | 재현 불가 상태 (재발견 노출 이력·피드백) — 위키에 커밋 |
| `<wiki>/_meta/webui/` | 재현 불가 상태 (읽기 이력·검색 갭) — 위키에 커밋, 기기 간 동기화 |
| `~/.cache/canopy/index/<해시>.db` | 파생 캐시 (FTS+벡터) — `reindex`로 언제든 재구축 |
| `~/.config/canopy/config.toml` | 전역 설정 (`default_wiki`) |
| `~/.config/canopy/webauth.json` | 웹 UI 계정 (bcrypt 해시) — 비밀이므로 위키 밖, 머신 로컬 |
| `~/.local/share/canopy/models/` | ONNX 모델, 빌드용 정적 라이브러리 |

`XDG_CONFIG_HOME`/`XDG_CACHE_HOME`/`XDG_DATA_HOME` 환경변수를 존중합니다.
위키 저장소 안에 canopy가 두는 것은 위의 스키마와 `_meta/` 상태뿐이며, 전부
"위키와 함께 여행해야 하는 이유"가 있는 것들입니다. 비밀은 위키에 넣지 않습니다.

## 성능

Apple Silicon, int8 양자화 모델 기준:

- 임베딩 ~11ms/청크, 모델 로드 ~0.5초(워밍업 상태) — fp32 대비 2.4× 빠르고
  임베딩 품질은 코사인 유사도 ≥0.988로 유지
- 무변경 `reindex` ≈ 1초, keyword 검색 <100ms (모델 로드 없음), 웹 UI 인스턴트
  서치 ~40ms/쿼리
- int8 양자화는 선택입니다: `canopy model pull`(fp32) 후
  `scripts/quantize-model.py`를 1회 실행 (변환에만 파이썬 필요, 런타임은 무관)

## 문서

위(개념)에서 아래(검증)로 읽으면 모든 구현의 이유를 코드 없이 추적할 수 있습니다.

**이해** — 왜 이렇게 만들었나

| 문서 | 내용 |
|------|------|
| [docs/philosophy.md](docs/philosophy.md) | 설계 철학 — 원칙마다 강제 주체([코드]/[검출]/[협약])와 점검 명령 |
| [docs/second-brain.md](docs/second-brain.md) | 재발견 루프(resurface/bridge/recall/digest)의 설계와 운영 |

**사용** — 어떻게 쓰나

| 문서 | 내용 |
|------|------|
| [docs/getting-started.md](docs/getting-started.md) | 처음 시작하는 사람용 — 개념·용어·15분 튜토리얼·일상 사용 |
| [docs/troubleshooting.md](docs/troubleshooting.md) | 자주 만나는 문제 (PATH, 시맨틱 검색, 웹 UI 인증) |

**설계 기록** — 각 기능이 어떤 결정으로 태어났나

| 문서 | 내용 |
|------|------|
| [docs/web-ui-plan.md](docs/web-ui-plan.md) | 1차(M1–M4): 검색-우선 뷰어, facet, 웹 편집 |
| [docs/web-ui-plan-2.md](docs/web-ui-plan-2.md) | 2차(M5–M8): 보안, 읽기 이력·새발견, 제안 링크, 말 거는 홈 |
| [docs/web-ui-plan-3.md](docs/web-ui-plan-3.md) | 3차(M9–M10+): 현대 뷰어, 지식 그래프, 섬 검출 |
| [docs/web-ui-write-design.md](docs/web-ui-write-design.md) | 웹 쓰기의 동시성·충돌 설계 (단일 쓰기 파이프라인) |
| [docs/web-ui-board.md](docs/web-ui-board.md) | 실행 보드 — 마일스톤별 작업·완료 기준(Exit)의 전 기록 |

**검증** — 지금 건강한가

| 문서 | 내용 |
|------|------|
| [docs/invariants.md](docs/invariants.md) | 점검 가능한 불변식 목록(A–I) + 감사 절차 |

기여 규칙 하나: **점검 명령이 없는 주장은 원칙이 아닙니다.** 새 기능은 불변식과
점검 방법을 invariants.md에 먼저 적고 구현합니다.

## 라이선스

[MIT](LICENSE)
