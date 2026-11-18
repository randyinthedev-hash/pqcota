English · [한국어](environment-variables.ko.md)

# Environment variable reference

Every environment variable that pqcota's programs read, in one place. Names, accepted values and defaults come from the code. How to use each command is in the command reference of its stage; this page links there instead of repeating it. For the overview of the documents, see the [developer documentation](developers.md).

- **Read when the program starts.** Changing a variable does not affect a program that is already running.
- **Keep the database and the organization the same for the commands that share a store.** `pqcota-ingest`, `pqcota-cbom-ingest`, `pqcota-inventory`, `pqcota-prune`, `pqcota-provision`, `pqcota-records`, `pqcota-hosts` and `pqcota-profile` read `PQCOTA_ORG`, so set it the same way for all of them. `pqcota-hosts` and `pqcota-profile` take the database from `--dsn`, not from `PQCOTA_DSN`, so point the flag at the same database yourself.
- **Earlier releases ignored the organization in two commands.** In v0.10.0 and earlier, `pqcota-hosts` and `pqcota-profile` wrote endpoints and profiles to the default organization whatever `PQCOTA_ORG` said, and `PQCOTA_REQUIRE_ORG=1` made them fail even when an organization was named. Rows they wrote that way stay in the default organization: pqcota cannot tell which organization they belonged to, so it does not move them. To have them under an organization, decide that yourself and run the command again with `PQCOTA_ORG` set; that writes new rows under the organization and leaves the old ones where they are.
- **Private keys are secrets.** `PQCOTA_SIGN_KEY` and `PQCOTA_APPROVAL_KEY` hold private keys. Treat the environment they are set in accordingly; the public keys (`PQCOTA_VERIFY_KEY`, `PQCOTA_APPROVAL_KEYS`) are not secret.

## Store and organization

