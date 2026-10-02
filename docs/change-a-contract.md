English · [한국어](change-a-contract.ko.md)

# Change a contract

For an engineer who needs to add or change a message, field or enum in pqcota's protobuf contracts. The contracts are the one place the stages agree on, so a change there reaches every stage. This page gives the order of work, what else has to move with the contract, and what the local checks do and do not catch. For the rules about what may change at all, read the [compatibility policy](compatibility.md); for the contracts themselves, the [contracts overview](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.md) and the [data model](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/data-model.md). For the overall layout, see the [developer documentation](developers.md).

## Before you start

**Do you need a proto change at all?** Tool-specific detail about an asset can travel in the CycloneDX `properties` under the `pqcota:` namespace without touching the proto. The [key convention](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.md#cyclonedx-properties-extension-key-convention) lists the existing keys. A new key still needs the core to read it, but it does not change the generated code or the signature scope.

**Is the change additive?** Within `v1`, contract changes are additive: add fields and enum values; do not renumber, retype, delete or rename anything already published ([compatibility policy, the contract](compatibility.md#1-contract-purely-additive)). A change that is not additive is a new `v2` package, not an edit to `v1`. In a package where a field is dropped (for example a new major version), mark its number `reserved` so the number is never reused. Append new enum values at the end.

**Does it touch the signature?** If you add a field to `CollectionResult`, `Envelope`, `MachineIdentity`, `Completeness` or `ObservedEdge`, the signature scope has to widen, and **widening the scope invalidates every existing signature.** That is only acceptable in a release that has accepted a signature migration ([compatibility policy, signatures](compatibility.md#2-signature-changing-the-scope-invalidates-the-past)). Stop and decide that first.

## What a contract change reaches

| Where | What moves | How you notice if you forget |
|---|---|---|
| `pqcota-common` | The `.proto` files, the committed generated code in `gen/`, the [data model](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/data-model.md) (hand-written), and `sign.Canonical` if the field is signed | Generated code and the proto disagree: CI fails. Signature coverage: a test fails. The data model: nothing fails |
| `pqcota-inventory` | Normalization that reads the field; the snapshot fingerprints in `history` if the field is substantive content; and `ruleset_version` if you changed a derivation rule | The fingerprints: **nothing fails, and a change to the field can disappear from the history**. See below |
| `pqcota-discovery` | Collectors that should now emit the field | The field stays empty |
| `pqcota-provisioning` | The generator and the plan checks, for `plan.proto` and `rollback.proto`. A plan is signed too: `sign.CanonicalPlan` in `pqcota-common` | A test in `pqcota-common` fails (see below) |
| `pqcota` | Cross-stage tests, the demo's expected output, documents | The cross-stage checks fail, or the demo output differs |

Consumers inside the workspace see your change immediately, because each module reads `../pqcota-common` through a `replace` directive. Consumers outside the workspace see it only after `pqcota-common` is tagged and their `require` points at that tag.

## Order of work

1. **Edit the proto** in `pqcota-common/contracts/proto/`. Do not edit `gen/` by hand.
2. **Regenerate** and commit `gen/` in the same commit as the proto:

   ```bash
   cd pqcota-common
   make tools && make generate     # contracts/proto → gen/
   ```

   Changing only a comment changes `gen/` too, so commit them together. See [Build guide, Change a contract](build.md#change-a-contract) for the `PATH` caveat with `make tools`.
3. **Lint and compare.** `make lint` runs `buf lint`. `make breaking` compares against the latest release tag (currently `v0.10.2`); to compare your branch against `main`, run `make breaking AGAINST=main`. Both need `buf`, which is installed separately.
4. **Run the ripple check** below. This is where the build can pass while behavior breaks, because only the signature coverage is guarded by tests.
5. **Update the data model document** for the messages you changed.
6. **Update the consumers**, in the order of the dependency direction: inventory, then discovery and provisioning, then the integration repository. Each repository reads the new generated code directly through `replace`.
7. **Run the checks** from `pqcota`: `make all` runs every sibling's own checks and then the cross-stage ones. See [Check your work](build.md#check-your-work).
8. **Release.** Releases are currently coordinated: the same tag on all five repositories. For a contract change that means publishing `pqcota-common` first, raising each stage's `require` to that tag, and checking the combination before tagging the integration repository. This is how v0.10.0 and v0.10.2 were released and how [the build guide](build.md#get-the-source) describes tags; it is the current practice, not a rule that is enforced, and the release commands are not repeated here.

## The ripple check

Two things in the code are derived from the contract. A test guards the first. **Nothing guards the second**, so it depends on you and on review.

**Signature coverage (guarded by tests).** `sign.Canonical` covers every field of the signed messages except the signature itself. `TestCanonicalCoversAllFields` compares the field counts of `CollectionResult`, `Envelope`, `MachineIdentity`, `Completeness` and `ObservedEdge` with expected numbers. If a count changes, the test fails and tells you what to do: change `Canonical`, then update the expected number and add a case to `TestTamperBreaksVerification`. **Do not make the failure go away by editing the expected number alone.** That is how a field becomes a signing blind spot: someone can alter it and verification still passes. Plans have their own copy of this guard in `pqcota-common/pkg/kernel/sign/plan_test.go`: `TestCanonicalPlanCoversAllFields` counts `FinalizedPlan`, `RemediationAction`, `ActivationHooks`, `ActionEvidenceSource`, `SnapshotReference` and `SnapshotContentReference`, and points you to `TestTamperBreaksApproval`. Widening either scope invalidates the signatures that already exist.

**Change detection (not guarded).** The inventory keeps a new snapshot only when an observation's substance changes. Today that decision, and the lookup of a snapshot by a plan's reference, use the fingerprint `history.ContentHashV1`, with its stored column `content_hash_v1`. The older `ContentHash` is still computed and stored but is not what decides whether a state is new. If you add a substantive content field to `Finding`, `ObservedEdge` or `Completeness` and it is not part of the fingerprint, a change to that field folds into "no change" and **disappears from the history without any error.** No test watches the field count of either fingerprint.

**Do not edit `ContentHashV1` to add the field.** Format v1 (`pqcota-snapshot-content/v1`) is frozen: stored references are recomputed by the same rule, and `TestContentHashV1IsFrozen` pins its bytes on fixed input. Changing it would leave every existing reference unfindable. A field that has to count needs a **new fingerprint format version**, with the stored column, the lookup, the de-duplication path and the downstream consumers of `SnapshotContentReference` designed together. That is a design task, not a one-line change, so raise it before you start.

**A changed derivation rule.** If you change how a derived value such as evidence strength or the quantum-resistance grade is computed, bump `ruleset_version` (a constant in the inventory's normalization code, `normalize.RulesetVersion`). A derived value is a function of the rule, not a stored fact. Earlier values can be recomputed only if you kept the original collector results: an inventory snapshot holds the derived findings, edges, completeness and rule version, **not** the raw collection result, and pqcota has no command that recomputes the past for you. If you did not keep the originals, old snapshots keep the values the old rule gave them, and the rule version on each snapshot says which. Derived values never go in the collector's `pqcota:` properties.

**Order-bearing fields.** Do not sort or normalize a field where order carries meaning, such as `provider_set`, whose registration order is the priority.

## Local checks and CI

The same commands exist in both places, and CI adds three things.

| | In `pqcota-common`, locally (`make`) | In CI |
|---|---|---|
| Regenerate | `make generate` rewrites `gen/` | The same, **then `git diff --exit-code` fails the build if `gen/` changed**, so a proto change without its regenerated code is caught |
| Lint, compare, format, vet, build, test | Yes | Yes |
| `buf` version | Whatever you installed | Pinned in the workflow. A different local `buf` can produce different generated output and a spurious drift |
| Release tags for `make breaking` | Whatever you have fetched | A full clone, so the latest tag is always there |
| The other four repositories | `make all` in `pqcota` runs their own checks | A `cross-stage` job runs the integration repository's checks with your commit and `main` of the other four |

So a clean local `make` does not prove the drift check will pass. CI regenerates in a clean checkout of the commit and then fails if anything in the working tree differs. Locally, `make generate && git diff --exit-code -- gen/` shows whether regenerating changes the generated code, which is the same question, but it is not an exact copy of CI: your uncommitted edits to protos and documents are in the tree, and `git diff` only sees tracked files. The reliable reproduction is to commit your change, then run the same two commands on a clean tree. For the full explanation of local and CI checks, see [Check your work](build.md#check-your-work).

## Things that go wrong

- **Editing `gen/` by hand.** Change the proto and regenerate.
- **Changing only the proto.** CI fails the drift check. Commit `gen/` with it.
- **Adding a signed field without widening `Canonical`.** The count test fails; do not fix it by editing the number alone.
- **Adding a content field that is not in the fingerprint.** Nothing fails. The field's changes vanish from the history. Do not "fix" it by editing the frozen `ContentHashV1`.
- **Putting a derived value into `properties`.** Evidence strength and grades are the core's job.
- **Reusing a field number or changing a meaning.** That is a breaking change and needs a `v2`.
- **Changing `go_package`.** `buf breaking` reports it even though the wire format is unchanged. It happened once, in the repository split, and is not an everyday change.
- **Forgetting the data model document.** It is hand-written, and nothing checks it against the proto.
