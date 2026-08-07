한국어 · [English](architecture.en.md)

# PQC 마이그레이션 플랫폼 — 아키텍처 설계

[규정서](regulation.md)가 정한 규칙을 **어떤 모듈·인터페이스·스키마로**
구현할지 확정한다.

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

**범위**: 3단계 종단 — Discovery(collector·정규화·history) → 중앙 인벤토리(뷰·엔드포인트·프로필·앱 귀속) → 프로비저닝 생성(L1/L2/L3 적용·롤백 플레이북·활성화 훅·롤백 레코드). 선언 대조·거버넌스와 플릿 오케스트레이션은 하지 않는다(아래 [§6 무판단 원칙](#6-무판단-원칙)).

> **규정서 핵심의 아키텍처 영향**:
> - **§4.3 단계적 배포 모델** = 위임 레벨 L1/L2/L3. **Ansible `copy` + sha256 게이트**로 구현한다
>   (모듈 파일은 사용자가 준비해 반입).
> - **수용 원칙 §2.3 provider 시그니처** = 디스커버리 강화 단계 입력. 살아 있는 출처는 `pkg/kernel/registry`.
> - **FIPS 라우팅** = `fips_validation` 요구가 FIPS 검증 provider **권고**로 나온다
>   (도구가 계획의 provider 선택을 막지는 않는다 — 검증서는 빌드 단위라 파일만 봐서 모른다).

> 설계 원칙: 규정서 §1(관통 원칙)은 코드에서도 불변 계약이다. 특히
> **원본 불변 + 파생 뷰**(§1.2), **Provenance Chain 4계열**(§1.3), **AUTO/PROPOSE/MANUAL 삼분**(§1.1),
> **intake 계약으로 Collector 은닉**(라이선스 정리)은 모듈 경계를 결정하는 1급 제약이다.

---

## 1. 기술 스택 (추천 조합)

### 1.1 결정을 강제하는 제약

스택은 취향이 아니라 **대상(런타임) 특성**이 강제한다. 규정서가 요구하는 수집 능력을 나열하면 언어가 거의 자동으로 결정된다.

| 요구 수집 능력 (규정서 근거) | 기술적 제약 |
|---|---|
| `/proc/*/maps`·`lsof`·`ss`, `ldd`/`readelf` (§2.3 OpenSSL) | 시스템 콜·네이티브 툴링. Go/Rust/C 계열 |
| 정적 ELF 심볼·문자열 시그니처로 fork·version 판별 (§2.3, §2.3) | ELF 파서. Go(`debug/elf`)·Rust(`goblin`) 둘 다 강함 |
| eBPF 동적 트레이스 (§2.2 crypto-tracer, §2.3 dynamic-trace) | **eBPF 로더 생태계 = Go(cilium/ebpf)가 사실상 표준** |
| **JVM attach → `Security.getProviders()` 실체 조회 (§2.2, §2.3)** | **JVM 내부에서만 가능 — JVM 강제(플랫폼 언어 Java). 우회 불가** |
| CycloneDX CBOM(ECMA-424) 입출력 (§2.4, §3.2) | 성숙 라이브러리 필요. JVM·JS·Go 순으로 성숙 |
| Ansible/Salt substrate 오케스트레이션 (§4.4) | 서브프로세스·SSH. 언어 무관, Go 편함 |
| 리뷰 큐·인벤토리 대시보드 UI (§3.7) | TypeScript/React |

**핵심 관찰**: JVM 인트로스펙션(§2.2 "자체 구현 공백 영역")은 **어떤 언어로도 우회 불가 — 반드시 JVM 안에서 실행**되어야 한다. 이것이 폴리글랏을 불가피하게 만든다. 나머지 시스템 수집은 Go 하나로 전부 커버된다.

### 1.2 추천 조합

| 레이어 | 언어/기술 | 근거 |
|---|---|---|
| **코어 서비스** (정규화·리뷰 큐·인벤토리·API) | **Go** | 단일 정적 바이너리 배포, gRPC, 동시성, 시스템 툴링, 허용적 라이선스(전염 없음) |
| **OpenSSL/시스템 Collector** | **Go** | 코어와 동일 언어. `/proc`·ELF(`debug/elf`)·eBPF(`cilium/ebpf`) 전부 네이티브 |
| **JVM Collector** (별도 사이드카) | **Java**(순수) | JVM Attach API(JVMTI/Attach)로 살아있는 JVM에 붙어 `getProviders()` 조회. **불가피한 폴리글랏 지점은 JVM**(언어 아님) — 플랫폼 언어 Java로, Kotlin·Gradle 없이 `javac` 빌드 |
| **eBPF 프로그램** | **C → Go에서 로드** | CO-RE, cilium/ebpf로 주입 |
| **UI** | **TypeScript + React** | 리뷰 큐·리컨실리에이션 뷰 표준 |
| **저장소** | **PostgreSQL** (JSONB) | append-only 히스토리 4계열 + CBOM JSONB. 이벤트소싱 친화 |
| **런타임 간 계약** | **gRPC + Protobuf** (+ CLI/stdout 폴백) | intake 계약(라이선스 정리)·서브프로세스 격리(라이선스 정리)를 동일 메커니즘으로 |

**한 줄 요약**: **Go 코어 + Go 시스템 Collector + JVM Collector 사이드카(순수 Java) + C/eBPF + TS/React UI + Postgres.** — 강제되는 건 *JVM*이지 특정 언어가 아니다(폴리글랏 지점=JVM). 사이드카는 플랫폼 언어 Java로 쓰고 Kotlin·Gradle 의존이 없다.

### 1.3 왜 Go 코어인가 (Rust 대비)

- **eBPF 생태계 정렬**: `cilium/ebpf`가 사실상 표준. 규정서 §2.2의 crypto-tracer 계열과 직결.
- **배포 단순성**: 온호스트 에이전트가 단일 정적 바이너리 → 규정서 §4.4 "자기잠금 회피"에 유리.
- **오케스트레이션·클라우드네이티브 정렬**: Ansible/Salt 서브프로세스·mTLS·gRPC 관성.
- **Rust는 어디에?** ELF 심볼 분석기(§2.3)는 순수하고 고립된 모듈이라 **나중에 Rust로 교체 가능한 명확한 후보**. intake 계약만 지키면 되므로 코어를 건드리지 않고 갈아끼울 수 있다. 지금은 Go로 시작하고 필요 시 격상.

---

## 2. 시스템 아키텍처

### 2.1 모듈 맵 — 규정서 3단계 + 관통 원칙의 코드 투영

```
                         ┌─────────────────────────────────────────────┐
                         │  스코프 마스터 (CMDB/자산 등록부) — §1.4       │
                         │  관리 대상 경계의 유일 권위 소스. 모든 게이트   │
                         └───────────────────┬─────────────────────────┘
                                             │ (대상 노드 목록)
   ┌── DISCOVERY (§2) ───────────────────────▼─────────────────────────┐
   │                                                                    │
   │  [Collector Framework] ── intake 계약(라이선스 정리) ─┐                      │
   │    ├ openssl-collector (Go)                 │  "노드 → 정규화된 CBOM"│
   │    ├ jvm-collector (순수 Java 사이드카)      │  코어는 백엔드를 모름  │
   │    ├ network-collector (Go)                 │                      │
   │    └ [gpl-adapter] CipherIQ/CBOMkit 서브프로세스 ─┘ ← GPL 격리 경계   │
   │                        │                                           │
   │            [정규화 파이프라인 6단계 — §2.4]                          │
   │   raw수집→파싱→강화→검증→동일성해소→영속화                            │
   │              └ 강화 입력: [crypto-registry] provider 시그니처 레지스트리 │
   │                (§2.3) provider JAR/모듈 → pqc_readiness·fips·알고리즘     │
   │                        │                                           │
   └────────────────────────┼───────────────────────────────────────────┘
                            ▼
              ╔═══════════════════════════════════╗
              ║  ① 디스커버리 히스토리 (상태 계열)  ║  ─┐
              ╚═══════════════════════════════════╝   │
   ┌── INVENTORY (§3) ──────────────────────────────┐ │  Provenance
   │  [Reconciliation Engine] 3-상태 대조 (§3.3)     │ │  Chain 4계열
   │    CONFIRMED / UNDECLARED / UNOBSERVED          │ │  (§1.3)
   │  [Confidence Scoring] (AUTO)                    │ │  상태→판단
   │  [Review Queue] 우선순위화 (PROPOSE)            │ │   →의도→행위
   │  [Decision Service] draft→in-review→finalized   │ │
   │    판정 영속화·델타 리뷰 (§3.6)                  │ │
   └────────────────────────┬───────────────────────┘ │
              ╔═══════════════════════════════════╗   │
              ║ ② 결정·계획 히스토리 (판단·의도)   ║ ─┤
              ╚═══════════════════════════════════╝   │
   ┌── PROVISIONING 생성 (§4) ──[finalized 계획만]──┐ │
   │  [계획→config 생성기] taxonomy→L1/L2/L3 플레이북 │ │
   │    (프로비저닝 설계 §4.1·§4.2) · before 캡처 · 롤백 레코드 영속  │ │
   │  [Substrate Adapter] Ansible/Salt (L1·L2, §4.4) │ │
   └────────────────────────┬───────────────────────┘ │
              ╔═══════════════════════════════════╗   │
              ║ ③ 프로비저닝 히스토리 (행위 계열)  ║ ─┘
              ╚═══════════════════════════════════╝
                            │ 재스캔 폐루프 → Discovery (§5)
                            ▼
   [Provenance/Audit Service] — 4계열을 인과 사슬로 봉합, 불변 감사 로그 (§1.3, §4.7)
   [API Gateway] + [Web UI] — 리뷰 큐·인벤토리·감사 뷰
```

### 2.2 데이터 흐름의 불변식 (§1.2 강제)

- **원본은 append-only.** `raw_capture`(collector 네이티브 출력)와 3개 히스토리는 절대 in-place 수정 금지.
- **정규화 결과·리컨실리에이션·확정 계획은 전부 파생 뷰** — 원본에서 재계산 가능해야 한다. 강화 규칙이 바뀌면 `raw_capture`에서 재실행.
- 코드에서: 파생 테이블은 `derived_from_snapshot_id` + `ruleset_version`을 항상 보유 → 재현 가능성 보장.

### 2.3 Collector 호스트 도달 — 원칙과 경계

**collector는 `CollectionResult`를 emit하는 CLI다**(`pqcota-nodescan`·`pqcota-jvmscan`·`pqcota-netcap`). 배포는 표준 substrate(Ansible)로 한다 — 디스커버리 실행 시 관측 대상 노드에 반입·실행·회수하고 잔재를 남기지 않는다([collector 배포 설계](../discovery/collector-deployment.md)). **자체 원격 실행 엔진은 만들지 않는다.**

- **이 리포**: collector CLI + **T1 self-service**(서명된 collector 번들을 사용자가 직접 실행 — 에어갭 포함) + **결과 서명·검증**(ed25519, `pqcota-keygen`·`PQCOTA_VERIFY_KEY`) + **스코프 마스터 게이트**(§1.4, `pqcota-ingest`가 등재 노드만 수용). collector를 사용자 자신의 substrate로 감싸 돌릴 수도 있다. 릴리스·번들 서명(공급망 위생)은 여기 속한다.
- **원칙(불변)**: 어느 경로든 **스코프 게이트 필수** + **RCE 대칭성**(레거시 호스트에 실행체 투입은 위험하므로 서명검증·최소권한·멱등). 부가가치는 push 채널 소유가 아니라 그 위의 게이트·서명·완전성 맵.

**호스트 발자국 (Phase 0 최소)** — [수용 원칙 §2.2 스택] 근거와 직결:
- **OpenSSL 노드**: Go 정적 바이너리 1개 + root/`CAP_SYS_PTRACE` + mTLS 자격. 그 외 의존 0(ELF·/proc 자립 파싱, `ldd`/`lsof`/`ss`/`readelf` 비의존 설계).
- **Java 노드**: **Go 바이너리 + 인트로스펙션 agent JAR**만 올리면 된다 — attach는 OS IPC(트리거 파일+SIGQUIT+유닉스 소켓)라 **JDK 없이 직접** 붙는다(대상이 순수 JRE·jlink 런타임이어도). **동일 UID/root** 필요. HotSpot이 아니면(OpenJ9) 머신의 JDK를 클라이언트로 쓰는 경로로, 그마저 막히면(`DisableAttachMechanism`·JEP 451) 정적 경로로 열화 → `evidence_strength` 하향(§2.3). 3계층 상세: [jvm-collector README](../discovery/collectors/jvm/README.md).
- **컨테이너 주의**: `/proc`·JVM attach는 **같은 PID/마운트 네임스페이스**에서만 → host PID namespace 또는 사이드카 주입 필요(실배포 최대 함정).
- **오프호스트로 미룰 것**: 네트워크 스캔(중앙 원격), 아티팩트/소스 스캔(CI·리포), eBPF dynamic-trace(PROPOSE·Phase 0 제외).

---

## 3. 핵심 데이터 모델 — 정규화된 CBOM Envelope

규정서 §3.2가 확정한 **"CycloneDX CBOM(표준 본문) + Envelope(provenance) + evidence 메타데이터(확장)"** 를 코드 스키마로 고정한다.

### 3.1 Envelope(Envelope) 스키마

```jsonc
// 하나의 Collector 실행이 반환하는 정규화된 산출물
{
  "envelope": {                          // §2.6 provenance — Envelope
    "collector_id": "openssl-collector",
    "collector_version": "0.1.0",
    "detection_method": "runtime-introspection", // §2.3 열거형(아래)
    "collected_at": "2026-07-07T05:00:00Z",
    "target_node_id": "cmdb://node/8f3a...", // 스코프 마스터 앵커 (§1.4)
    "signature": "ed25519:...",          // collector 서명 (§2.6 무결성)
    "scope_master_ref": "cmdb-snapshot://2026-07-07"
  },
  "cbom": { /* CycloneDX CBOM ECMA-424 표준 본문 — 내부 정규 버전으로 수렴 */ },
  "findings": [ /* §3.2 evidence 메타데이터가 부착된 정규화된 finding[] */ ],
  "completeness": {                       // §2.6 완전성 맵 — collector별·계층별 분리
    "layers_covered": ["artifact", "runtime-introspection"],
    "layers_missing": ["process"],        // "실제 없음" ≠ "원리상 관측하지 못함" 구분 (§2.6)
    "note": "프로세스 계층 미수집 — 대상 미실행"
  }
}
```

### 3.2 Finding 스키마 (런타임 추상 — 수용 원칙 §2.4, §2.4)

```go
// 런타임 무관 1급 필드 + 런타임별 분기 필드
type Finding struct {
    ID             string        `json:"id"`              // 정규화 해시 (dedup 앵커, §2.4)
    CryptoRuntime  string        `json:"crypto_runtime"`  // "openssl" | "jca"  ← 분기 결정 (수용 원칙 §1)
    UsageContext   string        `json:"usage_context"`   // server|client|at-rest|signing
    Algorithm      *string       `json:"algorithm"`       // nil 가능 (소스 부재 시 열화, §2.3)

    // ─ evidence 메타데이터 (§2.3 핵심) ─
    DetectionMethod  string      `json:"detection_method"` // 열거형 ↓
    EvidenceStrength string      `json:"evidence_strength"`// confirmed|inferred-high|inferred-low
                                                           // detection_method에서 결정론적 파생 (§2.5 AUTO)
    // ─ 런타임별 분기 (수용 원칙 §2.4) — oneof ─
    OpenSSL *OpenSSLAxes `json:"openssl,omitempty"`
    JCA     *JCAAxes     `json:"jca,omitempty"`

    // ─ 공통 판정 축 ─
    PQCReadiness    string  `json:"pqc_readiness"`
    FipsValidation  string  `json:"fips_validation"`
    RemediationClass string `json:"remediation_class"`
}

type OpenSSLAxes struct {
    Lib         string `json:"lib"`          // libssl/libcrypto
    Version     string `json:"version"`
    Fork        string `json:"fork"`         // OpenSSL|BoringSSL|LibreSSL|AWS-LC ("unknown" 허용)
    BindingMode string `json:"binding_mode"` // dynamic|static|dlopen|vendored
}

type JCAAxes struct {
    JDKVendor        string   `json:"jdk_vendor"`
    JDKVersion       string   `json:"jdk_version"`
    ProviderSet      []string `json:"provider_set"`      // 등록 순서 포함 (우선순위 협상 판정 근거, 수용 원칙 §2.2)
    RegistrationMode string   `json:"registration_mode"` // static|dynamic|explicit
}
```

**`detection_method` → `evidence_strength` 매핑 (규정서 §2.3 표를 결정론적 함수로)**:

```go
// §2.5: evidence_strength 부착은 AUTO(결정론적). 이 함수가 유일 소스.
func EvidenceStrength(method string) string {
    switch method {
    case "source":                 return "confirmed"      // Hyperion 등, algorithm+usage 완전
    case "runtime-introspection":  return "confirmed"      // /proc·getProviders()
    case "dynamic-trace":          return "confirmed"      // eBPF/ltrace, 실제 호출
    case "artifact":               return "inferred-high"  // Theia·JAR 스캔
    case "symbol-analysis":        return "inferred-low"   // 정적 바이너리, usage 없음
    default:                       return "unknown"        // §2.5 "unknown도 1급 증거"
    }
}
```

**불변 규칙 (§2.5)**: 채우지 못한 필드는 `nil`/빈값이 아니라 명시적 `"unknown"`. 자동 "부재" 처리 금지.

**스키마 반영 (필드가 어느 단계에 속하는가)**

- **`FipsValidation`는 이미 있음** — 규제 자산 FIPS 라우팅에서 이 필드가 **provider 선택을 강제**한다. Discovery는 값을 채우기만 하고(강화 단계), 라우팅 판정은 하지 않는다. `JCAAxes.ProviderSet`을 crypto-registry(§2.3)와 대조해 `pqc_readiness`·`fips_validation`을 파생한다.
- **`ProviderSet` → provider 시그니처 레지스트리 매핑**: 강화 단계에서 `bcprov-jdk18on`/`BC-FJA`/`JDK-native`/`openssl-jostle`/내부를 식별해 알고리즘 커버리지(특히 **SLH-DSA는 JDK 네이티브에 없음**)를 태깅. → §3.3 신규.
- **`deploy_automation_level`(L1/L2/L3)은 Finding 필드가 아니다** — Discovery 산출물이 아니라 **리뷰어가 자산별로 정하는 계획·자산 속성**(§4.3, MANUAL). 확정 계획(plan) 엔티티에 속한다. 단, 통제 어휘로서 SSOT(contracts)에는 등재한다(§3.3·contracts 참조).

### 3.3 provider 시그니처 레지스트리 (§2.3) — 강화 단계 참조 데이터

Discovery 강화(§2.4 step 3)가 참조하는 결정론적 매핑 테이블. **파생 규칙이므로 개선 시 원본에서 재계산**(§1.2), `ruleset_version`으로 버전 고정.

```go
// provider 시그니처 → 능력. Discovery 강화가 JCAAxes.ProviderSet과 대조.
type ProviderSignature struct {
    Match          string   `json:"match"`           // 예: "bcprov-jdk18on-*", "BC-FJA", "JDK-native>=24"
    Nature         string   `json:"nature"`          // pure-java | fips-native | jdk-builtin | jni-bridge | internal
    PQCAlgorithms  []string `json:"pqc_algorithms"`  // ["ML-KEM","ML-DSA","SLH-DSA"] — JDK 네이티브는 SLH-DSA 없음
    FipsValidation string   `json:"fips_validation"` // "140-3" | "none" | "jdk-dependent" | "module-dependent"
    LicenseClass   string   `json:"license_class"`   // "permissive"(BC 표준) | "fips-contract"(BC-FJA) | "gpl" | "internal"
}
// 규정: SLH-DSA 필요 자산은 JDK 버전 무관하게 BC/jostle 의존으로 태깅 (§2.3).
// LicenseClass="permissive"(BouncyCastle 표준판, MIT계열)는 GPL 격리 대상 아님 (프로비저닝 설계 §4.2).
```

### 3.4 PQC 알고리즘 성숙도 레지스트리 (§2.3 참조 데이터) — remediation 라우팅 입력

§3.3이 **provider의 능력**(어떤 알고리즘을 구현하는가)을 다룬다면, 이 절은 **알고리즘 자체의 표준화 성숙도**(그 알고리즘을 써도 되는가)를 다룬다. 관측된 협상 그룹(`negotiated_group`, network-collector)·provider 알고리즘 이름을 이 표와 대조해 성숙도를 파생하고, remediation 분기의 입력으로 쓴다. **파생 규칙**(§1.2)이며 이 리포의 공개 참조 데이터(`pkg/kernel/registry`).

`pkg/kernel/registry/pqc.go` — 4단계 성숙도(§NIST PQC 표준화 현황 기준):

| 성숙도 | 의미 | 예 | `FIPSValidatable()` |
|---|---|---|---|
| `fips-standard` | FIPS 203/204/205 최종 표준(2024.08) | ML-KEM · ML-DSA · SLH-DSA | ✅ |
| `draft` | 표준 전신/진행 중 | Kyber · Dilithium · SPHINCS+ · Falcon · HQC | ❌ |
| `experimental` | 연구·대안(비FIPS) | BIKE · FrodoKEM · McEliece · sntrup761 · MAYO · CROSS | ❌ |
| `broken` | 파훼됨 | Rainbow · GeMSS · SIKE | ❌ |

`MatchPQC(name)`은 협상 그룹/알고리즘명을 정규화(대문자·구분자 제거)해 부분문자열 매칭 → `(PQCAlgorithm, ok)`. 예: `X25519MLKEM768`→ML-KEM(fips), `sntrup761x25519-sha512@openssh.com`→NTRU-Prime(experimental), `x25519`→(false, 고전).

**성숙도 축은 등급 축과 직교한다** — `pkg/kernel/posture`의 "PQC냐 고전이냐"(🟢/🔴/⚪, §1.6) 위에 "표준이냐 실험이냐"를 더한다. `posture.Grade(group)`→성숙도, `posture.GradeLabel`→표준/초안/실험/취약 라벨(뷰 표기). 의존은 단방향(posture→registry).

**remediation 분기** — `registry.Remediation` + `PQCAlgorithm.Remediate(regulated)`가 성숙도를 조치로 라우팅하고, `posture.Recommend(group, cipher, regulated)`가 엣지 하나에 대한 종합 권고를 낸다(고전·미관측 포함):

| 입력 등급/성숙도 | Action | Priority | 근거 |
|---|---|---|---|
| PQC 표준 | `none` | 0 (규제=1) | 유지 — 규제 자산은 FIPS 검증 provider 확인(§3.3) |
| PQC 초안 | `upgrade` | 2 | 최종 표준(ML-KEM/ML-DSA)으로 상향 |
| PQC 실험 | `replace` | 3 | 표준으로 교체 |
| PQC 파훼 | `replace` | 4 | 즉시 교체 |
| 고전(🔴) | `migrate` | 3 (규제=4) | 양자취약(HNDL) — PQC 하이브리드 도입 |
| 미관측(⚪) | `none` | 0 | 판단 보류(인벤토리 설계 §6.2 정직성 — 안 본 걸 단정 안 함) |

목표 표준은 종류별로 갈린다 — KEM→ML-KEM(FIPS 203), 서명→ML-DSA(FIPS 204). 이 권고는 파생일 뿐 실행이 아니다.

---

## 4. Collector Intake 계약 (§1.6 — 플러그형 인터페이스)

코어의 유일한 Collector 의존성. **"노드를 주면 정규화된 CBOM Envelope를 반환한다"** 만 안다. 백엔드가 자체 collector/CipherIQ/CBOMkit인지 **몰라야 한다.**

### 4.1 인터페이스 (gRPC)

```protobuf
service Collector {
  // 능력 신고 — 코어가 완전성 맵·계층 커버리지 판단에 사용
  rpc Describe(DescribeRequest) returns (CollectorCapabilities);
  // 수집 실행 — 정규화된 CBOM Envelope 스트림 반환
  rpc Collect(CollectRequest) returns (stream CollectionResult);
}

message CollectorCapabilities {
  string collector_id = 1;
  string version = 2;
  repeated string crypto_runtimes = 3;   // ["openssl"] | ["jca"] | ...
  repeated string layers = 4;            // source|artifact|process|network|jvm-introspection
  repeated string detection_methods = 5; // §2.3 열거형
  string license = 6;                    // "Apache-2.0" | "GPL-3.0-or-later" ← 라이선스 정리 UX 표기용
}

message CollectRequest {
  repeated string target_node_ids = 1;   // 스코프 마스터 게이트 통과분만 (§1.4)
  ScopeMasterRef scope = 2;
  map<string,string> options = 3;        // dynamic-trace 등 침습 옵션 (PROPOSE 게이트 별도)
}

message CollectionResult {
  Envelope envelope = 1;                 // §3.1
  bytes cbom_cyclonedx = 2;              // 표준 CycloneDX JSON
  repeated Finding findings = 3;
  Completeness completeness = 4;
}
```

### 4.2 계약의 3대 불변식

1. **출력은 항상 정규화된 CBOM Envelope** — 어느 백엔드든 downstream(reconcile·리뷰·프로비저닝)이 백엔드 무관하게 동일 동작(라이선스 정리).
2. **스코프 마스터 게이트는 코어 책임** — Collector에 넘기기 전 코어가 대상 노드를 필터(§1.4). Collector는 받은 노드만 수집.
3. **완전성은 계층별로 신고** — Collector가 `Describe`로 커버 계층을 선언하고 `Collect`로 실제 커버를 보고. "관측하지 못한 것"을 코어가 갭으로 기록(§2.6).

### 4.3 GPL 격리와 동일 메커니즘

이 gRPC/CLI 경계가 곧 **GPL 전염 차단 경계**다. GPL collector(CipherIQ `cbom-generator` 등)는 **별도 프로세스로 실행 + stdout CycloneDX만 교환** → 라이브러리 링크 없음. `gpl-adapter`가 서브프로세스를 감싸 intake 계약으로 노출한다. → **라이선스 정리의 격리 3원칙이 코드에서 자동 성립.**

---

<a id="repo-scope"></a>

## 5. 리포 구조

```
pqcota/            # Apache-2.0 · 공개 · 전 범위(Discovery·인벤토리·프로비저닝 생성)
  # ── 최상위 = 산출물 종류(kind) / 단계는 그 안에서 (contracts·pkg는 단계 그룹, discovery/·… 는 실행 진입점) ──
  ├─ contracts/proto/pqcota/{common,discovery,inventory,provisioning}/v1/  # 계약 SSOT — 네임스페이스=단계
  ├─ gen/               # protobuf 생성 코드 (gitignore — make generate로 만든다)
  ├─ pkg/               # 라이브러리 로직 — 단계 그룹:
  │    ├─ discovery/    #   관측 레인: normalize(§2.4)·history(§2.4⑥ 스냅샷 스토어)
  │    ├─ inventory/    #   ingest(적재·CBOM 수신 SV-2)·중앙 뷰(§5)·머신 메타데이터 저장소(엔드포인트·프로필 upsert)·hosts 파서 + declaration(선언 레인). 대조·판정 엔진은 없다
  │    ├─ provisioning/ #   확정계획 게이트(§3.7)·taxonomy→config 생성기(프로비저닝 설계 §4.1·§4.2)·L1/L2 플레이북·before 캡처·롤백 레코드 저장소. 생성·영속까지
  │    └─ kernel/       #   단계 가로지르는 공유 규칙: registry·posture·scope·machineid·sign
  ├─ discovery/         # 실행 진입점(단계별):
  │    ├─ collectors/{openssl(Go),jvm(Java 사이드카 ★),network(Go)}  # §1.6 플러그인·GPL 격리 경계
  │    └─ cmd/{pqcota-hosts(접근준비),nodescan,netcap,jvmscan,procs,keygen}  # (테스트 하네스는 collectors/openssl/integration/probe)
  ├─ inventory/cmd/     # pqcota-ingest(적재) · pqcota-cbom-ingest(CBOM 수신) · pqcota-discover-view(파일 뷰) · pqcota-inventory(중앙 Postgres 조회: 엔드포인트·프로필·앱 귀속) · pqcota-profile(프로필 upsert) · pqcota-declare(선언 임포트) · pqcota-prune(보존 절단)
  ├─ provisioning/cmd/  # pqcota-provision(확정계획→L2 플레이북+before/롤백 레코드) · pqcota-records(롤백 레코드 조회) — 생성까지
  └─ LICENSE (Apache-2.0), CONTRIBUTING.md, README.md

pqcota-collectors-gpl/  # GPL-3.0 · 별도 리포 · 절대 번들·링크 금지 (라이선스 정리 — 배포 분리)
  └─ adapters/cipheriq/  adapters/cbomkit/   # 서브프로세스 어댑터만
```


---

## 6. 무판단 원칙

이 리포는 **관측·정규화·영속·생성**을 한다 — 사실을 수집하고, 그로부터 파생하고, 산출물을 만든다.
**판정은 하지 않는다**(§2.1): 무엇을 이관할지, 언제 실행할지는 사람이 정한다.

그 실무적 귀결이 **diff와 reconcile의 구분**이다. **스냅샷 간 변화(diff)는 관측 사실의 서술**이라
여기서 낸다 — "libssl 3.0.13 → 3.5.0으로 바뀜"은 판정이 아니다. 반면 **선언(CMDB) 대조**(3-상태·
confidence)는 "무엇이 옳은가"를 가리는 판정이라 하지 않는다.

### 6.1 핵심 구성요소

| 컴포넌트 | 규정서 근거 | AUTO/PROPOSE/MANUAL |
|---|---|---|
| Collector intake 계약(protobuf) + SDK | §1.6 | — (계약) |
| `openssl-collector`: `/proc`·`ldd`·`readelf`·ELF 심볼 | §2.3 | AUTO |
| `jvm-collector`: JVM attach → `getProviders()` | §2.2, §2.3 ★ | AUTO(실행 중), 미실행은 갭 |
| 선언 임포트(CMDB/자산 등록부 로드) | §1.4 | AUTO |
| 정규화 파이프라인 6단계 + `evidence_strength` 부착 | §2.4, §2.3 | AUTO |
| **provider 시그니처 레지스트리 강화**(JCA provider_set → pqc_readiness·fips·알고리즘, SLH-DSA 갭 태깅) | §2.3 v3 | AUTO |
| 완전성 맵(계층별) | §2.6 | AUTO |
| 디스커버리 히스토리(append-only) + **이력 열람·스냅샷 간 변화 diff**(관측 사실 서술이지 판정 아님 — §6 기준) | §2.4-6 | AUTO |
| **관측 기록/스냅샷 2층 분리** — 스냅샷은 실질 내용이 바뀔 때만, 관측 기록은 적재마다. 같은 상태 반복 관측이 저장을 늘리지 않되 "언제 봤나"는 보존 | §2.4-6, §1.2 | AUTO |
| **보존 정책 절단**(`pqcota-prune`) — 오래된 변화 지점 절단. 최신 불가침, 절단 사실을 기록으로 남겨 이력의 구멍을 고지 | §2.6 | — (사용자 지시) |
| **자산 스코프 게이트**(`scope.AssetPolicy`) — 노드 게이트(§1.4)를 자산 단위로. 사용자가 선언한 관리 대상만 적재, 제외 건수는 고지 | §1.4, §2.6 | AUTO(집행) |
| 중앙 인벤토리 뷰 (CLI+UI) + 머신 메타데이터(엔드포인트·프로필)·**앱 귀속** | §6 | — |
| 프로비저닝 생성 (확정계획 게이트 → L1/L2 플레이북 + before·**롤백 레코드**) | 프로비저닝 설계 §4.1·§4.2 | — (생성만) |

### 6.2 명시적 제외 / 경계

이 리포는 **생성까지**다. 그 다음은 셋으로 갈린다:

- **사용자가 직접 (이 리포의 산출물로)** — 생성된 L1/L2·롤백 플레이북을 사용자 Ansible로 **실행**(적용·롤백). 도구는 무엇을·어떻게 되돌릴지 생성하고, 실행·판단은 사용자다(§2.1 무판단).
- **이 리포에 없음 (다른 엔진, 공개 계약 `contracts/`로 연동)** — **선언(CMDB) 대조** 3-상태 reconciliation·confidence 스코어링, 리뷰-확정 거버넌스, 단계적 배포 오케스트레이션·안전 레일(L3 drain·rolling·fleet), 동적 프로비저닝, 배포 채널. *단, 스냅샷 간 변화 diff는 대조가 아니라 관측 사실이므로 이 리포다(§6 기준).*
- **PROPOSE 기본 비활성** — dynamic-trace(eBPF 침습, §2.5).

> **이 리포는 core 트랙만 다룬다**: **CORE 트랙**(OSS, 관측·정규화·영속·생성)은 능력 축으로 전개된다 — ① Discovery MVP → ② 중앙 인벤토리(적재·영속·조회 + 머신 메타데이터[엔드포인트·프로필]·**앱 귀속**) → ③ Provisioning 생성(L1/L2 플레이북 + before·**롤백 레코드**). 이음새는 `contracts/` SSOT — core가 관측을 생산하고, 소비자는 이를 계약으로만 소비한다.

### 6.3 "동작"의 정의 (Definition of Done)

실제 노드 한 대(OpenSSL 설치 + 실행 중 JVM)에 대해:
1. 스코프 마스터에 등재된 노드만 수집 대상이 된다(§1.4 게이트 동작).
2. openssl/jvm Collector가 각각 **정규화된 CBOM Envelope**를 반환한다.
3. 정규화 파이프라인이 두 산출물을 **하나의 인벤토리 뷰**로 수렴, 각 finding에 `crypto_runtime`·`detection_method`·`evidence_strength`가 결정론적으로 부착된다.
4. 미실행 프로세스·미수집 계층이 **완전성 맵에 갭으로** 남는다("없음"으로 오판하지 않음, §2.6).
5. 전 과정이 디스커버리 히스토리에 append되고 재계산으로 재현된다(§1.2).

---

## 7. 다음 액션 (제안)

1. **`contracts/` 먼저 확정** — Collector intake protobuf + CBOM Envelope 스키마가 모든 것의 SSOT(라이선스 정리). 이게 굳으면 Collector·코어를 독립 병렬 개발 가능.
2. **레퍼런스 Collector 2종 스켈레톤** — `openssl-collector`(Go), `jvm-collector`(순수 Java). JVM 쪽이 커뮤니티 유인 지점이므로 우선순위 상.
3. **리포 부트스트랩** — `pqcota` monorepo + Apache-2.0 LICENSE + CONTRIBUTING/GOVERNANCE.
4. **정규화 파이프라인 6단계 골격** — 각 단계 인터페이스부터.

