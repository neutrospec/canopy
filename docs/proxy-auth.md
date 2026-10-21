# 개인 Gateway 뒤에서 Canopy 사용하기

Canopy v0.12.0부터 서버 간 Bearer 인증을 지원한다. WebOS 같은 개인 Gateway가
로그인을 담당하고, Canopy로 보내는 **모든 요청**(HTML·정적 자원·검색 API·폼)에
`Authorization: Bearer <token>`을 붙인다. 브라우저에는 이 토큰을 전달하지 않는다.
앱별 공개 호스트의 `/`에 연결한다. `/apps/canopy/` 같은 경로 접두사는 지원하지 않는다.

```mermaid
flowchart LR
    Browser["브라우저 · WebOS 내부 창"] --> Gateway["Gateway · 로그인 확인"]
    Gateway -->|"서버에서 Bearer 헤더 추가"| Canopy["Canopy · Host / Origin / 인증 검사"]
```

## 토큰 파일 만들기

위키와 Git 저장소 밖에서 실행한다. 아래 명령은 새 파일에만 쓰며 토큰을 출력하지 않는다.
`XDG_CONFIG_HOME`을 사용하는 경우 해당 디렉토리로 경로를 바꾼다.

```bash
mkdir -p "$HOME/.config/canopy"
(umask 077; set -C; openssl rand -hex 32 > "$HOME/.config/canopy/webos-token")
chmod 600 "$HOME/.config/canopy/webos-token"
```

파일은 64자리 hex 토큰 하나이며 마지막 LF 또는 CRLF만 선택적으로 허용한다.
파일의 절대경로·일반 파일 여부·0600 권한·형식을 시작 시 검사한다.
누락·손상·권한 오류가 있으면 서비스 시작이 실패하며 무인증으로 강등하지 않는다.
난수성은 생성 도구가 책임진다. 토큰 값은 CLI 인자·URL·채팅에 넣지 않는다.
Gateway Secret에는 파일 내용을 안전한 관리 경로로 등록한다.

## Homebrew 서비스에 영구 설정

`~/.config/canopy/config.toml`(또는 `$XDG_CONFIG_HOME/canopy/config.toml`)의
기존 `default_wiki` 등은 유지하고 아래 `[serve]`를 추가한다. `/Users/YOUR_NAME`은
실제 홈 절대경로로 바꾼다. TOML 안의 `~`는 확장하지 않는다.

```toml
[serve]
addr = "192.168.1.22:8737"
auth_token_file = "/Users/YOUR_NAME/.config/canopy/webos-token"
# public_origin = "https://YOUR_APP_HOST"
# allow_hosts = ["YOUR_APP_HOST"]
```

앱 등록 후 `public_origin`에 실제 공개 HTTPS origin을 넣고 주석을 해제한다.
경로·끝 `/`·쿼리·와일드카드는 허용하지 않는다. 미설정 상태에서는 공개 origin을
가진 변경 요청이 거절될 수 있으므로 앱을 사용하기 전에 설정한다.

Gateway가 upstream Host를 `192.168.1.22:8737`로 바꾸면 `allow_hosts`는 불필요하다.
공개 앱 Host를 유지한다면 `allow_hosts`에 **실제로 보내는 Host 값**을 넣는다
(포트를 보내면 포트까지). `public_origin` 설정이 Host 허용 목록을 자동 확장하지 않는다.
`X-Forwarded-*`로 이 검사를 우회할 수 없다.

```bash
brew services restart canopy
```

서비스는 기존처럼 `canopy serve`를 실행하고 이 전역 파일을 매 시작마다 읽는다.
설정은 Cellar와 생성된 LaunchAgent 밖에 있어 업그레이드·서비스 재시작에 유지된다.
LaunchAgent를 수동 편집하거나 토큰 값을 환경변수에 넣을 필요가 없다.
CLI를 사용할 때는 다음처럼 같은 옵션을 직접 지정할 수 있다.

```bash
canopy serve --addr 192.168.1.22:8737 \
  --auth-token-file "$HOME/.config/canopy/webos-token" \
  --public-origin https://YOUR_APP_HOST
```

우선순위는 명시한 CLI 옵션 > 전역 `[serve]` > 기본값이다. 기본 주소는 계속
`localhost:8737`이다. 토큰 교체는 0600 새 파일을 원자적으로 교체하고 Gateway
Secret 갱신 및 서비스 재시작으로 적용한다. 운영 중인 토큰은 채팅·로그로 전달하지 않는다.
Gateway–Canopy 구간은 TLS 또는 암호화된 터널로 보호한다.

## 인증과 프록시 계약

- 토큰을 설정하면 loopback도 인증한다. 계정이 없어도 유효한 토큰으로 바로 사용하며
  계정·setup 코드·세션 쿠키를 생성하지 않는다. 토큰 전용 운영의 `/setup`은 비활성이다.
- 기존 계정은 그대로 로그인할 수 있다. Authorization이 없으면 세션을 확인한다.
  잘못된 Authorization(중복 포함)은 유효한 세션이 있어도 401이다.
- 토큰과 계정이 모두 없는 기존 loopback 동작 및 LAN 계정 설정 절차는 유지한다.
  토큰 전용 모드에서 자격증명 없는 콘텐츠 요청은 401이다. 로그인 화면에 필요한
  정적 자원은 기존처럼 공개하며 위키 콘텐츠는 포함하지 않는다.
- 변경 요청의 Origin은 요청 자체의 scheme·Host가 같거나 설정한 public origin과
  정확히 같아야 한다. Origin 없는 서버 요청은 기존처럼 허용한다. 다른 Origin은
  올바른 토큰으로도 403이고, 잘못된 Host 역시 403이다.
- 토큰 권한은 기존 웹 사용자와 같다. 수정은 제안 태스크만 만들며 위키 원문을 직접
  수정하지 않는다. 토큰 접속에서는 Canopy 로그아웃 버튼을 숨긴다. 로그아웃은 Gateway에서 한다.
- 기존 보안 헤더를 유지한다. iframe을 막는 헤더를 추가하지 않으며 내부 주소로의
  절대 리다이렉트를 만들지 않는다. 실제 WebOS 내부 창은 배포한 Gateway에서 검증한다.

## 점검

```bash
go test ./internal/webui -run 'TestProxyAuth|TestProxyOrigin|TestTokenFile'
go test ./internal/config -run TestServeConfig
go test ./cmd/canopy -run TestServe
```

실제 배포에서는 `brew upgrade canopy`와 `brew services restart canopy` 뒤 같은
설정으로 접속되는지, WebOS 내부 창의 페이지·검색·그래프·수정 제안이 동작하는지 확인한다.
이 운영 점검은 Go 테스트가 대신하지 않는다.
