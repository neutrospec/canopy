# 검색·모델 독립 반박과 조정자 처리

이 문서는 **조정자가 작성한 종합 기록**이다. 독립 검토자의 원시 결과는 아래 JSON과
재현 소스에 보존한다. 작업자의 마지막 실행은 provider의 응답 중단으로 끝났고
최종 보고서·`worker_done`을 받지 못했다. 조정자는 기존 증거 정리만 요청한 뒤 해당
전용 작업자를 종료했다. 따라서 모든 수정에 독립 재검사가 끝났다고 주장하지 않는다.

- [첫 실행](search-adversarial-results.json), [둘째 실행](search-adversarial-second-results.json),
  [셋째 실행](search-adversarial-third-results.json)은 당시 소스 해시·명령·출력·exit를 담는다.
- `search-adversarial-*.go.txt`는 inert Go 재현 소스다. 원시 기록의 실패와
  `[no tests to run]`은 그대로 남긴다. 해당 package의 검증 통과로 세지 않는다.
- 초기 실행의 module cache는 `-mod=readonly` 설정과 filesystem 읽기 전용을
  구별해야 한다. 최종 조정자 검사는 시작 전 HOME/XDG 격리와 macOS deny-write를 쓴다.

| 반박 | 독립 증거 | 적용한 수정과 현재 확인 경로 |
|---|---|---|
| SA-01: 모델 root/nested directory alias의 weight 변경이 profile·ready cache에 반영되지 않음 | 둘째 실행에서 alias만 이전 벡터 재사용; 셋째 실행은 alias도 재계산·score 1로 통과 | root symlink 해석, 선언 sidecar의 resolved identity 포함. `TestLegacyDirectoryAliasesInvalidateProfileAndReadiness` |
| SA-02: 명시적 adopt가 stale pooling manifest를 반복 거부 | 둘째·셋째 실행의 `TestAdversarialAdoptRepair/invalid-manifest` 실패 | 현재 bundle 검증·digest 후 manifest 교체. 깨진 bundle이면 이전 manifest 유지. `TestExplicitAdoptRepairsStaleManifestAfterBundleValidation` |
| SA-03: converter exit 0만으로 missing sidecar 묶음을 게시 | 둘째·셋째 실행의 inline 정상 대조와 missing-sidecar 오류 | ONNX external declaration을 확인한 뒤 backup/publication. `scripts/test-native-download.py`의 성공 종료·missing artifact 제어 사례 |
| SA-04: native marker의 JSON null/array가 retry 전 TypeError | 둘째·셋째 실행의 object/null/array 대조 | marker dict·필드 검사 후 정상 준비. 같은 Python fixture의 이전 자산 보존·retry 검사 |
| SA-05: FIFO ONNX를 regular-file 검사 전에 열어 readiness가 정지 | 셋째 실행의 regular/FIFO 대조 | open 전 stat과 handle stat. `TestModelReadinessRejectsSpecialFiles`는 child timeout으로 회귀도 제한 |
| SA-06: 8MiB manifest를 거부하기 전에 전체를 두 번 읽음 | 셋째 실행의 누적 allocation 16,909,864 bytes | 공통 bounded reader의 open 전 크기 검사, 최대 1MiB+1 읽기, profile 내부 재사용. `TestModelManifestSizeLimitAndNormalControl`, `BenchmarkRejectOversizedModelManifest` |
| SA-07: bridge 1건 요청에 전체 pair·excerpt를 생성 | 셋째 실행: 200페이지·1024차원·19,900 적격 pair·k=1에서 누적 allocation 103,710,480 bytes | 최대 k 후보 heap과 최종 excerpt만 생성. exhaustive 순위 oracle `TestBridgeTopKMatchesExhaustiveWithFiltersAndTies`, 같은 규모 `BenchmarkPickBridgesDense` |

SA-01의 마지막 원시 기록에는 독립 수정 후 통과가 있다. SA-02–07은 조정자의 durable
회귀·fixture·benchmark 결과와 최종 통합 검사를 근거로 삼는다. 그 결과는 별도
최종 검증 기록에 연결하며 기존 JSON을 사후 통과로 고치지 않는다. benchmark의 B/op는
한 연산의 누적 allocation이며 RSS가 아니다. bridge의 정확 pair 비교 CPU는 여전히
페이지 수의 제곱이다.

평가 해석의 두 반박도 반영했다. unicode61 keyword에서 C++와 C는 같은 토큰이므로
고정 corpus의 symbol 질의를 일반 기호 검색 품질로 확대하지 않는다. hybrid의 span
측정은 semantic top-3 중 hybrid 선택 페이지에 속한 범위이며 독립적인 hybrid chunk
검색 성공률이 아니다. [현재 평가 계약](../../../search-evaluation.md)과 새 출력의
`span_basis`가 이를 명시한다. 과거 corpus·기대값·측정 JSON은 변경하지 않았다.

현재 host의 실제 ORT compile/load/infer와 6페이지·10질의 평가를 다시 실행했다.
공식 CLS reference parity, 다른 native 플랫폼, 실제 대형 양자화와 일반 검색 품질은
미검증이다. 기존 pooling을 유지한 판단을 이 작은 smoke로 정당화하지 않는다.
