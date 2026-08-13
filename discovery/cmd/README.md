# discovery/cmd/ — 디스커버리 실행 진입점

디스커버리 단계의 CLI(Go 바이너리)들. 이름이 비슷하니 **어느 걸 언제 쓰는지**를 세 범주로 나눠 정리한다. 관측은 전부 ②의 collector가 대상 머신에서 하고, 낸 결과를 중앙에 쌓는 일은 [inventory/cmd](../../inventory/cmd/README.md)가 맡는다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

## ① 접근 준비 — 사용자 hosts 파일에서 discovery 접근을 세팅
discovery를 시작하기 전에, 접근 대상 노드들에 대한 접속 정보를 **사용자가 직접 작성한 hosts 파일**(CSV)로 정의한다. 접근 비밀(계정·SSH 키)은 이 파일에만 있고 **pqcota 인벤토리엔 적재하지 않는다**.

### `pqcota-hosts`

```
pqcota-hosts [--ansible-out <path>] [--dsn <postgres>] <hosts.csv>
```

| 인자·옵션 | 하는 일 |
|---|---|
| `<hosts.csv>` | 접속 정보 파일(사용자 작성) |
| `--ansible-out <path>` | Ansible 인벤토리(ini) 생성 — 계정·키가 담기므로 **소유자만 읽을 수 있게**(`0600`) 쓴다. 이걸로 ②를 각 노드에 돌린다 |
| `--dsn <postgres>` | 엔드포인트를 pqcota 인벤토리에 upsert — 계정·키 제외, 나중에 수정·재사용 가능 |

`<postgres>`는 Postgres 접속 문자열이다. 드라이버가 pgx라 URL 형식과 키=값 형식을 모두 받는다:

```
postgres://<user>:<password>@<host>:<port>/<db>     # 예: postgres://postgres:pqcota@localhost:5432/pqcota
host=localhost port=5432 user=postgres dbname=pqcota
```

같은 문자열을 `PQCOTA_DSN` 환경변수로도 준다(적재·조회 커맨드가 이걸 읽는다).

옵션 없이 돌리면 안전 엔드포인트(`node_id`·이름·ip·port)를 stdout에 요약만 한다.

### 그다음 — 만든 인벤토리로 collector 돌리기

`targets.ini`가 생겼다고 관측이 시작되지는 않는다. 그건 **도달 수단**일 뿐이고, 실제로 collector를 각 노드에서 돌리는 것은 사용자의 Ansible이다. 그 방법을 보이는 **참조 플레이북**이 리포에 있다 → [`discovery/ansible/discover.yml`](../ansible/discover.yml)

```bash
ansible-playbook -i targets.ini discovery/ansible/discover.yml
```

하는 일은 넷이다 — **반입**(collector 셋을 `/tmp/pqcota-collector`로) → **실행** → **회수**(결과 JSON을 컨트롤러로) → **정리**(노드에 아무것도 남기지 않는다). collector는 상주 에이전트가 아니라 실행 후 종료하는 CLI라 이 일회성 패턴이 맞다.

JVM 애드온(`collector.jar`)은 **모든 노드에 뿌리지 않는다** — `pqcota-jvmscan --recon`으로 그 노드에 JVM이 있는지 먼저 보고, 있는 노드에만 보낸다.

자기 인프라에 쓰려면 플레이북의 `collector_bin_dir`를 자기 빌드 산출(`dist/linux-amd64` 등)로 바꾸면 된다. 데모 전용은 트래픽 생성 헬퍼뿐이다. → [collector 배포 설계](../collector-deployment.md)

### 필수인가 — **아니다. "원격으로 여러 노드를 훑을 때"만 필요하다**

이 단계는 관측의 전제가 아니라 **원격 도달 수단**이다. 무엇을 하려는지에 따라 갈린다:

