---
name: canopy-wiki
description: canopy로 마크다운 위키를 검색하고 근거를 인용하며, 페이지를 작성·편집·동기화한다. 연결 제안, 위임 태스크, 외부 편집 검토에도 사용한다. 외부 자료 수집은 canopy-ingest를 참고한다.
category: note-taking
---

# Canopy Wiki

canopy가 스키마 검증, 인덱스 갱신, 변경 로그를 맡고, 에이전트는 내용의 정확성과
페이지 간 관계를 판단한다. 이 스킬은 **위키를 사용하는 절차**다. canopy 자체의
코드나 내장 스킬 소스를 개발할 때는 해당 저장소의 개발 규칙을 따른다.

## 시작하기

현재 작업 디렉토리에서 명령을 실행한다. 기본 위키는 `canopy init`이 기록한
`default_wiki`로 선택된다. 다른 위키를 지정하거나 `no wiki found` 오류를 해결할
때는 `--wiki <path>`를 붙인다. 임시 파일은 위키 밖에 둔다.

```bash
canopy status --json
canopy <command> --help
```

자동화에 필요한 출력은 해당 명령의 `--json` 동작을 확인해 사용한다. 최신 플래그는
CLI 도움말에서 확인한다. 설치된 `SKILL.md`는 `canopy skills install`로 갱신된다.
영구적인 지침 수정은 canopy 저장소의 내장 스킬 소스에 반영한다.

| 목적 | 시작 명령 |
|---|---|
| 관련 페이지 찾기 | `canopy search "주제"` |
| 위키의 근거로 답하기 | `canopy recall "질문" --json` |
| 페이지 일부 수정하기 | `canopy checkout <page> --json` |
| 외부 자료 편입하기 | `canopy-ingest` 스킬 |
| 위임된 작업 처리하기 | `canopy tasks list --json` |
| 직접 편집된 파일 검토하기 | `canopy reconcile --json` |

## 검색과 근거 확인

작성 전에 관련 주제와 다른 표현을 검색한다. 같은 주제를 다룬 페이지가 있으면
그 페이지를 보강하고, 독립적으로 설명할 내용이 있을 때 새 페이지를 만든다.

```bash
canopy search "질의"                  # 기본: hybrid, 키워드 + 시맨틱
canopy search "정확한 용어" --mode keyword
canopy list --type concept            # --tag로도 필터 가능
canopy grep "패턴" [page]             # 정규식 검색, 줄 번호 반환
canopy backlinks <page>
canopy show <page>                    # 전체 본문
canopy show <page> --section "배포"    # 해당 제목이 포함된 섹션
canopy show <page> --lines 306-340
canopy recall "질문" --json           # 근거 청크 원문과 출처 slug
```

페이지는 slug로 조회한다. `show`·`recall`·`checkout`을 통한 본문 소비는 에이전트
접근 기록에 반영된다. 검색 결과나 `grep` 매치는 읽음으로 기록되지 않는다.
시맨틱 검색은 첫 호출에 모델을 로드한다. 모델이 없는 환경에서는 hybrid가
keyword로 강등되므로 실제 사용된 검색 모드를 확인한다.

과거 설정이나 인프라 구성처럼 위키에 근거가 있을 법한 질문은 recall부터 확인하고,
답변에 `[[slug]]`를 인용한다. 근거가 없는 부분은 추론임을 밝힌다. 읽기 목적의
요청은 근거를 제시하는 것으로 마치고, 저장이 요청되었거나 위키 관리 범위에
포함된 분석은 적절한 페이지에 남긴다.

## 콘텐츠 판단 기준

- **독립 페이지의 필요성**: 여러 출처에서 반복되는 주제나 한 출처의 중심 주제는
  페이지 후보가 된다. 단순 언급은 기존 페이지의 문맥에 포함한다.
- **새 정보와 기존 정보**: 같은 주제는 보강한다. 주장이 충돌하면 날짜·출처를
  함께 기록해 차이를 설명한다. 정정인지 관점 차이인지 확인한 뒤 통합한다.
- **연결의 의미**: 링크는 독자가 관계를 이해하는 데 도움이 되는 곳에 넣는다.
  유사도는 검토의 근거이며 중복이나 관련성을 확정하는 판정값은 아니다.
- **언어와 분량**: 기본 본문은 한국어로 쓰고 고유명사·기술 용어는 영어를 허용한다.
  제목은 한국어로 쓸 수 있고 slug는 영문 소문자·숫자·하이픈으로 정한다.
  1,000줄을 넘는 페이지는 주제별 분할을 검토한다.
- **출처 보존**: `raw/`에는 확보한 원본을 보존한다. 정정과 해석은 위키 페이지에
  기록한다. 구조·관계·흐름은 아래 Mermaid 지침에 따라 표현한다.

## 새 페이지와 태그

