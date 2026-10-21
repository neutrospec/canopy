> **한국어** · [English](en/getting-started.md)

# 처음 시작하는 사람을 위한 안내

> "LLM으로 위키를 관리한다"는 게 처음이라면 이 문서부터 읽으세요.
> 개념 → 용어 → 15분 따라하기 → 일상 사용 순서로 안내합니다.
> 먼저 모델·Git 없이 페이지 생성과 keyword 검색을 확인하고, 필요한 기능을 추가합니다.

## 1. 이게 뭔가요?

메모 앱을 써본 적이 있다면 이런 경험도 있을 겁니다: 열심히 적어두긴 했는데,
**다시 찾지도 않고, 서로 연결되지도 않고, 시간이 지나면 낡아버리는** 메모 무덤.

Andrej Karpathy가 제안한 "LLM wiki"는 이 문제에 대한 답입니다. 아이디어는 단순해요:

- 지식을 **평범한 마크다운 파일**로 쌓는다 (특별한 앱·DB 없음)
- 정리·연결·일관성 유지 같은 **귀찮은 유지보수는 LLM(에이전트)이** 한다
- 사람은 **소스를 고르고, 질문하고, 판단**만 한다

챗봇에게 매번 처음부터 물어보는 방식(RAG)과의 차이는 "축적"입니다.
위키에서는 한 번 종합된 지식이 파일로 남고, 새 정보가 들어올 때마다 **기존
페이지가 갱신되고 연결이 늘어납니다.** 질문할수록, 읽을수록 위키가 부자가 돼요.

canopy는 이 방식을 실제로 굴러가게 하는 도구입니다. 원래는 에이전트에게
"인덱스 갱신해라, 로그 남겨라, 링크 깨뜨리지 마라" 같은 규칙을 긴 지시문으로
지키게 했는데, 에이전트도 사람처럼 깜빡합니다. canopy는 그 규칙들을
**명령어 안에 코드로** 넣었습니다. 검증 범위는 명령마다 다르며, 내용의 정확성과
요청 만족은 사람·에이전트가 확인합니다.

## 2. 미리 알아둘 용어 8개

| 용어 | 뜻 |
|------|-----|
| **페이지** | 마크다운 파일 하나 = 지식 한 조각. `concepts/`(개념), `entities/`(사람·제품·조직), `comparisons/`(비교) 세 폴더 중 하나에 산다 |
| **slug** | 파일 이름 (확장자 제외). `opc-ua.md`의 slug는 `opc-ua`. 영문 소문자·숫자·하이픈을 쓴다 |
| **frontmatter** | 페이지 맨 위의 `---`로 감싼 메타데이터 (제목, 날짜, 태그). canopy가 자동으로 만들어준다 |
| **위키링크** | 본문 속 `[[다른-페이지]]` — 페이지끼리의 연결. 연결이 많을수록 지식이 그물이 된다 |
| **백링크** | 반대 방향: "누가 이 페이지를 참조하지?" `canopy backlinks <페이지>`로 확인 |
| **시맨틱 검색** | 단어가 아니라 **의미**로 찾는 검색. "벡터 캐시"와 KV-cache처럼 표현이 다른 근거도 찾을 수 있다. 내장 추론 코드와 별도로 설치한 로컬 bge-m3 모델이 처리하며, 준비 후에는 인터넷·외부 서버가 필요 없다 |
| **facet** | 웹 UI의 분류 방식. 고정된 폴더 트리 대신 폴더×타입×태그를 **교차 필터**로 조합해 탐색한다 |
| **섬** | 서로는 링크됐지만 위키의 본체(최대 연결 덩어리)와 끊어진 페이지 무리. 고아 검사는 통과하므로 `canopy lint`가 따로 잡아준다 |

## 3. 준비물

