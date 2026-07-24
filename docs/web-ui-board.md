# Web UI 개발 보드

> [web-ui-plan.md](web-ui-plan.md)(M1–M4) · [web-ui-plan-2.md](web-ui-plan-2.md)(M5–M8)의 실행 보드. kanban처럼 쓴다.
>
> **규칙**: ① 착수할 작업은 Backlog에서 **Doing**으로 옮긴다(동시에 1–2개만).
> ② 끝나면 체크하고 **Done**으로 옮기며, 그 커밋에 보드 갱신을 포함한다.
> ③ 작업 중 발견된 새 일감은 바로 해당 마일스톤 Backlog에 추가한다.
> ④ 마일스톤 완료 기준(✓ Exit)이 모두 체크되기 전에는 다음 마일스톤을 열지 않는다.

## Doing

_(비어 있음 — 다음: M8)_

## Done

### M7 — 느슨한 제안 링크 ([설계 D5](web-ui-plan-2.md))

- [x] 페이지 하단 "제안 링크": 미연결(양방향) 유사 페이지 top-N, 코사인+태그 부스트, min-sim 0.7
- [x] 명시 링크와 시각 구분(추측 연결 스타일), 유사도 표기
- [x] 성능: 요청당 벡터 로드 포함 페이지 로드 24ms(warm) 실측 — 캐시 불필요
- ✓ Exit:
  - [x] 이미 링크된 페이지는 제안에 안 나옴
  - [x] 149페이지 실위키에서 페이지 로드 총 24ms — 추가 지연 예산(<50ms) 충족

### M6 — 읽기 히스토리와 새발견 ([설계 D3·D4](web-ui-plan-2.md))

- [x] reads 저장소: `<wiki>/_meta/webui/reads.json` (`{slug: {first,last,count,source}}`, 포맷 문서화)
- [x] `✓ 읽음` 버튼(form POST, JS 불필요) + 단축키 `r`, 읽음 상태·취소 UI
- [x] 자동 감지: 가시 탭 체류시간(본문 길이 비례, 최소 30s) + 스크롤 70% → `auto` 읽음, 취소 가능
- [x] `canopy mv` 시 reads 키 이관
- [x] 홈 "새발견" 섹션(3–5개): 신규성·허브성·관심 인접(최근 읽은 페이지와의 유사도) 랭킹
- [x] `/special/discover`: 전체 미독 목록 + facet, 읽기 진행률
- [x] invariants.md에 `_meta/webui` 점검 항목 추가
- ✓ Exit:
  - [x] 짧은 체류 방문이 읽음으로 기록되지 않음을 실측 확인
  - [x] 명시/자동 구분 기록·취소 동작, 읽은 페이지는 새발견에서 즉시 제외
  - [x] reads.json이 git 추적 대상으로 커밋된다 (`_meta/webui/`, sync의 CommitAll 범위)

### M5 — 보안: localhost 기본 + 인증 ([설계 D1·D2](web-ui-plan-2.md))

- [x] `--addr` 기본값 `localhost:8737`로 변경, 문서 갱신
- [x] 공개 바인딩 가드: loopback 밖 바인딩이면 인증 벽 자동 활성 — 계정이 없으면
      /setup 외 전부 차단(무보호 공개 상태가 존재하지 않음)
- [x] webauth 저장소: `~/.config/canopy/webauth.json`, bcrypt 해시, 계정 1개
- [x] 부트스트랩: 계정 없으면 1회용 설정 코드를 터미널에 출력 → `/setup`에서 코드+id/pw 등록
- [x] `/login` · `/logout`, 세션(메모리 토큰 + HttpOnly/SameSite=Lax 쿠키, 30일)
- [x] 로그인 실패 지수 지연, POST Origin 검사
- [x] troubleshooting: 계정 재설정(webauth.json 삭제), TLS는 tailscale/프록시 안내
- ✓ Exit:
  - [x] 공개 바인딩에서 인증 없이 어떤 페이지도 안 열림(302), 설정 코드 없이 계정 등록 불가(400)
  - [x] localhost 바인딩은 기존처럼 무인증으로 전 기능 동작
  - [x] 로그인 후 편집 포함 전 기능 동작 (브라우저 검증)

### M4 — 웹에서 쓰기

- [x] 동시 쓰기·충돌 처리 설계 문서: [web-ui-write-design.md](web-ui-write-design.md)
      (낙관적 잠금 = MediaWiki 편집 충돌 패턴, 자동 병합 없음, sync는 웹에서 안 함)
- [x] 편집 폼 `GET/POST /edit/{slug}` → frontmatter 검증 → 저장 → **`writeops.Run`**
      (afterWrite의 불변 파이프라인을 internal/writeops로 추출, CLI와 웹이 같은 함수 호출
      — 우회 쓰기 경로가 구조적으로 불가능). 본문만 편집, frontmatter·생성/이동/삭제는 CLI 전용.
      저장 후 이 페이지의 lint finding을 화면에 표시(저장은 막지 않음, CLI와 동일)
- [x] 충돌 처리: 폼의 SHA-256 해시 불일치 시 409 + 편집본 보존 화면 (curl로 검증)
- ✓ Exit:
  - [x] 스크래치 위키에서 CLI `update`와 웹 편집을 나란히 실행해 부수효과 비교:
        파일(updated 갱신·본문 교체)·logs 엔트리·genindex·FTS 인덱스 모두 동일 확인

### M3 — 브라우징 구조

- [x] `GET /browse?dir=&type=&tag=`: facet 교차 필터(다중 태그 AND), 칩 카운트는 현재
      결과 집합 기준(모든 칩이 유효한 refinement), 활성 칩 클릭 시 해제
