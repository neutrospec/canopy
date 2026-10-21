# 🌳 canopy

> **한국어** · [English](README.en.md)

**마크다운 위키를 위한 로컬 지식 관리 도구.** 스키마 검증·검색·웹 UI·재발견 루프를
단일 Go 바이너리로 제공합니다. 사람과 LLM 에이전트가 같은 위키를 함께 가꿀 수 있도록,
"판단은 LLM이, 불변식은 코드가" 원칙으로 설계되었습니다.

위키는 평범한 마크다운 + git 저장소입니다. canopy는 그 옆에서 명령별 검증, 인덱스 갱신,
활동 로그, 임베딩 동기화, git sync를 맡고 — Obsidian 등 기존 도구와
그대로 공존합니다.

## 주요 기능

- **검증과 후처리를 갖춘 쓰기** — `new`는 타입·태그·slug·`--links`를 검증하고,
  `checkin`은 편집 사본의 스키마와 base를 확인합니다. `update`는 기존 frontmatter를
  유지합니다. 쓰기 뒤 인덱스·로그·임베딩이 갱신되고, 이동 시 인바운드 링크를
  재작성합니다. 본문 링크와 외부 편집의 구조 문제는 `lint`로 확인합니다.
- **하이브리드 검색** — BM25 키워드 + 시맨틱 벡터(bge-m3, 완전 로컬 ONNX 추론)를
  융합합니다. 기본은 로컬 추론이며, 선택적으로 BGE-M3·EmbeddingGemma 2 등의
  외부 임베딩 API를 연결하면 내장 모델을 로딩하지 않습니다.
- **웹 UI** (`canopy serve`) — 검색에서 시작하는 위키 브라우징: 인스턴트 서치,
  백링크, hover 미리보기, dir×type×tag facet 탐색, 로컬 링크 그래프, 본문 수정 제안까지.
- **Second brain 루프** — 잊힌 페이지 재발견(`resurface`), 유사하지만 연결 안 된
  페이지 발견(`bridge`), 근거 청크 회수(`recall`), 기간 회고(`digest`).
- **에이전트 연동** — 기계 출력용 `--json`을 제공하며, 에이전트용 스킬 문서를
  바이너리에 내장해 한 명령으로 설치합니다.
- **가벼운 설치 공간** — 단일 바이너리, XDG Base Directory 준수. 위키 저장소 안에는
  스키마·파생 Markdown 인덱스·활동 로그·동기화할 상태를 두고, 검색 DB와 모델은
  위키 밖에 저장합니다.

## 설치

**Homebrew** (권장 — 소스에서 빌드하며 onnxruntime을 자동으로 링크합니다):

```bash
brew install neutrospec/tap/canopy
canopy model pull          # 시맨틱 검색 모델 bge-m3 ONNX ~2.3GB (1회)
```

아직 태그 전이라면 `brew install --HEAD neutrospec/tap/canopy`로 main을 빌드합니다.

**소스에서 직접** (개발):

```bash
brew install onnxruntime   # 시맨틱 검색용 (libonnxruntime)
make build                 # -tags ORT; 버전 각인 + 최초 1회 libtokenizers.a 자동 다운로드
make install               # ~/.local/bin (없으면 /opt/homebrew/bin)
```

