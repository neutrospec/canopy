# 문제 해결

자주 만나는 문제와 진단 명령 모음입니다.

## `canopy: command not found`

`make install`은 `~/.local/bin`에 우선 설치하고, 디렉토리가 없으면
`/opt/homebrew/bin`으로 폴백합니다. 다음 세 경우를 확인하세요.

1. **`~/.local/bin`이 PATH에 없음** — 대화형 rc(.zshrc)에만 PATH를 추가하면
   cron·에이전트 같은 non-interactive 셸에서 찾지 못합니다. zsh 기준
   `~/.zshenv`에 넣어야 모든 셸에서 잡힙니다:
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```
2. **자동화 환경은 사용자 rc를 아예 읽지 않을 수 있음** — cron 잡이나 에이전트
   프롬프트에는 절대경로(`~/.local/bin/canopy`)를 쓰는 것이 가장 확실합니다.
3. **위키 밖 디렉토리에서 실행** — canopy는 cwd에서 위로 올라가며 `canopy.toml`을
   찾습니다. cron처럼 임의 디렉토리에서 도는 환경에서는 탐색이 실패하므로
   `~/.config/canopy/config.toml`에 `default_wiki`를 설정하세요.

점검: `which canopy && cd /tmp && canopy status` — 둘 다 성공하면 자동화 준비 완료.

## 시맨틱 검색이 아무것도 찾지 못함

`canopy model status`가 진단해 줍니다. 체크 순서:

1. `canopy model pull`로 모델을 받았는지
2. 이 바이너리가 ORT 빌드인지 (`make build-lite` 빌드는 keyword 전용)
3. `libonnxruntime`이 설치되어 있는지 (`brew install onnxruntime`)
4. `canopy reindex`로 임베딩이 만들어졌는지

급할 때는 `--mode keyword`가 항상 동작합니다. `canopy.toml`의
`embedding.model`은 모델 디렉토리 이름과 일치해야 합니다
(`~/.local/share/canopy/models/` 아래, 기본 `bge-m3`).

## 웹 UI(serve) 관련

- **외부 접근 시 로그인 요구** — 정상 동작입니다. localhost 바인딩(기본)은
  무인증이지만, `--addr :8737`처럼 외부에서 접근 가능한 바인딩은 인증이
  필수입니다. 최초 1회 serve 터미널에 출력되는 설정 코드로 `/setup`에서
  계정을 만드세요.
- **계정 재설정** — `~/.config/canopy/webauth.json`을 삭제하고 serve를
  재시작하면 새 설정 코드로 다시 부트스트랩합니다.
- **HTTPS** — canopy는 TLS를 직접 제공하지 않습니다. tailscale(권장) 또는
  Caddy/nginx 리버스 프록시 뒤에 두세요.

- **포트가 이미 사용 중** — 이전 serve 프로세스가 남아 있을 수 있습니다:
  `pkill -f "canopy serve"` 후 재시작.
- **편집 저장 시 409(충돌)** — 폼을 연 뒤 다른 곳(CLI·에이전트·다른 탭)에서 같은
  페이지가 수정된 경우입니다. 화면에 남은 편집본을 현재 버전과 비교해 반영한 뒤
  다시 저장하세요.
- **검색이 keyword로만 동작** — serve 시작 로그에 임베딩 스택 상태가 출력됩니다.
  위의 "시맨틱 검색" 항목과 원인이 같습니다.

## 설치된 에이전트 스킬이 낡음

`canopy skills install`을 다시 실행하세요. 설치본을 손으로 수정했다면 다음
install이 덮어씁니다 — 스킬의 진실 소스는 바이너리(`internal/skills/*.md`)입니다.
