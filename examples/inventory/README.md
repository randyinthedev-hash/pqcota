# examples/inventory — 읽기전용 인벤토리 뷰

```bash
./examples/inventory/run.sh
```

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

## 무슨 일이 일어나나

### `pqcota-discover-view` — 결과 취합 → 자산 + 앱 + 등급

> 표본에 **Windows CNG 노드(`node-d`)** 가 하나 들어 있다. 리눅스 collector 셋과 나란히 놓이므로,
> 런타임이 늘어도 뷰가 같은 자리에 그린다는 것을 여기서 볼 수 있다 — 그 노드 줄에는 provider 수와
> 알고리즘 수, 그리고 PQC 요약(`네이티브(서명만 — KEM 미관측)`)이 함께 나온다.
[`../data/results`](../data)의 `CollectionResult` JSON들을 모아 **발견 자산**과 **관측 통신 엣지 등급**를 보여준다(파일 취합·휘발성, 저장소 불필요). 대조·판정(reconcile)은 하지 않는다(§2.1).

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
- 토폴로지 **DOT**도 함께 생성(색=등급) — `dot -Tsvg`로 SVG 렌더 가능.

> 앱 표시(`@app`)·엔드포인트/프로필 헤더는 **중앙 영속 뷰**(`pqcota-inventory`, Postgres)에서 함께 표시된다. 이 파일-취합 뷰는 자산·엣지 중심이다.

## 중앙 영속 조회 (`pqcota-inventory`)
파일 취합이 아니라 **여러 노드가 시간에 걸쳐 쌓은 누적 인벤토리**를 조회하려면 Postgres가 필요하다:
```bash
# 먼저 같은 DSN에 적재(pqcota-ingest)한 뒤:
PQCOTA_DSN=postgres://… go run ./inventory/cmd/pqcota-inventory
```
→ ▸엔드포인트·프로필 헤더 + `@`앱 표시(공유 .so는 다중)까지. 종단 흐름은 [demo/](../../demo) 참고. 커맨드 지도: [inventory/cmd/README](../../inventory/cmd/README.md).

### `pqcota-cbom-ingest` — 외부 도구가 낸 CBOM 받기
collector가 관측하지 않는 소스·빌드 아티팩트는, 사용자 CI에서 **CBOMkit** 등이 낸 표준 CycloneDX를 **받아서** 적재한다. [`sample-cbom.json`](sample-cbom.json)을 넣으면:
```
✓ 수용: node=node-b · detection_method=source/artifact · 자산 1개 · 저장소 인메모리(요약만)
```
- **검증(서명·구조·앵커)은 커맨드 내부에서 강제** — 부적합 CBOM은 거부(저장 안 함). 별도 프리플라이트 불필요.
- 관측 레인(`detection_method=source/artifact`)으로 붙어 **collector 관측과 같은 인벤토리로 수렴**한다(Postgres 영속 시).

> **샘플 형태 주의**: 현재 정규화는 CBOM의 **`pqcota:` 프로퍼티**를 읽는다([`sample-cbom.json`](sample-cbom.json)이 그 형태 — JCA/BouncyCastle을 소스에서 발견한 셈). CBOMkit 표준 출력(`cryptoProperties`)을 pqcota 스키마로 매핑하는 것은 import 어댑터의 **확장 지점**이다 — 지금은 pqcota-매핑된 CBOM만 자산으로 파싱된다. 상세: [위임 수신 설계](../../inventory/cbom-intake.md).
