# 불변식 목록 (Invariants)

> 시스템이 건강하다 = 아래 목록이 전부 통과한다.
> 각 항목은 **실행 가능한 점검 명령**을 갖는다. 명령이 없는 불변식은 등재 불가
> ([philosophy.md](philosophy.md) 원칙 8).
>
> `W=--wiki <path>` 를 환경에 맞게 지정. 전체 감사는 아래를 위에서부터 실행하면 된다.

## A. 스키마 (쓰기 시점에 코드가 강제)

| # | 불변식 | 점검 |
|---|--------|------|
| A1 | 페이지는 `entities/ concepts/ comparisons/` 안에만 존재 | `canopy lint $W --json` → `counts["stray-root"] == 0` |
| A2 | 파일명은 영문 lowercase-hyphen | `canopy lint $W --json` → `counts["bad-filename"] == 0` |
| A3 | 모든 페이지에 frontmatter(title/created/updated/type/tags) | `counts["no-frontmatter"] == 0 && counts["frontmatter-fields"] == 0` |
| A4 | type은 canopy.toml 열거형만 | `counts["invalid-type"] == 0` |
| A5 | tag는 canopy.toml taxonomy만 | `counts["invalid-tag"] == 0` |
| A6 | 거부 동작 자체의 확인 | `canopy new t --type guide $W` 가 **에러로 종료** (exit != 0) |
| A7 | 유효 taxonomy는 CLI로 조회 가능하고, 검증과 같은 소스를 쓴다 | `canopy tags --json $W \| jq '.tags\|length'` ≥ 1, 그리고 taxonomy 밖 태그의 거부 메시지에 `canopy tags` 안내 포함 |

## B. 연결성 (lint가 검출)

| # | 불변식 | 점검 |
|---|--------|------|
| B1 | 깨진 wikilink 없음 | `counts["broken-link"] == 0` |
| B2 | 새 페이지의 `--links`는 실존 페이지만 | `canopy new t --type concept --links no-such $W` 가 에러 |
| B3 | 개명 시 인바운드 링크가 따라감 | `canopy mv` 후 `counts["broken-link"]` 증가 없음 |
| B4 | 삭제는 백링크 있으면 거부 | 백링크 있는 페이지에 `canopy rm` → 에러 + 목록 출력 |
| B5 | 모든 페이지는 본토(최대 연결 성분)에서 위키링크로 도달 가능 — 섬 클러스터는 고아 검사를 통과하므로 별도 검출 (philosophy 원칙 13) | `canopy lint --json $W \| jq '.counts.island // 0'` == 0 (또는 각 섬이 의도된 것으로 확인됨). finding에 섬 구성원 나열; 웹 점검 페이지가 다리(섬↔본토 최고 유사도 페어) 제안 |

## C. 파생물 정합성 (생성 시점에 실측)

| # | 불변식 | 점검 |
|---|--------|------|
| C1 | index.md Total == 실제 파일 수 | `canopy status --json $W \| jq .pages` == index.md의 `Total pages` 숫자 |
| C2 | 카테고리 인덱스는 전량 나열 | `grep -c '^\- \[\[' <wiki>/index/concepts.md` == `ls <wiki>/concepts/*.md \| wc -l` |
| C3 | 쓰기마다 JSONL 로그 1건 이상 | 쓰기 직후 `tail -1 <wiki>/logs/$(date +%Y-%m).jsonl` 의 timestamp가 방금 것 |
| C4 | 검색 인덱스는 완전 재구축 가능 | `rm -rf ~/.cache/canopy && canopy reindex $W` 성공 후 `canopy search "test" $W` 동작 |
| C5 | 임베딩은 변경분만 갱신 | 무변경 상태에서 `canopy reindex $W` → `embedded_pages == 0`, 수 초 내 종료 |
| C6 | 페이지 열람은 CLI로 전수 가능하며 실측과 일치 | `canopy list --json $W \| jq .count` == `canopy status --json $W \| jq .pages` |

## D. git 동기화

| # | 불변식 | 점검 |
|---|--------|------|
| D1 | 미동기 상태는 배너로 노출 | 파일 touch 후 `canopy status $W` → ⚠ 배너 |
| D2 | sync는 pull이 선행 | `canopy sync $W` 출력/로그에 pull 단계 확인 |
| D3 | sync 후 클린 | `canopy sync $W && canopy status $W` → "✓ fully synced" |
| D4 | 위키 안에 canopy 캐시 없음 (캐시는 `$XDG_CACHE_HOME/canopy`) | `test ! -e <wiki>/.canopy` && `ls ~/.cache/canopy/index/*.db` 존재 |

