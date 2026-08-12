English · [한국어](data-model.md)

# Data model schema (the contracts SSOT reference)

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](data-model.md) is authoritative.

For the list of contract files and namespaces and the CycloneDX property mapping, see [contracts/README](README.en.md).

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

## 0. Conventions

- **Namespace = stage**: `pqcota.common.v1` (shared) · `pqcota.discovery.v1` · `pqcota.inventory.v1` · `pqcota.provisioning.v1`. The generated Go packages are `commonv1`, `discoveryv1`, `inventoryv1`, `provisioningv1`.
- **protojson representation**: fields are **camelCase** (`target_node_id`→`targetNodeId`), `bytes` is a **base64 string** (`cbom_cyclonedx`), enums are **name strings** (`"DETECTION_METHOD_RUNTIME_INTROSPECTION"`), and `Timestamp` is RFC3339.
- **enum 0 = `*_UNSPECIFIED` = "unknown"** (§2.5). Not "absent" but "could not determine" — together with the completeness map it separates "genuinely not there" from "impossible to observe in principle".
- **Backward compatibility**: never reuse a field number, mark removals `reserved`, append enum values only at the end, and make breaking changes as a new `v2`.

## 1. Four principles that run through the model

These explain why the fields split the way they do.

1. **Provenance lane separation (§1.2/§1.3)** — lanes are split by where the data came from. They are never mixed.
   - **Observed**: what a collector actually saw — `CollectionResult`, `ObservedEdge`, `MachineIdentity`.
   - **Declared**: what a CMDB or a user filled in — `MachineProfile`, the scope master, declared edges.
   - **Derived**: views the core **recomputes** from observations — `Finding`, `evidence_strength`, `QuantumPosture`. **They are not in collector output.** Always regenerable from the raw original (reproducibility).
   - **Action**: an append-only record of what the tool did — `ProvisioningRecord`, `Decision`.
2. **Identity model (§1.4)** — three layers. **Authority** = `node_id` (the scope master / CMDB; stable and globally unique). **Correlation** = the `MachineIdentity` fingerprint (machine-id, hw-uuid, cloud-id, fqdn — verifies node_id, and derives a self-id when there is no CMDB). **Locator** = the IP (not an ID; used only to resolve a network observation to a node).
3. **Derived-view reproducibility (§1.2)** — derivations are produced from `raw_capture` (the immutable original) by enrichment rules. That is why derived messages carry `derived_from_snapshot_id` and `ruleset_version` (which original and which rule they can be reproduced from).
4. **Secrets are never persisted (§1.5)** — access secrets (SSH keys, passwords, accounts) **have no field in any schema**. `MachineEndpoint` is the representative case: its type cannot hold a secret, so it is guaranteed at compile time.

---

## 2. `common.v1` — shared vocabulary (crosses stages)

### Controlled vocabulary (enums) — shared by all stages
| Enum | Meaning | Values (0=UNSPECIFIED omitted) |
|---|---|---|
| `CryptoRuntime` | the crypto runtime — what is accepted is in the [acceptance principles](../docs/runtime-acceptance.en.md). The first-class branch for every finding, asset, and remediation | `OPENSSL` · `JCA` · `WIN_CNG` (**schema reserved** in v0.1.0, unimplemented — the code that fills it comes in v0.5.0+) |
| `DetectionMethod` | the detection method. Reported by the collector → the basis for deriving evidence | `SOURCE`·`ARTIFACT`·`SYMBOL_ANALYSIS`·`RUNTIME_INTROSPECTION`·`DYNAMIC_TRACE` |
| `EvidenceStrength` | evidence strength. **Derived from detection_method** (only the core fills it) | `CONFIRMED`·`INFERRED_HIGH`·`INFERRED_LOW` |
| `UsageContext` | usage context | `SERVER`·`CLIENT`·`AT_REST`·`SIGNING` |
| `CollectionLayer` | collection layer (the unit of the completeness map) | `SOURCE`·`ARTIFACT`·`PROCESS`·`NETWORK`·`JVM_INTROSPECTION` |
| `OpensslBindingMode` | OpenSSL binding | `DYNAMIC`·`STATIC`·`DLOPEN`·`VENDORED` |
| `JcaRegistrationMode` | JCA provider registration | `STATIC`·`DYNAMIC`·`EXPLICIT` |

