# jvm-collector — 살아있는 JCA provider 체인 관측

**설계 목표**: 실행 중인 JVM에 붙어 `Security.getProviders()`의 **실체를 등록 순서까지** 본다. 이 관측은 **정적 분석으로 대체할 수 없다** — 코드가 런타임에 `Security.addProvider()`로 넣은 provider는 `java.security` 파일에도, JAR 목록에도 없기 때문이다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../../docs/플랫폼_규정.md)의 절 번호다.

근거: [디스커버리 설계 §2.2](../../디스커버리_설계.md) · 인수 기준: [테스트케이스 SD-2](../../디스커버리_테스트케이스.md) · 배포: [collector 배포 설계](../../collector_배포_설계.md)

---

## 1. attach란 무엇인가 — Java 기능이 아니라 **OS IPC**

`jdk.attach`(`VirtualMachine.attach`)는 마법이 아니라 **운영체제 수준의 절차**를 감싼 편의 API다. HotSpot에서 실제로 일어나는 일:

```
① 트리거 파일 생성      .attach_pid<nspid>   (대상의 cwd 또는 /tmp)
② SIGQUIT 전송          → 대상이 "attach 요청"으로 인식해 Attach Listener 스레드 개설
③ 유닉스 소켓 연결      /tmp/.java_pid<nspid>
④ 명령 전송             "1\0load\0instrument\0false\0<jar>=<옵션>\0"
⑤ 응답 읽기             첫 줄 = 리턴 코드(0=성공), 이후 = 메시지
```

**이 절차는 언어 무관**이다. 그래서 이걸 **Go로 직접 구현**했다(`NativeAttach`) — openssl collector가 `ldd`·`readelf` 없이 `/proc`·ELF를 자체 파싱하는 것과 같은 원칙(외부 툴체인 비의존, §2.4).

> ⚠️ **트리거 파일과 SIGQUIT은 둘 다, 이 순서로.** 파일 없이 SIGQUIT만 보내면 JVM은 평범한 스레드 덤프 요청으로 보고 **애플리케이션 stdout에 덤프를 쏟는다.** 신호 처리가 비동기라 **파일은 소켓이 열린 뒤에** 지워야 한다. (구현 중 실제로 이 실수를 했고 데모 실행이 잡아냈다.)

---

## 2. ★ attach 경로 3계층

하나가 막히면 다음으로, 다 막히면 **정직한 갭**으로 내려간다. 커버리지와 한계가 서로 보완적이라 **교체가 아니라 계층**이다.

| 순위 | 경로 | 클라이언트 | 커버 | 한계 |
|---|---|---|---|---|
| **①** | **Go 네이티브** (`NativeAttach`) | Go 바이너리 | **JDK 불필요** — 순수 JRE·jlink 런타임·최소 컨테이너 | **HotSpot 전용** |
| ② | JDK 클라이언트 (`SubprocessRunner`) | 대상 또는 머신의 JDK | **벤더 무관** — OpenJ9 등 비-HotSpot | 머신에 attach 가능 JDK가 있어야 |
| ③ | 정적 폴백 (**`StaticFallbackGo`**, Java판도 있음) | Go(또는 대상 JVM) | **어떤 JVM·런타임이어도** — `java.security`는 텍스트 파일 | **동적 등록 사각** → 강등·갭 고지 |

- **②가 남아 있는 이유**: ①의 소켓 프로토콜은 HotSpot 구현이라 **OpenJ9**(공유 세마포어 + 다른 IPC)엔 안 통한다. ②는 그 JDK 자신의 attach 구현을 쓰므로 벤더를 안 가린다.
- **②의 클라이언트 선택**: 대상이 순수 JRE여도, 머신에 attach 가능한 JDK가 있으면 **그걸 클라이언트로 재사용**한다(`AttachClient`). 클라이언트는 대상의 java일 필요가 없다.
- **③으로 내려가는 조건**: `DisableAttachMechanism`, JEP 451(최신 JDK는 동적 에이전트 로딩 기본 차단 — `-XX:+EnableDynamicAgentLoading` 필요), 권한 부족, 비-HotSpot+JDK 없음 등.
- **③이 Go에도 있는 이유**: 기존엔 `StaticFallback.java`뿐이라 **그걸 돌릴 java가 필요**했고, ②는 `--add-modules jdk.attach`로 떠서 순수 JRE에선 시작조차 못 해 **폴백까지 함께 못 돌았다**(노드가 통째로 갭). `java.security`는 텍스트 파일이라 Go가 직접 읽어 그 구멍을 닫았다.

### 배포에 미치는 영향

①이 있으므로 노드엔 **Go 바이너리 + `collector.jar`(에이전트)** 만 있으면 된다. **미니 JDK 동봉은 불필요**하다 → [collector 배포 설계 §2](../../collector_배포_설계.md).

---

## 3. 왜 이 collector만 폴리글랏인가

**에이전트는 JVM 안에서 돌아야 하므로 Java일 수밖에 없다.** 그게 유일한 강제이고, 나머지는 Go다.

