# 프로비저닝 테스트케이스 명세 (Scenario-driven Acceptance Tests)

[프로비저닝 설계](design.md)(규정서 §4)를 **검증 가능한 인수 기준**으로 옮긴 것이다. 구현은 이 테스트를 통과하는 것을 목표로 한다(TDD).

프로비저닝은 확정 계획에서 **플레이북과 롤백 근거를 만들고 영속**한다. 그래서 다루는 것은 실행 게이트(finalized-only) · 조치 아티팩트 렌더(OpenSSL/JCA) · 계획 채움(결정론) · L1/L2/L3 플레이북 생성과 롤백 · before 캡처와 레코드다. 플릿 오케스트레이션(drain·rolling·헬스체크 게이트)은 **하지 않아** 검증 대상이 아니다.


> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

---

## 0. 실행 환경

**케이스는 전부 unit이다** — 실물 없이 어디서나 돈다. 예외는 TP-RECORD-3 하나로, `PQCOTA_TEST_DSN`이 있으면 실 Postgres로도 돌고 없으면 스킵한다(**스킵은 통과가 아니다**).

생성→적용→되돌림 **종단**은 여기 케이스가 아니라 [데모 6/6](../demo/integration-verification.md)이 확인한다 — 생성물을 실제 노드에 적용하고 서비스 재시작·롤백까지 본다.

## 1. 상황 — 무엇이 생성되는지가 갈리는 지점

각 시나리오: **상황 → 생성물.** (억지 조합 없이, 산출물이 실제로 갈리는 것만.)

`SP-*`는 프로비저닝이 마주하는 **상황**(Scenario·Provisioning) 번호이고, §2 표 안의 `TP-*`는 그 상황을 검증하는 **테스트** 번호다.

#### SP-1. config-only — 최신 런타임 (OpenSSL 3.5+ / JDK-native)
- **상황**: 대상이 최신이라 하이브리드 그룹을 **config 한 줄**로 켤 수 있다.
- **생성물**: `[system_default_sect] Groups=X25519MLKEM768` config 조각(provider 로드 없음, 설계 §4.1) → L2 플레이북에 배치. L3면 계획의 `activation` 훅으로 활성화·재시작까지.
- 모듈을 놓지 않으므로 되돌림은 조각 제거만으로 끝난다.

#### SP-2. provider-inject — 레거시 런타임 (OpenSSL 3.0–3.4 / JCA=BC)
- **상황**: config만으론 PQC 그룹이 없어 provider 모듈 주입이 필요하다.
- **생성물**: provider 모듈 스테이지(L1) + `provider_sect·activate=1·module`(OpenSSL) 또는 `security.provider.N=BouncyCastleProvider`(JCA) config 조각(L2) (프로비저닝 설계 §4.1·§4.2). 모듈 배치에는 **sha256 게이트**가 붙는다(무엇을 심었는지 고정 — §2.3).
- **JCA 함정**: JAR '배치'만으로는 provider가 로드되지 않는다(JDK 9+엔 `lib/ext` 없음). classpath·`--module-path` 배선은 앱 기동 방식에 달렸으므로 `activation.activate`에 적는다 — 플레이북 헤더가 이 함정을 먼저 짚는다.

#### SP-3. non-config 조치 — fork 교체 (config로 주입 불가)
- **상황**: 셰이딩·정적 링크·소스 유실 등으로 **config로 못 바꾸는** 조치.
- **생성물**: **"config로 배포 불가"를 주석으로 명시**하고 건너뛴다(프로비저닝 설계 §4.1). 아티팩트를 지어내지 않는다 — 생성 불가를 감추면 배포된 척하는 플레이북이 된다.

#### SP-4. 에어갭 — 오프라인 반입
- **상황**: 중앙 연결이 없는 격리망.
- **생성물**: L1/L2/L3 아티팩트·플레이북·롤백 근거가 **파일**이라 그대로 반입 매체에 담긴다. 생성물은 연결 여부와 무관하게 동일하다.

#### SP-5. 규제 자산 — FIPS 라우팅
- **상황**: `fips_validation`을 요구하는 자산.
- **생성물**: provider 선택을 **FIPS 검증 provider로 강제 라우팅**한 config 조각(Java=BC-FJA `BouncyCastleFipsProvider`, 설계 §4.2 · 규정서 §4.10).

