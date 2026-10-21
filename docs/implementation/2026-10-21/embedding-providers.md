# 외부 임베딩 제공자 검증 — 2026-10-21

이번 변경은 선택적 OpenAI 호환 임베딩 API와 모델별 프로파일을 추가한다.
기본 내장 BGE-M3 경로를 유지하고, 외부 제공자를 선택하면 네이티브 모델을
확인하거나 로딩하지 않는다. EmbeddingGemma 2는 외부 API 경로로 지원한다.
설정과 운영 계약은 [제공자 설계](../../embedding-providers.md), 점검은
[invariants W](../../invariants.md#w-선택-가능한-임베딩-제공자)에 기록했다.

## 검토

제공자 선택이 CLI 검색·recall·reindex·쓰기 후 갱신·reconcile·serve에 연결되는지,
query/document 구분이 CLI와 웹에서 같은 경로를 거치는지 검토했다.
프로파일은 토큰을 제외하고, nil engine 검사도 같은 원격 프로파일을 사용한다.
외부 HTTP 응답은 개수·index·차원·수치·영벡터를 검증하며 오류에 서버 응답 전문을
넣지 않는다. 전송 취소·timeout·redirect 거부를 테스트했다.
별도 검토자의 승인을 받았다는 의미는 아니며, 작성 에이전트의 자체 검토다.

## 실행 결과

| 점검 | 결과 |
|---|---|
| `make test` | internal 및 CLI 전체 통과 |
| `make test-ui` | 37개 통과 |
| `make test-cli` | 37개 검사, lite hybrid 강등·실패 보존 통과 |
| `make test-assets` | native 다운로드 실패 fixture 6개 및 자산·고지 검사 통과 |
| `make i18n-check` | 번역 4개 최신 |
| `go vet ./...`, `gofmt -l .`, `git diff --check` | 통과·포맷 출력 없음 |
| `go test -tags ORT ./internal/embed ./cmd/canopy -run 'TestRemote\|TestExternalEmbeddingCommands'` | 외부 제공자는 ORT 빌드에서도 로컬 모델 없이 정상 |
| `ruby -c packaging/homebrew/canopy.rb` | Syntax OK |

Go/CLI 테스트는 저장소의 격리 실행기를 사용하고, HTTP fixture에는 로컬 포트
권한을 허용했다. 네이티브 빌드는 설치된 ONNX Runtime·libtokenizers를 사용했다.
릴리스 아카이브 내용 검증과 실제 Homebrew 업그레이드는 아직 수행하지 않았다.

## 실제 서비스 확인

사용자가 설정한 외부 서비스로 임시 문서 2개를 인덱싱하고, 한국어 의미 검색 2건이
맞는 문서를 반환함을 확인했다. 이후 사용자에게 실제 위키 전송 승인을 받고
289페이지를 새 모델로 재임베딩했다.

- 실제 `reindex`: 48.54초, 289페이지 갱신, 2,116청크, `needs_reindex = 0`.
- 위키의 `.git`을 제외한 전체 491개 파일의 전후 SHA-256 비교: 모두 동일.
- 실제 위키에서 문서 제목 질의 3건: 기대 문서가 모두 semantic 상위 5개에 포함.
  이것은 연동 smoke test이며 일반 질의 검색 품질 벤치마크는 아니다.
- `recall --no-mark`: 근거 청크 3개 반환.
- 재실행한 `reindex`: 갱신 0페이지, 기존 벡터 재사용.
- 임시 ORT 웹 서버: `/`, `/browse`, `/special/discover`, `/api/search` 모두 HTTP 200.
  검색 API는 `mode = hybrid`, 결과 5개, `needs_reindex = 0`.
- 해당 테스트 서버 RSS: 75.2 MiB. 특정 요청 후의 단일 관측이며 최대 사용량 보장은 아니다.

테스트 서버는 종료했다. 기존 Homebrew 서비스는 사용자가 내린 상태를 유지한다.
토큰·외부 서비스 URL·위키 본문은 이 보고서에 기록하지 않는다.
