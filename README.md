# canopy

LLM 위키(Karpathy-style markdown knowledge base)를 second brain으로 운영하기 위한 단일 Go CLI.

한 문장 요약: **판단은 LLM이, 불변식은 코드가.** 에이전트(hermes)가 산문 체크리스트를
외워 지키던 규칙 — 스키마 검증, index 갱신, JSONL 로그, 임베딩 동기화, git sync — 를
canopy가 코드로 강제하고, 위키가 축적한 지식을 다시 꺼내 보여주는 루프(resurface/bridge)까지
제공한다. 사람은 Obsidian으로 그대로 열람하고, 에이전트는 `--json`으로 조작한다.

## 문서 맵

| 문서 | 내용 | 언제 읽나 |
|------|------|-----------|
| [docs/getting-started.md](docs/getting-started.md) | **처음 시작하는 사람용** — 개념·용어부터 15분 따라하기까지 | LLM wiki가 처음이라면 여기부터 |
| [docs/philosophy.md](docs/philosophy.md) | 구조 철학 — 원칙마다 강제 주체와 점검 명령 | 설계 판단이 필요할 때 (구현보다 이 문서가 우선) |
| [docs/invariants.md](docs/invariants.md) | **점검 가능한 불변식 목록** + 감사 절차 | 시스템 건강 확인, 새 기능 추가 전 |
| [docs/second-brain.md](docs/second-brain.md) | resurface/bridge 설계, hermes 운영 레시피, P2–P4 로드맵 | 재발견 루프를 켜거나 확장할 때 |

문서의 규칙: **점검 명령이 없는 주장은 원칙이 아니다.** 새 기능은 불변식과 점검
방법을 invariants.md에 먼저 적고 구현한다.

## 빌드

```bash
brew install onnxruntime   # libonnxruntime.dylib
make build                 # -tags ORT; libtokenizers.a 자동 다운로드
make install               # ~/.local/bin 또는 /opt/homebrew/bin
# 임베딩 없는 경량 빌드: make build-lite (keyword 검색만, cgo 불필요)
```

### PATH 설정 (실제로 겪은 함정들)

`make install`은 `~/.local/bin`에 우선 설치하고, 디렉토리가 없으면 `/opt/homebrew/bin`으로
폴백한다. 여기서 세 가지 함정을 실제로 겪었다:

1. **`~/.local/bin`이 PATH에 없는 셸** — 대화형 rc(.zshrc)에만 PATH를 추가하면
   cron·에이전트 같은 non-interactive 셸에서 `canopy: command not found`가 난다.
   zsh 기준 `~/.zshenv`에 넣어야 모든 셸에서 잡힌다:
   `export PATH="$HOME/.local/bin:$PATH"`
2. **에이전트/cron 환경은 사용자 rc를 아예 안 읽을 수 있다** — 자동화 프롬프트에는
   절대경로를 쓰는 게 가장 확실하다. 이 프로젝트의 hermes cron 잡들도
   `~/.local/bin/canopy`로 명시한다.
3. **위키 밖 cwd에서 실행** — cron은 임의 디렉토리에서 돌므로 canopy.toml 상향 탐색이
   실패한다. `~/.config/canopy/config.toml`에 `default_wiki`를 설정해야
   어디서든 동작한다 (아래 시작 절차의 1번).

점검: `which canopy && cd /tmp && canopy status` — 둘 다 성공해야 자동화 준비 완료.

## 시작

```bash
mkdir -p ~/.config/canopy && echo 'default_wiki = "/path/to/wiki"' > ~/.config/canopy/config.toml
canopy init                           # canopy.toml(스키마) 생성, 인덱싱
canopy model pull                     # bge-m3 ONNX ~2.3GB → ~/.local/share/canopy/models (1회)
canopy reindex                        # 최초 전체 임베딩 (이후엔 변경분만)
```

## 명령

**읽기**

| 명령 | 설명 |
|------|------|
| `canopy search "질의" [--mode hybrid\|keyword\|semantic] [-k N]` | 하이브리드 검색 (기본). 임베딩 스택이 없으면 keyword로 강등 |
| `canopy backlinks <page>` / `--orphans` | 역링크 / 고아 페이지 |
| `canopy show <page>` · `canopy status` · `canopy lint` | 조회 · 상태 · 건강 검사 |

**쓰기** — 실행 후 index/log/임베딩 자동 갱신, `NEXT: canopy sync` 안내

