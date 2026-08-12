# 인벤토리 테스트케이스 명세 (Scenario-driven Acceptance Tests)

[인벤토리 설계](design.md)(규정서 §3)를 **검증 가능한 인수 기준**으로 옮긴 것이다. 구현은 이 테스트를 통과하는 것을 목표로 한다(TDD).

다루는 것은 **적재**(선언·CBOM 수신) · **이력 열람과 스냅샷 diff** · **보존 절단** · **자산 스코프** 넷이다.



> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

---

## 0. 실행 환경

**케이스는 대부분 unit이다** — 실물 없이 어디서나 돈다. 예외는 **TV-RETENTION-8과 TV-ORG-4** 둘로, `PQCOTA_TEST_DSN`이 있으면 실 Postgres로도 돌고 없으면 스킵한다(**스킵은 통과가 아니다**).

TV-ORG-4는 스킵되면 **격리를 확인하지 못한 것이다.** 인메모리 케이스(TV-ORG-3)는 저장소 객체가 애초에 다르므로 통과해도 격리를 증명하지 않는다 — 한 테이블을 공유하는 쪽에서만 잴 수 있다.

적재→조회→이력→절단→스코프 **종단**은 여기 케이스가 아니라 [데모 5/6](../demo/integration-verification.md)이 확인한다.

케이스 번호는 **`TV`(인벤토리) - 무엇을 보나 - 순번**이다 — `TV-IMPORT`(사용자 입력) · `TV-CBOM`(외부 수신) · `TV-INGEST`(적재 관문) · `TV-HISTORY` · `TV-RETENTION` · `TV-SCOPE` · `TV-ORG`(조직 격리) · `TV-REJECT`(받지 않은 사실). 번호는 그것을 검증하는 **테스트 파일로 이어진다**.

절 제목의 `SV-*`는 설계 문서가 매긴 **상황**(Scenario·inVentory) 번호이고([인벤토리 설계](design.md)의 선언 임포터, [위임 수신 설계](cbom-intake.md)), 표 안의 `TV-*`는 그 상황을 검증하는 **테스트** 번호다.

## 1. 테스트케이스

### SV-1. 사용자 입력 임포트 — 선언·접속 정보·프로필
| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-IMPORT-1](../pkg/inventory/declaration/import_test.go) | `TestImportCSV` — CMDB 선언 CSV 임포트 | **선언 레인** 라벨, detection_method=미지정(선언 ≠ 관측) | 선언을 관측과 섞지 않는다 — 강도가 unknown으로 남는 것이 정직하다 |
| [TV-IMPORT-2](../pkg/inventory/hosts_test.go) | `TestParseHostsAndSecretBoundary` · `NoNodeID` — 접속 정보 CSV | 계정·키는 인벤토리에 **남기지 않는다**. `node_id` 없는 헤더는 오류 | 접근 비밀이 인벤토리에 영속되지 않게 한다 |
| [TV-IMPORT-3](../pkg/inventory/profile_test.go) | `TestParseProfiles` — 머신 프로필 | 파싱된 속성이 머신 메타로 | 사용자가 적는 머신 정보의 형식을 고정한다 |
| [TV-IMPORT-4](../pkg/inventory/store_test.go) | `TestRenderStore` — 적재된 저장소 전체 | 누적 인벤토리 뷰 + posture 집계 | 스냅샷 여러 개가 하나의 현재 뷰로 접히는지 |

### SV-2. CBOM 수신 — 위임 경계
| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-CBOM-1](../pkg/inventory/ingest/cbom_test.go) | `TestImportCBOM` — 유효/깨진 CycloneDX, 앵커 없는 CBOM, 변조 | 유효분만 **관측 레인** 적재(봉투 detection_method=**artifact 고정**), 스키마 부적합은 거부, 앵커 없으면 스코프 판정으로 라우팅 | 검증을 통과하지 못한 것이 남으면 뒤 단계가 그것을 관측으로 취급한다. 그리고 외부 CBOM이 스코프를 우회하는 뒷문이 되지 않게 한다 |
| [TV-CBOM-2](../pkg/inventory/ingest/cbomingest_test.go) | `TestIngestCBOM` — 수신 → 정규화 → 적재 종단 | 통과분만 히스토리에 쌓임 | 어댑터를 통과한 것이 실제로 인벤토리까지 가는지 |
| [TV-CBOM-3](../pkg/inventory/ingest/identity_test.go) | `TestCheckIdentity` — 한 머신에 여러 이름, 한 이름에 여러 머신 | 지문으로 중복·충돌을 짚어낸다 | 사용자가 적는 node_id는 어긋나기 마련이라, 어긋난 채로 쌓이면 이력이 갈라진다 |

