# 구현 후 반박 처리

이 기록은 조정자의 최종 판정이다. [원 감사](../../review/2026-10-09/README.md)의
증거·95개 판정과 12개 최종 이의를 수정하지 않는다. 이번 독립 검토의 원시 실패도
그대로 남기고, 수정 뒤 실행과 소스 관찰을 구분한다. 아래의 해결은 선택한 최소
정책과 기록된 입력 범위에 관한 판정이다.

| 검토 | 받은 지적과 처리 | 확인의 범위 |
|---|---|---|
| [문서 AD-01–10](reports/docs-adversarial.md) | Host/인증·실행 예제·쓰기 권한·인간 CLI·int8 선택·random 설명·포트 복구·실습 경로·업그레이드 검증 강도·정확한 표현을 고쳤고 영문 미러를 갱신했다. | 독립 blind 실행 1회, 이후 문서 소스 대조. JSON selector만 저장된 출력으로 재검사했다. 일반 LLM 성공률이나 모델 품질 판정이 아니다. |
| [보존·상태 PSA-01–06](reports/preservation-state-adversarial.md) | 애매한 checkout discard, 생성 링크, 시간 정렬, 잘못된 링크 rewrite, alias scratch 정리를 수정했다. task GC는 기존 inclusive cutoff를 유지하고 계약을 바로잡았다. | 독립 검토자가 같은 반례를 재검사했다. Git 충돌·push 거부·사본 복구·다중 프로세스·unknown 보존 정상 대조도 기록했다. |
| [검색 SA-01–07](reports/search-adversarial.md) | 모델 alias identity, stale manifest adopt, missing ONNX sidecar, native marker null/array, FIFO, manifest 읽기 한도, bridge 후보 allocation을 수정했다. | SA-01은 독립 수정 후 통과. SA-02–07은 원시 독립 실패와 조정자의 durable 회귀·fixture·benchmark. 검토자의 provider 응답이 중단되어 전체 독립 acceptance를 주장하지 않는다. |
| [웹 WA-01–04](reports/web-adversarial.md) | 잘못된 GET query/k의 성공 fallback, 읽지 않은 body의 cap 우회, dotted 주소 preview, 빠진 math client 번역을 수정했다. | 독립 검토자가 같은 HTTP/실제 script adapter 재현으로 통과를 확인했다. HTTP와 Node 검사를 실제 브라우저 상호작용으로 확대하지 않는다. |
| [최종 대조 FC-01–07](reports/final-challenge.md) | 빠진 digest 날짜 진단·multipart file 거부·미래 known-type task done 거부를 구현했다. G2 원문 범위 검사를 강화하고 Git 주석·증거 provenance를 바로잡았다. native URL alias는 공식 metadata로 확인했다. | 소스·기존 증거 대조이며 새 제품 재현은 하지 않았다. multipart는 웹 검토자의 마지막 독립 재검사도 있다. 나머지 새 회귀는 조정자 실행이다. |

최종 전체 검사보다 앞선 author source manifest는 당시 스냅샷이다. 변경 중 전체
검사가 성공한 사실도 변경된 최종 소스의 통과를 뜻하지 않는다. 이를 드러내는
`reports/pre-final-checks.json`을 보존하고, 고정 소스 결과는
`reports/final-checks.json`의 before/after 해시와 명령별 exit로 판정한다.
`pre-final-checks-original.json`은 출력 파일 이름을 옮기기 전 원본이며, 실제 이동된
로그 경로는 `pre-final-checks.json`에 기록했다.

추가한 회귀 검사는 입력과 의미 있는 정상 대조를 함께 둔다. digest는 잘못된
created/updated 필드를 각각 제외·진단하되 전체 페이지 수를 유지한다. 미래
known-type task는 done을 검증 전에 거부하고 원문을 보존하며 명시적 dismiss만
unknown 값을 유지해 허용한다. G2는 Unicode·공백·여러 청크에서 실제 원문 바이트
범위와 현재 page/revision/profile을 확인한다. late 실패·수정 로그와 실제 native
실행, allocation benchmark는 `reports/coordinator-evidence.json`에서 추적한다.

새 snooze는 파일 version 2의 timestamp duration이다. 이를 쓰는 기기는 함께
업그레이드해야 하며, 옛 바이너리가 새 파일을 안전하게 다시 쓴다고 보장하지 않는다.
cache schema 3은 파생 인덱스를 재구축하며 머신 로컬 migration rung을 추가하지
않는다. 전역 page UUID, viewed-origin snapshot/retention, 전체 transaction/rollback,
새 pooling·ANN/DB/CRDT는 원 판정대로 보류했다.

실제 브라우저 paint·focus·IME·U5·offline, 공식 CLS parity, 다른 native 플랫폼과
대형 양자화는 미검증이다. bridge CPU는 여전히 페이지 쌍 수에 비례하며, event 조회는
matching history를 메모리에 읽는다. allocation B/op를 RSS나 일반 성능 보장으로
표시하지 않는다. 배포 결과는 실제 tag·Actions·아카이브·Homebrew 설치·서비스
재시작을 실행한 뒤 별도 기록한다.
