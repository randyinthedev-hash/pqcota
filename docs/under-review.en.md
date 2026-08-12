English · [한국어](under-review.md)

# Designs under review — what is not built yet

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](under-review.md) is authoritative.

> **This document changes no code.** It is where the design work happens for things that made the roadmap
> but are not committed to implementation. The real design documents (discovery, inventory, provisioning)
> record **only what stands today** — mixing in what is under review makes it impossible to tell fact from plan.

**Lifetime** — an item ends in one of four ways.

| | |
|---|---|
| **① Onto the roadmap** | once there is a direction, one line in the [roadmap](../RELEASE_NOTES.en.md); the design stays here |
| **② Designed here** | no code changes. As far as what gets stuck and what must be decided |
| **③ Promoted to a real design** | once the implementation plan is settled, it lands in that stage's design document and **is deleted from here** |
| **④ Decided against** | moved to the roadmap's ["what we do not build"](../RELEASE_NOTES.en.md) |

**An open question another document already holds does not live here.** Written in two places, only one gets fixed — the HSM axis really was in both here and the appendix of the [crypto runtime acceptance principles](runtime-acceptance.en.md). An open question stays with the document that produced it; this page takes only **what is on the roadmap**.

---

## 1. What "supporting" a provider means

**The user chooses and prepares the provider.** What receives that provider splits into three layers — the first two are things the tool does at run time, and the third is the check you go through when adding a candidate:

| Layer | What it is | Today |
|---|---|---|
| **The staging mechanism** | staging, the sha256 gate, and rollback accept the module file the user supplied (an OpenSSL `.so`, a JCA `.jar`) | ✅ provider-agnostic — already fully done |
| **The config vocabulary** | the tool produces **the configuration fragment that provider requires** | ⚠ today there is exactly one shape: `activate = 1` + `module = path` |
| **Verification against the real thing** | when adding a candidate, verify it end to end for real (stage → activate → observe) and write it into the candidate table | ◐ only oqsprovider is verified — the other candidates are not |

**"We support wolfProvider" = the config vocabulary knows that provider's shape + it has been verified for real.** What each candidate additionally needs is tabulated in the [plans README](../examples/provisioning/plans/README.md) (Korean) — that is where you pick `providerChoice` while writing a plan.

Confirming a FIPS-validated build is the user's job (a certificate is per build, and you cannot tell from the file alone). What the tool guarantees is **that the exact file the user chose is the one that got staged** (sha256).

**The remaining candidates get verified the same way** — stage and activate the real thing, measure before and after with `openssl list` to see whether the capability appeared, then undo. oqsprovider walked that path, and the demo's `DEMO_REAL_PROVIDER=1` runs exactly that ([demo README](../demo/README.en.md#optional-step--the-last-inch-with-a-real-provider-demo_real_provider1)).

**What must be decided — how to open up the config vocabulary.** Whether to add a per-provider fragment to the code one at a time, or to open a place in the plan where a config fragment can ride (something like `extraConfig`). The latter accepts every candidate without touching the code, but it has to be weighed against the "we do not invent" principle.

---

## 2. Runtime candidates not yet accepted

**A language surface is not a candidate.** Python (which links libcrypto), Node (mostly a delivery vehicle for OpenSSL), and .NET (CNG or OpenSSL depending on the OS underneath) have no substrate of their own and belong to openssl or CNG — only the three below actually had anything to test against the conditions.

### 2.1 Static Go — it gets stuck on condition 1

