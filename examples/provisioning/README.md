# examples/provisioning — 계획별로 무엇이 생성되는지 보기

같은 커맨드에 **계획만 바꿔 넣으면** 산출물이 어떻게 달라지는지 케이스로 갈라 뒀다. 각 케이스는 [`plans/`](plans/README.md)의 JSON 한 개다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/플랫폼_규정.md)의 절 번호다.

```bash
./examples/provisioning/run.sh                          # 케이스 목록 + 기본 실행
./examples/provisioning/run.sh openssl-3.0-provider-inject
./examples/provisioning/run.sh custom-jca-provider --rollback
./examples/provisioning/run.sh --all
```

> 전체 흐름과 판단 근거는 [Provisioning](../../provisioning/README.md)에 있다. 여기는 **케이스별 실물 산출물**을 보는 곳이다.

> ⚠️ **예제는 플레이북을 생성한다.** `oqsprovider.so`·`acme-jce.jar` 같은 **provider 모듈 바이너리는 이 리포에 없다**(arch별로 다르고, 더미를 넣으면 동작하는 것처럼 보여 해롭다). 생성된 플레이북을 실제로 돌리려면 [`files/`](files/README.md)에 자기 모듈을 두거나 `-e pqcota_module_src_<이름>=`로 경로를 준다.
>
> **진짜 provider로 끝까지 보고 싶다면** — 해시를 고정해 받고, 생성된 조각이 정말 provider를 등록하는지 실제 JVM에서 확인한다:
>
> ```bash
> ./examples/provisioning/files/fetch-example-provider.sh   # BC 1.85 (sha256 대조)
> ./examples/provisioning/files/verify-registration.sh      # 조각 적용 전후 provider 목록 대조
> ```
>
> **OpenSSL 쪽 같은 확인**은 데모의 선택 단계가 한다 — `DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh`. 실물 oqsprovider를 빌드해 3.0–3.4 노드에 배치·활성화하고 `openssl list`로 전후를 잰다([데모 README](../../demo/README.md#선택-단계--실물-provider로-마지막-한-칸까지-demo_real_provider1)).

## OpenSSL — 버전이 조치를 정한다

| 케이스 | 관측 상황 | `kind` | 생성되는 것 |
|---|---|---|---|
| [`openssl-3.5-config-only`](plans/openssl-3.5-config-only.json) | 3.5+ 네이티브 PQC | `CONFIG_ONLY` | `Groups = X25519MLKEM768:x25519` **한 줄**. provider 모듈 없음 |
| [`openssl-3.0-provider-inject`](plans/openssl-3.0-provider-inject.json) | 3.0–3.4 (provider API 있음) | `PROVIDER_INJECT` | 모듈 배치 `/opt/pqcota/oqsprovider.so` + 그 **절대 경로를 참조**하는 config |
| [`openssl-1.1.1-fork-replace`](plans/openssl-1.1.1-fork-replace.json) | 1.1.1·1.0.2 (provider API 없음) | `FORK_REPLACE` | **아무것도 배치 안 함** — `# config로 배포 불가 — 수동 단계` 주석 |

**핵심**: 버전이 낮을수록 도구가 해줄 수 있는 게 줄어든다. 1.1.1은 조용히 빠지지 않고 **왜 수동인지가 플레이북에 남는다.**

## JVM/JCA — provider 상황이 조치를 정한다

| 케이스 | 관측 상황 | `kind` · `providerChoice` | 생성되는 것 |
|---|---|---|---|
| [`jca-native-config-only`](plans/jca-native-config-only.json) | JDK 네이티브 PQC | `CONFIG_ONLY` | `jdk.tls.namedGroups=…` 한 줄. provider 무등록 |
| [`jca-provider-inject-bc`](plans/jca-provider-inject-bc.json) | PQC 없는 provider 체인 | `PROVIDER_INJECT` · `BC` | JAR 배치 + `security.provider.2=org.bouncycastle.jce.provider.BouncyCastleProvider` |
| [`jca-fips-bcfips`](plans/jca-fips-bcfips.json) | **규제 자산** | `PROVIDER_INJECT` · `BCFIPS` | 같은 흐름이나 **등록 클래스가 다르다**(`BouncyCastleFipsProvider`) — FIPS 라우팅 |
| [`jca-eol-jdk-upgrade`](plans/jca-eol-jdk-upgrade.json) | EOL JDK | `JDK_UPGRADE` | **아무것도 배치 안 함** — 수동 단계 주석 |

**핵심**: `providerChoice`가 **등록 클래스명을 정한다.** 규제 여부에 따라 BC ↔ BC-FJA가 갈리는 게 계획 단계의 판정이다.

> JCA `PROVIDER_INJECT`는 항상 **우선순위 2**에 등록한다. JCA는 목록에서 앞선 provider가 먼저 서비스하므로, 뒤에 넣으면 JAR이 있어도 **아무것도 바뀌지 않는다**(수용 원칙 §2.2(d)).

## 커스텀 provider

| 케이스 | 무엇을 보여주나 |
|---|---|
| [`custom-openssl-provider`](plans/custom-openssl-provider.json) | 사내 빌드를 가정한 `acme-pqc.so` — 모듈 절대 경로, `pqcota_module_src_acme_pqc` 변수, sha256 게이트가 자동 생성 |
| [`custom-jca-provider`](plans/custom-jca-provider.json) | 사내 빌드를 가정한 `acme-jce.jar` — **`providerClass`에 FQCN을 명시**해 계획만으로 완결 |
| [`custom-jca-missing-class`](plans/custom-jca-missing-class.json) | 같은 것에서 `providerClass`만 뺐을 때 — **placeholder + 무엇을 하면 되는지** 안내 |

둘 다 **계획만으로 완결**된다. 단, JCA는 한 필드가 더 필요하다:

```json
{"providerChoice": "acme-jce", "providerClass": "com.acme.jce.AcmeProvider"}
```

```
# OpenSSL — 경로만 알면 된다(생성기가 정한다)
module = /opt/pqcota/acme-pqc.so

# JCA — FQCN이 필요하다(계획이 알려줘야 한다)
security.provider.2=com.acme.jce.AcmeProvider
```

**왜 한 필드가 더 필요한가**: OpenSSL은 **경로**만 알면 되고 그 경로는 플레이북이 정한다. JCA는 java.security에 **FQCN**을 적어야 하는데 벤더마다 패키지 구조가 달라 provider 이름에서 유도할 수 없다. BC/BC-FJA만 알려져 있어 자동으로 채워지고, 그 외에는 계획이 알려줘야 한다.

명시하지 않으면 추측하지 않고 이렇게 남긴다(`custom-jca-missing-class`):

```
# ⚠ 아래 클래스명은 placeholder — provider 배포본의 정식 클래스로 교체하거나
#   계획의 provider_class에 FQCN을 넣으면 자동으로 채워진다.
security.provider.2=<acme-jce: provider 문서의 정식 클래스명 확인>
```

> **JAR을 JVM이 찾게 하려면** classpath에 얹어야 한다. 방법이 JDK 세대마다 달라(확장 메커니즘 `lib/ext`는 **JDK 9에서 제거**됨) 생성된 조각이 둘 다 안내한다.

모듈 반입 절차(컨트롤러 → 타깃, `files/` 관례, sha256)는 [provisioning/cmd § 적용하기](../../provisioning/cmd/README.md#적용하기).

## 경계 케이스

| 케이스 | 무엇을 보여주나 |
|---|---|
| [`signature-algorithm`](plans/signature-algorithm.json) | `targetAlgorithm`이 **서명**(ML-DSA)이면 KEM 그룹이 아니라 그룹 줄이 **주석으로** 나온다 — 추측해서 채우지 않는다 |
| [`00-basic-two-actions`](plans/00-basic-two-actions.json) | 한 계획에 노드 둘 — **노드별 play**로 갈리고 각자 자기 조치만 받는다 |

## 게이트 확인해 보기

아무 plan의 `"status"`를 `"PLAN_STATUS_DRAFT"`로 바꿔 돌리면 거부된다(exit 1):

```
거부: 계획이 FINALIZED 아님(PLAN_STATUS_DRAFT). 프로비저닝 근거는 확정 계획뿐(§3.7).
```

`approvalSignatures`를 비워도 마찬가지다.

## L3 — 활성화·재시작

L1/L2는 놓기만 한다. **L3는 놓은 것이 실제로 참조되게 만들고 프로세스를 다시 띄운다.** 활성화 지점은
환경마다 다르므로(systemd 드롭인·include 디렉터리·사내 기동 스크립트) 도구가 추측하지 않는다 —
명령은 계획의 `activation`에 적고, 생성기는 그것을 **의미 순서**로 배치한다.

```bash
./examples/provisioning/run.sh l3-activation-hooks             # forward
./examples/provisioning/run.sh l3-activation-hooks --rollback  # 역순
```

| 훅 | 언제 | forward | rollback |
|---|---|---|---|
| `pre` | 조치 전 내릴 것 | ① | ① |
| — | 모듈·config 배치 | ② | ③ (제거) |
| `activate` | 조각이 참조되게 | ③ | — |
| `deactivate` | `activate`의 역 | — | ② |
| `restart` | 새 provider 로드 | ④ | ④ |

즉 forward는 `pre → 배치 → activate → restart`, 롤백은 `pre → deactivate → 제거 → restart`.
한 노드에 조치가 여러 개여도 **같은 명령은 한 번만** 나간다 — 조치마다 재시작하면 서비스를 여러 번
흔들고, 활성화 사이에 재시작이 끼어 일부만 반영된 채 뜬다.

`l3-activation-hooks`는 JCA 케이스라 **JAR 배치≠로드** 함정을 훅으로 닫는 모습을 보인다. 그 안의
경로·변수명은 예시일 뿐이니 당신 앱의 기동 방식으로 바꿔 적는다.

### 훅이 없으면 — 지어내지 않는다

```bash
./examples/provisioning/run.sh l3-hooks-missing
```

같은 계획에서 훅만 뺐다. 생성기는 활성화 명령을 **만들지 않고**, 대신 무엇이 일어나지 *않는지*를
stderr로 알린다: `activate` 없음 = 조각을 놓기만 하고 참조되게 만들지 않음, `restart` 없음 = 새
provider가 로드되지 않을 수 있음, `deactivate` 없음 = 롤백이 활성화를 되돌리지 못함.

## 롤백

어느 케이스든 `--rollback`을 붙이면 **forward가 놓은 것을 제거하는** 플레이북이 나온다. 원본 설정을 덮은 적이 없으므로 제거만으로 이전 상태가 된다.

```bash
./examples/provisioning/run.sh custom-openssl-provider --rollback
```

## `--dsn`을 주면

히스토리에서 before-findings를 읽어 **조치 전 상태**와 **영향 앱**을 append-only 레코드로 남긴다(§6A). 저장소에 디스커버리가 먼저 적재돼 있어야 하므로 종단 흐름은 [demo/](../../demo)가 보인다.

## 없는 것

**동적 프로비저닝**(재시작 없이 실행 중 프로세스에 주입)은 하지 않는다. **플릿 오케스트레이션**
(drain·rolling·헬스체크 게이트)은 하지 않는다. 커맨드 지도: [provisioning/cmd/README](../../provisioning/cmd/README.md).
