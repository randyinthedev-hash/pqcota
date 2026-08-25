# provisioning/cmd/: 프로비저닝 진입점

프로비저닝 단계의 CLI(Go 바이너리). 확정 계획에서 **Ansible 플레이북을 생성**하고 **롤백 근거를 영속**한다. 세 범주로 나눠 정리한다.

플레이북의 **내용과 순서는 전부 생성기가 정한다.** 사용자가 하는 것은 그것을 자기 Ansible로 실행하는 일이다. 자체 원격 실행 엔진을 만들지 않는다.

## ① 생성: 확정 계획에서 플레이북을 만든다

### `pqcota-provision`

```
pqcota-provision [--level l1|l2|l3] [--rollback] [--dsn <postgres>] <plan.json>
```

| 인자·옵션 | 하는 일 |
|---|---|
| `<plan.json>` | 확정 계획(`FinalizedPlan`). **`PLAN_STATUS_FINALIZED`가 아니면 거부한다** |
| `--level l1` | **스테이지만**: 모듈을 타깃에 놓기까지 |
| `--level l2`(기본) | **설치까지**. 모듈 배치 + config 조각 배치 |
| `--level l3` | **활성화·재시작까지**. 계획의 `activation` 훅(`pre`·`activate`·`deactivate`·`restart`)을 의미 순서로 배치 |
| `--rollback` | 역방향 플레이북: forward가 배치한 파일을 제거한다 |
| `--dsn <postgres>` | 히스토리에서 before-findings를 읽어 **before 상태를 캡처**하고 append-only 레코드로 영속한다. 형식은 [DSN](../../discovery/cmd/README.md#pqcota-hosts) |

플레이북은 stdout으로 나온다. `> provision.yml`로 받는다.

**`--level l3`의 빈 훅은 지어내지 않는다.** 계획에 `activate`가 없으면 그 태스크를 만들지 않고 **무엇이 일어나지 않는지를 stderr로 고지한다**(예: 재시작 훅이 없으면 "새 provider가 로드되지 않을 수 있다"). 활성화 방법은 앱 기동 방식에 달려 있어 도구가 알 수 없다.

**`--dsn`이 하는 일은 기록이지 적용이 아니다.** 조치 *전* 상태(모듈@버전·config·provider 체인)를 캡처해 두는 것이라, 나중에 되돌릴 때 무엇으로 돌아가야 하는지를 밝히는 근거가 된다.

### 적용하기

```bash
pqcota-provision --level l2 plan.json > provision.yml
ansible-playbook -i targets.ini -e pqcota_module_sha256_oqsprovider=<sha256> provision.yml
```

`targets.ini`는 디스커버리에서 쓰던 것을 그대로 쓴다([`pqcota-hosts`](../../discovery/cmd/README.md#pqcota-hosts)).

**provider 모듈은 도구가 주지 않는다.** 플레이북은 컨트롤러의 `files/<모듈파일>`을 타깃으로 복사하므로, 그 파일을 사용자가 거기 둬야 한다.

| 변수 | 하는 일 |
|---|---|
| `pqcota_module_src_<provider>` | 그 모듈의 컨트롤러 로컬 경로. 없으면 `pqcota_module_src`, 그것도 없으면 `files/<모듈파일>` |
| `pqcota_module_sha256_<provider>` | **무결성 게이트**: 배치 후 타깃에서 sha256을 재고 다르면 중단한다. 없으면 `pqcota_module_sha256`, 둘 다 없으면 확인을 건너뛴다 |

`<provider>`는 계획의 `providerChoice`에서 **영숫자만 남기고 나머지를 `_`로** 바꾼 것이다(`acme-pqc` → `acme_pqc`): Ansible 변수명 규칙이라 그렇다. 하이픈을 그대로 주면 **변수가 인식되지 않아 검사가 조용히 건너뛰어진다.**

해시는 **복사 후 타깃에서** 잰다. 컨트롤러의 원본이 아니라 실제로 노드에 놓인 파일을 재므로 전송 손상·경로 착오도 함께 잡힌다. 불일치면 그 노드에서 중단한다.

sha256을 주는 것을 권한다. 타깃에서 암호 연산을 할 네이티브 코드를 심는 일이라 **무엇을 심었는지 고정할 수단**이 필요하다. 주지 않으면 오류가 아니라 **검사 태스크가 통째로 skip된다.**

### 되돌리기

```bash
pqcota-provision --level l2 --rollback plan.json > provision-rollback.yml
ansible-playbook -i targets.ini provision-rollback.yml
```

적용이 원본을 덮지 않고 파일을 *추가*하므로 그 추가분 제거가 곧 복원이다. L3면 `deactivate` 훅으로 활성화까지 되돌린다.

## ② 조회: 롤백 근거를 읽는다

### `pqcota-records`

```
pqcota-records [node]
```

| 인자 | 하는 일 |
|---|---|
| `[node]` | 그 노드만. 생략하면 전부 |

`env PQCOTA_DSN` 필수: `pqcota-provision --dsn`이 쓴 그 저장소를 읽는다. id·상태·영향 앱·before/after 모듈을 나열한다. **읽기전용이라 상태를 바꾸지 않는다.**

## ③ 입력은 어디서 오나. 확정 계획

**이 리포는 계획을 만들지 않는다. 읽기만 한다.** `FinalizedPlan`은 공개 계약(`plan.proto`)이라 JSON으로 직접 작성한다. 조치 종류·런타임별 견본과 필드 설명이 [`examples/provisioning/plans/`](../../examples/provisioning/plans/README.md)에 있다. 가장 가까운 것을 골라 `targetNodeId`·경로·provider를 자기 것으로 바꾸면 된다.

**`status`가 `PLAN_STATUS_FINALIZED`여야 한다**. 아니면 거부한다. 확정되지 않은 계획으로 배포하는 일을 막는 게이트다.

`--dsn`을 줄 때 읽는 before-findings와 `app_keys`는 인벤토리에 쌓인 히스토리(`pqcota-inventory`가 읽는 그 저장소)에서 온다. → [inventory/cmd 커맨드 지도](../../inventory/cmd/README.md)

---

**언제 무엇을 쓰나**
- 계획을 받아 적용 아티팩트 만들기 → **①**. `--level`로 어디까지 갈지 정한다.
- 조치 후 되돌리기 → **①** 같은 계획에 `--rollback`.
- 무엇이 어떤 before로 스테이징됐나 → **②**.

> 로직은 `pkg/provisioning/`(계획 게이트·taxonomy→config 생성기·`GenerateProvisioningPlaybook`·`CaptureState`·`RecordStore` Mem/Pg)에 있고, 이 커맨드들은 그걸 조립하는 얇은 진입점이다.

설계: [프로비저닝 설계](../design.md).
