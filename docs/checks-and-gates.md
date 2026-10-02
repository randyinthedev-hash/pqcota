English · [한국어](checks-and-gates.ko.md)

# Checks and gates

What the checks in pqcota's five repositories guard, who owns each, where each one runs, and what a green result does **not** tell you. Read it with [Check your work](build.md#check-your-work) in the build guide, which says how to run them, and with [Change a contract](change-a-contract.md), which covers the contract-specific ones. The [developer documentation](developers.md) is the table of contents.

## Two layers

- **Each repository has its own checks**, run by `make` in that repository and by that repository's CI.
- **The integration repository, `pqcota`, adds the gates that look across all five repositories** (documents, import direction, wiring, collector lists). `make all` in `pqcota` runs every sibling's own `make` first, then these gates.

A green `make all` on your machine and a green CI are **not the same statement.** CI adds checks the local run does not have, and a local run can skip tests that CI runs, or the other way round. The sections below say which is which.

## Checks each repository runs

Owner: the repository whose `make` and workflow they are. "Local" means `make` in that repository.

| Check | Blocks | Local | CI | A pass does not show |
|---|---|---|---|---|
| **Formatting** (`fmt-check`) | Unformatted Go (generated `gen/` is excluded) | Yes | Yes | Anything about behavior |
| **Vet** | What `go vet` reports | Yes | Yes | Correctness beyond `go vet` |
| **Build** | Code that does not compile for the host, linux/amd64 or windows/amd64 (each repository's `make build` cross-compiles for both) | Yes | Yes | That the binaries run. Linux-only code is cross-compiled so a macOS or Windows host cannot hide a break; it is not executed |
| **Java sidecar** (`build-jar`, discovery) | A sidecar that does not build | **Skipped with a warning, exit 0, when there is no JDK** | Runs (CI installs a JDK) | With no JDK a green local run built nothing: check that `build/collector.jar` exists |
| **Tests** (`go test ./...`) | Failing tests | Yes, **not verbose**: skipped tests are not listed | Yes. In **inventory, discovery and provisioning** CI runs verbose and prints a list of what was skipped; **common and the integration repository run without `-v`**, so their skips are not listed | Anything a skipped test would have checked. See [Tests that skip](#tests-that-skip) |
| **Contract lint** (`buf lint`, common) | Proto style problems | Yes (needs `buf`) | Yes | That the contract is right |
| **Contract compatibility** (`buf breaking`, common) | Changes that break the last release's contract | Yes, against the latest release tag you have fetched | Yes, with a full clone so the tag is always present | Compatibility of behavior, signatures or the database; see [Change a contract](change-a-contract.md) |
| **Generated-code drift** (common) | `gen/` that differs from what the proto produces | **Not checked.** `make generate` only regenerates | **Yes**: regenerate, then fail if the tree differs | See below |

### Generated-code drift

Locally, `make generate` rewrites `gen/` and says nothing if it changed. Only CI turns a difference into a failure, and it uses a pinned `buf` version, so a different local `buf` can produce a different `gen/`. A clean local `make all` therefore does not mean CI's drift check will pass. After you change a proto, commit the regenerated code with it, then run `make generate && git diff --exit-code -- gen/` on a clean tree. It is close to CI's check but not an exact copy, because uncommitted edits are in your tree. [Change a contract](change-a-contract.md#local-checks-and-ci) explains why.

## Gates in the integration repository

These live in `pqcota/tools/` and are run from `pqcota`. They measure all five repositories unless noted. In `make all` they run after the siblings' own checks.

| Gate | Blocks | Local | CI | A pass does not show |
|---|---|---|---|---|
| **`check-docs`** | In the Markdown of each repository: broken relative links and anchors, section references (`§N`) without a note saying which document they refer to, empty sections, a license table that disagrees with the real dependencies, the Go version in documents disagreeing with `go.mod`, English/Korean skeleton mismatches, personal identifiers such as home paths and machine names, and some Korean wording rules | Yes | Yes | That the text is true or that cross-repository links point to a live page. **It only reads files git tracks**, so stage a new document before you run it |
| **`check-deps`** | An import in a direction the allow-list does not permit (for example discovery importing provisioning, or a collector importing the inventory). A package path the table does not classify also fails | Yes | Yes | That every allowed line is used. It measures direct imports between pqcota packages (tests included), not standard-library or external modules |
| **`check-gates`** | A guarantee that is written as a function but that no product command calls (a function marked `// GATE` that nothing wires), and a placeholder ruleset version | Yes | Yes | That the rule itself is right, which needs its own tests and review, or that the wiring does what the guarantee claims. One gate is currently marked as deferred and reported, not failed |
| **`check-collectors`** | The release workflow and the reference playbook listing different collectors | Yes | Yes | That the binaries work on a node |
| **`check-boundary`** | Wording that names other product tiers, in `.md` and `.go` | Yes | Yes | Anything but that wording rule |
| **`check-prose`** | Korean phrases that were once removed coming back, against a per-file baseline: more hits fail, and fewer hits fail until the baseline is lowered | Yes | Yes | **It measures the `pqcota` repository only**, not the four stage repositories. It reads Markdown outside code blocks and inline code, the user-visible strings of a short list of Go files and the text of listed HTML pages; it skips comments, code blocks and lines without Korean |
| **Integration tests** (`go test ./...` in `pqcota`) | Failures in `test/crossstage`, in the tools' own tests and in the demo topology generator's test | Yes | Yes | Anything in the stage repositories. Their tests run in their own CI. |

Two practical notes. `check-docs` rebuilds its checker and runs it once per repository, so a documentation failure names the repository. And `check-prose` has a baseline file: if you fix wording and the count drops, run its baseline command and commit the new baseline in the same change, or the gate fails the other way.

## Tests that skip

A skipped test is not a pass. Tests skip themselves when the environment lacks something, and **`make` does not tell you which ones skipped**, because it runs `go test` without `-v`. Three CI workflows (inventory, discovery and provisioning) run with `-v` and print a list of skipped cases, so read that list there. The CI of `pqcota-common` and of `pqcota` does not print one; if it matters for those, run the tests verbose yourself.

| What is missing | What skips | Where it runs |
|---|---|---|
| **Postgres** (`PQCOTA_TEST_DSN` not set) | The database tests of the inventory history, the provisioning record store, and the `pqcota-hosts` and `pqcota-profile` organization tests (current `main`) | In CI for **inventory** and **provisioning**, which provide a Postgres service. **Not** in discovery's CI, which has none, so `pqcota-hosts`'s database test skips there and only its rule tests run |
| **`CAP_NET_RAW`** | The packet-capture test that needs the capability. A sibling test that needs the capability to be **absent** skips when you have it, so you cannot run both at once | CI runs the first one again in a separate step with `sudo`, and checks that it printed PASS |
| **A JDK** | The test that checks the attach-blocked path | CI installs a JDK, so it runs there |
| **A local `sshd` on port 22** | The test that observes a real SSH handshake | Whenever the machine has no sshd; the CI skip list shows what happened on the runner |
| **A readable `/proc`**, an ELF test binary | The host-scan path of the OpenSSL collector | Linux only; these tests are not even compiled on other systems |
| **`bash`** | The demo topology generator's test | Wherever bash is missing |
| **A git checkout** | The import-direction test, when the tree is not a checkout | |

**To see skips locally**, run the tests verbose into a file, look at their own result first, then at the skips (a plain `go test ... | grep` hides a failing run, because the pipeline reports the status of `grep`):

```bash
go test ./... -count=1 -v > /tmp/test.log 2>&1; status=$?
echo "go test exit status: $status"
grep -- '--- SKIP' /tmp/test.log
```

A test that returns early after its own environment check, without calling `t.Skip`, is not listed even with `-v`.

**To run the Postgres tests locally**, use a disposable database. They create their tables and append rows, and never delete them. The container name and host port below are examples: pick a free port if `55432` is taken, and remove only this container when you are done.

```bash
docker run -d --name pqcota-test-pg -e POSTGRES_USER=pqcota -e POSTGRES_PASSWORD=pqcota -e POSTGRES_DB=pqcota -p 127.0.0.1:55432:5432 postgres:16
docker exec pqcota-test-pg pg_isready -U pqcota        # repeat until it reports "accepting connections"
export PQCOTA_TEST_DSN='postgres://pqcota:pqcota@127.0.0.1:55432/pqcota?sslmode=disable'
# ... run the tests ...
docker rm -f pqcota-test-pg
```

[`PQCOTA_TEST_DSN`](environment-variables.md#tests-and-the-demo) is listed in the environment variable reference.

## What a green `make all` does not tell you

- **That CI will pass.** Locally, drift is not checked, skips are silent, and the `sudo` capture test, the arm64 cross-build and the Postgres service of the CI jobs are not reproduced.
- **That nothing was skipped.** Read `-v` output or the CI skip list.
- **That the Java sidecar was built.** Without a JDK the step warns and succeeds.
- **That the sibling repositories' own CI is green.** A green `make all` in `pqcota` means each sibling's own `make` passed on your working tree, in order, because any failing sibling stops it. It does not include what only CI runs (the drift check, the skipped tests that CI can run, the arm64 build). The gates in the table above run once, from the integration repository, after the siblings. In the other direction, the integration repository's own CI does not run the siblings' `make`: it calls the gates and its own tests individually, so a green run there says nothing about the siblings' CI.
- **That the documents are correct.** The document gate checks links, structure and a few rules, not truth. The claims in the user documents were checked by hand against the code, and no gate repeats that.
- **That the Windows binaries work.** They are cross-compiled; no check runs them.
- **That a release is consistent.** Releases are coordinated by hand, as the same tag on all five repositories (see [the build guide](build.md#get-the-source)). That is the current practice, not something a check enforces.

## Reading a CI run

Each of the four stage repositories (`pqcota-common`, `-inventory`, `-discovery`, `-provisioning`) has a `build` job for its own checks and a `cross-stage` job that calls the integration repository's workflow with that stage's commit. The integration repository, `pqcota`, has the workflow that is called (and also runs on its own pushes). Discovery adds a `cross-build` job for amd64 and arm64.

A failed `cross-stage` job means the integration checks failed for your commit combined with `main` of the other four. It does not say why: it can be an import direction, a gate wiring, a document link, but also a failing cross-stage test, a build problem, the checker environment, or a change that landed on another repository's `main`. Open the log of the step that failed.
