[English](environment-variables.md) · 한국어

# 환경변수 참조

pqcota 프로그램이 읽는 모든 환경변수를 한곳에 모았습니다. 이름, 허용하는 값, 기본값은 코드에서 가져왔습니다. 각 명령의 사용법은 해당 단계의 명령 참조에 있으며, 이 문서는 그것을 반복하지 않고 링크합니다. 문서 전체의 개요는 [개발자 문서](developers.ko.md)를 보세요.

- **프로그램이 시작될 때 읽습니다.** 변수를 바꿔도 이미 실행 중인 프로그램에는 영향이 없습니다.
- **스토어를 공유하는 명령끼리는 데이터베이스와 조직을 같게 두세요.** `pqcota-ingest`, `pqcota-cbom-ingest`, `pqcota-inventory`, `pqcota-prune`, `pqcota-provision`, `pqcota-records`, `pqcota-hosts`, `pqcota-profile`이 `PQCOTA_ORG`를 읽으므로 모두 같은 방식으로 설정하세요. `pqcota-hosts`와 `pqcota-profile`은 데이터베이스를 `PQCOTA_DSN`이 아니라 `--dsn`에서 받으므로, 플래그가 같은 데이터베이스를 가리키게 직접 맞춰야 합니다.
- **이전 릴리스는 두 명령에서 조직을 무시했습니다.** v0.10.0 이하에서 `pqcota-hosts`와 `pqcota-profile`은 `PQCOTA_ORG`에 무엇을 적었든 엔드포인트와 프로필을 기본 조직에 썼고, 조직을 지정해도 `PQCOTA_REQUIRE_ORG=1`이면 실패했습니다. 그렇게 쓴 행은 기본 조직에 그대로 남습니다. 원래 어느 조직의 것이었는지 pqcota가 알 수 없어서 옮기지 않습니다. 이 행들을 조직 아래에 두고 싶다면 직접 판단해 `PQCOTA_ORG`를 설정하고 명령을 다시 실행하세요. 그러면 새 행이 그 조직 아래에 쓰이고 옛 행은 그 자리에 남습니다.
- **개인 키는 비밀입니다.** `PQCOTA_SIGN_KEY`와 `PQCOTA_APPROVAL_KEY`는 개인 키를 담습니다. 이 변수를 설정한 환경을 그에 맞게 다루세요. 공개 키(`PQCOTA_VERIFY_KEY`, `PQCOTA_APPROVAL_KEYS`)는 비밀이 아닙니다.

## 스토어와 조직

| 변수 | 읽는 프로그램 | 값 | 설정하지 않으면 |
|---|---|---|---|
| `PQCOTA_DSN` | `pqcota-ingest`, `pqcota-cbom-ingest`(선택); `pqcota-inventory`, `pqcota-prune`, `pqcota-records`(필수) | URL(`postgres://user:password@host:5432/db`) 또는 `key=value` 쌍으로 쓴 Postgres 연결 문자열. 관측 명령 참조(`pqcota-hosts`)의 [형식](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)을 보세요 | 적재 명령 둘은 메모리 스토어를 쓰며, 요약을 출력하고 프로세스가 끝나면 아무것도 남기지 않습니다. 나머지 셋은 오류(상태 2)로 종료합니다. 그 보기에는 다른 프로세스가 써 둔 스토어가 필요하기 때문입니다 |
| `PQCOTA_ORG` | 위에 나열한 여덟 명령 | 조직 이름: 소문자, 숫자, 하이픈으로 이루어진 2자 이상 64자 이하의 문자열이며 하이픈으로 시작하지 않습니다. 맞지 않는 값은 항상 오류이며 기본값으로 대체되지 않습니다 | 레코드가 조직 `default`에 속합니다 |
| `PQCOTA_REQUIRE_ORG` | 스토어를 여는 모든 명령 | `1` | `1`이면 조직 없이 스토어를 여는 것도, `default`를 명시하는 것도(조직 없이 쓴 레코드를 가리키는 이름입니다) 오류이므로 실수로 `default`에 쓰이는 일이 없습니다. 다른 값이거나 설정하지 않으면 둘 다 허용합니다 |
| `PQCOTA_AUTO_DDL` | 인벤토리 **이력** 스토어에만 | `0` | 값이 정확히 `0`이 아니면 그 스토어를 열 때 이력 스키마를 만듭니다. `0`이면 만들지 않으며 스키마가 없을 때 열기가 실패합니다. **다른 테이블에는 적용되지 않습니다.** 기계 메타데이터 스토어(`pqcota-hosts`, `pqcota-profile`)와 전환물 생성의 기록 스토어(`pqcota-provision`)는 이 변수와 관계없이 자기 테이블을 만듭니다 |

