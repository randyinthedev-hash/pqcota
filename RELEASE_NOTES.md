# Release Notes — pqcota


Records the **goals** and **results** per version. Updated as versions advance, newest on top.

Sections are split by the **kind** of content — **What was built** · **What was learned** ·
**What was fixed**. A defect is not dressed up as a feature; it gets its own section, which states
three things: **what was wrong · which version it entered in · what came out wrong** (if nothing came
out wrong, it says so). **Sections of already-published versions do not have their facts changed.**
What happened when, and which version a defect entered in, are not rewritten later; a defect is
recorded in the version that fixed it, naming where it started.

**Wording and length, however, may be revised** — bringing prose in line with a later style, or
shortening a section that runs long for its weight (this is what happened to v0.6.4 and v0.6.5, which
changed nothing but wording). One test decides it: **what the reader learns must not change; how it is
said may.**

**Whether to cut a separate patch release is a distinct call.** The criterion is not a version-number
rule but *how long a known defect would sit in the latest release*. If the next minor is far off, cut the
patch (v0.1.3 did). If it ships shortly, carry the fix there — a release that lives for hours only adds
history. Either way the "What was fixed" section names **which version it entered in**, so the record is
the same.

---

## Roadmap — Upcoming releases (planned)

Directional, not fixed. Each version is promoted to a proper section per the rule above once started/completed. The **Windows CNG runtime is introduced in stages** , not all at once.

