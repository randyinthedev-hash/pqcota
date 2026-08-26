English · [한국어](README.md)

# The pqcota demo (OSS) — access prep → discovery → inventory → provisioning

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](README.md) is authoritative.

**With just Docker**, an end-to-end demo that installs, runs, and removes itself in one line each. Across nodes joined on a single virtual network, pqcota shows the whole scope: **① access prep from the user's hosts file (no secret is persisted) → ② discovery of OpenSSL, Java (JCA), and communication handshakes over Ansible/SSH → ③ a central inventory (endpoints, profiles, app attribution, history, asset scope) → ④ provisioning (**generating** an L2 playbook and rollback records → applying them → rolling them back)**.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

> **① is needed because of how this demo is built, not because observation requires it.** The demo runs
> the scanners **over SSH to several nodes** from a controller, so it needs a connection inventory. The path
> where you scan a single node in place, or collect result files and ingest them, needs no ① at all —
> what is required and what is optional is in the [discovery/cmd README](../discovery/cmd/README.en.md).

> **Boundary**: this demo is complete with this repo (Apache-2.0) alone. Provisioning goes as far as
> **generation and persistence**, and the demo **actually applies the generated playbook and rolls it back** —
> checking generation alone lets through a playbook that breaks on a clean node (there really was such a defect).
> **Declaration reconciliation (`UNDECLARED`/`UNOBSERVED`), review-and-finalize governance, and dynamic provisioning**
> are not done by this repo, so they are not in the demo either.
> (Diffing change between snapshots *is* an observed fact, so it is in this repo — per architecture §6.)

📊 **Expected results before you run**: [`expected-output/`](expected-output/) — sample console output and topology SVG, plus what may differ on a real run (edge capture timing, base image versions).

## Requirements
- Docker (Compose v2) · internet (for the first image build) · your user in the `docker` group (no root, no KVM)

