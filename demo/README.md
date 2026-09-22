한국어 · [English](README.en.md)

# pqcota 데모 (OSS): 접근준비 → 디스커버리 → 인벤토리 → 프로비저닝

**Docker만 있으면** 한 줄로 설치·수행·제거되는 종단 데모입니다. 단일 가상 네트워크에 묶인 노드에서
pqcota가 **① 사용자 hosts 파일로 접근 준비 → ② Ansible/SSH로 OpenSSL·Java(JCA)·통신 핸드셰이크
디스커버리 → ③ 중앙 인벤토리 → ④ 프로비저닝(플레이북 생성 → 적용 → 되돌림)**까지 전 범위를 보여줍니다.

이 문서는 **데모를 돌리는 사람**을 위한 것입니다. 데모가 왜 이렇게 구성됐고 안에서 무엇이 도는지는
[데모 설계](design.md)에, 무엇을 검증하고 무엇을 검증하지 않는지는
[데모가 검증하는 것](integration-verification.md)에 있습니다.

📊 **실행 전 예상 결과**는 [`expected-output/`](expected-output/)에 있습니다. 콘솔 출력·토폴로지 SVG 샘플과
실제로 실행하면 달라질 수 있는 점(엣지 캡처 타이밍·base 이미지 버전)도 설명합니다.

## 요구 사항
- Docker (Compose v2) · 인터넷(최초 이미지 빌드) · 사용자 `docker` 그룹 (루트/KVM 불필요)

