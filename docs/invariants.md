# 불변식 목록 (Invariants)

> 시스템이 건강하다 = 아래 목록이 전부 통과한다.
> 각 항목은 **실행 가능한 점검 명령**을 갖는다. 명령이 없는 불변식은 등재 불가
> ([philosophy.md](philosophy.md) 원칙 8).
>
> `W`는 옵션 묶음이 아니라 **위키 경로 하나**다. Bash와 zsh에서 동일하게 쓴다:
>
> ```bash
> W="/path/to/scratch wiki"
> canopy status --wiki "$W" --json
> git -C "$W" status --short
> ```
>
> 변경·거부·캐시 삭제 점검은 스크래치 위키에서 실행한다. HOME·모든 XDG·TMPDIR·
> GOCACHE를 새 임시 영역으로 지정한 **자식 프로세스**에서 Go와 CLI를 시작한다.
> `GOTELEMETRY=off GOPROXY=off GOTOOLCHAIN=local`을 적용하고 미리 준비한 모듈 캐시는
> 읽기만 한다. 실제 사용자 위키·전역 Git 설정·서비스를 점검 fixture로 사용하지 않는다.
> JSON counts는 없는 key를 0으로 읽는다 (`// 0`). lint의 exit 0은 보고 성공이며
> finding이 없다는 뜻이 아니다. 미실행·환경 미검증을 통과로 기록하지 않는다.

## A. 스키마 (명령별 쓰기 검증 + lint 검출)

new와 checkin의 gate, 기존 frontmatter를 유지하는 update, 사후 lint는 범위가 다르다.
A3는 lint의 다섯 키 진단이다. title/created/updated/type은 기존 비어 있음 진단을
유지하고 tags는 **키 존재**를 확인한다. `tags: []`·null은 존재하는 빈 집합이며
scalar는 관용 읽기와 taxonomy 검사를 유지한다. 모든 쓰기의 새 hard gate가 아니다.

| # | 불변식 | 점검 |
|---|--------|------|
| A1 | 페이지는 `entities/ concepts/ comparisons/` 안에만 존재 | `canopy lint --wiki "$W" --json` → `counts["stray-root"] == 0` |
| A2 | 파일명은 영문 소문자·숫자·하이픈 | `canopy lint --wiki "$W" --json` → `counts["bad-filename"] == 0` |
| A3 | frontmatter가 파싱되고 title/created/updated/type/tags 키 계약을 만족한다 | `canopy lint --wiki "$W" --json \| jq -e '([.counts["no-frontmatter"], .counts["bad-frontmatter"], .counts["frontmatter-fields"]] \| map(. // 0) \| add) == 0'` — bad YAML·키별 누락·tags 빈 목록/null/scalar 대조 |
| A4 | type은 canopy.toml 열거형만 | `counts["invalid-type"] == 0` |
| A5 | tag는 canopy.toml taxonomy만 | `counts["invalid-tag"] == 0` |
| A6 | 거부 동작 자체의 확인 | `canopy new t --type guide --wiki "$W"` 가 **에러로 종료** (exit != 0) |
| A7 | 유효 taxonomy는 CLI로 조회 가능하고, 검증과 같은 소스를 쓴다 | `canopy tags --json --wiki "$W" \| jq '.tags\|length'` ≥ 1, 그리고 taxonomy 밖 태그의 거부 메시지에 `canopy tags` 안내 포함 |

## B. 연결성 (lint가 검출)

| # | 불변식 | 점검 |
|---|--------|------|
| B1 | 깨진 wikilink 없음 | `counts["broken-link"] == 0` |
| B2 | 새 페이지의 `--links`는 실존 페이지만 | `canopy new t --type concept --links no-such --wiki "$W"` 가 에러 |
| B3 | 개명 시 인바운드 링크가 따라감 | `canopy mv` 후 `counts["broken-link"]` 증가 없음 |
| B4 | 삭제는 백링크 있으면 거부 | 백링크 있는 페이지에 `canopy rm` → 에러 + 목록 출력 |
| B5 | 모든 페이지는 본토(최대 연결 성분)에서 위키링크로 도달 가능 — 섬 클러스터는 고아 검사를 통과하므로 별도 검출 (philosophy 원칙 13) | `canopy lint --json --wiki "$W" \| jq '.counts.island // 0'` == 0 (또는 각 섬이 의도된 것으로 확인됨). finding에 섬 구성원 나열; 웹 점검 페이지가 다리(섬↔본토 최고 유사도 페어) 제안 |

## C. 파생물 정합성 (생성 시점에 실측)

| # | 불변식 | 점검 |
|---|--------|------|
| C1 | index.md Total == 실제 파일 수 | `canopy status --json --wiki "$W" \| jq .pages` == index.md의 `Total pages` 숫자 |
| C2 | 카테고리 인덱스는 전량 나열 | `rg -c '^- \[\[' "$W/index/concepts.md"` == `rg --files "$W/concepts" \| rg '\.md$' \| wc -l` |
| C3 | 쓰기마다 JSONL 로그 1건 이상 | 쓰기 직후 `tail -1 "$W/logs/$(date +%Y-%m).jsonl"` 의 timestamp가 방금 것 |
| C4 | 검색 인덱스는 완전 재구축 가능 | `canopy reindex --wiki "$W"` 후 검색. 캐시 삭제 대조는 아래 C4 절차로 원본·이력·상태 해시 불변 확인 |
| C5 | 임베딩은 변경분만 갱신 | 무변경 상태에서 `canopy reindex --wiki "$W"` → `embedded_pages == 0`, 수 초 내 종료 |
| C6 | 페이지 열람은 CLI로 전수 가능하며 실측과 일치 | `canopy list --json --wiki "$W" \| jq .count` == `canopy status --json --wiki "$W" \| jq .pages` |

## D. git 동기화

