English · [한국어](design.md)

# Inventory subsystem design

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](design.md) is authoritative.

The technical design for regulation §3 (Inventory). It presents Discovery's output (the canonical CBOM and completeness map) as **read-only inventory views** and manages machine metadata — the part that ties observations to nodes and apps and shows history and change.

**Basis**: regulation §3 and §5 · [discovery design](../discovery/design.en.md) · [architecture design](../docs/architecture.en.md) · [contracts](../contracts/).


> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

**Which rules this implements** — when the regulation changes, this table finds the sections to fix.

| This document | The regulation |
|---|---|
| 1. Component architecture | §3.1 purpose and boundary |
| 2. Data model | §3.2 the canonical format · §1.5 machine identity and the asset hierarchy |
| 3. State vocabulary | §3.3 reconcile → review → finalize |
| 4. Outputs and the Deploy handoff | §3.7 outputs (the Deploy gate) |
| 5. Automation rules → component mapping | §3.5 automation rules |
| 7. History and retention | §1.2 immutable originals + derived views |
| 8. Asset scope | §1.4 the scope master |

Section 6, the topology graph, has no corresponding rule — this document designed it first, in Phase 1.

---

## 0. What it does

What the inventory does is **ingest, persist, and serve**. **Browsing history and diffing change between snapshots** belong here too — that is listing observed facts, not judging. **Asset scope** (declaring and enforcing what stays managed) and the **retention policy** (truncating old points of change) are here as well — keeping one's own inventory in order, without which "it works end to end on its own" breaks.

The `Decision` and `FinalizedPlan` schemas live in the public contracts, [`contracts/`](../contracts).

---

## 1. Component architecture

```
  Discovery output ───────────────────────────────────────┐
   ├ observed lane (collector, CBOM import) : canonical CBOM + evidence │
   └ declared lane (declaration-importer)   : the user's declaration    │
                        │                                    │
        ┌───────────────▼─────────────────────────────────┐ │
        │ ingest (pqcota-ingest)                           │ │  §3.1
        │   scope gate → normalize → resolve identity      │ │
        └───────────────┬─────────────────────────────────┘ │
                        │                                    │
        ┌───────────────▼─────────────────────────────────┐ │
        │ append-only history + machine metadata           │ │  §6
        └───────────────┬─────────────────────────────────┘ │
                        │                                    │
        read-only views · history · snapshot diff · retention │
                        │ (declaration updates, recollection requests)
                        └──────────────────▶ Discovery ◀────┘
```