리포는 `pqcota-ctl` 컨테이너 안에서 빌드됩니다([설계 · 빌드](design.md#3-빌드-ctl-머신에서)).
빌드 대상은 **지금 체크아웃된 소스**이며 커밋하지 않은 수정도 포함됩니다.

## 빠른 시작
```bash
./demo/scripts/up.sh      # 이미지 → 컨테이너 → ctl에서 리포 빌드 → SSH 키 → hosts.csv
./demo/scripts/demo.sh    # 접근준비 → 디스커버리 → 인벤토리 → 프로비저닝(생성·적용·되돌림)
./demo/scripts/down.sh    # 정리 (--rmi 로 이미지까지)
```

`./demo/scripts/demo.sh --help`가 조정 지점을 전부 적습니다. 그중 하나가 아래 [선택 단계](#선택-단계-실물-provider로-마지막-한-칸까지-demo_real_provider1)입니다.

처음이면 [`scripts/`](scripts)의 이 세 개만 보면 됩니다. 나머지 폴더는 그 뒤에서 도는 부품이고,
무엇이 무엇인지는 [설계 · 부품](design.md#2-부품)에 적혀 있습니다.

> **데모 환경은 `demo/topology/topology.yaml` 하나가 정의합니다.** 첫 실행 때 샘플이 자동 복사되고
> (git 무시), 그 파일을 고치면 노드 수·종류·OpenSSL 버전·JCA provider·네트워크 세그먼트·핸드셰이크가
> 그대로 반영됩니다. 자기 환경에 가깝게 바꿔 같은 종단을 돌릴 수 있습니다.
> 상세: **[topology/README](topology/README.md)**.

## 무엇이 보이나

`demo.sh`는 진행을 `▶ N/6`으로 찍습니다. 기본 토폴로지의 노드는 web-gw(OpenSSL 3.x) · pay-app(JVM) ·
pay-db(OpenSSL 1.1.1, 레거시) 셋이고, 컨트롤러 `pqcota-ctl`이 SSH로 이들을 훑습니다. 단계마다 보이는 것은
다음과 같습니다.

| 단계 | 보이는 것 |
|---|---|
| **0/6 접근 준비** | `hosts.csv`에서 Ansible 인벤토리를 만들고 엔드포인트와 CMDB 프로필을 등록합니다. 인벤토리 테이블에 접속 비밀이 **0건**임을 SQL로 셉니다 |
| **1/6 SSH 확인** | 컨트롤러에서 모든 노드로 Ansible ping |
| **2/6 디스커버리** | 노드마다 OpenSSL 자산 · JCA provider 체인(런타임에 `addProvider`한 BouncyCastle까지) · TLS/SSH 핸드셰이크를 관측하고 결과를 회수합니다. 끝나면 노드에 남는 것이 없습니다 |
| **3/6 디스커버리 뷰** | 발견 자산과 관측 엣지의 등급: 🟢 PQC/하이브리드 · 🔴 고전(양자취약) · ⚪ 불명. 기본 토폴로지에서는 `web-gw → pay-app`이 🟢, `web-gw → pay-db`가 🔴입니다(TLS·SSH 모두) |
| **4/6 토폴로지** | 관측 결과를 그림으로 그려 `demo/.generated/topology.svg`에 둡니다 |
| **5/6 중앙 인벤토리** | 적재 후 조회: 엔드포인트·프로필 헤더, 자산마다 `@앱` 표시(pay-db의 공유 `libssl.so.1.1`은 `payment-gw`·`api-gw` 둘 다), 같은 결과를 한 번 더 적재한 뒤의 `-history`·`-snapshot`·`-diff`(정답은 **변화 없음**), 자산 스코프(제외 건수 고지), `pqcota-prune` dry-run |
| **6/6 프로비저닝** | 확정 계획으로 L2·L3 플레이북과 롤백 레코드를 **생성**하고, 대상 노드(기본 구성에선 pay-db)에 **적용**해 `/opt/pqcota/oqsprovider.so`·`/etc/pqcota/openssl-pqc.cnf`가 놓였는지 확인한 뒤, 롤백 플레이북으로 **되돌려** 두 파일이 사라지는 것까지 확인합니다 |

출력에 그대로 나오지만 오류가 아닌 것이 둘 있습니다.
- `⚠ duplicate: physical machine … → [pay-db web-gw]`: 데모의 타깃은 한 호스트 위의 컨테이너라 물리 장비 지문이 같습니다. 실운용에서 한 장비를 여러 이름으로 등재했을 때 보게 되는 표시입니다.
- 프로비저닝이 배치하는 `oqsprovider.so`는 **빈 파일**입니다. 데모가 보이는 것은 배포·가역성이지 암호 기능이 아닙니다([왜 그런지](design.md#1-원칙)). 실물로 확인하려면 아래 선택 단계를 켭니다.

### 선택 단계: 실물 provider로 마지막 한 칸까지 (`DEMO_REAL_PROVIDER=1`)

```bash
DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
```

빈 파일로는 못 보이는 것이 하나 남습니다: **도구가 낸 config와 배치가 정말 암호 알고리즘으로 반영되는가.** 이 변수를 켜면 실물 oqsprovider(liboqs + oqs-provider)를 노드와 같은 베이스에서 빌드해 그 한 칸까지 확인합니다. 첫 실행은 빌드에 수 분 걸리고, 이미지는 다음 실행부터 재사용됩니다.

대상은 pay-db가 아니라 인벤토리에서 OpenSSL 3.0–3.4를 관측한 노드입니다. provider는 OpenSSL 3의 개념이라 1.1.1 노드에는 넣을 자리가 없습니다. 같은 L2/L3 산출물로 배치·활성화한 뒤,

| | 보이는 것 |
|---|---|
| **능력** | `openssl list -kem-algorithms`의 ML-KEM 계열이 **0개 → 14개**, `list -providers`에 `oqsprovider … active` |
| **재관측** | 디스커버리를 다시 돌려 적재하고 `pqcota-inventory -diff`로 그 노드의 변화를 봅니다 |
| **되돌림** | L3→L2 순서로 되돌리면 다시 **0개**: 가역성도 같은 자로 잽니다 |

**재관측에서 인벤토리는 그대로입니다.** 오류가 아닙니다. OpenSSL 쪽은 provider 층을 관측하는 경로가 아직 없어서이고, 데모는 그 이유를 출력에 함께 냅니다. 자세한 것은 [설계 · 선택 단계](design.md#54-선택-단계-실물-provider)에 있습니다.

## 산출물은 어디에 생기나

**대부분은 컨테이너 안**에 생기고, 리포에 생기는 것은 **`demo/.generated/` 한 곳뿐**입니다(gitignore). `down.sh`가 그 폴더를 통째로 지우고, 컨테이너 것은 컨테이너와 함께 사라집니다.

| 어디 | 무엇 | 정리 |
|---|---|---|
| **리포** `demo/.generated/` | 리포에 생기는 **전부**: `topology.svg`·`topology.dot`(관측 토폴로지 그림) + 토폴로지에서 생성된 `docker-compose.yml`·`groups.ini`·`profiles.csv`·`manifest.env` | `down.sh`가 폴더째 삭제 (gitignore) |
| **컨트롤러** `pqcota-ctl:/work/` | 빌드 산출 `dist/linux-<arch>/`(collector 3종)·`dist/collector.jar` · 회수된 관측 결과 `results/*.json` · 접속 `hosts.csv`→`ansible/targets.ini`(0600·비밀) · `nodes.json` · `profiles.csv` · 확정 계획 `plan.json` · 생성된 `ansible/playbook{,-l3}.yml`·`rollback{,-l3}.yml` · 모듈 `ansible/files/oqsprovider.so`(빈 파일) | 컨테이너와 함께 소멸 |
| **Postgres** `pqcota-demo-pg` | 중앙 인벤토리: 스냅샷·관측 기록·엔드포인트·프로필·프로비저닝 레코드 | `down.sh`가 볼륨까지 삭제(`-v`) |
| **타깃 노드** | 적용 단계에서 `/opt/pqcota/oqsprovider.so` · `/etc/pqcota/openssl-pqc.cnf`, L3면 활성화 지점 `/etc/pqcota/service.env`: **되돌림 단계에서 제거**되어 원상복귀 | 데모가 스스로 롤백 |

들여다보려면(데모 종료 전):

```bash
docker exec pqcota-ctl ls -R /work           # 컨트롤러 산출물 전부
docker exec pqcota-ctl cat /work/ansible/provision.yml   # 생성된 플레이북
docker exec pqcota-demo-pg psql -U postgres -d pqcota -c '\dt'  # 인벤토리 테이블
```

> **호스트 파일시스템은 거의 안 건드립니다**. 리포에 남는 것은 위 그림·생성물뿐이고, 그마저 gitignore입니다.
> 접속 키(`/work/id_demo`)와 `targets.ini`는 **컨트롤러 안에만** 있고 인벤토리에 적재되지 않습니다([설계 · 접근 비밀 경계](design.md#6-접근-비밀-경계-15)).

## 내 환경(실제 자산)에 적용하려면

데모는 컨테이너를 세워 주지만, 실제 자산에선 **환경이 이미 있고** 사용자가 세 가지를 준비합니다.
순서대로 무엇이 나오는지는 [여정](../journey.md)이 컨테이너 없이 처음부터 끝까지 따라갑니다.
머신 구분은 [설계 · 도는 컨테이너](design.md#4-실행-시점-도는-컨테이너-기본-토폴로지)와 같습니다.
**`pqcota-ctl`이 곧 사용자가 리포를 클론·빌드하는 머신**이고, 노드에는 아무것도 미리 깔지 않습니다.
`hosts.csv` 하나로 끝나지 않습니다:

| # | 준비물 | 필수? | 무엇 |
|---|---|---|---|
| 1 | **`hosts.csv`** | 원격 다중 노드면 필수 | node_id·ip·port·계정·키 → `pqcota-hosts`가 Ansible 인벤토리(`targets.ini`, 0600·미영속) 생성. `--dsn`이면 엔드포인트도 upsert(비밀 제외). 한 호스트에서 그 자리에 훑는다면 **불필요** |
| 2 | **각 노드에 collector 바이너리** | 필수 | `pqcota-nodescan`·`pqcota-jvmscan`·`pqcota-netcap`을 ctl에서 빌드해 두면 됩니다. **반입은 데모의 플레이북이 그대로 해줍니다**(`discover.yml`이 반입→실행→회수→정리). 빌드 명령은 [루트 README · 빌드](../README.md#빌드)(arch별 사전 빌드 바이너리는 릴리스에 이미 있고, 무결성은 `SHA256SUMS`로 확인합니다. 그것이 진짜 이 리포가 낸 것인지 가리는 서명만 [로드맵](../RELEASE_NOTES.md)에 남아 있습니다) |
| 3 | **실행 수단** | 필수 | Ansible이든 손이든 각 노드에서 collector를 돌리고 결과 JSON을 회수. 데모의 [`discover.yml`](../discovery/ansible/discover.yml)이 **참조 구현**입니다 |

그다음은 데모와 같습니다. 모은 결과를 `pqcota-ingest`에 주면 정규화·적재되고 `pqcota-inventory`로 봅니다.

> **✅ collector 배포는 데모를 그대로 따라도 됩니다.** 노드 이미지엔 collector가 **없고**, `discover.yml`이
> ctl에서 반입→실행→회수→정리합니다(끝나면 노드 잔존물 0). JVM 애드온은 `-recon` 정찰로 **JVM이 있는
> 노드에만** 갑니다. 실환경 이식은 `collector_bin_dir`를 자기 빌드 산출(arch별)로 바꾸는 것뿐입니다.
>
> **데모에만 있는 것 두 가지**. 그대로 옮기면 안 됩니다:
> - **트래픽 생성**(`groups.ini`의 `traffic=`·`pqcota-gen-traffic.sh`): 데모는 관측할 핸드셰이크가 없어 **일부러 만들어 냅니다.** 실제 환경엔 진짜 트래픽이 흐르므로 `pqcota-netcap <node> <iface> <구간초>`로 **관측만** 하면 됩니다.
> - **그룹 멤버십**(`groups.ini`의 `[java]`): 어느 노드에 `pqcota-jvmscan`을 돌릴지 고르는 데모의 방식일 뿐, 자기 인벤토리 방식대로 하면 됩니다.

**선택 사항**: 노드 등재 게이트(`pqcota-ingest <dir> <scope-file>`) · 자산 스코프(`-scope-assets`) · CMDB 프로필(`pqcota-profile`) · Postgres 영속(`PQCOTA_DSN`) · 서명 검증(`PQCOTA_VERIFY_KEY`). 무엇이 필수·선택인지: [discovery/cmd README](../discovery/cmd/README.md#필수인가-아니다-원격으로-여러-노드를-훑을-때만-필요하다).

## 디스커버리 그 다음
디스커버리는 "무엇이 실제로 협상되는가"(등급)까지 보여줍니다. **"선언한 것과 얼마나 일치하는가
(CONFIRMED/UNDECLARED/UNOBSERVED)"**와 **거버넌스·대조**는 이 리포가 하지 않아 데모에도 없습니다.
