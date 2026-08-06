한국어 · [English](CONTRIBUTING.en.md)

# 기여 안내 (CONTRIBUTING)

pqcota를 **포크·확장·기여**하려는 개발자용. 플랫폼을 *써보려는* 사용자는 루트 [README](README.md)와 [demo/](demo/)를 보면 된다.

> **§ 표기**: 별도 언급이 없으면 [규정서](docs/PQC플랫폼_규정.md)의 절 번호다.

## 사전 요구

**Go 1.26.4+**(`go.mod`의 `go` 지시자보다 낮으면 툴체인이 거부한다)와 **buf** + `protoc-gen-go`·
`protoc-gen-go-grpc`가 필요하다. JVM collector를 만진다면 **JDK 11+**도 있어야 한다(선택 — 없으면 사이드카 빌드만 건너뛴다).
리포를 빌드했으면 [예제](examples/)는 그대로 돌아간다(JVM·OpenSSL 통합 예제만 **Docker**를 더 쓴다).

이 문서는 **리포에 기여하는** 경우를 다룬다. 쓰기만 한다면 빌드·실행은 [루트 README](README.md#빌드)로 충분하다.

### 어느 OS에서 빌드할 수 있나

| OS | `go build` · `go test` | `make`(게이트) | 노드에 올릴 바이너리 |
|---|---|---|---|
| **Linux** | ✅ | ✅ | ✅ 그대로 |
| **macOS** (amd64·arm64) | ✅ | ✅ | ✅ 교차 빌드 |
| **Windows** (amd64·arm64) | ✅ | POSIX 셸 필요 → **WSL** | ✅ 교차 빌드 |

리눅스 전용 코드(`/proc`·AF_PACKET·attach)는 `//go:build linux`로 갈라 두고 다른 OS에는 거부 스텁을
둔다. 그래서 macOS·Windows에선 그 코드가 컴파일 대상에서 빠지고, 깨져도 호스트 빌드는 통과한다 —
`make build`가 **호스트 + linux/amd64 교차**를 함께 확인하는 이유다.

## 개발 루프

빌드 절차는 [루트 README · 빌드](README.md#빌드)와 같다.
기여할 때 더 쓰는 것은 게이트와 테스트다:

```bash
make            # 전체: generate + lint + fmt-check + check-boundary + check-docs + vet + build + build-jar + test
go test ./...   # 단위
```

`make build`는 **산출물을 남기지 않는다** — 호스트와 linux/amd64 교차 컴파일이 되는지만 확인한다
(리눅스 전용 파일까지). 쓸 바이너리는 루트 README처럼 `-o`로 위치를 정해 만든다.
`make build-jar`는 JDK가 없으면 경고 후 건너뛰므로 Go만 만지는 기여자는 JDK 없이도 `make`가 돈다.
테스트는 실 JVM 없이 돈다.

> **`no required module provides package .../gen/pqcota/...`가 뜨면** 생성을 건너뛴 것이다.
> Go가 함께 제안하는 `go get github.com/pqcota/pqcota/gen/...`는 **답이 아니다** — 받을 수 있는
> 모듈이 아니라 이 리포에서 만들어내는 코드다. `make generate`를 먼저 돌린다.

계약을 바꿨으면 `make lint`(buf lint) + 하위호환 확인:

```bash
make breaking                  # 마지막 릴리스 태그와 대조 — 이미 나간 계약을 깨지 않는지 (CI가 도는 것)
make breaking AGAINST=main     # 작업 중인 브랜치를 main과 대조
```

릴리스 태그가 없는 동안(v0.1.0 전)에는 기준선이 없어 앞의 것이 건너뛴다 — 그 사실을 로그에 찍는다.

그리고 [계약 변경 시 파급 점검](contracts/README.md)(서명 범위·변화 판정)을 함께 볼 것.

## 코드 구조 — 최상위 = 종류, 단계 = 그 안

| 최상위 | 무엇 |
|---|---|
| `contracts/` | 계약 SSOT (protobuf). 네임스페이스가 곧 단계: `pqcota.{common,discovery,inventory,provisioning}.v1` |
| `gen/` | proto 생성 코드 (gitignore) |
| `pkg/` | 라이브러리 로직 — 단계 그룹 `discovery`·`inventory`·`provisioning` + 공유 `kernel`(registry·posture·scope·machineid·sign)·`cbom` |
| `discovery/` · `inventory/` · `provisioning/` | **실행 진입점**(단계별) — 각 `cmd/`(스캐너·드라이버·조회·생성), `discovery/`엔 `collectors/`(레퍼런스 collector)도 |
| `examples/` | **단계별 실행 예제** — 샘플 입력 + `run.sh`로 각 cmd를 최소 설정으로 돌려본다 |
| `demo/` | Docker 종단 데모(접근준비→디스커버리→인벤토리→프로비저닝) |
| `tools/` | 리포 도구 — `checkdocs`(문서 게이트, `make check-docs`가 빌드해 실행) |

즉 **`pkg/`·`contracts/`는 단계로, 최상위 실행 폴더도 단계로** 갈린다. **커맨드를 실제로 돌려보려면 [`examples/`](examples/)** (각 단계 `run.sh`), 어느 커맨드가 뭔지는 각 `<stage>/cmd/README`([discovery](discovery/cmd/README.md)·[inventory](inventory/cmd/README.md)·[provisioning](provisioning/cmd/README.md)) 참조.

## 계약 우선 (contract-first)

- 타입·enum을 바꾸려면 **`contracts/*.proto`를 고치고 `make generate`** 한다. `gen/`을 직접 손대지 않는다.
- `evidence_strength`·`pqc_readiness` 같은 **파생값은 Collector가 아니라 코어가 채운다**(§0.2 — 재계산 위해 규칙은 한 곳에). 상세: [contracts/README](contracts/README.md).
- 통제 어휘의 `*_UNSPECIFIED = 0`은 "unknown"이다 — 빈칸/누락으로 두지 않는다(§2.6).

## Collector 확장 — 계약이 곧 seam

레퍼런스 collector(openssl·jvm·network)는 세 가지 관측 방식의 예일 뿐이다. **관측 대상이 늘면 collector를 새로 붙이면 된다** — 코어를 고치지 않고. 접점은 하나, `CollectionResult`(정규화된 CycloneDX + `pqcota:` properties) 계약이다.

- collector가 하는 일은 **관측 → `CollectionResult` emit**까지다. `evidence_strength`·`pqc_readiness` 같은 파생값은 **채우지 않는다** — 코어가 계약 입력에서 파생한다(§0.2 — 규칙이 한 곳에 있어야 재계산으로 재현된다).
- 계약만 맞추면 언어도 자유다(레퍼런스도 Go·Java 폴리글랏). 도구 고유 enrichment는 표준 `properties` 확장 키(규약: [contracts/README](contracts/README.md))에 싣는다.
- 각 레퍼런스 collector의 설계 목표·경계·정직성 규칙은 [`discovery/collectors/<name>/README`](discovery/collectors) 참고 — 새 collector도 같은 틀(관측까지·못 본 건 갭으로·추측 금지)을 따른다.

> **provisioning 생성기는 아직 이런 플러그인 seam이 아니다** — 계획(`plan.proto`)은 공개 계약이지만 생성기 자체는 내부 로직이다. 오해 없게 collector 쪽만 확장 지점으로 둔다.

## 새 암호 런타임을 확장하려면

[암호 런타임 수용 원칙](docs/암호_런타임_수용_원칙.md)을 보라.

## 코딩 가이드라인

이 리포는 **정직성·결정론**을 코드에서 강제한다. 아래는 그 관례다 — 일반 Go 스타일이 아니라 **여기서 유독 지키는 것**만 적는다.

**포맷·검사.** `gofmt`(`go fmt ./...`)로 포맷한다. `make`(전체)가 `buf lint`·`fmt-check`·`check-boundary`·`check-docs`·`go vet`·`build`(호스트+리눅스 교차)·`build-jar`·`go test`를 돌리니 PR 전 **전부 그린**이어야 한다. `check-docs`는 md를 검사한다: 끊어진 링크·앵커, 리포 바깥을 가리키는 위치 선언(안 하는 일은 "하지 않는다"로 적는다), 역할분담 산문(문서에는 **기능과 사용법**만), 개인 개발 환경 정보, 라이선스 표와 실제 의존성의 불일치. 표준 Go 관용을 따르되 도메인 용어는 규정서 어휘를 그대로 쓴다(`finding`·`app_key`·`crypto_runtime`).

**주석은 "왜"를 국문으로, §를 달아서.** 코드가 *무엇을* 하는지는 코드가 말한다 — 주석은 *왜 이렇게* 했고 어긴 대안이 왜 틀린지를 적고, 근거를 규정서 §로 건다. 이 리포의 주석이 유독 긴 이유다. 예: `// ★ 제외는 "없음"이 아니다 — 정책으로 뺀 걸 조용히 사라지게 하면 인벤토리가 거짓말한다(§2.7)`.

**정직성을 코드로 강제한다** — 문서가 아니라 실행에서 지켜져야 한다:
- **unknown은 1급**(§2.6) — 판별 불가는 빈칸이 아니라 `*_UNSPECIFIED`/명시적 "미상"으로 둔다. 통제 어휘 enum의 `0`은 항상 unknown.
- **갭 ≠ 부재**(§2.7) — 못 본 것·정책으로 뺀 것을 조용히 드롭하지 않는다. **세어서 돌려주고 고지**한다(제외 건수·완전성 맵·`-diff` 역순 경고처럼).
- **추측·판정 금지**(§2.1) — 관측 안 한 걸 지어내지 않는다. diff가 "변화 없음"이면 그게 정답이다.

**파생값은 원본에서 재계산 가능하게**(§0.2). `evidence_strength` 같은 파생은 collector가 아니라 코어가 원본(`detection_method`)에서 결정론적으로 만든다 — 규칙이 한 곳(`pkg/discovery/normalize`)에 있어야 재현된다. **서명·정규화 경로엔 벽시계·난수를 넣지 않는다**(같은 입력→같은 바이트). 내용 지문은 휘발 필드(관측 횟수·`last_seen`)를 뺀다.

**순수 함수로 테스트 가능하게.** 파싱·판정 로직을 I/O에서 떼어 실물(프로세스·DB·네트워크) 없이 단위 테스트되게 쓴다 — 예: `ParseProcMaps(reader)`는 `/proc` 없이 돈다. **테스트는 동작만이 아니라 "왜 이 불변식인지"를 못박는다**(회귀 테스트엔 그 버그의 본질을 주석으로).

**외부 도구에 의존하지 않는다.** `ldd`·`lsof`·`ss`·`readelf`를 부르지 않고 `/proc`·ELF를 Go로 직접 파싱한다(최소 이미지·발자국, §2.4). 배포 바이너리는 `CGO_ENABLED=0` 정적 빌드. OS 프리미티브를 만지는 코드는 `//go:build linux`로 태그하고, 순수 헬퍼는 OS 무관으로 분리한다.

**계약을 바꾸면 딸린 것도.** collector가 주장하는 필드는 전부 `sign.Canonical`에 들어가야 한다(서명 사각 금지). oneof arm은 **메시지 전체에서 안 쓰인** 필드 번호를 쓴다(oneof는 메시지의 번호 공간을 공유한다). 상세 체크리스트: [contracts/README](contracts/README.md).

## 테스트

```bash
go test ./...                                              # 단위
bash discovery/collectors/openssl/integration/run.sh      # openssl collector 실물 통합(Docker, SD-1·SD-3·SD-4)
./demo/scripts/up.sh && ./demo/scripts/demo.sh            # 종단 디스커버리 데모
```

인수 기준·구현 순서는 [docs/디스커버리_테스트케이스](discovery/디스커버리_테스트케이스.md)·[인벤토리_테스트케이스](inventory/인벤토리_테스트케이스.md)(TDD).

## 언어

문서의 정본은 **한국어**이고, 영문(`*.en.md`)은 번역본이다 — 기계 번역의 도움을 받으며, 둘이 다르면 한국어가 맞다.

**이슈·PR은 한국어가 가장 빠르다.** 메인테이너가 한국어권이라 영어는 번역을 거쳐 읽고 답한다 — 받지 않는다는 뜻이 아니라 오가는 시간이 길어진다는 뜻이다. 코드·로그·오류 메시지는 원문 그대로 붙여주면 언어와 무관하게 읽힌다.

문서 번역 기여는 환영한다. 다만 **정본은 한국어로 둔다** — 1인 개발이라 두 언어를 동시에 저작하면 어긋난다.

## 이슈 · 제안

**버그·질문·제안은 이슈로 연다.** 숨길 이유가 없고, 공개된 논의가 다음 사람에게 남는다. 비공개로 알려야 하는 것은 **고치기 전에 알려지면 사용자가 공격당할 수 있는 것**뿐이고, 그 경로는 [SECURITY](SECURITY.md)에 있다.

버그를 적을 때 함께 주면 재현이 빠르다.

- 무엇을 기대했고 무엇이 나왔나
- 실행한 명령과 출력(민감한 값은 지우고)
- 환경 — 커널 버전(`uname -r`)·배포판·Go 버전. collector는 리눅스 전용이고 **커널 3.2 이상**을 가정한다
- 관측 쪽이면 대상 런타임(OpenSSL 버전·JDK 배포판)

**큰 변경은 PR보다 이슈가 먼저다.** 이 리포는 계약(`contracts/`)이 단일 진실이라 스키마·경계가 걸리는 변경은 설계 합의가 먼저 있어야 한다 — 코드를 다 쓴 뒤에 방향이 갈리면 서로 손해다.

## 설계를 먼저

기능을 더하기 전에 근거 문서를 본다 — [docs/](docs/README.md)에 규정서·서브시스템 설계가 있고, 코드의 `§` 참조는 전부 거기를 가리킨다.
