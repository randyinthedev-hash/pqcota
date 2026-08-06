English · [한국어](SECURITY.md)

# Security Policy

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](SECURITY.md) is authoritative.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](docs/regulation.md) (Korean).

`pqcota` is a tool that observes and normalizes an enterprise intranet's crypto assets and generates migration artifacts. Because of the data it handles (endpoint and crypto-asset inventory) and the privileges it requires (packet observation by the collector, etc.), we take security reports seriously.

## Supported versions

Before the first release (v0.1.0). Security fixes land only on **`main` latest**. After the first release, this table will be updated with the supported range.

| Version | Security fixes |
|---|---|
| `main` (in development) | ✅ |
| Official release | none yet |

## Reporting a vulnerability

**Do not open a public issue, PR, or discussion.** Use one of these private channels:

1. **GitHub private report (preferred)** — the repo's **Security tab → "Report a vulnerability"**. This follows a coordinated-disclosure process.
2. If that isn't possible, **contact the maintainer directly** — <randyinthedev@gmail.com>. Prefixing the subject with `[security]` helps it surface faster.

Including the following speeds up triage: affected component (collector/CLI/library), reproduction steps, impact (privilege escalation·information disclosure·integrity compromise, etc.), and a PoC if possible.

> **Anything that isn't a vulnerability belongs in a public issue.** Bugs, questions, and proposals go to **this repo's issues** — there's nothing to hide, and an open discussion stays for the next person. Private reporting is only for **things that could expose users to attack if known before a fix**.

## Response

As a solo project, response is **best-effort** — acknowledge receipt, reproduce and assess, then coordinate a fix plan with the reporter. There is no SLA before release, but valid reports are prioritized.

## Security scope (specific to this project)

The following in particular are treated as security issues:

- **Access-secret leakage** — connection keys/accounts live only in a runtime-only file, and only endpoints are ingested into the inventory (Postgres); secrets are not persisted (§1.5). If a secret leaks into a persistent store, logs, or artifacts, that is a vulnerability.
- **Signing·integrity** — bypassing the ed25519 signature on history records, or accepting forgery/tampering.
- **Collector privilege misuse** — actions beyond the packet-observation (`CAP_NET_RAW`) and process-scan privileges, or collecting the observed target's plaintext payloads (we only observe handshakes, without decryption).
- **Artifact injection** — paths where observed or contract inputs flow malicious content into the generated Ansible playbooks/config.

What is out of scope is discussed together at report time (e.g., operational problems in the environment where the user runs the playbook themselves are the execution side's responsibility, not this tool's).