이 변수들이 있는 이유: 여러 조직이 데이터베이스 하나를 공유할 때 잘못된 조직 아래에 쓰인 레코드는 나중에 가려낼 수 없고, 잘못된 연결 문자열을 향한 자동 `CREATE TABLE`은 빈 테이블을 만들어 거기에 아무 표시 없이 씁니다. `PQCOTA_REQUIRE_ORG=1`은 적용되는 자리에서 앞의 문제를 오류로 바꾸고, `PQCOTA_AUTO_DDL=0`은 이력 스키마에 한해 뒤의 문제를 오류로 바꿉니다. 자세한 내용은 [인벤토리 명령 참조](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.md)(영문)에 있습니다.

`pqcota-hosts`, `pqcota-profile`, `pqcota-provision`은 데이터베이스를 `PQCOTA_DSN`이 아니라 `--dsn` 플래그에서 받습니다.

## 관측 서명과 검증

| 변수 | 읽는 프로그램 | 값 | 설정하지 않으면 |
|---|---|---|---|
| `PQCOTA_SIGN_KEY` | `pqcota-nodescan`, `pqcota-netcap`, `pqcota-jvmscan`, `pqcota-cngscan`, `pqcota-declare`, `pqcota-declare-attribution` | `pqcota-keygen`이 만든 base64 ed25519 **개인** 키 | 결과에 서명하지 않습니다. 키를 설정했는데 서명할 수 없으면(base64가 아니거나 공개 키를 잘못 넣은 경우) 서명 없는 결과를 내지 않고 오류로 멈춥니다 |
| `PQCOTA_VERIFY_KEY` | `pqcota-ingest` | 쉼표로 구분한 `<collector-id>:<base64 공개 키>` 항목(**결합 형식**), 또는 쉼표로 구분한 base64 공개 키(**기존 형식**). 아래를 보세요 | 서명을 검증하지 않으며, 적재 요약이 그 결과를 *unchecked*로 셉니다 |
| `PQCOTA_REQUIRE_SIGNATURE` | `pqcota-ingest` | `1` | `1`이면 검증할 키가 없을 때 적재를 시작하지 않습니다. 다른 값이거나 설정하지 않으면 검증 없이 진행합니다. 결합 형식에서는 이 값과 상관없이 서명이 언제나 필수입니다 |

