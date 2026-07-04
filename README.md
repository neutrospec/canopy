# canopy

LLM 위키(Karpathy-style markdown knowledge base)를 second brain으로 운영하기 위한 단일 Go CLI.

한 문장 요약: **판단은 LLM이, 불변식은 코드가.** 에이전트(hermes)가 산문 체크리스트를
외워 지키던 규칙 — 스키마 검증, index 갱신, JSONL 로그, 임베딩 동기화, git sync — 를
canopy가 코드로 강제하고, 위키가 축적한 지식을 다시 꺼내 보여주는 루프(resurface/bridge)까지
제공한다. 사람은 Obsidian으로 그대로 열람하고, 에이전트는 `--json`으로 조작한다.

## 문서 맵

| 문서 | 내용 | 언제 읽나 |
|------|------|-----------|
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

## 시작

```bash
canopy init --wiki ~/workspace/wiki   # canopy.toml 생성, .canopy/ 준비, 인덱싱
canopy model pull                     # bge-m3 ONNX ~2.3GB → ~/.canopy/models (1회)
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
| `canopy resurface [-n N] [--strategy auto\|random\|hub]` | 잊힌 페이지 / 낡은 허브 재발견 후보 |
| `canopy resurface feedback <page> --up\|--down\|--snooze N` | 반응 기록 → 이후 pick 조정 |
| `canopy bridge [-n N] [--min-sim 0.7]` | 유사한데 연결 안 된 페이지 페어 |

**관리**: `canopy reindex [--no-embed]` · `canopy model pull/status` · `canopy skills install`

모든 명령 `--json` 지원. `--peek`(resurface/bridge)은 상태 기록 없이 미리보기.

## 데이터 배치

| 위치 | 성격 |
|------|------|
| `<wiki>/canopy.toml` | 스키마 진실 소스 (타입·태그 taxonomy) — 커밋 |
| `<wiki>/_meta/resurface/` | 재현 불가 상태 (pick 이력·피드백) — 커밋, 기기 간 동기화 |
| `<wiki>/.canopy/index.db` | 파생 캐시 (FTS+벡터) — gitignore, `reindex`로 언제든 재구축 |
| `~/.canopy/models/bge-m3/` | ONNX 모델 · `~/.canopy/lib/` 빌드용 정적 라이브러리 |

## 성능 (M-series 기준)

- 임베딩 21ms/청크 — 전체 재구축 819청크 ≈ 7분(최초 1회), 무변경 reindex ≈ 1초
- 모델 로드 cold ~11s / warm ~1s · keyword 검색 <100ms (모델 로드 없음)
