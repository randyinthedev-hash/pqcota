[English](README.md) · 한국어

# workloads/: 데모의 암호 워크로드(스캔하고 관측하는 대상)

노드에 실제로 **배포되어 실행되는** 암호 워크로드입니다. 관측이 보여 주는 자산과 등급은 **여기서 나옵니다**. 데모가 보여 주는 것을 바꾸려면 이 폴더를 고치세요.

| 워크로드 | 배포 대상 | 하는 일 | 만들어 내는 관측 |
|---|---|---|---|
| `CryptoApp.java` | **pay-app** | BouncyCastle provider를 `java.security`에 등록하고 주기적으로 서명합니다(그래서 JVM이 계속 살아 있습니다) | `pqcota-jvmscan`이 **JCA provider 체인(BC 포함)**을 관측합니다 |
| `pqc-echo/`(Go) | **pay-app** :8443에서 서버, 다른 노드가 클라이언트로 연결 | Go `crypto/tls`가 **X25519MLKEM768 하이브리드**를 협상합니다 | `pqcota-netcap`이 **🟢 양자내성 연결 간선**(`web-gw → pay-app`)을 관측합니다 |

즉 워크로드마다 관측될 것을 만들고, 수집기는 아무것도 복호화하지 않고 그것을 관측합니다.
- 🟢 양자내성 연결 간선 ← **pqc-echo**의 X25519MLKEM768 핸드셰이크
- JCA BouncyCastle provider 체인 ← **CryptoApp**의 provider 등록
- (🔴 고전 연결 간선과 OpenSSL 자산은 워크로드가 아니라 노드 베이스 이미지의 `sshd`와 `openssl s_server`에서 나옵니다)

> `pqc-echo`는 **데모용 트래픽 생성 픽스처**입니다(제품 명령이 아닙니다). 관측할 양자내성 협상을 만들어 내려고 있을 뿐입니다. 소스: [`pqc-echo/main.go`](pqc-echo/main.go).
