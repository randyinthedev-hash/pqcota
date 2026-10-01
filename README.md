English · [한국어](README.ko.md)

# pqcota

[![ci](https://github.com/randyinthedev-hash/pqcota/actions/workflows/ci.yml/badge.svg)](https://github.com/randyinthedev-hash/pqcota/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

**See which cryptography your systems use, and prepare the move to post-quantum cryptography with changes you can review, apply and remove.**

pqcota is open-source software (Apache-2.0) for the people who run a post-quantum cryptography (PQC) migration inside an organization, and for the people who receive their reports. You do not need to be a developer or a cryptographer to follow what it does and what it hands you.

> **In short:** pqcota observes the cryptographic assets and connection results of the systems it can reach, keeps a history of how they change, and, once *you* have decided what to change, generates reviewable files that carry out that change and remove it again.

**[Watch the demo](https://www.youtube.com/watch?v=4E26AJ6WCWw)** (3 min 38 s; [Korean version](https://www.youtube.com/watch?v=R0QD7Fv0KgE)) · **[Read the online documentation](https://randyinthedev-hash.github.io/pqcota/)** (each page shows the release it matches; the pages are also in this repository's [`docs/`](docs/)) · **[Run it yourself](#try-it)**

---

## Who this page is for

| You are… | Start here |
|---|---|
| **A manager or business owner** who receives migration reports | [What you receive](#what-you-receive), [How to read a result](#how-to-read-a-result), [What pqcota does not do](#what-pqcota-does-not-do), and the [reporting guide](docs/reporting-guide.md) |
| **The person running the migration** | [The three stages](#the-three-stages), [Supported systems](#supported-systems), [Data and operations](#data-and-operations), [Try it](#try-it), and the [reporting guide](docs/reporting-guide.md) |
| **New to PQC** | [Background](#background), the step-by-step [first migration](docs/pqc-migration-primer.md), then the [glossary](#glossary) |
| **An engineer or security architect** | The [developer documentation](docs/developers.md), the [build guide](docs/build.md) and [CONTRIBUTING](CONTRIBUTING.md) |

## What you receive

| Question you are asked | What pqcota gives you |
|---|---|
| "Which cryptography do we use, and where?" | For each observed system: the cryptography libraries loaded, the Java security providers registered, and, for network connections it watched, the algorithm the two ends agreed on |
| "How many of our connections are post-quantum?" | A count of observed connections graded post-quantum or hybrid (🟢), classical (🔴), or not graded (⚪) |
| "What changed since last time?" | A comparison of two observations: what was added, removed or replaced |
| "What did you *not* see?" | An explicit "not observed" record for anything a collector could not reach. A gap is not reported as "nothing there" |
| "What will you change, and how do we undo it?" | For each planned change that can be delivered through configuration: the files that make it, the files that remove it, and, when you give the generator the inventory database (`--dsn`), a record of the system's state before. Changes that cannot be delivered that way are listed as manual steps |
| "Who approved this?" | Generation requires a plan that carries an approval signature. By default the signature is checked against the approver's registered public key |

The figures describe the systems pqcota was able to observe, in the environments it supports, at the moment it looked. They are evidence for your migration report, not a certification.

## The three stages

| Stage | In plain words | Produces |
|---|---|---|
| **① Discovery** | Small programs visit a system, observe it, and are removed when the run completes (what a failed run can leave behind is under [Data and operations](#data-and-operations)). They read which cryptography libraries are loaded, which Java security providers are registered, and which algorithms were negotiated in connections they watched. | One observation per system |
| **② Inventory** | Collects the observations in one place, ties each to its system and application, and keeps every change as a new record without editing old ones. | A central history you can query and compare |
| **③ Provisioning** | Takes a plan that people wrote and approved and generates the files to carry it out: configuration changes, the modules to place, and the matching removal. | Standard Ansible files and, with `--dsn`, a "before" record |

The stages can be used separately. To look at one system you need one small program, no database and no central server.

## How to read a result

An abbreviated demo result (assets shortened; every connection of the demo is shown):

```
──────── ① discovered assets (per node) ────────
  pay-app
    • JCA provider chain: SUN,SunRsaSign,…,BC   [confirmed]
        ↑ this BC is not in java.security. The application registered
          it while running, so reading that file would miss it.
    • OpenSSL libcrypto 3.5.5                    [confirmed]
  pay-db
    • OpenSSL libcrypto 1.1.1f                   [confirmed]

──────── ② observed connections + quantum-resistance grade ────────
  🟢 web-gw → pay-app   TLS  X25519MLKEM768
  🟢 web-gw → pay-app   SSH  sntrup761x25519-sha512@openssh.com
  🔴 web-gw → pay-db    TLS  x25519
  🔴 web-gw → pay-db    SSH  curve25519-sha256

  grade totals: 🟢 2 · 🔴 2 · ⚪ 0
```

- **🟢** a post-quantum or hybrid algorithm was negotiated. **🔴** a classical algorithm was negotiated. **⚪** the grade could not be determined.
- **🔴 does not mean "vulnerable" or "non-compliant".** It means this connection used a classical algorithm while pqcota watched it. Whether and when to change it is your decision. pqcota does not score risk.
- **"confirmed"** is the strength of evidence, assigned by how the fact was collected. Facts seen in a running system, in source, or by tracing are "confirmed". Facts taken from a supplied bill of materials (CBOM) are graded lower ("inferred, high"), because nobody looked at the running system.
- **Confirming a change by re-observing has limits.** For OpenSSL, pqcota observes the library and its version, not which providers are loaded, so adding a provider does not show up in the inventory by itself. Java provider chains can be observed directly. A connection changes grade only when both ends support the new algorithm.
- Grades come from what was observed. A library that is loaded is not proof that it is used for every connection, and a connection is only seen if it happened while the observer was watching.
- The same data can be drawn as a map of systems and connections coloured by grade.

## What pqcota does not do

- **It does not decide what to migrate or when.** People write and approve the plan.
- **It does not judge risk or compliance, and it does not score.** There is no risk rating and no pass or fail.
- **It does not apply changes.** It generates standard Ansible files that you run with your own tools. There is no remote-execution engine.
- **It does not scan source code.** If your build already produces a cryptographic bill of materials (CBOM, CycloneDX), pqcota can receive it into the same inventory.
- **It is not a compliance product.** It supplies evidence and certifies nothing.
- **It does not manage your review process.** It checks a cryptographic approval signature on the plan. Who reviews, in what order, and who holds the approver keys is your organization's process. Reconciling a declared list of systems (a CMDB) against what was observed is not part of this software either.

## Supported systems

| | Observe | Generate a change |
|---|---|---|
| **OpenSSL on Linux** | ✅ | ✅ OpenSSL 3.5+: a configuration fragment only. 3.0–3.4: staging a provider module that you supply, plus a configuration fragment. 1.1.1 and older: nothing is generated; replacing the library is a manual step and the output says so |
| **Java (JCA)** | ✅ Linux, and Windows machines that have a JDK | ✅ on **Linux** targets. JDK 24+: a `java.security` fragment only, and it keeps a classical group alongside. JDK 8+: staging a provider library plus a registration fragment. Older, end-of-life JDKs: nothing is generated; a JDK upgrade is required |
| **Windows (CNG)** | ✅ | ❌ not yet |
| **Network connections** | ✅ Linux | not applicable |

pqcota is **pre-1.0** (current release: v0.10.1). Observation and generation work end to end on Linux and are demonstrated on test systems. Windows change generation is planned, not delivered; see the [release notes](RELEASE_NOTES.md).

## Data and operations

A security or operations team will have more questions than this page answers. See the [FAQ](docs/faq.md).

**What it collects.** For each system: the cryptography libraries loaded and their versions, the Java security providers registered, and, for connections it watched, the peer address and port, the protocol, the negotiated key-exchange group and the cipher. This is information about your infrastructure and should be handled as such. It does not decrypt traffic and does not write raw packet captures.

**Where results go.** To files and to the database you point it at. The pqcota programs contain no built-in upload or telemetry path. Downloads happen when you install it or run the demo (source code, container images), and Ansible reaches your systems over SSH.

**What runs on the observed systems.** No resident agent and no service. The supplied playbook copies the observation programs to a temporary directory, runs them, collects the results and deletes that directory when the run completes. If a run fails or is interrupted, that directory can be left behind and you should check for it. The Java observation also writes a small file under the target's `/tmp`, outside that directory, which is not deleted. Some observations have prerequisites: capturing connections needs a network-capture privilege, and reaching Java runtimes needs a JDK on some paths.

**Access credentials.** The inventory has no field for logins or keys. The Ansible target list that does contain them is written for the run, readable only by its owner, and is not stored in the inventory. Treat any file you write yourself as sensitive.

**Undoing a change.** For each generated change there is a matching removal file. Changes add files instead of overwriting existing ones, so removal deletes what pqcota added; where a plan defines activation steps (a restart, for example) the removal runs the plan's deactivation steps first. This is not a restore of a previous deployment: if the same paths were deployed to twice, removal deletes the files and does not bring back the earlier version. A change that cannot be delivered through configuration is marked as a manual step, and so is its undoing.

**Approval.** Generation stops unless the plan is finalized and approved. By default it verifies the approval signatures against the approvers' registered public keys and refuses when it cannot. A command-line option exists to continue without verifying; using it prints a warning that it was your choice.

**History.** The inventory records changes by adding new records, and its normal commands never edit an old one. One command, `pqcota-prune`, cuts old history you no longer want to keep, and it records that it did. This describes the tool's behavior. It is not tamper-proofing against someone with administrator access to the database.

## Try it

- **Watch (3 min 38 s):** the [demo video](https://www.youtube.com/watch?v=4E26AJ6WCWw) ([Korean version](https://www.youtube.com/watch?v=R0QD7Fv0KgE)) goes from observation to applying and removing a generated change on test systems. It is silent, with captions, and recorded from release v0.10.1.
- **Run it:** with Docker, [demo/](demo/README.md) runs the whole flow on containers. An optional stage (`DEMO_REAL_PROVIDER=1`) builds a real post-quantum provider, applies the generated files to a test node running OpenSSL 3.0.13 and undoes them. In a run on 2026-09-30 the number of ML-KEM entries in that node's `openssl list -kem-algorithms` went from 0 to 14 after applying and back to 0 after the undo. **That shows the algorithms became available on that node. It does not show that any connection used them:** the demo's own output notes that re-observing left the inventory unchanged, because pqcota does not yet observe OpenSSL's provider layer and a connection needs both ends to support the algorithm.
- **Your own systems:** ask your engineers to start from the [build guide](docs/build.md). Observation needs no resident software on the systems being observed, but some paths need a privilege or a JDK; see [Data and operations](#data-and-operations).

## Background

**Why the topic exists.** NIST states that no one knows exactly when a quantum computer able to break today's public-key cryptography will exist, with estimates from a few years to a few decades, that moving systems to new standards can take 10 to 20 years, and that adversaries can record encrypted data now to decrypt it later. It encourages organizations to begin their transition ([NIST](https://www.nist.gov/cybersecurity-and-privacy/what-post-quantum-cryptography)). Data that must stay confidential for many years is the first concern.

**What the new standards are.** On 13 August 2024 NIST published FIPS 203 (ML-KEM, for establishing keys), FIPS 204 (ML-DSA) and FIPS 205 (SLH-DSA), both for digital signatures ([NIST announcement](https://csrc.nist.gov/news/2024/postquantum-cryptography-fips-approved)).

**Hybrid.** A hybrid key exchange combines a classical and a post-quantum algorithm, aiming to keep the shared secret protected while at least one of them holds. It does not repair flaws in authentication or in an implementation ([IETF RFC 9954](https://datatracker.ietf.org/doc/html/rfc9954)).

**A program, not a patch.** PQC migration is not a single update. An organization typically works through four questions: where cryptography is used and which algorithms; which of it to change first (a human decision); how to change it without disrupting operations; and how to confirm afterwards, and keep confirming. pqcota helps with the first, third and fourth, and leaves the second to you.

## Glossary

| Term | Meaning |
|---|---|
| **PQC** | Post-quantum cryptography: algorithms designed to withstand attacks by both conventional and quantum computers |
| **Key exchange** | How two systems agree on a shared secret at the start of a connection. This is the part exposed to "record now, decrypt later" |
| **Hybrid** | A classical and a post-quantum algorithm used together |
| **CBOM** | Cryptographic bill of materials: a standard list (CycloneDX) of the cryptography a piece of software uses |
| **Provider** | A plug-in library that supplies algorithms to OpenSSL or Java. Adding one can add post-quantum algorithms without replacing the product |
| **Node** | One machine, virtual machine or container that is observed |
| **Snapshot** | The recorded state of one node at one time. A new snapshot is stored only when the state changes; repeat observations are counted against the existing one |
| **Ansible** | A common tool for applying configuration to many machines. pqcota's generated files are standard Ansible files |
| **Approval signature** | A cryptographic signature on the plan by a named approver, which pqcota checks before generating changes |

## License and more

Apache-2.0, see [LICENSE](LICENSE); dependency licensing in [License notes](docs/licensing.md) and [THIRD-PARTY-NOTICES](THIRD-PARTY-NOTICES.md). [Release notes](RELEASE_NOTES.md) · [Developer documentation](docs/developers.md) · [First migration, step by step](docs/pqc-migration-primer.md) · [FAQ](docs/faq.md) · [Reporting guide](docs/reporting-guide.md) · [Build guide](docs/build.md) · [Contributing](CONTRIBUTING.md) · [Compatibility policy](docs/compatibility.md) · [Platform structure diagram](https://randyinthedev-hash.github.io/pqcota/architectures/platform-structure.html). pqcota is one of five repositories, listed in CONTRIBUTING. The name is *PQC* plus *orchestra*: pqcota plays one part; you are the conductor.
