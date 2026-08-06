# pqcota 데모 (OSS) — 접근준비 → 디스커버리 → 인벤토리 → 프로비저닝

**Docker만 있으면** 한 줄로 설치·수행·제거되는 종단 데모입니다. 단일 가상 네트워크에 묶인
노드에서 pqcota가 **① 사용자 hosts 파일로 접근 준비(비밀 미영속) → ② Ansible/SSH로 OpenSSL·Java(JCA)·
통신 핸드셰이크 디스커버리 → ③ 중앙 인벤토리(엔드포인트·프로필·앱 귀속·이력·자산 스코프) →
④ 프로비저닝(L2 플레이북·롤백 레코드 **생성** → 적용 → 되돌림)** 까지 전 범위를 보여줍니다.

> **§ 표기**: 별도 언급이 없으면 [프로세스 규정서](../docs/PQC플랫폼_단계별_프로세스규정.md)의 절 번호다.

> **①은 이 데모의 구성 때문에 필요한 것이지, 관측의 전제가 아닙니다.** 데모는 컨트롤러에서 **여러 노드에
> SSH로** 스캐너를 돌리므로 접속 인벤토리가 필요합니다. 한 노드를 그 자리에서 훑거나 결과 파일을 모아
> 적재하는 경로는 ①이 아예 없어도 됩니다 — 무엇이 필수·선택인지는 [discovery/cmd README](../discovery/cmd/README.md#필수인가--아니다-원격으로-여러-노드를-훑을-때만-필요하다).

> **경계**: 이 데모는 이 리포(Apache-2.0)만으로 완결됩니다. 프로비저닝은 **생성·영속**까지이고,
> 데모는 생성한 플레이북을 **실제로 적용하고 되돌립니다** — 생성만 확인하면 깨끗한 노드에서 깨지는 플레이북도 통과합니다(실제로 그런 결함이 있었습니다).
> **선언 대조(shadow/미관측)·리뷰-확정 거버넌스·동적 프로비저닝**은 이 리포가 하지 않아 데모에도 없습니다.
> (스냅샷 간 변화 diff는 관측 사실이라 이 리포에 있습니다 — 아키텍처 §6 기준.)

📊 **실행 전 예상 결과**: [`expected-output/`](expected-output/) — 콘솔 출력·토폴로지 SVG 샘플과
실제 실행 시 달라질 수 있는 점(엣지 캡처 타이밍·base 이미지 버전)의 설명.

## 요구 사항
- Docker (Compose v2) · 인터넷(최초 이미지 빌드) · 사용자 `docker` 그룹 (루트/KVM 불필요)

리포는 `pqcota-ctl` 컨테이너 안에서 빌드됩니다([아래](#리포는-어디서-빌드되나--ctl-머신에서)).
빌드 대상은 **지금 체크아웃된 소스**이며 커밋하지 않은 수정도 포함됩니다.

## 빠른 시작
```bash
./demo/scripts/up.sh      # 이미지 → 컨테이너 → **ctl에서 리포 빌드** → SSH 키 → hosts.csv
./demo/scripts/demo.sh    # 접근준비 → 디스커버리 → 인벤토리(메타·앱귀속·이력·스코프) → 프로비저닝(생성·적용·되돌림)
./demo/scripts/down.sh    # 정리 (--rmi 로 이미지까지)
```

`./demo/scripts/demo.sh --help` 이 조정 지점을 전부 적는다 — 그중 하나가 아래 [선택 단계](#선택-단계--실물-provider로-마지막-한-칸까지-demo_real_provider1)다.

> **데모 환경은 `demo/topology/topology.yaml` 하나가 정의합니다.** 첫 실행 때 샘플이 자동 복사되고
> (git 무시), 그 파일을 고치면 노드 수·종류·OpenSSL 버전·JCA provider·네트워크 세그먼트·핸드셰이크가
> 그대로 반영됩니다 — 자기 환경에 가깝게 바꿔 같은 종단을 돌릴 수 있습니다.
> 상세: **[topology/README](topology/README.md)**.

## 폴더 구성
| 폴더/파일 | 의미 | 직접 실행? |
|---|---|---|
| [`scripts/`](scripts) | **사용자가 실행**하는 것 — `up.sh`(설치) · `demo.sh`(수행) · `down.sh`(제거) | ✅ 이 3개 |
| [`scripts/ansible/`](scripts/ansible) | demo.sh가 구동하는 디스커버리 **오케스트레이션** — SSH 인벤토리·플레이북(`discover.yml`) | ❌ |
| [`통합_검증.md`](통합_검증.md) | **이 데모가 검증하는 통합 케이스**와 커버하지 않는 것 |
| [`scripts/internal/`](scripts/internal) | 컨테이너 **안에서** 도는 헬퍼(노드 부팅·서비스 기동/정지·트래픽 생성·관측). `ssl-apps.sh`는 L3 훅이 가리키는 서비스 관리 지점 | ❌ |
| [`workloads/`](workloads) | 노드에 배포되는 **데모 크립토 워크로드**(스캔·관측 대상): `CryptoApp.java`(JCA/BouncyCastle) · `pqc-echo/`(PQC TLS 트래픽 생성기, Go) | ❌ |
| [`expected-output/`](expected-output) | 실행 전 **예상 결과** 미리보기(콘솔·토폴로지 SVG) | ❌ |
| [`topology/`](topology) | **데모 환경 정의** — `topology.yaml`(첫 실행 시 샘플 복사)과 생성기 | ✏️ 이 파일을 고쳐 구성 변경 |
| `Dockerfile` | 컨테이너 **빌드** 정의(노드 종류별 스테이지) | ❌(스크립트가 호출) |

> 처음이면 **`scripts/`의 up → demo → down** 세 개만 보면 됩니다. 나머지는 그 뒤에서 도는 부품입니다.

## 산출물은 어디에 생기나

**대부분은 컨테이너 안**에 생기고, 리포에 떨어지는 건 **`demo/.generated/` 한 곳뿐**입니다(gitignore). `down.sh`가 그 폴더를 통째로 지우고, 컨테이너 것은 컨테이너와 함께 사라집니다.

| 어디 | 무엇 | 정리 |
|---|---|---|
| **리포** `demo/.generated/` | 리포에 생기는 **전부** — `topology.svg`·`topology.dot`(관측 토폴로지 그림) + 토폴로지에서 생성된 `docker-compose.yml`·`groups.ini`·`profiles.csv`·`manifest.env` | `down.sh`가 폴더째 삭제 (gitignore) |
| **컨트롤러** `pqcota-ctl:/work/` | 빌드 산출 `dist/linux-<arch>/`(collector 3종)·`dist/collector.jar` · 회수된 관측 결과 `results/*.json` · 접속 `hosts.csv`→`ansible/targets.ini`(0600·비밀) · `nodes.json` · `profiles.csv` · 확정 계획 `plan.json` · 생성된 `ansible/playbook{,-l3}.yml`·`rollback{,-l3}.yml` · 모듈 `ansible/files/oqsprovider.so`(빈 파일) | 컨테이너와 함께 소멸 |
| **Postgres** `pqcota-demo-pg` | 중앙 인벤토리 — 스냅샷·관측 기록·엔드포인트·프로필·프로비저닝 레코드 | `down.sh`가 볼륨까지 삭제(`-v`) |
| **타깃 노드** | 적용 단계에서 `/opt/pqcota/oqsprovider.so` · `/etc/pqcota/openssl-pqc.cnf`, L3면 활성화 지점 `/etc/pqcota/service.env` — **되돌림 단계에서 제거**되어 원상복귀 | 데모가 스스로 롤백 |

들여다보려면(데모 종료 전):

```bash
docker exec pqcota-ctl ls -R /work           # 컨트롤러 산출물 전부
docker exec pqcota-ctl cat /work/ansible/provision.yml   # 생성된 플레이북
docker exec pqcota-demo-pg psql -U postgres -d pqcota -c '\dt'  # 인벤토리 테이블
```

> **호스트 파일시스템은 거의 안 건드립니다** — 리포에 남는 건 위 그림·생성물뿐이고, 그마저 gitignore입니다.
> 접속 키(`/work/id_demo`)와 `targets.ini`는 **컨트롤러 안에만** 있고 인벤토리에 적재되지 않습니다(§4A.3).

## 리포는 어디서 빌드되나 — **ctl 머신에서**

`pqcota-ctl`이 곧 빌드 머신입니다. `up.sh` 3단계가 그 컨테이너 안에서 컴파일하고, 같은 머신에서
디스커버리·인벤토리·프로비저닝을 돌립니다.

| 무엇을 | 어떤 옵션으로 | 어디에 |
|---|---|---|
| proto 생성 코드 | `make generate` (buf) | `/src/gen/` |
| 중앙 CLI (`ingest`·`inventory`·`provision`…) | `CGO_ENABLED=0 go build` | `/usr/local/bin/` (ctl에서 실행) |
| collector (`nodescan`·`netcap`·`jvmscan`) | `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build` | `/work/dist/linux-<arch>/` (노드로 반입) |
| JVM attach 사이드카 | `make build-jar` (javac + jar) | `/work/dist/collector.jar` |

실행하면 그대로 찍힙니다:

```console
▶ 3/6 리포 빌드 — **ctl 머신(pqcota-ctl)에서** 소스를 컴파일합니다
     [ctl] Ubuntu 24.04.4 LTS · x86_64 · go1.26.4
     [ctl] make generate …  go build -o /usr/local/bin/ …  GOARCH=amd64 go build -o dist/linux-amd64/ …
```

**당신 환경도 같습니다.** 빌드 머신은 리눅스면 되고(Go 1.26.4+·buf·JDK 11+는 선택), collector만 **노드 arch에
맞춰** 만들면 됩니다. `CGO_ENABLED=0` 정적 링크라 배포판·libc를 가리지 않습니다 — 이 데모에서도
Ubuntu 24.04에서 빌드한 바이너리가 20.04 노드에서 그대로 돕니다.

이미지 빌드(1단계)가 만드는 것은 OS·툴체인과 관측 **대상** 워크로드뿐입니다(`pqc-echo` = 현실에선
사용자의 앱, `topogen` = 컨테이너보다 먼저 필요). **pqcota 소프트웨어는 굽지 않습니다.**

## 실행 시점 — 도는 컨테이너 (기본 토폴로지)

[`topology/topology.yaml`](topology/README.md)이 정의합니다. 아래는 기본값이고 **띄운 상태에서 실측한 값**입니다.

| 컨테이너 | 베이스 OS · arch | 세그먼트 | 상시 프로세스 · 리스닝 | 실제 환경에서는 |
|---|---|---|---|---|
| **pqcota-ctl** | Ubuntu 24.04 · 호스트 arch | corp+db | `sleep infinity` · 없음 | 리포를 빌드하고 도구를 돌리는 머신 |
| **pqcota-demo-pg** | Debian 13 (postgres:16) · 호스트 arch | corp+db | `postgres` · :5432 | 중앙 인벤토리 DB (단일 호스트 경로엔 불필요) |
| **web-gw** | Ubuntu 24.04, OpenSSL **3.x** · 호스트 arch | corp | `sshd` · :22 | 관측 대상 — TLS/SSH 클라이언트 쪽 |
| **pay-app** | Ubuntu 26.04, temurin 21 · 호스트 arch | corp | `sshd`·`java`·`pqc-echo` · :22 :8443 | 관측 대상 — JVM 자산 |
| **pay-db** | Ubuntu 20.04, OpenSSL **1.1.1** · 호스트 arch | corp+db | `sshd`·`payment-gw`·`api-gw` · :22 :4433 :4434 | 관측 대상 — 레거시 자산 |

노드 OS는 `topology.yaml`의 `version`·`fork`가 고릅니다(3.x→24.04, 3.0→22.04, 1.1.1→20.04,
libressl→alpine). arch는 전부 호스트와 같습니다. **컨트롤러만 두 세그먼트에 붙습니다**(모든 노드에
SSH). IP는 매번 새로 배정되므로 세그먼트 이름으로만 참조합니다.

**노드에 collector는 없습니다** — 워크로드와 데모 헬퍼뿐이고, 디스커버리 후 잔재도 0입니다:

```console
$ docker exec pay-db ls /usr/local/bin
node-entrypoint.sh  pqc-echo  pqcota-gen-traffic.sh  pqcota-observe.sh  ssl-apps.sh
```

`topo-gen`이 0단계에 `--rm`으로 잠깐 더 돕니다(compose·groups.ini·SVG 생성 후 소멸).

## 디스커버리 (Ansible/SSH, 모두 실물)
1. **OpenSSL 자산** — `pqcota-nodescan`: `/proc` 스캔으로 로드된 libssl/libcrypto.
2. **JCA provider 체인** — `pqcota-jvmscan`: **정찰→attach**. `/proc`로 실행 중 JVM(pay-app의 CryptoApp)을 찾아 그 PID에 attach해 `Security.getProviders()` 실체를 본다. CryptoApp이 **런타임에 `addProvider`한 BouncyCastle**까지 잡는다 — java.security엔 정적 등록이 없어 **정적 스캔으론 못 보는** 것(openssl의 `/proc` 스캔과 대칭, `detection=runtime-introspection`). attach 불가 시 정적 프로브로 정직히 폴백.
3. **통신 엣지** — `pqcota-netcap`: AF_PACKET(`CAP_NET_RAW`)으로 TLS/SSH 핸드셰이크를 복호화 없이 관측.

`pqcota-discover-view`(OSS)가 결과를 모아 **발견 자산 + 관측 엣지 등급**를 낸다:
- 🟢 **PQC/하이브리드**(`X25519MLKEM768`, `sntrup761x25519`) · 🔴 **고전=양자취약**(`x25519`, `ECDHE`) · ⚪ **불명**
- 예: `web-gw → pay-app` 🟢 MLKEM · `web-gw → pay-db` 🔴 고전 · SSH도 같은 갈림(`→pay-app` 🟢 sntrup761 · `→pay-db` 🔴 — 레거시 OS의 OpenSSH엔 PQC KEX가 없다)

## 중앙 인벤토리 (엔드포인트·프로필·앱 귀속·이력·변화)
`pqcota-ingest`가 회수 결과를 append-only 히스토리에 적재하고, `pqcota-inventory`가 조회한다:
- **▸ 머신 헤더** — `pqcota-hosts`가 upsert한 **엔드포인트**(이름·ip:port, 비밀 없음) + **프로필**(display_name·env·role·owner, CMDB 선언 레인).
- **@앱 귀속** — 각 크립토 자산이 어느 앱 것인지(`app_keys`). pay-db의 공유 `libssl.so.1.1`은 `payment-gw`·`api-gw` **둘 다** 귀속(그 .so 교체는 두 앱 모두 영향).
- **이력·변화** — 같은 회수 결과를 한 번 더 적재해(실운용의 "다음 회차 스캔"에 해당) `-history`(변화 지점 + 관측 횟수) · `-snapshot`(자산 + 관측 엣지) · `-diff`(추가·사라짐·변경)를 보인다. 같은 관측이므로 diff는 **"변화 없음"** 이 정답이다 — 도구는 없는 변화를 지어내지 않는다. 실제로 버전이 바뀌면 finding id가 유지되어 **같은 자산의 "변경"** 으로 잡힌다.<br>스냅샷은 **내용이 바뀔 때만** 쌓이고, 반복 관측은 가벼운 관측 기록으로만 남는다 — 저장은 변화 횟수만큼만 자라되 "매번 스캔했다"는 증거는 보존된다.
- **자산 스코프** — 노드는 등재됐어도 그 안의 자산 전부가 관리 대상은 아니다. `sshd`·패키지 python 런타임 같은 잡음을 규칙으로 빼면 **앱이 실제로 쓰는 자산만** 남는다. 뺀 건수는 반드시 고지된다 — **제외는 부재가 아니다**(§2.7).
- **보존 정책** — `pqcota-prune`을 dry-run으로 돌려 **노드별 최신 스냅샷은 어떤 정책으로도 지우지 않음**을 보인다. 파괴적 동작이라 조회 커맨드와 분리했고, 실제 삭제는 `-apply`로만 한다.

## 프로비저닝 (생성 → 적용 → 되돌림)
발견된 finding에 **확정 계획(FINALIZED)**을 만들어 `pqcota-provision`을 돌린다:
- **§3.7 게이트** — FINALIZED 아니면 거부. **L2 플레이북 생성**(모듈 스테이지 + config 조각).
- **before 캡처 + 롤백 레코드 영속** — 조치 전 암호 상태(모듈·버전)와 **영향 앱(공유 .so면 다중)**을 append-only로 남긴다.

이어서 생성물을 **실제로 적용한다** — 도는지까지 봐야 "생성했다"가 말이 된다:

- **적용** — 생성된 플레이북을 `ansible-playbook`으로 대상 노드(기본 구성에선 pay-db)에 실행. 모듈 sha256 게이트도 함께 통과시킨다.
- **확인** — 타깃에 `/opt/pqcota/oqsprovider.so`와 `/etc/pqcota/openssl-pqc.cnf`가 놓였고, config가 **그 배치 경로를 참조**하는지(`module = /opt/pqcota/oqsprovider.so`).
- **되돌림** — `--rollback` 플레이북으로 제거. 원본 설정을 덮은 적이 없으니 **제거만으로 이전 상태**가 되고, 두 파일이 사라지는 것까지 확인한다.

> **왜 적용까지 하나**: 생성만 하고 안 돌리면 **문법은 맞는데 실제로는 깨지는** 플레이북이 통과한다. 실제로 그런 결함이 있었다(config 디렉터리를 안 만들어 `copy`가 실패). 이 단계가 그 부류를 상시로 잡는다.
>
> **provider 모듈은 도구가 주지 않는다.** 데모는 배포 경로만 보이려 **빈 파일**을 쓴다 — 실제 암호 기능은 없다. 실물 모듈은 사용자가 빌드하거나 벤더에서 받아 반입한다([커스텀 provider 절차](../provisioning/프로비저닝_설계.md#6b-커스텀-provider)). 데모가 굳이 빈 파일을 쓰는 건 **암호 기능 시연이 아니라 배포·가역성 시연**이 목적이고, "Docker만 있으면 된다"는 전제를 지키기 위해서다.

- **L2는 조각을 놓기만 한다** — 참조되게 만들지 않으므로 모든 산출물이 완전히 가역이다.
- **L3는 여기에 활성화·재시작을 더한다.** 명령은 계획의 `activation` 훅에 사용자가 적은 것을 쓴다 — 환경마다 활성화 지점이 다르므로 도구가 추측하지 않는다. 데모 노드는 `ssl-apps.sh`로 서비스를 관리하므로 훅이 그것을 가리킨다(현실의 systemd unit·사내 기동 스크립트에 해당).
- 데모의 L3가 보이는 것은 **훅 순서·활성화 지점 연결·재시작·가역성**이다. 레거시 노드의 OpenSSL은 이 조각의 PQC 그룹을 모르므로 **능력이 바뀌었다고 말하지 않는다** — 그 노드의 실제 조치는 fork 교체이고, 그건 config로 배포되지 않는다고 플레이북이 주석으로 말한다.

### 선택 단계 — 실물 provider로 마지막 한 칸까지 (`DEMO_REAL_PROVIDER=1`)

```bash
DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
```

빈 파일로는 못 보이는 것이 하나 남는다: **도구가 낸 config와 배치가 정말 암호 능력을 만드는가.** 이 변수를 켜면 실물 oqsprovider(liboqs + oqs-provider)를 노드와 같은 베이스에서 빌드해 그 한 칸까지 확인한다. 첫 실행은 빌드에 수 분 걸리고, 이미지는 다음 실행부터 재사용된다.

대상은 6단계의 pay-db가 **아니다** — provider는 OpenSSL 3의 개념이라 1.1.1 노드에는 넣을 자리가 없다. 인벤토리에서 3.x를 관측한 노드를 골라 같은 L2/L3 산출물로 배치·활성화한 뒤,

| | 보이는 것 |
|---|---|
| **능력** | `openssl list -kem-algorithms`의 ML-KEM 계열이 **0개 → 14개**, `list -providers`에 `oqsprovider … active` |
| **재관측** | 디스커버리를 다시 돌려 적재하고 `pqcota-inventory -diff`로 그 노드의 변화를 본다 |
| **되돌림** | L3→L2 순서로 되돌리면 다시 **0개** — 가역성도 같은 자로 잰다 |

**재관측에서 인벤토리는 그대로다.** 데모는 이것을 숨기지 않고 이유까지 함께 낸다: OpenSSL은 provider 층을 관측하는 경로가 아직 없고(`/proc/maps`의 libssl·libcrypto와 ELF 문자열까지다 — JCA는 attach로 provider 체인을 보지만 OpenSSL은 못 본다), 핸드셰이크도 협상은 양쪽이 알아야 하는데 이 토폴로지의 상대는 1.1.1이다. 설계 검토는 [디스커버리 설계 §2.1](../discovery/디스커버리_설계.md#21-openssl-collector-go--sd-1-sd-3)에 있다. 끝나면 L3→L2 순서로 되돌려 노드를 원래대로 둔다.

## 접근 비밀 경계 (§4A.3)
접속 키·계정은 **사용자 hosts.csv → 런타임 전용 `targets.ini`(소유자 전용 `0600`)**에만 실린다. pqcota 인벤토리(Postgres)엔
**엔드포인트(node_id·이름·ip·port)만** upsert되고 비밀은 적재하지 않는다(데모가 `pqcota_endpoint`에 비밀 0건임을 확인).

## 내 환경(실제 자산)에 적용하려면

데모는 컨테이너를 세워 주지만, 실제 자산에선 **환경이 이미 있고** 당신이 세 가지를 준비합니다.
머신 구분은 위 [어느 컨테이너가 무엇에 해당하나](#어느-컨테이너가-무엇에-해당하나)와 같습니다 —
**`pqcota-ctl`이 곧 당신이 리포를 클론·빌드하는 머신**이고, 노드에는 아무것도 미리 깔지 않습니다.
`hosts.csv` 하나로 끝나지 않습니다:

| # | 준비물 | 필수? | 무엇 |
|---|---|---|---|
| 1 | **`hosts.csv`** | 원격 다중 노드면 필수 | node_id·ip·port·계정·키 → `pqcota-hosts`가 Ansible 인벤토리(`targets.ini`, 0600·미영속) 생성. `--dsn`이면 엔드포인트도 upsert(비밀 제외). 한 호스트에서 그 자리에 훑는다면 **불필요** |
| 2 | **각 노드에 collector 바이너리** | 필수 | `pqcota-nodescan`·`pqcota-jvmscan`·`pqcota-netcap`을 ctl에서 빌드해 두면 됩니다 — **반입은 데모의 플레이북이 그대로 해줍니다**(`discover.yml`이 반입→실행→회수→정리). 빌드 명령은 [루트 README · 빌드](../README.md#빌드)(서명된 사전빌드 릴리스는 [로드맵](../RELEASE_NOTES.md)에 있다 — 그때까지는 소스 빌드가 원칙) |
| 3 | **실행 수단** | 필수 | Ansible이든 손이든 각 노드에서 collector를 돌리고 결과 JSON을 회수. 데모의 [`discover.yml`](../discovery/ansible/discover.yml)이 **참조 구현**입니다 |

그다음은 데모와 같습니다 — 모은 결과를 `pqcota-ingest`에 주면 정규화·적재되고 `pqcota-inventory`로 봅니다.

> **✅ collector 배포는 데모를 그대로 따라도 됩니다.** 노드 이미지엔 collector가 **없고**, `discover.yml`이
> ctl에서 반입→실행→회수→정리합니다(끝나면 노드 잔존물 0). JVM 애드온은 `-recon` 정찰로 **JVM이 있는
> 노드에만** 갑니다. 실환경 이식은 `collector_bin_dir`를 자기 빌드 산출(arch별)로 바꾸는 것뿐입니다.
>
> **데모에만 있는 것 두 가지** — 그대로 옮기면 안 됩니다:
> - **트래픽 생성**(`groups.ini`의 `traffic=`·`pqcota-gen-traffic.sh`): 데모는 관측할 핸드셰이크가 없어 **일부러 만들어 냅니다.** 실제 환경엔 진짜 트래픽이 흐르므로 `pqcota-netcap <node> <iface> <창>`으로 **관측만** 하면 됩니다.
> - **그룹 멤버십**(`groups.ini`의 `[java]`): 어느 노드에 `pqcota-jvmscan`을 돌릴지 고르는 데모의 방식일 뿐, 자기 인벤토리 방식대로 하면 됩니다.

**선택 사항**: 노드 등재 게이트(`pqcota-ingest <dir> <scope-file>`) · 자산 스코프(`-scope-assets`) · CMDB 프로필(`pqcota-profile`) · Postgres 영속(`PQCOTA_DSN`) · 서명 검증(`PQCOTA_VERIFY_KEY`). 무엇이 필수·선택인지: [discovery/cmd README](../discovery/cmd/README.md#필수인가--아니다-원격으로-여러-노드를-훑을-때만-필요하다).

## 디스커버리 그 다음
디스커버리는 "무엇이 실제로 협상되는가"(등급)까지 보여줍니다. **"선언한 것과 얼마나 일치하는가
(CONFIRMED/shadow/미관측)"** 와 **거버넌스·대조**는 이 리포가 하지 않아 데모에도 없습니다.
