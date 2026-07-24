# Web UI 개발 계획 (1차: M1–M4)

> 목표: Obsidian 없이 브라우저에서 위키를 **검색-우선(search-first)** 으로 탐색한다.
> 제약: 기존 canopy CLI 명령과 invariant는 전부 그대로 동작해야 한다.
> 상태: **구현 완료** (2026-07-24, M1–M4). 설계 결정 기록으로 보존.
> 후속: [web-ui-plan-2.md](web-ui-plan-2.md)(보안·읽기 이력·발견) ·
> [web-ui-plan-3.md](web-ui-plan-3.md)(뷰어·그래프·연결성) ·
> 실행 기록은 [web-ui-board.md](web-ui-board.md).
>
> ⚠ 이 문서의 일부 결정은 후속 계획으로 **대체**되었다. 본문에 표시해 두었다.

## 1. 문제 정의

1. **브라우징 UI가 없다.** 지금은 Obsidian이 git repo를 pull해서 보는 방식인데,
   위키 열람에 별도 앱 + pull 절차가 필요하고 모바일/타 기기 접근이 불편하다.
2. **index가 1차원이다.** `index/*.md`는 디렉토리별 평면 목록이고, 트리 복잡화를
   피하려고 카테고리 분류를 의도적으로 두지 않았다. 페이지가 늘수록 평면 목록은
   브라우징 수단으로 기능하지 못한다.
3. **검색에서 시작하는 경험이 없다.** hybrid search(FTS+임베딩)는 CLI에만 있고,
   위키를 "읽는" 흐름과 분리되어 있다. Wikipedia처럼 검색박스가 입구여야 한다.

## 2. 선행 사례 조사

