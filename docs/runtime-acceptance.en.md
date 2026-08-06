English · [한국어](runtime-acceptance.md)

# Crypto runtime acceptance principles

> 🌐 Translated from the Korean original. If the two differ, the [Korean version](runtime-acceptance.md) is authoritative.

> **§ notation**: unless stated otherwise, these are section numbers in the [process regulation](regulation.en.md).

---

## 1. The principle

What the platform targets is not a specific library but the abstraction **"a runtime that has crypto providers"**.

- **The process regulation (three stages, AUTO/PROPOSE/MANUAL) is invariant across runtimes.**
- What varies per runtime is four things — **(a) the discovery collection method, (b) the version and provider axis schema, (c) the remediation taxonomy branch, (d) the provisioning substrate.**
- Every finding and asset carries **`crypto_runtime` as a first-class field**. That field decides the runtime branch in each stage.

## 2. The two accepted — OpenSSL · JCA

The two runtimes are **conceptually isomorphic** in that both inject algorithm capability through providers. That isomorphism is what makes the "inject an internal provider without upgrading the version" strategy work on both sides.

### 2.1 Provider isomorphism

| | OpenSSL | JCA/JCE |
|---|---|---|
| Extension point | provider (`.so`), the 3.x provider API | Security Provider (JAR) |
| Activation | the provider section of `openssl.cnf` | `security.provider.N=` in `java.security` |
| Dynamic injection | dlopen/config | `Security.addProvider()` / `insertProviderAt()` |
| Requesting an algorithm | the high-level EVP API | `Cipher/Signature/KeyPairGenerator.getInstance()` |
| PQC provider precedents | oqs-provider, OpenSSL 3.5 native | BouncyCastle (ML-KEM/ML-DSA/SLH-DSA), recent JDKs natively |
| Internal provider injection | ✅ possible | ✅ possible |

### 2.2 The fundamental differences per runtime (why the branches exist)

**OpenSSL — the version decides whether a provider is possible at all**
- 3.0+ : the provider API exists → stage the file and activate it in `openssl.cnf`; injection without a rebuild
- 3.5+ : ML-KEM/ML-DSA/SLH-DSA natively, plus a TLS hybrid (X25519MLKEM768) by default → config only
- 1.1.1 / 1.0.2 : **no provider architecture** (ENGINE only) → TLS PQC via a loadable module is impossible; fork replacement or a proxy
- Version axis: `lib + version + fork` (OpenSSL/BoringSSL/LibreSSL/AWS-LC — the identical-soname problem)

**JCA/JCE — the registration mechanism and a two-part version axis are what matter**
- Registration layers: (a) the static ordered list in `java.security` (JRE-global), (b) runtime dynamic injection via `addProvider()` (**hidden in code, unscannable from files**), (c) explicit designation with `getInstance("...","BC")` (which makes a `java.security` change moot), (d) **priority negotiation** (if a provider earlier in the list services the same algorithm first, the new provider is ignored)
- **A two-part version axis**: `{jdk_vendor, jdk_version}` × `{provider_set}`. `pqc_readiness = "JDK native support" ∨ "provider augmentation"`, a logical OR
- Policies such as `jdk.tls.disabledAlgorithms` govern the effective posture

### 2.3 What is seen, and how (the detection branch per runtime)

**OpenSSL**
- Filesystem/packages: the actual `libssl`/`libcrypto`, `ldd`/`readelf` NEEDED, package reverse dependencies, **string signatures in static binaries** (to determine fork and version)
- Process/runtime: `/proc/*/maps`, `lsof`, `ss` (catches dlopen and vendoring; repeated across time windows so nothing is missed in a batch)
- Network (secondary): local TLS posture

**JCA/JCE**
- Artifacts: provider JARs inside JAR/WAR/EAR (`bcprov-*` and friends) plus parsing the Maven/Gradle **dependency graph**
- Policy parsing: registration order in `java.security` + `jdk.tls.*` + `disabledAlgorithms`
- Runtime introspection (ground truth): attach to a running JVM → `getProviders()` and the loaded provider chain (catching dynamic registration and explicit designation, which are invisible statically)
- Dynamic-registration blind spot: `addProvider()` is visible only through bytecode/source call-site analysis or a live query

**The provider signature registry** — registering the provider signatures below in the crypto registry lets discovery determine version, FIPS status, and algorithm coverage automatically. Each provider implies a different `pqc_readiness`, `fips_validation`, and algorithm coverage.

