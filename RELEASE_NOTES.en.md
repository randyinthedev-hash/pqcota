English · [한국어](RELEASE_NOTES.md)

# Release Notes — pqcota

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](RELEASE_NOTES.md) is authoritative.

Records the **goals** and **results** per version. This is still before the first official release (v0.1.0); the document is updated as versions advance. (Newest version on top.)

---

## Roadmap — Upcoming releases (planned)

Directional, not fixed. Each version is promoted to a proper section per the rule above once started/completed. The **Windows CNG runtime is introduced in stages** — why it isn't added all at once, plus the pressure test: [Runtime Extension Contract](docs/런타임_확장_계약.md) (Korean).

- **v0.2.0 (planned)** — **CNG discovery**: a Windows collector (`BCryptEnumProviders` · registry introspection) fills `CngAxes` so the assets converge into the inventory. (The schema was already reserved in v0.1.0 — this release is the "code that fills it".)
- **v0.3.0 (planned)** — **CNG provisioning**: **substrate generalization first** (moving past the POSIX-file assumption — Windows uses the registry/GPO, which doesn't fit `/opt/pqcota` file staging or file-removal rollback) → `renderCNG`. The generalization is done when this implementation is in hand (no speculative abstraction).

- **Signed binary release (planned · version TBD)** — per-arch build → SHA256SUMS → ed25519 signature →
  `pqcota-verify-bundle`. The bundle layout, signing, and verification are settled in the
  [collector deployment design](discovery/collector_배포_설계.md) (Korean); what remains is the workflow.
  **Until then, users build from source too** → [Build](README.en.md#build).

> **Why split it** — CNG's discovery half fits the current contract, but its provisioning half requires a new substrate. Each is completed honestly in order: schema → discovery → provisioning. Python · Go · .NET · Node are not peer runtimes — they resolve into OpenSSL/CNG — so they are not separate release targets (see the document above).

---

## v0.1.0 — First release (unreleased · in development)

**Goal** — a **three-stage end-to-end** you build from source and run. Distributing signed pre-built binaries is deferred to a later release (see the roadmap above).

### Results (done)

- **Contract SSOT** — protobuf across 4 namespaces (`common` · `discovery` · `inventory` · `provisioning`), code generated with `make generate`.
- **Discovery** — openssl · jvm · network reference collectors (both do a `/proc` reconnaissance first — openssl for loaded libs, jvm enumerates running JVMs → attach, distinguishing multiple JVMs per app), a normalization pipeline (evidence · completeness map), history ingestion with ed25519 signing (every collector assertion is signed), and delegated CBOM intake.
- **Inventory** — central ingestion/query (Postgres), machine metadata (endpoints · profiles), app attribution, and **history browsing · snapshot detail · change diff** (`-history` · `-snapshot` · `-diff`).
- **Retention policy** — two-tier separation of observation records and snapshots (repeated observations of the same state don't grow storage, yet "when it was seen" is preserved) + pruning (`pqcota-prune`, dry-run by default · latest is inviolable · pruning is recorded).
- **Asset scope** — extends the node-registration gate (§0.4) to the asset level. Only the assets the user declared as managed are ingested, and the excluded count is reported (`pqcota-ingest -scope-assets`).
- **Provisioning generation** — the execution gate (finalized-only), taxonomy→config artifacts, apply/rollback Ansible playbooks (**L1/L2/L3**), before-state capture and rollback records.
- **L3 activation and restart** — the commands come from the plan's `activation` hooks (pre·activate·deactivate·restart), **written by the user**; the generator places them in the order that makes them safe (bring down → change → make referenced → restart), and rollback is the exact reverse. How to activate differs per environment, so **the tool does not guess**: an empty hook generates nothing, and what will not happen is reported.
- **CNG schema reservation** — adds `CRYPTO_RUNTIME_WIN_CNG` enum + `CngAxes` (oneof arm) to the contract (**not implemented** — the collector, normalization, and provisioning that fill it come in v0.2.0/v0.3.0). This is both the starting point of the staged rollout and a proof of the contract claim that "a new runtime is accepted with no core change" (purely additive · backward compatibility verified).
- **Verification** — the demo's 6-stage end-to-end (the generated playbook is actually **applied, activated, and rolled back** on a real node), per-stage examples, a green test suite across all packages, and a docs gate (`make check-docs` — links, anchors, stale scope claims, personal data).

### Remaining (before the v0.1.0 release)

- **Pin and document the minimum supported kernel** — the static binaries don't care about distro or libc (verified on CentOS 7, Debian 8, Alpine), but the **kernel floor is set by the Go toolchain and is not yet stated.** Since the observed targets are legacy servers, this *is* coverage.
- **Verify on legacy hardware** — actually run on an old-kernel machine/VM. Containers share the host kernel, so they **cannot verify this.**

---

<!-- Add v0.2.0 and later as new sections above this line. -->
