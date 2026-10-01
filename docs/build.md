# Build guide

For engineers who build pqcota from source or work on it. If you only want to run pqcota, you do not need this page; see the shortcut below. For the overview, start at the [README](../README.md).

## If you only want to run it

Every release attaches ready-made static binaries and a checksum file to the [releases page](https://github.com/randyinthedev-hash/pqcota/releases). They are split by where they go:

| Bundle | Goes to |
|---|---|
| `pqcota-linux-amd64.tar.gz`, `pqcota-linux-arm64.tar.gz` | Linux systems you observe: `pqcota-nodescan`, `pqcota-netcap`, `pqcota-jvmscan` |
| `pqcota-windows-amd64.zip` | Windows systems you observe: `pqcota-cngscan`, `pqcota-jvmscan` |
| `pqcota-ctl-linux-amd64.tar.gz`, `pqcota-ctl-linux-arm64.tar.gz` | The one central machine: ingest, query, approval and generation commands |
| `collector.jar` | Only if you use the Java attach path |
| `SHA256SUMS` | Lists a checksum for every asset of the release |

To check what you downloaded, put the files next to `SHA256SUMS` and run `sha256sum -c --ignore-missing SHA256SUMS` (GNU coreutils; without `--ignore-missing` it reports an error for every asset you did not download). **Read the output:** it names each file it actually checked. A file you downloaded must appear there as OK; if the output names none, nothing was verified. A matching checksum shows the file was not damaged in transfer. It does not show who published it: signed releases are on the [roadmap](../RELEASE_NOTES.md).

Privileges and environment variables for running the collectors on a system are in the [command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md).

## Requirements

**To build**

- Go 1.26.4 or newer.
- Git, `make` and a POSIX shell (the commands below are written for one).
- `buf`, `protoc-gen-go` and `protoc-gen-go-grpc`, **only when you change a protobuf contract** (in `pqcota-common`). The generated Go code is committed, so a plain build does not need them. `buf` is installed separately (<https://buf.build/docs/installation>); `make tools` installs only the two plugins.
- A JDK 11 or newer, **optional**, only to build the Java attach sidecar. Without it that step is skipped.

**To run the checks (`make all`)**

- The tools above, including `buf` (installed separately) and the two plugins (`make tools` in `pqcota-common`). `make all` also runs each sibling repository's own checks. Those regenerate the protobuf code and run contract lint and the compatibility check, which is why `make all` needs `buf` even if you changed no contract.
- Postgres tests run only when `PQCOTA_TEST_DSN` is set. Without it they are skipped. The default local test run is not verbose and does not list skipped tests; run `go test -v ./...` in the repository to see which ones were skipped.

**To run**

- Several systems: Ansible on the controller and SSH access to the targets.
- One system: no resident agent or service. Run the binary on that system. Some observation paths have prerequisites: `pqcota-netcap` needs the `CAP_NET_RAW` capability, and some Java paths need a JDK on the machine or the `collector.jar` sidecar.
- Postgres, only if you want to accumulate and query history across many systems.

## Get the source

pqcota is **five repositories**. Clone them side by side, because each `go.mod` reads its siblings from `../` through a `replace` directive:

```bash
git clone https://github.com/randyinthedev-hash/pqcota
git clone https://github.com/randyinthedev-hash/pqcota-common        # contracts, generated code, shared logic
git clone https://github.com/randyinthedev-hash/pqcota-inventory     # the inventory stage
git clone https://github.com/randyinthedev-hash/pqcota-discovery     # collectors, their commands, the reference playbook
git clone https://github.com/randyinthedev-hash/pqcota-provisioning  # the provisioning stage
cd pqcota
```

| Repository | What is in it |
|---|---|
| `pqcota` | The demo, examples, release bundles, the gate tools and the checks that span stages, the contributing guide |
| `pqcota-common` | The protobuf contracts and their generated Go code, and shared code (identity, signing, key generation) |
| `pqcota-inventory` | Normalization, the append-only history, queries |
| `pqcota-discovery` | The collectors, their commands, the reference Ansible playbook |
| `pqcota-provisioning` | Plan approval, artifact generation, execution records |

Each release is the same tag (for example `v0.10.1`) on all five. A plain clone gives you `main` of each, which is development state. To build a release, check out that tag in **all five** repositories, for example, from the `pqcota` directory (the `cd pqcota` above), `for r in pqcota pqcota-common pqcota-inventory pqcota-discovery pqcota-provisioning; do git -C ../$r checkout v0.10.1; done`. The checkout leaves each repository on a detached HEAD at the tag. The `require` lines in `go.mod` do not pin the siblings, because the local `replace` directives read the working trees next to it.

**What a tag gives a consumer outside this workspace.** Importing a package from the tag as a library works: Go ignores the `replace` lines of a dependency and resolves the `require` lines to the tags. Running a command straight from the tag does not. `go install github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-hosts@v0.10.1`, and `go run github.com/randyinthedev-hash/pqcota/tools/checkprose@v0.10.1`, are refused by Go while a `go.mod` carries `replace` directives. To run a command, build it from the five-repository checkout above, or take the release bundle. As of 2026-10-01 the same holds for v0.10.0.

`go run …/tools/checkprose@v0.9.1` still works, because that tag predates the split. Pinning it keeps using the v0.9.1 checker; it is not a way to receive a newer one, and the checker has changed since.

## Build

pqcota has one **central controller** and the **target systems** it reaches over Ansible and SSH. Build on the controller: both the commands you run there and the collectors you ship to targets come from here.

**The commands you run on the controller.** The plain `go build` below is a development build for your own machine. The release bundles are built statically with `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build -trimpath -ldflags='-s -w'`; use the same flags if you want a comparable artifact.

```bash
D=github.com/randyinthedev-hash
go build -o bin/ $D/pqcota-common/cmd/... $D/pqcota-discovery/cmd/... $D/pqcota-inventory/cmd/... $D/pqcota-provisioning/cmd/...
```

**The collectors that go on target systems**: built statically for the target's OS and architecture. Which collector runs on which OS is in the [command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md).

```bash
D=github.com/randyinthedev-hash/pqcota-discovery/cmd

# Linux targets
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux-amd64/ $D/pqcota-nodescan $D/pqcota-netcap $D/pqcota-jvmscan

# Windows targets
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows-amd64/ $D/pqcota-cngscan $D/pqcota-jvmscan

make build-jar        # only for Java targets: builds the attach sidecar to build/collector.jar (needs a JDK 11 or newer)
```

**Without a JDK, `make build-jar` prints a warning and still exits successfully, and no `collector.jar` is produced.** Check that `build/collector.jar` exists before you rely on the Java attach path.

`CGO_ENABLED=0` (static linking, independent of the Linux distribution and libc) stays fixed. Change `GOOS` and `GOARCH`; the accepted values are in the [Go documentation](https://go.dev/doc/install/source#environment).

**Kernel floor.** Go's own minimum for Linux is kernel 3.2 ([Go minimum requirements](https://go.dev/wiki/MinimumRequirements)), so that is the general floor for binaries built this way. Individual pqcota features can need more; those are listed in the [command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md). The Windows binaries are cross-compiled here; this guide does not claim they were run on a particular Windows version.

## Check your work

From `pqcota`, after `make tools` in `pqcota-common`:

```bash
make all
```

`make all` runs each sibling repository's own checks, then the checks that span all five: formatting, wording boundaries, documentation links, the collector lists, gate wiring, import direction between stages, the prose gate, vet, build and tests. The local checks regenerate the protobuf code and run contract lint; **CI additionally rejects any difference between the regenerated code and the committed files**, so commit regenerated code together with the proto change. CI also provides a Postgres service, runs the tests that need `CAP_NET_RAW` in a privileged job, and cross-builds for arm64. A passing local `make all` does not reproduce those. For what each check guards and what a pass does not show, see [Checks and gates](checks-and-gates.md). A change to any stage repository also triggers the cross-stage checks in CI.

## Change a contract

Contracts live in `pqcota-common`. After you change a `.proto` file, regenerate the Go code and commit it **in the same commit** as the proto change:

```bash
cd ../pqcota-common
make tools && make generate     # contracts/proto → gen/
```

`make tools` installs the plugins into `$(go env GOPATH)/bin`. If that directory is not on your `PATH`, `make generate` reports that a plugin is not found, which looks like a failed install but only means it is not visible. Add it to your shell profile: `export PATH="$PATH:$(go env GOPATH)/bin"`.

Contract changes must stay additive; see the [compatibility policy](compatibility.md). `make breaking` compares against the last release tag. For the order of work and what else has to change with a contract, see [Change a contract](change-a-contract.md).

## Stack

- **Go**: every collector and command. Release bundles are static single binaries (`CGO_ENABLED=0`).
- **Java**: only the attach sidecar, because that observation is possible only from inside the JVM.
- **Protobuf and gRPC**: the contracts that join the stages ([`contracts/`](https://github.com/randyinthedev-hash/pqcota-common/tree/main/contracts)).
- **Postgres**: only for accumulating and querying many systems over time. Observing a single system does not use it.

## Where to go next

The [developer documentation](developers.md) for how the five repositories fit together and what to read next · [CONTRIBUTING](../CONTRIBUTING.md) for tests, gates and how to propose a change · the [demo](../demo/README.md) for the whole flow on containers · the [examples](../examples/README.md) for one command at a time, with Go only.
