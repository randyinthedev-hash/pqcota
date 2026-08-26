English · [한국어](RELEASE_NOTES.md)

# Release Notes — pqcota

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](RELEASE_NOTES.md) is authoritative.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](docs/regulation.en.md).

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

Directional, not fixed. Each version is promoted to a proper section per the rule above once started/completed. The **Windows CNG runtime is introduced in stages** — why it isn't added all at once, plus the pressure test: [Accepting a new crypto runtime](docs/runtime-acceptance.en.md).

- **v0.7.0 (planned)** — **CNG provisioning**: **substrate generalization first** (moving past the POSIX-file assumption — Windows uses the registry/GPO, which doesn't fit `/opt/pqcota` file staging or file-removal rollback) → `renderCNG`. The generalization is done together with that implementation (no speculative abstraction). Where to draw the seam is still undecided — [Designs under review §2.2](docs/under-review.en.md).

- **Observing OpenSSL on Windows (planned · version TBD)** — `pqcota-nodescan` has a single
  implementation today and it reads `/proc`. Run it on Windows and it emits a gap rather than an empty
  result, but it **cannot observe**. What is needed is swapping two pieces: finding the loaded modules
  (`procmaps.go` → the Toolhelp32 module list) and pulling strings out of a binary (`elfstrings.go` →
  PE). **Fork detection (`registry.MatchFork`) takes only the extracted strings, so it is reused
  as-is** — the same shape the jvm reconnaissance took, per-OS I/O over shared pure matching. Which
  collector runs on which OS is in the [command reference](discovery/cmd/README.en.md).

- **Accepting the provider ecosystem (under review · version TBD)** — choosing which provider to use, and obtaining its file, is done by whoever writes the plan. What this repo does is **write the configuration file that activates that provider**. Today it only knows one shape, `activate`+`module` — and since each provider demands different settings, it cannot yet produce one for OpenSSL's own `fips` module (which has to pull in the file `fipsinstall` generates) or for pkcs11-provider (which needs additional entries such as the driver path). What each candidate would additionally require, along with provider observation and the HSM axis, is worked out in [Designs under review](docs/under-review.en.md).

- **Observing server-role edges (planned · version TBD)** — today netcap emits **only the edges the observing node initiated**. So a **process serving on that node gets no edge at all**, and the comparison that ties "this app loaded that library" (found by nodescan) to "this is what it actually negotiated" through `app_key` is entirely empty on the server side. The contract can already carry this with `EdgeRole.SERVER`; what blocks it is the value conventions and deduplication. The design is in [Designs under review §5](docs/under-review.en.md).

- **Separating the traffic-observation cadence (planned · version TBD)** — assets and communication edges are observed on different cadences. Today they are bound into one playbook, so seeing more edges means re-running the `/proc` sweep and the JVM attach along with it. `discover.yml` is left alone and a playbook that runs netcap in rounds is **added**. What has to be solved along with it is **merging repeated observations**: the core only drops duplicate edges today, so `observed_count` stays at 1, `first_seen` and `last_seen` are filled by nobody, and the merge key differs in three places. The design is in [Designs under review §7](docs/under-review.en.md).

- **Scope classification for the edge peer (planned · version TBD)** — when an observation meets a node that is not in the scope master, §1.4 and §2.5 say to send it to a **registration request**, and SD-5 even set an acceptance criterion for it — yet **the classifier (`ClassifyObserved`) exists and nothing calls it.** An unregistered peer stays as an address in `dst_addr`, visible on screen but never collected. This is the only automatic way to surface a node the operator failed to register, and it is not connected today. The design is in [Designs under review §6](docs/under-review.en.md).

- **Binding the signature to a collector (planned · version TBD)** — the ingest today confirms only that
  **someone signed**. `PQCOTA_VERIFY_KEY` is a list of keys, so there is no way to check it against
  `envelope.collector_id`, and whoever holds one key can arrive **wearing any collector's name**.
  `sign.VerifyFrom`, which verifies against that binding, already exists and **nothing calls it**. What
  blocks it is the environment-variable format, so it is not a one-line change. The design is in
  [Designs under review §8](docs/under-review.en.md).

- **Release signing (planned · version TBD)** — the **ed25519 signature and `pqcota-verify-bundle`**. The bundle layout, signing, and verification are settled in the [collector deployment design](discovery/collector-deployment.md) (Korean). Until then, verify integrity with `sha256sum -c`.

### Not on the roadmap — deliberately

These are **boundaries**, not directions. Written down so no one waits for them.

| Not built | Instead |
|---|---|
| **Fleet orchestration** — drain · rolling · health gates | Standard Ansible playbooks come out, so your deployment tooling drives them |
| **Remote execution engine** — resident agents, push channels | You run the generated artifacts on your own substrate |
| **Source / artifact CBOM scanner** | CI already has the source. CycloneDX from CBOMkit and friends is **ingested** instead |
| **Dynamic tracing** (eBPF · ltrace) | Invasive, so it isn't done. Observing the actual negotiation on the wire was chosen instead |
| **Verdicts and scoring** — "risky" grades | Only observed facts are emitted. What to change, and when, is yours to decide |
| **An admin screen for entering values by hand** | These arrive as files (`hosts.csv`, `scope-assets.csv`, machine profiles, the finalized plan JSON, `pqcota-declare`). Build a screen and a review queue and an approve button follow, and both are [explicitly excluded by the architecture](docs/architecture.en.md#62-explicit-exclusions--boundaries). At that moment an observation tool becomes an adjudication tool |


---

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
  the `fips-standard` from the maturity table in the [architecture](docs/architecture.en.md).

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
  | [collector deployment](discovery/collector-deployment.md) (Korean) | `①` meant both a **deployment step** and an **attach layer** | the layer is spelled out |
  | [inventory/cmd](inventory/cmd/README.md) (Korean) | "② signs it" — another document's section number, **and the wrong actor** | the **collector on the node** signs |
  | [provisioning](provisioning/README.en.md) | `L1/L2/L3` in the first sentence, explained only inside a **collapsed** diagram | the first sentence now says what they are |

- **③ `pqcota-keygen` was documented in two places** (v0.6.3). It now lives with the command, in
  [discovery/cmd](discovery/cmd/README.en.md), and the `PQCOTA_VERIFY_KEY` row points there.

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
  calls no `certutil`, PowerShell or WMI, for the same reason the cng-collector does not (§2.3). **It
  also never reads another process's memory (the PEB)** — so the command line, and with it the app
  name, stays empty. That fact is carried as a value and reported on screen.
- **`pqcota-windows-amd64.zip` is attached to the release** — until now you had to build it yourself.
- **The command reference is complete again** — `pqcota-cngscan` (added in v0.6.0 but never listed) and
  `pqcota-keygen` are in, and the per-collector OS table now lives in **one** place.

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
  JVM existed**. "Not observed" wore the face of "not there" (§2.6). netcap sends its capture failure
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

- **The «Language» rule — [CONTRIBUTING](CONTRIBUTING.en.md)** — which language each kind of text
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
  [expected-output](demo/expected-output/README.md) (Korean) was captured fresh. The console blocks in the
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
  **empty** — "not asked" and "asked and found none" are not written the same way (§2.6).
- **Windows `hardware_uuid`** — SMBIOS Type 1, read via `GetSystemFirmwareTable('RSMB')`; no WMI or
  PowerShell (§2.3). `MachineGuid` is per **installation** (a reinstall changes it), so a per
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
  `derived_from` stayed put even once `hardware_uuid` appeared (§1.4). The thing to fear when adding
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

- **`pqcota-cngscan` · [cng-collector](discovery/collectors/cng/README.md) (Korean)** — calls
  `bcrypt.dll` directly (`BCryptEnumRegisteredProviders`, `BCryptEnumAlgorithms`). It does not invoke
  `certutil`, PowerShell, or WMI — on servers where script execution is blocked by policy, a failed
  observation must not scatter into "environment problems" (§2.3). **Off Windows it emits a gap, not
  an empty result**, and exits 0.
- **`CngAxes.algorithms` and `CngAlgorithm`** — added after the measurement, with new field numbers
  (purely additive), exactly as the v0.1.0 reservation note prescribed. Provider names alone do not
  answer the question (below).
- **`COLLECTION_LAYER_CNG_INTROSPECTION`** — for the same reason JCA has its own layer. It is neither
  process nor artifact but **a query of the providers registered on the machine**, so what it fails to
  see differs from every other layer.
- **The screen** — the file view and the inventory view both render CNG assets. `readiness` is derived
  through `registry.MatchPQC` and is **a summary of the observation, not a verdict** (architecture §6).
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
- **CNG's FIPS mode cannot be known from an algorithm enumeration** — `fips_validation` is `unknown` (§2.5).

### Fixed

- **Windows nodes hung on the hostname**(v0.1.0–v0.5.0; surfaced by the first Windows observation).

  **What was wrong** — machine fingerprinting only read `/etc/machine-id` and DMI. Those paths do not
  exist on Windows, so every field came back empty and the last-resort `fqdn` became the anchor.

  **What came out wrong** — the first measurement's `derived_from` was `fqdn`. **Rename the host and
  the same machine becomes a different node**, splitting its history. Linux nodes never showed this
  because they were already on `machine-id`.

  **What changes** — only the *source* of the fingerprint is split per OS; the rules stay in one
  place. Windows reads the registry's `MachineGuid` directly. `hardware_uuid` stays **empty** for now
  — it lives in SMBIOS, and what cannot be read is not invented (§2.5).

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
  fetches the new path. No signature or type changed ([compatibility policy §3④](docs/compatibility.md) (Korean)).

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

- **[The journey](journey.md) (Korean)** — one document that walks the whole way: preparation,
  observation, ingestion, querying, generation, application, rollback. The regulation fixes the rules
  and each stage design covers its own inside, but nothing answered "in what order does what come out,
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
  **signatures produced at v0.2.0 or earlier are invalid.** [Compatibility policy §2](docs/compatibility.md) (Korean)
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
- **A [compatibility policy](docs/compatibility.md)** (Korean) — five distinct faces: contract, signature, Go API,
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
  [contracts/README](contracts/README.en.md).

The contract itself (proto) did not change — the `buf breaking` baseline is untouched.

---

## v0.1.0 — First release (2026-08-11)

**Goal** — a **three-stage end-to-end** you can download and run. Per-arch static binaries and `SHA256SUMS` ship with the release; **signing** (ed25519) is deferred to a later release (see the roadmap above).

### What was built

- **Contract SSOT** — protobuf across 4 namespaces (`common` · `discovery` · `inventory` · `provisioning`), code generated with `make generate`.
- **Discovery** — three reference collectors. **openssl and jvm both start with a `/proc` sweep** — openssl for loaded libs, jvm enumerating running JVMs to attach to, distinguishing multiple JVMs per app. **network does not touch `/proc`**; it observes the wire passively through `AF_PACKET`. On top of those: a normalization pipeline (evidence · completeness map), history ingestion with ed25519 signing (every collector assertion is signed), and delegated CBOM intake.
- **Inventory** — central ingestion/query (Postgres), machine metadata (endpoints · profiles), app attribution, and **history browsing · snapshot detail · change diff** (`-history` · `-snapshot` · `-diff`).
- **Retention policy** — two-tier separation of observation records and snapshots (repeated observations of the same state don't grow storage, yet "when it was seen" is preserved) + pruning (`pqcota-prune`, dry-run by default · latest is inviolable · pruning is recorded).
- **Asset scope** — extends the node-registration gate (§1.4) to the asset level. Only the assets the user declared as managed are ingested, and the excluded count is reported (`pqcota-ingest -scope-assets`).
- **Provisioning generation** — the execution gate (finalized-only), taxonomy→config artifacts, apply/rollback Ansible playbooks (**L1/L2/L3**), before-state capture and rollback records.
- **L3 activation and restart** — the commands come from the plan's `activation` hooks (pre·activate·deactivate·restart), **written by the user**; the generator places them in the order that makes them safe (bring down → change → make referenced → restart), and rollback is the exact reverse. How to activate differs per environment, so **the tool does not guess**: an empty hook generates nothing, and what will not happen is reported.
- **CNG schema reservation** — adds `CRYPTO_RUNTIME_WIN_CNG` enum + `CngAxes` (oneof arm) to the contract (**not implemented** — the collector, normalization, and provisioning that fill it come in v0.2.0/v0.3.0 — **that was the plan at the time; they are now v0.4.0/v0.5.0, and the reason for the move is in the [roadmap](#roadmap--upcoming-releases-planned)**). This is the starting point of the staged rollout. What was confirmed is that **the contract has room for it** (purely additive — existing field numbers and types unchanged). Nothing has been run on real Windows, so this does not mean "CNG is supported".
- **Verification** — the demo's 6-stage end-to-end (the generated playbook is actually **applied, activated, and rolled back** on a real node), per-stage examples, all 172 tests green ([level distribution](docs/test-map.md) (Korean)), and a docs gate (`make check-docs` — links, anchors, stale scope claims, personal data).
- **Real-provider check (optional stage)** — with `DEMO_REAL_PROVIDER=1` the demo builds a real oqsprovider, deploys and activates it on an OpenSSL 3.0–3.4 node, and measures whether the capability **actually appeared**, via `openssl list` (ML-KEM KEMs 0 → 14; back to 0 after rollback). This check caught a defect in the config-file generation: the generated fragment lacked the top-level `openssl_conf = openssl_init`, so in environments that point `OPENSSL_CONF` at it, **the module was placed and the sha256 gate passed while the provider never loaded**. Fixed, with a regression test.

- **Releasing** — pushing a tag makes CI build per-arch static binaries (`linux-amd64`, `linux-arm64`) and `collector.jar`, then attach `SHA256SUMS`. Verify what you download with `sha256sum -c SHA256SUMS`.

### What was established

- **Minimum supported kernel = 3.2** (the floor the Go toolchain sets — it became this in 1.24 and has held since. Building needs Go 1.26.4, per `go.mod`). Nothing here needs anything newer; the one per-feature addition is `NSpid` (4.1) for JVM attach inside containers, and that falls back to the host PID. Table: [discovery/cmd — supported range](discovery/cmd/README.en.md).
- **Legacy verification done** — all three collectors ran on kernel **3.2** (Ubuntu 12.04) and **3.10** (CentOS 7.9) VMs. They work at the floor itself, and neither kernel has `NSpid`, so the host-PID fallback was exercised for real.

---

<!-- Add each new version as a new section above this line (newest on top). -->