먼저 `canopy tags --json`으로 현재 위키의 type과 taxonomy를 확인한다. 기본 type은
`entity`(사람·조직·제품), `concept`(개념·방법·가이드), `comparison`(비교)이다.

본문은 위키 밖 임시 파일에 작성한다. 아래처럼 따옴표로 감싼 heredoc을 사용하면
`$`, 백틱, `<path>`를 셸이 해석하지 않고 그대로 저장한다.

```bash
body_file=$(mktemp)
cat > "$body_file" <<'MARKDOWN'
## 정의

핵심 내용을 적고, 필요한 문맥에 [[related-page]]를 연결한다.
MARKDOWN
canopy new "새 개념" --type concept --slug new-concept \
  --body-file "$body_file"
canopy show new-concept
rm "$body_file"
```

예시의 `related-page`는 검색으로 확인한 실제 slug로 바꾼다. `--tags`에는 유효 태그를
쉼표로 구분해 넣고 `--links`에는 실존 페이지를 지정한다. 본문에도 실제 대상만
링크한다. `new`는 frontmatter를 생성하므로 `--body-file`에는 본문만 넣는다.
`sources` 같은 메타데이터는 생성 후 checkout 사본의 frontmatter에 추가한다.

태그는 **topic**(주제)과 **form**(글의 형식)으로 나뉜다. 기존 태그의 동의어면
기존 태그를 쓴다. 새 topic은 적용할 페이지가 이미 3개 이상 있을 때
`canopy.toml`의 `topics`에 추가하고 같은 변경에서 기존 페이지도 retag한다.
가까운 태그가 없으면 무관한 태그를 붙이기보다 수요가 쌓일 때까지 보류한다.
form은 닫힌 집합이므로 기존 형식으로 표현할 수 있는지 먼저 검토한다.
`canopy tags --audit`는 미사용 topic과 지나치게 넓은 topic을 검출한다.

## 기존 페이지 편집

부분 수정과 frontmatter 변경에는 checkout을 사용한다. 명령이 반환한 위키 밖
사본을 네이티브 read·grep·edit 도구로 편집하고 checkin으로 반영한다.

```bash
canopy checkout <page> --json
# 반환된 path의 사본 편집
canopy checkin <page>
canopy checkout --list                # 남은 사본 확인
canopy checkin <page> --discard       # 편집을 포기할 때
```

checkin은 스키마·Mermaid 문법·base 충돌을 검증한다. 검증 오류는 사본에서 수정한다.
base 충돌이면 변경 내용을 따로 보존한 뒤 `--discard`로 기존 사본을 회수하고,
다시 checkout해 현재 본문에 변경을 적용한다. 편집 중인 사본은 재checkout만으로
새로워지지 않는다. `type` 변경은 `canopy mv`로 처리한다.

```mermaid
flowchart LR
    Wiki["위키 페이지"] -->|"checkout"| Copy["위키 밖 사본"]
    Copy -->|"네이티브 도구로 편집"| Edited["편집된 사본"]
    Edited -->|"checkin: 검증 통과"| Saved["위키 반영 · 사본 회수"]
    Edited -->|"검증 오류"| Copy
```

열린 사본의 변경은 sync나 reconcile 대상이 아니므로, 편집을 마친 사본은
checkin하거나 discard해 회수한다. 위키 트리의 페이지·파생 인덱스·로그·상태 파일은
각각의 canopy 명령으로 변경한다. 새 raw 자료의 저장은 ingest 절차를 따른다.

| 작업 | 명령과 주의점 |
|---|---|
| 본문 전체 교체 | `canopy update <page> --body-file <file>` — frontmatter 유지 |
| slug 변경 | `canopy mv <page> --slug new-slug` — 인바운드 링크 자동 갱신 |
| type 변경 | `canopy mv <page> --type comparison` |
| 대체된 페이지 보관 | `canopy archive <page>` — 인바운드 링크는 일반 텍스트로 변환 |
| 페이지 삭제 | `canopy rm <page>` — 백링크가 있으면 거부 |

## 위임 태스크 처리

웹과 CLI로 접수된 요청은 `_meta/tasks/`에 저장된다. 위임된 작업을 수행하는
세션에서는 시작 시 sync해 다른 머신에서 처리한 결과를 가져오고 대기 목록을 읽는다.

```bash
canopy sync
canopy tasks list --json
# 유형에 맞게 페이지 검토·수정
canopy tasks done <id> --note "변경 내용"
canopy tasks dismiss <id> --note "처리하지 않는 이유"
canopy sync
```

- **connect**: 두 페이지를 읽어 관계를 확인한다. 적절하면 각각 checkout하고
  관련 문맥에 상호 링크를 넣어 checkin한다. 부적절하면 이유를 적어 dismiss한다.
