English · [한국어](licensing.md)

# License notes (third-party & project licensing)

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](licensing.md) is authoritative. In particular, the dependency version column in §2 is the one CI checks against `go list -deps`, on the Korean original only — if a version here disagrees, the Korean table is the current one.

**What this document is**: an accounting of **every license `pqcota` uses internally** as of today, organized by how it is consumed. The consumption form matters because license obligations change completely depending on whether something is linked into a distributed binary.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](regulation.en.md).

> ⚠️ **Disclaimer**: this document is not legal advice. Before distribution, due diligence by counsel specializing in OSS licensing (GPL vs AGPL, or-later, the exact per-version conditions) is required.

---

## 0. The conclusion in one line

**Every third party linked or bundled into a pqcota product binary is permissively licensed (Apache-2.0 / MIT / BSD-3).**
Copyleft (GPL and friends) exists **only in tools that run as separate processes** (Ansible, the JDK in the demo environment), and the process boundary blocks contagion (§5). So **this repo's Apache-2.0 distribution carries no copyleft contamination.**

---

## 1. pqcota's own licensing

**This repo (`pqcota`) is Apache-2.0.** Every third party linked or bundled into the distributed outputs (Go binaries, the Java sidecar) is permissively licensed (the §2 table).

GPL-family tools (CBOMkit and friends) are **neither linked nor executed** — we only receive the CycloneDX files they produce (`pqcota-cbom-ingest`). That is a file exchange, so there is no contagion path → [delegated CBOM intake design](../inventory/cbom-intake.md) (Korean).

## 2. Runtime dependencies linked into the build output (Go)

Dependencies **compiled and linked** into the static Go binaries built from this repo (collectors, CLIs). **All permissive.**

| Module | Version | License | Note |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | v5.10.0 | **MIT** | Postgres driver (persistence) |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT | pgx indirect |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT | pgx indirect |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT | pgx connection pool |
| `google.golang.org/grpc` | v1.82.0 | **Apache-2.0** | intake contract transport |
| `google.golang.org/protobuf` | v1.36.11 | **BSD-3-Clause** | contract serialization (protojson included) |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260414002931-afd174a4e478 | Apache-2.0 | grpc indirect |
| `golang.org/x/sys` | v0.43.0 | **BSD-3-Clause** | AF_PACKET (network collector) |
| `golang.org/x/net` | v0.53.0 | BSD-3-Clause | grpc indirect |
| `golang.org/x/sync` | v0.20.0 | BSD-3-Clause | indirect |
| `golang.org/x/text` | v0.36.0 | BSD-3-Clause | indirect |

`gopkg.in/yaml.v3` (MIT) is not in the list above — it is used only by the demo topology generator and by tests, so it is not linked into the collectors or CLIs.

**Summary**: **no** copyleft is linked. Apache-2.0, MIT, and BSD-3 are mutually compatible and pose no problem for an Apache-2.0 distribution.
(BSD-3 and MIT only require preserving the copyright notice → shipping `THIRD-PARTY-NOTICES` with the distribution is recommended, §6.)

---

## 3. Build-time tools (not linked into the output)

Used only for code generation and compilation; **not linked into the resulting binaries**.

| Tool | License | Purpose |
|---|---|---|
| Go toolchain (`golang:1.26`) | BSD-3-Clause (Go) | compilation |
| `buf` (bufbuild/buf) | Apache-2.0 | proto code generation (`buf generate`) |
| `protoc-gen-go` | BSD-3-Clause | Go message generation |
| `protoc-gen-go-grpc` | Apache-2.0 | gRPC stub generation |

---

## 4. Demo environment components (`demo/`) — separate processes/containers, not linked

> The components below run in the discovery demo (`demo/`) over SSH, as subprocesses, or in containers, and are not linked into any pqcota binary.

`demo/` operates through containers and separate executables. Nothing there is **statically or dynamically linked** into a pqcota binary; it all runs beyond an SSH, subprocess, or container process boundary → **even where GPL/copyleft is present, there is no contagion** (the same principle as above).

