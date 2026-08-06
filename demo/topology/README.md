# demo/topology — 데모가 세우는 레거시 환경 정의

**관측 대상 환경이 이 YAML 하나로 정의된다.** 노드 수·종류·OpenSSL 버전·JCA provider·네트워크
세그먼트·핸드셰이크 엣지를 선언하면, 생성기가 compose·groups·profiles를 뽑아 그 위에서
디스커버리→인벤토리→프로비저닝을 돌린다. 별도 모드나 플래그는 없다 — 데모는 **이 경로 하나뿐**이다.

> **§ 표기**: 별도 언급이 없으면 [프로세스 규정서](../../docs/PQC플랫폼_단계별_프로세스규정.md)의 절 번호다.

> **여기 없는 것 — 도구 쪽 컨테이너 둘.** `pqcota-ctl`(컨트롤러)과 `pqcota-demo-pg`(인벤토리 저장소)는
> 명세에 안 적고 생성기가 **자동으로 넣는다.** 이 YAML은 **pqcota가 들여다보는 대상**을 적는 곳이지
> pqcota 자신을 적는 곳이 아니기 때문이다(지워서 데모를 망가뜨릴 여지도 없앤다).
> 둘은 **모든 세그먼트에 붙는다** — 컨트롤러가 어느 노드에든 SSH로 닿아야 하기 때문. 실제 환경에선
> 컨트롤러가 격리 세그먼트에 못 닿을 수 있는데, 그 제약은 데모가 일부러 단순화한 부분이다.

> **호스트 전제는 여전히 Docker뿐.** 생성기(`topogen`, Go)는 컨테이너 안에서 돈다 — 호스트에 Go도 설치할 필요 없다.

## 빠른 시작

**아무것도 준비할 필요 없다.** `up.sh`가 `topology.yaml`이 없으면 샘플을 복사해 그 구성으로 돌린다:

```bash
./demo/scripts/up.sh      # (없으면 샘플 복사) → 생성 → 빌드 → 기동 → 키·IP맵
./demo/scripts/demo.sh    # 접근준비→디스커버리→인벤토리→프로비저닝
./demo/scripts/down.sh    # 정리 (--rmi 로 이미지까지)
```

**자기 환경에 맞추려면** 복사된 `demo/topology/topology.yaml`을 고치고 다시 `up.sh` 하면 된다.

> **추적되는 건 샘플(`topology.example.yaml`)뿐이다.** 실제로 쓰이는 `topology.yaml`·생성물
> (`demo/.generated/`)은 gitignore라, 마음껏 고쳐도 `git status`가 깨끗하고 `pull`이 충돌하지 않는다.
> 기본 구성으로 되돌리려면 `topology.yaml`을 지우고 `up.sh`를 다시 돌리면 된다(샘플이 다시 복사된다).

### 기본 구성이 보여주는 것

샘플은 결제 서비스 3노드를 세우고, 한 번에 여러 관측 축을 드러낸다:

| 노드 | 무엇 | 무엇을 보이나 |
|---|---|---|
| `web-gw` | OpenSSL **3.x** 클라이언트 (corp) | 현대 스택 · 트래픽 소스(**SSH 등급은 이 노드의 클라이언트가 가른다**) |
| `pay-app` | Java + **BC 런타임 등록** (corp) | 정적 스캔으론 안 보이고 **attach로만** 잡히는 provider |
| `pay-db` | OpenSSL **1.1.1** 서버, 앱 2개 (corp+db) | **레거시=양자취약** · 공유 `.so` **다중 귀속**(영향 반경) · 세그먼트 2개에 걸침 |

엣지 4개가 **TLS·SSH 각각에서 현대↔레거시**를 가른다:

| 엣지 | 등급 | 왜 |
|---|---|---|
| `web-gw→pay-app` TLS | 🟢 X25519MLKEM768 | Go `crypto/tls` 하이브리드 |
| `web-gw→pay-db` TLS | 🔴 x25519 | OpenSSL **1.1.1**엔 PQC 그룹이 없다 |
| `web-gw→pay-app` SSH | 🟢 sntrup761 | 양쪽 다 OpenSSH 9+ (클라이언트가 기본 제안) |
| `web-gw→pay-db` SSH | 🔴 curve25519 | 레거시 OS의 **OpenSSH 8.2**엔 PQC KEX가 없다 |

레거시 노드는 **TLS도 SSH도** 고전으로 남는다 — 등급은 도구가 관측한 대로 매긴 것이지 지정한 게 아니다.

### 편집하는 건 `topology.yaml` 하나다 (`hosts.csv`는 생성물)

데모에 CSV가 여럿 보이지만 **사용자가 손대는 파일은 `topology.yaml`뿐**이다. 나머지는 전부 생성된다:

| 파일 | 무엇 | 누가 만드나 |
|---|---|---|
| **`topology.yaml`** | **무엇을 세울지** — 노드·종류·네트워크·엣지 | **사용자(편집)** |
| `docker-compose.yml` · `groups.ini` · `profiles.csv` | 컨테이너·Ansible 그룹·CMDB 프로필 | 생성기(`demo/.generated/`) |
| `hosts.csv` | **어디에 어떻게 접속할지** — node_id·IP·계정·키 | `up.sh` (컨테이너가 떠야 IP가 정해지므로) |

