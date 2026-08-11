한국어 · [English](RELEASE_NOTES.en.md)

# 릴리스 노트 — pqcota

버전별 **목표**와 **성과**를 기록한다. 버전이 올라갈 때마다 갱신하고, 최신 버전을 맨 위에 둔다.

> **§ 표기**: 별도 언급이 없으면 [규정서](docs/regulation.md)의 절 번호다.

---

## 로드맵 — 예정 릴리스 (계획)

확정된 것이 아니라 방향이다. 각 버전은 착수·완료 시 위 규칙대로 정식 섹션으로 승격한다. **Windows CNG
런타임은 단계적으로** 도입한다 — 한 번에 넣지 않는 이유와 남은 미결: [검토 중인 설계 §2.2](docs/under-review.md).

- **v0.2.0 (계획)** — **CNG 디스커버리**: Windows 콜렉터(`BCryptEnumProviders`·레지스트리 인트로스펙션)로
  `CngAxes`를 채워 자산이 인벤토리에 수렴. (스키마는 v0.1.0에서 이미 예약 — 이 릴리스는 "채우는 코드".)
  설계 검토: [검토 중인 설계 §2.2](docs/under-review.md).
- **v0.3.0 (계획)** — **CNG 프로비저닝**: **substrate 일반화 선행**(POSIX 파일 가정 탈피 — Windows는
  레지스트리/GPO라 `/opt/pqcota` 파일 스테이징·파일제거 롤백에 안 맞음) → `renderCNG`. 일반화는 이 구현과
  함께 한다(투기적 추상화 금지). seam을 어디에 그을지는 아직 정하지 않았다 —
  [검토 중인 설계 §2.2](docs/under-review.md).

- **v0.4.0 (계획)** — **엣지를 앱에 귀속**: 지금 `ObservedEdge`는 노드까지만 간다. 한 노드에서 두 앱이
  같은 lib을 쓰면 관측된 엣지가 어느 쪽 것인지 알 수 없다. 캡처 시점에 소켓 inode(`/proc/net/tcp`)와
  `/proc/*/fd`를 대조해 `app_key`를 채운다(계약은 순수 additive). 자동으로 못 잡는 것은 선언 레인으로
  받고, 관리 UI는 만들지 않는다 — 설계와 그 이유: [검토 중인 설계](docs/under-review.md).

- **provider 생태계 수용 (검토 중 · 버전 미정)** — 어떤 provider를 쓸지 고르고 그 파일을 구해 오는 것은 계획을 쓰는 사용자가 한다. 이 리포가 하는 일은 **그 provider를 활성화하는 설정 파일을 대신 만드는 것이다.** 그런데 지금은 `activate`+`module` 한 가지 모양만 만들 줄 안다 — provider마다 요구하는 설정이 달라서, OpenSSL 자체 `fips` 모듈(`fipsinstall`이 만들어 주는 파일을 끌어와야 한다)이나 pkcs11-provider(드라이버 경로 같은 항목이 더 필요하다)는 아직 만들지 못한다. 후보별로 무엇이 더 필요한지, 그리고 provider 관측·HSM 축은 [검토 중인 설계](docs/under-review.md)에서 다룬다.

- **릴리스 서명 (계획 · 버전 미정)** — **ed25519 서명과 `pqcota-verify-bundle`**. 번들 구성·서명·검증
  절차는 [collector 배포 설계](discovery/collector-deployment.md)에서 정해 뒀고, 그때까지 무결성 확인은
  `sha256sum -c`로 한다.

### 로드맵에 없는 것 — 안 만든다

방향이 아니라 **경계**다. 언젠가 오리라 기대하지 않도록 적는다.

| 안 만드는 것 | 대신 |
|---|---|
| **플릿 오케스트레이션** — drain · rolling · 헬스체크 게이트 | 표준 Ansible 플레이북을 내므로 사용자의 배포 도구가 한다 |
| **원격 실행 엔진** — 에이전트 상주·push 채널 | 생성물을 사용자가 자기 substrate로 실행한다 |
| **소스·아티팩트 CBOM 스캐너** | CI가 이미 소스를 갖고 있다. CBOMkit 등이 낸 CycloneDX를 **받는다** |
| **동적 추적**(eBPF·ltrace) | 침습적이라 하지 않는다. 회선에서 실제 협상을 관측하는 쪽을 택했다 |
| **판정·점수화** — "위험함" 같은 등급 매기기 | 관측 사실만 낸다. 무엇을 언제 바꿀지는 사용자가 정한다 |


---

## v0.1.1 — 계약을 받아 쓸 수 있게 (2026-08-11)

**고친 것** — 생성 코드(`gen/`)가 `.gitignore`에 있어, 계약을 소비하려는 쪽이 `go get`으로
받아도 타입이 없었다. `contracts/README.md`가 "소비자 엔진이 같은 어휘를 쓰도록"이라고 적어
두었는데 정작 그 어휘를 import할 수 없었다 — 첫 외부 소비가 생기며 드러났다.

- **`gen/` 커밋** — `go get` 만으로 `commonv1`·`discoveryv1`·`inventoryv1`·`provisioningv1`을 쓴다.
  손으로 고친 생성 코드는 CI의 generate 드리프트 검사가 끊는다(이제 그 검사가 실제로 의미를 갖는다).
- **buf 버전 고정**(CI, 1.69.0) — 생성 코드를 커밋했으므로, 도구 버전이 바뀌어 출력이 달라지면
  코드 변경 없이도 드리프트 검사가 실패한다.
