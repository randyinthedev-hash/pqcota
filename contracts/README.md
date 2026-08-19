한국어 · [English](README.en.md)

# pqcota/contracts — 계약 SSOT (Single Source of Truth)

이 디렉터리는 PQC 마이그레이션 플랫폼의 **모든 컴포넌트가 의존하는 유일 계약**이다

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

근거 문서:
- [**데이터 모델 스키마**](data-model.md) — 전 메시지·enum의 목적·핵심 필드·관계 지도(사람용 레퍼런스). 아래 파일 목록보다 먼저 보면 전체 그림이 잡힌다.
- [규정서](../docs/regulation.md)
- [아키텍처 & OSS 경계 설계](../docs/architecture.md)

## 파일

네임스페이스는 **제품 3단계 + 공유 어휘**로 나뉜다(`pkg/`와 대칭). 네임스페이스만 봐도 어느 단계 계약인지 안다.

| 파일 | 패키지 | 정의 | 규정서 근거 |
|---|---|---|---|
| `proto/pqcota/common/v1/common.proto` | `pqcota.common.v1` | 공유 어휘: Envelope·완전성·통제 어휘 enum (전 단계 가로지름) | 수용 원칙 §2.4, §2.4, §2.7, §3.1 |
| `proto/pqcota/discovery/v1/cbom.proto` | `pqcota.discovery.v1` | 파생 Finding · OpensslAxes · JcaAxes | 수용 원칙 §2.4, §2.4, §3.2 |
| `proto/pqcota/discovery/v1/collector.proto` | `pqcota.discovery.v1` | Collector intake gRPC 서비스 · CollectionResult | §1.6 |
| `proto/pqcota/discovery/v1/edge.proto` | `pqcota.discovery.v1` | 통신 엣지 관측 · ObservedEdge · QuantumPosture | 인벤토리 설계 §6 |
| `proto/pqcota/discovery/v1/asset.proto` | `pqcota.discovery.v1` | 자산 계층 · Application · ProcessMatch · LiveProcess (Machine→App→Process) | §1.4, §2 |
| `proto/pqcota/inventory/v1/decision.proto` | `pqcota.inventory.v1` | 리뷰 판정 · Decision · DecisionStatus · DecisionConclusion | §2, §3.3③, §3.6 |
| `proto/pqcota/inventory/v1/machine.proto` | `pqcota.inventory.v1` | 머신 프로필(사람-대면) · MachineProfile · Environment (표시명·환경·역할·소유자·태그) | §3 |
| `proto/pqcota/provisioning/v1/plan.proto` | `pqcota.provisioning.v1` | 확정 계획 · FinalizedPlan · RemediationAction · RemediationKind · DeployAutomationLevel | 규정서 §4.1·§3.7 · 프로비저닝 설계 §4.1·§4.2 |
| `proto/pqcota/provisioning/v1/rollback.proto` | `pqcota.provisioning.v1` | 프로비저닝 히스토리·롤백 · ProvisioningRecord · CryptoState (before/after) | §1.3, §4.3 |

> `common`은 pkg/kernel의 계약판 — 한 단계에 속하지 않는 공유 어휘(CryptoRuntime·DetectionMethod·Envelope·Completeness 등)만. 생성 Go 패키지: `gen/pqcota/{common,discovery,inventory,provisioning}/v1` → `commonv1`·`discoveryv1`·`inventoryv1`·`provisioningv1`. `Decision`·`FinalizedPlan` **스키마는 SSOT**(소비자 엔진이 같은 어휘를 쓰도록).

## 소비자가 쓰는 법

**생성 코드(`gen/`)를 커밋해 둔다.** 계약이 SSOT라는 말이 성립하려면 그 코드가 받아지는
자리에 있어야 한다 — 소비자에게 `buf`와 protoc 플러그인을 깔라고 요구하면 계약이 아니라
빌드 절차를 나눠 갖는 셈이다.

```go
import (
	commonv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/common/v1"
	discoveryv1 "github.com/randyinthedev-hash/pqcota/gen/pqcota/discovery/v1"
)
```

> **v0.5.0에서 모듈 경로가 바뀌었다** — `github.com/pqcota/pqcota` → `github.com/randyinthedev-hash/pqcota`.
> 선언한 경로가 리포 주소와 달라 `go get`이 해소하지 못했고, 소비자는 `replace`로 우회해야 했다.
> v0.4.0 이하를 쓰던 쪽은 그 줄을 지우고 import를 새 경로로 옮긴다.

생성 코드는 손으로 고치지 않는다. proto를 고치고 `make generate`를 돌린다 — CI가 둘이
어긋나는지 매 변경 검사한다.

## 핵심 설계 결정 (읽고 시작할 것)

### 1. 책임 경계 — Collector는 강화하지 않는다

```
Collector  →  §2.4 step 1~2 : 원시 포집(raw_capture) + 표준 CycloneDX 변환 + Envelope
정규화     →  §2.4 step 3~6 : 강화 → 검증 → 동일성해소 → 영속화 → 파생 Finding[]
```

