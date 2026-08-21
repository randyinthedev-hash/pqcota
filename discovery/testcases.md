# 디스커버리 테스트케이스 명세 (Scenario-driven Acceptance Tests)

[디스커버리 설계](design.md)가 세운 상황(SD-1–SD-7)과 network-collector(설계 §2.3)를 **검증 가능한 인수 기준**으로 옮긴 것이다. 구현은 이 테스트를 통과하는 것을 목표로 한다(TDD).

각 절은 **상황 · 사용자 준비 · 기대 결과**로 시작하고, 그 아래가 케이스 표다.

관측 결과가 거치는 **파생 규칙**(증거 강도·정규화·등급)은 단계를 가로지르므로 [커널 테스트케이스](../docs/kernel-testcases.md)에 있다.


> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

---

## 0. 테스트 레벨

| 레벨 | 정의 | 환경 | 입력 |
|---|---|---|---|
| **unit** | 결정론적 순수 로직. 실물 호스트 불필요 | 어디서나 | 테스트 내부 상수(`/proc` 스냅샷·ELF 바이트·`getProviders()` 출력·패킷) |
| **integration** | 실물 바이너리/JVM/프로세스 필요 | **리눅스** | 실제 OpenSSL·JDK 등 |
| **e2e** | 컨테이너 등 환경 조립 | 리눅스 + Docker | [데모](../demo/integration-verification.md) |

---

## 1. 시나리오별 테스트케이스

케이스 번호는 **`TD`(디스커버리) - 무엇을 보나 - 순번**이다 — `TD-OPENSSL` · `TD-JVM` · `TD-FORK` · `TD-CONTAINER` · `TD-SCOPE` · `TD-GAP` · `TD-SIGN` · `TD-NETWORK` · `TD-CNG`(Windows CNG provider) · `TD-PROVENANCE`(수집 시각) · `TD-ATTR`(엣지를 연 앱). 번호는 그것을 검증하는 **테스트 파일로 이어진다**(링크가 없는 것은 데모가 본다).

절 제목의 `SD-*`는 [디스커버리 설계](design.md)가 매긴 **상황**(Scenario·Discovery) 번호이고, 표 안의 `TD-*`는 그 상황을 검증하는 **테스트** 번호다 — 다른 축이라 섞지 않는다.

### SD-1. OpenSSL 실행중

- **상황**: CMDB 등재된 VM, `libssl` 동적 링크, 서비스 구동 중.
- **[사용자]** 스코프 마스터 등재 · root/`CAP_SYS_PTRACE` 부여 · mTLS 자격 발급 · collector를 자신의 substrate로 실행(또는 self-service).
- **결과**: `evidence_strength=confirmed`, 전 계층 커버.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-OPENSSL-1](collectors/openssl/procmaps_test.go) | unit | `TestParseProcMaps` · `_none` — `/proc/<pid>/maps` 스냅샷 파싱 | 로드된 `libssl`/`libcrypto` 경로 목록. 없으면 빈 목록 | 실행 중인 프로세스에 로드된 lib을 잡아낸다 — 디스커버리의 출발점이다 |
| [TD-OPENSSL-2](collectors/openssl/scan_test.go) | unit | `TestMergeByPathUnionsAppKeys` · `NoAppKeys` — 같은 `.so`를 여러 프로세스가 물고 있을 때 | 경로 하나로 합치고 `app_keys`는 **합집합**. 쓰는 앱이 없으면 `nil` | 공유 라이브러리를 프로세스 수만큼 중복 등재하지 않으면서, 그것을 쓰는 앱을 하나도 잃지 않는다 |
| [TD-OPENSSL-3](collectors/openssl/service_test.go) | unit | `TestCollectorServiceContract` — Describe·Collect gRPC 왕복 | 능력 신고와 결과 스트림이 계약대로 | 코어가 collector를 계약으로만 부르는지 — 배선이 끊기면 실물이 멀쩡해도 아무것도 안 온다 |
| TD-OPENSSL-4 | **integration** — [데모 2/6](../demo/integration-verification.md) | 실 OpenSSL 컨테이너 노드 수집 | 정규화된 CBOM, evidence_strength=CONFIRMED | 손으로 만든 스냅샷이 아니라 실물 호스트에서 같은 결과가 나오는지 확인한다 |
| [TD-OPENSSL-5](collectors/openssl/build_test.go) | unit | `TestBuildResult` · `NoDetection` · `RawCaptureRoundTripsAndIsStable` — 탐지 결과 → CollectionResult | CycloneDX 본문·원본·완전성이 채워지고, 탐지가 없으면 전 계층 갭 + 사유. 같은 관측이면 같은 바이트 | 조립이 비면 무엇을 봤든 코어에 도달하지 않는다. 원본이 흔들리면 서명이 깨진다(§2.6) |
| [TD-OPENSSL-6](collectors/openssl/scanhost_test.go) | unit(linux) | `TestScanHostStatsAreConsistent` · `DetectForPIDMissingProcess` · `DetectForPIDSelf` · `ExtractStringsRejectsNonELF` · `ExtractStringsOnRealELF` — 실물 `/proc`·ELF | 집계가 자기모순이 아니고, 없는 PID는 오류. ELF가 아니면 문자열을 지어내지 않는다 | 호스트 훑기와 fork 판정의 입력이 실물에서 도는지 |