### Messages
| Message | Purpose | Key fields |
|---|---|---|
| **`Envelope`** | the provenance attached to every collection output (§3.1) | `collector_id`·`detection_method`·`target_node_id` (the authoritative anchor)·`scope_master_ref`·`signature` (ed25519)·`collector_license`·`machine` (fingerprint) |
| **`MachineIdentity`** | the machine correlation and self-id fingerprint (§1.4). Filled by the collector | `machine_id`·`hardware_uuid`·`cloud_instance_id`·`fqdn`·`ips` (locators)·`self_assigned_id` (deterministically derived when there is no CMDB)·`derived_from` |
| **`Completeness`** | per-layer coverage (§2.6) | `layers_covered`·`layers_missing` (the gap — never auto-treated as "absent")·`note` |

---

## 3. `discovery.v1` — observation and derivation

### `collector.proto` — the intake contract (what a collector **returns**, §1.6)
The core depends only on the abstraction "give it a node, get back a canonical CBOM". This gRPC boundary is also the GPL contagion barrier (license notes).

| Message | Purpose | Key fields |
|---|---|---|
| `CollectorCapabilities` | capability declaration (`Describe`) | `crypto_runtimes`·`layers`·`detection_methods`·`license`·`invasive` (invasive → the PROPOSE gate) |
| `CollectRequest` | a collection request | `target_node_ids` (only those past the scope gate)·`options` |
| **`CollectionResult`** | one canonical CBOM envelope | `envelope` · `raw_capture` (the immutable original) + `raw_format` · **`cbom_cyclonedx`** (base64 standard CycloneDX body) + `cyclonedx_spec_version` · `completeness` · `observed_edges` |

> Collector = §2.4 steps 1–2 (raw capture + CycloneDX conversion) + the Envelope. **It does not produce derived `Finding`s.**

### `cbom.proto` — the derived `Finding` (**produced** by the normalization pipeline, §2.4 steps 3–6)
The typed view the core normalization pipeline derives from the `cbom_cyclonedx` body. **Recomputable** (§1.2).

| Message | Purpose | Key fields |
|---|---|---|
| `OpensslAxes` | the OpenSSL branch axis | `lib`·`version`·`fork` (OpenSSL/BoringSSL/…)·`binding_mode` |
| `JcaAxes` | the JCA branch axis | `jdk_vendor`·`jdk_version`·`provider_set` (**order is meaningful** — priority negotiation)·`registration_mode` |
| `CngAxes` | the Windows CNG branch axis (**reserved in v0.1.0**, unimplemented) | `provider_set` (KSP/SSP, **order is meaningful**). The remaining fields, which real observation will settle, are added additively in v0.5.0 |
| **`Finding`** | one crypto asset (a derived view) | `id` (canonical hash)·`crypto_runtime`·`usage_context`·`algorithm` · `detection_method` + **`evidence_strength`** (derived) · `oneof {openssl\|jca}` · `pqc_readiness`·`fips_validation`·`remediation_class` · `derived_from_snapshot_id` + `ruleset_version` (reproduction) · **`app_keys`** (asset attribution; a shared .so has several) |

### `asset.proto` — the asset hierarchy (Machine → Application → Process)
| Message/enum | Purpose | Key fields |
|---|---|---|
| `ApplicationKind` | the origin of the stable key | `SYSTEMD_UNIT` (recommended)·`EXE_PATH`·`DECLARED` |
| **`Application`** | the target app (the first-class unit of provisioning). Globally identified by `(node_id, app_key)` | `node_id`·`app_key`·`name`·`kind`·`match` |
| `ProcessMatch` | the rule matching an app to a live process (no PID is stored) | `systemd_unit` (cgroup, exact) > `exe_path` > `cmdline_regex` |
| `LiveProcess` | the result of runtime resolution (volatile, query-only) | `pid`·`cmdline`·`started_at` |
| `ProcessResolution` | a snapshot of an app's live processes | `node_id`·`app_key`·`processes`·`resolved_at` (stale immediately) |