---

### TV-INGEST. 적재 관문 — 스코프·서명·정규화

| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-INGEST-1](../pkg/inventory/ingest/central_test.go) | `TestIngestResults` — 스코프 게이트 + Normalize + 히스토리 적재 + 엣지 부착 종단 | 통과분이 스냅샷으로, 엣지가 붙어서 | 관문 하나가 네 일을 순서대로 하는지 — 하나라도 빠지면 뒤가 조용히 빈다 |
| [TV-INGEST-2](../pkg/inventory/ingest/central_test.go) | `TestIngestSignatureReject` — 서명 검증 실패 | **거부**(§2.6) | 손댄 결과가 인벤토리에 들어오지 않게 한다 |
| [TV-INGEST-3](../pkg/inventory/ingest/central_test.go) | `TestIngestNoMaster` — 스코프 마스터가 없을 때(로컬·데모) | 게이트 생략, 전부 수용 | 스코프를 안 쓰는 사용자를 막지 않는다 |
| [TV-INGEST-4](../pkg/inventory/render_test.go) | `TestRenderEndToEnd` — collector 산출물(CycloneDX) → Normalize → 읽기전용 뷰 | 관측이 자산 표와 posture 집계까지 이어진다 | 적재와 뷰가 따로는 되는데 이어지지 않으면 사용자에게는 아무것도 안 보인다 |

---

### TV-HISTORY. 이력 열람·변화 diff (설계 §7 이력·보존)
| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-HISTORY-1](../pkg/inventory/history_view_test.go) | `TestRenderHistory` — 스냅샷 이력 조회(`-history`) | 변화 지점을 오래된 것부터 + 관측 횟수·관측 창. 스냅샷 id는 **자르지 않는다** | 스냅샷 id가 다음 명령의 입력이 된다 — 자르면 이력에서 상세·diff로 이어갈 수 없다 |
| [TV-HISTORY-2](../pkg/inventory/history_view_test.go) | `TestByID` · `TestRenderDetailShowsEdges` — 스냅샷 단건 조회(`-snapshot`) | 자산 표 + **그 스냅샷의 관측 엣지**. 없는 id는 `(nil, nil)` | 그 시점의 자산과 엣지를 함께 편다. 누적 뷰는 합계만 내므로 여기서만 볼 수 있다 |
| [TV-HISTORY-3](../pkg/inventory/history_view_test.go) | `TestRenderDiff` — 버전이 바뀐 자산으로 두 스냅샷 diff | finding id가 (node, name, runtime, fork) 해시라 유지 → **"변경"** 한 줄. 판정 어휘 없음 | 버전만 바뀐 자산이 추가+삭제로 흩어지면 무엇이 달라졌는지 읽을 수 없다. 그리고 관측 사실만 서술한다(§2.1) |
| [TV-HISTORY-4](../pkg/inventory/history_view_test.go) | `TestRenderDiffNoChange` — 같은 스냅샷끼리 diff | "변화 없음"을 **명시**(빈 출력 아님) | 변화가 없을 때 없다고 말한다. 빈 출력은 "안 봤다"와 구분되지 않는다 |
| [TV-HISTORY-5](../pkg/inventory/render_test.go) | `TestRenderDiffDirection` — 인자를 **시간 역순**으로 준 diff | 추가·사라짐이 뒤집혀 읽히므로 **역순 경고**를 낸다 | 인자 순서를 잘못 주면 결과가 정반대로 읽힌다 — 조용히 뒤집히지 않게 한다 |
| [TV-HISTORY-6](../pkg/inventory/render_test.go) | `TestRenderDiffWarnsOnRulesetChange` — `ruleset`이 다른/같은 두 스냅샷 diff | 다르면 재계산 경고, 같으면 안 뜸(§1.2) | 파생값 차이를 실제 변화로 읽으면 없던 변경을 쫓게 된다. 매번 뜨는 경고는 읽히지 않는다 |

