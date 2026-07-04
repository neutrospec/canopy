---
name: canopy-ingest
description: 외부 콘텐츠(아티클/스레드/문서/교육자료)를 수집해서 위키 페이지로 변환하는 워크플로우. 페이지 조작 자체는 canopy-wiki 스킬 참조.
category: note-taking
---

# Canopy Ingest — 외부 콘텐츠 위키화

소스를 수집해 위키에 편입하는 절차. CLI 조작법은 `canopy-wiki` 스킬이 진실 소스다.

## 절차

1. **원문 전체 확보** — 요약본으로 만족하지 마라.
   - 웹 아티클: `web_extract` 먼저 → 5000자 초과로 잘리면 browser로 전체 텍스트 추출
   - X/Twitter: Nitter 미러(`https://nitter.net/<handle>/status/<id>`) → 원문 링크 따라가 전체 수집
   - GitHub raw: `https://raw.githubusercontent.com/...` 직접
   - 첨부파일(pptx/pdf 등): `raw/attachments/<주제>/`에 저장. **한글 파일명은 영문으로 변경**
     (의미 기반 번역, lowercase-hyphen, 날짜/버전 유지, 원본명은 페이지에 기록)
2. **기존 페이지 검색** — `canopy search "주제"` (hybrid). 같은 주제가 있으면
   새 페이지가 아니라 **기존 페이지 보강**이 기본.
3. **raw 저장** — 원문을 `raw/articles/` 또는 `raw/transcripts/`에 저장
   (파일명: `<주제>-<출처>-<연도>.md`). raw/는 이후 불변.
4. **페이지 작성/갱신** — `canopy new` 또는 `canopy update` (canopy-wiki 스킬 참조).
   frontmatter `sources:`에 raw 경로 기재는 본문 작성 시 포함.
   하나의 소스가 여러 페이지를 갱신하는 것은 정상이다 (엔티티/개념 교차 갱신).
5. **`canopy sync`** — 마무리. 절대 생략 금지.

## 페이지 유형별 구조

- **entity**: 개요 → 핵심 사실/날짜 → 다른 엔티티와의 관계([[링크]]) → 출처
- **concept**: 정의 → 현재 지식 상태 → 미해결 질문 → 관련 개념([[링크]])
- **comparison**: 비교 대상과 이유 → 차원별 표 → 결론/종합 → 출처

## 주의

- 외부 도구의 설치법/경로/명령은 **공식 문서로 검증 후** 페이지에 기록 (추측 금지)
- API 오류를 "지원 안 함"으로 단정하지 말 것 — 진단(서버 상태 → 모델/엔드포인트 확인 →
  근본 원인) 후 결론
- 대량 ingest(10+ 페이지 영향)는 시작 전에 사용자에게 범위 확인
