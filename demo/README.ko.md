[English](README.md) · 한국어

# pqcota 데모(OSS): 접근 준비 → 관측 → 인벤토리 → 전환물 생성

**Docker만 있으면** 한 줄로 설치하고, 실행하고, 스스로 지우는 종단 데모입니다. 가상 네트워크 하나에 묶인 노드들에서 pqcota가 전체 범위를 보여 줍니다. **① 사용자의 hosts 파일로 접근 준비 → ② Ansible/SSH로 OpenSSL, Java(JCA), 통신 핸드셰이크 관측 → ③ 중앙 인벤토리 → ④ 전환물 생성(플레이북 생성 → 적용 → 되돌림)**입니다.

이 문서는 **데모를 돌리는 사람**을 위한 것입니다. 연결된 문서 중 한국어판이 없는 것은 영문입니다.

📊 **실행하기 전에 볼 기대 결과**: [`expected-output/`](expected-output/)에 콘솔 출력 샘플과 토폴로지 SVG가 있고, 실제 실행에서 달라질 수 있는 것(엣지 캡처 시점, 베이스 이미지 버전)도 적혀 있습니다.

## 요구 사항
- Docker(Compose v2), 인터넷(처음 이미지를 빌드할 때), `docker` 그룹에 속한 사용자(root나 KVM은 필요 없음)
- 다섯 리포를 **나란히** 클론해 둘 것(이 리포의 디렉터리 이름은 `pqcota`이고, 그 옆에 `pqcota-common`, `pqcota-inventory`, `pqcota-discovery`, `pqcota-provisioning`을 둡니다). 데모 이미지가 모두 빌드합니다. [빌드 안내](../docs/build.ko.md#소스-받기)를 보세요.

리포는 `pqcota-ctl` 컨테이너 안에서 빌드합니다.
빌드되는 것은 **지금 체크아웃한 소스 그대로**이며 커밋하지 않은 변경도 포함합니다.

## 빠른 시작
```bash
./demo/scripts/up.sh      # images → containers → build the repo on ctl → SSH keys → hosts.csv
./demo/scripts/demo.sh    # access prep → discovery → inventory → provisioning (generate, apply, roll back)
./demo/scripts/down.sh    # clean up (--rmi also removes images)
```

`./demo/scripts/demo.sh --help`는 조절할 수 있는 항목을 모두 보여 줍니다. 그중 하나가 아래의 [선택 단계](#선택-단계-실물-provider로-마지막-한-걸음demo_real_provider1)입니다.

처음이라면 [`scripts/`](scripts)의 이 스크립트 세 개면 충분합니다. 다른 폴더는 그 뒤에서 도는 장치입니다.

> **데모 환경은 파일 하나, `demo/topology/topology.yaml`로 정의됩니다.** 처음 실행하면 샘플이 자동으로
> 복사됩니다(git은 무시합니다). 이 파일을 고치면 노드 수, 노드 종류, OpenSSL 버전, JCA provider,
> 네트워크 구간, 핸드셰이크가 모두 따라 바뀝니다. 그러니 사용자 환경에 더 가깝게 만들어
> 같은 종단 흐름을 돌려 볼 수 있습니다.
> 자세한 내용: **[topology/README](topology/README.md)**(영문).

## 무엇이 보이나

`demo.sh`는 진행 상황을 `▶ N/6`으로 보고합니다. 기본 토폴로지에는 노드가 셋 있고(web-gw(OpenSSL 3.x), pay-app(JVM), pay-db(OpenSSL 1.1.1, 레거시)), 컨트롤러 `pqcota-ctl`이 SSH로 이 노드들을 스캔합니다. 단계마다 보이는 것은 다음과 같습니다.

| 단계 | 보이는 것 |
|---|---|
| **0/6 접근 준비** | `hosts.csv`로 Ansible 인벤토리를 만들고, 엔드포인트와 CMDB 프로파일을 등록합니다. 인벤토리 표에 접근 비밀이 몇 개인지 SQL로 세며 그 값은 **0**입니다 |
| **1/6 SSH 확인** | 컨트롤러에서 모든 노드로 Ansible ping을 보냅니다 |
| **2/6 관측** | 노드마다 OpenSSL 자산, JCA provider 체인(`addProvider`로 실행 중에 추가된 BouncyCastle 포함), TLS/SSH 핸드셰이크를 관측하고 결과를 가져옵니다. 실행이 끝나면 플레이북의 임시 디렉터리를 노드에서 지웁니다(Java 관측은 대상의 `/tmp`에 작은 파일을 남깁니다. [FAQ](../docs/faq.ko.md#실행-중인-시스템에-미치는-영향)를 보세요) |
| **3/6 관측 보기** | 발견한 자산과 관측한 엣지의 등급입니다. 🟢 PQC/하이브리드, 🔴 고전(양자에 취약), ⚪ 등급 미정. 기본 토폴로지에서 `web-gw → pay-app`은 🟢이고 `web-gw → pay-db`는 🔴입니다(TLS와 SSH 모두) |
| **4/6 토폴로지** | 관측한 것을 그림으로 그려 `demo/.generated/topology.svg`에 저장합니다 |
| **5/6 중앙 인벤토리** | 적재한 뒤 조회합니다. 엔드포인트와 프로파일 머리말, 모든 자산의 `@app` 라벨(pay-db가 공유하는 `libssl.so.1.1`은 `payment-gw`와 `api-gw` 둘 다에 귀속됩니다), 같은 결과를 두 번째로 적재해도 **새 스냅샷이 생기지 않음**(`-history`), 그 스냅샷의 자산과 엣지(`-snapshot`), 자산 범위(제외한 수를 보고함)와 그 범위를 가로지르는 `-diff`, 그리고 `pqcota-prune` 시험 실행 |
| **6/6 전환물 생성** | 확정된 계획에서 L2와 L3 플레이북과 되돌림 기록을 **생성**하고, 그것을 대상 노드(기본 설정에서는 pay-db)에 **적용**하며, `/opt/pqcota/oqsprovider.so`와 `/etc/pqcota/openssl-pqc.cnf`가 놓였는지 데모가 확인합니다. 그다음 되돌림 플레이북이 이를 **취소**하고 두 파일이 사라졌는지 데모가 확인합니다 |

출력에 그대로 나오지만 오류가 아닌 것이 둘 있습니다.
- `⚠ duplicate: physical machine … → [pay-db web-gw]`: **Linux 호스트에서** 나타납니다. 데모의 대상은 한 호스트의 컨테이너이고, Linux에서는 컨테이너 안에서 호스트의 머신 지문이 보여서 셋의 값이 같습니다. 한 머신을 여러 이름으로 등록했을 때 운영에서 보게 되는 것과 같습니다. Mac의 Docker Desktop에서는 지문이 다른 곳에서 오므로 이 줄이 나타나지 않습니다(까닭).
- 전환물 생성이 놓는 `oqsprovider.so`는 **빈 파일**입니다. 데모가 보이는 것은 배포와 되돌릴 수 있음이지 암호 능력이 아닙니다(까닭). 실물로 확인하려면 아래 선택 단계를 켜세요.

### 선택 단계: 실물 provider로 마지막 한 걸음(`DEMO_REAL_PROVIDER=1`)

```bash
DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
```

빈 파일로는 보일 수 없는 것이 하나 있습니다. **도구가 만든 설정과 배치가 실제로 암호 능력을 만드는가**입니다. 이 변수를 켜면 노드와 같은 베이스 위에서 실물 oqsprovider(liboqs와 oqs-provider)를 빌드해 그 마지막 한 걸음을 확인합니다. 처음 실행에는 빌드에 몇 분이 걸리고, 그 뒤로는 이미지를 재사용합니다.

대상은 pay-db가 아니라 인벤토리에서 OpenSSL 3.0–3.4로 관측된 노드입니다. provider는 OpenSSL 3의 개념이라 1.1.1 노드에는 놓을 곳이 없습니다. 같은 L2/L3 산출물이 그것을 배치하고 활성화한 다음 이렇게 됩니다.

| | 보이는 것 |
|---|---|
| **능력** | `openssl list -kem-algorithms`의 ML-KEM 계열이 **0에서 14**가 되고, `list -providers`에 `oqsprovider … active`가 보입니다 |
| **다시 관측** | 관측을 다시 돌려 적재하고, `pqcota-inventory -diff`가 그 노드의 변경을 보여 줍니다 |
| **되돌림** | L3→L2 순서로 취소하면 **0**으로 돌아갑니다. 같은 잣대로 잰 되돌릴 수 있음입니다 |

**다시 관측해도 인벤토리는 바뀌지 않습니다.** 오류가 아닙니다. OpenSSL에는 아직 provider 계층을 관측할 경로가 없고, 데모가 그 까닭을 함께 출력합니다.

## 출력이 놓이는 곳

**대부분은 컨테이너 안에 있고**, 리포에 놓이는 것은 **`demo/.generated/`**(git은 무시)뿐입니다. `down.sh`는 그 폴더를 통째로 지우고, 컨테이너 안에 있는 것은 컨테이너와 함께 사라집니다.

| 위치 | 내용 | 정리 |
|---|---|---|
| **리포**, `demo/.generated/` | 리포에 놓이는 **모든 것**: `topology.svg`와 `topology.dot`(관측한 토폴로지 그림), 그리고 토폴로지에서 생성한 `docker-compose.yml`, `groups.ini`, `profiles.csv`, `manifest.env` | `down.sh`가 폴더를 지웁니다(git은 무시) |
| **컨트롤러**, `pqcota-ctl:/work/` | 빌드 산출물 `dist/linux-<arch>/`(수집기 셋)와 `dist/collector.jar`, 가져온 관측 `results/*.json`, 접속 정보 `hosts.csv`→`ansible/targets.ini`(0600, 비밀), `nodes.json`, `profiles.csv`, 확정된 계획 `plan.json`, 생성한 `ansible/playbook{,-l3}.yml`과 `rollback{,-l3}.yml`, 모듈 `ansible/files/oqsprovider.so`(빈 파일) | 컨테이너와 함께 사라집니다 |
| **Postgres**, `pqcota-demo-pg` | 중앙 인벤토리: 스냅샷, 관측 기록, 엔드포인트, 프로파일, 전환물 생성 기록 | `down.sh`가 볼륨도 지웁니다(`-v`) |
| **대상 노드** | 적용 단계에서 `/opt/pqcota/oqsprovider.so`와 `/etc/pqcota/openssl-pqc.cnf`, 그리고 L3 활성화 지점 `/etc/pqcota/service.env`. **되돌림 단계에서 제거되어** 원래 상태로 돌아갑니다 | 데모가 스스로 되돌립니다 |

안을 들여다보려면(데모를 내리기 전에):

```bash
docker exec pqcota-ctl ls -R /work           # everything on the controller
docker exec pqcota-ctl cat /work/ansible/provision.yml   # the generated playbook
docker exec pqcota-demo-pg psql -U postgres -d pqcota -c '\dt'  # the inventory tables
```

> **호스트 파일 시스템은 거의 건드리지 않습니다.** 리포에 남는 것은 위의 그림과 생성 파일뿐이고, 그것들도 git이 무시합니다.
> 접속 키(`/work/id_demo`)와 `targets.ini`는 **컨트롤러 안에만** 있으며 인벤토리로 적재되지 않습니다.

## 사용자의 환경(실제 자산)에 적용하려면

데모는 컨테이너를 대신 띄워 줍니다. 실제 자산에서는 **환경이 이미 있으므로** 세 가지를 준비합니다.
머신의 역할은 데모와 같습니다. **`pqcota-ctl`은 리포를 클론해 빌드하는 머신**이고, 노드에는 미리 설치된 것이 없습니다.
`hosts.csv`로 끝나지 않습니다.

| # | 준비할 것 | 필수? | 무엇인가 |
|---|---|---|---|
| 1 | **`hosts.csv`** | 원격 다중 노드에서 필수 | node_id, ip, port, 계정, 키 → `pqcota-hosts`가 Ansible 인벤토리(`targets.ini`, 0600, 저장하지 않음)를 생성합니다. `--dsn`을 주면 엔드포인트도 upsert합니다(비밀은 제외). 호스트 하나를 그 자리에서 스캔한다면 **필요 없습니다** |
| 2 | **노드별 수집기 바이너리** | 필수 | ctl에서 `pqcota-nodescan`, `pqcota-jvmscan`, `pqcota-netcap`을 빌드합니다. **데모의 플레이북은 이를 대신 배포합니다**(`discover.yml`이 배포하고, 실행하고, 가져오고, 정리합니다). 빌드 명령은 [빌드 안내](../docs/build.ko.md#빌드)에 있습니다(아키텍처별 미리 빌드된 바이너리가 이미 릴리스에 있고 `SHA256SUMS`로 무결성을 확인합니다. 이 리포에서 나왔음을 증명하는 서명만 아직 [로드맵](../RELEASE_NOTES.md)(영문)에 있습니다) |
| 3 | **실행하는 방법** | 필수 | Ansible이나 직접, 노드마다 수집기를 실행하고 결과 JSON을 가져옵니다. 데모의 [`discover.yml`](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/ansible/discover.yml)이 **참조 구현**입니다 |

그 뒤는 데모와 같습니다. 수집한 결과를 `pqcota-ingest`에 넘기면 정규화해 저장하고, `pqcota-inventory`로 봅니다.

> **✅ 데모의 수집기 배포를 그대로 복사해도 됩니다.** 노드 이미지에는 수집기가 **없고**, `discover.yml`이
> ctl에서 배포하고, 실행하고, 결과를 가져오고, 정리합니다(실행이 끝나면 `/tmp`의 작은 Java 파일을 빼고 노드에는 자기 것이 남지 않습니다). JVM
> 추가 구성요소는 `-recon`으로 **JVM이 있는 노드에만** 갑니다. 실제 환경으로 옮기려면 `collector_bin_dir`이
> 사용자의 빌드 산출물(아키텍처별)을 가리키게 하기만 하면 됩니다.
>
> **데모에만 있는 것이 둘 있으니** 가져가지 마세요.
> - **트래픽 생성**(`groups.ini`의 `traffic=`, `pqcota-gen-traffic.sh`): 데모에는 관측할 핸드셰이크가 없어서 **일부러 만들어 냅니다.** 실제 환경에는 실제 트래픽이 있으므로 `pqcota-netcap <node> <iface> <window>`로 **관측**하기만 하면 됩니다.
> - **그룹 소속**(`groups.ini`의 `[java]`): 어느 노드에 `pqcota-jvmscan`을 보낼지 고르는 데모의 방식일 뿐입니다. 사용자의 인벤토리가 하는 방식을 쓰세요.

**선택**: 노드 등록 관문(`pqcota-ingest <dir> <scope-file>`), 자산 범위(`-scope-assets`), CMDB 프로파일(`pqcota-profile`), Postgres 저장(`PQCOTA_DSN`), 서명 검증(`PQCOTA_VERIFY_KEY`). 무엇이 필수이고 무엇이 선택인지는 [pqcota-discovery cmd README](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)를 보세요.

## 관측 너머
관측은 「실제로 협상된 것」(등급)까지 보여 줍니다. **「그것이 선언된 것과 얼마나 맞는가(CONFIRMED/UNDECLARED/UNOBSERVED)」**와 거버넌스, 대조는 이 리포가 하지 않으므로 데모에도 없습니다.
