한국어

# cng-collector — Windows CNG provider 관측

Windows에 **등록된 CNG provider(KSP/SSP)** 와 그 머신이 열거하는 알고리즘을 본다. provider
아키텍처라 JCA와 같은 축을 보되([수용 원칙](../../../docs/runtime-acceptance.md) §2.1), 수집
수단은 하나도 겹치지 않는다 — `/proc`도 ELF도 attach도 아닌 `bcrypt.dll`의 열거 API다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../../docs/regulation.md)의 절 번호다.

## 무엇을 관측하나

| 축 | 어떻게 | 왜 |
|---|---|---|
| **등록 provider** | `BCryptEnumRegisteredProviders` | 능력의 경계가 여기서 정해진다. **순서가 곧 우선순위**라 정렬하지 않는다 |
| **알고리즘** | `BCryptEnumAlgorithms`(전 종류) | 그 머신이 실제로 서비스하는 알고리즘. ML-KEM·ML-DSA가 보이면 PQC 능력의 직접 증거다 |

## 외부 도구에 의존하지 않는다

`certutil`·PowerShell을 부르지 않고 `bcrypt.dll`을 직접 호출한다. 정책으로 스크립트 실행이 막힌
서버가 흔하고, 외부 도구에 기대면 **관측 실패가 환경 탓으로 흩어져** 무엇을 못 봤는지조차
불분명해진다(§2.3). 발자국은 정적 바이너리 하나로 끝난다.

## 정직성 — 못 본 것을 "없다"로 만들지 않는다

- **Windows가 아니면** 빈 결과가 아니라 **갭**을 낸다. 종료코드는 0이다 — 갭이 중앙까지 가야
  "CNG가 없는 노드"와 "CNG를 못 본 노드"가 구별된다(§2.6).
- **열거가 실패하면** 사유를 완전성 노트에 그대로 싣고 계층을 커버로 세지 않는다.
- **봤는데 없었다**면 그건 관측 결과다 — 계층은 커버되고 노트가 "등록된 provider가 없다"로 남는다.

## 계약에 싣는 것

파생 뷰(`CngAxes`)가 지금 담는 것은 **`provider_set` 하나**다. 관측한 알고리즘 목록은 계약에
자리가 없어 파생으로 가지 않고 **원본(`raw_capture`)에만** 실린다 — 관측한 것을 버리지 않으면서,
실물을 재기 전에 계약을 넓히지 않는다. 무엇을 더할지는 실측 뒤에 번호를 부여해 정한다.

| 속성 | 값 |
|---|---|
| `pqcota:crypto_runtime` | `cng` |
| `pqcota:detection_method` | `runtime-introspection` |
| `pqcota:cng.provider_set` | 등록 순서 CSV |

## 지금 어디까지 됐나

| | 상태 |
|---|---|
| 순수 조립·완전성 규칙 + 단위 테스트 | **된다** — Windows 없이 돈다 |
| `bcrypt.dll` 열거 구현 | **쓰여 있다.** 교차 컴파일로 확인했고, 실제 장비 실측은 다음 단계다 |
| provider 활성화·설정 변경(프로비저닝) | **하지 않는다** — 이 collector는 관측까지다 |