| 층 | 언어 | 하는 일 |
|---|---|---|
| **에이전트** (`IntrospectAgent.java`) | **Java (불가피)** | 대상 JVM **안에서** `Security.getProviders()` 조회 → 결과 파일 기록 |
| 정적 폴백 (`StaticFallback.java`) | Java | attach 불가 시 `java.security` 정적 등록만 읽음 |
| attach 클라이언트 ①·정찰·정규화 | **Go** | OS IPC로 직접 attach, `/proc` 정찰, 정규화된 CBOM Envelope 변환, intake 계약(§6.1) |
| attach 클라이언트 ② (`Attacher.java`) | Java | 벤더 무관 폴백 경로에서만 쓰임 |

> **제약은 "JVM"이지 특정 언어가 아니다.** 사이드카는 플랫폼 자신의 언어인 **순수 Java**로 쓴다 — Kotlin·Gradle 없이 `javac`+`jar`, 산출물은 이식적인 JAR 하나.

---

## 4. 먼저 정찰한다 — 어떤 JVM이 도는지 (openssl과 대칭)

openssl collector가 `/proc`를 훑어 로드된 libssl을 스스로 찾듯, **jvm도 실행 중인 JVM을 먼저 조사한다**(`ScanJVMs`, [procscan.go](procscan.go)). 머신에 JDK가 여럿일 수 있고 **어느 JVM을 보느냐가 결과를 바꾸므로**, 호출자가 PID·JDK 경로를 미리 알아야 하던 비대칭을 없앤다.

- **식별**: `/proc/<pid>/exe`가 `java`거나 `/proc/<pid>/maps`에 `libjvm.so`가 있는 프로세스(래퍼로 재실행돼 exe가 java가 아니어도 잡는다).
- **뽑는 것**: PID · 런처 경로 · 파생 `JAVA_HOME` · `release`의 버전 · **`AttachCapable`**(=`$JAVA_HOME/lib/libattach.so` 존재 → jdk.attach 있는 JDK인가). best-effort라 못 짚으면 빈 값 — 추측하지 않는다(§2.6).
- **`AttachCapable`의 쓰임**: ②의 클라이언트 선택, 그리고 **attach 실패 사유를 미리 설명**(§2.7 갭 고지의 질), 나아가 [배포 결정](../../collector_배포_설계.md)의 입력.
- **못 읽은 프로세스는 갭**: 타 사용자·종료로 접근 불가면 `Denied`로 세어 완전성 갭의 원천으로(§2.7). 조용한 0이 아니다.
- **커버리지는 권한에 달렸다**: root(또는 동일 UID)면 그 사용자 프로세스를 본다.

**정찰 → attach로 이으면** 발견한 각 PID에 실제로 붙어 provider 체인(동적 등록 포함)을 관측한다(`AttachAll`, [attach.go](attach.go)).

- **다중 JVM 구별**: 한 노드에 JVM이 여럿이면 각각 **구별되는 finding**이 된다 — 식별자는 **앱**(cmdline의 main 클래스·`-jar`) 우선, 없으면 JAVA_HOME→exe. **PID는 안 쓴다**(매 스캔 달라져 이력이 "매번 새 자산"으로 깨진다). 한 JDK에 앱이 여럿이어도 dedup으로 하나가 사라지지 않는다.
- **attach 실패는 갭**: 차단·권한 부족한 JVM은 조용히 버리지 않고 `AttachStats.Failed`로 센다(§2.7).

> **관측 경로 두 갈래**: **프로브**(경량)는 별도 JVM을 띄워 **정적 등록 체인만** 본다. **attach**는 실행 중 앱의 `addProvider()` **동적 등록까지** 본다. 정찰은 어느 쪽이든 대상을 찾아주는 선행 단계다.

---

## 5. 등록 **순서**가 의미를 가진다

`provider_set`은 정렬하지 않고 **등록 순서 그대로** 보존한다. JCA는 목록에서 앞선 provider가 같은 알고리즘을 먼저 서비스하므로, **BouncyCastle을 넣어도 앞자리에 없으면 무시된다**(§1.2). 순서를 잃으면 "PQC provider가 있다"는 관측이 "실제로 그게 쓰인다"를 보장하지 못한다.

---

## 6. 열화 — 실패가 조용한 0이 되지 않는다

③ 정적 폴백으로 내려가면:

- `java.security`의 **정적 등록** provider만 읽는다.
- **동적 등록은 이 경로의 사각지대**이며, 그 사실 자체를 **완전성 갭으로 보고**한다. "없다"고 말하지 않는다(§2.6).
- `detection_method`: runtime-introspection → **artifact**, `evidence_strength`: confirmed → **inferred**로 내려간다.

즉 **관측 실패가 증거 등급의 하락으로 정직하게 드러난다**(§2.7).

---

## 7. 전제 (권한·환경)

