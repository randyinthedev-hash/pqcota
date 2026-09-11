# provisioning/cmd/: 프로비저닝 진입점

프로비저닝 단계의 CLI(Go 바이너리). 확정 계획에 **승인 서명을 붙이고**, 그 계획에서 **Ansible 플레이북을 생성**하며 **롤백 근거를 영속**한다. 네 범주로 나눠 정리한다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

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
| `--allow-incomplete` | 계획에 빈칸이 있어도 **0으로 끝낸다**. 경고는 그대로 나온다 |
| `--allow-unverified-approvals` | 확인할 키가 없어도 **진행한다**. 기본은 거절이다 |
| `--dsn <postgres>` | 히스토리에서 before-findings를 읽어 **before 상태를 캡처**하고 append-only 레코드로 영속한다. 형식은 [DSN](../../discovery/cmd/README.md#pqcota-hosts) |

| 환경변수 | 하는 일 |
|---|---|
| `PQCOTA_APPROVAL_KEYS` | `<승인자>=<base64 공개키>` 콤마 구분. 승인 서명을 **그 승인자의 키로** 검증한다. 하나라도 어긋나면 거절하고, 확인된 승인이 하나도 없어도 거절한다. **비어 있어도 거절한다** |
| `PQCOTA_REQUIRE_APPROVAL` | `1`은 그대로 받는다. 이제 기본과 같은 뜻이라 아무것도 바꾸지 않는다 |

**확인할 키가 없으면 거절한다.** 승인은 책임의 소재인데, 아무 문자열이나 그 자리를 채울 수 있으면 그 자리는 비어 있는 것과 같다. 전에는 경고하고 통과시켰고 닫는 것은 `PQCOTA_REQUIRE_APPROVAL=1`을 따로 건 배포에서만 일어났는데, 그러면 승인 무결성이 「닫을 수 있는 수단」에 머물고 기본 경로는 열린 채다.

알고 여는 길은 `--allow-unverified-approvals` 하나다. **명령줄에 적어야 열린다.** 환경변수로도 열리면 무엇이 검증됐는지가 셸 설정에 숨어, 로그만 보고는 이 산출물이 확인된 승인 위에 섰는지 알 수 없다. 열어도 「확인하지 않았다」는 그대로 나온다 — 넘기는 것이지 확인한 것이 아니다.

서명은 [`pqcota-approve`](#pqcota-approve)가 붙이고 키쌍은 `pqcota-keygen`이 낸다. [예제 실행기](../../examples/provisioning/README.md)가 그 경로를 그대로 밟는다.

**`--level`은 계획이 말하지 않은 조치의 기본값이다.** 위임 수준은 계약이 정한 **자산별 속성**이라(§4.3 "결제 서버=L2, 무상태 워커=L3"), 조치가 `automation_level`을 말하면 그것을 따른다. 그 값은 승인 서명이 덮는 것이라, 전역 플래그로 덮어쓰면 승인자가 서명한 위임 수준과 실제 실행 수준이 갈린다.

플레이북은 stdout으로 나온다. `> provision.yml`로 받는다.

**빈칸이 남은 계획은 성공으로 끝나지 않는다.** 산출물은 그대로 내지만 종료 상태가 **3**이다. 생성물이 stdout으로 먼저 나가고 경고는 뒤에 stderr로 나가므로, 종료 상태까지 0이면 stderr를 모으지 않는 자동화에서 **불완전한 플레이북이 정상 산출물로 남는다.** 막지 않는 이유는 사람이 손으로 채우는 것이 정당한 경로여서다. 알고 넘길 때는 `--allow-incomplete`를 적는다.

| 종료 상태 | 뜻 |
|---|---|
| `0` | 산출물이 나왔고 빈칸이 없다. `--dsn`을 주면 조치마다 스냅샷 참조를 이력에서 찾아 레코드에 남긴다(`pqcota-records`가 `snapshot:` 줄로 보인다) |
| `1` | **거절**: 이 계획은 실행 근거가 아니다(확정 아님·승인 없음·조치 없음·승인을 확인할 수 없음·승인 검증 실패). 플레이북은 한 줄도 나오지 않는다 |
| `3` | **불완전**: 산출물은 나왔지만 빈칸이 있다. 목표 알고리즘·활성화 훅·추적 근거가 비었거나, **스냅샷 참조의 모양이 틀렸거나**(`--dsn` 없이도 잡힌다), **`--dsn`으로 찾았는데 이력에 없거나 찾은 스냅샷에 그 finding 이 없다.** 어느 것인지 stderr 가 이름으로 말한다 |
| `2` | 사용법이 틀렸다 |

`--rollback`도 같은 검사를 거친다. 파일을 지우는 일에 목표 알고리즘은 상관이 없어 그 경고는 내지 않지만, `deactivate`·`restart`가 없어 활성화를 되돌리지 못하는 것과 무엇을 되돌리는지 되짚을 근거가 없는 것은 정방향과 같은 무게로 본다.

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

## ② 승인: 계획에 서명을 붙인다

### `pqcota-approve`

```
pqcota-approve --approver <id> <plan.json>
```

| 인자·옵션 | 하는 일 |
|---|---|
| `--approver <id>` | 승인자 id. **`:`는 쓸 수 없다** — 서명 문자열의 구분자다 |
| `env PQCOTA_APPROVAL_KEY` | base64 ed25519 **개인키**. [`pqcota-keygen`](../../discovery/cmd/README.md)이 낸 것 |

서명한 계획이 stdout으로 나온다. 서명 문자열의 꼴은 `<승인자>:ed25519:<base64>`이고, **승인 서명 자신을 뺀 계획 전부**를 덮는다.

**첫 승인이 계획을 확정한다.** 판정을 끝낸 계획은 `status=IN_REVIEW`에 승인 칸과 확정 시각이 빈 채로 온다. 이 명령이 상태를 `FINALIZED`로 올리고 `finalized_at`을 찍은 **뒤에** 서명한다 — 서명이 그 둘을 덮기 때문에 순서가 바뀌면 방금 만든 서명이 깨진다. 두 번째 승인부터는 아무것도 바꾸지 않고 서명만 더한다.

| 들어온 상태 | 하는 일 |
|---|---|
| `IN_REVIEW` | 승인과 확정 시각이 **없어야** 한다. 있으면 손상으로 거절한다. 조치가 있고 조치마다 대상 노드와 종류가 있어야 한다. 지나면 `FINALIZED`로 올리고 시각을 찍고 서명한다 |
| `FINALIZED` | 승인과 확정 시각이 **모두 있어야** 한다. 하나라도 없으면 손상으로 거절한다. 조치 구조도 `IN_REVIEW`와 같이 본다. 갖췄으면 서명만 더한다 |
| `DRAFT` · `UNSPECIFIED` | 거절한다 |

거절하면 stdout에 아무것도 내지 않는다. 종료 상태는 `1`(거절)·`2`(사용법)다.

**승인자 id가 서명 안에 들어가는 이유**는 검증할 때 그 사람의 키로만 확인하기 위해서다. 키를 목록으로 받으면 어느 키로든 통과한 서명이 아무 이름이나 달고 들어와, 서명이 「누군가 승인했다」까지만 답하게 된다.

**서명한 뒤에 계획을 고치면 승인은 무효가 된다.** 그것이 요점이다. 승인자는 계획의 이름이 아니라 조치의 내용에 책임을 진다.

```bash
pqcota-keygen                                     # 승인자 키쌍
PQCOTA_APPROVAL_KEY=<priv> pqcota-approve --approver reviewer-1 plan.json > plan.signed.json
PQCOTA_APPROVAL_KEYS=reviewer-1=<pub> pqcota-provision --level l2 plan.signed.json > provision.yml
```

## ③ 조회: 롤백 근거를 읽는다

### `pqcota-records`

```
pqcota-records [node]
```

| 인자 | 하는 일 |
|---|---|
| `[node]` | 그 노드만. 생략하면 전부 |

`env PQCOTA_DSN` 필수: `pqcota-provision --dsn`이 쓴 그 저장소를 읽는다. id·상태·영향 앱·before/after 모듈을 나열한다. **읽기전용이라 상태를 바꾸지 않는다.**

## ④ 입력은 어디서 오나. 확정 계획

**이 리포는 계획을 만들지 않는다. 읽기만 한다.** `FinalizedPlan`은 공개 계약(`plan.proto`)이라 JSON으로 직접 작성한다. 조치 종류·런타임별 견본과 필드 설명이 [`examples/provisioning/plans/`](../../examples/provisioning/plans/README.md)에 있다. 가장 가까운 것을 골라 `targetNodeId`·경로·provider를 자기 것으로 바꾸면 된다.

**`status`가 `PLAN_STATUS_FINALIZED`여야 한다**. 아니면 거부한다. 확정되지 않은 계획으로 배포하는 일을 막는 게이트다.

`--dsn`을 줄 때 읽는 before-findings와 `app_keys`는 인벤토리에 쌓인 히스토리(`pqcota-inventory`가 읽는 그 저장소)에서 온다. → [inventory/cmd 커맨드 지도](../../inventory/cmd/README.md)

---

**언제 무엇을 쓰나**
- 계획을 받아 적용 아티팩트 만들기 → **①**. `--level`로 어디까지 갈지 정한다.
- 조치 후 되돌리기 → **①** 같은 계획에 `--rollback`.
- 계획에 승인 서명 붙이기 → **②**. 계획을 다 고친 **뒤에** 한다.
- 무엇이 어떤 before로 스테이징됐나 → **③**.

> 로직은 `pkg/provisioning/`(계획 게이트·taxonomy→config 생성기·`GenerateProvisioningPlaybook`·`CaptureState`·`RecordStore` Mem/Pg)에 있고, 이 커맨드들은 그걸 조립하는 얇은 진입점이다.

설계: [프로비저닝 설계](../design.md).
