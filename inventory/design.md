한국어 · [English](design.en.md)

# 인벤토리 서브시스템 설계 (Inventory Subsystem Design)

규정서 §3(Inventory)의 기술 설계. Discovery 산출(정규화된 CBOM·완전성 맵)을 **읽기전용 인벤토리 뷰**로
제시하고 머신 메타데이터를 관리한다. 관측을 노드·앱에 이어 붙이고 이력과 변화를 보여주는 부분이다.

**기준**: 규정서 §3·§5 · [디스커버리 설계](../discovery/design.md) · [아키텍처 설계](../docs/architecture.md) · [contracts](../contracts/).


> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

**어느 규정을 구현하나.** 규정이 바뀌면 이 표로 고칠 절을 찾는다.

| 이 문서 | 규정서 |
|---|---|
| 1. 컴포넌트 아키텍처 | §3.1 목적과 경계 |
| 2. 데이터 모델 | §3.2 정규 형식 · §1.5 머신 식별·자산 계층 |
| 3. 상태 어휘 | §3.3 대조 → 리뷰 → 확정 |
| 4. 산출물 & Deploy 핸드오프 | §3.7 산출물(Deploy 게이트) |
| 5. 자동화 규정 → 컴포넌트 매핑 | §3.5 자동화 규정 |
| 7. 이력·보존 | §1.2 원본 불변 + 파생 뷰 |
| 8. 자산 스코프 | §1.4 스코프 마스터 |

6. 토폴로지 그래프는 대응 규정이 없다. Phase 1에서 이 문서가 먼저 설계한 것이다.

---

## 0. 무엇을 하나

인벤토리가 하는 일은 **적재·영속·조회**다. **이력 열람과 스냅샷 간 변화 diff**도 여기 포함된다. 관측 사실을 나열하는 것이지 판정이 아니기 때문이다. **자산 스코프**(무엇을 계속 관리할지 선언·집행)와 **보존 정책**(오래된 변화 지점 절단)도 여기다. 자기 인벤토리를 스스로 건사하는 일이라, 없으면 "혼자 끝까지 된다"가 깨진다.

`Decision`·`FinalizedPlan` 스키마는 공개 계약 [`contracts/`](../contracts)에 있다.

---

## 1. 컴포넌트 아키텍처

```
  Discovery 산출 ─────────────────────────────────────────┐
   ├ 관측 레인(collector·CBOM import) : 정규화된 CBOM + evidence │
   └ 선언 레인(declaration-importer)  : 사용자 선언            │
                        │                                    │
        ┌───────────────▼─────────────────────────────────┐ │
        │ 적재 (pqcota-ingest)                             │ │  §3.1
        │   스코프 게이트 → 정규화 → 동일성 해소            │ │
        └───────────────┬─────────────────────────────────┘ │
                        │                                    │
        ┌───────────────▼─────────────────────────────────┐ │
        │ append-only 히스토리 + 머신 메타데이터            │ │  §6
        └───────────────┬─────────────────────────────────┘ │
                        │                                    │
        읽기전용 뷰 · 이력 · 스냅샷 diff · 보존 절단          │
                        │ (선언 갱신·재수집 요청)            │
                        └──────────────────▶ Discovery ◀────┘
```

**두 레인이 여기서 만난다**: Discovery에서 확립한 **선언 vs 관측**(전달 방식이 아니라 Envelope `detection_method`로 구분)이 같은 히스토리에 라벨을 단 채로 쌓인다.

---

## 2. 데이터 모델

Discovery의 `Finding`(노드 내부 크립토 자산)에 더해, Inventory는 **통신 엣지**와 **판정·계획**을 1급으로 다룬다.

### 2.0 머신 메타데이터: 식별과 분리 (계약: `inventory/v1 machine.proto`)