#### SP-6. 롤백 — before 캡처와 되돌림
- **상황**: 조치 후 검증 실패, 또는 되돌리기로 결정.
- **생성물**: 조치 *전* `CaptureState`로 before(모듈@버전·config·provider 체인) 캡처 + `ProvisioningRecord`(STAGED, `app_keys` 다중 귀속) append-only 영속(§6A). 되돌림은 `--rollback` 플레이북 — forward가 원본을 덮지 않고 파일을 *추가*하므로 그 추가분 제거가 곧 복원이다. L3면 `deactivate` 훅으로 활성화까지 되돌린다.

---

---

## 2. 테스트케이스

케이스 번호는 **`TP`(프로비저닝) - 무엇을 보나 - 순번**이다 — `TP-GATE`(실행 게이트) · `TP-RENDER`(조치 렌더) · `TP-PLAYBOOK`(플레이북 생성) · `TP-RECORD`(before 캡처·레코드). 번호는 그것을 검증하는 **테스트 파일로 이어진다**. 구현 순서는 §3.

### TP-GATE. 실행 게이트 — finalized-only (§3) — 핵심 안전 게이트
`FINALIZED` + 승인 서명 ≥1 + 조치 ≥1 만 실행 근거로 인정한다(규정서 §3.7 최강 게이트).

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TP-GATE-1](../pkg/provisioning/plan_test.go) | `TestExecutable` — FINALIZED+승인 서명+조치 ≥1 / draft·in-review / 서명 없음 / 조치 0건 / `nil` | 첫째만 **실행 가능**, 나머지는 전부 **거부** | 확정 전 계획으로 머신을 건드리지 못하게 하되, 정당한 계획까지 막으면 아무것도 배포할 수 없다. 잘못된 입력에서 터지면 게이트가 없는 것과 같다 |
| [TP-GATE-2](../pkg/provisioning/plan_test.go) | `TestProviderClassWarnings` — placeholder를 낳는 조치 | 경고 1건(provider 이름·해결책 포함). `Executable`은 **여전히 통과** | 조각 안 주석은 열어봐야 보이므로 경고로도 띄운다. 미확정과 실행 거부는 별개다 |

