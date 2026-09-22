English · [한국어](README.md)

# The pqcota demo (OSS) — access prep → discovery → inventory → provisioning

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](README.md) is authoritative.

**With just Docker**, an end-to-end demo that installs, runs, and removes itself in one line each. Across nodes joined on a single virtual network, pqcota shows the whole scope: **① access prep from the user's hosts file → ② discovery of OpenSSL, Java (JCA), and communication handshakes over Ansible/SSH → ③ a central inventory → ④ provisioning (generate the playbooks → apply → roll back)**.

This document is for **the person running the demo**. Why the demo is built this way and what runs inside it is in [the demo design](design.en.md); what it verifies and what it does not is in [what the demo verifies](integration-verification.md) (Korean).

📊 **Expected results before you run**: [`expected-output/`](expected-output/) — sample console output and topology SVG, plus what may differ on a real run (edge capture timing, base image versions).

## Requirements
- Docker (Compose v2) · internet (for the first image build) · your user in the `docker` group (no root, no KVM)

The repo is built inside the `pqcota-ctl` container ([design · build](design.en.md#3-build-on-the-ctl-machine)).
What gets built is **the source as currently checked out**, including uncommitted changes.

## Quick start
```bash
./demo/scripts/up.sh      # images → containers → build the repo on ctl → SSH keys → hosts.csv
./demo/scripts/demo.sh    # access prep → discovery → inventory → provisioning (generate, apply, roll back)
./demo/scripts/down.sh    # clean up (--rmi also removes images)
```

`./demo/scripts/demo.sh --help` lists every knob — one of them is the [optional step](#optional-step--the-last-inch-with-a-real-provider-demo_real_provider1) below.

If this is your first time, these three scripts in [`scripts/`](scripts) are all you need. The other folders are machinery running behind them, and [design · parts](design.en.md#2-parts) says what each one is.

> **The demo environment is defined by one file, `demo/topology/topology.yaml`.** A sample is copied in
> automatically on the first run (and git-ignored); edit it and the node count, node kinds, OpenSSL versions,
> JCA providers, network segments, and handshakes all follow — so you can shape it closer to your own
> environment and run the same end-to-end flow.
> Details: **[topology/README](topology/README.md)** (Korean).

## What you see

`demo.sh` reports progress as `▶ N/6`. The default topology has three nodes — web-gw (OpenSSL 3.x) · pay-app (JVM) · pay-db (OpenSSL 1.1.1, legacy) — and the controller `pqcota-ctl` scans them over SSH. What each step shows:

| Step | What you see |
|---|---|
| **0/6 access prep** | The Ansible inventory is generated from `hosts.csv`, and the endpoints and CMDB profiles are registered. The inventory table is counted by SQL for access secrets: **0** |
| **1/6 SSH check** | Ansible ping from the controller to every node |
| **2/6 discovery** | On each node, OpenSSL assets · the JCA provider chain (including the BouncyCastle added at runtime with `addProvider`) · TLS/SSH handshakes are observed and the results are retrieved. Nothing is left on the nodes afterward |
| **3/6 discovery view** | The discovered assets and the grade of the observed edges: 🟢 PQC/hybrid · 🔴 classical (quantum-vulnerable) · ⚪ unknown. In the default topology `web-gw → pay-app` is 🟢 and `web-gw → pay-db` is 🔴 (for both TLS and SSH) |
| **4/6 topology** | The observations drawn as a picture, saved to `demo/.generated/topology.svg` |
| **5/6 central inventory** | Ingest, then query: the endpoint and profile header, an `@app` label on every asset (pay-db's shared `libssl.so.1.1` is attributed to both `payment-gw` and `api-gw`), `-history` · `-snapshot` · `-diff` after ingesting the same results a second time (the correct answer is **no change**), asset scope (the number excluded is reported), and a `pqcota-prune` dry run |
| **6/6 provisioning** | From a finalized plan, the L2 and L3 playbooks and the rollback record are **generated**; they are **applied** to the target node (pay-db in the default setup) and the demo checks that `/opt/pqcota/oqsprovider.so` and `/etc/pqcota/openssl-pqc.cnf` landed; then the rollback playbook **undoes** it and the demo checks that both files are gone |

Two things appear in the output as-is and are not errors.
- `⚠ duplicate: physical machine … → [pay-db web-gw]`: the demo targets are containers on one host, so they share a physical-machine fingerprint. It is what you see in production when one machine has been registered under several names.
- The `oqsprovider.so` that provisioning stages is an **empty file**. What the demo shows is deployment and reversibility, not cryptographic capability ([why](design.en.md#1-principles)). To check it with the real thing, turn on the optional step below.

### Optional step — the last inch with a real provider (`DEMO_REAL_PROVIDER=1`)

```bash
DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
```

One thing an empty file cannot show: **whether the config and staging the tool produced really create cryptographic capability.** Turning this variable on builds a real oqsprovider (liboqs + oqs-provider) on the same base as the nodes and checks that last inch. The first run takes a few minutes to build; the image is reused from then on.

The target is not pay-db but a node observed in the inventory as OpenSSL 3.0–3.4 — a provider is an OpenSSL 3 concept, so there is nowhere to put it on a 1.1.1 node. The same L2/L3 output stages and activates it, and then:

| | What you see |
|---|---|
| **Capability** | the ML-KEM family in `openssl list -kem-algorithms` goes from **0 to 14**, and `list -providers` shows `oqsprovider … active` |
| **Re-observation** | discovery is run and ingested again, and `pqcota-inventory -diff` shows that node's change |
| **Rollback** | undoing in L3→L2 order takes it back to **0** — reversibility measured by the same ruler |

**On re-observation the inventory is unchanged.** That is not an error: OpenSSL has no path yet for observing the provider layer, and the demo prints the reason alongside. The details are in [design · optional step](design.en.md#54-the-optional-step-a-real-provider).

## Where the outputs land

**Most of it lives inside the containers**, and the only thing that lands in the repo is **`demo/.generated/`** (gitignored). `down.sh` deletes that folder wholesale, and whatever is in the containers disappears with them.

| Where | What | Cleanup |
|---|---|---|
| **The repo**, `demo/.generated/` | **everything** that lands in the repo — `topology.svg` and `topology.dot` (the observed topology drawing) plus the `docker-compose.yml`, `groups.ini`, `profiles.csv`, and `manifest.env` generated from the topology | `down.sh` deletes the folder (gitignored) |
| **The controller**, `pqcota-ctl:/work/` | build output `dist/linux-<arch>/` (the three collectors) and `dist/collector.jar` · retrieved observations `results/*.json` · connection `hosts.csv`→`ansible/targets.ini` (0600, secret) · `nodes.json` · `profiles.csv` · the finalized plan `plan.json` · the generated `ansible/playbook{,-l3}.yml` and `rollback{,-l3}.yml` · the module `ansible/files/oqsprovider.so` (an empty file) | disappears with the container |
| **Postgres**, `pqcota-demo-pg` | the central inventory — snapshots, observation records, endpoints, profiles, provisioning records | `down.sh` deletes the volume too (`-v`) |
| **The target nodes** | at the apply step, `/opt/pqcota/oqsprovider.so` and `/etc/pqcota/openssl-pqc.cnf`, plus the L3 activation point `/etc/pqcota/service.env` — **removed at the rollback step**, back to the original state | the demo rolls itself back |

To look inside (before you tear the demo down):

```bash
docker exec pqcota-ctl ls -R /work           # everything on the controller
docker exec pqcota-ctl cat /work/ansible/provision.yml   # the generated playbook
docker exec pqcota-demo-pg psql -U postgres -d pqcota -c '\dt'  # the inventory tables
```

> **The host filesystem is barely touched** — all that stays in the repo is the drawing and generated files above, and even those are gitignored.
> The connection key (`/work/id_demo`) and `targets.ini` exist **only inside the controller** and are never ingested into the inventory ([design · the access secret boundary](design.en.md#6-the-access-secret-boundary-15)).

## To apply it to your own environment (real assets)

The demo stands the containers up for you; against real assets **the environment already exists** and you prepare three things.
[The journey](../journey.md) (Korean) follows what comes out, in what order, from start to finish without containers.
The machine roles are the same as in [design · the running containers](design.en.md#4-at-run-time-the-running-containers-default-topology) —
**`pqcota-ctl` is the machine where you clone and build the repo**, and nothing is pre-installed on the nodes.
It does not end at `hosts.csv`:

| # | What you prepare | Required? | What it is |
|---|---|---|---|
| 1 | **`hosts.csv`** | required for remote multi-node | node_id, ip, port, account, key → `pqcota-hosts` generates the Ansible inventory (`targets.ini`, 0600, not persisted). With `--dsn` it also upserts the endpoint (secrets excluded). **Not needed** if you are scanning one host in place |
| 2 | **the collector binaries on each node** | required | build `pqcota-nodescan`, `pqcota-jvmscan`, and `pqcota-netcap` on ctl — **the demo's playbook ships them for you** (`discover.yml` ships → runs → retrieves → cleans up). Build commands are in [the root README · Build](../README.en.md#build) (per-arch prebuilt binaries are already in the releases and their integrity is checked with `SHA256SUMS`; only the signature that proves they came from this repo is still on the [roadmap](../RELEASE_NOTES.en.md)) |
| 3 | **a way to run them** | required | Ansible or by hand, run the collectors on each node and retrieve the result JSON. The demo's [`discover.yml`](../discovery/ansible/discover.yml) is the **reference implementation** |

After that it is the same as the demo — hand the collected results to `pqcota-ingest` and they are normalized and stored; view them with `pqcota-inventory`.

> **✅ You can copy the demo's collector deployment as-is.** The node images contain **no** collector; `discover.yml`
> ships them from ctl, runs them, retrieves the results, and cleans up (zero residue on the nodes afterward). The JVM
> add-on goes **only to nodes that have a JVM**, via `-recon`. Porting it to a real environment is just pointing
> `collector_bin_dir` at your own build output (per arch).
>
> **Two things exist only in the demo** — do not carry them over:
> - **Traffic generation** (`traffic=` in `groups.ini`, `pqcota-gen-traffic.sh`): the demo has no handshakes to observe, so it **manufactures them on purpose.** A real environment has real traffic, so you only need to **observe** with `pqcota-netcap <node> <iface> <window>`.
> - **Group membership** (`[java]` in `groups.ini`): that is just the demo's way of picking which nodes get `pqcota-jvmscan`; use whatever your own inventory does.

**Optional**: the node registration gate (`pqcota-ingest <dir> <scope-file>`) · asset scope (`-scope-assets`) · CMDB profiles (`pqcota-profile`) · Postgres persistence (`PQCOTA_DSN`) · signature verification (`PQCOTA_VERIFY_KEY`). What is required and what is optional: [discovery/cmd README](../discovery/cmd/README.en.md).

## Beyond discovery
Discovery shows you as far as "what is actually negotiated" (the grade). **"How well does that match what was declared (CONFIRMED/UNDECLARED/UNOBSERVED)"**, along with governance and reconciliation, is not done by this repo, so it is not in the demo either.