| # | 불변식 | 점검 |
|---|--------|------|
| D1 | 미동기 상태는 일반 text 배너·status로 노출 (JSON·show는 배너 없음) | 파일 **내용 수정** 후 `canopy status --wiki "$W"`에서 dirty 확인; `status --json`의 git 필드와 대조 |
| D2 | upstream이 있으면 sync는 pull이 선행; 전체 dirty 변경을 commit/push | upstream 있는 스크래치 repo의 `canopy sync --wiki "$W"`에서 pull 단계 선행 확인; unrelated dirty 파일도 포함되는 정상 대조 |
| D3 | sync 후 클린 | `canopy sync --wiki "$W" && canopy status --wiki "$W"` → "✓ fully synced" |
| D4 | 위키 안에 canopy 캐시 없음 (캐시는 `$XDG_CACHE_HOME/canopy`) | `test ! -e "$W/.canopy"` && `rg --files "${XDG_CACHE_HOME:-$HOME/.cache}/canopy/index" \| rg '\.db$'` 존재 |

## E. Second-brain 루프 (resurface/bridge)

| # | 불변식 | 점검 |
|---|--------|------|
| E1 | resurface 풀은 30일+ 미접촉 페이지만 | `canopy resurface -n 5 --peek --json --wiki "$W" \| jq '.picks[].days_stale'` → 전부 ≥ 30 |
| E2 | bridge는 미연결 페어만 | `canopy bridge --peek --json --wiki "$W"` 결과 페어에 상호 wikilink 없음 (`canopy backlinks <a>` 로 확인) |
| E3 | 같은 페이지 45일 내 재노출 없음 | pick 후(--peek 없이) 즉시 재실행 → 같은 slug 안 나옴 |
| E4 | 새 snooze는 접수 직후부터 요청 기간 동안 후보를 억제한다 | `canopy resurface feedback <slug> --snooze 1 --wiki "$W"` 후 즉시 `resurface --peek`에서 제외; `go test ./internal/resurface/`의 1/7일·offset·경계 대조. 새 deadline은 RFC3339 duration, legacy 날짜도 읽음 |
| E5 | 상태는 git 추적 (기기 간 동기화) | pick 후 `git -C "$W" status --short _meta/resurface` 에 나타나고 sync로 커밋됨 |
| E6 | --peek은 흔적을 남기지 않음 | `--peek` 실행 전후 `_meta/resurface/state.json` 해시 동일 |

## F. 빌드/테스트

| # | 불변식 | 점검 |
|---|--------|------|
| F1 | 전체 테스트 통과 | `make test` |
| F2 | 임베딩 없는 환경에서도 동작(우아한 강등) | `make build-lite` 바이너리로 `search --mode hybrid` → keyword로 강등 + 경고, exit 0 |
| F3 | 포맷 준수 | `gofmt -l .` 출력 없음 |
| F4 | 어떤 cwd에서도 동작 — 위키로 `cd` 할 이유가 없다 (에이전트가 위키를 작업 공간으로 삼는 것을 막는다) | `which canopy && cd /tmp && canopy status` 성공. 신선한 환경에서도 성립: `init`이 비어 있는 `default_wiki`를 채운다 (`canopy init --json \| jq .default_wiki_set`, 기존 값은 불변) — `go test ./internal/config/`의 adopt 테스트 |
| F5 | 스킬 설치는 멱등·재현 가능하며 기존 설치를 내장 원문으로 갱신한다 | `canopy skills install --dir /tmp/sc` 2회 실행 → 동일 내용, exit 0; `go test ./internal/skills/ -run TestInstallRefreshesEmbeddedSkills` |
| F6 | pi 감지는 pi 자신의 home 해석을 그대로 따른다 — `$PI_CODING_AGENT_DIR` 우선, 없으면 `~/.pi/agent`; agent 디렉토리가 없는 머신에선 pi에 설치하지 않는다 (skills/ 하위는 설치가 만든다) | `h=$(mktemp -d); mkdir -p "$h/pi-home" "$h/.claude/skills"; HOME="$h" PI_CODING_AGENT_DIR="$h/pi-home" canopy skills install >/dev/null; test -f "$h/pi-home/skills/canopy-wiki/SKILL.md" && test ! -e "$h/.pi"` — `go test ./internal/skills/`의 pi 감지 테스트 |

## G. 에이전트 메모리 / 회고 (recall · digest · semantic 후보)

| # | 불변식 | 점검 |
|---|--------|------|
| G1 | recall 결과의 모든 출처는 실존 페이지 | `canopy recall "질문" --json --wiki "$W" \| jq -r '.chunks[].slug'` 의 각 slug가 `canopy show <slug>` 성공 |
| G2 | recall은 현재 parsed body의 원문 범위를 그대로 반환 (요약·제목 프리픽스 없음) | `go test ./internal/indexer/ -run '^TestRecallSourceRoundTrip$' -count=1` — `text == body[start_byte:end_byte]`, source revision·profile 일치 |
| G3 | recall 결과는 score 내림차순 | `.chunks[].score` 가 단조 감소 |
| G4 | digest 기간은 유효한 날짜만 세며 누락·무효 값은 진단하고 전체 페이지 수를 보존한다 | `go test ./internal/digest/ -run '^TestCollectDiagnosesInvalidDatesWithoutLosingPages$' -count=1`; `canopy digest --since 30d --json --wiki "$W" \| jq '{since,stats,invalid_dates}'` — 유효한 미래 날짜와 all-time decisions 정책은 유지 |
| G5 | digest 수치는 실측 | `.stats.created` == `.created_pages \| length` (내부 일관성), created는 frontmatter 기준 |
| G6 | bridge는 기본적으로 미연결 페어만, --include-linked 시 linked 필드로 구분 | 플래그 없이 → 전부 `linked == false`; 플래그 있이 → linked true/false 혼재 가능하되 필드 존재 |
| G7 | new의 관련 페이지 제안은 임계값 이상 + 태그 일치 우선 | (모델과 관련 페이지가 있는 임시 위키에서) `canopy new "related-probe" --slug related-probe --type concept --wiki "$W" --json \| jq '.result.related[].score'` 전부 ≥ 0.8, 최대 5건; 태그가 겹치는 페이지가 앞에 온다. 모델이 없어 빈 목록이면 추천 품질은 미검증 |
| G8 | digest 소비 통계는 이벤트 로그 실측 | `canopy digest --since 30d --json --wiki "$W" \| jq '.top_consumed[]?.events'` 전부 ≥ 1, 각 `.slug`는 실존 페이지 (`canopy show <slug>` 성공) |

