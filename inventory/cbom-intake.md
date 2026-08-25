# 위임 수신 설계 (Delegated Intake: CBOM 임포트)

**문서 성격**: discovery에서 **직접 만들지 않고 받기만 하는** 기능을 collector(직접 관측)와 분리해 다룬다. 소스·빌드 아티팩트 스캔은 기존 도구(CBOMkit 등)에 위임하고, pqcota는 그 표준 산출물(CycloneDX)을 **수신·검증·정규화**만 한다.
**기준**: [디스커버리 설계](../discovery/design.md) §2.3·SV-2·SD-7 · 규정서 §2.
**구현**: `pkg/inventory/ingest`(수신 어댑터: 검증+임포트) · [`inventory/cmd/pqcota-cbom-ingest`](cmd/README.md)(수신·검증·적재 종단 CLI).

> **왜 collector와 분리하나**. openssl·jvm·network collector는 **직접 만드는 런타임 관측(핵심 IP)**이다. CBOM 임포트는 정반대다. 스캔은 남이 하고 pqcota는 결과를 받는다. 코드 형태(계약 뒤 얇은 어댑터)·유지보수·라이선스(GPL 격리)·서비스 경계가 collector와 모두 달라 별도 문서로 둔다.

---

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

## 0. 핵심 경계: CI 위임 / 런타임 자체제공 (§1.1)

| 레인 | 무엇이 보이나 | 누가 스캔 | pqcota가 하는 일 |
|---|---|---|---|
| **CI 가시 영역** | 소스·의존성 그래프·빌드 아티팩트(JAR/WAR) | **사용자 CI + CBOMkit** (위임) | **수신·처리만** (import 어댑터 + 정규화). 실행·오케스트레이션 안 함 |
| **런타임 가시 영역** | 로드된 lib·`/proc`·살아있는 provider·배포 바이너리 | **pqcota collector** | **직접 제공 (핵심 IP)** → [collectors/](../discovery/collectors) |

**근거**: 소스가 있으면 CI가 이미 스캔하거나 해야 할 일이라, pqcota가 CBOMkit을 오케스트레이션할 이유가 없다. 대체 불가 차별점은 전부 런타임 레인에 있다(§2.2 "JVM 공백", §2.3 "소스 부재=지배 케이스"). 소스 스캔은 이미 상품화된 영역이라 위임한다.

## 1. 무엇을 하나. 받는 입구

- **수신**: 사용자가 CI/로컬에서 만든 표준 CycloneDX CBOM 파일을 받는다.
- **검증**: 서명(옵션)·구조·앵커를 `IngestCBOM`(=`ImportCBOM`) 내부에서 **강제**. 부적합이면 **거부**(저장 안 함, TV-CBOM-2).
- **정규화**: Envelope 부착(`detection_method=source/artifact`) → collector 산출과 **같은 파이프라인·같은 인벤토리**로 수렴.

**파일만 오간다.** CBOMkit은 사용자의 CI가 돌리고 pqcota는 그 산출 파일을 읽을 뿐이다. 호출·번들·링크가 없어 GPL이 전염되지 않는다(§4).

## 2. provenance: 전달 방식 ≠ 증거 의미론

"임포트됐다"는 **사실**이 provenance를 정하지 않는다. provenance는 Envelope의 `detection_method`로 결정된다.
- 같은 "파일 임포트"라도 **CBOM(관측 레인)** 과 **CMDB 선언(선언 레인 → 인벤토리 `pkg/inventory/declaration`)** 은 다른 레인이다.
- CBOM 수신 = **관측 레인**, `evidence_strength`는 `confirmed`–`inferred-high`(소스/아티팩트 기반).

## 3. 메커니즘: 파일 기반 intake (SV-2·SD-7 공용)

