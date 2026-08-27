한국어

# cng-collector: Windows CNG provider 관측

Windows에 **등록된 CNG provider(KSP/SSP)** 와 그 머신이 열거하는 알고리즘을 본다. provider
아키텍처라 JCA와 같은 축을 보되([수용 원칙](../../../docs/runtime-acceptance.md) §2.1), 수집
수단은 하나도 겹치지 않는다. `/proc`도 ELF도 attach도 아닌 `bcrypt.dll`의 열거 API다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../../docs/regulation.md)의 절 번호다.

## 무엇을 관측하나

| 축 | 어떻게 | 왜 |
|---|---|---|
| **등록 provider** | `BCryptEnumRegisteredProviders` | 능력의 경계가 여기서 정해진다. **관측된 순서 그대로** 담는다. 정렬하면 관측을 고치는 것이 된다 |
| **알고리즘** | `BCryptEnumAlgorithms`(전 종류) | 그 머신이 실제로 서비스하는 알고리즘. ML-KEM·ML-DSA가 보이면 PQC 능력의 직접 증거다 |
| **알고리즘→provider** | `BCryptEnumProviders`(알고리즘마다) | 등록 목록은 "무엇이 있나"만 답한다. **누가 ML-DSA를 하나**는 여기에만 있다. 조치 대상을 고르는 근거 |

## 외부 도구에 의존하지 않는다

`certutil`·PowerShell을 부르지 않고 `bcrypt.dll`을 직접 호출한다. 정책으로 스크립트 실행이 막힌
서버가 흔하고, 외부 도구에 기대면 **관측 실패가 환경 탓으로 흩어져** 무엇을 못 봤는지조차
불분명해진다(§2.3). 노드에 올라가는 것은 정적 바이너리 하나로 끝난다.

## 정직성: 못 본 것을 "없다"로 만들지 않는다

- **Windows가 아니면** 빈 결과가 아니라 **갭**을 낸다. 종료코드는 0이다. 갭이 중앙까지 가야
  "CNG가 없는 노드"와 "CNG를 못 본 노드"가 구별된다(§2.6).
- **열거가 실패하면** 사유를 완전성 노트에 그대로 싣고 계층을 커버로 세지 않는다.
- **봤는데 없었다**면 그건 관측 결과다. 계층은 커버되고 노트가 "등록된 provider가 없다"로 남는다.

## 계약에 싣는 것

파생 뷰(`CngAxes`)는 **provider 목록과 알고리즘 목록**을 담는다. 알고리즘은 처음엔 계약에 담을 곳이
없어 원본에만 실었는데, 실측에서 **provider 이름 9개가 전부 Microsoft**라 그것만으로는 "이 노드가
ML-DSA를 할 수 있나"에 답할 수 없다는 것이 드러나 계약에 더했다(순수 additive). 실물을 재고 나서
번호를 부여한다는 규칙을 그대로 따른 것이다.

| 속성 | 값 |
|---|---|
| `pqcota:crypto_runtime` | `cng` |
| `pqcota:detection_method` | `runtime-introspection` |
| `pqcota:cng.provider_set` | 등록 순서 CSV |
| `pqcota:cng.algorithms` | `이름:종류[:provider\|provider]`의 CSV. 종류를 모르면 빈 값, provider를 못 물었으면 셋째 칸이 없다 |

## 실측: Windows 11 Pro 25H2 (빌드 26200, x64)

첫 실물 관측. **provider 9개 · 알고리즘 50개**가 나왔다.

```
Microsoft Key Protection Provider · Microsoft Passport Key Storage Provider ·
Microsoft Platform Crypto Provider · Microsoft Pluton Cryptographic Provider ·
Microsoft Primitive Provider · Microsoft Smart Card Key Storage Provider ·
Microsoft Software Key Storage Provider · Microsoft SSL Protocol Provider ·
Windows Client Key Protection Provider
```

**PQC는 절반만 있다**. 서명 `ML-DSA`는 있고, 키 교환 `ML-KEM`은 **없다**. SLH-DSA도 없다.
이 빌드의 CNG로는 서명은 양자내성으로 갈 수 있어도 TLS 키 교환은 갈 수 없다는 뜻이고,
그것이 이 노드의 사실이다. 다른 빌드로 일반화하지 않는다.

