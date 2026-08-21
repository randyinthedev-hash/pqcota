English · [한국어](regulation.md)

# PQC migration management platform — the regulation

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](regulation.md) is authoritative.

**What this document is**: the **reference document** for implementation, operation, and audit. What is settled here is what the designs, contracts, and code follow.

> **§ notation**: section numbers below refer to this document.

---

## 1. Cross-cutting principles — shared by Discovery, Inventory, and Deploy

The principles governing this entire regulation. The per-stage rules (§2–§4) are all derived from them.

### 1.1 The automation boundary principle — "automate what is known, gate what is judgment"

At every stage, actions are classified into three grades. This three-way split is the spine of each stage's automation rules.

| Grade | Definition | Handling |
|---|---|---|
| **AUTO** | a machine can decide it deterministically and correctly | fully automatic execution |
| **PROPOSE** | a machine can offer candidates or a recommendation, but the decision belongs to a person | automatic proposal + a human approval gate |
| **MANUAL** | a machine cannot decide it in principle | a human verdict is mandatory; the machine only organizes information |

The moment something ambiguous is placed in AUTO, the platform's safety collapses. When the boundary is uncertain, always drop it one grade.

### 1.2 The immutable original + derived view principle

**The originals of collection, verdicts, and plans are never modified in place.** Normalization results, the reconciliation graph, and the finalized plan are all **derived views** computed on top of the originals. When a rule (enrichment logic, a mapping table) improves, recompute from the original. Audit integrity is the foundation of trust in the product.

### 1.3 The provenance chain principle

Every execution (every act) must be traceable back through a causal chain spanning four lanes of history.

```
state (a discovery snapshot) → judgment (a human verdict) → intent (a finalized plan) → act (a provisioning run)
```

An execution that this chain does not complete is a violation of the regulation.

### 1.4 The scope master principle

