English · [한국어](README.md)

# pqcota/contracts — the contract SSOT (Single Source of Truth)

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](README.md) is authoritative.

This directory holds the **one contract every component of the PQC migration platform depends on**.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

Supporting documents:
- [**Data model schema**](data-model.en.md) — purpose, key fields, and relationship map for every message and enum (a human-facing reference). Read it before the file list below and the whole picture snaps into place.
- [Process regulation](../docs/regulation.en.md)
- [Architecture & OSS boundary design](../docs/architecture.en.md)

## Files

The namespaces split into the **three product stages plus a shared vocabulary** (symmetrical with `pkg/`). The namespace alone tells you which stage a contract belongs to.

| File | Package | Defines | Basis in the regulation |
|---|---|---|---|
| `proto/pqcota/common/v1/common.proto` | `pqcota.common.v1` | shared vocabulary: Envelope, completeness, controlled-vocabulary enums (crosses all stages) | acceptance principles §2.4, §2.4, §2.7, §3.1 |
| `proto/pqcota/discovery/v1/cbom.proto` | `pqcota.discovery.v1` | derived Finding · OpensslAxes · JcaAxes | acceptance principles §2.4, §2.4, §3.2 |
| `proto/pqcota/discovery/v1/collector.proto` | `pqcota.discovery.v1` | collector intake gRPC service · CollectionResult | §1.6 |
| `proto/pqcota/discovery/v1/edge.proto` | `pqcota.discovery.v1` | communication edge observation · ObservedEdge · QuantumPosture | inventory design §6 |
| `proto/pqcota/discovery/v1/asset.proto` | `pqcota.discovery.v1` | asset hierarchy · Application · ProcessMatch · LiveProcess (Machine→App→Process) | §1.4, §2 |
| `proto/pqcota/inventory/v1/decision.proto` | `pqcota.inventory.v1` | review verdicts · Decision · DecisionStatus · DecisionConclusion | §2, §3.3③, §3.6 |
| `proto/pqcota/inventory/v1/machine.proto` | `pqcota.inventory.v1` | human-facing machine profile · MachineProfile · Environment (display name, environment, role, owner, tags) | §3 |
| `proto/pqcota/provisioning/v1/plan.proto` | `pqcota.provisioning.v1` | finalized plan · FinalizedPlan · RemediationAction · RemediationKind · DeployAutomationLevel | regulation §4.1·§3.7 · provisioning design §4.1·§4.2 |
| `proto/pqcota/provisioning/v1/rollback.proto` | `pqcota.provisioning.v1` | provisioning history and rollback · ProvisioningRecord · CryptoState (before/after) | §1.3, §4.3 |

> `common` is the contract-side counterpart of pkg/kernel — only shared vocabulary that belongs to no single stage (CryptoRuntime, DetectionMethod, Envelope, Completeness, and so on). Generated Go packages: `gen/pqcota/{common,discovery,inventory,provisioning}/v1` → `commonv1`, `discoveryv1`, `inventoryv1`, `provisioningv1`. The `Decision` and `FinalizedPlan` **schemas are SSOT** (so consuming engines share the same vocabulary).

## Core design decisions (read before you start)

### 1. Responsibility boundary — collectors do not enrich

```
Collector    →  §2.4 steps 1–2 : raw capture + conversion to standard CycloneDX + Envelope
Normalization →  §2.4 steps 3–6 : enrich → validate → resolve identity → persist → derive Finding[]
```

- The collector output (`CollectionResult`) contains **no `Finding`**. It returns only standard CycloneDX plus an Envelope.
- **Interpretive enrichment** — `evidence_strength`, `pqc_readiness`, fork determination — **is done by the core alone**.
  Why (regulation §1.2, §2.4): when an enrichment rule (a mapping table) improves, results must be **recomputed from the original**; if enrichment were scattered across collectors, recomputation would be impossible and the rules would drift apart. Enrichment lives in one place.
- Hence `Finding.derived_from_snapshot_id` plus `ruleset_version` are mandatory — which original and which rule a derivation came from is always traceable and reproducible (§1.2, audit integrity).

### 2. `unknown` is a first-class value (regulation §2.5)

Every enum's `*_UNSPECIFIED = 0` means "could not determine = unknown".
A field you could not fill is left as an **explicit zero value**, not blank or missing.
Do not silently treat it as "absent" — "genuinely not there" and "impossible to observe in principle" are distinguished by `Completeness.layers_missing`.

### 3. The provider signature registry drives enrichment (acceptance principles §2.3)

`JcaAxes.provider_set` (registration order included) is the **original** the collector observed.
The core enrichment step compares it against the provider signature registry and **derives** `pqc_readiness`, `fips_validation`, and algorithm coverage (identifying BouncyCastle / BC-FJA / JDK-native / openssl-jostle / in-house).
- **SLH-DSA is not in the JDK natively** → assets that need it are tagged as depending on BC/jostle regardless of JDK version.
- A `fips_validation` requirement surfaces in the Deploy stage **as a recommendation to use a FIPS-validated provider** (FIPS routing). The tool does not block the provider the plan chose — a validation certificate is per build, and you cannot tell from the file alone.
- The registry is a derivation rule, so it is pinned by `ruleset_version`; when it improves, recompute from the original (§1.2).

### 4. `deploy_automation_level` is a plan attribute, not a Discovery one (regulation §4.3 v4)

`DeployAutomationLevel` (L1/L2/L3) is registered here as controlled vocabulary, but **collectors do not fill it in.**
It is a plan/asset attribute a reviewer decides per asset (MANUAL) and it rides on the plan entity (the workflow lives on the plan side).
Discovery's `Finding` does not have this field — a deliberate separation to prevent stage confusion.

