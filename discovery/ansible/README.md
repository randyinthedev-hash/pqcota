# discovery/ansible/ — 참조 플레이북

준비된 노드들에서 collector를 한꺼번에 돌리는 **참조 구현**이다. 데모가 이걸 그대로 쓰고, 자기 인프라에도 그대로 이식하라고 둔 것이다.

```bash
pqcota-hosts --ansible-out targets.ini hosts.csv     # ① 접근 준비
ansible-playbook -i targets.ini discovery/ansible/discover.yml
```

| 파일 | 하는 일 |
|---|---|
| [`discover.yml`](discover.yml) | **반입 → 실행 → 회수 → 정리** — collector 셋을 `/tmp/pqcota-collector`에 올려 돌리고, 결과 JSON을 컨트롤러로 가져온 뒤 노드에서 지운다 |

**리눅스 노드 전용이다.** 리눅스 collector 셋만 돌린다 — `become`·`/tmp` 스테이징·`copy`가 POSIX를 전제한다. Windows 노드(`pqcota-cngscan`)를 이 경로로 돌리는 방법은 아직 없다.

**노드에 아무것도 남기지 않는다.** collector는 상주 에이전트가 아니라 실행 후 종료하는 CLI라 이 일회성 패턴이 맞다.

**JVM 애드온(`collector.jar`)은 모든 노드에 뿌리지 않는다** — `pqcota-jvmscan --recon`으로 그 노드에 JVM이 있는지 먼저 보고, 있는 노드에만 보낸다.

## 자기 인프라에 쓰려면

`collector_bin_dir`를 자기 빌드 산출(`dist/linux-amd64` 등)로 바꾸면 된다 → [루트 README · 빌드](../../README.md#빌드). `become: true`가 필요한 이유는 `pqcota-netcap`의 `CAP_NET_RAW`와 `/proc` 전 프로세스 커버리지다.

**fleet 규모로 플레이북을 찍어 주는 생성기는 없다.** 자체 원격 실행 엔진을 만들지 않고 사용자의 기존 substrate(Ansible·Salt 등)가 실행하는 모델이다 → [collector 배포 설계](../collector-deployment.md).
