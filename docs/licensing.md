English · [한국어](licensing.ko.md)

# License notes (third-party & project licensing)


**What this document is**: an accounting of **every license `pqcota` uses internally** as of today, organized by how it is consumed. The consumption form matters because license obligations change completely depending on whether something is linked into a distributed binary.

> This is the **licensing axis**: what each dependency is licensed under, and how it is consumed.

> **§ notation**: a `§N` in this document is a section number of **this document**.

> ⚠️ **Disclaimer**: this document is not legal advice. Before distribution, due diligence by counsel specializing in OSS licensing (GPL vs AGPL, or-later, the exact per-version conditions) is required.

---

## 0. The conclusion in one line

**Every third party linked or bundled into a pqcota product binary is permissively licensed (Apache-2.0 / MIT / BSD-3).**
Copyleft (GPL and friends) exists **only in tools that run as separate processes** (Ansible, the JDK in the demo environment); none of it is linked into or bundled with a pqcota binary (§5). What that means under each tool's license is a separate review, not something this document settles.

---

## 1. pqcota's own licensing

**This repo (`pqcota`) is Apache-2.0.** Every third party linked or bundled into the distributed outputs (Go binaries, the Java sidecar) is permissively licensed (the §2 table).

`pqcota-cbom-ingest` does not run CBOMkit or other GPL-family generators; it only reads the CycloneDX files they produce (delegated CBOM intake design). What that means under those tools' licenses is a separate review (below).

## 2. Runtime dependencies linked into the build output (Go)

Dependencies **compiled and linked** into the static Go binaries built from this repo (collectors, CLIs). **All permissive.**

| Module | Version | License | Note |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | v5.10.0 | **MIT** | Postgres driver (persistence) |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT | pgx indirect |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT | pgx indirect |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT | pgx connection pool |
| `google.golang.org/grpc` | v1.83.2 | **Apache-2.0** | intake contract transport |
| `google.golang.org/protobuf` | v1.36.11 | **BSD-3-Clause** | contract serialization (protojson included) |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260526163538-3dc84a4a5aaa | Apache-2.0 | grpc indirect |
| `golang.org/x/sys` | v0.47.0 | **BSD-3-Clause** | AF_PACKET (network collector) |
| `golang.org/x/net` | v0.58.0 | BSD-3-Clause | grpc indirect |
| `golang.org/x/sync` | v0.22.0 | BSD-3-Clause | indirect |
| `golang.org/x/text` | v0.41.0 | BSD-3-Clause | indirect |

`gopkg.in/yaml.v3` (MIT) is not in the list above — it is used only by the demo topology generator and by tests, so it is not linked into the collectors or CLIs.