- macOS (Apple Silicon 권장) + [Homebrew](https://brew.sh)
- Go 툴체인: `brew install go onnxruntime`
- (선택) [Obsidian](https://obsidian.md) — 위키를 예쁘게 보는 뷰어. 위키 폴더를
  vault로 열기만 하면 된다
- (선택) LLM 에이전트 — 없어도 canopy는 혼자 쓸 수 있다. 에이전트는 §5에서

## 4. 15분 따라하기

### 4-1. 설치

```bash
git clone https://github.com/neutrospec/canopy && cd canopy
make build && make install        # canopy → ~/.local/bin
which canopy                      # 안 나오면 troubleshooting.md의 PATH 항목 참조
```

### 4-2. 위키 만들기

```bash
wiki="$HOME/canopy-demo"           # 아직 canopy.toml이 없는 새 폴더를 선택
mkdir -p "$wiki"
canopy init --wiki "$wiki"
canopy status --wiki "$wiki" --json # root가 이 경로인지 확인
```

`canopy.toml`이라는 파일이 생깁니다 — 이 위키의 규칙(허용 태그, 페이지 종류)이
적힌 파일입니다. `init`은 전역 `default_wiki`가 비어 있을 때만 채웁니다. 기존 설정은
유지됩니다. 이 안내에서는 모든 위키 명령에 `--wiki "$wiki"`를 붙여 대상을 고정합니다.

### 4-3. 첫 페이지

```bash
echo "OPC-UA는 산업 장비들이 서로 대화할 때 쓰는 표준 프로토콜이다." | \
  canopy new "OPC-UA란 무엇인가" --slug opc-ua --type concept --tags infrastructure --body-file - --wiki "$wiki"
canopy show opc-ua --wiki "$wiki"
```

```
✓ created concepts/opc-ua.md
NEXT: canopy sync   (commit & push this change, ...)
```

이 한 줄로 canopy가 한 일: 파일 생성 + 규칙 검사(잘못된 태그였다면 거부) +
목차(index.md) 재생성 + 활동 로그 기록. 예전엔 전부 사람/에이전트가 잊지 말아야
했던 것들입니다.

> 제목이 한글뿐이면 `--slug`로 영문 파일명을 꼭 지정하세요. 제목은 한글이어도
> ASCII 문자가 없는 제목에서는 파일명을 자동으로 만들 수 없습니다.
> `show`가 성공했다는 것과 요청한 본문이 맞다는 것은 별개이므로 실제 내용을 확인하세요.

### 4-4. Git에 저장하기 (선택)

페이지는 이미 로컬 파일에 저장됐습니다. Git 이력을 남기려면 저장소와 작성자 설정이
필요합니다. 아래 이름·이메일은 자신의 값으로 바꾸세요. 원격 없이도 로컬 commit은 됩니다.

```bash
git -C "$wiki" init -b main
git -C "$wiki" config user.name "Your Name"
git -C "$wiki" config user.email "you@example.com"
canopy sync -m "첫 페이지" --wiki "$wiki"
```

sync는 upstream이 있으면 pull → commit → push하고, 없으면 원격 단계를 건너뜁니다.
기존 변경을 포함한 **위키 전체 dirty 파일**이 대상입니다. 여러 기기에서 쓰려면 원격
저장소를 연결하고 첫 push의 upstream을 설정하세요. 충돌이나 실패가 나면 현재 Git
상태를 확인하고 해결합니다. 일반 text 명령에는 상태 배너가 있지만 JSON과 `show`에는
없으므로 `canopy status --json --wiki "$wiki"`로 직접 확인합니다.

### 4-5. 검색

```bash
canopy search "OPC" --mode keyword --wiki "$wiki"   # 모델 없이 동작
canopy search "산업 장비 통신" --wiki "$wiki"        # hybrid, 미준비면 keyword 강등
```

첫 시맨틱 검색 전에 `canopy model pull --wiki "$wiki"`로 모델을 받고(약 2.3GB),
`canopy reindex --wiki "$wiki"`로 임베딩을 만듭니다. 선택적인 int8 변환은 저장소의
`scripts/quantize-model.py --help`에서 의존성·경로를 확인하세요. 기본 출력인
`bge-m3-int8`을 쓰려면 위키 `canopy.toml`의 `[embedding]`에서 `model`을 그 이름으로
바꾸고, 같은 `--wiki`로 `model status` 확인과 `reindex`를 실행합니다. lite 빌드는 내장 추론 없이 keyword 검색과 외부 임베딩 API를 지원합니다. `recall`은 임베딩이
필수이므로 준비되지 않았다면 keyword search와 show로 근거를 확인합니다.

외부 서비스를 쓰려면 위키 파일 대신 `~/.config/canopy/config.toml`
(`XDG_CONFIG_HOME` 설정 시 그 아래 `canopy/config.toml`)에 추가합니다.

```toml
[embedding]
provider = "openai"
url = ""
token = ""
model = "embeddinggemma-2"
dimension = 768
input_format = "auto"
revision = ""
```

`url`에는 전체 OpenAI 호환 endpoint(예: `https://host/v1/embeddings`), `token`에는
Bearer 토큰을 넣습니다. 서버가 받는 모델명이 다르면 `model`을 맞추세요.
BGE-M3 서비스는 `model = "bge-m3"`, `dimension = 1024`로 설정합니다.
외부 제공자는 내장 모델을 로딩하지 않으며 문서와 검색어를 설정한 서버로 전송합니다.
URL이 비어 있으면 연결 준비가 실패하며 hybrid는 keyword로 강등합니다.
서버가 EmbeddingGemma 접두사를 이미 붙인다면 `input_format = "plain"`을 사용하세요.

```bash
canopy model status --wiki "$wiki" --json
canopy reindex --check --wiki "$wiki" --json
canopy reindex --wiki "$wiki"
```

`--check`는 API 호출 없이 `needs_reindex`를 보고하고 호환되지 않는 캐시 벡터를
무효화합니다. 모델·주소·차원·입력 형식·`revision` 변경은 재인덱싱 대상으로 판단하고,
토큰 교체는 기존 벡터를 유지합니다. 같은 모델명으로 서버 가중치를 교체하면
`revision`을 바꾸세요. 실행 중인 `serve`에는 재시작 후 적용됩니다.
상세: [임베딩 제공자 설계](embedding-providers.md).

### 4-6. 웹으로 보기

```bash
canopy serve --wiki "$wiki"      # → http://localhost:8737
```

브라우저에서 위키가 검색-우선 웹사이트로 열립니다: 검색박스에 타이핑하면
즉시 결과가 뜨고, 페이지에는 문법 하이라이팅·수식·다이어그램이 렌더되며,
상단 `그래프` 메뉴에서 지식 그물 전체를 줌/드래그로 탐험할 수 있습니다.
페이지를 읽고 `✓ 읽음`(단축키 `r`)을 누르면 홈의 "새발견"이 아직 안 읽은
페이지를 추천해줍니다.

### 4-7. Obsidian으로 보기 (선택)

Obsidian → "Open folder as vault" → 이 실습에서 만든 `~/canopy-demo`를 선택합니다.
기존 위키로 진행했다면 `$wiki`가 가리키는 폴더를 선택하세요.
위키링크가 클릭 가능해지고, Graph View에서 지식 그물이 보입니다.
canopy와 Obsidian은 같은 파일을 다루므로 어느 쪽에서 고쳐도 됩니다 —
단, Obsidian에서 고쳤다면 `canopy reconcile --wiki "$wiki"`로 외부 변경을 검토하고,
필요한 내용을 정규화한 뒤 `update` 또는 `reconcile bless`로 수용합니다
([정규화 계약](reconcile-design.md)). 부분 편집에는 위키 밖 사본을 빌리는
`checkout`/`checkin`도 사용할 수 있습니다.

## 5. 일상 사용

### 혼자 쓸 때

```bash
canopy search "주제" --wiki "$wiki"   # 이미 있나? 뭐랑 연결되나?
canopy list --wiki "$wiki"            # --type, --tag로 필터
canopy checkout opc-ua --wiki "$wiki" # 반환된 사본에서 부분 편집
# 편집 사본을 별도 백업한 뒤 checkin, 현재 내용 확인
canopy checkin opc-ua --wiki "$wiki"
canopy lint --wiki "$wiki"            # 관련 구조·링크 finding 검토
canopy sync --wiki "$wiki"            # 전체 변경을 저장할 범위일 때
```

### 에이전트와 함께 쓸 때 (진짜 재미는 여기부터)

에이전트에게 canopy 사용법을 가르치는 건 명령 하나입니다:

```bash
canopy skills install    # 에이전트용 사용 설명서 2개를 스킬 폴더에 설치
```

자동 감지는 지원하는 에이전트 경로에 한정됩니다. 임의 경로는
`canopy skills install --dir <path>`로 매번 지정하며, 설치 후 해당 에이전트의
스킬 활성화·로드 방법도 확인하세요. 이후 역할 분담은 이렇게 됩니다:

| 사람 | 에이전트 | canopy |
|------|----------|--------|
| 소스를 던진다 ("이 글 정리해줘") | 읽고 요약하고 페이지 작성 | 규칙 검사·인덱스·로그·저장 |
| 질문한다 | recall 또는 keyword search + show로 근거를 찾아 답변 | 근거와 출처 제공 |
| 판단한다 ("연결해" / "아니야") | 실행 | 기록 |

페이지 생성·수정·이동·삭제는 **canopy 명령을 거치게** 하세요. 부분 편집은
`checkout` 사본에서 하고 `checkin`으로 반영합니다. `raw/`에 원본 자료를 추가하거나
`canopy.toml` 설정을 수정할 때는 기존 내용을 보존하며 해당 파일을 직접 다룹니다.
명령의 검증과 실제 내용 확인을 함께 사용합니다.
읽기 질문은 근거와 한계로 마치며 자동으로 페이지나 sync 작업을 만들지 않습니다.

### 브라우저로 읽을 때

```bash
canopy serve --wiki "$wiki"               # localhost:8737 — 인증 없음
canopy serve --addr :8737 --wiki "$wiki"   # 외부 접근 — 인증 필수
```

검색박스에서 시작해 페이지·백링크·태그(facet)·전체 그래프로 이동하는 웹
UI입니다. 없는 페이지로 가면 Wikipedia처럼 검색 결과를 대신 보여주고,
페이지의 "✎ 편집"에서 본문 수정안을 제출할 수 있습니다. 저장하면 전문이 edit
태스크로 접수되고 **현재 페이지는 그대로**입니다. `할 일` 화면에서 제안을 확인하고,
에이전트가 처리한 뒤 실제 페이지를 확인하세요. canopy가 에이전트를 자동으로 실행하거나
처리 시간을 보장하지는 않습니다. frontmatter와 페이지 생성/이동/삭제는 CLI 전용입니다.

웹 요청은 에이전트의 검토를 거치는 접수 경로입니다. 기존 내용을 검색하고 중복·
모순·출처를 판단해 반영합니다. 위임 큐 처리 절차와 done의 검사 범위는
[agent-tasks.md](agent-tasks.md)에 있습니다.

폰이나 다른 컴퓨터에서도 보려면 두 번째 형태로 열면 되는데, 이때는
**인증이 자동으로 요구됩니다**: 최초 1회 serve를 실행한 터미널에 설정 코드가
출력되고, 브라우저의 `/setup`에서 그 코드와 함께 원하는 id/비밀번호를 만들면
됩니다. tailscale 같은 사설망과 함께 쓰는 것을 권장합니다
(문제가 생기면 [troubleshooting.md](troubleshooting.md) 참고).

## 6. 위키가 먼저 말을 걸게 하기

쌓기만 하는 위키는 창고입니다. canopy에는 **잊힌 지식을 되살리는** 명령이 있어요:

```bash
canopy resurface --wiki "$wiki"         # 잊힌 페이지 재발견
canopy bridge --wiki "$wiki"            # 유사하지만 연결 안 된 페이지
canopy digest --since 30d --wiki "$wiki" # 회고 소재
```

이 루프는 두 접점으로 동작합니다:

- **웹 홈** — `canopy serve`의 홈 화면에 매일 "오늘의 재발견" 카드(잊힌 페이지
  1개 + 👍/👎/😴 버튼)와 "연결 제안" 카드가 뜹니다. 버튼 반응은 CLI와 같은
  상태 파일에 기록되므로 어느 쪽에서 반응해도 중복 노출이 없습니다.
- **에이전트** — 매주 위 명령을 돌려 요약을 보내달라고 하면(예: Telegram 아침
  브리핑) 위키가 찾아와서 말을 겁니다.

답을 못 찾은 검색도 버려지지 않습니다 — 웹 UI의 `점검 → 검색 갭`에 쌓여서
"위키에 없는 지식 = 다음에 만들 페이지" 후보가 됩니다. 자세한 설계와 cron
레시피는 [second-brain.md](second-brain.md)에.

## 7. 자주 묻는 질문

**Q. 시맨틱 검색이 아무것도 안 찾아요.**
`canopy model pull`을 했는지, `canopy reindex`로 임베딩이 만들어졌는지 확인.
`canopy model status`가 진단해줍니다. 급하면 `--mode keyword`는 항상 동작해요.

**Q. 페이지를 지우려는데 거부당해요.**
다른 페이지가 참조 중이라서입니다(링크가 깨지니까). 목록을 보여주니
`canopy archive`(보관)를 쓰거나, 정말 지우려면 `--force`(참조가 평문으로 바뀜).

**Q. 태그를 새로 쓰고 싶은데 거부당해요.**
의도된 동작입니다 — 태그가 아무렇게나 늘면 분류가 무의미해져요.
먼저 `canopy tags --wiki "$wiki"`로 기존 태그를 확인하세요. 새 topic은 적용할
페이지가 3개 이상일 때 `canopy.toml`의 `topics`에 추가하고 기존 페이지도 retag합니다.
`forms`는 닫힌 집합이며 구형 `tags`는 호환용입니다. 새 init의 추천 목록은 bootstrap이므로
처음부터 모두 사용할 필요는 없습니다. `tags --audit`로 운영 중 수요에 맞춰 정리합니다.

**Q. 여러 컴퓨터에서 써도 되나요?**
네, 그게 git을 쓰는 이유입니다. 각 기기에서 `canopy sync`만 잘 하면 되고,
sync는 pull을 먼저 하지만 충돌할 수 있습니다. 실패 시 양쪽 내용을 보존하고
Git 상태를 확인해 해결한 뒤 진행하세요.

**Q. 위키가 망가진 것 같아요.**
`canopy lint`가 뭐가 문제인지 알려줍니다. 검색 인덱스가 이상하면
`canopy reindex --wiki "$wiki"`로 재구축하세요. 검색 DB는 파생 캐시이지만
`logs/`와 `_meta/`는 복원할 수 없는 이력·상태를 포함하므로 보존해야 합니다.
**페이지·raw·이력·상태를 함께 Git에 저장하세요.**

## 8. 다음 읽을 문서

- [troubleshooting.md](troubleshooting.md) — 뭔가 안 될 때 (PATH, 시맨틱 검색, 웹 인증)
- [philosophy.md](philosophy.md) — 왜 이렇게 설계했는지 (원칙마다 검증 방법 포함)
- [invariants.md](invariants.md) — 시스템이 건강한지 직접 점검하는 체크리스트
- [second-brain.md](second-brain.md) — 재발견 루프의 설계와 에이전트 운영 레시피
- 웹 UI가 왜 이런 모양인지 궁금하면 — README의 "설계 기록" 문서 지도를 따라가세요
