한국어 · [English](data-model.en.md)

# 데이터 모델 스키마 (contracts SSOT 레퍼런스)

계약 파일·네임스페이스 목록과 CycloneDX property 매핑은 [contracts/README](README.md).

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

## 0. 규약

- **네임스페이스 = 단계**: `pqcota.common.v1`(공유) · `pqcota.discovery.v1` · `pqcota.inventory.v1` · `pqcota.provisioning.v1`. 생성 Go 패키지는 `commonv1`·`discoveryv1`·`inventoryv1`·`provisioningv1`.
- **protojson 표현**: 필드는 **camelCase**(`target_node_id`→`targetNodeId`), `bytes`는 **base64 문자열**(`cbom_cyclonedx`), enum은 **이름 문자열**(`"DETECTION_METHOD_RUNTIME_INTROSPECTION"`), `Timestamp`는 RFC3339.
- **enum 0 = `*_UNSPECIFIED` = "unknown"**(§2.5). "없음"이 아니라 "판별 못 함" — 완전성 맵과 함께 "실제 없음"과 "원리상 관측하지 못함"을 가른다.
- **하위호환**: 필드 번호 재사용 금지, 삭제는 `reserved`, enum 값은 끝에만 추가, 파괴 변경은 `v2` 신설.

## 1. 모델을 관통하는 4가지 원칙

이 원칙들이 왜 필드가 그렇게 갈리는지를 설명한다.

1. **Provenance 레인 분리(§1.2/§1.3)** — 데이터가 어디서 왔는지로 레인을 나눈다. 섞지 않는다.
   - **관측(observed)**: collector가 실제로 본 것 — `CollectionResult`·`ObservedEdge`·`MachineIdentity`.
   - **선언(declared)**: CMDB/사용자가 채운 것 — `MachineProfile`·스코프 마스터·선언 엣지.
   - **파생(derived)**: core가 관측에서 **재계산**하는 뷰 — `Finding`·`evidence_strength`·`QuantumPosture`. **collector 출력엔 없다.** 원본(raw)에서 항상 재생성 가능(재현성).
   - **행위(action)**: 도구가 한 일의 append-only 이력 — `ProvisioningRecord`·`Decision`.
2. **식별 모델(§1.4)** — 세 층. **권위** = `node_id`(스코프 마스터/CMDB, 안정·전역 유일). **상관** = `MachineIdentity` 지문(machine-id·hw-uuid·cloud-id·fqdn — node_id 검증·CMDB 없을 때 self-id 파생). **로케이터** = IP(ID 아님, 네트워크 관측을 노드로 잇는 데만).
3. **파생 뷰 재현성(§1.2)** — `raw_capture`(불변 원본)에서 강화 규칙으로 파생물을 만든다. 그래서 파생 메시지엔 `derived_from_snapshot_id`·`ruleset_version`이 붙는다(어떤 원본·어떤 규칙으로 재현되는지).
4. **비밀 미영속(§1.5)** — 접근 비밀(SSH 키·비밀번호·계정)은 **어떤 스키마에도 필드가 없다**. `MachineEndpoint`가 대표 — 타입상 비밀을 담을 수 없어 컴파일 타임에 보장된다.

---

## 2. `common.v1` — 공유 어휘 (단계 가로지름)

### 통제 어휘 (enum) — 전 단계 공유
| enum | 뜻 | 값(0=UNSPECIFIED 생략) |
|---|---|---|
| `CryptoRuntime` | 암호 런타임 — 무엇을 받는지는 [수용 원칙](../docs/runtime-acceptance.md). 모든 finding·자산·조치의 1급 분기 | `OPENSSL` · `JCA` · `WIN_CNG` |
| `DetectionMethod` | 탐지 방법. collector가 신고 → evidence 파생 근거 | `SOURCE`·`ARTIFACT`·`SYMBOL_ANALYSIS`·`RUNTIME_INTROSPECTION`·`DYNAMIC_TRACE` |
| `EvidenceStrength` | 증거 강도. **detection_method에서 파생**(core만 채움) | `CONFIRMED`·`INFERRED_HIGH`·`INFERRED_LOW` |
| `UsageContext` | 사용 맥락 | `SERVER`·`CLIENT`·`AT_REST`·`SIGNING` |
| `CollectionLayer` | 수집 계층(완전성 맵 단위) | `SOURCE`·`ARTIFACT`·`PROCESS`·`NETWORK`·`JVM_INTROSPECTION` |
| `OpensslBindingMode` | OpenSSL 바인딩 | `DYNAMIC`·`STATIC`·`DLOPEN`·`VENDORED` |
| `JcaRegistrationMode` | JCA provider 등록 | `STATIC`·`DYNAMIC`·`EXPLICIT` |

