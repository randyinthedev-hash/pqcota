# plans/ — 계획(`FinalizedPlan`) 견본

`pqcota-provision`의 **입력**이다. 이 리포는 계획을 만들지 않고 읽기만 하므로, 쓰려면 사용자가 직접 작성한다. 여기 견본에서 가장 가까운 것을 골라 `targetNodeId`·경로·provider를 자기 것으로 바꾸면 된다.

```bash
pqcota-provision --level l2 plans/openssl-3.5-config-only.json > provision.yml
./run.sh openssl-3.5-config-only          # 예제 러너로 산출물만 보기
```

**무엇이 생성되는지**는 [상위 README](../README.md)에 케이스별로 있다. 이 문서는 **JSON 자체가 어떻게 생겼는지**를 다룬다.

## 견본 한눈에

가르는 것은 `kind`(조치 종류)와 `cryptoRuntime`이다. 나머지 필드는 그 조합이 요구하는 것만 채운다.

| 파일 | `cryptoRuntime` | `kind` | 특징적인 필드 |
|---|---|---|---|
| [`openssl-3.5-config-only`](openssl-3.5-config-only.json) | OPENSSL | `CONFIG_ONLY` | 없음 — config 한 줄이면 끝나므로 provider를 지정하지 않는다 |
| [`openssl-3.0-provider-inject`](openssl-3.0-provider-inject.json) | OPENSSL | `PROVIDER_INJECT` | `providerChoice: oqsprovider` |
| [`openssl-1.1.1-fork-replace`](openssl-1.1.1-fork-replace.json) | OPENSSL | `FORK_REPLACE` | 없음 — config로 못 하는 조치라 주석만 남는다 |
| [`jca-native-config-only`](jca-native-config-only.json) | JCA | `CONFIG_ONLY` | 없음 |
| [`jca-provider-inject-bc`](jca-provider-inject-bc.json) | JCA | `PROVIDER_INJECT` | `providerChoice: BC` |
| [`jca-fips-bcfips`](jca-fips-bcfips.json) | JCA | `PROVIDER_INJECT` | `providerChoice: BCFIPS` — 등록 클래스가 달라진다 |
| [`jca-eol-jdk-upgrade`](jca-eol-jdk-upgrade.json) | JCA | `JDK_UPGRADE` | 없음 — 생성물 없음 |
| [`custom-openssl-provider`](custom-openssl-provider.json) | OPENSSL | `PROVIDER_INJECT` | `providerChoice: acme-pqc` — 알려지지 않은 이름 |
| [`custom-jca-provider`](custom-jca-provider.json) | JCA | `PROVIDER_INJECT` | `providerClass`에 FQCN 명시 |
| [`custom-jca-missing-class`](custom-jca-missing-class.json) | JCA | `PROVIDER_INJECT` | 같은 것에서 `providerClass`를 **뺐을 때** |
| [`l3-activation-hooks`](l3-activation-hooks.json) | JCA | `PROVIDER_INJECT` | **`activation`** 네 훅 전부 |
| [`l3-hooks-missing`](l3-hooks-missing.json) | JCA | `PROVIDER_INJECT` | `activation` **없음** — 무엇이 안 일어나는지 고지된다 |
| [`signature-algorithm`](signature-algorithm.json) | OPENSSL | `CONFIG_ONLY` | `targetAlgorithm`이 **서명**(ML-DSA) |
| [`00-basic-two-actions`](00-basic-two-actions.json) | 둘 다 | `PROVIDER_INJECT` ×2 | 노드 둘 — 노드별 play로 갈린다 |

## 필드

### 계획 최상위

| 필드 | 필수 | 하는 일 |
|---|---|---|
| `id` | ✅ | 계획 식별자 |
| `status` | ✅ | **`PLAN_STATUS_FINALIZED`가 아니면 거부한다.** 확정되지 않은 계획으로 배포하는 일을 막는 게이트 |
| `scope` | | 계획의 적용 범위 라벨(예: `ring-0`) |
| `approvalSignatures` | | 승인 기록 |
| `actions` | ✅ | 조치 목록. **노드별로 play가 갈린다** |

### 조치(`actions[]`)

| 필드 | 필수 | 하는 일 |
|---|---|---|
| `id` | ✅ | 조치 식별자. 경고 메시지가 이 값으로 어느 조치인지 가리킨다 |
| `targetNodeId` | ✅ | 이 조치가 갈 노드. 플레이북의 `hosts:`가 된다 |
| `findingId` | | 근거가 된 관측. 인벤토리의 자산과 잇는다 |
| `cryptoRuntime` | ✅ | `CRYPTO_RUNTIME_OPENSSL` \| `CRYPTO_RUNTIME_JCA` — config 조각의 문법을 가른다 |
| `kind` | ✅ | 조치 종류(아래) |
| `targetAlgorithm` | | 목표 알고리즘. KEM이면 하이브리드 그룹 줄이 나가고, **서명이면 그룹 줄 대신 주석**이 나간다 |
| `providerChoice` | `PROVIDER_INJECT`면 | 넣을 provider 이름 — **이 값이 파일명이 된다**(아래) |
| `providerClass` | JCA 커스텀이면 | `java.security`에 적을 FQCN. **없으면** 알려진 이름(BC·BCFIPS)만 확정하고 그 외는 placeholder + 경고 |
| `rollbackNote` | | 되돌림 메모 |
| `activation` | `--level l3`면 | 활성화 훅(아래) |

