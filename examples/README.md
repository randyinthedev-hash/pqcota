# examples/: where the runnable examples are

The examples are minimal, one-command-at-a-time runs for getting a feel for how each command is used. Each stage's examples now live **in the repository of the stage whose commands they run**, so whoever changes a command finds its example next to it. Where the demo (`demo/`) shows the whole flow end to end with containers, an example runs **one stage's commands with minimal setup**.

| Stage | Where | What you run | Prerequisites |
|---|---|---|---|
| **Discovery** | [pqcota-discovery `examples/discovery`](https://github.com/randyinthedev-hash/pqcota-discovery/tree/main/examples/discovery) | `pqcota-hosts` (hosts → Ansible and endpoints) · `pqcota-ingest` (① load direct observation) | Go only |
| ↳ JVM | [pqcota-discovery `examples/discovery/jvm`](https://github.com/randyinthedev-hash/pqcota-discovery/tree/main/examples/discovery/jvm) | `pqcota-jvmscan` reconnaissance → attach (the dynamic registrations of a running JVM) | **Go + Docker + JDK** |
| **Inventory** | [pqcota-inventory `examples/inventory`](https://github.com/randyinthedev-hash/pqcota-inventory/tree/main/examples/inventory) | `pqcota-discover-view` · `pqcota-declare-attribution` · `pqcota-cbom-ingest` | Go only |
| **Provisioning** | [pqcota-provisioning `examples/provisioning`](https://github.com/randyinthedev-hash/pqcota-provisioning/tree/main/examples/provisioning) | `pqcota-provision` (finalized plan → L2/L3 playbook), `pqcota-approve`, rollback | Go only |

The sample collector results that the discovery and inventory examples share are in [pqcota-inventory `examples/data`](https://github.com/randyinthedev-hash/pqcota-inventory/tree/main/examples/data), because the inventory is what reads them.

## Prerequisites

- **The Go toolchain.** The generated contract code is committed in `pqcota-common`, so right after cloning you can run from source with `go run`, and no Postgres or target nodes are needed. If you changed a proto, run `make generate` in `pqcota-common` first (it needs buf).
- **One exception:** the JVM example needs a **live JVM**, so it uses **Docker** (the JDK is inside the container).
- Clone the repositories side by side, as described in the [README](../README.md#build). Run an example from the root of its own repository, for example:

```bash
cd ../pqcota-inventory
./examples/inventory/run.sh
```

## The end-to-end flow

For the whole flow through Postgres persistence, real Ansible/SSH connections and live scans, see [demo/](../demo) (Docker, six steps).