### 메시지
| 메시지 | 목적 | 핵심 필드 |
|---|---|---|
| **`Envelope`** | 모든 수집 산출물에 붙는 provenance(§3.1) | `collector_id`·`detection_method`·`target_node_id`(권위 앵커)·`scope_master_ref`·`signature`(ed25519)·`collector_license`·`machine`(지문) |
| **`MachineIdentity`** | 머신 상관·self-id 지문(§1.4). collector가 채움 | `machine_id`·`hardware_uuid`·`cloud_instance_id`·`fqdn`·`ips`(로케이터)·`self_assigned_id`(CMDB 없을 때 결정론 파생)·`derived_from` |
| **`Completeness`** | 계층별 커버리지(§2.6) | `layers_covered`·`layers_missing`(갭 — 자동 "부재" 금지)·`note` |

---

## 3. `discovery.v1` — 관측·파생

### `collector.proto` — Intake 계약 (Collector가 **반환**하는 것, §1.6)
core는 "노드를 주면 정규화된 CBOM을 반환한다"는 추상만 의존한다. 이 gRPC 경계가 곧 GPL 전염 차단 경계다(라이선스 정리).

| 메시지 | 목적 | 핵심 필드 |
|---|---|---|
| `CollectorCapabilities` | 능력 신고(`Describe`) | `crypto_runtimes`·`layers`·`detection_methods`·`license`·`invasive`(침습 시 PROPOSE 게이트) |
| `CollectRequest` | 수집 요청 | `target_node_ids`(스코프 게이트 통과분만)·`options` |
| **`CollectionResult`** | 정규화된 CBOM Envelope 한 단위 | `envelope` · `raw_capture`(불변 원본)+`raw_format` · **`cbom_cyclonedx`**(base64 CycloneDX 표준 본문)+`cyclonedx_spec_version` · `completeness` · `observed_edges` |

> Collector = §2.4 step 1–2(원시 포집 + CycloneDX 변환) + Envelope. **파생 `Finding`은 만들지 않는다.**

### `cbom.proto` — 파생 `Finding` (정규화 파이프라인이 **생성**, §2.4 step 3–6)
core 정규화 파이프라인이 `cbom_cyclonedx` 본문에서 파생하는 타입드 뷰다. **재계산할 수 있다**(§1.2).

| 메시지 | 목적 | 핵심 필드 |
|---|---|---|
| `OpensslAxes` | OpenSSL 분기축 | `lib`·`version`·`fork`(OpenSSL/BoringSSL/…)·`binding_mode` |
| `JcaAxes` | JCA 분기축 | `jdk_vendor`·`jdk_version`·`provider_set`(**순서 유의미** — 우선순위 협상)·`registration_mode` |
| `CngAxes` | Windows CNG 분기축 | `provider_set`(KSP/SSP, **관측된 순서 그대로**) · `algorithms`(이름·종류·서비스하는 provider) |
| **`Finding`** | 크립토 자산 한 건(파생 뷰) | `id`(정규화 해시)·`crypto_runtime`·`usage_context`·`algorithm` · `detection_method`+**`evidence_strength`**(파생) · `oneof {openssl\|jca}` · `pqc_readiness`·`fips_validation`·`remediation_class` · `derived_from_snapshot_id`+`ruleset_version`(재현) · **`app_keys`**(자산이 어느 앱 것인지, 공유 .so는 다중) |

### `asset.proto` — 자산 계층 (Machine → Application → Process)
| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| `ApplicationKind` | 안정 키 출처 | `SYSTEMD_UNIT`(권장)·`EXE_PATH`·`DECLARED` |
| **`Application`** | 타깃 앱(프로비저닝 1급 단위). 전역 식별=`(node_id, app_key)` | `node_id`·`app_key`·`name`·`kind`·`match` |
| `ProcessMatch` | app→라이브 프로세스 매칭 규칙(PID 저장 안 함) | `systemd_unit`(cgroup, 정확)>`exe_path`>`cmdline_regex` |
| `LiveProcess` | 런타임에 이어 붙인 결과(휘발, 조회 전용) | `pid`·`cmdline`·`started_at` |
| `ProcessResolution` | app의 라이브 프로세스 스냅샷 | `node_id`·`app_key`·`processes`·`resolved_at`(즉시 낡음) |

> **Process는 저장하지 않는다** — PID는 휘발. 프로비저닝 직전 `ProcessMatch`로 **그때그때 이어 붙인다**.

