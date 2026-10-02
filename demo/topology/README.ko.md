[English](README.md) · 한국어

# demo/topology: 데모가 세우는 레거시 환경의 정의

**관측할 환경은 이 YAML 하나로 정의합니다.** 노드의 수와 종류, OpenSSL 버전, JCA provider, 네트워크
구간, 핸드셰이크 연결 간선을 선언하면 생성기가 compose, groups, profiles를 만들고 그 위에서
관측 → 인벤토리 → 전환물 생성을 돌립니다. 별도의 모드나 플래그는 없습니다. 데모에는 **이 경로 하나뿐**입니다.

> **여기에 나열하지 않는 것: 도구 쪽 컨테이너 둘.** `pqcota-ctl`(컨트롤러)과 `pqcota-demo-pg`(인벤토리 저장소)는
> 명세에 적지 않으며 생성기가 **자동으로 더합니다.** 이 YAML은 **pqcota가 보는 것**을 적는 자리이지
> pqcota 자체를 적는 자리가 아닙니다(그래서 그것들을 지워 데모를 망가뜨릴 여지도 없습니다).
> 둘 다 **모든 구간에 붙습니다.** 컨트롤러가 SSH로 모든 노드에 닿아야 하기 때문입니다. 실제 환경에서는
> 컨트롤러가 격리된 구간에 닿지 못할 수 있으며, 그 제약은 데모가 일부러 단순화한 것입니다.

> **호스트에 필요한 것은 여전히 Docker뿐입니다.** 생성기(`topogen`, Go)는 컨테이너 안에서 돕니다. 호스트에 Go가 없어도 됩니다.

## 빠른 시작

**아무것도 준비할 필요가 없습니다.** `topology.yaml`이 없으면 `up.sh`가 샘플을 복사해 그 구성으로 실행합니다.

```bash
./demo/scripts/up.sh      # (copies the sample if missing) → generate → build → start → keys and IP map
./demo/scripts/demo.sh    # access prep → discovery → inventory → provisioning
./demo/scripts/down.sh    # clean up (--rmi removes the images too)
```

**사용자의 환경에 맞추려면** 복사된 `demo/topology/topology.yaml`을 고치고 `up.sh`를 다시 실행하세요.

> **샘플(`topology.example.yaml`)만 추적합니다.** 실제로 쓰는 `topology.yaml`과 생성된 파일
> (`demo/.generated/`)은 git이 무시하므로 마음껏 고칠 수 있고, `git status`는 깨끗하게 남으며 `pull`이 충돌하지 않습니다.
> 기본 구성으로 돌아가려면 `topology.yaml`을 지우고 `up.sh`를 다시 실행하세요(샘플이 다시 복사됩니다).

### 기본 구성이 보여 주는 것

샘플은 결제 서비스 노드 셋을 세우고 여러 관측 축을 한꺼번에 드러냅니다.

| 노드 | 무엇인가 | 보여 주는 것 |
|---|---|---|
| `web-gw` | OpenSSL **3.x** 클라이언트(corp) | 최신 스택이자 트래픽 출발지(**SSH 등급은 이 노드의 클라이언트가 결정합니다**) |
| `pay-app` | Java + **실행 중에 등록된 BC**(corp) | 정적 스캔으로는 보이지 않고 **attach만 잡는** provider |
| `pay-db` | OpenSSL **1.1.1** 서버, 앱 둘(corp+db) | **레거시 = 양자에 취약** · **여러 앱에 걸친** 공유 `.so`(영향 범위) · 구간 둘에 걸침 |

연결 간선 넷은 **TLS와 SSH에서 각각 최신과 레거시를 구분합니다**.

| 연결 간선 | 등급 | 까닭 |
|---|---|---|
| `web-gw→pay-app` TLS | 🟢 X25519MLKEM768 | Go `crypto/tls` 하이브리드 |
| `web-gw→pay-db` TLS | 🔴 x25519 | **OpenSSL 1.1.1**에는 양자내성 그룹이 없음 |
| `web-gw→pay-app` SSH | 🟢 sntrup761 | 양쪽 모두 OpenSSH 9 이상(클라이언트가 기본으로 제시함) |
| `web-gw→pay-db` SSH | 🔴 curve25519 | 레거시 OS의 **OpenSSH 8.2**에는 양자내성 KEX가 없음 |

레거시 노드는 **TLS와 SSH 모두에서** 고전으로 남습니다. 등급은 도구가 관측한 것이며 지정한 것이 아닙니다.

### 고치는 것은 `topology.yaml` 하나뿐입니다(`hosts.csv`는 생성됩니다)

데모는 CSV를 여럿 보여 주지만 **사용자가 손대는 파일은 `topology.yaml` 하나뿐**입니다. 나머지는 모두 생성됩니다.

| 파일 | 무엇인가 | 누가 만드나 |
|---|---|---|
| **`topology.yaml`** | **무엇을 세울지**: 노드, 종류, 네트워크, 연결 간선 | **사용자(고침)** |
| `docker-compose.yml` · `groups.ini` · `profiles.csv` | 컨테이너, Ansible 그룹, CMDB 프로파일 | 생성기(`demo/.generated/`) |
| `hosts.csv` | **어디에 어떻게 연결할지**: node_id, IP, 계정, 키 | `up.sh`(IP는 컨테이너가 뜬 뒤에야 정해집니다) |

