[English](README.md) · 한국어

# examples/: 실행 가능한 예제가 있는 곳

예제는 명령 하나씩 실행해 보며 각 명령의 사용법을 익히는 최소한의 실행입니다. 단계별 예제는 이제 **그 예제가 실행하는 명령이 있는 단계의 리포지터리**에 있어, 명령을 바꾸는 사람이 바로 옆에서 예제를 찾을 수 있습니다. 데모(`demo/`)가 컨테이너로 전체 흐름을 처음부터 끝까지 보여 준다면, 예제는 **최소한의 준비로 한 단계의 명령만** 실행합니다.

| 단계 | 위치 | 실행하는 것 | 사전 준비 |
|---|---|---|---|
| **관측(Discovery)** | [pqcota-discovery `examples/discovery`](https://github.com/randyinthedev-hash/pqcota-discovery/tree/main/examples/discovery) | `pqcota-hosts`(호스트 → Ansible과 엔드포인트) · `pqcota-ingest`(① 직접 관측 적재) | Go만 |
| ↳ JVM | [pqcota-discovery `examples/discovery/jvm`](https://github.com/randyinthedev-hash/pqcota-discovery/tree/main/examples/discovery/jvm) | `pqcota-jvmscan` 정찰 → attach(실행 중인 JVM의 동적 등록) | **Go + Docker + JDK** |
| **인벤토리** | [pqcota-inventory `examples/inventory`](https://github.com/randyinthedev-hash/pqcota-inventory/tree/main/examples/inventory) | `pqcota-discover-view` · `pqcota-declare-attribution` · `pqcota-cbom-ingest` | Go만 |
| **전환물 생성(Provisioning)** | [pqcota-provisioning `examples/provisioning`](https://github.com/randyinthedev-hash/pqcota-provisioning/tree/main/examples/provisioning) | `pqcota-provision`(확정된 계획 → L2/L3 플레이북), `pqcota-approve`, 되돌림 | Go만 |

관측 예제와 인벤토리 예제가 함께 쓰는 수집 결과 샘플은 [pqcota-inventory `examples/data`](https://github.com/randyinthedev-hash/pqcota-inventory/tree/main/examples/data)에 있습니다. 그것을 읽는 쪽이 인벤토리이기 때문입니다.

## 사전 준비

- **Go 툴체인.** 생성된 계약 코드는 `pqcota-common`에 커밋되어 있으므로 클론한 직후 `go run`으로 소스에서 바로 실행할 수 있고, Postgres나 대상 노드는 필요 없습니다. proto를 바꿨다면 먼저 `pqcota-common`에서 `make generate`를 실행하세요(buf가 필요합니다).
- **한 가지 예외:** JVM 예제는 **실행 중인 JVM**이 있어야 하므로 **Docker**를 씁니다(JDK는 컨테이너 안에 있습니다).
- [빌드 안내](../docs/build.ko.md#소스-받기)에 따라 리포지터리들을 나란히 클론하세요. 예제는 해당 리포지터리의 루트에서 실행합니다. 예를 들면 다음과 같습니다.

```bash
cd ../pqcota-inventory
./examples/inventory/run.sh
```

## 처음부터 끝까지의 흐름

Postgres 영속화, 실제 Ansible/SSH 연결, 실제 스캔까지 이어지는 전체 흐름은 [demo/](../demo)(Docker, 여섯 단계)를 보세요.