- **CNG provisioning (planned · version TBD)** — **substrate generalization first** (moving past the POSIX-file assumption — Windows uses the registry/GPO, which doesn't fit `/opt/pqcota` file staging or file-removal rollback) → `renderCNG`. The generalization is done together with that implementation (no speculative abstraction). Where to draw the seam is still undecided.

- **Observing OpenSSL on Windows (planned · version TBD)** — `pqcota-nodescan` has a single
  implementation today and it reads `/proc`. Run it on Windows and it emits a gap rather than an empty
  result, but it **cannot observe**. What is needed is swapping two pieces: finding the loaded modules
  (`procmaps.go` → the Toolhelp32 module list) and pulling strings out of a binary (`elfstrings.go` →
  PE). **Fork detection (`registry.MatchFork`) takes only the extracted strings, so it is reused
  as-is** — the same shape the jvm reconnaissance took, per-OS I/O over shared pure matching. Which
  collector runs on which OS is in the [command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md).

- **Accepting the provider ecosystem (under review · version TBD)** — choosing which provider to use, and obtaining its file, is done by whoever writes the plan. What this repo does is **write the configuration file that activates that provider**. Today it only knows one shape, `activate`+`module` — and since each provider demands different settings, it cannot yet produce one for OpenSSL's own `fips` module (which has to pull in the file `fipsinstall` generates) or for pkcs11-provider (which needs additional entries such as the driver path). What each candidate would additionally require, along with provider observation and the HSM axis, is still being worked out.

- **Observing server-role edges (planned · version TBD)** — today netcap emits **only the edges the observing node initiated**. So a **process serving on that node gets no edge at all**, and the comparison that ties "this app loaded that library" (found by nodescan) to "this is what it actually negotiated" through `app_key` is entirely empty on the server side. The contract can already carry this with `EdgeRole.SERVER`; what blocks it is the value conventions and deduplication.

- **Separating the traffic-observation cadence (planned · version TBD)** — assets and communication edges are observed on different cadences. Today they are bound into one playbook, so seeing more edges means re-running the `/proc` sweep and the JVM attach along with it. `discover.yml` is left alone and a playbook that runs netcap in rounds is **added**. What has to be solved along with it is **merging repeated observations**: the count from one capture is recorded (`observed_count`), but counts across several capture rounds are not merged, `first_seen` and `last_seen` are filled by nobody, and the merge key differs in three places.

- **Scope classification for the edge peer (planned · version TBD)** — when an observation meets a node that is not in the scope master, the design says to send it to a **registration request** — yet **the classifier (`ClassifyObserved`) exists and nothing calls it.** An unregistered peer stays as an address in `dst_addr`, visible on screen but never collected. This is the only automatic way to surface a node the operator failed to register, and it is not connected today.

- **Binding the signature to a collector (planned · version TBD)** — the ingest today confirms only that
  **someone signed**. `PQCOTA_VERIFY_KEY` is a list of keys, so there is no way to check it against
  `envelope.collector_id`, and whoever holds one key can arrive **wearing any collector's name**.
  `sign.VerifyFrom`, which verifies against that binding, already exists and **nothing calls it**. What
  blocks it is the environment-variable format, so it is not a one-line change.

- **Observing communication on Windows (planned · version TBD)** — `pqcota-netcap` is Linux-only, so on
  Windows **no binary is produced at all**. That node's completeness map therefore never records "NETWORK
  layer not observed", and on screen it looks like a node with no communication. The dissection layer
  knows nothing about the OS, so only the piece that obtains frames has to be swapped. Npcap is not taken
  because its driver stays on the node; the built-in `SIO_RCVALL` is (mostly IPv4, no loopback). 

- **Release signing (partly delivered in v0.14.0 · rest version TBD)** — from v0.14.0 every release carries a **per-file SHA-256 manifest** (`pqcota-release-manifest.json`: every file inside every bundle with its SHA-256, version and module hashes, and each repository's tag commit) and **GitHub build provenance attestations** for every asset and every program inside the bundles, signed by the release workflow. They are checked with `sha256sum`, the manifest, and `gh attestation verify` against the expected repository, workflow, tag and commit ([build guide](docs/build.md#check-what-you-downloaded)). What is still planned is a signature that can be checked **without GitHub and without network access**: the ed25519 signature and `pqcota-verify-bundle` settled in the collector deployment design.

- **Observing and provisioning other Unix systems (planned · version TBD)** — observation and generation work end to end on Linux only. The collectors read `/proc` and ELF files and capture packets with `AF_PACKET`, all Linux mechanisms; on other systems they report a gap rather than an empty result. The matching that decides what was found (fork detection, the ruleset, plans and approvals) does not depend on the OS, so the work is swapping the pieces that read the system, as for Windows: finding the loaded libraries, reading the binary format, attaching to a JVM, and capturing packets. The generated playbooks stage files under `/opt/pqcota` and `/etc/pqcota` and run with privilege escalation; on each new system those paths, the privilege model and the restart hooks have to be checked too. The first system is not decided. Observation on a system counts as done when results collected there are matched correctly, and provisioning when the generated playbooks are applied there and rolled back; only what was confirmed that way is listed. When a system is chosen, this item is split per system.

### Not on the roadmap — deliberately

These are **boundaries**, not directions. Written down so no one waits for them.

| Not built | Instead |
|---|---|
| **Fleet orchestration** — drain · rolling · health gates | Standard Ansible playbooks come out, so your deployment tooling drives them |
| **Remote execution engine** — resident agents, push channels | You run the generated artifacts on your own substrate |
| **Source / artifact CBOM scanner** | CI already has the source. CycloneDX from CBOMkit and friends is **ingested** instead |
| **Dynamic tracing** (eBPF · ltrace) | Invasive, so it isn't done. Observing the actual negotiation on the wire was chosen instead |
| **Verdicts and scoring** — "risky" grades | Only observed facts are emitted. What to change, and when, is yours to decide |
| **An admin screen for entering values by hand** | These arrive as files (`hosts.csv`, `scope-assets.csv`, machine profiles, the finalized plan JSON, `pqcota-declare`). Build a screen and a review queue and an approve button follow, and both are explicitly excluded by the architecture. At that moment an observation tool becomes an adjudication tool |


---

## v0.14.0 — Releases say which version and commit they were built from, and can be checked file by file (2026-11-12)

**Goal** — make a released program show what it was built from, and let anyone check a single file against the release and
its origin. Until now every program's build information showed the pqcota modules as `(devel)` pointing at a local folder,
`collector.jar` carried no version, and `SHA256SUMS` covered only the archives, with nothing to show who built them. **The
contract, the ruleset and the database schema are unchanged, and so is what the programs do.** What changed is how the
release is built and what it publishes: the release workflow, `collector.jar`'s manifest, and a new release asset.

**Upgrade all five repositories together, as before.** The programs behave as in v0.13.1; replace them when you want
programs you can check.

### Built

- **Programs carry their module versions.** The release builds the five modules from their tags instead of from folders side
  by side, so `go version -m` on a released program shows each pqcota module at the release version with its content hash
  (`h1:`), for example `dep github.com/randyinthedev-hash/pqcota-common v0.14.0 h1:…`, and no `=> ../… (devel)` line. A
  program does not carry its source commit; the release manifest lists each repository's tag commit. Programs you build
  yourself from the five repositories side by side still show `(devel)`.
- **`collector.jar` carries its version and commit** in its manifest: `Implementation-Version`, `X-Pqcota-Commit` (the
  pqcota-discovery commit) and `X-Pqcota-Dirty`.
- **A per-file SHA-256 manifest**, `pqcota-release-manifest.json`, is a new release asset. It lists every file inside every
  bundle with its SHA-256 and the versions and hashes from its build information, `collector.jar` with the values of its
  manifest, and the tag commit of each repository. The commits come from the module origin of each tag and must equal the
  commits the workflow checked out; when they cannot be confirmed, the release stops. `SHA256SUMS` now also covers the
  manifest.
- **GitHub build provenance attestations** for every asset and every program inside the bundles, signed by the release
  workflow. `gh attestation verify` checks a file against the repository, the release workflow, the tag and the tag's commit.
  The [build guide](docs/build.md#check-what-you-downloaded) gives the three checks (asset checksums, files inside the
  bundles, origin) and how to read a program's version.
- **Tried first as a pre-release** (`v0.14.0-rc.1`, marked as a pre-release): the downloaded assets matched their checksums
  and the manifest; rebuilding the tag gave the same SHA-256 for all 46 programs; all 54 files (8 assets and 46 programs)
  passed attestation verification against the pre-release tag and its commit, and a copy changed by one byte, another tag
  or another commit was rejected.
- **Go toolchain.** The published programs are built with Go 1.26.11 (the `toolchain` line in `go.mod`). `govulncheck`
  found no vulnerabilities in the five repositories.
- **Release assets** (checked after publishing): the checksums of all seven files in `SHA256SUMS` match; the manifest
  matched the downloaded assets; all 46 programs are built with Go 1.26.11, show the pqcota modules at v0.14.0 with `h1:`
  hashes and no `replace`, and `govulncheck -mode=binary` found no vulnerabilities in any of them; the commits in the
  manifest equal the `v0.14.0` tag commits of the five repositories; rebuilding the tag gave the same SHA-256 for all 46
  programs. All 54 files passed attestation verification against the repository, the release workflow, `refs/tags/v0.14.0`
  and the tag's commit taken from the tag, and a copy changed by one byte, the pre-release tag or another commit was
  rejected.

### Known limitations

- **The attestations are checked through GitHub.** Checking them needs `gh` (or another Sigstore client) and network
  access; a system without network access is checked from a machine that has it. A signature that can be checked offline
  is on the roadmap.
- **`collector.jar` is not byte-for-byte reproducible** across Java versions; its version and commit are in its manifest,
  and its SHA-256 is in the release manifest and `SHA256SUMS`.
- Installing a command with `go install …@v0.14.0` still does not work, because the repositories' `go.mod` files keep
  `replace` lines for the local build. Use the release assets, or build from the five repositories side by side.
- The other limitations of v0.13.1 are unchanged.

---

## v0.13.1 — The latest snapshot of a node is the last state loaded (2026-11-12)

**Goal** — fix which snapshot counts as a node's latest. A state seen before reuses its earlier snapshot, so the snapshot
with the highest sequence number is not always the state that was loaded last. **The contract, the ruleset and the
database schema are unchanged.** The change is in the history of `pqcota-inventory`, so every program that reads a node's
latest snapshot changes with it: `pqcota-ingest` (its summary), `pqcota-inventory`, `pqcota-prune` and `pqcota-provision`.
The collectors are unchanged.

**Upgrade all five repositories together, as before.** **Upgrade every controller tool that loads into or prunes the same
PostgreSQL history at the same time** (`pqcota-ingest`, `pqcota-cbom-ingest` and `pqcota-prune`): the per-node lock only
works when both the loader and the prune tool take it, so an older loader or prune tool on the same history can still race
with the new ones. The collectors on the nodes can stay at v0.13.0.

### Fixed

- **A state that came back was not shown as the current one.** When a node's content went A → B → A, the third load
  reused the earlier snapshot A, as intended, but the latest snapshot stayed B, the one with the highest sequence number.
  *Which version it entered in:* v0.1.0. *What came out wrong:* reproduced on v0.13.0 with real collector output. After the
  third load, the load summary and the inventory view showed B, and `pqcota-provision` wrote B's app keys into the
  provisioning record. Now a node's latest snapshot is the one its most recent observation record points to, and
  `pqcota-inventory -history` marks that row `← current (last ingested)`.
- **The retention policy could delete the current state.** "The latest is untouchable" protected the snapshot with the
  highest sequence number. *Which version it entered in:* v0.1.0. *What came out wrong:* in the same reproduction,
  `pqcota-prune -keep-last 1 -apply` after A → B → A deleted A, the state just loaded, and kept B. Now the latest snapshot
  and the newest change point are both kept.
- **A load and a prune of the same node were not coordinated.** A prune that decided while a load was reusing an older
  snapshot could delete that snapshot, so the newest observation record pointed at nothing. *Which version it entered in:*
  v0.1.0. *What came out wrong:* not seen in a real run; a test that runs loads and prunes of one node at the same time
  failed in every run without the change. Now a load and a prune of a node take the same lock in Postgres, and the latest
  snapshot is chosen and read in one SQL statement, so a read during a load and a prune returns the state of one moment.

### Compatibility

- **History without observation records** keeps working as before: when a node has no observation record, when a snapshot
  was written after its newest observation record, or when that record's snapshot is gone, the newest snapshot is the
  latest.
- **No migration.** Existing histories are read with the new rule as they are. A node whose content came back reads as its
  last loaded state on the first run of the new version.
- The in-memory store (tests and single runs) follows the same rules: it reuses a snapshot from anywhere in the history,
  as the Postgres store always did.

### Built

- **The same tests on both stores.** A → B → A leaves two snapshots and three observation records, the latest is A, and
  `keep-last 1` keeps A and B, on the in-memory store and on Postgres.
- **Postgres only:** the three cases of history without observation records above; twenty rounds of two loads and a prune
  of one node at the same time, after which the newest observation record always points at a stored snapshot; and readers
  calling the latest snapshot about 50,000 times per run while a writer loads a new state and prunes 150 times, with no empty
  result. A two-statement read (choose, then read), tried during development and never published, showed no empty result in
  this test either; it failed only when a 1 ms delay was put between the two statements. So the read race was reproduced
  only under that condition.
- **Across the stages.** After A → B → A, a provisioning record takes its before state and app keys from A.
- **The reproduction above** on v0.13.0 and on this version: v0.13.0 shows B in the load summary, the view and the
  provisioning record, and pruning deletes A; this version shows A everywhere and pruning keeps A and B.
- **Go toolchain.** The published programs are built with Go 1.26.11 (the `toolchain` line in `go.mod`). `govulncheck`
  found no vulnerabilities in the five repositories.
- **Release assets** (checked after publishing): the checksums of the five bundles and `collector.jar` match `SHA256SUMS`;
  all 46 programs are built with Go 1.26.11, and `govulncheck -mode=binary` found no vulnerabilities in any of them. Their
  build information declares the pqcota modules at v0.13.1, but each of those modules points through `replace` to a local
  source directory (`(devel)`), and none of the 46 programs carries VCS information, so the build information alone does
  not show which commit was built. With the published controller tools, the A → B → A reproduction showed A in the load
  summary, the view and the provisioning record, and `keep-last 1` kept A and B.

### Known limitations

- The limitations of v0.13.0 are unchanged.

---

## v0.13.0 — One app key rule for every runtime, and edges attributed per connection (2026-11-12)

**Goal** — make an app key mean the same thing for every runtime, and give each observed connection to the application
that opened it. OpenSSL findings and network edges now choose the app key with the rule Java deployments have used since
v0.12.0. Network edges are attributed **per connection**, not per destination, and `observed_count` now counts
**distinct connection tuples**. **The contract fields are unchanged**; only their descriptions changed. **The ruleset is still
`pqcota-enrich/v4`.** Programs that changed: `pqcota-nodescan` (the OpenSSL app keys), `pqcota-netcap` (how edges are
attributed and counted) and `pqcota-inventory` (the diff now lists edges).

**Upgrade all five repositories together, as before,** and replace `pqcota-nodescan` and `pqcota-netcap` on the nodes.

### Fixed

- **The user services of one account shared one app key.** The app key of OpenSSL findings and network edges took the
  first `.service` or `.scope` in the process's cgroup path. For a user service that is the user manager,
  `user@<uid>.service`, so every user service of that account had the same key, and PID 1 had `init.scope`.
  *Which version it entered in:* v0.1.0. *What came out wrong:* reproduced on v0.12.0 with two user services of one
  account, `libssl.so.3` listed `user@1001.service` once instead of the two services. Now the key is the last `.service`
  in the path, a user service is named `user@<uid>.service/<unit>` (`app_key_kind: systemd-user-unit`), and a `.scope` is
  never used; a process without a service falls back to its executable path.
- **Services that talked to the same destination became one edge.** An edge's application was looked up by the
  destination address and port only, so the sockets of every application connected there were gathered. When they all had
  the same key the edge took it; when the keys differed the edge was left without an application. *Which version it
  entered in:* v0.3.0. *What came out wrong:* in the same reproduction, the connections of both services were one edge
  under `user@1001.service`. Now each connection is matched to its own socket by both endpoints (local and remote address
  and port), and the edges are grouped by the application found. The source port is used only for that lookup; it is not
  part of the edge, so a service that reconnects stays one edge.

### Edge attribution: what the diff shows

- **When attribution changes an edge's app key, the edge has a new id.** The diff lists the old edge under
  `edges removed` and the new ones under `edges added`. In the reproduction, the one v0.12.0 edge was replaced by three:
  `svc-a`, `svc-b`, and an edge whose application was not identified.
- **A connection whose application cannot be identified is still kept.** If its socket was already closed when it was
  looked up, or the socket is held by processes of different applications, the connection is counted on an edge with an
  empty app key. The completeness note gives the reason, and the diff writes it as `(app not identified)`. It is never
  given to another service talking to the same destination.
- The ruleset did not change, so **the diff shows no ruleset warning** for this. A diff across the upgrade of the
  collectors can mix in the effect of the attribution change: removed and added edges there are not, by themselves,
  applications that stopped or started talking.

### `observed_count`: what it counts

- **`observed_count` is now the number of distinct connection tuples whose handshake the capture observed** in its window
  for that edge. A tuple is both addresses and ports and the protocol; the observations of one tuple are counted once, so a
  tuple reused within the window is not counted as another connection. Up to v0.12.0 it counted handshake messages, so one
  connection added several. *Which version it entered in:* v0.1.0.
- **This shows in the diff differently from the attribution change.** An edge whose id stays the same can show a different
  count, under `edges changed` (`×N → ×M`). In the reproduction, eight connections were counted as `×23` by v0.12.0
  (handshake messages) and as `2 + 5 + 1` by this version (connections in the window).
- **So the counts before and after the upgrade cannot be compared.** A lower count after the upgrade does not mean that
  traffic went down.

### Source evidence: checked with real tool output

- The standard source location fields (`evidence.occurrences`) were checked against the real output of CBOMkit
  `cbomkit-action` v2.3.0 for a Java 21 example and a Python 3.13 example (`pyca/cryptography`), and of CryptoScan 1.5.0
  for a C example (OpenSSL 3 EVP). All were read without a preset, and the location chosen in a plan stayed the same
  reference through approval and into the generation record. The versions, what each tool did not report in these runs,
  and the C++ case (not checked) are in the
  [inventory command reference](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.md).

### Compatibility

- **The first snapshot after you replace `pqcota-nodescan` and `pqcota-netcap`** shows OpenSSL findings as `changed` (same
  ids, new app keys) and edges as removed and added, as described above. OpenSSL finding ids do not include the app key,
  so the number of findings and their ids stay the same.
- **Anything that refers to a user service by the app key `user@<uid>.service`** (a rule or a report kept outside
  pqcota) no longer matches it. The key is now `user@<uid>.service/<unit>`.
- Results from older collectors are read as before; nothing in the core changed how they are normalized.

### Built

- **A reproduction with fixed connection counts**, in one container, v0.12.0 first and then this version: three user
  services of one account connect to the same destination, one holding 2 connections, one 5, and one opening a single
  connection and closing it at once. This version reported `×2`, `×5` and one connection whose application was not
  identified, both in the collector output and in the stored result. For the third service the run confirmed that its
  handshake was captured (the collector named its source port) and that its socket had closed before the lookup.
- **Unit tests** for the cases the reproduction does not produce: a socket held by processes of two different services is
  left unattributed, a socket inherited by a child process keeps the parent's key, IPv4 connections on dual-stack sockets
  are found, and a socket opened after the process scan was cached is found by scanning again.
- **Go toolchain.** The published programs are built with Go 1.26.11 (the `toolchain` line in `go.mod`). `govulncheck`
  found no vulnerabilities in the five repositories.
- **Release assets** (checked after publishing): the checksums of the five bundles and `collector.jar` match `SHA256SUMS`;
  all 46 programs are built with Go 1.26.11, and `govulncheck -mode=binary` found no vulnerabilities in any of them.
  Their build information declares the pqcota modules at v0.13.0, but each of those modules points through `replace` to a
  local source directory (`(devel)`), and none of the 46 programs carries VCS information. The build information alone
  therefore does not show which commit was built. With the published `pqcota-nodescan`, `pqcota-netcap` and `pqcota-ingest`, the
  reproduction above reported `×2`, `×5` and one connection whose application was not identified, in the collector output
  and in the stored result, and `libssl.so.3` kept the two services as separate app keys.

### Known limitations

- **A socket held by processes of different applications** is left unattributed. This case was checked only in unit
  tests, not on a real system.
- **Short connections are still often unidentified.** The application is looked up after the observation window ends,
  so a connection that closed before then is counted without an application.
- The other limitations of v0.12.0 are unchanged.

---

## v0.12.0 — JCA deployment identity and JVM configuration variants (2026-11-12)

**Goal** — fix how Java runtimes are identified. Until now each JVM's asset was named after a short label of what it runs,
so two JVMs that shared the label became one asset, and one of them was lost. Now the JVMs that run the same application
form one **deployment**, which is one asset, and each JVM of it is kept inside that asset. **The contract changed by
additive fields** (`JcaAxes.instances`, `app_key_kind`, `observation_scope`, and the new messages `JcaInstance` and
`JcaModeCount`). **The ruleset is now `pqcota-enrich/v4`.** Programs that changed: `pqcota-jvmscan` (its output),
`pqcota-ingest` and the other programs that normalize (the new identity), `pqcota-inventory` (it shows the variants), and
`pqcota-provision` (when it records a before state).

**Upgrade all five repositories together, as before,** and replace `pqcota-jvmscan` on the nodes.

### Fixed

- **Two JVMs with the same name became one finding, and one of them was lost.** The asset name kept only the file name of
  `-jar`, so `/opt/a/app.jar` and `/opt/b/app.jar` were one asset, and two JVMs started from the same source with different
  settings were one asset too. *Which version it entered in:* v0.1.0. *What came out wrong:* reproduced with four real JVMs on
  v0.10.7, they became two findings, and a JVM without Bouncy Castle was not in the inventory. Now JARs with the same file name
  run from different paths stay separate findings, and when one deployment is observed with different provider chains, each
  chain is kept as its own variant. The same four JVMs are now three deployments, and the one that runs a source file with two
  settings keeps both chains.
- **Static fallback results are now reported as `static`.** When attach was not possible and the collector read `java.security`
  instead, it wrote `registration_mode: dynamic`.

### Deployments and variants

- **The deployment key** is chosen in this order: the systemd service (a system service by its unit name, a user service as
  `user@<uid>.service/<unit>`; a `.scope` is never used), the absolute path of `-jar`, the module of `-m`, the main class with
  its class path, the source file, and `JAVA_HOME`. The finding says which one was used (`app_key_kind`). A relative `-jar`
  whose working directory cannot be read keeps only the file name and is marked `jar-name`: JVMs with the same file name in
  different directories then still fall into one deployment, but none of them is dropped.
- **Variants.** JVMs of one deployment with the same `JAVA_HOME`, the same `-Djava.security.properties` setting and the same
  observed provider chain form one variant. **A variant's `count` is the number of JVMs in that variant**, and the deployment's
  JVMs are the sum of the counts. For example, a service with two JVMs, scanned with `--pid` so that only one was observed,
  has two variants with `count` 1 each: the observed one and the one without a chain. Two JVMs observed with the same chain
  are one variant with `count` 2. Each variant also counts how its JVMs were read (attach or static fallback).
- **One value per finding is kept only when it is one fact.** When the variants differ, the finding's `provider_set` is empty
  and its `pqc_readiness` and `fips_validation` read `varies (N variants)`; the values of each variant are in `instances`.
  Nothing is folded into a single worst value. An empty `provider_set` therefore has two meanings: with `instances`, the JVMs
  differ or some were not observed; without, the chain was not observed.
- **`pqcota-jvmscan --pid`** observes only the selected JVM. Its result is marked `observation_scope: selected-pid`, and the
  other JVMs found in the same deployment are kept as variants without a chain.
- `pqcota-inventory` shows the variants in the detail and the diff, so a change in a variant's `count`, or in how its JVMs were
  read, appears as `changed`.

### Provisioning records

- A deployment's provider chain is recorded as the before state **only when it stands for the whole deployment**: every JVM
  observed, with one chain. When the JVMs differ, some were not observed, or the result came from `--pid`, the record keeps no
  chain, its note says why (`before not captured: …`), and the summary counts it in `before not captured`. Generation goes on.

### Compatibility

- **Older inputs keep their ids.** Results from collectors before v0.12.0 are normalized with the old rule: their finding ids
  and fingerprints are the same as in v0.11.0 (checked against v0.11.0 with a real collector output).
- **The first snapshot after you replace `pqcota-jvmscan`** has new ids for Java assets. Its diff against the last v0.11.0
  snapshot shows the old assets as removed and the deployments as added, with the ruleset warning. That is the identity change,
  not applications removed or added.
- **Upgrade the tools that read the same database before v0.12.0 writes new snapshots.** v0.11.0 `pqcota-inventory` cannot open
  the history of a node once it holds a new snapshot (`unknown field "instances"`).
- **v0.11.0 `pqcota-provision --dsn` can write the playbook to stdout and then fail with exit 1** on such a database, without
  writing a record. In automation, apply a playbook only after checking that the exit status is 0.
- `pqcota-records` from v0.11.0 reads the new records, including the new `before not captured` notes.

### Built

- **Tests across the stages.** The collector's output is carried through normalization and into a provisioning record on a
  real Postgres.
- **A full scan, a `--pid` scan and a full scan of the same deployment** keep one finding id, show two changes, and fill the
  before state again at the end.
- **Real JVMs.** The four-JVM reproduction above, and two users each running `foo.service` as a user service, which stay two
  deployments with two finding ids.
- **Go toolchain.** The published programs are built with Go 1.26.11 (the `toolchain` line in `go.mod`). `govulncheck` found no vulnerabilities in the five repositories.
- **Release assets** (checked after publishing): the checksums of all six files match `SHA256SUMS`; all 46 programs are built with Go 1.26.11, and `govulncheck -mode=binary` found no vulnerabilities in any of them. Their build information declares the pqcota modules at v0.12.0, but each of those modules points through `replace` to a local source directory (`(devel)`), and none of the 46 programs carries VCS information. The build information alone therefore does not show which commit was built (corrected after the v0.13.0 asset check). The published `pqcota-jvmscan` with `collector.jar` reported two JVMs of one `-jar` path as one deployment (one variant, `count` 2) and a JAR with the same file name in another directory as a separate deployment.

### Known limitations

- **A weak `jar-name` key can still group JVMs.** When the working directory of a relative `-jar` cannot be read, only the file
  name is kept, and JVMs with that file name in different directories fall into one deployment. There, JVMs whose variant key
  is also the same are added to one variant's `count`; only JVMs whose variant key differs show as separate variants.
  An absolute `-jar` path, or a relative one whose working directory is readable, stays separate.
- The app key of OpenSSL findings and network edges is unchanged: on a desktop session it can still group processes under
  `user@<uid>.service`. See the roadmap.
- On Windows the command line is not read, so the deployment key falls back to `JAVA_HOME` (a weak key), as before.
- The other limitations of v0.11.0 are unchanged.

---

## v0.11.0 — CBOM source evidence and approved migration guides (2026-11-11)

An external CycloneDX CBOM can now be kept as **source evidence**: the crypto items it reports and the source locations it gives, for
review, without turning it into a runtime snapshot. A plan's action can point at that evidence and at the migration guide its author chose,
and **the approval signature covers both the source evidence and the chosen guide**: changing either, or moving it to another action,
invalidates the approval. The generated playbook shows the reported locations and the approved guide.

**Upgrade all five repositories together, as before.** One release is five tags.

### Source evidence
- `pqcota-cbom-ingest <cbom> [node] --source-evidence [--preset <id>] [--source-map <file>] [--dry-run] [--json]` stores the CBOM as
  received, the mapping files used, the conversion and a receipt, in one transaction for the organization. No snapshot is created, so the
  runtime view, the history and the provisioning before-capture do not change. Read it back with the new command `pqcota-source-evidence`
  (list, detail, `--input`/`--map` export, `--presets`).
- The standard `evidence.occurrences` are always read. For tools that put locations in `properties`, a mapping file
  (`pqcota-source-map/v1`) says where to read them; two presets come built in (`cdxgen-dotnet-sourcelocation@1`,
  `dosai-cyclonedx-location@1`), each checked only against the tool version it names.
- A location is shown as a **reported location for review**. Its kind (call, declaration, comment) is not supplied, and numbers are
  turned into 1-based values only when the counting base is known.
- Limits: the CBOM at most 32 MiB (exit 1 over it), a mapping file at most 256 KiB (exit 2). A mapping you name must convert without
  errors, or nothing is stored.

### Plans: source evidence and an approved guide on an action
- `RemediationAction.source_evidence` points at stored evidence (conversion, item, optionally one location, the CBOM's hash). An action
  based only on source evidence is accepted for `APP_RECONFIG` and `REBUILD` only, and all of its references must resolve before it is
  approved and again before anything is generated.
- `RemediationAction.guide` names the guide the author chose, by id, revision and the sha256 of its manifest and body, with the purpose and
  conditions the author confirmed and which evidence it was chosen for. Guides are compared by hash with a local, versioned catalog
  (`--guide-dir`; the pqcota catalog is `docs/guides/catalog/`). Nothing is fetched from a URL and another revision is never used instead.
- `pqcota-approve --review` prints what would be approved, including **the full text of each verified guide body**, without signing.
  A plan with a guide is signed only when every guide matches the catalog and the evidence it was chosen for resolves.
- `pqcota-provision` writes the reported locations and the guide into the playbook as comments. **It says `approved guide` only when the
  run verified the approval signatures**; with `--allow-unverified-approvals` the same guide is shown as named in the plan, not approved.

### Getting the guide catalog
The controller bundle holds the CLIs only. Take the catalog from the `v0.11.0` source and point `--guide-dir` at it:

```bash
curl -L https://github.com/randyinthedev-hash/pqcota/archive/refs/tags/v0.11.0.tar.gz | tar -xz pqcota-0.11.0/docs/guides/catalog
pqcota-approve --review --guide-dir pqcota-0.11.0/docs/guides/catalog plan.json
```

The tools compare the catalog with the hashes in the plan, so a wrong or changed copy is refused rather than used.

### Compatibility
- **Plans without the new fields** keep the same canonical form; existing approvals stay valid (frozen by tests).
- **Plans with the new fields** are refused by the v0.10.7 `pqcota-approve` and `pqcota-provision` at parse time (`unknown field`), before
  any signature check. A consumer that drops unknown fields fails the signature check instead — but only if it actually verifies the
  approval signatures.
- **Records**: `ProvisioningRecord` gains `source_evidence_resolutions` and `guide_resolution`. The v0.10.7 `pqcota-records` fails
  when the records it lists include a row with `sourceEvidenceResolutions` or `guideResolution`; rows without them still read.
  **Upgrade the tools that read records from the same database before v0.11.0 writes new records.**
- **Action ids must be non-empty and unique in the whole plan.** A plan that breaks this is refused at approval and at generation.
  Before, two actions sharing an id silently dropped the second action's config from the playbook while the run reported success. To fix
  such a plan: give every action a unique id, check the activation hooks that refer to the config paths built from those ids
  (`openssl-pqc.<id>.cnf` and the like), and approve the plan again — changing an id changes the canonical form, so the old approval no
  longer holds.
- `pqcota-cbom-ingest`: any option now routes to the source evidence path, and an unknown option exits with 2 instead of falling back to
  writing a snapshot. A file name that starts with `-` needs a path, such as `./-name.json`.
- `pqcota-approve` now prints a summary of what it signs on stderr.
- CBOM ingest records the `specVersion` the document declares (1.6 or 1.7); it used to record 1.6 for both.
- Before this release, `main` changed how a conversion's id is computed. A database that stored source evidence with the ingest CLI from
  before that change keeps those results under the old ids; receiving the same input again adds a result under the new id.

### Built

- **The published programs are built with Go 1.26.11** (the `toolchain` line in `go.mod`). Go 1.26.11 fixes GO-2026-6859 in `net/http`, where an HTTP/1 server could crash on a header race in `MaxBytesReader`. With Go 1.26.10, `govulncheck` reported it in all five repositories.

### Known limitations

- The guide catalog has one guide, in English. Choosing a guide is the plan author's decision; pqcota does not suggest one.
- The limitations of v0.10.7 are unchanged.

---

## v0.10.7 — A security fix for plan approvals, and nodes with several JVMs can be ingested (2026-11-10)

**Goal** — fix three defects. One is a security defect in plan approvals. **No contract changed** and the ruleset stays `pqcota-enrich/v3`. Programs that changed: `pqcota-approve` and `pqcota-provision` (approval signing and checking), `pqcota-records` (how it prints an empty before state), and `pqcota-jvmscan` (its output format). `collector.jar` did not change.

**Upgrade all five repositories together, as before.** One release is five tags. Replace `pqcota-jvmscan` on the nodes as well.

### Security

- **An approval signature could pass on a different plan.** The canonical form that approvals sign separated fields with a NUL byte and no length, and no field was checked for NUL. So two different plans could have the same canonical form, for example `provider_choice="BC\u0000X"` with `config_artifact="Y"`, and `provider_choice="BC"` with `config_artifact="X\u0000Y"`. *Which version it entered in:* v0.7.0, the release that added approval signatures. *What came out wrong:* reproduced on v0.10.6, an approval made with `pqcota-approve` for one plan was accepted by `pqcota-provision` on the other, and the two plans generated different playbooks. The playbooks were not applied to a node. Now a plan with a NUL character in any string is refused when it is signed, when its approvals are checked, and when it is judged as grounds for provisioning, also with `--allow-unverified-approvals`. When the check fails, no approval on that plan counts as verified.
- **Plans without a NUL character are not affected.** Their canonical form is byte for byte the same, and their existing approvals stay valid. A plan that contains a NUL character must be corrected and approved again.

### Fixed

- **`pqcota-jvmscan` wrote several results over many lines.** On the attach path it meant to write one result per line, but the JSON encoder turns on multi-line output whenever an indent is set. *Which version it entered in:* v0.1.0. *What came out wrong:* on a node with two or more running JVMs, `pqcota-ingest` could not read the file line by line and refused **the whole ingest request, every node in it** (`refusing to ingest a partial set`); the inventory kept showing the state from before. With one JVM the whole file read as one object, so it did not show. It was not a silent loss: the ingest ended in failure. The ingester's input format did not change.
- **`pqcota-provision --dsn` recorded the before state of the whole node, not of the action's finding.** *Which version it entered in:* v0.1.0. *What came out wrong:* with several JVMs on a node, the record's `before.provider_chain` was the chains of every JVM joined into one list (reproduced: 25 entries where the action's finding had 13), and `before.modules` held every module on the node. The rollback path in this repository does not use these values; what was wrong is the accuracy of the record. Whether a consumer outside this repository reads them was not checked. Now the before state comes from the action's finding only, looked up in the target node's latest snapshot. If the finding is not there, the before state is left empty and nothing else is put in its place: a warning is printed, the record's note says so, the summary line counts it (`before not captured N`), and `pqcota-records` prints the empty before state as nothing captured. An empty before state is not evidence that the app has no crypto configuration.

### Built

- **Tests.** A plan without NUL keeps the canonical form and the approval that v0.10.6 produced (fixed values); a NUL in each of the 23 string positions of the current contract is refused both when signing and when checking; the reproduced pair is refused, with and without `--allow-unverified-approvals`, and produces no output; the collector's output with several results is read by the ingester, and a damaged line still refuses the whole request; the record captures only the action's finding, and says so when it cannot. The string check walks every string field of the plan, so a new string field is covered; if the canonical form ever takes a value that is not a string, or changes its layout, the check and its tests have to be revisited with it.

### Known limitations

- **Two JVMs with the same name still become one finding.** With the output fixed, nodes with several JVMs now ingest, and the identity defect that the refusal used to hide becomes visible: two JARs with the same file name in different directories, or the same source file started with different settings, become one finding that keeps the most recent result. Reproduced: four JVMs became two findings, and a JVM without Bouncy Castle was not in the inventory. The completeness note says that one collector reported different content for one identity; it does not say which JVM was dropped. JCA identity is designed separately.
- The other limitations of v0.10.6 are unchanged.

---

## v0.10.6 — Two installations on one machine stay two assets, and the JCA readiness counts what the providers register (2026-11-05)

**Goal** — fix three defects in how observations are identified and graded, all present since v0.1.0. **The contract changed by one additive field:** `OpensslAxes.path`. **The ruleset is now `pqcota-enrich/v3`**, and some finding ids change with it; read "What changes for a consumer" before upgrading. Programs that changed: `pqcota-jvmscan`, `pqcota-netcap`, `collector.jar`, and every program that normalizes or reads snapshots (`pqcota-ingest`, `pqcota-inventory` and the others built from `pqcota-inventory`).

**Upgrade the five repositories together, and replace the collectors on the nodes.** One release is five tags, as before: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning` and `pqcota` all carry `v0.10.6`. **Back up the database before upgrading** (see below).

### Fixed

- **`pqcota-jvmscan` and `pqcota-netcap` run without a node id attributed the machine to another node.** They used `host://local` instead of the self-id derived from the machine fingerprint, as `pqcota-nodescan` and `pqcota-cngscan` do, and they did not attach the fingerprint to the result. *Which version it entered in:* v0.1.0. *What came out wrong:* on a machine observed this way, the JVM and network results landed under `host://local` while the OpenSSL results landed under the machine's own id, and the ingest's node-id duplicate and collision check skipped those results because they carried no fingerprint. Runs that pass a node id, which the Ansible playbook and the demo do, were not affected. The rule now lives in one place for both collectors.
- **Two OpenSSL libraries with the same file name on one node became one asset.** The finding id was built from the node, the name, the runtime and the fork, not the path, so a system `libssl.so.3` and a `libssl.so.3` bundled with an application got the same id and normalization kept one of them. *Which version it entered in:* v0.1.0. *What came out wrong:* the inventory showed one installation where there were two, and the completeness note gave the cause as "the collectors disagree". The id now includes the library path when the input has one. When one collector reports two different contents for one id, the note now says so without naming a cause.
- **The JCA readiness counted JDK providers by name.** The registry matched `SunJCE` and `SunEC` by name and credited both with ML-KEM and ML-DSA. `SunEC` serves neither in any version; ML-KEM is served by `SunJCE` and ML-DSA by `SUN`, and only in JDK builds that register them. *Which version it entered in:* v0.1.0. *What came out wrong:* a JVM that registers no ML-KEM service, for example Eclipse Temurin 21.0.11, was graded `provider-augmented (SLH-DSA gap)`. `provider-augmented (all standard algorithms)` did not check ML-DSA.

### What changes for a consumer

- **The ruleset is `pqcota-enrich/v3`, and the id of an OpenSSL finding that has a path changes.** Comparing a v2 snapshot with a v3 snapshot can therefore show **removed** and **added** entries that come from the change of identity rule. Do not read them alone as an installation removed or added; a real installation change may have happened in the same period too. Comparing new snapshots one after another, the effect shows once, at the point where the ruleset changed. Comparing against an old v2 baseline, it keeps showing. `pqcota-inventory -diff` warns when the rulesets differ, but that warning does not replace this note.
- The id of a finding without a path (an external CBOM, a re-normalization of old raw results) does not change.
- The first ingest after the ruleset changes creates a new snapshot even if nothing changed on the node. It is a trace of the migration, not a change.
- **An older reader refuses a snapshot that holds the new field.** pqcota stores findings as ProtoJSON, which rejects unknown fields: a v0.10.5 reader fails with `proto: unknown field "path"` on a finding that has a path. A finding without a path still reads.
  - **Upgrading:** upgrade the five repositories together. If one stays on the old version, its reader refuses the newly ingested snapshots.
  - **Downgrading:** a database that has stored findings with `path` can fail in a v0.10.5 reader. To downgrade, restore the backup taken before the upgrade. Restoring loses what was recorded after the backup, so keep that separately.
  - **Downstream:** anything that reads the finding JSON pqcota writes must move to the v0.10.6 contract first.
  - `DiscardUnknown` is not turned on to avoid this. It would drop unknown fields without a mark, and "read" would no longer mean "read all of it".
- **Downstream consumers rebuild their joined identifiers**, as on every ruleset change.
- **The JCA `pqc_readiness` counts the standard PQC services each provider registers.** A JVM whose services were observed and include no standard PQC algorithm is now `none (no standard PQC service observed)`. One that registers ML-KEM and ML-DSA, such as Temurin 21.0.13 (backported) or 24 and later, stays `provider-augmented (SLH-DSA gap)`. Some but not all of the three gives `partial (only … observed)`. `provider-augmented (all standard algorithms)` now needs all three. What is observed is the list of registered services, not that an operation succeeds or that the algorithm is used on the wire. The finding id does not change, only the value, so a comparison with a v2 snapshot shows the JCA finding as **changed** when nothing on the node changed. The snapshot fingerprints (`ContentHash`, `ContentHashV1`) include the readiness and change with it.
- **Upgrade the collectors with the core.** The service list arrives in a new CBOM property, `pqcota:jca.pqc_services`. Results of the jvm collector and `collector.jar` before v0.10.6, and of the static fallback, do not have it; for them the JDK's built-in providers are not counted, so without Bouncy Castle the readiness is `unknown`. The same holds when re-normalizing old raw results. Replace the `collector.jar` deployed on the nodes as well.
- **Observation is judged per provider.** If only some providers in a chain were observed and none could be counted, the readiness is `unknown`, not `none`.
- **Bouncy Castle**, when observed, is counted by the services it registers. When it was not observed, it is counted by the registry as before, without checking its version. The readiness string does not say which of the two it came from.
- **A consumer that filters on the readiness string** must handle the new values `none (…)` and `partial (…)`, and the fact that a JVM with only the JDK's providers now grades differently by build.
- Data made with ruleset v3 from `main` before this release may differ in the JCA readiness; v3 appears in a release for the first time here.

### Built

- **Contract:** `OpensslAxes.path`, the absolute path of the library file. Empty means the input did not know the path; it is not filled in by guess.
- **The jvm collector and `collector.jar`** print, per provider, the registered service names that contain ML-KEM, ML-DSA or SLH-DSA, and the parser carries them as `pqcota:jca.pqc_services`. A provider observed with none is written `SunEC=`; a provider not observed has no key.
- **Dependencies: `golang.org/x/net` v0.60.0 → v0.62.0**, and `x/sys` v0.49.0, `x/text` v0.43.0 and `x/sync` v0.24.0 rise with it, in all five repositories. This is a dependency update, not a fix of a vulnerability the programs reach. v0.62.0 fixes two advisories in `golang.org/x/net/html`, GO-2026-6866 and GO-2026-6860. No pqcota program imports that package, and a source scan finds them only at the level of the required module, not in any imported package. The update is made so that the required version no longer carries a known advisory. The Go toolchain stays at 1.26.10, still the latest 1.26 patch; the `go` line stays at 1.26.4.
- **Tests.** Node-id rule; two OpenSSL installations with one file name; overlap within one collector; findings with and without a path on one node; readiness for observed, partly observed and unobserved chains. The new tests were confirmed to fail on the code before the fix. The JCA change was also run against real JVMs (Eclipse Temurin 21.0.11, 21.0.12, 21.0.13 and 25.0.5; probe, attach, and attach with Bouncy Castle 1.79 registered at run time).

### Known limitations

- **Two JVMs with the same name still become one asset**: the same JAR started from different directories or with different settings, or the same main class. When their contents differ, the completeness note says that one collector reported different content for one identity and that the most recent was kept; it does not name a cause. JCA identity is designed in a later release.
- The readiness was checked on Eclipse Temurin only. Other distributions may have backported at different points and were not checked. Not checked either: Windows, and an end-to-end run through ingest, diff and history.
- `JcaAxes.jdk_version` is still empty.
- `go install` and `go run` with `@v0.10.6` still fail because of the `replace` lines in `go.mod`; importing a package as a library works. See the [build guide](docs/build.md).

---

## v0.10.5 — The published programs are rebuilt with Go 1.26.10 and golang.org/x/net v0.60.0, and `pqcota-provision --dsn` keeps its records in the organization (2026-10-22)

**Goal** — take the vulnerabilities of the Go standard library and of `golang.org/x/net` announced since v0.10.4 out of the published programs, and fix the defects found while preparing the functional test. **No contract changed:** the `.proto` files and the generated code differ from v0.10.4 only in the license header comment. Two programs changed: `pqcota-provision`, in how `--dsn` opens the record store, and `pqcota-netcap`, in the note it prints under `--strict`. Two scripts changed: `examples/provisioning/files/verify-registration.sh` and `demo/scripts/demo.sh`.

**If you run a published program, replace it.** The programs of v0.10.4 were built with Go 1.26.6 and `golang.org/x/net` v0.58.0, and they are not rebuilt. Download the v0.10.5 files, check them against `SHA256SUMS`, and put them where the old ones were. `collector.jar` is Java code and is not a target of this Go fix.

**One release is five tags**, as before: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning` and `pqcota` all carry `v0.10.5`.

### Fixed

- **The published programs contained known vulnerabilities of the Go standard library and of `golang.org/x/net`.** Go 1.26.9 and `golang.org/x/net` v0.60.0 fix them; the advisories were published on 2026-10-08. *Which version it entered in:* not traced. We confirmed it in the files of v0.10.4; the files of earlier releases were not scanned. *What came out wrong:* scanning the 44 executables of v0.10.4 (both Linux architectures and Windows) for the symbols of known vulnerabilities finds 12 advisories in each Linux file and 13 in each Windows file, in `net/http`, `crypto/tls`, `net/textproto`, `html/template` and `os`, some of them also through `golang.org/x/net/http2`. A finding of that kind says the vulnerable code is in the file. It does not say that a path to it exists, and it does not say that anyone can attack the program. A scan of the source finds call paths into seven of them in all five repositories: GO-2026-6603, -6607, -6608, -6611, -6612, -6613 and -6617, in `net/http`, `crypto/tls` and `net/textproto`. We did not assess whether any of them can be reached with what a program is given to read, and the fix does not depend on that.
- **The build moves to Go 1.26.10, the latest patch release.** Go 1.26.10 (2026-10-19) adds a security fix to `crypto/x509`. The Go vulnerability database had no entry for it when we scanned (database of 2026-10-20 00:20 UTC), so no scan above reports it, and we do not say whether the programs use the affected code.
- **`pqcota-provision --dsn` opened the record store without the organization.** It opened the history in `PQCOTA_ORG` but the record store in no organization. *Which version it entered in:* v0.2.0, the release that added organizations. *What came out wrong:* in a deployment that uses organizations, the rollback evidence went to the `default` organization and `pqcota-records` did not find it; with `PQCOTA_REQUIRE_ORG=1` this path did not open. It is the same kind of defect as the one fixed in `pqcota-hosts` and `pqcota-profile` in v0.10.1.
- **`verify-registration.sh` run without arguments failed with no message.** It passed the example plan, which is waiting for approval, to `pqcota-provision` without approving it, and it threw away the error. *Which version it entered in:* v0.8.0, the release in which the example plans became plans that wait for approval. *What came out wrong:* the script ended with exit code 1 and printed nothing. It now creates a key, approves the plan with it and checks the approval with the same key, as `run.sh` does, and it stops with the reason when approval or generation fails.
- **`pqcota-netcap --strict` said it would exit successfully while exiting 1.** When the capture socket could not be opened, the note on stderr was the same with and without `--strict`, so a `--strict` run ended with exit code 1 while saying "that is why this exits successfully … use --strict to fail instead". *Which version it entered in:* v0.1.0 (then in Korean). *What came out wrong:* two runs with the same output and different exit codes could not be told apart from the screen. The gap record on stdout and the exit codes were right. With `--strict` the last line now reads "--strict: exiting 1".
- **The example README called the OpenSSL 3.5 config-only output "one line".** It is a configuration snippet wrapped in the `openssl_init` and `ssl_conf` sections. *Which version it entered in:* not traced.
- **`demo.sh` read the database without the organization.** When the same database held a snapshot of another organization, the demo could pick that snapshot and stop at step 5/6 with "no such snapshot". *Which version it entered in:* v0.2.0, the release that added organizations; the SQL itself dates from v0.1.0.
- **`sha256sum -c SHA256SUMS` in the release body and the roadmap lacked `--ignore-missing`.** For someone who downloaded one file, it reported every other file as missing and failed. *Which version it entered in:* v0.1.0. The bodies of releases already published are not edited.
- **Documents corrected.** Sentences that did not match the code and wording that could be read as denying what the program does were rewritten in the five repositories: 35 review items, addressed in the English and Korean documents. *Which version it entered in:* differs per item, not traced.
- **The license guide stated legal conclusions.** Sentences such as "GPL copyleft contagion is blocked structurally" now state the technical separation, and what needs a separate legal review is listed apart from it, in `pqcota` and `pqcota-common`, English and Korean. *Which version it entered in:* not traced.

### Built

- **Copyright and license notices.** Source files carry an SPDX header, and each repository has `AUTHORS`, `REUSE.toml`, `LICENSES/Apache-2.0.txt` and a copyright line in its README. The repositories follow REUSE 3.3.
- **Tests.** The approval signature is checked against a registered key; a plan with a valid and an invalid signature is rejected; `PQCOTA_REQUIRE_APPROVAL` does not change the exit code, stdout or stderr; the `--dsn` path runs when `PQCOTA_TEST_DSN` is set.
- **`go.mod` of each repository now has `toolchain go1.26.10`**, and `golang.org/x/net` is v0.60.0; `x/sys`, `x/sync` and `x/text` rise with it. The `go` line stays at 1.26.4. gRPC stays at 1.83.2 and protobuf at 1.36.11; no advisory is published for them.
- **The demo image builds with Go 1.26.10**, both the builder stage and the controller.

### How to check a file

`go version -m <file>` prints the Go version and the module versions a program was built with, and `govulncheck -mode=binary <file>` scans it for known vulnerabilities.

### Known limitations (unchanged)

- The files attached to earlier releases are not rebuilt. They stay as they were built.
- `go install` and `go run` with `@v0.10.5` still fail because of the `replace` lines in `go.mod`; importing a package as a library works. See the [build guide](docs/build.md).
- The limitations of v0.10.3 about Korean search and the limitations of v0.10.1 about `PQCOTA_AUTO_DDL=0` and a non-positive observation window are unchanged.

---

## v0.10.4 — The published programs are rebuilt with Go 1.26.6 and gRPC 1.83.2 (2026-10-02)

**Goal** — take the known vulnerabilities of the Go standard library and of gRPC out of the published programs. **No program code and no contract changed.** The commands, the `.proto` files and the generated code are the same as in v0.10.3; the one Go file edited is the documentation check `tools/checkdocs`, which is not shipped. What changed is the toolchain the programs are built with and the version of one library; the documents state the new release number.

**If you run a published program, replace it.** The programs of v0.10.0 to v0.10.3 were built with Go 1.26.4 and gRPC 1.82.1, and they are not rebuilt; neither are those of earlier releases. Download the v0.10.4 files, check them against `SHA256SUMS`, and put them where the old ones were. `collector.jar` is Java code and is not a target of this Go and gRPC fix.

**One release is five tags**, as before: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning` and `pqcota` all carry `v0.10.4`.

### Fixed

- **The published programs contained known vulnerabilities of the Go standard library.** Built with Go 1.26.4, they carried the code of ten advisories in `crypto/tls`, `encoding/asn1`, `encoding/xml`, `net`, `net/http`, `net/url`, `html/template` and `os`. Go 1.26.5 fixes two of them and Go 1.26.6 fixes the other eight; a standard-library fix reaches a program only when the program is rebuilt, so updating a module does not remove it. *Which version it entered in:* every release built with a Go older than the fixing version. v0.10.0 to v0.10.3 were all built with Go 1.26.4; earlier releases come from before the repository was split and were not examined here. *What came out wrong:* scanning the 23 executables of v0.10.3 for the symbols of known vulnerabilities finds 13 in each of them, ten in the standard library and three in gRPC. A finding of that kind says the vulnerable code is in the file. It does not say that a path to it exists, and it does not say that anyone can attack the program. A scan of the source did find call paths into `crypto/tls` and `encoding/asn1` in all five repositories, and into `encoding/xml` in three of them. We did not assess whether any of them can be reached with what a program is given to read, and the fix does not depend on that.
- **gRPC 1.82.1 has three published advisories:** GHSA-2v4p-qf9q-27wj (a crash in xDS servers), GHSA-vp52-pcj8-j9qc (memory exhaustion through fragmented HTTP/2 data frames) and GHSA-qc2q-p7wx-3px3 (a bypass of the xDS RBAC filter). gRPC is now 1.83.2, the lowest release that closes all three; the first advisory still affects 1.83.0 and 1.83.1. The programs start no gRPC server and use no xDS: `grpc.NewServer` and `grpc.NewClient` appear only in two tests, over in-process connections. This holds for the programs we publish. A consumer who imports these packages and builds a gRPC server of their own is not covered by it. `golang.org/x/net`, `x/sys`, `x/text`, `x/sync` and `genproto/googleapis/rpc` rise with gRPC.

### Built

- **`go.mod` of each repository now has `toolchain go1.26.6`.** The `go` line stays at 1.26.4, because it is the minimum a consumer of the module must have and raising it would force 1.26.6 on them. The official CI and the release build with Go 1.26.6. The `toolchain` line is a recommendation: which Go your own machine uses follows your `GOTOOLCHAIN` setting. See the [build guide](docs/build.md).
- **The demo image builds with Go 1.26.6**, both the builder stage and the controller.
- **`make check-docs` accepts either the `go` line or the `toolchain` line** as the Go version a document may state.

### How to check a file

`go version -m <file>` prints the Go version and the gRPC version a program was built with, and `govulncheck -mode=binary <file>` scans it for known vulnerabilities.

### Known limitations (unchanged)

- The files attached to earlier releases are not rebuilt. They stay as they were built.
- `go install` and `go run` with `@v0.10.4` still fail because of the `replace` lines in `go.mod`; importing a package as a library works. See the [build guide](docs/build.md).
- The limitations of v0.10.3 about Korean search and the limitations of v0.10.1 about `PQCOTA_AUTO_DDL=0` and a non-positive observation window are unchanged.

---

## v0.10.3 — The Korean documents are complete, and four statements are corrected (2026-10-02)

**Goal** — give the Korean reader the whole documentation, and correct four statements that said more than the code does. **No code and no contract changed.** The commands, the `.proto` files and the generated code are the same as in v0.10.2; the stage repositories differ only in their documents and in the release number those documents state.

**One release is five tags**, as before: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning` and `pqcota` all carry `v0.10.3`.

### Built

- **Korean documents for everything but the release notes and the third-party notices.** Added to the five of v0.10.2: the developer home page, the build guide, the environment-variable reference, the guide to changing a contract, the checks and gates, the compatibility policy and the licensing guide; the README, command reference and examples of each stage repository (and the contract overview and data model of `pqcota-common`); `CONTRIBUTING`, the code of conduct and the security policy; and the guides under `demo/`. That is 38 translated documents, served as 40 pages of the Korean site (`/pqcota/ko/`) with a language switch on all 38 pairs. The third-party notices stay in English because they are legal notices.
- **The Korean site and the English site now show the same set of pages.** The English addresses that already existed are unchanged.
- **The landing page** has the source link at the top and the five repositories as cards; with Korean selected, its links go to the Korean pages.

### Fixed

These are statements in the documents, not behavior. Nothing the programs do changed.

- **The clean-up of the discovery playbook was described as total.** The documents said the playbook leaves nothing on the node. It removes only its staging directory (`/tmp/pqcota-collector`), and only when the run completes without failure; after a failure the directory stays. On the Java attach path the observed JVM also writes `/tmp/pqcota-providers-<pid>.txt` in its own `/tmp`, which nothing removes. *Which version it entered in:* not traced to a release. *What came out wrong:* a reader could believe a node was clean when it was not.
- **The security policy named the wrong thing as signed.** It said the history records carry the ed25519 signature. The signatures are on a collection result (the collector signs it, and `pqcota-ingest` checks it when a verification key is configured) and on a plan approval (`pqcota-approve` signs it, and `pqcota-provision` checks it). History records are not signed. *Which version it entered in:* not traced to a release. *What came out wrong:* a reader could believe stored history carries a signature it does not carry.
- **The licensing guide named a CI license scanner that does not exist.** It said such a scanner blocks a cross dependency. No license scan runs in CI; the separation is kept by review.
- **The provisioning examples gave the wrong rollback order.** They said rollback is the exact reverse of apply. The code and its test run `pre`, deactivate, remove, `restart`. The demo guide also said six steps and an L2 playbook; the demo has seven steps (`0/6` to `6/6`) and generates L2 and L3 playbooks, and the recording guide had the wrong path of the provenance file (`out/clips/<lang>/PROVENANCE.txt`).

### Known limitations (unchanged)

- Search in Korean is by prefix, so a word with a particle attached matches fewer pages. The check that a Korean page marks its links to English pages works per block, not per link.
- `go install` and `go run` with `@v0.10.3` still fail because of the `replace` lines in `go.mod`; importing a package as a library works. See the [build guide](docs/build.md).
- The limitations of v0.10.1 about `PQCOTA_AUTO_DDL=0` and a non-positive observation window are unchanged.

---

## v0.10.2 — The documentation site goes live, in English and Korean (2026-10-02)

**Goal** — publish the documents of v0.10.1 as an online site, and give the people who read Korean the
five documents they most need in Korean. **No code and no contract changed.** The commands, the `.proto`
files and the generated code are the same as in v0.10.1; the stage repositories differ only in the
release number their documents state.

**One release is five tags**, as before: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`,
`pqcota-provisioning` and `pqcota` all carry `v0.10.2`.

### Built

- **An online documentation site**, <https://randyinthedev-hash.github.io/pqcota/>, built from the
  Markdown of the five repositories at one release tag, with the tools pinned to a commit. Each page shows
  the release it was built from. The addresses that already existed (`/pqcota/`, the structure diagram and
  the raw `.md` files) are unchanged. The site is built and published by hand from the `site` workflow;
  a push does not publish anything.
- **Korean documents**: the root README (`README.ko.md`), the primer on PQC migration, the FAQ, the
  reporting guide and the demo guide. They are served under `/pqcota/ko/`, with a language switch on the
  pages that have a counterpart. English stays the original; a Korean page that points to a document not yet
  translated points to the English page and says so.
- **A new landing page** with a Korean toggle, the five repositories as cards, and a link to the source at
  the top.
- **A demo video in English and in Korean**, and the pipeline that makes both from one script
  (`demo/recording`). The generated files are not kept in the repository.
- **A prose gate that also reads `.json` and `.sh`**, so the video captions and recording scripts follow
  the same rules as the documents.

### Known limitations

- **Search in Korean is by prefix.** A word with a particle attached (a postposition) matches fewer pages
  than the bare word.
- **The check that a Korean page marks its links to English pages works per block, not per link**, and it
  ignores external links.
- **In the Korean video the terminal output is English**, because the programs print English.
- **`go install` and `go run` with `@v0.10.2` still fail** because of the `replace` lines in `go.mod`;
  importing a package as a library works. See the [build guide](docs/build.md).
- Both limitations of v0.10.1 about `PQCOTA_AUTO_DDL=0` and a non-positive observation window are unchanged.

### Not done

- The developer documents and the per-stage references are not translated.

---

## v0.10.1 — Two commands ignored the organization, and the documents were rewritten (2026-10-01)

**Goal** — fix two commands that wrote to the wrong organization, and replace the documents written for
the people who built this with documents written for the people who use it. **No contract changed.** The
only edits to the `.proto` files and the generated code are comments; no field, number, type or enum value
differs from v0.10.0.

**One release is five tags**, as in v0.10.0: `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`,
`pqcota-provisioning` and `pqcota` all carry `v0.10.1`.

### Fixed

- **`pqcota-hosts` and `pqcota-profile` ignored `PQCOTA_ORG`.**
  - *What was wrong.* When either command opened the metadata store through `--dsn` (endpoints and
    profiles), it did not pass the organization on, so it **always wrote to the `default` organization.**
    With `PQCOTA_REQUIRE_ORG=1`, neither command could open the store at all, whatever `PQCOTA_ORG` said.
  - *Which version it entered in.* v0.2.0, the release that added the organization axis. Both commands
    already opened the store without an organization there, and stayed that way.
  - *What came out wrong.* In a deployment that sets an organization, the endpoints and profiles these two
    commands wrote landed in `default`, while `pqcota-inventory` reads under the configured organization,
    so that metadata did not show up in its results. A deployment that never sets `PQCOTA_ORG` saw no
    difference.
  - **Rows already written to `default` are not moved automatically.** The tool cannot know which
    organization they belonged to, and moving them would risk filing them under the wrong one. To move
    them, decide the organization yourself, set `PQCOTA_ORG`, and run the command again: the new rows
    appear under that organization and the old rows stay where they are.
  - *Tests.* Each command has rule tests (no database) and a Postgres test for the default organization,
    an explicit one and the required mode. Without the fix they fail.
- **`pqcota-jvmscan` explained a probe result wrongly.** When it found a JVM but was not given an agent,
  the note said "no JVM was running". The note now says which of three things happened. The result itself
  did not change; only the explanation did.

### Built

- **Documents for readers who do not write code.** A new front page, a build guide, a reporting guide for
  people who receive the report, an FAQ, and a primer on PQC migration. They state what observation can and
  cannot show, and every claim was checked against the code.
- **Documents for contributors.** A developer home page, an environment-variable reference, a guide to
  changing a contract, and a guide to the checks and gates.
- **Paths moved.** The documents now live at `docs/build.md`, `docs/reporting-guide.md`, `docs/faq.md`,
  `docs/pqc-migration-primer.md`, `docs/developers.md`, `docs/environment-variables.md`,
  `docs/change-a-contract.md` and `docs/checks-and-gates.md`. Links to the v0.10.0 layout need updating.
- **A build for an online documentation site** (`tools/site`): it gathers the Markdown from the five
  repositories at one tag, with a pinned tool version, a pinned page list and a link check. **Nothing is
  published from it yet. GitHub Pages has not been switched over;** the site you can reach at
  `https://randyinthedev-hash.github.io/pqcota/` is still the hand-written landing page, whose display
  now defaults to English (the Korean toggle stays).

### Known limitations (unchanged)

These are not new and are not fixed here. They are written down so nobody is surprised.

- **`PQCOTA_AUTO_DDL=0` covers only the inventory history schema.** The metadata store and the provisioning
  record store create their tables regardless.
- **A zero or negative observation window is not an error.** `pqcota-netcap` accepts it, and the window
  actually used is 3 seconds; the start-up message still prints the value you gave.

### Not done

- **Pages is not switched to the new site**, and no policy has been decided for the two limitations above.

## v0.10.0 — The repository is split into five (2026-09-30)

**Goal** — give the contracts, the shared code and each stage a module of their own, so each has one
owner and a dependency direction that a machine checks. Nothing observable changes: the wire format, the
signatures and the behavior of every command are the same as v0.9.1. **Every consumer changes its imports.**

**One release is five tags.** `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`,
`pqcota-provisioning` and `pqcota` all carry `v0.10.0`, and the release workflow builds from the five
together. The stage modules do not release on their own schedule.

### Built

- **Five repositories.** `pqcota-common` holds the contracts (`proto`, `gen`), `pkg/kernel`, `pkg/org`
  and `pqcota-keygen`. `pqcota-inventory` holds the inventory stage. `pqcota-discovery` holds the
  collectors, their commands and the reference playbook. `pqcota-provisioning` holds the provisioning
  stage. `pqcota` keeps the demo, the checks across stages, the gate tools and the release bundle.
- **The dependency direction is a rule, and `make check-deps` enforces it across the five.**
  `common ← inventory ← {discovery, provisioning}`. Discovery and provisioning never reference each other,
  and inventory references neither. Before the split, inventory used code that lived under discovery
  (`history`, `normalize`, `resultio`). That code now lives in inventory, which is why the split needed
  no cycle.
- **Sibling modules resolve through `replace => ../pqcota-*` for local work, and `require` names the
  real tag.** A consumer outside this workspace ignores the `replace` and fetches `v0.10.0`.

### Module path mapping

Every path below is under `github.com/randyinthedev-hash/`. Left is the v0.9.1 path (inside `pqcota`),
right is the v0.10.0 path.

| v0.9.1 | v0.10.0 |
|---|---|
| `pqcota/gen/pqcota/{common,discovery,inventory,provisioning}/v1` | `pqcota-common/gen/pqcota/{common,discovery,inventory,provisioning}/v1` |
| `pqcota/pkg/kernel/...` (`machineid`, `posture`, `registry`, `scope`, `sign`) | `pqcota-common/pkg/kernel/...` |
| `pqcota/pkg/org` | `pqcota-common/pkg/org` |
| `pqcota/pkg/inventory/...` (`declaration`, `ingest`) | `pqcota-inventory/pkg/inventory/...` |
| `pqcota/pkg/discovery/{history,normalize,resultio}` | `pqcota-inventory/pkg/inventory/{history,normalize,resultio}` |
| `pqcota/pkg/discovery/procs` | `pqcota-discovery/pkg/discovery/procs` |
| `pqcota/pkg/provisioning` | `pqcota-provisioning/pkg/provisioning` |

Commands move with their stage: `discovery/cmd/*` and `discovery/collectors/*` to `pqcota-discovery`,
`inventory/cmd/*` to `pqcota-inventory`, `provisioning/cmd/*` to `pqcota-provisioning`, and
`pqcota-keygen` to `pqcota-common/cmd`. The `pqcota` module keeps `tools/checkprose`, so
`go run github.com/randyinthedev-hash/pqcota/tools/checkprose@v0.9.1` and later tags keep working.

### Learned

- **The wire format did not change.** Only the `go_package` option of each proto file did. `buf breaking`
  (FILE) of the new contracts against v0.9.1 reports nine `go_package` differences and nothing else.
  From this release on, `pqcota-common` compares against its own previous tag, and v0.10.0 is the first
  baseline.
- **A path change is a decision about the people who import it.** See
  [compatibility policy](docs/compatibility.md), on module paths.

### Not done

- **A change in a stage repository does not yet start the checks that span stages.** The integration
  checks run on this repository's own pushes and pull requests. Until a stage repository triggers them,
  run `make all` here after changing a stage.

## v0.9.1 — The prose gate lives in this repo (2026-09-17)

**Goal** — keep `tools/checkprose`, the checker that stops once-removed Korean expressions from
creeping back into documents, HTML and tool output, in this repo, and let other repos run it with
`go run github.com/randyinthedev-hash/pqcota/tools/checkprose@v0.9.1` instead of copying the code.
Contracts and binaries are the same as v0.9.0.

### Built

- **`make check-prose`**: fails when a word listed in `rules.tsv` grows past the baseline, and also
  when it shrinks, so the baseline gets lowered. Markdown is read with code blocks and inline code
  masked, HTML with code·pre·script·style and comments masked, Go with string literals only. English
  twins (`*.en.md`) are not read. The 128 hits that remain today are carried by `baseline.tsv`.
- **Notices (`notices.tsv`) are not a gate.** Things a machine cannot separate from legitimate use
  (a spaced hyphen that is a proper separator in headings and tables) are only reported as candidates.
  They never fail the run and never enter the baseline.
- **All configuration lives in one directory and the code knows nothing about the repo.** Rules,
  notices, overlap words, the baseline, and the list of non-Markdown files to read (`files.txt`) come
  from `-dir`. Another repo only needs its own configuration directory.

### Learned

- **Rules kept by eye do not survive.** Em dashes were removed from `.md` on 2026-08-25 but stayed in
  `.html`, and "조용히" (silently) piled up past fifty lines afterwards. Only rules a machine enforces
  remained.
- **A checker belongs to the repo that originates it, not to the repos that use it.** Copying the same
  code into every repo means every rule change has to land in three places in lockstep, and which copy
  is the original becomes a licensing question. With this repo owning it and others pinning a version,
  both problems disappear.

## v0.9.0 — Tracing an action back to the snapshot state it came from (2026-09-11)

**Goal** — make `derived_from_snapshot_id` fillable. The side producing the plan normalizes the same
results under the same rules and gets the same fingerprint; the history finds the snapshot by it and
records it. **Every existing approval signature becomes invalid** (see "What changes for consumers").

### Built

- **The reference fingerprint v1** (`history.ContentHashV1`, format `pqcota-snapshot-content/v1`). It
  answers a **different question** than the dedup fingerprint — not "was the same state observed
  again?" but "which snapshot state did this action come from?". It includes the ruleset version, the
  excluded count, the layers covered and the edge's app, and sorts edges by every stable field. **v1 is
  frozen.** A change means v2. A test pins the value of a fixed input.

- **Per-node merging is deterministic.** Results are sorted by time, collector and content; a finding
  that arrives differently keeps the most recent and says so in the note; edges merge only on an exact
  match of every stable field; no completeness note is dropped. The order of result files no longer
  changes the snapshot.

- **The history looks up by fingerprint.** A `content_hash_v1` column and an `(org, node_id,
  ruleset_ver, content_hash_v1)` index on `pqcota_snapshots`. `SnapshotLookup{ByID, ByContentHashV1}`
  is implemented by Postgres and the in-memory store. `Store` was not widened — it is public, and
  widening it breaks whoever implements it.

- **Actions carry their evidence.** `RemediationAction.evidence_sources[] = {finding_id, snapshot}` —
  which finding, in which snapshot state. The reference (`SnapshotReference`) is either the history's
  real id or a content fingerprint (`{format_version, digest, ruleset_version}`), and names the
  **source node** (the name the history stored it under). The plural unit is "evidence", not
  "snapshot": listing snapshots alone leaves a single `finding_id` that cannot say which finding in the
  second snapshot is the basis. The plan-level `derived_from_snapshot_id` stays as a compatibility path.

- **Evidence must say what it is evidence of.** `evidence_sources[].finding_id` cannot be empty, and the
  compatibility `finding_id` must equal `evidence_sources[0].finding_id` — **empty is not an exception.**
  Allowing empty would let old consumers read no basis while new ones read the primary. A mismatch leaves
  that evidence unresolved and counts as incomplete.

- **The generator actually resolves references.** Shape is checked without `--dsn` (malformed → exit
  3); with `--dsn` it looks the reference up, checks that **the found snapshot contains the finding**,
  and records the submitted reference paired with the real id in
  `ProvisioningRecord.snapshot_resolutions`. Not found → exit 3, and the warning names the possible
  causes as values (another history, the source node's name, the ruleset, the scope policy, the result
  set). `pqcota-records` shows it as a `snapshot:` line. This closes the record → plan → snapshot chain.

### Fixed

- **Per-node merging depended on input order** (v0.1.0–v0.8.0). Duplicate findings, duplicate edges and
  the completeness note were all "first one wins". **What came out wrong**: reordering result files
  changed the snapshot. Which of two collectors' views of one asset survived depended on directory
  order, and the later collector's gap note vanished. The ruleset version moved to `pqcota-enrich/v2`
  because the same input now yields a different snapshot.

- **Edge identity was too narrow** (v0.1.0–v0.8.0). Only direction, protocol and negotiated group.
  **What came out wrong**: two observations differing only in cipher or app folded into one, keeping the
  first.

- **Deduplication folded on the old fingerprint.** Keeping the v1 column while still folding on the old
  fingerprint would have reused old rows with an empty v1 forever, leaving the column empty and the
  references unresolvable. Folding now keys on v1. **The first ingest after upgrading creates a new row
  even for an unchanged state** — a mark of the migration, not a change. Old rows keep an empty v1 and
  are not back-filled.

### What changes for consumers

- **Every existing approval signature is invalid.** The action's evidence is part of `CanonicalPlan`.
  An empty list still occupies a slot in the canonical form, so plans that had no evidence must also be
  re-approved. Left uncovered, a reference could be swapped after signing and "traceable" would not be
  a guarantee.
- **Old consumers cannot read new plans.** Generators and approvers up to `v0.8.0` do not know
  `evidence_sources`; `protojson` rejects unknown fields, so they refuse the plan. Roll back with the
  same release of the generator.
- **The ruleset version is `pqcota-enrich/v2`.** History comparison shows "the rules changed, so a
  derived value moved". Downstream rebuilds its combined identifier.
- **The first ingest after upgrading creates a new row even for an unchanged state.** A mark of the
  migration, not a change.

## v0.8.0 — Approval finalizes the plan (2026-09-11)

**Goal** — separate judging from approving execution by contract status. The judging side hands over
`IN_REVIEW`; approval raises it to `FINALIZED`.

### Built

- **`pqcota-approve` raises the status.** A judged plan arrives as `status=IN_REVIEW` with the approval
  slot and `finalized_at` empty; the first approval raises it to `FINALIZED`, stamps `finalized_at`,
  and **then** signs. `CanonicalPlan` covers both fields, so the other order would break the signature
  just made. Further approvals change nothing and only add a signature — two approvers must sign the
  same canonical bytes for both to verify. The per-status behaviour lives in one place,
  `provisioning.PrepareApproval`, and `check-gates` checks that it is wired.

- **Structure is checked before approval.** There must be actions, each with a target node and a kind
  (the same content layer as `Executable`). It runs for the first approval **and** for further ones — a
  plan whose actions were removed after the first approval must not collect another. A plan that fails this and then gets signed becomes
  "approved, yet not executable" — a state in which nobody can say what the approver took
  responsibility for.

- **Corrupt plans are refused.** `IN_REVIEW` with an approval or a `finalized_at`, or `FINALIZED`
  missing either, is not approved. `FINALIZED` only ever comes from an approval, so an empty field means
  the status was edited in.

- **The contract comment is corrected.** Despite its name, `FinalizedPlan` is an envelope that carries
  `IN_REVIEW` too. Renaming would break consumers, so the name stays and the comment says so.
  `buf breaking` does not object to comments.

### Fixed

- **`pqcota-approve` never looked at status** (v0.7.0–v0.7.6). It signed whatever came in, including
  `DRAFT`. **What came out wrong**: nothing executed, because `Executable` blocked it downstream. But
  approval was an act that never checked what it was about, and when the plan-producing side put an
  unverifiable label into the approval slot, the plan had the shape of `FINALIZED` with an approval
  entry and **looked as though it satisfied the structural gate.** That shape is now refused as corrupt.

- **The samples and the demo carried labels on `FINALIZED`** (v0.1.0–v0.7.6). Fourteen sample plans
  had values like `"reviewer:alice"` in the approval slot, and the three demo plans wrote `FINALIZED`
  with no approval slot at all. **What came out wrong**: the docs said the samples were not approved
  plans, but their shape said otherwise, and every normal run printed "it is a label and proves
  nothing". A warning that always fires is never read. All seventeen now arrive as `IN_REVIEW` with the
  approval slot and `finalized_at` empty.

### What changes for consumers

**Whoever writes a plan by hand does not write `FINALIZED`.** Leave it `IN_REVIEW` and run
`pqcota-approve`. A plan previously written as `FINALIZED` with an empty approval slot is now
**refused as corrupt** — set its status back to `IN_REVIEW`. A plan that skipped `pqcota-approve` used
to be refused with "approvals cannot be checked" and is now refused with "status is not `FINALIZED`".
Same procedure, different message.

## v0.7.6 — The gate matches the whole family (2026-09-11)

**Goal** — catch the old copies that survive a ruleset bump.

### Fixed

- **Only copies of the current version were caught** (v0.7.4–v0.7.5). Once the constant moves to
  `pqcota-enrich/v2`, a leftover `pqcota-enrich/v1` passes as "not the current value, so not a
  placeholder". **What came out wrong**: that leftover is exactly what needs catching. A copied value
  does not follow the constant, so **code stamping the old version quietly survives**, and from then on
  those snapshots look as though the current rules produced them. The family prefix
  (`pqcota-enrich/`) is now derived from the constant, catching past and future versions alike. The
  check does not depend on the constant's current value, so bumping it needs no test edits.

> **This gate took three releases to get right** (`v0.7.4` introduced it, `v0.7.5` made it pass its own
> rule, `v0.7.6` widened it to the family). All three were the same kind of defect: correct today,
> wrong after the next change. When writing a checker, ask what it catches **after the value moves**,
> not what it catches now.

## v0.7.5 — The checker passes its own rule (2026-09-11)

**Goal** — actually establish what `v0.7.4` promised: that a placeholder cannot come back.

### Fixed

- **The gate was doing the very duplication it forbids** (v0.7.4). The authoritative constant's value
  was copied into the checker, and the checker's own file was therefore excluded from the scan.
  **What came out wrong**: a copied value does not follow the constant. Once the ruleset version moves,
  **copies of the new value go uncaught.** It worked today and guaranteed nothing tomorrow. The
  constant is now read from `normalize` and the name is matched by segment, so the checker passes its
  own rule with no exemption. A test holds that in place — copy the value again and it breaks.

## v0.7.4 — The delegated intake endpoint stamps the ruleset too (2026-09-11)

**Goal** — make it impossible for a placeholder to come back.

### Fixed

- **`pqcota-cbom-ingest` was still stamping `ruleset-demo`** (v0.1.0–v0.7.3). v0.7.3 said the ingest
  placeholder was fixed, but **only one of the two ingest endpoints actually moved.** This command is
  the documented endpoint for receiving external CBOMs (delegated intake), not a demo path, so history
  built from CI-produced CycloneDX still could not answer "did the rules change, or did something?"
  after v0.7.3.

### Built

- **`check-gates` blocks ruleset placeholders.** It finds string literals that look like a ruleset
  identifier in tracked Go files; the file declaring the constant and the tests are exempt. It lives in
  the same gate as the wiring check because it is the same failure: the rule is written down, the
  product uses its own value instead, and **nothing fails while the guarantee quietly disappears.**
  That is how two endpoints carried the same defect across two releases. The word `ruleset` inside a
  format string is a label, not a value, so it is not flagged.

## v0.7.3 — This repo owns the enrichment ruleset version (2026-09-10)

**Goal** — give the rules that produce derived values a version of their own.

### Built

- **`normalize.RulesetVersion`** (`pqcota-enrich/v1`). It moves only when the rules that produce derived
  values (`evidence_strength`, `pqc_readiness`, posture, maturity) change. **It is not the release
  version** — editing prose or adding a CLI does not change what those rules produce. Consumers append
  their own ruleset version to it, because reconciliation and plan conversion are not this repo's rules.

### Fixed

- **Ingest stamped `ruleset-demo` onto snapshots** (v0.1.0–v0.7.2). A placeholder went into the real
  history. **What came out wrong**: history comparison uses this value to tell "the rules changed, so a
  derived value moved" apart from "something actually changed", and every snapshot carrying the same
  placeholder could not answer that. The two view-only paths (`pqcota-discover-view` and the local view)
  carried `ruleset-1` and `ruleset-demo` too.

## v0.7.2 — The controller CLIs ship too (2026-09-10)

**Goal** — let someone who took only the release run all three stages.

### Built

- **A controller bundle is attached to the release.** `pqcota-ctl-linux-amd64.tar.gz` and `-arm64.tar.gz`
  carry every CLI, including ingest (`pqcota-ingest`), query (`pqcota-inventory`), approval
  (`pqcota-approve`) and generation (`pqcota-provision`). It is kept apart from the node bundles **by where
  they go**: those are carried onto the hosts you observe, this one sits on the single central machine.
  Building the operator CLIs from source used to be the only way, so anyone who took only the release could
  observe and go no further.

## v0.7.1 — Two places where the document ran ahead of the code (2026-09-10)

**Goal** — fix the wording v0.7.0 left out of step. No code changes.

### Fixed

- **A comment on the approval check contradicted its own next sentence** (v0.7.0). Replacing the paragraph
  left the old opening line, "with no keys it does not block", sitting right above "with no key to check
  with, it refuses by default". **What came out wrong**: it told anyone reading the code the opposite of the
  default. The behaviour was refusal from the start.
- **The README described the release contents too broadly** (v0.1.0–v0.7.0). Saying only "per-architecture
  static binaries" read as if everything shipped. What actually ships is **the collectors that go onto
  target nodes, plus the JVM sidecar**; the controller CLIs are built from source. The release body said so;
  the README did not. **What came out wrong**: someone who took only the release would go looking for
  commands that were never there. Fixed in both languages.

## v0.7.0 — Tightening plan intake and execution safety (2026-09-10)

**Goal** — stop execution from discarding judgements that arrived inside the contract, and **stop
counting what could not be checked as a pass.** The deployment-readiness review pointed at these.
**This is not the release that completes the end-to-end handoff** — filling the plan in on the way out,
and the approval handoff itself, are still open.

> **Read before upgrading.** Two defaults changed, so **existing automation will not pass as-is.**
> ① With no key to check approvals with (`PQCOTA_APPROVAL_KEYS`), it **refuses with exit 1**; pass
> `--allow-unverified-approvals` to go on knowingly. ② A plan with blanks still produces the artifact but
> **exits 3**; pass `--allow-incomplete`. ③ An unknown `--level` is now **exit 2** (it used to fall back
> to L2 silently).

### Built

- **Approvals are checked with the approver's key.** The shape is `<approver>:ed25519:<base64>`, and the
  approver id lives inside the signature, so verification uses **only that person's registered key**. The
  signature covers the whole plan except the approvals themselves, so **it covers the order of the actions
  and `status` too** — an approval given on a draft cannot be moved onto a finalized plan.
  `PQCOTA_APPROVAL_KEYS` is a `<approver>=<public key>` map for this reason: a bare list of keys lets a
  signature that passes under any key arrive wearing any name.
- **With no key to check with, it refuses.** It used to warn and go on, and closing that path happened only
  where a deployment set `PQCOTA_REQUIRE_APPROVAL=1`, which leaves approval integrity as something you
  *can* close. The one way to open it knowingly is `--allow-unverified-approvals`, and **it has to be
  written on the command line** — an environment variable would hide what was verified in shell config.
- **The generator follows the per-asset delegation level.** `automation_level` is a first-class property of
  the action in the contract. `--level` is now **the default for actions the plan leaves unset**. Rollback
  splits by the same rule, and the activation warnings are per action.
- **A plan that does not state the level counts as having a blank.** When it is unset the level comes from
  `--level`, and that flag is **outside the approval signature** — the signature covers the plan, not the
  command line that invokes it. So the effective level of an unset action is not covered by the approval,
  and that is now reported by name. It does not block: deciding the level on the command line is a
  legitimate path, for instance when trying one node in place.
- **A plan with blanks exits 3.** The artifact is still produced — filling blanks in by hand is a legitimate
  path, so it does not block — but it does not finish as a success. To accept it knowingly,
  `--allow-incomplete`. It is kept apart from a refusal (1) because what you have to fix differs.
- **The execution gate looks at minimum executable content, not only procedure.** If an action has no target
  node, or its kind is `UNSPECIFIED`, it **refuses before producing anything**. The tool does not guess the
  plan, so rather than ship a fragment stating something untrue it blocks.
- **Blanks a person can fill are reported by name alongside the artifact.** Actions whose target does not
  resolve to a hybrid group and therefore **turn nothing on when deployed**, actions whose provider class is
  empty so a placeholder ships, and plans with no basis to trace back to. The artifact is produced and the
  run **exits 3**. On rollback, missing activation hooks and traceability gaps are reported the same way.
- **Five more gates.** A rule written down but never called by the product; the Go version in the docs
  against `go.mod`; the test level counts; the governance numbers; and the section structure of a Korean document
  against its English counterpart.
- **CI brings up Postgres.** The five org-isolation cases ran there for the first time. The case that needs
  `CAP_NET_RAW` is run once more on its own.
- **Result files are told apart by content, not by extension.** The place where a name could lie is gone.
- **The rollback playbook states what it undoes.** It **removes the artifacts this plan manages at their
  fixed paths.** Originals were never overwritten and survive, but if a second run deployed under the same
  name and path, the first run's pqcota artifact is already overwritten — undoing it then deletes the file
  instead of restoring the earlier one. **It is not a version rollback.**

### Learned

- **What a signature protects and what execution follows can diverge.** The approval signature covers
  `automation_level`, but the generator did not read it. An approver signed "the payment server goes no further
  than L1" while execution flattened everything with `--level l3`. When the judgement that split delegation
  by risk disappears at execution time, the stage boundary stops working as a gate.
- **Closing a default moves every place that command is written down.** Making approval checks mandatory
  left five documented paths that no longer produce a playbook. The lesson that changing one word moves four
  places turned out to hold for exit statuses too.
- **A skip is only counted if it reaches the log.** The workflow comment promised to record what was
  skipped, but without `-v` no skip was ever printed — which is why nobody noticed that five Postgres cases
  had never run in CI. The document was ahead of the code.
- **Cases with opposite environments cannot be measured in one run.** TD-NETWORK-15 needs `CAP_NET_RAW`;
  TD-NETWORK-16 needs its absence. No single runner satisfies both, so it runs twice.
- **The commit-count gate has to be re-checked after committing**, because committing raises the number the
  gate compares against.
- **Splitting traffic observation from asset scanning is a design question first.** The implementation went
  in and came back out. The merge rule for repeated observations and the merge key disagreed in three
  places; dividing the cadence without settling that leaves edges that never accumulate.

### Fixed

- **The generator ignored the per-asset delegation level** (v0.1.0–v0.6.7). It never read the per-action
  value and emitted every action at the global `--level`. **What came out wrong**: in a plan mixing levels,
  assets finalized at a lower level still received activation and restart tasks.
- **Approvals were only counted** (v0.1.0–v0.6.7). A single string like `reviewer:demo` passed.
  **What came out wrong**: a plan carrying an approval nobody could verify passed as grounds to execute.
- **The CLI never called the execution gate** (through v0.6.7). The rule had tests and the documents
  promised it, but the product path only compared status. **What came out wrong**: a finalized plan with no
  approvals and no action content was handed a playbook. `check-gates` now catches that class.
- **An unknown `--level` fell back to L2 silently** (v0.1.0–v0.6.7). `--level L3` differed only in case and
  landed on the default. **What came out wrong**: it handed over an artifact with no activation or restart
  while reading as if the request had been honoured. A level is delegation by risk, so running lower than
  asked is as wrong as running higher. It is exit 2 now.
- **CNG still said "not yet"** (v0.6.0–v0.6.7). Documents and comments called it unimplemented after real
  hardware had confirmed it. **What came out wrong**: it told readers something worked when it did not.
- **Four words that had been reverted survived in other file types.** `실기` in 12 places,
  `저장된 값과 갈린다` in 4, `해소` in 22, `조용히 틀린다` in 2. Sweeping the documents had not covered
  `.proto`, `.sh` and comments. **What came out wrong**: the documents and the code called the same thing by
  different names.

## v0.6.7 — One name per thing (2026-08-21)
**Goal** — make the same thing carry the same name everywhere, and turn what v0.6.6 exposed — "the
same list in two places drifts apart" — into **a gate**.

> **One value on screen changes** — the top PQC maturity goes from `standard` to **`fips-standard`**
> (① below). Anything scraping the output has to follow that string.

### Built

- **`make check-collectors`** — checks that the collectors the release workflow **builds** and the
  ones the reference playbook **deploys** are the same set. They really did drift in v0.6.6. On a
  mismatch it prints what differs along with both lists. If either file changes shape so that nothing
  can be read, **that fails too** — passing quietly would leave the gate blind. CI runs it.

### Fixed

- **① The PQC maturity was named one way in the data and another on screen** (v0.6.2–v0.6.6).

  **What was wrong** — the stored value is `fips-standard`, while the display label was just
  `standard`. The split appeared when the Korean `표준` was translated in v0.6.2; in Korean there was
  nothing to collide with.

  **What came out wrong** — seeing `X25519MLKEM768 [standard]`, a reader could not tell whether it was
  the `fips-standard` from the maturity table in the architecture.

  **What changes** — the screen now prints **the value itself** (`[fips-standard]`), returning the
  constant rather than re-typing the string, so the display follows if the value ever changes. The
  samples (`discover-view.txt`, `topology.svg`, the README console blocks) were regenerated **by
  running the demo again**.

- **② `CONFIRMED` lives in two different enums, and one place abbreviated it** (v0.1.0–v0.6.6). The
  root README's console block said `[CONFIRMED]` where the screen prints
  `[EVIDENCE_STRENGTH_CONFIRMED]`. Abbreviated, it is indistinguishable from `ReconState.CONFIRMED`.
  The block now quotes the screen — so a bare `CONFIRMED` in the documents always means `ReconState`.

- **③ Three states were called by three different kinds of name** (v0.1.0–v0.6.6). The demo README
  wrote `CONFIRMED/shadow/unobserved` — a contract name, a gloss, and a plain word. Nothing told the
  reader whether `shadow` was `UNDECLARED` or a fourth thing. The names are now
  `CONFIRMED`/`UNDECLARED`/`UNOBSERVED`, and `shadow` stays only where it **explains** what
  `UNDECLARED` is.

- **④ The English documents used a different name from the screen** (v0.6.2–v0.6.6). The root
  `README.en` console block said `quantum posture` and `posture totals`, while the screen prints
  `quantum-resistance grade` and `grade totals`. The Korean side moved to 등급 in v0.6.2 and the
  English side did not follow. Four prose sites were aligned as well. Released identifiers such as
  `pkg/kernel/posture` and `QuantumPosture` are untouched.

## v0.6.6 — The release bundle had drifted from the playbook (2026-08-21)
**Goal** — fix what fails when you download it and run it. And make the sentences that could only be
read with another document open say what they mean where they stand.

### Learned

- **The same list kept in two places drifts apart in silence.** The set of collectors to put on a
  Windows node lived in both the release workflow and the playbook, and nothing caught it when only
  one changed. A comment in the workflow now says it must match the playbook — but **that is not a
  gate.**

### Fixed

- **① `pqcota-jvmscan` was missing from the Windows bundle** (v0.6.3–v0.6.5).

  **What was wrong** — v0.6.3 made the Windows block of `discover.yml` deploy **both**
  `pqcota-cngscan` and `pqcota-jvmscan`, while the release workflow still built only `cngscan`.

  **What came out wrong** — **download the release, run the playbook, and it fails at deployment.**
  During the real-hardware run `jvmscan.exe` was built by hand, so this path was never exercised.

  **What changes** — the bundle carries both (`pqcota-windows-amd64.zip`).

- **② Sentences that could only be read with another document open.** The symbols pointed somewhere
  the document itself never defined.

  | Where | What | Now |
  |---|---|---|
  | root README, support table | `layer ① is Linux-only; ② and ③ are OS-independent` | **the path that attaches without a JDK is Linux-only**, spelled out |
  | 〃 | "the observed lane and the declared lane" | **what a machine saw and what a person wrote down** |
  | 〃 | `(measured up to 25)` | `measured up to JDK 25` |
  | collector deployment | `①` meant both a **deployment step** and an **attach layer** | the layer is spelled out |
  | [inventory/cmd](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.md) | "② signs it" — another document's section number, **and the wrong actor** | the **collector on the node** signs |
  | [provisioning](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/README.md) | `L1/L2/L3` in the first sentence, explained only inside a **collapsed** diagram | the first sentence now says what they are |

- **③ `pqcota-keygen` was documented in two places** (v0.6.3). It now lives with the command, in
  [discovery/cmd](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md), and the `PQCOTA_VERIFY_KEY` row points there.

- **④ The support table wrote down where we measured as if it were the supported range** (v0.6.3).
  "measured on Windows 11 build 26200" read as **only there**, which is not true — it runs on any
  Windows with CNG. What the build decides is not *whether* it works but **what you will find**
  (26200 had ML-DSA and no ML-KEM). The measurement belongs to the test map.

## v0.6.5 — Unpacking compound loanwords and an English idiom (2026-08-21)
**Goal** — untangle three words in the Korean documents. **No code changed**, and the English
documents are untouched; what changes is the Korean vocabulary for things that already worked this way.

### Fixed

- **`해소` ("resolve")** — one word had been doing five jobs, so it is unpacked per site: an IP mapped
  to a node is now "linked", a dependency `go get` could not fetch is "not fetched", an app key found
  from a process is "found", a volatile process is "linked at the time", and a FIPS gap is "filled".

- **"조용히 틀린다"** (a literal rendering of *silently wrong*) → **"오류 없이 틀린다"** — *wrong
  without raising an error*. The point is that nothing fails, not that the code is quiet.

- **`갈리다`** now only means branching. Where it meant values diverging, it is "달라진다" — one word
  covering both "splits" and "does not match" left the reader to work out which.
- Identifiers are untouched — `ProcessMatch`, `TestResolve`, `procs.Attribution`.

## v0.6.4 — The documents catch up with the code (2026-08-21)
**Goal** — close the stale spots the documents were left with after the screen and the code moved on.
**Not a line of code changed** — there is nothing to upgrade for.

### Learned

- **When the screen changes, quotations in the documents go stale quietly.** `checkdocs` looks at
  links, anchors and wording, but **not at whether the output a document quotes still matches
  reality**. Re-running the demo and diffing the quotes against the run surfaced one. That gate does
  not exist.

### Fixed

- **Four documents were brought in line with the screen and the code.** The openssl collector's
  "Assumptions" named the wrong build tag (v0.1.0–v0.6.3); the example's `os` column description
  (v0.6.3); `pqcota-cngscan` missing from the architecture document's list of collectors (v0.6.0); and
  the rollback-record quotation in `expected-output` still being Korean (v0.6.2).
- **[Observing OpenSSL on Windows] was added to the roadmap**, and the fact that `pqcota-nodescan` is
  Linux-only was written wherever that command is shown. The per-collector table stays in one place —
  the command reference.

## v0.6.3 — Windows nodes join the discovery path (2026-08-21)
**Goal** — v0.6.0 made CNG observable, but **there was no way to get that collector onto a node and the
result back.** Reach a real Windows machine with Ansible and run the whole loop.

### Built

- **`os` and `connection` columns in `hosts.csv`** — `pqcota-hosts` emits `[targets_linux]` and
  `[targets_windows]` groups plus the connection settings (`ansible_connection=winrm`, or
  `ansible_shell_type=powershell`). `targets` is their **parent**, so playbooks written against
  `hosts: targets` keep working. **The connection belongs in the CSV** because `targets.ini` is
  overwritten on every run — anything added there by hand is gone next time.
- **`discover.yml` branches on the node's OS** — `os_family` from `gather_facts` selects the three
  Linux collectors or `pqcota-cngscan` and `pqcota-jvmscan`. **Nothing is shipped to a node just to
  find out what it is.** Deploy → recon → run → fetch → clean is the same shape; only the modules
  differ (`ansible.windows` required).
- **The jvm reconnaissance runs on Windows** — Toolhelp32 for processes, and the module list for
  `jvm.dll` when the launcher is not `java.exe` (the same place as `/proc` and `maps` on Linux). It
  calls no `certutil`, PowerShell or WMI, for the same reason the cng-collector does not. **It
  also never reads another process's memory (the PEB)** — so the command line, and with it the app
  name, stays empty. That fact is carried as a value and reported on screen.
- **`pqcota-windows-amd64.zip` is attached to the release** — until now you had to build it yourself.
- **The command reference is complete again** — `pqcota-cngscan` (added in v0.6.0 but never listed) and
  `pqcota-keygen` are in, and the per-collector OS table now lives in **one** place. It also points to
  where `hosts.csv` is explained.

> **The contract is unchanged; one more kind of result appears.** Not a line of `.proto` was touched.
> But the jvm-collector can now emit a `CollectionResult` **carrying only a completeness gap, with no
> CBOM body** — a JVM that was found but could not be observed (Fixed ③ below). Anything counting
> components may receive an empty body.

### Learned

- **On Windows, privilege decides more than half of what is visible.** A normal user could not open
  **163** of 265 processes; an Administrator could not open **3** of 264. Those 3 are the floor. Java
  servers on Windows commonly run as services under SYSTEM, so running without privilege **hides
  exactly the JVMs worth looking at**.
- **The three-layer attach paid off here.** Layer ① (Go native) is Linux-only, but ② and ③ were
  already OS-independent — **there was nothing to port**. ② attached for real, and the target JVM
  printed `A Java agent has been loaded dynamically`. Attach on Windows injects a thread; this machine
  did not block it.
- **Something can be named `java.exe` and not be a JVM.** Oracle's javapath launcher shim is one. The
  JDK says so itself: `jvm.dll not loaded by target process`. There is no such shape on Linux, so
  **only real hardware could surface it**.
- **Three defects that Linux had been hiding came out on Windows** — because ① usually succeeds first,
  the client happens to be the target's own JDK, and there are no launcher shims. All three are the
  same kind: **the tool writing down something it made or borrowed as an observation.**
- **Even with every code path correct, Ansible reaching the node is its own thing.** That was
  confirmed last, against a real Windows machine over Win32-OpenSSH with a key (TD-WIN-1, TD-WIN-2).
  One thing broke, and it was a missing `ssh` client in my container — nothing in the repo.

### Fixed

- **① When the ② JDK client could not attach, it reported its own configuration as the target's**
  (v0.1.0–v0.6.2).

  **What was wrong** — `Attacher` caught the failure and read `java.security` from
  `System.getProperty("java.home")`. That `java.home` is **the client's, not the target's**.

  **What came out wrong** — the javapath shim was given **the client JDK's 13 providers**, character
  for character identical to the JVM that had genuinely been attached. The degraded marks
  (`inferred_high`, `artifact`) do not make the values right — this is **worse than an empty result**:
  a plausible answer attached to the wrong asset.

  **What changes** — ② no longer falls back; it ends with the reason. The static fallback belongs to
  the Go side, which uses **the target's** JAVA_HOME and reports a gap when it does not know it. Some
  places that used to produce values now produce gaps; what disappears was someone else's data all
  along. `StaticFallback.java` had no caller left and was removed.

- **② With no JVM running, the tool started one and reported it as an observation** (v0.1.0–v0.6.2).

  **What was wrong** — the probe path ran `java`, read that JVM's provider chain, and emitted it as
  `confirmed` / `runtime-introspection`. That contradicts `nodescan`, which only sees libssl actually
  **loaded** in `/proc`.

  **What came out wrong** — the screen contradicted itself: `1 JVMs observed · confirmed` directly
  below `recon: JVMs 0`.

  **What changes** — the value is kept but named for what it is: degraded, with the reason ("no JVM
  was running — the machine's java launcher was started for this probe"), and the headline reads
  `0 JVMs observed`.

- **③ A JVM that was found but not observed never reached the centre** (v0.1.0–v0.6.2).

  **What was wrong** — when every attach path failed, no result was emitted for that JVM at all.

  **What came out wrong** — the failure lived only on stderr, so **the centre did not even know such a
  JVM existed**. "Not observed" wore the face of "not there". netcap sends its capture failure
  onward as a gap; only this place did not.

  **What changes** — the gap and its reason travel through the contract. **No component is built** —
  an empty provider chain would read as "this JVM has no providers".

- **④ The security policy said the project was "before the first release"** (v0.1.0–v0.6.2), and the
  supported-versions table said "none yet". It now states the actual policy (no backports). For the
  same reason, sentences pinning already-working features to "as of vN" were removed — that is not an
  answer to someone asking what works now, and version history belongs to these notes.

## v0.6.2 — What the program says, in English (2026-08-19)
**Goal** — set the rule that **code and its output default to English** while the documents stay
Korean-first, and bring the whole repo in line. A console line ends up in a log and gets pasted into
an issue; there, Korean narrows who can read it.

### Built

- **The «Language» rule — [CONTRIBUTING](CONTRIBUTING.md)** — which language each kind of text
  uses, and **why**. Comments are Korean (they never leave the program). Console output, flag help,
  error values, test failure messages, and **strings that travel out through the contract** are
  English (where an error flows is the caller's decision). The three exceptions the rule sets for
  itself are written down too: comments in `.proto` and the CI workflows, comments inside SQL DDL
  strings, and the **patterns** in `tools/checkdocs` (it is a tool for catching Korean prose — what
  it **says** is English).
- **Scope** — console output and flag help, the narration in the demo and example scripts, 53 error
  values, 396 test failure messages across 65 files, and the strings carried by the contract:
  `Completeness.Note`, `Attribution.Reason`, the `Remediation` rationale, and the maturity labels
  (`standard`/`draft`/`experimental`/`broken`).
- **Regenerated samples** — the demo was actually run again and
  [expected-output](demo/expected-output/README.md) was captured fresh. The console blocks in the
  READMEs and examples now match, as do output strings the documents had only named — `added`,
  `removed` and `changed` for `-diff`. Display names in the demo and examples are English as well,
  because they appear on screen verbatim.

> **The contract's shape is unchanged; some values are not.** Not one line of `.proto` was touched.
> But the **string values** in the four places above differ, so anything branching on those sentences
> must move to the new ones. Matching on prose was never advisable — to tell reasons apart, compare
> against the `Attribution.Reason` constants.

### Learned

- **Korean has no number agreement, so some faults only surface after the move.** The `-history`
  header printed `1 change points`. The Korean original had no place to be wrong. `inventory.Plural`
  now covers the view and `pqcota-prune` alike.
- **Reordering a sentence silently misaligns format arguments.** Korean puts the subject first
  ("web-01 records: 2"), English the count ("2 records for web-01") — translate the sentence and
  `%s` and `%d` swap places while the arguments stay put. It still builds. `go vet` caught it twice;
  the fix is explicit argument indexes (`%[2]d`). **This is where translation becomes a type error.**
- **A test whose input is non-ASCII has nothing left to check once translated.**
  `safeName("노드/1")` checks that non-ASCII collapses to hyphens and leading/trailing hyphens are
  trimmed. Translating the input broke the test, and the right response was to restore the input,
  not to adjust the expectation.

### Fixed

- **Nothing.** Everything touched here is a string this release wrote itself; no published version
  emitted anything wrong. The two mistakes made during the move (format arguments, the non-ASCII
  test) were caught by `go vet` and the tests before release.

## v0.6.1 — The two gaps left in CNG observation (2026-08-19)
**Goal** — fill the two slots left open after v0.6.0. Both were confirmed in **a single run on the
real hardware**.

### Built

- **`CngAlgorithm.providers`** — for each algorithm, `BCryptEnumProviders` answers **who actually
  serves it**. The registration list (`provider_set`) only says what exists on the machine, so
  choosing what to act on needs this. Purely additive. When it could not be asked, the list stays
  **empty** — "not asked" and "asked and found none" are not written the same way.
- **Windows `hardware_uuid`** — SMBIOS Type 1, read via `GetSystemFirmwareTable('RSMB')`; no WMI or
  PowerShell. `MachineGuid` is per **installation** (a reinstall changes it), so a per
  **hardware** anchor is needed as well. The formatting matches Linux's
  `/sys/class/dmi/id/product_uuid` — the first three groups are little-endian, and without reversing
  them the same machine looks like a different UUID when dual-booted. Firmware placeholders (all
  `0x00` or all `0xFF`) are **not used as identifiers** (they would merge distinct machines into one node).

### Learned

- **A premise inherited from JCA does not hold for CNG.** In the measurement, **all 50 algorithms had
  exactly one provider** — no algorithm is served by two, so there is no priority contention at all.
  The order in `provider_set` is preserved for a different reason: **it is what was observed**, and
  sorting it would be editing the observation.
- **Of the nine registered providers, the one that actually serves algorithms is `Microsoft Primitive
  Provider`.** The other eight are key-storage providers and do not appear in an algorithm
  enumeration. This narrows "what must be touched to use ML-DSA" for v0.7.0.
- **Adding a fingerprint did not move `node_id`.** Self-id priority consults `machine-id` first, so
  `derived_from` stayed put even once `hardware_uuid` appeared. The thing to fear when adding
  fingerprints is a split history, and the priority order already covered that.
- **The SMBIOS value matched what Windows itself reports** (`Win32_ComputerSystemProduct.UUID`).

### Fixed

- **A sentence asserting something never verified**(v0.6.0).

  **What was wrong** — the contract comment, `contracts/README`, the collector README, and the test
  case table all called the order of `provider_set` a **"priority"**. That premise was carried over
  from JCA and had never been checked for CNG.

  **What came out wrong** — whoever reads the contract reads it **as a verified fact**. And the
  measurement pointed the other way: with no overlapping providers, order has no place to act as a
  priority at all.

  **What changes** — it now says the order is kept **as observed**, and whether that order means
  priority is stated as **unverified**. It will be measured again on a machine with a third-party
  provider where two overlap.

## v0.6.0 — Observing Windows CNG (2026-08-19)
**Goal** — fill `CngAxes`, which v0.1.0 **reserved as schema only**. Measure it on real Windows and
confirm the observation reaches the inventory screen. This release closes the state where the
contract had a slot but no code to fill it.

### Built

- **`pqcota-cngscan` · cng-collector** — calls
  `bcrypt.dll` directly (`BCryptEnumRegisteredProviders`, `BCryptEnumAlgorithms`). It does not invoke
  `certutil`, PowerShell, or WMI — on servers where script execution is blocked by policy, a failed
  observation must not scatter into "environment problems". **Off Windows it emits a gap, not
  an empty result**, and exits 0.
- **`CngAxes.algorithms` and `CngAlgorithm`** — added after the measurement, with new field numbers
  (purely additive), exactly as the v0.1.0 reservation note prescribed. Provider names alone do not
  answer the question (below).
- **`COLLECTION_LAYER_CNG_INTROSPECTION`** — for the same reason JCA has its own layer. It is neither
  process nor artifact but **a query of the providers registered on the machine**, so what it fails to
  see differs from every other layer.
- **The screen** — the file view and the inventory view both render CNG assets. `readiness` is derived
  through `registry.MatchPQC` and is **a summary of the observation, not a verdict**.
- **A Windows cross-compilation gate** — `make build` and CI now also build windows/amd64. It was put
  in place **before any Windows code was written**: Linux-only code leaking outside its build tag
  breaks only on Windows, and only this gate catches that.
- **A sample** — `examples/data/results/node-d-cng.json`. The demo is six Linux containers and cannot
  host a Windows node, so a real measurement rides in as a sample that runs with the Go toolchain
  alone (the machine fingerprint was removed).

### Learned

- **The CNG on Windows 11 Pro 25H2 (build 26200) has `ML-DSA` and does not have `ML-KEM`.** Nine
  providers and fifty algorithms were observed. Signatures can go post-quantum; **TLS key exchange
  cannot** — that is this node's fact, and it is not generalized to other builds.
- **Provider names tell nodes apart not at all.** All nine observed are `Microsoft …` names, so the
  capability difference between nodes appears only in the algorithm list. That is the evidence behind
  adding the algorithm axis to the contract.
- **`dwClass` is an interface constant, not the operation bitmask used to request enumeration.** The
  values overlap (both have a 4), so 18 of 50 came back with an empty class and the five DH/ECDH
  entries were labelled `asymmetric-encryption` **instead of** `secret-agreement`. A rule that leaves
  the unknown blank does not save you: **overlapping values fail silently.** Classification buried
  inside an OS call cannot be caught without the real hardware — it was pulled out into a pure
  function and pinned to the measurement.
- **Adding one runtime means two places to render.** Had this been closed at the derived view, the
  file view and the inventory view would both have shipped **blank**. An observation that never
  reaches the screen is the same as one never written down.
- **CNG's FIPS mode cannot be known from an algorithm enumeration** — `fips_validation` is `unknown`.

### Fixed

- **Windows nodes hung on the hostname**(v0.1.0–v0.5.0; surfaced by the first Windows observation).

  **What was wrong** — machine fingerprinting only read `/etc/machine-id` and DMI. Those paths do not
  exist on Windows, so every field came back empty and the last-resort `fqdn` became the anchor.

  **What came out wrong** — the first measurement's `derived_from` was `fqdn`. **Rename the host and
  the same machine becomes a different node**, splitting its history. Linux nodes never showed this
  because they were already on `machine-id`.

  **What changes** — only the *source* of the fingerprint is split per OS; the rules stay in one
  place. Windows reads the registry's `MachineGuid` directly. `hardware_uuid` stays **empty** for now
  — it lives in SMBIOS, and what cannot be read is not invented.

## v0.5.0 — Aligning the module path with the repository address (2026-08-18)
**Goal** — let anyone consuming the contract start with a single `go get`, and make the documents and
the screen call the same thing by the same name. **CNG discovery moved back one slot** — unblocking a
contract nobody could fetch came first.

### Fixed

- **`go get` could not resolve the module, because its path did not match the repository**(v0.1.0–v0.4.0).

  **What was wrong** — `go.mod` declared `github.com/pqcota/pqcota`, and no repository lives at that
  address. `gen/` is committed precisely so that *"a consumer can use the contract types with `go get`
  alone"*, and the path was cancelling that.

  **What came out wrong** — consumers had to carry a `replace` line in their own `go.mod`
  **permanently**. The documentation described that workaround, so it was a known flaw — and settling
  for documenting it was the mistake. A newcomer only sees "module declares its path as …" and has no
  way to trace the cause.

  **What changes** — the path becomes `github.com/randyinthedev-hash/pqcota`. In Go a path change makes
  it a **new module**, so every import must move. Anyone on v0.4.0 or below drops the `replace` and
  fetches the new path. No signature or type changed ([compatibility policy, section 3④](docs/compatibility.md)).

- **`.gitignore` swallowed new `*.pb.go` files**(v0.1.0–v0.4.0). `*.pb.go` sat in the very block that
  says `gen/` is committed. The ten already tracked stay, but **adding a new proto would silently drop
  its generated code** — a consumer would meet a type that isn't there. The rule was removed.

- **A build binary was committed to the repository**(v0.4.0). `checkdocs` (3.3 MB, a macOS
  executable) sat tracked at the root. `make check-docs` builds into `build/`; one made by hand at the
  root had slipped in. Removed, and blocked in `.gitignore`.

- **A sample artifact had gone stale against the code**(v0.3.0–v0.4.0). The last line of the discovery
  view had changed, but `demo/expected-output/discover-view.txt` still carried the old sentence. The
  demo was re-run and the artifact recaptured.

### Built

- **The journey** (this document is no longer part of the repository) — one document that walks the whole way: preparation,
  observation, ingestion, querying, generation, application, rollback. The process rules
  and each stage's design covered its own inside, but nothing answered "in what order does what come out,
  from start to finish". It puts the three entrances (one node in place · many nodes over Ansible ·
  delegated CBOM intake from CI) and the single exit (a playbook) in one picture.

### Learned

- **The documents' vocabulary changed; the identifiers did not.** Words carried over from English were
  rewritten in Korean and the on-screen strings moved with them, but `procs.Attribution`,
  `pqcota-declare-attribution`, `pqcota_edge_attribution`, and `pkg/kernel/posture` were left alone.
  **The only thing a consumer has to change is the module path** — CLI names, table names, and package
  names are unchanged.


## v0.4.0 — a person fills what observation could not attribute (2026-08-12)
**Goal** — fill the place v0.3.0's automatic path cannot see by construction. The demo **missed 3 of 4
edges**, all for the same reason: short-lived connections — which is what batch jobs, health checks,
cron, and SSH all are.

### What was built

- **`pqcota-declare-attribution`** — imports a CSV (`node_id,dst,app_key`) as declarations. A row that
  does not identify an edge (any of the three empty) **stops rather than being guessed at** —
  attributing to the wrong app changes what gets acted on.
- **`pqcota_edge_attribution`** — where declarations live, **outside the node's snapshot timeline.**
  Ingest separates declarations out and routes them here without creating a snapshot. Declaring the same
  `(org, node_id, dst)` again overwrites — a declaration is something a person corrects, so it does not
  follow the append-only rule that observation does.
- **`AttributionOverlay`** — reads that store at query time and lays it over. What observation already
  filled stays untouched; only blanks are filled, shown as `@app(declared)`, with a line saying how many
  came from declarations.

### What was learned

- **Ingest must not patch the observed edge.** That looked simpler at first, but two things block it:
  ① `sign.Canonical` covers `ObservedEdge` including `app_key` (v0.3.0), so filling it at ingest makes
  **what is stored differ from what the collector signed** — and a declaration is not the collector's
  claim. ② When rules improve and results are recomputed from `raw_capture`, the recomputed and stored
  values diverge, and it stops being clear which is the original. **So storage stays separate and the
  screen joins them.**
- **The contract did not grow.** `app_key_kind`, added in v0.3.0, already answers "what does this key
  rest on", so `declared` simply joins `systemd-unit` and `exe-path`. Signatures are untouched too — one
  break in v0.3.0 was enough without a second right behind it.
- **`ObservedEdge.detection_method` must not be used for this.** It says *how the edge was observed*, not
  where the key came from. Putting `UNSPECIFIED` there would **blur the fact that the communication
  really was seen.**
- **Declarations had to be separated at the storage layer — filtering per view was not enough.** The first
  attempt put declarations on the same snapshot timeline and filtered them out on screen. Two demo runs
  showed two leaks: ① a declaration became the node's **latest snapshot**, so the default view showed
  *1 observed edge instead of 4* (observation appearing to have vanished), and ② history **lined the
  declaration up as a state change** (a node that had never had 0 assets and 1 edge). `-diff` would have
  been the third. **Filter per view and every new view leaks the same way** — so the storage was split.
- **`dst` already carries the port.** The contract defines `dst_addr` as `"ip:port"`, yet the CSV and the
  store key also held a port — writing the same fact twice means one of them can be wrong and the match
  fails silently.
- **In-memory tests cannot prove isolation** — the store objects differ to begin with. Only Postgres,
  where one table is shared, can measure it (TV-ATTR-7); without that counterpart, a green light reads
  as isolation.


## v0.3.0 — attributing edges to apps (2026-08-12)
**Goal** — carry observed communication past "somewhere on this server" and **all the way to the app**.
What a person acts on is an app, not a server.

### What was built

- **`ObservedEdge.app_key` and `app_key_kind`** — filled at capture time by correlating the socket inode
  from `/proc/net/tcp` against `/proc/*/fd`. The value has the same shape as asset attribution: systemd
  unit first, exe path otherwise.
- **`procs.AttributeRemote` and `Attributor`** — attribution happens **where the edge is seen**. Doing it
  after the capture window loses sockets that closed in between. Only the expensive fd scan is reused,
  for one second inside the window.
- **Attribution results in the completeness note** — the count and reasons for what was missed. The
  reasons are sorted: if their order wobbles, the same observation becomes a different snapshot through
  its content fingerprint, and history grows without anything having changed.
- **The inventory shows it** — `@app_key` on the edge line. What was missed is `@?`, not a blank, and if
  the basis was not a systemd unit, `(exe-path)` is shown alongside. The same value can deserve different
  trust.

### What was fixed

- **Completeness notes never reached the screen** (v0.1.0–v0.2.0).

  **What was wrong** — the note was printed only when `layers_missing` was non-empty. But notes that are
  not layer gaps do exist — netcap's warning when the capture window is cut short is one.

  **What came out wrong** — *"the capture window was cut short by a read error — this result does not
  represent the whole window"* **has never once appeared on screen.** Something written down honestly
  that never reaches its reader is the same as not writing it. This release's attribution reasons were
  about to land in the same place; writing the test is what exposed it.

### What was learned

- **Measuring on real hardware changed three things in the design.** ① File descriptors are inherited, so
  several processes hold one socket (three, measured) — taking the first PID found attributes the edge to
  the process that inherited it, not the one that opened it. Take the shallowest up the parent chain.
  ② A connection closed immediately is already gone by scan time — **attribution is best-effort.**
  ③ Reading another user's file descriptors takes more than `CAP_NET_RAW`.
- **An empty `app_key` means "could not attribute", not "no app".** Four distinct reasons: the socket
  closed, permission was missing, no stable key could be derived, or it was ambiguous. The rule this repo
  keeps for observation gaps carries over unchanged.
- **Ambiguity is not guessed.** When two apps talk to the same peer, the machine does not pick one.
  Attributing to the wrong app changes what gets acted on — worse than leaving it blank.
- **The demo missed 3 of 4 edges.** Its traffic is short-lived by construction, so that is close to a
  worst case, but short-lived connections themselves are not the exception. **The declared lane is now
  fixed as v0.4.0** — decided after measuring, so it is not speculative abstraction.
- **The signature range changed** — a contract field was added and `sign.Canonical` updated with it, so
  **signatures produced at v0.2.0 or earlier are invalid.** [Compatibility policy, section 2](docs/compatibility.md)
  is written for exactly this case, and before any real deployment is the cheapest time for it.
- **What had been deferred behind CNG was pulled forward.** The reason for deferring was "settle the
  app-pinning model after seeing both substrates, files and the registry" — but the axes were
  different. A substrate is a provisioning concept, where generated output is placed; pinning an app is
  discovery, matching a socket inode to a process. What changes on Windows is the *collection method*
  (`GetExtendedTcpTable`), not the way an app is pinned. And the materials were already here:
  `/proc/net/tcp` and `/proc/*/fd`, with netcap already running on that node. CNG, by contrast, needs
  real Windows hardware, and doing it first without that would add one more place like the v0.1.0
  `CngAxes` reservation — **schema present, never run**.


## v0.2.0 — moving the ingest path onto a many-users premise (2026-08-12)
**Goal** — fix the places where the inventory still ran on the assumption of one organization and one
execution. All six items apply **a principle this repo already keeps — drop something silently and it
reads as "absent" — to the ingest path.** A review from a consumer of the contract pointed at the spots.

### What was built

- **An organization axis** across six tables — `pqcota_snapshots` · `observations` ·
  `retention_events` · `provisioning_record` · `endpoint` · `profile`. **The store handle is bound to an
  organization**, so every query carries that condition and there is no way to drop it — nothing to
  remember per query means nothing to forget. `Nodes()` and `ByID()`, which used to sweep globally, came
  inside the organization without any interface change.
- **`pkg/org`** — the vocabulary for organization names. Lowercase, digits, hyphen, 2–64 characters (so
  `Acme` and `acme` cannot diverge); no empty organization; with `PQCOTA_REQUIRE_ORG=1` a store cannot be
  opened without one. `default` is **reserved** — it passes the shape rule, so leaving it open would let
  it be assigned as a real organization name and merge with single-organization-era data.
- **A guard on automatic DDL** — with `PQCOTA_AUTO_DDL=0` the schema is not created, and a missing schema
  is an error. This closes the case where a misdirected connection **created a fresh empty set of tables
  and wrote into them.**
- **A rejection history** (`pqcota_rejections`) — signature failures, unverified results, off-scope
  entries, and identity conflicts are recorded. The payload is not stored, only its canonical
  fingerprint: the store never holds unverified data, yet repeats can still be counted.
- **A mandatory signature mode** — with `PQCOTA_REQUIRE_SIGNATURE=1`, ingest **does not start** when there
  is no key to verify with. The report also counts `Unverified` separately: "verified and passed" and
  "there was no key to verify with" do not collapse into one number.
- **`sign.VerifyFrom`** — matches on `collector_id → public key`. The existing `Verify` tries **every**
  key it is handed, so a single list covering several collectors let a result that passed under any key
  arrive wearing any collector's name. Signatures now answer *who* produced this.
- **A `raw_capture` convention in the contract** — no file contents, packet payloads, or credentials.
  The field is free-form `bytes`, so the schema cannot enforce it; it is written down as a convention.
- **A [compatibility policy](docs/compatibility.md)** — five distinct faces: contract, signature, Go API,
  DB schema, mixed versions. So that "it is compatible" does not stay vague about which.

### What was fixed

- **Falling back to an in-memory store when Postgres could not be opened** (v0.1.0–v0.1.3,
  `pqcota-ingest` · `pqcota-cbom-ingest`).

  **What was wrong** — with `PQCOTA_DSN` set, a failure to connect or to prepare the schema printed one
  warning and **carried on with the in-memory store.** Supplying a DSN is a request for persistence, and
  that request was being silently cancelled.

  **What came out wrong** — the screen said **success.** The full "ingested: N accepted … N nodes
  observed" line printed normally, and the data vanished with the process. A failure that looks like a
  success. It surfaced while actually running `PQCOTA_AUTO_DDL=0` in this release: the schema was not to
  be created, there was no schema, and ingest reported success.

  **The fix** — if a DSN is given and the store cannot be opened, **stop.** The message that blamed an
  organization error on "Postgres connection failed" was corrected too.

### What was learned

- **Consumer code does not change by a single line.** The existing constructors stay, bound to
  `org.Default`, and new ones were added alongside. The `history.Store` interface is untouched — to ask
  about the organization, type-assert to `org.Scoped`. Single-organization users never meet the concept.
- **One migration was not idempotent.** In `pqcota_endpoint` and `pqcota_profile`, `node_id` was both the
  primary key and the upsert conflict target, so merely adding a column left **organization A's `web-01`
  still overwriting B's.** `ADD PRIMARY KEY` has no `IF NOT EXISTS`, so it runs conditionally on how many
  columns the current primary key has. Verified against real Postgres, run twice: existing rows are
  preserved as `default`, and the same `web-01` now coexists per organization.
- **An old binary can still write to the new schema — and that is the trap.** `DEFAULT` fills the
  organization column, so nothing blocks it, and a binary that knows nothing about organizations
  **writes silently into someone else's place.** The last (optional) migration step is therefore to drop
  the default, after which such an insert fails on `NOT NULL`. A loud failure instead of quiet
  contamination.
- **An in-memory isolation test does not prove isolation.** Separate objects cannot see each other by
  construction. A Postgres test that shares one table sits alongside it (`PQCOTA_TEST_DSN`).


## v0.1.3 — the collection timestamp was empty (2026-08-12)
**Goal** — fix one defect that had been there since v0.1.0. No functional change. It surfaced while a
downstream consumer was designing a per-result deduplication key.

### What was fixed

- **`Envelope.collected_at` was empty** (v0.1.0–v0.1.2 — four of the five places a result is built).

  **What was wrong** — only **one** place filled it: the openssl collector's gRPC service path. The jvm
  collector, the network collector, and the openssl **CLI path the demo actually uses**
  (`pqcota-nodescan` → `BuildResult`) emitted results with it left empty. Same collector, different
  provenance depending on which door the result came out of.

  **What came out wrong** — **nothing did.** The only reader of this value inside the repo is
  `sign.Canonical`, and the inventory's "when was this seen" comes from the ingest timestamp
  (`pqcota_observations.observed_at`). What was wrong is that **the signature was covering an empty
  field** — the collection time was inside the signed range, and what got signed was "we don't know
  when we looked".

  **When it would start coming out wrong** — the moment anything reads it. The jvm collector emits
  **one result per JVM**, so a receiver keying deduplication on `(collector_id, node_id, collected_at)`
  would **collapse several JVMs on one node into one.** It is latent, which is why the fix did not wait
  for a release.

  **Why it went unseen** — `collected_at` was the **only one of the Envelope's nine fields without a
  comment**. So the comment was fixed too: what the value means (not the ingest or output time), and
  that it **is filled on failed collections as well** — when the attempt happened is the basis of the
  gap record.

  **Signature compatibility** — `Canonical` reads the value out of the message, so **existing signatures
  remain valid**: an old result signed with a zero timestamp still canonicalizes to the same bytes. The
  contract change is comment-only, so `buf breaking` stays clean.

  **API compatibility** — no signature changed. The clock is a package variable (`var now = time.Now`)
  that tests swap out. Three regression tests, one per collector, hold the place.


## v0.1.2 — Putting reconciliation state in the vocabulary (2026-08-11)

`decision.proto` had the **conclusion** and the **lifecycle** of a verdict in the contract, but never
defined **what the verdict is about**. A comment pointed at `UNOBSERVED` while that value existed
nowhere in the contract ([#3](https://github.com/randyinthedev-hash/pqcota/issues/3)).

- **`ReconState` added** — `CONFIRMED` (declared ∩ observed), `UNDECLARED` (observed only = shadow),
  `UNOBSERVED` (declared only). **The reconciliation engine is not in this repository** — like
  `DecisionConclusion` and `FinalizedPlan`, only the schema lives here, so that consumer engines
  speak the same vocabulary.
- **`Decision.state` (field 10) added** — which state the verdict was made against. Without it a
  consumer receives a `conclusion` with no idea what it was about.

Purely additive — existing field numbers and types are untouched, so the `buf breaking` baseline holds.

---

## v0.1.1 — Making the contract consumable (2026-08-11)

**What was wrong** — the generated code (`gen/`) sat in `.gitignore`, so anyone trying to consume
the contract got no types from `go get`. `contracts/README.md` said the schemas were SSOT "so that
consumer engines speak the same vocabulary", yet that vocabulary could not be imported. The first
outside consumer surfaced it.

- **`gen/` is committed** — `go get` alone now gives you `commonv1`, `discoveryv1`, `inventoryv1`
  and `provisioningv1`. Hand-edited generated code is cut by the CI generate-drift check, which
  only now has anything to check.
- **buf pinned** (CI, 1.69.0) — with generated code committed, a tool version change could fail the
  drift check without any code change.
- How to consume it, and the module-path workaround, are written up in
  [contracts/README](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.md).

The contract itself (proto) did not change — the `buf breaking` baseline is untouched.

---

## v0.1.0 — First release (2026-08-11)

**Goal** — a **three-stage end-to-end** you can download and run. Per-arch static binaries and `SHA256SUMS` ship with the release; **signing** (ed25519) is deferred to a later release (see the roadmap above).

### What was built

- **Contract SSOT** — protobuf across 4 namespaces (`common` · `discovery` · `inventory` · `provisioning`), code generated with `make generate`.
- **Discovery** — three reference collectors. **openssl and jvm both start with a `/proc` sweep** — openssl for loaded libs, jvm enumerating running JVMs to attach to, distinguishing multiple JVMs per app. **network does not touch `/proc`**; it observes the wire passively through `AF_PACKET`. On top of those: a normalization pipeline (evidence · completeness map), history ingestion with ed25519 signing (every collector assertion is signed), and delegated CBOM intake.
- **Inventory** — central ingestion/query (Postgres), machine metadata (endpoints · profiles), app attribution, and **history browsing · snapshot detail · change diff** (`-history` · `-snapshot` · `-diff`).
- **Retention policy** — two-tier separation of observation records and snapshots (repeated observations of the same state don't grow storage, yet "when it was seen" is preserved) + pruning (`pqcota-prune`, dry-run by default · latest is inviolable · pruning is recorded).
- **Asset scope** — extends the node-registration gate to the asset level. Only the assets the user declared as managed are ingested, and the excluded count is reported (`pqcota-ingest -scope-assets`).
- **Provisioning generation** — the execution gate (finalized-only), taxonomy→config artifacts, apply/rollback Ansible playbooks (**L1/L2/L3**), before-state capture and rollback records.
- **L3 activation and restart** — the commands come from the plan's `activation` hooks (pre·activate·deactivate·restart), **written by the user**; the generator places them in the order that makes them safe (bring down → change → make referenced → restart), and rollback is the exact reverse. How to activate differs per environment, so **the tool does not guess**: an empty hook generates nothing, and what will not happen is reported.
- **CNG schema reservation** — adds `CRYPTO_RUNTIME_WIN_CNG` enum + `CngAxes` (oneof arm) to the contract (**not implemented** — the collector, normalization, and provisioning that fill it come in v0.2.0/v0.3.0 — **that was the plan at the time; they are now v0.4.0/v0.5.0, and the reason for the move is in the [roadmap](#roadmap--upcoming-releases-planned)**). This is the starting point of the staged rollout. What was confirmed is that **the contract has room for it** (purely additive — existing field numbers and types unchanged). Nothing has been run on real Windows, so this does not mean "CNG is supported".
- **Verification** — the demo's 6-stage end-to-end (the generated playbook is actually **applied, activated, and rolled back** on a real node), per-stage examples, all 172 tests green (level distribution), and a docs gate (`make check-docs` — links, anchors, stale scope claims, personal data).
- **Real-provider check (optional stage)** — with `DEMO_REAL_PROVIDER=1` the demo builds a real oqsprovider, deploys and activates it on an OpenSSL 3.0–3.4 node, and measures whether the capability **actually appeared**, via `openssl list` (ML-KEM KEMs 0 → 14; back to 0 after rollback). This check caught a defect in the config-file generation: the generated fragment lacked the top-level `openssl_conf = openssl_init`, so in environments that point `OPENSSL_CONF` at it, **the module was placed and the sha256 gate passed while the provider never loaded**. Fixed, with a regression test.

- **Releasing** — pushing a tag makes CI build per-arch static binaries (`linux-amd64`, `linux-arm64`) and `collector.jar`, then attach `SHA256SUMS`. Verify what you download with `sha256sum -c SHA256SUMS`.

### What was established

- **Minimum supported kernel = 3.2** (the floor the Go toolchain sets — it became this in 1.24 and has held since. Building needs Go 1.26.4, per `go.mod`). Nothing here needs anything newer; the one per-feature addition is `NSpid` (4.1) for JVM attach inside containers, and that falls back to the host PID. Table: [discovery/cmd — supported range](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md).
- **Legacy verification done** — all three collectors ran on kernel **3.2** (Ubuntu 12.04) and **3.10** (CentOS 7.9) VMs. They work at the floor itself, and neither kernel has `NSpid`, so the host-PID fallback was exercised for real. 3.2 has no systemd, so app attribution by executable path was observed as well.

---

<!-- Add each new version as a new section above this line (newest on top). -->