인벤토리가 관리하는 머신 정보는 **기술 식별(node_id·지문)과 분리**돼 둘로 나뉜다. 식별은 기계가 관측하고(디스커버리 §1.5) 이 둘은 **선언(CMDB)/리뷰어가 채우고 사용자가 수정**한다:

- **`MachineEndpoint`**(node_id·name·ip·port)는 discovery 재접속용 **재사용 연결 메타데이터**. 사용자 hosts 파일에서 안전 부분집합만 적재(§1.5). **접근 비밀은 미적재**(타입에 비밀 필드 없음).
- **`MachineProfile`**(display_name·environment·role·owner·location·`labels` map·source)는 **사람이 보고 구분**하는 시각 메타데이터다. UI 그룹핑·필터·색상. `labels`로 임의 축(팀·규제·티어) 확장. 출처(cmdb/reviewer/observed) provenance.

→ node_id 같은 기술 ID로는 사람이 못 알아보므로, 표시명·환경 등을 별개 레인으로 둔다. 재사용·수정할 수 있고, 비밀은 절대 저장하지 않는다.

**구현: 런타임 서비스**
- **`inventory.MetaStore`**(Mem/Pg)는 엔드포인트·프로필을 node_id PK로 **upsert**한다(히스토리의 append-only와 달리 이 둘은 사용자가 재사용·수정하는 가변 메타데이터). Pg 스키마에 접근 비밀 컬럼이 없다.
- **적재 경로.** `pqcota-hosts`(discovery/cmd)가 사용자 hosts 파일에서 (a) 런타임 전용 Ansible 인벤토리(접속 비밀 포함·미영속) (b) 안전 엔드포인트 upsert(`--dsn`)를 만든다. 비밀은 (a)에만.
- **뷰 결합.** `RenderStore(store, meta)`가 노드별로 `▸ 이름 (ip:port) │ display_name · env · role · owner` 헤더를 파생 Finding 위에 얹는다(`meta` nil이면 헤더 생략, 무해). `pqcota-inventory`가 같은 DSN에서 조회.

**자산이 어느 앱 것인가(§1.5 자산 모델).** 각 파생 `Finding`은 `app_keys`(복수)로 애플리케이션에 붙고 뷰에 `@app`으로 표시된다. host-wide 스캔에서 하나의 공유 라이브러리(예 `libcrypto.so.3`)를 여러 앱이 로드하면 **여러 앱에 걸린다.** 그 .so 교체가 미치는 영향 범위를 온전히 보이기 위해서다. 판단은 하지 않는다(§2.1).

```go
// 통신 엣지 — reconciliation 단위 (§3.3). 노드 간 TLS/SSH 연결.
type CommunicationEdge struct {
    ID        string          // 정규화 해시 (src·dst·port·proto)
    Src, Dst  string          // 스코프 마스터 노드 ID (앵커)
    Declared  *DeclaredEdge   // 선언 레인 (nil 가능)
    Observed  *ObservedEdge   // 관측 레인 (network collector·CBOM, nil 가능)
    State     ReconState      // CONFIRMED | UNDECLARED | UNOBSERVED
    Confidence float64        // §3.5 스코어
}
type ReconState string // "confirmed" | "undeclared" | "unobserved"

// 판정 — "인간의 결론"(§3.6). 엣지 상태가 아니라 결론이라 재수집에도 부착 유지.
type Decision struct {
    Subject     string     // 엣지 ID 또는 정책 템플릿 ID (정책단위 §3.4)
    Conclusion  string     // 실존/stale/제외/승인 등
    Reviewer    string
    Signature   string     // 승인 서명 (finalize 필수 §3.3③)
    BasisHash   string     // 판정 근거 증거의 해시 → 무효화 트리거(§3.6)
    Status      string     // draft | in-review | finalized
    DecidedAt   string
}

// 확정 계획 — 프로비저닝의 유일 실행 근거(§3.7·§5). 스키마는 contracts SSOT(공개).
type FinalizedPlan struct {
    Scope           string                 // 링/도메인 (부분 확정 허용 §3.3③)
    Items           []PlanItem
    ApprovalSigs    []string               // 전 필수항목 판정 + 서명
    DerivedFrom     string                 // 어떤 리컨실리에이션 스냅샷에서 (§1.2)
}
type PlanItem struct {
    NodeID          string
    RemediationClass string                // taxonomy 분기 (프로비저닝 설계 §4.1·§4.2)
    DeployAutomationLevel DeployAutomationLevel // L1/L2/L3 — 리뷰어 자산별 판정(§4.3)
    ProviderChoice  string                 // FIPS 라우팅 결과: BC-FJA/내부/…
}
```
> `DeployAutomationLevel`·`RemediationClass`는 이미 contracts가 정한 통제 어휘다. **여기서 채워진다**(Discovery 아님, MANUAL 리뷰어). `Decision`·`FinalizedPlan` 스키마는 contracts에 있다(공개 스키마).

