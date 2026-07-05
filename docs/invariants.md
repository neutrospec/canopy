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

## B. 연결성 (lint가 검출)

| # | 불변식 | 점검 |
|---|--------|------|
| B1 | 깨진 wikilink 없음 | `counts["broken-link"] == 0` |
| B2 | 새 페이지의 `--links`는 실존 페이지만 | `canopy new t --type concept --links no-such $W` 가 에러 |
| B3 | 개명 시 인바운드 링크가 따라감 | `canopy mv` 후 `counts["broken-link"]` 증가 없음 |
| B4 | 삭제는 백링크 있으면 거부 | 백링크 있는 페이지에 `canopy rm` → 에러 + 목록 출력 |

## C. 파생물 정합성 (생성 시점에 실측)

| # | 불변식 | 점검 |
|---|--------|------|
| C1 | index.md Total == 실제 파일 수 | `canopy status --json $W \| jq .pages` == index.md의 `Total pages` 숫자 |
| C2 | 카테고리 인덱스는 전량 나열 | `grep -c '^\- \[\[' <wiki>/index/concepts.md` == `ls <wiki>/concepts/*.md \| wc -l` |
| C3 | 쓰기마다 JSONL 로그 1건 이상 | 쓰기 직후 `tail -1 <wiki>/logs/$(date +%Y-%m).jsonl` 의 timestamp가 방금 것 |
| C4 | 검색 인덱스는 완전 재구축 가능 | `rm -rf ~/.cache/canopy && canopy reindex $W` 성공 후 `canopy search "test" $W` 동작 |
| C5 | 임베딩은 변경분만 갱신 | 무변경 상태에서 `canopy reindex $W` → `embedded_pages == 0`, 수 초 내 종료 |

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

## 감사 절차

1. `make test && gofmt -l .` (F)
2. `canopy lint $W --json` 하나로 A1–A5, B1 일괄 (counts 확인)
3. C1–C5, D1–D4 순서대로 (D는 dirty 상태를 만들었다가 sync로 정리)
4. E는 `--peek`으로 안전하게

> 위반을 발견하면: (1) 그 위반이 **어느 명령을 우회해서** 생겼는지 찾고,
> (2) 우회 경로를 막는 코드/lint를 추가하고, (3) 필요하면 이 목록에 항목을 늘린다.
> 목록이 늘어나는 것은 건강하다. **문서에만 있고 점검 안 되는 규칙이 늘어나는 것이 병이다.**