### SD-2. JVM attach — provider 체인 ★

- **상황**: 구동 중인 Java 앱, BouncyCastle 동적 등록 여부 정적으로 불명.
- **[사용자]** 대상 JVM과 **동일 UID** 접근 보장 · attach 허용(`DisableAttachMechanism` 아님) 확인 · JDK 동봉 배포 승인.
- **결과**: 정적으로 관측되지 않는 동적 등록·명시 지목 포착(§2.3).

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-JVM-1](../pkg/kernel/registry/provider_test.go) | unit | `TestMatchProvider` — provider 체인(BC · JDK 네이티브 · 미등록) → 레지스트리 매핑 | BC=ML-KEM·ML-DSA·SLH-DSA·fips=none, JDK 네이티브는 **SLH-DSA 갭 태깅**, 모르는 이름은 미매칭 | 등록된 provider에서 PQC 능력을 읽어내고, 못 하는 것을 할 수 있다고 보지 않는다 |
| [TD-JVM-2](collectors/jvm/nativeattach_parse_test.go) | unit | `TestLoadAgentRequest` · `ParseAttachResponse` · `ParseNSpid` — attach 프로토콜 조립·응답·컨테이너 PID | 인자 슬롯 3개, 리턴 코드 0만 성공, `NSpid` 마지막 값 | 침묵을 성공으로 치지 않는다(§2.5). 컨테이너는 **네임스페이스 내부 PID**로 찾아야 만난다 |
| [TD-JVM-3](collectors/jvm/procscan_test.go) | unit | `TestDeriveJavaHome` · `ParseReleaseVersion` · `JavaBinFor` · `IsJavaExe` · `ParseMainId` · `Ident` · `AttachCapable` — 정찰 파싱 | JAVA_HOME·버전·앱 식별자를 뽑고, 못 짚으면 빈 값 | 정찰이 못 짚은 값을 뒤 단계가 사실로 이어받지 않게 한다 |
| [TD-JVM-4](collectors/jvm/attach_test.go) | unit | `TestAttachAll` · `AttachAllEmpty` · `AttachClient` — 발견 JVM 여럿에 주입된 attach 실행 | 성공은 체인 수집, 실패는 **갭으로 카운트**. 클라이언트는 머신의 attach 가능 JDK를 재사용 | attach에 실패한 JVM이 조용히 사라져 "깨끗함"으로 남지 않게 한다(§2.6) |
| [TD-JVM-5](collectors/jvm/attach_test.go) | unit | `TestBuildResultForDistinguishesJVMs` · `IdentIsStable` — 서로 다른 JDK 둘 | 각 JVM이 **구별되는 finding**. 식별자는 PID가 아니라 앱(main·jar, 없으면 JAVA_HOME) | 한 노드의 JVM 여럿이 뭉개지지 않게, 그리고 재기동마다 새 자산이 되어 이력이 끊기지 않게 한다 |
| [TD-JVM-6](collectors/jvm/staticfallback_test.go) | unit | `TestParseJavaSecurity` · `Empty` · `StaticFallbackNoJavaHome` — attach가 막혔을 때 | `java.security`를 **N 순서대로** 파싱. 빈 목록도 오류가 아님. JAVA_HOME 미상이면 오류 + 강등 | attach가 막힌 노드가 "provider 없음"이 아니라 "관측하지 못했음"으로 남게 한다. 순서가 곧 우선순위다 |
| [TD-JVM-7](collectors/jvm/jvm_test.go) | unit | `TestParseProviders` · `BuildResult` · `JvmServiceContract` — 사이드카 출력 → 정규화·계약 노출 | provider 순서 보존, 원본(`Raw`) 보관, Describe/Collect 왕복 | 사이드카가 본 것이 순서와 원본을 잃지 않고 코어까지 간다 |
| TD-JVM-8 | **integration** — [데모 2/6](../demo/integration-verification.md) | 정찰→실 agent attach 종단 (`PQCOTA_JVM_AGENT`) | 발견된 PID에 실제 attach해 provider 체인 관측 | 정찰과 attach가 실물에서 하나로 이어지는지 확인한다 |
| [TD-JVM-9](collectors/jvm/disableattach_test.go) | **integration** | `TestDisabledAttachFallsBackToJavaSecurity` — `-XX:+DisableAttachMechanism`으로 띄운 실 JVM | attach 실패가 **갭으로 세어지고**, java.security 폴백이 provider를 읽어낸다(열화 표시 + 원본 보존) | attach가 막혔다고 "provider 없음"이 되면 관측하지 못한 것과 없는 것이 뒤섞인다 |
| [TD-JVM-10](collectors/jvm/procscan_test.go) | unit | `TestDeriveJavaHomeWindows`·`TestIsJavaExeWindows` — Windows 경로 규칙 | `...\bin\java.exe` → JAVA_HOME, `bin`이 없으면 **빈 값** | 경로 규칙은 순수 문자열 처리라 리눅스 CI에서 검증한다 — 실기 없이 못 잡는 자리를 여기서 못 박는다 |
| TD-JVM-11 | **실물** — 확인 | Windows 11(26200)에서 `pqcota-jvmscan --recon` | 프로세스 255–261개를 훑고, `\fakejdk\bin\java.exe`로 뜬 프로세스를 잡아 `javaHome`을 `\fakejdk`로 낸다. `attachCapable`=false·`version` 없음 | 교차 컴파일은 API 이름만 보증한다. Toolhelp32가 실제로 무엇을 돌려주는지, 경로 규칙이 실기와 맞는지는 돌려 봐야 안다 |
| TD-JVM-12 | **실물** — 확인 | 같은 명령을 일반 사용자와 관리자로 | 일반 265개 중 **163개를 못 열고**, 관리자는 264개 중 **3개** | Windows에서 Java 서버는 보통 서비스(SYSTEM)로 돈다 — 권한 없이 돌리면 봐야 할 JVM이 통째로 안 보인다. 숫자만 내면 "JVM 0개"로 읽히므로 화면이 뜻과 넓히는 법을 함께 낸다 |
| [TD-JVM-13](collectors/jvm/attach_test.go) | unit | `TestDegradedNoteNamesTheJVM`·`TestDegradedNoteCarriesItsOwnReason`·`TestGapResultCarriesTheJVMToTheCentre` | 갭 노트가 **어느 JVM·왜**인지 밝히고, 찾았는데 못 본 JVM은 컴포넌트 없이 갭만 실어 보낸다 | 셋 다 실기에서 드러난 결함을 못 박은 것이다(아래) |
| TD-JVM-14 | **실물** — 확인 | Windows 11 + JDK 21에서 `pqcota-jvmscan --output table` | ①이 실패하고 **②가 붙는다**(대상 JVM이 `A Java agent has been loaded dynamically`를 찍는다). javapath 런처 심은 `jvm.dll not loaded by target process`로 **갭**이 되고, 그 사유가 화면과 계약에 남는다 | Windows attach 경로의 첫 실물 확인이다. 여기서 결함 셋이 나왔다: ②가 실패 시 **클라이언트 자신의** java.security를 읽어 남의 provider를 대상에 붙였고, 도는 JVM이 없으면 프로브가 **띄운 JVM**을 confirmed로 냈으며, 관측 못 한 JVM이 **중앙에 가지 않았다** |
| TD-WIN-1 | **실물 종단** — 확인 | 실 Windows 노드(192.168.1.25)에 `ansible-playbook discover.yml` | `os_family`로 리눅스 블록 14개가 건너뛰어지고 Windows 블록이 돈다: 반입 2개 → JVM 정찰 → CNG 관측 → 회수 → 정리. `failed=0`, 노드에 **아무것도 안 남는다**(`Test-Path` false) | 코드 경로가 다 맞아도 앤서블이 그 노드에 닿는 부분은 따로다. 연결은 Win32-OpenSSH + 키(`ansible_shell_type=powershell`) — `pqcota-hosts`가 `hosts.csv`의 `os`·`connection`에서 그 인벤토리를 만든다 |
| TD-WIN-2 | **실물 종단** — 확인 | 회수한 JSON을 `pqcota-discover-view`로 | `win-01 · CNG providers: 9 · 50 algorithms · PQC native (signature only — no KEM observed) [CONFIRMED]`. 머신 지문에 `hardware_uuid`(SMBIOS)까지 차고 `derived_from=machine-id` | 관측이 화면까지 오지 않으면 적지 않은 것과 같다. 플레이북이 낸 산출물이 그대로 뷰의 입력이 되는지 확인한다 |

