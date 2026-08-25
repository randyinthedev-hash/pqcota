# workloads/: 데모 크립토 워크로드 (스캔·관측 대상)

노드에 실제로 **배포·실행**되는 크립토 워크로드다. 디스커버리가 보여주는 발견 자산·등급는 **여기서 나온다**. 데모 결과를 바꾸려면 이 폴더를 손댄다.

| 워크로드 | 배포 노드 | 하는 일 | 만들어내는 관측 결과 |
|---|---|---|---|
| `CryptoApp.java` | **pay-app** | BouncyCastle provider를 `java.security`에 등록하고 주기적으로 서명(JVM을 살아있게 유지) | `pqcota-jvmscan`이 **JCA provider 체인(BC 포함)** 관측 |
| `pqc-echo/` (Go) | **pay-app** :8443 서버 + 타 노드가 client로 접속 | Go `crypto/tls`가 **X25519MLKEM768 하이브리드**를 협상 | `pqcota-netcap`이 **🟢 PQC 엣지**(`web-gw → pay-app`) 관측 |

즉 각 워크로드가 "관측될 무언가"를 만들고, collector가 그걸 복호화 없이 관측한다:
- 🟢 PQC 엣지 ← **pqc-echo**의 X25519MLKEM768 핸드셰이크
- JCA BouncyCastle provider 체인 ← **CryptoApp**의 provider 등록
- (🔴 고전 엣지·OpenSSL 자산은 노드 base 이미지의 `sshd`·`openssl s_server`에서: 워크로드 아님)

> `pqc-echo`는 **데모 전용 트래픽 생성 픽스처**다(제품 커맨드 아님). 관측할 PQC 협상을 만들기 위해서만 존재한다. 소스: [`pqc-echo/main.go`](pqc-echo/main.go).