## H. 웹 UI 상태 (_meta/webui)

| # | 불변식 | 점검 |
|---|--------|------|
| H1 | 읽기 이력은 위키에 커밋되는 상태다 (파생 캐시 아님) | 읽음 표시 후 `git -C "$W" status --short` 에 `_meta/webui/reads.json` 등장, `canopy sync` 후 clean |
| H2 | reads의 source는 explicit\|auto 뿐이고 explicit은 auto로 강등되지 않는다 | `jq -r '.reads[].source' "$W/_meta/webui/reads.json" \| sort -u` ⊆ {explicit, auto}; 읽음 페이지에서 auto 감지가 다시 와도 source 유지 |
| H3 | mv는 읽기 이력을 함께 이관한다 | 읽음 표시 → `canopy mv <page> --slug new-name` → reads.json에 새 slug만 존재 |
| H4 | agent 접근은 human 읽음 등급을 오염시키지 않는다 (unread 배지 유지) | 미읽음 페이지에 `canopy show <slug>` → `jq '.reads' "$W/_meta/webui/reads.json"`에 해당 slug 없음(또는 기존 source 불변), `_meta/attention/agent-reads.json`에만 기록. 웹에서 여전히 unread |
| H5 | 검색 노출은 읽음이 아니다 | `canopy search "질의"` 실행 전후 `_meta/webui/reads.json`·`_meta/attention/agent-reads.json` 해시 동일 |
| H6 | resurface는 모든 문(웹·에이전트)의 최근 접근을 존중한다 | 오늘 `canopy show <slug>`(또는 웹 읽음) → `canopy resurface -n 20 --peek --json --wiki "$W"`의 picks에 그 slug 없음 |
| H7 | agent 접근의 위키 기록은 일 단위 양자화 (읽기마다 커밋 소음 없음) | 같은 날 `canopy show <slug>` 2회 → 2회차 전후 `agent-reads.json` 해시 동일 (이벤트 DB에만 2건) |
| H8 | 이벤트 DB는 위키 밖 머신-로컬, 위키 안에는 집계 JSON만 | `git -C "$W" ls-files _meta/attention/` → `agent-reads.json`뿐(*.db 없음) && `rg --files "${XDG_STATE_HOME:-$HOME/.local/state}/canopy/attention" \| rg '\.db$'` (기본 ~/.local/state/canopy) 존재 |
| H9 | 기록 페이지(/history)는 이벤트 로그 실측 | `canopy show <slug>` 후 `curl -s localhost:8737/history` 응답에 그 slug 존재 (serve 실행 상태) |
| H10 | 검색 갭은 문과 무관하게 한 파일에 쌓인다 | CLI `canopy search "없는단어xyz" --wiki "$W"` → `tail -1 "$W/_meta/webui/search-gaps.jsonl"`의 `query` 일치·`door == "agent"`; 웹 검색 갭은 같은 파일에 `door == "web"` |
| H11 | grep은 검색이지 읽음이 아니다 (H5와 같은 규율) | `canopy grep "패턴" --wiki "$W"` 실행 전후 `_meta/attention/agent-reads.json` 해시 동일 |

## I. 웹 UI 쓰기·보안 (serve 실행 상태에서 점검)

