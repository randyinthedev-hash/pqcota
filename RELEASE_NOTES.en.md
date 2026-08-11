English · [한국어](RELEASE_NOTES.md)

# Release Notes — pqcota

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](RELEASE_NOTES.md) is authoritative.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](docs/regulation.en.md).

Records the **goals** and **results** per version. Updated as versions advance, newest on top.

---

## Roadmap — Upcoming releases (planned)

Directional, not fixed. Each version is promoted to a proper section per the rule above once started/completed. The **Windows CNG runtime is introduced in stages** — why it isn't added all at once, plus the pressure test: [Accepting a new crypto runtime](docs/runtime-acceptance.en.md).

- **v0.2.0 (planned)** — **CNG discovery**: a Windows collector (`BCryptEnumProviders` · registry introspection) fills `CngAxes` so the assets converge into the inventory. (The schema was already reserved in v0.1.0 — this release is the "code that fills it".) Design review: [Designs under review §2.2](docs/under-review.md) (Korean).
- **v0.3.0 (planned)** — **CNG provisioning**: **substrate generalization first** (moving past the POSIX-file assumption — Windows uses the registry/GPO, which doesn't fit `/opt/pqcota` file staging or file-removal rollback) → `renderCNG`. The generalization is done together with that implementation (no speculative abstraction). Where to draw the seam is still undecided — [Designs under review §2.2](docs/under-review.md) (Korean).

- **v0.4.0 (planned)** — **attributing edges to apps**: `ObservedEdge` stops at the node today. When two
  apps on one node use the same library, which of them owns an observed edge is unknown. Correlating
  socket inodes (`/proc/net/tcp`) against `/proc/*/fd` at capture time fills in `app_key` (a purely
  additive contract change). What the automatic path misses arrives through the declared lane, and no
  admin UI is built — the design and the reasoning: [designs under review](docs/under-review.en.md).

- **Accepting the provider ecosystem (under review · version TBD)** — choosing which provider to use, and obtaining its file, is done by whoever writes the plan. What this repo does is **write the configuration file that activates that provider**. Today it only knows one shape, `activate`+`module` — and since each provider demands different settings, it cannot yet produce one for OpenSSL's own `fips` module (which has to pull in the file `fipsinstall` generates) or for pkcs11-provider (which needs additional entries such as the driver path). What each candidate would additionally require, along with provider observation and the HSM axis, is worked out in [Designs under review](docs/under-review.md) (Korean).

- **Release signing (planned · version TBD)** — the **ed25519 signature and `pqcota-verify-bundle`**. The bundle layout, signing, and verification are settled in the [collector deployment design](discovery/collector-deployment.md) (Korean). Until then, verify integrity with `sha256sum -c`.

### Not on the roadmap — deliberately

These are **boundaries**, not directions. Written down so no one waits for them.

| Not built | Instead |
|---|---|
| **Fleet orchestration** — drain · rolling · health gates | Standard Ansible playbooks come out, so your deployment tooling drives them |
| **Remote execution engine** — resident agents, push channels | You run the generated artifacts on your own substrate |
| **Source / artifact CBOM scanner** | CI already has the source. CycloneDX from CBOMkit and friends is **ingested** instead |
| **Dynamic tracing** (eBPF · ltrace) | Invasive, so it isn't done. Observing the actual negotiation on the wire was chosen instead |
| **Verdicts and scoring** — "risky" grades | Only observed facts are emitted. What to change, and when, is yours to decide |


---

## v0.1.2 — Putting reconciliation state in the vocabulary (2026-08-11)

`decision.proto` had the **conclusion** and the **lifecycle** of a verdict in the contract, but never
defined **what the verdict is about**. A comment pointed at `UNOBSERVED` while that value existed
nowhere in the contract ([#3](https://github.com/randyinthedev-hash/pqcota/issues/3)).

- **`ReconState` added** — `CONFIRMED` (declared ∩ observed), `UNDECLARED` (observed only = shadow),
  `UNOBSERVED` (declared only). **The reconciliation engine is not in this repository** — like
  `DecisionConclusion` and `FinalizedPlan`, only the schema lives here, so that consumer engines
  speak the same vocabulary.
- **`Decision.state` (field 10) added** — which state the verdict was made against. Without it a
  consumer receives a `conclusion` with no idea what it was about.

Purely additive — existing field numbers and types are untouched, so the `buf breaking` baseline holds.

---

## v0.1.1 — Making the contract consumable (2026-08-11)

**What was wrong** — the generated code (`gen/`) sat in `.gitignore`, so anyone trying to consume
the contract got no types from `go get`. `contracts/README.md` said the schemas were SSOT "so that
consumer engines speak the same vocabulary", yet that vocabulary could not be imported. The first
outside consumer surfaced it.

- **`gen/` is committed** — `go get` alone now gives you `commonv1`, `discoveryv1`, `inventoryv1`
  and `provisioningv1`. Hand-edited generated code is cut by the CI generate-drift check, which
  only now has anything to check.
- **buf pinned** (CI, 1.69.0) — with generated code committed, a tool version change could fail the
  drift check without any code change.
- How to consume it, and the module-path workaround, are written up in
  [contracts/README](contracts/README.md) (Korean).

The contract itself (proto) did not change — the `buf breaking` baseline is untouched.

---

## v0.1.0 — First release (2026-08-11)

**Goal** — a **three-stage end-to-end** you can download and run. Per-arch static binaries and `SHA256SUMS` ship with the release; **signing** (ed25519) is deferred to a later release (see the roadmap above).

### What was built

- **Contract SSOT** — protobuf across 4 namespaces (`common` · `discovery` · `inventory` · `provisioning`), code generated with `make generate`.
- **Discovery** — three reference collectors. **openssl and jvm both start with a `/proc` sweep** — openssl for loaded libs, jvm enumerating running JVMs to attach to, distinguishing multiple JVMs per app. **network does not touch `/proc`**; it observes the wire passively through `AF_PACKET`. On top of those: a normalization pipeline (evidence · completeness map), history ingestion with ed25519 signing (every collector assertion is signed), and delegated CBOM intake.
- **Inventory** — central ingestion/query (Postgres), machine metadata (endpoints · profiles), app attribution, and **history browsing · snapshot detail · change diff** (`-history` · `-snapshot` · `-diff`).
- **Retention policy** — two-tier separation of observation records and snapshots (repeated observations of the same state don't grow storage, yet "when it was seen" is preserved) + pruning (`pqcota-prune`, dry-run by default · latest is inviolable · pruning is recorded).
- **Asset scope** — extends the node-registration gate (§1.4) to the asset level. Only the assets the user declared as managed are ingested, and the excluded count is reported (`pqcota-ingest -scope-assets`).
- **Provisioning generation** — the execution gate (finalized-only), taxonomy→config artifacts, apply/rollback Ansible playbooks (**L1/L2/L3**), before-state capture and rollback records.
- **L3 activation and restart** — the commands come from the plan's `activation` hooks (pre·activate·deactivate·restart), **written by the user**; the generator places them in the order that makes them safe (bring down → change → make referenced → restart), and rollback is the exact reverse. How to activate differs per environment, so **the tool does not guess**: an empty hook generates nothing, and what will not happen is reported.
- **CNG schema reservation** — adds `CRYPTO_RUNTIME_WIN_CNG` enum + `CngAxes` (oneof arm) to the contract (**not implemented** — the collector, normalization, and provisioning that fill it come in v0.2.0/v0.3.0). This is the starting point of the staged rollout. What was confirmed is that **the contract has room for it** (purely additive — existing field numbers and types unchanged). Nothing has been run on real Windows, so this does not mean "CNG is supported".
- **Verification** — the demo's 6-stage end-to-end (the generated playbook is actually **applied, activated, and rolled back** on a real node), per-stage examples, all 172 tests green ([level distribution](docs/test-map.md) (Korean)), and a docs gate (`make check-docs` — links, anchors, stale scope claims, personal data).
- **Real-provider check (optional stage)** — with `DEMO_REAL_PROVIDER=1` the demo builds a real oqsprovider, deploys and activates it on an OpenSSL 3.0–3.4 node, and measures whether the capability **actually appeared**, via `openssl list` (ML-KEM KEMs 0 → 14; back to 0 after rollback). This check caught a defect in the config-file generation: the generated fragment lacked the top-level `openssl_conf = openssl_init`, so in environments that point `OPENSSL_CONF` at it, **the module was placed and the sha256 gate passed while the provider never loaded**. Fixed, with a regression test.

- **Releasing** — pushing a tag makes CI build per-arch static binaries (`linux-amd64`, `linux-arm64`) and `collector.jar`, then attach `SHA256SUMS`. Verify what you download with `sha256sum -c SHA256SUMS`.

### What was established

- **Minimum supported kernel = 3.2** (the floor the Go toolchain sets — it became this in 1.24 and has held since. Building needs Go 1.26.4, per `go.mod`). Nothing here needs anything newer; the one per-feature addition is `NSpid` (4.1) for JVM attach inside containers, and that falls back to the host PID. Table: [discovery/cmd — supported range](discovery/cmd/README.md#실행-요건--커널권한) (Korean).
- **Legacy verification done** — all three collectors ran on kernel **3.2** (Ubuntu 12.04) and **3.10** (CentOS 7.9) VMs. They work at the floor itself, and neither kernel has `NSpid`, so the host-PID fallback was exercised for real.

---

<!-- Add v0.2.0 and later as new sections above this line. -->
