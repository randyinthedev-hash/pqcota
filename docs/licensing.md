한국어 · [English](licensing.en.md)

# 라이선스 정리 (Third-Party & Project Licensing)

**문서 성격**: 현 시점 `pqcota`가 **내부적으로 사용하는 모든 라이선스**를 소비 형태별로 정리한다.
소비 형태를 나누는 이유는 라이선스 의무가 "배포 바이너리에 링크되는가"에 따라 완전히 달라지기 때문이다.

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

> ⚠️ **면책**: 본 문서는 법률 자문이 아니다. 배포 전 OSS 라이선스 전문 변호사의
> 실사(GPL vs AGPL, or-later, 정확한 버전별 조건)가 필수다.

---

## 0. 핵심 결론 (한 줄)

**pqcota 제품 바이너리에 링크·번들되는 서드파티는 전부 허용적 라이선스(Apache-2.0 / MIT / BSD-3)뿐이다.**
카피레프트(GPL 등)는 **오직 별도 프로세스로 실행되는 도구**(데모 환경의 Ansible·JDK 등)에만 존재하며,
프로세스 경계로 전염이 차단된다(§5). 따라서 **이 리포의 Apache-2.0 배포에 카피레프트 오염이 없다.**

---

## 1. pqcota 자체 라이선싱

**이 리포(`pqcota`)는 Apache-2.0**이다. 배포되는 산출물(Go 바이너리·Java 사이드카)에 링크·번들되는
서드파티는 전부 허용적 라이선스뿐이다(§2 표).

GPL 계열 도구(CBOMkit 등)는 **링크하지도 실행하지도 않는다** — 그 도구가 낸 CycloneDX 파일을
받기만 한다(`pqcota-cbom-ingest`). 파일 교환이라 전염 경로가 없다 → [위임 수신 설계](../inventory/cbom-intake.md).

## 2. 빌드 산출물에 링크되는 런타임 의존성 (Go)

이 리포를 빌드한 Go 정적 바이너리(collector·CLI)에 **컴파일·링크되는** 의존성. **전부 허용적.**

| 모듈 | 버전 | 라이선스 | 비고 |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | v5.10.0 | **MIT** | Postgres 드라이버(영속화) |
| `github.com/jackc/pgpassfile` | v1.0.0 | MIT | pgx 간접 |
| `github.com/jackc/pgservicefile` | v0.0.0-20240606120523-5a60cdf6a761 | MIT | pgx 간접 |
| `github.com/jackc/puddle/v2` | v2.2.2 | MIT | pgx 커넥션 풀 |
| `google.golang.org/grpc` | v1.82.1 | **Apache-2.0** | intake 계약 전송 |
| `google.golang.org/protobuf` | v1.36.11 | **BSD-3-Clause** | 계약 직렬화(protojson 포함) |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260414002931-afd174a4e478 | Apache-2.0 | grpc 간접 |
| `golang.org/x/sys` | v0.46.0 | **BSD-3-Clause** | AF_PACKET(network-collector) |
| `golang.org/x/net` | v0.56.0 | BSD-3-Clause | grpc 간접 |
| `golang.org/x/sync` | v0.21.0 | BSD-3-Clause | 간접 |
| `golang.org/x/text` | v0.39.0 | BSD-3-Clause | 간접 |

`gopkg.in/yaml.v3`(MIT)는 위 목록에 없다 — 데모 토폴로지 생성기와 테스트에서만 쓰여 collector·CLI에 링크되지 않는다.

**정리**: 링크되는 카피레프트 **없음**. Apache-2.0/MIT/BSD-3은 상호 호환이며 Apache-2.0 배포에 문제 없다.
(BSD-3·MIT는 저작권 고지 유지 의무만 있음 → 배포물에 `THIRD-PARTY-NOTICES` 동봉 권장, §6.)

---

## 3. 빌드 타임 도구 (산출물에 링크되지 않음)

코드 생성·컴파일에만 쓰이고 **산출 바이너리에 링크되지 않는다**.

| 도구 | 라이선스 | 용도 |
|---|---|---|
| Go 툴체인 (`golang:1.26`) | BSD-3-Clause (Go) | 컴파일 |
| `buf` (bufbuild/buf) | Apache-2.0 | proto 코드 생성(`buf generate`) |
| `protoc-gen-go` | BSD-3-Clause | Go 메시지 생성 |
| `protoc-gen-go-grpc` | Apache-2.0 | gRPC 스텁 생성 |

---

## 4. 데모 환경 구성요소 (`demo/`) — 별도 프로세스/컨테이너, 링크 안 됨

> 아래 구성요소는 디스커버리 데모(`demo/`)에서 SSH·서브프로세스·컨테이너로 돌며 pqcota 바이너리에 링크되지 않는다.

`demo/`는 컨테이너·별도 실행 파일로 동작한다. pqcota 바이너리에 **정적/동적 링크되지 않으며**, 전부
SSH·서브프로세스·컨테이너 프로세스 경계 너머에서 실행된다 → **GPL/카피레프트가 있어도 전염되지 않는다**(라이선스 정리 원칙과 동일).