### TV-RETENTION. 보존 정책 (설계 §7.2·§7.4)
| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-RETENTION-1](../pkg/inventory/history_view_test.go) | `TestRepeatObservationDoesNotDuplicateSnapshot` — 같은 내용을 다시 적재 | 스냅샷 **안 늘어남** + 관측 기록만 +1 | 같은 상태를 다시 봤을 때 스냅샷을 늘리지 않되 "매번 봤다"는 증거는 남긴다 |
| [TV-RETENTION-2](../pkg/inventory/history_view_test.go) | `TestVolatileEdgeFieldsAreNotChange` — 엣지 `observed_count`만 다른 재적재 | 휘발 필드라 **변화 아님**. 협상 그룹이 바뀌면 변화로 잡힘 | 휘발 필드를 변화로 세면 중복 억제가 무력해진다 |
| [TV-RETENTION-3](../pkg/discovery/history/prune_test.go) | `TestPruneRequiresPolicy` — `Policy{}`(축 없음)로 절단 | `ErrNoPolicy` 거부 | "최신만 남기고 전부 삭제"가 사고로 일어나지 않게 |
| [TV-RETENTION-4](../pkg/discovery/history/prune_test.go) | `TestPruneDryRun` — 기본 실행 | 계획만 산출, **아무것도 지우지 않음**, 절단 기록도 없음 | 지우는 일은 명시적으로만 일어나야 한다 |
| [TV-RETENTION-5](../pkg/discovery/history/prune_test.go) | `TestPruneNeverDeletesLatest` — 400일 지난 스냅샷이 노드별 최신 | **지우지 않음**(최신 불가침) | 노드별 최신은 인벤토리 뷰와 before 캡처의 근거다 |
| [TV-RETENTION-6](../pkg/discovery/history/prune_test.go) | `TestPruneConservativeWithBothAxes` — `older-than` + `keep-last` 동시 | **보수적** — 최근 N개 안이면 오래돼도 보존 | 두 축이 부딪히면 더 많이 남기는 쪽으로. 지운 것은 되돌릴 수 없다 |
| [TV-RETENTION-7](../pkg/discovery/history/prune_test.go) | `TestPruneRecordsEvent` — 절단 실행(`-apply`) | 스냅샷·관측 기록 삭제 + **절단 기록 영속** | 절단한 사실이 없으면 이력의 구멍이 "관측 안 함"과 구분되지 않는다 |
| [TV-RETENTION-8](../pkg/discovery/history/pg_test.go) | `TestPgStore` — Postgres 영속(`PQCOTA_TEST_DSN` 있을 때) | 2층 저장·조회가 인메모리와 같은 계약 | 저장소를 바꿔도 이력의 뜻이 달라지지 않는다 |
| [TV-ORG-1](../pkg/org/org_test.go) | `TestParseRejectsWhatCannotBeToldApart` · `TestEmptyIsNotAChoice` — `Acme`·`ACME`·빈 값·`acme_corp` 등 | 전부 거절. 소문자·숫자·하이픈 2–64자만 | 사람은 같게 읽고 기계는 다르게 읽는 이름이 있으면 한 조직이 둘로 갈린다 |
| [TV-ORG-2](../pkg/org/org_test.go) | `TestRequiredModeRefusesTheDefaultStore` · `TestDefaultIsReservedInRequiredMode` — 필수 모드에서 조직 없음·`default` | 둘 다 **여는 자리에서** 거절 | 데이터가 섞인 뒤에는 되돌릴 수 없다. `default`는 모양 규칙을 통과하므로 막지 않으면 배정된다 |
| [TV-ORG-3](../pkg/discovery/history/org_test.go) | `TestOrgsDoNotSeeEachOther` — 두 인메모리 저장소가 같은 `web-01` | `Nodes()`·`ByID()`·`Latest()`가 남의 것을 안 준다 | **모양만 확인한다.** 객체가 다르므로 통과해도 격리를 증명하지 않는다 — TV-ORG-4가 그 일을 한다 |
| [TV-ORG-4](../pkg/discovery/history/org_pg_test.go) | `TestPgOrgsShareATableAndStillDoNotSeeEachOther` — **한 테이블을 공유하는** 두 조직(`PQCOTA_TEST_DSN` 있을 때) | 서로를 못 보고, 자기 것은 보인다 | `web-01` 충돌은 예외가 아니라 기본값에 가깝다. 섞이면 한 노드의 이력으로 병합되어 되돌릴 수 없다 |
| [TV-REJECT-1](../pkg/inventory/ingest/rejection_test.go) | `TestRequiredModeRefusesToIngestWithoutAVerifier` — 서명 필수인데 검증기 없음 | 결과별 거절이 아니라 **적재가 시작되지 않는다** | 조용히 통과하는 경로가 열려 있는지가 문제이지 어떤 결과가 왔는지가 아니다 |
| [TV-REJECT-2](../pkg/inventory/ingest/rejection_test.go) | `TestUnverifiedIsNotTheSameAsPassed` — 검증기 없이 적재 | `Unverified` 1 · `Rejected` 0 · `Accepted` 1 | "검증했고 통과했다"와 "검증할 키가 없었다"를 한 숫자로 합치면 리포트가 실제보다 강한 말을 한다 |
| [TV-REJECT-3](../pkg/inventory/ingest/rejection_test.go) | `TestRejectionsOutliveTheProcess` — 미등재·앵커없음 결과 적재 | 저장소에 사유·collector·지문·시각이 남는다 | 남기지 않으면 "계속 거절당하고 있었다"와 "아무 일도 없었다"가 구분되지 않는다 |
| [TV-REJECT-4](../pkg/inventory/ingest/rejection_test.go) | `TestRejectionStoreIsOptional` — 남길 곳 없이 적재 | v0.1.x와 같은 결과 | 기록을 더한 것이 적재 자체를 바꾸면 안 된다 |

