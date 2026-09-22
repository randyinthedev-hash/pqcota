English · [한국어](design.md)

# Demo design: what runs where, and why it was built this way

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](design.md) is authoritative.

**What this document is**: how `demo/` is put together and why. How to run it is in the [README](README.en.md), the cases it verifies are in [what the demo verifies](integration-verification.md) (Korean), and the environment definition is in [topology/README](topology/README.md) (Korean).

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](../docs/regulation.en.md).

## 1. Principles

- **All you need is Docker.** Building and observing both happen inside containers, and the only thing that lands in the repo is `demo/.generated/` (gitignored).
- **Access prep (①) is needed because of how this demo is built, not because observation requires it.** The demo runs the scanners **over SSH to several nodes** from a controller, so it needs a connection inventory. The path where you scan a single node in place, or collect result files and ingest them, needs no ① at all. What is required and what is optional is in the [discovery/cmd README](../discovery/cmd/README.en.md).
- **This demo is complete with this repo (Apache-2.0) alone.** Provisioning goes as far as **generation and persistence**, and the demo **actually applies the generated playbook and rolls it back**. Declaration reconciliation (`UNDECLARED`/`UNOBSERVED`), review-and-finalize governance, and dynamic provisioning are not done by this repo, so they are not in the demo either. Diffing change between snapshots *is* an observed fact, so it is in this repo (per architecture §6).
- **It does not stop at generation; it applies.** Generating without running lets through a playbook that is **syntactically fine but broken in practice**. There really was such a defect (the config directory was not created, so `copy` failed). `6/6` catches that whole class continuously.
- **The tool does not supply the provider module, so the demo uses an empty file.** It has no actual crypto capability. A real module is built by the user or obtained from a vendor and brought in ([the custom provider procedure](../provisioning/design.en.md)). The empty file is deliberate: the point is **deployment and reversibility, not a cryptography demonstration**, and it keeps the "all you need is Docker" premise intact. The last inch an empty file cannot show is covered by [the optional step](#54-the-optional-step-a-real-provider).

## 2. Parts

| Folder/file | What it is |
|---|---|
| [`scripts/`](scripts) | the three the user runs: `up.sh` (install) · `demo.sh` (run) · `down.sh` (remove) |
| [`scripts/ansible/`](scripts/ansible) | the discovery orchestration demo.sh drives: SSH inventory and playbook (`discover.yml`) |
| [`scripts/internal/`](scripts/internal) | helpers that run inside the containers (node boot, service start/stop, traffic generation, observation). `ssl-apps.sh` is the service management point the L3 hook points at |
| [`workloads/`](workloads) | the demo crypto workloads deployed to nodes (the things being scanned and observed): `CryptoApp.java` (JCA/BouncyCastle) · `pqc-echo/` (a PQC TLS traffic generator, in Go) |
| [`topology/`](topology) | the environment definition: `topology.yaml` (a sample is copied on first run) and the generator `topo-gen` |
| [`expected-output/`](expected-output) | a preview of the expected results before you run (console, topology SVG) |
| [`integration-verification.md`](integration-verification.md) | the integration cases this demo verifies, and what it does not cover (Korean) |
| [`recording/`](recording/README.md) | the procedure and editing skeleton for turning the demo into a screencast (Korean). Not needed to use the product; it is for rebuilding the presentation video |
| `Dockerfile` | the container build definition (a stage per node kind). The scripts call it |

## 3. Build: on the ctl machine

`pqcota-ctl` *is* the build machine. `3/6` of `up.sh` compiles inside that container, and discovery, inventory, and provisioning all run on that same machine.

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

The image build (`1/6` of `up.sh`) produces only the OS, the toolchain, and the workloads that are the **subject** of observation (`pqc-echo` = your app in the real world, `topogen` = needed before the containers exist). **The pqcota software itself is not baked in.**

## 4. At run time: the running containers (default topology)

Defined by [`topology/topology.yaml`](topology/README.md) (Korean). The table below is the default, **measured with the demo up**.

| Container | Base OS · arch | Segment | Long-running process · listening | In a real environment |
|---|---|---|---|---|
| **pqcota-ctl** | Ubuntu 24.04 · host arch | corp+db | `sleep infinity` · none | the machine where you build the repo and run the tools |
| **pqcota-demo-pg** | Debian 13 (postgres:16) · host arch | corp+db | `postgres` · :5432 | the central inventory DB (not needed on the single-host path) |
| **web-gw** | Ubuntu 24.04, OpenSSL **3.x** · host arch | corp | `sshd` · :22 | observed — the TLS/SSH client side |
| **pay-app** | Ubuntu 26.04, temurin 21 · host arch | corp | `sshd`·`java`·`pqc-echo` · :22 :8443 | observed — the JVM asset |
| **pay-db** | Ubuntu 20.04, OpenSSL **1.1.1** · host arch | corp+db | `sshd`·`payment-gw`·`api-gw` · :22 :4433 :4434 | observed — the legacy asset |

The node OS is chosen by `version` and `fork` in `topology.yaml` (3.x→24.04, 3.0→22.04, 1.1.1→20.04, libressl→alpine). Every arch matches the host. **Only the controller joins both segments** (SSH to every node). IPs are reassigned on every run, so nodes are referred to by segment name only.

**There is no collector on the nodes.** Only the workloads and demo helpers, and discovery leaves nothing behind:

```console
$ docker exec pay-db ls /usr/local/bin
node-entrypoint.sh  pqc-echo  pqcota-gen-traffic.sh  pqcota-observe.sh  ssl-apps.sh
```

`topo-gen` runs briefly at `0/6` of `up.sh` with `--rm` (it generates the compose file, `groups.ini`, and the SVG, then disappears).

## 5. Step by step

This follows the `▶ N/6` order that `demo.sh` prints. What the user sees is in the [README](README.en.md#what-you-see); this is what runs behind it and how.

### 5.1 Discovery (Ansible/SSH, all of it real)
1. **OpenSSL assets** come from `pqcota-nodescan`: the loaded libssl/libcrypto, found by scanning `/proc`.
2. **The JCA provider chain** comes from `pqcota-jvmscan`: **recon then attach**. It finds the running JVM (pay-app's CryptoApp) through `/proc`, attaches to that PID, and reads the real `Security.getProviders()`. That catches **the BouncyCastle CryptoApp registered at runtime with `addProvider`**. There is no static registration in java.security, so **a static scan cannot see it** (symmetrical with openssl's `/proc` scan; `detection=runtime-introspection`). If attach is impossible it falls back to a static probe, and whatever it could not observe is recorded as a gap.
3. **Communication edges** come from `pqcota-netcap`: TLS/SSH handshakes observed through AF_PACKET (`CAP_NET_RAW`) without decryption.

`pqcota-discover-view` (OSS) collates the results into **discovered assets plus the grade of the observed edges**:
- 🟢 **PQC/hybrid** (`X25519MLKEM768`, `sntrup761x25519`) · 🔴 **classical = quantum-vulnerable** (`x25519`, `ECDHE`) · ⚪ **unknown**
- For example: `web-gw → pay-app` 🟢 MLKEM · `web-gw → pay-db` 🔴 classical · SSH splits the same way (`→pay-app` 🟢 sntrup761 · `→pay-db` 🔴, since the legacy OS's OpenSSH has no PQC KEX)

### 5.2 The central inventory (endpoints, profiles, app attribution, history, change)
`pqcota-ingest` appends the retrieved results to an append-only history, and `pqcota-inventory` queries it:
- **▸ Machine header**: the **endpoint** `pqcota-hosts` upserted (name, ip:port, no secrets) plus the **profile** (display_name, env, role, owner: the CMDB declaration lane).
- **One machine registered under several names is reported**: the ingest output carries `⚠ duplicate: physical machine … → [pay-db web-gw]`, and that is not an error. The demo targets are containers on one host, so `pay-db` and `web-gw` share the same physical-machine fingerprint, and the platform states that instead of hiding it (TK-MACHINE). This is what you see in production when one machine has been registered under several names.
- **@App attribution**: which app each crypto asset belongs to (`app_keys`). pay-db's shared `libssl.so.1.1` is attributed to **both** `payment-gw` and `api-gw` (replacing that .so affects both apps).
- **History and change**: the same retrieved results are ingested a second time (the equivalent of "the next scan round" in production) to show `-history` (points of change plus observation count), `-snapshot` (assets plus observed edges), and `-diff` (`added`, `removed`, `changed`). Because it is the same observation, **"no change"** is the correct diff. The tool does not invent change that is not there. When a version really does change, the finding id is preserved and it shows up as **a `changed` on the same asset**.<br>Snapshots accumulate **only when the content changes**; repeated observation leaves only a lightweight observation record. Storage grows with the number of changes, while the evidence that "we scanned every time" is preserved.
- **Asset scope**: a node being registered does not make every asset inside it in-scope. Filtering out noise like `sshd` and the packaged python runtime by rule leaves **only the assets the apps actually use**. The number filtered out is always reported. **Exclusion is not absence** (§2.6).
- **Retention policy**: `pqcota-prune` is run as a dry run to show that **the newest snapshot per node is deleted by no policy**. Being destructive, it is separated from the query commands, and actual deletion happens only with `-apply`.

### 5.3 Provisioning (generate → apply → roll back)
The target node is chosen from the inventory among the nodes that have an openssl finding, preferring **the one whose shared .so is attributed to several apps** (the case where the affected range is clearest). In the default topology that is pay-db. If there is no such node, the step is skipped and says so (§2.5). A **finalized plan (FINALIZED)** is built for the chosen finding and `pqcota-provision` is run on it:
- **The §3.7 gate**: anything not FINALIZED is refused. **An L2 playbook is generated** (module staging + config fragment).
- **Before capture and a persisted rollback record**: the crypto state before the remediation (module, version) and the **affected apps (several, for a shared .so)** are recorded append-only.

Then the output is **actually applied**. "We generated it" only means something once you have seen it run:
- **Apply**: the generated playbook is run against the target node with `ansible-playbook`, passing the module sha256 gate along the way.
- **Verify**: that `/opt/pqcota/oqsprovider.so` and `/etc/pqcota/openssl-pqc.cnf` landed on the target, and that the config **references that staged path** (`module = /opt/pqcota/oqsprovider.so`).
- **Roll back**: removal via the `--rollback` playbook. Nothing ever overwrote the original config, so **removal alone restores the prior state**, and the demo checks that both files are gone.

L2 and L3 differ like this:
- **L2 only stages the fragment**. It never makes it referenced, so every output is fully reversible.
- **L3 adds activation and restart on top.** The commands are what the user wrote in the plan's `activation` hook. The activation point differs per environment, so the tool does not guess. The demo nodes manage services through `ssl-apps.sh`, so the hook points at that (the counterpart of a real systemd unit or an in-house startup script).
- What L3 shows in the demo is **hook ordering, the connection to the activation point, restart, and reversibility**. The legacy node's OpenSSL does not know this fragment's PQC group, so **the demo does not claim its capability changed**. The real remediation for that node is a fork replacement, and the playbook says in a comment that this cannot be deployed through config.

### 5.4 The optional step: a real provider
With `DEMO_REAL_PROVIDER=1`, a real oqsprovider (liboqs + oqs-provider) is built on the same base as the nodes, staged and activated, and `openssl list` measures whether the capability really appeared (ML-KEM KEMs 0 → 14, and back to 0 after rollback). It is off by default because the first build takes a few minutes.

The target is not the node of `6/6`. The only nodes with a place for a provider are **OpenSSL 3.0–3.4**: 1.1.1 has no notion of a provider, and on 3.5+ ML-KEM is native, so the remediation is CONFIG_ONLY rather than staging a provider (`pkg/provisioning/openssl.go`). A node observed in that band is chosen from the inventory; if there is none, the step is skipped (§2.5).

**On re-observation the inventory is unchanged.** The demo does not hide that; it states the reason alongside: OpenSSL has no path yet for observing the provider layer (it goes as far as libssl/libcrypto in `/proc/maps` and ELF strings; JCA sees the provider chain through attach, OpenSSL cannot), and a handshake requires both ends to know the negotiation, while the peer in this topology is 1.1.1. The design discussion is in [discovery design §2.1](../discovery/design.en.md). When it finishes, it undoes L3→L2 and leaves the node as it was.

## 6. The access secret boundary (§1.5)
Connection keys and accounts ride only in **the user's hosts.csv → the runtime-only `targets.ini` (owner-only `0600`)**. Only **the endpoint (node_id, name, ip, port)** is upserted into the pqcota inventory (Postgres); no secret is ingested (`0/6` verifies there are zero secrets in `pqcota_endpoint`). The connection key (`/work/id_demo`) and `targets.ini` exist **only inside the controller** and disappear with it.
