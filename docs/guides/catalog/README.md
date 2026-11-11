# Guide catalog: one directory per guide and revision

This directory is what `pqcota-approve --guide-dir` and `pqcota-provision --guide-dir` read. Each guide revision has its own
directory, and a revision is never edited after it is published: a changed guide is a new revision.

```
catalog/<name>/<revision>/guide.json    the manifest: id, revision, title, the purposes it covers, its observation limits, the body and the examples
catalog/<name>/<revision>/README.md     the body
catalog/<name>/<revision>/<examples>    the example files the manifest lists, each with its sha256
```

A plan names a guide by id and revision and pins the sha256 of both the manifest and the body. The tools compare those hashes
with the files here, and the manifest's hash for each example with the example file. Nothing is fetched from a URL, and another
revision is never used in place of the one the plan names. Files outside the revision directory are not read.

| Guide | Revision | Readable copy |
|---|---|---|
| `pqcota-guide/jca-doc-key-transport-mlkem` | [1](jca-doc-key-transport-mlkem/1/guide.json) | [the guide](../jca-doc-key-transport-mlkem/README.md) |
