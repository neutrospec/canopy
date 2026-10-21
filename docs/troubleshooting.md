> **한국어** · [English](en/troubleshooting.md)

# 문제 해결

자주 만나는 문제와 진단 명령 모음입니다.

## `canopy: command not found`

`make install`은 `~/.local/bin`에 우선 설치하고, 디렉토리가 없으면
`/opt/homebrew/bin`으로 폴백합니다. 다음 경우를 확인하세요.

1. **`~/.local/bin`이 PATH에 없음** — 대화형 rc(.zshrc)에만 PATH를 추가하면
   cron·에이전트 같은 non-interactive 셸에서 찾지 못합니다. zsh 기준
   `~/.zshenv`에 넣어야 모든 셸에서 잡힙니다:
   ```bash
   export PATH="$HOME/.local/bin:$PATH"
   ```
2. **자동화 환경은 사용자 rc를 아예 읽지 않을 수 있음** — cron 잡이나 에이전트
   프롬프트에는 절대경로(`~/.local/bin/canopy`)를 쓰는 것이 가장 확실합니다.
위키를 못 찾는 오류는 PATH와 별개입니다. 요청의 경로를 명시해
`canopy status --wiki "/path/to/wiki" --json`의 `root`를 확인하세요. 대상 선택은
`--wiki` → `CANOPY_WIKI` → cwd의 조상 → 전역 `default_wiki` 순입니다.
`init --wiki <path>`는 비어 있는 default만 채웁니다. 여러 위키 자동화에서는
전역 설정을 바꾸기보다 매 명령에 같은 `--wiki`를 유지하세요.

점검: `which canopy`와 자동화와 같은 환경에서의 `canopy status --wiki <path> --json`.
실제 자동화의 PATH·HOME·XDG 환경이 대화형 셸과 다를 수 있습니다.

## 시맨틱 검색이 아무것도 찾지 못함

`canopy model status`가 진단해 줍니다. 내장 제공자의 체크 순서:

1. `canopy model pull`로 모델을 받았는지
2. 이 바이너리가 ORT 빌드인지 (`make build-lite`에는 내장 추론이 없음)
3. `libonnxruntime`이 설치되어 있는지 (`brew install onnxruntime`)
4. `canopy reindex`로 임베딩이 만들어졌는지

외부 제공자는 ORT·모델 다운로드가 필요 없습니다. 머신 로컬 `config.toml`의
`[embedding]`에서 URL·토큰·서버 모델명·차원을 확인하고, `canopy reindex --check --json`의
`needs_reindex`가 0보다 크면 `canopy reindex`를 실행하세요. 같은 이름으로 서버 모델을
교체했다면 `revision`도 바꿉니다. HTTP 오류는 상태 코드로 보고되며 내장 모델로
자동 전환하지 않습니다. [설정 방법](getting-started.md#4-5-검색).

모델 없이 읽으려면 `search --mode keyword`로 찾고 `show`로 본문을 확인하세요.
hybrid search는 keyword로 강등되지만 `recall`은 임베딩 없이 실패합니다. `canopy.toml`의
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
  Caddy/nginx 리버스 프록시 뒤에 두세요. 프록시가 원래 Host를 전달한다면
  `canopy serve --allow-host wiki.example`처럼 그 authority를 명시적으로 허용합니다.
  비표준 포트가 전달되면 `wiki.example:8443`처럼 포트까지 맞추세요.
  `--allow-host`는 Host 허용만 설정하며 인증이나 TLS를 활성화하지 않습니다.
  localhost backend를 공개 프록시로 노출한다면 프록시에서 인증을 제공하거나,
  canopy의 외부 바인딩 인증을 사용하는 backend를 구성해야 합니다.
  forwarded header는 Host 허용이나 인증 판단의 근거로 사용하지 않습니다.

- **포트가 이미 사용 중** — 해당 포트를 점유한 PID와 명령을 확인하고 자신이
  시작한 대상 프로세스만 종료하거나 `--addr localhost:8738`처럼 다른 포트를 사용하세요.
- **편집을 저장했는데 본문이 그대로** — 웹 저장은 edit 제안 접수입니다. `할 일` 화면
  또는 `canopy tasks list --json --wiki <path>`에서 요청을 확인하세요. 에이전트가
  처리해야 페이지에 반영되며 자동 실행·처리 시간 보장은 없습니다. `base`는 접수 시점
  파일 해시입니다. 폼을 열 당시 본문의 보존본이나 409 충돌 검사를 뜻하지 않습니다.
- **검색이 keyword로만 동작** — serve 시작 로그에 임베딩 스택 상태가 출력됩니다.
  위의 "시맨틱 검색" 항목과 원인이 같습니다.

## 그래프·수식·다이어그램이 안 보임

- 그래프/수식/mermaid는 브라우저 JS로 렌더됩니다 — JS를 끈 브라우저에서는
  본문 텍스트만 보입니다(읽기·검색·편집은 JS 없이도 동작).
- 모든 에셋은 바이너리에 내장되어 있어 인터넷 연결과 무관합니다. 업그레이드
  후 이상하면 브라우저 캐시를 강력 새로고침(⌘⇧R) 하세요.
- 원문의 raw HTML은 기본적으로 렌더하지 않습니다. 생성된 위키링크·수식·Mermaid는
  지원 경로로 표시됩니다. `lint`의 `invalid-mermaid`는 문법 오류,
  `mermaid-unchecked`는 환경 때문에 미검증이라는 뜻입니다.

## 쓰기 실패·사본 복구

본문 파일은 실제 내용 확인까지 유지하고, checkin 전에 편집 사본을 위키 밖 별도
파일로 백업하세요. checkin은 사본을 회수하므로 실패 뒤에도 남는다고 가정하면 안 됩니다.
명령이 실패해도 페이지가 이미 바뀌었을 수 있습니다. 원본·백업과 현재 `show` 출력,
stderr·적용 상태를 비교하고 미완료 단계를 확인한 뒤 복구하세요. 같은 쓰기를 바로
반복하지 마세요. `show`의 exit 0은 내용이 요청에 맞다는 보증이 아닙니다.
변경 응답의 `applied`·`stage`·`repair`·`sync`와 보존된 checkout의 복구 절차는
[mutation-results.md](mutation-results.md)를 따릅니다.

JSON 모드와 `show`에는 상태 배너가 없습니다. 대상·dirty·열린 사본·대기 작업은
`canopy status --json --wiki <path>`로 확인합니다. sync는 기존 dirty 변경까지 전부
commit/push하며 push가 실패하면 local commit이 남을 수 있습니다.

## 설치된 에이전트 스킬이 낡음

`canopy skills install`을 다시 실행하세요. 설치본을 손으로 수정했다면 다음
install이 덮어씁니다 — 스킬의 진실 소스는 바이너리(`internal/skills/*.md`)입니다.
임의 디렉토리에 설치했다면 같은 `--dir <path>`를 다시 지정하세요. 설치는 자동 감지
등록이나 에이전트의 활성화를 보장하지 않습니다. 해당 에이전트의 로드 규칙을 확인하세요.