## E. Second-brain 루프 (resurface/bridge)

| # | 불변식 | 점검 |
|---|--------|------|
| E1 | resurface 풀은 30일+ 미접촉 페이지만 | `canopy resurface -n 5 --peek --json $W \| jq '.picks[].days_stale'` → 전부 ≥ 30 |
| E2 | bridge는 미연결 페어만 | `canopy bridge --peek --json $W` 결과 페어에 상호 wikilink 없음 (`canopy backlinks <a>` 로 확인) |
| E3 | 같은 페이지 45일 내 재노출 없음 | pick 후(--peek 없이) 즉시 재실행 → 같은 slug 안 나옴 |
| E4 | 피드백/스누즈 반영 | `canopy resurface feedback <slug> --snooze 7` 후 해당 slug 미출현 |
| E5 | 상태는 git 추적 (기기 간 동기화) | pick 후 `git -C <wiki> status --short _meta/resurface` 에 나타나고 sync로 커밋됨 |
| E6 | --peek은 흔적을 남기지 않음 | `--peek` 실행 전후 `_meta/resurface/state.json` 해시 동일 |

## F. 빌드/테스트

| # | 불변식 | 점검 |
|---|--------|------|
| F1 | 전체 테스트 통과 | `make test` |
| F2 | 임베딩 없는 환경에서도 동작(우아한 강등) | `make build-lite` 바이너리로 `search --mode hybrid` → keyword로 강등 + 경고, exit 0 |
| F3 | 포맷 준수 | `gofmt -l .` 출력 없음 |
| F4 | 어떤 cwd에서도 동작 (자동화 전제) | `which canopy && cd /tmp && canopy status` 성공 (PATH + default_wiki 구성 검증) |
| F5 | 스킬 설치는 멱등·재현 가능 | `canopy skills install --dir /tmp/sc` 2회 실행 → 동일 내용, exit 0 |

## G. 에이전트 메모리 / 회고 (recall · digest · semantic 후보)

| # | 불변식 | 점검 |
|---|--------|------|
| G1 | recall 결과의 모든 출처는 실존 페이지 | `canopy recall "질문" --json $W \| jq -r '.chunks[].slug'` 의 각 slug가 `canopy show <slug>` 성공 |
| G2 | recall은 청크 원문을 그대로 반환 (요약·변형 없음) | 반환된 text가 해당 페이지 본문에 부분 문자열로 존재 (청크 앞의 제목 프리픽스 제외) |
| G3 | recall 결과는 score 내림차순 | `.chunks[].score` 가 단조 감소 |
| G4 | digest 범위 필터 정확 | `canopy digest --since 30d --json $W \| jq -r '.updated_pages[].updated'` 전부 30일 이내 |
| G5 | digest 수치는 실측 | `.stats.created` == `.created_pages \| length` (내부 일관성), created는 frontmatter 기준 |
| G6 | bridge는 기본적으로 미연결 페어만, --include-linked 시 linked 필드로 구분 | 플래그 없이 → 전부 `linked == false`; 플래그 있이 → linked true/false 혼재 가능하되 필드 존재 |
| G7 | new의 관련 페이지 제안은 임계값 이상 + 태그 일치 우선 | (임시 위키에서) `canopy new … --json \| jq '.related[].score'` 전부 ≥ 0.8, 최대 5건; 태그가 겹치는 페이지가 앞에 온다 |
| G8 | digest 소비 통계는 이벤트 로그 실측 | `canopy digest --since 30d --json $W \| jq '.top_consumed[]?.events'` 전부 ≥ 1, 각 `.slug`는 실존 페이지 (`canopy show <slug>` 성공) |

## H. 웹 UI 상태 (_meta/webui)

