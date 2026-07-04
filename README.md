# canopy

LLM 위키(Karpathy-style markdown knowledge base)를 관리하는 단일 Go CLI.

에이전트(hermes)가 프롬프트 체크리스트로 지키던 규칙 — 스키마 검증, index 갱신,
JSONL 로그, 임베딩 동기화, git push — 를 **코드로 강제**한다. 사람은 Obsidian으로
그대로 열람하고, 에이전트는 CLI(`--json`)로 조작한다.

## 설계 원칙

- **index.md, index/*.md, logs/*.jsonl, 임베딩은 전부 canopy가 생성하는 파생물.**
  손으로 만지지 않는다. 수치는 항상 파일시스템 실측 → "+1 드리프트" 원천 차단.
- **검색은 하이브리드**: SQLite FTS5(BM25) + bge-m3 벡터(코사인), RRF 융합.
- **임베딩은 완전 자체 내장**: hugot + ONNX Runtime이 프로세스 안에서 bge-m3 실행.
  외부 임베딩 서버·API 없음. 모델은 `canopy model pull`로 1회 다운로드.
- **커밋은 자동이 아니다**: 여러 쓰기를 묶어 `canopy sync` 한 번으로 pull→commit→push.
  잊어버림은 모든 명령 시작 시 미동기 배너 + 쓰기 끝의 `NEXT: canopy sync` 안내가 막는다.
  단건 작업은 쓰기 명령에 `--sync`를 붙이면 즉시 처리.

## 빌드

```bash
brew install onnxruntime   # libonnxruntime.dylib
make build                 # -tags ORT; libtokenizers.a 자동 다운로드
# 임베딩 없는 경량 빌드: make build-lite (keyword 검색만)
```

## 시작

```bash
canopy init --wiki ~/workspace/wiki   # canopy.toml 생성, .canopy/ 준비(gitignore), 인덱싱
canopy model pull                     # bge-m3 ONNX ~2.3GB → ~/.canopy/models
canopy reindex                        # 최초 전체 임베딩 (이후엔 변경분만)
```

## 명령

| 명령 | 설명 |
|------|------|
| `canopy status` | 페이지 수, git dirty/ahead, 초기화 여부 |
| `canopy search "질의" [--mode hybrid\|keyword\|semantic] [-k N]` | 하이브리드 검색 (기본) |
| `canopy backlinks <page>` / `--orphans` | 역링크 / 고아 페이지 |
| `canopy show <page>` | 페이지 출력 |
| `canopy lint` | 스키마·링크·신선도 검사 |
| `canopy new <title> --type T --tags a,b [--slug s] [--body-file f\|-] [--links p1,p2]` | 검증된 페이지 생성 + 관련 페이지 제안 |
| `canopy update <page> [--body-file f]` | updated 갱신(+본문 교체), 재인덱싱, 로그 |
| `canopy mv <page> [--type T] [--slug s]` | 카테고리 이동/개명 (인바운드 링크 자동 재작성) |
| `canopy rm <page> [--force]` | 삭제 (백링크 있으면 거부) |
| `canopy archive <page>` | `_archive/`로 이동, 인바운드 링크 평문화 |
| `canopy sync [-m msg]` | pull --rebase → commit → push → 인덱스 갱신 |
| `canopy reindex [--no-embed]` | 파생 인덱스 재구축 |
| `canopy model pull/status` | 임베딩 모델 관리 |

모든 명령은 `--json`을 지원한다 (에이전트/cron 연동).

## 데이터 배치

| 위치 | 내용 |
|------|------|
| `<wiki>/canopy.toml` | 스키마 설정(타입, 태그 taxonomy, 디렉토리) — 커밋 대상, 진실 소스 |
| `<wiki>/.canopy/index.db` | 파생 캐시(FTS + 벡터). gitignore, 삭제해도 `reindex`로 복구 |
| `~/.canopy/models/bge-m3/` | ONNX 모델 (EmbeddedLLM/bge-m3-onnx-o2-cpu) |
| `~/.canopy/lib/libtokenizers.a` | 빌드용 정적 토크나이저 (daulet/tokenizers 릴리스) |

## 성능 (M2 Pro 기준)

- 임베딩: 21ms/청크, 전체 재구축 819청크 ≈ 7분(최초 1회), 증분 no-op ≈ 1초
- 모델 로드: cold ~11s / warm ~1s (fp32 2.3GB; int8 양자화 시 단축 여지)
- 키워드 검색: <100ms (모델 로드 없음)