내장 추론 없이 쓰려면: `make build-lite` — cgo 불필요, keyword 검색과 외부 임베딩
API를 지원합니다(GitHub 릴리스의 `*_lite` 아카이브도 동일).
외부 API 설정과 모델 변경 후 재인덱싱은 [시작 가이드](docs/getting-started.md#4-5-검색)를 참고하세요.

## 빠른 시작

```bash
# 새 폴더를 지정한다. 모델·Git·서비스 없이 첫 페이지와 검색을 확인할 수 있다.
wiki="$HOME/canopy-demo"
mkdir -p "$wiki"
canopy init --wiki "$wiki"
canopy status --wiki "$wiki" --json   # root가 지정한 위키인지 확인
canopy new "첫 페이지" --slug first-page --type concept --tags tool --wiki "$wiki"
canopy search "첫 페이지" --mode keyword --wiki "$wiki"
canopy show first-page --wiki "$wiki"
```

`init`은 비어 있는 `default_wiki`만 채우며 기존 전역 설정을 덮어쓰지 않습니다.
시맨틱 검색은 선택입니다: ORT 빌드에서 `canopy model pull` 후
`canopy reindex --wiki "$wiki"`를 실행하세요. 웹은 `canopy serve --wiki "$wiki"`로
엽니다. Git 저장과 원격 연결은 입문 가이드의 별도 단계입니다.

처음이라면 [docs/getting-started.md](docs/getting-started.md)의 15분 튜토리얼을 권합니다.

## 웹 UI

`canopy serve`는 위키를 검색-우선(search-first) 웹사이트로 띄웁니다.

- **검색이 입구** — 홈의 검색박스에서 타이핑하면 즉시 결과가 뜨고(↑↓/Enter 키보드
  탐색), slug가 정확히 일치하면 바로 그 페이지로 이동합니다. 결과에는 페이지 제목이
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
- **본문 수정 제안** — 페이지의 `✎ 편집`에서 저장하면 본문 전문이 edit 태스크로
  접수됩니다. **접수는 페이지 반영이 아닙니다.** `할 일` 화면에서 제안 상태를 보고,
  에이전트가 검토·반영한 뒤 현재 페이지를 확인하세요. canopy는 에이전트를 자동으로
  실행하지 않습니다. frontmatter와 페이지 생성/이동/삭제는 CLI로 처리합니다
  ([태스크 계약](docs/agent-tasks.md)).
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
| `canopy grep "패턴" [page]` | 줄 번호가 있는 정규식 검색 |
| `canopy status` · `canopy lint` | 위키 상태 · 스키마/링크/신선도 검사 |

**쓰기** — 실행 후 index/log/임베딩 자동 갱신

| 명령 | 설명 |
|------|------|
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | 검증된 생성 + 관련 페이지 제안 |
| `canopy update <page> [--body-file f]` | updated 갱신 (+본문 교체) |
| `canopy checkout <page>` / `canopy checkin <page>` | 위키 밖 사본에서 부분 편집 → 검증·반영·사본 회수 |
| `canopy mv <page> [--type T] [--slug s]` | 이동/개명 — 인바운드 링크 자동 재작성 |
| `canopy rm <page> [--force]` / `canopy archive <page>` | 삭제(백링크 있으면 거부) / 아카이브 |
| `canopy sync [-m msg]` | pull --rebase → Markdown 인덱스 재생성 → commit → push → 검색 캐시 갱신 |

**Second brain** — canopy가 조건·점수로 후보를 추리고, 에이전트나 사람이 판단·전달합니다. `resurface auto/random`은 적격 후보에서 표본을 고릅니다. ([상세](docs/second-brain.md))

| 명령 | 설명 |
|------|------|
| `canopy recall "질문" [-k N] [--per-page N]` | 청크 원문 + 출처 slug. 임베딩 필수; 없으면 keyword search + show 사용 |
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | 잊힌 페이지 / 낡은 허브 재발견 후보 |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | 반응 기록 → 이후 후보 선정에 반영 |
| `canopy bridge [-n N] [--min-sim 0.7] [--include-linked]` | 유사하지만 연결 안 된 페이지 페어 발견 |
| `canopy digest [--since 90d]` | 기간 회고 소재: 생성/갱신 페이지, 태그 분포 |

**관리**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install` · `canopy version` · `canopy migrate [status]`

작업 큐는 `canopy tasks list --json`, 외부 편집 검토는 `canopy reconcile --json`,
머신 활동 조회는 `canopy events --json`에서 시작합니다.
자동화는 명령별 `--json` 출력·stderr·exit를 함께 확인하세요. 인자 파싱·초기화 실패는
JSON 없이 종료될 수 있습니다. JSON 모드와 `show`에는 상태 배너가 없으므로
`canopy status --json`으로 대상·dirty·열린 사본·대기 작업을 확인합니다.
여섯 페이지 변경 명령의 응답·부분 반영·복구 계약은 [mutation-results.md](docs/mutation-results.md)에
있습니다. exit 0은 요청한 단계 완료이며 내용의 정확성까지 검증하지는 않습니다.
`--peek`(resurface/bridge)은 노출 상태를 기록하지 않습니다.

## 에이전트 연동

LLM 에이전트에게 canopy 사용법을 가르치는 스킬 2종(`canopy-wiki`: CLI 사용법과
콘텐츠 판단 규칙, `canopy-ingest`: 외부 콘텐츠 수집 워크플로우)이 바이너리에
내장되어 있습니다:

```bash
canopy skills install              # 감지된 모든 에이전트에 설치/갱신 (업그레이드 후 이 한 명령)
canopy skills install --dir <path> # 지정한 스킬 디렉토리만 설치/갱신 (없으면 생성)
```

- 자동 감지 대상: 존재하는 `~/.hermes/skills`(hermes), `~/.claude/skills`(Claude Code),
  pi의 agent 홈(`PI_CODING_AGENT_DIR`, 기본 `~/.pi/agent`)입니다. pi는 홈이 있으면
  `skills/`를 생성합니다. 일반 에이전트에는 `<skill>/SKILL.md` flat
  레이아웃으로, hermes에는 카테고리 레이아웃(`note-taking/…`)으로 설치됩니다.
  `--dir`은 임의 경로를 자동 감지에 등록하지 않으므로 이후 갱신에도 같은 경로를
  지정하세요. 설치와 활성화는 별개입니다. 해당 에이전트의 스킬 탐색·로드 규칙을 확인하세요.
- 설치본의 진실 소스는 바이너리입니다 — 설치된 SKILL.md를 직접 수정하지 마세요
  (다음 install이 되돌립니다). canopy 업그레이드 후 재실행하면 새 명령이 반영됩니다.
- 페이지 생성·수정·이동·삭제는 **canopy 명령을 거치게** 하세요. 부분 편집은
  `checkout` 사본에서 하고 `checkin`으로 반영합니다. 원본 자료를 `raw/`에 추가하거나
  `canopy.toml` 설정을 수정할 때는 해당 파일을 직접 다루되 기존 내용을 보존합니다.
  명령별 검증과 후처리를 사용하고 내용·관련 lint 결과를 확인합니다.
- 주기 실행(주간 저널, lint 등) 레시피는 [docs/second-brain.md](docs/second-brain.md) 참조.

## 데이터 배치 (XDG Base Directory 준수)

| 위치 | 성격 |
|------|------|
| `<wiki>/canopy.toml` | 위키의 스키마 (타입·태그 taxonomy) — 데이터와 함께 여행하므로 위키에 커밋 |
| `<wiki>/index.md`, `<wiki>/index/` | 전체 재생성되는 Markdown 인덱스 — 직접 편집하지 않음 |
| `<wiki>/logs/*.jsonl` | append-only 활동 이력 — 현재 페이지에서 재구축할 수 없으므로 보존 |
| `<wiki>/_meta/tasks/`, `<wiki>/_meta/reconcile/` | 위임 큐·검토 기준선 — 위키와 함께 커밋 |
| `<wiki>/_meta/resurface/` | 재현 불가 상태 (재발견 노출 이력·피드백) — 위키에 커밋 |
| `<wiki>/_meta/webui/` | 재현 불가 상태 (읽기 이력·검색 갭) — 위키에 커밋, 기기 간 동기화 |
| `<wiki>/_meta/attention/` | 재현 불가 상태 (에이전트 접근 집계, 일 단위) — 위키에 커밋 |
| `~/.local/state/canopy/attention/<해시>.db` | 접근 이벤트 상세 (머신 로컬) — 타임라인 등 리치 UI용 |
| `~/.cache/canopy/index/<해시>.db` | 파생 캐시 (FTS+벡터) — `reindex`로 언제든 재구축 |
| `~/.config/canopy/config.toml` | 전역 설정 (`default_wiki`) |
| `~/.config/canopy/state.json` | 데이터 스키마 버전 (마이그레이션 이행 지점) — 머신 로컬, 캐시 삭제·업그레이드에도 생존 |
| `~/.config/canopy/webauth.json` | 웹 UI 계정 (bcrypt 해시) — 비밀이므로 위키 밖, 머신 로컬 |
| `~/.local/share/canopy/models/` | ONNX 모델, 빌드용 정적 라이브러리 |

`XDG_CONFIG_HOME`/`XDG_CACHE_HOME`/`XDG_DATA_HOME`/`XDG_STATE_HOME` 환경변수를
존중합니다. 캐시 삭제는 활동 이력이나 `_meta/`의 복구 방법이 아닙니다. 머신-로컬
재현 불가 상태는 마이그레이션 사다리, 여행하는 `_meta/*`는 파일 안의 version,
파생 검색 DB는 캐시 버전과 재구축으로 진화합니다. 비밀은 위키에 넣지 않습니다.

## 성능

이전 Apple Silicon·int8 양자화 모델 측정값입니다. 이번 릴리스의 전체 성능·품질 보증은 아닙니다.

- 임베딩 ~11ms/청크, 모델 로드 ~0.5초(워밍업 상태) — fp32 대비 2.4× 빠르고
  임베딩 품질은 코사인 유사도 ≥0.988로 유지
- 무변경 `reindex` ≈ 1초, keyword 검색 <100ms (모델 로드 없음), 웹 UI 인스턴트
  서치 ~40ms/쿼리
- int8 양자화는 선택입니다. `canopy model pull`(fp32) 후 저장소의
  `scripts/quantize-model.py --help`에서 변환 의존성·경로를 확인하고 실행합니다.
  기본 출력은 별도 `bge-m3-int8` 디렉토리입니다. 대상 위키의 `canopy.toml`에서
  `[embedding]`의 `model`을 `bge-m3-int8`로 바꾸고 같은 `--wiki`로
  `canopy model status`를 확인한 뒤 `canopy reindex`를 실행합니다.

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
| [docs/upgrading.md](docs/upgrading.md) | 설치·운영 이행 가이드 (소스/make install → Homebrew + brew services, 정기 업그레이드) |
| [docs/troubleshooting.md](docs/troubleshooting.md) | 자주 만나는 문제 (PATH, 시맨틱 검색, 웹 UI 인증) |
| [docs/mutation-results.md](docs/mutation-results.md) | 쓰기 JSON·부분 반영·checkout 복구·협력 쓰기 범위 |

**설계 기록** — 각 기능이 어떤 결정으로 태어났나

| 문서 | 내용 |
|------|------|
| [docs/versioning.md](docs/versioning.md) | 세 버전 번호·마이그레이션 사다리·릴리스 절차·Homebrew 배포 |
| [docs/i18n.md](docs/i18n.md) | 다국어 문서 관리 — 소스/번역, staleness 점검(invariants L) |
| [docs/homebrew-guide.md](docs/homebrew-guide.md) | Homebrew 배포·릴리스 실습 가이드 (처음 하는 사람용) |
| [docs/web-ui-plan.md](docs/web-ui-plan.md) | 1차(M1–M4): 검색-우선 뷰어, facet, 웹 편집 |
| [docs/web-ui-plan-2.md](docs/web-ui-plan-2.md) | 2차(M5–M8): 보안, 읽기 이력·새발견, 제안 링크, 말 거는 홈 |
| [docs/web-ui-plan-3.md](docs/web-ui-plan-3.md) | 3차(M9–M10+): 현대 뷰어, 지식 그래프, 섬 검출 |
| [docs/web-ui-plan-4.md](docs/web-ui-plan-4.md) | 4차(M11–M13): 주의(attention) 2계층 저장, 접근 추적, 계기판·비주얼 리프레시 |
| [docs/web-ui-write-design.md](docs/web-ui-write-design.md) | 과거 직접 저장 설계와 명시적 sync 결정 — 현재 웹 제안 계약은 agent-tasks 참고 |
| [docs/web-ui-i18n.md](docs/web-ui-i18n.md) | 웹 UI 다국어 — go-i18n, 로케일 협상, 언어 선택 (invariants M) |
| [docs/reconcile-design.md](docs/reconcile-design.md) | 정규화 게이트 설계 — 편집은 어디서든, 정규화는 한 문으로 (모든 문의 변경을 본길이 재판단) |
| [docs/agent-tasks.md](docs/agent-tasks.md) | 에이전트 태스크 큐 — 웹의 위임(연결·수정 요청)을 파일 큐로, 완료는 유형별 코드 검증으로 (invariants T) |
| [docs/events.md](docs/events.md) | 이벤트 로그 일반화 — 머신-로컬 sqlite에 주의+라이프사이클 관측 타임라인, 비권위·best-effort (invariants N) |
| [docs/web-ui-board.md](docs/web-ui-board.md) | 실행 보드 — 마일스톤별 작업·완료 기준(Exit)의 전 기록 |

**검증** — 지금 건강한가

| 문서 | 내용 |
|------|------|
| [docs/invariants.md](docs/invariants.md) | 점검 가능한 불변식 목록 + 실행 전제·감사 절차 |

기여 규칙 하나: **점검 명령이 없는 주장은 원칙이 아닙니다.** 새 기능은 불변식과
점검 방법을 invariants.md에 먼저 적고 구현합니다. 개발 규칙(빌드·버전·마이그레이션·
커밋 규약)은 [AGENTS.md](AGENTS.md)에 정리돼 있습니다.

## 개발 방식에 대하여

이 프로젝트의 코드 대부분은 **AI 지원 코딩**(Claude Code)으로 작성되었습니다.
방향과 판단은 사람이, 구현·검증은 AI가 맡는 방식으로 — 이 도구가 위키에
적용하는 "판단은 LLM이, 불변식은 코드가" 원칙과 같은 구도입니다. 각 기능이
어떤 결정을 거쳐 태어났는지는 위 "설계 기록" 문서들로 추적할 수 있습니다.
이 공개가 곧 귀속이므로, 이후 커밋 메시지에는 AI 공저자 트레일러를 따로 달지
않습니다(초기 일부 커밋에는 남아 있습니다 — 역사는 다시 쓰지 않습니다).

## 라이선스

[MIT](LICENSE)