- [x] 태그 페이지 `GET /tag/{tag}` → `/browse?tag=`로 통합. 단일 태그 필터 시 co-occurrence
      태그가 카운트순으로 나옴(연관 태그). 페이지 메타 카드의 type/태그 칩도 여기로 연결
- [x] Special: 최근 변경 `/special/recent` — `logops.ReadRecent` 신설(월별 jsonl 역순,
      깨진 줄 스킵), 100건 테이블
- [x] Special: 고아·stale `/special/attention` — `scan.Backlinks()` 고아 + `Schema.StaleDays`
      기준 오래된 페이지 (resurface의 랜덤 픽 대신 전량 결정적 목록이 브라우징에 맞음)
- [x] Special: random page `/special/random` → 302
- [x] (선택) 로컬 그래프 뷰 — 페이지 하단 `<details>`에 서버 렌더 SVG(중심+이웃 radial,
      1-hop이라 물리엔진 불필요), red 노드 = missing
- [x] 헤더 내비(탐색·최근·점검·랜덤) + 홈 디렉토리 카운트를 /browse 링크로
- ✓ Exit:
  - [x] 검색 없이 3클릭 도달: 홈 → 탐색(1) → 태그 칩(2) → 페이지(3) — Chrome에서 확인

### M2 — 검색-우선 UX

- [x] 홈을 검색박스 중심으로 (M1 히어로가 이미 Wikipedia 메인 패턴 — 브라우저로 확인)
- [x] `GET /api/search?q=&k=` JSON 엔드포인트 (~44ms/쿼리, 키스트로크마다 감당 가능)
- [x] 인스턴트 서치: debounce 150ms, ↑↓/Enter/Esc 키보드 내비, 응답 순서 가드
- [x] `/search` exact slug 일치 시 페이지로 302 (Wikipedia "Go" 동작)
- [x] 검색 결과에 chunk 스니펫(`SearchChunks` 재사용, 페이지당 1개) — 매칭 문단 표시
- [x] 위키링크 hover popover(`/api/preview/{slug}`, 250ms 지연, 캐시, red link 제외)
- ✓ Exit:
  - [x] "검색박스 → 타이핑 → ↓↓ Enter → 페이지" 동선을 실제 Chrome에서 검증(콘솔 에러 0)

### M1 — 읽기 뷰어 (MVP)

- [x] `canopy serve` 명령 뼈대: `--addr`(기본 `:8737`), loadWiki, graceful shutdown
- [x] wikilink 렌더: `[[slug]]` → `/page/{slug}`, 없는 페이지는 red link
      (goldmark Resolver 대신 코드 영역을 피하는 전처리로 구현 — class 제어가 쉬움)
- [x] 페이지 렌더 `GET /page/{slug}`: 본문 + frontmatter 메타 카드(type/tags/updated)
- [x] 백링크 섹션: `scan.Backlinks()` 재사용, 페이지 하단 "여기를 링크한 페이지"
- [x] 검색 페이지 `GET /search?q=`: hybrid(`Fuse` + keyword + semantic), 임베딩 스택 없으면 keyword 폴백
- [x] 없는 slug → 404 대신 그 문자열로 검색 결과 폴백(Wikipedia 패턴)
- [x] 홈 `GET /`: 대시보드 요약(디렉토리별 카운트·최근 수정) + 검색박스
- [x] 기본 레이아웃/CSS: `embed.FS` 내장, 모바일 리더블, 다크모드(`prefers-color-scheme`)
- [x] 요청 시 keyword 인덱스 갱신(`indexer.Reindex` 재사용) — serve 중 CLI search/list 동시 실행 확인
- [x] 문서: getting-started에 serve 섹션 추가
- ✓ Exit:
  - [x] 네트워크 인터페이스(192.168.x.x:8737)로 페이지·검색 200 확인 — 폰은 같은 경로(tailscale/LAN)
  - [x] serve 실행 중에도 기존 CLI 명령이 정상 동작한다 (search/list로 확인)
  - [x] `make build` 하나로 빌드된다(Node 툴체인 없음, goldmark 순수 Go 의존성만 추가)

---

## Backlog

### M8 — 위키가 먼저 말을 거는 홈 ([설계 D6](web-ui-plan-2.md))

- [ ] 홈 "오늘의 재발견" 카드: `resurface.PickPages` 1건 + 발췌, 👍/👎/😴 버튼
- [ ] 웹 피드백 → CLI와 같은 `_meta/resurface/state.json` 기록 (쿨다운 공유)
- [ ] bridge 제안 카드(보기 전용, 연결은 CLI/에이전트 안내)
- [ ] 검색 갭 로그: 0건/저점수 질의 → `_meta/webui/search-gaps.jsonl`, `/special/gaps` 화면
- ✓ Exit:
  - [ ] 웹 👍 직후 CLI `resurface`에서 같은 페이지가 쿨다운으로 제외됨을 확인
  - [ ] 빈약한 검색이 갭 로그에 쌓이고 /special/gaps에서 보인다

### Icebox (지금 할 일 아님 — 명시된 조건이 생기면 Backlog로 승격)

- reads를 resurface 후보 선정의 진짜 열람 신호로 사용 (M6 데이터가 쌓이면)
- 제안 링크 원클릭 승격 — writeops 경유 본문 링크 삽입 (M7 사용 경험을 보고)
- digest에 읽기 통계 포함 (M6 이후)
- TLS 직접 종단 (요구 생기면; 그 전엔 tailscale/프록시)
- Quartz exporter로 정적 퍼블리싱 (외부 공유 요구가 생기면)
- fsnotify 캐시 (위키가 수천 페이지에 도달하면)