| # | 불변식 | 점검 |
|---|--------|------|
| H1 | 읽기 이력은 위키에 커밋되는 상태다 (파생 캐시 아님) | 읽음 표시 후 `git -C $W status --short` 에 `_meta/webui/reads.json` 등장, `canopy sync` 후 clean |
| H2 | reads의 source는 explicit\|auto 뿐이고 explicit은 auto로 강등되지 않는다 | `jq -r '.reads[].source' $W/_meta/webui/reads.json \| sort -u` ⊆ {explicit, auto}; 읽음 페이지에서 auto 감지가 다시 와도 source 유지 |
| H3 | mv는 읽기 이력을 함께 이관한다 | 읽음 표시 → `canopy mv <page> --slug new-name` → reads.json에 새 slug만 존재 |
| H4 | agent 접근은 human 읽음 등급을 오염시키지 않는다 (unread 배지 유지) | 미읽음 페이지에 `canopy show <slug>` → `jq '.reads' $W/_meta/webui/reads.json`에 해당 slug 없음(또는 기존 source 불변), `_meta/attention/agent-reads.json`에만 기록. 웹에서 여전히 unread |
| H5 | 검색 노출은 읽음이 아니다 | `canopy search "질의"` 실행 전후 `_meta/webui/reads.json`·`_meta/attention/agent-reads.json` 해시 동일 |
| H6 | resurface는 모든 문(웹·에이전트)의 최근 접근을 존중한다 | 오늘 `canopy show <slug>`(또는 웹 읽음) → `canopy resurface -n 20 --peek --json $W`의 picks에 그 slug 없음 |
| H7 | agent 접근의 위키 기록은 일 단위 양자화 (읽기마다 커밋 소음 없음) | 같은 날 `canopy show <slug>` 2회 → 2회차 전후 `agent-reads.json` 해시 동일 (이벤트 DB에만 2건) |
| H8 | 이벤트 DB는 위키 밖 머신-로컬, 위키 안에는 집계 JSON만 | `git -C $W ls-files _meta/attention/` → `agent-reads.json`뿐(*.db 없음) && `ls $XDG_STATE_HOME/canopy/attention/*.db` (기본 ~/.local/state/canopy) 존재 |
| H9 | 기록 페이지(/history)는 이벤트 로그 실측 | `canopy show <slug>` 후 `curl -s localhost:8737/history` 응답에 그 slug 존재 (serve 실행 상태) |
| H10 | 검색 갭은 문과 무관하게 한 파일에 쌓인다 | CLI `canopy search "없는단어xyz" $W` → `tail -1 $W/_meta/webui/search-gaps.jsonl`의 `query` 일치·`door == "agent"`; 웹 검색 갭은 같은 파일에 `door == "web"` |

## I. 웹 UI 쓰기·보안 (serve 실행 상태에서 점검)

| # | 불변식 | 점검 |
|---|--------|------|
| I1 | 공개 바인딩은 무인증으로 어떤 페이지도 서빙하지 않는다 (philosophy 원칙 11) | `canopy serve --addr :8737` 후 무인증 `curl -s -o /dev/null -w '%{http_code}' http://<LAN-IP>:8737/page/anything` → 302 (로그인/설정으로 리다이렉트, 본문 없음) |
| I2 | 웹 편집은 CLI `update`와 같은 파이프라인이다 (philosophy 원칙 9) | 스크래치 위키에서 웹 편집 1회 vs `canopy update` 1회 → 파일(updated 갱신·본문 교체), logs 엔트리 형태, `index/*.md` 재생성, keyword 검색 반영이 모두 동일 |
| I3 | 계정 등록은 터미널의 설정 코드 없이는 불가 | 공개 바인딩·무계정 상태에서 코드 없이 `POST /setup` → 400 |
| I4 | 교차 출처 변조 거부 | `curl -X POST -H "Origin: https://evil.example" http://localhost:8737/logout` → 403 |

## J. 버전·마이그레이션 ([versioning.md](versioning.md))