### SD-3. 바이너리 fork 매처 — IP

- **상황**: 소스 유실·정적 링크·셰이딩된 벤더 바이너리.
- **[사용자]** 바이너리 위치·벤더 정보 제공.
- **결과**: `evidence_strength=inferred-low`, "정보 부재 아닌 열화"(§2.3). confirmed로 올리려면 벤더 문의·역분석 같은 **사람 손**이 필요하다 — pqcota는 표기까지다.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-FORK-1](../pkg/kernel/registry/fork_test.go) | unit | `TestMatchFork` — 스트립 ELF의 마커 문자열로 fork 판정 | BoringSSL·AWS-LC는 각각 짚고, 시그니처가 없으면 `fork=""`(unknown 명시). OpenSSL 버전도 추출 | 심볼 없는 바이너리를 "크립토 없음"으로 넘기지 않으면서, 모를 때 지어내지도 않는다. 이름이 같은 다른 fork를 묶으면 조치 방법이 달라진다 |

### SD-4. 컨테이너

- **상황**: `/proc`·JVM attach가 컨테이너 네임스페이스에 갇힘.
- **[사용자]** 사이드카 주입 vs host PID namespace 권한 정책 결정.
- **결과**: 주입 성공 시 confirmed, 실패 시 완전성 맵에 계층 갭.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| TD-CONTAINER-1 | **e2e** — [데모 2/6](../demo/integration-verification.md) | 대상 안에서 실행, 동일 PID namespace | `/proc` 가시 → CONFIRMED | 컨테이너 안에서 `/proc`이 보이는 조건을 확인한다 |
| [TD-CONTAINER-2](collectors/openssl/scanhost_test.go) | unit(linux) | `TestCollectSeparatesUnseenFromAbsent` — 볼 수 없는 프로세스 / 읽을 수 있는 프로세스 | 관측하지 못했으면 PROCESS를 **커버로 세지 않고** 사유 고지, 봤는데 없으면 커버로 센다 | 한 문구로 뭉뚱그리면 결함이 갭처럼, 갭이 부재처럼 읽힌다 |