- Collector 출력(`CollectionResult`)에는 **`Finding`이 없다.** 표준 CycloneDX + Envelope만 반환한다.
- `evidence_strength`·`pqc_readiness`·`fork 판별` 등 **해석적 강화는 코어가 단독 수행**한다.
  이유(규정서 §1.2·§2.4): 강화 규칙(매핑 테이블)이 개선되면 **원본에서 재계산**해야 하는데,
  강화가 collector마다 흩어져 있으면 재계산이 불가능하고 규칙이 어긋난다. 강화는 한 곳에.
- 그래서 `Finding.derived_from_snapshot_id` + `ruleset_version`이 필수 — 어떤 원본·어떤 규칙으로
  파생됐는지 항상 추적되어 재현 가능(§1.2 감사 무결성).

### 2. `unknown`은 1급 값 (규정서 §2.5)

모든 enum의 `*_UNSPECIFIED = 0`은 "판별 불가 = unknown"을 의미한다.
채우지 못한 필드를 빈칸/누락으로 두지 말고 **명시적 0값**으로 남긴다.
자동 "부재" 처리 금지 — "실제 없음"과 "원리상 관측하지 못함"은 `Completeness.layers_missing`으로 구분한다.

### 3. provider 시그니처 레지스트리가 강화를 구동 (수용 원칙 §2.3)

`JcaAxes.provider_set`(등록 순서 포함)은 Collector가 관측한 **원본**이다.
코어 강화 단계가 이 값을 provider 시그니처 레지스트리와 대조해 `pqc_readiness`·`fips_validation`·
알고리즘 커버리지를 **파생**한다(BouncyCastle/BC-FJA/JDK-native/openssl-jostle/내부 식별).
- **SLH-DSA는 JDK 네이티브에 없다** → 필요 자산은 JDK 버전 무관하게 BC/jostle 의존으로 태깅.
- `fips_validation` 요구는 Deploy 단계에서 **FIPS 검증 provider를 쓰라는 권고로 나온다**(FIPS 라우팅). 계획이 고른 provider를 도구가 막지는 않는다 — 검증서는 빌드 단위라 파일만 봐서 알 수 없다.
- 레지스트리는 파생 규칙이므로 `ruleset_version`으로 고정, 개선 시 원본에서 재계산(§1.2).

### 4. `deploy_automation_level`은 Discovery가 아니라 계획 속성 (규정서 §4.3 v4)

`DeployAutomationLevel`(L1/L2/L3)은 통제 어휘로 여기 등재하되 **Collector가 채우지 않는다.**
리뷰어가 자산별로 판정하는 계획·자산 속성(MANUAL)이며 확정 계획(plan) 엔티티에 실린다(워크플로는 계획 쪽에 있다).
Discovery `Finding`에는 이 필드가 없다 — 단계 혼선을 막기 위한 의도적 분리.

### 5. gRPC 경계 = GPL 전염 차단 경계 ([라이선스 정리](../docs/licensing.md))

`Collector` 서비스 경계는 곧 라이선스 격리 경계다.
GPL collector(CipherIQ `cbom-generator` 등)는 **별도 프로세스**로 실행되고
**stdout CycloneDX만 교환**한다. 코어는 라이브러리로 링크하지 않는다.
`CollectorCapabilities.license`로 사용자에게 함의를 표기한다([라이선스 정리](../docs/licensing.md)).

### 6. 계획 스키마는 공개 계약 (규정서 §4.1)

`plan.proto`(`FinalizedPlan`·`RemediationAction`·`RemediationKind`)는 SSOT라 **OSS**다 — 프로비저닝의
아티팩트 생성기(`pkg/provisioning`)와 실행 채널이 같은 어휘를 써야 하기 때문.
- **`Executable()` 게이트**(§3.7 "finalized만 실행 근거")는 공유 계약 규칙 → OSS `pkg/provisioning`.
- **계획 저작·리뷰-확정**(§3.3③)과 **플릿 오케스트레이션**(§4.3)은 하지 않는다.
- taxonomy(`RemediationKind`) → config 조각 생성은 결정론적 파생(§1.2)이라 OSS 생성기가 담당.
> `DeployAutomationLevel`·계획 엔티티의 소관은 *워크플로 소유*를 뜻하며 *스키마 위치*가 아니다.
> 상세: [프로비저닝 설계](../provisioning/design.md).

## CycloneDX `properties` 확장 키 규약 (§3.2)

도구 고유 enrichment는 표준 CycloneDX `properties`에 `pqcota:` 네임스페이스로 싣는다.
코어 파이프라인이 이 키를 읽어 타입드 `Finding`으로 매핑한다.