| 종류 | 개수 | 관측된 것 |
|---|---|---|
| hash | 18 | SHA1 · SHA256/384/512 · SHA3-256/384/512 · SHAKE128/256 · CSHAKE128/256 · KMAC128/256 · MD2/4/5 · AES-CMAC · AES-GMAC |
| cipher | 9 | 3DES · 3DES_112 · AES · CHACHA20_POLY1305 · DES · DESX · RC2 · RC4 · XTS-AES |
| signature | 7 | RSA_SIGN · DSA · ECDSA(P256/384/521) · **ML-DSA** |
| key-derivation | 7 | HKDF · PBKDF2 · TLS1_1/1_2_KDF · SP800_108_CTR_HMAC · SP800_56A_CONCAT · CAPI_KDF |
| secret-agreement | 5 | DH · ECDH(P256/384/521): **PQC 없음** |
| rng | 3 | RNG · DUALECRNG · FIPS186DSARNG |
| asymmetric-encryption | 1 | RSA |

**이 실측이 결함을 둘 잡았다.** 처음엔 `BCRYPT_ALGORITHM_IDENTIFIER.dwClass`를 열거 요청의
연산 비트마스크와 같은 어휘로 봤는데, 그것은 **인터페이스 상수**(1–7)다. 값이 겹쳐 50개 중 18개가
빈 종류로 나오고 DH·ECDH는 `secret-agreement`가 아니라 `asymmetric-encryption`으로 **틀리게**
붙었다. 모르는 것을 비우는 규칙이 있어도, 겹치는 값은 오류 없이 틀린다. 매핑을 OS 호출에서 떼어
순수 함수로 옮기고 실측을 근거로 못 박았다(TD-CNG-6). 고친 뒤 재측정에서 **빈 종류가 0개**가 됐고
23개가 바로잡혔다(18개는 비어 있었고 DH·ECDH 5개는 틀린 종류였다). 두 실행의 provider 목록은
같았다. 관측이 흔들려서 달라진 것이 아니라 매핑만 틀렸다는 뜻이다.

둘째는 **노드 식별**이다. 첫 실행의 `derived_from`이 `fqdn`이었다. 지문 수집이 리눅스 경로만
보고 있었다. Windows는 레지스트리 `MachineGuid`를 읽게 고쳤고, 재측정에서 `machine-id`로
바뀌었다. 그 결과 **같은 머신의 node_id도 바뀌었다**(호스트명 기반 → 설치 기반). Windows 노드를
적재한 적이 없어 이행할 것은 없지만, 이름에 매달린 앵커가 어떤 모양으로 새는지가 여기 남는다.

**JCA에서 물려받은 전제 하나가 CNG에서는 서지 않는다.** 알고리즘 50개가 **전부 provider 하나씩**
이었다. 같은 알고리즘을 둘이 서비스하는 경우가 없으니 **우선순위 다툼 자체가 일어나지 않는다.**
그래서 `provider_set`의 순서를 우선순위로 읽지 않는다. 순서를 보존하는 이유는 따로다: 관측한 대로
적기 때문이다. 서드파티 provider(스마트카드 벤더 등)가 깔린 머신에서 둘이 겹치면 그때 순서가
무엇을 뜻하는지 다시 재야 한다. **지금은 확인되지 않았다고 적는다.**

**provider 매핑과 하드웨어 UUID도 실측으로 닫았다.** 알고리즘마다 `BCryptEnumProviders`를 물으면
`ML-DSA`·`ECDH_P256`·`SHA256`이 모두 **`Microsoft Primitive Provider`** 하나를 가리킨다. 등록 목록의
아홉 중 알고리즘을 실제로 서비스하는 것은 그것이라는 뜻이고, 조치 대상을 고를 때 봐야 할 자리다.
`hardware_uuid`는 SMBIOS에서 읽은 값이 **Windows 자신이 보고하는 UUID와 일치**했다. 지문이 하나
늘었어도 `derived_from`은 `machine-id` 그대로라 **node_id는 흔들리지 않았다**(§1.4 우선순위).

## 지금 어디까지 됐나

| | 상태 |
|---|---|
| 순수 조립·완전성 규칙 + 단위 테스트 | **된다**. Windows 없이 돈다 |
| `bcrypt.dll` 열거 | **된다**. Windows 11 26200에서 실측(위) |
| 노드 식별 | **된다**. `MachineGuid`(설치 단위)와 SMBIOS `hardware_uuid`(하드웨어 단위) 둘 다 실측 확인. 표기는 리눅스 `product_uuid`와 같다 |
| 정규화 → 파생 뷰 | **된다**. provider·알고리즘이 `CngAxes`까지 온다(실측 값으로 못 박음) |
| 적재 → 인벤토리 조회 | **된다**. 실제 장비 결과를 Postgres에 적재해 뷰까지 확인(TD-CNG-9). 화면에 이렇게 나온다:<br>`win_cng  confirmed  runtime_introspection  providers=9 algorithms=50 네이티브(서명만: KEM 미관측)` |
| provider 활성화·설정 변경(프로비저닝) | **하지 않는다**. 이 collector는 관측까지다 |
