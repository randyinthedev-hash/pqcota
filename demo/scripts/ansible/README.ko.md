[English](README.md) · 한국어

# scripts/ansible/: 관측 오케스트레이션

`demo.sh`가 구동하는 Ansible 구성입니다. 컨트롤러(pqcota-ctl)가 **대상 노드마다 SSH로 연결 → 수집기 실행 → 결과 JSON 가져오기**를 합니다. 빌드할 때 컨트롤러 이미지의 `/work/ansible`로 복사됩니다.

| 파일 | 역할 |
|---|---|
| [`discover.yml`](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/ansible/discover.yml) | **참조 플레이북**(이 폴더에는 없습니다. 원본은 `pqcota-discovery`의 `ansible/`에 있고 이미지를 빌드할 때 여기로 복사됩니다). 네 단계입니다. 수집기 배포 → 실행 → 가져오기 → 정리. JVM 추가 구성요소는 `--recon` 정찰 결과에 따라 조건부로 배포됩니다. 실제 환경으로 옮기려면 `collector_bin_dir`만 바꾸면 됩니다 |
| `groups.ini` | **그룹 소속과 트래픽 시나리오만**(접속 정보가 아닙니다). 노드별 `traffic=`(관측 창을 채우는 핸드셰이크 대상).<br>**생성됨**: `topogen`이 `demo/topology/topology.yaml`로 만들어 `demo/.generated/`에 두므로 손으로 고치지 마세요 |
| `targets.ini` | **생성됨(커밋하지 않음).** `pqcota-hosts`가 사용자의 `hosts.csv`로 만듭니다. `[targets]`에 `ansible_host`, `ansible_user`, `ansible_ssh_private_key_file`이 들어갑니다(**접속 비밀: 실행 중에만 존재하며 저장하지 않음**) |
| `ansible.cfg` | 호스트 키와 SSH 옵션만 있습니다. **기본 접속 사용자나 키는 없습니다.** 항상 `targets.ini`(pqcota-hosts의 출력)에서 옵니다 |

관측은 두 인벤토리를 **합쳐서** 돌립니다. `ansible-playbook -i targets.ini -i groups.ini discover.yml`. 접속(신원과 비밀)은 `targets.ini`가, 시나리오(트래픽과 그룹)는 `groups.ini`가 맡으므로 두 레인이 나뉘어 있습니다.

## 데모를 사용자의 호스트에 돌리려면: `hosts.csv`를 고칩니다(pqcota 인벤토리가 아닙니다)

실제 호스트로 바꾸는 진입점은 **사용자가 관리하는 호스트 파일**입니다.
- `hosts.csv`(헤더 `node_id,name,ip,port,ssh_user,ssh_key`)의 IP, 사용자, 키를 실제 인프라 것으로 바꿉니다 → `pqcota-hosts`가 `targets.ini`(접속 비밀 포함, 실행 중에만 존재)와 엔드포인트의 인벤토리 upsert(비밀 제외)를 만듭니다.
- `groups.ini`의 `traffic="pqc:host:port ssl:host:port ssh:host:port"`: 관측 창 동안 만들어 낼 핸드셰이크 대상입니다(없으면 `traffic=""`).
- **노드 ID는 평범한 이름으로 씁니다**(`web-gw`). `node://...` 같은 접두어는 쓰지 않습니다. 인벤토리 비교가 범위 마스터의 노드 ID와 정확히 일치해야 하기 때문입니다.
- **접근 비밀(키와 계정)은 `hosts.csv`(사용자의 파일)에만 있습니다.** pqcota 인벤토리에는 절대 적재되지 않습니다. 관측을 돌릴 때마다 사용자가 이 파일을 가리켜 줍니다.

## 알려진 요구 사항(주의점)

- **jvmscan 환경**: JCA provider 체인을 조회하려면 `JAVA_BIN`(실제 java 경로)과 `JVMSCAN_CP`(bcprov 같은 provider JAR)가 있어야 provider가 체인에 나타납니다. 데모는 플레이북에서 주입합니다. 실제 호스트에서는 그 노드의 실제 경로로 설정하세요.
- **SSH에 닿아야 합니다**: 컨트롤러에서 대상으로 가는 키 배포가 먼저 이뤄져야 합니다(데모에서는 `up.sh`가 처리합니다).
- **netcap 권한**: 관측에는 `CAP_NET_RAW`가 필요합니다(핸드셰이크 평문만 보며 복호화는 하지 않습니다).
