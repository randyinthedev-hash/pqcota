English · [한국어](under-review.md)

# Designs under review

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](under-review.md) is authoritative.


> **This document changes no code.** It is where the design work happens for things that made the roadmap
> but are not committed to implementation. The real design documents (discovery, inventory, provisioning)
> record **only what has been decided** — mixing in what is under review makes it impossible to tell fact from plan.

> **§ notation**: unless stated otherwise, these are section numbers in the
> [process regulation](regulation.en.md). Sections of this document itself are referred to in context,
> as in `§5.1`, or by link.

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

## 5. Attributing edges to apps (done — automatic in v0.3.0, declared in v0.4.0)

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

#### Ingest must not patch the observed edge

Filling the blank `app_key` during ingest looks simpler. **Two things block it.**

**① The signature.** `sign.Canonical` covers `ObservedEdge` including `app_key` (v0.3.0). If ingest
fills that field, **what is stored differs from what the collector signed.** A declaration is not the
collector's claim, so it has no place inside that signature.

**② Re-normalization** ([process regulation](regulation.en.md) §1.2). When the enrichment rules improve, results are recomputed from
`raw_capture`. If ingest has edited the observation, the recomputed value and the stored one diverge —
and it stops being clear which is the original.

**So storage stays separate and the screen joins them.**

| | |
|---|---|
| Declaration | its own `CollectionResult` — `detection_method=UNSPECIFIED`, the lane `pqcota-declare` already uses |
| Observed edge | **untouched.** The signature and re-normalization still hold |
| Screen | the declaration is laid over the blank as `@payment-gw(declared)` — the basis stays visible |

**The contract is not touched.** `app_key_kind`, added in v0.3.0, already answers "what does this key
rest on" — `declared` simply joins `systemd-unit` and `exe-path`. `ObservedEdge.detection_method` must
not be used for this: it says **how the edge itself was observed**, not where the key came from, so
putting `UNSPECIFIED` there would blur the fact that the communication really was seen.

#### What the demo measured — the automatic path is not enough on its own

From the v0.3.0 demo (six stages):

```
3 of 4 edges could not be attributed to an app.
Reason: the socket closed between capture and lookup (3)

  web-gw → 172.18.0.3:8443  tls  X25519MLKEM768  @?
  web-gw → 172.18.0.4:4433  tls  x25519          @/usr/bin/openssl(exe-path)
  web-gw → 172.18.0.3:22    ssh  sntrup761x25519 @?
  web-gw → 172.18.0.4:22    ssh  curve25519      @?
```

**A caveat comes first** — this demo's traffic is **short-lived by construction**
(`echo Q | openssl s_client`, `ssh … true`). So 75% is not an average but close to a worst case; a
long-running service is caught far more often. The one that was caught was the relatively long-lived
`s_client`.

The conclusion still holds. **Short-lived connections are not the exception** — batch jobs, health
checks, cron, and SSH itself are exactly that. There is a place the automatic path cannot see by
construction, and a way to fill it is needed.

**Decision: build the declared lane. It is v0.4.0.** With a measurement behind it, this is no longer
speculative abstraction — the road for filling gaps is being laid after counting how many are left.

### 5.3 No admin UI

Facts entered by people already arrive as files here — `hosts.csv`, `scope-assets.csv`, machine profiles,
the finalized plan JSON, `pqcota-declare`. Five formats already work that way. **A UI would add
convenience, not capability.**

