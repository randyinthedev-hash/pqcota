[English](change-a-contract.md) · 한국어

# 계약 변경하기

pqcota의 protobuf 계약에 메시지, 필드, 열거형 값을 추가하거나 바꿔야 하는 엔지니어를 위한 문서입니다. 계약은 단계들이 합의하는 유일한 자리라서, 여기서 바꾸면 모든 단계에 영향이 갑니다. 이 문서는 작업 순서, 계약과 함께 움직여야 하는 것, 로컬 점검이 잡는 것과 잡지 못하는 것을 다룹니다. 무엇을 바꿀 수 있는지에 관한 규칙은 [호환성 정책](compatibility.ko.md)을, 계약 자체는 [계약 개요](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.md)(영문)와 [데이터 모델](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/data-model.md)(영문)을 읽으세요. 전체 구조는 [개발자 문서](developers.ko.md)에 있습니다.

## 시작하기 전에

**proto를 바꿔야 하는 일인가요?** 자산에 관한 도구별 세부 정보는 proto를 건드리지 않고 `pqcota:` 네임스페이스의 CycloneDX `properties`에 실어 보낼 수 있습니다. [키 규약](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.md#cyclonedx-properties-extension-key-convention)(영문)에 기존 키가 나열되어 있습니다. 새 키도 코어가 읽도록 만들어야 하지만, 생성 코드와 서명 범위는 바뀌지 않습니다.

**추가만 하는 변경인가요?** `v1` 안에서 계약 변경은 추가만 허용합니다. 필드와 열거형 값은 더할 수 있지만, 이미 공개된 것의 번호를 다시 매기거나, 타입을 바꾸거나, 지우거나, 이름을 바꾸지 마세요([호환성 정책, 계약](compatibility.ko.md#1-계약-더하기만-한다)). 추가만 하는 변경이 아니면 `v1`을 고치는 것이 아니라 새 `v2` 패키지입니다. 필드를 없애는 패키지(예를 들어 새 메이저 버전)에서는 그 번호를 `reserved`로 표시해 다시 쓰이지 않게 하세요. 새 열거형 값은 맨 끝에 붙입니다.

**서명에 닿는 변경인가요?** `CollectionResult`, `Envelope`, `MachineIdentity`, `Completeness`, `ObservedEdge`에 필드를 더하면 서명 범위를 넓혀야 하고, **범위를 넓히면 기존 서명이 모두 무효가 됩니다.** 서명 이전(migration)을 받아들인 릴리스에서만 허용되는 일입니다([호환성 정책, 서명](compatibility.ko.md#2-서명-범위를-바꾸면-과거가-무효가-된다)). 멈추고 그것부터 결정하세요.

## 계약 변경이 닿는 곳

| 위치 | 움직이는 것 | 빠뜨리면 알아채는 방법 |
|---|---|---|
| `pqcota-common` | `.proto` 파일, `gen/`에 커밋된 생성 코드, [데이터 모델](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/data-model.md)(영문, 손으로 쓴 문서), 서명되는 필드라면 `sign.Canonical` | 생성 코드와 proto가 어긋나면 CI가 실패합니다. 서명 범위는 테스트가 실패합니다. 데이터 모델은 아무것도 실패하지 않습니다 |
| `pqcota-inventory` | 그 필드를 읽는 정규화, 필드가 실질 내용이라면 `history`의 스냅샷 지문, 파생 규칙을 바꿨다면 `ruleset_version` | 지문은 **아무것도 실패하지 않으며, 그 필드의 변경이 이력에서 사라질 수 있습니다.** 아래를 보세요 |
| `pqcota-discovery` | 이제 그 필드를 내보내야 하는 수집기 | 필드가 빈 채로 남습니다 |
| `pqcota-provisioning` | `plan.proto`와 `rollback.proto`에 대한 생성기와 계획 점검. 계획도 서명됩니다. `pqcota-common`의 `sign.CanonicalPlan`입니다 | `pqcota-common`의 테스트가 실패합니다(아래 참조) |
| `pqcota` | 단계 간 테스트, 데모의 기대 출력, 문서 | 단계 간 점검이 실패하거나 데모 출력이 달라집니다 |

작업 공간 안의 소비자는 변경을 바로 봅니다. 각 모듈이 `replace` 지시문으로 `../pqcota-common`을 읽기 때문입니다. 작업 공간 밖의 소비자는 `pqcota-common`에 태그가 붙고 그들의 `require`가 그 태그를 가리킨 뒤에야 봅니다.

## 작업 순서

1. `pqcota-common/contracts/proto/`에서 **proto를 고칩니다.** `gen/`을 손으로 고치지 마세요.
2. **다시 생성하고** `gen/`을 proto와 같은 커밋에 넣습니다.

   ```bash
   cd pqcota-common
   make tools && make generate     # contracts/proto → gen/
   ```

   주석 하나만 바꿔도 `gen/`이 바뀌므로 둘을 함께 커밋하세요. `make tools`의 `PATH` 주의 사항은 [빌드 안내, 계약 변경](build.ko.md#계약-변경)을 보세요.
3. **린트하고 비교합니다.** `make lint`는 `buf lint`를 실행합니다. `make breaking`은 최신 릴리스 태그(현재 `v0.10.2`)와 비교합니다. 브랜치를 `main`과 비교하려면 `make breaking AGAINST=main`을 실행하세요. 둘 다 `buf`가 필요하며 따로 설치합니다.
4. 아래의 **파급 확인**을 합니다. 빌드는 통과하는데 동작이 깨질 수 있는 단계입니다. 테스트가 지키는 것은 서명 범위뿐이기 때문입니다.
5. 바꾼 메시지에 대해 **데이터 모델 문서를 고칩니다.**
6. 의존 방향 순서로 **소비자를 고칩니다.** 인벤토리, 그다음 관측과 전환물 생성, 마지막으로 통합 리포지터리입니다. 각 리포지터리는 `replace`를 통해 새 생성 코드를 직접 읽습니다.
7. `pqcota`에서 **점검을 실행합니다.** `make all`은 각 형제 리포지터리의 자체 점검을 돌린 다음 단계 간 점검을 돌립니다. [작업 확인](build.ko.md#작업-확인)을 보세요.
8. **릴리스합니다.** 지금은 릴리스를 조율해서 다섯 리포지터리에 같은 태그를 붙입니다. 계약 변경이라면 `pqcota-common`을 먼저 공개하고, 각 단계의 `require`를 그 태그로 올리고, 조합을 확인한 뒤 통합 리포지터리에 태그를 붙인다는 뜻입니다. v0.10.0과 v0.10.2를 이렇게 릴리스했고 [빌드 안내](build.ko.md#소스-받기)도 태그를 이렇게 설명합니다. 이것은 현재의 관행이지 강제되는 규칙이 아니며, 릴리스 명령은 여기서 되풀이하지 않습니다.

## 파급 확인

코드에서 계약으로부터 파생되는 것이 둘 있습니다. 첫째는 테스트가 지킵니다. **둘째는 아무것도 지키지 않으므로** 당신과 리뷰에 달려 있습니다.

**서명 범위(테스트가 지킴).** `sign.Canonical`은 서명되는 메시지의 모든 필드를 덮습니다. 서명 자체는 제외합니다. `TestCanonicalCoversAllFields`는 `CollectionResult`, `Envelope`, `MachineIdentity`, `Completeness`, `ObservedEdge`의 필드 수를 기대하는 수와 비교합니다. 수가 바뀌면 테스트가 실패하면서 할 일을 알려 줍니다. `Canonical`을 고치고, 기대하는 수를 갱신하고, `TestTamperBreaksVerification`에 케이스를 추가하라는 것입니다. **기대하는 수만 고쳐서 실패를 없애지 마세요.** 그렇게 하면 그 필드가 서명의 사각지대가 됩니다. 누군가 그 필드를 바꿔도 검증이 통과합니다. 계획에는 이 보호 장치의 사본이 따로 있습니다(`pqcota-common/pkg/kernel/sign/plan_test.go`). `TestCanonicalPlanCoversAllFields`가 `FinalizedPlan`, `RemediationAction`, `ActivationHooks`, `ActionEvidenceSource`, `SnapshotReference`, `SnapshotContentReference`의 수를 세고 `TestTamperBreaksApproval`을 가리킵니다. 어느 쪽 범위든 넓히면 이미 있는 서명이 무효가 됩니다.

**변경 감지(지키는 것 없음).** 인벤토리는 관측의 실질이 바뀔 때만 새 스냅샷을 남깁니다. 지금은 그 판단과 계획의 참조로 스냅샷을 찾는 일에 지문 `history.ContentHashV1`과 그 저장 열 `content_hash_v1`을 씁니다. 예전의 `ContentHash`도 여전히 계산해 저장하지만 상태가 새로운지 결정하는 것은 아닙니다. `Finding`, `ObservedEdge`, `Completeness`에 실질 내용 필드를 더했는데 그것이 지문에 들어 있지 않으면, 그 필드의 변경은 「변경 없음」으로 합쳐지고 **오류 없이 이력에서 사라집니다.** 두 지문 어느 쪽도 필드 수를 지켜보는 테스트는 없습니다.

**필드를 넣으려고 `ContentHashV1`을 고치지 마세요.** v1 형식(`pqcota-snapshot-content/v1`)은 고정되어 있습니다. 저장된 참조는 같은 규칙으로 다시 계산되고, `TestContentHashV1IsFrozen`이 정해진 입력에서 그 바이트를 고정해 둡니다. 고치면 기존 참조가 모두 찾을 수 없게 됩니다. 반영되어야 하는 필드에는 **새 지문 형식 버전**이 필요하고, 저장 열, 조회, 중복 제거 경로, `SnapshotContentReference`의 하위 소비자를 함께 설계해야 합니다. 한 줄 수정이 아니라 설계 작업이므로 시작하기 전에 제기하세요.

**파생 규칙을 바꾼 경우.** 증거 강도나 양자내성 등급처럼 파생되는 값의 계산 방식을 바꾸면 `ruleset_version`을 올립니다(인벤토리 정규화 코드의 상수 `normalize.RulesetVersion`입니다). 파생 값은 저장된 사실이 아니라 규칙의 함수입니다. 이전 값은 수집기의 원래 결과를 보관해 두었을 때만 다시 계산할 수 있습니다. 인벤토리 스냅샷에는 파생된 발견 항목, 연결 간선, 완전성, 규칙 버전이 들어 있고 원본 수집 결과는 **없으며**, pqcota에는 과거를 대신 다시 계산해 주는 명령이 없습니다. 원본을 보관하지 않았다면 이전 스냅샷에는 이전 규칙이 낸 값이 그대로 남고, 어느 규칙이었는지는 각 스냅샷의 규칙 버전이 알려 줍니다. 파생 값을 수집기의 `pqcota:` properties에 절대 넣지 않습니다.

**순서가 의미를 갖는 필드.** 순서가 의미를 갖는 필드는 정렬하거나 정규화하지 마세요. 등록 순서가 곧 우선순위인 `provider_set`이 그런 예입니다.

## 로컬 점검과 CI

같은 명령이 두 곳에 모두 있고, CI는 세 가지를 더합니다.

| | `pqcota-common`의 로컬(`make`) | CI |
|---|---|---|
| 다시 생성 | `make generate`가 `gen/`을 다시 씁니다 | 같습니다. **그다음 `git diff --exit-code`가 `gen/`이 바뀌었으면 빌드를 실패시키므로**, 다시 생성한 코드 없이 proto만 바꾼 변경은 잡힙니다 |
| 린트, 비교, 서식, vet, 빌드, 테스트 | 예 | 예 |
| `buf` 버전 | 설치한 것 | 워크플로에 고정되어 있습니다. 로컬 `buf`가 다르면 생성 결과가 달라져 거짓 어긋남이 생길 수 있습니다 |
| `make breaking`용 릴리스 태그 | 가져온 것 | 전체 클론이라 최신 태그가 항상 있습니다 |
| 나머지 네 리포지터리 | `pqcota`의 `make all`이 그쪽의 자체 점검을 실행합니다 | `cross-stage` 작업이 당신의 커밋과 나머지 네 리포지터리의 `main`으로 통합 리포지터리의 점검을 실행합니다 |

그래서 로컬 `make`가 깨끗해도 어긋남 점검을 통과한다는 증거는 아닙니다. CI는 커밋의 깨끗한 체크아웃에서 다시 생성한 다음 작업 트리에 다른 것이 있으면 실패시킵니다. 로컬에서 `make generate && git diff --exit-code -- gen/`를 실행하면 다시 생성했을 때 생성 코드가 바뀌는지 알 수 있습니다. 같은 질문이지만 CI와 똑같은 복사본은 아닙니다. 커밋하지 않은 proto와 문서 수정이 트리에 있고 `git diff`는 추적되는 파일만 보기 때문입니다. 믿을 만한 재현은 변경을 커밋한 다음 깨끗한 트리에서 같은 두 명령을 실행하는 것입니다. 로컬 점검과 CI 점검 전체의 설명은 [작업 확인](build.ko.md#작업-확인)을 보세요.

## 잘못되기 쉬운 것

- **`gen/`을 손으로 고침.** proto를 바꾸고 다시 생성하세요.
- **proto만 바꿈.** CI의 어긋남 점검이 실패합니다. `gen/`을 함께 커밋하세요.
- **서명되는 필드를 더하고 `Canonical`은 넓히지 않음.** 개수 테스트가 실패합니다. 수만 고쳐서 해결하지 마세요.
- **지문에 없는 내용 필드를 더함.** 아무것도 실패하지 않습니다. 그 필드의 변경이 이력에서 사라집니다. 고정된 `ContentHashV1`을 고쳐서 「해결」하지 마세요.
- **파생 값을 `properties`에 넣음.** 증거 강도와 등급은 코어의 일입니다.
- **필드 번호를 재사용하거나 의미를 바꿈.** 호환이 깨지는 변경이므로 `v2`가 필요합니다.
- **`go_package`를 바꿈.** 와이어 형식은 그대로여도 `buf breaking`이 보고합니다. 리포지터리 분리 때 한 번 있었고 평소에 하는 변경이 아닙니다.
- **데이터 모델 문서를 잊음.** 손으로 쓰는 문서이고, proto와 맞는지 점검하는 것은 없습니다.
