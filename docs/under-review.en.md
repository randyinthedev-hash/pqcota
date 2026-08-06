English · [한국어](under-review.md)

# Designs under review — what is not built yet

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](under-review.md) is authoritative.

> **This document changes no code.** It is where the design work happens for things that made the roadmap
> but are not committed to implementation. The real design documents (discovery, inventory, provisioning)
> record **only what stands today** — mixing in what is under review makes it impossible to tell fact from plan.

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