### TP-RENDER. 조치 아티팩트 렌더 (§4)
조치 taxonomy(`RemediationKind`)별로 config 조각을 **결정론적으로** 렌더한다(§1.2 재계산 가능). config로 못 넣는 건 정직하게 비-config임을 명시.

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TP-RENDER-1](../pkg/provisioning/render_test.go) | `TestRenderOpenSSLConfigOnly` — OpenSSL 3.5+ | `[system_default_sect]` · `Groups`만. **provider 로드 없음** | 최신 런타임에 필요 없는 provider를 로드하지 않는다 |
| [TP-RENDER-2](../pkg/provisioning/render_test.go) | `TestRenderOpenSSLProviderInject` — 3.0–3.4 | `provider_sect` · `activate = 1` · `module` + 그룹 활성화 | config만으로는 PQC 그룹이 없어 모듈을 함께 내야 동작한다 |
| [TP-RENDER-3](../pkg/provisioning/render_test.go) | `TestRenderOpenSSLConfigIsUsableStandalone` — config-only·주입 둘 다 | 최상위 `openssl_conf = openssl_init`이 **첫 섹션보다 앞**에 있다 | 실측: 이 줄이 없으면 OpenSSL이 `[openssl_init]`을 아예 안 읽어, 모듈이 놓이고 sha256 게이트도 통과하는데 provider는 안 올라온다. 섹션 뒤로 가면 그 섹션 소속이 되어 같은 실패로 돌아간다 |
| [TP-RENDER-4](../pkg/provisioning/render_test.go) | `TestRenderOpenSSLNonConfig` — fork-replace | config 조각이 아니라 **주석 조치**로 명시 | 아티팩트를 지어내면 배포된 척하는 플레이북이 된다 |
| [TP-RENDER-5](../pkg/provisioning/render_test.go) | `TestRenderJCAConfigOnly` — JDK 네이티브 | `jdk.tls.namedGroups`만. **provider 등록 없음** | JDK가 이미 그룹을 아는데 provider까지 등록하면 체인을 건드리는 위험만 는다 |
| [TP-RENDER-6](../pkg/provisioning/render_test.go) | `TestRenderJCAProviderInject` — BC·BCFIPS | `security.provider.N` 등록 + `namedGroups`. 규제 자산은 `BouncyCastleFipsProvider`로 라우팅 | 등록만 하고 그룹을 안 켜면 provider는 로드되고 협상은 그대로다 |
| [TP-RENDER-7](../pkg/provisioning/render_test.go) | `TestRenderJCAExplicitProviderClass` — `providerClass`에 FQCN 명시 | 그 FQCN이 그대로 등록. placeholder 경고 **없음** | 사용자가 이미 준 답을 다시 찾게 만들지 않는다 |
| [TP-RENDER-8](../pkg/provisioning/render_test.go) | `TestRenderJCAUnknownProviderKeepsPlaceholder` — 모르는 이름, 클래스 미지정 | `<이름: …>` placeholder + `providerClass`로 푸는 법 | 모르는 provider의 클래스명을 지어내지 않는다(§2.5) |
| [TP-RENDER-9](../pkg/provisioning/render_test.go) | `TestRenderJCAJarPlacementGuidance` — JAR 배치 안내 | 실제 배치 경로 + **JDK 9+** 세대 차이 | 안내가 실제 경로와 어긋나지 않게. `lib/ext`는 JDK 9에서 없어졌다 |
| [TP-RENDER-10](../pkg/provisioning/render_test.go) | `TestBCDefaultClassStatesVersionAssumption` — BC 기본 클래스 | **버전 전제 명시**(`1.80+` · `BouncyCastlePQCProvider`). 클래스를 명시했거나 BCFIPS면 안 붙음 | 실측: 1.80/1.81엔 ML-KEM 서비스 17개, 1.78.1엔 0개. 계획은 JAR 버전을 알려주지 않는다 |
| [TP-RENDER-11](../pkg/provisioning/render_test.go) | `TestProviderSlotReplacementIsStated` — `security.provider.2=` | **자리 대체**임을 조각과 `ProviderSlotWarnings` 양쪽에 명시. config-only엔 경고 없음 | 실측: JDK 21에서 목록은 12개 그대로고 원래 2번이던 SunRsaSign이 사라진다. 삽입처럼 읽히면 생성물이 거짓말을 한다 |
| [TP-RENDER-12](../pkg/provisioning/render_test.go) | `TestNamedGroupsAlwaysKeepsClassicFallback` — config-only·주입 둘 다 | PQC 그룹 뒤에 **고전 폴백**을 항상 남긴다 | 실측: 미지 그룹만 주면 JDK 21 JSSE가 초기화에서 터진다 — 생성물이 앱을 죽인다 |
| [TP-RENDER-13](../pkg/provisioning/render_test.go) | `TestFillPlan` — 혼합 계획에 계획 채움 | 모든 조치의 `config_artifact`가 채워짐 | 스냅샷과 ruleset만으로 같은 아티팩트가 다시 나와야 한다(§1.2) |