### SD-5. 스코프 게이트·라우팅

- **상황**: 수집 중 스코프 마스터에 없는 통신 상대·노드 발견.
- **[사용자]** 등재/제외 판정(MANUAL) · 스코프 마스터 갱신.
- **결과**: 신규 리뷰 항목 생성, 자산 경계 정정.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-SCOPE-1](../pkg/kernel/scope/gate_test.go) | unit | `TestScopeGate` — 미등재 노드가 대상에 섞였을 때, 수집 중 미등재 노드를 관측했을 때 | 대상에서 **필터 제외**하고, 관측된 미등재 노드는 **수집하지 않고** 등재 판정 큐로 라우팅(PROPOSE) | 스코프 밖 노드를 동의 없이 건드리지 않으면서, 발견한 사실을 삼키지도 임의로 등재하지도 않는다 |

### SD-6. 완전성 맵 갭≠부재

- **상황**: 야간 배치·주기 기동 노드라 수집 시점에 미실행.
- **[사용자]** 배치 윈도우·기동 스케줄 정보 제공.
- **결과**: "실제 없음" ≠ "원리상 관측하지 못함" 구분 유지(§2.6).

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-GAP-1](../pkg/discovery/normalize/completeness_test.go) | unit | `TestCompleteness` — Describe가 선언한 계층을 Collect가 못 덮었을 때 / 전부 덮었을 때 | `layers_missing` + note를 남기고, **"부재"로 처리하지 않는다**. 전부 덮으면 갭 없음 | 볼 수 있다고 신고한 계층을 못 덮고도 덮은 것처럼 내지 않는다 |

### SD-7. 에어갭

