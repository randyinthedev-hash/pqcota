# Frequently asked questions

The questions a security team, an operations team or a manager tends to ask before allowing pqcota near real systems. Each answer says what pqcota does, and where it stops. For the overview, start at the [README](../README.md); for how to use results in a report, see the [reporting guide](reporting-guide.md); for building and running it, see the [build guide](build.md).

- [Before you start](#before-you-start)
- [Effect on running systems](#effect-on-running-systems)
- [Data](#data)
- [Reading results](#reading-results)
- [Changes](#changes)
- [The program around it](#the-program-around-it)

---

## Before you start

**Do I have to install anything on the systems I want to observe?**
There is no resident agent and no service. The supplied Ansible playbook copies the observation programs to a temporary directory on each system, runs them, brings the results back and deletes that directory. Some observations have prerequisites, and the Java observation also writes a file outside that directory; see the next questions. If you observe a single system by hand, you run one program on it and nothing else.

**What privileges do the observation programs need?**
Less privilege means a narrower view. How that shows up depends on the program and the kind of failure: a gap record, a count in the program's statistics, or only a message in the run log. The [reporting guide](reporting-guide.md#3-what-was-not-seen) lists which is which.

| Program | Privilege | If it is missing |
|---|---|---|
| `pqcota-nodescan` (OpenSSL) | Its own processes work as-is. Seeing other users' processes needs root or `CAP_SYS_PTRACE` | It sees fewer processes, and the ones it was denied are counted in its statistics. If it cannot read the process list at all, that is noted in the result |
| `pqcota-netcap` (connections) | `CAP_NET_RAW` (or root) | It does not capture, prints how to grant the capability, and writes a gap record |
| `pqcota-jvmscan` (Java) | The same user as the target JVM, or root | If the JVM blocks attach, it falls back to reading the static configuration, and runtime-registered providers stay unseen |
| `pqcota-cngscan` (Windows) | No special privilege | |

On Windows, Java processes run by other users are visible only when the machine is reached with an administrator account.

**Which systems does it support?**
See [Supported systems](../README.md#supported-systems). In short: OpenSSL and Java on Linux, Windows CNG for observation, and generated changes for Linux targets only. Linux targets need kernel 3.2 or newer, which is Go's own floor; the [command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md) lists what individual features need beyond that.

**What does it need on the central side?**
A machine with the controller programs, Ansible and SSH access to the targets if you observe many systems, and Postgres if you want history and comparisons across systems and time. To look at one system, you need neither.

**Does it need internet access?**
Installing and building need it (source code, dependencies). Running the demo downloads container images. In normal operation pqcota reaches your systems over SSH (or WinRM for Windows), and writes to the database or files you point it at. The pqcota programs contain no built-in upload or telemetry path; this comes from a search of the source for network client code, not from measuring traffic. If your policy requires proof, watch the traffic yourself.

## Effect on running systems

**Can observing disturb production?**
It depends on the observation, and they differ in what they touch.

| Observation | What it does on the system |
|---|---|
| **OpenSSL** (`pqcota-nodescan`) | Reads the process list and the loaded libraries through `/proc`, and reads library files to find their version. It does not attach to processes or run other tools |
| **Connections** (`pqcota-netcap`) | Opens a raw packet socket and receives the frames on a network interface into memory for a short window (8 seconds by default), then extracts the negotiation details from the TLS and SSH handshakes. It is not limited to receiving handshakes. It does not decrypt traffic and does not write raw packet captures to a file |
| **Java** (`pqcota-jvmscan`) | Not passive. To read the provider list of a running JVM it attaches to it and loads a small agent into that JVM. The agent lists the registered security providers and writes them to a file in the target's `/tmp` (`pqcota-providers-<pid>.txt`); it does not change the providers |

pqcota labels all three as non-invasive in its own capability report. The Java step still runs code inside the target JVM and uses the JVM's attach mechanism, so decide whether that fits your policy. You can look first without observing: `pqcota-jvmscan --recon` lists the JVMs found, and `--pid` limits an observation to one JVM. Which path `pqcota-jvmscan` takes depends on what you give it. With an agent file (`PQCOTA_JVM_AGENT`) and at least one JVM found, it attaches. Without an agent file, but with `--pid`, it reads that JVM's static configuration and cannot see runtime registrations. **Without an agent file and without `--pid`, it starts a Java launcher of its own** to read the default provider chain, even if JVMs are running, and records the result as degraded, not as an observation of a running application. Starting it needs a Java runtime on the machine. The result's note says why the probe ran (JVMs found but no agent given, process list unreadable, or no JVM running), so read it before relying on the result.

**What happens if a run fails or is interrupted?**
The playbook's cleanup removes its own temporary directory as the last step of a successful run. After a failure or interruption that directory can be left behind, so check for it. **A successful run does not remove everything**: the Java attach path writes `pqcota-providers-<pid>.txt` in the target's `/tmp`, outside the directory the playbook deletes, and the collector reads that file but does not delete it. The fallback that uses a JDK client deletes its own file. Check the targets' `/tmp` if that matters to you. A collector that did not run leaves no gap record in the inventory; compare your run log with the list of systems you meant to observe.

**Does pqcota change my systems by itself?**
It does not apply post-quantum changes: those are generated as files, and you apply them with your own tools. Observation is not free of side effects, though. The Java step runs code inside the target JVM and leaves a file behind, and the connection capture uses a raw packet socket.

## Data

**What does it collect?**
Per system: which cryptography libraries are loaded and their versions, which Java security providers are registered, and, for connections it watched, the peer address and port, the protocol, the negotiated key-exchange group and the cipher. That is information about your infrastructure and should be handled as such.

**Does it read our traffic, passwords or keys?**
The connection capture receives frames into memory and extracts the negotiated algorithm names and the peer address from the handshakes. This capture path does not decrypt traffic and does not write raw packet capture files. It does not promise that no content is ever present in memory while it runs. The inventory has no field for logins or keys. The Ansible target list that does hold access details is written for the run, readable only by its owner, and is not stored in the inventory. pqcota is not designed to read key material, but this page does not claim an audit of every code path for it.

**Where is the data stored, and who can read it?**
In the Postgres database and files you choose. Without a database configured, the inventory history of that run exists only in memory and is lost when the program ends. The result files the collectors produced, any output you saved and your run logs are not affected by that, and they need a retention and deletion policy of their own. Who can read any of it is decided by the access controls on that database and those files. If several organizations share one database, set an organization identifier so their records are kept apart; once records are mixed they cannot be separated.

**Are results signed?**
Optionally. If a collector is given a private key it signs its result, and if the loading side has the matching public key it checks the signatures and refuses mismatches. If no key is configured, verification is skipped and the load summary counts those results as *unchecked*, separately from checked ones. You can require verification so that loading does not start without a key.

**Does anything leave our network?**
See "Does it need internet access?" above. There is no built-in upload path. What you do with the results afterwards is up to you.

## Reading results

**What does 🔴 mean?**
A classical algorithm was negotiated on that connection while pqcota was watching. It is not a finding of vulnerability or non-compliance, and pqcota does not score risk. See [How to read a result](../README.md#how-to-read-a-result).

**What does "confirmed" mean?**
The strength of evidence, assigned by how a fact was collected. Facts seen in a running system, in source, or by tracing are "confirmed". Facts from a supplied cryptographic bill of materials are graded lower, because nobody observed the running system.

**Why is a system missing from the results?**
There are several possible reasons, and pqcota records some of them and not others. It may not be on your registration list; a collector may not support its operating system; a needed privilege may be missing (recorded as a gap); or the collector may not have run at all (nothing is recorded). The [reporting guide](reporting-guide.md#3-what-was-not-seen) has the full table.

**Why did the inventory not change after I applied a change?**
For OpenSSL, pqcota observes the library and its version, not which providers are loaded, so adding a provider does not show up by itself. In the project's own demo, the number of ML-KEM algorithms available on a test node went from 0 to 14 after applying a provider and back to 0 after the undo, and re-observation still showed no change. A connection also changes grade only when both ends support the new algorithm. See [Confirming a change, and its limits](reporting-guide.md#7-confirming-a-change-and-its-limits).

**What does `@?` mean next to a connection?**
The application that opened the connection could not be identified, usually because the connection closed before the lookup. A person can fill it in with `pqcota-declare-attribution`; values filled in that way are marked as declared and are not mixed into the observation.

## Changes

**Can pqcota apply the change for me?**
No. It generates standard Ansible files, and you run them. It has no remote-execution engine, so whether and when a change was applied is recorded by your deployment tool, not by pqcota.

**Can a change be undone?**
Each generated change has a matching removal file. Changes add files instead of overwriting existing ones, so removal deletes what was added, and when the plan defines activation steps, the removal runs its deactivation steps first. This is not a restore of an earlier deployment: if the same paths were deployed to twice, removal deletes the files and does not bring the earlier version back. A change that cannot be delivered through configuration is marked as a manual step, and so is its undoing.

**Who approves a change, and how is that checked?**
Generation requires a finalized plan with an approval signature. By default the generator checks the signature against the approver's registered public key and refuses when it cannot. Two separate options relax it. `--allow-unverified-approvals` lets generation continue without verifying approval signatures. `--allow-incomplete` lets output that has blanks end with exit code 0; it does not skip signature verification. Each prints a warning saying it was a choice, and the [reporting guide](reporting-guide.md#5-approval) says how to report them. Your organization's review process and the custody of approver keys are outside pqcota.

**What if we run OpenSSL 1.1.1 or an old JDK?**
Nothing is generated for them, because configuration cannot add post-quantum algorithms there. The output marks the change as a manual step: replacing the library or upgrading the JDK. See [Supported systems](../README.md#supported-systems) for the full table.

**Does it generate changes for Windows?**
Not yet. Windows systems can be observed. Generating their changes is on the [roadmap](../RELEASE_NOTES.md).

**Does it handle runtimes other than OpenSSL, Java and Windows CNG?**
Those three are what it observes. Other runtimes are not observed as a category of their own.

## The program around it

**Is pqcota a compliance tool, or does it tell me how many systems are "quantum-safe"?**
No. It supplies observations, comparisons and generated files as evidence, and certifies nothing. It grades observed connections, not systems, and it does not score risk. See [Sentences a pqcota result does not support](reporting-guide.md#8-sentences-a-pqcota-result-does-not-support).

**How often should we run it?**
Observation shows what is in use at the moment it looks, so repeating it over time is what makes the history useful. pqcota does not schedule runs; use the scheduler you already have.

**Does it scan our source code or container images?**
It does not scan source code. If your build pipeline produces a cryptographic bill of materials (CycloneDX), pqcota can receive it and put it in the same inventory, marked with the lower evidence strength described above. pqcota has no container-image scanning feature.

**How mature is it?**
Pre-1.0 (current release v0.10.0). Contract changes are kept additive; see the [compatibility policy](compatibility.md). Security fixes land on `main` and the latest release only; see [SECURITY](../SECURITY.md).

**What does it cost, and under what license?**
It is open source under Apache-2.0; see [LICENSE](../LICENSE) and the [license notes](licensing.md).

**Where do I report a problem or ask a question?**
Bugs, questions and proposals go to the repository's issues. Report a vulnerability privately, as [SECURITY](../SECURITY.md) describes.
