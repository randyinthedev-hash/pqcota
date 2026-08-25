# 커널 테스트케이스 명세

커널(`pkg/kernel/` + 정규화 파이프라인)은 **단계를 가로지르는 결정론적 판정 규칙**이다. 어느 단계의 관측이든 이 규칙을 거쳐 값이 정해지므로, 단계별 명세 어디에도 자리가 없다. 그래서 따로 세운다.

**커널이 하는 일은 파생이다.** collector는 본 것을 그대로 내고, 등급·강도·조치·앱은 여기서 계산된다(§1.2: 규칙이 한 곳에 있어야 재계산이 가능하다). 그래서 여기가 틀리면 관측이 옳아도 결론이 틀린다.

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

---

## 0. 실행 환경

**전부 unit이다**. 순수 함수와 인메모리 픽스처만 쓴다. 실물 호스트도, 외부 저장소도 필요 없다.

케이스 번호는 **`TK`(커널) - 무엇을 보나 - 순번**이다. `TK-EVIDENCE`(증거 강도) · `TK-PIPELINE`(정규화) · `TK-RAW`(원본 보존) · `TK-POSTURE`(등급·권고) · `TK-ALGO`(알고리즘 레지스트리) · `TK-REMEDIATION`(조치 taxonomy) · `TK-ATTRIBUTION`(어느 앱 것인가) · `TK-MACHINE`(머신 식별). 번호는 그것을 검증하는 **테스트 파일로 이어진다**.

---

## 1. 테스트케이스

### TK-EVIDENCE. 증거 강도: 모든 관측이 이 판정을 거친다

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-EVIDENCE-1](../pkg/discovery/normalize/evidence_test.go) | `TestEvidenceStrength`: 계약(`common.proto`)의 `detection_method` 여섯 값 | `runtime_introspection` · `source` · `dynamic_trace`→`CONFIRMED`, `artifact`→`INFERRED_HIGH`, `symbol_analysis`→`INFERRED_LOW`, 미지정→`UNSPECIFIED` | 규정서 §2.3 표를 코드로 고정한다. 지금 그 값을 내는 곳이 없어도 답이 정해져 있어야, 생산자가 생겼을 때 그 자리에서 지어내거나 `default`로 흘러 조용히 `UNSPECIFIED`가 되지 않는다 |
| [TK-EVIDENCE-2](../pkg/discovery/normalize/finding_test.go) | `TestParseDetectionMethod`: 자산 프로퍼티 / 수집 Envelope 두 자리, 복합·미지 어휘 포함 | 프로퍼티가 Envelope를 이기고(더 약해도), 복합이면 가장 강한 것(적힌 순서 무관), 없거나 모르면 Envelope로 폴백 | 강도를 파생하기 전에 방법이 정해져야 한다. 모르는 어휘를 그럴듯한 값으로 옮기지 않는다(§2.5) |
| [TK-EVIDENCE-3](../pkg/discovery/normalize/finding_test.go) | `TestDetectionMethodVocabularyDoesNotOverlap`: 어휘 다섯 개의 상호 포함 | 어느 것도 다른 것을 품지 않는다 | 어휘를 부분 문자열로 찾으므로, 한 어휘가 다른 어휘를 품으면 짧은 쪽이 영영 안 잡힌다. 어휘를 늘릴 때 걸리라고 둔다 |

### TK-PIPELINE. 정규화: CycloneDX에서 Finding으로

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-PIPELINE-1](../pkg/discovery/normalize/pipeline_test.go) | `TestDeriveFindings` · `_MultiApp` · `_JCA`: openssl·JCA collector 산출물 | 런타임 축(`openssl.fork` · `binding_mode` / `provider_set`)이 채워지고, `pqcota:app_keys` CSV가 **정렬된 복수**로 옮겨진다. 강도·스냅샷·ruleset이 부착된다 | 표준 CycloneDX 본문에서 파생 뷰를 만드는 유일한 자리다. 여기서 축을 잃으면 조치 계획이 무엇을 바꿔야 하는지 모른다 |
| [TK-PIPELINE-2](../pkg/discovery/normalize/pipeline_test.go) | `TestNormalizePipeline`: 결과 여럿 → `Normalize` 종단, 같은 입력으로 두 번 | 스냅샷에 finding·엣지·완전성이 모이고, **같은 입력 + 같은 ruleset이면 같은 finding**이 나온다 | 파생은 재계산 가능해야 한다(§1.2). 흔들리면 이력의 "변화"가 실제 변화인지 계산 차이인지 구분할 수 없다 |

### TK-RAW. 원본 보존: 재정규화의 전제

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-RAW-1](../pkg/discovery/normalize/rawcapture_test.go) | `TestRawFormatImpliesRawCapture`: openssl·jvm·network·선언 네 빌더의 산출물 | `raw_format`이 있으면 `raw_capture`도 반드시 있다 | 강화 규칙이 좋아지면 원본에서 다시 정규화한다고 계약이 적는다(§1.2·§2.4). 형식 이름만 있고 내용이 없으면 **재정규화할 것이 없다**. 빌더는 늘어나고 원본 채우기는 잊기 쉽다 |
| [TK-RAW-2](../pkg/discovery/normalize/rawcapture_test.go) | `TestRawCaptureDeterministic`: 같은 탐지 집합을 두 번 | 같은 바이트 | `raw_capture`는 서명이 덮는 값이라(§2.6) 흔들리면 검증이 깨진다 |