- **상황**: 인터넷·중앙 연결 없는 격리망.
- **[사용자]** 오프라인 번들 반입 → 실행 → 결과 반출(사용자 절차).
- **결과**: T1만 성립(T2/T3 채널 부재), 배치형 수집.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-SIGN-1](../pkg/kernel/sign/sign_test.go) | unit | `TestSignVerify` — 서명 후 검증 | 왕복이 통과 | 검증이 정상 반입까지 막으면 격리망에서 아무것도 들일 수 없다 — 거부만 시험하면 이쪽을 놓친다 |
| [TD-SIGN-2](../pkg/inventory/ingest/central_test.go) | unit | `TestIngestSignatureReject` — 서명 검증에 실패한 결과를 적재 시도 | **거부**, 저장하지 않음 | 손댄 결과가 인벤토리에 들어오지 않게 한다 |
| [TD-SIGN-3](../pkg/kernel/sign/coverage_test.go) | unit | `TestTamperBreaksVerification` · `EdgeOrderDoesNotMatter` · `CanonicalCoversAllFields` — 필드를 하나씩 변조, 엣지 순서 뒤섞기, 계약 필드 수 가드 | 어느 필드를 건드려도 검증이 깨지고, 순서만 다른 같은 관측은 통과. 계약에 필드가 늘면 **실패** | 완전성 선언과 `raw_capture`까지 서명이 덮는지, 그리고 **서명 사각지대가 조용히 생기지 않는지** |
| [TD-SIGN-4](../pkg/inventory/ingest/central_test.go) | unit | `TestIngestAcceptsValidSignature` — 서명한 결과를 검증기와 함께 적재 | 거부 0, 수용 1, 스냅샷 1 | 거부만 시험하면 게이트가 정상 반입까지 막는 것을 못 잡는다 |
| [TD-SIGN-5](../pkg/kernel/sign/sign_test.go) | unit | `TestVerifyFromBindsKeysToCollectors` — A의 키로 서명한 결과에 **B의 collector 이름**을 달아 검증 | `Verify`는 통과시키고 `VerifyFrom`은 거절. 모르는 collector도 거절 | `Verify`는 넘긴 키를 전부 시도해 "누군가는 냈다"까지만 답한다. 서명은 **누가 냈나**를 답해야 한다 |
| [TD-ATTR-1](../pkg/discovery/procs/socket_test.go) | unit | `TestAttributionPicksTheProcessThatOpenedTheSocket` — 한 소켓을 부모와 자식 둘이 쥔 상태 | **연결을 연 부모**의 유닛으로 짚는다. 먼저 찾은 자식이 아니다 | fd는 상속된다. 실제 장비에서 한 inode에 PID 셋이 걸렸고, 첫 PID를 쓰면 앱을 잘못 짚게 된다 |
| [TD-ATTR-2](../pkg/discovery/procs/socket_test.go) | unit | `TestUnattributedIsNotNoApp` — 소켓이 닫힌 경우·안정 키를 못 뽑는 경우 | 빈 키 + **사유가 남는다** | 빈 `app_key`가 "이 통신에 앱이 없다"로 읽히면 안 된다. 관측 갭과 같은 규칙 |
| [TD-ATTR-3](../pkg/discovery/procs/socket_test.go) | unit | `TestAmbiguousIsNotGuessed` · `TestSameAppOnBothSocketsIsNotAmbiguous` — 같은 상대로 두 앱 / 한 앱이 연결 둘 | 앞은 **고르지 않고**, 뒤는 잡는다 | 앱을 잘못 짚으면 조치 대상이 바뀐다 — 비워 두는 것이 낫다. 다만 과하게 비우면 쓸모가 없다 |
| [TD-ATTR-4](../discovery/cmd/pqcota-netcap/note_test.go) | unit(linux) | `TestAttributionNoteSaysWhatItDoesNotMean` — 못 잡은 엣지가 있는 결과 | 완전성 노트에 건수·사유가 남고 **순서가 흔들리지 않는다** | 사유 순서가 흔들리면 같은 관측이 내용 지문 차이로 다른 스냅샷이 된다 |
| [TD-PROVENANCE-1](../discovery/collectors/network/collected_at_test.go) | unit | `TestEveryResultCarriesCollectedAt`(network·jvm) · `TestBuildResultCarriesCollectedAt`(openssl) — 세 collector가 내는 모든 결과 | 주입한 시계가 `collected_at`에 실린다. 관측 실패(`DegradedResult`)도 예외 아님 | 비어 있으면 서명이 빈 값을 덮는다 — "언제 봤는지 모른다"에 서명하는 것이다. 갭 기록도 **언제 시도했는지**가 근거다 |

> 오프라인 **번들 생성**(턴키 배포)은 하지 않는다 — 여기 테스트는 **임포트 시 provenance 서명 검증**(수신 측)만.

### TD-CNG. cng-collector — Windows CNG provider 관측 (검토 중인 설계 §2.2)

