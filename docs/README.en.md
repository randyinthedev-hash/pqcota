English · [한국어](README.md)

# docs/ — design and contributor (developer) documents

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](README.md) is authoritative.

> **This folder is for developers, forks, and contributors.** If you want to *try* the platform, start from the root [README's "Try it"](../README.en.md#try-it--demo) and [demo/](../demo/).

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](regulation.en.md).

The documents that fix the platform's "what, under which rules (WHAT)" and "through which modules and contracts (HOW)". The `§` references in the code point here.

## Reading order

```
the regulation ──┬── architecture      (modules, contracts, and schemas crossing stages)
                 └── three per-stage designs (that stage's components, data, and flow)
```

**Read the regulation first.** The rules stand first, and a design answers what implements them — a design cites the regulation; the regulation knows nothing of designs. The architecture and the per-stage designs sit **side by side** (both cite the regulation directly).

| Order | Document | The question it answers |
|---|---|---|
| **1** | [The regulation](regulation.en.md) | what must and must not be done |
| **2** | [Architecture](architecture.en.md) | through which modules, contracts, and schemas |
| **2** | The three [per-stage designs](#subsystem-designs-three-stages) | how that stage does it |

On the side: [crypto runtime acceptance principles](runtime-acceptance.en.md) (what is accepted as a runtime) ·
[license notes](licensing.en.md) (what is used, and what enforces isolation) ·
[designs under review](under-review.en.md) (what is not decided yet).

> **At a glance**: [platform structure and stakeholders](https://randyinthedev-hash.github.io/pqcota/architectures/platform-structure.html) — who supplies what, what the platform produces, and where execution happens.

## Rules and boundaries (read first)
| Document | Contents |
|---|---|
| [The regulation](regulation.en.md) | **what must and must not be done** — cross-cutting principles · per-stage rules · the automation boundary (AUTO/PROPOSE/MANUAL) · the handoff contract. The original that the code's § references point to |
| [Architecture](architecture.en.md) | **through which modules, contracts, and schemas** the regulation is implemented — the tech stack · system composition · the canonical CBOM Envelope · the intake contract · repo structure |
| [License notes](licensing.en.md) | what is used under which license (by consumption form) + **what enforces copyleft isolation** |

## Subsystem designs (three stages)

> The **conceptual overview** of each stage starts in its folder README — [discovery/](../discovery/README.en.md) · [inventory/](../inventory/README.en.md) · [provisioning/](../provisioning/README.en.md). Below are the detailed design documents.

| Document | Stage |
|---|---|
| [Discovery design](../discovery/design.en.md) · [collector deployment](../discovery/collector-deployment.md) (Korean) · [delegated intake](../inventory/cbom-intake.md) (Korean) | Discovery — collectors (direct observation), delegated CBOM intake, the pipeline, SD-1–SV-2, the asset model |
| [Inventory design](../inventory/design.en.md) | Inventory — the machine metadata store (endpoints, profiles) and the central view (app attribution) |
| [Provisioning design](../provisioning/design.en.md) | Provisioning — the plan gate, taxonomy→artifacts, L1/L2/L3 playbook generation (L3 activates and restarts through the plan's `activation` hook), before/rollback records |

## Tests and verification
| Document | Contents |
|---|---|
| [**The test specification map**](test-map.md) (Korean) | where each case is written and where it runs — group→code mapping, level distribution, the unverified set. **Start here** |
| [Kernel test cases](kernel-testcases.md) (Korean) | the **derivation rules** crossing stages — evidence strength, normalization, posture, the remediation taxonomy, app attribution |
| [Discovery test cases](../discovery/testcases.md) · [inventory test cases](../inventory/testcases.md) · [provisioning test cases](../provisioning/testcases.md) (Korean) | per-situation acceptance criteria + implementation order (TDD) |
| [What the demo verifies](../demo/integration-verification.md) (Korean) | the six demo steps take on the cases that need the real thing. What it does not cover is written down too |

## Contracts and the data model
| Document | Contents |
|---|---|
| [**Data model schema**](../contracts/data-model.en.md) | a human-facing map of every specified message and enum — purpose, key fields, relationships, provenance lanes. The contracts SSOT reference |
| [contracts/README](../contracts/README.en.md) | the list of protobuf files and namespaces + the CycloneDX property mapping |
| [Designs under review](under-review.en.md) | designs for what is on the roadmap but **not committed to implementation** — accepting providers (the config vocabulary to generate), observing providers, the HSM axis, and the three netcap designs (server-role edges, scope classification for the edge peer, the traffic-observation cadence). Once settled it moves into the real design documents and is deleted from here |
| [Crypto runtime acceptance principles](runtime-acceptance.en.md) | what is accepted as a first-class crypto runtime — why the two accepted (OpenSSL, JCA) are isomorphic, the three acceptance conditions the four axes leave unsaid, and the decision tree for a new candidate |

[`contracts/`](../contracts/) — the protobuf SSOT. The namespace is the stage.

---
The build, test, and contribution workflow: [CONTRIBUTING.md](../CONTRIBUTING.en.md).
