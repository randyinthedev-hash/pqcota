# Reporting guide

For the person who reports on a PQC migration, and for the people who read those reports. It explains which parts of a report pqcota can support with evidence, what to say about what it could not see, and which sentences a pqcota result does not support.

**pqcota does not write reports.** It has no report generator and no scoring. It produces observations, a history, comparisons and generated change files, and you assemble a report from them. **Every number has a unit, and the units differ.** A count of results, a count of findings and a count of systems cannot be added together. This guide says which is which, and a report should too. This guide follows the order in which a reader tends to ask questions: *what was covered, what was found, what was not seen, what changed, who approved a change, and did it work.* For the overview, start at the [README](../README.md).

## What each part of a report can rest on

| Part of the report | What pqcota can give you | Where it comes from |
|---|---|---|
| **Scope and period** | Which systems were on the registration list you supplied, how many findings were excluded by asset rules, when each state was recorded | Your registration list and asset rules; the load summary; `pqcota-inventory -history <node>` |
| **Findings** | Per system: cryptography libraries and versions, Java provider chains, each with an evidence strength. Per connection: the negotiated algorithm and a grade | `pqcota-inventory` (central) or `pqcota-discover-view` (one look, no store) |
| **What was not seen** | Gaps, excluded findings, results from systems that were not registered, results whose signatures were never checked | The load summary, the gap lines of the history and snapshot views, your run logs |
| **Changes since last time** | Added, removed and changed findings between two snapshots | `pqcota-inventory -diff <older>,<newer>` |
| **Approval** | Whether a plan was finalized and carries a signature that checks against a registered key | `pqcota-provision` (it refuses otherwise) and the plan's approval entries |
| **Changes generated** | Which actions the generator produced files for, and the observed state recorded before | `pqcota-provision` output and `pqcota-records` |
| **Changes applied** | Nothing. pqcota does not apply changes and does not record that they were applied | Your deployment tool's own log |
| **Confirmation** | A new observation after the change, compared with the old one | `pqcota-inventory -diff`, with the limits below |

## 1. Scope and period

A number in a report means nothing until the reader knows what it covers. State these three things first.