| 하려는 일 | 접근 준비 |
|---|---|
| 한 노드를 그 자리에서 훑고 보기 (`pqcota-nodescan --output table`) | **불필요** — SSH·Ansible을 아예 안 쓴다 |
| 결과 JSON을 직접 모아 적재하기 (`pqcota-ingest <dir>`) | **불필요** — 파일만 있으면 된다 |
| 컨트롤러에서 **여러 노드에 SSH로** 스캐너를 돌리기 | **필요** — 노드에 닿으려면 Ansible 인벤토리가 있어야 한다.<br>`targets.ini`를 직접 써도 된다. `pqcota-hosts`를 쓰면 CSV 한 벌에서 그 ini를 만들어 주고(계정·키가 담겨 소유자 전용 `0600`), **pqcota 인벤토리에는 계정·키를 뺀 `node_id`·이름·ip·port만** 넣는다 |
| 인벤토리 뷰에 **▸머신 헤더**(이름·ip:port) 띄우기 | **선택** — `--dsn`으로 엔드포인트를 넣으면 헤더가 붙고, 없으면 헤더만 생략된다(자산·엣지는 그대로) |

노드 **등재 게이트**(`pqcota-ingest`의 scope-master 인자)도 마찬가지로 **선택**이다 — 안 주면 게이트를 생략한다(로컬·데모). 등재는 "관리 대상 경계를 선언하고 싶을 때" 쓰는 것이지 적재의 전제가 아니다.

## ② collector — 대상 머신에서 관측한다

| collector | 관측 대상 | 산출 |
|---|---|---|
| `pqcota-nodescan` | `/proc`의 로드된 OpenSSL(libssl/libcrypto) | CollectionResult |
| `pqcota-jvmscan` | 실 JVM의 JCA provider 체인(`Security.getProviders()`) | CollectionResult |
| `pqcota-netcap` | TLS/SSH 핸드셰이크(AF_PACKET, linux 전용) | CollectionResult(관측 엣지) |

셋 다 `CollectionResult`를 내고, 루트 README의 빌드가 `dist/linux-<arch>/`로 내는 것도 이 셋이다. 각각 `discovery/collectors/{openssl,jvm,network}` 패키지를 감싼 얇은 진입점이라, 새 관측 대상이 늘면 collector를 하나 더 붙이면 된다 — 코어는 그대로다.

