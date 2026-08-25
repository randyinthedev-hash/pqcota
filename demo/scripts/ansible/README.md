# scripts/ansible/: 디스커버리 오케스트레이션

`demo.sh`가 구동하는 Ansible 설정. 컨트롤러(pqcota-ctl)가 **SSH로 각 타깃 노드에 접속 → collector 실행 → 결과 JSON 회수**한다. 빌드 시 컨트롤러 이미지의 `/work/ansible`로 복사된다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../../docs/regulation.md)의 절 번호다.

| 파일 | 역할 |
|---|---|
| [`discover.yml`](../../../discovery/ansible/discover.yml) | **참조 플레이북**(이 폴더에 없다. `discovery/ansible/`이 원본, 이미지 빌드 때 여기로 복사된다). collector 반입→실행→회수→정리 4단계. JVM 애드온은 `--recon` 정찰 결과로 조건부 반입. 실환경 이식은 `collector_bin_dir`만 바꾸면 된다 |
| `groups.ini` | **그룹 멤버십 + 트래픽 시나리오만**(접속 정보 아님). 노드별 `traffic=`(관측 구간을 채울 핸드셰이크 대상).<br>**생성물**: `demo/topology/topology.yaml`에서 `topogen`이 만들어 `demo/.generated/`에 둔다(직접 편집하지 않는다) |
| `targets.ini` | **생성물(커밋 안 됨)**. `pqcota-hosts`가 사용자 `hosts.csv`에서 만든다. `[targets]`에 `ansible_host`·`ansible_user`·`ansible_ssh_private_key_file`(**접속 비밀·런타임 전용·미영속**) |
| `ansible.cfg` | host key·SSH 옵션만. **접속 user/key 기본값 없음**. 반드시 `targets.ini`(pqcota-hosts 산출)에서 온다 |

디스커버리는 두 인벤토리를 **병합**해 실행한다: `ansible-playbook -i targets.ini -i groups.ini discover.yml`. 접속(정체성·비밀)은 `targets.ini`, 시나리오(트래픽·그룹)는 `groups.ini`가 담당: 레인 분리(§1.5).

## 자기 호스트로 데모를 돌리려면: `hosts.csv`를 고친다(pqcota 인벤토리 아님)

실제 호스트 대상으로 바꾸는 진입점은 **사용자 관리 hosts 파일**이다:
- `hosts.csv`(헤더 `node_id,name,ip,port,ssh_user,ssh_key`)의 IP·사용자·키를 실 인프라로 교체 → `pqcota-hosts`가 `targets.ini`(접속 비밀 포함·런타임 전용) + 엔드포인트(비밀 제외) 인벤토리 upsert를 만든다.
- `groups.ini`의 `traffic="pqc:host:port ssl:host:port ssh:host:port"`: 관측 구간 동안 생성할 핸드셰이크 대상(없으면 `traffic=""`).
- **노드 ID는 plain 이름**(`web-gw`)을 쓴다. `node://...` 같은 접두어 금지. 인벤토리 대조가 스코프 마스터의 노드 ID와 정확히 일치해야 하기 때문이다.
- **접근 비밀(키·계정)은 `hosts.csv`(사용자 파일)에만**: pqcota 인벤토리엔 적재하지 않는다. discovery 실행 시마다 사용자가 이 파일을 지정한다.

## 알려진 요구사항 (gotcha)

- **jvmscan 환경**: JCA provider 체인 조회는 `JAVA_BIN`(실 java 경로)·`JVMSCAN_CP`(provider JAR, 예 bcprov)가 있어야 provider가 체인에 뜬다. 데모는 플레이북에서 주입한다. 실 호스트에선 해당 노드의 실제 경로로 맞춘다.
- **SSH 도달 전제**: 컨트롤러→타깃 키 배포가 선행돼야 한다(데모는 `up.sh`가 처리).
- **netcap 권한**: 관측은 `CAP_NET_RAW`가 필요하다(복호화 없이 핸드셰이크 평문만).
