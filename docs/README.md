한국어 · [English](README.en.md)

# docs/: 설계·기여자(개발자) 문서

> **이 폴더는 개발자·포크·기여자용**이다. 플랫폼을 *써보려는* 사용자는 루트 [README의 "써보기"](../README.md#써보기-데모)와 [demo/](../demo/)부터 보면 된다.

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

> **설계를 파기 전에** 리포 루트의 [여정](../journey.md)을 한 번 훑으면 좋다. 준비부터 관측·적재·조회·
> 생성·적용까지 어떤 순서로 무엇이 나오는지만 보는 문서라, 여기 있는 것들이 무엇을 정하는 문서인지
> 감이 잡힌다. **쓰는 사람을 향한 문서라 이 폴더에 두지 않는다.**

플랫폼의 "무엇을·어떤 규칙으로(WHAT)"와 "어떤 모듈·계약으로(HOW)"를 고정하는 근거 문서들. 코드의 `§` 참조는 여기를 가리킨다.

## 읽는 순서

```
플랫폼 규정 ──┬── 아키텍처        (단계를 가로지르는 모듈·계약·스키마)
              └── 단계별 설계 3    (그 단계의 컴포넌트·데이터·흐름)
```

**규정부터 읽는다.** 규칙이 먼저 서고, 설계는 그것을 무엇으로 구현하는지 답한다. 설계가 규정을
인용하지 규정은 설계를 모른다. 아키텍처와 단계별 설계는 **나란히** 있다(둘 다 규정을 직접 인용).

| 순서 | 문서 | 답하는 질문 |
|---|---|---|
| **1** | [규정서](regulation.md) | 무엇을 해야/하지 말아야 하나 |
| **2** | [아키텍처](architecture.md) | 그것을 어떤 모듈·계약·스키마로 |
| **2** | [단계별 설계](#서브시스템-설계-3단계) 셋 | 그 단계는 어떻게 |

곁가지: [암호 런타임 수용 원칙](runtime-acceptance.md)(무엇을 런타임으로 받나) ·
[라이선스 정리](licensing.md)(무엇을 쓰고 무엇이 격리를 강제하나) ·
[검토 중인 설계](under-review.md)(아직 안 정한 것).

> **한눈에 보기**: [플랫폼 구조와 이해관계자](https://randyinthedev-hash.github.io/pqcota/architectures/platform-structure.html)를 본다. 누가 무엇을 주고, 플랫폼이 무엇을 내고, 실행은 어디서 일어나는지가 한 장에 있다.

## 규정·경계 (먼저 읽기)
| 문서 | 내용 |
|---|---|
| [규정서](regulation.md) | **무엇을 해야/하지 말아야 하나.** 관통 원칙 · 단계 규정 · 자동화 경계(AUTO/PROPOSE/MANUAL) · 단계 간 핸드오프 계약을 담는다. 코드 §참조의 원본이다 |
| [아키텍처](architecture.md) | 규정을 **어떤 모듈·계약·스키마로** 구현하나. 기술 스택 · 시스템 구성 · 정규화 CBOM Envelope · intake 계약 · 리포 구조를 다룬다 |
| [라이선스 정리](licensing.md) | 무엇을 어떤 라이선스로 쓰나(소비 형태별) + **카피레프트 격리를 무엇이 강제하나** |

## 서브시스템 설계 (3단계)

> 각 단계의 **개념 개요**는 폴더 README에서 시작한다([discovery/](../discovery/README.md) · [inventory/](../inventory/README.md) · [provisioning/](../provisioning/README.md)). 아래는 그 상세 설계 문서다.

| 문서 | 단계 |
|---|---|
| [디스커버리 설계](../discovery/design.md) · [collector 배포](../discovery/collector-deployment.md) · [위임 수신](../inventory/cbom-intake.md) | Discovery: collector(직접 관측)·CBOM 위임 수신·파이프라인·SD-1–SV-2·자산 모델 |
| [인벤토리 설계](../inventory/design.md) | Inventory: 머신 메타데이터(엔드포인트·프로필) 저장소·중앙 뷰(앱 표시) [이 리포] |
| [프로비저닝 설계](../provisioning/design.md) | Provisioning: 계획 게이트·taxonomy→아티팩트·L1/L2/L3 플레이북 생성(L3은 계획의 `activation` 훅으로 활성화·재시작)·before 롤백 레코드 |

## 테스트·검증
| 문서 | 내용 |
|---|---|
| [**테스트 명세 지도**](test-map.md) | 케이스가 어디 적혀 있고 어디서 도나. 그룹→코드 대응, 레벨 분포, 미검증 셋을 담는다. **여기서 시작한다** |
| [커널 테스트케이스](kernel-testcases.md) | 단계를 가로지르는 **파생 규칙**이다. 증거 강도·정규화·등급·조치 taxonomy·앱 표시를 다룬다 |
| [디스커버리 테스트케이스](../discovery/testcases.md) · [인벤토리 테스트케이스](../inventory/testcases.md) · [프로비저닝 테스트케이스](../provisioning/testcases.md) | 상황별 인수 기준 + 구현 순서 (TDD) |
| [데모가 검증하는 것](../demo/integration-verification.md) | 실물이 필요한 케이스를 데모 6단계가 맡는다. 덮지 않는 것도 적어 둔다 |
| [**관리체계**](governance.md) | 게이트·기록·수명을 한자리에. 무엇이 자동으로 막히고, 무엇을 기록하며, 결정이 어떻게 사는지 |

## 계약·데이터 모델
| 문서 | 내용 |
|---|---|
| [**데이터 모델 스키마**](../contracts/data-model.md) | 규격한 전 메시지·enum의 사람용 지도다. 목적·핵심 필드·관계·provenance 레인을 적는다. contracts SSOT 레퍼런스다 |
| [contracts/README](../contracts/README.md) | protobuf 파일·네임스페이스 목록 + CycloneDX property 매핑 |
| [검토 중인 설계](under-review.md) | 로드맵에 있으나 **구현을 확정하지 않은 것**의 설계다. provider 수용(만들 config 어휘)·provider 관측·HSM 축, 그리고 netcap 설계 셋(서버 역할 엣지·엣지 상대의 스코프 판정·통신 관측 주기)을 다룬다. 확정되면 본 설계 문서로 옮기고 여기서 지운다 |
| [암호 런타임 수용 원칙](runtime-acceptance.md) | 무엇을 1급 암호 런타임으로 받나. 수용된 둘(OpenSSL·JCA)이 왜 동형인지, 축 넷이 말하지 않는 수용 조건 셋, 새 후보를 만났을 때의 결정 트리를 담는다 |

[`contracts/`](../contracts/)는 protobuf SSOT다. 네임스페이스가 곧 단계다.

---
빌드·테스트·기여 워크플로: [CONTRIBUTING.md](../CONTRIBUTING.md).
