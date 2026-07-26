# AGENTS.md — canopy 개발 규칙

canopy는 마크다운 LLM 위키를 관리하는 단일 Go 바이너리다. 이 문서는 이 저장소에서
**코드를 고치는 에이전트·사람**을 위한 규칙이다. 위키를 *사용하는* 법이 아니라
canopy 자체를 *개발하는* 법을 다룬다.

## 황금률: 문서와 불변식이 코드보다 먼저다

이 프로젝트의 철학은 "판단은 LLM이, 불변식은 코드가"이며(→ [docs/philosophy.md](docs/philosophy.md)),
그 개발 규율은 하나다: **새 기능은 (1) 불변식을 정하고 (2) 점검 방법을
[docs/invariants.md](docs/invariants.md)에 먼저 적은 뒤 (3) 구현한다.** 점검 명령이
없는 주장은 문서에 넣지 않는다. 목록이 길어지는 건 건강하고, 점검 안 되는 규칙이
느는 게 병이다.

## 빌드 · 테스트 · 포맷

```bash
make build        # 전체(시맨틱 검색, -tags ORT). onnxruntime + libtokenizers.a 필요
make build-lite   # keyword 전용, cgo 없음 (우아한 강등 경로)
make test         # go test ./internal/...
make fmt          # gofmt -w .
```

커밋·릴리스 전에 반드시 초록: 테스트 통과(F1), `gofmt -l .` 빈 출력(F3),
lite 빌드로도 `search --mode hybrid`가 keyword로 강등되며 exit 0(F2).
전체 감사 절차는 [docs/invariants.md](docs/invariants.md).

## 버전과 마이그레이션 (실수가 잦은 지점 — 반드시 읽을 것)

세 버전 번호가 서로 다른 것을 센다. 뭉뚱그리지 말 것. 전체 설계는
[docs/versioning.md](docs/versioning.md).

| 번호 | 무엇 | 어디 |
|---|---|---|
| 앱 버전 (`v0.1.0`, semver) | 릴리스 | `internal/buildinfo` (ldflags 각인) |
| 데이터 스키마 (정수) | 재현 불가 상태를 어디까지 이행했나 | `$XDG_CONFIG_HOME/canopy/state.json` |
| 캐시 스키마 (`"2"`) | 파생 인덱스 DB 레이아웃 | `internal/store.SchemaVersion` |

### 규칙 1 — 재현 불가 **머신-로컬** 상태를 바꾸면 사다리에 rung을 추가한다

전역 설정 형식, 모델·XDG 디렉토리 레이아웃, `$XDG_STATE_HOME`의 이벤트 DB처럼
**이 머신에 있고 마크다운에서 다시 만들 수 없는** 상태를 옛 데이터가 만족 못 하게
바꾼다면:

1. `internal/migrate/migrations.go`의 `ladder` **끝에** `Migration{To: N, …}` 추가.
   **이미 배포된 rung은 절대 수정·삭제·재배열하지 않는다** — 역사적 사실이다.
   잘못은 새 rung으로 바로잡는다.
2. `Run`은 **idempotent + 이미 충족 시 no-op**. 기존 설치가 사다리 전체를 재생하므로.
   목적지가 이미 있으면 원본을 덮어쓰지 말 것 (`migrate001`의 `relocate`가 본보기).
3. `internal/migrate/migrate_test.go`에 **테스트 먼저**: 임시 HOME에 옛 상태 심기 →
   `Ensure` → 이행 확인 → 재실행 no-op 확인.
4. 필요하면 [docs/invariants.md](docs/invariants.md) **J 섹션**에 점검 항목 추가.

startup에서 `migrate.Ensure()`가 자동으로 도므로, 새 버전 첫 실행이 간극을 밟아
올라간다. 손으로 확인: `canopy migrate status`.

### 규칙 1b — 위키와 함께 여행하는 파일(`_meta/*`)은 사다리가 아니라 self-version

사다리의 이행 기록은 **머신-로컬**(`state.json`)인데 위키 파일은 git으로 기기 간
여행한다 — 이행을 마친 머신이 옛 형식의 위키를 clone하면 사다리는 다시 돌지 않는다.
그래서 `_meta/*` 파일 형식은 **파일 안의 `version` 필드**로 진화한다: Load가 옛
버전을 관용적으로 읽고, 형식 변경은 additive하게(모르는 필드를 깨뜨리지 않게).
구버전 canopy가 다시 저장하며 새 필드를 떨어뜨릴 위험이 있으면 **별도 파일**로
분리한다 (`agent-reads.json`이 본보기 — [docs/web-ui-plan-4.md](docs/web-ui-plan-4.md)).

### 규칙 2 — 인덱스 DB를 바꾸면 사다리가 아니라 캐시 버전을 올린다

검색 인덱스 DB는 **파생 캐시**다(`canopy reindex`로 언제든 재구축, 불변식 C4).
레이아웃을 바꿀 땐 `internal/store.SchemaVersion`을 올린다 — 불일치 시 코드가
drop 후 재생성한다. **사다리에 넣지 마라.** 던져버릴 것을 이행하는 건 낭비다.

## 커밋 규칙

- **Conventional Commits**: `feat: …`, `fix: …`, `docs: …`, `refactor:`, `test:`,
  `chore:`. GoReleaser 변경로그가 이 접두사로 그룹을 나눈다.
- **AI 공저자 트레일러(`Co-Authored-By`, `Claude-Session`)를 넣지 않는다.** AI 협업은
  [README](README.md) "개발 방식에 대하여"에 공개되어 있고 결정 경로는 `docs/` 설계
  기록으로 추적되므로, 그 공개로 귀속은 충분하다. 커밋 메시지는 깨끗하게 둔다.

## 릴리스

`make release-check` → `make release-tag V=x.y.z`(태그 push). 그러면 GitHub Actions
(`.github/workflows/release.yml`)가 GitHub Release·변경로그·lite 아카이브 생성 + Homebrew
탭 포뮬러 자동 갱신. 포뮬러를 고칠 땐 **`packaging/homebrew/canopy.rb`(원본)만** — 탭
파일은 릴리스가 덮어쓴다. 초심자 실습 가이드: [docs/homebrew-guide.md](docs/homebrew-guide.md),
설계: [docs/versioning.md](docs/versioning.md).