| # | 불변식 | 점검 |
|---|--------|------|
| J1 | 세 버전 번호(앱·데이터 스키마·캐시 스키마)가 노출된다 | `canopy version --json` → `version`(semver 문자열) && `schema_version`/`schema_target`(정수) && `cache_schema` 필드 존재 |
| J2 | 사다리는 조밀하고(rung i → 버전 i+1) append-only | `canopy migrate status --json \| jq '.target'` == `internal/migrate/migrations.go`의 `ladder` 항목 수; `go test ./internal/migrate/`의 `TestLadderIsDense` 통과 |
| J3 | 첫 실행이 버전을 각인하고, 재실행은 no-op이다 (idempotent) | 빈 `XDG_CONFIG_HOME`에서 아무 명령 → `state.json` 생성; 이어서 `canopy migrate` → "up to date". `go test`의 `TestEnsureFreshInstall`·`TestLegacyRelocation`(2회차 무변경) |
| J4 | 데이터가 바이너리보다 새로우면 거부한다 (다운그레이드 가드) | `state.json`의 `schema_version`을 `schema_target`보다 크게 써두고 아무 명령 실행 → 에러로 종료. `go test`의 `TestDowngradeGuard` |
| J5 | 파생 캐시(인덱스 DB)는 마이그레이션 대상이 아니라 재구축 대상이다 (C4 재확인) | `migrate`가 `store`에 의존하지 않음: `grep -rn '"github.com/neutrospec/canopy/internal/store"' internal/migrate/` → 빈 출력. 캐시 손실 복구는 C4가 보증 |
| J6 | 마이그레이션은 순서대로 적용되고 idempotent하다 | `go test ./internal/migrate/` 전부 통과 (F1에 포함) |
| J7 | 모든 릴리스 태그가 가리키는 커밋은 `origin/main`의 조상이다(자기 자신 포함) — 태그만 push되고 브랜치가 뒤처지거나(v0.4.1이 그랬다), force-push로 릴리스 커밋이 유실되는 걸 잡는다 | `make release-lineage-check` (`scripts/release-lineage-check.sh`); `ci.yml`이 매 push마다(fetch-depth 0 필수 — shallow면 태그 0개로 공허 통과), `release.yml`이 릴리스마다 GoReleaser 전에 실행. 위반 예방은 `Makefile`의 `release-tag` 3중 가드 — `main에서만` + 로컬 main이 `origin/main` **포함**(뒤처진 clone 차단; 옛 커밋도 조상이라 J7만으론 못 잡음) + `git push origin main`을 태그보다 **먼저**. 위반 복구는 versioning.md "릴리스는 main 위에서만" |

## K. 정규화(reconcile) 게이트 ([reconcile-design.md](reconcile-design.md))

> "정규화는 한 문으로": 판단을 거친 내용의 해시가 원장(`_meta/reconcile/state.json`)에
> 실리고 — 파이프라인 쓰기는 자동으로, 뒷길 변경은 검토(`bless`) 후 수동으로 — 원장과
> 다른 페이지 내용이 곧 미정규화 외부 변경이다. 예방이 아니라 [검출]+사후 정규화.
> 게이트는 opt-in: `bless --all`로 기준선을 잡기 전에는 침묵한다.

| # | 불변식 | 점검 |
|---|--------|------|
| K1 | 원장은 위키에 커밋되는 상태다 (기기 간 동기화) | `bless` 후 `git -C $W status --short`에 `_meta/reconcile/state.json` 등장, `canopy sync` 후 clean |
| K2 | 뒷길 변경은 후보로 검출되고, 파이프라인 쓰기는 후보가 아니다 (자동 축복) | 페이지 파일 직접 편집 → `canopy reconcile --json $W \| jq '.foreign[].rel_path'`에 등장; 이어서 그 페이지를 `canopy update`로 고치면 foreign에서 사라짐 |
| K3 | 후보는 결정론적이다 (원칙 6) | 같은 상태에서 `canopy reconcile --json` 반복 → 동일 출력; `.foreign[].dup_candidates[].similarity` 전부 ≥ 0.8 |
| K4 | bless는 현재 내용(부재 포함)을 원장에 기록해, 검토한 변경은 다시 뜨지 않는다 | `canopy reconcile bless <path>` 후 foreign에서 그 path 사라짐; 파일을 지운 뒤 bless → `deleted` 후보도 사라짐 |
| K5 | 보고(기본 실행)는 흔적을 남기지 않는다 (E6와 같은 규율) | `canopy reconcile` 실행 전후 `_meta/reconcile/state.json` 해시 동일 |
| K6 | 미정규화 외부 변경은 배너로 노출된다 (게이트 초기화 후; 원칙 5·2) | 페이지 파일 직접 편집 후 `canopy status $W` → stderr에 ⚠ 미정규화 N건 |
| K7 | 정규화의 콘텐츠 수정은 writeops 경유다 (한 복도, 원칙 9) **[협약]** | 정규화로 페이지 수정 후 `logs/*.jsonl`에 엔트리 + `index/*.md` 재생성 (I2와 같은 확인) — 그 결과는 자동 축복(K2 후단) |