### TP-PLAYBOOK. 플레이북 생성 — 단계적 배포 위임 (§5)
산출물은 표준 Ansible 플레이북이다 — `ansible-playbook`으로 적용하고, 롤백 플레이북으로 되돌린다.

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TP-PLAYBOOK-1](../pkg/provisioning/ansible_test.go) | `TestGeneratePlaybook` — collector 배포 | 노드별 `- host` + `copy` + `become: true` | collector 바이너리를 노드에 놓는 플레이북을 낸다 |
| [TP-PLAYBOOK-2](../pkg/provisioning/stage_test.go) | `TestProvisioningPlaybookL2` — 확정 계획 → L2 | 노드별 play + 모듈 스테이지 + config 조각 배치. **재시작 없음** | L2가 배치까지만 하고 활성화로 넘어가지 않게 — 단계 경계를 코드로 강제한다 |
| [TP-PLAYBOOK-3](../pkg/provisioning/stage_test.go) | `TestProvisioningPlaybookL1` — L1 | 모듈 스테이지만. config 배치 없음 | L1은 놓기만 하는 단계다. config가 새면 승인한 것보다 멀리 간다 |
| [TP-PLAYBOOK-4](../pkg/provisioning/rollback_test.go) | `TestRollbackPlaybookL2` — 롤백 L2 | forward 역방향 — `state: absent`. 재시작 없음 | 되돌림이 배치와 어긋나면 흔적이 남거나 필요한 것까지 지운다 |
| [TP-PLAYBOOK-5](../pkg/provisioning/rollback_test.go) | `TestRollbackPlaybookL1` — 롤백 L1 | 스테이지 모듈만 제거 | L1 롤백이 놓지도 않은 config를 지우려 하지 않게 |
| [TP-PLAYBOOK-6](../pkg/provisioning/stage_test.go) | `TestL3ActivationOrder` — 훅 4개 → L3 | pre → 배치 → activate → restart. 롤백은 정확한 역순. L2엔 새지 않음 | 순서가 곧 안전성이다 — 내리고, 바꾸고, 참조되게 하고, 새로 로드한다 |
| [TP-PLAYBOOK-7](../pkg/provisioning/stage_test.go) | `TestL3MissingHooksWarnButDoNotGuess` — 훅이 빈 계획 | shell 태스크 0개 + `ActivationWarnings`로 무엇이 안 일어나는지 고지 | 빈 자리에 그럴듯한 명령을 채우면 도구가 모르는 것을 아는 척한다(§2.5) |
| [TP-PLAYBOOK-8](../pkg/provisioning/stage_test.go) | `TestL3HooksGroupedAndDeduped` — 한 노드에 조치 여럿, 같은 재시작 | 단계별로 모으고 **같은 명령은 한 번만** | 조치별로 내면 서비스를 n번 흔들고, 활성화 사이에 재시작이 끼어 일부만 반영된 채 뜬다 |
| [TP-PLAYBOOK-9](../pkg/provisioning/stage_test.go) | `TestConfigFragmentsNeverOverwriteEachOther` — 내용이 다른 조각 2개 | 조치별 경로로 분리 + `ConfigConflictWarnings`. 같으면 한 경로 | 같은 경로에 두 번 쓰면 뒤가 앞을 조용히 덮어써 앞 조치가 사라진다 |
| [TP-PLAYBOOK-10](../pkg/provisioning/stage_test.go) | `TestJCAClasspathHintInHeader` — JCA 주입이 섞인 계획 | classpath·`--module-path` 함정과 `activation.activate` 안내가 헤더에. openssl 전용엔 **안 뜸** | JAR 배치만으로는 provider가 로드되지 않는다 — 먼저 짚되 무관한 노트로 어지럽히지 않는다 |
| [TP-PLAYBOOK-11](../pkg/provisioning/stage_test.go) | `TestGeneratedPlaybooksAreValidYAML` — 따옴표·`#`·줄바꿈이 섞인 id·이름·훅 | 여섯 산출물이 전부 **YAML로 파싱된다** | 문법부터 깨지면 ansible이 파일을 읽지도 못한다 — 실제로 여러 줄 명령에서 그렇게 깨졌다 |
| [TP-PLAYBOOK-12](../pkg/provisioning/paths_test.go) | `TestModulePathAgreesAcrossGenerators` — config 렌더·L2 배치·L2 롤백 | 셋이 **같은 절대 경로** | 어긋나면 OpenSSL이 모듈을 못 찾고 조용히 실패한다 |
| [TP-PLAYBOOK-13](../pkg/provisioning/paths_test.go) | `TestConfigNeverUsesRelativeModule` — 이름이 빈 값·기본·커스텀 | `module =` 줄이 항상 `/`로 시작 | 상대 경로면 OpenSSL이 모듈 디렉터리에서 찾다 실패한다 |
| [TP-PLAYBOOK-14](../pkg/provisioning/paths_test.go) | `TestPerProviderModuleSourceVariable` — 이름에 비영숫자(`my-prov.1`) | `pqcota_module_src_my_prov_1` → 전역 → `files/` 순 폴백 | 한 플레이북에 여러 provider가 섞여도 각자 소스를 지정할 수 있게. 변수명은 Ansible 규칙에 맞춘다 |
| [TP-PLAYBOOK-15](../pkg/provisioning/paths_test.go) | `TestChecksumGate` — 모듈 배치 | `checksum_algorithm: sha256` + `assert` + `… is defined` 가드 | 무엇을 심었는지 고정한다. 안 주면 검사만 건너뛴다 |
| [TP-PLAYBOOK-16](../pkg/provisioning/paths_test.go) | `TestJCAModuleIsJar` — JCA 주입 | 배치 경로가 `.jar`로 끝나고 조각 안내와 일치 | 런타임에 따라 갈리는 확장자·경로를 한 곳에서 정한다 |
| [TP-PLAYBOOK-17](../pkg/provisioning/paths_test.go) | `TestL2CreatesConfigDirectory` — 깨끗한 노드에 L2 | 디렉터리를 `state: directory`로 **배치보다 먼저**. L1은 만들지 않음 | 실 ansible에서 잡힌 회귀 — `copy`는 대상 디렉터리가 없으면 실패한다 |

