# A first PQC migration, step by step

For someone who has just been asked to run, or to follow, a post-quantum cryptography (PQC) migration. It follows one small example through the five steps of a migration and says, at each step, **what pqcota gives you and what a person has to decide.** It is an introduction to the order of work, not a project plan. For the overview of pqcota, see the [README](../README.md); for operational questions, the [FAQ](faq.md); for turning results into a report, the [reporting guide](reporting-guide.md).

## Why this is a project and not a patch

NIST states that no one knows exactly when a quantum computer able to break today's public-key cryptography will exist, with estimates from a few years to a few decades. It also says that moving systems to new standards can take 10 to 20 years, and that an adversary can record encrypted data now to decrypt it later; it encourages organizations to begin their transition ([NIST](https://www.nist.gov/cybersecurity-and-privacy/what-post-quantum-cryptography)). On 13 August 2024 NIST published the first post-quantum standards: FIPS 203 (ML-KEM, for establishing keys), FIPS 204 (ML-DSA) and FIPS 205 (SLH-DSA), the last two for digital signatures ([NIST announcement](https://csrc.nist.gov/news/2024/postquantum-cryptography-fips-approved)). During the transition many systems use a *hybrid* key exchange that combines a classical and a post-quantum algorithm, aiming to keep the shared secret protected while at least one of them holds ([IETF RFC 9954](https://datatracker.ietf.org/doc/html/rfc9954)).

None of that tells you what to do first, because the answer depends on what your systems actually use. Deadlines, and which data or systems come first, depend on your regulator, your industry and your own risk decisions. This guide does not set them; take them from your own authorities.

What stays the same everywhere is the order of the work:

1. Decide what is in scope, and how you will observe it.
2. Find out what is really in use, and what you could not see.
3. Decide what to change, and in what order.
4. Approve the change, make it, and keep a way back.
5. Observe again, and write down what that does and does not prove.

pqcota helps with the first two steps and the last two. **It deliberately leaves step 3 to you.**

## The example

A small payment service, as in pqcota's own [demo](../demo/README.md):

- `web-gw`, a web gateway running OpenSSL 3.0.13. It connects to the next two systems.
- `pay-app`, a Java application on a system with OpenSSL 3.5.5. It also uses a security provider (`BC`) that the application registers while it runs.
- `pay-db`, a database server running OpenSSL 1.1.1f.

The output quoted below is from a run of the demo on 2026-09-30 and is abbreviated. The demo is a lab setup, so where it differs from what you would do in a real environment, this guide says so.

---

## Step 1. Decide what is in scope, and how you will observe it

**Decide (a person):**
- Which systems to cover, and why those first. This is a business and risk decision, not a technical one.
- Which found assets are noise you will not manage, such as a distribution's default libraries.
- How often you will look again. A single observation shows the moment it was taken.

**pqcota gives you:**
- A way to *register* systems, so results from anything else are refused and counted rather than silently accepted.
- Asset rules that leave out what you do not want to manage, and the **number** it left out. Excluded is not absent.
- A history that records each observation, so "last looked" and "last changed" are both known.

**In the example:** in a real environment you would register `web-gw`, `pay-app` and `pay-db` and add an asset rule that leaves out the runtimes belonging to other packages. The demo does not do the first part: it is not given a registration list, and its load output says `scope gate: skipped (no CMDB given)`. It does apply asset rules, leaving out a Python runtime and the SSH daemon, and the load summary reports how many findings that excluded, so a report does not read as if they never existed.

