# Compatibility policy: what we do not break

This repo publishes a contract, and other people consume it. So we write down **what we have promised not to break**.
Where a gate enforces the promise, that is written down too. A rule without a check drifts sooner or later.

> **§ notation**: a `§N` in this document is a section number of **this document**. References to other documents are written as links.

Compatibility is not one thing but **five faces**. Without splitting them, "compatible" stops meaning anything specific.

| Face | What breaks | What guards it |
|---|---|---|
| ① Contract (proto) | new code cannot read old results | `buf breaking` (CI) |
| ② Signature | the signature on an old result becomes invalid | `TestCanonicalCoversAllFields` |
| ③ Go API | consumer code no longer compiles | the §3 rules below |
| ④ DB schema | new code fails on an old DB | idempotent DDL + §4 |
| ⑤ Mixed versions | an old **binary** writes wrongly into a new DB | §5: **writes without an organization are refused** |

---

## 1. Contract: purely additive

We only **add** fields and enum values. We do not renumber, retype or delete. We do not rename either
(only the number travels on the wire, but the symbol in the generated code changes and breaks consumers).

CI runs `buf breaking` against the previous tag. It is a gate, not a rule.

**Comments are not an exception.** Changing only a comment changes `gen/`, so commit them together. CI fails if
`make generate` leaves a difference.

## 2. Signature: changing the scope invalidates the past

`sign.Canonical` covers **every field** except the signature field. When a field is added to the contract it must be updated here too
(`TestCanonicalCoversAllFields` watches the field count), and **the moment it is updated, every existing signature
becomes invalid.**

So a field is added **only in a release that has accepted a signature migration**. Filling in a value is different.
`Canonical` reads **values**, not the set of fields, so if we start filling a field that used to be empty, past results
still canonicalize to the same bytes ([v0.1.3](../RELEASE_NOTES.md) was such a case).

## 3. Go API: do not delete, do not change, add

This module is consumed with `go get`. Keep these four.

**① Do not change a function signature.** If it needs more arguments, make **a new function that takes an options struct**
and leave the existing function as a shell that calls it.

```go
func IngestResults(...) (*IngestReport, error)   // unchanged: calls the one below internally
func IngestWith(results []*discoveryv1.CollectionResult, o IngestOptions) (*IngestReport, error)
```

**② Do not add methods to an exported interface.** Every implementation outside would break. If you need one,
make **a separate interface**, and let the caller receive it through a type assertion or an option.

```go
type Store interface{ ... }          // untouched
type RejectionStore interface{ ... } // new: PgStore satisfies it as well
```

**③ Add constructors.** Keep `NewPgStore(ctx, dsn)` and add `NewPgStoreIn(ctx, dsn, org)`.
Existing callers do not change a single line.

To ask a value received through an interface about a new capability, **make one more small interface and use a type assertion.**
`org.Scoped` is the example: the organization can be asked without adding `Org()` to `history.Store`.

**④ The module path is outside these three. Changing it makes a new module.** Even when no signature is touched, changing the path
breaks every import on the consuming side. So this is the one thing that cannot be absorbed by adding, and the only way is to
ship it in a minor release and **write down what changes and how**.

We did it in v0.5.0: `github.com/pqcota/pqcota` → `github.com/randyinthedev-hash/pqcota`.
No repo existed at the declared path, so `go get` could not fetch it at all, and consumers had to carry a `replace` in their own
`go.mod` permanently. The path then equalled the repo address, so that reason could not arise again.

The path changed a second time when the repository was split into five (`pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning`, and the integration repository `pqcota`). This time the reason was structural: the contracts and the shared code moved to their own module, so `github.com/randyinthedev-hash/pqcota/gen/...`, `.../pkg/kernel/...` and `.../pkg/org` became `github.com/randyinthedev-hash/pqcota-common/...`, the inventory packages became `.../pqcota-inventory/...`, and so on for discovery and provisioning. The release that carries the split lists the mapping. **The wire format did not change:** only the `go_package` option of each proto file did, and `buf breaking` against the previous release shows no other difference. **Splitting again is a decision to make as carefully as this one**, because each module path is a promise to whoever imports it.

## 4. DB schema: idempotent, and never half-applied

The schema grows through **`CREATE TABLE IF NOT EXISTS` + `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`**.
When new code meets an old DB, it catches up on the spot.

**One thing is not idempotent: changing a PRIMARY KEY.** `ADD PRIMARY KEY` has no `IF NOT EXISTS`, and running a new
`ON CONFLICT` clause against a DB whose PK has not changed **stops with an error at run time.** Write such a migration as
a `DO $$ ... $$` block that looks in `pg_index` at **how many columns the PK has now** and runs conditionally.
Running it twice must give the same result. Add it only after running it twice against a real Postgres.

**Automatic schema creation is on by default. `PQCOTA_AUTO_DDL=0` turns it off, and with it off a missing schema stops with an error.** A constructor that
runs `CREATE TABLE` without telling anyone is convenient, but when it points at the wrong place, **a new empty table is created and written to.** The data then looks
as if it vanished. So there is a way to turn it off, and with it off the code checks instead of creating.

## 5. Mixed versions: refuse writes from an old binary that does not know the organization

This is the face that is easiest to miss. If the schema was migrated well, **an old binary can keep writing into the new DB**,
because `DEFAULT` fills the new column. That looks convenient, and it is the trap: an old binary does not know about
organizations, so it **writes into another organization's slot without raising any error.**

So in a deployment that requires organization isolation, **the default is dropped** as the last migration step.

```sql
ALTER TABLE pqcota_snapshots ALTER COLUMN org DROP DEFAULT;
```

After that, an INSERT from an old binary that does not know the organization **fails** with a `NOT NULL` violation. It is a failure that shows at once,
in place of contamination nobody notices. It is the same choice this repo has kept through exit code 0, the completeness map and the truncation record:
*what drops out without notice gets read as "not there".*

Users running a single organization do not take this step. The default stays, and nothing changes.

---

## Where this policy was applied

| Release | What it kept |
|---|---|
| v0.1.3 | Filled the empty `collected_at`. **Because it fills a value, past signatures stay valid** (§2). The clock became a package variable so no signature changed (§3①) |
| v0.2.0 | Added the organization axis with a new constructor (§3③) and without touching an interface (§3②). Only the PK change used conditional DDL (§4), and dropping the default is an optional step (§5) |
| v0.5.0 | Aligned the module path with the repo address (§3④). It is the one kind that cannot be absorbed by adding, so it went into a minor release with a note of what changes |
| the split | Moved the contracts and stages into their own modules (§3④). The wire format and the signatures are unchanged, so §1 and §2 are untouched. Consumers change imports and require the new modules |
