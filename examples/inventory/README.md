# examples/inventory: 읽기전용 인벤토리 뷰

```bash
./examples/inventory/run.sh
```

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

## 무슨 일이 일어나나

### `pqcota-discover-view`: 결과 취합 → 자산 + 앱 + 등급

> 표본에 **Windows CNG 노드(`node-d`)** 가 하나 들어 있다. 리눅스 collector 셋과 나란히 놓이므로,
> 런타임이 늘어도 뷰가 같은 자리에 그린다는 것을 여기서 볼 수 있다. 그 노드 줄에는 provider 수와
> 알고리즘 수, 그리고 PQC 요약(`네이티브(서명만: KEM 미관측)`)이 함께 나온다.
[`../data/results`](../data)의 `CollectionResult` JSON들을 모아 **발견 자산**과 **관측 통신 엣지 등급**을 보여준다(파일 취합·휘발성, 저장소 불필요). 대조·판정(reconcile)은 하지 않는다(§2.1).

기대 출력(요지):
```
──────── ① discovered assets (per node) ────────
  node-a
    • OpenSSL  libssl.so.3 3.0.13 (OpenSSL) [EVIDENCE_STRENGTH_CONFIRMED]
  node-b
    • JCA provider chain: SUN,SunJCE,BC [EVIDENCE_STRENGTH_CONFIRMED]

──────── ② observed edges + quantum-resistance grade ────────
  🟢 node-a      → node-b             TLS   X25519MLKEM768 [fips-standard]
  🔴 node-a      → node-c             TLS   x25519
  🟢 node-a      → node-b             SSH   sntrup761x25519-sha512@openssh.com [experimental]

  grade totals: 🟢 PQC 2 · 🔴 classical 1 · ⚪ unknown 0
```
- **등급**: 🟢 PQC/하이브리드 · 🔴 고전=양자취약 · ⚪ 불명. PQC 그룹엔 성숙도(`fips-standard`/`draft`/`experimental`/`broken`)를 함께 적는다.
- **IP→노드 잇기**: `nodes.json`으로 `10.0.0.9` → `node-c`(엣지의 `dstAddr`가 이름으로 표시됨).
- 토폴로지 **DOT**도 함께 생성(색=등급): `dot -Tsvg`로 SVG 렌더 가능.

> 앱 표시(`@app`)·엔드포인트/프로필 헤더는 **중앙 영속 뷰**(`pqcota-inventory`, Postgres)에서 함께 표시된다. 이 파일-취합 뷰는 자산·엣지 중심이다.

### `pqcota-declare-attribution`: 관측이 못 짚은 엣지의 앱을 사람이 적는다

네트워크 관측은 **캡처하는 순간 소켓이 살아 있어야** 그 연결을 연 앱을 알아낸다. 짧게 붙었다
끊기는 연결(배치·헬스체크·cron·SSH)은 그 구간을 벗어나므로 `app_key`가 빈 채로 남고, 조회 화면에서
`@?`로 보인다. **그건 "앱이 없다"가 아니라 "어느 앱인지 밝히지 못했다"이고**, 그 자리를 운영자가
메우는 길이 이 커맨드다.

입력은 [`attribution.csv`](attribution.csv) 한 장이다.

```csv
node_id,dst,app_key
node-a,10.0.0.9:443,nightly-sync.service
```

| 컬럼 | 무엇 |
|---|---|
| `node_id` | 관측 호스트, 즉 엣지의 src |
| `dst` | 상대. 엣지에 찍힌 주소 그대로 쓴다. 계약이 `dst_addr`를 `"ip:port"`로 정해 포트가 이미 들어 있으므로 **포트를 따로 적지 않는다** |
| `app_key` | 이 엣지를 연 앱 |

첫 줄의 첫 칸이 `node_id`면 헤더로 보고 건너뛴다. 세 값 중 하나라도 비면 그 줄 번호를 대고
**멈춘다**. 어느 엣지를 가리키는지 모르는 채로 앱을 짚으면 조치 대상이 바뀐다.