키 쌍은 [`pqcota-keygen`](https://github.com/randyinthedev-hash/pqcota-common/blob/main/cmd/README.md#pqcota-keygen)(영문)이 만듭니다. 계획 승인도 같은 종류의 키를 쓰기 때문에 이 프로그램은 `pqcota-common`에 있습니다. `pqcota-keygen -collector <id>`는 검증 줄을 결합 형식으로 출력합니다.

### 키를 수집기 ID에 묶기

결과마다 그것을 낸 수집기의 ID가 적혀 있습니다(`openssl-collector`, `network-collector`, `jvm-collector`, `cng-collector`, `declaration-importer`, `attribution-importer`). 그 키가 이 수집기의 것이어야 하는지는 `PQCOTA_VERIFY_KEY`의 형식이 정합니다.

```
PQCOTA_VERIFY_KEY="openssl-collector:<키 A>,network-collector:<키 B>,jvm-collector:<키 C>"
```

- **결합 형식**(모든 항목이 `<collector-id>:<키>`): 결과는 자기가 적은 수집기 ID에 등록된 키로만 검증합니다. 키가 등록되지 않은 ID를 적은 결과, 다른 수집기의 키로 서명한 결과, 서명이 없는 결과는 모두 저장하기 전에 거부하고, 사유와 함께 거부 기록을 남깁니다. 사람이 만든 선언(`pqcota-declare`, `pqcota-declare-attribution`)도 같은 검증을 거칩니다. 검증에 실패해도 `pqcota-ingest`는 다른 키로 다시 시도하지 않습니다.
- **기존 형식**(콜론 없이 키만 나열): 등록된 키 가운데 아무것으로나 맞으면 어느 수집기 ID든 통과합니다. v0.15.0 이전과 같습니다. 앱 선언은 서명을 확인하지 않고 받습니다. `pqcota-ingest`가 경고를 출력합니다.
- **두 형식을 섞으면 오류입니다.** ID나 키가 비어 있거나, 키가 32바이트 ed25519 공개 키가 아니거나, 한 ID에 같은 키를 두 번 적어도 오류입니다. `pqcota-ingest`는 결과를 하나도 읽기 전에 상태 2로 멈춥니다. 메시지는 몇 번째 항목이 틀렸는지 알려 주고 키 값은 출력하지 않습니다.
- 이 판의 도구가 쓰지 않는 ID는 오타일 수 있으므로 경고만 합니다.

**한 ID에 키 두 개는 키 교체입니다.** 같은 ID에 옛 키와 새 키를 함께 적으면 어느 쪽으로 서명한 결과도 통과합니다. 그 ID는 여전히 다른 ID와 구별됩니다.

**한 키를 여러 ID에 적으면 그 ID들은 서로 구별되지 않습니다.** 그 키를 가진 수집기는 키가 적힌 어느 ID로든 서명할 수 있습니다. 노드 하나가 수집기 셋을 같은 키로 돌리고 그 키를 세 ID에 모두 적으면, 그중 하나가 다른 수집기라고 주장한 결과도 통과합니다. 그 키를 적지 않은 ID만 보호됩니다. 수집기마다 신원을 따로 확인하려면 ID마다 다른 키를 쓰세요.

결과가 적은 노드는 아직 키에 묶이지 않습니다. 등록된 키로 서명한 결과는 어느 노드를 적었든 이 검증을 통과합니다. 등재 목록과 머신 지문 검사는 따로 있습니다.

`pqcota-cbom-ingest`는 수집기 결과가 아니라 여러분의 CI가 낸 CycloneDX를 받고, 서명을 확인하지 않습니다. 이 설정은 그 명령에 적용되지 않습니다.

### 기존 배포를 결합 형식으로 옮기기

아래 순서대로 합니다. `pqcota-ingest`를 먼저 결합 형식으로 바꾸면, 아직 서명하지 않거나 그 ID에 등록하지 않은 키로 서명하는 수집기가 모두 끊깁니다.

1. **중앙 도구를 올립니다**(v0.15.0 이상). `PQCOTA_VERIFY_KEY`는 그대로 둡니다. 경고와 함께 지금처럼 동작합니다.
2. **노드의 수집기를 올립니다.** v0.15.0부터는 `PQCOTA_SIGN_KEY`를 설정하면 수집기 넷과 선언 도구 둘이 모두 서명합니다. 기존 키 목록으로도 검증됩니다.
3. **어느 키가 무엇에 서명했는지 확인합니다.** 지금 설정 그대로 `pqcota-ingest -verify-keys <결과 폴더>`를 실행합니다. 아무것도 저장하지 않습니다. 결과마다 수집기 ID, 서명 여부, 맞은 키의 지문, 판정을 출력합니다. 이것을 **어느 노드·수집기에 어느 키를 배포했는지 적은 기록**과 대조합니다. 지문은 어느 키가 서명했는지만 알려 줍니다. 그 키를 정말 그 수집기 ID에 쓰려고 했는지는 배포 기록이 말해야 합니다. 둘이 다르면 다음 단계로 가기 전에 원인을 찾습니다.
4. **결합 형식으로 적습니다.** 어느 ID에 어느 키를 적을지는 3단계 표가 아니라 **배포 기록으로** 정합니다. 표에 나오지 않았다는 이유만으로 키를 빼지 마세요. 점검하는 동안 실행되지 않은 수집기가 끊깁니다. 배포 기록상 그 키를 가진 수집기가 하나도 없을 때 키를 뺍니다.
5. **확인합니다.** 새 값으로 `-verify-keys`를 다시 실행해 거부가 없는지 봅니다. 다음 실제 적재의 요약에 `signature check: bound to collector IDs (N IDs, M keys)`와 `signature-rejected 0`이 나옵니다. 배포 기록의 모든 수집기가 한 번 이상 적재되면 전환이 끝납니다.

되돌리려면 기존 키 목록을 다시 넣습니다.

나중에 키를 바꿀 때는, 같은 ID에 새 키를 더하고, 그 수집기의 `PQCOTA_SIGN_KEY`를 바꾸고 배포 기록을 고친 다음, 배포 기록상 옛 키를 가진 수집기가 남지 않았을 때 옛 키를 뺍니다.

`-verify-keys`는 이 점검에만 씁니다. 적재를 대신하지 않고, 등재 목록도 적용하지 않습니다.

## 계획 승인

| 변수 | 읽는 프로그램 | 값 | 설정하지 않으면 |
|---|---|---|---|
| `PQCOTA_APPROVAL_KEY` | `pqcota-approve` | base64 ed25519 **개인** 키 | 명령이 오류(상태 2)로 종료합니다. 승인에 쓸 키가 없기 때문입니다 |
| `PQCOTA_APPROVAL_KEYS` | `pqcota-provision` | 쉼표로 구분한 `<approver>=<base64 public key>` 쌍. 그 형식이 아닌 쌍은 오류입니다 | 기본적으로 생성기가 실행을 거부합니다. 확인할 수 없는 승인은 누가 책임을 졌는지 아무것도 보여 주지 않기 때문입니다. `--allow-unverified-approvals` 플래그를 주면 경고를 출력하고 확인 없이 계속합니다 |

`PQCOTA_REQUIRE_APPROVAL=1`은 코드가 더 이상 읽지 않는 예전 이름입니다. 확인할 수 없는 승인을 거부하는 것이 이제 기본이므로 설정해도 효과가 없습니다.

승인이 동작하는 방식과 종료 코드의 의미는 [전환물 생성 명령 참조](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/cmd/README.md)(영문)와 [보고 안내](reporting-guide.ko.md#5-승인)에 있습니다.

## 관측 프로그램

| 변수 | 읽는 프로그램 | 값 | 기본값과 참고 |
|---|---|---|---|
| `NETCAP_IFACE` | `pqcota-netcap` | 네트워크 인터페이스 이름 | `eth0`. 명령줄의 인터페이스 인자가 우선합니다 |
| `NETCAP_WINDOW_SEC` | `pqcota-netcap` | 0보다 큰 정수 초로 쓴 관측 창 | `8`. 명령줄의 창 인자가 우선합니다. 정수가 아닌 값은 아무 메시지 없이 무시되고 기본값을 씁니다. 0이나 음수는 받아들여지지만 그러면 캡처가 3초 동안 실행되고, 시작 메시지에는 준 값이 출력됩니다 |
| `PQCOTA_JVM_AGENT` | `pqcota-jvmscan` | `collector.jar` 경로 | 설정하지 않으면 attach하지 않습니다. 설정했고 JVM이 하나 이상 발견되면 각각에 attach합니다. 설정하지 않았을 때는 아래 참고를 보세요 |
| `JAVA_BIN` | `pqcota-jvmscan` | Java 런처 경로 | 탐침 경로에서만 씁니다. 설정하지 않으면 발견된 JVM의 런처를, 없으면 `PATH`의 `java`를 씁니다 |
| `JVMSCAN_CP` | `pqcota-jvmscan` | 클래스 경로 | 탐침 경로의 Java 런처에 `--class-path`로 전달합니다. 설정하지 않으면 없음 |
| `PQCOTA_CLOUD_INSTANCE_ID` | `pqcota-nodescan`, `pqcota-cngscan`(머신 식별자 지문을 통해) | 운영자가 제공하는 클라우드 인스턴스 식별자 | 설정하지 않으면 클라우드 인스턴스 식별자가 지문에 들어가지 않습니다. 앞뒤 공백은 잘라 냅니다. 설정하면 노드가 스스로 정하는 식별자를 도출할 때 가장 먼저 쓰는 입력이며, 기계 id, 하드웨어 UUID, 호스트 이름보다 앞섭니다 |

**탐침 경로.** `PQCOTA_JVM_AGENT`도 `--pid`도 없으면 `pqcota-jvmscan`은 기본 provider 체인을 읽으려고 자체 Java 런처를 시작하고, JVM이 실행 중이어도 결과를 관측 범위가 제한된 대체 결과(degraded)로 표시합니다. `--pid`만 있고 에이전트가 없으면 그 JVM의 정적 설정을 읽습니다. 권한과 다른 수집기에 대해서는 [자주 묻는 질문](faq.ko.md#실행-중인-시스템에-미치는-영향)과 [관측 명령 참조](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)를 보세요.

## 테스트와 데모

운영용이 아닙니다.

| 변수 | 위치 | 의미 |
|---|---|---|
| `PQCOTA_TEST_DSN` | Postgres 테스트 케이스 | 버려도 되는 데이터베이스의 연결 문자열. 설정하지 않으면 그 케이스는 건너뜁니다. [작업 확인](build.ko.md#작업-확인)를 보세요 |
| `DEMO_REAL_PROVIDER` | `demo/scripts/demo.sh` | `1`이면 실제 양자내성 provider를 빌드하고 적용하는 선택 단계를 켭니다. [데모](../demo/README.ko.md)를 보세요 |
| `PQCOTA_PROVIDERS`, `PQC_SERVER` | 데모의 워크로드 컨테이너 | 데모의 토폴로지 생성기가 컨테이너마다 provider를 주려고 설정합니다. 직접 설정하지 마세요 |

## 환경변수가 아닌 것

비슷해 보이지만 환경에서 읽지 않는 것들입니다.

- **명령줄 플래그**: `--dsn`, `--level`, `--allow-unverified-approvals`, `--allow-incomplete`, `--strict`, `--pid` 등. 명령 참조에 있습니다.
- 생성된 플레이북의 **Ansible 변수**: `pqcota_module_sha256`, `pqcota_module_sha256_<provider>` 등. 플레이북을 실행할 때 전달합니다. [전환물 생성 명령 참조](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/cmd/README.md)(영문)를 보세요.

## 이 문서를 정확하게 유지하기

기준은 코드이지만 검색 한 번으로는 완전한 목록이 나오지 않습니다. 변수를 추가하거나 바꿀 때는 변수가 나올 수 있는 모든 자리를 확인하세요. 다섯 리포지터리의 테스트가 아닌 Go 코드(`os.Getenv`로 읽는 이름이나 `org.Env` 같은 상수에 담긴 이름), 데모 셸 스크립트, 테스트 코드, 그리고 주석입니다. 주석에는 `PQCOTA_REQUIRE_APPROVAL`처럼 어느 코드도 읽지 않는 이름이 남아 있을 수 있습니다. 그런 다음 호출 경로를 따라가 어느 명령이 그 변수에 닿는지 확인하세요. "스토어와 조직" 아래의 예외가 그런 경우입니다. 이 문서는 같은 변경에서 함께 고치세요.