### `providerChoice` — 이름이 파일명이 된다

| | OpenSSL | JCA |
|---|---|---|
| 이 값이 되는 것 | 파일명 `<이름>.so`, 배치 경로 `/opt/pqcota/<이름>.so` | 파일명 `<이름>.jar` **+ 등록 클래스 결정** |
| 아무 이름이나 되나 | **된다** — config가 참조하는 것이 경로뿐이라 이름은 자유다 | **아니다** — 아는 이름이 아니면 `providerClass`(FQCN)를 함께 줘야 한다 |
| 이름에서 클래스를 아는 것 | (해당 없음) | `BC` · `BCFIPS`(=`BC-FJA`) |
| 비우면 | `provider.so` | `BC`로 본다 |

JCA에서 아는 이름이 아닌데 `providerClass`도 없으면, 등록 줄이 `<이름: provider 문서의 정식 클래스명 확인>` placeholder로 나가고 경고가 함께 뜬다 — 지어내지 않는다는 뜻이다. [`custom-jca-missing-class`](custom-jca-missing-class.json)가 그 케이스다.

#### 어떤 provider를 넣을 수 있나

이름은 자유지만, **도구가 낼 수 있는 설정 조각의 모양은 하나**다 — `activate = 1` + `module = 경로`.
그 모양으로 충분한 provider는 그대로 되고, 다른 모양을 요구하는 것은 아직 못 낸다.

| 후보 | 되나 |
|---|---|
| [oqsprovider](https://github.com/open-quantum-safe/oqs-provider) | ✅ 현행 모양으로 충분 — 실물 확인됨(2026-08-06, OpenSSL 3.0.13) |
| [wolfProvider](https://github.com/wolfSSL/wolfProvider) | ◐ 충분해 보이나 실물 미확인 |
| OpenSSL 자체 `fips` 모듈 | ❌ 다른 모양 — `fipsinstall`이 만드는 `fipsmodule.cnf`를 끌어와야 한다 |
| [pkcs11-provider](https://github.com/openssl-projects/pkcs11-provider) | ❌ 다른 모양 — 드라이버 경로 등 키가 더 필요 |
| JCA 커스텀 | ✅ `providerClass`(FQCN)가 이미 일반 경로다 |

❌ 인 것을 받으려면 도구가 그 모양을 알아야 한다 — [검토 중인 설계](../../../docs/under-review.md)에서 다룬다.
모듈 파일 자체는 어느 경우든 사용자가 구해 [`files/`](../files/README.md)에 둔다.

**이름은 Ansible 변수명으로도 쓰인다.** `pqcota_module_src_<이름>`·`pqcota_module_sha256_<이름>`에서 `<이름>`은 **영숫자만 남기고 나머지를 `_`로** 바꾼 것이다(`acme-pqc` → `acme_pqc`) — Ansible 변수명에 하이픈을 쓸 수 없어서다. 하이픈을 그대로 준 변수는 **인식되지 않고 조용히 무시된다**(무결성 검사가 통째로 skip된다).

### `kind` — 무엇이 생성되는지를 가른다

| 값 | 생성물 |
|---|---|
| `REMEDIATION_KIND_CONFIG_ONLY` | config 조각만. provider를 놓지 않는다 |
| `REMEDIATION_KIND_PROVIDER_INJECT` | provider 모듈 배치 + sha256 게이트 + 그 모듈을 참조하는 config 조각 |
| `REMEDIATION_KIND_FORK_REPLACE`·`JDK_UPGRADE`·`APP_RECONFIG`·`REBUILD`·`DECOMMISSION` | **그 조치로는 아무것도 배치하지 않는다** — config로 할 수 없는 조치라, `# 조치 a1(…): config로 배포 불가 — 수동 단계` 주석만 남긴다. play의 공통 골격(디렉터리 생성)은 그대로 나온다 |

### `activation` — L3 훅

명령은 사용자가 쓰고, **어느 순서로 놓을지는 생성기가 정한다**(적용: `pre` → 배치 → `activate` → `restart`, 롤백: 정확한 역순).

```json
"activation": {
  "pre":        "systemctl stop payments.service",
  "activate":   "…앱이 새 provider를 보게 만드는 명령…",
  "deactivate": "…activate를 되돌리는 명령…",
  "restart":    "systemctl start payments.service"
}
```

**빈 훅은 지어내지 않는다.** 없으면 그 태스크를 만들지 않고 무엇이 일어나지 않는지 stderr로 고지한다 — `l3-hooks-missing`이 그 케이스다.