---

## 3. 상태 어휘 (뷰 렌더링에 쓰는 것)

- **상태 어휘**(뷰 렌더링에 필요한 스키마 의미): `CONFIRMED` = 선언 ∩ 관측, `UNDECLARED` = 관측 only(**shadow 통신**), `UNOBSERVED` = 선언 only. `UNOBSERVED`가 "실제 없음"인지 "완전성 맵상 갭(원리상 관측하지 못함)"인지는 Discovery **완전성 맵**(§2.6, 갭은 부재가 아니다)으로 갈린다. 갭이면 재수집하고, 아니면 사람이 판정한다.

---

## 4. 산출물 & Deploy 핸드오프 (§3.7·§5)

> 핸드오프 대상(Deploy 서브시스템)의 계획 스키마·게이트·아티팩트 생성기 설계는 [프로비저닝 설계](../provisioning/design.md) 참조. 계획·판정 스키마의 정식 정의는 `contracts/plan.proto`·`decision.proto`([데이터 모델 스키마](../contracts/data-model.md)).

| 산출물 | 성격 | 핸드오프 |
|---|---|---|
| 리컨실리에이션 뷰 | 파생(엣지별 state+confidence+provenance), 읽기전용 뷰 | — |
| **finalized 확정 계획** | 프로비저닝 **유일 실행 근거** | **Inventory→Deploy(§5): finalized 아니면 실행 거부** |
| 결정·계획 히스토리 | provenance 판단·의도 계열 | §1.3 |

## 5. 자동화 규정 (§3.5) → 컴포넌트 매핑

| 동작 | 등급 | 컴포넌트 |
|---|---|---|
| 버전정규화·Envelope·evidence 부착 | AUTO | (Discovery 파이프라인 재사용) |

---

## 6. 크립토 통신 토폴로지 그래프 (Phase 1): 자동 완성 지도

