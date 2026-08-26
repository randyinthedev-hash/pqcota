한국어 · [English](runtime-acceptance.en.md)

# 암호 런타임 수용 원칙

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

---

## 1. 원칙

플랫폼의 대상은 특정 라이브러리가 아니라 **"암호 provider를 갖는 런타임"**이라는 추상이다.

- **프로세스 규정(3단계, AUTO/PROPOSE/MANUAL)은 런타임 무관하게 불변**이다.
- 런타임별로 달라지는 것은 넷이다. **(a) 디스커버리 수집 방법, (b) 버전·provider 축 스키마,
  (c) remediation taxonomy 분기, (d) 프로비저닝 substrate.**
- 모든 finding·자산에 **`crypto_runtime`을 1급 필드**로 둔다. 이 필드가 각 단계의 런타임 분기를 결정한다.

## 2. 수용된 둘: OpenSSL · JCA

두 런타임은 "provider로 알고리즘 능력을 주입한다"는 점에서 **개념적으로 동형**이다. 이 동형성이 "버전 안 올리고 내부 provider 주입" 전략을 양쪽에 성립시킨다.

### 2.1 provider 동형성

| | OpenSSL | JCA/JCE |
|---|---|---|
| 확장점 | Provider (`.so`), 3.x provider API | Security Provider (JAR) |
| 활성화 | `openssl.cnf` provider 섹션 | `java.security` `security.provider.N=` |
| 동적 주입 | dlopen/config | `Security.addProvider()` / `insertProviderAt()` |
| 알고리즘 요청 | EVP 고수준 API | `Cipher/Signature/KeyPairGenerator.getInstance()` |
| PQC provider 선례 | oqs-provider, OpenSSL 3.5 네이티브 | BouncyCastle(ML-KEM/ML-DSA/SLH-DSA), 최신 JDK 네이티브 |
| 내부 provider 주입 | ✅ 가능 | ✅ 가능 |

### 2.2 런타임별 근본 차이 (분기 근거)

**OpenSSL: 버전이 provider 가능 여부를 가른다**
- 3.0+ : provider API 존재 → 파일 배치 + `openssl.cnf` 활성화로 재빌드 없이 주입
- 3.5+ : ML-KEM/ML-DSA/SLH-DSA 네이티브 + TLS 하이브리드(X25519MLKEM768) 기본 → config만
- 1.1.1 / 1.0.2 : **provider 아키텍처 없음**(ENGINE만) → 로더블 모듈로 TLS PQC 불가, 포크 교체 또는 프록시
- 버전 축: `lib + version + fork`(OpenSSL/BoringSSL/LibreSSL/AWS-LC이며, 동일 soname 문제가 여기서 나온다)

**JCA/JCE: provider 등록 메커니즘과 이원 버전 축이 관건**
- 등록 층위: (a) `java.security` 정적 순서 목록(JRE 전역), (b) `addProvider()` 런타임 동적 주입(**코드에 숨어 파일 스캔 불가**), (c) `getInstance("...","BC")` 명시 지목(java.security 변경 무효), (d) **우선순위 협상**(목록상 앞선 provider가 같은 알고리즘을 먼저 서비스하면 새 provider 무시)
- **이원 버전 축**: `{jdk_vendor, jdk_version}` × `{provider_set}`. `pqc_readiness = "JDK 네이티브 지원" ∨ "provider 보강"`의 논리합
- `jdk.tls.disabledAlgorithms` 등 정책이 실제 등급을 좌우

### 2.3 무엇을 어떻게 보나 (런타임별 탐지 분기)

**OpenSSL**
- 파일시스템/패키지: `libssl/libcrypto` 실물, `ldd`/`readelf` NEEDED, 패키지 역의존성, **정적 바이너리 문자열 시그니처**(fork·버전 판별)
- 프로세스/런타임: `/proc/*/maps`·`lsof`·`ss` (dlopen·벤더링 포착, 배치 누락 방지 위해 시간대별 반복)
- 네트워크(보조): 로컬 TLS 등급

**JCA/JCE**
- 아티팩트: JAR/WAR/EAR 내 provider JAR(`bcprov-*` 등) + Maven/Gradle **의존성 그래프** 파싱
- 정책 파싱: `java.security` 등록 순서 + `jdk.tls.*` + `disabledAlgorithms`
- 런타임 인트로스펙션(ground truth): 실행 중 JVM에 attach → `getProviders()`·로드된 provider 체인 조회 (정적으로 관측되지 않는 동적 등록·명시 지목 포착)
- 동적 등록 사각지대: `addProvider()`는 바이트코드/소스 호출지점 분석 또는 실행 중 조회로만

**provider 시그니처 레지스트리**: crypto-registry에 아래 provider 시그니처를 등록해 디스커버리가 버전·FIPS·알고리즘 커버리지를 자동 판정한다. 각 provider가 서로 다른 `pqc_readiness`·`fips_validation`·알고리즘 커버리지를 함의한다.