> Write down your scope now, in numbers: how many systems you meant to cover. Later you will need it to say how much you did *not* see. The [reporting guide](reporting-guide.md#1-scope-and-period) explains the units.

## Step 2. Find out what is really in use, and what you could not see

**pqcota gives you:** per system, the cryptography libraries and Java security providers it found, and per connection, the algorithm the two ends agreed on while it watched, each with a grade.

```
pay-app
  • JCA provider chain: SUN,SunRsaSign,…,BC   [confirmed]
      (this BC is in no configuration file; the application registered it while running)
  • OpenSSL libcrypto 3.5.5                    [confirmed]
pay-db
  • OpenSSL libcrypto 1.1.1f                   [confirmed]

🟢 web-gw → pay-app   TLS  X25519MLKEM768
🟢 web-gw → pay-app   SSH  sntrup761x25519-sha512@openssh.com
🔴 web-gw → pay-db    TLS  x25519
🔴 web-gw → pay-db    SSH  curve25519-sha256
grade totals: 🟢 2 · 🔴 2 · ⚪ 0
```

**Read it like this:**
- `X25519MLKEM768` names a hybrid group: X25519 ECDH combined with ML-KEM-768 ([IETF RFC 10024, section 7.1](https://www.rfc-editor.org/rfc/rfc10024.html#section-7.1)). 🟢 means a post-quantum or hybrid algorithm was negotiated; 🔴 means a classical one was. 🔴 is **not** a verdict of "vulnerable", only the fact of what was negotiated.
- The 🔴 connections go to `pay-db`, and the demo explains the two differently. The TLS connection is classical because OpenSSL 1.1.1 has no post-quantum group. The SSH connection is classical because the legacy system there runs OpenSSH 8.2, which offers no post-quantum key exchange. A negotiated algorithm needs both ends to support it, and a library being loaded on a system does not show that a particular connection used it.
- "confirmed" is the strength of evidence, given by how a fact was collected.

**What you could not see matters as much.** In the same run, three of the four connections could not be attributed to an application: they closed before the lookup. The inventory prints that as a gap and says it does not mean there is no application. Three other cases are recorded differently. A collector that ran but could not see something writes a gap into its own result. When you use a registration list, results from systems that are not on it are refused, and the load summary counts them as results (not as systems). And a collector that never ran leaves no result and no gap at all, so you find those only by comparing your run log with your list of systems. See [What was not seen](reporting-guide.md#3-what-was-not-seen).

**Decide (a person):** whether this coverage is enough to act on, and what to do about the gaps.

## Step 3. Decide what to change, and in what order

**pqcota gives you:** the facts for the decision, and a description of what it can generate for each situation. It does not rank, score or recommend. There is no risk rating by design, because once the tool decides for you, "🔴 is an observation, not a verdict" stops being true.

**What it can generate depends on the runtime and version.** The table below applies the [supported systems](../README.md#supported-systems) to the example systems as an illustration of the choices. **It is not the plan the demo runs.** The default demo generates a provider-injection action for `pay-db` and places an *empty* test module there, only to show the delivery path; that module adds no cryptography. The optional real-provider stage (`DEMO_REAL_PROVIDER=1`) picks a node observed with OpenSSL 3.0 to 3.4, which is `web-gw` in the default topology, and installs a real provider.

| Situation in the example | What pqcota can generate |
|---|---|
| OpenSSL 3.5+ (the OpenSSL on `pay-app`) | A configuration fragment only |
| OpenSSL 3.0 to 3.4 (`web-gw`) | A configuration fragment, plus staging a provider module **that you supply** |
| OpenSSL 1.1.1 and older (`pay-db`) | Nothing. Configuration cannot add post-quantum algorithms there; replacing the library is a manual step and the output says so |
| SSH (`pay-db`) | Not covered by these actions. The demo's explanation is that the OpenSSH on that system offers no post-quantum key exchange |
| Java providers | Depends on the JDK: settings only on recent JDKs, a provider library plus a registration on others, nothing on end-of-life JDKs |

**Decide (a person):**
- Which actions, on which systems, in which order, and by when. Take deadlines and data priorities from your own authorities, not from this page.
- Which provider and where to get its files, if you use one. You supply the module and its expected checksum. The generated playbook verifies the placed file against that checksum **only when you pass it** (as a `pqcota_module_sha256` variable when you run the playbook); without it that check is skipped.
- What to do about `pay-db`. For the TLS connection to it to turn 🟢, its end has to support a post-quantum group, and for OpenSSL 1.1.1 that means replacing the library. The SSH connection depends on the SSH software there and is a separate decision.

You record the decisions in a **plan**, a file that people write. pqcota does not create plans and only reads them; there are [sample plans](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/examples/provisioning/plans/README.md) for each situation.

## Step 4. Approve the change, make it, and keep a way back

**pqcota gives you:**
- A check that the plan is finalized and carries an approval signature that matches an approver's registered public key. By default it refuses otherwise.
- Generated files, as standard Ansible, at the level of automation you choose. For a provider-injection action: **L1** places the module on the target; **L2** (the default) also places the configuration; **L3** also runs the activation steps your plan defines, such as a restart. The level sets how far the automation goes; what is actually placed depends on the kind of action. A configuration-only action has no module to place, and a manual step places nothing.
- A matching removal file for each generated change, and, when the generator is given a database (`--dsn`), a record of the state observed before.

**Decide (a person):**
- Who approves, and who holds the approval keys. pqcota checks a signature; your organization's review process is outside it.
- Whether to use the options that relax checks. `--allow-unverified-approvals` skips signature verification; `--allow-incomplete` lets output with blanks end with exit code 0. If either is used, the report should say so.
- When and where to apply the files. **pqcota does not apply them.** You run them with your own tools, and your tool's log is the record of what was applied and when.

**Keep these straight:**
- The generator's exit code is not a certificate. Read its messages, the options used and whether the record was saved ([details](reporting-guide.md#5-approval)).
- A record marked `STAGED` means the generator recorded the change, not that anything was placed on a node.
- The removal file deletes what the plan staged. It is not a restore of an earlier deployment ([FAQ](faq.md#changes)).

## Step 5. Observe again, and write down what it proves

**pqcota gives you:** a new observation and a comparison against the old one.

**What that proves depends on the layer:**
- **Java:** the provider chain can be read directly, so a newly registered provider shows up in it.
- **OpenSSL:** pqcota observes the library and its version, not which providers are loaded. In the demo's optional real-provider stage (`DEMO_REAL_PROVIDER=1`, not the default run), applying a real post-quantum provider to a test node raised the number of ML-KEM entries in `openssl list -kem-algorithms` from 0 to 14, and the undo returned it to 0. The re-observation in between showed **no change**: the provider layer is not observed, and the connection did not change either, because a connection changes grade only when both ends support the algorithm, and the demo notes that the peer in that topology is OpenSSL 1.1.1.
- **Connections:** a 🔴 connection turns 🟢 only when both ends support a post-quantum algorithm and the connection is seen again.

**Decide (a person):** what you are entitled to say. "The provider was staged and the node lists the algorithms" is a statement about capability. "Connections now negotiate a post-quantum algorithm" needs an observation of those connections. Write the first when that is what you have. The [reporting guide](reporting-guide.md#7-confirming-a-change-and-its-limits) has the wording to avoid.

Then the cycle starts again: scope, observe, decide, approve, observe.

---

## Who decides what

| | pqcota | A person |
|---|---|---|
| What is in scope | Applies the registration list and asset rules you give it | Chooses the list and the rules |
| What is in use | Observes and grades | Interprets, and decides if the coverage is enough |
| What was not seen | Records many gaps and counts | Reads them, and chases the ones nothing records |
| What to change, and when | Describes what it can generate | Decides, using their own deadlines and priorities |
| Approval | Verifies a signature | Approves, and keeps the keys |
| Applying the change | Generates files | Applies them and keeps the log |
| Did it work | Compares observations | Decides what the comparison proves |

## Where to go next

- The [FAQ](faq.md) for what runs on your systems, what data it collects and what privileges it needs.
- The [reporting guide](reporting-guide.md) for turning the results into a report.
- The [demo](../demo/README.md) to run this whole example on containers (engineers; needs Docker).
- The [build guide](build.md) to install or build it.

## Sources for the general PQC statements

- NIST, [What is post-quantum cryptography?](https://www.nist.gov/cybersecurity-and-privacy/what-post-quantum-cryptography): uncertain timing, 10 to 20 years to migrate, harvest now and decrypt later, the encouragement to begin.
- NIST, [FIPS 203, 204 and 205 announcement](https://csrc.nist.gov/news/2024/postquantum-cryptography-fips-approved), 13 August 2024.
- IETF, [RFC 9954, Hybrid Key Exchange in TLS 1.3](https://datatracker.ietf.org/doc/html/rfc9954).
- IETF, [RFC 10024, Post-Quantum Traditional (PQ/T) Hybrid Key Agreement Mechanisms for TLS 1.3](https://www.rfc-editor.org/rfc/rfc10024.html), section 7.1, for what `X25519MLKEM768` combines.
