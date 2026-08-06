# 검토 중인 설계 — 아직 짓지 않은 것

> **코드를 바꾸지 않는 문서다.** 로드맵에 올라왔지만 구현을 확정하지 않은 것들의 설계를 여기서
> 진행한다. 본 설계 문서(디스커버리·인벤토리·프로비저닝)는 **지금 서 있는 것만** 적는다 — 검토 중인
> 것이 섞이면 무엇이 사실이고 무엇이 계획인지 갈리지 않는다.

**수명** — 항목은 넷 중 하나로 끝난다.

| | |
|---|---|
| **① 로드맵 등재** | 방향이 생기면 [로드맵](../RELEASE_NOTES.md#로드맵--예정-릴리스-계획)에 한 줄, 설계는 여기서 |
| **② 여기서 설계** | 코드 무변경. 무엇이 걸리고 무엇을 정해야 하는지까지 |
| **③ 본 설계로 승격** | 구현 계획이 확정되면 해당 단계 설계 문서에 반영하고 **여기서는 지운다** |
| **④ 안 하기로** | 로드맵의 [「안 만드는 것」](../RELEASE_NOTES.md#로드맵에-없는-것--안-만든다)으로 옮긴다 |

**이미 다른 문서가 들고 있는 미결은 여기 두지 않는다.** 양쪽에 적히면 한쪽만 고쳐진다 — 실제로
HSM 축이 여기와 [암호 런타임 수용 원칙](runtime-acceptance.md) 부록에 함께 있었다. 미결은 그것을
낳은 문서가 계속 들고, 여기는 **로드맵에 오른 것**만 받는다.

---

## 1. provider를 "지원한다"의 뜻

provider는 **사용자가 선택하고 준비한다.** 그 provider를 받는 쪽은 세 층으로 나뉜다 — 앞의 둘은
도구가 실행 중에 하는 일이고, 셋째는 후보를 늘릴 때 거치는 확인 절차다:

| 층 | 내용 | 지금 상태 |
|---|---|---|
| **배치 메커니즘** | 사용자가 준 모듈 파일을 스테이징·sha256 게이트·롤백이 받는다(OpenSSL `.so`·JCA `.jar`) | ✅ provider 무관 — 이미 전부 됨 |
| **config 어휘** | 도구가 **그 provider가 요구하는 설정 조각**을 만든다 | ⚠ 지금은 `activate = 1` + `module = 경로` 한 모양뿐 |
| **실물 확인** | 후보를 추가할 때 실물로 종단(배치→활성→관측)까지 확인해 후보 표에 적는다 | ◐ oqsprovider만 확인됨 — 나머지 후보는 미확인 |

**"wolfProvider를 지원한다" = config 어휘가 그 provider의 모양을 아는 것 + 실물 확인.** 후보별로
무엇이 더 필요한지는 [plans README](../examples/provisioning/plans/README.md#어떤-provider를-넣을-수-있나)에
표로 있다 — 계획을 쓰면서 `providerChoice`를 고르는 자리가 거기다.

FIPS 검증본 확인은 사용자가 한다(검증서는 빌드 단위라 파일만 봐서 모른다). 도구가 보장하는 것은
**사용자가 고른 그 파일이 그대로 배치됐다**(sha256)까지다.

**남은 후보의 실물 확인도 같은 방법으로 한다** — 실물을 배치·활성화하고 능력이 생겼는지 `openssl list`로
전후를 잰 뒤 되돌린다. oqsprovider가 그 경로를 밟았고, 데모의 `DEMO_REAL_PROVIDER=1`이 그것을
그대로 돌린다([데모 README](../demo/README.md#선택-단계--실물-provider로-마지막-한-칸까지-demo_real_provider1)).

**정해야 하는 것 — config 어휘를 어떻게 여나.** provider별 조각을 코드에 하나씩 넣을지, 계획에 config
조각을 실을 자리(`extraConfig` 류)를 열지. 후자는 코드를 고치지 않고 후보 전부를 받지만 "지어내지
않는다" 원칙과 저울질이 필요하다.

---

## 2. 아직 수용하지 않은 런타임 후보

**언어 표면은 후보가 아니다.** Python(링크한 libcrypto)·Node(대체로 OpenSSL 배달 차량)·
.NET(하부 OS 따라 CNG 또는 OpenSSL)은 자기 substrate가 없어 openssl/CNG로 귀속된다 — 아래 셋만
실제로 조건에 대볼 값이 있었다.

### 2.1 정적 Go — 조건 1에 걸린다

런타임 introspection 대상(provider)이 **없다.** 정적 ELF 심볼·`debug/buildinfo`로 `artifact`·
`symbol-analysis` 레인까지고(`inferred-low`, `confirmed` 아님), 협상 알고리즘은 netcap이 언어무관으로
와이어에서 이미 본다. provider가 없으니 remediation은 `REBUILD` 하나뿐이고, 배포할 파일 아티팩트도
없다(재빌드는 CI 몫).

**판정: 계약은 버티되 버팀의 방식이 "흡수"다.** 새 enum·render·substrate가 전부 불필요하다 — Go는
새 런타임이 아니라 **기존 taxonomy의 소비자**다.

### 2.2 Windows CNG — 조건 2에 걸린다

수집은 완전 신규다(`BCryptEnumProviders`·레지스트리 — `/proc`·ELF·AF_PACKET 재사용 0). 스키마는
oneof라 순수 additive로 붙는다. Kind 어휘도 런타임 무관이라 **분류까지는** 재사용된다.

부러지는 곳은 아티팩트다. `Render`가 내야 하는 것이 `[openssl_init]` 텍스트가 아니라 레지스트리·GPO·
`Register-CngProvider`이고, 그건 곧 substrate다 — **render 층은 substrate와 분리되지 않는다.**

**판정: 절반만 얹힌다.** 디스커버리·인벤토리 절반은 지금 프레임에 그대로 오지만, 프로비저닝 절반은
substrate 일반화를 **선행 요구**한다. 그 일반화는 **Windows를 구현할 때 함께** 한다 — 구체 사례
없이 Registry/GPO 인터페이스를 미리 뽑으면 잘못된 seam을 뚫는다(투기적 추상화 금지).

**정하지 않은 것** — substrate 추상 인터페이스의 seam(File-Stage vs Registry/GPO vs Config-Only)을
어디에 그을지. Windows 구현이 손에 오기 전에는 정하지 않는다.

### 2.3 HSM / PKCS#11 — 어느 조건도 건드리지 않는다

remediation이 대개 **openssl(`pkcs11-provider`)이나 jca(`SunPKCS11`)를 HSM으로 *가리키는*** 것이다.
렌더는 기존 `PROVIDER_INJECT`에 "이 provider의 target이 HSM 슬롯"이라는 파라미터를 얹는 일이고,
substrate(파일 스테이징 + config 조각)는 그대로 쓰인다.

**판정: 새 `CryptoRuntime` enum이 아니다.** 디스커버리 axis 신설 + provider가 가리키는 **backend
target**으로 모델링한다. 진짜 peer인 경우는 앱이 벤더 `.so`를 openssl/jca **없이 직접 링크**할 때뿐이고,
그마저 provider 동형 대상이라 openssl-계열 render로 근사된다.

> **정직성 각주** — HSM 하드웨어가 PQC를 아직 지원 안 하는 건은 억지 아티팩트를 지어내지 않고
> `DECOMMISSION`·`APP_RECONFIG` 주석으로 처리된다. taxonomy가 이미 이 비-config 종류를 갖고 있다.

**정하지 않은 것** — HSM axis의 실제 필드(슬롯·모듈 경로·펌웨어 버전 등). 실물 PKCS#11 관측
사례가 있을 때 정한다.

### 2.4 걸리는 층은 하나다

| 후보 | 걸리는 가정 | 판정 |
|---|---|---|
| 정적 Go | 1 (provider 동형성) | `REBUILD` taxonomy로 **흡수** — enum 아님 |
| Windows CNG | 2 (POSIX 파일) | 별도 트랙 — substrate 일반화가 선행 |
| HSM/PKCS#11 | 없음 | 새 **axis + backend target** — enum 아님 |

조건과 판정하는 법은 [암호 런타임 수용 원칙](runtime-acceptance.md)이 정한다.
여기 있는 것은 그 조건에 **특정 후보 셋을 대본 결과**다 — 후보가 늘면 여기에 붙는다.
