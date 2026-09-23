한국어 · [English](design.en.md)

# 데모 설계: 무엇이 어디서 돌고, 왜 그렇게 만들었나

**문서 성격**: `demo/`가 어떻게 구성됐고 왜 그런지를 적는다. 돌리는 방법은 [README](README.md),
검증하는 케이스는 [데모가 검증하는 것](integration-verification.md), 환경 정의는
[topology/README](topology/README.md)에 있다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

## 1. 원칙

- **Docker만 있으면 된다.** 빌드도 관측도 컨테이너 안에서 하고, 리포에 생기는 것은 `demo/.generated/` 하나다(gitignore).
- **접근 준비(①)는 이 데모의 구성 때문에 필요한 것이지, 관측의 전제가 아니다.** 데모는 컨트롤러에서 **여러 노드에 SSH로** 스캐너를 돌리므로 접속 인벤토리가 필요하다. 한 노드를 그 자리에서 훑거나 결과 파일을 모아 적재하는 경로는 ①이 아예 없어도 된다. 무엇이 필수·선택인지는 [discovery/cmd README](../discovery/cmd/README.md#필수인가-아니다-원격으로-여러-노드를-훑을-때만-필요하다).
- **이 리포(Apache-2.0)만으로 완결된다.** 프로비저닝은 **생성·영속**까지이고, 데모는 생성한 플레이북을 **실제로 적용하고 되돌린다**. 선언 대조(`UNDECLARED`/`UNOBSERVED`)·리뷰-확정 거버넌스·동적 프로비저닝은 이 리포가 하지 않아 데모에도 없다. 스냅샷 간 변화 diff는 관측 사실이라 이 리포에 있다(아키텍처 §6 기준).
- **생성에서 멈추지 않고 적용까지 한다.** 생성만 하고 안 돌리면 **문법은 맞는데 실제로는 깨지는** 플레이북이 통과한다. 실제로 그런 결함이 있었다(config 디렉터리를 안 만들어 `copy`가 실패). `6/6`이 그 부류를 상시로 잡는다.
- **provider 모듈은 도구가 주지 않는다. 데모는 빈 파일을 쓴다.** 실제 암호 기능은 없다. 실물 모듈은 사용자가 빌드하거나 벤더에서 받아 반입한다([커스텀 provider 절차](../provisioning/design.md#6b-커스텀-provider)). 빈 파일을 쓰는 것은 목적이 **암호 기능 시연이 아니라 배포·가역성 시연**이고, "Docker만 있으면 된다"는 전제를 지키기 위해서다. 빈 파일로 못 보이는 마지막 한 칸은 [선택 단계](#54-선택-단계-실물-provider)가 맡는다.

## 2. 부품

| 폴더/파일 | 무엇 |
|---|---|
| [`scripts/`](scripts) | 사용자가 실행하는 셋: `up.sh`(설치) · `demo.sh`(수행) · `down.sh`(제거) |
| [`scripts/ansible/`](scripts/ansible) | demo.sh가 구동하는 디스커버리 오케스트레이션: SSH 인벤토리·플레이북(`discover.yml`) |
| [`scripts/internal/`](scripts/internal) | 컨테이너 안에서 도는 헬퍼(노드 부팅·서비스 기동/정지·트래픽 생성·관측). `ssl-apps.sh`는 L3 훅이 가리키는 서비스 관리 지점 |
| [`workloads/`](workloads) | 노드에 배포되는 데모 크립토 워크로드(스캔·관측 대상): `CryptoApp.java`(JCA/BouncyCastle) · `pqc-echo/`(PQC TLS 트래픽 생성기, Go) |
| [`topology/`](topology) | 데모 환경 정의: `topology.yaml`(첫 실행 시 샘플 복사)과 생성기 `topo-gen` |
| [`expected-output/`](expected-output) | 실행 전 예상 결과 미리보기(콘솔·토폴로지 SVG) |
| [`integration-verification.md`](integration-verification.md) | 이 데모가 검증하는 통합 케이스와 커버하지 않는 것 |
| [`recording/`](recording/README.md) | 데모를 스크린캐스트로 만들 때의 절차와 편집 틀. 제품을 쓰는 데는 필요 없고, 발표·출품용 영상을 다시 만들 때 쓴다 |
| `Dockerfile` | 컨테이너 빌드 정의(노드 종류별 스테이지). 스크립트가 호출한다 |

## 3. 빌드: ctl 머신에서

`pqcota-ctl`이 곧 빌드 머신이다. `up.sh`의 `3/6`이 그 컨테이너 안에서 컴파일하고, 같은 머신에서
디스커버리·인벤토리·프로비저닝을 돌린다.

| 무엇을 | 어떤 옵션으로 | 어디에 |
|---|---|---|
| proto 생성 코드 | `make generate` (buf) | `/src/gen/` |
| 중앙 CLI (`ingest`·`inventory`·`provision`…) | `CGO_ENABLED=0 go build` | `/usr/local/bin/` (ctl에서 실행) |
| collector (`nodescan`·`netcap`·`jvmscan`) | `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build` | `/work/dist/linux-<arch>/` (노드로 반입) |
| JVM attach 사이드카 | `make build-jar` (javac + jar) | `/work/dist/collector.jar` |

실행하면 그대로 찍힌다:

```console
▶ 3/6 building the repo — the source is compiled **on the ctl machine (pqcota-ctl)**
     [ctl] Ubuntu 24.04.4 LTS · x86_64 · go1.26.4
     [ctl] make generate …  go build -o /usr/local/bin/ …  GOARCH=amd64 go build -o dist/linux-amd64/ …
```

**사용자 환경도 같다.** 빌드 머신은 리눅스면 되고(Go 1.26.4+·buf·JDK 11+는 선택), collector만 **노드 arch에
맞춰** 만들면 된다. `CGO_ENABLED=0` 정적 링크라 배포판·libc를 가리지 않는다. 이 데모에서도
Ubuntu 24.04에서 빌드한 바이너리가 20.04 노드에서 그대로 돈다.

이미지 빌드(`up.sh`의 `1/6`)가 만드는 것은 OS·툴체인과 관측 **대상** 워크로드뿐이다(`pqc-echo` = 현실에선
사용자의 앱, `topogen` = 컨테이너보다 먼저 필요). **pqcota 소프트웨어는 이미지에 넣지 않는다.**

## 4. 실행 시점: 도는 컨테이너 (기본 토폴로지)

[`topology/topology.yaml`](topology/README.md)이 정의한다. 아래는 기본값이고 **띄운 상태에서 실측한 값**이다.

| 컨테이너 | 베이스 OS · arch | 세그먼트 | 상시 프로세스 · 리스닝 | 실제 환경에서는 |
|---|---|---|---|---|
| **pqcota-ctl** | Ubuntu 24.04 · 호스트 arch | corp+db | `sleep infinity` · 없음 | 리포를 빌드하고 도구를 돌리는 머신 |
| **pqcota-demo-pg** | Debian 13 (postgres:16) · 호스트 arch | corp+db | `postgres` · :5432 | 중앙 인벤토리 DB (단일 호스트 경로엔 불필요) |
| **web-gw** | Ubuntu 24.04, OpenSSL **3.x** · 호스트 arch | corp | `sshd` · :22 | 관측 대상: TLS/SSH 클라이언트 쪽 |
| **pay-app** | Ubuntu 26.04, temurin 21 · 호스트 arch | corp | `sshd`·`java`·`pqc-echo` · :22 :8443 | 관측 대상: JVM 자산 |
| **pay-db** | Ubuntu 20.04, OpenSSL **1.1.1** · 호스트 arch | corp+db | `sshd`·`payment-gw`·`api-gw` · :22 :4433 :4434 | 관측 대상: 레거시 자산 |

노드 OS는 `topology.yaml`의 `version`·`fork`가 고른다(3.x→24.04, 3.0→22.04, 1.1.1→20.04,
libressl→alpine). arch는 전부 호스트와 같다. **컨트롤러만 두 세그먼트에 붙는다**(모든 노드에
SSH). IP는 매번 새로 배정되므로 세그먼트 이름으로만 참조한다.

**노드에 collector는 없다.** 워크로드와 데모 헬퍼뿐이고, 디스커버리 후 잔재도 0이다:

```console
$ docker exec pay-db ls /usr/local/bin
node-entrypoint.sh  pqc-echo  pqcota-gen-traffic.sh  pqcota-observe.sh  ssl-apps.sh
```

`topo-gen`이 `up.sh`의 `0/6`에 `--rm`으로 잠깐 더 돈다(compose·groups.ini·SVG 생성 후 소멸).

## 5. 단계별 동작

`demo.sh`가 찍는 `▶ N/6`의 순서다. 사용자가 보게 되는 것은 [README](README.md#무엇이-보이나)에 있고,
여기는 그 뒤에서 무엇이 어떻게 도는지다.

### 5.1 디스커버리 (Ansible/SSH, 모두 실물)
1. **OpenSSL 자산**은 `pqcota-nodescan`이 낸다. `/proc` 스캔으로 로드된 libssl/libcrypto를 본다.
2. **JCA provider 체인**은 `pqcota-jvmscan`이 낸다. **정찰→attach** 순서다. `/proc`로 실행 중 JVM(pay-app의 CryptoApp)을 찾아 그 PID에 attach해 `Security.getProviders()` 실체를 본다. CryptoApp이 **런타임에 `addProvider`한 BouncyCastle**까지 잡는다. java.security엔 정적 등록이 없어 **정적 스캔으론 관측되지 않는** 것이다(openssl의 `/proc` 스캔과 대칭, `detection=runtime-introspection`). attach가 안 되면 정적 프로브로 내려가되, 관측하지 못한 것은 갭으로 남긴다.
3. **통신 엣지**는 `pqcota-netcap`이 낸다. AF_PACKET(`CAP_NET_RAW`)으로 TLS/SSH 핸드셰이크를 복호화 없이 관측한다.

`pqcota-discover-view`(OSS)가 결과를 모아 **발견 자산 + 관측 엣지 등급**을 낸다:
- 🟢 **PQC/하이브리드**(`X25519MLKEM768`, `sntrup761x25519`) · 🔴 **고전=양자취약**(`x25519`, `ECDHE`) · ⚪ **불명**
- 예: `web-gw → pay-app` 🟢 MLKEM · `web-gw → pay-db` 🔴 고전 · SSH도 같은 갈림(`→pay-app` 🟢 sntrup761 · `→pay-db` 🔴이고, 레거시 OS의 OpenSSH엔 PQC KEX가 없다)

### 5.2 중앙 인벤토리 (엔드포인트·프로필·앱 표시·이력·변화)
`pqcota-ingest`가 회수 결과를 append-only 히스토리에 적재하고, `pqcota-inventory`가 조회한다:
- **▸ 머신 헤더**: `pqcota-hosts`가 upsert한 **엔드포인트**(이름·ip:port, 비밀 없음) + **프로필**(display_name·env·role·owner, CMDB 선언 레인).
- **같은 장비가 여러 이름으로 등재되면 고지한다**: 적재 결과에 `⚠ duplicate: physical machine … → [pay-db web-gw]`가 나오는데 이것은 오류가 아니다. 플랫폼은 지문이 같다는 사실을 감추지 않고 그대로 알린다(TK-MACHINE). 실운용에서 한 장비를 여러 이름으로 등재했을 때 보게 되는 표시가 이것이다.<br>**이 줄은 호스트를 탄다.** 지문은 `cloud-instance-id` → `/etc/machine-id` → DMI `product_uuid` → 호스트명 순으로 고른다(`pkg/kernel/machineid`). 컨테이너의 `/etc/machine-id`는 비어 있으므로 갈림은 DMI에서 난다. **리눅스 호스트**에서는 컨테이너 안에서도 호스트의 `/sys/class/dmi/id/product_uuid`가 보여 노드 셋의 지문이 같아지고, 그래서 이 줄이 나온다. **macOS의 Docker Desktop**에서는 그 VM에 DMI가 없어 빈 값이 되고, 지문이 호스트명(= 컨테이너 이름)으로 내려가 노드마다 달라지므로 이 줄이 나오지 않는다. 지어내지 않고 읽은 대로 낸 결과다(§2.5).
- **@앱 표시**: 각 크립토 자산이 어느 앱 것인지(`app_keys`). pay-db의 공유 `libssl.so.1.1`은 `payment-gw`·`api-gw` **둘 다**에 걸린다(그 .so 교체는 두 앱 모두 영향).
- **이력**: 같은 회수 결과를 한 번 더 적재한다(실운용의 "다음 회차 스캔"에 해당). 내용이 같으니 **새 스냅샷이 생기지 않고** 관측 기록만 하나 는다. `-history`가 그것을 변화 지점 하나에 관측 둘로 보이고, `-snapshot`이 그 스냅샷의 자산과 관측 엣지를 편다. 저장은 변화 횟수만큼만 자라되 "매번 스캔했다"는 증거는 보존된다. 도구는 없는 변화를 지어내지 않는다.
- **변화**: `-diff`가 견주는 두 점은 **자산 스코프 전후**다. 스코프가 잡음을 빼면 내용이 바뀌어 새 스냅샷이 생기고, 그 둘을 견주므로 기본 토폴로지에서는 `removed` 한 건이 나온다. 여기서 `removed`는 **관리 대상에서 뺐다**는 뜻이지 자산이 사라졌다는 뜻이 아니다. 실제로 버전이 바뀌는 경우라면 finding id가 유지되어 **같은 자산의 `changed`**로 잡힌다.
- **자산 스코프**: 노드는 등재됐어도 그 안의 자산 전부가 관리 대상은 아니다. `sshd`·패키지 python 런타임 같은 잡음을 규칙으로 빼면 **앱이 실제로 쓰는 자산만** 남는다. 뺀 건수는 반드시 고지된다. **제외는 부재가 아니다**(§2.6).
- **보존 정책**: `pqcota-prune`을 dry-run으로 돌려 **노드별 최신 스냅샷은 어떤 정책으로도 지우지 않음**을 보인다. 파괴적 동작이라 조회 커맨드와 분리했고, 실제 삭제는 `-apply`로만 한다.

### 5.3 프로비저닝 (생성 → 적용 → 되돌림)
대상 노드는 인벤토리에서 openssl finding이 있는 노드 가운데 **공유 .so에 여러 앱이 걸린 쪽**을 우선해 고른다
(영향 범위가 가장 또렷한 케이스). 기본 토폴로지에서는 pay-db다. 그런 노드가 없으면 이 단계는 생략하고
그 사실을 알린다(§2.5). 고른 finding에 **확정 계획(FINALIZED)**을 만들어 `pqcota-provision`을 돌린다:
- **§3.7 게이트**: FINALIZED 아니면 거부. **L2 플레이북 생성**(모듈 스테이지 + config 조각).
- **before 캡처 + 롤백 레코드 영속**: 조치 전 암호 상태(모듈·버전)와 **영향 앱(공유 .so면 다중)**을 append-only로 남긴다.

이어서 생성물을 **실제로 적용한다**. 도는지까지 봐야 "생성했다"가 말이 된다:
- **적용**: 생성된 플레이북을 `ansible-playbook`으로 대상 노드에 실행. 모듈 sha256 게이트도 함께 통과시킨다.
- **확인**: 타깃에 `/opt/pqcota/oqsprovider.so`와 `/etc/pqcota/openssl-pqc.cnf`가 놓였고, config가 **그 배치 경로를 참조**하는지(`module = /opt/pqcota/oqsprovider.so`).
- **되돌림**: `--rollback` 플레이북으로 제거. 원본 설정을 덮은 적이 없으니 **제거만으로 이전 상태**가 되고, 두 파일이 사라지는 것까지 확인한다.

L2와 L3는 이렇게 다르다:
- **L2는 조각을 놓기만 한다**. 참조되게 만들지 않으므로 모든 산출물이 완전히 가역이다.
- **L3는 여기에 활성화·재시작을 더한다.** 명령은 계획의 `activation` 훅에 사용자가 적은 것을 쓴다. 환경마다 활성화 지점이 다르므로 도구가 추측하지 않는다. 데모 노드는 `ssl-apps.sh`로 서비스를 관리하므로 훅이 그것을 가리킨다(현실의 systemd unit·사내 기동 스크립트에 해당).
- 데모의 L3가 보이는 것은 **훅 순서·활성화 지점 연결·재시작·가역성**이다. 레거시 노드의 OpenSSL은 이 조각의 PQC 그룹을 모르므로 **능력이 바뀌었다고 말하지 않는다**. 그 노드의 실제 조치는 fork 교체이고, 그것은 config로 배포되지 않는다고 플레이북 주석에 적혀 있다.

### 5.4 선택 단계: 실물 provider
`DEMO_REAL_PROVIDER=1`이면 실물 oqsprovider(liboqs + oqs-provider)를 노드와 같은 베이스에서 빌드해
배치·활성화하고, `openssl list`로 능력이 실제로 생겼는지를 잰다(ML-KEM KEM 0개 → 14개, 되돌리면 다시 0개).
기본이 꺼져 있는 것은 첫 빌드가 수 분 걸리기 때문이다.

대상은 `6/6`의 노드가 아니다. provider를 넣을 자리가 있는 노드는 **OpenSSL 3.0–3.4**뿐이다. 1.1.1엔
provider라는 개념이 없고, 3.5+는 ML-KEM이 네이티브라 조치가 provider 배치가 아니라 CONFIG_ONLY다
(`pkg/provisioning/openssl.go`). 인벤토리에서 그 대역을 관측한 노드를 고르고, 없으면 생략한다(§2.5).

**재관측에서 인벤토리는 그대로다.** 데모는 이것을 숨기지 않고 이유까지 함께 낸다: OpenSSL은 provider 층을
관측하는 경로가 아직 없고(`/proc/maps`의 libssl·libcrypto와 ELF 문자열까지다. JCA는 attach로 provider 체인을
보지만 OpenSSL은 관측하지 못한다), 핸드셰이크도 협상은 양쪽이 알아야 하는데 이 토폴로지의 상대는 1.1.1이다.
설계 검토는 [디스커버리 설계 §2.1](../discovery/design.md#21-openssl-collector-go-sd-1-sd-3)에 있다.
끝나면 L3→L2 순서로 되돌려 노드를 원래대로 둔다.

## 6. 접근 비밀 경계 (§1.5)
접속 키·계정은 **사용자 hosts.csv → 런타임 전용 `targets.ini`(소유자 전용 `0600`)**에만 실린다. pqcota
인벤토리(Postgres)엔 **엔드포인트(node_id·이름·ip·port)만** upsert되고 비밀은 적재하지 않는다(`0/6`이
`pqcota_endpoint`에 비밀 0건임을 확인한다). 접속 키(`/work/id_demo`)와 `targets.ini`는 **컨트롤러 안에만**
있고 컨테이너와 함께 사라진다.