**The asset management record (the user's CMDB or asset registry) is the sole authority on the boundary of what is managed.** Discovery deployment, inventory registration, and provisioning execution targets are all gated by that master. A node not in the master is, as a rule, not a collection or execution target; when observation finds one, it flows to "a request for a registration decision", not to "an execution target".

### 1.5 Machine identity, the asset hierarchy, and the access boundary

Identity rules that cross stages. What we call what, and what we do not store.

**Identity, correlation, and location are never mixed.**

| Layer | What | Rule |
|---|---|---|
| **Authoritative ID** | `node_id` | set by the scope master (§1.4). Without one, **deterministically** derived from the fingerprint |
| **Correlation fingerprint** | machine-id, hardware UUID, cloud instance id, fqdn | for identity resolution. Not an ID |
| **Locator** | IP | not an ID — used only to tie a network observation to a node |

**The asset hierarchy is Machine → Application → Process, and identity stability differs by layer.**

- **Machine** = `node_id` (stable)
- **Application** = `(node_id, app_key)` — a key that is stable within the machine's scope
- **Process** = **not stored.** A PID is volatile and would break the history — it is found again by a matching rule at remediation time

**Access secrets are never ingested into the inventory.** Machine access information (accounts, keys, passwords) lives in a file the user manages directly; the tool reads it to build a **runtime-only** deployment inventory. Only the connection point (`node_id`, name, ip, port) remains in the central inventory, and a secret field **does not exist in the type** — there must be no place to put one by accident.

---

### 1.6 We do not know what is behind the boundary

The core does not know the implementation beyond a boundary. It depends only on the collector abstraction, **"give it a node, get back a canonical CBOM"**. Whether CBOMkit, CipherIQ, or an in-house collector sits behind it, the core **must not know.** The user picks the collector backend through deployment and configuration, and because every backend emits the same canonical CBOM, everything downstream (reconciliation, review, provisioning) behaves identically regardless of backend.

The same rule applies to other boundaries — provisioning does not know the deployment substrate (it emits only standard playbooks), and the inventory does not know the datastore. What is behind a boundary can change without shaking anything downstream.

---

## 2. DISCOVERY stage rules

### 2.1 Purpose and boundary

Collect crypto asset information from managed nodes and aggregate it centrally in a canonical form. **It does not judge** — Discovery is fact collection; interpretation, classification, and decision belong to later stages.

### 2.2 Collection layers — pluggable

Any collector attaches as long as it satisfies the intake contract (§1.6). There are four layers, and they are **the unit of the completeness map** (§2.6):

| Layer | What it sees |
|---|---|
| **Source / artifact** | source, libraries, certificates, keys |
| **Runtime / process** | what is actually loaded and called (dlopen included) |
| **Network** | the groups negotiated by TLS/SSH/QUIC |
| **Runtime introspection** | the reality obtained by asking a running process |

**No single collector is complete** — a layer that was not observed is left as a gap, not as "absent" (§2.6).

### 2.3 Absence of source is the dominant case

In a legacy estate, absent source is **the default** (vendor binaries, lost source, shading/static linking, COTS). So the mainstay of discovery is not source scanning but **artifact and runtime first**.

**The key rule — absence of source is not absence of information but degradation of information.** Without source, "what is linked or loaded" is still caught, while "how it is used" (algorithm, usage_context) degrades. That degradation must always be stated through `evidence_strength`.

| detection_method | evidence_strength | Fields it fills |
|---|---|---|
| source (Hyperion and the like) | confirmed | algorithm and usage_context, complete |
| artifact (Theia, JAR scanning) | inferred-high | libraries and dependencies; usage partial |
| symbol-analysis (a static binary) | **inferred-low** | fork and version inferred; no usage |
| runtime-introspection (`/proc`, `getProviders()`) | confirmed | what is actually loaded, the provider chain |
| dynamic-trace (eBPF/ltrace) | confirmed | the algorithms actually called (invasive, PROPOSE+) |

### 2.4 The normalization pipeline (a contract fixed at every stage boundary)

1. **Raw capture** — the collector's native output is preserved immutably (the source for renormalization)
2. **Parse/convert** — native → canonical CycloneDX CBOM
3. **Enrich** — map version → pqc_readiness, determine fork and `{jdk×provider}`, tag the server/client role, attach evidence_strength
4. **Validate** — schema conformance plus plausibility (flag contradictions)
5. **Identity resolution + dedup** — node identity (anchored on the master ID), finding identity (a canonical hash)
6. **Persist** — append a state snapshot to the discovery history

### 2.5 Automation rules

| Action | Grade | Rule |
|---|---|---|
| deploying and running collectors | AUTO | only on nodes past the scope master gate |
| raw capture, parsing, normalization | AUTO | deterministic |
| determining fork / `{jdk×provider}` | AUTO | state `unknown` explicitly when it cannot be determined |
| attaching evidence_strength | AUTO | deterministic, based on detection_method |
| JVM introspection (of a running process) | AUTO | a process that is not running is recorded as a gap in the completeness map |
| dynamic-trace (eBPF/ltrace) | PROPOSE | invasive; applied selectively to high-value assets |
| discovering communication outside scope | PROPOSE | routed as "a request for a registration/exclusion decision" |
| nodes that did not report, or where collection failed | MANUAL | stated in the completeness map; never automatically treated as "absent" |

**Rule**: a field that could not be filled must state `unknown` explicitly ("unknown is first-class evidence too").

### 2.6 Outputs / integrity

- **A canonical CBOM + Envelope**: the body (the fields in acceptance principles §2.4) plus an Envelope (collector id and version, collection method, time, target node, **the collector's signature**)
- **A completeness map**: coverage against scope — **recorded separately per collector and per layer** ("scanned by Theia, process layer not collected"). Only then does Inventory's UNOBSERVED verdict avoid confusing "genuinely not there" with "impossible to observe in principle".
- Collection channel: mTLS authentication plus signed reports. Avoid self-reference (the management-plane crypto is separated from and stated apart from the data plane)

---

## 3. INVENTORY stage rules

### 3.1 Purpose and boundary

Refine the collected facts into a **state you can decide on**. Reconcile the two evidence sources (declared and observed) and, through human review, produce the reconciliation view that will be the basis of a finalized plan. **Finalizing the inventory is not a mechanical merge; it happens through review, planning, and finalization.**

### 3.2 Canonical format rules

**"The CBOMkit format" and "the CipherIQ format" are misnomers.** Both emit the **CycloneDX CBOM (ECMA-424) standard**. Three layers are distinguished, though:

- **Schema (standard, identical)**: interoperability is guaranteed. Parsers and viewers can be shared
- **Spec version (varies)**: 1.6 vs 1.7 divergence → **the normalization pipeline converges on an internal canonical version**
- **Coverage and enrichment (tool-specific, different)**: even for the same field, which tool fills what differs. CipherIQ's dependency DAG, NIST SP 800-57 lifecycle state, and so on ride in the standard `properties` extension

**The canonical format = CycloneDX CBOM (the standard body) + Envelope (provenance) + evidence metadata (the extension).** A collection original is never taken as the canonical form as-is. It must always pass through (1) version normalization, (2) Envelope attachment, (3) evidence_strength tagging, and (4) mapping and preserving tool-specific `properties` (DAG, lifecycle, and so on) into canonical fields.

### 3.3 Reconcile → review → finalize

**① The reconciliation engine (three-state reconciliation)** — each communication edge is classified:
- **CONFIRMED** (declared ∩ observed): the highest confidence
- **UNDECLARED** (observed only): shadow communication, the top security finding
- **UNOBSERVED** (declared only): real (DR/batch) vs stale vs a coverage gap — **a machine cannot decide**

**② Building the review queue** — the reconciliation engine does not produce answers; it structures and prioritizes what needs a verdict:
- Auto-pass candidates: CONFIRMED + high confidence + low risk → a bundle for batch approval
- Mandatory individual review: UNDECLARED, low-confidence UNOBSERVED, anything requiring a legacy touch, and edges whose cutover would break the peer
- Priority: risk × blast radius × data sensitivity

**③ Review and finalization** — `draft → in-review → finalized`. Provisioning cannot run before finalized (every mandatory item decided plus approval signatures). Partial finalization per ring or domain is allowed.

### 3.4 Review granularity (hybrid)

- **Policy-level by default**: the reviewer decides on remediation rules. **The version × link-mode (and JDK × provider) solution catalog is itself the policy template under review**
- **Individually isolated exceptions**: only policy exceptions, high-risk items, and `UNDECLARED` edges are handled per edge

### 3.5 Automation rules

| Action | Grade | Rule |
|---|---|---|
| version normalization, Envelope, evidence attachment | AUTO | a derived view |
| three-state reconciliation and edge classification | AUTO | CONFIRMED/UNDECLARED are deterministic |
| confidence scoring | AUTO | f(observation frequency, duration, declaration freshness, source agreement) |
| selecting auto-pass candidates | PROPOSE | batch approval is proposed; approval is human |
| handling UNOBSERVED | MANUAL | deciding real / stale / gap requires a person |
| finalizing a plan | MANUAL | approval signatures are mandatory |

### 3.6 Persisting verdicts (review reuse)

- A verdict is "a human conclusion", not an edge state → it stays attached across re-collection
- Invalidation trigger: when the underlying evidence changes substantively, only that verdict is flagged for re-review → **a delta review**
- Expiry of stale verdicts: confidence decays; periodic reconfirmation

### 3.7 Outputs

- **The reconciliation view** (derived): per-edge state + confidence + provenance
- **The finalized plan**: the only grounds on which provisioning runs
- **Decision and plan history**: the judgment and intent lanes of the provenance chain

---

## 4. DEPLOY (provisioning) stage rules

### 4.1 Purpose and boundary

Execute remediation according to the finalized plan. **Separation of responsibility**: the reviewer owns "what, and in what order" (the plan layer); the platform owns "how, safely" (the execution layer). They are never mixed.

### 4.2 The core strategy — keep the version, inject an internal provider (both runtimes)

Without upgrading the version, **inject an internally developed PQC provider** and replace only the algorithm capability. In most cases the remediation shrinks from "replace the entire library" to "stage the provider + activate + verify" (atomic and reversible, easing the risk of locking yourself out).

### 4.3 The staged deployment model — delegation levels

A remediation is not finished in one shot; **each asset chooses how far to automate**, by risk. Every stage boundary is both a gate and a rollback point.

**Staged delegation levels — chosen per asset (`deploy_automation_level`, a first-class attribute)**

| Level | Action | Risk | Reversibility | Service impact | For |
|---|---|---|---|---|---|
| **L1 Stage-only** | download the PQC module into the agreed folder, nothing more | lowest | fully reversible (delete) | none | the most conservative, highest-risk assets |
| **L2 Stage+Install** | through module installation; activation and restart are the user's manual step | low | reversible (the old state is preserved) | none | **the realistic production default** |
| **L3 Full auto** | through activation and restart, via the agreed script | high | needs atomic rollback | yes | stateless workers, emergency response |

The level is a **per-asset attribute**, not a company-wide setting ("payment server = L2, stateless worker = L3"). The reviewer decides it per asset during review and finalization.

**A stage boundary is a gate and a rollback point**
- After L1: verify download integrity (signature, hash). On failure, deleting restores it completely, with no service impact.
- After L2: verify installation (location, form). Rollback = uninstall.
- During L3: graceful drain → restart → **boot verification** (comes up cleanly + the new provider loads + it enters the dispatch chain + TLS connects backward-compatibly). On failure, **atomic rollback** (restore the old provider/config and restart).
- After L3: confirm the state change with a rescan (the Deploy→Discovery closed loop).

**Non-negotiable contracts (because running the agreed script is remote code execution)**
1. **Pin artifact integrity**: the module to be staged is pinned by sha256 — not "run it if it is in the folder", but "run it only if it is the file we expected". Signature (ed25519) verification is on the [roadmap](../RELEASE_NOTES.en.md)
2. **Folder access control**: write permission on the agreed folder is restricted to the deployment channel. Local users and other processes cannot write.
3. **Idempotence at every stage**: resending a command or retrying a partial failure is safe (for batch and intermittent nodes).
4. **Least-privilege execution**: the script's execution scope is whitelisted, sandboxed, and audited.
5. **Avoid locking yourself out (L3)**: processes tied to the management channel (SSH, agent TLS) are excluded from L3 automatic restart, or ordered last. The deployment channel's own communication must never depend on the module being deployed (no restarting yourself).

**Substrate separation**: L1 and L2 (download and install) are done by standard config-management deployment (Ansible and the like). **L3 (activation and restart) is a separate step on top that touches the activation point.** Orchestration intelligence (drain, rolling, gates, rollback) is the **central planning engine's** responsibility, not the node's.

### 4.4 Execution substrate rules

**We do not build our own remote execution engine.** Proven config management (Ansible/Salt/package repos) is used as the orchestration substrate, and the added value sits above it, in crypto intelligence and safety rails.

### 4.5 Safety rails

**Three things the tool enforces** — everything else is set by the plan.

- **Only a finalized plan is grounds for execution**: without FINALIZED plus approval signatures, nothing is generated (§3.7)
- **Reversibility**: every output comes with a symmetric rollback. Nothing overwrites the original, so removal is restoration
- **Stage boundaries**: L1 only stages, L2 installs, and only L3 activates and restarts. It never goes further than what was approved

**What the plan sets** — ring rollout, canaries, health gates, and rollback judgment are questions of remediation order and target selection; they differ per environment, so the tool does not guess. What is issued, and in what order, comes from what is written in the plan and the `activation` hook (§2.5 · [roadmap, "what we do not build"](../RELEASE_NOTES.en.md)).

- **Backward compatibility first**: keep the hybrid (X25519MLKEM768) as the default and leave the classical fallback in place. A pure-PQC cutover comes only after confirming the peer is ready — the output must never sever a connection
- **Avoid locking yourself out**: on a host where the management channel (SSH/TLS) is involved, put the restart **last** in the order

### 4.6 Role-based ordering

Because TLS is asymmetric, **widen client-side support first and switch the server side later** (a client switched early is harmless because the fallback exists; a server switched early risks locking out unprepared clients). A mesh (server and client at once) manages crypto assets separately per role and breaks the cycle by preferring the hybrid.

### 4.7 Automation rules

| Action | Grade | Rule |
|---|---|---|
| turning a plan into execution commands | AUTO | only for a finalized plan |
| dry-run / plan mode | AUTO | present the changes before execution |
| ring rollout, health gates, rollback | AUTO | enforced by the platform, not bypassable |
| provider injection, config change | AUTO (within the gate) | conditional on ring progress and passing the gate |
| a pure-PQC cutover | MANUAL | forbidden while peer readiness is unconfirmed |
| a legacy touch (fork replacement, rebuild, JDK upgrade) | PROPOSE→MANUAL | high risk, approved individually |
| L1 download / L2 install | AUTO (within the gate) | no service impact, after integrity verification |
| L3 activation + restart | AUTO (within the gate) | graceful drain + boot verification + automatic rollback enforced. Management-channel processes excluded or ordered last |
| deciding deploy_automation_level | MANUAL | a per-asset reviewer verdict |

## 5. The handoff contract between stages

```
Discovery ──[canonical CBOM + Envelope + completeness map]──▶ Inventory
Inventory ──[a finalized plan]─────────────────────────────▶ Deploy
Deploy    ──[state change]─────────────────────────────────▶ Discovery (the rescan closed loop)
```

- **Discovery→Inventory**: an unverified or schema-nonconforming CBOM is refused registration. `unknown` and `evidence_strength` must be explicit
- **Inventory→Deploy**: **a plan that is not finalized is refused execution** (the strongest gate)
- **Deploy→Discovery**: after execution, a rescan confirms the state change; on drift, a new review item is created

---