| Variable | Read by | Value | When unset |
|---|---|---|---|
| `PQCOTA_DSN` | `pqcota-ingest`, `pqcota-cbom-ingest` (optional); `pqcota-inventory`, `pqcota-prune`, `pqcota-records` (required) | A Postgres connection string, as a URL (`postgres://user:password@host:5432/db`) or as `key=value` pairs. See the [format](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md) in the discovery command reference (`pqcota-hosts`) | The two ingest commands use an in-memory store, which prints a summary and keeps nothing after the process ends. The other three exit with an error (status 2), because their view needs a store that another process wrote |
| `PQCOTA_ORG` | The eight commands listed above | An organization name: 2 to 64 characters of lowercase letters, digits and hyphens, not starting with a hyphen. A value that does not fit is always an error and is never replaced by the default | Records belong to the organization `default` |
| `PQCOTA_REQUIRE_ORG` | Every command that opens the store | `1` | With `1`, opening the store without an organization is an error, and so is naming `default` explicitly (it is the name for records written with no organization), so nothing is written to `default` by mistake. Any other value, or unset, allows both |
| `PQCOTA_AUTO_DDL` | Every command that writes to the store | `0` | Unless it is exactly `0`, a writing command creates or brings up to date the tables it uses each time it opens the store, which only the owner of the tables may do. With `0` it does not, and it stops if a table or column is missing. It covers the history, machine metadata, source-evidence and provisioning-record tables. The query commands never create tables, and `-init-schema` does not read this variable. See [setting up the database](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.md#setting-up-the-database) |

Why these exist: when several organizations share one database, records that were written under the wrong organization cannot be separated afterwards, and an automatic `CREATE TABLE` against a wrong connection string quietly creates and writes to an empty table. `PQCOTA_REQUIRE_ORG=1` turns the first into an error where it applies, and `PQCOTA_AUTO_DDL=0` does the same for the second; the tables are then created on purpose with `-init-schema`. Details are in the [inventory command reference](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.md).

`pqcota-hosts`, `pqcota-profile` and `pqcota-provision` take the database as a `--dsn` flag, not from `PQCOTA_DSN`.

## Signing and verifying observations

| Variable | Read by | Value | When unset |
|---|---|---|---|
| `PQCOTA_SIGN_KEY` | `pqcota-nodescan`, `pqcota-netcap`, `pqcota-jvmscan`, `pqcota-cngscan`, `pqcota-declare`, `pqcota-declare-attribution` | A base64 ed25519 **private** key, made by `pqcota-keygen` | The result is not signed. If the key is set but cannot sign (not base64, or a public key by mistake), the program stops with an error instead of emitting an unsigned result |
| `PQCOTA_VERIFY_KEY` | `pqcota-ingest` | Comma-separated `<collector-id>:<base64 public key>` entries (**bound form**), or comma-separated base64 public keys (**legacy form**). See below | Signatures are not verified, and the load summary counts those results as *unchecked* |
| `PQCOTA_VERIFY_KEY_FILE` | `pqcota-ingest` | The path of a key file that binds each key to nodes and collector IDs. See [binding keys to nodes](#binding-keys-to-nodes). Cannot be set together with `PQCOTA_VERIFY_KEY` | `PQCOTA_VERIFY_KEY` applies |
| `PQCOTA_REQUIRE_SIGNATURE` | `pqcota-ingest` | `1` | With `1`, loading does not start when there is no key to verify with. Any other value, or unset, lets it proceed unverified. In the bound form a signature is always required, whatever this is set to |

The key pair is generated by [`pqcota-keygen`](https://github.com/randyinthedev-hash/pqcota-common/blob/main/cmd/README.md#pqcota-keygen), which lives in `pqcota-common` because plan approval uses the same kind of key. `pqcota-keygen -collector <id>` prints the verify line in the bound form, and with `-node <node-id>` also the line for a key file.

### Binding keys to collector IDs

Every result names the collector that produced it (`openssl-collector`, `network-collector`, `jvm-collector`, `cng-collector`, `declaration-importer`, `attribution-importer`). The form of `PQCOTA_VERIFY_KEY` decides whether the key has to belong to that collector.

```
PQCOTA_VERIFY_KEY="openssl-collector:<key A>,network-collector:<key B>,jvm-collector:<key C>"
```

- **Bound form** (every entry is `<collector-id>:<key>`). A result is checked only against the keys listed for the collector it names. A result that names a collector with no listed key, a result signed with another collector's key, and a result with no signature are all refused before anything is stored. The refusal is recorded with its reason. Declarations made by a person (`pqcota-declare`, `pqcota-declare-attribution`) go through the same check. When a check fails, `pqcota-ingest` does not try the other keys.
- **Legacy form** (bare keys, no colon). Any listed key passes for any collector, as before v0.15.0. App declarations are taken without checking their signature. `pqcota-ingest` prints a warning.
- **Mixing the two forms is an error.** So is an empty ID or key, a key that is not a 32-byte ed25519 public key, or the same key listed twice for one ID. `pqcota-ingest` stops with status 2 before it reads any result. The message says which entry is wrong and does not print the key.
- An ID that this version's tools do not write only gives a warning, in case it is a typo.

**Two keys for one ID is a rotation.** List the old key and the new key under the same ID; results signed with either pass. That ID is still told apart from the others.

**One key for several IDs means those IDs cannot be told apart.** A collector holding that key can sign as any ID the key is listed under. If one node runs three collectors with the same key and you list it under all three, a result from one of them can claim to be another and still pass. Only IDs that do not list the key are protected. If you want each collector's identity checked on its own, give each ID its own key.

With `PQCOTA_VERIFY_KEY` the node a result names is not bound to the key: a result signed with a listed key passes this check whatever node it names. To bind keys to nodes, use a [key file](#binding-keys-to-nodes).

`pqcota-cbom-ingest` takes CycloneDX from your own CI, not collector results, and does not check signatures. These settings do not apply to it.

### Moving an existing deployment to the bound form

Do the steps in this order. If `pqcota-ingest` switches to the bound form first, every collector that does not sign yet, or signs with a key you did not list for it, is cut off.

1. **Update the central tools** to v0.15.0 or later. Keep `PQCOTA_VERIFY_KEY` as it is. It keeps working, with a warning.
2. **Update the collectors** on the nodes. From v0.15.0 all four collectors and both declaration tools sign when `PQCOTA_SIGN_KEY` is set. The old key list still verifies them.
3. **Check which key signed what.** Run `pqcota-ingest -verify-keys <results-dir>` with the current setting. It stores nothing. For each result it prints the collector ID, whether it is signed, the fingerprint of the key that matched, and the verdict. Compare that with **your record of which key you deployed to which node and collector**. The fingerprint only tells you which key signed; whether that key was really meant for that collector ID is for your record to say. If they disagree, find out why before going on.
4. **Write the bound form.** Decide which key goes under which ID **from your deployment record**, not from the table in step 3. Do not leave a key out only because it did not show up in the table. A collector that did not run during the check would be cut off. Leave a key out when your record shows no collector holds it.
5. **Confirm.** Run `-verify-keys` again with the new value and check that nothing is refused. In the next real load the summary says `signature check: bound to collector IDs (N IDs, M keys)` and `signature-rejected 0`. When every collector in your record has been loaded at least once, the move is done.

To undo, put the old key list back.

To rotate a key later: add the new key under the same ID, switch that collector's `PQCOTA_SIGN_KEY` and update your record, then remove the old key once your record shows no collector still holds it.

`-verify-keys` is for these checks only. It does not replace loading and does not apply the scope list.

### Binding keys to nodes

`PQCOTA_VERIFY_KEY` binds a key to a collector ID, not to a node. To bind keys to nodes, give `pqcota-ingest` a **key file** instead: `PQCOTA_VERIFY_KEY_FILE=<path>`. A result is then checked only against the keys listed for **its collector ID on the node it names** (`target_node_id`). A key given to `web-01` cannot sign for `web-02`.

```
# node ID     collector IDs                                     public key
web-01        openssl-collector,network-collector,jvm-collector  <key of web-01>
web-02        openssl-collector,network-collector,jvm-collector  <key of web-02>
*             declaration-importer,attribution-importer          <key of the declaration tools>
```

- One line per key: the node ID, the collector IDs (comma-separated), the base64 public key, separated by spaces. Everything after `#` is a comment. `pqcota-keygen -node <node-id> -collector <ids>` prints the line.
- **A signature is always required**, and declarations are checked too, as in the bound form of `PQCOTA_VERIFY_KEY`. A result whose node and collector have no line, or whose signature does not match the keys on its line, is refused before anything is stored. The reason names where the key does belong, for example `signature does not match the keys registered for openssl-collector on web-02; it matches a key registered for openssl-collector on web-01`. `pqcota-ingest` does not try other lines.
- **The node a result writes about must be the node it was signed for.** The edges in a result carry their source node (`src_node_id`), and app declarations are stored under it. If an edge's source node is not the result's `target_node_id`, the whole result is refused: `edge source node "web-02" is outside the node this result was signed for ("web-01")`. The other end of an edge (the peer) is not restricted. The collectors and declaration tools of this release already write results this way.
- **`*` in the node column matches every node.** It is for the declaration tools, which sign declarations for many nodes at once. A collector that runs on a node listed under `*` is not bound to that node, and `pqcota-ingest` warns about it.
- **Which node IDs to write**: the node IDs you have approved. That is the node argument given to the collectors, or the self-assigned ID (`node:…`) a collector derives from the machine when run without one. Check a self-assigned ID on the machine first and compare it with your deployment record before you list it. `host://local` cannot be listed: it means the collector did not know its node, and every machine can use it.
- **Rotation**: two lines for the same node and collector; results signed with either pass. **One key on several nodes** makes those nodes indistinguishable, and `pqcota-ingest` warns.
- **Errors stop `pqcota-ingest` with status 2 before it reads any result**: a missing or unreadable file, a line without three fields, an empty collector ID, a key that is not a 32-byte ed25519 public key, `host://local`, the same key listed again for one node and collector, a file with no lines, or `PQCOTA_VERIFY_KEY` set at the same time. The message gives the line number and does not print the key.
- **The file is read once.** `pqcota-ingest` reads it when it starts and uses what it read for the whole run.
- **The scope list is separate.** If you also give a scope list, `pqcota-ingest` warns about nodes that are in one and not in the other. A key line does not register a node.

**Who may change the file.** The key file decides which keys are trusted, so the operator owns it and the account that runs `pqcota-ingest` can only read it. For example, the file is `root:pqcota-ingest` with mode `0640` and its directory is not writable by that account. Using a file does not by itself stop the loading account from changing it: if that account can write the file or its directory, it can change which keys are trusted. `pqcota-ingest` warns when the account it runs as can write the file or the directory, or when the file is writable by its group or others. These warnings do not stop it.

### Moving from collector-bound keys to a key file

`pqcota-ingest` refuses to run with both `PQCOTA_VERIFY_KEY` and the key file set, so the ingest stays on one setting throughout. Add the new public keys to the current setting **before** switching any collector, so both old and new keys pass while the collectors change one by one.

1. **Make the node keys.** For each node, `pqcota-keygen -node <node-id> -collector openssl-collector,network-collector,jvm-collector`. Collect the key-file lines into the key file, and add a `*` line for the declaration tools' key. Do not give the file to the ingest yet.
2. **Add the new public keys to `PQCOTA_VERIFY_KEY`.** Add each node key under every collector ID it signs for (`pqcota-keygen` prints that form too). Keep the old keys. Restart the ingest. At this step the new keys are bound to collector IDs only, as the old ones are.
3. **Switch the collectors one by one** to their node key in `PQCOTA_SIGN_KEY`. Results signed with the old and the new keys are both accepted.
4. **Check against the key file.** When every node has switched, run the check on freshly collected results, unsetting the old variable for that one command:
   ```bash
   env -u PQCOTA_VERIFY_KEY PQCOTA_VERIFY_KEY_FILE=/etc/pqcota/verify-keys pqcota-ingest -verify-keys /var/lib/pqcota/results
   ```
   It stores nothing. Every result must pass; the key column shows the line that matched. A result still signed with an old key means that node's collector has not switched yet.
5. **Switch the ingest to the key file.** Remove `PQCOTA_VERIFY_KEY` from its settings and set `PQCOTA_VERIFY_KEY_FILE`. Old keys are not in the file and are refused from now on.
6. **Retire the old keys** once your deployment record shows no collector still holds them.

To undo, put `PQCOTA_VERIFY_KEY` back and remove `PQCOTA_VERIFY_KEY_FILE`.

## Plan approval

| Variable | Read by | Value | When unset |
|---|---|---|---|
| `PQCOTA_APPROVAL_KEY` | `pqcota-approve` | A base64 ed25519 **private** key | The command exits with an error (status 2): there is no key to approve with |
| `PQCOTA_APPROVAL_KEYS` | `pqcota-provision` | Comma-separated `<approver>=<base64 public key>` pairs. A pair that is not in that form is an error | By default the generator refuses to run, because an approval it cannot check shows nothing about who took responsibility. The `--allow-unverified-approvals` flag continues without checking, with a warning |

`PQCOTA_REQUIRE_APPROVAL=1` is an older name that the code no longer reads. Refusing an unverifiable approval is now the default, so setting it has no effect.

How approval works and what the exit codes mean: the [provisioning command reference](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/cmd/README.md) and the [reporting guide](reporting-guide.md#5-approval).

## Observation programs

| Variable | Read by | Value | Default and notes |
|---|---|---|---|
| `NETCAP_IFACE` | `pqcota-netcap` | A network interface name | `eth0`. The interface argument on the command line overrides it |
| `NETCAP_WINDOW_SEC` | `pqcota-netcap` | Observation window in whole seconds, greater than zero | `8`. The window argument on the command line overrides it. A value that is not a whole number is ignored without a message and the default is used. Zero or a negative number is accepted, but the capture then runs for 3 seconds, while the start-up message prints the value you gave |
| `PQCOTA_JVM_AGENT` | `pqcota-jvmscan` | Path to `collector.jar` | Unset: no attach. With it, and at least one JVM found, each is attached. Without it, see the note below |
| `JAVA_BIN` | `pqcota-jvmscan` | Path to a Java launcher | Used by the probe path only. If unset: the launcher of a JVM that was found, otherwise `java` on the `PATH` |
| `JVMSCAN_CP` | `pqcota-jvmscan` | A class path | Passed to the probe path's Java launcher as `--class-path`. Unset: none |
| `PQCOTA_CLOUD_INSTANCE_ID` | `pqcota-nodescan`, `pqcota-cngscan` (through the machine identity fingerprint) | A cloud instance identifier, supplied by the operator | Unset: no cloud instance identifier is part of the fingerprint. Surrounding whitespace is trimmed. When set, it is the first input used to derive a node's self-assigned identifier, ahead of the machine id, the hardware UUID and the host name |

**The probe path.** Without `PQCOTA_JVM_AGENT` and without `--pid`, `pqcota-jvmscan` starts a Java launcher of its own to read the default provider chain and marks the result degraded, even if JVMs are running. With `--pid` and no agent it reads that JVM's static configuration. See the [FAQ](faq.md#effect-on-running-systems) and the [discovery command reference](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md) for privileges and the other collectors.

## Tests and the demo

These are not for production use.

| Variable | Where | Meaning |
|---|---|---|
| `PQCOTA_TEST_DSN` | The Postgres test cases | A connection string for a disposable database. Without it those cases are skipped. See [Check your work](build.md#check-your-work) |
| `DEMO_REAL_PROVIDER` | `demo/scripts/demo.sh` | `1` enables the optional stage that builds and applies a real post-quantum provider. See the [demo](../demo/README.md) |
| `PQCOTA_PROVIDERS`, `PQC_SERVER` | The demo's workload containers | Set by the demo's topology generator to give each container its providers. Do not set them yourself |

## Not environment variables

These look similar and are not read from the environment.

- **Command-line flags**, such as `--dsn`, `--level`, `--allow-unverified-approvals`, `--allow-incomplete`, `--strict` and `--pid`. They are in the command references.
- **Ansible variables** in the generated playbooks, such as `pqcota_module_sha256` and `pqcota_module_sha256_<provider>`. They are passed when you run the playbook; see the [provisioning command reference](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/cmd/README.md).

## Keeping this page right

The authority is the code, but one search does not give a complete list. When you add or change a variable, check each place a variable can come from: the non-test Go code of the five repositories (names read with `os.Getenv`, or held in constants such as `org.Env`), the demo shell scripts, the test code, and the comments, which may still mention a name that no code reads, as `PQCOTA_REQUIRE_APPROVAL` is. Then follow the call path to see which commands reach it, as the exceptions under "Store and organization" show. Update this page in the same change.