| property key | 값 | 대응 Finding 필드 |
|---|---|---|
| `pqcota:crypto_runtime` | `openssl` \| `jca` \| `cng` | `crypto_runtime` |
| `pqcota:detection_method` | `source`\|`artifact`\|`symbol-analysis`\|`runtime-introspection`\|`dynamic-trace` | `detection_method` — **어떻게 봤나**. 강도는 여기서 파생된다(실물을 본 것이 추론보다 강하다) |
| `pqcota:usage_context` | `server`\|`client`\|`at-rest`\|`signing` | `usage_context` |
| `pqcota:openssl.fork` | `OpenSSL`\|`BoringSSL`\|… | `openssl.fork` |
| `pqcota:openssl.binding_mode` | `dynamic`\|`static`\|`dlopen`\|`vendored` | `openssl.binding_mode` |
| `pqcota:jca.provider_set` | 등록 순서 CSV | `jca.provider_set` |
| `pqcota:jca.registration_mode` | `static`\|`dynamic`\|`explicit` | `jca.registration_mode` |
| `pqcota:cng.provider_set` | 등록 순서 CSV | `cng.provider_set` — Windows CNG. JCA와 같이 **순서 유의미**(우선순위) |
| `pqcota:app_keys` | 앱 키 CSV(공유 .so는 다중) | `app_keys`(repeated) — 자산이 어느 앱 것인지(§1.5) |

> `evidence_strength`·`pqc_readiness`는 **여기 넣지 않는다** — 코어 파생 값이다(위 결정 1).

> **외부 collector 주의 — `pqcota:detection_method`를 반드시 실어라.** `evidence_strength`는 코어가
> `detection_method`에서 결정론적으로 파생한다(§2.3 표). 이 키가 없으면 코어는 증거 강도를 지어내지
> 않고 `UNSPECIFIED`로 정직히 떨어뜨린다(§2.5 — unknown 1급, 추측 금지). 즉 **빠뜨린 벌점이 아니라
> 규정된 결과**다. CBOMkit 등 표준 CycloneDX만 내는 collector는 그 `cryptoProperties`를 이 키로
> 매핑하는 import 어댑터를 거쳐야 강도가 온전히 산다 — 매핑 없이 들어온 자산은 강도 미상으로 남는다.

## 버전·호환성 규칙

- 패키지 `pqcota.{common,discovery,inventory,provisioning}.v1`. **호환 파괴 변경은 `v2` 신설** — `v1` 필드 재사용/의미 변경 금지.
- 필드 삭제 시 번호를 `reserved` 처리. enum 값 추가는 하위호환(항상 끝에 추가).
- 이 계약은 이 리포(Apache-2.0)에 속한다 — 정규화된 CBOM 스키마·프로파일은 공개(§5.1).

## 계약을 바꿀 때 — 파급 점검

proto만 고치고 끝나지 않는다. **계약에서 파생된 두 가지가 코드에 있고, 둘 다 잊으면 조용히 깨진다.**

| 함께 볼 것 | 언제 | 잊으면 |
|---|---|---|
| [`sign.Canonical`](../pkg/kernel/sign) | `CollectionResult`·`Envelope`·`MachineIdentity`·`Completeness`·`ObservedEdge`에 **필드 추가** | 새 필드가 **서명 사각지대**가 된다 — 변조해도 검증이 통과.<br>범위를 넓히면 **기존 서명은 전부 무효**가 되므로 릴리스 후엔 마이그레이션 필요 |
| [`history.ContentHash`](../pkg/discovery/history) | `Finding`·`ObservedEdge`·`Completeness`에 **실질 내용 필드 추가** | 그 필드가 바뀌어도 "변화 없음"으로 접혀 **이력에서 조용히 사라진다**([인벤토리 설계 §7.3](../inventory/design.md)) |

둘 다 **테스트가 지켜본다** — 필드 수가 바뀌면 `TestCanonicalCoversAllFields`가 실패하며 무엇을 해야 하는지 알려준다. 실패를 기대값 수정만으로 넘기지 말 것. 그게 바로 사각지대를 만드는 경로다.

**판정 규칙을 바꿨다면** `ruleset_version`을 올리고, 과거 스냅샷은 원본에서 **재계산**해 새 판정을 얻는다(§1.2). 파생값은 저장된 값이 아니라 규칙의 함수다.

**순서 자체가 뜻인 필드**에 주의한다 — `JcaAxes.provider_set`은 등록 순서가 우선순위 협상을 결정하므로(수용 원칙 §2.2) 정렬·정규화 대상이 아니다.

## 코드 생성

```bash
# 생성물은 커밋하지 않는다 — 계약이 바뀌면 각자 다시 만든다
make generate                              # = cd contracts && buf generate
cd contracts && buf lint                   # 또는 make lint

# 호환성 검사는 **리포 루트에서** — .git이 여기 있고 proto는 contracts/ 아래라 subdir이 필요하다
buf breaking contracts --against '.git#branch=main,subdir=contracts'
buf breaking contracts --against '.git#ref=HEAD~1,subdir=contracts'   # 직전 커밋 대비
```
