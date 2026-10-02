[English](licensing.md) · 한국어

# 라이선스 안내 (서드파티와 프로젝트 라이선스)


**이 문서가 다루는 것**: 오늘 기준으로 `pqcota`가 내부에서 쓰는 **모든 라이선스**를, 쓰이는 형태별로 정리한 것입니다. 배포하는 바이너리에 링크되는지에 따라 라이선스 의무가 완전히 달라지므로 쓰이는 형태가 중요합니다.

> 이 문서는 **라이선스 축**입니다. 각 의존성이 어떤 라이선스인지, 어떤 형태로 쓰이는지를 다룹니다.

> **§ 표기**: 이 문서에 나오는 `§N`은 **이 문서**의 절 번호입니다.

> ⚠️ **면책**: 이 문서는 법률 자문이 아닙니다. 배포 전에는 OSS 라이선스를 전문으로 하는 법률가의 실사(GPL과 AGPL의 구분, or-later, 버전별 정확한 조건)가 필요합니다.

---

## 0. 한 줄 결론

**pqcota 제품 바이너리에 링크되거나 묶여 들어가는 서드파티는 모두 허용형(permissive) 라이선스(Apache-2.0 / MIT / BSD-3)입니다.**
카피레프트(GPL 계열)는 **별도 프로세스로 실행되는 도구**(Ansible, 데모 환경의 JDK)에만 있고, 프로세스 경계가 전염을 막습니다(§5). 그래서 **이 리포의 Apache-2.0 배포에는 카피레프트 오염이 없습니다.**

---

## 1. pqcota 자체의 라이선스

**이 리포(`pqcota`)는 Apache-2.0입니다.** 배포 산출물(Go 바이너리, Java 사이드카)에 링크되거나 묶여 들어가는 서드파티는 모두 허용형입니다(§2 표).

GPL 계열 도구(CBOMkit 등)는 **링크하지도 실행하지도 않습니다.** 그 도구가 만든 CycloneDX 파일만 받습니다(`pqcota-cbom-ingest`). 파일을 주고받는 것이므로 전염 경로가 없습니다 → CBOM 수신을 위임하는 설계.

## 2. 빌드 산출물에 링크되는 런타임 의존성 (Go)

이 리포에서 빌드하는 정적 Go 바이너리(수집기, CLI)에 **컴파일되어 링크되는** 의존성입니다. **모두 허용형입니다.**

| 모듈 | 버전 | 라이선스 | 비고 |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | v5.10.0 | **MIT** | Postgres 드라이버(영속화) |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT | pgx 간접 의존 |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT | pgx 간접 의존 |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT | pgx 연결 풀 |
| `google.golang.org/grpc` | v1.83.2 | **Apache-2.0** | 수신 계약 전송 |
| `google.golang.org/protobuf` | v1.36.11 | **BSD-3-Clause** | 계약 직렬화(protojson 포함) |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260526163538-3dc84a4a5aaa | Apache-2.0 | grpc 간접 의존 |
| `golang.org/x/sys` | v0.47.0 | **BSD-3-Clause** | AF_PACKET(네트워크 수집기) |
| `golang.org/x/net` | v0.58.0 | BSD-3-Clause | grpc 간접 의존 |
| `golang.org/x/sync` | v0.22.0 | BSD-3-Clause | 간접 의존 |
| `golang.org/x/text` | v0.41.0 | BSD-3-Clause | 간접 의존 |

`gopkg.in/yaml.v3`(MIT)는 위 목록에 없습니다. 데모 토폴로지 생성기와 테스트에서만 쓰이므로 수집기나 CLI에는 링크되지 않습니다.

**요약**: 링크되는 카피레프트는 **없습니다.** Apache-2.0, MIT, BSD-3은 서로 호환되며 Apache-2.0 배포에 문제가 없습니다.
(BSD-3과 MIT는 저작권 고지를 보존하라고만 요구합니다 → 배포할 때 `THIRD-PARTY-NOTICES`를 함께 싣기를 권합니다.)

---

## 3. 빌드 시점 도구 (산출물에 링크되지 않음)

코드 생성과 컴파일에만 쓰이고, 결과 바이너리에는 **링크되지 않습니다.**

| 도구 | 라이선스 | 용도 |
|---|---|---|
| Go 툴체인(`golang:1.26`) | BSD-3-Clause (Go) | 컴파일 |
| `buf` (bufbuild/buf) | Apache-2.0 | proto 코드 생성(`buf generate`) |
| `protoc-gen-go` | BSD-3-Clause | Go 메시지 생성 |
| `protoc-gen-go-grpc` | Apache-2.0 | gRPC 스텁 생성 |

---

## 4. 데모 환경 구성 요소 (`demo/`): 별도 프로세스와 컨테이너, 링크되지 않음

> 아래 구성 요소는 관측 데모(`demo/`)에서 SSH, 하위 프로세스, 컨테이너로 실행되며 어떤 pqcota 바이너리에도 링크되지 않습니다.

`demo/`는 컨테이너와 별도 실행 파일로 동작합니다. 거기 있는 어느 것도 pqcota 바이너리에 **정적으로도 동적으로도 링크되지 않습니다.** 모두 SSH, 하위 프로세스, 컨테이너의 프로세스 경계 너머에서 실행됩니다 → **GPL이나 카피레프트가 있어도 전염되지 않습니다**(위와 같은 원리).

