English · [한국어](design.md)

# Discovery subsystem design

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](design.md) is authoritative.

**What this document is**: the technical design of the Discovery subsystem. Per-situation acceptance criteria are in the [test cases](testcases.md) (SD-1–SD-7, Korean). It joins regulation §2, the [architecture design](../docs/architecture.en.md), and the [contracts SSOT](../contracts/) at the implementation level.
**Scope**: Phase 0 (read-only discovery + normalization + inventory views). No judgment, no remediation.
**Design discipline**: the only things built here are the **runtime-lane collectors (SD-1, SD-2, SD-3) and the honest evidence layer**. Source and artifacts are territory CI can see, so no collector is placed there.


> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

**Which rules this implements** — when the regulation changes, this table finds the sections to fix.

| This document | The regulation |
|---|---|
| 1. Component architecture | §2.1 purpose and boundary · §1.6 the intake contract |
| 2. Collector detailed design | §2.2 the collector landscape · §2.3 three-layer cross-checking · §2.3 absent source |
| 3. The six-step normalization pipeline | §2.4 the normalization pipeline (immutable originals are §1.2) |
| 4. The scope gate and router | §1.4 the scope master |
| 4A. Machine identity and the UID scheme | §1.5 machine identity, the asset hierarchy, and the access boundary |
| 5. The completeness map | §2.6 outputs/integrity (gap ≠ absence is §2.5) |
| 6. Collector deployment | §4.3 staged deployment · §4.4 the trust boundary |

Automation grades (AUTO/PROPOSE/MANUAL) are set by §2.5 and taken as given here — the grades themselves are not re-decided in this document.

---

## 0. Concepts that run through everything

- **The observed lane vs the declared lane** — what a collector *saw* (observed) and what a user *reported* (declared/CMDB) are labeled apart by the Envelope's `detection_method`. That label is the basis on which the inventory keeps them separate.
- **`evidence_strength`** — `confirmed` (the running reality) → `inferred` (a static inference) → `unknown`. Attached deterministically according to how it was observed.
- **The completeness map (gap ≠ absence)** — a layer that was not observed is not declared "absent"; it is stated as a **gap**. That honesty is the basis of audit integrity and reproducibility.
- **The provider signature registry** — provider and module signatures → automatic determination of PQC maturity, FIPS status, and algorithm coverage.

---

## 0.1 The axes that separate situations

A scenario is a combination of the axes below. The combination is what determines coverage, evidence strength, and the division of responsibility.

| Axis | Values | Effect |
|---|---|---|
| Runtime | OpenSSL / JCA (Java) | branches the collection method and schema ([acceptance principles](../docs/runtime-acceptance.en.md)) |
| Evidence availability | source / artifact / binary only / running | `evidence_strength` (§2.3) |
| Host environment | bare metal·VM / container / air-gapped·batch | deployment and namespaces (§2.3) |
| Deployment tier | T1 self-service / T2 substrate / T3 resident | the trust barrier (§2.3) |
| Scope state | registered / unregistered-but-observed | the gate (§1.4) |
| Collection result | confirmed / degraded / unknown / gap | the completeness map (§2.6) |

---

## 1. Component architecture

```
                    [scope master gate] ──(registered nodes only)──┐  §1.4
                                                                   ▼
  ┌─ intake contract (gRPC, contracts/collector.proto) ───────────────┐
  │                                                                   │
  │  the runtime lane (built here)                                    │
  │  ├ openssl-collector (Go)    SD-1·SD-3                            │
  │  │   /proc·ELF·ss·the fork matcher                                │
  │  ├ jvm-collector (Java)      SD-2                                 │
  │  │   attach→getProviders()                                        │
  │  └ (namespace wrapping)      SD-4          (the declared lane)     │
  └───────────────────────────┬───────────────────────────────────────┘
                              │  CollectionResult(Envelope+raw+CycloneDX+completeness)
                              ▼
   [the six-step normalization pipeline]  §2.4 ── enrichment input: crypto-registry (provider signatures)
   raw capture→parse→enrich→validate→resolve identity→persist
     │                              └ evidence_strength·fork determination·role tagging (the honest evidence layer)
     ├─▶ [completeness map]  gap≠absence  SD-6
     ├─▶ [out-of-scope router]  registration request  SD-5
     ▼
   [discovery history] append-only, reproducible (§1.2)  ──▶ read-only inventory views
```

