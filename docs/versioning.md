# 버전 관리와 마이그레이션

> 이 문서의 규칙(philosophy 원칙 8을 따른다): 마이그레이션에 관한 모든 주장은
> [invariants.md](invariants.md)의 **J 섹션** 점검 명령으로 확인된다. 점검할 수
> 없는 규칙은 여기 적지 않는다.

canopy에는 **세 개의 버전 번호**가 있고, 서로 다른 것을 센다. 셋을 뭉뚱그리면
"버전 올렸는데 왜 마이그레이션이 안 도나" 같은 혼란이 생기므로 먼저 구분한다.

| 번호 | 무엇을 세나 | 어디 있나 | 언제 오르나 |
|---|---|---|---|
| **앱 버전** (semver, `v0.1.0`) | 이 바이너리 릴리스 | `internal/buildinfo` (ldflags로 각인) | 사람이 릴리스할 때 |
| **데이터 스키마 버전** (정수, `1`) | 재현 불가 온디스크 상태를 어디까지 이행했나 | `$XDG_CONFIG_HOME/canopy/state.json` | 사다리에 rung을 추가할 때 |
| **캐시 스키마 버전** (문자열, `"2"`) | 파생 인덱스 DB의 레이아웃 | `internal/store.SchemaVersion` + DB의 `meta` | 캐시 구조를 바꿀 때 |

`canopy version`이 셋을 모두 보여준다 (`--json`이면 `version`/`schema_version`/`cache_schema`).

## 원칙: 재현 불가한 것만 마이그레이션한다. 파생물은 버린다.

이 프로젝트의 핵심 구분(philosophy 원칙 3)이 여기서 방법론이 된다.

- **재현 불가 상태** — 전역 설정(`config.toml`, `webauth.json`), 모델·XDG 디렉토리
  레이아웃, 위키 안의 `_meta/*` 형식. 마크다운으로부터 **다시 만들 수 없다.**
  → **마이그레이션 사다리**로 단계적으로 이행한다.
- **파생 캐시** — 검색 인덱스 DB(`$XDG_CACHE_HOME/canopy/index/<해시>.db`). 언제든
  `canopy reindex`로 재구축된다. → **절대 마이그레이션하지 않는다.** 레이아웃이
  바뀌면 `store.SchemaVersion`을 올리고, 불일치 시 코드가 전체를 drop 후 재생성한다.

> 던져버릴 것을 이행하는 것은 낭비다. 그래서 두 관심사를 코드에서 분리한다
> (`internal/migrate` vs `internal/store`). 캐시가 통째로 사라져도 손실이 없다는 것은
> invariants **C4**가 이미 보증한다 — 마이그레이션은 그 보증이 없는 상태에만 쓴다.

## 마이그레이션 사다리 (`internal/migrate`)

재현 불가 상태의 진화는 **append-only 정수 사다리**다. rung 1, 2, 3, … 각 rung은
"버전 N-1의 상태를 버전 N으로 올리는" 한 단계다.

```
현재 기록된 rung ──▶ [rung k+1] ──▶ [rung k+2] ──▶ … ──▶ [Target = rung 개수]
```

### 어떻게 도는가

1. 이행 위치는 `$XDG_CONFIG_HOME/canopy/state.json`에 **기계-로컬**로 기록된다.
   위키가 아니라 설정 디렉토리에 있으므로 캐시를 지워도, 바이너리를 갈아끼워도 살아남는다. **[코드]**
2. 모든 명령이 시작할 때 `migrate.Ensure()`가 돈다 (`version`/`migrate`/`help`는
   예외 — 스스로를 점검·복구하는 명령이라 자동 이행에서 뺀다). 기록된 rung에서
   `Target()`까지 **차례대로** 각 단계를 실행하고, **한 단계 끝날 때마다** 버전을
   각인한다. 그래서 중간에 실패해도 마지막 성공 rung에서 재개된다. **[코드]**
