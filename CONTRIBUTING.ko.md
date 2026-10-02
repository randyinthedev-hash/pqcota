[English](CONTRIBUTING.md) · 한국어

# 기여 안내 (CONTRIBUTING)


> **깨지 않는 것**: 계약, 서명, Go API, 데이터베이스 스키마, 혼합 버전을 서로 다른 다섯 가지로
> 풀어 적은 [호환성 정책](docs/compatibility.ko.md)입니다. 이 중 무엇이든 바꾸기 전에 읽으세요.

pqcota를 **포크하고 확장하고 기여**하려는 개발자를 위한 문서입니다. 플랫폼을 *써 보기만* 하려는 사용자는 루트 [README](README.ko.md)와 [demo/](demo/README.ko.md)를 보세요.

## 사전 준비

**Go 1.26.4 이상**(`go.mod`의 `go` 지시어보다 낮은 버전에서는 툴체인이 빌드를 거부합니다)과
**buf**, `protoc-gen-go`, `protoc-gen-go-grpc`가 필요합니다. JVM 수집기를 건드린다면 **JDK 11 이상**도 필요합니다(선택 사항입니다. 없으면 사이드카 빌드만 건너뜁니다).
리포지터리가 빌드되면 [예제](examples/README.ko.md)를 바로 실행할 수 있습니다(JVM 예제와 OpenSSL 통합 예제만
**Docker**도 필요합니다).