- 소비 방법과 모듈 경로 우회를 [contracts/README](contracts/README.md#소비자가-쓰는-법)에 적었다.

계약(proto) 자체는 바뀌지 않았다 — `buf breaking` 기준선 그대로다.

---

## v0.1.0 — 첫 릴리스 (2026-08-11)

**목표** — 받아서 바로 쓸 수 있는 **3단계 종단**. arch별 정적 바이너리와 `SHA256SUMS`가 릴리스에 붙는다. **서명**(ed25519)은 이후 릴리스로 미뤘다(위 로드맵).

### 만든 것

- **계약 SSOT** — protobuf 4 네임스페이스(`common`·`discovery`·`inventory`·`provisioning`), `make generate`로 코드 생성.
- **Discovery** — 레퍼런스 collector 셋. **openssl·jvm은 `/proc` 선행 정찰**로 시작한다 — openssl은 로드된 lib을, jvm은 실행 중 JVM을 열거해 attach하고 다중 JVM을 앱 단위로 구별한다. **network는 `/proc`을 쓰지 않고** `AF_PACKET`으로 회선을 수동 관측한다. 그 위에 정규화 파이프라인(evidence·완전성 맵), 히스토리 적재·ed25519 서명(collector 주장 전부 서명), CBOM 위임 수신.
- **Inventory** — 중앙 적재·조회(Postgres), 머신 메타데이터(엔드포인트·프로필), 앱 귀속, **이력 열람·스냅샷 상세·변화 diff**(`-history`·`-snapshot`·`-diff`).
- **보존 정책** — 관측 기록/스냅샷 2층 분리(같은 상태 반복 관측은 저장을 늘리지 않되 "언제 봤나"는 보존) + 절단(`pqcota-prune`, 기본 dry-run·최신 불가침·절단 사실 기록).
- **자산 스코프** — 노드 등재 게이트(§1.4)를 자산 단위로 확장. 사용자가 선언한 관리 대상만 적재하고 제외 건수를 고지한다(`pqcota-ingest -scope-assets`).
- **Provisioning 생성** — 실행 게이트(finalized-only), taxonomy→config 아티팩트, 적용·롤백 Ansible 플레이북(**L1/L2/L3**), before 캡처·롤백 레코드.
- **L3 활성화·재시작** — 계획의 `activation` 훅(pre·activate·deactivate·restart)에 **사용자가 적은 명령**을 의미 순서(내리고 → 바꾸고 → 켜고 → 재시작)로 배치하고, 롤백은 정확한 역순으로 낸다. 활성화 방법은 환경마다 달라 **도구가 추측하지 않는다** — 빈 훅은 만들지 않고 무엇이 일어나지 않는지 고지한다.
- **CNG 스키마 예약** — `CRYPTO_RUNTIME_WIN_CNG` enum + `CngAxes`(oneof arm)를 계약에 추가(**미구현** — 채우는 콜렉터·정규화·프로비저닝은 v0.2.0/v0.3.0). 단계적 도입의 시작점이다. **계약에 자리가 난다는 것까지** 확인했다(순수 additive — 기존 필드 번호·타입 불변). 실물 Windows에서 무엇 하나 돌려본 바 없으므로 "CNG를 지원한다"는 뜻이 아니다.
- **검증** — 데모 6단계 종단(생성한 플레이북을 실제 노드에 **적용·활성화·되돌림**까지 실행해 확인), 단계별 examples, 테스트 172개 전부 그린([레벨 분포](docs/test-map.md)), 문서 게이트(`make check-docs` — 링크·앵커·낡은 범위 표현·개인정보).
- **실물 provider 확인 (선택 단계)** — `DEMO_REAL_PROVIDER=1`이면 데모가 실물 oqsprovider를 빌드해 OpenSSL 3.0–3.4 노드에 배치·활성화하고, **능력이 실제로 생겼는지**를 `openssl list`로 잰다(ML-KEM KEM 0개 → 14개, 되돌리면 다시 0개). 이 확인이 설정 파일을 만드는 쪽의 결함 하나를 잡았다 — 생성한 조각에 최상위 `openssl_conf = openssl_init`이 없어, 조각을 `OPENSSL_CONF`로 가리키는 환경에서 **배치도 sha256 게이트도 통과하는데 provider가 올라오지 않았다**. 고치고 회귀 테스트를 붙였다.

- **릴리스 발행** — 태그를 밀면 CI가 arch별 정적 바이너리(`linux-amd64`·`linux-arm64`)와 `collector.jar`를 만들고 `SHA256SUMS`를 붙인다. 받은 뒤 `sha256sum -c SHA256SUMS`로 확인한다.

### 알아낸 것

- **지원 커널 하한 = 3.2** (Go 툴체인이 정하는 값 — 1.24에서 이 값이 됐고 이후 유지된다. 빌드에 필요한 Go는 `go.mod` 기준 1.26.4다). 이 리포는 그보다 새 기능을 요구하지 않고, 기능별 추가 요구는 컨테이너 안 JVM attach의 `NSpid`(4.1) 하나뿐이며 그마저 호스트 PID로 폴백한다. 표는 [discovery/cmd — 지원 범위](discovery/cmd/README.md#실행-요건--커널권한).
- **레거시 실기 확인 완료** — 커널 **3.2**(Ubuntu 12.04)와 **3.10**(CentOS 7.9) VM에서 세 collector 실행. 하한 그 자체에서 돌고, 둘 다 `NSpid`가 없어 호스트 PID 폴백까지 실물로 확인됐다. 3.2에는 systemd가 없어 앱 귀속이 실행 파일 경로로 떨어지는 것도 관측됐다.
---

<!-- v0.2.0 이후는 이 위에 새 섹션으로 추가한다. -->