### TK-POSTURE. 등급·권고: 관측을 판정으로

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-POSTURE-1](../pkg/kernel/posture/posture_test.go) | `TestClassify`: 협상 그룹(하이브리드·순수 PQC·고전·빈 값·미지) | 하이브리드/순수 PQC는 🟢, 고전은 🔴, **빈 값과 미지 그룹은 ⚪** | 관측하지 못한 것을 고전으로 단정하지 않는다. 모르는 새 그룹도 마찬가지다(§2.5) |
| [TK-POSTURE-2](../pkg/kernel/posture/posture_test.go) | `TestGrade`(+`GradeLabel`): 같은 PQC라도 표준·초안·실험 | `X25519MLKEM768`=표준, `Kyber768Draft00`=초안, `sntrup761`=실험. 고전·불명은 등급 없음 | PQC냐 아니냐만으로는 부족하다. 초안·실험 그룹은 나중에 다시 바꿔야 한다 |
| [TK-POSTURE-3](../pkg/kernel/posture/posture_test.go) | `TestRecommend`: 성숙도 × 규제 자산 여부 | 표준=조치 없음, 초안=상향, 실험=교체, 고전=마이그레이션, 미관측=관측 먼저 | 등급이 조치로 이어지는 분기다. 여기가 틀리면 멀쩡한 자산을 건드리거나 위험한 자산을 넘긴다 |
| [TK-POSTURE-4](../pkg/kernel/posture/posture_test.go) | `TestSymbol`: 등급 열거값 | 🟢 / 🔴 / ⚪ 표시 | 사람이 보는 유일한 요약이라, 기호가 어긋나면 뷰 전체가 잘못 읽힌다 |

### TK-ALGO. 알고리즘 레지스트리: 이름에서 성숙도로

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-ALGO-1](../pkg/kernel/registry/pqc_test.go) | `TestMatchPQC`: 하이브리드·서명·전신·OpenSSH·기타 이름 | 계열(ML-KEM · ML-DSA · SLH-DSA · Kyber · NTRU-Prime · Falcon · FrodoKEM)과 성숙도를 짚는다 | 이름은 벤더마다 다르게 쓰인다. 여기서 못 짚으면 등급도 조치도 서지 않는다 |
| [TK-ALGO-2](../pkg/kernel/registry/pqc_test.go) | `TestFIPSValidatable`: 최종 표준 vs 초안·실험 | **최종 표준만** true | 규제 자산을 FIPS 검증 provider로 라우팅하는 게이트다. 초안을 통과시키면 규제 위반이 된다 |
| [TK-ALGO-3](../pkg/kernel/registry/pqc_test.go) | `TestPQCKind`: 알고리즘 | KEM인지 서명인지 | 조치가 갈린다. KEM은 하이브리드 그룹 줄, 서명은 그 줄이 없다 |

### TK-REMEDIATION. 조치 taxonomy: 무엇을 목표로 바꾸나

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-REMEDIATION-1](../pkg/kernel/registry/remediation_test.go) | `TestRemediate`: 성숙도 × 규제 여부 | 조치 종류와 **우선순위**(표준 0 · 규제+표준 1 · 초안 2 · 실험 3 · 파훼 4) | 무엇부터 손댈지가 여기서 정해진다. 파훼된 것을 초안과 같은 급으로 두면 순서가 무너진다 |
| [TK-REMEDIATION-2](../pkg/kernel/registry/remediation_test.go) | `TestRemediateTarget`: KEM / 서명 자산 | KEM은 ML-KEM, 서명은 ML-DSA를 목표로 | 목표를 잘못 잡으면 생성물이 엉뚱한 알고리즘을 켠다 |

### TK-ATTRIBUTION. 어느 앱 것인가: 자산을 누가 쓰나

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-ATTRIBUTION-1](../pkg/discovery/procs/procs_test.go) | `TestMatch`: `systemd_unit` · `exe_path` · `cmdline_regex`, 그리고 둘을 함께 준 규칙 | 각각 매칭하고, 함께 주면 **둘 다** 만족해야 한다. 다른 유닛은 안 걸린다 | 앱을 잘못 짚으면 스코프 정책이 엉뚱한 자산을 빼고 조치가 엉뚱한 앱에 간다 |
| [TK-ATTRIBUTION-2](../pkg/discovery/procs/procs_test.go) | `TestResolve`: 가짜 `/proc` 트리(exe·cmdline) | 프로세스에서 앱 키를 찾아낸다 | 프로세스는 휘발이라 실시간으로만 풀 수 있다. 그것이 자산과 앱을 잇는 유일한 다리다 |

### TK-MACHINE. 머신 식별: 같은 머신을 같은 것으로

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TK-MACHINE-1](../pkg/kernel/machineid/machineid_test.go) | `TestSelfAssign`: 같은 지문을 두 번, 그리고 지문 소스 우선순위 | **결정론적**(같은 지문 → 같은 id)이고 우선순위를 지킨다 | id가 흔들리면 같은 머신이 매번 새 자산으로 쌓여 이력이 갈라진다 |

---

## 2. 이 문서가 다루지 않는 것

`pkg/kernel/` 안에 있어도 **단계 시나리오가 이미 주인인 것**은 그쪽에 있다. fork 시그니처 매처는 [TD-FORK-1](../discovery/testcases.md), provider 레지스트리는 TD-JVM-1, 서명은 TD-SIGN-1–3, 노드 스코프 게이트는 TD-SCOPE-1, 자산 스코프는 [TV-SCOPE-*](../inventory/testcases.md)다.

CLI 진입점(`*/cmd/*`)과 데모 생성기(`demo/topology/topogen`)의 테스트는 케이스 번호를 달지 않는다. 파생 규칙이 아니라 조립·도구라서다.

전체 지도는 [테스트 명세](test-map.md)에 있다.
