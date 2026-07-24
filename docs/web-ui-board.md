# Web UI 개발 보드

> [web-ui-plan.md](web-ui-plan.md)의 실행 보드. kanban처럼 쓴다.
>
> **규칙**: ① 착수할 작업은 Backlog에서 **Doing**으로 옮긴다(동시에 1–2개만).
> ② 끝나면 체크하고 **Done**으로 옮기며, 그 커밋에 보드 갱신을 포함한다.
> ③ 작업 중 발견된 새 일감은 바로 해당 마일스톤 Backlog에 추가한다.
> ④ 마일스톤 완료 기준(✓ Exit)이 모두 체크되기 전에는 다음 마일스톤을 열지 않는다.

## Doing

_(비어 있음 — 다음: M2)_

## Done

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

### M2 — 검색-우선 UX

- [ ] 홈을 검색박스 중심으로 교체(Wikipedia 메인 패턴)
- [ ] `GET /api/search?q=` JSON 엔드포인트
- [ ] 인스턴트 서치: debounce, ↑↓/Enter 키보드 내비, slug 정확 일치 시 즉시 이동
- [ ] 검색 결과에 chunk 스니펫(`SearchChunks` 재사용) — 매칭된 문단 표시
- [ ] 위키링크 hover popover preview(대상 페이지 첫 문단)
- ✓ Exit:
  - [ ] "검색박스에서 시작 → 두세 타이핑 → Enter → 페이지"가 기본 동선이 된다

### M3 — 브라우징 구조

- [ ] `GET /browse?dir=&type=&tag=`: facet 교차 필터(다중 태그 AND)
- [ ] 태그 페이지 `GET /tag/{tag}`: 소속 페이지 + 연관 태그(co-occurrence)
- [ ] Special: 최근 변경(logs 재사용)
- [ ] Special: 고아·stale 페이지(resurface 재사용)
- [ ] Special: random page
- [ ] (선택) 로컬 그래프 뷰 — 안 해도 M3 완료
- ✓ Exit:
  - [ ] 검색 없이 facet만으로 임의 주제의 페이지에 3클릭 내 도달

### M4 — (선택) 웹에서 쓰기

- [ ] 동시 쓰기·충돌 처리 설계 문서 먼저 작성
- [ ] 편집 폼 → lint → 저장 → afterWrite 파이프라인 호출(우회 쓰기 경로 금지)
- ✓ Exit:
  - [ ] 웹에서 한 편집이 CLI 편집과 완전히 같은 부수효과(genindex/logops/reindex)를 남긴다

### Icebox (요구 생기면)

- [ ] Quartz exporter로 정적 퍼블리싱
- [ ] fsnotify 캐시(수천 페이지 도달 시)
