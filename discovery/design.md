한국어 · [English](design.en.md)

# 디스커버리 서브시스템 설계 (Discovery Subsystem Design)

**문서 성격**: Discovery 서브시스템의 기술 설계. 상황별 인수 기준은 [테스트케이스](testcases.md)(SD-1–SD-7). 규정서 §2, [아키텍처 설계](../docs/architecture.md), [contracts SSOT](../contracts/)를 구현 레벨로 잇는다.
**범위**: Phase 0(읽기전용 Discovery + 정규화 + 인벤토리 뷰). 판단·remediation 없음.
**설계 규율**: 직접 만드는 것은 **런타임 레인 collector(SD-1·SD-2·SD-3) + 정직한 증거 계층**뿐이다. 소스·아티팩트는 CI가 볼 수 있는 영역이라 collector를 두지 않는다.


> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

**어느 규정을 구현하나** — 규정이 바뀌면 이 표로 고칠 절을 찾는다.

| 이 문서 | 규정서 |
|---|---|
| 1. 컴포넌트 아키텍처 | §2.1 목적과 경계 · §1.6 intake 계약 |
| 2. Collector 상세 설계 | §2.2 collector 지형 · §2.3 3계층 교차검증 · §2.3 소스 부재 |
| 3. 정규화 파이프라인 6단계 | §2.4 정규화 파이프라인 (원본 불변은 §1.2) |
| 4. 스코프 게이트 & 라우터 | §1.4 스코프 마스터 |
| 4A. 머신 식별·UID 체계 | §1.5 머신 식별·자산 계층·접근 경계 |
| 5. 완전성 맵 | §2.6 산출물/무결성 (갭≠부재는 §2.5) |
| 6. Collector 배포 | §4.3 단계적 배포 · §4.4 신뢰 경계 |

자동화 등급(AUTO/PROPOSE/MANUAL)은 §2.5이 정하고 이 문서는 그것을 전제로 만든다 — 등급 자체를
여기서 다시 정하지 않는다.

---

## 0. 관통 개념

- **관측 레인 vs 선언 레인** — collector가 *본 것*(observed)과 사용자가 *신고한 것*(declared/CMDB)을 Envelope의 `detection_method`로 구분해 라벨한다. 그 라벨이 인벤토리에서 둘을 가르는 근거가 된다.
- **`evidence_strength`** — `confirmed`(실행 중 실체) → `inferred`(정적 추정) → `unknown`. 관측 방법에 따라 결정론적으로 부착된다.
- **완전성 맵 (갭 ≠ 부재)** — 관측하지 못한 계층은 "없음"으로 단정하지 않고 **갭**으로 명시한다. 이 정직성이 감사 무결성·재현성의 근거다.
- **provider 시그니처 레지스트리** — provider·모듈 시그니처 → PQC 성숙도·FIPS·알고리즘 커버리지를 자동으로 판정한다.

---

## 0.1 상황을 가르는 축

시나리오는 아래 축의 조합이다. 조합이 곧 커버리지·증거강도·역할분담을 가른다.

| 축 | 값 | 영향 |
|---|---|---|
| 런타임 | OpenSSL / JCA(Java) | 수집법·스키마 분기 ([수용 원칙](../docs/runtime-acceptance.md)) |
| 증거 가용성 | 소스 / 아티팩트 / 바이너리만 / 실행 중 | `evidence_strength` (§2.3) |
| 호스트 환경 | 베어메탈·VM / 컨테이너 / 에어갭·배치 | 배포·네임스페이스 (§2.3) |
| 배포 티어 | T1 self-service / T2 substrate / T3 상주 | 신뢰장벽 (§2.3) |
| 스코프 상태 | 등재 / 미등재-관측 | 게이트 (§1.4) |
| 수집 결과 | confirmed / 열화 / unknown / 갭 | 완전성 맵 (§2.6) |

---

## 1. 컴포넌트 아키텍처