> **Processes are not stored** — a PID is volatile. It is **resolved live** through `ProcessMatch` right before provisioning.

### `edge.proto` — communication edges (relations between nodes)
| Message/enum | Purpose | Key fields |
|---|---|---|
| `NetworkProtocol` | the observed protocol | `TLS`·`SSH`·`QUIC` (its handshake is encrypted → usually unknown) |
| `EdgeRole` | the direction of src | `CLIENT`·`SERVER` |
| `QuantumPosture` | quantum posture (§1.6). A **derived view** — the core classifies it from `negotiated_group` | 🟢`PQC_HYBRID`·🔴`CLASSICAL`·⚪`UNSPECIFIED` |
| **`ObservedEdge`** | one observed communication edge | `src_node_id`·`dst_node_id` (empty plus `dst_addr` if unresolved)·`protocol`·`role`·**`negotiated_group`** (the posture input)·`cipher`·`observed_count`·`first/last_seen` |

---

## 4. `inventory.v1` — metadata and verdicts

### `machine.proto` — machine metadata (human-facing information, **separate** from identity)
| Message/enum | Purpose | Key fields |
|---|---|---|
| `Environment` | the deployment environment (a visual axis) | `PRODUCTION`·`STAGING`·`DEVELOPMENT`·`TEST` |
| `ProfileSource` | where the profile came from | `CMDB`·`REVIEWER`·`OBSERVED` |
| **`MachineProfile`** | the metadata people read to tell machines apart (filled by declaration or a reviewer) | `node_id` (the anchor)·`display_name`·`environment`·`role`·`owner`·`location`·`labels` (map)·`source` |
| **`MachineEndpoint`** | **reusable connection metadata** for reconnecting during discovery | `node_id`·`name`·`ip`·`port` — ★**no secret field** (keys, accounts, and passwords live only in the user's files, §1.5) |

### `decision.proto` — review verdicts (schema only — there is no verdict engine)
The inventory counterpart of `FinalizedPlan` (provisioning). When a verdict is finalized it leads into a finalized plan.

| Message/enum | Purpose | Key fields |
|---|---|---|
| `DecisionStatus` | the verdict lifecycle (§3.3③) | `DRAFT`·`IN_REVIEW`·`FINALIZED` |
| `DecisionConclusion` | the reviewer's conclusion (especially for UNOBSERVED items) | `EXISTS`·`STALE`·`EXCLUDED`·`APPROVED` |
| **`ReconState`** | the result of reconciling a declaration against observation (vocabulary only — **the engine is not in this repository**) | `CONFIRMED` (declared ∩ observed) · `UNDECLARED` (observed only = shadow) · `UNOBSERVED` (declared only — no machine decides this) |
| **`Decision`** | one verdict | `subject` (an edge or policy ID)·`conclusion`·`status`·`reviewer`·`signature`·`basis_hash` (invalidated when the basis changes)·`derived_from_snapshot_id` |

---

## 5. `provisioning.v1` — generation and rollback

### `plan.proto` — the finalized plan (the only grounds for provisioning to run)
| Message/enum | Purpose | Key fields |
|---|---|---|
| `DeployAutomationLevel` | staged delegation of deployment (§4.3). Decided per asset | `L1_STAGE_ONLY`·`L2_STAGE_INSTALL` (the production default)·`L3_FULL_AUTO` (through activation and restart — the plan's `activation` hook) |
| `PlanStatus` | the plan lifecycle | `DRAFT`·`IN_REVIEW`·**`FINALIZED`** (the grounds for execution, the §3.7 gate) |
| `RemediationKind` | the kind of remediation (provisioning design §4.1·§4.2) → the core generator's branch | `CONFIG_ONLY`·`PROVIDER_INJECT`·`FORK_REPLACE`·`PROXY_FRONT`·`REBUILD`·`JDK_UPGRADE`·`APP_RECONFIG`·`DECOMMISSION` |
| **`RemediationAction`** | the remediation for one asset | `target_node_id`·`finding_id`·`crypto_runtime`·`kind`·`automation_level`·`target_algorithm`·`provider_choice`·`provider_class` (an explicit FQCN — without it only known names are certain)·**`config_artifact`** (rendered by the core generator)·**`activation`** (the L3 hook)·`rollback_note`·`priority` |
| **`ActivationHooks`** | the **commands the user wrote** to run at L3 (activation differs per environment, so the tool does not guess) | `pre`·`activate`·`deactivate`·`restart` — the generator places them in a meaningful order: forward `pre→stage→activate→restart`, rollback `pre→deactivate→remove→restart` |
| **`FinalizedPlan`** | the finalized plan (schema only — there is no authoring or finalization engine) | `id`·`status`·`scope`·`actions`·`approval_signatures` (a precondition of finalize)·`derived_from_snapshot_id`·`ruleset_version` |

### `rollback.proto` — provisioning history and rollback (the schema is OSS, and this repo generates the rollback playbook too)
| Message/enum | Purpose | Key fields |
|---|---|---|
| **`CryptoState`** | the crypto state at a point in time (shared by before/after) | `modules` (e.g. `libcrypto.so.3@3.0.13`)·`config_digest`·`provider_chain`·`config_snapshot_ref` (a reference to the original text, for rollback) |
| `ProvisioningStatus` | progress (a stage boundary is a rollback point) | `STAGED` (L1)·`INSTALLED` (L2)·`ACTIVATED` (L3 — activation and restart done)·`ROLLED_BACK`·`FAILED` |
| **`ProvisioningRecord`** | the append-only record of one provisioning act | `node_id`·**`app_keys`** (the affected apps; a shared .so has several)·`action_id`·`plan_id`·**`before`** (the rollback baseline)·`after`·`status`·`note`·`at` |

---

## 6. Relationship map — how the messages connect

```
[Collector]  ──returns──▶  CollectionResult { Envelope(+MachineIdentity) · raw_capture · cbom_cyclonedx · ObservedEdge[] · Completeness }
                                     │  (§1.4 scope gate · ed25519 verification)
                                     ▼
[Normalize]  ──derives──▶  Finding[] (evidence_strength·app_keys) ─┐   ObservedEdge + QuantumPosture (derived)
                                     │                            │
                          app_keys attribution                    │
                                     ▼                            ▼
                             Application (node_id, app_key)   [central inventory view]
                                     │                            ▲  ▸MachineEndpoint · MachineProfile (the metadata lane)
                         ProcessMatch │ (live)                    │
                                     ▼                            │
                              LiveProcess (volatile)         [review] Decision ══(finalize)══╗
                                                                                             ║
                                                                                             ▼
[Provisioning]  FinalizedPlan { RemediationAction[] } ──§3.7 FINALIZED gate──▶ playbooks (L1/L2/L3)
                                     │                                                  +
                                     ▼                                         ProvisioningRecord
                          before = CryptoState(the Findings)  ────────────▶  { before/after · app_keys · status }  (append-only rollback basis)
```

**Seen again as lanes**: observed (`CollectionResult`, `ObservedEdge`) → derived (`Finding`, `QuantumPosture`) → declared/metadata (`MachineProfile`, `Decision`) → action (`ProvisioningRecord`). `node_id` is the anchor threading every lane, and `app_key(s)` attributes crypto assets to apps, flowing from discovery all the way to provisioning.

> **`app_key` is not everywhere.** `Finding` and `ProvisioningRecord` attribute to an app, but
> **`ObservedEdge` stops at the node** — passive observation of the wire carries no PID for the socket
> that opened the connection. So when two processes on one node use the same library, which of them
> owns an observed edge is unknown. The two observations meet only at `node_id`
> Since v0.3.0 an edge also carries `app_key` through to the app — though the automatic path misses
> short-lived connections, so an empty `app_key` means "could not attribute", not "no app"
> ([under review §5](../docs/under-review.en.md)).

---

Related designs: [discovery](../discovery/design.en.md) · [inventory](../inventory/design.en.md) · [provisioning](../provisioning/design.en.md) · [architecture and the OSS boundary](../docs/architecture.en.md). Runnable examples: [examples/](../examples).