> **실물로 확인했다** — Windows 11 Pro 25H2(빌드 26200)에서 provider 9개·알고리즘 50개.
> `ML-DSA`는 있고 `ML-KEM`은 **없다**. 그 실행이 결함 둘을 함께 잡았다 — dwClass 매핑(TD-CNG-6)과
> 노드 식별(TD-CNG-7). 고친 뒤 재측정으로 둘 다 닫았다.
> 관측된 목록은 [collector README](collectors/cng/README.md)에 있다.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-CNG-1](collectors/cng/cng_test.go) | unit | `TestProviderOrderIsPreserved` — 관측 순서가 있는 provider 셋 | `pqcota:cng.provider_set`이 **그 순서 그대로** | **관측한 대로 적는다** — 정렬하면 관측을 고치는 것이 된다. (그 순서가 우선순위인지는 CNG에서 미확인: 실측에서 알고리즘 50개가 전부 provider 하나씩이라 다툼이 없었다) |
| [TD-CNG-2](collectors/cng/cng_test.go) | unit | `TestUnobservedIsNotAbsence` — 열거 실패 / 봤는데 0건 | 앞은 계층 미커버 + 사유 노트, 뒤는 **커버**로 센다 | 못 본 것과 없는 것을 같은 얼굴로 내보내면 "이 노드엔 CNG가 없다"로 읽힌다(§2.6) |
| [TD-CNG-3](collectors/cng/cng_test.go) | unit | `TestRawFormatEmptyWithoutRaw` — 원본이 없는 결과 | 형식 이름도 빈다 | 재정규화할 것이 없는데 있다고 적으면 §1.2의 약속이 거짓이 된다 |
| [TD-CNG-4](collectors/cng/cng_test.go) | unit | `TestAlgorithmsRideOnBothLanes` — 알고리즘까지 관측한 결과 | 파생 레인(`pqcota:cng.algorithms`)과 원본 **양쪽**에 남는다 | provider 이름 9개가 전부 Microsoft라, 알고리즘이 파생까지 가지 않으면 "이 노드가 ML-DSA를 하나"에 답할 수 없다 |
| TD-CNG-5 | **실물** — 확인 | Windows 11 Pro 25H2(26200)에서 `pqcota-cngscan --output json` | provider 9개가 **순서대로**, 알고리즘 50개. `CNG_INTROSPECTION` 커버, 노트 없음 | 스키마만 있고 채우는 코드가 없던 자리를 실측으로 닫는다 |
| [TD-CNG-6](collectors/cng/cng_test.go) | unit | `TestAlgorithmClassFollowsTheInterfaceConstants` — dwClass 1–7과 모르는 값들 | 1–7이 각 종류로, 그 밖은 **빈 값** | 실측에서 나온 결함이다: 열거 요청의 연산 비트마스크와 반환값의 인터페이스 상수는 다른 어휘인데 값이 겹쳐 오류 없이 틀렸다 |
| [TD-CNG-8](collectors/cng/cng_test.go) | unit | `TestUnknownClassKeepsTheAlgorithm` — 종류를 모르는 알고리즘 · 이름 없는 항목 | 앞은 **이름을 살리고** 종류만 비운다. 뒤는 세지 않는다 | 종류를 못 읽은 것이 알고리즘을 못 본 것이 되면 안 된다(§2.6). 빈 항목이 개수에 섞이면 집계가 거짓이 된다 |
| TD-CNG-11 | **실물** — 확인 | Windows 노드에서 `hardware_uuid` 관측 | `0CA88DB0-…`가 나오고 **Windows 자신이 보고하는 값과 같다**(`Win32_ComputerSystemProduct.UUID` 대조). `derived_from`은 `machine-id` 그대로 | SMBIOS는 앞 세 묶음이 리틀엔디언이라 되돌리지 않으면 같은 머신이 듀얼 부팅에서 다른 UUID로 보인다. 그리고 지문이 하나 늘어도 **node_id는 흔들리지 않아야** 한다(우선순위가 machine-id 먼저) |
| TD-CNG-12 | **실물** — 확인 | 알고리즘마다 `BCryptEnumProviders` | `ML-DSA`·`ECDH_P256`·`SHA256` 모두 `Microsoft Primitive Provider`. **50개 전부 provider가 하나씩** | 등록 목록은 "무엇이 있나"만 답한다. 조치 대상을 고르려면 **누가 그 알고리즘을 서비스하나**가 필요하다. 그리고 겹치는 provider가 없다는 것은 우선순위 전제가 CNG에서 서지 않는다는 뜻이다 |
| TD-CNG-9 | **실물 종단** — 확인 | 실기 결과를 `pqcota-ingest` → `pqcota-inventory` | `win_cng  confirmed  runtime_introspection  providers=9 algorithms=50 네이티브(서명만 — KEM 미관측)` · 이력 1건 | 관측이 화면까지 오지 않으면 적지 않은 것과 같다. 파생까지만 보고 닫으면 그리는 자리가 비어 있는 것을 못 잡는다 — 실제로 뷰 둘이 비어 있었다 |
| TD-CNG-7 | **실물** — 확인 | Windows 노드에서 `pqcota-cngscan` 재실행 | `machine_id`가 `MachineGuid`로 차고 `derived_from`=`machine-id`. 알고리즘 50개 **전부** 종류가 붙는다(빈 값 0) | 첫 실측에서 `fqdn`으로 떨어졌다 — 호스트명을 바꾸면 같은 머신이 다른 노드가 된다. `hardware_uuid`는 SMBIOS라 아직 빈다 |

### TD-NETWORK. network-collector — 통신 엣지 관측 (설계 §2.3, Phase 1)
> 다른 collector가 노드의 **능력**(로드된 lib)을 본다면, network-collector는 **실제 등급**(그 연결이 실제로 PQC로 협상됐나)를 본다. **책임은 협상 그룹 "관측"까지** — 등급 분류는 코어 파생(§1.2, `pkg/kernel/posture`). Finding이 아니라 **`ObservedEdge`**(인벤토리 인벤토리 설계 §6)를 채운다. 복호화 없이 핸드셰이크 평문만 관측. **구현**: `collectors/network/`(tls.go·ssh.go·dissect.go·edge.go·service.go·capture_linux.go). 라이브 캡처는 libpcap 없이 순수 Go AF_PACKET(`x/sys/unix`). 실측: TD-NETWORK-11=실 crypto/tls로 X25519MLKEM768 협상 관측, TD-NETWORK-12=로컬 OpenSSH 9.6 KEXINIT에서 sntrup761x25519 관측.

