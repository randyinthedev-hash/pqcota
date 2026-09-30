# pkg: 라이브러리 로직

실행 진입점(`discovery/cmd`·`inventory/cmd`·`provisioning/cmd`)이 **얇은 조립층**이고, 실제 로직은 여기 있다. 커맨드를 바꾸지 않고도 로직을 테스트·재사용할 수 있게 가른 것이다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

두 갈래다. **단계별 패키지**와 **단계를 가로지르는 [`kernel`](kernel/README.md)**.

## 단계별

각 패키지의 "왜 이렇게 만들었나"는 해당 **단계 설계 문서**에 있다. 여기서는 어디를 보면 되는지만 가리킨다.

통과해야 하는 **인수 기준**은 단계별 테스트케이스 문서에 있다. 문서는 단계로, 패키지는 재사용 단위로 나뉘어 한 단계의 케이스가 패키지 여럿에 걸치므로, 그 대응은 [테스트 명세 지도](../docs/test-map.md)에 한자리로 펴 뒀다.

| 패키지 | 하는 일 | 설계 근거 |
|---|---|---|
| [`inventory/normalize`](inventory/normalize) | 정규화 파이프라인 후단: Finding 파생, 동일성 해소, 완전성 병합, 자산 스코프 게이트 | [규정서 §2.4](../docs/regulation.md) · [디스커버리 설계 §3](../discovery/design.md) |
| [`inventory/history`](inventory/history) | append-only 히스토리: 스냅샷·관측 기록 2층, 내용 지문, 보존 정책 절단 | [인벤토리 §7](../inventory/design.md) |
| [`inventory/resultio`](inventory/resultio) | 회수한 `CollectionResult`를 파일(`*.json`·`*.jsonl`)에서 읽는 **공식 디코더**. 형식은 확장자가 아니라 내용으로 가린다 | [인벤토리 설계](../inventory/design.md) |
| [`inventory/ingest`](inventory/ingest) | 중앙 적재 관문: 스코프 게이트, 서명 검증, 외부 CBOM 수신 | [위임 수신 설계](../inventory/cbom-intake.md) |
| [`discovery/procs`](discovery/procs) | 프로세스↔앱 잇기 | [디스커버리 설계 §2.3](../discovery/design.md) |
| [`inventory`](inventory) | 읽기전용 뷰 렌더(누적·이력·상세·diff), 머신 메타데이터 스토어, hosts 파서 | [인벤토리 설계](../inventory/design.md) |
| [`inventory/declaration`](inventory/declaration) | 사용자 선언(CMDB) 임포트: 관측 레인과 구분되는 선언 레인 | [인벤토리 §2](../inventory/design.md) |
| [`provisioning`](provisioning) | 확정 계획 게이트, taxonomy→config 아티팩트, 적용·롤백 플레이북 생성, before 캡처·롤백 레코드 | [프로비저닝 설계](../provisioning/design.md) |

## 공유

| 패키지 | 하는 일 |
|---|---|
| [**`kernel`**](kernel/README.md) | 단계를 가로지르는 **결정론적 판정 규칙**과 게이트: 스코프, 등급, 시그니처 레지스트리, 서명, 머신 지문, 완전성 맵 조립. **무엇이 kernel에 속하는지를 가르는 기준**이 그 README에 있다 |

## 방향 규칙

**인벤토리가 허브다.** 디스커버리와 프로비저닝이 인벤토리를 참조하고, 인벤토리는 두 단계 어느 쪽도 참조하지 않는다. 공통(`gen`·`kernel`·`org`)은 어느 단계도 참조하지 않는다. 순환을 막고, 단계를 떼어내도 공통이 따라오지 않게 하려는 것이다.

```
common      gen · pkg/kernel · pkg/org
   ↑
inventory   pkg/inventory(+history · normalize · resultio · ingest · declaration) · inventory/cmd
   ↑                  ↑
discovery         provisioning
 (cmd · collectors · procs)
```

- **collector는 공통과 `procs`만 쓴다.** 노드로 나가는 바이너리에 다른 단계 코드가 딸려 가지 않게 하려는 것이다. `procs`도 공통과 자기 자신만 참조한다(`collector → procs → 인벤토리`라는 간접 경로를 막는다).
- **단계 둘 이상을 함께 다루는 테스트**는 [`test/crossstage`](../test/crossstage)에 둔다. 대상 코드 옆에 두면 그 패키지가 다른 단계를 참조하게 된다.
- 이 방향은 **`make check-deps`가 잰다.** 규칙은 [`tools/checkdeps/rules.tsv`](../tools/checkdeps/rules.tsv)의 허용 목록이고, 분류표에 없는 경로도 실패로 센다. **재지 못하는 것**도 있다: 배포 명령(`pqcota-nodescan` 등)이 링크하는 의존 폐포(로컬 뷰가 인벤토리와 Postgres 드라이버를 끌어온다)는 재지 않는다.

파생값(`evidence_strength`·`pqc_readiness`·등급)은 **collector가 아니라 코어가** 채운다. 규칙이 한 곳에 있어야 원본에서 재계산으로 재현된다(§1.2). collector 쪽 경계는 [`discovery/collectors`](../discovery/collectors) 각 README 참조.