| Component | Version | License | Consumption form |
|---|---|---|---|
| BouncyCastle `bcprov-jdk18on` | 1.85 | **Bouncy Castle Licence** (MIT X11 family, permissive) | pay-app's JCA provider (a separate JVM). Being permissive, bundling would also be allowed (provisioning design §4.2) |
| Eclipse Temurin (OpenJDK) | 21 | **GPLv2 + Classpath Exception** | pay-app's runtime (separate container and process). With the CPE, Java apps are not infected by GPL |
| OpenSSL | 3.x (Ubuntu) | **Apache-2.0** | TLS for web-gw/pay-db (separate processes) |
| OpenSSH (server/client) | 9.x | **BSD family** (+ some public domain) | sshd and ssh (Ansible transport, and the subject of SSH edge observation) |
| Ansible | (distro) | **GPL-3.0-or-later** | a **standalone executable** run on the controller. Not linked with pqcota (an orchestration tool) |
| Graphviz (`dot`) | (distro) | **CPL-1.0** (Common Public License) | topology SVG rendering (separate process). The output (SVG) is data |
| Ubuntu 24.04 base image | — | an aggregate (many packages, mostly GPL/LGPL/MIT/BSD) | container base |
| `golang:1.26` base image | — | an aggregate (Go = BSD-3 + a Debian base) | builder stage |

> **How the GPL tools (Ansible, Temurin) are treated**: these are separate programs pqcota **invokes**, not link targets.
> Ansible is the orchestrator that runs playbooks; Temurin is just the runtime on the target node. GPL propagates through
> "derivation from and linking against a work", so it does not apply to a relationship that is only a process call.
> When the demo is distributed or redistributed, these tools are **installed by the user** (downloaded at image build), so pqcota does not redistribute them.

---

## 5. Copyleft isolation — what enforces it

**GPL copyleft contagion is blocked structurally.** Three principles:

1. **Process separation** — a GPL component is invoked as an independent binary, never linked as a library.
2. **A standard data boundary** — inter-process exchange happens only through CycloneDX CBOM (a standard). Core internal APIs never cross it.
3. **Distribution separation** — GPL code is never bundled, statically linked, or vendored into this repo or its distributions.

The same boundary applies even if we write our own GPL collector. The exact license per component (GPL vs AGPL, or-later) and the AGPL implications for a SaaS deployment **require due diligence** — this document is not legal advice.

What enforces those principles is the table below.

| Principle | What enforces it |
|---|---|
| Separate process | the intake contract in `contracts/.../collector.proto` (§1.6) — a GPL collector stands behind gRPC/CLI |
| Standard data only | that contract carries nothing but CycloneDX + Envelope. Core internal types never cross it |
| Distribution separation | the GPL adapter is a **separate repo** and is not in `go.mod`. The CI license scanner blocks a cross dependency |

Ansible (GPL-3) and Temurin (GPLv2+CE) in `demo/` likewise run as separate processes outside the same boundary.

**This repo is published under Apache-2.0.** The point is to open up observability so the community can add collectors, and the reference collectors are OSS for the same reason (including the JVM introspection that §2.2 calls an "own-implementation gap").

| Category | License | What it contains |
|---|---|---|
| **This repo** | **Apache-2.0** | contracts, normalization, inventory, provisioning generation + reference collectors |
| **GPL adapter** (optional) | GPL-3.0, a separate repo | CipherIQ/CBOMkit subprocess wrappers |
| **PQC provider libraries** | varies per asset | BouncyCastle (permissive) · BC-FJA (FIPS, separate agreement) — procured by the user |

**The implications are shown at the point of choice.** The collector selection UI uses `CollectorCapabilities.license` to state whether a backend means *"a GPL component installed separately"* or *"the reference = Apache-2.0, included"*.

---

## Appendix: consumption forms at a glance

| Consumption form | Copyleft present? | Contagion risk | Examples |
|---|---|---|---|
| Linked into the build output (§2) | ❌ none | none | pgx (MIT), grpc (Apache), protobuf (BSD) |
| Build time (§3) | ❌ none | none | buf, protoc-gen-* |
| Separate processes in the demo (§4) | ✅ yes (Ansible, Temurin) | **blocked by isolation** | Ansible (GPL-3), Temurin (GPLv2+CE) |