미리 생성된 CycloneDX를 **서명검증 후 관측 레인으로 수신**하는 파일 기반 intake. 두 시나리오가 공용한다:
- **SV-2** (사용자 CI 산출): CI에서 CBOMkit(hyperion/theia) 실행 → CycloneDX 제출.
- **SD-7** (에어갭 오프라인 산출): 격리망에서 오프라인 산출물 반입.

같은 어댑터(`pkg/inventory/ingest`)가 처리한다. "CBOMkit을 pqcota가 돌린다"가 아니라 "결과를 받는다".

## 4. 라이선스 경계 (GPL 격리)

CBOMkit은 GPL 계열이다. pqcota는 **링크·번들하지 않고 파일(CycloneDX)만 교환**하므로 전염되지 않는다. CBOMkit을 직접 실행해야 한다면 **프로세스를 격리**해야 한다. 코어(Apache-2.0)엔 넣지 않는다. → [라이선스 정리](../docs/licensing.md)

## 5. 역할 분담 (SV-2 요약)

| 역할 | 하는 일 |
|---|---|
| **사용자** | 자기 CI/로컬에서 CBOMkit 실행 → CycloneDX 제출 |
| **pqcota** | 수신·검증·적재(`pqcota-cbom-ingest`, 검증 내장)·Envelope·정규화. 오케스트레이션 안 함 |


---

## 시나리오 SV-2: 소스/아티팩트가 있는 앱

소스·아티팩트는 **pqcota가 스캔하지 않고** CBOMkit 등에 위임, 결과(표준 CycloneDX)만 받는다(§1.1 CI 위임). 상세 설계·라이선스·provenance 경계: **[위임수신_설계(이 문서)**.

- **상황**: 소스 리포·빌드 아티팩트(JAR/WAR·의존성 매니페스트)가 남아 있음. CI에서 CBOMkit 실행 가능.
- **[사용자]** 자기 **CI/로컬에서 CBOMkit(hyperion/theia) 실행** → CycloneDX 산출·제출. (SD-3의 반대 케이스)
- **[pqcota]** **오케스트레이션 안 함.** 제출된 CBOM을 `pqcota-cbom-ingest`로 수신 → (내부) 서명·구조·앵커 검증 → Envelope 부착(`detection_method=source/artifact`) → 정규화·적재. **관측 레인**, `confirmed`–`inferred-high`.
- **결과**: 관측 계열(소스/아티팩트) 확보. *collector를 만들지 않는 대표 케이스(§1.1 위임).*

> **메커니즘: CBOM 임포트 어댑터**: SV-2(사용자 CI 산출)와 SD-7(에어갭 오프라인 산출)이 공용하는 파일 기반 intake. 시나리오 아님. 미리 생성된 CycloneDX를 서명검증 후 관측 레인으로 수신. "CBOMkit을 pqcota가 돌린다"가 아니라 "결과를 받는다".

---

---

## 어댑터 명세

**책임**: 미리 생성된 CycloneDX(사용자 CI의 CBOMkit, 에어갭 오프라인 산출)를 **수신·처리**. **CBOMkit을 실행하지 않는다**(§1.1).

```
입력   : CycloneDX 파일(업로드/경로)
검증   : 스키마 적합성 + 스펙버전(1.6/1.7) 확인  →  부적합 거부(§5 핸드오프)
바인딩 : 사용자가 target_node_id(스코프 마스터) 지정 필수. 없으면 등재요청(SD-5)으로
Envelope   : detection_method=source|artifact 부착, collected_at, 서명 있으면 검증
반환   : CollectionResult(관측 레인) → 정규화 파이프라인 공용 진입
```
- **`pqcota-cbom-ingest <cbom.json> <node-id>`**: CBOM **수신·검증·적재** 종단 CLI. 검증(서명·구조·앵커)은 `ImportCBOM` 내부에서 강제: 부적합은 거부(저장 안 함). 런북(문서)을 실행 가능하게 하는 진입점.
- SD-7(에어갭)은 이 어댑터 + T1 오프라인 번들을 조합.