사용자가 준 레거시 IP 목록을 스코프 마스터로 구축한 뒤, 각 머신의 [network-collector](../discovery/design.md#23-network-collector-go-af_packet--네트워크-계층-phase-1) 관측을 집계해 **크립토 통신 지도를 자동 생성**한다. reconciliation 뷰(§3.7)를 그래프로 렌더링한 것이다.

### 6.1 구성

- **노드** = 스코프 마스터 등재 머신(사용자 IP). 미등재 관측 상대는 "등재 판정 요청"(§1.4/§5)으로 별도 표시한다.
- **엣지** = 관측된 통신(§2 CommunicationEdge). src→dst, role(client/server), 프로토콜(TLS/SSH), **협상된 KEX 그룹·cipher**.
- **엣지 색 = 양자내성 등급** (마이그레이션 대시보드의 핵심):

| 색 | 의미 | 예 |
|---|---|---|
| 🟢 | 하이브리드/PQC 협상 | `X25519MLKEM768`, `sntrup761x25519` |
| 🔴 | 고전 = **양자 취약** | `X25519`, `ECDHE`, `RSA` |
| ⚪ | **불명·미관측** | 암호화된 핸드셰이크·QUIC / 수집 구간 밖 |

### 6.2 정직성 규정 (이 그래프의 신뢰성)

**"관측된 것"과 "원리상 관측하지 못한 것"을 구분한다**(§2.6, §3.3):
- **미관측 엣지는 "연결 없음"으로 그리지 않는다.** 점선이나 ⚪로 "unobserved"라고 표기한다. 완전성 맵과 연동.
- **coverage 표시**: collector 미설치 노드는 회색 처리(그 노드의 엣지는 반쪽만 보임).
- **capability vs actual 오버레이**: 노드에 "PQC 가능 lib 로드"(디스커버리 Finding)와 엣지 "실제 고전 협상"(관측)을 겹쳐 표시 → *"PQC 되는데 고전으로 폴백 중"* 같은 **정확한 다음 조치**가 드러난다.

### 6.3 산출물

- **엣지 그래프**(파생 뷰): 리컨실리에이션 뷰의 그래프 표현. **DOT/Graphviz** 자동 생성 → SVG, 또는 웹 뷰(D3).
- CONFIRMED/UNDECLARED/UNOBSERVED 3-상태(§3.3)가 그래프에 색·선형으로 매핑: UNDECLARED = **shadow 연결**(굵은 경고선), UNOBSERVED = 점선.

### 6.4 한계 (디스커버리 설계 §2.3 network-collector에서 상속)

수동 관측 구간·coverage 의존·암호화 한계 → **지도는 "본 것"이지 "존재하는 것 전체"가 아니다**. 이 부분성을 갭으로 정직히 표기하는 게 §1.2 감사 무결성과 일관된다. "가능한 한도 안에서"가 정확한 표현이다.

---

## 7. 이력·보존 (2026-07-21)

### 7.1 이력을 두는 목적은 셋이고, 요구가 서로 다르다

| 목적 | 필요한 것 | 반복 측정의 가치 |
|---|---|---|
| **변화 추적** | 바뀐 지점의 전/후 | 없다. 같은 값 반복은 잡음이다 |
| **관측 증명** (감사에 "매일 스캔했다"를 보이기) | 언제·몇 번 봤는지 | **크다.** 이것이 증거다 |
| **재계산 재현**(§1.2) | 원본 + 그때의 ruleset | ruleset 경계에서만 |

이 셋을 한 테이블이 떠안으면 무한 증식한다. 하루 1회 × 1000노드 × 3년 = 스냅샷 100만 건인데, 그중 의미 있는 변화는 노드당 연 몇 건뿐이다. **그래서 해법은 "지우기"가 아니라 "분리"다.**

### 7.2 2층 분리: 무거운 것과 가벼운 것

- **스냅샷**(무거움, `pqcota_snapshots`)은 실질 내용이 **바뀔 때만** 쌓인다. 변화 추적·재계산 재현의 근거.
- **관측 기록**(가벼움, `pqcota_observations`)은 적재할 때마다 1행(`node_id, snapshot_id, ruleset, observed_at`). 관측 증명의 근거.

무거운 저장은 **변화 횟수만큼만** 자라면서 "매번 봤다"는 사실은 온전히 남는다. §1.2도 깨지 않는다. 원본을 고치는 것이 아니라 **중복 저장을 안 하는** 것뿐이다.

이력 뷰(`-history`)는 이제 "변화 지점 + 그 상태를 몇 번·언제까지 재확인했는지(obs·observed)"를 보인다.

### 7.3 동등성 정의: 이 설계의 급소 (`pkg/discovery/history/fingerprint.go`)

"같은 상태인가"를 무엇으로 판정하느냐가 전부를 좌우한다. **휘발 필드를 포함하면 항상 "변화"가 되어 분리가 무력해진다.**

| 제외 (휘발) | 왜 |
|---|---|
| `ObservedEdge.observed_count`, `first_seen`, `last_seen` | 관측할 때마다 달라진다. 빈도가 늘었다고 암호 구성이 바뀐 건 아니다 |
| `Finding.derived_from_snapshot_id`, `ruleset_version` | 스냅샷마다 다르다(파생 추적용 필드) |

| 포함 (실질) | 왜 |
|---|---|
| finding id, runtime, usage, detection_method, evidence_strength, algorithm, pqc_readiness, fips, remediation_class, **app_keys** | 관리 판단이 달라지는 값들 |
| OpenSSL 축: lib, fork, version, binding_mode | 버전 교체가 곧 변화 |
| JCA 축: jdk, **provider_set(순서 유지)** | 순서가 우선순위 협상을 결정한다(수용 원칙 §2.2 (d)). **정렬하면 안 된다** |
| Completeness: layers_missing, note | 갭이 달라지면 해석이 달라진다 |

> ⚠️ 여기서 빠뜨린 필드가 바뀌면 "변화 없음"으로 접혀 **이력에서 사라진다**. 계약에 필드를 더할 때 이 함수를 함께 갱신해야 한다.

`ExcludedByScope`(§8)는 지문에서 **뺀다.** 관리 대상 밖 자산의 증감은 관리 인벤토리의 변화가 아니다.

### 7.4 절단 정책 (`pqcota-prune`)

§7.2로 같은 상태의 반복은 이미 접히므로, 절단이 다루는 것은 **오래된 변화 지점**뿐이다. 저장된 스냅샷은 전부 변화 지점이라 "변화점 보존" 같은 축은 필요 없다.

**불변식 셋:**

1. **수정 금지.** 남은 스냅샷은 바이트 그대로다. 절단은 과거를 *바꾸는* 게 아니라 보관을 *끝내는* 것이라 append-only(§1.2)와 양립한다.
2. **최신 불가침.** 노드별 최신은 어떤 정책으로도 지우지 않는다. 인벤토리 뷰와 프로비저닝 before 캡처(§8)의 근거이기 때문이다.
3. **지운 사실을 남긴다.** `pqcota_retention_events`에 기록하고 `-history`가 `⌫` 줄로 고지한다. 없으면 이력의 구멍이 **"관측을 안 함"과 구분되지 않는다.** §2.6의 "갭은 부재가 아니다"를 시간축으로 옮긴 것이다.

**정책 판정**: `Policy{OlderThan, KeepLast}`. 두 축을 다 주면 **보수적으로** 판정한다. 둘 다 "버려도 된다"고 할 때만 지운다(최근 N개 안이거나 아직 안 오래됐으면 보존). 파괴적 동작이라 의심스러우면 남긴다. 축을 하나도 안 주면 "최신만 남기고 전부 삭제"가 되므로 `ErrNoPolicy`로 **거부**한다.

**왜 별도 커맨드인가**: `Pruner`를 조회용 `Store` 인터페이스에서 분리하고 `pqcota-prune`을 따로 뒀다. 읽기 도구가 파괴적 동작을 겸하면 실수 한 번이 이력을 지운다. 기본은 dry-run이고 실제 삭제는 `-apply`로만 한다.

---

## 8. 자산 스코프 (2026-07-21)

### 8.1 노드 게이트(§1.4)를 자산 단위로

노드를 등재해도 그 안에서 관측되는 것 **전부가 관리 대상은 아니다**. 시스템 기본 라이브러리, 패키지 매니저가 딸려 넣은 런타임, 일회성 프로세스가 섞이면 인벤토리가 잡음에 묻혀 못 쓰게 된다. 무엇을 계속 볼지는 **사용자가 선언**하고 도구는 그 선언을 집행한다(§1.1: 판단은 마에스트로가, 집행은 단원이 한다).

구현: `pkg/kernel/scope/asset.go`의 `AssetPolicy`. 적용 지점은 **정규화 직후·적재 직전**이다(`normalize.Normalize`). 관측은 하되 관리 대상만 영속한다.

### 8.2 판정 순서

기본 **포함** → 규칙을 **순서대로** 적용하고 **뒤 규칙이 이긴다**(매치되는 마지막 규칙이 결정).

- **include는 exclude "뒤에" 두어 예외로 쓴다.** "이 계열은 전부 빼되 이것만 예외"가 그렇게 표현된다. **무조건 우선이 아니라 순서 기반**이다(include를 앞에 두면 뒤의 exclude가 이긴다).
- 규칙이 없으면 전부 관리 대상이다. 스코프를 안 쓰는 사용자를 막지 않는다.
- 규칙 축: `runtime`, `lib`(openssl은 soname, jca는 provider), `app_key`. 빈 칸·`*`는 모두, 패턴은 glob.
- **공유 `.so`는 쓰는 앱이 여럿**이라(§1.5) 그중 **하나만 맞아도** 규칙이 걸린다.

> **★ 공유 `.so`를 제외할 때의 영향 범위는 빠지기 쉬운 함정이다.** 공유 라이브러리는 쓰는 앱이 여럿이라, 그중 **한 앱(예: 테스트 앱 `internal-test-*`)만 겨냥해 exclude**해도 그 .so를 함께 쓰는 **운영 앱(`payment-gw`)의 자산까지 함께 빠진다.** 하나만 맞아도 finding 전체가 걸리기 때문이다. 운영 앱을 지키려면 그 앱을 되살리는 **include를 exclude 뒤에** 둔다(뒤가 이긴다). 제외분은 §8.3대로 세어 고지되므로 "0건"으로 숨지는 않지만, **어떤 공유 자산이 어느 앱 때문에** 빠졌는지가 의도한 결과인지는 규칙 작성자가 확인해야 한다(테스트 TV-SCOPE-7).

### 8.3 ★ 제외 ≠ 부재

정책으로 뺀 자산을 조용히 사라지게 하면 인벤토리는 **"그런 게 없다"고 거짓말한다.** §2.6이 금지하는 바로 그것이다. 그래서 제외분을 세어 `Snapshot.ExcludedByScope`에 남기고, **적재 요약과 인벤토리 뷰 양쪽이 건수를 고지**한다.

같은 이유로 이력 diff에서 스코프 적용은 `removed`로 잡히는데, 이는 **자산이 없어진 게 아니라 관리 대상에서 뺐다는 뜻**이다(데모가 이 구분을 캡션으로 명시한다).

### 8.4 범위 경계

규칙의 **정의·집행은 이 리포**다. 혼자서 자기 자산을 고르는 일이고, 이게 없으면 잡음 때문에 인벤토리 자체를 못 쓴다(아키텍처 §6 기준).

---

## 선언 임포터 (SV-1)

**책임**: 사용자 **선언**(CMDB·기존 인벤토리) 임포트 → 정규화 → **선언 레인** 라벨. 관측이 아니다(§3.3 선언≠관측).

> **코드 위치**: `pkg/inventory/declaration`. 산출물이 인벤토리에 쌓이는 데이터라 코드도 인벤토리에 둔다.

### 시나리오

- **상황**: 사용자가 기존 크립토 인벤토리·CMDB **선언** 보유. (※ CBOM 파일은 아니다. 그건 [위임 수신](cbom-intake.md)이다)
- **[사용자]** 기존 선언 데이터 제공.
- **[pqcota]** 선언 임포트 → 정규 형식으로 정규화. **선언 레인**으로 라벨(관측과의 **대조는 하지 않는다**). ※ 구현은 인벤토리로 옮겼다(`pkg/inventory/declaration`). 선언은 대조 기준선이라 인벤토리 단계 소관이다.
- **결과**: 선언 계열 확보(추후 reconciliation의 한 축).
