English · [한국어](README.ko.md)

# Expected output (sample)

This is the **representative result** you get when you run the demo (`../scripts/up.sh` → `demo.sh`). Check what you will see
before you run it. (Captured from a fully observed warm run.) The demo has seven steps (`0/6` to `6/6`): **access prep → SSH check → discovery → view → topology → inventory → provisioning**.
The sample below is the capture of the **discovery view** (`3/6`).

| File | Contents |
|---|---|
| [discover-view.txt](discover-view.txt) | console output: discovered assets (OpenSSL · JCA/BouncyCastle) + the grade of each observed communication edge |
| [topology.svg](topology.svg) | the observed topology (colour = grade: 🟢 PQC / 🔴 classical / ⚪ unknown, solid line = observed) |

**The core story: a modern stack and a legacy one split on both TLS and SSH:**

| Edge | Grade | Why |
|---|---|---|
| `web-gw→pay-app` TLS | 🟢 X25519MLKEM768 | Go `crypto/tls` hybrid |
| `web-gw→pay-app` SSH | 🟢 sntrup761 | both sides run OpenSSH 9+ |
| `web-gw→pay-db` TLS | 🔴 x25519 | **OpenSSL 1.1.1** has no PQC group |
| `web-gw→pay-db` SSH | 🔴 curve25519 | **OpenSSH 8.2** on the legacy OS has no PQC KEX |

On `pay-app`, the **JCA provider chain including BouncyCastle** (a runtime `addProvider`, which a static scan cannot see) is also
observed by attach. The composition is defined by [topology/topology.yaml](../topology/README.md), and editing it changes this result too.

> **A grade is an observation, not a setting.** The SSH grade is computed from the **intersection of both sides' KEXINIT** (RFC 4253).
> If the client offers sntrup761 but the server does not support it, the edge is 🔴. When only one side is observed, no negotiation is invented and
> the edge stays ⚪ unknown.

What you additionally see in later steps:
- **Access prep (0)**: `pqcota-hosts` turns hosts.csv into an Ansible inventory (connection keys, runtime-only) plus an endpoint upsert. **Zero secrets** in the inventory.
- **Central inventory (5)**: an **endpoint and profile header** such as `▸ Payments DB (ip:22) │ Payments DB · production · db · owner=DBA team`, plus `@app` labels. pay-db's shared `libssl.so.1.1` is **attached to both apps**: `@/opt/apps/api-gw,/opt/apps/payment-gw`.
- **The app of an edge (5)**: the `@app` at the end of an observed-edge line. `@payment.service` when the observation caught it, `@/usr/bin/openssl(exe-path)` when it was caught by the exe path, and **`@?` when it could not be caught.** In this demo three of the four edges show `@?`
  with the reason printed alongside: `socket closed between capture and lookup — short-lived connections are missed(3)`. That happens because all the demo traffic
  is short-lived connections. If you assign one of them with `pqcota-declare-attribution`, it becomes
  `@batch-runner.service(declared)`, and **a cell the observation already caught stays as it was.**
- **Provisioning (6)**: finalized plan → L2/L3 playbooks + rollback record: `affected apps: /opt/apps/api-gw, /opt/apps/payment-gw` · `before : libssl.so.1.1@1.1.1f`.

## What can differ when you actually run it (and why)

Two devices were added for determinism, so **even the first run matches the result above**:
- **Edge capture retry-until-complete**: `demo.sh` re-collects until it reaches the target number of edges (the number of edges in the topology), up to 4 times.
  It absorbs the race between netcap's observation window and the traffic timing, so a cold-start first run also produces the complete edges.
  (Adjust with `DEMO_TARGET_EDGES` / `DEMO_MAX_ATTEMPTS`. Edges fall short only in an extremely constrained environment that cannot reach the target within the maximum attempts.)
- **The base images are pinned**, and so is BouncyCastle `1.85` (hash-checked). So the OpenSSL/Go/JDK versions and strings such as `1.1.1f`/`3.0.13`/`3.5.5`
  stay mostly **identical** across tag updates (a minor bump is possible when a distribution ships a security update).

What can still differ:
- **Container IPs** (172.18.0.x) change dynamically on every run. They do not matter to the story (nodes are joined by name).
- **If you edit the topology**, the nodes, edges and grades change accordingly. This sample is based on the **default composition**.

## Determinism

The same input gives the same output: the grade classification (🟢/🔴/⚪), the discovered assets and the three reconcile states (in a separate extension) are all
deterministic logic. The "differences" above are a matter of **what gets observed (capture timing)** and **the base image versions**, not non-determinism
in the classification or judgement logic.