| 항목 | 필요 | 비고 |
|---|---|---|
| **동일 UID** (또는 root) | ✅ 필수 | 대상 프로세스에 신호·소켓 접근 |
| 네임스페이스 | 컨테이너 대상이면 `/proc/<pid>/root`·`NSpid` 경유 | 소켓·트리거 파일 이름은 **네임스페이스 내부 PID** |
| 대상의 attach 허용 | ✅ | `DisableAttachMechanism` 아님, JEP 451이면 `-XX:+EnableDynamicAgentLoading` |
| **머신의 JDK** | ❌ **불필요** | ①(Go 네이티브)가 JDK 없이 붙는다. ②로 폴백할 때만 필요 |

---

## 8. 사용법

```bash
# 정찰만 (프로브 경로 — 정적 등록 체인)
pqcota-jvmscan <node-id>

# 정찰 → attach (동적 등록까지) — 에이전트 JAR를 주면 이 경로를 탄다
PQCOTA_JVM_AGENT=/opt/collector.jar pqcota-jvmscan <node-id>
```

| 환경변수 | 뜻 |
|---|---|
| `PQCOTA_JVM_AGENT` | 에이전트 JAR 경로. **있으면 attach 경로**(①→②→③), 없으면 프로브 |
| `JAVA_BIN` | 프로브에 쓸 java 명시(미지정 시 정찰로 발견한 java → PATH의 java) |
| `JVMSCAN_CP` | 프로브 classpath(bcprov 등) |

출력은 **JSON Lines** — attach 경로는 JVM별로 한 줄씩 낸다(다중 JVM). `pqcota-ingest`가 `*.jsonl`을 읽는다.

---

## 9. 경계 — collector는 관측까지

provider 이름에서 `pqc_readiness`(전 표준 커버 / SLH-DSA 갭 등)를 판정하는 것은 **코어의 provider 시그니처 레지스트리**(§2.3)다. 이 collector는 "무엇이 어떤 순서로 등록돼 있나"까지만 낸다.

---

## 10. 구조

| 경로 | 역할 |
|---|---|
| `procscan.go` · `procscan_parse.go` | `/proc` 정찰 — JVM 발견·JAVA_HOME·버전·`AttachCapable`(순수 파싱 분리) |
| **`nativeattach_linux.go`** | **① Go 네이티브 attach** — 트리거·SIGQUIT·소켓·네임스페이스 교차 |
| **`nativeattach_parse.go`** | 프로토콜 순수 부분(요청 조립·응답 파싱·NSpid) — 실 JVM 없이 테스트 |
| `attach.go` | 다중 JVM attach 오케스트레이션 + `AttachClient`(②용 JDK 선택) |
| `runner.go` | ② JDK 클라이언트 서브프로세스 실행(주입 가능) |
| `parse.go` | 사이드카 출력 → 정규화된 CBOM Envelope. 순서·갭 보존 |
| `service.go` | intake 계약(§6.1) 노출 — openssl-collector와 대칭 |
| `collector/` | **순수 Java 사이드카** — `IntrospectAgent.java`(에이전트) · `Attacher.java`(② 클라이언트) · `StaticFallback.java`(③) |
| `attach-poc/` | attach 가능성 검증용 최소 PoC(설계 근거 자료) |

## 11. 돌려보기

```bash
go test ./discovery/collectors/jvm/...            # 단위(실 JVM 없이 — 프로토콜·정찰 파싱·오케스트레이션)
./examples/discovery/jvm/run.sh                   # 실 JVM attach (Docker + JDK)
```

진입점은 [`discovery/cmd/pqcota-jvmscan`](../../cmd/README.md).

## 12. 버전 축 둘 — 빌드 JDK와 대상 JVM

| 무엇 | 요구 | 왜 |
|---|---|---|
| **사이드카 빌드** | JDK 11+ (`javac`) | `Attacher`가 `com.sun.tools.attach`(JDK 9+ 모듈)를 쓴다 |
| **대상 JVM**(관측 대상) | **Java 8+** | 대상 안에서 로드되는 것은 `IntrospectAgent`뿐이고 `--release 8`로 컴파일한다 |

**둘을 나누는 이유가 곧 커버리지다.** 클래스 파일 버전이 대상보다 높으면 JVM은 **로드조차 하지
않는다**(`UnsupportedClassVersionError`). 한 번에 컴파일하면 전부 빌드 JDK의 버전이 되어, JDK 21로
만든 jar은 JDK 17·11·8 대상에서 전부 실패한다(실측 확인). 관측 대상이 대개 레거시라 이 하한이 그대로
관측 가능 범위가 된다.

그래서 `make build-jar`는 두 번 컴파일한다 — `IntrospectAgent`만 `--release 8`, 나머지는 `--release 11`.
`IntrospectAgent`가 Java 9+ API를 직접 부르지 않는 이유도 이것이다(provider 버전은 `getVersionStr()`을
리플렉션으로 찾고 없으면 Java 8의 `getVersion()`으로 떨어진다).

실측: **JDK 1.8.0_492 대상에 attach 성공** — provider 체인 9개(`SUN,SunRsaSign,…,SunPCSC`)를 읽었다.
