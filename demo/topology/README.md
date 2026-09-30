# demo/topology: the definition of the legacy environment the demo stands up

**The environment to be observed is defined by this one YAML.** Declare the number and kind of nodes, the OpenSSL versions, the JCA providers, the network
segments and the handshake edges, and the generator produces compose, groups and profiles, and runs
discovery → inventory → provisioning on top of them. There is no separate mode or flag: the demo has **this one path only**.

> **Not listed here: the two tool-side containers.** `pqcota-ctl` (the controller) and `pqcota-demo-pg` (the inventory store)
> are not written in the spec, and the generator **adds them automatically.** This YAML is where you write **what pqcota looks at**, not
> where you write pqcota itself (which also removes any chance of deleting them and breaking the demo).
> Both **attach to every segment**, because the controller has to reach every node over SSH. In a real environment the
> controller may be unable to reach an isolated segment, and that constraint is something the demo deliberately simplifies.

> **The only host prerequisite is still Docker.** The generator (`topogen`, Go) runs inside a container. You do not need Go on the host.

## Quick start

**You do not need to prepare anything.** If `topology.yaml` is missing, `up.sh` copies the sample and runs with that composition:

```bash
./demo/scripts/up.sh      # (copies the sample if missing) → generate → build → start → keys and IP map
./demo/scripts/demo.sh    # access prep → discovery → inventory → provisioning
./demo/scripts/down.sh    # clean up (--rmi removes the images too)
```

**To fit your own environment**, edit the copied `demo/topology/topology.yaml` and run `up.sh` again.

> **Only the sample (`topology.example.yaml`) is tracked.** The `topology.yaml` actually in use and the generated files
> (`demo/.generated/`) are gitignored, so you can edit freely, `git status` stays clean and `pull` does not conflict.
> To return to the default composition, delete `topology.yaml` and run `up.sh` again (the sample is copied again).

### What the default composition shows

The sample stands up three payment-service nodes and exposes several observation axes at once:

| Node | What | What it shows |
|---|---|---|
| `web-gw` | OpenSSL **3.x** client (corp) | a modern stack and the traffic source (**the SSH grade is decided by this node's client**) |
| `pay-app` | Java + **BC registered at runtime** (corp) | a provider a static scan cannot see and **only attach** catches |
| `pay-db` | OpenSSL **1.1.1** server, two apps (corp+db) | **legacy = quantum-vulnerable** · a shared `.so` **spanning several apps** (blast radius) · spans two segments |

Four edges split **modern from legacy on TLS and on SSH separately**:

| Edge | Grade | Why |
|---|---|---|
| `web-gw→pay-app` TLS | 🟢 X25519MLKEM768 | Go `crypto/tls` hybrid |
| `web-gw→pay-db` TLS | 🔴 x25519 | **OpenSSL 1.1.1** has no PQC group |
| `web-gw→pay-app` SSH | 🟢 sntrup761 | both sides run OpenSSH 9+ (the client offers it by default) |
| `web-gw→pay-db` SSH | 🔴 curve25519 | **OpenSSH 8.2** on the legacy OS has no PQC KEX |

The legacy node stays classical on **both TLS and SSH**. The grade is what the tool observed, not something assigned.

### What you edit is `topology.yaml` alone (`hosts.csv` is generated)

The demo shows several CSVs, but **the only file the user touches is `topology.yaml`**. The rest are all generated:

| File | What | Who makes it |
|---|---|---|
| **`topology.yaml`** | **What to stand up**: nodes, kinds, networks, edges | **the user (edits)** |
| `docker-compose.yml` · `groups.ini` · `profiles.csv` | containers, Ansible groups, CMDB profiles | the generator (`demo/.generated/`) |
| `hosts.csv` | **Where and how to connect**: node_id, IP, account, key | `up.sh` (the IPs are only fixed once the containers are up) |

In the product model, `hosts.csv` is **the file where the user writes their own hosts**. In the demo Docker assigns the IPs at run time, so `up.sh` plays that role instead. It is the same arrangement as the demo applying the playbook on the user's behalf. So **even with a custom topology you never write `hosts.csv` yourself.**

## The spec (`topology.yaml`)

```yaml
networks: [dmz, app, db]        # bridge segments (imitating network separation). A single net if omitted

nodes:
  - id: web-gw                  # container name and node_id (lowercase, digits, -)
    name: Payments Web Gateway  # the name shown in the inventory view
    kind: openssl               # openssl | java  ← only what pqcota really observes
    role: client                # openssl: client | server
    openssl: { fork: openssl, version: "3.0" }   # when fork=openssl, version → base image
    networks: [dmz, app]        # may span several segments
    profile: { env: production, role: web, owner: Platform team }

  - id: pay-app
    name: Payments App
    kind: java
    jca: { providers: [BC] }    # providers registered at runtime (SUN and SunJCE are JDK defaults). Whether BC is present decides the grade
    networks: [app]

  - id: pay-db
    name: Payments DB
    kind: openssl
    role: server
    openssl: { fork: openssl, version: "1.1.1" }  # legacy = quantum-vulnerable
    apps: [payment-gw, api-gw]  # (openssl server) several apps load one libssl → the shared .so spans several apps

edges:                          # handshakes to observe → grade (🟢 PQC / 🔴 classical)
  - { from: web-gw, to: pay-app, proto: pqc, port: 8443 }
  - { from: web-gw, to: pay-db,  proto: ssl, port: 4433 }
```

### The axes you can adjust

| Axis | Values | What it shows |
|---|---|---|
| **node kind** | `openssl` · `java` | only the runtimes pqcota observes |
| **openssl fork** | `openssl` · `libressl` | discovery's **fork detection** (the same soname, a different fork) |
| **openssl version** | `1.1.1` · `3.0` · `3` | the OpenSSL version of the base image (legacy ↔ modern, version detection) |
| **jca providers** | e.g. `[BC]` · `[]` | whether BC is present → a JCA grade difference (attach catches the dynamic registration) |
| **networks** | any list of segments | network separation through several bridges; a node spans several segments |
| **apps** (openssl server) | a list of app names | several apps load a shared `.so` → **spanning several apps · blast radius** |
| **edges** | `pqc` · `ssl` · `ssh` | handshake observation → 🟢/🔴 grade |

### Server and traffic rules (derived automatically)

- A node that is the `to` of a `pqc` edge → a PQC TLS server (:8443) is started.
- An openssl node that is the `to` of an `ssl` edge or has `role: server` → a classical s_server is started.
- A `from` node → traffic is generated toward those edges (it fills the observation window).

## Honesty boundaries · known limits

- **A runtime that cannot be observed cannot be added.** Kinds such as `.NET` and `Go` are rejected. Nothing pretends to be there.
- **A fork without an s_server** (BoringSSL, AWS-LC) cannot be started as a demo server node, so it is **rejected with a clear error**.
- **A topology with no openssl finding at all** skips the provisioning demonstration (there is nothing to act on).
- **Edge observation happens on the first segment (eth0) of the traffic-source node** (netcap watches one interface). You may use several
  segments, but **the pair you want to observe has to reach each other on the source node's first segment** to be caught. An edge in an isolated segment the source
  cannot reach is not captured (the example spans pay-db across corp and db and observes from corp).
- **LibreSSL disguises its version as OpenSSL** (the `OPENSSL_VERSION_NUMBER` compatibility value). So a `fork: libressl`
  node really loads LibreSSL, but the collector **reports OpenSSL 3.1.x**. It is an honest limit where the same-soname problem
  is compounded by version disguise. This axis is supported but left out of the example.

The generated files land in `demo/.generated/` (gitignored, the single place for the repo's build outputs). Open them and you see exactly what was made.