| # | 불변식 | 점검 |
|---|--------|------|
| I1 | 공개 바인딩은 무인증으로 어떤 페이지도 서빙하지 않는다 (philosophy 원칙 11) | `canopy serve --addr :8737` 후 무인증 `curl -s -o /dev/null -w '%{http_code}' http://<LAN-IP>:8737/page/anything` → 302 (로그인/설정으로 리다이렉트, 본문 없음) |
| I2 | 웹 저장은 edit **제안 태스크** 접수이며 페이지 Markdown을 바꾸지 않는다 (큐·읽음·피드백 상태 쓰기는 별도) | `POST /edit/{slug}` 후 페이지 파일 해시 **동일** + `_meta/tasks/`의 body == 제출 전문. `go test ./internal/webui/ -run TestWebEditFilesProposalNotFile` (현재 계약: [agent-tasks.md](agent-tasks.md)) |
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
| K1 | 원장은 위키에 커밋되는 상태다 (기기 간 동기화) | `bless` 후 `git -C "$W" status --short`에 `_meta/reconcile/state.json` 등장, `canopy sync` 후 clean |
| K2 | 뒷길 변경은 후보로 검출되고, 파이프라인 쓰기는 후보가 아니다 (자동 축복) | 페이지 파일 직접 편집 → `canopy reconcile --json --wiki "$W" \| jq '.foreign[].rel_path'`에 등장; 이어서 그 페이지를 `canopy update`로 고치면 foreign에서 사라짐 |
| K3 | 후보는 결정론적이다 (원칙 6) | 같은 상태에서 `canopy reconcile --json` 반복 → 동일 출력; `.foreign[].dup_candidates[].similarity` 전부 ≥ 0.8 |
| K4 | bless는 현재 내용(부재 포함)을 원장에 기록해, 검토한 변경은 다시 뜨지 않는다 | `canopy reconcile bless <path>` 후 foreign에서 그 path 사라짐; 파일을 지운 뒤 bless → `deleted` 후보도 사라짐 |
| K5 | 보고(기본 실행)는 흔적을 남기지 않는다 (E6와 같은 규율) | `canopy reconcile` 실행 전후 `_meta/reconcile/state.json` 해시 동일 |
| K6 | 미정규화 외부 변경은 배너로 노출된다 (게이트 초기화 후; 원칙 5·2) | 페이지 파일 직접 편집 후 `canopy status --wiki "$W"` → stderr에 ⚠ 미정규화 N건 |
| K7 | 정규화의 콘텐츠 수정은 writeops 경유다 (한 복도, 원칙 9) **[협약]** | 정규화로 페이지 수정 후 `logs/*.jsonl`에 엔트리 + `index/*.md` 재생성 — 그 결과는 자동 축복(K2 후단) |

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
| M1 | 템플릿에 하드코딩된 UI-언어 리터럴이 없다 | `go test ./internal/webui/ -run '^TestNoHardcodedUIStrings$' -count=1` — macOS·Linux 모두 같은 검사 |
| M2 | 모든 로케일이 같은 메시지 ID 집합을 정의한다 | `go test`의 `TestLocaleKeyParity` — `active.en.toml` ID == `active.ko.toml` ID (F1에 포함) |
| M3 | 안전한 폴백 (모르는 언어→기본, 없는 ID→ID 반환, 크래시·빈칸 없음) | `go test`의 `TestLocaleFallback`; 그리고 `curl -H 'Accept-Language: xx' localhost:8737/` → 200 (기본 로케일) |
| M4 | 로케일 파일 추가 = 언어 추가 (코드 변경 0) | `active.<new>.toml` 임베드 후 재빌드 → 언어 선택에 등장; 로더는 `locales/*.toml` 글로브 |
| M5 | 로케일은 데이터를 바꾸지 않는다 (chrome만) | `curl -H 'Accept-Language: en' .../api/search?q=x`와 `ko` 결과 JSON 동일 (serve 실행 상태) |
| M6 | 클라이언트가 추가하는 복사·읽음·그래프 UI도 현재 로케일을 따른다 | `go test ./internal/webui/ -run TestClientMessagesLocalized` — 로케일별 메시지 전달과 HTML 안전성; `make test-ui` — 복사·자동 읽음 표시 검증 |
| M7 | 언어 변경은 검색어·필터를 보존하며 같은 사이트 안으로만 돌아간다 | `go test ./internal/webui/ -run 'TestLanguageSwitchPreservesQuery\|TestLanguageSwitchRejectsExternalRedirect'` |

## N. 이벤트 로그 일반화 ([events.md](events.md))

> 머신-로컬 sqlite 이벤트 DB($XDG_STATE_HOME)는 주의 + 라이프사이클(task/sync/
> reconcile)의 관측 타임라인이다. 관측이지 진실이 아니다 — 위키는 이 DB 없이도
> 완전하다.

| # | 불변식 | 점검 |
|---|--------|------|
| N1 | 이벤트 DB는 비권위 — 지워도 판단·상태가 보존된다 (이력 계기판만 빈다) | DB 삭제(`rm "${XDG_STATE_HOME:?}/canopy/attention/"*.db`) 후 `canopy status --json`·`tasks list --json`·`reconcile --json` 출력이 삭제 전과 동일 |
| N2 | 기록은 best-effort — 실패해도 원 작업은 성공한다 | `XDG_STATE_HOME`을 쓰기 불가 경로로 두고 `canopy tasks add …` → exit 0, 태스크 파일 생성됨 |
| N3 | 라이프사이클 이벤트는 주의 계기판을 오염시키지 않는다 | `canopy tasks add` 직후 `canopy digest --json`의 `top_consumed`에 그 페이지 미출현(태스크 사유로는), `/history`·홈 "오늘의 주의"에 task 행 없음. `go test ./internal/attention/`의 lifecycle 필터 테스트 |
| N4 | kind 어휘 규칙 — 라이프사이클은 `도메인.동작`(점 포함), 주의는 무점 | `canopy events -n 500 --json \| jq -r '.events[].kind' \| sort -u` 가 events.md §2 표의 부분집합 |
| N5 | events gc는 머신-로컬만 지우고 보존 창을 존중한다 | `canopy events gc --days 0` 전후 `git -C "$W" status --short` 동일(위키 무접촉), `--days 365`면 최근 이벤트 잔존 |
| N6 | 위키 진실은 이벤트로 복사하지 않는다 (포인터만) **[협약]** | `task.*` 이벤트의 meta가 task id/사유뿐(본문 없음); `write.*` kind 부재 (`canopy events --kind 'write.*' --json` → 0건) |

## P. Mermaid 다이어그램 검증

> 위키의 mermaid 블록은 웹 UI가 렌더하는 **바로 그 파서**(내장 mermaid.min.js를
> goja로 구동)로 검증한다 — 휴리스틱이 아니라 실제 문법 검사이므로 렌더러와
> 문법 버전의 차이를 줄인다. 검사 환경의 성공과 브라우저 렌더·배치의 성공은 별개다.
> 파서가 못 잡는 의미상 함정(예: sequence 메시지의
> 따옴표)은 스킬 지침이 담당한다.

