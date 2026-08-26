English · [한국어](CONTRIBUTING.md)

# Contributing (CONTRIBUTING)

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](CONTRIBUTING.md) is authoritative.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](docs/regulation.en.md).

> **What will not be broken** — contract, signature, Go API, DB schema, and mixed versions, written out
> as five distinct faces: [compatibility policy](docs/compatibility.md) (Korean). Read it before changing
> any of them.

For developers who want to **fork·extend·contribute** to pqcota. Users who just want to *try* the platform should see the root [README](README.en.md) and [demo/](demo/).

## Prerequisites

You need **Go 1.26.4+** (below the `go` directive in `go.mod` the toolchain refuses to build) and
**buf** + `protoc-gen-go`·`protoc-gen-go-grpc`. Add **JDK 11+** if you touch the JVM collector (optional — without it only the sidecar build is skipped).
Once the repo builds, the [examples](examples/) just run (only the JVM and OpenSSL integration
examples need **Docker** as well).

Runtime requirements (Ansible, SSH) are in the [root README](README.en.md#requirements); demo requirements in [demo/](demo/README.en.md#requirements).

### Which OS can you build on

| OS | `go build` · `go test` | `make` (gates) | Node binaries |
|---|---|---|---|
| **Linux** | ✅ | ✅ | ✅ directly |
| **macOS** (amd64·arm64) | ✅ | ✅ | ✅ cross-compiled |
| **Windows** (amd64·arm64) | ✅ | needs a POSIX shell → **WSL** | ✅ cross-compiled |

Linux-only code (`/proc`, AF_PACKET, attach) sits behind `//go:build linux`, with a refusing stub on
other platforms. So on macOS and Windows that code is excluded from compilation and breaking it would
still pass a host build — which is why `make build` also cross-compiles **linux/amd64 and windows/amd64**; the CNG collector will grow on that target.

## Development loop

The build procedure is the same as [root README · Build](README.en.md#build). What you additionally use when contributing are the gates and tests:

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

If you changed a contract, run `make lint` (buf lint) and check compatibility **from the repo root**:

```bash
buf breaking contracts --against '.git#branch=main,subdir=contracts'
```

Also read [the ripple checklist for contract changes](contracts/README.en.md) (signature coverage, change detection).

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

That is, **`pkg/`·`contracts/` split by stage, and so do the top-level execution folders**. To **actually run the commands, use [`examples/`](examples/)** (each stage's `run.sh`); for what each command is, see each `<stage>/cmd/README` ([discovery](discovery/cmd/README.en.md)·[inventory](inventory/cmd/README.md) (Korean)·[provisioning](provisioning/cmd/README.md) (Korean)).

## Contract-first

- To change a type/enum, **edit `contracts/*.proto` and `make generate`**. Do not touch `gen/` directly.
- Derived values like `evidence_strength`·`pqc_readiness` are **filled by the core, not the collector** (§1.2 — one place for the rules so they can be recomputed). Details: [contracts/README](contracts/README.en.md).
- A controlled-vocabulary `*_UNSPECIFIED = 0` means "unknown" — don't leave it blank/missing (§2.5).

## Collector extension — the contract is the seam

The reference collectors (openssl·jvm·network) are just three examples of ways to observe. **When there's more to observe, add a collector** — without touching the core. The single seam is the `CollectionResult` contract (canonical CycloneDX + `pqcota:` properties).

- A collector's job ends at **observe → emit `CollectionResult`**. It does **not** fill derived values like `evidence_strength`·`pqc_readiness` — the core derives those from the contract input (§1.2 — the rules live in one place so they can be recomputed).
- Match the contract and the language is free (the references themselves are Go·Java polyglot). Tool-specific enrichment rides on the standard `properties` extension keys ([contracts/README](contracts/README.en.md)).
- Each reference collector's design goals, boundary, and honesty rules are in [`discovery/collectors/<name>/README`](discovery/collectors) — a new collector follows the same shape (observe only · unseen = gap · no guessing).

> **The provisioning generator is not yet such a plugin seam** — the plan (`plan.proto`) is a public contract, but the generator itself is internal logic. To avoid confusion, only the collector side is presented as an extension point.

## Extending with a new crypto runtime

See the [crypto runtime acceptance principles](docs/runtime-acceptance.en.md).

## Coding guidelines

This repo enforces **honesty and determinism in the code itself**. Below are the conventions — not generic Go style, only **what is specifically upheld here**.

**Formatting & checks.** Format with `gofmt` (`go fmt ./...`). `make` (full) runs every gate, so all of them must pass before a PR. Which gate blocks what is in the table in [governance](docs/governance.md) (Korean). It is not listed again here because every new gate would then need two places kept in step — and `check-collectors` was in fact missing from this sentence for a while. Follow standard Go idioms, but use the spec's vocabulary for domain terms (`finding` · `app_key` · `crypto_runtime`).

**Comments explain "why", with a §.** *What* the code does, the code says — comments say *why it's done this way* and why the rejected alternative is wrong, anchored to a spec § (the original is Korean). This is why comments here run long. Example: `// exclusion is not "absence" — silently dropping a policy-excluded asset makes the inventory lie (§2.6)`.

**Enforce honesty in code** — it must hold at runtime, not just in docs:
- **unknown is first-class** (§2.5) — an undeterminable value is not a blank but `*_UNSPECIFIED` / an explicit "unknown". A controlled-vocabulary enum's `0` is always unknown.
- **gap ≠ absence** (§2.6) — never silently drop what wasn't seen or what a policy excluded. **Count it, return it, and report it** (excluded counts, the completeness map, the `-diff` reverse-order warning, etc.).
- **no guessing or judgment** (§2.1) — don't fabricate what wasn't observed. If a diff is "no change", that *is* the answer.

**Derived values must be recomputable from the source** (§1.2). Derivations like `evidence_strength` are produced by the core from the source (`detection_method`), not by the collector — the rule lives in one place (`pkg/discovery/normalize`) so it reproduces. **No wall-clock or randomness in signing/canonicalization paths** (same input → same bytes). Content fingerprints exclude volatile fields (observation count, `last_seen`).

**Keep logic pure and testable.** Separate parsing/decision logic from I/O so it unit-tests without the real thing (process, DB, network) — e.g. `ParseProcMaps(reader)` runs without `/proc`. **Tests pin not just behavior but "why this invariant holds"** (a regression test comments the essence of the bug).

**Don't depend on external tools.** Parse `/proc`·ELF directly in Go instead of shelling out to `ldd`·`lsof`·`ss`·`readelf` (minimal image/footprint, §2.3). Release binaries are `CGO_ENABLED=0` static builds. Tag code that touches OS primitives with `//go:build linux`, and split pure helpers out as OS-agnostic.

**Change the contract, change what rides on it.** Every field a collector asserts must be covered by `sign.Canonical` (no signing blind spots). A oneof arm uses a field number **unused across the whole message** (a oneof shares the message's number space). Full checklist: [contracts/README](contracts/README.en.md).

## Testing

```bash
go test ./...                                              # unit
bash discovery/collectors/openssl/integration/run.sh      # openssl collector real integration (Docker, SD-1·SD-3·SD-4)
./demo/scripts/up.sh && ./demo/scripts/demo.sh            # end-to-end discovery demo
```

Acceptance criteria and implementation order: [discovery test cases](discovery/testcases.md) (Korean)·[inventory test cases](inventory/testcases.md) (Korean) (TDD).

## Language

**Documents are Korean; whatever the code emits is English.** The dividing line is not "who reads it"
but **how far it travels**.

| | Language | Why |
|---|---|---|
| Documents (`*.md`) | **Korean** (canonical); `*.en.md` are translations | the maintainer is Korean-speaking, and design judgments are most precise in one's first language |
| **Comments** | **Korean** | they reach only whoever reads the code; they never leave the program |
| **Console output** (stdout, stderr, flag help) | **English** | it ends up in logs, gets pasted into issues, and is read by strangers |
| **Strings carried by the contract** (`Completeness.Note`, `Attribution.Reason`, remediation notes) | **English** | they are stored and **travel out through the contract** — in Korean, that language becomes part of the contract |
| **Error values** (`errors.New`, `fmt.Errorf`) | **English** | where they flow is the caller's decision, not ours |
| **Test failure messages** | **English** | they land in CI logs |

**That table is the whole reason comments alone stay Korean** — everything else is something the
program **emits**, and what you emit does not get to choose its reader. If Korean should appear on a
screen, that is a job for the view, not a reason to put Korean into observation data.

> **One exception** — the **patterns** in `tools/checkdocs` are Korean. It gates Korean documents, so
> what it searches for cannot be anything else. What that tool **says** is English.

The **Korean** documents are authoritative; the English ones (`*.en.md`) are translations — machine-assisted, and where the two differ the Korean is correct.

**When you edit a Korean document, re-translate its `*.en.md` counterpart against the new original.** Fixing only one
side lets the translation go quietly stale, which turns "where the two differ the Korean is correct" into an excuse.
Links inside an English document **point at the English counterpart when one exists**; otherwise they point at the
Korean document and say so with `(Korean)`.

**Issues and PRs are fastest in Korean.** The maintainer is a Korean speaker, so English goes through translation both ways — that isn't a refusal, it just makes the round trip longer. Code, logs, and error messages read the same in any language, so paste them verbatim.

Translation contributions are welcome. The **source of truth stays Korean**, though — authoring two languages at once drifts apart in a one-person project.

**How far does the English go** — not all the way. This is a one-person project, and keeping both
languages at the same breadth would leave neither current. So the scope is fixed.

| Has an English version | Korean only |
|---|---|
| The front door — README, CONTRIBUTING, SECURITY, RELEASE_NOTES | test case lists, the test map, kernel cases |
| The regulation, architecture, per-stage designs, contracts | collector deployment design, each `cmd/README` |
| Licensing, runtime acceptance, under review | **the compatibility policy**, the journey, the demo, every example |

When an English document points at something with no English counterpart, it says `(Korean)`. Contributions
that widen this table are welcome — just start knowing that widening it also widens the upkeep.

### Write Korean, do not carry English sentences over

The canonical documents are Korean. Thinking in English and translating produces sentences that are
correct and unreadable. These are real examples from this repository.

| Not this | This | The English behind it |
|---|---|---|
| the **axis** it unfolds along is capability | capability **grows in this order** | unfolds along a capability axis |
| the **seam** is `contracts/` | the only **join** to anything outside is `contracts/` | the seam is |
| **consumers consume** it through the contract | **whatever picks it up** sits beyond the contract | consumers consume |
| order **has meaning** | order **is** the priority | order has meaning |
| it **has** two kinds | it **splits in two** | has two kinds |

Words like `축` (axis) that have settled into Korean technical writing are fine — the problem is not
vocabulary but **carrying English sentence structure across**. Nominalising verbs into `~을 가진다`
or `~을 제공한다` is usually a sign that a sentence was translated rather than written.

**No gate catches this.** `make check-docs` checks links, anchors and scope wording, not the grain of a
sentence. Read it aloud; if it sounds off, that is the evidence.

### When you write "it does not", attach the reason on the spot

This repo does not hide its limits, so negative sentences are common: *it does not always work · it is
not settled · we do not build it*. But **when the reason arrives two sentences later, the reader fills
the gap with a guess.** A judgement and its basis belong together.

| Not this | This |
|---|---|
| It goes as far as the app, but **it does not always work**. (…two sentences of explanation later…) | It goes as far as the app, but **only if the socket is still alive at lookup time** |
| The machine **does not settle it**. | The machine does not settle it. **Whether it is live or stale is something only a person knows** |
| An admin UI **is not built**. | An admin UI is not built. **Once there is a screen, "let's approve here too" is the next step** |

Saying the same thing twice is a signal as well. If the headline says "it does not always work" and the
next paragraph says "it is not always filled in", the first one was floating without a reason.

**The gate does not catch this.** `make check-docs` looks at links, anchors and scope wording, not at
the grain of a sentence. If it reads awkwardly, that is your evidence.

## Issues · proposals

**Bugs, questions, and proposals go in issues.** There's nothing to hide, and an open discussion stays for the next person. The only thing that must stay private is **something that could expose users to attack if known before a fix** — that path is in [SECURITY](SECURITY.en.md).

Including this with a bug report speeds up reproduction:

- what you expected and what happened
- the command you ran and its output (redact sensitive values)
- environment — OS and Go version (on Linux, `uname -r` and the distro). The Linux collectors assume **kernel 3.2 or later**
- for observation issues, the target runtime (OpenSSL version, JDK distribution)

**For large changes, open an issue before a PR.** Contracts (`contracts/`) are the single source of truth here, so anything touching the schema or a boundary needs design agreement first — discovering a disagreement after the code is written costs us both.

## Design first

Before adding a feature, read the rationale docs — [docs/](docs/README.en.md) has the process spec and subsystem designs, and every `§` reference in the code points there.
