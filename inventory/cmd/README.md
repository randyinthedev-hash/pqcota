# inventory/cmd/: 인벤토리 진입점

인벤토리 단계의 CLI(Go 바이너리). collector가 낸 관측을 **중앙에 적재**하고, 쌓인 것을 **읽기전용**으로 조회한다. 다섯 범주로 나눠 정리한다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

## ① 적재: 회수된 결과를 히스토리에 쌓음

collector가 낸 `CollectionResult` JSON들을 **`pqcota-ingest`가 한 디렉터리에서 읽어** 스코프 게이트 → 정규화 → append-only 히스토리에 적재한다. 아래 조회 커맨드가 읽는 것을 여기서 만든다.

그 디렉터리에 파일을 모으는 것은 **사용자**다. 데모에선 Ansible이 각 노드에서 [collector](../../discovery/cmd/README.md)를 돌려 컨트롤러로 회수한다.

### `pqcota-ingest`

```
pqcota-ingest [-scope-assets <csv>] <results-dir> [scope-master-file]
```

| 인자·옵션 | 하는 일 |
|---|---|
| `<results-dir>` | `*.json`(단일 객체)·`*.jsonl`(한 줄=한 결과)을 모두 읽는다. jvm attach 경로가 노드당 JVM 여럿을 JSONL로 낸다 |
| `[scope-master-file]` | 노드 등재 게이트. 안 주면 게이트를 생략한다 |
| `-scope-assets <csv>` | 자산 스코프: 등재된 노드 안에서 계속 관리할 자산만 남긴다(아래) |