pqcota는 **리포지터리 다섯 개**입니다(아래 [리포지터리](#리포지터리) 참고). 나란히 클론하세요. `go.mod`가 `replace` 지시어로 네 모듈을 `../`에서 읽고(`require` 줄은 릴리스 태그를 가리킵니다), 이 리포지터리의 게이트는 다섯 개를 함께 잽니다. 이 문서는 **리포지터리에 기여하는 방법**을 다룹니다. 쓰기만 한다면 빌드와 실행은 [빌드 안내](docs/build.ko.md)에 있습니다.

### 어느 OS에서 빌드할 수 있나

| OS | `go build` · `go test` | `make`(게이트) | 노드 바이너리 |
|---|---|---|---|
| **Linux** | ✅ | ✅ | ✅ 직접 빌드 |
| **macOS**(amd64·arm64) | ✅ | ✅ | ✅ 크로스 컴파일 |
| **Windows**(amd64·arm64) | ✅ | POSIX 셸이 필요하므로 **WSL** | ✅ 크로스 컴파일 |

Linux 전용 코드(`/proc`, AF_PACKET, attach)는 `//go:build linux` 뒤에 있고, 다른 플랫폼에는 실행을 거부하는 스텁이 있습니다. 그래서 macOS와 Windows에서는 그 코드가 컴파일에서 빠지므로, 그 코드를 깨뜨려도 호스트 빌드는 통과합니다. `make build`가 **linux/amd64와 windows/amd64**도 크로스 컴파일하는 이유입니다. CNG 수집기를 Windows에서 빌드하고 검증하므로 Windows도 포함합니다.

## 개발 순환

빌드 절차는 [빌드 안내](docs/build.ko.md#빌드)와 같습니다. 기여할 때 추가로 쓰는 것은 게이트와 테스트입니다.

```bash
make            # in any repo: that repo's own checks. In pqcota (this repo): every sibling's `make`, then the gates across all five
go test ./...   # unit
```

`make build`는 **산출물을 남기지 않습니다.** 호스트, linux/amd64, windows/amd64 크로스 빌드가 컴파일되는지만 확인합니다(그래서 Linux 전용 파일도 포함됩니다). 쓸 바이너리는 README처럼 `-o`로 빌드하세요. `make build-jar`는 JDK가 없으면 경고하고 건너뛰므로, Go만 건드리는 기여자는 JDK 없이 `make`를 실행할 수 있습니다. 테스트는 실제 JVM 없이 실행됩니다.

계약을 바꿨다면 `pqcota-common`에서 작업합니다. `make generate`와 `make lint`(buf lint)를 실행하고 하위 호환성을 확인하세요.

```bash
make breaking                  # against the last release tag — does it break a contract already shipped (what CI runs)
make breaking AGAINST=main     # compare the branch you are working on against main
```

기준선은 가장 최근 릴리스 태그입니다(`pqcota-common`에서 현재 `v0.10.2`). 릴리스 태그가 없으면 기준선도 없으므로, 점검은 건너뛰고 그 사실을 로그에 남깁니다.

[계약 변경의 파급 점검 목록](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.ko.md)(서명 범위, 변경 감지)도 읽으세요. 변경 감지 함수 `history.ContentHash`는 `pqcota-inventory`에 있으므로, 내용 필드를 추가하는 계약 변경은 두 리포지터리를 바꾸는 변경입니다.

## 리포지터리

| 리포지터리 | 내용 |
|---|---|
| [`pqcota-common`](https://github.com/randyinthedev-hash/pqcota-common) | 계약의 단일 원본(protobuf. 네임스페이스가 곧 단계입니다: `pqcota.{common,discovery,inventory,provisioning}.v1`), `gen/`에 **커밋된** 생성 Go 코드, `pkg/kernel`(레지스트리·상태·범위·머신 식별자·서명·완전성)과 `pkg/org`의 공유 로직, `cmd/pqcota-keygen`(수집기 서명과 계획 승인 모두에 쓰입니다) |
| [`pqcota-inventory`](https://github.com/randyinthedev-hash/pqcota-inventory) | 인벤토리 단계: `pkg/inventory`(이력·정규화·적재·결과 읽기·선언)와 `cmd/`의 명령 |
| [`pqcota-discovery`](https://github.com/randyinthedev-hash/pqcota-discovery) | 관측 단계: `collectors/`(참조 수집기), `cmd/`, `ansible/`의 참조용 Ansible 플레이북, `pkg/discovery/procs` |
| [`pqcota-provisioning`](https://github.com/randyinthedev-hash/pqcota-provisioning) | 전환물 생성 단계: `pkg/provisioning`과 `cmd/`의 명령 |
| `pqcota`(이 리포지터리) | `demo/`(Docker 종단 간 데모), `tools/`(다섯 리포지터리를 모두 재는 게이트), `test/crossstage/`(단계에 걸친 테스트), 릴리스 워크플로. 단계별로 실행할 수 있는 예제는 단계 리포지터리(각각의 `examples/`)에 있고, 여기의 [examples/README.ko.md](examples/README.ko.md)는 안내판입니다 |

**의존 방향.** `pqcota-common`은 다른 모듈을 import하지 않습니다. `pqcota-inventory`는 common만 import합니다. `pqcota-discovery`와 `pqcota-provisioning`은 common과 inventory를 import하며 서로는 import하지 않습니다. discovery 안에서 수집기는 common과 `pkg/discovery/procs`만 import합니다. 수집기는 관측 대상 노드에 올라가는 바이너리에 들어가기 때문입니다. `make check-deps`가 이 규칙을 강제합니다(규칙은 `tools/checkdeps/rules.tsv`에 있습니다).

명령을 **실제로 실행하려면 예제를 쓰세요**([안내판](examples/README.ko.md). 단계 리포지터리마다 `run.sh`가 있는 `examples/`가 있습니다). 각 명령이 무엇인지는 각 `<stage>/cmd/README`를 보세요([관측](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.ko.md)·[인벤토리](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/cmd/README.ko.md)·[전환물 생성](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/cmd/README.ko.md)).

## 계약 우선

- 타입이나 enum을 바꾸려면 **`pqcota-common`의 `contracts/proto/**/*.proto`를 고치고 `make generate`를 실행합니다.** `gen/`은 직접 건드리지 마세요.
- `evidence_strength`·`pqc_readiness` 같은 도출 값은 **수집기가 아니라 코어가 채웁니다**(결과를 다시 계산할 수 있도록 규칙을 한곳에 둡니다). 자세한 내용은 [contracts/README](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.ko.md)에 있습니다.
- 통제 어휘의 `*_UNSPECIFIED = 0`은 "알 수 없음"을 뜻합니다. 비워 두거나 빠뜨리지 마세요.

## 수집기 확장: 이음매는 계약

참조 수집기(openssl·jvm·network)는 관측 방법의 예시 세 가지일 뿐입니다. **관측할 것이 더 있으면 수집기를 추가하세요.** 코어는 건드리지 않습니다. 이음매는 `CollectionResult` 계약 하나입니다(표준 CycloneDX와 `pqcota:` 속성).

- 수집기의 일은 **관측해서 `CollectionResult`를 내는 것**으로 끝납니다. `evidence_strength`·`pqc_readiness` 같은 도출 값은 채우지 **않습니다**. 코어가 계약 입력에서 도출합니다(결과를 다시 계산할 수 있도록 규칙을 한곳에 둡니다).
- 계약만 맞추면 언어는 자유입니다(참조 수집기부터 Go와 Java를 섞어 씁니다). 도구 고유의 보강 정보는 표준 `properties` 확장 키에 싣습니다([contracts/README](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.ko.md)).
- 참조 수집기마다 설계 목표, 경계, 정직성 규칙이 [`collectors/<name>`](https://github.com/randyinthedev-hash/pqcota-discovery/tree/main/collectors) 아래 코드 주석에 적혀 있습니다. 새 수집기도 같은 틀을 따릅니다(관측만 한다 · 보지 못한 것은 갭 · 추측하지 않는다).

> **전환물 생성기는 아직 이런 플러그인 이음매가 아닙니다.** 계획(`plan.proto`)은 공개 계약이지만 생성기 자체는 내부 로직입니다. 혼동을 피하려고 확장 지점으로는 수집기 쪽만 소개합니다.

## 새 암호 런타임으로 확장하기

플랫폼의 대상은 라이브러리 하나가 아니라 **암호 provider가 있는 런타임**이며, 어느 런타임이든 과정은 같습니다. 런타임마다 달라지는 것은 네 가지입니다. 관측이 수집하는 방법, 버전과 provider 축의 스키마, 조치 분기, 전환물 생성의 기반입니다. 모든 발견 항목과 자산은 `crypto_runtime`을 1급 필드로 갖고, 이 필드가 각 단계의 분기를 고릅니다.

새 런타임은 한꺼번에가 아니라 **단계적으로 도입**합니다. 계약 어휘가 먼저고, 그다음이 관측, 마지막이 조치입니다. 코드를 쓰기 전에 네 가지를 모두 설명하는 이슈를 여세요([이슈와 제안](#이슈와-제안) 참고).

## 코딩 지침

이 리포지터리들은 **정직성과 결정성을 코드 자체로 강제**합니다. 아래는 일반적인 Go 스타일이 아니라 **여기서 특별히 지키는 것**만 적은 관례입니다.

**서식과 점검.** `gofmt`(`go fmt ./...`)로 서식을 맞춥니다. `make`(전체)는 모든 게이트를 실행하므로 PR 전에 모두 통과해야 합니다(건드린 리포지터리마다 실행하고, 리포지터리에 걸친 게이트는 `pqcota`에서 실행합니다). 게이트와 각 게이트가 막는 것은 `Makefile`과 `.github/workflows/ci.yml`에 나열되어 있습니다. 표준 Go 관용구를 따르되, 도메인 용어에는 계약의 어휘를 씁니다(`finding` · `app_key` · `crypto_runtime`).

**주석은 "왜"를 설명합니다.** 코드가 *무엇을* 하는지는 코드에 드러나고, 주석은 *왜 이렇게 했는지*와 버린 대안이 왜 틀렸는지를 적습니다. 여기 주석이 긴 까닭입니다. 예: `// exclusion is not "absence" — silently dropping a policy-excluded asset makes the inventory lie`. 기존 주석은 한국어로 쓰여 있고, 이 리포지터리들에 포함되지 않은 프로세스 규정의 절 번호(`§`)를 인용합니다. 그 번호는 뜻을 알 수 없는 이름표로 취급하세요.

**정직성을 코드로 강제합니다.** 문서에서만이 아니라 실행 중에도 지켜져야 합니다.
- **알 수 없음은 1급 값입니다.** 판정할 수 없는 값은 빈칸이 아니라 `*_UNSPECIFIED` 또는 명시적인 "알 수 없음"입니다. 통제 어휘 enum의 `0`은 항상 알 수 없음입니다.
- **갭은 없음이 아닙니다.** 보지 못한 것과 정책이 제외한 것을 아무 표시 없이 버리지 않습니다. **세고, 돌려주고, 보고합니다**(제외 개수, 완전성 지도, `-diff`의 역순 경고 등).
- **추측도 판단도 하지 않습니다.** 관측하지 않은 것을 지어내지 않습니다. diff가 "변경 없음"이면 그것이 답입니다.

**도출 값은 원본에서 다시 계산할 수 있어야 합니다.** `evidence_strength` 같은 도출은 수집기가 아니라 코어가 원본(`detection_method`)에서 만들며, 규칙은 재현되도록 한곳(`pkg/inventory/normalize`)에 있습니다. **서명과 정규화 경로에는 벽시계도 난수도 쓰지 않습니다**(같은 입력이면 같은 바이트). 내용 지문은 변동 필드(관측 횟수, `last_seen`)를 제외합니다.

**로직은 순수하고 테스트하기 쉽게 둡니다.** 파싱과 판단 로직을 I/O에서 분리해 실물(프로세스, DB, 네트워크) 없이 단위 테스트할 수 있게 합니다. 예를 들어 `ParseProcMaps(reader)`는 `/proc` 없이 실행됩니다. **테스트는 동작뿐 아니라 "이 불변식이 왜 성립하는가"도 고정합니다**(회귀 테스트는 버그의 본질을 주석으로 적습니다).

**외부 도구에 의존하지 않습니다.** `ldd`·`lsof`·`ss`·`readelf`를 셸로 호출하지 말고 `/proc`과 ELF를 Go에서 직접 파싱합니다(이미지와 풋프린트를 최소로 유지하기 위해서입니다). 릴리스 바이너리는 `CGO_ENABLED=0` 정적 빌드입니다. OS 기본 기능을 건드리는 코드에는 `//go:build linux`를 붙이고, 순수 헬퍼는 OS와 무관하게 분리합니다.

**계약을 바꾸면 계약에 얹힌 것도 바꿉니다.** 수집기가 주장하는 모든 필드는 `sign.Canonical`이 덮어야 합니다(서명 사각지대 없음). oneof 갈래는 **메시지 전체에서 쓰이지 않은** 필드 번호를 씁니다(oneof는 메시지의 번호 공간을 공유합니다). 전체 점검 목록은 [contracts/README](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.ko.md)에 있습니다.

## 테스트

```bash
go test ./...                                              # unit (run it in each repo)
(cd ../pqcota-discovery && bash collectors/openssl/integration/run.sh)   # openssl collector real integration (Docker, SD-1·SD-3·SD-4)
./demo/scripts/up.sh && ./demo/scripts/demo.sh            # end-to-end discovery demo
```

테스트를 먼저 씁니다(TDD). 각 단계의 테스트가 수용 기준을 고정합니다.

## 언어

**문서는 영문이고, 주석은 한국어이며, 코드가 내보내는 것은 영문입니다.** 기준은 "누가 읽는가"가 아니라 **어디까지 퍼지는가**입니다.

| | 언어 | 이유 |
|---|---|---|
| 문서(`*.md`) | **영문**(정본). 한국어 번역이 있으면 `*.ko.md`입니다 | 영문이 가장 넓은 독자에게 닿고, 필요한 문서만 번역합니다 |
| **주석** | **한국어**(기존) | 코드를 읽는 사람에게만 닿고, 프로그램 밖으로 나가지 않습니다 |
| **콘솔 출력**(stdout, stderr, 플래그 도움말) | **영문** | 로그에 남고, 이슈에 붙여넣어지고, 낯선 사람이 읽습니다 |
| **계약에 실리는 문자열**(`Completeness.Note`, `Attribution.Reason`, 조치 메모) | **영문** | 저장되어 **계약을 통해 밖으로 나가므로**, 언어가 계약의 일부가 됩니다 |
| **오류 값**(`errors.New`, `fmt.Errorf`) | **영문** | 어디로 흘러가는지는 호출자가 정하며 우리가 정하지 않습니다 |
| **테스트 실패 메시지** | **영문** | CI 로그에 남습니다 |

주석을 뺀 모든 것은 프로그램이 **내보내는** 것이고, 내보내는 것은 독자를 고를 수 없습니다. 화면에 한국어가 나와야 한다면 그것은 뷰의 일이며, 관측 데이터에 한국어를 넣을 이유가 아닙니다.

**`*.ko.md` 번역이 있는 영문 문서를 고칠 때는 같은 변경에서 번역도 고치세요**(또는 PR에 번역이 낡았다고 적으세요). 번역 기여를 환영합니다.

### "안 된다"라고 쓸 때는 이유를 그 자리에 붙입니다

이 리포지터리는 한계를 숨기지 않으므로 부정문이 흔합니다. *항상 되지는 않는다 · 정해지지 않았다 · 만들지 않는다*. 그러나 **이유가 두 문장 뒤에 나오면 독자는 그 빈칸을 추측으로 채웁니다.** 판단과 그 근거는 함께 있어야 합니다.

| 이렇게 쓰지 않습니다 | 이렇게 씁니다 |
|---|---|
| 앱까지는 알아내지만 **항상 되지는 않습니다**. (…두 문장 뒤에 설명이 나옵니다…) | 앱까지는 알아내지만 **조회하는 시점에 소켓이 살아 있을 때만** 됩니다 |
| 자동으로는 **판정하지 않습니다**. | 자동으로는 판정하지 않습니다. **살아 있는지 낡았는지는 사람만 압니다** |
| 관리자 UI는 **만들지 않았습니다**. | 관리자 UI는 만들지 않았습니다. **화면이 생기면 "여기서도 승인하자"가 다음 단계입니다** |

## 이슈와 제안

**버그, 질문, 제안은 이슈로 올립니다.** 숨길 것이 없고, 열린 논의는 다음 사람을 위해 남습니다. 이슈와 PR은 영어나 한국어로 쓸 수 있습니다. 비공개로 두어야 하는 것은 **수정 전에 알려지면 사용자를 공격에 노출할 수 있는 것** 하나뿐이며, 그 경로는 [SECURITY](SECURITY.ko.md)에 있습니다.

버그 보고에 다음을 포함하면 재현이 빨라집니다.

- 기대한 것과 실제로 일어난 일
- 실행한 명령과 그 출력(민감한 값은 가리세요)
- 환경: OS와 Go 버전(Linux라면 `uname -r`과 배포판). Linux 수집기는 **커널 3.2 이상**을 전제합니다
- 관측 관련 이슈라면 대상 런타임(OpenSSL 버전, JDK 배포판)

**큰 변경은 PR 전에 이슈를 여세요.** 계약(`contracts/`)이 여기서 단일 원본이므로, 스키마나 경계를 건드리는 것은 먼저 설계 합의가 필요합니다. 코드를 쓴 뒤에 의견이 다르다는 것을 알게 되면 양쪽 모두 비용이 듭니다.

## 설계 먼저

기능을 추가하기 전에 단계별 README([관측](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/README.ko.md) · [인벤토리](https://github.com/randyinthedev-hash/pqcota-inventory/blob/main/README.ko.md) · [전환물 생성](https://github.com/randyinthedev-hash/pqcota-provisioning/blob/main/README.ko.md))와 [contracts/](https://github.com/randyinthedev-hash/pqcota-common/blob/main/contracts/README.ko.md)를 읽고, 변경이 경계를 건드린다면 이슈를 여세요.
