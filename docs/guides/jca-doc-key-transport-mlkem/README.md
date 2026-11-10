# Guide: moving document-key transport from RSA-OAEP to ML-KEM (JCA + Bouncy Castle)

| | |
|---|---|
| Guide ID | `pqcota-guide/jca-doc-key-transport-mlkem` |
| Revision | 1 |
| Crypto purpose | document-key transport: a document is encrypted with a symmetric key, and that key is passed to the recipient |
| Remediation kind | `REMEDIATION_KIND_APP_RECONFIG` — a configuration change is not enough; the application code changes |
| Example | [`DocKeyExample.java`](DocKeyExample.java), sha256 `28ad0e87a6c78865c7b1cf783f71c933c80f5b896a840ee4869ff3a3bbb1123f` |

**What exists today.** This page and its example can be used on their own: the example runs, and its tests are listed below. **pqcota does not link guides to plans yet.** Recording an approved guide in a plan and printing it with the generated artifacts is planned, not delivered.

**Which copy was approved.** Refer to this guide by a path fixed to a tag or a commit, for example `https://github.com/randyinthedev-hash/pqcota/blob/<tag-or-commit>/docs/guides/jca-doc-key-transport-mlkem/README.md`, not by the `main` branch. The sha256 of the example above is part of this page, so the hash of this page also fixes the example. A hash shows that something changed; only the fixed path lets you get the approved copy back.

## 1. When this guide applies

All of these must hold:

- A document (or other data) is encrypted with a symmetric key such as AES, and that **document key** is encrypted with the recipient's RSA public key (RSA-OAEP or RSA PKCS#1 v1.5).
- The cryptography goes through **JCA** (`Cipher.getInstance(…)`, `KeyPairGenerator`, and so on).
- The JDK is 21 or later, and the Bouncy Castle provider (`bcprov-jdk18on`) can be used.

It does not apply to:
- TLS key exchange. That is a TLS group setting (`jdk.tls.namedGroups` and similar), covered elsewhere.
- Signatures (approvals, integrity). Those move to ML-DSA, covered elsewhere.
- Code that calls the Bouncy Castle **lightweight API** (`org.bouncycastle.crypto.*`) directly, without JCA. It can move the same way, but the code differs, and runtime observation (pqcota) does not see that use.

## 2. What changes and what does not

- **ML-KEM encrypts neither the document nor the document key.** The sender uses the recipient's ML-KEM public key to produce a **shared secret** (encapsulation), derives a **key-encryption key (KEK)** from it with HKDF, and wraps the document key with the KEK (AES-KWP).
- Document encryption (AES-256-GCM) stays as it is.
- The recipient keeps an ML-KEM key pair (ML-KEM-768 recommended) instead of an RSA key pair.
- The sender passes, with the document, an **encapsulation** (1088 bytes for ML-KEM-768) and the **document key wrapped with the KEK** (40 bytes for an AES-256 key), instead of one RSA-wrapped key.
- The recipient's public key: the raw ML-KEM-768 encapsulation key is 1184 bytes (FIPS 203). The X.509 SubjectPublicKeyInfo encoding returned by `getEncoded()` is 1206 bytes. Say which encoding you mean when counting storage or transfer sizes.

## 3. Dependencies

- `org.bouncycastle:bcprov-jdk18on` — tested with **1.85**. Earlier versions were not tested.
- The standard KEM API of JDK 21 (`javax.crypto.KEM`). HKDF comes from Bouncy Castle's `SecretKeyFactory("HKDF-SHA256")`; JDK 21 has no HKDF.
- Provider registration: `Security.addProvider(new BouncyCastleProvider())`, or a registration in `java.security`.

## 4. Steps

1. Create an ML-KEM-768 key pair for each recipient: `KeyPairGenerator.getInstance("ML-KEM-768", "BC")`.
2. On the sending side, where the document key is wrapped:
   - Before: `Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")` and `wrap(docKey)`.
   - After: `KEM.getInstance("ML-KEM", "BC").newEncapsulator(pub).encapsulate()` → shared secret → HKDF-SHA256 with a fixed `info` value → a 32-byte KEK → `Cipher.getInstance("AES/KWP/NoPadding")` and `wrap(docKey)`. Pass the encapsulation and the wrapped key together.
3. On the receiving side: `newDecapsulator(priv).decapsulate(encapsulation)` → the same HKDF → `unwrap`.
4. Fix the `info` value (the name and revision of this use) on both sides. If it changes, earlier bundles no longer unwrap.
5. During the transition, documents wrapped with RSA keep being opened with RSA; new documents are wrapped with ML-KEM. How the two are told apart is up to the application's storage format, which this guide does not cover.

## 5. Tests (run them after the change)

| Test | Expected |
|---|---|
| Round trip | unwrapping the key and decrypting gives the original document |
| Changed encapsulation | unwrapping fails (ML-KEM yields another shared secret, so the KEK differs and AES-KWP refuses) |
| Changed wrapped document key | unwrapping fails |
| Changed document ciphertext | GCM authentication fails |
| Another recipient's private key | unwrapping fails |

The example runs these five, plus a round trip of the current RSA-OAEP way. They passed with JDK 21.0.11 (Eclipse Temurin) and Bouncy Castle 1.85 on 2026-11-10.

These tests show that changing parts of a bundle that **was already made** is refused. They do not show who sent it, or that the recipient's public key is genuine (see Limits).

## 6. Limits

- **The sender is not authenticated.** Anyone who has the recipient's public key can make a new document and a new bundle for that recipient. "Refused when changed" in the tests above is about a bundle that was altered, not about who made it. If the sender has to be verified, add a signature (for example ML-DSA) or an authenticated channel. RFC 9180 (HPKE) also treats encryption and sender authentication as separate; this example is not an implementation of HPKE.
- **The recipient's public key is not verified.** How the public key is obtained and trusted (certificates, key enrollment) is up to the application. A document wrapped for a fake public key is opened by whoever holds that key.
- **It is not hybrid.** Only ML-KEM is used. Combining it with a classical scheme (for example X25519 + ML-KEM), so that both have to be broken, is a separate design.
- **Private-key storage** (HSM, key stores) is not covered.
- **The storage format** (how the encapsulation, wrapped key and IV are kept together) is the application's. The example's output is a bundle of fields for the tests.
- Doing the same with the ML-KEM of the default provider (SunJCE) in JDK 24 and later, without Bouncy Castle, was not tested.
- **pqcota does not choose this guide.** Runtime observation sees that Bouncy Castle is registered and offers an ML-KEM service. **It does not see that the application passes document keys under RSA.** Whoever writes the plan chooses the guide; a source-analysis CBOM can supply the reason for the choice.

## 7. Sources

- NIST FIPS 203 (ML-KEM), RFC 5869 (HKDF), RFC 5649 (AES Key Wrap with Padding), RFC 9180 (HPKE; encryption and sender authentication are separate).
- The Bouncy Castle provider and the JDK 21 `javax.crypto.KEM` API documentation.

These are references. The steps and tests to follow are all in this page and the example. If a referenced page changes later, the hash of this page stays the same, so what was approved is this page and the example only.
