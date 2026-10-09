# v0.11.0 릴리스와 기존 서비스 갱신

2026-10-10, 릴리스 소스는 `c32685aa20d94f8dc8757ed1187d52b9bc66528e`다.
[GitHub Release](https://github.com/neutrospec/canopy/releases/tag/v0.11.0)와
[release / publish-tap Actions](https://github.com/neutrospec/canopy/actions/runs/37978808709)가
실제로 성공했다. Homebrew tap은 `00e4fd86b636dffd39ffe37d19a3fd4c87f1b533`로
자동 갱신됐다. 생성된 tap 포뮬러를 직접 편집하지 않았다.

| 검사 | 실제 결과 | 증거 |
|---|---|---|
| 고정 소스 | 전체 Go·race, CLI 37개, Node 37개, 자산·번역 4쌍·vet·포맷·설정 통과; before/after 해시 일치 | [최종 검사](reports/final-checks.json), [릴리스 소스 연결](reports/release-source-binding.json) |
| 공식 릴리스 절차 | 깨끗한 tree에서 `make release-check` → `make release-tag V=0.11.0`; main을 먼저 push한 뒤 태그 push | [release-check](reports/delivery-release-check.txt), [tag](reports/delivery-release-tag.txt) |
| GitHub 실행 | Ubuntu에서 태그 소스 재검사, Release와 tap 게시 모두 success | [Actions 상태](reports/delivery-actions-status.json) |
| 실제 배포 아카이브 | darwin/linux × amd64/arm64 네 파일의 SHA-256이 GitHub digest와 checksums.txt에 일치; 실제 고지·manifest 검사 통과 | [배포물 검사](reports/published-archive-checks.json) |
| 배포 바이너리 | 내려받은 darwin/arm64 lite는 `0.11.0`, commit `c32685a`, cache schema 3, semantic false; CLI 37개와 F2 강등 통과 | [CLI](reports/delivery-published-cli.txt) |
| Homebrew | 0.10.0 → 0.11.0 소스 빌드·설치, formula test 통과; pkgshare의 고지·manifest 바이트가 소스와 일치 | [설치](reports/delivery-brew-upgrade.txt), [test](reports/delivery-brew-test.txt), [고지](reports/installed-notices.json) |
| 설치된 full binary | 실제 read-only 모델로 임시 위키 두 페이지 색인, hybrid 검색, 유효한 원문 byte 범위·profile의 semantic recall 통과; fixture 제거 | [native 실행](reports/installed-native.json), [inert harness](reports/installed-native.py.txt) |
| 기존 서비스 | `brew services restart neutrospec/tap/canopy`; PID 893 → 92180; `/browse` HTTP 200; 앱 자산 3개의 소스 SHA·URL version·ETag/304 일치 | [이전](reports/service-before.json), [이후](reports/service-after.json), [재시작](reports/delivery-service-restart.txt) |

웹 주소는 **http://localhost:8737** 그대로다. 설치 바이너리는
`/opt/homebrew/Cellar/canopy/0.11.0/bin/canopy`, SHA-256은
`3b551d017ea7226ece2bb72b251bbd84c1564c45480046916dd0be4489fec9d6`다.
실행 중 서비스가 보내는 CSS·JS 바이트도 릴리스 소스와 일치했다. 이전 버전의 자동
정리는 하지 않았다. 실제 위키를 fixture로 사용하지 않았고, 서비스 확인은 `/browse`와
정적 자산의 읽기로 제한했다. 실제 위키의 본문·제목·페이지 목록은 증거에 저장하지 않았다.

실제 native 확인은 HOME/XDG를 바이너리 시작 전에 격리하고, macOS sandbox에서
모델·모듈 쓰기와 사용자 위키 읽기를 금지했다. native 검사 harness의 첫 실행은 TOML
들여쓰기 가정으로, 둘째는 CLI `hits`를 HTTP의 `results`로 읽는 해석 실수로 중단됐다.
둘째 실행에서도 실제 색인과 검색은 exit 0이었다. 초기 두 기록을 보존한 뒤 고친
harness로 전체 과정을 새 fixture에서 통과했다. 이를 제품 실패나 최초 통과로 바꾸지 않는다.

초기 `release-check` 두 번은 outer HOME/XDG를 바꿔 정상 Git ignore가 사라지면서
runtime 디렉토리를 untracked로 본 guard에서 중단됐다. 정상 Git 설정을 유지하고
테스트 wrapper 내부 격리와 OS 차단을 사용한 최종 실행은 통과했다. 실행 기록과
초기 중단 로그는 [배포 증거 목록](reports/delivery.json)에 남긴다.

네 종류의 lite를 빌드·검사한 사실은 네 native 플랫폼을 실행한 뜻이 아니다.
현재 Mac의 실제 native 실행과 Ubuntu의 Go/CLI 실행을 각각 기록했다. 실제 브라우저
paint/focus/IME/U5/offline, 공식 CLS parity, 다른 native 플랫폼과 대형 양자화는
여전히 미검증이다. 이번 배포 기록은 앞선 독립 보고의 당시 미검증 상태를 소급해
바꾸지 않는다.
