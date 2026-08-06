# openssl-collector — 로드된 OpenSSL 런타임 관측

**설계 목표**: 그 호스트에 *설치된* 게 아니라 **실행 중인 프로세스가 실제로 로드한** libssl/libcrypto를 본다. 패키지 목록은 "깔려 있다"까지만 말하지만, 마이그레이션이 알아야 하는 건 **무엇이 지금 쓰이고 있고, 그걸 바꾸면 어느 앱이 영향받는가**다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../../docs/플랫폼_규정.md)의 절 번호다.

근거: [디스커버리 설계 §2.1](../../디스커버리_설계.md) · 인수 기준: [테스트케이스 SD-1·SD-3·SD-4](../../디스커버리_테스트케이스.md)

## 무엇을 관측하나

| 축 | 어떻게 | 왜 |
|---|---|---|
| **로드된 라이브러리** | `/proc/<pid>/maps` 파싱 | 실행 중 실체. "설치됨"과 구별된다 |
| **버전** | ELF에서 추출 | 교체 대상 판정의 기준 |
| **fork** (OpenSSL·BoringSSL·LibreSSL·AWS-LC) | ELF 문자열 시그니처 매칭 | **soname이 같아 이름으론 구분 불가**하다.<br>fork마다 PQC 지원·이관 경로가 다르다 |
| **바인딩 방식** (동적·정적) | `/proc` 매핑 + ELF | 정적 링크는 교체 방법이 달라진다 |
| **앱 귀속**(`app_keys`) | 어느 프로세스가 로드했나 | 공유 `.so` 하나를 여러 앱이 쓰면 **다중 귀속** — 교체의 영향 반경 |

## 외부 도구에 의존하지 않는다

`/proc`과 ELF를 **Go로 자체 파싱**한다(`debug/elf`). `ldd`·`lsof`·`ss`·`readelf`를 부르지 않는다.

레거시 서버는 최소 이미지이거나 도구가 없거나 버전이 제각각이다. 외부 도구에 기대면 **관측 실패가 환경 탓으로 흩어져** 무엇을 못 봤는지조차 불분명해진다. 자체 파싱이면 발자국이 정적 바이너리 하나로 끝나고, 실패 원인도 collector 안에서 통제된다.

## 정직성 — 못 본 것을 "없다"로 만들지 않는다

- **fork를 못 가리면** 빈 문자열로 두고 `evidence_strength=INFERRED_LOW`로 낮춘다. 추정해서 채우지 않는다(TD-FORK-2).
- **대상이 실행 중이 아니면** 자산 0이 아니라 **PROCESS 계층 갭**으로 보고한다 — "안 쓴다"가 아니라 "이번 창에서 못 봤다"(§2.7).
- 네임스페이스가 분리돼 `/proc`이 안 보이면 역시 **갭**이다(TD-CONTAINER-2). 수집 실패를 조용한 0으로 바꾸지 않는다.

## 경계 — collector는 관측까지

`evidence_strength`·`pqc_readiness` 같은 **파생값은 코어가 채운다**(§0.2 — 규칙이 한 곳에 있어야 재계산으로 재현된다). collector는 본 것을 정규화된 CBOM Envelope로 낼 뿐이다.

## 전제

- **Linux 전용** — `/proc` 의존(`//go:build linux`).
- 다른 사용자의 프로세스를 보려면 **root 또는 `CAP_SYS_PTRACE`**.
- 컨테이너에서는 대상과 **같은 PID 네임스페이스**여야 한다(아니면 갭).

## 구조

| 파일 | 역할 |
|---|---|
| `procmaps.go` | `/proc/<pid>/maps` 파싱 → 로드된 라이브러리 |
| `elfstrings.go` | ELF 문자열 섹션에서 fork 판별용 문자열 추출 |
| `detect.go` · `scan.go` | 라이브러리별 탐지 결과 조립, 노드 단위 스캔 |
| `build.go` | 정규화된 CBOM Envelope(`CollectionResult`) 생성 |
| `service.go` | intake 계약(§6.1)으로 노출 |
| `integration/` | 실물 통합 테스트(Docker) — SD-1·SD-3·SD-4 |

## 돌려보기

```bash
go test ./discovery/collectors/openssl/...              # 단위
bash discovery/collectors/openssl/integration/run.sh    # 실물 통합(Docker)
```

진입점은 [`discovery/cmd/pqcota-nodescan`](../../cmd/README.md).