| 환경변수 | 하는 일 |
|---|---|
| `PQCOTA_DSN` | Postgres 접속 문자열([형식](../../discovery/cmd/README.md#pqcota-hosts)). 없으면 인메모리 요약만: 영속되지 않는다 |
| `PQCOTA_VERIFY_KEY` | 공개키(콤마 구분). 있으면 결과 서명을 검증하고 불일치는 거부한다. 키쌍은 [`pqcota-keygen`](../../discovery/cmd/README.md#pqcota-keygen)이 만들고, 짝이 되는 개인키는 **노드에서 collector가** 쓴다 |
| `PQCOTA_REQUIRE_SIGNATURE` | `1`이면 검증할 키가 없을 때 **적재를 시작하지 않는다.** 없으면 검증을 건너뛰되 그 건수를 "서명 미확인"으로 따로 보고한다. 통과와 같은 자리에 두지 않는다 |
| `PQCOTA_ORG` | 이 적재가 속할 조직(소문자·숫자·하이픈 2–64자). 없으면 `default`에 묶인다. **저장소를 여는 모든 명령이 같은 값을 봐야 한다**. 읽는 쪽과 쓰는 쪽이 다르면 데이터가 있는데 안 보인다 |
| `PQCOTA_REQUIRE_ORG` | `1`이면 조직 없이 저장소를 열 수 없다. `default`도 이름으로 쓸 수 없다(예약). 여러 조직이 한 저장소를 쓰는 배포용: 섞인 뒤에는 되돌릴 수 없어 **여는 자리에서** 막는다 |
| `PQCOTA_AUTO_DDL` | `0`이면 스키마를 만들지 않는다. 없으면 에러로 끊는다. 가리키는 곳이 어긋났을 때 빈 테이블을 새로 만들어 거기 쓰는 것을 막는다 |

> **`PQCOTA_DSN`을 줬는데 저장소를 열지 못하면 멈춘다.** v0.1.x는 인메모리로 대체하고 계속했는데,
> 그러면 화면에 성공이 찍히고 데이터는 프로세스와 함께 사라졌다. DSN을 준 것은 영속을 요구한 것이다.

미등재 노드의 결과는 버리지 않고 **등재요청**으로 남는다.

### `pqcota-declare-attribution`

```bash
pqcota-declare-attribution [--out <dir>] <attribution.csv>   # CSV: node_id,dst,app_key
pqcota-ingest <dir>                                          # 선언 레인으로 적재
```

네트워크 관측은 **캡처하는 순간 소켓이 살아 있어야** 앱을 알아낸다. 짧게 붙었다 끊기는
연결(배치·헬스체크·cron·SSH)은 그 구간을 벗어나므로 `app_key`가 빈다. 조회 화면에서 `@?`로 보이는
자리다. 그 자리를 운영자가 메우는 길이다.

| | |
|---|---|
| `node_id` | 관측 호스트(엣지의 src) |
| `dst` | 상대. 엣지에 찍힌 주소 그대로: `pqcota-inventory -snapshot`에서 보인다. **포트를 따로 적지 않는다**: 계약이 `dst_addr`를 `"ip:port"`로 정해 이미 들어 있고, 두 곳에 적으면 한쪽만 틀렸을 때 조용히 안 맞는다 |
| `app_key` | 이 엣지를 연 앱 |

**그대로 돌려볼 수 있는 샘플**은 [examples/inventory](../../examples/inventory/README.md#pqcota-declare-attribution--관측이-못-짚은-엣지의-앱을-사람이-적는다)에 있다([attribution.csv](../../examples/inventory/attribution.csv)).

> **관측을 고치지 않는다.** 이 선언은 자기 레인(`detection_method=UNSPECIFIED`)으로 쌓이고,
> 합치는 일은 **조회할 때 화면에서** 일어난다. 관측이 이미 짚은 앱은 덮지 않고 **빈 자리만**
> 메우며, 메운 것은 `@app(declared)`로 표시되고 몇 건인지도 함께 나온다.
>
> 저장을 가르는 이유는 둘이다. 서명이 `app_key`를 덮으므로 고치면 collector가 서명한 것과
> 달라지고, 원본에서 다시 계산할 때 저장된 값과 달라진다.

### 자산 스코프(`-scope-assets`)

노드를 등재해도 그 안에서 관측되는 것 **전부가 관리 대상은 아니다**. 시스템 기본 라이브러리나 패키지가 딸려 넣은 런타임이 섞이면 인벤토리가 잡음에 묻힌다. 무엇을 계속 볼지는 **사용자가 선언**하고 도구는 집행한다.

```csv
action,runtime,lib,app_key,note
exclude,*,*,/usr/bin/python*,패키지 python 런타임 — 관리 대상 아님
exclude,openssl,libcrypto.so.*,*,이 계열은 전부 제외
include,openssl,libcrypto.so.3,/opt/apps/payment-gw,결제 게이트웨이만 예외
```

- 빈 칸과 `*`는 "모두"를 뜻한다. 패턴은 glob이다. 규칙이 없으면 **전부 관리 대상**이다(기본 포함).
- 판정: 기본 포함 → `exclude`로 빼고 → `include`로 되돌린다. **include가 exclude를 이기므로** "이 계열은 전부 빼되 이것만 예외"를 쓸 수 있다.
- 공유 `.so`는 쓰는 앱이 여럿이라, **하나만 맞아도** 규칙이 걸린다.
- **제외는 "없음"이 아니다**. 뺀 건수를 적재 요약과 인벤토리 뷰가 고지한다. 조용히 사라지면 인벤토리가 "그런 자산은 없다"고 거짓말한다.

> 근거·경계 상세: [인벤토리 설계 §14 자산 스코프](../design.md), 인수 기준: [테스트케이스 S](../testcases.md).

## ② CBOM 수신: 외부 도구가 낸 결과 임포트

collector가 **직접 관측**하는 런타임이라면, 소스·빌드 아티팩트는 **스캔하지 않고 위임**한다. 사용자 CI에서 CBOMkit이 낸 표준 CycloneDX를 **받아서** 검증·정규화·적재한다. CBOMkit을 pqcota가 실행하지 않는다. → [discovery/README ②](../../discovery/README.md)

### `pqcota-cbom-ingest`

```
pqcota-cbom-ingest <cbom.json | -> <target-node-id>
```

CycloneDX를 수신·검증·적재한다. 부적합은 거부하고 저장하지 않는다.

| 인자 | 하는 일 |
|---|---|
| `<cbom.json>` | 받을 CycloneDX 파일. `-`면 stdin(아래 CI 주입) |
| `<target-node-id>` | 그 CBOM을 어느 노드 자산으로 앵커할지 |

`env PQCOTA_DSN`이 있으면 Postgres에 영속하고, 없으면 인메모리 요약만 낸다.

> **거부는 판단이 아니라 결정론적 검증이다.** 거부 사유는 딱 둘: (1) **구조 부적합**(malformed JSON · CycloneDX 아님(`bomFormat`) · 미지원 `specVersion`), (2) **서명 검증 실패**(검증 키가 설정된 경우). 둘 다 기계적으로 판정된다. 반대로 `target_node_id` 바인딩이 없는 건 **거부가 아니라** 스코프 판정으로 라우팅되고, `pqcota:` 프로퍼티가 없는 자산은 **거부가 아니라** 강도 미상으로 파싱만 안 된다. "못 믿을 것은 버리되, 안 본 것을 없다고는 안 한다".

`ImportCBOM`이 서명(옵션)·구조·앵커를 검증한다. 통과분은 관측 레인(`detection_method=source/artifact`)으로 위와 같은 히스토리에 수렴한다. 어댑터: `pkg/inventory/ingest`.

> **CI 파이프라인 주입**: 중간 파일 없이 CBOMkit 출력을 바로 흘려보낼 수 있다(GitHub Actions·GitLab CI 등):
> ```bash
> cbomkit scan ./repo | pqcota-cbom-ingest - cmdb://payment-gw
> ```
> CI는 자기가 무엇을 빌드하는지 아니까 `target-node-id`를 여기서 못박는다(앵커 없으면 스코프 판정으로 라우팅, [discovery/README](../../discovery/README.md) 참고).

## ③ 조회: 쌓인 것을 읽기전용으로 본다

핵심 구분은 **파일 취합(휘발성·로컬) vs 중앙 저장소 조회(영속·별도 프로세스)**다.

### `pqcota-discover-view`

```
pqcota-discover-view <results-dir> [nodes.json] [topology-out.dot]
```

| 인자 | 하는 일 |
|---|---|
| `<results-dir>` | 회수된 `CollectionResult` JSON들을 그 자리에서 취합 |
| `[nodes.json]` | 관측 IP를 노드명으로 잇기(`10.0.0.9` → `node-c`) |
| `[topology-out.dot]` | 통신 토폴로지를 DOT로 쓴다(색=등급) |

발견 자산(OpenSSL·JCA)과 관측 통신 엣지 등급을 낸다. **저장소를 쓰지 않는다**. 휘발성 뷰다.

### `pqcota-inventory`

```
pqcota-inventory [-history <node>] [-snapshot <id>] [-diff <과거id>,<최신id>]
```

인자 없이 돌리면 **전 노드 최신 스냅샷 + 등급 집계**를 낸다. `▸`머신 헤더(엔드포인트·프로필)와 `@`앱 표시(공유 `.so`는 다중)이 붙는다. `env PQCOTA_DSN`가 있어야 한다(Postgres의 append-only 히스토리 + 머신 메타데이터를 읽는다).

| 플래그 | 하는 일 |
|---|---|
| `-history <node>` | 그 노드의 스냅샷을 **오래된 것부터** 나열: seq, 적재 시각, ruleset, findings·edges 수, 갭 |
| `-snapshot <id>` | 스냅샷 **단건 상세**: 자산 표 + 그 스냅샷의 **관측 엣지**(누적 뷰는 합계만 내므로 여기서만 펼침) |
| `-diff <과거id>,<최신id>` | 두 스냅샷 **변화**: `added`·`removed`·`changed` |

`-diff`의 **방향 규약: 첫 인자=과거, 둘째=최신**(`added`=둘째에만 있는 것, `removed`=첫째에만 있던 것). 시간 역순으로 주면 방향이 뒤집혀 읽히므로 **역순이면 경고한다.** finding id가 (node, name, runtime, fork) 해시라 **버전이 바뀌어도 같은 자산의 `changed`로** 잡힌다. ruleset이 다르면 파생값 차이가 재계산 결과일 수 있다고 경고한다.

**스냅샷은 변화 지점에만 쌓인다.** 같은 상태를 다시 관측하면 스냅샷을 새로 만들지 않고 **관측 기록**(가벼움)만 남으므로, `-history`의 `obs`·`observed` 열이 "그 상태를 몇 번·언제까지 재확인했는지"를 보여준다. 덕분에 무거운 저장은 **변화 횟수만큼만** 자라면서도 "매번 스캔했다"는 증거는 보존된다.

## ④ 보존 정책: 오래된 변화 지점 절단

### `pqcota-prune`

```
pqcota-prune [-older-than 90d] [-keep-last N] [-apply]
```

| 플래그 | 하는 일 |
|---|---|
| `-older-than <기간>` | 이보다 오래된 변화 지점을 절단(예: `90d`, `720h`) |
| `-keep-last <N>` | 노드별 최근 N개 변화 지점은 보존 |
| `-apply` | 실제로 삭제한다. **없으면 계획만 보인다**(기본 dry-run) |

두 축을 다 주면 **보수적**으로 판정한다(둘 다 버려도 될 때만). 정책을 하나도 안 주면 거부한다.

불변식 셋: **최신 불가침**(노드별 최신은 어떤 정책으로도 안 지운다. 인벤토리 뷰·프로비저닝 before 캡처의 근거), **수정 금지**(남은 스냅샷은 바이트 그대로), **절단 사실 기록**(`-history`가 `⌫` 줄로 고지. 없으면 이력의 구멍이 "관측 안 함"으로 읽힌다). 조회 커맨드와 **일부러 분리**했다. 읽기 도구가 파괴적 동작을 겸하면 실수 한 번이 이력을 지운다.

> 근거·판정 규칙 상세: [인벤토리 설계 §7 이력·보존](../design.md) (동등성 정의는 §7.3: 계약에 필드를 더할 때 함께 갱신해야 한다), 인수 기준: [테스트케이스 H·T](../testcases.md).

## ⑤ 메타데이터 · 선언 임포트

엔드포인트는 `discovery/cmd/pqcota-hosts --dsn`이 채우고, 프로필·선언은 아래 둘이 채운다. → [collector·접근 준비 커맨드 지도](../../discovery/cmd/README.md)

### `pqcota-profile`

```
pqcota-profile [--dsn <postgres>] <profiles.csv>
```

| 인자·옵션 | 하는 일 |
|---|---|
| `<profiles.csv>` | 머신 프로필(`display_name`·`environment`·`role`·`owner`·`location`·`labels`). 출처는 CMDB |
| `--dsn <postgres>` | 주면 인벤토리에 upsert. 없으면 파싱 결과만 보인다 |

식별과 분리된 **사람-대면 메타데이터**다. 뷰의 `▸`헤더를 채운다.

### `pqcota-declare`

```
pqcota-declare [--out <dir>] <declaration.csv>
```

| 인자·옵션 | 하는 일 |
|---|---|
| `<declaration.csv>` | 사용자 선언 인벤토리(`node_id`,`crypto_runtime`,`component`) |
| `--out <dir>` | `CollectionResult` JSON 출력 디렉터리(기본 `declared-results`) |

**관측이 아니다**. `detection_method`가 비어 나간다. 낸 JSON을 `pqcota-ingest <dir>`(①)로 적재하면 대조의 기준선이 된다.

## 언제 무엇을 쓰나
- 회수한 결과 파일을 **한 번 취합해 그 자리에서 보기** → **`pqcota-discover-view`**(저장소 불필요·휘발성).
- 여러 노드가 시간에 걸쳐 쌓은 **누적 인벤토리를 중앙에서 조회**(엔드포인트·프로필·앱 표시 포함) → **`pqcota-inventory`**(Postgres).

> 로직은 `pkg/inventory/`(적재 어댑터 `ingest`·뷰 렌더·`RenderStore`·머신 메타데이터 `MetaStore`·hosts 파서)와 `pkg/discovery/`(정규화·히스토리 스토어)에 있고, 이 커맨드들은 그걸 조립하는 얇은 진입점이다.
