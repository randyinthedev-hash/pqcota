English · [한국어](architecture.md)

# PQC migration platform — architecture design

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](architecture.md) is authoritative.

This settles **which modules, interfaces, and schemas** implement the rules the [process regulation](regulation.en.md) set.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](regulation.en.md).

**Scope**: the three stages end to end — Discovery (collectors, normalization, history) → the central inventory (views, endpoints, profiles, app attribution) → provisioning generation (L1/L2/L3 apply and rollback playbooks, activation hooks, rollback records). Declaration reconciliation, governance, and fleet orchestration are not done (see [§6 the no-judgment principle](#6-the-no-judgment-principle) below).

> **How the regulation's core shapes the architecture**:
> - **§4.3 the staged deployment model** = the delegation levels L1/L2/L3. Implemented with **Ansible `copy` + a sha256 gate**
>   (the module file is prepared and brought in by the user).
> - **Acceptance principles §2.3, provider signatures** = the input to the discovery enrichment step. The living source is `pkg/kernel/registry`.
> - **FIPS routing** = a `fips_validation` requirement surfaces as a **recommendation** to use a FIPS-validated provider
>   (the tool does not block the plan's provider choice — a validation certificate is per build and cannot be told from the file alone).

> Design principle: regulation §1 (the cross-cutting principles) is an invariant contract in code too. In particular
> **immutable originals + derived views** (§1.2), **the four lanes of the provenance chain** (§1.3), **the AUTO/PROPOSE/MANUAL split** (§1.1),
> and **hiding collectors behind the intake contract** (see the license notes) are first-class constraints that decide module boundaries.

---

## 1. Technology stack (the recommended combination)

### 1.1 The constraints that force the decision

The stack is forced by **the nature of the targets (the runtimes)**, not by taste. List the collection capabilities the regulation demands and the language is very nearly decided for you.

| Capability required (basis in the regulation) | Technical constraint |
|---|---|
| Reading `/proc/*/maps` and `/proc/*/exe` (§2.3 OpenSSL) | syscalls and native tooling. The Go/Rust/C family |
| Determining fork and version from static ELF symbols and string signatures (§2.3) | an ELF parser. Both Go (`debug/elf`) and Rust (`goblin`) are strong |
| **JVM attach → querying the reality of `Security.getProviders()` (§2.2, §2.3)** | **possible only from inside the JVM — the JVM is forced (its platform language is Java). No way around it** |
| CycloneDX CBOM (ECMA-424) input/output (§2.4, §3.2) | needs mature libraries. Maturity runs JVM > JS > Go |
| Ansible/Salt substrate orchestration (§4.4) | subprocesses and SSH. Language-agnostic; Go is comfortable |
| The review queue and inventory dashboard UI (§3.7) | TypeScript/React — **not in this repository** (§6.2) |

> **One requirement was dropped** — dynamic tracing (eBPF, ltrace) is invasive, so it is not done
> ([RELEASE_NOTES](../RELEASE_NOTES.en.md#not-on-the-roadmap--deliberately)). Observing what the wire
> actually negotiated was chosen instead, so it comes off this table. The detection-method taxonomy in
> regulation §2.5 still lists `dynamic-trace` — the vocabulary is a contract, separate from what is built.

**The key observation**: JVM introspection (§2.2's "the gap where we implement ourselves") **cannot be worked around in any language — it must run inside the JVM.** That is what makes polyglot unavoidable. Everything else in system collection is covered by Go alone.

### 1.2 The recommended combination

| Layer | Language/technology | Rationale |
|---|---|---|
| **Core services** (normalization, inventory, API) | **Go** | single static binary distribution, gRPC, concurrency, systems tooling, a permissive license (no contagion) |
| **The OpenSSL/system collector** | **Go** | the same language as the core. `/proc` and ELF (`debug/elf`) are parsed directly — no dependency on external tools such as `ldd` or `readelf` |
| **The JVM collector** (a separate sidecar) | **Java** (pure) | attaches to a live JVM through the JVM Attach API (JVMTI/Attach) and queries `getProviders()`. **The unavoidable polyglot point is the JVM**, not a language — written in the platform language Java, built with `javac`, no Kotlin or Gradle |
| ~~**UI**~~ | ~~TypeScript + React~~ | **Not in this repository** (§6.2). The review queue and sign-off governance are out of scope, so their UI is too |
| **Storage** | **PostgreSQL** (JSONB) | the four append-only history lanes + CBOM JSONB. Friendly to event sourcing |
| **The contract between runtimes** | **gRPC + Protobuf** (+ a CLI/stdout fallback) | the intake contract and subprocess isolation through one mechanism |

**In one line**: **a Go core + Go system collectors + a JVM collector sidecar (pure Java) + Postgres.** — what is forced is *the JVM*, not a particular language (the polyglot point is the JVM). The sidecar is written in the platform language Java with no Kotlin or Gradle dependency.

### 1.3 Why a Go core (versus Rust)

- **It leaves no footprint on legacy hosts**: `CGO_ENABLED=0` yields a single static binary with no
  runtime dependency. The targets are old servers where neither sources nor package management can be
  assumed — copy it, run it, delete it. This is also where the kernel floor comes from: whatever the Go
  toolchain requires (3.2) becomes this repository's floor (§4.4, avoid locking yourself out).
- **Cross-compilation needs no build infrastructure**: `GOOS=linux GOARCH=arm64` is the whole story, and
  CI verifies static linking per architecture.
- **It digs into `/proc` and ELF on its own**: `debug/elf` is in the standard library. Never calling `ldd`
  or `readelf` means observation does not wobble when the target host lacks those tools — or has them with
  a different output format.
- **Alignment with orchestration and cloud-native**: Ansible/Salt subprocesses, mTLS, gRPC momentum.
- **Where does Rust fit?** The ELF symbol analyzer (§2.3) is a pure, isolated module and therefore **a clear candidate for later replacement in Rust**. It only has to honour the intake contract, so it can be swapped without touching the core. Start in Go and promote it if needed.

---

## 2. System architecture

### 2.1 The module map — the regulation's three stages plus cross-cutting principles, projected into code

> **This map is the platform as a whole; the scope of this repository is set by [§6](#6-the-no-judgment-principle).**
> Of the boxes below, the Reconciliation Engine, Confidence Scoring, the Review Queue and the Decision
> Service are **not in this repository** — the contracts (`contracts/`) merely hold their place. §6.2 is
> decisive on what is built and what is not.

```
                         ┌─────────────────────────────────────────────┐
                         │  the scope master (CMDB/asset registry) — §1.4 │
                         │  the sole authority on the managed boundary.  │
                         └───────────────────┬─────────────────────────┘
                                             │ (the target node list)
   ┌── DISCOVERY (§2) ───────────────────────▼─────────────────────────┐
   │                                                                    │
   │  [collector framework] ── the intake contract ─┐                    │
   │    ├ openssl-collector (Go)                 │  "node → canonical CBOM"│
   │    ├ jvm-collector (a pure Java sidecar)     │  the core does not know the backend │
   │    ├ network-collector (Go)                 │                      │
   │    └ [gpl-adapter] a CipherIQ/CBOMkit subprocess ─┘ ← the GPL isolation boundary │
   │                        │                                           │
   │            [the six-step normalization pipeline — §2.4]             │
   │   raw capture→parse→enrich→validate→resolve identity→persist        │
   │              └ enrichment input: [crypto-registry], the provider signature registry │
   │                (§2.3) provider JAR/module → pqc_readiness, fips, algorithms │
   │                        │                                           │
   └────────────────────────┼───────────────────────────────────────────┘
                            ▼
              ╔═══════════════════════════════════╗
              ║  ① discovery history (the state lane) ║  ─┐
              ╚═══════════════════════════════════╝   │
   ┌── INVENTORY (§3) ──────────────────────────────┐ │  the four lanes
   │  [reconciliation engine] three-state (§3.3)     │ │  of the provenance
   │    CONFIRMED / UNDECLARED / UNOBSERVED          │ │  chain (§1.3)
   │  [confidence scoring] (AUTO)                    │ │  state→judgment
   │  [review queue] prioritization (PROPOSE)        │ │   →intent→act
   │  [decision service] draft→in-review→finalized   │ │
   │    persisting verdicts, delta review (§3.6)     │ │
   └────────────────────────┬───────────────────────┘ │
              ╔═══════════════════════════════════╗   │
              ║ ② decision and plan history (judgment, intent) ║ ─┤
              ╚═══════════════════════════════════╝   │
   ┌── PROVISIONING generation (§4) ──[finalized plans only]──┐ │
   │  [plan→config generator] taxonomy→L1/L2/L3 playbooks │ │
   │    (provisioning design §4.1·§4.2) · before capture · persisted rollback records │ │
   │  [substrate adapter] Ansible/Salt (L1·L2, §4.4)      │ │
   └────────────────────────┬───────────────────────┘ │
              ╔═══════════════════════════════════╗   │
              ║ ③ provisioning history (the action lane) ║ ─┘
              ╚═══════════════════════════════════╝
                            │ the rescan closed loop → Discovery (§5)
                            ▼
   [provenance/audit service] — stitches the four lanes into a causal chain, an immutable audit log (§1.3, §4.7)
   [API gateway] + [web UI] — the review queue, inventory, and audit views
```

### 2.2 The invariants of the data flow (enforcing §1.2)

- **Originals are append-only.** `raw_capture` (the collector's native output) and the three histories are never modified in place.
- **Normalization results, reconciliation, and finalized plans are all derived views** — they must be recomputable from the originals. When an enrichment rule changes, re-run from `raw_capture`.
- In code: derived tables always carry `derived_from_snapshot_id` + `ruleset_version` → reproducibility guaranteed.

### 2.3 Collectors reaching the host — principles and boundaries

**A collector is a CLI that emits a `CollectionResult`** (`pqcota-nodescan`, `pqcota-jvmscan`, `pqcota-netcap`, `pqcota-cngscan`). Deployment goes through a standard substrate (Ansible) — during discovery it is shipped to the observed node, run, retrieved, and leaves no residue ([collector deployment design](../discovery/collector-deployment.md), Korean). **No remote execution engine of our own is built.**

- **This repo**: the collector CLI + **T1 self-service** (the user runs the collector bundle themselves — air-gapped included; integrity is checked today with `SHA256SUMS`, and bundle signing is on the [roadmap](../RELEASE_NOTES.en.md#roadmap--upcoming-releases-planned)) + **result signing and verification** (ed25519, `pqcota-keygen`, `PQCOTA_VERIFY_KEY`) + **the scope master gate** (§1.4; `pqcota-ingest` accepts only registered nodes). The collector can also be wrapped and run by the user's own substrate. Release and bundle signing (supply-chain hygiene) belongs here.
- **The principle (invariant)**: whatever the path, **the scope gate is mandatory** plus **RCE symmetry** (putting an executable on a legacy host is risky, so: signature verification, least privilege, idempotence). The value added is not owning a push channel but the gate, signing, and completeness map above it.

**The host footprint (the Phase 0 minimum)** — directly tied to the rationale in acceptance principles §2.2:
- **An OpenSSL node**: one static Go binary + root/`CAP_SYS_PTRACE` + mTLS credentials. Zero other dependencies (ELF and /proc are parsed self-sufficiently; by design it does not depend on `ldd`/`lsof`/`ss`/`readelf`).
- **A Java node**: only the **Go binary plus the introspection agent JAR** need to go up — attach uses OS IPC (a trigger file + SIGQUIT + a unix socket), so it **attaches directly without a JDK** (even if the target is a pure JRE or a jlink runtime). It needs **the same UID or root**. On a non-HotSpot VM (OpenJ9) it goes through the machine's JDK as the client, and if even that is blocked (`DisableAttachMechanism`, JEP 451) it degrades to the static path → `evidence_strength` is lowered (§2.3). The three-layer detail: [the jvm-collector README](../discovery/collectors/jvm/README.md) (Korean).
- **A container caveat**: `/proc` and JVM attach work only **within the same PID/mount namespace** → a host PID namespace or sidecar injection is needed (the biggest trap in real deployments).
- **What to defer off-host**: network scanning (remote, central), artifact/source scanning (CI and repos), eBPF dynamic-trace (PROPOSE, excluded from Phase 0).

---

## 3. The core data model — the canonical CBOM Envelope

This fixes regulation §3.2's **"CycloneDX CBOM (the standard body) + Envelope (provenance) + evidence metadata (the extension)"** as a code schema.

### 3.1 The Envelope schema

```jsonc
// the canonical output one collector run returns
{
  "envelope": {                          // §2.6 provenance — the Envelope
    "collector_id": "openssl-collector",
    "collector_version": "0.1.0",
    "detection_method": "runtime-introspection", // the §2.3 enumeration (below)
    "collected_at": "2026-07-07T05:00:00Z",
    "target_node_id": "cmdb://node/8f3a...", // the scope master anchor (§1.4)
    "signature": "ed25519:...",          // the collector's signature (§2.6 integrity)
    "scope_master_ref": "cmdb-snapshot://2026-07-07"
  },
  "cbom": { /* the CycloneDX CBOM ECMA-424 standard body — converged on the internal canonical version */ },
  "findings": [ /* canonical finding[] with §3.2 evidence metadata attached */ ],
  "completeness": {                       // the §2.6 completeness map — separated per collector and per layer
    "layers_covered": ["artifact", "runtime-introspection"],
    "layers_missing": ["process"],        // separating "genuinely not there" from "impossible to observe" (§2.6)
    "note": "process layer not collected — the target was not running"
  }
}
```

### 3.2 The Finding schema (the runtime abstraction — acceptance principles §2.4)

```go
// runtime-agnostic first-class fields + per-runtime branch fields
type Finding struct {
    ID             string        `json:"id"`              // canonical hash (the dedup anchor, §2.4)
    CryptoRuntime  string        `json:"crypto_runtime"`  // "openssl" | "jca"  ← decides the branch (acceptance principles §1)
    UsageContext   string        `json:"usage_context"`   // server|client|at-rest|signing
    Algorithm      *string       `json:"algorithm"`       // may be nil (degraded when source is absent, §2.3)

    // ─ evidence metadata (the heart of §2.3) ─
    DetectionMethod  string      `json:"detection_method"` // the enumeration ↓
    EvidenceStrength string      `json:"evidence_strength"`// confirmed|inferred-high|inferred-low
                                                           // derived deterministically from detection_method (§2.5 AUTO)
    // ─ the per-runtime branch (acceptance principles §2.4) — oneof ─
    OpenSSL *OpenSSLAxes `json:"openssl,omitempty"`
    JCA     *JCAAxes     `json:"jca,omitempty"`

    // ─ shared decision axes ─
    PQCReadiness    string  `json:"pqc_readiness"`
    FipsValidation  string  `json:"fips_validation"`
    RemediationClass string `json:"remediation_class"`
}

type OpenSSLAxes struct {
    Lib         string `json:"lib"`          // libssl/libcrypto
    Version     string `json:"version"`
    Fork        string `json:"fork"`         // OpenSSL|BoringSSL|LibreSSL|AWS-LC ("unknown" allowed)
    BindingMode string `json:"binding_mode"` // dynamic|static|dlopen|vendored
}

type JCAAxes struct {
    JDKVendor        string   `json:"jdk_vendor"`
    JDKVersion       string   `json:"jdk_version"`
    ProviderSet      []string `json:"provider_set"`      // registration order included (the basis for priority negotiation, acceptance principles §2.2)
    RegistrationMode string   `json:"registration_mode"` // static|dynamic|explicit
}
```

**The `detection_method` → `evidence_strength` mapping (regulation §2.3's table as a deterministic function)**:

```go
// §2.5: attaching evidence_strength is AUTO (deterministic). This function is the single source.
func EvidenceStrength(method string) string {
    switch method {
    case "source":                 return "confirmed"      // Hyperion and the like; algorithm+usage complete
    case "runtime-introspection":  return "confirmed"      // /proc, getProviders()
    case "dynamic-trace":          return "confirmed"      // eBPF/ltrace, the actual calls
    case "artifact":               return "inferred-high"  // Theia, JAR scanning
    case "symbol-analysis":        return "inferred-low"   // a static binary, no usage
    default:                       return "unknown"        // §2.5 "unknown is first-class evidence too"
    }
}
```

**An invariant rule (§2.5)**: a field that could not be filled is an explicit `"unknown"`, not `nil` or blank. Never treat it automatically as "absent".

**How this lands in the schema (which stage a field belongs to)**

- **`FipsValidation` already exists** — in FIPS routing for regulated assets this field **drives the provider choice**. Discovery only fills the value (at the enrichment step); it does not make the routing decision. `JCAAxes.ProviderSet` is compared against the crypto registry (§2.3) to derive `pqc_readiness` and `fips_validation`.
- **`ProviderSet` → provider signature registry mapping**: at the enrichment step, identify `bcprov-jdk18on` / `BC-FJA` / `JDK-native` / `openssl-jostle` / in-house and tag the algorithm coverage (notably that **SLH-DSA is not in the JDK natively**). → new in §3.3.
- **`deploy_automation_level` (L1/L2/L3) is not a Finding field** — it is not a Discovery output but a **plan/asset attribute a reviewer decides per asset** (§4.3, MANUAL). It belongs to the plan entity. It is still registered as controlled vocabulary in the SSOT (contracts) (§3.3 and the contracts).

### 3.3 The provider signature registry (§2.3) — reference data for the enrichment step

The deterministic mapping table discovery enrichment (§2.4 step 3) consults. **Being a derivation rule, when it improves you recompute from the original** (§1.2), and it is pinned by `ruleset_version`.

```go
// provider signature → capability. Discovery enrichment compares it against JCAAxes.ProviderSet.
type ProviderSignature struct {
    Match          string   `json:"match"`           // e.g. "bcprov-jdk18on-*", "BC-FJA", "JDK-native>=24"
    Nature         string   `json:"nature"`          // pure-java | fips-native | jdk-builtin | jni-bridge | internal
    PQCAlgorithms  []string `json:"pqc_algorithms"`  // ["ML-KEM","ML-DSA","SLH-DSA"] — the JDK natively lacks SLH-DSA
    FipsValidation string   `json:"fips_validation"` // "140-3" | "none" | "jdk-dependent" | "module-dependent"
    LicenseClass   string   `json:"license_class"`   // "permissive" (standard BC) | "fips-contract" (BC-FJA) | "gpl" | "internal"
}
// Rule: assets needing SLH-DSA are tagged as depending on BC/jostle regardless of JDK version (§2.3).
// LicenseClass="permissive" (the standard BouncyCastle edition, MIT family) is not among the GPL-isolated items (provisioning design §4.2).
```

### 3.4 The PQC algorithm maturity registry (§2.3 reference data) — input to remediation routing

Where §3.3 deals with **a provider's capability** (which algorithms it implements), this section deals with **the standardization maturity of the algorithm itself** (whether you should be using it). The observed negotiated group (`negotiated_group`, from the network collector) and provider algorithm names are compared against this table to derive maturity, which feeds the remediation branch. It is a **derivation rule** (§1.2) and public reference data in this repo (`pkg/kernel/registry`).

`pkg/kernel/registry/pqc.go` — four maturity levels (based on the state of NIST PQC standardization):

| Maturity | Meaning | Examples | `FIPSValidatable()` |
|---|---|---|---|
| `fips-standard` | the final FIPS 203/204/205 standards (Aug 2024) | ML-KEM · ML-DSA · SLH-DSA | ✅ |
| `draft` | a predecessor or in progress | Kyber · Dilithium · SPHINCS+ · Falcon · HQC | ❌ |
| `experimental` | research or alternatives (non-FIPS) | BIKE · FrodoKEM · McEliece · sntrup761 · MAYO · CROSS | ❌ |
| `broken` | broken | Rainbow · GeMSS · SIKE | ❌ |

`MatchPQC(name)` normalizes a negotiated group or algorithm name (uppercase, separators removed) and matches by substring → `(PQCAlgorithm, ok)`. For example `X25519MLKEM768`→ML-KEM (fips), `sntrup761x25519-sha512@openssh.com`→NTRU-Prime (experimental), `x25519`→(false, classical).

**The maturity axis is orthogonal to the posture axis** — on top of `pkg/kernel/posture`'s "PQC or classical" (🟢/🔴/⚪, §1.6) it adds "standard or experimental". `posture.Grade(group)`→maturity, `posture.GradeLabel`→a standard/draft/experimental/broken label (for display). The dependency is one-way (posture→registry).

**The remediation branch** — `registry.Remediation` + `PQCAlgorithm.Remediate(regulated)` route maturity into a remediation, and `posture.Recommend(group, cipher, regulated)` produces an overall recommendation for a single edge (classical and unobserved included):

| Input grade/maturity | Action | Priority | Rationale |
|---|---|---|---|
| PQC standard | `none` | 0 (regulated = 1) | keep — for regulated assets, confirm a FIPS-validated provider (§3.3) |
| PQC draft | `upgrade` | 2 | move up to the final standard (ML-KEM/ML-DSA) |
| PQC experimental | `replace` | 3 | replace with a standard |
| PQC broken | `replace` | 4 | replace immediately |
| Classical (🔴) | `migrate` | 3 (regulated = 4) | quantum-vulnerable (HNDL) — introduce a PQC hybrid |
| Unobserved (⚪) | `none` | 0 | judgment withheld (inventory design §6.2 honesty — do not assert about what was not observed) |

The target standard differs by kind — KEM→ML-KEM (FIPS 203), signature→ML-DSA (FIPS 204). This recommendation is a derivation, not an execution.

---

## 4. The collector intake contract (§1.6 — the pluggable interface)

The core's only collector dependency. It knows only **"give it a node, get back a canonical CBOM Envelope"**. Whether the backend is our own collector, CipherIQ, or CBOMkit, it **must not know.**

### 4.1 The interface (gRPC)

```protobuf
service Collector {
  // capability declaration — used by the core to judge the completeness map and layer coverage
  rpc Describe(DescribeRequest) returns (CollectorCapabilities);
  // run a collection — returns a stream of canonical CBOM Envelopes
  rpc Collect(CollectRequest) returns (stream CollectionResult);
}

message CollectorCapabilities {
  string collector_id = 1;
  string version = 2;
  repeated string crypto_runtimes = 3;   // ["openssl"] | ["jca"] | ...
  repeated string layers = 4;            // source|artifact|process|network|jvm-introspection
  repeated string detection_methods = 5; // the §2.3 enumeration
  string license = 6;                    // "Apache-2.0" | "GPL-3.0-or-later" ← surfaced in the UX
}

message CollectRequest {
  repeated string target_node_ids = 1;   // only what passed the scope master gate (§1.4)
  ScopeMasterRef scope = 2;
  map<string,string> options = 3;        // invasive options such as dynamic-trace (a separate PROPOSE gate)
}

message CollectionResult {
  Envelope envelope = 1;                 // §3.1
  bytes cbom_cyclonedx = 2;              // standard CycloneDX JSON
  repeated Finding findings = 3;
  Completeness completeness = 4;
}
```

### 4.2 The contract's three invariants

1. **The output is always a canonical CBOM Envelope** — whatever the backend, everything downstream (reconciliation, review, provisioning) behaves identically.
2. **The scope master gate is the core's responsibility** — the core filters the target nodes before handing them to a collector (§1.4). A collector collects only what it was given.
3. **Completeness is declared per layer** — a collector declares the layers it covers with `Describe` and reports what it actually covered with `Collect`. The core records "what was not observed" as a gap (§2.6).

### 4.3 The same mechanism as GPL isolation

This gRPC/CLI boundary *is* the **GPL contagion barrier**. A GPL collector (CipherIQ's `cbom-generator`, for instance) **runs as a separate process and exchanges only CycloneDX on stdout** → no library linking. The `gpl-adapter` wraps the subprocess and exposes it through the intake contract. → **the three isolation principles from the license notes hold automatically in code.**

---

<a id="repo-scope"></a>

## 5. Repository structure

```
pqcota/            # Apache-2.0 · public · the whole scope (Discovery, inventory, provisioning generation)
  # ── top level = kind of output / the stage sits inside it (contracts and pkg group by stage; discovery/… are execution entry points) ──
  ├─ contracts/proto/pqcota/{common,discovery,inventory,provisioning}/v1/  # the contract SSOT — namespace = stage
  ├─ gen/               # protobuf-generated code (committed · regenerate with make generate when a proto changes)
  ├─ pkg/               # library logic — grouped by stage:
  │    ├─ discovery/    #   the observed lane: normalize (§2.4), history (§2.4⑥ the snapshot store)
  │    ├─ inventory/    #   ingest (ingestion, CBOM intake SV-2), the central view (§5), the machine metadata store (endpoint/profile upsert), the hosts parser + declaration (the declared lane). There is no reconciliation or verdict engine
  │    ├─ provisioning/ #   the finalized-plan gate (§3.7), taxonomy→config generator (provisioning design §4.1·§4.2), L1/L2 playbooks, before capture, the rollback record store. Generation and persistence only
  │    └─ kernel/       #   shared rules crossing stages: registry, posture, scope, machineid, sign
  ├─ discovery/         # execution entry points (per stage):
  │    ├─ collectors/{openssl(Go),jvm(a Java sidecar ★),network(Go)}  # §1.6 plugins, the GPL isolation boundary
  │    └─ cmd/{pqcota-hosts(access prep),nodescan,netcap,jvmscan,procs,keygen}  # (the test harness is collectors/openssl/integration/probe)
  ├─ inventory/cmd/     # pqcota-ingest (ingestion) · pqcota-cbom-ingest (CBOM intake) · pqcota-discover-view (file view) · pqcota-inventory (central Postgres queries: endpoints, profiles, app attribution) · pqcota-profile (profile upsert) · pqcota-declare (declaration import) · pqcota-prune (retention truncation)
  ├─ provisioning/cmd/  # pqcota-provision (finalized plan → L2 playbook + before/rollback records) · pqcota-records (querying rollback records) — generation only
  └─ LICENSE (Apache-2.0), CONTRIBUTING.md, README.md

pqcota-collectors-gpl/  # GPL-3.0 · a separate repo · never bundled or linked (distribution separation)
  └─ adapters/cipheriq/  adapters/cbomkit/   # subprocess adapters only
```


---

## 6. The no-judgment principle

This repo **observes, normalizes, persists, and generates** — it collects facts, derives from them, and produces outputs.
**It does not judge** (§2.1): what to migrate and when to execute is decided by people.

The practical consequence is **the distinction between diff and reconcile**. **Change between snapshots (diff) is a description of observed fact**, so it is produced here — "libssl changed 3.0.13 → 3.5.0" is not a verdict. **Reconciling against a declaration (CMDB)** (three states, confidence), by contrast, decides "which is right", so it is not done.

### 6.1 Core components

| Component | Basis in the regulation | AUTO/PROPOSE/MANUAL |
|---|---|---|
| The collector intake contract (protobuf) + SDK | §1.6 | — (a contract) |
| `openssl-collector`: parses `/proc` and ELF symbols itself (no `ldd`, no `readelf`) | §2.3 | AUTO |
| `jvm-collector`: JVM attach → `getProviders()` | §2.2, §2.3 ★ | AUTO (while running); not running is a gap |
| Declaration import (loading a CMDB/asset registry) | §1.4 | AUTO |
| The six-step normalization pipeline + attaching `evidence_strength` | §2.4, §2.3 | AUTO |
| **Provider signature registry enrichment** (JCA provider_set → pqc_readiness, fips, algorithms, tagging the SLH-DSA gap) | §2.3 | AUTO |
| The completeness map (per layer) | §2.6 | AUTO |
| The discovery history (append-only) + **browsing history and diffing change between snapshots** (a description of observed fact, not a verdict — per §6) | §2.4-6 | AUTO |
| **The two-layer split of observation records and snapshots** — snapshots only when the substantive content changes, an observation record on every ingest. Repeated observation of the same state does not grow storage, yet "when we looked" is preserved | §2.4-6, §1.2 | AUTO |
| **Retention truncation** (`pqcota-prune`) — truncating old points of change. The newest is inviolable, and the truncation is recorded so a hole in the history is reported | §2.6 | — (on the user's instruction) |
| **The asset scope gate** (`scope.AssetPolicy`) — the node gate (§1.4) at asset granularity. Only what the user declared as managed is ingested, and the excluded count is reported | §1.4, §2.6 | AUTO (enforcement) |
| The central inventory view (CLI+UI) + machine metadata (endpoints, profiles) and **app attribution** | §6 | — |
| Provisioning generation (the finalized-plan gate → L1/L2 playbooks + before and **rollback records**) | provisioning design §4.1·§4.2 | — (generation only) |

### 6.2 Explicit exclusions / boundaries

This repo goes **as far as generation**. After that it splits three ways:

- **The user does it themselves (with this repo's output)** — the generated L1/L2 and rollback playbooks are **run** with the user's Ansible (apply and roll back). The tool generates what to change and how to undo it; execution and judgment are the user's (§2.1, no judgment).
- **Not in this repo (another engine, joined through the public contracts in `contracts/`)** — **reconciling against a declaration (CMDB)**: three-state reconciliation and confidence scoring, review-and-finalize governance, staged deployment orchestration and safety rails (L3 drain, rolling, fleet), dynamic provisioning, the deployment channel. *Diffing change between snapshots, though, is observed fact rather than reconciliation, so it is in this repo (per §6).*
- **PROPOSE, off by default** — dynamic-trace (invasive eBPF, §2.5).

> **Capability grows in this order** — ① a Discovery MVP → ② the central inventory (ingest, persist, query + machine metadata [endpoints, profiles] and **app attribution**) → ③ provisioning generation (L1/L2 playbooks + before and **rollback records**). The only join to anything outside is `contracts/`. This repository puts observations out; whatever picks them up sits on the far side of that contract — which is exactly what was excluded above.

### 6.3 The definition of done

For one real node (OpenSSL installed, a JVM running):
1. Only nodes registered in the scope master become collection targets (the §1.4 gate works).
2. The openssl and jvm collectors each return a **canonical CBOM Envelope**.
3. The normalization pipeline converges both outputs into **one inventory view**, with `crypto_runtime`, `detection_method`, and `evidence_strength` attached deterministically to each finding.
4. Processes that were not running and layers that were not collected remain **as gaps in the completeness map** (never misread as "absent", §2.6).
5. The whole process is appended to the discovery history and is reproducible by recomputation (§1.2).

---

## 7. Where things stand

What of §1–§6 is standing and what remains is tracked in one place only —
the "What was built", "What was established", "Not on the roadmap" and "Roadmap" sections of [RELEASE_NOTES](../RELEASE_NOTES.en.md).
A design document still carrying finished work as "next actions" is exactly the state where the
implementation has run ahead and the documentation has fallen behind.

Designs still under consideration, belonging to neither side yet, live in [under-review](under-review.en.md).