```
                    [스코프 마스터 게이트] ──(등재 노드만)──┐  §1.4
                                                          ▼
  ┌─ intake 계약(gRPC, contracts/collector.proto) ────────────────────┐
  │                                                                   │
  │  런타임 레인 (직접 만듦)                                          │
  │  ├ openssl-collector (Go)    SD-1·SD-3                                │
  │  │   /proc·ELF·ss·fork매처                                        │
  │  ├ jvm-collector (Java)      SD-2                                   │
  │  │   attach→getProviders()                                        │
  │  └ (네임스페이스 래핑)       SD-4          (선언 레인)               │
  └───────────────────────────┬───────────────────────────────────────┘
                              │  CollectionResult(Envelope+raw+CycloneDX+완전성)
                              ▼
   [정규화 파이프라인 6단계]  §2.4 ── 강화 입력: crypto-registry(provider 시그니처)
   raw수집→파싱→강화→검증→동일성해소→영속화
     │                              └ evidence_strength·fork판별·역할태깅 (정직한 증거 계층)
     ├─▶ [완전성 맵]  갭≠부재  SD-6
     ├─▶ [스코프 밖 라우터]  등재판정요청  SD-5
     ▼
   [디스커버리 히스토리] append-only, 재현가능(§1.2)  ──▶ 읽기전용 인벤토리 뷰
```

