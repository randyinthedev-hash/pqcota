# discovery/ansible/: 참조 플레이북

준비된 노드들에서 collector를 한꺼번에 돌리는 **참조 구현**이다. 데모가 이걸 그대로 쓰고, 자기 인프라에도 그대로 이식하라고 둔 것이다.

```bash
pqcota-hosts --ansible-out targets.ini hosts.csv     # ① 접근 준비
ansible-playbook -i targets.ini discovery/ansible/discover.yml           # 자산
ansible-playbook -i targets.ini discovery/ansible/discover_traffic.yml   # 통신 엣지
```

| 파일 | 하는 일 |
|---|---|
| [`discover.yml`](discover.yml) | **자산**을 본다(`nodescan`·`jvmscan`·`cngscan`). 반입 → 실행 → 회수 → 정리 |
| [`discover_traffic.yml`](discover_traffic.yml) | **통신 엣지**를 본다(`netcap`). 같은 네 단계이되 **실행을 회차로 반복**한다 |

**둘로 가른 이유는 관측 주기가 다르기 때문이다.** 자산은 좀처럼 바뀌지 않아 정기적으로 한 번 보면 되지만, 통신 엣지는 캡처하는 구간에 흐른 것만 잡히므로 더 자주·더 길게 봐야 채워진다. 한 플레이북에 묶여 있으면 엣지를 더 보려고 `/proc` 훑기와 JVM attach까지 함께 다시 돌려야 했다.

`discover_traffic.yml`이 받는 값은 다섯이다.

| 변수 | 기본 | 무엇 |
|---|---|---|
| `netcap_runs` | `1` | 몇 회 관측하나 |
| `netcap_window` | `8`(`window` 변수가 있으면 그 값) | 한 회차의 관측 구간(초) |
| `netcap_interval` | `0` | 회차 사이에 쉬는 시간(초) |
| `netcap_budget` | `0`(제한 없음) | 전체 상한(초). 넘으면 남은 회차를 시작하지 않고 **몇 회를 걸렀는지 고지한다** |
| `netcap_cleanup` | `true` | 끝나고 노드를 원래대로 되돌릴지. 끄면 바이너리가 남아 다음 회차의 반입을 건너뛴다 |

```bash
# 10분마다 60초씩, 한 시간을 넘기지 않는다
ansible-playbook -i targets.ini discovery/ansible/discover_traffic.yml \
  -e netcap_runs=6 -e netcap_window=60 -e netcap_interval=540 -e netcap_budget=3600
```

회차마다 결과가 `<node>-net-01.json`처럼 따로 나온다. 각각이 제 `collected_at`을 지닌 별개의 `CollectionResult`이고, 같은 엣지를 다시 봐도 스냅샷은 늘지 않는다(인벤토리 설계 §7.2·§7.3).

**노드 OS로 갈린다.** `gather_facts`가 알려 주는 `os_family`로 리눅스면 collector 셋, Windows면 `pqcota-cngscan`·`pqcota-jvmscan` 둘을 돌린다. 어느 쪽도 아니면 아무것도 하지 않는다. 결과가 없을 뿐, "그 노드에 아무것도 없다"가 아니다.

**Windows 노드를 돌리려면 둘이 더 필요하다:**

```bash
ansible-galaxy collection install ansible.windows            # win_copy·win_command 모듈
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows-amd64/ \
  ./discovery/cmd/pqcota-cngscan ./discovery/cmd/pqcota-jvmscan
```

Windows 노드는 **관리자 계정으로 접속해야** 다른 사용자로 도는 JVM까지 보인다(실측: 일반 사용자 265개 중 163개를 못 열었다). 리눅스 블록의 `become: true`에 해당하는 자리다.

접속 방법은 `hosts.csv`의 `connection` 열이 정한다(`ssh` 또는 `winrm`): `targets.ini`는 매 실행 덮어써지므로 손으로 더한 설정은 남지 않는다 → [작성법](../../examples/discovery/README.md). 사이트마다 갈리는 값(WinRM transport·인증서 검증, sshd 기본 셸이 cmd인 경우)만 `group_vars/targets_windows.yml`에 둔다.

> **한 번은 실물로 돌렸다**(TD-WIN-1·2): Win32-OpenSSH + 키로 붙어 반입·관측·회수·정리가 끝까지 돌았고 노드엔 아무것도 남지 않았다. 다만 **데모가 이 경로를 덮지는 못한다**. 데모는 리눅스 컨테이너뿐이라 Windows 분기에는 매 실행 확인되는 게이트가 없다.

**기본값은 노드에 아무것도 남기지 않는다.** collector는 상주 에이전트가 아니라 실행 후 종료하는 CLI라 이 일회성 패턴이 맞다. `netcap_cleanup=false`로 바이너리를 남길 수는 있지만 **상주하는 것은 프로세스가 아니라 파일뿐**이고, 일정의 주인은 여전히 이 플레이북을 부르는 쪽이다.

**JVM 애드온(`collector.jar`)은 모든 노드에 뿌리지 않는다**. `pqcota-jvmscan --recon`으로 그 노드에 JVM이 있는지 먼저 보고, 있는 노드에만 보낸다.

## 자기 인프라에 쓰려면

`collector_bin_dir`(리눅스)·`collector_bin_dir_win`(Windows)을 자기 빌드 산출로 바꾸면 된다 → [루트 README · 빌드](../../README.md#빌드). `discover.yml`의 리눅스 블록에 `become: true`가 붙는 이유는 `/proc` 전 프로세스 커버리지이고, `discover_traffic.yml`은 `pqcota-netcap`의 `CAP_NET_RAW` 때문이다.

**fleet 규모로 플레이북을 찍어 주는 생성기는 없다.** 자체 원격 실행 엔진을 만들지 않고 사용자의 기존 substrate(Ansible·Salt 등)가 실행하는 모델이다 → [collector 배포 설계](../collector-deployment.md).