The real reason to leave it out is elsewhere. Build a screen and **the review queue and the sign-off
button follow.** Both are [explicitly excluded](architecture.en.md#62-explicit-exclusions--boundaries),
and on a screen "let people approve it here too" is the natural next step. At that moment an observation
tool becomes a judgment tool.

### 5.4 Why it came ahead of CNG

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

---

## 6. Edges where the observing node is the server

Today netcap emits **only the edges the observing node initiated**. [`edgeFor`](../discovery/collectors/network/capture_linux.go)
treats the lower port as the server side and emits nothing when the local end is not the client. The point
was to remove duplication and direction confusion, since the same handshake is visible at both ends, and
the assumption was that whatever the server side saw would be reported by the peer node.

### 6.1 What is missing

**A process serving on that node gets no edge at all.** The observing node plainly sees that handshake on
the wire and even parses it, then drops it just before emitting. Two things are blocked together.

**① The comparison that ties assets to observation is empty on the server side.** What this repo records as
its core value is that "where the other collectors see a node's **capability**, netcap sees the **grade in
effect** — and the comparison of those two" ([network collector README](../discovery/collectors/network/README.md) (Korean)).
That comparison runs through `app_key`. nodescan says `payment-gw` loaded `libssl.so.1.1`; netcap says what
that app actually negotiated. But when `payment-gw` is the one **listening**, netcap emits no edge for it,
so one side of the comparison is absent. Pinning down which app negotiates what through which library is
within reach, and right now it is open only for the client role.

**② When the peer is not in scope, nobody reports that connection.** If the peer is a registered node running
a collector, that side reports it. But an external client, or a workload nobody has registered yet, has no
one to report it. [The process regulation §4.6](regulation.en.md) records the server-side migration as the
more dangerous one, and yet there is no path to see what my own server negotiates, and with what.

"Not observed" and "observed and then dropped" are different things. This is the second.

### 6.2 The contract can already carry it

[`edge.proto`](../contracts/proto/pqcota/discovery/v1/edge.proto) modelled direction from the start.

```proto
string src_node_id = 1;   // the scope-master anchor — the capturing host
EDGE_ROLE_CLIENT = 1;     // src opened the connection
EDGE_ROLE_SERVER = 2;     // src received it
```

`src_node_id` is the observing node regardless of role, and `role` states which way the arrow points. So no
new field and no contract change is needed. `edgeRole` already knows how to emit `SERVER`; only the path
that fills it is missing.

### 6.3 Two value conventions to settle

**`dst_addr` is emitted as `peerIP:0`.** In the server role the peer is a client, and its port is an ephemeral
one that changes per connection. Carrying it verbatim makes [`edgeKey`](../pkg/discovery/normalize/pipeline.go)
differ every time, so the same service with the same peer becomes as many edges as there were connections,
`observed_count` stays at 1, and the snapshot fingerprint changes on every run — which breaks "snapshots
accumulate only at points of change" from [inventory design §7.2](../inventory/design.en.md). That collides
head-on with the goal of observing more often.

Appending `:0` also has a practical reason. Several places assume `dst_addr` is `ip:port` and read it with
`net.SplitHostPort` (netcap's app pinning, the query view, the `dst` column of `pqcota-declare-attribution`),
and keeping the shape keeps those working. And `:0` tells a reader plainly that this port carries no meaning.

**`port` is pinned to "the service port of that connection".** In the client role it is on the peer; in the
server role it is on us. That looks like two meanings, but it is one definition. Flattening the port to 0
altogether would collapse two services into one when a node has both 443 and 8443 open. The service is
exactly what has to be changed, so it must not be lost.

**App pinning uses the real value.** At capture time the peer's ephemeral port is known, so the `/proc/net/tcp`
lookup uses it and only the emitted edge carries `:0`. In the server role the app that gets pinned is the
**listening process**, and that is one side of the comparison from §6.1.

### 6.4 Two things that must be fixed along with it

**The SSH client/server decision inverts.** `sshPending.add` in [`capture_linux.go`](../discovery/collectors/network/capture_linux.go)
splits its lists on the premise that "a KEXINIT leaving my own IP is the client's". That premise held because
the local end was always the client. Once server-role edges are emitted, the server list lands on the client
side and `NegotiateSSHKex` computes the swap backwards, producing a **wrong negotiation result**. A grade
going quietly wrong is worse than an observation failing. The decision has to move to a port heuristic.

**`edgeKey` has to fold to a canonical direction.** When both ends are registered nodes and both observe, A
emits `src=A · dst=B · role=SERVER` and B emits `src=B · dst=A · role=CLIENT`. It is the same connection, so
the core must fold it into the canonical client-to-server direction and see one. Today `edgeKey` leaves the
direction as it is. Deduplication is step 5 of normalization, so that is the right place (§2.4⑤).

### 6.5 Off by default, switched on per node

**Server-role observation is not on by default.** What happens when it is switched on differs sharply with
the character of the node. Switching it on for a public web service that anyone can reach makes three things
worse at once.

| What | Why |
|---|---|
| **Registration requests flood** | An unregistered peer has to go to a registration request (§1.4). That wiring does not exist yet (§7), so there is no queue to flood today; connect §7 and thousands of internet clients become registration requests. It becomes a list no person can read |
| **The snapshot changes every time** | Client IPs keep changing, so the fingerprint never settles. The two-layer split from §7.2 stops helping |
| **Client IPs accumulate** | These are not like internal infrastructure IPs. A repo that removed access secrets from its types would start collecting something close to personal data at the other end |

So **a person picks which nodes get it.** The value is high on internal services with a fixed set of peers,
and that is the legacy internal network this repo aimed at in the first place. Public services simply do not
get it switched on.

**Switching it on prints a warning.** Stating that it is on is not enough — the three points above have to be
said on the spot. The text goes to stderr and carries three things.

- the **IPs of the clients** reaching this node **appear in the observation results**
- unregistered peers **pile up as registration requests**, and with many peers the review queue floods
- **we advise against switching it on** for a public service anyone can reach

**Not switching it on is recorded in the completeness note.** Without that it reads as "there is no inbound
traffic on that server". A gap is not an absence (§2.6).

### 6.6 Not settled

**Where the switch lives.** Whether it is a collector flag (something like `--observe-inbound`) or a playbook
variable is not settled. A flag makes it something the collector knows for itself; a playbook variable makes
it something orchestration decides. Either way, **the collector must not be told the scope master** (§1.6).
"Emit it only when the peer is unregistered" looks attractive, but that decision is the core's job.

**The road to aggregation is left open.** Instead of one edge per client IP, connections could be grouped by
`(service port, protocol, negotiated group)` and only counted. Writing "1,204 of the connections to my 443
were x25519" answers exactly what a migration asks, with no cardinality problem and no IPs stored. But
`ObservedEdge` is a model that requires a peer, so the contract would have to widen. Settle it after running
§6.5 on an internal network produces a reason to say "we need to see public services too". Building it now
would be speculative abstraction.

**An IP-range filter** (record private ranges only, say) overlaps heavily with §6.5 and adds one more concept.
Layer it on top if it becomes necessary.

### 6.7 What it touches on the reading side

| Where | What |
|---|---|
| `capture_linux.go` | `edgeFor` also emits the server side, and the SSH decision moves to a port basis |
| `edge.go`, `pqcota-netcap/main.go` | the `dst_addr` and `port` conventions, the merge key |
| `normalize/pipeline.go` | `edgeKey` folds to a canonical direction |
| the query view, `declare-attribution` | how to show `:0` in `dst_addr`, what to write in the `dst` column |
| the contract documents | the wording for `dst_addr` and `port` (the schema itself is unchanged) |
| tests | a server-role case in TD-NETWORK plus a regression for the SSH inversion |

---

## 7. Scope classification for the edge peer

What to do when an observation meets a node that is not in the scope master is already settled. Regulation
§1.4 and §2.5 say to send it to a **registration request**; [discovery design §4](../discovery/design.en.md)
writes it as "after-the-fact routing: an out-of-scope node observed during collection (a communication peer,
for instance) goes not to the collection set but to the registration/exclusion request queue (PROPOSE)"; and
[SD-5](../discovery/testcases.md) (Korean) sets "an observed unregistered node is not collected but routed to
the registration queue" as an acceptance criterion. It is the path where the tool does not decide on its own
what to manage, and hands it to a person instead.

### 7.1 The classifier exists; the wiring does not

[`scope.ClassifyObserved`](../pkg/kernel/scope/gate.go) classifies an unregistered node as a
`RegistrationRequest`. It has unit tests. **And nothing calls it.** Leave the tests aside and there is not
one caller.

`IngestReport.OffScope` on the ingest side counts something else — it counts when **the node that produced
the result** is unregistered, not when **the peer of an edge** is. So today it ends like this.

```
netcap observes a connection to 172.18.0.9:443
  → dst_node_id stays empty, dst_addr keeps "172.18.0.9:443"
  → the query view shows that address as it is
  → it never collects into a registration request. It is not even counted
```

**The information is visible but not acted on.** With hundreds of edges, a person has to spot the one
unfamiliar address by eye. The value of this path is that observation surfaces a node the operator failed to
register, and there is no code collecting what it surfaced.

### 7.2 Why it is missed now

This path is **the only automatic way to find a node the operator did not know about**. A person maintains
the list of observation targets, and a person leaves things out: an old batch job, a workload somebody stood
up, a DR route that is in no document. Those leave traces on the wire, so netcap meets them.

Two things raise that value. **Observing periodically** catches the nightly batch and the intermittent link
that one short window misses. **Emitting server-role edges** (§6) widens discovery from destinations to
sources. An unknown node is usually on the "something is connecting to my server" side, and that side is
entirely invisible right now.

So §6 and the cadence split supply this path with material, and this path is where that material gets used.
The three interlock.

### 7.3 Where it happens

**In the core.** The collector does not know the scope master (§1.6). While the ingest normalizes, it checks
an edge's `dst_addr` against the master, fills `dst_node_id` when it resolves and sends a registration request
when it does not. `ClassifyObserved` already makes that call, so it only has to be called.

**There is no automatic registration.** A person decides (MANUAL). Neither swallowing the discovery nor
registering it unilaterally — those are the two prohibitions §1.4 set.

### 7.4 Not settled

**Emit a count, or emit the addresses.** A count gets as far as "there are three unknown nodes" and no
further. Addresses let a person look straight at them, but then where they are kept follows.
`IngestReport` is the summary of one run and disappears when the run ends.

**Where they are kept.** A registration request is not a state of a node, so it must not go on the snapshot
timeline — the same reason app declarations were split off (§5.2 of this document). One approach is a separate
store, like the rejection history (`AppendRejection`), and then the names have to separate so that "what was
not accepted" and "what is asking to be registered" do not sit in one place.

**When the same address repeats.** Observe periodically and the same unregistered peer appears every round.
Registration requests must not pile up once per round, and something a person ruled "excluded" must not be
asked again. That needs a rule for persisting the decision, which is the same family as "decision persistence"
in regulation §3.6 — except that one is about plan decisions, so it cannot be used verbatim.

**How it appears on screen.** Mixing it into the query view puts the observed fact and the registration
request in one place. Emitting it separately looks right, but which command emits it is not settled.

### 7.5 Where it meshes with §6

Connect §7 and the situation §6.5 warned about becomes real. Switch server observation on for a public web
service and internet clients become registration requests directly, putting the list beyond a person's reach.
So the two designs have an order. **Server observation from §6 must not be on by default before §7 settles
"when the same address repeats" and "where they are kept".** §6 choosing to stay off by default and be
switched on per node is also the device that holds that order.

---

## 8. Separating the traffic-observation cadence from asset scanning

Assets and communication edges are observed on different cadences. Loaded libraries and provider chains
rarely change, so looking once on a regular schedule is enough; a communication edge, on the other hand, is
caught **only if it flowed during the capture window**. Discovery design §2.3 wrote "repeat across time
bands" as the remedy, but everything is bound into one [`discover.yml`](../discovery/ansible/discover.yml)
today, so there is no way to run it. Seeing more edges means re-running the `/proc` sweep and the JVM attach
along with it.

**Only one direction hurts.** Carrying eight seconds of netcap along with an asset scan is bearable; re-running
an expensive asset scan just to see more edges is not.

### 8.1 The existing playbook is left alone

`discover.yml` stays as it is, and a playbook that runs netcap in rounds is **added**. This matches "do not
delete, do not change — add" from [compatibility policy §3](compatibility.md) (Korean), and it leaves the demo
wiring and eight documents untouched. Whoever was using it keeps using it, and only whoever needs more edges
calls the new playbook as well.

**Nothing is installed on the node.** The controller calls as many times as there are rounds; it does not
place a timer on the node. The collector is still a CLI that runs and exits, and what repeats is the
invocation, not a process (collector deployment design §5). Putting a timer on the node would first require
settling result retrieval, retention, and privileges, so it is not covered here.

### 8.2 The values it takes

| Variable | What |
|---|---|
| `netcap_runs` | how many rounds to observe |
| `netcap_window` | the observation window of one round, in seconds |
| `netcap_interval` | the pause between rounds, in seconds |
| `netcap_budget` | the overall ceiling in seconds. Past it, the remaining rounds are not started and **how many were skipped is reported** |
| `netcap_cleanup` | whether to restore the node afterwards. Turn it off and **what is left behind is reported** |

**They live in `vars:` so they can be overridden per node.** That opens both paths at once: `-e` overrides
everything, and `host_vars` gives different values per node. Even when §6 arrives and "the receiving side
needs frequent looks too" becomes true, the cadence can still be tuned per node.

**One long window and several short ones buy different things.** A long window is better for collecting many
edges; several short ones are better for pinning apps, because an app is only caught if the socket is still
alive between the moment the handshake is seen and the moment `/proc` is read. That is why keeping rounds and
window as separate values genuinely matters.

### 8.3 Each round emits its own result file

Each round emits its own file, like `<node>-net-01.json`, so it does not collide with the `<node>-net.json`
that `discover.yml` emits. `pqcota-ingest` reads the directory whole and groups by node, so when one node has
several results, normalization **merges them into one snapshot**.

**A round that found zero edges is retrieved too.** netcap emits a `CollectionResult` even with no edges, and
the completeness note is inside it. Deciding "nothing was seen, so write no file" makes "no traffic in that
window" and "could not observe" the same shape (§2.6).

**Ingesting per round or all at once decides the resolution of the observation evidence.** `pqcota_observations`
holds **one row per node per ingest**. Ingest six rounds at once and "we looked six times" is left as one
line; ingest per round and it is six. That is not for the playbook to decide but for whoever calls it, so the
fact is recorded here.

### 8.4 Merging repeated observations

Raise the cadence and the same edge is seen many times. **Repeated observations of the same target have to be
merged into one.** Today they are not.

**What happens now.** Identity resolution in [`pipeline.go`](../pkg/discovery/normalize/pipeline.go) only
**drops** duplicate edges.

```go
if seenEdge[k] { continue }   // drops the later one — does not merge
```

Two things go wrong as a result. `observed_count` stays at the first round's value: netcap counts within its
own window (`ex.ObservedCount++` in `main.go`) but nothing accumulates across rounds. And `first_seen` and
`last_seen` are **filled by nobody**. The fields are in the contract and `sign.Canonical` even reads them, but
nothing writes them, so they are always empty. "When did I last see this edge" cannot be answered from the
edge itself.

**The merge key differs in three places.** This has to be reconciled before the merge rules can be settled.

| Where | Key |
|---|---|
| inside netcap (`main.go`) | `src · dst · port · protocol` |
| the dedup in normalization (`pipeline.go`) | `src · dst · protocol · negotiated_group` |
| the snapshot fingerprint (`fingerprint.go`) | `src · dst_node_id · dst_addr · port · protocol · role · detection_method` |

**The fingerprint is the right one.** Take the group out of the key and stamp it separately as a value
(`E|key|group|cipher`), and a changed group is caught as **a change to the same edge**. That is the wanted
behaviour: when the migration takes and `x25519` becomes `X25519MLKEM768`, a new snapshot should appear and
`-diff` should catch it as `changed`.

**An edge's identity is `(src, dst, port, protocol, role)`, and the negotiated group is a value.**
Normalization is brought in line with the fingerprint.

**The unknown does not overwrite the known.** This is the most important rule for repeated observation. A
round failing to learn the group must not erase a value already learnt.

| This round | Already known | Result |
|---|---|---|
| empty (unknown) | present | **keep what is known.** The unknown does not overwrite the known |
| a real value | absent | fill it |
| a real value | a different real value | **that is a change.** Move to the new value and a new snapshot appears |

netcap already does this within its own window (`main.go` promotes an observation with no group to one with a
group), because TLS carries the negotiated group only in the ServerHello, which splits two observations of one
connection. **The rule has to hold across rounds too.** Today the group is in the normalization key, so it
behaves the opposite way: learn the group in round 1, miss the ServerHello in round 2, and it becomes **two
different edges**, drawing the same link side by side as 🟢 and ⚪. The ⚪ is not a new fact but something that
round did not see, so this amounts to updating it to "absent".

**Keep it, but record its age.** Holding on to a known value blurs when that value was seen: a link last
observed three months ago can still show as 🟢 on screen. This is where `last_seen` earns its place. Keep the
value and record when it was last confirmed, and "we do not erase what we know" stands together with "we do
not pass off something old as current". It is the same shape as not writing "absent" for what was not seen
while still recording that it was not seen (§2.6).

**What gets merged.** Edges with the same identity add their `observed_count`, take the earliest `first_seen`
and the latest `last_seen`. The group and the cipher update per the table above.

**One more defect to fix along with it.** The normalization key has **no port**.

```go
dst := e.GetDstNodeId(); if dst == "" { dst = e.GetDstAddr() }
return src + "|" + dst + "|" + protocol + "|" + group   // no port
```

While `dst_addr` is used, that string happens to contain the port and separates them by accident; but once the
peer resolves to a node and `dst_node_id` is filled, **the port disappears from the key**. Then 443 and 8443
on the same node collapse into one edge whenever the group matches. What disappears is which service has to be
changed, which is the same ground on which §6.3 pinned `port` to "the service port of that connection".
Aligning identity with the fingerprint resolves both.

**Storage does not grow.** [Inventory design §7.3](../inventory/design.en.md) already took `observed_count`,
`first_seen`, and `last_seen` out of the fingerprint, so an observation that differs only in count and
timestamps makes no new snapshot. That is why this design does not collide with the two-layer split in §7.2.

**Neither the originals nor the history are removed.** Merging is "seeing observations that point at the same
target as one", not erasing the past. The append-only history stays, and so do the per-round result files
(§1.2). The rows in `pqcota_observations` are not deleted either: they are **the evidence that we looked every
time**, and deleting them makes them indistinguishable from a period with no observation at all (the same
reason as the truncation rule in §7.4, which is §2.6 moved onto the time axis).

### 8.5 Where the merge meshes with signing

This is the place in the design to be most careful about. [`sign.Canonical`](../pkg/kernel/sign/sign.go) puts
an edge's `observed_count`, `first_seen`, and `last_seen` **inside the signature scope**. So if the core
rewrites those values on a stored edge, **what is stored differs from what the collector signed**.

This repo has met the same problem once already. §5.2 caught the ingest at the same spot when it tried to fill
an observed edge's empty `app_key`, and the answer was **to split the storage and merge in the view**. There
are three options.

**㉠ Merge in the view.** Leave the stored per-round edges as they are and, on query, group them by the
canonical key and compute the count and the timestamps for display. Being the same answer as §5.2, it stays
in this repo's grain, and both the signature and the immutability of the originals hold. The cost is that the
query computes every time and the same edge remains in the snapshot once per round.

**㉡ Merge on write.** The query gets simple and the snapshot gets smaller. The cost is touching the signature
scope: a merged edge holds values no collector ever signed.

**㉢ Take the volatile fields out of the signature scope.** Excluding `observed_count`, `first_seen`, and
`last_seen` from `Canonical` makes ㉡ work. But [compatibility policy §2](compatibility.md) (Korean) records
that "changing the signature scope invalidates the past". Every signature already issued would have to be
recomputed, so a whole release would go to that work.

**㉠ is recommended.** It matches the earlier precedent, it holds the signature and the immutability of the
originals together, and it is easy to reverse. If edges ever grow numerous enough for the query cost to
matter, ㉡ and ㉢ get looked at again then, with evidence.

### 8.6 The view shows a round ratio

What `observed_count` counts is **handshakes**. netcap increments it every time it sees the same edge within
its window, so a link connected fifty times in eight seconds is 50 and one connected once is 1. Added across
rounds, it becomes "how often does this communicate".

But what a migration asks is a little different. **"Is this link always alive, or does it happen once in a
while?"** decides the order of the work. A link seen in four of six rounds and one seen only once are handled
differently.

**So the view shows a round ratio.** It emits the denominator alongside, as `4/6`. A number without a
denominator is not a shape this repo uses. Showing only "observed 4 times" leaves it unknown whether that is
four of six rounds or four of a hundred, and that is the same reason exclusions are counted and reported, and
"verification failed" is kept apart from "never verified".

**The contract's meaning does not change.** `observed_count` remains observation frequency as the proto states,
and an input to §3.5 confidence. The round ratio does not replace it; it is **a value the view derives
separately**. Since §8.5 settled on splitting the storage and merging in the view, per-round observations stay
in the snapshot as they are, and the view only counts how many collection results that edge appeared in. Where
the handshake total is wanted, it is emitted alongside on the detail view.

### 8.7 netcap fills the timestamps

`first_seen` and `last_seen` are **observed facts, not derivations**. On the §1.2 divide between the collector
emitting what it saw and the core deriving from it, these are on the near side. So netcap fills them: the
moment it first saw that edge in that window and the moment it last saw it, equal to each other when it was
seen once.

Deriving them from `Envelope.collected_at` is possible but not chosen. One round has one timestamp, so **the
longer the window, the coarser the approximation.** At an eight-second window there is no difference; at a
300-second window it flattens five minutes. Making long observation windows possible is the point of §8, so
that direction works against it.

**Signing and the fingerprint are not affected.** [Compatibility policy §2](compatibility.md) (Korean) records
that "filling a value is different — `Canonical` reads values, not the set of fields, so results from the past
still canonicalize to the same bytes even once a previously empty field starts being filled". And
[inventory design §7.3](../inventory/design.en.md) took both fields out of the fingerprint, so timestamps
differing per round make no new snapshot.

**On merge, `first_seen` takes the earliest and `last_seen` the latest.** That is where "keep what is known,
but record its age" from §8.4 holds.

> **Implementation note**: the capture loop does not record a timestamp per packet today. The timestamp has to
> be carried from where the frame is read, and for testability it uses the same approach as the `now` variable
> in `edge.go`.

### 8.8 Ordering against §6 and §7

This design is independent of both and can go first. But once §6 arrives, the answer to "which nodes should run
often" changes, so keeping the per-node variables of §8.2 open is enough to keep it from being shaken later.