| 케이스 | 레벨 | Given → When | Then | 목적 |
|---|---|---|---|---|
| [TD-NETWORK-1](collectors/network/network_test.go) | unit | `TestParseClientHello` — ClientHello 레코드 | `supported_groups`/`key_share`에서 후보 KEX 그룹, role=client | 제안된 후보를 못 읽으면 협상 관측의 입력이 빈다 |
| [TD-NETWORK-2](collectors/network/network_test.go) | unit | `TestParseServerHello` — ServerHello 레코드 | 선택 그룹=X25519MLKEM768 + cipher + TLS version → `negotiated_group` | 인벤토리의 등급 근거는 제안이 아니라 합의된 결과여야 한다 |
| [TD-NETWORK-3](collectors/network/network_test.go) | unit | `TestParseSSHKexInit` — SSH KEXINIT | KEX 목록에서 `sntrup761x25519` 관측, protocol=SSH | SSH는 TLS와 프레이밍이 달라 같은 파서로 못 읽는다 |
| [TD-NETWORK-4](collectors/network/network_test.go) | unit | `TestNegotiateSSHKex` — 클라이언트는 제안, **서버는 미지원** | 협상 결과는 **고전** — 양쪽 교집합으로 판정 | 제안만 보고 PQC로 보고하면 레거시 서버(OpenSSH 8.2)와의 SSH가 🟢로 나간다 — 서버가 지원하지 않아 실제로는 고전인데도 |
| [TD-NETWORK-5](collectors/network/network_test.go) | unit | `TestBuildEdge` — 파싱 결과 → 엣지 | `ObservedEdge`{src_node · dst_addr:port · protocol · negotiated_group · role} | 관측이 자산과 이어지려면 노드·상대·프로토콜이 한 레코드에 함께 있어야 한다 |
| [TD-NETWORK-6](collectors/network/network_test.go) | unit | `TestQUICUnknownPosture` — 암호화된 핸드셰이크 | `negotiated_group`="" → **불명**(코어에서 등급 ⚪) | 관측하지 못한 것을 고전으로 단정하지 않는다 |
| [TD-NETWORK-7](collectors/network/network_test.go) | unit | `TestBuildResult` — CollectionResult 조립 | `crypto_runtime` **미상**, `layers_covered`=[NETWORK], `observed_edges` 채움 | 그 연결이 무엇으로 구현됐는지는 회선에서 안 보인다 — TLS 엣지를 OpenSSL 자산으로 잇지 않는다 |
| [TD-NETWORK-8](collectors/network/network_test.go) | unit | `TestBuildResult_windowNote` — 구간 안에 트래픽이 없던 링크 | completeness에 관측 구간 갭 note(§2.6) | 구간 안에 없던 것을 "암호를 안 쓴다"로 보지 않는다 |
| [TD-NETWORK-9](collectors/network/network_test.go) | unit | `TestShouldObserve_selfReference` · `TestCollect_filtersSelf` — 자기 노드/자기 트래픽 | 엣지에서 **제외** | 관측 도구가 만든 트래픽이 결과에 섞이면 토폴로지가 자기 자신을 가리킨다 |
| [TD-NETWORK-10](collectors/network/network_test.go) | unit | `TestBuildEdge_offScopeRawAddr` — dst가 스코프 밖 | `dst_node_id` 빈칸 + `dst_addr` 채움 → 코어가 등재 판정 | 모르는 상대라고 버리면 등재 판정의 입력이 사라진다 |
| [TD-NETWORK-11](collectors/network/network_test.go) | unit | `TestDescribe` — 능력 신고 | layers=[NETWORK], detection_methods=[runtime-introspection], **invasive=false**, Apache-2.0 | 신고가 실제와 어긋나면 완전성 계산과 침습성 판단이 함께 틀린다 |
| [TD-NETWORK-12](collectors/network/dissect_test.go) | unit | `TestDissectAndParse_TLS` · `_SSH` · `_IPv6` · `TestDissect_skipsNonTCP` — 프레임 → TCP 세그먼트 → 핸드셰이크 | IPv4·IPv6 모두 종단 디섹션, 비-TCP는 건너뜀 | 라이브 캡처 파이프라인의 순수 코어다. 여기가 막히면 파서가 옳아도 아무것도 도달하지 않는다 |
| [TD-NETWORK-13](collectors/network/dissect_test.go) | unit | `TestCollect_degradesOnCaptureUnavailable` — 소스가 `ErrCaptureUnavailable` | 노드별 완전성 갭 결과를 스트림 | 캡처가 불가할 때 크래시하거나 "없음"으로 보고하지 않는다 |
| [TD-NETWORK-14](collectors/network/capture_linux_test.go) | unit(linux) | `TestEdgeFor_clientOnly` — 양쪽 방향 관측 | **로컬이 클라이언트인 엣지만** client→server로 방출(낮은 포트=서버) | 같은 연결을 양쪽에서 두 번 세지 않는다 |
| [TD-NETWORK-15](collectors/network/capture_linux_test.go) | unit(linux) | `TestObserveFillsTheWholeWindow` — 원시 syscall이 시그널(SIGURG 선점)에 깨어 EINTR | 재시도하고 구간을 유지. 정상 종료면 `Truncated`가 서지 않는다 | 구간이 무작위 시점에 잘려 결과가 "핸드셰이크 없음"이 되는 것 — **결함이 갭으로 위장**(§2.6). 실측: 25초 구간이 0·0·14·25초에 종료 |
| [TD-NETWORK-16](collectors/network/capture_linux_test.go) | **integration** | `TestLiveSource_noCapPerm` — `CAP_NET_RAW` 없이 AF_PACKET | EPERM→`ErrCaptureUnavailable`(크래시 아님). 권한이 있으면 스킵 | 권한이 없을 때 갭으로 강등한다 |
| [TD-NETWORK-17](collectors/network/real_test.go) | **integration** | `TestRealTLSHandshake` — 실 crypto/tls 핸드셰이크 | ClientHello의 후보에 X25519MLKEM768, ServerHello의 `negotiated_group`이 X25519MLKEM768. 그 값이 코어에서 🟢(PQC_HYBRID)로 분류된다 | 손으로 만든 바이트가 아니라 진짜 와이어에서 파서가 도는지. 관측과 등급 파생이 실물에서 맞물리는지도 함께 본다 |
| [TD-NETWORK-18](collectors/network/real_test.go) | **integration** | `TestRealSSHKexInit` — 로컬 sshd(OpenSSH 9.6) | **제공 목록**에 `sntrup761x25519`가 있고, 단일 KEXINIT 관측이라 `negotiated_group`은 **비어 있다** | 〃 (실물 sshd). 서버가 제안한 것이지 합의된 것이 아니라는 구분이 실물에서도 지켜지는지 |
| [TD-NETWORK-19](collectors/network/network_test.go) | unit | `TestCollect_marksTruncatedWindow` — 구간이 중단된 소스(엣지 있음/없음) | 완전성 노트에 중단과 사유. **엣지가 하나도 없어도** 노드별 결과를 내보낸다. 중단 안 됐으면 안 뜬다 | 아무 말도 안 하면 "핸드셰이크 없음"으로 읽혀 결함이 갭으로 위장된다(§2.6) |