### `edge.proto` — 통신 엣지 (노드 간 관계)
| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| `NetworkProtocol` | 관측 프로토콜 | `TLS`·`SSH`·`QUIC`(핸드셰이크 암호화→대개 불명) |
| `EdgeRole` | src 방향 | `CLIENT`·`SERVER` |
| `QuantumPosture` | 양자내성(§1.6). **파생 뷰** — core가 `negotiated_group`에서 분류 | 🟢`PQC_HYBRID`·🔴`CLASSICAL`·⚪`UNSPECIFIED` |
| **`ObservedEdge`** | 관측된 통신 엣지 한 건 | `src_node_id`·`dst_node_id`(이어지지 않았으면 빈값 + `dst_addr`)·`protocol`·`role`·**`negotiated_group`**(등급 입력)·`cipher`·`observed_count`·`first/last_seen` |

---

## 4. `inventory.v1` — 메타데이터·판정

### `machine.proto` — 머신 메타데이터 (식별과 **분리**된 사람-대면 정보)
| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| `Environment` | 배포 환경(시각 축) | `PRODUCTION`·`STAGING`·`DEVELOPMENT`·`TEST` |
| `ProfileSource` | 프로필 출처 | `CMDB`·`REVIEWER`·`OBSERVED` |
| **`MachineProfile`** | 사람이 보고 구분하는 메타데이터(선언/리뷰어가 채움) | `node_id`(앵커)·`display_name`·`environment`·`role`·`owner`·`location`·`labels`(map)·`source` |
| **`MachineEndpoint`** | discovery 재접속용 **재사용 연결 메타데이터** | `node_id`·`name`·`ip`·`port` — ★**비밀 필드 없음**(키·계정·암호는 사용자 파일에만, §1.5) |

### `decision.proto` — 리뷰 판정 (스키마만 — 판정 엔진은 없다)
`FinalizedPlan`(provisioning)의 인벤토리 짝. 판정이 finalize되면 확정 계획으로 이어진다.

| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| `DecisionStatus` | 판정 lifecycle(§3.3③) | `DRAFT`·`IN_REVIEW`·`FINALIZED` |
| `DecisionConclusion` | 리뷰어 결론(특히 UNOBSERVED 항목) | `EXISTS`·`STALE`·`EXCLUDED`·`APPROVED` |
| **`ReconState`** | 선언과 관측을 대조한 결과 (어휘만 — **대조 엔진은 이 리포에 없다**) | `CONFIRMED`(선언∩관측)·`UNDECLARED`(관측만=shadow)·`UNOBSERVED`(선언만 — 기계가 확정하지 않는다) |
| **`Decision`** | 판정 한 건 | `subject`(엣지/정책 ID)·**`state`**(무엇에 대한 판정인가)·`conclusion`·`status`·`reviewer`·`signature`·`basis_hash`(근거 변하면 무효화)·`derived_from_snapshot_id` |

---

## 5. `provisioning.v1` — 생성·롤백

### `plan.proto` — 확정 계획 (프로비저닝의 유일 실행 근거)
| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| `DeployAutomationLevel` | 단계적 배포 위임(§4.3). 자산별 판정 | `L1_STAGE_ONLY`·`L2_STAGE_INSTALL`(프로덕션 기본)·`L3_FULL_AUTO`(활성화·재시작까지 — 계획의 `activation` 훅) |
| `PlanStatus` | 계획 lifecycle | `DRAFT`·`IN_REVIEW`·**`FINALIZED`**(실행 근거, §3.7 게이트) |
| `RemediationKind` | 조치 종류(프로비저닝 설계 §4.1·§4.2) → core 생성기 분기 | `CONFIG_ONLY`·`PROVIDER_INJECT`·`FORK_REPLACE`·`PROXY_FRONT`·`REBUILD`·`JDK_UPGRADE`·`APP_RECONFIG`·`DECOMMISSION` |
| **`RemediationAction`** | 자산 한 건에 대한 조치 | `target_node_id`·`finding_id`·`crypto_runtime`·`kind`·`automation_level`·`target_algorithm`·`provider_choice`·`provider_class`(FQCN 명시 — 없으면 알려진 이름만 확정)·**`config_artifact`**(core 생성기가 렌더)·**`activation`**(L3 훅)·`rollback_note`·`priority` |
| **`ActivationHooks`** | L3에서 실행할 **사용자가 적은 명령**(활성화 방법은 환경마다 달라 도구가 추측하지 않는다) | `pre`·`activate`·`deactivate`·`restart` — 생성기가 의미 순서로 배치: forward `pre→배치→activate→restart`, rollback `pre→deactivate→제거→restart` |
| **`FinalizedPlan`** | 확정 계획(스키마 — 저작·확정 엔진은 없다) | `id`·`status`·`scope`·`actions`·`approval_signatures`(finalize 전제)·`derived_from_snapshot_id`·`ruleset_version` |

