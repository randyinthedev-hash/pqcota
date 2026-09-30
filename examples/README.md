# examples/: runnable examples per stage (Discovery · Inventory · Provisioning)

There are several commands, so these are minimal examples to give you a feel for **how each one is actually run**. Each stage folder has **sample input + `run.sh` (copy and run) + a README (what happens)**. Where the demo (`demo/`) shows the whole end-to-end flow with containers, here you run **each command on its own with minimal setup**.

## Prerequisites
- **The Go toolchain**: `gen/` is committed, so right after cloning you can run from source with `go run`, and no Postgres or target nodes are needed. If you changed a proto, run `make generate` first (it needs buf, see [CONTRIBUTING](../CONTRIBUTING.md)).
- **There is one exception ([discovery/jvm](discovery/jvm/README.md)).** Reconnaissance → attach needs a **live JVM**, so it uses **Docker** (the JDK is inside the container). It is marked in the table below.

## What is here
| Stage | Folder | What you run | Prerequisites |
|---|---|---|---|
| **Discovery** | [`discovery/`](discovery) | `pqcota-hosts` (hosts → Ansible and endpoints) · `pqcota-ingest` (① load direct observation) · `pqcota-cbom-ingest` (② receive delegated CBOM) | Go only |
| ↳ JVM | [`discovery/jvm/`](discovery/jvm) | `pqcota-jvmscan` reconnaissance → attach (the dynamic registrations of a running JVM) | **Go + Docker + JDK** |
| **Inventory** | [`inventory/`](inventory) | `pqcota-discover-view` (assets, app labels, grades) · `pqcota-declare-attribution` (a person writes the app for an edge the observation could not attribute) · `pqcota-cbom-ingest` (receive an external CBOM) | Go only |
| **Provisioning** | [`provisioning/`](provisioning) | `pqcota-provision` (finalized plan → L2 playbook) | Go only |

```bash
./examples/discovery/run.sh
./examples/discovery/jvm/run.sh    # needs Docker + JDK (attaches to a live JVM)
./examples/inventory/run.sh
./examples/provisioning/run.sh
```

## Shared sample data: [`data/`](data)
The **retrieved results** that the Discovery and Inventory examples share (`CollectionResult` JSON of the kind a real collector would produce):
- `data/results/node-a-openssl.json`: an OpenSSL asset (the shared `libssl.so.3` **spans** two apps, `api-gw` and `payment-gw`).
- `data/results/node-b-jca.json`: a JCA provider chain (SUN · SunJCE · **BC**).
- `data/results/node-a-net.json`: three observed communication edges (🟢 MLKEM · 🔴 x25519 · 🟢 SSH sntrup761).
- `data/results/node-d-cng.json`: 9 Windows CNG providers and 50 algorithms. These are **values observed on a real machine**
  (Windows 11 Pro 25H2 · build 26200). The machine fingerprint was removed and only the node name was changed to fit the examples.
- `data/nodes.json`: mapping observed IPs to node names (10.0.0.9 → node-c).
- [`inventory/attribution.csv`](inventory/attribution.csv): one declaration line in which a person assigns the app for an edge the observation could not attribute.

> The `cbomCyclonedx` in a `CollectionResult` is **base64-wrapped CycloneDX** (protobuf `bytes`). Each stage's README carries the decoded content.

## What about the end-to-end flow with storage and targets?
For the whole flow that continues through Postgres persistence (the central inventory), real Ansible/SSH connections and live scans, see **[demo/](../demo)** (Docker, six steps). The examples here are for running each of its commands separately, at minimum.