**`dst`는 `pqcota-inventory -snapshot` 화면의 값을 옮긴다.** 위 표본에서 그 엣지는 이 파일-취합
뷰에 `node-a → node-c`로 보이는데, 그건 `nodes.json`이 읽기 좋으라고 IP를 이름으로 바꿔 준
것이고 엣지에 실제로 실린 값은 `10.0.0.9:443`이다. 선언이 맞춰야 하는 것은 실린 값이다. 상대가
스코프 마스터의 노드로 이어져 주소가 비어 있으면 그때는 그 `node_id`를 적는다.

**키는 (관측 호스트, 상대) 둘뿐이다.** 프로토콜도 포트도 키에 없으므로, 같은 상대와의 엣지가
여럿이면(예: 같은 노드로 가는 TLS와 SSH) 한 줄이 그 전부를 채운다.

```bash
pqcota-declare-attribution --out ./declared-attr examples/inventory/attribution.csv
pqcota-ingest ./declared-attr
```

첫 줄이 내는 것은 **선언 레인의 `CollectionResult`**다(`detection_method` 없음 = UNSPECIFIED).
`run.sh`가 그 JSON을 그대로 찍으므로 무엇이 만들어지는지 눈으로 볼 수 있다.

> **관측을 고치지 않는다.** 선언은 자기 레인에 쌓이고, 관측 엣지는 그대로 남는다. 둘을 합치는
> 일은 **조회할 때 화면에서** 일어나며, 관측이 이미 짚은 앱은 덮지 않고 빈 자리만 메운다. 메운
> 것은 `@app(declared)`로 표시된다. 저장을 가르는 이유는 둘이다. 서명이 `app_key`를 덮으므로
> 고치면 collector가 서명한 것과 달라지고, 원본에서 다시 계산할 때 저장된 값과 달라진다.
>
> **합쳐진 화면은 `pqcota-inventory`(Postgres)에서만 보인다.** 얹는 일을 하는 오버레이가 선언
> 저장소를 읽으므로, 저장소가 없는 이 파일-취합 뷰(`pqcota-discover-view`)는 그 단계까지 가지
> 않는다. 종단으로 보려면 아래 중앙 영속 조회를 쓰거나 [demo/](../../demo)를 돌린다.

## 중앙 영속 조회 (`pqcota-inventory`)
파일 취합이 아니라 **여러 노드가 시간에 걸쳐 쌓은 누적 인벤토리**를 조회하려면 Postgres가 필요하다:
```bash
# 먼저 같은 DSN에 적재(pqcota-ingest)한 뒤:
PQCOTA_DSN=postgres://… go run ./inventory/cmd/pqcota-inventory
```
→ ▸엔드포인트·프로필 헤더 + `@`앱 표시(공유 .so는 다중)까지. 종단 흐름은 [demo/](../../demo) 참고. 커맨드 지도: [inventory/cmd/README](../../inventory/cmd/README.md).

### `pqcota-cbom-ingest`: 외부 도구가 낸 CBOM 받기
collector가 관측하지 않는 소스·빌드 아티팩트는, 사용자 CI에서 **CBOMkit** 등이 낸 표준 CycloneDX를 **받아서** 적재한다. [`sample-cbom.json`](sample-cbom.json)을 넣으면:
```
✓ 수용: node=node-b · detection_method=source/artifact · 자산 1개 · 저장소 인메모리(요약만)
```
- **검증(서명·구조·앵커)은 커맨드 내부에서 강제**: 부적합 CBOM은 거부(저장 안 함). 별도 프리플라이트 불필요.
- 관측 레인(`detection_method=source/artifact`)으로 붙어 **collector 관측과 같은 인벤토리로 수렴**한다(Postgres 영속 시).

> **샘플 형태 주의**: 현재 정규화는 CBOM의 **`pqcota:` 프로퍼티**를 읽는다([`sample-cbom.json`](sample-cbom.json)이 그 형태이고, JCA/BouncyCastle을 소스에서 발견한 셈이다). CBOMkit 표준 출력(`cryptoProperties`)을 pqcota 스키마로 매핑하는 것은 import 어댑터의 **확장 지점**이다. 지금은 pqcota-매핑된 CBOM만 자산으로 파싱된다. 상세: [위임 수신 설계](../../inventory/cbom-intake.md).