## L. 다국어 문서 (i18n) ([i18n.md](i18n.md))

> 소스는 한국어 하나, 영어는 파생. 파생의 낡음은 검출 가능해야 한다(원칙 8) — 소스
> 해시가 바뀌면 번역이 STALE. reconcile 원장과 같은 "소스 해시 vs 기록" 메커니즘.

| # | 불변식 | 점검 |
|---|--------|------|
| L1 | 모든 번역은 소스와 소스 버전을 기록한다 | 각 `README.en.md`·`docs/en/*.md` 1행에 `<!-- i18n-source: <path> sha:<40hex> -->`; `make i18n-check`가 없으면 MISSING |
| L2 | 번역은 소스와 동기 상태다 (낡지 않음) | `make i18n-check` → 기록 sha == `git hash-object <source>`; 불일치는 STALE. CI(ci.yml) 포함 |
| L3 | 코드·명령·경로는 번역하지 않는다 (바이트 동일) **[협약]** | 번역은 코드펜스 안을 건드리지 않는다 — 부분 점검으로 소스·번역의 ` ``` ` 개수 일치(FENCE MISMATCH가 아니어야) |

## M. 웹 UI 다국어 (i18n) ([web-ui-i18n.md](web-ui-i18n.md))

> UI chrome만 로케일별로 스왑, 데이터·동작은 불변. go-i18n + `active.<lang>.toml`.
> 위키 페이지 내용은 번역 대상이 아니다(사용자 마크다운 그대로).

| # | 불변식 | 점검 |
|---|--------|------|
| M1 | 템플릿에 하드코딩된 UI-언어 리터럴이 없다 | `grep -lP '[\x{AC00}-\x{D7A3}]' internal/webui/templates/*.html` → 빈 출력. `go test ./internal/webui/`의 `TestNoHardcodedUIStrings` |
| M2 | 모든 로케일이 같은 메시지 ID 집합을 정의한다 | `go test`의 `TestLocaleKeyParity` — `active.en.toml` ID == `active.ko.toml` ID (F1에 포함) |
| M3 | 안전한 폴백 (모르는 언어→기본, 없는 ID→ID 반환, 크래시·빈칸 없음) | `go test`의 `TestLocaleFallback`; 그리고 `curl -H 'Accept-Language: xx' localhost:8737/` → 200 (기본 로케일) |
| M4 | 로케일 파일 추가 = 언어 추가 (코드 변경 0) | `active.<new>.toml` 임베드 후 재빌드 → 언어 선택에 등장; 로더는 `locales/*.toml` 글로브 |
| M5 | 로케일은 데이터를 바꾸지 않는다 (chrome만) | `curl -H 'Accept-Language: en' .../api/search?q=x`와 `ko` 결과 JSON 동일 (serve 실행 상태) |

## 감사 절차

1. `make test && gofmt -l .` (F)
2. `canopy lint $W --json` 하나로 A1–A5, B1, B5 일괄 (counts 확인)
3. C1–C5, D1–D4 순서대로 (D는 dirty 상태를 만들었다가 sync로 정리)
4. E는 `--peek`으로 안전하게
5. G1–G7 (recall·digest·bridge — 임베딩 인덱스 필요)
6. H·I는 `canopy serve`를 띄운 상태에서 (I1은 공개 바인딩 별도 기동, I2는 스크래치 위키 권장)
7. J1–J5는 `canopy version --json` / `canopy migrate status --json`으로 (J6은 1의 `make test`에 포함, J7은 `make release-lineage-check`)
8. K1–K7은 스크래치 위키에서 파일 직접 편집 후 `canopy reconcile --json` / `bless`로
9. L1–L3은 `make i18n-check` (1의 CI에도 포함)
10. M1–M5는 `make test`(M1·M2·M3 테스트) + serve 실행 상태(M3·M5 curl)

> 위반을 발견하면: (1) 그 위반이 **어느 명령을 우회해서** 생겼는지 찾고,
> (2) 우회 경로를 막는 코드/lint를 추가하고, (3) 필요하면 이 목록에 항목을 늘린다.
> 목록이 늘어나는 것은 건강하다. **문서에만 있고 점검 안 되는 규칙이 늘어나는 것이 병이다.**
