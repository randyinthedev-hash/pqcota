# discovery/ansible/ — 참조 플레이북

준비된 노드들에서 collector를 한꺼번에 돌리는 **참조 구현**이다. 데모가 이걸 그대로 쓰고, 자기 인프라에도 그대로 이식하라고 둔 것이다.

```bash
pqcota-hosts --ansible-out targets.ini hosts.csv     # ① 접근 준비
ansible-playbook -i targets.ini discovery/ansible/discover.yml
```

| 파일 | 하는 일 |
|---|---|
| [`discover.yml`](discover.yml) | **반입 → 실행 → 회수 → 정리** — collector를 스테이징 디렉터리에 올려 돌리고, 결과 JSON을 컨트롤러로 가져온 뒤 노드에서 지운다 |

**노드 OS로 갈린다.** `gather_facts`가 알려 주는 `os_family`로 리눅스면 collector 셋, Windows면 `pqcota-cngscan`을 돌린다. 어느 쪽도 아니면 아무것도 하지 않는다 — 결과가 없을 뿐, "그 노드에 아무것도 없다"가 아니다.

**Windows 노드를 돌리려면 둘이 더 필요하다:**

```bash
ansible-galaxy collection install ansible.windows            # win_copy·win_command 모듈
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows-amd64/ ./discovery/cmd/pqcota-cngscan
```

접속 방법은 `hosts.csv`의 `connection` 열이 정한다(`ssh` 또는 `winrm`) — `targets.ini`는 매 실행 덮어써지므로 손으로 더한 설정은 남지 않는다 → [작성법](../../examples/discovery/README.md). 사이트마다 갈리는 값(WinRM transport·인증서 검증, sshd 기본 셸이 cmd인 경우)만 `group_vars/targets_windows.yml`에 둔다.

> **이 경로는 데모가 덮지 못한다.** 데모는 리눅스 컨테이너뿐이라 Windows 분기는 매 실행 확인되는 게이트가 없다. 리눅스 쪽과 달리 "돌려 봤더니 됐다"는 근거가 얇다.

**노드에 아무것도 남기지 않는다.** collector는 상주 에이전트가 아니라 실행 후 종료하는 CLI라 이 일회성 패턴이 맞다.

**JVM 애드온(`collector.jar`)은 모든 노드에 뿌리지 않는다** — `pqcota-jvmscan --recon`으로 그 노드에 JVM이 있는지 먼저 보고, 있는 노드에만 보낸다.

## 자기 인프라에 쓰려면

`collector_bin_dir`(리눅스)·`collector_bin_dir_win`(Windows)을 자기 빌드 산출로 바꾸면 된다 → [루트 README · 빌드](../../README.md#빌드). 리눅스 블록에만 `become: true`가 붙는 이유는 `pqcota-netcap`의 `CAP_NET_RAW`와 `/proc` 전 프로세스 커버리지다.

**fleet 규모로 플레이북을 찍어 주는 생성기는 없다.** 자체 원격 실행 엔진을 만들지 않고 사용자의 기존 substrate(Ansible·Salt 등)가 실행하는 모델이다 → [collector 배포 설계](../collector-deployment.md).