`hosts.csv`는 제품 모델에선 **사용자가 자기 호스트를 적는 파일**이다(§4A.3). 데모에선 IP를 Docker가 런타임에 배정하니 `up.sh`가 그 역할을 대신 수행한다 — 플레이북 적용을 데모가 대행하는 것과 같은 구도다. 그래서 **커스텀 토폴로지에서도 `hosts.csv`를 직접 쓸 일은 없다.**

## 명세 (`topology.yaml`)

```yaml
networks: [dmz, app, db]        # 브리지 세그먼트(망 분리 흉내). 생략 시 단일 net

nodes:
  - id: web-gw                  # 컨테이너명·node_id (소문자/숫자/-)
    name: 결제 웹 게이트웨이       # 인벤토리 뷰에 뜨는 이름
    kind: openssl               # openssl | java  ← pqcota가 실제로 관측하는 것만
    role: client                # openssl: client | server
    openssl: { fork: openssl, version: "3.0" }   # fork=openssl일 때 version→base 이미지
    networks: [dmz, app]        # 여러 세그먼트에 걸칠 수 있다
    profile: { env: production, role: web, owner: 플랫폼팀 }

  - id: pay-app
    name: 결제 앱
    kind: java
    jca: { providers: [BC] }    # 런타임 등록 provider(SUN·SunJCE는 JDK 기본). BC 유무가 posture를 가른다
    networks: [app]

  - id: pay-db
    name: 결제 DB
    kind: openssl
    role: server
    openssl: { fork: openssl, version: "1.1.1" }  # 레거시 = 양자취약
    apps: [payment-gw, api-gw]  # (openssl server) 여러 앱이 한 libssl 로드 → 공유 .so 다중 귀속

edges:                          # 관측할 핸드셰이크 → 등급(🟢 PQC / 🔴 고전)
  - { from: web-gw, to: pay-app, proto: pqc, port: 8443 }
  - { from: web-gw, to: pay-db,  proto: ssl, port: 4433 }
```

### 조절 가능한 축

| 축 | 값 | 무엇을 보이나 |
|---|---|---|
| **노드 종류** | `openssl` · `java` | pqcota가 관측하는 런타임만 |
| **openssl fork** | `openssl` · `libressl` | discovery의 **fork 판별**(§1.2, 같은 soname 다른 fork) |
| **openssl version** | `1.1.1` · `3.0` · `3` | base 이미지의 OpenSSL 버전(레거시↔현대·버전 탐지) |
| **jca providers** | 예: `[BC]` · `[]` | BC 유무 → JCA posture 차이(attach로 동적 등록 포착) |
| **networks** | 임의 세그먼트 목록 | 다중 브리지로 망 분리, 노드가 여러 세그먼트에 걸침 |
| **apps** (openssl server) | 앱 이름 목록 | 공유 `.so`를 여러 앱이 로드 → **다중 귀속·영향 반경** |
| **edges** | `pqc` · `ssl` · `ssh` | 핸드셰이크 관측 → 🟢/🔴 등급 |

### 서버/트래픽 규칙 (자동 유도)

- `to`가 `pqc` 엣지인 노드 → PQC TLS 서버(:8443)를 띄운다.
- `to`가 `ssl` 엣지이거나 `role: server`인 openssl 노드 → 고전 s_server를 띄운다.
- `from` 노드 → 그 엣지들을 트래픽으로 생성(관측 창을 채운다).

## 정직성 경계 · 알려진 한계

- **관측 못 하는 런타임은 못 넣는다.** `.NET`·`Go` 같은 종류는 거부된다 — 있는 척하지 않는다(§2.6).
- **s_server가 없는 fork**(BoringSSL·AWS-LC)는 데모 서버 노드로 못 띄우므로 **명확한 오류로 거부**한다.
- **openssl finding이 하나도 없는 토폴로지**면 프로비저닝 시연은 정직히 생략된다(대상이 없으니).
- **엣지 관측은 트래픽 소스 노드의 첫 세그먼트(eth0)에서** 한다(netcap이 한 인터페이스를 본다). 다중
  세그먼트를 써도 되지만, **관측하려는 쌍은 소스 노드의 첫 세그먼트에서 서로 도달**해야 잡힌다 — 소스가
  안 닿는 격리 세그먼트의 엣지는 캡처되지 않는다(예시는 pay-db를 corp+db 양쪽에 걸쳐 corp에서 관측한다).
- **LibreSSL은 버전을 OpenSSL로 위장한다**(`OPENSSL_VERSION_NUMBER` 호환값). 그래서 `fork: libressl`
  노드는 실제로 LibreSSL을 로드하지만 collector는 **OpenSSL 3.1.x로 보고**한다 — 같은 soname 문제(§1.2)가
  버전 위장까지 겹친 정직한 한계다. 이 축은 지원하되 예시엔 두지 않았다.

생성물은 `demo/.generated/`(gitignore, 리포 산출물 단일 위치)에 떨어진다 — 열어보면 무엇이 만들어졌는지 그대로 보인다.