- **edit / request**: 요청의 의도를 확인하고 현재 페이지에 필요한 수정을 적용한다.
- **edit / body**: 웹 편집기의 본문 제안이다. `base`와 현재 내용을 확인하고 제안과
  비교해 반영한다. 이후에 생긴 수정도 보존하고, 메모(`request`)를 의도 설명으로 쓴다.

`done`은 connect의 상호 링크와 edit의 본문 변경을 검증한다. 거부 메시지에 따라
누락된 작업이나 충돌을 확인한다. 이미 만족된 요청은 이유를 적어 dismiss하고, 지원하지
않는 유형은 호환되는 canopy 버전에서 처리한다. 큐 파일은 CLI로 관리한다.

## 재발견과 연결 제안

후보는 canopy가 선정하고, 에이전트는 내용을 확인해 설명하거나 연결한다.
미리 검토할 때는 상태를 기록하지 않는 `--peek`를 쓴다.

```bash
canopy resurface -n 1 --peek --json
canopy bridge -n 3 --peek --json
canopy resurface -n 1 --json           # 노출 이력 기록
canopy resurface feedback <slug> --up # --down 또는 --snooze 7
canopy bridge --dismiss a:b           # 검토 후 부적절한 페어 제외
```

수락한 연결은 양쪽 페이지의 관련 문맥에 삽입한다. 사용자에게 전달할 때는 출처와
다시 볼 이유를 짧게 적는다. 외부 채널 전송은 요청되거나 설정된 루프의 대상·범위에
따른다. 상태 파일은 해당 명령으로 갱신하고 sync에 포함한다.

## 외부 편집과 건강 점검

```bash
canopy status --json
canopy lint --json
canopy reconcile --json
canopy tags --audit --json
canopy events --since 7d --json
canopy events --kind 'task.*' --json
```

reconcile은 canopy 밖에서 생긴 `edited`·`new`·`deleted` 후보를 보여준다.
`dup_candidates`는 기존 페이지와 비교하는 데 쓰고 `issues`는 구조 문제를 찾는 데
쓴다. 수정은 checkout/checkin·update·new·mv로 반영하고, 검토한 현재 내용이나
삭제는 `canopy reconcile bless <path>`로 수용한다. `bless --all`은 최초 기준선 설정
또는 전체를 검토한 경우에 사용한다.

`island`는 최대 연결 성분에서 떨어진 페이지 묶음이다. 실제 관계가 있으면
상호 링크로 연결하고, 독립 주제라면 의도된 분리인지 확인한다. 이벤트 로그는
이 머신의 활동을 설명하는 보조 자료다. 현재 페이지와 태스크 상태는 위키에서 확인한다.

## Mermaid 다이어그램

구조·관계·흐름이 설명의 중심이고 다이어그램이 이해에 도움이 될 때
`mermaid` 코드펜스를 쓴다. 비교 차원은 표로, 독립 조건은 목록으로 표현한다.
웹 UI와 GitHub가 렌더하며 canopy는 같은 내장 렌더러(v11)의 파서로 검증한다.
오류가 나면 진단의 줄 번호를 확인해 사본이나 본문 파일을 고쳐 다시 제출한다.

- flowchart의 노드·edge·subgraph 라벨은 큰따옴표로 감싼다:
  `A["검증 (스키마·링크)"] -->|"통과"| B["반영"]`.
- 라벨의 줄바꿈은 따옴표 안의 `<br/>`, 큰따옴표는 `#quot;`, `#`는 `#35;`로 쓴다.
- 노드 ID는 `end` 대신 `End`를 쓰고, 소문자 `o`·`x`로 시작하는 ID를 피한다.
  `A---oB`와 `A---xB`는 특수 edge로 해석된다.
- 굵은 글씨나 여러 줄 라벨은 v11 Markdown Strings를 쓴다:
  ``A["`**제목**: 설명`"]``. 실제 줄바꿈과 자동 줄바꿈을 지원한다.
- sequence diagram의 메시지는 `A->>B: 메시지`로 쓴다. 감싼 따옴표는 문자 그대로
  표시된다. 큰따옴표 규칙은 flowchart 라벨에 적용한다.

## 작업 마무리

편집한 페이지를 `canopy show`로 확인하고 관련 lint 결과를 점검한다. 열린 사본을
정리한 뒤, 저장할 페이지·raw·태스크·피드백 변경을 한 묶음으로 sync한다.

```bash
canopy checkout --list
canopy status
canopy sync -m "작업 요약"
```

sync는 위키의 변경 전체를 커밋하므로 기존 변경도 포함되는지 status에서 확인한다.
여러 페이지를 다룬 작업은 마지막에 한 번, 단건 쓰기는 `--sync`로 묶을 수 있다.
push가 실패하면 커밋은 로컬에 남는다. 충돌이나 연결 문제를 해결한 뒤 재실행하고,
미완료 상태는 사용자에게 알린다.
