# pkg — 라이브러리 로직

실행 진입점(`discovery/cmd`·`inventory/cmd`·`provisioning/cmd`)이 **얇은 조립층**이고, 실제 로직은 여기 있다. 커맨드를 바꾸지 않고도 로직을 테스트·재사용할 수 있게 가른 것이다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/플랫폼_규정.md)의 절 번호다.

두 갈래다 — **단계별 패키지**와 **단계를 가로지르는 [`kernel`](kernel/README.md)**.

## 단계별

각 패키지의 "왜 이렇게 만들었나"는 해당 **단계 설계 문서**에 있다. 여기서는 어디를 보면 되는지만 가리킨다.

통과해야 하는 **인수 기준**은 단계별 테스트케이스 문서에 있다. 문서는 단계로, 패키지는 재사용 단위로 나뉘어 한 단계의 케이스가 패키지 여럿에 걸치므로, 그 대응은 [테스트 명세 지도](../docs/테스트_명세_지도.md)에 한자리로 펴 뒀다.

| 패키지 | 하는 일 | 설계 근거 |
|---|---|---|
| [`discovery/normalize`](discovery/normalize) | 정규화 파이프라인 후단 — Finding 파생, 동일성 해소, 완전성 병합, 자산 스코프 게이트 | [디스커버리 §2.5](../discovery/디스커버리_설계.md) |
| [`discovery/history`](discovery/history) | append-only 히스토리 — 스냅샷·관측 기록 2층, 내용 지문, 보존 정책 절단 | [인벤토리 §13](../inventory/인벤토리_설계.md) |
| [`inventory/ingest`](inventory/ingest) | 중앙 적재 관문 — 스코프 게이트, 서명 검증, 외부 CBOM 수신 | [위임 수신 설계](../inventory/위임수신_설계.md) |
| [`discovery/procs`](discovery/procs) | 프로세스↔앱 귀속 해소 | [디스커버리 §0.5](../discovery/디스커버리_설계.md) |
| [`inventory`](inventory) | 읽기전용 뷰 렌더(누적·이력·상세·diff), 머신 메타데이터 스토어, hosts 파서 | [인벤토리 설계](../inventory/인벤토리_설계.md) |
| [`inventory/declaration`](inventory/declaration) | 사용자 선언(CMDB) 임포트 — 관측 레인과 구분되는 선언 레인 | [인벤토리 §2](../inventory/인벤토리_설계.md) |
| [`provisioning`](provisioning) | 확정 계획 게이트, taxonomy→config 아티팩트, 적용·롤백 플레이북 생성, before 캡처·롤백 레코드 | [프로비저닝 설계](../provisioning/프로비저닝_설계.md) |

## 공유

| 패키지 | 하는 일 |
|---|---|
| [**`kernel`**](kernel/README.md) | 단계를 가로지르는 **결정론적 판정 규칙**과 게이트 — 스코프, 등급, 시그니처 레지스트리, 서명, 머신 지문. **무엇이 kernel에 속하는지의 기준**이 그 README에 있다 |

## 방향 규칙

의존은 **단계 → kernel** 한 방향이다. kernel은 단계 패키지를 import하지 않는다 — 순환을 막고, 단계를 떼어내도 kernel이 따라오지 않게.

파생값(`evidence_strength`·`pqc_readiness`·등급)은 **collector가 아니라 코어가** 채운다. 규칙이 한 곳에 있어야 원본에서 재계산으로 재현된다(§0.2). collector 쪽 경계는 [`discovery/collectors`](../discovery/collectors) 각 README 참조.