| Provider JAR/module signature | Nature | PQC coverage | FIPS |
|---|---|---|---|
| `bcprov-jdk18on-*` (BouncyCastle 1.79+) | pure Java, every JDK (1.8+) | ML-KEM/ML-DSA/**SLH-DSA** | not validated (standard edition) |
| BC-FJA (the FIPS variant) | FIPS 140-3 certified (native acceleration) | ML-KEM/ML-DSA/SLH-DSA | **140-3** |
| JDK native (24/25+, SunJCE extensions) | built into the runtime | ML-KEM/ML-DSA **only** (no SLH-DSA) | per JDK |
| openssl-jostle (a JNI bridge) | exposes native OpenSSL through JCA | ML-KEM/ML-DSA/SLH-DSA | follows the OpenSSL module |
| an in-house PQC provider | your own | to be defined | not validated |

**Rule**: SLH-DSA is not in the JDK natively, so assets that need SLH-DSA are tagged as depending on BC (or jostle) regardless of JDK version.

This table is an **initial seed**. The living source is `pkg/kernel/registry`, and it changes first whenever a provider is released — where it disagrees with the versions written here, the code is right.

### 2.4 How this lands in the schema

- `crypto_runtime`: `openssl` | `jca`
- OpenSSL: `lib`, `version`, `fork`, `binding_mode` (dynamic/static/dlopen/vendored)
- JCA: `jdk_vendor`, `jdk_version`, `provider_set` (registration order included), `registration_mode` (static/dynamic/explicit)
- Shared: `usage_context` (server/client/at-rest/signing), `pqc_readiness`, `fips_validation`, `remediation_class`, **`evidence_strength`**, **`detection_method`**

**Axis (d), the substrate, does not yet stand as a schema.** The staging path and config location are not a field; they split on a **two-way openssl/jca branch** — both being POSIX files is why this axis never surfaced until now. A runtime that uses the registry or GPO cannot be expressed by that two-way branch, so the generalization happens together with implementing that runtime.

---

## 3. Acceptance conditions — three assumptions the four axes do not state

The four axes say **what varies**, but the code demands three more things they leave unsaid.
Where a candidate gets stuck is always these conditions, never the axes.

1. **Provider isomorphism** — "algorithm capability is injected through a provider" must hold for
   `PROVIDER_INJECT` and `CONFIG_ONLY` to mean anything. Without it, remediation collapses into the single
   direction `REBUILD`. (How OpenSSL and JCA are isomorphic → the [§2.1](#21-provider-isomorphism) table.)
2. **A POSIX file substrate** — the artifact is a file, it is staged at a path, and removal reverses it
   (`ModulePath` → `/opt/pqcota/*.so|*.jar`, Ansible `copy`, `state: absent`).
3. **Distinct observation** — it must see **something different** from what existing collectors already see.
   Otherwise it is not a new runtime but a duplicate of an existing one.

**Condition 2 is what axis (d), the substrate, actually is.** Because openssl and jca are **both POSIX files**, this axis stayed hidden behind a `jca bool` two-way switch for a long time — until it was named as an axis, we did not even know it was a requirement.

---

## 4. How to decide

### 4.1 The decision tree

When you meet a new candidate, ask in order. A single "no" means it is not a new first-class runtime.

1. **Is the observation different from what an existing collector already sees?**
   No → **absorb it**: attach it to an existing runtime's finding via `app_keys` and `usage_context` (no new enum). *Example: Python.*
2. **Can the remediation be expressed as provider injection or config, or is `REBUILD` the only option?**
   `REBUILD` only → **absorb into the taxonomy**; no new render needed. *Example: Go (condition 1).*
3. **Does the deployment artifact fit POSIX file + path staging + rollback by file removal?**
   No → a separate track that requires **generalizing the substrate first**. *Example: Windows (condition 2).*
4. **Does the remediation point an existing openssl/jca provider at some *target*?**
   Yes → not a new enum, but a **backend target plus a new discovery axis**. *Example: HSM.*

Only after passing all four (distinct observation + provider-injection-shaped remediation + POSIX file artifact + its own provider model) is it truly a new first-class `CryptoRuntime`.

### 4.2 If you decide to accept it — what you touch

| Layer | File | Minimum output |
|---|---|---|
| Contract | `contracts/.../common.proto` (enum), `.../cbom.proto` (the `XxxAxes` oneof) | purely additive — `make breaking` compares against the released contract |
| Collection | `discovery/collectors/<r>/` | emit a `CollectionResult` and record `detection_method` |
| Normalization | `pkg/discovery/normalize/` (the enrichment step) | derive `evidence_strength` from `detection_method` |
| Provisioning | `pkg/provisioning/render.go` (the branch) + `renderXxx` + `paths.go` | render + stage + **rollback symmetry** |

One minimum test per layer. **Absorption is not concealment**: absorbing Python into openssl still shows up in the view as "attributed to a Python app", and absorbing Go as `REBUILD` is still reported honestly as "static, so a rebuild is required" — we never call something absent just because it is not visible.

**Layers (a) collection, (b) the schema oneof, and (c) the taxonomy vocabulary are designed as extension points**, so a new runtime is accepted without core changes. **The one layer that does not hold is the "POSIX file" assumption in (d), the substrate** — and only a non-POSIX runtime touches it.