| 장르 | 대표 | 얻을 것 | 못 얻는 것 |
|---|---|---|---|
| 정적 digital garden | [Quartz 4](https://quartz.jzhao.xyz/) | 위키링크·백링크·popover preview·전문검색이 "기본 장착"이라는 UX 기준선 | 정적 사이트라 클라이언트 FlexSearch 키워드 검색뿐 — canopy의 시맨틱/하이브리드 검색 불가 |
| 동적 vault 뷰어 | [Perlite](https://github.com/secure-77/Perlite) | "vault 디렉토리를 그대로 서빙"하는 단순한 배포 모델 | PHP 별도 스택, canopy 스키마·store를 모름 |
| 서버형 마크다운 위키 | [Silverbullet](https://alternativeto.net/software/silverbullet/about/), Gollum, Wiki.js | 로컬 서버 + 브라우저 편집 UX | 자체 쓰기 경로를 가짐 → canopy의 afterWrite invariant와 충돌 |
| Wikipedia UX | MediaWiki | 검색-우선 첫 화면, "What links here", 없는 페이지 → 검색 폴백, 카테고리 = 다대다 라벨(트리 아님) | [연구](https://iskouk.wordpress.com/2008/09/22/wikipedias-approach-to-categorization/)상 위키피디아식 자유 카테고리는 폴크소노미라 노이즈가 큼 |
| 분류 이론 | [Faceted classification](https://en.wikipedia.org/wiki/Faceted_classification) | 고정 트리 대신 **여러 축의 교차 필터**가 브라우징에 우월 | — |

결론: **완제품을 가져다 쓰면 검색이 죽고(정적) 또는 쓰기 invariant가 깨진다(서버형 위키).**
UX 패턴만 가져오고 구현은 canopy 내부 패키지 재사용이 맞다.

## 3. 핵심 결정

### D1. `canopy serve` — 같은 바이너리 안의 Go HTTP 서버 (읽기 전용부터)

- 킬러 기능인 hybrid search는 store(SQLite FTS + 벡터) + ORT 임베딩 엔진이
  필요하다. **정적 사이트로는 재현 불가**이므로 로컬 서버가 유일한 선택지.
- `internal/{wiki,store,search,indexer,resurface,logops}`를 그대로 재사용한다.
  CLI와 같은 코드 경로 → 스키마 표류 없음, CLI는 명령 하나(`serve`) 추가될 뿐
  아무것도 변하지 않는다.
- 배포 없음: `canopy serve` → `localhost:PORT`. 다른 기기는 tailscale 등으로.
  인증·다중 사용자는 비범위(로컬 신뢰 네트워크 전제).

### D2. 카테고리는 트리가 아니라 facet — dir × type × tags 교차 필터

- 새 분류 체계를 만들지 않는다. 이미 모든 페이지가 가진 세 축
  (`dir`: entities/concepts/comparisons, frontmatter `type`, `tags`)을
  faceted browsing으로 노출한다. Wikipedia 카테고리의 노이즈 문제를 피하고,
  "canopy가 개념 정리하다 트리를 복잡하게 만드는" 시나리오 자체가 없다.
- `genindex`의 1차원 `index/*.md`는 **에이전트용으로 그대로 유지**한다.
  웹 UI의 목록은 store에서 동적으로 계산하므로 genindex와 충돌하지 않는다.

### D3. 서버는 쓰기 금지 (M4 전까지)

- 모든 mutation은 CLI(`new/update/mv/rm/archive/sync`)로만. 서버는 파일과
  DB를 읽기만 한다. 편집 기능은 M4에서 afterWrite 파이프라인을 그대로 호출하는
  방식으로만 추가한다(우회 쓰기 경로 신설 금지 — philosophy.md 원칙).
- CLI와의 동시성: serve는 요청마다 `store.Open`(read) 또는 read-only 커넥션.
  keyword 인덱스 갱신은 기존 `refreshIndex` 재사용(~250페이지 스캔은 저렴),
  임베딩 갱신은 지금처럼 쓰기 시점에만.

### D4. 프론트엔드는 SSR + 최소 JS — 빌드체인 없음

- goldmark + [abhinav/goldmark-wikilink](https://go.abhg.dev/goldmark/wikilink)
  (Resolver로 `[[slug]]` → `/page/slug`), `html/template`, `embed.FS`로 정적
  에셋 내장. Node 툴체인을 도입하지 않는다 — `make build` 하나로 끝나야 한다.
- 인스턴트 서치·popover 정도는 vanilla JS 수백 줄이면 충분하다(Quartz가 증명).

## 4. 마일스톤

### M1 — 읽기 뷰어 (MVP): "Obsidian 없이 읽을 수 있다"

- `canopy serve [--addr :8737]`
- `GET /page/{slug}`: 마크다운 렌더(위키링크 해석, frontmatter 메타 카드,
  존재하지 않는 링크는 붉은 링크로), 하단에 **백링크 섹션**("What links here",
  store의 links 데이터 재사용).
- `GET /search?q=`: 기존 hybrid 검색(`search.Fuse` + `SearchKeyword` +
  `SearchSemantic`) 결과 페이지.
- 없는 slug 접근 → 404 대신 해당 문자열로 검색 결과 표시(Wikipedia 폴백 패턴).
- `GET /`: 일단 대시보드(index.md 상당) + 검색박스.
- 완료 기준: 폰 브라우저(tailscale)로 위키 전체를 읽고 검색할 수 있다.

### M2 — 검색-우선 UX: "입구가 검색이다"

- 홈을 Wikipedia 메인처럼 **큰 검색박스 중심**으로 교체.
- `GET /api/search?q=` JSON + 타이핑 인스턴트 서치(debounce, ↑↓/Enter 키보드
  내비게이션, slug 정확 일치 시 바로 이동).
- 검색 결과에 chunk 단위 스니펫(recall의 `SearchChunks` 재사용) — 페이지 제목이
  아니라 "어느 문단이 맞았는지"를 보여준다.
- 위키링크 hover 시 popover preview(Quartz 패턴, 대상 페이지 첫 문단).

### M3 — 브라우징 구조: "검색 안 하고도 길을 잃지 않는다"

- `GET /browse?dir=&type=&tag=`: facet 교차 필터 목록(다중 태그 AND).
- 태그 페이지 `GET /tag/{tag}`: 해당 태그 전체 + 연관 태그(co-occurrence).
- Special pages(모두 기존 패키지 재사용):
  - 최근 변경(logs), 고아 페이지·stale 페이지(resurface), random page.
- (선택) 로컬 그래프 뷰 — 백링크+related가 본질이므로 우선순위 낮음. 하지 않아도
  M3 완료로 친다.

### M4 — (선택) 웹에서 쓰기: "Obsidian 완전 대체"

- 편집 폼 → 서버 내부에서 lint → 저장 → **afterWrite 파이프라인 호출**
  (genindex/logops/reindex/sync 안내까지 CLI와 동일 경로).
- 여기서부터는 동시 쓰기·충돌 처리가 필요하므로 별도 설계 문서를 먼저 쓴다.

### 부록 — 정적 export가 필요해지면

외부 공유/퍼블리싱 요구가 생기면 정적 생성기를 자작하지 말고
[Quartz](https://quartz.jzhao.xyz/)를 exporter로 붙인다(Obsidian 호환 vault를
그대로 읽으므로 우리 repo 구조와 이미 호환). 시맨틱 검색만 포기하면 된다.

## 5. 비범위 / 리스크

- ~~인증, 다중 사용자, 공개 인터넷 노출: 비범위. 로컬/사설망 전제.~~
  → **대체됨**: 2차 계획 M5에서 localhost 기본 바인딩 + 공개 바인딩 시 인증
  필수로 구현 ([web-ui-plan-2.md](web-ui-plan-2.md) D1·D2, philosophy 원칙 11).
  다중 사용자는 여전히 비범위.
- Obsidian 전용 문법(callout, embed 등): goldmark 확장으로 발견되는 대로 대응.
  M1에서는 깨지지 않고 원문이 보이는 수준이면 충분.
- 성능: ~250페이지 규모에서는 문제 없음. 요청당 재스캔이 느려지는 시점
  (수천 페이지)에 fsnotify 캐시를 도입한다 — 그 전에 미리 만들지 않는다.