| 구성요소 | 버전 | 라이선스 | 소비 형태 |
|---|---|---|---|
| BouncyCastle `bcprov-jdk18on` | 1.85 | **Bouncy Castle Licence**(MIT X11 계열, 허용적) | pay-app의 JCA provider(별도 JVM). 허용적 라이선스라 번들도 가능(프로비저닝 설계 §4.2) |
| Eclipse Temurin (OpenJDK) | 21 | **GPLv2 + Classpath Exception** | pay-app 런타임(별도 컨테이너·프로세스). CPE로 Java 앱은 GPL 미전염 |
| OpenSSL | 3.x(우분투) | **Apache-2.0** | web-gw/pay-db의 TLS(별도 프로세스) |
| OpenSSH (server/client) | 9.x | **BSD 계열**(+일부 public domain) | sshd·ssh(Ansible 전송·SSH 엣지 관측 대상) |
| Ansible | (배포판) | **GPL-3.0-or-later** | 컨트롤러에서 실행되는 **독립 실행 파일**. pqcota와 링크 없음(오케스트레이션 도구) |
| Graphviz (`dot`) | (배포판) | **CPL-1.0**(Common Public License) | 토폴로지 SVG 렌더(별도 프로세스). 산출물(SVG)은 데이터 |
| Ubuntu 24.04 base image | — | 집합(주로 GPL/LGPL/MIT/BSD 등 다수 패키지) | 컨테이너 베이스 |
| `golang:1.26` base image | — | 집합(Go=BSD-3 + Debian 베이스) | 빌더 스테이지 |

> **GPL 도구(Ansible·Temurin) 취급**: 이들은 pqcota가 **호출**하는 별도 프로그램이지 링크 대상이 아니다.
> Ansible은 플레이북을 실행하는 오케스트레이터, Temurin은 타깃 노드의 런타임일 뿐이다. GPL은
> "저작물의 파생·링크"에 전염되므로, 프로세스로만 부르는 이 관계엔 적용되지 않는다.
> 데모를 배포·재배포할 때도 이 도구들은 **사용자가 각자 설치**(이미지 빌드 시 다운로드)하는 형태라 pqcota가 재배포하지 않는다.

---

## 5. 카피레프트 격리 — 무엇이 그것을 강제하나

**GPL copyleft 전염을 구조로 차단한다.** 원칙 셋:

1. **프로세스 분리** — GPL 컴포넌트를 라이브러리 링크가 아니라 독립 바이너리로 호출한다.
2. **표준 데이터 경계** — 프로세스 간 교환은 CycloneDX CBOM(표준)으로만. 코어 내부 API를 넘기지 않는다.
3. **배포 분리** — GPL 코드를 이 리포·배포물에 번들·정적 링크·소스 포함하지 않는다.

자체 GPL collector를 만들더라도 같은 경계를 적용한다. 컴포넌트별 정확한 라이선스(GPL vs AGPL,
or-later)와 SaaS 배포 시 AGPL 함의는 **실사가 필요하다** — 이 문서는 법률 자문이 아니다.

무엇이 그 원칙을 강제하는지는 아래 표다.

| 원칙 | 무엇이 강제하나 |
|---|---|
| 별도 프로세스 | `contracts/.../collector.proto`의 intake 계약(§1.6) — GPL collector는 gRPC/CLI 뒤에 선다 |
| 표준 데이터만 교환 | 그 계약이 주고받는 것은 CycloneDX + Envelope뿐. 코어 내부 타입은 넘지 않는다 |
| 배포 분리 | GPL 어댑터는 **별도 리포**이고 `go.mod`에 없다. CI 라이선스 스캐너가 교차 의존을 막는다 |

`demo/`의 Ansible(GPL-3)·Temurin(GPLv2+CE)도 같은 경계 밖에서 별도 프로세스로 돈다.

**이 리포는 Apache-2.0으로 공개한다.** 관측성을 오픈소스로 풀어 커뮤니티가 collector를 늘릴 수
있게 하려는 것이고, 레퍼런스 collector도 같은 이유로 OSS다(§2.2 "자체 구현 공백"인 JVM 인트로스펙션 포함).

| 구분 | 라이선스 | 포함물 |
|---|---|---|
| **이 리포** | **Apache-2.0** | 계약·정규화·인벤토리·프로비저닝 생성 + 레퍼런스 collector |
| **GPL 어댑터**(선택) | GPL-3.0, 별도 리포 | CipherIQ/CBOMkit 서브프로세스 래퍼 |
| **PQC provider 라이브러리** | 자산별 상이 | BouncyCastle(허용적) · BC-FJA(FIPS, 별도 계약) — 사용자가 조달 |

**사용자가 고를 때 함의를 보인다.** collector 선택 UI는 `CollectorCapabilities.license`로
*"이 백엔드는 GPL 컴포넌트 별도 설치"* 인지 *"레퍼런스 = Apache-2.0 포함"* 인지 밝힌다.

---

## 부록: 소비 형태별 한눈 요약

| 소비 형태 | 카피레프트 존재? | 전염 위험 | 예 |
|---|---|---|---|
| 빌드 산출물 링크(§2) | ❌ 없음 | 없음 | pgx(MIT), grpc(Apache), protobuf(BSD) |
| 빌드 타임(§3) | ❌ 없음 | 없음 | buf, protoc-gen-* |
| 데모 별도 프로세스(§4) | ✅ 있음(Ansible·Temurin) | **격리로 차단** | Ansible(GPL-3), Temurin(GPLv2+CE) |