| # | 불변식 | 점검 |
|---|--------|------|
| P1 | 모든 mermaid 블록의 문법이 검증됐다고 하려면 invalid·unchecked가 모두 0 | `canopy lint --wiki "$W" --json \| jq -e '([.counts["invalid-mermaid"], .counts["mermaid-unchecked"]] \| map(. // 0) \| add) == 0'`. invalid는 문법 실패, unchecked는 환경 미검증으로 따로 보고 |
| P2 | 깨진 mermaid는 쓰기 시점에 거부된다 (A6과 같은 규율) — 거부 메시지에 파서의 줄 번호 포함 | 깨진 블록 본문으로 `canopy new t --type concept --wiki "$W"` → **에러** (exit != 0), stderr에 `Parse error` |
| P3 | 검증 파서와 웹 렌더러는 같은 번들이다 (버전 스큐 없음) | `internal/mermaid`가 번들을 소유(embed)하고 webui가 그것을 서빙 — `go test ./internal/webui/`의 번들 서빙 테스트 + `grep -rn "mermaid.min.js" internal/webui/static/vendor/` 빈 출력 |
| P4 | 검증 환경의 결함(JS 셔임 갭·미해결 promise)은 **fail-open** — 다이어그램 탓으로 돌리지 않는다 | `go test ./internal/mermaid/`의 에러 분류 테스트; env 갭은 lint에서 `mermaid-unchecked`(info)로 가시화, 쓰기는 통과 |

## R. 체크아웃 편집 ([checkout-design.md](checkout-design.md))

> 수술적 편집은 에이전트의 네이티브 도구가 제일 잘한다 — canopy는 경쟁하는 대신
> **파일을 빌려주고**(checkout, 위키 밖 working copy) 되돌려받을 때 검증한다
> (checkin = 게이트 + base 대조). 위키 트리는 에이전트에게 읽기 전용이 될 수 있다.

| # | 불변식 | 점검 |
|---|--------|------|
| R1 | checkout은 페이지를 바꾸지 않으며 첫 접근의 agent 집계만 허용한다; 당일 반복은 위키 no-op | 아래 전체 파일 snapshot 비교에서 첫 checkout의 차이는 `_meta/attention/agent-reads.json`뿐, 당일 반복은 전체 동일. 사본이 위키 밖인지 별도 확인 (R2) |
| R2 | working copy는 위키 밖 머신-로컬 — git·reconcile이 보지 못한다 (H8과 같은 규율) | 사본 경로가 `$XDG_STATE_HOME/canopy/checkout/` 아래, `git -C "$W" ls-files \| grep checkout` 빈 출력, 편집해도 `canopy reconcile --json` 후보 0건 |
| R3 | checkin은 사본의 명령별 gate를 통과해야 반영된다 (parse/title·type/created 불변·tag taxonomy·Mermaid; A3 키 진단은 lint) | 사본에 깨진 mermaid/무효 태그를 넣고 `canopy checkin <slug>` → **에러** (exit != 0), 위키 파일 불변 |
| R4 | base 불일치는 거부 — 조용한 merge 없음 (T9의 base와 같은 개념) | checkout 후 위키 파일을 직접 바꾸고 checkin → 에러 + 재checkout 안내 |
| R5 | checkin·discard는 working copy를 회수한다 (rm 불필요) | checkin 성공 후 사본·메타 부재; `--discard`도 동일 |
| R6 | 열린 checkout은 일반 text 배너와 status에 노출된다 (JSON·show 배너 예외) | 열린 상태의 text 명령에서 `✎ … N건`; `canopy status --wiki "$W" --json \| jq .checkouts_open` ≥ 1 |
| R7 | type·created 변경은 checkin이 거부한다 (type 이동은 `canopy mv`가 유일 경로) | 사본의 type을 바꿔 checkin → 에러 |
| R8 | 무변경 checkin은 no-op + 회수 | 편집 없이 checkin → 위키 파일 해시 동일, 사본 회수, exit 0 |

## S. 태그 taxonomy 거버넌스 ([taxonomy.md](taxonomy.md))

> taxonomy는 실측 사용의 반영이지 희망사항 목록이 아니다. 태그는 주제(topic,
> 열린 집합·수요 기반)와 형식(form, 닫힌 집합·동결) 두 facet으로 분리 선언하고,
> 증감 압력은 topic에만 건다. 판단(동의어·분할 설계·회수 확정)은 LLM, 실측은 코드.

| # | 불변식 | 점검 |
|---|--------|------|
| S1 | 현재 taxonomy는 topic/form 두 facet을 쓴다; 구형 단일 tags는 관용 읽기 | `canopy tags --json --wiki "$W"`로 facets 확인; 둘 다 없을 때 `tags --audit --json`의 legacy == true. 혼합 설정의 legacy tags는 facet audit 밖일 수 있으므로 canopy.toml과 합집합도 직접 대조 |
| S2 | 검증(new/lint)의 유효 태그는 topics ∪ forms ∪ legacy tags의 중복 없는 합집합이다 (A5·A7) | `canopy tags --json --wiki "$W" \| jq -e '(.tags \| length) == (.tags \| unique \| length)'` 및 canopy.toml의 세 목록 합집합 대조; `go test ./internal/config/ ./internal/lint/`의 taxonomy 대조 |
| S3 | 운영 taxonomy의 미사용 topic을 회수 검토한다 — 새 topic은 수요(페이지 ≥ 3) 후 추가 **[협약]** | `canopy tags --audit --json --wiki "$W" \| jq '.unused_topics'`를 검토. 성숙한 taxonomy는 0건 또는 사유 기록; 새 init 추천 목록은 bootstrap 예외 |
| S4 | 분별력 잃은 topic 없음 — 한 topic이 전체 페이지의 `broad_topic_pct`%(기본 25, 0=끔)를 초과하면 분할 검토 대상. form은 면제(형식의 본성) | `canopy tags --audit --json --wiki "$W" \| jq '.overbroad_topics\|length'` == 0 |
| S5 | audit는 보고만 한다 — 흔적 없음 (E6·K5와 같은 규율) | `canopy tags --audit --wiki "$W"` 실행 전후 `git -C "$W" status --short` 동일, exit 0 |

## T. 에이전트 태스크 큐 ([agent-tasks.md](agent-tasks.md))

