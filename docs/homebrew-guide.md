# Homebrew 배포 가이드 (처음 하는 사람용)

canopy를 `brew install`로 배포하고, 새 버전을 릴리스하는 전 과정을 처음부터
설명합니다. Homebrew를 한 번도 안 다뤄봤어도 이 문서만 따라오면 됩니다.

> 이 문서는 **실행 방법(how)**입니다. 왜 이렇게 설계했는지(why)는
> [versioning.md](versioning.md)에, 개발 규칙은 [../AGENTS.md](../AGENTS.md)에 있습니다.

---

## TL;DR — 새 버전 릴리스 (자주 볼 3줄)

```bash
make release-check            # ① 트리 clean·테스트·gofmt 게이트
git push origin main          # ② 변경사항을 main에 올려두고
make release-tag V=0.2.0      # ③ 태그 v0.2.0 생성+push → 나머지는 자동
```

③ 이후는 GitHub Actions가 알아서 합니다: **GitHub Release**(변경로그 + lite 아카이브)
생성 → **탭 포뮬러**의 버전·해시 갱신 → 테스터는 `brew upgrade canopy`.

단, 자동화의 탭 갱신은 **최초 1회 토큰 설정**([§4](#4-릴리스-자동화-준비-1회-pat-토큰))이
되어 있어야 동작합니다. 안 해두면 릴리스 워크플로우의 마지막 단계가 빨간 X로 실패하고,
친절한 에러 메시지로 알려줍니다(릴리스 자체와 GitHub Release는 정상 생성됩니다).

---

## 1. 5분 개념 정리

세 단어만 알면 됩니다.

- **Homebrew** — macOS·Linux의 패키지 매니저. 사용자는 `brew install <이름>`으로 설치합니다.
- **포뮬러(formula)** — "이 프로그램을 어떻게 받아서·빌드해서·설치하나"를 적은 Ruby
  레시피 파일 하나(`canopy.rb`). 우리 것은 **소스에서 빌드**하는 포뮬러입니다.
- **탭(tap)** — 포뮬러들을 담는 GitHub 저장소. 이름이 `homebrew-<탭이름>`이면
  사용자는 `brew install <소유자>/<탭이름>/<포뮬러>`로 씁니다. 우리는
  `neutrospec/homebrew-tap` → `brew install neutrospec/tap/canopy`.

**왜 소스 빌드인가?** canopy의 시맨틱 검색은 ONNX Runtime과 네이티브 tokenizer
라이브러리에 링크됩니다. 이걸 미리 컴파일한 바이너리로 배포하면 깨지기 쉬워서,
사용자 머신에서 `brew`가 직접 컴파일하도록 했습니다(그래서 `onnxruntime`이 의존성).

### 우리 구조 (그림)

```
canopy 저장소                              homebrew-tap 저장소
─────────────                              ───────────────────
packaging/homebrew/canopy.rb   ── 릴리스 ──▶  Formula/canopy.rb   ── 사용자 ──▶  brew install
   (포뮬러 원본 = 진실의 소스)      워크플로우     (배포되는 포뮬러)
.goreleaser.yaml (릴리스 빌드)                README.md (설치 안내)
.github/workflows/release.yml                .github/workflows/audit.yml
```

핵심: **포뮬러는 canopy 저장소의 `packaging/homebrew/canopy.rb` 한 곳에서만 고칩니다.**
릴리스할 때 워크플로우가 그걸 탭으로 복사하면서 버전(url)과 해시(sha256)만 갈아끼웁니다.
탭의 `Formula/canopy.rb`는 손으로 고치지 마세요(다음 릴리스가 덮어씁니다).

---

## 2. 지금 무엇이 준비돼 있나

이미 되어 있는 것 (v0.1.0은 수동으로 완료했습니다):

- **탭 저장소**: https://github.com/neutrospec/homebrew-tap (public) — 지금도
  `brew install neutrospec/tap/canopy`로 설치됩니다.
- **canopy 쪽 릴리스 도구**: `packaging/homebrew/canopy.rb`(원본 포뮬러),
  `.goreleaser.yaml`, `Makefile`의 `release-check`/`release-tag`,
  `scripts/brew-sha256.sh`, 그리고 워크플로우 `.github/workflows/{ci,release}.yml`.

앞으로 **딱 한 번** 해두면 릴리스가 완전 자동이 되는 것: 아래 §4의 토큰 설정.

---

## 3. 사용자(테스터)는 어떻게 설치하나

전달할 안내는 간단합니다 (탭 README에도 있음):

```bash
brew install neutrospec/tap/canopy      # 안정 버전
canopy model pull                        # 시맨틱 검색 모델 ~2.3GB (선택, 1회)
```

- 최신 개발 버전을 시험하려면: `brew install --HEAD neutrospec/tap/canopy` (main을 빌드)
- 업그레이드: `brew upgrade canopy` · 제거: `brew uninstall canopy && brew untap neutrospec/tap`
- 최초 빌드에 Go·Xcode Command Line Tools가 필요하며 `brew`가 알아서 챙깁니다.
- 웹 UI를 상시 서비스로: `brew services start canopy` (localhost:8737). 소스/`make install`로
  쓰던 사용자가 brew로 옮겨오는 절차는 [upgrading.md](upgrading.md).

---

## 4. 릴리스 자동화 준비 (1회) — PAT 토큰

릴리스 워크플로우는 **canopy 저장소**에서 돌면서 **다른 저장소**(homebrew-tap)에
커밋을 push해야 합니다. 기본 `GITHUB_TOKEN`은 자기 저장소 밖으로 push할 수 없으므로,
탭에 쓸 수 있는 토큰을 하나 만들어 canopy에 **시크릿**으로 넣어줍니다. 딱 한 번입니다.

### 4-1. Fine-grained 토큰 만들기 (최소 권한)

1. https://github.com/settings/tokens?type=beta → **Generate new token**.
2. **Token name**: `canopy-tap-bump` (아무거나).
3. **Resource owner**: `neutrospec`.
4. **Expiration**: 원하는 만료일(예: 90일 — 만료되면 재발급).
5. **Repository access**: *Only select repositories* → **`neutrospec/homebrew-tap`** 하나만.
6. **Permissions** → *Repository permissions* → **Contents: Read and write**.
   (그 외 전부 No access. 이 토큰은 오직 탭 저장소의 파일만 쓸 수 있습니다.)
7. **Generate token** → 화면에 뜨는 `github_pat_...` 값을 복사(다시 못 봅니다).

> 왜 fine-grained? "탭 저장소 콘텐츠 쓰기"만 가능한 최소 권한이라, 혹시 유출돼도
> 피해가 그 저장소에 한정됩니다(최소 권한 원칙).

### 4-2. canopy 저장소에 시크릿으로 넣기

**터미널**(권장 — 값이 화면에 안 남게):

```bash
gh secret set HOMEBREW_TAP_TOKEN --repo neutrospec/canopy
# 프롬프트가 뜨면 복사한 github_pat_... 값을 붙여넣고 Enter
```

또는 **웹 UI**: canopy 저장소 → **Settings** → **Secrets and variables** → **Actions**
→ **New repository secret** → Name `HOMEBREW_TAP_TOKEN`, Secret에 값 붙여넣기 → Add.

확인: `gh secret list --repo neutrospec/canopy` 에 `HOMEBREW_TAP_TOKEN`이 보이면 끝.

---

## 5. 릴리스 하는 법 (매번)

### 5-1. 자동 (권장)

```bash
# 0) 버전 정할 것: 0.x 동안은 기능=가운데 숫자, 수정=끝 숫자 (semver)
make release-check              # 트리 clean·테스트·gofmt 통과해야 진행 (게이트)
git push origin main           # 반영할 커밋을 main에 올려두기
make release-tag V=0.2.0        # 내부에서 release-check 재확인 후 v0.2.0 태그+push
```

`v0.2.0` 태그가 push되면 GitHub Actions의 **release** 워크플로우가 자동으로:

1. **release** 잡 — GoReleaser가 GitHub Release를 만들고, Conventional Commit
   메시지로 변경로그를 생성하고, 의존성 없는 `*_lite` 아카이브(4플랫폼)를 첨부.
2. **publish-tap** 잡 — v0.2.0 소스 tarball의 sha256을 계산해 탭의
   `Formula/canopy.rb`에 url·sha256을 갈아끼우고 커밋·push. 이어서 탭의 **audit**
   워크플로우가 그 포뮬러를 자동 검증.

끝나면 테스터는 `brew upgrade canopy`로 새 버전을 받습니다.

**진행 상황 보기**:

```bash
gh run watch --repo neutrospec/canopy         # 방금 뜬 실행을 실시간으로
gh run list --repo neutrospec/canopy --limit 3
```

또는 GitHub의 canopy 저장소 → **Actions** 탭.

### 5-2. 손으로 (자동화 없이 / 디버깅용)

워크플로우가 막혔거나 토큰 설정 전이라면 이렇게 직접 할 수 있습니다:

```bash
make release-tag V=0.2.0                       # 태그+push (Release는 안 만들어짐)
./scripts/brew-sha256.sh v0.2.0                # url + sha256 두 줄 출력
```

출력된 두 줄을 `packaging/homebrew/canopy.rb`의 `url`/`sha256`(맨 위 2칸 들여쓰기
라인)에 붙여넣고, 그 파일을 탭의 `Formula/canopy.rb`로 복사한 뒤 커밋·push:

```bash
# canopy 저장소에서 포뮬러 갱신 커밋 후…
cp packaging/homebrew/canopy.rb ../homebrew-tap/Formula/canopy.rb   # 탭 클론 경로에 맞게
git -C ../homebrew-tap commit -am "canopy v0.2.0" && git -C ../homebrew-tap push
```

> GoReleaser로 Release만 따로 만들려면: `goreleaser release --clean`
> (로컬에서 실제 발행) 또는 발행 없이 확인만 `goreleaser release --snapshot --clean`.

---

## 6. 로컬에서 시험하는 법 (발행 전에 확인)

무엇이든 push하기 전에 손으로 확인할 수 있습니다.

```bash
# 릴리스 빌드를 발행 없이 로컬에서 (dist/ 에 아카이브 생성)
goreleaser release --snapshot --clean

# 포뮬러 문법·스타일·해시 검증
brew style   packaging/homebrew/canopy.rb          # Ruby 스타일
brew audit --strict --online neutrospec/tap/canopy # url/sha 실다운로드 검증 (탭 설치 후)

# 실제 설치 테스트 → 확인 → 원복
brew install neutrospec/tap/canopy                 # 소스 빌드 (수십 초)
canopy version                                     # 동작 확인
brew uninstall canopy                              # 원복 (원한다면 brew untap neutrospec/tap)

# 아직 태그 안 찍은 main 코드를 설치해 보기
brew install --HEAD neutrospec/tap/canopy
```

CI도 자동으로 돌아갑니다: canopy에 push하면 **ci** 워크플로우(gofmt·vet·test·lite 빌드),
탭에 push하면 **audit** 워크플로우(brew style·audit)가 검증합니다.

---

## 7. 포뮬러를 바꿔야 할 때

의존성 추가, tokenizer 버전 변경 등 포뮬러 내용을 바꿀 일이 생기면:

- **`packaging/homebrew/canopy.rb` (canopy 저장소)만 고칩니다.** 탭 파일은 릴리스 때
  자동 생성되므로 직접 건드리지 않습니다.
- **tokenizer 버전을 바꾸면** 두 곳을 함께: `Makefile`의 `TOKENIZERS_VERSION`과
  포뮬러의 `resource "tokenizers"` url·sha256(플랫폼별). 새 sha256은
  `curl -sL <release-url>/libtokenizers.<platform>.tar.gz | shasum -a 256`로 구합니다.
- 바꾼 뒤 `brew style packaging/homebrew/canopy.rb`로 검증하고, 가능하면
  `brew install --HEAD`로 한 번 빌드해 봅니다.

> ⚠️ 릴리스 워크플로우는 포뮬러의 **맨 위 2칸 들여쓰기** `url`/`sha256`만 갈아끼웁니다.
> `resource` 안의 8칸 들여쓰기 sha256(= tokenizer 해시)은 건드리지 않으니, 그 들여쓰기
> 구조는 유지하세요.

---

## 8. 문제 해결

| 증상 | 원인 / 해결 |
|---|---|
| 릴리스 워크플로우의 **publish-tap** 잡만 빨간 X, `HOMEBREW_TAP_TOKEN … not set` | 토큰 시크릿 미설정/만료 → [§4](#4-릴리스-자동화-준비-1회-pat-토큰) 다시. Release 자체는 이미 생성됨. `make release-tag`를 다시 찍지 말고, 시크릿만 넣은 뒤 Actions에서 그 실행을 **Re-run failed jobs** |
| publish-tap이 `Permission … denied` / 403 | 토큰이 `homebrew-tap`에 대해 **Contents: write**가 아님, 또는 다른 저장소를 가리킴 → 토큰 재발급 |
| 사용자가 `brew install` 시 **`libonnxruntime not found`** | 보통 `onnxruntime`이 의존성으로 자동 설치됨. 특수 환경이면 `export CANOPY_ONNXRUNTIME_DIR="$(brew --prefix onnxruntime)/lib"` |
| 사용자가 **`canopy: command not found`** | `$(brew --prefix)/bin`이 PATH에 없음 |
| `brew audit`가 **sha256 mismatch** | 태그를 강제로 옮겼거나 소스가 바뀜 → 해당 태그의 tarball로 sha256 재계산. 태그는 한 번 push했으면 옮기지 말 것 |
| GoReleaser가 **changelog empty** 등 | 태그 push 시 얕은 클론이면 발생 — 워크플로우는 `fetch-depth: 0`이라 괜찮음. 로컬에선 전체 히스토리 필요 |

사용자 단계의 문제는 탭 [README](https://github.com/neutrospec/homebrew-tap#readme)와
[docs/troubleshooting.md](troubleshooting.md)에도 정리돼 있습니다.

---

## 9. 용어 사전

- **formula** — 프로그램 설치 레시피(Ruby). 우리 것은 `canopy.rb`.
- **tap** — 포뮬러를 담는 GitHub 저장소(`homebrew-<이름>`).
- **bottle** — 미리 컴파일된 바이너리 패키지. 우리는 쓰지 않고 **소스 빌드**함.
- **resource** — 포뮬러가 본체 외에 추가로 받는 파일(우리는 tokenizer 정적 라이브러리).
- **caveats** — 설치 후 사용자에게 보여주는 안내문(우리는 `canopy model pull` 안내).
- **HEAD 설치** — 릴리스가 아니라 저장소의 최신 커밋(main)을 빌드 (`--HEAD`).
- **PAT** — Personal Access Token. 여기선 탭에 push할 최소 권한 토큰.
- **ldflags** — 빌드 시 바이너리에 버전 문자열을 새겨 넣는 링커 옵션.
- **semver** — `MAJOR.MINOR.PATCH`. 1.0 전(0.x)에는 기능=MINOR, 수정=PATCH.

---

## 부록: 파일 지도

| 파일 | 저장소 | 역할 |
|---|---|---|
| `packaging/homebrew/canopy.rb` | canopy | **포뮬러 원본(진실의 소스)** — 고칠 땐 여기만 |
| `.goreleaser.yaml` | canopy | 릴리스 빌드(변경로그 + lite 아카이브) 설정 |
| `Makefile` (`release-check`/`release-tag`) | canopy | 릴리스 게이트 + 태깅 |
| `scripts/brew-sha256.sh` | canopy | 태그 소스 tarball의 url+sha256 출력(수동 fallback) |
| `.github/workflows/release.yml` | canopy | 태그 push → Release + 탭 갱신 (자동) |
| `.github/workflows/ci.yml` | canopy | push/PR마다 gofmt·vet·test·lite 빌드 |
| `Formula/canopy.rb` | homebrew-tap | **배포되는 포뮬러**(자동 생성 — 손대지 말 것) |
| `README.md` | homebrew-tap | 사용자 설치 안내 |
| `.github/workflows/audit.yml` | homebrew-tap | 포뮬러 style·audit 자동 검증 |