| 명령 | 설명 |
|------|------|
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | 검증된 생성 + 관련 페이지 제안 |
| `canopy update <page> [--body-file f]` | updated 갱신(+본문 교체) |
| `canopy mv <page> [--type T] [--slug s]` | 이동/개명 — 인바운드 링크 자동 재작성 |
| `canopy rm <page> [--force]` / `canopy archive <page>` | 삭제(백링크 시 거부) / 아카이브 |
| `canopy sync [-m msg]` | pull --rebase → commit → push → 인덱스 갱신 |

**Second brain** — 후보는 canopy가, 판단·전달은 에이전트가 ([상세](docs/second-brain.md))

| 명령 | 설명 |
|------|------|
| `canopy recall "질문" [-k N] [--per-page N]` | **청크 단위** 근거 + 출처 slug (에이전트 컨텍스트 주입용) |
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | 잊힌 페이지 / 낡은 허브 재발견 후보 |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | 반응 기록 → 이후 pick 조정 |
| `canopy bridge [-n N] [--min-sim 0.7] [--include-linked]` | 유사-미연결 페어; `--include-linked`는 통합/모순 후보(semantic lint) |
| `canopy digest [--since 90d]` | 기간 회고 소재: 생성/갱신 페이지, 태그 분포, decision 시계열 |

**관리**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install`

모든 명령 `--json` 지원. `--peek`(resurface/bridge)은 상태 기록 없이 미리보기.

## 에이전트(hermes) 연동 — 스킬이 설치되는 과정

에이전트용 스킬 2개(`canopy-wiki`: CLI 사용법+콘텐츠 판단 규칙, `canopy-ingest`:
외부 콘텐츠 수집 워크플로우)는 **바이너리에 내장**(go:embed)되어 있고,
설치는 명시적 명령으로 한다:

```bash
canopy skills install              # → ~/.hermes/skills/note-taking/{canopy-wiki,canopy-ingest}/SKILL.md
canopy skills install --dir <path> # 다른 스킬 디렉토리 지정
```

동작 방식과 의미:

- **덮어쓰기가 기본이다** — 이 두 SKILL.md의 진실 소스는 바이너리이므로 설치본을
  손으로 수정하지 말 것 (다음 install이 되돌린다). 스킬을 고치려면 repo의
  `internal/skills/*.md`를 수정하고 다시 빌드·설치한다.
- **canopy 업그레이드 후 재실행** — 새 명령/규칙이 스킬에 반영되는 경로가 이것뿐이다.
- **구스킬 감지** — 과거 프롬프트-체크리스트 방식 스킬(wiki-management 등 6종)이
  스킬 디렉토리에 남아 있으면 목록을 출력한다. 자동 삭제는 하지 않으니 백업 후 제거할 것
  (둘이 공존하면 에이전트가 낡은 워크플로우를 탈 수 있다).
- cron(주간 저널·lint) 등록 레시피는 [docs/second-brain.md](docs/second-brain.md) 참조.

점검: `canopy skills install --dir /tmp/skills-check && head -3 /tmp/skills-check/note-taking/canopy-wiki/SKILL.md`

## 데이터 배치 (XDG Base Directory 준수)

| 위치 | 성격 |
|------|------|
| `<wiki>/canopy.toml` | 위키의 스키마 (타입·태그 taxonomy) — 데이터와 함께 여행하므로 위키에 커밋 |
| `<wiki>/_meta/resurface/` | 재현 불가 상태 (pick 이력·피드백) — 기기 간 동기화 필요, 위키에 커밋 |
| `~/.cache/canopy/index/<해시>.db` | 파생 캐시 (FTS+벡터), 위키별 — `reindex`로 언제든 재구축 |
| `~/.config/canopy/config.toml` | 전역 설정 (`default_wiki`) |
| `~/.local/share/canopy/models/` | ONNX 모델 · `lib/` 빌드용 정적 라이브러리 |

`XDG_CONFIG_HOME` / `XDG_CACHE_HOME` / `XDG_DATA_HOME` 환경변수를 존중한다.
위키 안에는 위 두 항목 외에 canopy 파일이 없다 — gitignore 항목도 필요 없다.

## 성능 (M-series, int8 양자화 모델 기준)

- 모델: bge-m3 **int8 동적 양자화** (585MB; fp32 대비 임베딩 코사인 ≥0.988로 품질 유지)
- 임베딩 ~11ms/청크 (fp32 대비 2.4×) · 모델 로드 <0.5s
- 무변경 reindex ≈ 1초 · keyword 검색 <100ms (모델 로드 없음)
- 양자화 재현: `canopy model pull`(fp32)을 받은 뒤 `scripts/quantize-model.py` 1회 실행
  (onnxruntime 파이썬 venv 필요 — 오프라인 변환이며 런타임엔 파이썬 불필요)