| provider JAR/모듈 시그니처 | 성격 | PQC 커버리지 | FIPS |
|---|---|---|---|
| `bcprov-jdk18on-*` (BouncyCastle 1.79+) | 순수 자바, 전 JDK(1.8+) | ML-KEM/ML-DSA/**SLH-DSA** | 미검증(표준판) |
| BC-FJA (FIPS 변형) | FIPS 140-3 인증(네이티브 가속) | ML-KEM/ML-DSA/SLH-DSA | **140-3** |
| JDK 네이티브(24/25+, SunJCE 확장) | 런타임 내장 | ML-KEM/ML-DSA **only** (SLH-DSA 없음) | JDK별 |
| openssl-jostle (JNI 브릿지) | 네이티브 OpenSSL을 JCA로 노출 | ML-KEM/ML-DSA/SLH-DSA | OpenSSL 모듈 따름 |
| 내부 PQC provider | 자체 | 정의 대상 | 미검증 |

**규정**: SLH-DSA는 JDK 네이티브에 없으므로 SLH-DSA 필요 자산은 JDK 버전 무관하게 BC(또는 jostle) 의존으로 태깅한다.

이 표는 **초기 시드**다. 살아 있는 출처는 `pkg/kernel/registry`이고, provider가 릴리스될 때마다
그쪽이 먼저 바뀐다. 여기 적힌 버전과 어긋나면 코드가 옳다.

### 2.4 스키마 반영 규칙

- `crypto_runtime`: `openssl` | `jca`
- OpenSSL: `lib`, `version`, `fork`, `binding_mode`(dynamic/static/dlopen/vendored)
- JCA: `jdk_vendor`, `jdk_version`, `provider_set`(등록 순서 포함), `registration_mode`(static/dynamic/explicit)
- 공통: `usage_context`(server/client/at-rest/signing), `pqc_readiness`, `fips_validation`, `remediation_class`, **`evidence_strength`**, **`detection_method`**

**축 (d) substrate는 아직 스키마로 서 있지 않다.** 배치 경로·config 위치는 필드가 아니라 openssl/jca
**2-way 분기**로 갈린다. 둘 다 POSIX 파일이라 이 축이 지금까지 드러나지 않았다. 레지스트리·GPO를
쓰는 런타임은 그 2-way로 표현되지 않으므로, 일반화는 해당 런타임을 구현할 때 함께 한다.

---

---

## 3. 수용 조건: 축 넷이 말하지 않는 가정 셋

축 넷은 **무엇이 달라지는가**를 말하지만, 코드는 그 넷이 말하지 않는 것 셋을 더 요구한다.
후보가 걸리는 자리는 언제나 축이 아니라 이 조건이다.

1. **provider 동형성**: "provider로 알고리즘 능력을 주입한다"가 성립해야 `PROVIDER_INJECT`·
   `CONFIG_ONLY`가 의미를 갖는다. 없으면 remediation은 `REBUILD` 한 방향으로 붕괴한다.
   (OpenSSL·JCA가 어떻게 동형인지는 [§2.1](#21-provider-동형성) 표.)
2. **POSIX 파일 substrate**: 아티팩트가 파일이고, 경로에 스테이징되고, 제거로 가역이다
   (`ModulePath` → `/opt/pqcota/*.so|*.jar`, Ansible `copy`, `state: absent`).
3. **관측 distinct**: 기존 collector가 이미 보는 것과 **다른 것**을 봐야 한다. 아니면 새 런타임이
   아니라 기존 런타임의 중복이다.

**조건 2가 (d) substrate 축의 실체다.** openssl·jca가 **둘 다 POSIX 파일**이라 이 축은 오래
`jca bool` 2-way 스위치 뒤에 가려져 있었다. 축으로 세우기 전에는 요구인 줄도 몰랐다.

---

## 4. 판정하는 법

### 4.1 결정 트리

새 후보를 만났을 때 순서대로 묻는다. 하나라도 "아니오"면 새 1급 런타임이 아니다.

1. **관측이 기존 collector가 이미 보는 것과 다른가?**
   아니오 → **흡수**: 기존 런타임 finding에 `app_keys`·`usage_context`로 붙인다(새 enum 금지). *예: Python.*
2. **remediation이 provider 주입·config로 표현되나, 아니면 `REBUILD`뿐인가?**
   `REBUILD`뿐 → **taxonomy 흡수**, 새 render 불필요. *예: Go(조건 1).*
3. **배포 아티팩트가 POSIX 파일 + 경로 스테이징 + 파일 제거 롤백에 맞나?**
   아니오 → **substrate 일반화가 선행**돼야 하는 별도 트랙. *예: Windows(조건 2).*
4. **remediation이 기존 openssl/jca provider를 어떤 *target*으로 가리키는 것인가?**
   예 → 새 enum 아님, **backend target + 새 디스커버리 axis**. *예: HSM.*

넷을 다 통과(관측 distinct + provider-주입형 remediation + POSIX 파일 아티팩트 + 자체 provider 모델)해야
비로소 진짜 새 1급 `CryptoRuntime`이다.

### 4.2 수용하기로 했다면: 건드릴 곳

| 층 | 파일 | 최소 산출 |
|---|---|---|
| 계약 | `contracts/.../common.proto`(enum), `.../cbom.proto`(oneof `XxxAxes`) | 순수 additive: `make breaking`이 릴리스한 계약과 대조한다 |
| 수집 | `discovery/collectors/<r>/` | `CollectionResult` emit + `detection_method` 표기 |
| 정규화 | `pkg/discovery/normalize/`(강화 단계) | `detection_method`→`evidence_strength` 파생 |
| 프로비저닝 | `pkg/provisioning/render.go`(분기) + `renderXxx` + `paths.go` | render + stage + **롤백 대칭** |

각 층 최소 테스트 1. **흡수는 은폐가 아니다**: Python을 openssl로 흡수해도 뷰엔 "이 Python 앱이 쓴다"로
보이고, Go를 `REBUILD`로 흡수해도 "정적이라 재빌드 필요"로 정직히 고지된다. 안 보이는 걸
없다고 하지 않는다.

**(a) 수집 · (b) 스키마 oneof · (c) taxonomy 어휘는 확장점으로 설계돼 있어** 새 런타임을 코어 변경
없이 받는다. **버티지 않는 유일한 층은 (d) substrate의 "POSIX 파일" 가정**이고, non-POSIX 런타임만
그것을 건드린다.