**계약 하나**: collector가 결과를 넘기는 창구는 `Collector` intake 계약(§1.6) 하나뿐이다. 그 창구 뒤에 무엇이 있는지를 **코어**(정규화·인벤토리·API를 맡는 중앙 서비스 — [아키텍처 §1.2](../docs/architecture.md#12-추천-조합))는 알지 못한다 — collector가 늘어도 코어는 그대로다. 결과가 어떻게 얻어진 것인지는 `Envelope.detection_method`로만 드러난다.

---

## 2. Collector 상세 설계

> **용어 규칙**: **Collector = *pqcota가 제공하는* 노드 관측기**(intake 계약 뒤, **런타임**: openssl·jvm·network).

> **플랫폼 제약**: **전 바이너리가 Go**(collector도 운영자 CLI도) — 유일한 비-Go는 jvm-collector의 Java 사이드카뿐이다. 그래서 **OS 갈림은 언어가 아니라 "무엇을 만지느냐"로 정해진다**:
> - **OS API를 만지는 것 = 그 OS 전용**. openssl(§2.1)·procs는 `/proc`·ELF, network(§2.3)는 AF_PACKET, cng(§2.4)는 `bcrypt.dll`이다. 다른 OS에는 **거부 스텁**을 둬 빈 결과가 아니라 갭을 낸다(§2.6).
> - **같은 축을 OS마다 다른 API로 보는 것도 있다**. jvm(§2.2)의 정찰이 그렇다 — 리눅스는 `/proc`, Windows는 Toolhelp32다. 이때 갈리는 것은 **커버의 깊이**이지 되고 안 되고가 아니다.
> - **파일·DB만 만지는 것 = 크로스플랫폼**. 중앙·운영자 CLI(ingest·inventory·provision 등)는 OS 프리미티브를 안 만져 어디서든 돈다.
>
> collector별 대응 OS는 [커맨드 레퍼런스](cmd/README.md)에 있다. 배포 바이너리는 정적(`CGO_ENABLED=0`)이라 OS×arch 교차 컴파일이 자명하다.

### 2.1 openssl-collector (Go) — SD-1, SD-3

**책임**: OpenSSL 런타임의 파일·프로세스·심볼 계층 수집. `ldd`/`lsof`/`ss`/`readelf`에 **의존하지 않고 Go로 자립 구현**(최소 이미지 대응, §2.3 노드에 남는 것).

| 계층 | 방법 | detection_method | 산출 |
|---|---|---|---|
| 패키지/파일 | dpkg/rpm DB 조회, FS walk, `debug/elf` NEEDED·역의존 | artifact | lib·version·경로 |
| 프로세스 | `/proc/<pid>/maps` 파싱(로드된 libssl, **dlopen·벤더링 포착**), `/proc/<pid>/fd`, netlink로 TLS 등급 | runtime-introspection | 실제 로드 |
| 심볼(정적/스트립) | ELF `.rodata` 문자열 + 심볼 시그니처 → **fork 매처**(§2.2) | symbol-analysis | fork·version 추정 |

> **provider 층은 보지 않는다.** 여기까지가 lib 경로·fork·버전이고, 그 위에 얹힌 provider(예: `oqsprovider`가 이미 깔려 있는지)는 관측 대상이 아니다 — jvm-collector가 attach로 provider 체인 전부를 보는 것(§2.2)과 갈리는 지점이다. 프로비저닝을 막지는 않는다: 어떤 provider를 쓸지는 계획에 사용자가 적고, 버전 근거는 인벤토리에 있다. 조치 뒤 재관측해도 이 층의 변화는 인벤토리에 나타나지 않는다.

**fork 시그니처 매처 (SD-3 핵심 IP)** — 동일 soname 문제(수용 원칙 §2.2) 해결:
```go
// crypto-registry의 fork 시그니처. OpenSSL/BoringSSL/LibreSSL/AWS-LC 판별.
type ForkSignature struct {
    Fork      string   // "OpenSSL" | "BoringSSL" | "LibreSSL" | "AWS-LC"
    Strings   []string // 예: "OpenSSL 3.0.2", "BoringSSL", AWS-LC 빌드 마커
    Symbols   []string // 내보낸/내부 심볼 이름 패턴
    Version   string   // 매칭 시 확정/추정 버전
}
// 매칭 실패 → OpensslAxes.Fork = "" (unknown, §2.5). evidence_strength=inferred-low.
```
**산출**: CycloneDX 컴포넌트(lib) + `pqcota:` properties(crypto_runtime=openssl, detection_method, openssl.fork, binding_mode) + raw_capture(네이티브 JSON) + Envelope. **Finding 파생은 코어**(§2.4 계약).

### 2.2 jvm-collector (순수 Java 사이드카) — SD-2 ★킬러

**책임**: 살아있는 JVM의 provider 체인 **실체** 조회. 전용 OSS 공백(§2.2)이라 자체 구현.

**attach 전략**:
```
1. JVM 정찰   : /proc 스캔으로 실행 중 JVM 열거 (ScanJVMs, 순수 Go)
                — exe=java 또는 maps에 libjvm.so, PID·JAVA_HOME·버전(release)·앱(cmdline) 추출
2. attach     : 발견된 각 PID에 VirtualMachine.attach → loadAgent(introspect-agent.jar)
                (전제: 동일 UID 또는 root, DisableAttachMechanism 아님)
3. 에이전트   : 대상 JVM 내부에서 실행 —
                Security.getProviders() → 각 provider {name, version, className,
                등록 순서, 서비스(algorithm) 목록} 수집 → 소켓/파일로 반환
4. detach     : 원상 복귀 (읽기 전용, 상태 변경 없음)
```

**정찰이 선행한다 — openssl(§2.1)과 대칭.** openssl의 `ScanHost`가 `/proc`를 훑어 로드된 lib를 스스로 찾듯, jvm도 `ScanJVMs`가 실행 중 JVM을 **직접 조사한다**(호출자가 PID·JDK를 미리 알아 넘기던 비대칭 제거). 접근 불가 프로세스는 `Denied`로 세어 완전성 갭의 원천으로 삼는다(§2.5). `AttachAll`이 발견한 각 JVM에 attach하고, **attach 실패도 조용히 버리지 않고 갭으로** 센다(openssl의 프로세스별 탐지 합산과 대칭). 구현: `collectors/jvm/{procscan,attach}.go`.

**다중 JVM 식별 — 앱 단위, PID 아님.** 한 노드에 JVM이 여럿이면 각각 **구별되는 finding**이어야 한다(하나가 dedup으로 사라지면 §2.6 정직성 위반 — 실제 자산 은폐). 식별자는 **앱**(cmdline의 main 클래스·`-jar`) 우선, 없으면 JAVA_HOME. **PID는 쓰지 않는다** — 매 스캔 달라져 finding id가 흔들리고 이력이 "매번 새 자산"으로 깨진다(같은 JDK의 두 앱도 앱 키로 갈린다).
**동봉물**: `introspect-agent.jar`(attach 사이드카)뿐. **런타임은 동봉하지 않는다** — attach 클라이언트는 대상의 java일 필요가 없어 **머신에 있는 JDK를 재사용**한다(대상이 JRE여도 무방). attach 가능 JDK가 전무하면 정적 폴백으로 정직히 강등 → [collector 배포 설계](collector-deployment.md).

**정책·아티팩트 병행 수집**(온호스트 파일):
- `java.security` 등록 순서 + `jdk.tls.*` + `disabledAlgorithms` 파싱.
- provider JAR 시그니처(`bcprov-jdk18on`·`BC-FJA`·`SunJCE`…) → crypto-registry 매핑.
- jvm-collector가 보는 것은 **런타임 실체 + 온호스트 정책**이다.

**열화 경로 (JEP 451 대응 — 판정됨)**: attach가 불가한 경우 —
`DisableAttachMechanism`, 또는 **JEP 451로 향후 JDK가 `-XX:+EnableDynamicAgentLoading` 없이 dynamic
attach를 기본 차단** — 3단 전략으로 대응한다:
1. **1순위**: dynamic attach → `getProviders()` 실체, `evidence_strength=confirmed`.
2. **보장 폴백**: attach 실패 시 **정적 경로**(java.security 등록 순서·JAR/classpath 시그니처 스캔)로
   열화 → `evidence_strength` 하향(confirmed→inferred), 완전성 맵에 "runtime-introspection 미수집"
   **갭 기록**(§2.5 "unknown 1급", 자동 부재 금지). **동적 등록(addProvider)은 이 경우 사각지대**로
   남고 그 사실 자체를 갭으로 명시한다.
3. **운영 옵션**: confirmed 증거가 필요한 고가치 자산은 배포 시 대상 JVM에
   `-XX:+EnableDynamicAgentLoading`(또는 `-XX:+StartAttachListener`) 기동 플래그를 **권고**(운영 협의).
   플랫폼이 강제하지 않는다 — 사용자 자산 기동 방식은 사용자 소유([[deploy-script-boundary]] 대칭).

> 비-agent 경로(JMX/JVMTI)는 후속 검토. JMX도 대개 비활성이고 JVMTI 네이티브 agent도 기동 플래그가
> 필요해 "레거시 지배 케이스"를 완전히 풀지 못한다. 그래서 **이 리포의 보장 폴백은 정적 열화**로 확정했다.

**provider 레지스트리 매핑** → `pqc_readiness`·`fips_validation`·알고리즘 커버리지. **SLH-DSA는 JDK 네이티브에 없음 → BC/jostle 의존 태깅**(§2.3).

### 2.3 network-collector (Go, AF_PACKET) — 네트워크 계층 (Phase 1)

**책임**: 실제 TLS/SSH 핸드셰이크를 **수동 관측**해 협상된 크립토와 통신 엣지를 잡는다(§2.2 네트워크 계층).
다른 collector가 노드의 **능력**(로드된 lib=PQC 가능 여부)을 본다면, network-collector는 **실제 등급**
(그 연결이 실제로 PQC 하이브리드로 협상됐나)를 본다. 이 둘의 대조가 핵심 가치다.

**기술적 근거 — 복호화 없이 관측**: 핸드셰이크의 알고리즘 협상은 **평문**이다.
- **TLS**: ClientHello `supported_groups`·`key_share`, ServerHello 선택 그룹 → KEX 그룹(X25519MLKEM768 등)·cipher·버전 관측.
- **SSH**: KEXINIT의 KEX 알고리즘(예 `sntrup761x25519` PQC 여부) — 레거시는 SSH 비중이 커 큰 가치.
- QUIC·TLS1.3 인증서 서명 등 암호화 부분은 관측하지 못함 → `unknown`으로 정직 표기.

**구현**:
```
캡처   : 순수 Go AF_PACKET(x/sys/unix, CAP_NET_RAW), BPF 필터로 핸드셰이크 레코드만(페이로드 제외 → 프라이버시)
파싱   : ClientHello/ServerHello, SSH KEXINIT → 협상 알고리즘·KEX 그룹
산출   : 통신 엣지(src→dst:port, negotiated_group, role, tls/ssh version)
반환   : CollectionResult(관측 레인). crypto_runtime=UNSPECIFIED(TLS를 OpenSSL로 단정할 수 없다, §2.2)
         — 노드 crypto Finding이 아니라 **통신 엣지**를 채운다(인벤토리 §2 ObservedEdge)
```

**권한·침습**: `CAP_NET_RAW`(openssl collector의 `CAP_SYS_PTRACE` 수준). **수동·비침습**이라 eBPF
dynamic-trace(PROPOSE)보다 가볍다. 단 데이터 평면을 건드리므로 handshake-only 필터 + 자기참조 회피(§2.6).

**한계(반드시 갭으로 표기)**:
- **관측 구간**: 캡처 중 흐른 트래픽만 — 유휴·배치·DR 링크 미관측 → 갭(≠부재, §2.6). 시간대별 반복(§2.3).
- **coverage 의존**: collector 도는 호스트의 연결만. 양쪽 다 미설치 엣지는 SPAN/탭 없으면 안 보임.
- **어느 노드인가**: 스코프 밖 IP·NAT·프록시 → "등재 판정 요청"(§5).

> **Phase 1 기능**(관측 병행 + `UNDECLARED` 엣지 발견 — 선언에 없는 통신). 이 엣지 관측이 인벤토리 reconciliation의
> 관측 소스가 되어 **크립토 통신 토폴로지**를 완성한다([인벤토리 설계](../inventory/design.md) §12).

### 2.4 cng-collector (Go, `bcrypt.dll`) — CNG 계층

**책임**: Windows에 등록된 CNG provider와 그 머신이 열거하는 알고리즘을 본다. provider 아키텍처라
jvm(§2.2)과 같은 축을 보되 수집 수단은 하나도 겹치지 않는다 — `/proc`도 ELF도 attach도 아닌
열거 API다. 그래서 **자기 계층**(`COLLECTION_LAYER_CNG_INTROSPECTION`)을 갖는다: 무엇을 못 봤는지가
다른 계층과 다르다.

**외부 도구를 부르지 않는다** — `certutil`·PowerShell·WMI 대신 `bcrypt.dll` 직접 호출(§2.3).
스크립트 실행이 정책으로 막힌 서버에서 관측 실패가 환경 탓으로 흩어지지 않는다.

관측 축과 API 대응은 → [cng-collector](collectors/cng/README.md).

---

## 3. 정규화 파이프라인 6단계 (전 시나리오 관통)

규정서 §2.4를 인터페이스로 고정한다. **강화·검증·동일성·영속화가 코어 단독**(§1.2 재계산 위해).

| 단계 | 입력→출력 | 핵심 로직 | 시나리오 |
|---|---|---|---|
| ①원시 포집 | Collector raw → append-only 저장 | 불변 보존, 재정규화 원천 | 전 |
| ②파싱/변환 | 네이티브 → 정규화된 CycloneDX | 스펙버전을 내부 정규 버전으로 수렴 | 전 |
| ③**강화** | CycloneDX+Envelope → Finding | `EvidenceStrength(method)` 결정론 부착, crypto-registry(fork·provider) 매핑, server/client 역할 태깅, pqc_readiness | SD-2·SD-3 핵심 |
| ④검증 | Finding → 검증된 Finding | 스키마 + 타당성(모순 플래그) | 전 |
| ⑤동일성/dedup | 노드=스코프 마스터 앵커, finding=정규화 해시 | 재수집 병합 | 전 |
| ⑥영속화 | → 디스커버리 히스토리 append | `derived_from_snapshot_id`+`ruleset_version` | §1.2 |

강화의 유일 소스는 코어다. Collector가 같은 로직을 중복 구현하지 않는다 → 규칙 개선 시 원본에서 재계산(§1.2).

---

## 4. 스코프 게이트 & 라우터 — SD-5

- **사전 게이트**: `CollectRequest.target_node_ids`는 코어가 스코프 마스터로 **이미 필터**한 것만(§1.4). Collector는 받은 것만 수집.
- **사후 라우팅**: 수집 중 관측된 스코프 밖 노드(통신 상대 등) → **수집 대상 아님** → "등재/제외 **판정 요청**" 큐로(PROPOSE). 자동 수집 금지.
- 산출: 신규 리뷰 항목(사용자 MANUAL 판정 대상). 스코프 마스터의 정합성 격차를 드러내는 것 자체가 서비스다.

## 4A. 머신 식별·UID 체계·자산 계층 (계약: `discovery/asset.proto`·`common.proto MachineIdentity`)

> 규칙은 **§1.5**가 정한다 — 신원·상관·위치 3층 분리, PID 미저장, 접근 비밀 미적재. 여기서는
> 그것을 무엇으로 구현하는지를 적는다.

### 4A.1 머신 UID — 3층 분리 (혼선·중복 없음)

신원(ID) / 상관(지문) / 위치(IP)를 섞지 않는다:

| 층 | 무엇 | 비고 |
|---|---|---|
| **권위 ID** | `node_id` = 스코프 마스터(CMDB) (§1.4) | **주 경로: 사용자가 discovery 시작 시 입력**(Ansible inventory/CMDB). 안정·유일 |
| **상관 지문** | `Envelope.MachineIdentity`: `machine_id`(/etc/machine-id)·`hardware_uuid`·`cloud_instance_id`·`fqdn` | 사용자 라벨을 **물리 머신에 앵커링·검증**(생성 아님) |
| **로케이터** | IP | ID 아님 — 네트워크 관측을 노드로 잇기만 |

- **자동 self-id (폴백)**: CMDB 없이 bare 실행 시 지문에서 **결정론적** 파생(`machineid.SelfAssign`: cloud>machine-id>hw>fqdn 우선순위 → `"node:"+sha256[:16]`). 같은 머신이면 같은 값이 나오므로 스캔마다 중복이 생기지 않는다. §1.4에 따르면 권위 ID가 아니므로 RegistrationRequest로 간다.
- **사용자 입력 중복/충돌 검증**: 사용자 node_id는 오류 가능 → 지문으로 교차검증(`ingest.CheckIdentity`). 한 물리머신키→여러 node_id=**중복**(한 머신 여러 이름), 한 node_id→여러 키=**충돌**(한 이름 여러 머신·재이미지). 판정하지 않고 드러내기만 한다(§2.5, 사람·reconcile 몫).

### 4A.2 자산 계층 Machine → Application → Process

식별 안정성이 계층마다 다르다:
- **Machine** = node_id (안정).
- **Application** = `(node_id, app_key)` — app_key는 머신 스코프 안정 키(systemd 유닛명·exe 경로·CMDB 선언). node_id가 전역 유일이라 **다른 머신 동명 앱과 충돌 없음.** Finding은 `app_keys`(복수)로 앱에 붙는다 — 보통 1개지만, host-wide 스캔에서 하나의 공유 라이브러리(예: `libcrypto.so.3`)를 **여러 앱이 로드하면 여러 앱에 걸친다**(`ScanHost`가 경로별 dedup 시 app_key를 합집합). 그 .so 교체는 로더 앱 전부에 영향이므로 하나로 뭉개지 않는다.
- **Process** = **PID 휘발 → 저장 안 함.** `ProcessMatch`(systemd_unit>exe_path>cmdline_regex)로 **프로비저닝 직전에 그때그때 이어 붙인다**(`LiveProcess`) — 저장된 PID는 이미 낡음.

### 4A.3 접근 — 사용자 hosts 파일 → Ansible (비밀 미영속)

- 사용자가 머신 접근 정보 파일(node_id·name·ip·port·계정·**키 또는 비밀번호**)을 **직접 관리**하고 **discovery 실행마다 지정**한다(`pkg/inventory.ParseHosts`, CSV 헤더 필수·순서 자유). 인증은 **호스트별 독립** — `ssh_key`(개인키 경로, 권장) 또는 `ssh_pass`(비밀번호, 지원하나 권장 안 함; Ansible 접속엔 컨트롤러에 `sshpass` 필요). 키 생성·등록(`ssh-keygen`·`ssh-copy-id`)은 [examples/discovery](../examples/discovery/README.md) 참고.
- pqcota는 그 파일을 읽어: (a) **런타임 전용** Ansible 인벤토리 생성(`RenderAnsibleInventory`, 비밀 포함·미영속)으로 실행, (b) **안전 부분집합만 인벤토리 적재**(`MachineEndpoint`: node_id·name·ip·port — 재사용·수정용).
- **접근 비밀(암호·SSH 키·계정)은 pqcota 인벤토리에 적재하지 않는다** — `MachineEndpoint` 타입에 비밀 필드가 아예 없어 컴파일 타임 보장. 비밀은 파일→Ansible 전달 후 버림.

## 5. 완전성 맵 — SD-6 (갭 ≠ 부재)

- Collector `Describe`(커버 가능 계층) vs `Collect`(실제 커버) **차이 = 갭**(`Completeness.layers_missing`).
- **배치/간헐 노드**: 수집 시점 미실행 → **갭 기록, 자동 "부재" 금지**(§2.5). 시간대별 반복 수집(§2.3)으로 dlopen·배치 누락 방지.
- 이 구분이 Inventory의 UNOBSERVED 판정에서 "실제 없음"과 "원리상 관측하지 못함"을 가르는 근거다(§2.6).

## 6. Collector 배포 (호스트 도달) — 시나리오별

아키텍처 §2.3 원칙을 시나리오에 매핑한다. **이 리포는 collector CLI와 결과 서명, 스코프 게이트만 두고 자체 push 엔진은 만들지 않는다**(§4.4).

| 시나리오 | 배포(호스트 도달) | 실체 |
|---|---|---|
| SD-1·SD-2·SD-3 (일반) | **T1 self-service** — 서명된 번들을 사용자가 실행(또는 자기 substrate로) | 서명 번들 + 원커맨드 실행 |
| SD-4 컨테이너 | 사이드카 주입 / hostPID | 네임스페이스 인지(collector 동일, 배포만 다름) |
| SD-7 에어갭 | **T1 오프라인 번들** | 서명 번들 반입 실행, 결과는 파일로 반출 |

> **T2/T3는 만들지 않는다**: 사용자 substrate(Ansible/Salt)용 플레이북·패키지 **생성기**(T2)와 상주 에이전트 push(T3)는 fleet 배포 운영 도구다. 이 리포는 collector를 **직접 실행할 수 있게**(T1 self-service·번들 서명) 제공하고, 대규모 롤아웃 생성은 별도 코드베이스가 담당한다.

### 6.1 collector 배포 저작권 ≠ remediation 저작권 (경계 원칙)

collector 배포(호스트 도달)를 누가 저작하든 Deploy의 [스크립트 경계](../provisioning/design.md)(§4.5) "스크립트 저작·서명은 사용자 몫"과 다르다 — 그건 *앱 재시작 로직이 사용자의 도메인 지식이자 책임*이라서다. **collector 설치는 read-only 바이너리를 놓는 일**이라 도메인 지식이 불필요하고 GPL 전염과도 무관(플레이북=데이터). 단 §2.3 **RCE 대칭성**으로 서명검증·최소권한·멱등은 T1부터 적용한다.

**T1 가드레일(self-service)**: ① 최소 caps — root 아님, `CAP_NET_RAW`(network-collector)·`CAP_SYS_PTRACE`(/proc)만. ② 번들 digest 핀 + 서명 검증 + 멱등(포크 가능한 투명 아티팩트). 번들이 올바른 호출(caps·co-location·버전 핀·재시도)을 한 번 인코딩한다. ③ 대상은 사용자 스코프 마스터(§1.4). 실행 주체가 pqcota가 되면 T3(상주 에이전트)가 되는데, 그건 만들지 않는다.

---

## 7. 시나리오 → 설계 요소 추적성 (satisfaction matrix)

**각 시나리오를 어느 컴포넌트가 만족시키는가** — 이 표가 설계 완결성의 증명이다.

| 시나리오 | 만족시키는 설계 요소 | 증거 결과 |
|---|---|---|
| **SD-1** OpenSSL 실행중 | openssl-collector(§2.1 프로세스+패키지) + 파이프라인 (배포=T1 self-service 또는 사용자 substrate) | confirmed |
| **SD-2** JVM attach | jvm-collector(§2.2 attach→getProviders) + provider 레지스트리 강화(§3③) | confirmed |
| **SD-3** 바이너리 열화 | openssl-collector fork 매처(§2.1) + `EvidenceStrength`(§3③) | inferred-low, unknown 명시 |
| **SD-4** 컨테이너 | 동일 collector + 네임스페이스 배포 + 완전성 맵 | 주입 성공=confirmed / 실패=갭 |
| **SD-5** 스코프 밖 | 스코프 게이트·라우터(§4) | 등재판정요청(PROPOSE) |
| **SD-6** 배치 노드 | 완전성 맵(§5) + 시간대별 반복 | 갭 기록(≠부재) |
| **SD-7** 에어갭 | T1 오프라인 번들 | T1 배치 수집 |

**빠진 것 없음 확인**: SD-1–SD-7 전부 최소 하나의 컴포넌트에 담기고, 새로 만드는 것은 §2.1–2.4(런타임 collector 3) + §3 파이프라인 + §4·§5 코어뿐이다.

---

## 8. 열린 설계 질문

- **JEP 451 — 동적 agent 로딩 제약**(대응 판정됨, §2.2 열화 경로): 향후 JDK가 `-XX:+EnableDynamicAgentLoading` 없이 기동된 JVM의 dynamic attach를 막을 수 있다. 이 리포는 정적 경로(java.security·JAR 스캔)로 열화 + 완전성 맵 갭 기록, **회복 탐지(비-agent 경로로 실체 조회)는 하지 않는다**. 남은 열린 질문: 비-agent 경로(JMX/로컬 JVMTI)로 `getProviders()` 실체를 어디까지 조회 가능한지(별도 collector 티어 상세).
- **jvm-collector 다중 UID**: 서로 다른 서비스 계정의 JVM들 → root로 seteuid 전환 attach vs per-UID collector 인스턴스. (보안·격리 트레이드오프)
- **fork 시그니처 레지스트리 시드**: 초기 커버 fork·버전 범위. 커뮤니티 기여 유인 지점(OSS).
- **완전성 맵 시간축**: 반복 수집 주기·갭 만료 정책(§2.3 "시간대별").
- **컨테이너 자동 탐지**: 네임스페이스 환경 자동 인지 → 사이드카 vs hostPID 자동 선택 범위.

---

## 부록 A. build vs reuse — 무엇을 새로 만들고 무엇을 쓰나

각 시나리오가 요구하는 능력을 "새로 만든다 / 기존 래핑 / 코어(신규)"로 판정한다. **이 표가 곧 개발 우선순위다.**

| 시나리오 | 필요 능력 | 기존 도구 | 새로 만들 것 | 신규성 |
|---|---|---|---|---|
| **SD-2** JVM attach | `getProviders()` 실체 | **없음**(§2.2 공백) | ★ `jvm-collector` | **높음·킬러** |
| **SD-3** 바이너리 | fork·version 시그니처 판별 | strings/readelf 원시만 | ★ **fork 시그니처 매처**(동일 soname 구분 수용 원칙 §2.2) | **높음·IP** |
| **전 시나리오** | evidence_strength·완전성 맵·provider 레지스트리·Envelope·히스토리 | 없음 | ★ **정직한 증거 계층 + 정규화 파이프라인** | **높음·코어** |
| **SD-5** 스코프 밖 | 게이트+판정요청 라우팅 | 없음 | 스코프 게이트·라우터(코어) | 중간·코어 |
| **SD-6** 배치 노드 | 갭 기록+반복 수집 | cron만 | 완전성 맵(갭≠부재) | 중간·코어 |
| **SD-1** OpenSSL 실행중 | `/proc`·ELF 조립 | 원시 유틸만 | `openssl-collector`(조립) | 낮음 |
| **SD-7** 에어갭 | 오프라인 서명 번들 | 서명 원시(cosign 등) | T1 번들러 | 중간 |
| **SD-4** 컨테이너 | 네임스페이스 인지 배포 | K8s 프리미티브 | SD-1/SD-2 collector 네임스페이스 래핑 | 중간·통합 |

**안 만드는 것**: 소스·아티팩트 스캔(CI가 볼 수 있는 영역)·eBPF dynamic-trace(침습적, Phase 0 제외).

**핵심 차별점은 3덩어리로 수렴**:
1. **SD-2** — JVM 인트로스펙션(유일한 전용 OSS 공백).
2. **SD-3** — 바이너리 fork 인텔리전스(동일 soname 판별 IP).
3. **정직한 증거 계층** — evidence_strength·완전성 맵(갭≠부재)·provider 레지스트리. *어떤 기존 도구에도 없고, §1.2 감사 무결성의 근거.*

나머지(SD-1 조립)는 저난도다. **차별화는 collector 개수가 아니라 런타임 실체 + 정직성에 있다**(부록 A: 관측성은 상품화 중).