---

## 2. 구현 순서

| 순서 | 대상 | 케이스 |
|---|---|---|
| 1 | **fork 시그니처 매처** | TD-FORK-1 |
| 2 | **완전성 맵 로직** | TD-GAP-1 |
| 3 | **스코프 게이트·라우터** | TD-SCOPE-1 |
| 4 | **provider 레지스트리 매핑** | TD-JVM-1 |
| 5 | `/proc` 파서·경로 병합·intake 계약·결과 조립 | TD-OPENSSL-1–5 |
| 6 | **JVM 정찰·attach·다중 JVM 구별·정적 폴백** | TD-JVM-2–7·TD-JVM-10 |
| 7 | **서명과 그 커버리지** | TD-SIGN-1–3 |
| 8 | **network-collector 파서·엣지·디섹션·서비스** | TD-NETWORK-1–15 |
| 9 | 실 캡처·실 핸드셰이크 통합 | TD-NETWORK-16–18 |
| 10 | **실 호스트 수집**(OpenSSL·JVM attach·Windows 플레이북) | TD-OPENSSL-4·TD-JVM-8·TD-JVM-11·TD-JVM-12·TD-JVM-14·TD-WIN-1·TD-WIN-2 · [데모 2/6](../demo/integration-verification.md) |
| 11 | **관측하지 못한 것과 없는 것을 가르는 자리** | TD-OPENSSL-6 · TD-JVM-9·TD-JVM-13 · TD-CONTAINER-2 · TD-NETWORK-19 |

**관찰**: 순서 1–7이 전부 **unit** — 핵심 로직(정직한 증거·fork·라우팅·위임경계)이 실물 없이 TDD된다. 실물 의존은 리눅스가 필요하다. **가치 있는 로직을 먼저, 환경 리스크는 조기 PoC로.**