### `rollback.proto` — 프로비저닝 히스토리·롤백 (스키마=OSS, 롤백 플레이북도 이 리포가 생성)
| 메시지/enum | 목적 | 핵심 필드 |
|---|---|---|
| **`CryptoState`** | 특정 시점 암호 상태(before/after 공통) | `modules`(예 `libcrypto.so.3@3.0.13`)·`config_digest`·`provider_chain`·`config_snapshot_ref`(롤백용 원문 참조) |
| `ProvisioningStatus` | 진행 상태(단계 경계=롤백 지점) | `STAGED`(L1)·`INSTALLED`(L2)·`ACTIVATED`(L3 — 활성화·재시작 완료)·`ROLLED_BACK`·`FAILED` |
| **`ProvisioningRecord`** | 프로비저닝 행위 1건의 append-only 이력 | `node_id`·**`app_keys`**(영향 앱, 공유 .so는 다중)·`action_id`·`plan_id`·**`before`**(롤백 기준)·`after`·`status`·`note`·`at` |

---

## 6. 관계 지도 — 메시지가 어떻게 이어지나

```
[Collector]  ──반환──▶  CollectionResult { Envelope(+MachineIdentity) · raw_capture · cbom_cyclonedx · ObservedEdge[] · Completeness }
                                     │  (§1.4 스코프 게이트 · ed25519 검증)
                                     ▼
[정규화]  ──파생──▶  Finding[] (evidence_strength·app_keys) ─┐   ObservedEdge + QuantumPosture(파생)
                                     │                            │
                              app_keys│앱                          │
                                     ▼                            ▼
                             Application (node_id, app_key)   [중앙 인벤토리 뷰]
                                     │                            ▲  ▸MachineEndpoint · MachineProfile (메타데이터 레인)
                           ProcessMatch│(실시간)                    │
                                     ▼                            │
                              LiveProcess (휘발)              [리뷰·판정] Decision ══(finalize)══╗
                                                                                              ║
                                                                                              ▼
[프로비저닝 생성]  FinalizedPlan { RemediationAction[] } ──§3.7 FINALIZED 게이트──▶ 플레이북(L1/L2/L3)
                                     │                                                  +
                                     ▼                                         ProvisioningRecord
                          before = CryptoState(Finding들)  ─────────────────▶  { before/after · app_keys · status }  (append-only 롤백 근거)
```

**레인으로 다시 보기**: 관측(`CollectionResult`·`ObservedEdge`) → 파생(`Finding`·`QuantumPosture`) → 선언/메타(`MachineProfile`·`Decision`) → 행위(`ProvisioningRecord`). `node_id`가 전 레인을 꿰는 앵커이고, `app_key(s)`가 크립토 자산을 앱에 이어 discovery→provisioning까지 흐른다.

> **`app_key`가 늘 채워지는 것은 아니다.** `Finding`과 `ProvisioningRecord`는 관측한 프로세스에서
> 바로 나오므로 앱이 항상 붙는다. **`ObservedEdge`도 앱까지 가되, 조회하는 순간 소켓이
> 살아 있어야 된다** — 회선을 수동 관측하는 방식에는 소켓을 연 PID가 없어서, 캡처 시점에 소켓
> inode를 `/proc/*/fd`와 대조해 채우기 때문이다.
>
> 그래서 짧게 붙었다 끊긴 연결은 비고, 권한이 모자라 남의 프로세스를 못 읽어도 빈다. **빈
> `app_key`는 "이 엣지에 앱이 없다"가 아니라 "어느 앱인지 밝히지 못했다"이고**, 왜 못 했는지는 완전성 맵의
> note에 적혀 있다. 못 채운 자리는 사람이 지정할 수 있다
> (`pqcota-declare-attribution`) — 다만 그 선언은 **관측을 고치지 않고** 자기 레인에 쌓이고,
> 합치는 일은 조회 화면에서 일어난다([검토 중인 설계 §5.2](../docs/under-review.md)).

---

관련 설계: [디스커버리](../discovery/design.md) · [인벤토리](../inventory/design.md) · [프로비저닝](../provisioning/design.md) · [아키텍처·OSS 경계](../docs/architecture.md). 실행 예제: [examples/](../examples).