> 위임은 파일로(`_meta/tasks/<id>.json`, 태스크당 하나), 완료는 검증으로:
> done은 유형별 기계 조건을 확인한다. 요청의 의미적 만족은 현재 본문과 별도 대조한다.
> edit는 파일 전체 해시를 비교하므로 metadata-only 변경도 통과하고 base 없는 구형식은
> 존재만 확인한다. 접수·수행·검증의 분담 — 원칙 6의 태스크판.

| # | 불변식 | 점검 |
|---|--------|------|
| T1 | 태스크는 위키와 함께 여행하는 self-versioned 상태다 | 접수 후 `git -C "$W" status --short _meta/tasks/` 등장, 파일에 `"version"` 필드; `canopy sync` 후 clean |
| T2 | done은 유형별 검증을 통과해야만 닫힌다 | 미링크 페어로 `canopy tasks add connect a b --wiki "$W"` → `canopy tasks done <id>` **에러** (exit != 0); 양쪽에 `[[상호 링크]]` 추가 + `canopy update` 후 done 성공 |
| T3 | 닫힌 태스크는 기본 목록에 안 나온다 (loop 재수행 없음) | done/dismiss 후 `canopy tasks list --json --wiki "$W"` 에 그 id 미출현; `--all` 에는 status와 함께 표시 |
| T4 | 같은 connect 페어는 보존된 태스크를 재사용한다 (dismissed 포함); GC 이후 영구 억제는 보장하지 않음 | 같은 페어 접수 2회 → `_meta/tasks/` 파일 1개; dismiss 파일이 유지되는 동안 재접수해도 pending으로 안 살아남 |
| T5 | 모르는 유형 또는 미래 version은 done이 거부된다 — list·dismiss는 unknown을 보존하며 동작 | `go test ./internal/tasks/ -run 'Test(Unknown|Future)' -count=1` — 미래 known-type·미지 type의 done 거부·원본 hash 유지와 dismiss 정상 대조 |
| T6 | 접수는 페이지를 수정하지 않는다 (수행은 에이전트 판단 이후) | `POST /task/edit/{slug}` (또는 `tasks add`) 전후 그 페이지 파일 해시 동일 |
| T7 | pending은 노출된다 (원칙 5) | pending 1건 이상일 때 `canopy status --wiki "$W"` 출력에 tasks 라인, `--json`에 `tasks_pending` ≥ 1 |
| T8 | gc는 pending을 절대 지우지 않는다 | pending + 오래된 done 혼재 상태에서 `canopy tasks gc --days 0` → done만 삭제, pending 잔존 |
| T9 | 웹 편집 제안은 제출 본문 전문을 태스크에 보존한다 (diff 아님 — 에이전트가 base 대비 비교) | 웹 편집 저장 후 태스크 JSON의 `body` == 제출 본문(개행 정규화 제외), `base` == 접수 시점 파일 sha256. `go test ./internal/webui/`의 `TestWebEditFilesProposalNotFile` |
| T10 | 큐는 웹에서 열람 가능하고, 철회는 pending edit에 한한다 (connect 기각은 에이전트 판단) | serve 상태에서 `/special/tasks`에 pending 표시; edit 태스크 철회 버튼 → dismissed; connect에 `POST /task/cancel/<id>` → 400. `TestTasksScreenAndWithdraw` |

## U. 웹 작업 공간

| # | 불변식 | 점검 |
|---|--------|------|
| U1 | 탐색 메뉴에 있는 화면은 해당 항목에 현재 위치를 표시하고, 모바일에서도 키보드로 메뉴·검색에 접근할 수 있다 | `go test ./internal/webui/ -run TestWorkspaceNavigation`; `make test-ui` — 메뉴 토글·Escape·검색 단축키 검증 |
| U2 | 홈의 페이지·읽음·미읽음·대기 작업 수는 위키 상태의 실측이며 열람만으로 읽음 처리하지 않는다 | `go test ./internal/webui/ -run TestWorkspaceSummary` — 빈 위키·페이지·읽음·닫힌 태스크 포함 |
| U3 | 자동완성은 현재 질의의 응답만 표시하고, 지우기·Escape·IME 조합 중에는 이전 응답이 다시 열리지 않는다 | `make test-ui` — 응답 순서 역전·입력 지우기·Escape·한글 조합·키보드 이동 검증 |
| U4 | 읽음 표시와 복사 완료는 실제 작업 성공 뒤에만 표시된다 | `make test-ui` — HTTP 실패·네트워크 실패·클립보드 실패 검증 |
| U5 | 360px 화면에서 페이지 전체가 가로로 넘치지 않고, 넓은 표·코드·그래프는 자체 영역에서 스크롤된다 | `canopy serve` 후 브라우저의 360px·1280px 뷰포트에서 홈·탐색·페이지·그래프 확인; 콘솔 `document.documentElement.scrollWidth <= window.innerWidth` → `true` (폭이 넓은 표·코드 포함) |
| U6 | 본문 첫 제목이 표시 제목과 정확히 같을 때만 중복 표시를 접는다 (원문 보존) | `make test-ui` — 동일 제목·다른 제목 표시 검증 |

## V. 감사 후 보존·정합성 강화 (구현 수용 계약)

아래는 [2026-10-10 구현](implementation/2026-10-10/README.md)의 수용 계약이다.
새 검사는 해당 묶음의 구현과 함께 추가하며, 구현 보고서에 실제 실행 결과를 남긴다.
검사 명령은 임시 HOME/XDG에서 실행한다. 아직 실행하지 않은 항목을 통과로 읽지 않는다.

