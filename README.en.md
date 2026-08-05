English · [한국어](README.md)

# pqcota

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](README.md) is authoritative.

The **public core — Community Edition** of a PQC migration management platform (OSS, [Apache-2.0](LICENSE)). It handles the PQC migration of legacy crypto runtimes (OpenSSL · Java JCE/JCA) across three stages: **Discovery → Inventory → Provisioning**.

**Name** — *pqcota* (pronounced **P-cota**) = **PQC** (post-quantum cryptography) + **Orchestra** (-ota). **This software is a *player* in the orchestra, not the maestro.** The **maestro who decides what to migrate and when is the user** wielding the tool; pqcota plays its own part (observe · normalize · generate) precisely.

---

## What it does — three stages

| Stage | What it does | Output |
|---|---|---|
| ① **[Discovery](discovery/README.md)** | **Observes which cryptography is in use** on running systems — loaded libraries, JVM provider chains, algorithms negotiated in the handshake | per-node observations (canonical CBOM) |
| ② **[Inventory](inventory/README.md)** | **Ties each observation to the node and the apps it belongs to, and accumulates them** — machine metadata, diffs between snapshots | a central, append-only inventory |
| ③ **[Provisioning](provisioning/README.md)** | **Generates the PQC migration artifacts** from a finalized plan — config fragments, apply/rollback Ansible playbooks (L1/L2/L3), rollback basis | playbooks + before records |

## Try it — demo

**With just Docker**, run the whole scope at once — access prep → discovery → inventory →
provisioning (generate, apply, roll back), against nodes it stands up as containers.

```bash
./demo/scripts/up.sh && ./demo/scripts/demo.sh   # tear down: ./demo/scripts/down.sh
```

Setup, expected output, and how to point it at your own hosts → **[demo/](demo/README.md)** (Korean)

---

## Requirements

**To build**
- Go 1.26.4+
- buf (+`protoc-gen-go`·`protoc-gen-go-grpc`)
- JDK 11+ — **optional**, only to build the JVM attach sidecar; without it that step is skipped

**To run**
- Multiple nodes — Ansible on the controller, SSH access to the targets
- A single node — nothing to install; run the binary on that node directly (`pqcota-netcap` needs `CAP_NET_RAW`) → [discovery/cmd](discovery/cmd/README.md)

## Build

pqcota consists of one **central controller node** and the **target nodes** it reaches over
Ansible/SSH. **You build on the controller** — both the CLIs you run there and the collectors you
ship to the target nodes are produced here.

**① Generate the contract code** — Go code is generated from the contracts (`contracts/*.proto`).
`make tools` installs the generator plugins (`protoc-gen-go`, `-grpc`) and `make generate` does the
conversion. The generated `gen/` is gitignored.

```bash
make tools && make generate     # contracts/*.proto → gen/
```

**② The CLIs you run on the controller** — ingest and query observations, generate playbooks.

```bash
go build -o bin/ ./discovery/cmd/... ./inventory/cmd/... ./provisioning/cmd/...
```

**③ The collectors that go on the target nodes** — just three, built statically **for the node's arch**
→ [deployment design](discovery/collector_배포_설계.md) (Korean).

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux-amd64/ \
  ./discovery/cmd/pqcota-nodescan ./discovery/cmd/pqcota-netcap ./discovery/cmd/pqcota-jvmscan

make build-jar                  # only if you have JVM nodes: attach sidecar → build/collector.jar
```

The collectors are **Linux-only**, so `GOOS=linux` and `CGO_ENABLED=0` (static linking — distro/libc
agnostic) are fixed. The only thing you change per node is `GOARCH`; for the accepted values see the
[Go documentation](https://go.dev/doc/install/source#environment).

Privileges and environment variables for running the collectors on a node → [discovery/cmd](discovery/cmd/README.md#권한--환경변수).

Contributing to the repo (tests, gates, contract changes) → [CONTRIBUTING](CONTRIBUTING.en.md).

## Stack

- **Go** — every collector and CLI; `CGO_ENABLED=0` static single binaries
- **Java** — only the JVM attach sidecar (that observation is possible only from inside the JVM)
- **Protobuf/gRPC** — the contracts that join the stages ([`contracts/`](contracts/))
- **Postgres** — only when accumulating and querying many nodes over time; not used for single-node observation

## Supported scope

**Observation**

| What is observed | Target | Why |
|---|---|---|
| OpenSSL assets · communication edges | **Linux** (amd64·arm64) | depends on `/proc`, ELF, AF_PACKET |
| JVM provider chains | **Java 8+**, wherever a JVM runs | attach is OS-independent (verified on Linux) |

**Migration (provisioning)** — what is generated depends on the remediation kind in the plan.

| Runtime | Situation | What is generated |
|---|---|---|
| **OpenSSL** | 3.5+ (native PQC) | a config fragment only — the legacy runtime is untouched |
| | 3.0–3.4 | provider module staging + a config fragment referencing it. You supply the module |
| | 1.1.1 and older | **nothing** — a fork replacement is required, so it is marked as a manual step |
| **JCA** (Java) | **JDK 24+** (native PQC) | a `java.security` fragment only. It keeps a classical group alongside — released JDKs still do not negotiate the hybrid TLS group (measured up to 25) |
| | **JDK 8+** (provider injection) | provider JAR staging + a `java.security` registration fragment. Staging the JAR alone does not load it, so activation is a separate step |
| | older (EOL) | **nothing** — a JDK upgrade is required, so it is marked as a manual step |

Application goes through Ansible playbooks. The generated playbook assumes POSIX paths and modules (`ansible.builtin.copy`, `/opt/pqcota`), so the target nodes are **Linux** — Ansible itself also drives Windows, but this output does not yet.

Windows (CNG) is on the [roadmap](RELEASE_NOTES.en.md) — v0.1.0 reserves the schema in the contract and the collector comes in a later release.

---

## Status · version

In development — before the first release (v0.1.0). **Per-version goals and results** are in the [release notes](RELEASE_NOTES.en.md).

## License

- **Apache-2.0** — full text in [LICENSE](LICENSE)
- Dependency licensing → [License notes](docs/라이선스_정리.md) (Korean)
- Third-party notices → [THIRD-PARTY-NOTICES](THIRD-PARTY-NOTICES.en.md)