3. `Target()`은 **등록된 마이그레이션 개수**다. 새 바이너리를 깔면 사다리가 길어지고,
   첫 실행이 그 간극을 밟아 올라간다 — 이것이 "새 버전 첫 실행 = 마이그레이션 먼저"다.

- 점검: invariants **J1**(세 버전 노출), **J2**(사다리 조밀·append-only),
  **J3**(첫 실행 각인 + 재실행 no-op).

### 첫 실행의 기준선 판정

`state.json`이 아직 없을 때 시작 rung을 정해야 한다:

- **완전 신규 설치** (canopy 흔적이 디스크에 전혀 없음) → 곧장 `Target()`에서 시작.
  올릴 옛 상태가 없으므로 어떤 마이그레이션도 돌리지 않는다.
- **버전 관리 이전의 기존 설치** (설정/모델/캐시나 레거시 `~/.canopy`가 이미 있음)
  → rung 0에서 시작해 **사다리 전체를 재생한다.** 모든 마이그레이션이 idempotent하고
  이미 충족된 상태에선 no-op이므로, 전체 재생은 안전하고 옛 레이아웃을 빠짐없이 끌어올린다.

### 다운그레이드 가드

`state.json`의 스키마 버전이 바이너리의 `Target()`보다 크면(= 더 새 canopy가 이미
데이터를 앞으로 이행함) 옛 바이너리는 **거부하고 종료한다.** 옛 로직을 새 데이터에
적용해 깨뜨리는 대신, "canopy를 업그레이드하라"고 말한다. **[코드]**

- 점검: invariants **J4**.

## 개발 규칙: 언제, 어떻게 rung을 추가하나

> 이 규칙은 [AGENTS.md](../AGENTS.md)에도 요약되어 개발 중 실수를 막는다.

**언제 필요한가** — 재현 불가 온디스크 상태를 옛 데이터가 만족하지 못하는 방식으로
바꿀 때. 예: 설정 필드를 이름 바꾸거나 비자명한 기본값과 함께 추가, 디렉토리 이동,
`_meta/*` 형식 변경.

**필요 없는 경우** — 인덱스 DB 레이아웃 변경. 그건 `store.SchemaVersion`을 올리고
캐시가 재구축되게 둔다. 사다리에 넣지 않는다.

**어떻게** ([invariants.md](invariants.md) 감사 절차의 "새 기능" 규칙 그대로):

1. `internal/migrate/migrations.go`의 `ladder` **끝에** `Migration{To: N, …}`을 추가한다.
   이미 배포된 rung은 **절대 수정·삭제·재배열하지 않는다.** 그것은 역사적 사실이다.
   잘못은 새 rung으로 바로잡는다.
2. `Run`은 **idempotent**하게, 이미 충족된 상태에선 **no-op**으로 짠다 (기존 설치가
   사다리를 재생하므로). 목적지가 이미 있으면 원본을 지우지 말고 그대로 둔다
   (`migrate001`의 `relocate`가 본보기 — 덮어쓰지 않는다).
3. 구체적 동작만 `ctx.Log`로 알린다. 아무 일도 안 하면 조용해야 한다(시작 시 매번 도는데
   위양성 로그를 남기면 안 된다).
4. `internal/migrate/migrate_test.go`에 **테스트를 먼저** 추가한다 (임시 HOME에서
   기존 상태를 심고 → `Ensure` → 이행 확인 → 재실행 no-op 확인).
5. 필요하면 invariants **J 섹션**에 점검 항목을 늘린다.

## 릴리스 절차

> 처음이라면 단계별 실습 가이드부터: [homebrew-guide.md](homebrew-guide.md) — 개념·토큰
> 설정·릴리스·로컬 시험·문제 해결을 초심자용으로 풀어 놓았다. 아래는 그 요약이다.