| # | 불변식 | 점검 |
|---|--------|------|
| V1 | sync는 pull 전·후와 add/commit 전에 미해결 index와 진행 중 Git 작업을 거부하며 양쪽 원문을 보존한다 | `go test ./internal/gitops/` — bare remote·두 clone의 autostash 충돌 및 정상 대조 |
| V2 | 단일 파일은 완성 후 원자적으로 게시하고 create/archive는 목적지를 덮지 않는다; 협력 쓰기의 잠금은 alias·프로세스 간에 동일하다 | `go test ./internal/safefile/ ./internal/wikilock/` — 게시 전 실패·no-clobber·mode·독자·두 프로세스·종료 회복 |
| V3 | checkout orphan은 덮지 않고, 게시 후 실패는 실제 적용과 미완료 단계를 구별한다 | `go test ./internal/checkout/ ./internal/writeops/ ./cmd/canopy/` — corrupt/missing meta·사본 보존·후처리/cleanup 오류 |
| V4 | scan/lookup과 checkout 폐기의 slug 모호성, 생성 인덱스 링크, code 경계, YAML scalar 의미, rename 이력과 태그 집계가 일치한다 | `go test ./internal/wiki/ ./internal/checkout/ ./internal/genindex/ ./internal/reads/ ./internal/config/` — duplicate/nested·dotted discard/checkin·생성 링크 해석·alias/path/self/code·unknown YAML·중복 태그 |
| V5 | 재현 불가 상태는 unknown과 정확한 수를 보존하거나 쓰기 전에 거부하고, 손상을 정상 0건으로 숨기지 않는다 | `go test ./internal/tasks/ ./internal/reads/ ./internal/resurface/ ./internal/reconcile/ ./internal/config/` — old/current/future·top/nested·null·큰 정수 |
| V6 | GC는 실제 열거한 known-closed 파일만 지우고, 시간 조회는 instant로 비교하며 새 snooze는 즉시 억제한다 | `go test ./internal/tasks/ ./internal/attention/ ./internal/resurface/` — filename/id·unknown status·offset/cutoff/tie·1/7일 경계 |
| V7 | 성공한 동시 제안의 전문이 모두 남고 connect ID가 다른 pair를 재사용하지 않는다 | `go test ./internal/tasks/` — 두 프로세스·완성 게시·pair 충돌·legacy dismissed |
| V8 | mutation JSON은 한 object이고 exit·적용·후처리·요청 sync 결과가 실제 파일/원격 효과와 일치한다 | `go test ./cmd/canopy/`; `make test-cli` — six verbs·기본/sync·거부·게시 후/commit/push 실패 |
| V9 | 검색 근거는 현재 원문이며 빈/삭제/변경 페이지와 profile 변경의 stale chunk를 사용하지 않는다 | `go test ./internal/indexer/ ./internal/store/ ./internal/search/ ./cmd/canopy/` — cold/cache·raw 근거·추론 실패·profile 변경 |
| V10 | 잘못된 k/vector는 정상 오류이고 취소된 대기 요청은 Embed를 시작하지 않으며 동률은 결정론적이다 | `go test ./internal/search/ ./internal/embed/ ./cmd/canopy/` — bounds·NaN/dimension/norm·취소·per-page cap 대조 |
| V11 | 모델/native 준비는 required 파일과 전송·digest·추출 완료를 확인하며 기존 정상 묶음을 보존한다 | `go test ./internal/embed/`; `make test-assets` — fake transport/tar·중단·XDG·unsupported tuple·고지 |
| V12 | HTTP는 허용 Host·route 예산과 parse 4xx를 강제하고 사용하지 않는 file upload를 거부하며 raw 실행 HTML 및 무인증 logout을 안전하게 처리한다 | `go test ./internal/webui/` — hostile/allowed Host·query/form/body cap·unused multipart 400·정상 렌더링·logout |
| V13 | 읽음/제안/반영 UI는 실제 payload·상태에 따라 변하며 HEAD가 노출 기억을 소비하지 않는다 | `go test ./internal/webui/`; `make test-ui` — false/HTML/bad JSON·세션·실패·unknown input·미읽음/추천 구분 |
| V14 | 지원 Markdown code 경계를 지키며 내장 수식 자산과 두 vendor 고지가 배포물에 포함된다 | `go test ./internal/webui/ ./internal/wiki/`; `make test-assets` — fence/span 원문·asset route·license/manifest |
| V15 | 실제 lite CLI 실패 경로와 사용자 상태 격리가 CI와 release의 동일 소스에 연결된다 | `make test-cli`; `make release-check` — F2 강등·실패 보존·HOME/XDG·발행 전 검사 |
| V16 | Overview·Discover 등 탐색 HTML은 추론을 기다리지 않고, 완전한 현재 캐시만 의미 추천에 사용한다; 서버당 하나의 백그라운드 갱신은 요청 취소와 독립적이며 종료 시 정리한다 | `go test ./internal/webui/ -run 'TestNavigation|TestBackground'`; `go test ./internal/indexer/ -run TestInspect` — 멈춘 엔진·반복 요청·keyword 검색·변경 원문·profile 변경·완료 후 의미 추천·종료 대조 |

## 감사 절차

사전 조건은 임시 HOME/XDG/TMPDIR, quoted 경로 `W`, 준비된 바이너리·모듈 캐시다.
원격 sync 점검은 자기 bare remote와 clone, Git identity를 준비한다. G의 semantic·
recall과 관련 후보는 ORT·모델·임베딩 인덱스가 필요하며 lite 결과와 구분한다.
HTTP·실제 브라우저·타 플랫폼 native 검사는 각각 pass/fail/unverified를 남긴다.

1. 격리된 환경에서 `make test`와 `gofmt -l .` (F). `-run`을 좁히면 먼저
   `go test <package> -list '<pattern>'`에 실제 테스트가 있는지 확인하고 `-count=1`로 실행한다.