There is **no** target for runtime introspection (no provider). Static ELF symbols and `debug/buildinfo` get you as far as the `artifact` and `symbol-analysis` lanes (`inferred-low`, not `confirmed`), and the negotiated algorithm is already seen on the wire by netcap, language-agnostically. With no provider, remediation is the single option `REBUILD`, and there is no file artifact to deploy either (a rebuild is CI's job).

**Verdict: the contract holds, but it holds by "absorption".** No new enum, render, or substrate is needed — Go is not a new runtime but **a consumer of the existing taxonomy**.

### 2.2 Windows CNG — it gets stuck on condition 2

Collection is entirely new (`BCryptEnumProviders`, the registry — zero reuse of `/proc`, ELF, or AF_PACKET). The schema attaches purely additively, being a oneof. The kind vocabulary is runtime-agnostic too, so **classification** is reused.

Where it breaks is the artifact. What `Render` has to emit is not `[openssl_init]` text but the registry, GPO, and `Register-CngProvider` — and that *is* the substrate: **the render layer does not separate from the substrate.**

**Verdict: only half of it lands.** The discovery and inventory half comes into today's frame as-is, but the provisioning half **requires substrate generalization first**. That generalization happens **together with implementing Windows** — extracting a Registry/GPO interface before there is a concrete case would cut the seam in the wrong place (no speculative abstraction).

**What is undecided** — where to draw the seam of the substrate abstraction (File-Stage vs Registry/GPO vs Config-Only). It is not decided until a Windows implementation is actually in hand.

### 2.3 HSM / PKCS#11 — it touches none of the conditions

The remediation is usually ***pointing* openssl (`pkcs11-provider`) or jca (`SunPKCS11`) at the HSM**. Rendering means putting a parameter on the existing `PROVIDER_INJECT` saying "this provider's target is an HSM slot", and the substrate (file staging + a config fragment) is used unchanged.

**Verdict: it is not a new `CryptoRuntime` enum.** It is modeled as a new discovery axis plus the **backend target** the provider points at. The only genuine peer case is an app linking a vendor `.so` **directly, without openssl or jca**, and even that is provider-isomorphic, so it is approximated by the openssl-family render.

> **An honesty footnote** — where the HSM hardware does not support PQC yet, no artifact is invented; it is handled with a `DECOMMISSION` or `APP_RECONFIG` comment. The taxonomy already has these non-config kinds.

**What is undecided** — the actual fields of the HSM axis (slot, module path, firmware version, and so on). They get decided when there is a real PKCS#11 observation to work from.

### 2.4 Only one layer gets stuck

| Candidate | The assumption it hits | Verdict |
|---|---|---|
| Static Go | 1 (provider isomorphism) | **absorbed** into the `REBUILD` taxonomy — not an enum |
| Windows CNG | 2 (POSIX files) | a separate track — substrate generalization comes first |
| HSM/PKCS#11 | none | a new **axis + backend target** — not an enum |

The conditions and how to decide are set by the [crypto runtime acceptance principles](runtime-acceptance.en.md).
What is here is **the result of holding three specific candidates against those conditions** — as candidates accumulate, they attach here.

---

## 3. Moving the case ↔ test correspondence into a gate

`checkdocs` blocks links, anchors, empty sections and the licence table — yet **the one thing this
repository leans on to claim coverage is left to human hands.** The counts in the
[test map](test-map.md) (Korean) can drift from the links in the case tables and the build still passes.

| | State |
|---|---|
| Case ID → test file link | present in the case tables, but only the generic link check sees it |
| Case ID → test **function** name | written in the case tables, **never verified** |
| Test file → its own case ID | **absent** |
| Per-level counts (unit/integration) | written by hand in the docs |

One rule would cover it — scrape `TD-`, `TV-`, `TP-`, `TK-` IDs with their links and function names out
of the case tables, then check that the file really contains that function. Whether to force the reverse
direction (a first-line comment in each test file) is a separate question: weigh the cost of annotating
55 files against the risk of those annotations going stale.

**Why not yet** — every gate added also raises the bar for contributors. While the case tables are
maintained by hand, a rule may block people more than it catches. Add it once the tables have settled.

---

## 4. Letting a person correct identity resolution

**Identity resolution**, the fifth step of the six-step normalization, runs on rules alone today — same
path, soname and hash means one asset, otherwise they stay apart. There are real cases the rules cannot
merge.

| Situation | Today's result |
|---|---|
| The same library lives at different paths on different nodes | they stay separate assets |
| A vendor-renamed build and the original are the same thing | they stay separate assets |
| An app loaded the real file behind a symlink | different paths, so they stay separate |

There is no way for an operator to say "these two are the same". The inventory can therefore show more
assets than really exist, and whoever plans the migration has to track that duplication by hand.

**This is correcting an observation, not passing judgment.** It differs from the tool deciding what is
vulnerable or what to change first — "these two observations point at the same thing" is a statement of
fact, so it does not collide with the [no-judgment principle](architecture.en.md#6-the-no-judgment-principle).

**Three things it would have to honour**

1. **The original is never edited** — merging happens only in the derived view. The raw observation has to
   survive so that a better rule can split it again on recomputation (reproducibility).
2. **Who merged, and when, is recorded** — a judgment a person entered has to stay distinguishable from an
   observation. Just as `detection_method` records where an observation came from, a merge needs its own.
3. **It can be undone** — there has to be a path back out of a wrong merge.

**Why not yet** — honouring those three needs a new place in the derived view for facts a person entered,
and putting that in `contracts/` widens the contract. Better to settle it after real cases have shown
where the rules actually fail — building the schema first would be speculative abstraction, the same
reason [CNG was deferred](#2-runtime-candidates-not-yet-accepted).

---

## 5. Attributing edges to apps (planned for v0.3.0)

> **Decided to do it.** It is v0.3.0 on the [roadmap](../RELEASE_NOTES.en.md#roadmap--upcoming-releases-planned).
> What remains here is **how** — the three paths below, and what was ruled out.

The two observations **meet only at the node** today.

| Observation | Attributed to |
|---|---|
| `Finding` from the openssl and jvm collectors | `app_keys` — **per app (process)** |
| `ObservedEdge` from netcap | `src_node_id`, `dst_node_id`, `port` — **per node** |

`ObservedEdge` has no `app_key`. So when two server processes on one node use the same libcrypto and one
edge is observed, **the inventory does not know which process opened that connection.**

There is a reason in principle. netcap observes the wire passively through `AF_PACKET`, and packets carry
no PID for the socket that opened them. The port allows a guess, and this repository does not write
guesses down as facts.

**What that costs today** — you cannot tell in advance which edge will follow when one app is moved to
PQC. It has to be confirmed by observing again afterwards. The closed loop is what covers that gap.

### 5.1 The automatic path — one more observation

Correlating socket inodes from `/proc/net/tcp` (and `tcp6`) against `/proc/*/fd` at capture time yields
the PID that opened the connection. netcap already runs on that node, so the place is right, and the
contract change is purely additive — one `app_key` field on `ObservedEdge`, no existing field numbers or
types touched.

The limit is clear: **the socket has to be alive at the moment of capture.** Short-lived connections are
missed. What is missed stays blank with a lower `evidence_strength` — as long as it is not written down
as absent.

#### What measuring it revealed

The correlation was tried against real TCP connections on Linux hardware. **It works** — an inode yields
a PID and its cgroup. But three things the design did not mention came out.

**① Several processes hold one socket.** File descriptors are inherited, so children keep a connection
their parent opened. The measurement found **three PIDs** on one inode (`bash` plus its two `sleep`
children). Reading `comm` off the first hit answers `sleep`, but **the process that opened the connection
was `bash`.** Taking the first PID found attributes the edge to the wrong app.

→ A rule is needed: among the PIDs holding the same inode, take **the shallowest one up the parent
chain.** If an ancestor is in the set, that is the side that opened the socket.

**② The race is worse than assumed.** Right after opening and immediately closing three connections,
`/proc/net/tcp` held **zero** of them. "Correlate at capture time" was the plan, but the socket can
already be gone between seeing the handshake on the wire and reading `/proc`. **Attribution is
best-effort** — an implementation can narrow that window, not close it.

→ So **what is missed must not be written as "no app".** An empty `app_key` means "could not attribute",
not "this edge has no app", and drawing that line is what the completeness map already does. The rule
this repo keeps for observation gaps applies unchanged to attribution.

**③ Another user's file descriptors are not readable.** As an ordinary user, `/proc/1/fd` is denied.
netcap already requires `CAP_NET_RAW`, and **that is not enough here.** Missing something for lack of
permission is also "could not attribute", not "no app" — and since the reason differs from ②, it is
worth recording separately.

**What the app key comes from** — the contract defines `app_key` as a *"machine-scoped stable key
(systemd unit name, exe path, …)"*. In the measurement `comm` gave `sleep` (unstable), `exe` gave
`/usr/bin/sleep` (a path, but not an app), and **`cgroup` gave the systemd unit**. cgroup is closest to
what the contract describes — and `procs.AppKey` already derives exactly that.

### 5.2 The manual path — reuse the declared lane

For what the automatic path misses, an operator supplies it. This has the same shape as something that
already exists: `pqcota-declare` imports CMDB declarations as `detection_method=UNSPECIFIED`, keeping them
distinguishable from observations. An attribution a person entered can arrive through the same lane and
never mix with what was observed. A CSV line is enough.

> **Not in v0.3.0.** It waits until the demo measures how much the automatic path (5.1) actually misses.
> Building it now would be speculative abstraction — laying the road for filling gaps before knowing how
> many are left to fill. The automatic path now **reports the count and the reasons in the completeness
> note.** That number is what this decision will rest on.

### 5.3 No admin UI

Facts entered by people already arrive as files here — `hosts.csv`, `scope-assets.csv`, machine profiles,
the finalized plan JSON, `pqcota-declare`. Five formats already work that way. **A UI would add
convenience, not capability.**

The real reason to leave it out is elsewhere. Build a screen and **the review queue and the sign-off
button follow.** Both are [explicitly excluded](architecture.en.md#62-explicit-exclusions--boundaries),
and on a screen "let people approve it here too" is the natural next step. At that moment an observation
tool becomes a judgment tool.

### 5.4 Why v0.3.0 — ahead of CNG

This work once sat behind CNG. The reason given was **"settle the attribution model after seeing both a
file substrate and a registry one"** — on a second look, that reason does not hold.

**Different axes.** Substrate is a provisioning concept: where generated artifacts land (`/opt/pqcota`
file staging vs. the registry/GPO). Attribution is discovery: matching a socket inode to a process. What
changes on Windows is the *collection method* (`GetExtendedTcpTable`), not the attribution model.

**App identity is already settled.** v0.1.0 already separates multiple JVMs per app through `app_key`.
What 5.1 does is carry that key onto `ObservedEdge` — using something that exists in one more place,
not standing up a new model.

**The materials are already here.** `/proc/net/tcp` and `/proc/*/fd`, and netcap already runs on that
node. CNG, by contrast, needs real Windows hardware — doing it first without that would add one more
place like the v0.1.0 `CngAxes` reservation, **schema present, never run**.

**It makes what already ships more correct.** Singling out communication that no declaration covers is
the first value of this observation, and an edge that stops at the node stops at "somewhere on this
server". What a person acts on is an app, not a server.

**Then why was it not folded into v0.2.0** — v0.2.0 (the ingest path) was work a consumer of the
contract was **blocked on and waiting for**, and it was mechanical. Nothing waits on this. Releases are
cut by who is waiting.