제품 모델에서 `hosts.csv`는 **사용자가 자기 호스트를 적는 파일**입니다. 데모에서는 Docker가 IP를 실행할 때 배정하므로 그 역할을 `up.sh`가 대신합니다. 데모가 사용자 대신 플레이북을 적용하는 것과 같은 방식입니다. 그래서 **토폴로지를 직접 정해도 `hosts.csv`를 스스로 쓸 일은 없습니다.**

## 명세(`topology.yaml`)

```yaml
networks: [dmz, app, db]        # bridge segments (imitating network separation). A single net if omitted

nodes:
  - id: web-gw                  # container name and node_id (lowercase, digits, -)
    name: Payments Web Gateway  # the name shown in the inventory view
    kind: openssl               # openssl | java  ← only what pqcota really observes
    role: client                # openssl: client | server
    openssl: { fork: openssl, version: "3.0" }   # when fork=openssl, version → base image
    networks: [dmz, app]        # may span several segments
    profile: { env: production, role: web, owner: Platform team }

  - id: pay-app
    name: Payments App
    kind: java
    jca: { providers: [BC] }    # providers registered at runtime (SUN and SunJCE are JDK defaults). Whether BC is present decides the grade
    networks: [app]

  - id: pay-db
    name: Payments DB
    kind: openssl
    role: server
    openssl: { fork: openssl, version: "1.1.1" }  # legacy = quantum-vulnerable
    apps: [payment-gw, api-gw]  # (openssl server) several apps load one libssl → the shared .so spans several apps

edges:                          # handshakes to observe → grade (🟢 PQC / 🔴 classical)
  - { from: web-gw, to: pay-app, proto: pqc, port: 8443 }
  - { from: web-gw, to: pay-db,  proto: ssl, port: 4433 }
```

### 조절할 수 있는 축

| 축 | 값 | 보여 주는 것 |
|---|---|---|
| **노드 종류** | `openssl` · `java` | pqcota가 관측하는 런타임만 |
| **openssl 포크** | `openssl` · `libressl` | 관측의 **포크 탐지**(같은 soname, 다른 포크) |
| **openssl 버전** | `1.1.1` · `3.0` · `3` | 베이스 이미지의 OpenSSL 버전(레거시 ↔ 최신, 버전 탐지) |
| **jca providers** | 예: `[BC]` · `[]` | BC가 있는지 여부 → JCA 등급 차이(attach가 동적 등록을 잡음) |
| **networks** | 임의의 구간 목록 | 브리지 여럿으로 하는 네트워크 분리. 노드 하나가 여러 구간에 걸침 |
| **apps**(openssl 서버) | 앱 이름 목록 | 여러 앱이 공유 `.so`를 로드 → **여러 앱에 걸침 · 영향 범위** |
| **edges** | `pqc` · `ssl` · `ssh` | 핸드셰이크 관측 → 🟢/🔴 등급 |

### 서버와 트래픽 규칙(자동으로 파생됩니다)

- `pqc` 연결 간선의 `to`인 노드 → 양자내성 TLS 서버(:8443)가 시작됩니다.
- `ssl` 연결 간선의 `to`이거나 `role: server`인 openssl 노드 → 고전 s_server가 시작됩니다.
- `from` 노드 → 그 연결 간선들을 향한 트래픽이 생성됩니다(관측 창을 채웁니다).

## 정직성의 경계 · 알려진 한계

- **관측할 수 없는 런타임은 추가할 수 없습니다.** `.NET`, `Go` 같은 종류는 거부됩니다. 있는 척하는 것은 없습니다.
- **s_server가 없는 포크**(BoringSSL, AWS-LC)는 데모 서버 노드로 시작할 수 없으므로 **분명한 오류와 함께 거부됩니다.**
- **openssl 발견 항목이 전혀 없는 토폴로지**는 전환물 생성 시연을 건너뜁니다(조치할 대상이 없습니다).
- **연결 간선 관측은 트래픽 출발 노드의 첫 구간(eth0)에서 이뤄집니다**(netcap은 인터페이스 하나를 봅니다). 구간을 여럿 써도 되지만
  **관측하려는 쌍이 출발 노드의 첫 구간에서 서로 닿아야** 잡힙니다. 출발 노드가 닿지 못하는 격리된 구간의 연결 간선은 캡처되지 않습니다(예제는 pay-db를 corp와 db에 걸치게 하고 corp에서 관측합니다).
- **LibreSSL은 자기 버전을 OpenSSL로 위장합니다**(`OPENSSL_VERSION_NUMBER` 호환 값). 그래서 `fork: libressl`
  노드는 실제로 LibreSSL을 로드하지만 수집기는 **OpenSSL 3.1.x로 보고합니다**. 같은 soname 문제에
  버전 위장이 겹친 정직한 한계입니다. 이 축은 지원되지만 예제에서는 뺐습니다.

생성된 파일은 `demo/.generated/`에 놓입니다(git은 무시하며, 리포의 빌드 산출물이 놓이는 단 한 곳입니다). 열어 보면 무엇이 만들어졌는지 그대로 보입니다.
