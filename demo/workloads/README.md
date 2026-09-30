# workloads/: the demo's crypto workloads (what gets scanned and observed)

These are the crypto workloads that are actually **deployed and run** on the nodes. The assets and grades discovery shows **come from here**. To change what the demo shows, edit this folder.

| Workload | Deployed to | What it does | The observation it produces |
|---|---|---|---|
| `CryptoApp.java` | **pay-app** | registers the BouncyCastle provider in `java.security` and signs periodically (which keeps the JVM alive) | `pqcota-jvmscan` observes the **JCA provider chain (BC included)** |
| `pqc-echo/` (Go) | **pay-app** :8443 as the server, other nodes connect as clients | Go `crypto/tls` negotiates the **X25519MLKEM768 hybrid** | `pqcota-netcap` observes a **🟢 PQC edge** (`web-gw → pay-app`) |

In other words, each workload creates something to be observed, and the collectors observe it without decrypting anything:
- 🟢 PQC edge ← the X25519MLKEM768 handshake of **pqc-echo**
- JCA BouncyCastle provider chain ← the provider registration of **CryptoApp**
- (the 🔴 classical edge and the OpenSSL asset come from `sshd` and `openssl s_server` in the node base image, not from a workload)

> `pqc-echo` is a **traffic-generation fixture for the demo** (not a product command). It exists only to produce a PQC negotiation to observe. Source: [`pqc-echo/main.go`](pqc-echo/main.go).