**One contract**: a collector hands its results over through exactly one opening — the `Collector` intake contract (§1.6). What sits behind that opening is unknown to the **core** (the central service that owns normalization, the inventory, and the API — [architecture §1.2](../docs/architecture.en.md#12-the-recommended-combination)) — collectors can multiply and the core stays the same. How a result was obtained is revealed only through `Envelope.detection_method`.

---

## 2. Collector detailed design

> **Terminology**: **a collector is a node observer *provided by pqcota*** (behind the intake contract; **runtimes**: openssl, jvm, network).

> **Platform constraint (current scope)**: **every binary is Go** (collectors and operator CLIs alike) — the only non-Go piece is the jvm-collector's Java sidecar. So **the Linux-only vs cross-platform split is decided not by language but by "what it touches"**:
> - **Touching Linux OS APIs = Linux-only.** openssl (§2.1) and process scanning depend on `/proc` and ELF; network (§2.3) depends on AF_PACKET (`//go:build linux`, `CAP_NET_RAW`). The premise is that **what is observed is a Linux legacy crypto-runtime server.** The jvm-collector (§2.2) runs on the JVM (pure Java) so it is portable in principle, but its target is Linux servers (the verified scope today is Linux). The attach client reuses the machine's existing JDK (no runtime is bundled).
> - **Touching only files and databases = cross-platform.** The central/operator CLIs (ingest, inventory, provision, and so on) touch no OS primitives, so they run on macOS and Windows operator machines too.
>
> **Observing crypto runtimes on Windows/macOS servers needs its own collector implementation** and is out of scope today. The distributed binaries are static (`CGO_ENABLED=0`), so cross-compiling across **Linux × arch (amd64/arm64…)** is trivial — the matrix is per arch, not per OS.

### 2.1 openssl-collector (Go) — SD-1, SD-3

**Responsibility**: collect the file, process, and symbol layers of the OpenSSL runtime. Implemented **self-sufficiently in Go without depending on** `ldd`/`lsof`/`ss`/`readelf` (to work on minimal images; the §2.3 footprint).

| Layer | Method | detection_method | Output |
|---|---|---|---|
| Package/file | dpkg/rpm DB queries, an FS walk, `debug/elf` NEEDED and reverse dependencies | artifact | lib, version, path |
| Process | parsing `/proc/<pid>/maps` (the loaded libssl, **catching dlopen and vendoring**), `/proc/<pid>/fd`, TLS posture via netlink | runtime-introspection | what is actually loaded |
| Symbol (static/stripped) | ELF `.rodata` strings + symbol signatures → **the fork matcher** (§2.2) | symbol-analysis | fork and version inferred |

> **The provider layer is not observed.** This goes as far as the library path, fork, and version; a provider layered on top (whether `oqsprovider` is already installed, for instance) is not observed — this is where it parts ways with the jvm-collector, which sees the entire provider chain through attach (§2.2). It does not block provisioning: which provider to use is written into the plan by the user, and the version evidence is in the inventory. Even re-observing after a remediation, a change at this layer does not appear in the inventory.

**The fork signature matcher (SD-3, the core IP)** — solving the identical-soname problem (acceptance principles §2.2):
```go
// Fork signatures from the crypto registry. Distinguishes OpenSSL/BoringSSL/LibreSSL/AWS-LC.
type ForkSignature struct {
    Fork      string   // "OpenSSL" | "BoringSSL" | "LibreSSL" | "AWS-LC"
    Strings   []string // e.g. "OpenSSL 3.0.2", "BoringSSL", AWS-LC build markers
    Symbols   []string // exported/internal symbol name patterns
    Version   string   // the confirmed or inferred version on a match
}
// No match → OpensslAxes.Fork = "" (unknown, §2.5). evidence_strength=inferred-low.
```
**Output**: a CycloneDX component (lib) + `pqcota:` properties (crypto_runtime=openssl, detection_method, openssl.fork, binding_mode) + raw_capture (native JSON) + an Envelope. **Deriving the Finding is the core's job** (the §2.4 contract).

### 2.2 jvm-collector (a pure Java sidecar) — SD-2 ★the killer

**Responsibility**: query the **reality** of a live JVM's provider chain. Built in-house because it is a dedicated-OSS gap (§2.2).

**The attach strategy**:
```
1. JVM recon  : enumerate running JVMs by scanning /proc (ScanJVMs, pure Go)
                — exe=java or libjvm.so in maps; extract PID, JAVA_HOME, version (release), app (cmdline)
2. attach     : VirtualMachine.attach on each PID found → loadAgent(introspect-agent.jar)
                (assumes the same UID or root, and that DisableAttachMechanism is not set)
3. the agent  : runs inside the target JVM —
                Security.getProviders() → collect per provider {name, version, className,
                registration order, list of services (algorithms)} → return over a socket or file
4. detach     : back to as it was (read-only, no state change)
```

**Recon comes first — symmetrical with openssl (§2.1).** Just as openssl's `ScanHost` walks `/proc` and finds the loaded libraries itself, jvm's `ScanJVMs` **investigates running JVMs directly** (removing the asymmetry where the caller had to know the PID and JDK in advance). Inaccessible processes are counted as `Denied` and become a source of completeness gaps (§2.5). `AttachAll` attaches to each JVM it found, and **an attach failure is not silently dropped either — it is counted as a gap** (symmetrical with openssl summing detections per process). Implementation: `collectors/jvm/{procscan,attach}.go`.

**Identifying multiple JVMs — by app, not by PID.** When a node has several JVMs, each must be a **distinct finding** (if one vanished into a dedup it would violate §2.6 honesty — a real asset concealed). The identifier is the **app** first (the main class or `-jar` in the cmdline), falling back to JAVA_HOME. **PIDs are not used** — they change on every scan, which would shake the finding id and break the history into "a new asset every time" (two apps on the same JDK are also separated by the app key).
**What is bundled**: only `introspect-agent.jar` (the attach sidecar). **No runtime is bundled** — the attach client does not have to be the target's java, so **the JDK already on the machine is reused** (even if the target is a JRE). If no attach-capable JDK exists at all, it degrades honestly to the static fallback → [collector deployment design](collector-deployment.md) (Korean).

**Policy and artifacts are collected alongside** (on-host files):
- Parse the registration order in `java.security` plus `jdk.tls.*` and `disabledAlgorithms`.
- Provider JAR signatures (`bcprov-jdk18on`, `BC-FJA`, `SunJCE`, …) → mapped through the crypto registry.
- What the jvm-collector sees is **the runtime reality plus the on-host policy**.

**The degradation path (the JEP 451 response — decided)**: when attach is impossible —
`DisableAttachMechanism`, or **future JDKs blocking dynamic attach by default under JEP 451 without
`-XX:+EnableDynamicAgentLoading`** — the response is a three-step strategy:
1. **First choice**: dynamic attach → the reality of `getProviders()`, `evidence_strength=confirmed`.
2. **The guaranteed fallback**: when attach fails, degrade to the **static path** (the `java.security` registration
   order, JAR/classpath signature scanning) → lower `evidence_strength` (confirmed→inferred) and **record a gap**
   in the completeness map for "runtime-introspection not collected" (§2.5, "unknown is first class", no automatic
   absence). **Dynamic registration (`addProvider`) is a blind spot in that case**, and that fact itself is stated as a gap.
3. **An operational option**: for high-value assets that need confirmed evidence, **recommend** (through operational
   agreement) that the target JVM be started with `-XX:+EnableDynamicAgentLoading` (or `-XX:+StartAttachListener`).
   The platform does not force it — how the user's assets start is the user's own ([[deploy-script-boundary]] symmetry).

> Non-agent paths (JMX/JVMTI) are for later review. JMX is usually disabled too, and a native JVMTI agent also needs
> a startup flag, so neither fully solves the "legacy dominant case". Hence **this repo's guaranteed fallback is fixed
> as static degradation.**

**Provider registry mapping** → `pqc_readiness`, `fips_validation`, algorithm coverage. **SLH-DSA is not in the JDK natively → tag it as depending on BC/jostle** (§2.3).

### 2.3 network-collector (Go, AF_PACKET) — the network layer (Phase 1)

**Responsibility**: **passively observe** real TLS/SSH handshakes to catch the negotiated cryptography and the communication edges (the §2.2 network layer).
Where the other collectors see a node's **capability** (the loaded library = whether PQC is possible), the network-collector sees the **actual posture** (whether that connection really negotiated a PQC hybrid). Comparing the two is the core value.

**The technical basis — observation without decryption**: the algorithm negotiation in a handshake is **plaintext**.
- **TLS**: ClientHello `supported_groups` and `key_share`, the group ServerHello selects → the KEX group (X25519MLKEM768 and the like), cipher, and version are observed.
- **SSH**: the KEX algorithms in KEXINIT (whether it is PQC, e.g. `sntrup761x25519`) — legacy environments lean heavily on SSH, so this is valuable.
- Encrypted parts such as QUIC and TLS 1.3 certificate signatures cannot be seen → honestly marked `unknown`.

**Implementation**:
```
capture : pure Go AF_PACKET (x/sys/unix, CAP_NET_RAW), a BPF filter for handshake records only (payload excluded → privacy)
parse   : ClientHello/ServerHello, SSH KEXINIT → the negotiated algorithms and KEX group
output  : communication edges (src→dst:port, negotiated_group, role, tls/ssh version)
return  : a CollectionResult (the observed lane). crypto_runtime=UNSPECIFIED (TLS≠OpenSSL, weak attribution, §2.2)
          — it fills in **communication edges**, not node crypto Findings (inventory §2 ObservedEdge)
```

**Privilege and invasiveness**: `CAP_NET_RAW` (on par with the openssl collector's `CAP_SYS_PTRACE`). Being **passive and non-invasive**, it is lighter than eBPF dynamic-trace (PROPOSE). It does touch the data plane, though, so: a handshake-only filter plus avoiding self-reference (§2.6).

**Limits (always stated as gaps)**:
- **The observation window**: only traffic that flowed during the capture — idle, batch, and DR links go unobserved → a gap (≠ absence, §2.6). Repeat across time windows (§2.3).
- **Coverage dependence**: only connections on the host where the collector runs. An edge with neither side instrumented is invisible without a SPAN or tap.
- **Attribution**: out-of-scope IPs, NAT, and proxies → "a registration decision request" (§5).

> **A Phase 1 capability** (observation in parallel + shadow discovery). This edge observation becomes the observed source for inventory reconciliation and completes the **crypto communication topology** ([inventory design](../inventory/design.en.md) §6).

---

## 3. The six-step normalization pipeline (running through every scenario)

Regulation §2.4 is fixed as an interface. **Enrichment, validation, identity, and persistence are the core's alone** (so that §1.2 recomputation is possible).

| Step | Input→output | Core logic | Scenarios |
|---|---|---|---|
| ① raw capture | collector raw → append-only storage | immutable preservation, the source for renormalization | all |
| ② parse/convert | native → canonical CycloneDX | converge the spec version on the internal canonical version | all |
| ③ **enrich** | CycloneDX+Envelope → Finding | attach `EvidenceStrength(method)` deterministically, map through the crypto registry (fork, provider), tag the server/client role, pqc_readiness | the heart of SD-2 and SD-3 |
| ④ validate | Finding → validated Finding | schema + plausibility (flag contradictions) | all |
| ⑤ identity/dedup | node = anchored on the scope master, finding = a canonical hash | merging re-collections | all |
| ⑥ persist | → append to the discovery history | `derived_from_snapshot_id` + `ruleset_version` | §1.2 |

The core is the only source of enrichment. Collectors do not reimplement the same logic → when a rule improves, recompute from the original (§1.2).

---

## 4. The scope gate and router — SD-5

- **The pre-gate**: `CollectRequest.target_node_ids` contains only what the core has **already filtered** through the scope master (§1.4). A collector collects only what it was given.
- **Post-routing**: an out-of-scope node observed during collection (a communication peer, say) → **not a collection target** → into the "registration/exclusion **decision request**" queue (PROPOSE). Automatic collection is forbidden.
- Output: a new review item (for a MANUAL user verdict). The gap in scope-master consistency is itself a service.

## 4A. Machine identity, the UID scheme, and the asset hierarchy (contracts: `discovery/asset.proto`, `common.proto MachineIdentity`)

> The rules are set by **§1.5** — the three-layer separation of identity, correlation, and location; no PID storage;
> no ingestion of access secrets. What is written here is how that is implemented.

### 4A.1 Machine UID — a three-layer separation (no confusion, no duplication)

Identity (ID), correlation (fingerprint), and location (IP) are not mixed:

| Layer | What | Note |
|---|---|---|
| **Authoritative ID** | `node_id` = the scope master (CMDB) (§1.4) | **The main path: the user supplies it when starting discovery** (an Ansible inventory or the CMDB). Stable and unique |
| **Correlation fingerprint** | `Envelope.MachineIdentity`: `machine_id` (/etc/machine-id), `hardware_uuid`, `cloud_instance_id`, `fqdn` | **anchors and verifies** the user's label against a physical machine (it does not create one) |
| **Locator** | IP | not an ID — only for resolving a network observation to a node |

- **Automatic self-id (the fallback)**: when run bare without a CMDB, derived **deterministically** from the fingerprint (`machineid.SelfAssign`: cloud > machine-id > hw > fqdn in priority → `"node:"+sha256[:16]`). The same machine yields the same value, so scans do not create duplicates. It is not authoritative under §1.4 → a RegistrationRequest.
- **Validating duplicate/conflicting user input**: a user-supplied node_id can be wrong → cross-check it with the fingerprint (`ingest.CheckIdentity`). One physical machine key → several node_ids = **duplication** (one machine under several names); one node_id → several keys = **conflict** (one name across several machines, or a re-image). It does not decide, only surfaces (§2.5 — that belongs to a person or to reconciliation).

### 4A.2 The asset hierarchy Machine → Application → Process

Identity stability differs by layer:
- **Machine** = node_id (stable).
- **Application** = `(node_id, app_key)` — app_key is a key that is stable within the machine's scope (a systemd unit name, an exe path, a CMDB declaration). Because node_id is globally unique, **there is no collision with a same-named app on another machine.** A Finding is attributed to apps by `app_keys` (plural) — usually one, but in a host-wide scan a single shared library (`libcrypto.so.3`, say) that **several apps load gets multiple attributions** (`ScanHost` unions the app_keys when deduping by path). Replacing that .so affects every app that loads it, so they are not collapsed into one.
- **Process** = **a PID is volatile → not stored.** It is **resolved live right before provisioning** (`LiveProcess`) through `ProcessMatch` (systemd_unit > exe_path > cmdline_regex) — a stored PID is already stale.

### 4A.3 Access — the user's hosts file → Ansible (no secret is persisted)

- The user **manages the machine access file directly** (node_id, name, ip, port, account, **key or password**) and **specifies it on each discovery run** (`pkg/inventory.ParseHosts`; the CSV header is required, the column order is free). Authentication is **independent per host** — `ssh_key` (a private key path, recommended) or `ssh_pass` (a password; supported but not recommended, and Ansible connections need `sshpass` on the controller). For key generation and installation (`ssh-keygen`, `ssh-copy-id`) see [examples/discovery](../examples/discovery/README.md).
- pqcota reads that file and: (a) generates a **runtime-only** Ansible inventory (`RenderAnsibleInventory`, secrets included, never persisted) to run with, and (b) **ingests only the safe subset** into the inventory (`MachineEndpoint`: node_id, name, ip, port — for reuse and editing).
- **Access secrets (passwords, SSH keys, accounts) are never ingested into the pqcota inventory** — the `MachineEndpoint` type has no secret field at all, which guarantees it at compile time. The secret goes from the file to Ansible and is then discarded.

## 5. The completeness map — SD-6 (gap ≠ absence)

- The difference between a collector's `Describe` (the layers it can cover) and its `Collect` (what it actually covered) **is the gap** (`Completeness.layers_missing`).
- **Batch and intermittent nodes**: not running at collection time → **record a gap; never treat it automatically as "absent"** (§2.5). Repeat collection across time windows (§2.3) to avoid missing dlopen and batch cases.
- This distinction is what lets Inventory's UNOBSERVED verdict separate "genuinely not there" from "impossible to observe in principle" (§2.6).

## 6. Collector deployment (reaching the host) — per scenario

Architecture §2.3 principles mapped onto the scenarios. **This repo provides only the collector CLI, result signing, and the scope gate; there is no push engine of our own** (§4.4).

| Scenario | Deployment (reaching the host) | What it is |
|---|---|---|
| SD-1·SD-2·SD-3 (general) | **T1 self-service** — the user runs a signed bundle (or ships it with their own substrate) | a signed bundle + one-command execution |
| SD-4 container | sidecar injection / hostPID | namespace-aware (the collector is the same; only deployment differs) |
| SD-7 air-gapped | **a T1 offline bundle** | carry in the signed bundle and run it; results are carried out as files |

> **T2/T3 are not built**: a **generator** of playbooks and packages for the user's substrate (Ansible/Salt) (T2) and a resident agent push (T3) are fleet deployment operations tooling. This repo provides the collector so it **can be run directly** (T1 self-service, bundle signing), and generating large-scale rollouts belongs to a separate codebase.

### 6.1 Authoring collector deployment ≠ authoring remediation (the boundary principle)

Whoever authors collector deployment (reaching the host), it is different from Deploy's [script boundary](../provisioning/design.en.md) (§4.5) rule that "authoring and signing the script is the user's" — that rule exists because *app restart logic is the user's domain knowledge and liability*. **Installing a collector is putting down a read-only binary**, which needs no domain knowledge and has nothing to do with GPL contagion (a playbook is data). Still, by the **RCE symmetry** of §2.3, signature verification, least privilege, and idempotence apply from T1 onward.

**T1 guardrails (self-service)**: ① minimal caps — not root; only `CAP_NET_RAW` (network-collector) and `CAP_SYS_PTRACE` (/proc). ② bundle digest pinning + signature verification + idempotence (a transparent, forkable artifact). The bundle encodes the correct invocation (caps, co-location, version pinning, retries) exactly once. ③ The targets come from the user's scope master (§1.4). If pqcota became the thing doing the running, that would be T3 (a resident agent) — which is not built.

---

## 7. Scenario → design element traceability (the satisfaction matrix)

**Which component satisfies each scenario** — this table is the proof that the design is complete.

| Scenario | The design element that satisfies it | Evidence outcome |
|---|---|---|
| **SD-1** OpenSSL running | openssl-collector (§2.1 process + package) + the pipeline (deployment = T1 self-service or the user's substrate) | confirmed |
| **SD-2** JVM attach | jvm-collector (§2.2 attach→getProviders) + provider registry enrichment (§3③) | confirmed |
| **SD-3** binary degradation | the openssl-collector fork matcher (§2.1) + `EvidenceStrength` (§3③) | inferred-low, unknown stated |
| **SD-4** container | the same collector + namespace deployment + the completeness map | injection succeeded = confirmed / failed = a gap |
| **SD-5** out of scope | the scope gate and router (§4) | a registration decision request (PROPOSE) |
| **SD-6** batch node | the completeness map (§5) + repetition across time windows | a recorded gap (≠ absence) |
| **SD-7** air-gapped | a T1 offline bundle | T1 batch collection |

**Confirming nothing is missed**: SD-1 through SD-7 all map to at least one component, and the only new construction is §2.1–2.4 (the three runtime collectors) plus the §3 pipeline plus the §4 and §5 core.

---

## 8. Open design questions

- **JEP 451 — the dynamic agent loading constraint** (the response is decided, the §2.2 degradation path): future JDKs may block dynamic attach for a JVM started without `-XX:+EnableDynamicAgentLoading`. This repo degrades to the static path (java.security, JAR scanning) and records a completeness gap; it **does not attempt recovery detection (querying the reality through a non-agent path)**. The open question that remains: how far a non-agent path (JMX, local JVMTI) can actually query the reality of `getProviders()` (the details of a separate collector tier).
- **Multiple UIDs for the jvm-collector**: JVMs under different service accounts → attach with a seteuid switch as root, versus a collector instance per UID. (A security/isolation trade-off.)
- **Seeding the fork signature registry**: the initial range of forks and versions covered. A natural point for community contribution (OSS).
- **The time axis of the completeness map**: the repeat collection interval and the gap expiry policy (§2.3, "across time windows").
- **Automatic container detection**: how far to take automatic namespace-environment recognition → automatically choosing sidecar vs hostPID.

---

## Appendix A. Build vs reuse — what is written new and what is reused

The capability each scenario requires, judged as "build new / wrap existing / core (new)". **This table is the development priority.**

| Scenario | Capability needed | Existing tools | What to build | Novelty |
|---|---|---|---|---|
| **SD-2** JVM attach | the reality of `getProviders()` | **none** (the §2.2 gap) | ★ `jvm-collector` | **high · the killer** |
| **SD-3** binary | determining fork and version signatures | only raw strings/readelf | ★ **the fork signature matcher** (distinguishing identical sonames, acceptance principles §2.2) | **high · IP** |
| **every scenario** | evidence_strength, the completeness map, the provider registry, the Envelope, history | none | ★ **the honest evidence layer + the normalization pipeline** | **high · core** |
| **SD-5** out of scope | the gate + decision-request routing | none | the scope gate and router (core) | medium · core |
| **SD-6** batch node | gap recording + repeat collection | only cron | the completeness map (gap ≠ absence) | medium · core |
| **SD-1** OpenSSL running | assembling `/proc` and ELF | only raw utilities | `openssl-collector` (assembly) | low |
| **SD-7** air-gapped | an offline signed bundle | raw signing (cosign and the like) | the T1 bundler | medium |
| **SD-4** container | namespace-aware deployment | K8s primitives | namespace wrapping of the SD-1/SD-2 collectors | medium · integration |

**What is not built**: source and artifact scanning (territory CI can see) and eBPF dynamic-trace (invasive, excluded from Phase 0).

**The differentiators converge on three things**:
1. **SD-2** — JVM introspection (the one dedicated-OSS gap).
2. **SD-3** — binary fork intelligence (the IP for distinguishing identical sonames).
3. **The honest evidence layer** — evidence_strength, the completeness map (gap ≠ absence), the provider registry. *Present in no existing tool, and the basis of §1.2 audit integrity.*

The rest (SD-1 assembly) is low difficulty. **The differentiation is not in the number of collectors but in runtime reality plus honesty** (Appendix A: observability is being commoditized).