버전 정책은 [semver](https://semver.org): `0.y.z` 동안은 y가 기능, z가 수정. 1.0 전에는
파괴적 변경도 y로 낸다. 태그는 Go 모듈 규약대로 **`v` 접두사**(`v0.1.0`)를 쓴다.

태그를 push하면 GitHub Actions(`​.github/workflows/release.yml`)가 GitHub Release +
변경로그 + `*_lite` 아카이브를 만들고, 탭 포뮬러의 url·sha256을 자동 갱신한다(최초 1회
`HOMEBREW_TAP_TOKEN` 시크릿 필요 — 가이드 §4). 손으로 할 때의 절차는 다음과 같다.

```bash
make release-check                 # 트리 clean·테스트·gofmt 게이트 (F1/F3)
make release-tag V=0.1.0           # v0.1.0 태그 + push
goreleaser release --clean         # GitHub 릴리스 + 변경로그 + lite 아카이브
scripts/brew-sha256.sh v0.1.0      # 소스 tarball의 url+sha256 출력
#   → packaging/homebrew/canopy.rb 의 url/sha256에 붙여넣고
#   → neutrospec/homebrew-tap 의 Formula/canopy.rb 로 push
```

버전을 올릴 때 코드에 손대지 않는다 — `buildinfo`의 값은 `git describe` 기반으로
`make`가 각인한다(`make build` 하나로 태그 빌드는 `v0.1.0`, 태그 사이는
`v0.1.0-3-gabc1234`, dirty면 `-dirty`).

## Homebrew 배포

전체 빌드(시맨틱 검색)는 ONNX Runtime과 네이티브 tokenizer 라이브러리를 빌드 시점에
요구한다. 이건 프리빌트 병으로 깔끔히 크로스컴파일되지 않으므로 — **소스 빌드
포뮬러**로 배포한다. 사용자의 머신에서 `brew`가 컴파일하며, 그래서 자연히 brew의
onnxruntime과 링크된다.

- **탭**: `neutrospec/homebrew-tap` (사용자: `brew install neutrospec/tap/canopy`).
  포뮬러 원본은 이 저장소의 [`packaging/homebrew/canopy.rb`](../packaging/homebrew/canopy.rb);
  릴리스 때 탭 저장소로 복사한다.
- **HEAD 설치**: 태그·sha256이 아직 없어도 `brew install --HEAD neutrospec/tap/canopy`로
  `main`을 빌드해 테스트할 수 있다 (초기 테스터용).
- **런타임 onnxruntime 탐색**: `$CANOPY_ONNXRUNTIME_DIR` → `$HOMEBREW_PREFIX/lib` →
  `/opt/homebrew/lib`·`/usr/local/lib`·`/usr/lib` 순. 표준 brew(Apple Silicon/Intel)는
  자동으로 찾고, Linuxbrew·비표준 prefix는 포뮬러 caveats가 환경변수를 안내한다.
- **모델**: 설치 후 `canopy model pull`(1회 ~2.3GB)이 필요하다 — 포뮬러 caveats에 명시.

GoReleaser가 만드는 `*_lite` 아카이브는 **keyword 검색만** 되는 의존성 없는 바이너리다
(cgo 없음). 시맨틱 검색을 원하면 Homebrew 소스 빌드나 `make build`를 쓴다.

## 커밋 규칙

- **[Conventional Commits](https://www.conventionalcommits.org)**: `feat:`, `fix:`,
  `docs:`, `refactor:`, `test:`, `chore:` … GoReleaser 변경로그가 이 접두사로 그룹을 나눈다.
- **AI 공저자 트레일러를 넣지 않는다.** AI 협업 사실은 [README](../README.md)의
  "개발 방식에 대하여"에 공개되어 있고, 각 기능의 결정 경로는 `docs/`의 설계 기록으로
  추적된다. 그 공개로 귀속은 충분하므로 커밋 메시지는 깨끗하게 유지한다. (이전 커밋에는
  트레일러가 남아 있다 — 역사는 다시 쓰지 않는다.)