The repo is built inside the `pqcota-ctl` container ([below](#where-the-repo-is-built--on-the-ctl-machine)).
What gets built is **the source as currently checked out**, including uncommitted changes.

## Quick start
```bash
./demo/scripts/up.sh      # images → containers → **build the repo on ctl** → SSH keys → hosts.csv
./demo/scripts/demo.sh    # access prep → discovery → inventory (metadata, app attribution, history, scope) → provisioning (generate, apply, roll back)
./demo/scripts/down.sh    # clean up (--rmi also removes images)
```

`./demo/scripts/demo.sh --help` lists every knob — one of them is the [optional step](#optional-step--the-last-inch-with-a-real-provider-demo_real_provider1) below.

> **The demo environment is defined by one file, `demo/topology/topology.yaml`.** A sample is copied in
> automatically on the first run (and git-ignored); edit it and the node count, node kinds, OpenSSL versions,
> JCA providers, network segments, and handshakes all follow — so you can shape it closer to your own
> environment and run the same end-to-end flow.
> Details: **[topology/README](topology/README.md)** (Korean).

## Folder layout
| Folder/file | What it is | Run it directly? |
|---|---|---|
| [`scripts/`](scripts) | what **the user runs** — `up.sh` (install) · `demo.sh` (run) · `down.sh` (remove) | ✅ these three |
| [`scripts/ansible/`](scripts/ansible) | the discovery **orchestration** demo.sh drives — SSH inventory and playbook (`discover.yml`) | ❌ |
| [`integration-verification.md`](integration-verification.md) | **the integration cases this demo verifies**, and what it does not cover (Korean) | ❌ |
| [`scripts/internal/`](scripts/internal) | helpers that run **inside** the containers (node boot, service start/stop, traffic generation, observation). `ssl-apps.sh` is the service management point the L3 hook points at | ❌ |
| [`workloads/`](workloads) | the **demo crypto workloads** deployed to nodes (the things being scanned and observed): `CryptoApp.java` (JCA/BouncyCastle) · `pqc-echo/` (a PQC TLS traffic generator, in Go) | ❌ |
| [`expected-output/`](expected-output) | a preview of the **expected results** before you run | ❌ |
| [`topology/`](topology) | the **environment definition** — `topology.yaml` (a sample is copied on first run) and the generator | ✏️ edit this file to change the setup |
| `Dockerfile` | the container **build** definition (a stage per node kind) | ❌ (the scripts call it) |

> If this is your first time, the three scripts **up → demo → down** are all you need. Everything else is machinery running behind them.

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
> The connection key (`/work/id_demo`) and `targets.ini` exist **only inside the controller** and are never ingested into the inventory (§1.5).

## Where the repo is built — **on the ctl machine**

`pqcota-ctl` *is* the build machine. Step 3 of `up.sh` compiles inside that container, and discovery, inventory, and provisioning all run on that same machine.

| What | With which options | Where |
|---|---|---|
| proto-generated code | `make generate` (buf) | `/src/gen/` |
| the central CLIs (`ingest`, `inventory`, `provision`, …) | `CGO_ENABLED=0 go build` | `/usr/local/bin/` (run on ctl) |
| the collectors (`nodescan`, `netcap`, `jvmscan`) | `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build` | `/work/dist/linux-<arch>/` (shipped to the nodes) |
| the JVM attach sidecar | `make build-jar` (javac + jar) | `/work/dist/collector.jar` |

Run it and it says so:

```console
▶ 3/6 building the repo — the source is compiled **on the ctl machine (pqcota-ctl)**
     [ctl] Ubuntu 24.04.4 LTS · x86_64 · go1.26.4
     [ctl] make generate …  go build -o /usr/local/bin/ …  GOARCH=amd64 go build -o dist/linux-amd64/ …
```

**Your environment works the same way.** The build machine just has to be Linux (Go 1.26.4+ and buf; JDK 11+ is optional), and only the collectors need to be built **for the node's arch**. Being `CGO_ENABLED=0` statically linked, they do not care about distro or libc — in this very demo, binaries built on Ubuntu 24.04 run as-is on a 20.04 node.

The image build (step 1) produces only the OS, the toolchain, and the workloads that are the **subject** of observation (`pqc-echo` = your app in the real world, `topogen` = needed before the containers exist). **The pqcota software itself is not baked in.**

## At run time — the running containers (default topology)

Defined by [`topology/topology.yaml`](topology/README.md) (Korean). The table below is the default, **measured with the demo up**.

| Container | Base OS · arch | Segment | Long-running process · listening | In a real environment |
|---|---|---|---|---|
| **pqcota-ctl** | Ubuntu 24.04 · host arch | corp+db | `sleep infinity` · none | the machine where you build the repo and run the tools |
| **pqcota-demo-pg** | Debian 13 (postgres:16) · host arch | corp+db | `postgres` · :5432 | the central inventory DB (not needed on the single-host path) |
| **web-gw** | Ubuntu 24.04, OpenSSL **3.x** · host arch | corp | `sshd` · :22 | observed — the TLS/SSH client side |
| **pay-app** | Ubuntu 26.04, temurin 21 · host arch | corp | `sshd`·`java`·`pqc-echo` · :22 :8443 | observed — the JVM asset |
| **pay-db** | Ubuntu 20.04, OpenSSL **1.1.1** · host arch | corp+db | `sshd`·`payment-gw`·`api-gw` · :22 :4433 :4434 | observed — the legacy asset |

The node OS is chosen by `version` and `fork` in `topology.yaml` (3.x→24.04, 3.0→22.04, 1.1.1→20.04, libressl→alpine). Every arch matches the host. **Only the controller joins both segments** (SSH to every node). IPs are reassigned on every run, so nodes are referred to by segment name only.

**There is no collector on the nodes** — only the workloads and demo helpers, and discovery leaves nothing behind:

```console
$ docker exec pay-db ls /usr/local/bin
node-entrypoint.sh  pqc-echo  pqcota-gen-traffic.sh  pqcota-observe.sh  ssl-apps.sh
```

`topo-gen` runs briefly at step 0 with `--rm` (it generates the compose file, `groups.ini`, and the SVG, then disappears).

## Discovery (Ansible/SSH, all of it real)
1. **OpenSSL assets** — `pqcota-nodescan`: the loaded libssl/libcrypto, found by scanning `/proc`.
2. **The JCA provider chain** — `pqcota-jvmscan`: **recon then attach**. It finds the running JVM (pay-app's CryptoApp) through `/proc`, attaches to that PID, and reads the real `Security.getProviders()`. That catches **the BouncyCastle CryptoApp registered at runtime with `addProvider`** — there is no static registration in java.security, so **a static scan cannot see it** (symmetrical with openssl's `/proc` scan; `detection=runtime-introspection`). If attach is impossible it falls back honestly to a static probe.
3. **Communication edges** — `pqcota-netcap`: TLS/SSH handshakes observed through AF_PACKET (`CAP_NET_RAW`) without decryption.

`pqcota-discover-view` (OSS) collates the results into **discovered assets plus the grade of the observed edges**:
- 🟢 **PQC/hybrid** (`X25519MLKEM768`, `sntrup761x25519`) · 🔴 **classical = quantum-vulnerable** (`x25519`, `ECDHE`) · ⚪ **unknown**
- For example: `web-gw → pay-app` 🟢 MLKEM · `web-gw → pay-db` 🔴 classical · SSH splits the same way (`→pay-app` 🟢 sntrup761 · `→pay-db` 🔴 — the legacy OS's OpenSSH has no PQC KEX)

## The central inventory (endpoints, profiles, app attribution, history, change)
`pqcota-ingest` appends the retrieved results to an append-only history, and `pqcota-inventory` queries it:
- **▸ Machine header** — the **endpoint** `pqcota-hosts` upserted (name, ip:port, no secrets) plus the **profile** (display_name, env, role, owner — the CMDB declaration lane).
- **One machine registered under several names is reported** — the ingest output carries `⚠ duplicate: physical machine … → [pay-db web-gw]`, and that is not an error. The demo targets are containers on one host, so `pay-db` and `web-gw` share the same physical-machine fingerprint, and the platform states that instead of hiding it (TK-MACHINE). This is what you see in production when one machine has been registered under several names.
- **@App attribution** — which app each crypto asset belongs to (`app_keys`). pay-db's shared `libssl.so.1.1` is attributed to **both** `payment-gw` and `api-gw` (replacing that .so affects both apps).
- **History and change** — the same retrieved results are ingested a second time (the equivalent of "the next scan round" in production) to show `-history` (points of change plus observation count), `-snapshot` (assets plus observed edges), and `-diff` (added, gone, changed). Because it is the same observation, **"no change"** is the correct diff — the tool does not invent change that is not there. When a version really does change, the finding id is preserved and it shows up as **a "change" to the same asset**.<br>Snapshots accumulate **only when the content changes**; repeated observation leaves only a lightweight observation record — storage grows with the number of changes, while the evidence that "we scanned every time" is preserved.
- **Asset scope** — a node being registered does not make every asset inside it in-scope. Filtering out noise like `sshd` and the packaged python runtime by rule leaves **only the assets the apps actually use**. The number filtered out is always reported — **exclusion is not absence** (§2.6).
- **Retention policy** — `pqcota-prune` is run as a dry run to show that **the newest snapshot per node is deleted by no policy**. Being destructive, it is separated from the query commands, and actual deletion happens only with `-apply`.

## Provisioning (generate → apply → roll back)
A **finalized plan (FINALIZED)** is built for the discovered findings and `pqcota-provision` is run on it:
- **The §3.7 gate** — anything not FINALIZED is refused. **An L2 playbook is generated** (module staging + config fragment).
- **Before capture and a persisted rollback record** — the crypto state before the remediation (module, version) and the **affected apps (several, for a shared .so)** are recorded append-only.

Then the output is **actually applied** — "we generated it" only means something once you have seen it run:

- **Apply** — the generated playbook is run against the target node (pay-db in the default setup) with `ansible-playbook`, passing the module sha256 gate along the way.
- **Verify** — that `/opt/pqcota/oqsprovider.so` and `/etc/pqcota/openssl-pqc.cnf` landed on the target, and that the config **references that staged path** (`module = /opt/pqcota/oqsprovider.so`).
- **Roll back** — removal via the `--rollback` playbook. Nothing ever overwrote the original config, so **removal alone restores the prior state**, and the demo checks that both files are gone.

> **Why apply at all**: generating without running lets through a playbook that is **syntactically fine but broken in practice**. There really was such a defect (the config directory was not created, so `copy` failed). This step catches that whole class continuously.
>
> **The tool does not supply the provider module.** To show only the deployment path, the demo uses an **empty file** — it has no actual crypto capability. A real module is built by the user or obtained from a vendor and brought in ([the custom provider procedure](../provisioning/design.en.md)). The demo deliberately uses an empty file because the point is **deployment and reversibility, not a cryptography demonstration**, and because it keeps the "all you need is Docker" premise intact.

- **L2 only stages the fragment** — it never makes it referenced, so every output is fully reversible.
- **L3 adds activation and restart on top.** The commands are what the user wrote in the plan's `activation` hook — the activation point differs per environment, so the tool does not guess. The demo nodes manage services through `ssl-apps.sh`, so the hook points at that (the counterpart of a real systemd unit or an in-house startup script).
- What L3 shows in the demo is **hook ordering, the connection to the activation point, restart, and reversibility**. The legacy node's OpenSSL does not know this fragment's PQC group, so **the demo does not claim its capability changed** — the real remediation for that node is a fork replacement, and the playbook says in a comment that this cannot be deployed through config.

### Optional step — the last inch with a real provider (`DEMO_REAL_PROVIDER=1`)

```bash
DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
```

One thing an empty file cannot show: **whether the config and staging the tool produced really create cryptographic capability.** Turning this variable on builds a real oqsprovider (liboqs + oqs-provider) on the same base as the nodes and checks that last inch. The first run takes a few minutes to build; the image is reused from then on.

The target is **not** the pay-db of step 6 — a provider is an OpenSSL 3 concept, so there is nowhere to put it on a 1.1.1 node. A node observed as 3.x in the inventory is chosen, the same L2/L3 output stages and activates it, and then:

| | What you see |
|---|---|
| **Capability** | the ML-KEM family in `openssl list -kem-algorithms` goes from **0 to 14**, and `list -providers` shows `oqsprovider … active` |
| **Re-observation** | discovery is run and ingested again, and `pqcota-inventory -diff` shows that node's change |
| **Rollback** | undoing in L3→L2 order takes it back to **0** — reversibility measured by the same ruler |

**On re-observation the inventory is unchanged.** The demo does not hide that; it states the reason alongside. OpenSSL has no path yet for observing the provider layer (it goes as far as libssl/libcrypto in `/proc/maps` and ELF strings — JCA sees the provider chain through attach, OpenSSL cannot), and a handshake requires both ends to know the negotiation, while the peer in this topology is 1.1.1. The design discussion is in [discovery design §2.1](../discovery/design.en.md). When it finishes, it undoes L3→L2 and leaves the node as it was.

## The access secret boundary (§1.5)
Connection keys and accounts ride only in **the user's hosts.csv → the runtime-only `targets.ini` (owner-only `0600`)**. Only **the endpoint (node_id, name, ip, port)** is upserted into the pqcota inventory (Postgres); no secret is ingested (the demo verifies there are zero secrets in `pqcota_endpoint`).

## To apply it to your own environment (real assets)

The demo stands the containers up for you; against real assets **the environment already exists** and you prepare three things.
The machine roles are the same as in [At run time — the running containers](#at-run-time--the-running-containers-default-topology) above —
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
