English · [한국어](CONTRIBUTING.md)

# Contributing (CONTRIBUTING)

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](docs/PQC플랫폼_단계별_프로세스규정.md) (Korean).

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](CONTRIBUTING.md) is authoritative.

For developers who want to **fork·extend·contribute** to pqcota. Users who just want to *try* the platform should see the root [README](README.en.md) and [demo/](demo/).

## Prerequisites

You need **Go 1.26.4+** (below the `go` directive in `go.mod` the toolchain refuses to build) and
**buf** + `protoc-gen-go`·`protoc-gen-go-grpc`. Add **JDK 11+** if you touch the JVM collector (optional — without it only the sidecar build is skipped).
Once the repo builds, the [examples](examples/) just run (only the JVM and OpenSSL integration
examples need **Docker** as well).

Runtime requirements (Ansible, SSH) are in the [root README](README.en.md#requirements); demo requirements in [demo/](demo/README.md#요구-사항).

### Which OS can you build on

| OS | `go build` · `go test` | `make` (gates) | Node binaries |
|---|---|---|---|
| **Linux** | ✅ | ✅ | ✅ directly |
| **macOS** (amd64·arm64) | ✅ | ✅ | ✅ cross-compiled |
| **Windows** (amd64·arm64) | ✅ | needs a POSIX shell → **WSL** | ✅ cross-compiled |

Linux-only code (`/proc`, AF_PACKET, attach) sits behind `//go:build linux`, with a refusing stub on
other platforms. So on macOS and Windows that code is excluded from compilation and breaking it would
still pass a host build — which is why `make build` also cross-compiles **linux/amd64**.

## Development loop

The build procedure is the same as [root README · Build](README.en.md#build). What you additionally use when contributing are the gates and tests:

```bash
make            # all: generate + lint + fmt-check + check-boundary + check-docs + vet + build + build-jar + test
go test ./...   # unit
```

`make build` **leaves no artifacts** — it only checks that the host and linux/amd64 cross-builds
compile (so Linux-only files are covered). Build the binaries you use with `-o`, as the root README does. `make build-jar` warns and skips without a JDK, so contributors touching only Go can run `make`
without one. Tests run without a real JVM.

> **If you see `no required module provides package .../gen/pqcota/...`**, you skipped generation.
> The `go get github.com/pqcota/pqcota/gen/...` that Go suggests alongside it is **not the fix** —
> that is not a fetchable module but code this repo generates. Run `make generate` first.

If you changed a contract, run `make lint` (buf lint) and check compatibility **from the repo root**:

```bash
buf breaking contracts --against '.git#branch=main,subdir=contracts'
```

Also read [the ripple checklist for contract changes](contracts/README.md) (signature coverage, change detection).

## Code layout — top level = kind, stage = inside it

| Top level | What |
|---|---|
| `contracts/` | Contract SSOT (protobuf). The namespace *is* the stage: `pqcota.{common,discovery,inventory,provisioning}.v1` |
| `gen/` | proto-generated code (gitignored) |
| `pkg/` | Library logic — stage groups `discovery`·`inventory`·`provisioning` + shared `kernel` (registry·posture·scope·machineid·sign)·`cbom` |
| `discovery/` · `inventory/` · `provisioning/` | **Execution entry points** (per stage) — each with `cmd/` (scanner·driver·query·generate); `discovery/` also has `collectors/` (reference collectors) |
| `examples/` | **Per-stage runnable examples** — sample inputs + `run.sh` to run each cmd with minimal setup |
| `demo/` | Docker end-to-end demo (access prep → discovery → inventory → provisioning) |
| `tools/` | Repo tooling — `checkdocs` (the docs gate; `make check-docs` builds and runs it) |

That is, **`pkg/`·`contracts/` split by stage, and so do the top-level execution folders**. To **actually run the commands, use [`examples/`](examples/)** (each stage's `run.sh`); for what each command is, see each `<stage>/cmd/README` ([discovery](discovery/cmd/README.md)·[inventory](inventory/cmd/README.md)·[provisioning](provisioning/cmd/README.md)).

## Contract-first

- To change a type/enum, **edit `contracts/*.proto` and `make generate`**. Do not touch `gen/` directly.
- Derived values like `evidence_strength`·`pqc_readiness` are **filled by the core, not the collector** (§0.2 — one place for the rules so they can be recomputed). Details: [contracts/README](contracts/README.md).
- A controlled-vocabulary `*_UNSPECIFIED = 0` means "unknown" — don't leave it blank/missing (§2.6).

## Collector extension — the contract is the seam

The reference collectors (openssl·jvm·network) are just three examples of ways to observe. **When there's more to observe, add a collector** — without touching the core. The single seam is the `CollectionResult` contract (canonical CycloneDX + `pqcota:` properties).

- A collector's job ends at **observe → emit `CollectionResult`**. It does **not** fill derived values like `evidence_strength`·`pqc_readiness` — the core derives those from the contract input (§0.2 — the rules live in one place so they can be recomputed).
- Match the contract and the language is free (the references themselves are Go·Java polyglot). Tool-specific enrichment rides on the standard `properties` extension keys ([contracts/README](contracts/README.md)).
- Each reference collector's design goals, boundary, and honesty rules are in [`discovery/collectors/<name>/README`](discovery/collectors) — a new collector follows the same shape (observe only · unseen = gap · no guessing).

> **The provisioning generator is not yet such a plugin seam** — the plan (`plan.proto`) is a public contract, but the generator itself is internal logic. To avoid confusion, only the collector side is presented as an extension point.

## Coding guidelines

This repo enforces **honesty and determinism in the code itself**. Below are the conventions — not generic Go style, only **what is specifically upheld here**.

**Formatting & checks.** Format with `gofmt` (`go fmt ./...`). `make` (full) runs `buf lint` · `fmt-check` · `check-boundary` · `check-docs` · `go vet` · `build` (host + linux cross) · `build-jar` · `go test`, so everything must be **green** before a PR. `check-docs` gates the Markdown: broken links/anchors, sentences calling something "out of this repo" that this repo actually does, role-division prose (docs carry **function and usage** only), and personal dev-environment details. Follow standard Go idioms, but use the spec's vocabulary for domain terms (`finding` · `app_key` · `crypto_runtime`).

**Comments explain "why", with a §.** *What* the code does, the code says — comments say *why it's done this way* and why the rejected alternative is wrong, anchored to a spec § (the original is Korean). This is why comments here run long. Example: `// exclusion is not "absence" — silently dropping a policy-excluded asset makes the inventory lie (§2.7)`.

**Enforce honesty in code** — it must hold at runtime, not just in docs:
- **unknown is first-class** (§2.6) — an undeterminable value is not a blank but `*_UNSPECIFIED` / an explicit "unknown". A controlled-vocabulary enum's `0` is always unknown.
- **gap ≠ absence** (§2.7) — never silently drop what wasn't seen or what a policy excluded. **Count it, return it, and report it** (excluded counts, the completeness map, the `-diff` reverse-order warning, etc.).
- **no guessing or judgment** (§2.1) — don't fabricate what wasn't observed. If a diff is "no change", that *is* the answer.

**Derived values must be recomputable from the source** (§0.2). Derivations like `evidence_strength` are produced by the core from the source (`detection_method`), not by the collector — the rule lives in one place (`pkg/discovery/normalize`) so it reproduces. **No wall-clock or randomness in signing/canonicalization paths** (same input → same bytes). Content fingerprints exclude volatile fields (observation count, `last_seen`).

**Keep logic pure and testable.** Separate parsing/decision logic from I/O so it unit-tests without the real thing (process, DB, network) — e.g. `ParseProcMaps(reader)` runs without `/proc`. **Tests pin not just behavior but "why this invariant holds"** (a regression test comments the essence of the bug).

**Don't depend on external tools.** Parse `/proc`·ELF directly in Go instead of shelling out to `ldd`·`lsof`·`ss`·`readelf` (minimal image/footprint, §2.4). Release binaries are `CGO_ENABLED=0` static builds. Tag code that touches OS primitives with `//go:build linux`, and split pure helpers out as OS-agnostic.

**Change the contract, change what rides on it.** Every field a collector asserts must be covered by `sign.Canonical` (no signing blind spots). A oneof arm uses a field number **unused across the whole message** (a oneof shares the message's number space). Full checklist: [contracts/README](contracts/README.md).

## Testing

```bash
go test ./...                                              # unit
bash discovery/collectors/openssl/integration/run.sh      # openssl collector real integration (Docker, SD-1·SD-3·SD-4)
./demo/scripts/up.sh && ./demo/scripts/demo.sh            # end-to-end discovery demo
```

Acceptance criteria and implementation order: [discovery test cases](discovery/디스커버리_테스트케이스.md)·[inventory test cases](inventory/인벤토리_테스트케이스.md) (TDD).

## Language

The **Korean** documents are authoritative; the English ones (`*.en.md`) are translations — machine-assisted, and where the two differ the Korean is correct.

**Issues and PRs are fastest in Korean.** The maintainer is a Korean speaker, so English goes through translation both ways — that isn't a refusal, it just makes the round trip longer. Code, logs, and error messages read the same in any language, so paste them verbatim.

Translation contributions are welcome. The **source of truth stays Korean**, though — authoring two languages at once drifts apart in a one-person project.

## Issues · proposals

**Bugs, questions, and proposals go in issues.** There's nothing to hide, and an open discussion stays for the next person. The only thing that must stay private is **something that could expose users to attack if known before a fix** — that path is in [SECURITY](SECURITY.md).

Including this with a bug report speeds up reproduction:

- what you expected and what happened
- the command you ran and its output (redact sensitive values)
- environment — kernel version (`uname -r`), distro, Go version. Collectors are Linux-only and assume **kernel 3.2 or later**
- for observation issues, the target runtime (OpenSSL version, JDK distribution)

**For large changes, open an issue before a PR.** Contracts (`contracts/`) are the single source of truth here, so anything touching the schema or a boundary needs design agreement first — discovering a disagreement after the code is written costs us both.

## Design first

Before adding a feature, read the rationale docs — [docs/](docs/README.md) has the process spec and subsystem designs, and every `§` reference in the code points there.