### TP-RECORD. before 캡처 · 롤백 레코드 (§6A)
롤백 근거 = 조치 *전* before 상태. append-only 보존, 노드별로 되찾는다.

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TP-RECORD-1](../pkg/provisioning/capture_test.go) | `TestCaptureState` — findings → `CaptureState` | before `modules`에 `libcrypto.so.3@3.0.13`(버전 없으면 이름만)·`jca:BC` + `provider_chain` | 되돌릴 때 무엇으로 돌아가야 하는지는 조치 전에만 알 수 있다 |
| [TP-RECORD-2](../pkg/provisioning/record_test.go) | `TestMemRecordStore` — 여러 노드 레코드 append | before 캡처 + 초기 **STAGED** + `app_keys` 다중 귀속. **append-only**, 노드별 조회가 순서 보존·노드 간 격리 | 롤백 근거를 덮어쓰지 않는다 |
| [TP-RECORD-3](../pkg/provisioning/record_pg_test.go) | `TestPgRecordStore` — 노드 둘에 레코드 append(`PQCOTA_TEST_DSN` 있을 때) | append 순서 보존·노드 간 격리, before 상태와 `app_keys`·STAGED가 왕복 | 저장소를 바꿨더니 순서가 섞이거나 노드가 새면, 되돌릴 때 무엇으로 돌아가야 하는지를 잘못 짚는다 |

> **영속화**: `PgRecordStore`는 `PQCOTA_TEST_DSN`이 있을 때 TP-RECORD-3가 실 Postgres로 돌고, 종단은 [데모 6/6](../demo/integration-verification.md)이 `pqcota-records` 조회로 확인한다.

---

## 3. 구현 순서 (unit 먼저)

| # | 대상 | 케이스 | 레벨 |
|---|---|---|---|
| 1 | **실행 게이트**(finalized-only) · 경고 표면화 | TP-GATE-1–2 | unit |
| 2 | **조치 아티팩트 렌더**(OpenSSL/JCA) · 계획 채움 | TP-RENDER-1–13 | unit |
| 3 | **플레이북 생성**(collector 배포 + L1/L2/L3 적용·롤백) | TP-PLAYBOOK-1–11 | unit |
| 4 | **경로·무결성**(삼자 일치·절대 경로·sha256·디렉터리) | TP-PLAYBOOK-12–17 | unit |
| 5 | **before 캡처 · 레코드** | TP-RECORD-1–3 | unit + Postgres |

**핵심 인수 기준**: **TP-GATE-1(finalized+서명+조치 아니면 실행 거부)** — 확정되지 않은 계획으로 머신을 건드리지 못하게 하는 최강 게이트. 그리고 **TP-RENDER-3·A2(config로 못 넣는 조치는 정직하게 주석·L2에서는 재시작을 만들지 않음)** — 단계 경계가 코드로 강제됨을 보장. **TP-PLAYBOOK-6–A9**는 L3 훅의 의미 순서·롤백 대칭, 빈 훅에서 명령을 지어내지 않음, 그리고 **조각·재시작이 서로를 덮어쓰지 않음**을 못박는다.

**실제 장비에서 잡힌 회귀**는 따로 못 박아 뒀다 — TP-PLAYBOOK-12(경로 삼자 일치)·TP-PLAYBOOK-17(깨끗한 노드의 디렉터리)·TP-PLAYBOOK-11(YAML 문법)·TP-RENDER-11(JSSE 초기화)·TP-RENDER-10(provider 자리 대체). 생성만 보면 통과하고 적용에서 깨지던 것들이다.

---

## 4. 데이터 모델 매핑 (계약)

| TC 그룹 | 계약 타입 (contracts SSOT) |
|---|---|
| 실행 게이트 | `provisioning/v1` `FinalizedPlan` · `PlanStatus` · `RemediationAction` |
| 조치 렌더 | `RemediationKind`(config-only / provider-inject / fork-replace) · `RemediationAction.config_artifact` |
| 플레이북 L1/L2 | `DeployAutomationLevel`(L1_STAGE_ONLY · L2_STAGE_INSTALL) |
| before·롤백 | `rollback.proto` `CryptoState` · `ProvisioningRecord` · `ProvisioningStatus`(STAGED…) |

> `ProvisioningStatus_ACTIVATED`(L3 활성화·재시작 완료)는 플레이북 적용 후의 상태다. 여기 테스트는 생성물과 STAGED 캡처를 검증한다 — 적용 결과는 데모가 실물로 확인한다.
