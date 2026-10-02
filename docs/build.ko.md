[English](build.md) · 한국어

# 빌드 안내

pqcota를 소스에서 빌드하거나 pqcota 자체를 개발하는 엔지니어를 위한 안내입니다. pqcota를 실행만 하려면 이 문서는 필요 없습니다. 아래 바로가기를 보세요. 전체 개요는 [README](../README.ko.md)에서 시작하세요.

## 실행만 하려면

릴리스마다 미리 빌드한 정적 바이너리와 체크섬 파일이 [릴리스 페이지](https://github.com/randyinthedev-hash/pqcota/releases)에 첨부됩니다. 용도별로 나뉘어 있습니다.

| 묶음 | 보낼 곳 |
|---|---|
| `pqcota-linux-amd64.tar.gz`, `pqcota-linux-arm64.tar.gz` | 관측할 Linux 시스템: `pqcota-nodescan`, `pqcota-netcap`, `pqcota-jvmscan` |
| `pqcota-windows-amd64.zip` | 관측할 Windows 시스템: `pqcota-cngscan`, `pqcota-jvmscan` |
| `pqcota-ctl-linux-amd64.tar.gz`, `pqcota-ctl-linux-arm64.tar.gz` | 중앙 기계 한 대: 적재, 조회, 승인, 생성 명령 |
| `collector.jar` | Java attach 경로를 쓸 때만 |
| `SHA256SUMS` | 릴리스의 모든 자산에 대한 체크섬 목록 |

내려받은 파일을 확인하려면 파일들을 `SHA256SUMS` 옆에 두고 `sha256sum -c --ignore-missing SHA256SUMS`를 실행합니다(GNU coreutils 기준입니다. `--ignore-missing`이 없으면 내려받지 않은 자산마다 오류를 냅니다). **출력을 읽으세요.** 실제로 확인한 파일이 하나하나 출력에 나옵니다. 내려받은 파일은 OK로 나와야 하고, 출력에 파일이 하나도 없으면 아무것도 검증하지 않은 것입니다. 체크섬이 일치하면 전송 중에 파일이 손상되지 않았다는 것까지만 확인됩니다. 누가 배포했는지는 확인되지 않습니다. 서명된 릴리스는 [로드맵](../RELEASE_NOTES.md)(영문)에 있습니다.

시스템에서 수집기를 실행할 때 필요한 권한과 환경변수는 [명령 레퍼런스](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)에 있습니다.

## 요구 사항

**빌드하려면**

- Go 1.26.4 이상. 공식 CI와 릴리스는 Go 1.26.6(`go.mod`의 `toolchain` 줄)으로 빌드합니다. 내 컴퓨터에서 어느 Go를 쓰는지는 `GOTOOLCHAIN` 설정에 따릅니다. 기본값에서는 1.26.6보다 낮은 Go가 1.26.6을 받고, `GOTOOLCHAIN=local`이면 설치된 Go를 그대로 씁니다.
- Git, `make`, POSIX 셸(아래 명령은 POSIX 셸 기준으로 적었습니다).
- `buf`, `protoc-gen-go`, `protoc-gen-go-grpc`: **protobuf 계약을 바꿀 때만** 필요합니다(`pqcota-common`에서). 생성된 Go 코드는 커밋되어 있으므로 일반 빌드에는 필요 없습니다. `buf`는 따로 설치하고(<https://buf.build/docs/installation>), `make tools`는 두 플러그인만 설치합니다.
- JDK 11 이상, **선택 사항**이며 Java attach 사이드카를 빌드할 때만 필요합니다. 없으면 그 단계는 건너뜁니다.

**점검(`make all`)을 실행하려면**

- 위의 도구 전부. `buf`(따로 설치)와 두 플러그인(`pqcota-common`의 `make tools`)을 포함합니다. `make all`은 형제 리포지터리 각각의 자체 점검도 실행합니다. 그 점검은 protobuf 코드를 다시 생성하고 계약 린트와 호환성 점검을 돌리므로, 계약을 바꾸지 않았어도 `make all`에는 `buf`가 필요합니다.
- Postgres 테스트는 `PQCOTA_TEST_DSN`을 설정했을 때만 실행됩니다. 설정하지 않으면 건너뜁니다. 기본 로컬 테스트 실행은 상세 출력이 아니어서 건너뛴 테스트를 나열하지 않습니다. 어느 것을 건너뛰었는지 보려면 리포지터리에서 `go test -v ./...`를 실행하세요.

**실행하려면**

- 여러 시스템: 제어 서버의 Ansible과 대상으로의 SSH 접근.
- 시스템 하나: 상주 에이전트도 서비스도 없습니다. 그 시스템에서 바이너리를 실행하면 됩니다. 일부 관측 경로에는 전제가 있습니다. `pqcota-netcap`에는 `CAP_NET_RAW` 권한이 필요하고, 일부 Java 경로에는 그 기계의 JDK나 `collector.jar` 사이드카가 필요합니다.
- Postgres: 여러 시스템의 이력을 쌓고 조회하려는 경우에만.

## 소스 받기

pqcota는 **리포지터리 다섯 개**입니다. 각 `go.mod`가 `replace` 지시자로 형제 리포지터리를 `../`에서 읽으므로 나란히 클론합니다.

```bash
git clone https://github.com/randyinthedev-hash/pqcota
git clone https://github.com/randyinthedev-hash/pqcota-common        # contracts, generated code, shared logic
git clone https://github.com/randyinthedev-hash/pqcota-inventory     # the inventory stage
git clone https://github.com/randyinthedev-hash/pqcota-discovery     # collectors, their commands, the reference playbook
git clone https://github.com/randyinthedev-hash/pqcota-provisioning  # the provisioning stage
cd pqcota
```

| 리포지터리 | 들어 있는 것 |
|---|---|
| `pqcota` | 데모, 예제, 릴리스 묶음, 게이트 도구와 단계에 걸치는 점검, 기여 안내 |
| `pqcota-common` | protobuf 계약과 그 생성 Go 코드, 공유 코드(식별, 서명, 키 생성) |
| `pqcota-inventory` | 정규화, 추가만 가능한 이력, 조회 |
| `pqcota-discovery` | 수집기, 그 명령, 참조 Ansible 플레이북 |
| `pqcota-provisioning` | 계획 승인, 산출물 생성, 실행 기록 |

릴리스는 다섯 리포지터리 모두에 붙는 같은 태그입니다(예: `v0.10.4`). 그냥 클론하면 각 리포지터리의 `main`을 받는데, 이는 개발 중인 상태입니다. 릴리스를 빌드하려면 **다섯 리포지터리 모두**에서 그 태그를 체크아웃합니다. 예를 들어 `pqcota` 디렉터리(위의 `cd pqcota`)에서 `for r in pqcota pqcota-common pqcota-inventory pqcota-discovery pqcota-provisioning; do git -C ../$r checkout v0.10.4; done`을 실행합니다. 체크아웃하면 각 리포지터리는 그 태그의 detached HEAD 상태가 됩니다. `go.mod`의 `require` 줄은 형제 리포지터리를 고정하지 않습니다. 로컬 `replace` 지시자가 바로 옆의 작업 트리를 읽기 때문입니다.

**태그가 이 작업 공간 밖의 사용자에게 주는 것.** 태그의 패키지를 라이브러리로 임포트하는 것은 됩니다. Go는 의존 대상의 `replace` 줄을 무시하고 `require` 줄을 태그로 해석합니다. 태그에서 바로 명령을 실행하는 것은 되지 않습니다. `go install github.com/randyinthedev-hash/pqcota-discovery/cmd/pqcota-hosts@v0.10.4`와 `go run github.com/randyinthedev-hash/pqcota/tools/checkprose@v0.10.4`는 `go.mod`에 `replace` 지시자가 있는 동안 Go가 거부합니다. 명령을 실행하려면 위의 다섯 리포지터리 체크아웃에서 빌드하거나 릴리스 묶음을 받으세요. 2026-10-02 현재 v0.10.0, v0.10.1, v0.10.2도 마찬가지입니다.

`go run …/tools/checkprose@v0.9.1`은 그 태그가 분리보다 앞서므로 지금도 동작합니다. 이 태그를 고정하면 v0.9.1의 검사기를 계속 쓰는 것이며, 더 새로운 검사기를 받는 방법이 아닙니다. 검사기는 그 뒤로 바뀌었습니다.

## 빌드

pqcota에는 **중앙 제어 서버** 하나와, 제어 서버가 Ansible과 SSH로 닿는 **대상 시스템**이 있습니다. 제어 서버에서 빌드합니다. 제어 서버에서 실행할 명령과 대상에 보낼 수집기가 모두 여기서 나옵니다.

**제어 서버에서 실행할 명령.** 아래의 단순한 `go build`는 자기 기계용 개발 빌드입니다. 릴리스 묶음은 `CGO_ENABLED=0 GOOS=linux GOARCH=<arch> go build -trimpath -ldflags='-s -w'`로 정적 빌드합니다. 비교할 수 있는 산출물이 필요하면 같은 플래그를 쓰세요.

```bash
D=github.com/randyinthedev-hash
go build -o bin/ $D/pqcota-common/cmd/... $D/pqcota-discovery/cmd/... $D/pqcota-inventory/cmd/... $D/pqcota-provisioning/cmd/...
```

**대상 시스템에 올릴 수집기**: 대상의 운영체제와 아키텍처에 맞춰 정적으로 빌드합니다. 어느 수집기가 어느 운영체제에서 도는지는 [명령 레퍼런스](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)에 있습니다.

```bash
D=github.com/randyinthedev-hash/pqcota-discovery/cmd

# Linux targets
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux-amd64/ $D/pqcota-nodescan $D/pqcota-netcap $D/pqcota-jvmscan

# Windows targets
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows-amd64/ $D/pqcota-cngscan $D/pqcota-jvmscan

make build-jar        # only for Java targets: builds the attach sidecar to build/collector.jar (needs a JDK 11 or newer)
```

**JDK가 없으면 `make build-jar`는 경고를 출력하고도 성공으로 종료하며, `collector.jar`는 만들어지지 않습니다.** Java attach 경로를 쓰기 전에 `build/collector.jar`가 있는지 확인하세요.

`CGO_ENABLED=0`(정적 링크이며 Linux 배포판과 libc에 의존하지 않음)은 고정입니다. 바꾸는 것은 `GOOS`와 `GOARCH`이고, 허용되는 값은 [Go 문서](https://go.dev/doc/install/source#environment)에 있습니다.

**커널 하한.** Go 자체의 Linux 최소 요구는 커널 3.2이므로([Go 최소 요구 사항](https://go.dev/wiki/MinimumRequirements)), 이렇게 빌드한 바이너리의 일반적인 하한은 그것입니다. pqcota의 개별 기능은 더 높은 버전이 필요할 수 있고, 그것은 [명령 레퍼런스](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/cmd/README.md)(영문)에 있습니다. Windows 바이너리는 여기서 교차 컴파일한 것입니다. 이 안내는 특정 Windows 버전에서 실행해 보았다고 주장하지 않습니다.

## 작업 확인

`pqcota-common`에서 `make tools`를 한 뒤 `pqcota`에서 실행합니다.

```bash
make all
```

`make all`은 형제 리포지터리 각각의 자체 점검을 실행한 다음, 다섯 리포지터리 전체에 걸치는 점검을 실행합니다. 포맷, 표현 경계, 문서 링크, 수집기 목록, 게이트 배선, 단계 사이의 임포트 방향, 산문 게이트, vet, 빌드, 테스트입니다. 로컬 점검은 protobuf 코드를 다시 생성하고 계약 린트를 돌립니다. **CI는 여기에 더해, 다시 생성한 코드와 커밋된 파일의 차이를 모두 거부하므로** 생성한 코드는 proto 변경과 함께 커밋하세요. CI는 또 Postgres 서비스를 제공하고, `CAP_NET_RAW`가 필요한 테스트를 권한 있는 잡에서 돌리고, arm64용으로 교차 빌드합니다. 로컬 `make all`이 통과해도 이것들은 재현되지 않습니다. 각 점검이 무엇을 지키고 통과해도 무엇을 보여 주지 않는지는 [점검과 게이트](checks-and-gates.ko.md)를 보세요. 어느 단계 리포지터리든 바꾸면 CI에서 단계를 가로지르는 점검도 실행됩니다.

## 계약 변경

계약은 `pqcota-common`에 있습니다. `.proto` 파일을 바꾼 뒤에는 Go 코드를 다시 생성하고, proto 변경과 **같은 커밋**에 넣습니다.

```bash
cd ../pqcota-common
make tools && make generate     # contracts/proto → gen/
```

`make tools`는 플러그인을 `$(go env GOPATH)/bin`에 설치합니다. 그 디렉터리가 `PATH`에 없으면 `make generate`가 플러그인을 찾지 못했다고 알리는데, 설치에 실패한 것처럼 보이지만 보이지 않는다는 뜻일 뿐입니다. 셸 프로필에 `export PATH="$PATH:$(go env GOPATH)/bin"`을 추가하세요.

계약 변경은 추가만 가능해야 합니다. [호환성 정책](compatibility.ko.md)을 보세요. `make breaking`은 마지막 릴리스 태그와 비교합니다. 작업 순서와 계약과 함께 바꿔야 하는 다른 것들은 [계약 변경](change-a-contract.ko.md)을 보세요.

## 기술 스택

- **Go**: 모든 수집기와 명령. 릴리스 묶음은 정적 단일 바이너리입니다(`CGO_ENABLED=0`).
- **Java**: attach 사이드카만. 그 관측은 JVM 안에서만 가능하기 때문입니다.
- **Protobuf와 gRPC**: 단계를 잇는 계약([`contracts/`](https://github.com/randyinthedev-hash/pqcota-common/tree/main/contracts)).
- **Postgres**: 많은 시스템을 시간에 걸쳐 쌓고 조회할 때만. 시스템 하나를 관측할 때는 쓰지 않습니다.

## 다음에 볼 곳

리포지터리 다섯 개가 어떻게 맞물리고 무엇을 읽어야 하는지는 [개발자 문서](developers.ko.md) · 테스트, 게이트, 변경 제안 방법은 [CONTRIBUTING](../CONTRIBUTING.ko.md) · 컨테이너에서 전체 흐름을 보려면 [데모](../demo/README.ko.md) · 명령 하나씩 Go만으로 해 보려면 [예제](../examples/README.ko.md).
