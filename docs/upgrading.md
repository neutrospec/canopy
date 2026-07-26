# 업그레이드 · 이행 가이드

> canopy를 **다른 설치·운영 방식으로 옮길 때**의 단계별 안내다. "버전 변화"에는 세
> 종류가 있고 이 문서는 그중 하나만 다룬다 — 셋을 뭉뚱그리지 말 것:
>
> | 종류 | 누가 하나 | 어디 |
> |---|---|---|
> | **데이터 스키마 이행** | canopy가 자동 (새 버전 첫 실행이 `migrate.Ensure()`로 밟아 올라감) | 설계 [versioning.md](versioning.md), 점검 [invariants.md](invariants.md) J |
> | **설치·운영 이행** | 사람이 수동 (설치 방식·실행 방식 변경) | **이 문서** |
> | 릴리스 변경 로그 | GoReleaser가 자동 생성 | GitHub Releases (커밋 규약 → 자동) |
>
> 각 항목은 실행 가능한 점검 명령을 포함한다([philosophy.md](philosophy.md) 원칙 8).
> 항목은 최신 순으로 쌓는다.

## 먼저: 지금 나는 어떻게 깔려 있나

```bash
which -a canopy                 # 어느 바이너리가 PATH에 잡히는지 (여러 개면 두 벌 설치)
brew list --versions canopy     # brew 설치 여부·버전
canopy version                  # 앱·데이터 스키마·캐시 스키마 버전
```

- `/opt/homebrew/bin/canopy`(Apple Silicon) 또는 `/usr/local/bin/canopy`(Intel)만 나오면 **brew 단일 설치** — 권장 상태.
- `~/.local/bin/canopy`가 함께 나오면 소스/`make install` 사본이 남아 있는 것 — 아래로 정리.

> **핵심 사실 — 바이너리 위치와 XDG 데이터는 별개다.** canopy의 설정·모델·캐시·상태는
> `$XDG_*_HOME`(또는 HOME 기반 기본값)로 정해지므로 바이너리가 어디 있든 같은 곳을 쓴다.
> 그래서 바이너리를 옮기거나 지워도 데이터는 그대로다 — 아래 이행의 점검이 이를 증명한다.

---

## 소스 / `make install` → Homebrew + `brew services` (v0.2.0~, 2026-07)

**대상** — `make install`이나 직접 소스 빌드로 `~/.local/bin/canopy`(또는 `/opt/homebrew/bin`에 손수 복사)를 써 온 사람.

**왜** — v0.2.0부터 [Homebrew 탭](homebrew-guide.md)이 정식 배포 채널이다. brew가
onnxruntime·tokenizer 의존과 업그레이드를 관리하고, `brew services`로 웹 UI를 상시
서비스(launchd)로 돌릴 수 있다. 두 벌 설치는 어느 것이 실행되는지 헷갈리고 버전이
어긋나는 원인이 된다.

**전제** — brew 설치가 되어 있고(`brew install neutrospec/tap/canopy`), 위키가
`~/.config/canopy/config.toml`의 `default_wiki`로 지정되어 있을 것. 서비스는 cwd도
`--wiki`도 없이 돌기 때문에 이 설정이 위키를 찾는 유일한 길이다.

### 단계

```bash
# 1. brew로 설치 (아직 안 했다면)
brew install neutrospec/tap/canopy

# 2. 모델은 이전할 필요 없음 — XDG 데이터라 그대로 재사용된다. 확인만:
canopy model status --json | jq .model_path
#   → ~/.local/share/canopy/models/... (brew 바이너리도 같은 경로를 본다)

# 3. 옛 바이너리 제거
rm ~/.local/bin/canopy
hash -r                          # 셸의 명령 경로 캐시 비우기 (또는 새 셸)

# 4. 이제 bare `canopy`가 brew로 해석되는지 확인
which canopy                     # → /opt/homebrew/bin/canopy

# 5. (선택) 웹 UI를 상시 서비스로
brew services start canopy       # localhost:8737, 로그인 시 자동 시작도 등록
```

### 점검

```bash
# 한 벌만 남았나
which -a canopy | sort -u                 # brew 경로 하나만
test ! -e ~/.local/bin/canopy && echo ok  # 옛 사본 없음

# 데이터 손실 없음 (제거 전후 동일해야 한다 — 바이너리 위치 ≠ XDG)
canopy model status --json | jq .model_path
canopy status --json | jq .pages          # 페이지 수 그대로

# 서비스가 실제로 서빙하나
brew services list | grep canopy          # started
curl -s -o /dev/null -w '%{http_code}\n' localhost:8737/   # 200
```

### 되돌리기

데이터·설정은 XDG에 그대로 있으므로 어느 쪽으로 가든 무손실이다.

```bash
brew services stop canopy         # 서비스 내리기 (필요 시)
# 소스 설치로 복귀하려면 canopy 저장소에서:
make install                      # ~/.local/bin/canopy 재생성
```

### 함정

- **`make install` 재실행은 `~/.local/bin/canopy`를 되살린다.** brew 단일화 후에는 daily
  바이너리를 `brew upgrade canopy && brew services restart canopy`로 올린다. 로컬 개발
  빌드를 시험할 땐 PATH를 더럽히지 말고 저장소에서 `make build` 후 `./canopy …`를 쓰거나,
  main을 brew로 시험하려면 `brew install --HEAD neutrospec/tap/canopy`.
- **최소 PATH 환경(system cron·launchd 등)은 `.zshenv`를 읽지 않는다.** 그런 곳에서
  `canopy`를 부르면 `/opt/homebrew/bin`이 PATH에 없어 실패할 수 있다 — 절대경로를 쓰거나
  그 잡의 PATH에 brew prefix를 명시할 것. (hermes처럼 로그인 환경을 물려받는 에이전트는
  `.zshenv`의 PATH를 타므로 bare `canopy`로 충분하다.)
- **옛 경로를 하드코딩한 자동화**(cron 프롬프트, 스크립트)가 있으면 `~/.local/bin/canopy`를
  bare `canopy`로 바꿔 둔다 — 제거 후 낡은 절대경로는 깨진다.

---

## 앞으로: 정기 업그레이드

brew 단일 설치라면 새 릴리스 반영은 두 줄이다.

```bash
brew upgrade canopy
brew services restart canopy      # 서비스로 웹 UI를 돌리는 경우 (에셋이 바이너리에
                                  # 내장되므로 재시작해야 새 UI가 반영된다)
```

데이터 스키마 이행이 필요한 릴리스라면 업그레이드 후 **첫 `canopy` 실행이 자동으로**
밟아 올라간다(`canopy migrate status`로 확인). 사용자가 손으로 할 일은 없다 —
그것이 자동 이행과 이 문서(수동 이행)의 경계다.