### 5. The gRPC boundary is the GPL-contagion barrier ([license notes](../docs/licensing.en.md))

The `Collector` service boundary is also the license isolation boundary.
A GPL collector (CipherIQ's `cbom-generator`, for instance) runs as a **separate process** and
**exchanges only CycloneDX on stdout**. The core never links it as a library.
`CollectorCapabilities.license` surfaces the implications to the user ([license notes](../docs/licensing.en.md)).

### 6. The plan schema is a public contract (regulation §4.1)

`plan.proto` (`FinalizedPlan`, `RemediationAction`, `RemediationKind`) is SSOT and therefore **OSS** — the provisioning artifact generator (`pkg/provisioning`) and the execution channel must speak the same vocabulary.
- The **`Executable()` gate** (§3.7, "only a finalized plan is grounds for execution") is a shared contract rule → OSS `pkg/provisioning`.
- Plan **authoring and review-finalization** (§3.3③) and **fleet orchestration** (§4.3) are out of scope.
- Turning the taxonomy (`RemediationKind`) into config fragments is a deterministic derivation (§1.2), so the OSS generator owns it.
> Saying `DeployAutomationLevel` and the plan entity are the plan's concern means *who owns the workflow*, not *where the schema lives*.
> Details: [provisioning design](../provisioning/design.en.md).

## CycloneDX `properties` extension key convention (§3.2)

Tool-specific enrichment rides on standard CycloneDX `properties` under the `pqcota:` namespace.
The core pipeline reads those keys and maps them into a typed `Finding`.

| property key | Value | Corresponding Finding field |
|---|---|---|
| `pqcota:crypto_runtime` | `openssl` \| `jca` | `crypto_runtime` |
| `pqcota:detection_method` | `source`\|`artifact`\|`symbol-analysis`\|`runtime-introspection`\|`dynamic-trace` | `detection_method` — **how it was seen**. Strength is derived from this (seeing the real thing beats inferring it) |
| `pqcota:usage_context` | `server`\|`client`\|`at-rest`\|`signing` | `usage_context` |
| `pqcota:openssl.fork` | `OpenSSL`\|`BoringSSL`\|… | `openssl.fork` |
| `pqcota:openssl.binding_mode` | `dynamic`\|`static`\|`dlopen`\|`vendored` | `openssl.binding_mode` |
| `pqcota:jca.provider_set` | CSV in registration order | `jca.provider_set` |
| `pqcota:jca.registration_mode` | `static`\|`dynamic`\|`explicit` | `jca.registration_mode` |
| `pqcota:app_keys` | CSV of app keys (a shared .so has several) | `app_keys` (repeated) — asset attribution (§1.5) |

> `evidence_strength` and `pqc_readiness` **do not go here** — they are core-derived values (decision 1 above).

> **A note for external collectors — you must carry `pqcota:detection_method`.** The core derives
> `evidence_strength` from `detection_method` deterministically (the §2.3 table). Without that key the
> core does not invent an evidence strength; it falls honestly to `UNSPECIFIED` (§2.5 — unknown is
> first class, no guessing). That is **the regulated outcome, not a penalty for forgetting.** A
> collector that emits only standard CycloneDX (CBOMkit and friends) has to pass through an import
> adapter that maps its `cryptoProperties` onto these keys for strength to survive — assets that come
> in without the mapping stay at unknown strength.

## Versioning and compatibility rules

- Packages are `pqcota.{common,discovery,inventory,provisioning}.v1`. **A breaking change means a new `v2`** — never reuse a `v1` field number or change its meaning.
- When a field is removed, mark its number `reserved`. Adding an enum value is backward compatible (always append at the end).
- This contract belongs to this repo (Apache-2.0) — the canonical CBOM schema and profiles are public (§5.1).

## When you change the contract — ripple check

Fixing the proto is not the end. **Two things in the code are derived from the contract, and forgetting either breaks things silently.**

| Also look at | When | If you forget |
|---|---|---|
| [`sign.Canonical`](../pkg/kernel/sign) | **adding a field** to `CollectionResult`, `Envelope`, `MachineIdentity`, `Completeness`, or `ObservedEdge` | the new field becomes a **signature blind spot** — tamper with it and verification still passes.<br>Widening the scope **invalidates every existing signature**, so after a release it needs a migration |
| [`history.ContentHash`](../pkg/discovery/history) | **adding a substantive content field** to `Finding`, `ObservedEdge`, or `Completeness` | a change to that field folds into "no change" and **vanishes silently from the history** ([inventory design §7.3](../inventory/design.en.md)) |

**Tests watch both** — if the field count changes, `TestCanonicalCoversAllFields` fails and tells you what to do. Do not wave the failure away by editing the expected value. That is precisely how blind spots get made.

**If you changed a derivation rule**, bump `ruleset_version` and **recompute** past snapshots from their originals to get the new verdicts (§1.2). A derived value is a function of the rule, not a stored value.

Watch out for **fields where order carries meaning** — `JcaAxes.provider_set` registration order determines priority negotiation (acceptance principles §2.2), so it is never sorted or normalized.

## Code generation

```bash
# generated output is not committed — when the contract changes, everyone regenerates
make generate                              # = cd contracts && buf generate
cd contracts && buf lint                   # or make lint

# compatibility checks run **from the repo root** — .git is there and the protos are under contracts/, so a subdir is needed
buf breaking contracts --against '.git#branch=main,subdir=contracts'
buf breaking contracts --against '.git#ref=HEAD~1,subdir=contracts'   # against the previous commit
```