**The two lanes meet here**: the **declared vs observed** split established in Discovery (distinguished by the Envelope's `detection_method`, not by how it was delivered) accumulates in the same history with its label intact.

---

## 2. Data model

On top of Discovery's `Finding` (a crypto asset inside a node), Inventory treats **communication edges** and **verdicts and plans** as first-class.

### 2.0 Machine metadata — separate from identity (contract: `inventory/v1 machine.proto`)

The machine information the inventory manages comes in two kinds, both **separate from technical identity** (node_id, fingerprints) — identity is machine-observed (discovery §1.5), while these two are **filled by declaration (CMDB) or a reviewer and edited by the user**:

- **`MachineEndpoint`** (node_id, name, ip, port) — **reusable connection metadata** for reconnecting during discovery. Only the safe subset from the user's hosts file is ingested (§1.5). **Access secrets are not ingested** (the type has no secret field).
- **`MachineProfile`** (display_name, environment, role, owner, location, a `labels` map, source) — visual metadata **people read to tell machines apart**. Used for UI grouping, filtering, and color. `labels` extends it along arbitrary axes (team, regulation, tier). The source (cmdb/reviewer/observed) is provenance.

→ A technical ID like node_id means nothing to a person, so display name, environment, and the rest live in their own lane. Reusable and editable; secrets are never stored.

**Implementation — the runtime services**:
- **`inventory.MetaStore`** (Mem/Pg) — **upserts** endpoints and profiles keyed by node_id (in contrast to the append-only history — these two are mutable metadata the user reuses and edits). The Pg schema has no access-secret column.
- **The ingest path** — `pqcota-hosts` (discovery/cmd) turns the user's hosts file into (a) a runtime-only Ansible inventory (secrets included, never persisted) and (b) a safe endpoint upsert (`--dsn`). Secrets go only into (a).
- **View composition** — `RenderStore(store, meta)` puts a per-node header `▸ name (ip:port) │ display_name · env · role · owner` above the derived Findings (with `meta` nil the header is omitted, harmlessly). `pqcota-inventory` queries the same DSN.

**App attribution (the §1.5 asset model)** — each derived `Finding` is attributed to applications through `app_keys` (plural) and shown as `@app` in the view. In a host-wide scan, a single shared library (`libcrypto.so.3`, say) loaded by several apps gets **multiple attributions** — so the blast radius of replacing that .so is shown in full. It does not judge (§2.1).

```go
// A communication edge — the unit of reconciliation (§3.3). A TLS/SSH connection between nodes.
type CommunicationEdge struct {
    ID        string          // canonical hash (src, dst, port, proto)
    Src, Dst  string          // scope master node IDs (the anchor)
    Declared  *DeclaredEdge   // the declared lane (may be nil)
    Observed  *ObservedEdge   // the observed lane (network collector, CBOM; may be nil)
    State     ReconState      // CONFIRMED | UNDECLARED | UNOBSERVED
    Confidence float64        // the §3.5 score
}
type ReconState string // "confirmed" | "undeclared" | "unobserved"

// A verdict — "a human conclusion" (§3.6). Being a conclusion rather than an edge state, it stays attached across re-collection.
type Decision struct {
    Subject     string     // an edge ID or a policy template ID (policy-level, §3.4)
    Conclusion  string     // real / stale / excluded / approved, and so on
    Reviewer    string
    Signature   string     // the approval signature (required to finalize, §3.3③)
    BasisHash   string     // the hash of the evidence behind the verdict → the invalidation trigger (§3.6)
    Status      string     // draft | in-review | finalized
    DecidedAt   string
}

// The finalized plan — the only grounds on which provisioning runs (§3.7, §5). The schema is the contracts SSOT (public).
type FinalizedPlan struct {
    Scope           string                 // a ring or domain (partial finalization allowed, §3.3③)
    Items           []PlanItem
    ApprovalSigs    []string               // every mandatory item decided + signatures
    DerivedFrom     string                 // from which reconciliation snapshot (§1.2)
}
type PlanItem struct {
    NodeID          string
    RemediationClass string                // the taxonomy branch (provisioning design §4.1·§4.2)
    DeployAutomationLevel DeployAutomationLevel // L1/L2/L3 — a per-asset reviewer verdict (§4.3)
    ProviderChoice  string                 // the FIPS routing result: BC-FJA / in-house / …
}
```
> `DeployAutomationLevel` and `RemediationClass` are already controlled vocabulary in the contracts. **They are filled in here** (not in Discovery; a MANUAL reviewer act). The `Decision` and `FinalizedPlan` schemas live in the contracts (public schemas).

---

## 3. State vocabulary (what the views render with)

- **The state vocabulary** (the schema meaning the views need): `CONFIRMED` = declared ∩ observed, `UNDECLARED` = observed only (**shadow communication**), `UNOBSERVED` = declared only. Whether an `UNOBSERVED` is "genuinely not there" or "a gap in the completeness map (impossible to see in principle)" is settled by Discovery's **completeness map** (§2.6, gap ≠ absence) — if it is a gap, re-collect; otherwise a person decides.

---

## 4. Outputs and the Deploy handoff (§3.7, §5)

> For the handoff target (the Deploy subsystem) — the plan schema, the gate, and the artifact generator design — see the [provisioning design](../provisioning/design.en.md). The formal definitions of the plan and verdict schemas are `contracts/plan.proto` and `decision.proto` ([data model schema](../contracts/data-model.en.md)).

| Output | Nature | Handoff |
|---|---|---|
| The reconciliation view | derived (per-edge state + confidence + provenance), a read-only view | — |
| **The finalized plan** | the **only grounds** on which provisioning runs | **Inventory→Deploy (§5): anything not finalized is refused execution** |
| Decision and plan history | the judgment and intent lanes of provenance | §1.3 |

## 5. Automation rules (§3.5) → component mapping

| Action | Grade | Component |
|---|---|---|
| version normalization, Envelope, evidence attachment | AUTO | (reuses the Discovery pipeline) |

---

## 6. The crypto communication topology graph (Phase 1) — a self-completing map

After building the scope master from the legacy IP list the user supplied, the [network-collector](../discovery/design.en.md) observations from each machine are aggregated to **generate a crypto communication map automatically**. It is the graph rendering of the reconciliation view (§3.7).

### 6.1 Composition

- **Nodes** = machines registered in the scope master (the user's IPs). An observed peer that is not registered is marked separately as "a registration decision request" (§1.4/§5).
- **Edges** = observed communication (the §2 CommunicationEdge). src→dst, role (client/server), protocol (TLS/SSH), and the **negotiated KEX group and cipher**.
- **Edge color = quantum posture** (the heart of the migration dashboard):

| Color | Meaning | Examples |
|---|---|---|
| 🟢 | a hybrid/PQC negotiation | `X25519MLKEM768`, `sntrup761x25519` |
| 🔴 | classical = **quantum-vulnerable** | `X25519`, `ECDHE`, `RSA` |
| ⚪ | **unknown / unobserved** | an encrypted handshake or QUIC / outside the capture window |

### 6.2 The honesty rules (what makes this graph trustworthy)

**"Observed" and "impossible to see in principle" are kept apart** (§2.6, §3.3):
- **An unobserved edge is never drawn as "no connection"** — it is marked "unobserved" with a dashed line or ⚪, tied to the completeness map.
- **Coverage is shown**: a node without a collector is greyed out (only half of that node's edges are visible).
- **A capability vs actual overlay**: a node's "a PQC-capable library is loaded" (a discovery Finding) is overlaid on an edge's "actually negotiated classical" (observed) → which surfaces **the precise next action**, such as *"PQC is available but it is falling back to classical"*.

### 6.3 Outputs

- **The edge graph** (a derived view): the graph representation of the reconciliation view. **DOT/Graphviz** generated automatically → SVG, or a web view (D3).
- The three states CONFIRMED/UNDECLARED/UNOBSERVED (§3.3) map onto graph color and line style: UNDECLARED = a **shadow connection** (a thick warning line), UNOBSERVED = dashed.

### 6.4 Limits (inherited from the network-collector, discovery design §2.3)

The passive observation window, coverage dependence, and encryption limits → **the map is "what was seen", not "everything that exists"**. Marking that partiality honestly as a gap is consistent with §1.2 audit integrity. "As far as is possible" is the accurate framing.

---

## 7. History and retention (2026-07-21)

### 7.1 There are three reasons to keep history, and their demands differ

| Purpose | What it needs | The value of repeated measurement |
|---|---|---|
| **Tracking change** | before and after at the point of change | none — repeating the same value is noise |
| **Proving observation** (showing an auditor "we scanned daily") | when, and how many times, it was seen | **large** — this is the evidence |
| **Reproducing a recomputation** (§1.2) | the original + the ruleset at that time | only at ruleset boundaries |

One table carrying all three grows without bound. Once a day × 1,000 nodes × 3 years = a million snapshots, of which only a few per node per year are meaningful changes. **So the solution is not "delete" but "separate".**

### 7.2 A two-layer split — the heavy and the light

- **Snapshots** (heavy, `pqcota_snapshots`) — accumulate **only when the substantive content changes**. The basis for tracking change and reproducing recomputation.
- **Observation records** (light, `pqcota_observations`) — one row per ingest (`node_id, snapshot_id, ruleset, observed_at`). The basis for proving observation.

Heavy storage grows **only with the number of changes**, while the fact "we looked every time" survives intact. It does not break §1.2 either — the original is not modified; it simply **is not stored twice**.

The history view (`-history`) now shows "the point of change + how many times and until when that state was reconfirmed (obs, observed)".

### 7.3 Defining equality — the weak point of this design (`pkg/discovery/history/fingerprint.go`)

Everything hinges on what decides "is this the same state". **Include a volatile field and it becomes "changed" every time, which defeats the split.**

| Excluded (volatile) | Why |
|---|---|
| `ObservedEdge.observed_count`, `first_seen`, `last_seen` | they differ on every observation. More frequent traffic does not mean the crypto configuration changed |
| `Finding.derived_from_snapshot_id`, `ruleset_version` | they differ per snapshot (fields for tracking derivation) |

| Included (substantive) | Why |
|---|---|
| finding id, runtime, usage, detection_method, evidence_strength, algorithm, pqc_readiness, fips, remediation_class, **app_keys** | the values that change a management decision |
| the OpenSSL axis: lib, fork, version, binding_mode | a version replacement *is* the change |
| the JCA axis: jdk, **provider_set (order preserved)** | order determines priority negotiation (acceptance principles §2.2 (d)) — **it must not be sorted** |
| Completeness: layers_missing, note | a different gap means a different interpretation |

> ⚠️ If a field left out here changes, it folds into "no change" and **disappears from the history**. When you add a field to the contract, this function must be updated with it.

`ExcludedByScope` (§8) is **left out** of the fingerprint — assets outside management coming and going is not a change to the managed inventory.

### 7.4 The truncation policy (`pqcota-prune`)

Because §7.2 already folds repetitions of the same state, truncation deals only with **old points of change**. Every stored snapshot is a point of change, so an axis like "preserve change points" is unnecessary.

**Three invariants:**

1. **No modification** — the snapshots that remain are byte-identical. Truncation does not *change* the past; it *ends* retention, which is compatible with append-only (§1.2).
2. **The newest is inviolable** — the newest snapshot per node is deleted by no policy. It is the basis for the inventory view and for the provisioning before capture (§8).
3. **The deletion itself is recorded** — written to `pqcota_retention_events`, and `-history` reports it with a `⌫` line. Without it, a hole in the history **cannot be told apart from "we did not observe"** — this is §2.6's "gap ≠ absence" moved onto the time axis.

**Policy decision**: `Policy{OlderThan, KeepLast}`. Given both axes it is **conservative** — it deletes only when both say "this may go" (within the last N, or not yet old enough → keep). Being destructive, when in doubt it keeps. Given neither axis it would mean "delete everything but the newest", so it is **refused** with `ErrNoPolicy`.

**Why a separate command**: `Pruner` is split out of the query-side `Store` interface and `pqcota-prune` stands on its own. If a read tool also did destructive work, one mistake would erase the history. The default is a dry run, and actual deletion happens only with `-apply`.

---

## 8. Asset scope (2026-07-21)

### 8.1 The node gate (§1.4), at asset granularity

Registering a node does not make **everything observed inside it** managed. Once system default libraries, runtimes a package manager dragged in, and one-off processes are mixed in, the inventory drowns in noise and becomes useless. What stays in view is **declared by the user**, and the tool enforces that declaration (§1.1 — the maestro judges, the player executes).

Implementation: `AssetPolicy` in `pkg/kernel/scope/asset.go`. It applies **right after normalization and right before ingest** (`normalize.Normalize`) — observe everything, persist only what is managed.

### 8.2 Order of decision

**Include** by default → apply the rules **in order**, and **the later rule wins** (the last matching rule decides).

- **Put include *after* exclude and use it as an exception.** "Exclude this whole family except this one" is expressed that way — it is **order-based, not unconditionally prioritized** (put include first and the later exclude wins).
- With no rules, everything is managed — a user who does not use scope is not obstructed.
- Rule axes: `runtime`, `lib` (a soname for openssl, a provider for jca), `app_key`. Blank or `*` means all; patterns are globs.
- **A shared `.so` is attributed to several apps** (§1.5), so a rule fires if **any one of them** matches.

> **★ The blast radius of excluding a shared `.so` (a footgun).** Because a shared library is attributed to several apps, excluding it while **aiming at just one app (a test app `internal-test-*`, say)** also removes the asset **for the production app (`payment-gw`) that shares that .so** — a single match catches the whole finding. To protect the production app, put an **include that restores it after the exclude** (later wins). The excluded count is reported per §8.3, so it does not hide as "0 items", but whether **which shared asset dropped out because of which app** is the intended result is for the rule author to check (test TV-SCOPE-7).

### 8.3 ★ Excluded ≠ absent

Letting a policy-excluded asset quietly disappear makes the inventory **lie that "there is no such thing"** — exactly what §2.6 forbids. So the excluded items are counted into `Snapshot.ExcludedByScope`, and **both the ingest summary and the inventory view report the count**.

For the same reason, applying scope shows up in a history diff as "gone", which means **the asset did not vanish; it was taken out of management** (the demo states that distinction in a caption).

### 8.4 Scope boundary

**Defining and enforcing the rules belongs to this repo** — it is choosing one's own assets alone, and without it the noise makes the inventory itself unusable (per architecture §6).

---

## The declaration importer (SV-1)

**Responsibility**: import the user's **declarations** (CMDB, an existing inventory) → normalize → label them as the **declared lane**. It is not observation (§3.3, declared ≠ observed).

> **Where the code lives**: `pkg/inventory/declaration`. The output is data that accumulates in the inventory, so the code lives in the inventory too.

### The scenario

- **The situation**: the user holds an existing crypto inventory or CMDB **declaration**. (Note: not a CBOM file — that is [delegated intake](cbom-intake.md), Korean.)
- **[The user]** supplies the existing declaration data.
- **[pqcota]** imports the declaration → normalizes it into the canonical format. Labels it as the **declared lane** (it **does not reconcile** it against observations). Note: the implementation moved to the inventory (`pkg/inventory/declaration`) — a declaration is the baseline for reconciliation, so it belongs to the inventory stage.
- **The result**: the declared lane is in place (one axis of later reconciliation).