2. `canopy lint --wiki "$W" --json` 하나로 A1–A5, B1, B5 일괄 (A3의 bad-frontmatter 포함)
3. C1–C5, D1–D4 순서대로 (D는 dirty 상태를 만들었다가 sync로 정리)
4. E는 `--peek`으로 안전하게
5. G1–G7 (recall·digest·bridge — 임베딩 인덱스 필요)
6. H·I는 `canopy serve`를 띄운 상태에서 (I1은 공개 바인딩 별도 기동, I2는 스크래치 위키 권장)
7. J1–J5는 `canopy version --json` / `canopy migrate status --json`으로 (J6은 1의 `make test`에 포함, J7은 `make release-lineage-check`)
8. K1–K7은 스크래치 위키에서 파일 직접 편집 후 `canopy reconcile --json` / `bless`로
9. L1–L3은 `make i18n-check` (1의 CI에도 포함)
10. M1–M7은 `make test`(M1·M2·M3·M6·M7 테스트) + serve 실행 상태(M3·M5 curl)
11. T1–T10은 스크래치 위키에서 `canopy tasks add/done/dismiss/gc`로 (T6·T7은 파일 해시·status 확인, T9·T10은 `go test ./internal/webui/` + serve 상태)
12. N1–N6은 스크래치 위키 + 임시 `XDG_STATE_HOME`에서 `canopy events`/`tasks`로 (N3은 1의 `make test`에도 포함)
13. S1–S5는 `canopy tags --json` / `canopy tags --audit --json`으로 (jq 확인)
14. P1–P4는 스크래치 위키에서 깨진 mermaid 블록으로 `canopy new`(P2 거부)·파일 직접
    심기 후 `canopy lint --json`(P1)으로, P3·P4는 1의 `make test`에 포함
15. R1–R8은 스크래치 위키 + 임시 `XDG_STATE_HOME`에서 `canopy checkout/checkin`
    사이클로 (R3은 사본 오염 후, R4는 위키 파일 병행 수정 후), H11은 grep 전후
    agent-reads.json 해시로
16. U1–U6은 `make test-ui`와 `go test ./internal/webui/`로 확인하고, U5는 360px·1280px
    브라우저에서 긴 제목·표·코드를 포함한 페이지로 점검한다.

### C4·R1의 파일 보존 비교

아래 helper는 `.git`을 제외한 **전체 위키 파일**의 상대 경로·SHA-256을 기록한다.
페이지 개수나 Git status만 비교하면 놓치는 비추적 파일도 포함한다. 증거는 위키 밖에 둔다.

```bash
audit_tmp=$(mktemp -d)
snapshot_wiki() {
  python3 - "$W" "$1" <<'PY'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
files = {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
         for p in root.rglob('*') if p.is_file()
         and p.relative_to(root).parts[0] != '.git'}
pathlib.Path(sys.argv[2]).write_text(json.dumps(files, sort_keys=True, indent=2) + '\n')
PY
}
```

C4는 **새 스크래치 위키·임시 XDG_CACHE_HOME**에서만 삭제 대조를 한다. 기록 전
검색어가 매치하는 페이지를 만들어 검색 갭 기록이 섞이지 않게 한다. 재구축 후 검색은
hit의 slug·본문 근거도 확인한다. 로그를 새로 append하거나 과거 줄을 재생성하면 실패다.

```bash
canopy new "캐시 점검" --slug cache-probe --type concept --wiki "$W"
snapshot_wiki "$audit_tmp/cache-before.json"
rm -rf "${XDG_CACHE_HOME:?}/canopy/index"
canopy reindex --no-embed --wiki "$W"
canopy search "캐시 점검" --mode keyword --wiki "$W" --json
snapshot_wiki "$audit_tmp/cache-after.json"
cmp "$audit_tmp/cache-before.json" "$audit_tmp/cache-after.json"
```

R1은 아직 읽지 않은 cache-probe로 첫 접근과 당일 반복을 나눈다. 첫 접근에는
agent 집계만 달라질 수 있고 `.agent["cache-probe"]`의 days가 1이어야 한다. 두 번째
checkout은 위키 전체가 같아야 한다. 파일 밖 이벤트 DB의 추가 기록은 이 비교 범위 밖이다.

```bash
snapshot_wiki "$audit_tmp/read-before.json"
canopy checkout cache-probe --wiki "$W" --json > "$audit_tmp/checkout.json"
snapshot_wiki "$audit_tmp/read-after.json"
jq -S 'del(."_meta/attention/agent-reads.json")' "$audit_tmp/read-before.json" > "$audit_tmp/a.json"
jq -S 'del(."_meta/attention/agent-reads.json")' "$audit_tmp/read-after.json" > "$audit_tmp/b.json"
cmp "$audit_tmp/a.json" "$audit_tmp/b.json"
jq -e '.agent["cache-probe"].days == 1' "$W/_meta/attention/agent-reads.json"
canopy checkout cache-probe --wiki "$W" --json
snapshot_wiki "$audit_tmp/read-repeat.json"
cmp "$audit_tmp/read-after.json" "$audit_tmp/read-repeat.json"
canopy checkin cache-probe --discard --wiki "$W"
```

기록한 사본 path가 W 밖인지 확인한다(R2). A3는 bad YAML과 키별 누락·tags=[]/null/
scalar, P1은 정상·문법 오류·환경 미검증을 별도로 대조한다. predicate의 false와 CLI
자체 실행 오류도 구분한다. 이 순서는 점검 방법이며 현재 실행 결과는 구현 보고서에 남긴다.

> 위반을 발견하면: (1) 그 위반이 **어느 명령을 우회해서** 생겼는지 찾고,
> (2) 우회 경로를 막는 코드/lint를 추가하고, (3) 필요하면 이 목록에 항목을 늘린다.
> 목록이 늘어나는 것은 건강하다. **문서에만 있고 점검 안 되는 규칙이 늘어나는 것이 병이다.**