### TV-SCOPE. 자산 스코프 (설계 §14)
| 케이스 | Given → When | Then | 목적 |
|---|---|---|---|
| [TV-SCOPE-1](../pkg/kernel/scope/asset_test.go) | `TestNoPolicyKeepsEverything` — 정책 없음(nil) | 관측된 자산 **전부 관리 대상** | 정책을 안 쓰는 사용자를 막지 않는다 |
| [TV-SCOPE-2](../pkg/kernel/scope/asset_test.go) | `TestExcludeByAppKeyGlob` — `exclude`가 app_key glob에 매치 | 그 finding 제외 | 잡음을 앱 귀속으로 걸러낸다 — 없으면 인벤토리가 못 쓰게 된다 |
| [TV-SCOPE-3](../pkg/kernel/scope/asset_test.go) | `TestIncludeOverridesExclude` — `exclude` **뒤에** `include` | **뒤 규칙이 이긴다**(순서 기반) | "계열 전부 빼되 이것만 예외"를 쓸 수 있어야 한다 |
| [TV-SCOPE-4](../pkg/kernel/scope/asset_test.go) | `TestMultiAppAttribution` — 공유 `.so`(귀속 앱 다중) 중 하나만 매치 | 매치로 판정 | 공유 `.so`는 귀속 앱이 여럿이라 하나만 걸려도 규칙이 걸린다 |
| [TV-SCOPE-5](../pkg/kernel/scope/asset_test.go) | `TestBadAction` — `action`에 오타(`drop` 등) | 오류 | 조용히 무시하면 정책이 안 먹은 걸 모른다 |
| [TV-SCOPE-6](../pkg/kernel/scope/asset_test.go) | `TestSharedLibExcludeRescuedByTrailingInclude` — 공유 `.so`를 한 앱만 겨냥해 exclude | 그 `.so`를 함께 쓰는 **운영 앱까지 제외됨**. 운영 앱 `include`를 뒤에 두어 구제 | 겨냥한 앱만 빠질 것 같지만 영향 반경이 넓다는 것을 드러낸다 |
| [TV-SCOPE-7](../pkg/inventory/ingest/central_test.go) | `TestIngestReportsScopeExclusions` — 정책이 자산을 뺀 적재 | 적재 요약·스냅샷·인벤토리 뷰 **셋 다** 건수 고지, 제외한 자산은 남지 않음 | 스코프가 조용히 자산을 지우면 인벤토리가 거짓말을 한다(§2.6·§8.3) |

## 2. 구현 순서 (unit 먼저)

| # | 대상 | 케이스 | 레벨 |
|---|---|---|---|
| 1 | 선언·접속 정보·프로필 임포트 | TV-IMPORT-1–4 | unit |
| 2 | CBOM 수신·검증·동일성 | TV-CBOM-1–3 | unit |
| 3 | 적재 관문(스코프·서명·정규화·뷰 종단) | TV-INGEST-1–4 | unit |
| 4 | 이력 열람·변화 diff | TV-HISTORY-1–6 | unit |
| 5 | 보존 정책 절단 | TV-RETENTION-1–8 | unit + Postgres |
| 6 | 자산 스코프 | TV-SCOPE-1–7 | unit |

---