**Summary**: **no** copyleft is linked. Apache-2.0, MIT and BSD-3 are generally treated as compatible with an Apache-2.0 distribution, provided their notices are kept (below).
(A distribution has to keep the notices and conditions each license requires: for MIT, its copyright and permission notice; for BSD-3-Clause, its copyright notice, conditions and disclaimer, and it limits using contributors' names to endorse a product. This repo collects them in `THIRD-PARTY-NOTICES`; that file name is not itself a requirement.)

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

`demo/` operates through containers and separate executables. Nothing there is **statically or dynamically linked** into a pqcota binary; it all runs beyond an SSH, subprocess, or container process boundary → GPL/copyleft tools there stay separate programs; what their licenses require is reviewed separately (§5).

| Component | Version | License | Consumption form |
|---|---|---|---|
| BouncyCastle `bcprov-jdk18on` | 1.85 | **Bouncy Castle Licence** (MIT X11 family, permissive) | pay-app's JCA provider (a separate JVM). Permissive; bundling it would mean keeping its notice |
| Eclipse Temurin (OpenJDK) | 21 | **GPLv2 + Classpath Exception** | pay-app's runtime (separate container and process). GPLv2 with the Classpath Exception applies; how far the exception covers a given use is reviewed separately |
| OpenSSL | 3.x (Ubuntu) | **Apache-2.0** | TLS for web-gw/pay-db (separate processes) |
| OpenSSH (server/client) | 9.x | **BSD family** (+ some public domain) | sshd and ssh (Ansible transport, and the subject of SSH edge observation) |
| Ansible | (distro) | **GPL-3.0-or-later** | a **standalone executable** run on the controller. Not linked with pqcota (an orchestration tool) |
| Graphviz (`dot`) | (distro) | **CPL-1.0** (Common Public License) | topology SVG rendering (separate process). The output (SVG) is data |
| Ubuntu 24.04 base image | — | an aggregate (many packages, mostly GPL/LGPL/MIT/BSD) | container base |
| `golang:1.26` base image | — | an aggregate (Go = BSD-3 + a Debian base) | builder stage |

> **How the GPL tools (Ansible, Temurin) are treated**: these are separate programs pqcota **invokes**, not link targets.
> Ansible is the orchestrator that runs playbooks; Temurin is just the runtime on the target node. Whether a use of them
> creates obligations under the GPL depends on more than how the programs communicate, so that is left to a separate review.
> With the build procedure provided today, the user downloads these tools when building the image; pqcota does not ship them. If a built container image is distributed, the redistribution terms of what it contains are checked separately.

---

## 5. Keeping GPL code separate — what enforces it

**GPL code is kept apart from pqcota's code and distributions by structure.** Three principles (technical measures, not a legal conclusion):

1. **Process separation** — a GPL component is invoked as an independent binary, never linked as a library.
2. **A standard data boundary** — inter-process exchange happens only through CycloneDX CBOM (a standard). Core internal APIs never cross it.
3. **Distribution separation** — GPL code is never bundled, statically linked, or vendored into this repo or its distributions.

The same boundary applies even if we write our own GPL collector. The exact license per component (GPL vs AGPL, or-later) and the AGPL implications for a SaaS deployment **require due diligence** — this document is not legal advice.

What enforces those principles is the table below.

| Principle | What enforces it |
|---|---|
| Separate process | the intake contract in `contracts/.../collector.proto` — a GPL collector, if one is added, would stand behind gRPC/CLI |
| Standard data only | that contract carries nothing but CycloneDX + Envelope. Core internal types never cross it |
| Distribution separation | GPL code, if ever used, goes in a **separate repo** and not in `go.mod` (no such repo exists today). No automated license scan runs in CI today, so this is kept by review |

Ansible (GPL-3) and Temurin (GPLv2+CE) in `demo/` likewise run as separate processes outside the same boundary.

**This repo is published under Apache-2.0.** The point is to open up observability so the community can add collectors, and the reference collectors are OSS for the same reason (including the JVM introspection, which nothing off the shelf covers).

| Category | License | What it contains |
|---|---|---|
| **This repo** | **Apache-2.0** | contracts, normalization, inventory, provisioning generation + reference collectors |
| **PQC provider libraries** | varies per asset | BouncyCastle (permissive) · BC-FJA (FIPS, separate agreement) — procured by the user |

**Where the license is stated.** Each collector states its license in `CollectorCapabilities.license` (and `Envelope.collector_license`), so a caller can show whether a backend means *"a GPL component installed separately"* or *"the reference = Apache-2.0, included"*.

---

## Appendix: consumption forms at a glance

| Consumption form | Copyleft present? | How it is kept separate | Examples |
|---|---|---|---|
| Linked into the build output (§2) | ❌ none | not applicable | pgx (MIT), grpc (Apache), protobuf (BSD) |
| Build time (§3) | ❌ none | not applicable | buf, protoc-gen-* |
| Separate processes in the demo (§4) | ✅ yes (Ansible, Temurin) | separate processes (license terms reviewed separately) | Ansible (GPL-3), Temurin (GPLv2+CE) |
