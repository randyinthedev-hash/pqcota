# Contributing (CONTRIBUTING)


> **What will not be broken** — contract, signature, Go API, DB schema, and mixed versions, written out
> as five distinct faces: [compatibility policy](docs/compatibility.md) Read it before changing
> any of them.

For developers who want to **fork·extend·contribute** to pqcota. Users who just want to *try* the platform should see the root [README](README.md) and [demo/](demo/).

## Prerequisites

You need **Go 1.26.4+** (below the `go` directive in `go.mod` the toolchain refuses to build) and
**buf** + `protoc-gen-go`·`protoc-gen-go-grpc`. Add **JDK 11+** if you touch the JVM collector (optional — without it only the sidecar build is skipped).
Once the repo builds, the [examples](examples/) just run (only the JVM and OpenSSL integration
examples need **Docker** as well).

This document covers **contributing to the repo**. If you only use it, building and running are covered by the [root README](README.md#build).

### Which OS can you build on

| OS | `go build` · `go test` | `make` (gates) | Node binaries |
|---|---|---|---|
| **Linux** | ✅ | ✅ | ✅ directly |
| **macOS** (amd64·arm64) | ✅ | ✅ | ✅ cross-compiled |
| **Windows** (amd64·arm64) | ✅ | needs a POSIX shell → **WSL** | ✅ cross-compiled |

Linux-only code (`/proc`, AF_PACKET, attach) sits behind `//go:build linux`, with a refusing stub on
other platforms. So on macOS and Windows that code is excluded from compilation and breaking it would
still pass a host build — which is why `make build` also cross-compiles **linux/amd64 and windows/amd64**. Windows is included because the CNG collector is built and verified there.

## Development loop

The build procedure is the same as [root README · Build](README.md#build). What you additionally use when contributing are the gates and tests:

```bash
make            # every gate (what it runs is the Makefile's all target)
go test ./...   # unit
```

`make build` **leaves no artifacts** — it only checks that the host, linux/amd64, and windows/amd64 cross-builds
compile (so Linux-only files are covered). Build the binaries you use with `-o`, as the root README does. `make build-jar` warns and skips without a JDK, so contributors touching only Go can run `make`
without one. Tests run without a real JVM.

> **If you see `no required module provides package .../gen/pqcota/...`**, you skipped generation.
> The `go get github.com/randyinthedev-hash/pqcota/gen/...` that Go suggests alongside it is **not the fix** —
> that is not a fetchable module but code this repo generates. Run `make generate` first.

If you changed a contract, run `make lint` (buf lint) and check backward compatibility:

```bash
make breaking                  # against the last release tag — does it break a contract already shipped (what CI runs)
make breaking AGAINST=main     # compare the branch you are working on against main
```

While there is no release tag (before v0.1.0) there is no baseline, so the first one skips and says so in the log.

Also read [the ripple checklist for contract changes](contracts/README.md) (signature coverage, change detection).

## Code layout — top level = kind, stage = inside it

| Top level | What |
|---|---|
| `contracts/` | Contract SSOT (protobuf). The namespace *is* the stage: `pqcota.{common,discovery,inventory,provisioning}.v1` |
| `gen/` | proto-generated code — **committed** (so consumers can use it with `go get` alone) |
| `pkg/` | Library logic — stage groups `discovery`·`inventory`·`provisioning` + shared `kernel` (registry·posture·scope·machineid·sign)·`cbom` |
| `discovery/` · `inventory/` · `provisioning/` | **Execution entry points** (per stage) — each with `cmd/` (scanner·driver·query·generate); `discovery/` also has `collectors/` (reference collectors) |
| `examples/` | **Per-stage runnable examples** — sample inputs + `run.sh` to run each cmd with minimal setup |
| `demo/` | Docker end-to-end demo (access prep → discovery → inventory → provisioning) |
| `tools/` | Repo tooling — `checkdocs` (the docs gate; `make check-docs` builds and runs it) |

That is, **`pkg/`·`contracts/` split by stage, and so do the top-level execution folders**. To **actually run the commands, use [`examples/`](examples/)** (each stage's `run.sh`); for what each command is, see each `<stage>/cmd/README` ([discovery](discovery/cmd/README.md)·[inventory](inventory/cmd/README.md)·[provisioning](provisioning/cmd/README.md)).

## Contract-first

- To change a type/enum, **edit `contracts/*.proto` and `make generate`**. Do not touch `gen/` directly.
- Derived values like `evidence_strength`·`pqc_readiness` are **filled by the core, not the collector** (the rules live in one place so results can be recomputed). Details: [contracts/README](contracts/README.md).
- A controlled-vocabulary `*_UNSPECIFIED = 0` means "unknown" — don't leave it blank/missing.

## Collector extension — the contract is the seam

The reference collectors (openssl·jvm·network) are just three examples of ways to observe. **When there's more to observe, add a collector** — without touching the core. The single seam is the `CollectionResult` contract (canonical CycloneDX + `pqcota:` properties).

- A collector's job ends at **observe → emit `CollectionResult`**. It does **not** fill derived values like `evidence_strength`·`pqc_readiness` — the core derives those from the contract input (the rules live in one place so results can be recomputed).
- Match the contract and the language is free (the references themselves are Go·Java polyglot). Tool-specific enrichment rides on the standard `properties` extension keys ([contracts/README](contracts/README.md)).
- Each reference collector's design goals, boundary, and honesty rules are in [`discovery/collectors/<name>/README`](discovery/collectors) — a new collector follows the same shape (observe only · unseen = gap · no guessing).

> **The provisioning generator is not yet such a plugin seam** — the plan (`plan.proto`) is a public contract, but the generator itself is internal logic. To avoid confusion, only the collector side is presented as an extension point.

## Extending with a new crypto runtime

The platform targets not one library but **a runtime that has crypto providers**, and the process is the same for every runtime. Four things vary per runtime: how discovery collects, the version and provider axis schema, the remediation branch, and the provisioning substrate. Every finding and asset carries `crypto_runtime` as a first-class field, and that field selects the branch in each stage.

A new runtime is **introduced in stages**, not all at once: contract vocabulary first, then observation, then remediation. Open an issue describing all four before writing code (see [Issues · proposals](#issues--proposals)).

## Coding guidelines

This repo enforces **honesty and determinism in the code itself**. Below are the conventions — not generic Go style, only **what is specifically upheld here**.

**Formatting & checks.** Format with `gofmt` (`go fmt ./...`). `make` (full) runs every gate, so all of them must pass before a PR. The gates and what each one blocks are listed in the `Makefile` and `.github/workflows/ci.yml`. Follow standard Go idioms, but use the contract's vocabulary for domain terms (`finding` · `app_key` · `crypto_runtime`).

**Comments explain "why".** *What* the code does, the code says — comments say *why it's done this way* and why the rejected alternative is wrong. This is why comments here run long. Example: `// exclusion is not "absence" — silently dropping a policy-excluded asset makes the inventory lie`. Existing comments are written in Korean and cite section numbers (`§`) of a process regulation that is not part of this repository; treat those numbers as opaque labels.

**Enforce honesty in code** — it must hold at runtime, not just in docs:
- **unknown is first-class** — an undeterminable value is not a blank but `*_UNSPECIFIED` / an explicit "unknown". A controlled-vocabulary enum's `0` is always unknown.
- **gap ≠ absence** — never silently drop what wasn't seen or what a policy excluded. **Count it, return it, and report it** (excluded counts, the completeness map, the `-diff` reverse-order warning, etc.).
- **no guessing or judgment** — don't fabricate what wasn't observed. If a diff is "no change", that *is* the answer.

**Derived values must be recomputable from the source.** Derivations like `evidence_strength` are produced by the core from the source (`detection_method`), not by the collector — the rule lives in one place (`pkg/inventory/normalize`) so it reproduces. **No wall-clock or randomness in signing/canonicalization paths** (same input → same bytes). Content fingerprints exclude volatile fields (observation count, `last_seen`).

**Keep logic pure and testable.** Separate parsing/decision logic from I/O so it unit-tests without the real thing (process, DB, network) — e.g. `ParseProcMaps(reader)` runs without `/proc`. **Tests pin not just behavior but "why this invariant holds"** (a regression test comments the essence of the bug).

**Don't depend on external tools.** Parse `/proc`·ELF directly in Go instead of shelling out to `ldd`·`lsof`·`ss`·`readelf` (minimal image/footprint). Release binaries are `CGO_ENABLED=0` static builds. Tag code that touches OS primitives with `//go:build linux`, and split pure helpers out as OS-agnostic.

**Change the contract, change what rides on it.** Every field a collector asserts must be covered by `sign.Canonical` (no signing blind spots). A oneof arm uses a field number **unused across the whole message** (a oneof shares the message's number space). Full checklist: [contracts/README](contracts/README.md).

## Testing

```bash
go test ./...                                              # unit
bash discovery/collectors/openssl/integration/run.sh      # openssl collector real integration (Docker, SD-1·SD-3·SD-4)
./demo/scripts/up.sh && ./demo/scripts/demo.sh            # end-to-end discovery demo
```

Write the test first (TDD): each stage's tests pin the acceptance criteria.

## Language

**Documents are English; comments are Korean; whatever the code emits is English.** The dividing line is not "who reads it" but **how far it travels**.

| | Language | Why |
|---|---|---|
| Documents (`*.md`) | **English** (canonical); a Korean translation, when one exists, is `*.ko.md` | English reaches the widest audience; only the documents that need it are translated |
| **Comments** | **Korean** (existing) | they reach only whoever reads the code; they never leave the program |
| **Console output** (stdout, stderr, flag help) | **English** | it ends up in logs, gets pasted into issues, and is read by strangers |
| **Strings carried by the contract** (`Completeness.Note`, `Attribution.Reason`, remediation notes) | **English** | they are stored and **travel out through the contract**, so the language becomes part of the contract |
| **Error values** (`errors.New`, `fmt.Errorf`) | **English** | where they flow is the caller's decision, not ours |
| **Test failure messages** | **English** | they land in CI logs |

Everything except comments is something the program **emits**, and what you emit does not get to choose its reader. If Korean should appear on a screen, that is a job for the view, not a reason to put Korean into observation data.

**When you edit an English document that has a `*.ko.md` translation, update the translation in the same change** (or say in the PR that it is stale). Translation contributions are welcome.

### When you write "it does not", attach the reason on the spot

This repo does not hide its limits, so negative sentences are common: *it does not always work · it is not settled · we do not build it*. But **when the reason arrives two sentences later, the reader fills the gap with a guess.** A judgement and its basis belong together.

| Not this | This |
|---|---|
| It goes as far as the app, but **it does not always work**. (…two sentences of explanation later…) | It goes as far as the app, but **only if the socket is still alive at lookup time** |
| The machine **does not settle it**. | The machine does not settle it. **Whether it is live or stale is something only a person knows** |
| An admin UI **is not built**. | An admin UI is not built. **Once there is a screen, "let's approve here too" is the next step** |

## Issues · proposals

**Bugs, questions, and proposals go in issues.** There's nothing to hide, and an open discussion stays for the next person. Issues and PRs may be written in English or Korean. The only thing that must stay private is **something that could expose users to attack if known before a fix** — that path is in [SECURITY](SECURITY.md).

Including this with a bug report speeds up reproduction:

- what you expected and what happened
- the command you ran and its output (redact sensitive values)
- environment — OS and Go version (on Linux, `uname -r` and the distro). The Linux collectors assume **kernel 3.2 or later**
- for observation issues, the target runtime (OpenSSL version, JDK distribution)

**For large changes, open an issue before a PR.** Contracts (`contracts/`) are the single source of truth here, so anything touching the schema or a boundary needs design agreement first — discovering a disagreement after the code is written costs us both.

## Design first

Before adding a feature, read the per-stage READMEs ([discovery](discovery/README.md) · [inventory](inventory/README.md) · [provisioning](provisioning/README.md)) and [contracts/](contracts/README.md), and open an issue if the change touches a boundary.
