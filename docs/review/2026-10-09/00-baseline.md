# 감사 기준과 실행 기록

이 검토는 2026-10-09의 `v0.10.0`, 커밋
`ede67e302a8487aecbb4d59587a8686292018cf6`을 대상으로 한다. 아래 기록은 그 시점의
관측이며, 현재 제품의 불변식을 새로 선언하는 문서가 아니다.

사용자의 목표는 자신의 지식을 오래 보존하는 로컬 LLM 위키다. 검토는
`docs/philosophy.md`의 역할 분담과 `docs/invariants.md`의 점검 가능한 보장에서
출발한다. 페이지 원문·출처·재현 불가 상태의 보존, 현재 원문에 근거한 검색,
에이전트가 따라 할 수 있는 작업 절차, 실제 동작과 일치하는 문서, 사람이 작업을
완료할 수 있는 웹 UI를 함께 살핀다.

## 검토 경계

기존 구현을 수정하거나 배포하지 않는다. 운영 중인 웹 서비스와 실제 사용자 위키
`/Users/nobocop/workspace/wiki`도 변경하거나 읽지 않는다. 재현에는 별도 위키와
HOME·XDG 디렉토리를 사용한다. 독립 리뷰는 주장을 수집하는 단계이고, 최종 판정은
반박·재반박 뒤 종합 보고서에 남긴다. 독립 리뷰 파일의 최초 심각도를 그대로
최종 결론으로 읽어서는 안 된다.

```mermaid
flowchart LR
  A["원래 목표와 명세"] --> B["영역별 독립 리뷰"]
  B --> C["코드·격리된 재현 증거"]
  C --> D["독립적인 반박"]
  D --> E["재반박과 범위 조정"]
  E --> F["최종 판정과 개선 계획"]
```

## 기준선 점검

모든 Go 명령은 `GOCACHE=/private/tmp/canopy-review/go-cache GOPROXY=off`로 실행했다.

| 점검 | 관측 | 이 점검으로 증명되지 않는 것 |
|---|---|---|
| `go test ./internal/...` | 통과 | CLI 전체 작업, 다중 프로세스 경합, 중간 실패 복구 |
| `go test -race -cover ./internal/...` | 통과 | 실행하지 않은 경로의 race, 프로세스 간 파일 갱신 충돌, ORT 태그 구현 |
| `make test-ui` | 18개 통과 | 실제 브라우저의 렌더링·접근성·360px 레이아웃 |
| `make i18n-check` | 4개 미러가 소스 해시와 일치 | 번역 의미의 정확성, 소스 내용의 사실성 |
| `gofmt -l .` | 빈 출력 | 구현·설계의 정확성 |
| `CGO_ENABLED=0 go build -o /private/tmp/canopy-audit-20261009/canopy-lite ./cmd/canopy` | 성공 | ORT 빌드·추론, Homebrew 설치, 새 릴리스 |
| 격리된 위키에서 `init → new → search --mode hybrid → show → status` | 전부 exit 0; 검색 결과의 `mode`는 `keyword`, stderr에 강등 이유 | 검색의 의미적 품질, 큰 위키의 성능, 동시 변경 |
| 실제 브라우저 검증 시도 | 테스트 서버 기동 성공; 브라우저 연결 불가 | U5의 화면 폭, 실제 DOM·렌더링·접근성 전부 미검증 |

lite 빌드 중 Go의 기존 모듈 캐시에 버전 정보 임시 파일을 쓰려는 시도가 sandbox에서
거부되었다. 빌드는 exit 0으로 완료되었고, 생성한 바이너리의 격리된 실행도 성공했다.
이 경고를 제품의 실행 실패로 집계하지 않는다.

기준선 Go 테스트 전체가 HOME·XDG 쓰기를 격리했다고 이 기록만으로 보장하지 않는다.
기존 테스트의 일부 helper는 위키만 임시로 만들고 이벤트 DB의 기본 경로를 남겨 둔다.
격리된 HOME에서 그 부수효과를 확인한 독립 관측은
[운영 검토 OPS-06](06-ops.md)에 있다. SDK telemetry 등 도구가 실제 HOME에 자동으로
기록했는지도 전수 확인하지 않았다. 사용자의 상태를 정리하거나 되돌리는 작업은
수행하지 않았다. 개별 canopy 재현 명령의 HOME·XDG는 임시 경로였다.

기준선 이후 추가한 실제 ORT 빌드·추론과 lite 크로스 컴파일의 결과·정확한 실행
범위도 [운영 검토](06-ops.md)에 있다. 아래 coverage 수치나 lite 검사와 별개의
관측이며, 공식 임베딩 계약이나 검색 품질 평가를 대신하지 않는다.

레이스 검사에 포함한 기존 테스트에서 `embed`, `gitops`, `indexer`, `writeops`,
`buildinfo`의 statement coverage는 0%였다. `search`의 100%는 해당 패키지의 융합
함수 테스트에 한정되며, 검색 시스템 전체를 뜻하지 않는다. `store`는 41.4%,
`webui`는 61.1%였다. 수치 자체로 결함을 판정하지 않고, 빠진 실패 경로의 실제
위험과 함께 검토한다.

lite 실행의 원시 결과는 [evidence/baseline.json](evidence/baseline.json)에 보관한다.
재현 당시 임시 디렉토리 경로가 포함되어 있다. 해당 디렉토리는 제품 데이터가 아니다.

브라우저 검증에는 별도 HOME·XDG와 시험 페이지가 있는 테스트 위키를 만들고
`127.0.0.1:18737`로 lite 서버를 실행했다. Orca의 새 탭 생성은
`Browser tabs are unavailable while a remote runtime is active`로 거부되었다.
Computer Use의 in-app browser도 `Browser is not available: iab`였고,
`cua.listBrowsers()` 결과는 `[]`였다. 환경 설정을 바꾸거나 브라우저를 설치하지
않았으며 테스트 서버는 SIGINT로 종료해 exit 0을 확인했다. 이는 화면 검증의 한계이며
제품의 렌더링 실패가 관측된 것은 아니다.

종합 단계에서도 Computer Use 안내를 다시 읽고 `cua.listBrowsers()`를 재조회했으며
결과는 여전히 `[]`였다. 실제 화면 검증의 미실행 상태는 최종 보고서에도 유지한다.