- **Which systems.** The inventory can require systems to be registered before their results are accepted. The total number of registered systems is the size of the list you supplied; pqcota does not print it, so count it yourself. The load summary reports **results** that were refused as unregistered or without an anchor (the line reads `unregistered/no-anchor N`). That is a count of results, not of systems, and it does not distinguish the two causes.
- **Which assets.** You can declare which assets to keep watching (for example, to leave out a distribution's default libraries). **Excluded is not absent.** The load summary and the inventory view state how many were excluded, and that number counts **findings** (assets), not systems. Quote it with that unit so a reader does not conclude that the excluded items do not exist, and do not add it to a count of systems.
- **When.** The inventory keeps a new snapshot only when the observed state changes. Repeat observations of an unchanged state are recorded as observation counts and time ranges against the existing snapshot, and `pqcota-inventory -history <node>` shows both. "Last changed" and "last observed" are different dates; say which one you mean.

## 2. Findings

Report what was observed, and keep the three kinds of statement apart.

- **A library or provider present on a system.** This shows what the system can use. It does not show that every connection uses it. For OpenSSL, pqcota observes the library and its version. For Java it can read the provider chain, including providers the application registered while running.
- **A negotiated algorithm on a connection.** This shows what two ends agreed on while pqcota was watching. A connection that did not happen in that window is not in the result.
- **A grade.** 🟢 post-quantum or hybrid, 🔴 classical, ⚪ undetermined. Grades describe what was negotiated. **🔴 is not a finding of vulnerability or non-compliance.** Whether and when to change it is a decision the report should attribute to the people who make it.

Quote the **evidence strength** with the finding. It records how a fact was collected: facts seen in a running system, in source, or by tracing are "confirmed"; facts taken from a supplied cryptographic bill of materials are graded lower, because nobody observed the running system.

## 3. What was not seen

This is the part of a report that most often gets left out, and the part a careful reader looks for first. pqcota is built so that many gaps are on the record. The `gap` column of `pqcota-inventory -history` names the observation layer a snapshot could not see; the detail, such as why connections could not be attributed to an application, is printed by `-snapshot`. Unchecked signatures, duplicates and refused results appear only in the load output. Gather these from the places they are printed rather than from one view.

| What happened | How pqcota records it | What to write |
|---|---|---|
| A collector is run on an operating system it does not support | A gap record and exit code 0, so the gap can reach the inventory | "Not observed: system X, reason Y." Never "no cryptography found" |
| Network capture without the needed privilege | A gap record for the network layer and exit code 0 | "Connections on X were not observed" |
| The OpenSSL scan cannot open the process list | The scan marks the process list as unavailable in its completeness note and produces a result | "Processes on X could not be listed" |
| The OpenSSL scan can list processes but is denied access to some of them | Counted as denied in the scan statistics, not recorded as a gap | Quote the denied count if you have it |
| A collector does not start, or its result file never reaches the controller | **Nothing is recorded.** There is no gap record to load | Compare your run log with the list of systems you meant to observe. A gap record being generated is also not proof that loading it succeeded |
| Java attach blocked | The static provider chain is read; runtime-registered providers stay a blind spot and are reported as a gap | "Runtime-registered providers on X may be missing" |
| Result signatures were not checked (no verification key) | The load summary counts them as *unchecked*, separately from the checked ones | State the number, and whether verification was turned on |
| Findings excluded by asset rule | The load summary and the view state how many were removed (a count of findings) | Quote the count and its unit |
| Results from systems that are not registered | Counted in the load summary (a count of results). A refusal entry (system, collector, reason, content hash) goes to the database if one is configured, and to memory, lost when the process ends, if not. The reviewed commands do not list these entries, and the original result is not kept there | Quote the count; **keep the load output**, because it is the record you will have |
| The same machine registered under several names | Flagged as a duplicate in the load output; pqcota does not merge or pick a name. Detection compares the results loaded together in that run. It does not search the whole history, and a machine without an identity fingerprint is not covered | Say that the systems count may be overstated until it is resolved, and that duplicates loaded in separate runs may go unflagged |
| A connection's application could not be identified (short-lived connections) | Shown as `@?`; a person can fill it in, and the filled-in value is marked as declared | Say how many are unattributed and how many were declared by a person |

## 4. Changes since last time

`pqcota-inventory -diff <older>,<newer>` lists findings added, removed and changed between two snapshots. Three cautions for the report:

- **Argument order matters.** The first argument is the older snapshot. If you give them the other way round, "added" reads as "removed"; the tool prints a warning when the order is reversed. Check that the warning is not present.
- **Different rule versions.** If the two snapshots were derived with different rule versions, the tool warns that a difference in derived values may be a recomputation and not a real change. Do not report such a difference as a change on the system.
- **"No change" needs a reason.** No new snapshot means the observed state was identical. It does not by itself say the systems are unchanged in ways pqcota does not observe.

## 5. Approval

A change is generated only from a plan that is finalized and carries an approval signature. What you can report:

- **The plan was finalized and approved, and the approval was verified.** By default the generator checks the signatures against the approvers' registered public keys and refuses when it cannot. State which approvers' keys were registered.
- **Say if verification or completeness was waived.** Two command-line options let generation continue: `--allow-unverified-approvals` (continue without verifying signatures) and `--allow-incomplete` (accept output that has blanks). Each prints a warning on standard error saying it was a choice. If either was used, the report should say so.
- **An exit code is not a certificate.** Exit 0 means the command did not fail. With `--allow-incomplete` it also means blanks were passed over, and the only trace is a warning on standard error. Exit 3 means the output has blanks. Exit 1 means the command stopped: the plan was refused (not finalized, no approval, an approval that cannot be verified), or a database connection, lookup or record write failed, and in that case the playbook may already have been written to standard output. Exit 2 means a usage or input error. Keep the standard error text, the options that were used, the generated files and whether the record was saved, and report from those, not from the exit code alone.
- **What pqcota does not manage.** Who reviews a plan, in what order, and who holds the approver keys belong to your organization's process. A signature shows that a holder of a registered key approved this plan; it does not show that your process was followed.

## 6. Changes generated and applied

`pqcota-records` lists what was recorded for each change: its id and status, the node, the plan, the affected applications, the modules recorded before (and after, if filled in), and whether the snapshot reference could be resolved.

- **`STAGED` means the generator recorded the change. It does not mean anything was placed on a node.** The generator does not connect to nodes. Keep two lines in a report: *generated and recorded* (from pqcota) and *applied* (date and result, from your deployment tool's log only).
- **Records exist only if the generator was given a database** (`--dsn`). Without one, the before state is not captured and nothing is saved.
- **The recorded "before" is the state the inventory had observed**, such as module and version. It is not a backup, and it does not hold the full configuration or the provider chain of the system.
- Each generated change has a matching removal file. Removal deletes what the plan staged. It is not a restore of an earlier deployment, and a change that cannot be delivered through configuration is marked as a manual step, so its undoing is manual too.

## 7. Confirming a change, and its limits

The natural last line of a report is "and we confirmed it worked". Be careful with it.

- **Observe again and compare.** Load a new observation and use `-diff`. For a Java provider chain the new provider appears in the chain, so the change is visible.
- **For OpenSSL the inventory may not move.** pqcota observes OpenSSL's library and version, not which providers are loaded. In the project's own demo, adding a real post-quantum provider raised the number of ML-KEM entries in `openssl list -kem-algorithms` from 0 to 14, and the re-observation still showed **no change**, because the provider layer is not observed. That result is the tool being honest, not a failure to apply.
- **A connection changes grade only when both ends support the algorithm.** Upgrading one end does not turn a 🔴 connection 🟢.
- **So write what you actually verified.** "The provider was staged and the node lists the algorithms" is a statement about capability. "Connections now negotiate a post-quantum algorithm" needs an observation of those connections.

## 8. Sentences a pqcota result does not support

| Do not write | Because | Write instead |
|---|---|---|
| "X% of our systems are quantum-safe" | pqcota grades observed connections, not systems, and does not score | "N of M observed connections negotiated a post-quantum or hybrid algorithm" |
| "No quantum-vulnerable cryptography remains" | pqcota shows what it observed; gaps and unregistered systems exist | "None was observed on the N systems covered; these systems were not observed: …" |
| "We are compliant" | pqcota is not a compliance product and certifies nothing | "Evidence attached: …" |
| "All systems were covered" | Registration and exclusion rules shape coverage | State registered, excluded and not-observed counts |
| "The change was verified" (from a re-observation alone) | The observation may not see the layer that changed | Say what was observed and what could not be |
| "Nobody can alter the history" | The tool's normal commands do not edit old records, but that is not tamper-proofing against someone with database administrator access. `pqcota-prune` can also cut old history, and it records that it did | "The tool records changes as new entries; access to the database is controlled by …" |

## Where each number appears in the output

Lines quoted from a run of the demo on 2026-09-30, abbreviated. The wording can change between versions, so search for the words, not the exact line.

| Report item | Command | Line to look for | Unit | Notes |
|---|---|---|---|---|
| Results accepted, refused, unchecked | `pqcota-ingest` | `ingest result: accepted 7 · unregistered/no-anchor 0 · signature-rejected 0 → 3 nodes observed (changed 3 · identical 0)` | results; the last part counts nodes | `unregistered/no-anchor` merges two causes |
| Signatures never checked | `pqcota-ingest` | `unverified signatures: 7 … they were never checked` | results | Printed only in the load output |
| Same machine, several names | `pqcota-ingest` | `⚠ duplicate: physical machine … registered under several node_ids → [pay-db web-gw]` | machines | Only among the results loaded in that run |
| Findings excluded by asset rule | `pqcota-ingest`, `pqcota-inventory` | `asset scope: 3 excluded as out of scope`, `excluded by asset scope: 1 (… not absence)` | findings | Not systems |
| When a state was recorded and re-confirmed | `pqcota-inventory -history <node>` | columns `changed`, `obs`, `observed`, `gap` | snapshots, observations | `gap` names the layer that could not be seen |
| Why something could not be seen | `pqcota-inventory -snapshot <id>` | `gap (unobservable by design != absent): … 3 of 4 edges could not be attributed to an app` | edges | The reasons are in this line, not in `-history` |
| Applications declared by a person | `pqcota-inventory -snapshot <id>` | `(1 of them are not observations but apps declared by a person)` | edges | |
| Grade totals | `pqcota-inventory`, `pqcota-discover-view` | `totals: 2 assets · 4 observed edges (🟢 PQC 2 · 🔴 classical 2 · ⚪ unknown 0)` | assets, edges | |
| What changed | `pqcota-inventory -diff <older>,<newer>` | `removed 1`, `added N`, `changed N`; warnings for reversed order and different rule versions | findings | After an asset rule, `removed` means dropped from management, not gone |
| Approval verified | `pqcota-provision` | `approvals verified: reviewer-1` | approvers | Or the warning that verification was not done |
| Waived completeness | `pqcota-provision` | `N incomplete spot(s) — passed over because --allow-incomplete was given` | blanks | Standard error; the exit code stays 0 |
| Record saved | `pqcota-provision --dsn` | `persisted 1 records (before capture · STAGED · rollback basis) · snapshot references resolved 1, unresolved 0` | records | `STAGED` is not application |
| What was recorded | `pqcota-records` | `• plan-demo:a1 [STAGED] node=… affected apps: … before : libssl.so.1.1@1.1.1f snapshot: …` | records | Modules only, not the full configuration |

Every line in the table was printed in that demo run except the two warnings in the `-diff` row. Those are printed under conditions in the code (the two snapshots given in reverse time order; the two snapshots derived with different rule versions) and did not occur in the run quoted here.

## A report skeleton

| Section | Fill in | Source |
|---|---|---|
| Scope | Size of your registration list (systems); excluded findings (findings); refused results (results). Keep the units apart | Your list, load summary |
| Period | Observation dates; last change date per system | `-history` |
| Findings | Libraries, provider chains, evidence strengths; connection grades and totals | `pqcota-inventory` |
| Not observed | Gaps with reasons; unchecked signatures; duplicates; unattributed connections; systems whose collector never produced a result | Load summary, `-history` gap column, `-snapshot` gap lines, your run logs |
| Changes since last report | Added, removed, changed; note the rule versions | `-diff` |
| Approvals | Plans, approvers, whether signatures were verified, options used (`--allow-unverified-approvals`, `--allow-incomplete`), generator exit code and standard error | `pqcota-provision`, plan files |
| Changes generated and recorded | Actions with generated files, the recorded before state, whether a record was saved | `pqcota-provision` output, `pqcota-records` |
| Changes applied | Date and result per system | Deployment tool log only |
| Confirmation | What was re-observed, what changed, what cannot be seen by pqcota | `-diff`, section 7 |
| Open items | Manual steps, incomplete plans, systems still not observed | Generator output |

## Questions to ask of a report you receive

- Which systems does this cover, and how many were registered, excluded or not observed?
- When was each system last observed, and when did it last change?
- Is a 🔴 here a connection that was watched, and who decides what to do about it?
- What was not seen, and why?
- Was every approval signature verified, or was verification or completeness waived by an option?
- Is each number a count of systems, results or findings?
- Does "applied" come from a deployment log, or only from a pqcota record?
- For a change described as done: what was observed afterwards, and could the tool see the layer that changed?