여러 노드에서 한꺼번에 돌리는 법은 [①의 참조 플레이북](#그다음--만든-인벤토리로-collector-돌리기).

### `pqcota-nodescan`

```
pqcota-nodescan [--output json|table] [node-id]
```

| 인자·옵션 | 하는 일 |
|---|---|
| `[node-id]` | CMDB 권위 id. 생략하면 머신 지문에서 결정론적 self-id를 만들고, 그것도 없으면 `host://local` |
| `--output` | 출력 형식 → [아래 공통](#--output--nodescanjvmscan-공통) |

`/proc`를 열지 못하면 **빈 결과를 내지 않는다** — "OpenSSL 없음"이 아니라 관측 자체가 불가한 것이라, 완전성 노트에 갭으로 적고 stderr로 알린다.

### `pqcota-jvmscan`

```
pqcota-jvmscan [--output json|table] [--pid N] [node-id]
pqcota-jvmscan --recon
```

| 인자·옵션 | 하는 일 |
|---|---|
| `[node-id]` | 생략하면 `host://local` |
| `--pid N` | 그 PID의 JVM 하나만 관측. 기본은 정찰로 찾은 전부 |
| `--recon` | 정찰만 하고 발견된 JVM을 JSON으로 낸다(관측 안 함) |
| `--output` | 출력 형식 → [아래 공통](#--output--nodescanjvmscan-공통) |

`--pid`가 지목한 PID가 실행 중 JVM에 없으면 **전부 훑기로 갈아타지 않고 실패한다** — 관측하지 못한 것은 갭이지 다른 대상으로 대체할 일이 아니다.

`--recon`은 오케스트레이터가 "이 노드에 JVM이 있나"를 보고 에이전트 JAR를 보낼지 정하는 근거다. JVM이 없으면 `[]`를 낸다.

attach는 막힐 수 있다(`DisableAttachMechanism`, JEP 451, 권한). 그때는 실패로 끝내지 않고 정적 체인 읽기로 떨어지며 **동적 등록은 사각으로 남아 갭으로 고지된다** — 어떤 순서로 떨어지는지는 [jvm collector](../collectors/jvm/README.md).

### `pqcota-netcap`

```
pqcota-netcap [--strict] <node-id> [iface] [window-seconds]
```

| 인자·옵션 | 기본값 | 하는 일 |
|---|---|---|
| `<node-id>` | `host://local` | 관측 결과를 달아 둘 노드 |
| `[iface]` | `eth0` (env `NETCAP_IFACE`) | 포집할 인터페이스 |
| `[window-seconds]` | `8` (env `NETCAP_WINDOW_SEC`) | 관측 구간 길이 |
| `--strict` | 꺼짐 | 관측 불가일 때 종료코드 1로 실패 |

**`CAP_NET_RAW`가 없으면 관측이 안 된다.** 그때 netcap은 stderr로 그 사실과 부여 방법(`setcap cap_net_raw+ep`)을 알리고, stdout으로는 `layers_missing=[NETWORK]`인 **갭 기록**을 낸다. 기본 종료코드는 **0**이다.

0인 이유는 이 갭이 중앙까지 가야 하기 때문이다. Ansible로 여러 노드를 돌릴 때 종료코드가 1이면 그 태스크가 실패로 처리돼 결과 파일을 회수하지 않고, 중앙에는 그 노드에 대한 기록이 **아무것도** 남지 않는다. 그러면 인벤토리 뷰에서 "이 노드엔 TLS 링크가 없다"로 읽힌다 — 실제로는 관측하지 못한 것인데. 갭을 실어 보내야 "관측하지 못했다"와 "없다"가 구분된다.

손으로 돌리면서 실패로 끝내고 싶으면 `--strict`를 준다(갭은 그대로 stdout에 낸다).

### `--output` — nodescan·jvmscan 공통

같은 수집이 돌고 **어느 층을 내보내느냐**가 갈린다.

| 값 | 내는 것 | 쓸 때 |
|---|---|---|
| `json`(기본) | **CollectionResult** — collector 네이티브 원본(`raw_capture`)과 CycloneDX 표준 본문(`cbom_cyclonedx`)이 Envelope·완전성 맵과 함께 한 메시지에 담긴다 | 중앙(③)이 회수해 쌓는다 |
| `table` | 그 결과를 **정규화해 파생한 Finding[]**을 사람이 읽는 표로 | 한 노드를 그 자리에서 확인한다 |

진행·경고는 두 경우 모두 stderr로 간다 — stdout엔 요청한 것만 담긴다.

**`table`는 저장하지 않는다.** 중앙이 하는 정규화를 인메모리로 한 번 돌리고 버린다 — 히스토리·스냅샷 diff는 ③에 쌓아야 생긴다.

### 권한 · 환경변수

노드에서 돌릴 때 필요한 것. 권한이 모자라면 **보이는 범위가 줄거나 그 커맨드가 아예 못 돈다**. 무엇을 관측하지 못했는지는 완전성 맵으로 고지되지만, 애초에 갖추는 편이 낫다.

| 커맨드 | 권한 | 환경변수 |
|---|---|---|
| `pqcota-nodescan` | 자기 프로세스는 그냥 된다. **다른 사용자 것까지 보려면 root**(또는 `CAP_SYS_PTRACE`) | `PQCOTA_SIGN_KEY` — 있으면 결과에 서명(선택) |
| `pqcota-netcap` | **`CAP_NET_RAW` 필수**(`setcap` 또는 root) — 없으면 포집이 시작되지 않는다 | `NETCAP_IFACE`(기본 `eth0`) · `NETCAP_WINDOW_SEC`(기본 8초) |
| `pqcota-jvmscan` | 대상 JVM과 **같은 UID**(또는 root). 대상이 attach를 막고 있으면 정적 프로브로 떨어진다 | `PQCOTA_JVM_AGENT`=collector.jar 경로 — 주면 attach 경로, 없으면 정적 프로브 |


### 실행 요건 — 커널·권한

**커널 하한은 3.2다.** Go 툴체인이 정하는 값이고(Go 1.24부터), 이 리포는 그보다 새 기능을 요구하지 않는다. 그 위라면 배포판·libc는 가리지 않는다 — 정적 링크라서다.

| 기능 | 커널 요구 | 그 아래에서는 |
|---|---|---|
| 노드 스캔 · fork 판정 · JVM 정찰 | **3.2**(툴체인 하한) | 바이너리가 실행되지 않는다 |
| 통신 엣지 관측 | 추가 요구 없음 (`AF_PACKET`은 2.2 계열) | — |
| 앱 표시(systemd 유닛) | systemd가 도는 환경 | 유닛명 대신 **실행 파일 경로**로 짚는다(upstart 시절 배포판) |
| **컨테이너 안 JVM attach** | **4.1**(`/proc/<pid>/status`의 `NSpid`) | 호스트 PID로 폴백 — 그 JVM만 미관측(갭으로 고지) |

하한 미만에서도 **조용히 틀리지 않는다**. 관측하지 못한 것은 완전성 갭으로 나가고(§2.6), `NSpid`가 없으면 호스트 PID를 그대로 쓴다.

**실측**(KVM VM — 컨테이너는 호스트 커널을 공유해 이 항목을 검증할 수 없다):

| 커널 | 배포판 | 결과 |
|---|---|---|
| **3.2.0** | Ubuntu 12.04 | 세 collector 모두 정상 종료. OpenSSL 1.0.0g 탐지(fork=OpenSSL·dynamic), AF_PACKET 8초 구간 관측 성공. systemd가 없어 앱을 **실행 파일 경로**로 짚음(`/usr/sbin/sshd` 등) |
| **3.10.0** | CentOS 7.9 | 세 collector 모두 정상 종료. OpenSSL 1.0.2k 탐지, cgroup v1에서 **systemd 유닛명으로 짚기 성공**(`sshd.service` 등) |

두 커널 모두 `/proc/<pid>/status`에 **`NSpid` 줄이 없고**, 호스트 PID 폴백이 동작해 죽지 않았다 — 4.1 경계가 실물로 확인된 셈이다.

> **PoC/테스트 하네스는 여기 없다.** openssl collector의 실물 `/proc`·ELF 검증 CLI는 유일 소비자인 통합 테스트와 co-locate — [`discovery/collectors/openssl/integration/probe`](../collectors/openssl/integration). 이 폴더(discovery/cmd/)는 **제품 디스커버리 진입점만** 둔다.

## ③ 기타 — collector가 아닌 노드측 커맨드

대상 머신에서 실행되지만 **관측이 아니다**. `CollectionResult`를 내지 않으므로 중앙에 적재되지 않는다.

### `pqcota-procs`

```
pqcota-procs [--unit UNIT] [--exe PATH] [--cmd REGEX]
```

| 옵션 | 하는 일 |
|---|---|
| `--unit UNIT` | systemd 유닛명(cgroup 매칭) |
| `--exe PATH` | 실행 파일 경로(정확 일치) |
| `--cmd REGEX` | cmdline 정규식 |

**셋 중 하나 이상**을 줘야 한다(전부 비면 종료코드 2). 다른 사용자 프로세스까지 보려면 root가 필요하다.

프로비저닝 직전 **재시작 대상**을 찾는 용도다 — PID는 휘발이라 저장하지 않고 그때그때 조회한다. 아직 이걸 부르는 자동 경로는 없다(플레이북의 `activation.restart`는 사용자가 쓴 명령을 그대로 실행한다).

---
**언제 무엇을 쓰나**
- 여러 노드를 관측해 인벤토리에 쌓기 → **②**를 각 노드에서(기본 `--output json`) → [`pqcota-ingest`](../../inventory/cmd/README.md)로 중앙 적재.
- 한 노드를 그 자리에서 확인만 하기 → **②**를 `--output table`로. 쌓이지 않는다.

> 로직은 전부 `pkg/discovery/`(정규화·히스토리)·`discovery/collectors/`(수집)에 있고, 이 커맨드들은 그걸 조립하는 얇은 진입점이다. 회수된 결과는 인벤토리의 `pqcota-ingest`가 **append-only 히스토리로 누적**한다.