| 구성 요소 | 버전 | 라이선스 | 쓰이는 형태 |
|---|---|---|---|
| BouncyCastle `bcprov-jdk18on` | 1.85 | **Bouncy Castle Licence** (MIT X11 계열, 허용형) | pay-app의 JCA provider(별도 JVM). 허용형이라 묶어 넣는 것도 허용됩니다 |
| Eclipse Temurin (OpenJDK) | 21 | **GPLv2 + Classpath Exception** | pay-app의 런타임(별도 컨테이너와 프로세스). CPE가 있어 Java 앱은 GPL에 감염되지 않습니다 |
| OpenSSL | 3.x (Ubuntu) | **Apache-2.0** | web-gw와 pay-db의 TLS(별도 프로세스) |
| OpenSSH (서버/클라이언트) | 9.x | **BSD 계열** (+ 일부 퍼블릭 도메인) | sshd와 ssh(Ansible 전송, 그리고 SSH 엣지 관측의 대상) |
| Ansible | (배포판) | **GPL-3.0-or-later** | 컨트롤러에서 실행하는 **독립 실행 파일**. pqcota와 링크되지 않습니다(오케스트레이션 도구) |
| Graphviz (`dot`) | (배포판) | **CPL-1.0** (Common Public License) | 토폴로지 SVG 렌더링(별도 프로세스). 출력(SVG)은 데이터입니다 |
| Ubuntu 24.04 베이스 이미지 | 없음 | 모음(여러 패키지, 대부분 GPL/LGPL/MIT/BSD) | 컨테이너 베이스 |
| `golang:1.26` 베이스 이미지 | 없음 | 모음(Go = BSD-3 + Debian 베이스) | 빌더 단계 |

> **GPL 도구(Ansible, Temurin)를 다루는 방식**: 이 도구들은 pqcota가 **호출하는** 별도 프로그램이지 링크 대상이 아닙니다.
> Ansible은 플레이북을 실행하는 오케스트레이터이고, Temurin은 대상 노드의 런타임일 뿐입니다. GPL은
> 「저작물에서 파생하거나 링크하는 것」을 통해 전파되므로, 프로세스 호출에 그치는 관계에는 적용되지 않습니다.
> 데모를 배포하거나 재배포할 때 이 도구들은 **사용자가 설치**하므로(이미지를 빌드할 때 내려받음) pqcota는 재배포하지 않습니다.

---

## 5. 카피레프트 격리: 무엇이 지키는가

**GPL 카피레프트의 전염은 구조적으로 막혀 있습니다.** 원칙은 셋입니다.

1. **프로세스 분리**: GPL 구성 요소는 독립된 바이너리로 호출하고, 라이브러리로 링크하지 않습니다.
2. **표준 데이터 경계**: 프로세스 사이의 교환은 CycloneDX CBOM(표준)으로만 합니다. 핵심 내부 API는 이 경계를 넘지 않습니다.
3. **배포 분리**: GPL 코드는 이 리포나 그 배포물에 묶어 넣거나, 정적으로 링크하거나, vendor하지 않습니다.

직접 GPL 수집기를 만들더라도 같은 경계가 적용됩니다. 구성 요소별 정확한 라이선스(GPL과 AGPL, or-later)와 SaaS 배포에서의 AGPL 영향은 **실사가 필요합니다.** 이 문서는 법률 자문이 아닙니다.

이 원칙을 지키는 것은 아래 표입니다.

| 원칙 | 무엇이 지키는가 |
|---|---|
| 별도 프로세스 | `contracts/.../collector.proto`의 수신 계약: GPL 수집기는 gRPC/CLI 뒤에 서 있습니다 |
| 표준 데이터만 | 이 계약은 CycloneDX + Envelope만 싣습니다. 핵심 내부 타입은 넘지 않습니다 |
| 배포 분리 | GPL 어댑터는 **별도 리포**이고 `go.mod`에 없습니다. 지금은 CI에서 라이선스를 자동으로 점검하지 않으므로 검토로 지킵니다 |

`demo/`의 Ansible(GPL-3)과 Temurin(GPLv2+CE)도 같은 경계 밖에서 별도 프로세스로 실행됩니다.

**이 리포는 Apache-2.0으로 공개됩니다.** 요점은 관측 가능성을 열어 커뮤니티가 수집기를 더할 수 있게 하는 것이고, 참조 수집기가 OSS인 것도 같은 이유입니다(시중 도구가 다루지 않는 JVM 내부 조회도 포함합니다).

| 구분 | 라이선스 | 담는 것 |
|---|---|---|
| **이 리포** | **Apache-2.0** | 계약, 정규화, 인벤토리, 전환물 생성 + 참조 수집기 |
| **GPL 어댑터**(선택) | GPL-3.0, 별도 리포 | CipherIQ/CBOMkit 하위 프로세스 래퍼 |
| **PQC provider 라이브러리** | 자산마다 다름 | BouncyCastle(허용형) · BC-FJA(FIPS, 별도 계약): 사용자가 조달합니다 |

**영향은 선택하는 자리에서 보여 줍니다.** 수집기 선택 UI는 `CollectorCapabilities.license`를 써서, 그 백엔드가 *「따로 설치하는 GPL 구성 요소」*인지 *「참조 구현 = Apache-2.0, 포함됨」*인지 밝힙니다.

---

## 부록: 쓰이는 형태별 한눈에 보기

| 쓰이는 형태 | 카피레프트가 있는가 | 전염 위험 | 예 |
|---|---|---|---|
| 빌드 산출물에 링크됨(§2) | ❌ 없음 | 없음 | pgx (MIT), grpc (Apache), protobuf (BSD) |
| 빌드 시점(§3) | ❌ 없음 | 없음 | buf, protoc-gen-* |
| 데모의 별도 프로세스(§4) | ✅ 있음 (Ansible, Temurin) | **격리로 막힘** | Ansible (GPL-3), Temurin (GPLv2+CE) |
