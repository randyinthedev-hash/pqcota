# collector 배포 설계 (T1 self-service)

> **성격**: 설계 문서 + 일부 구현. **반입 방식(§4B)은 구현됐다** — Ansible 참조 플레이북이 collector를
> 반입→실행→회수→정리하고, 데모가 매번 그 경로로 돌아 상시 검증된다. **아직인 것은 서명**(`pqcota-verify-bundle`)이며 [로드맵](../RELEASE_NOTES.md)에 있다 —
> 릴리스에는 arch별 정적 바이너리와 `SHA256SUMS`가 붙으므로 그때까지 무결성 확인은 `sha256sum -c`로 한다.

collector를 **관측 대상 호스트에 어떻게 올리고, 그것이 진짜 이 리포가 낸 것인지 어떻게 확인하나**를 정한다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../docs/regulation.md)의 절 번호다.

## 0. 전제

- **collector는 기존 substrate(Ansible·Salt 등)에 얹혀 돈다** — 노드 도달은 이미 그쪽이 푸는
  문제라 자체 원격 실행 엔진을 두지 않는다 ([아키텍처 §2.3](../docs/architecture.md)).
- **여기서 만드는 것은 서명된 번들**(T1 self-service)이다. 사용자가 그것을 직접 실행한다 —
  에어갭에서도 성립하는 유일한 채널이다.

## 1. 노드엔 collector만 올린다

| 번들 | 담기는 것 | 어디로 |
|---|---|---|
| **slim** (arch별) | `pqcota-nodescan` · `pqcota-netcap` · `pqcota-jvmscan` | 관측 대상 호스트 |
| **jvm 애드온** (arch 무관) | `collector.jar`(attach 사이드카) | **JVM이 도는 호스트만** |
| (운영자 CLI) | `pqcota-ingest`·`inventory`·`provision` 등 | **중앙에서만** — 노드에 올리지 않는다 |

> 데모도 이대로 한다 — 노드 이미지엔 collector가 **없고**(관측 대상 워크로드만), Ansible이 실행
> 시점에 collector 3종만 반입한다. 노드에 중앙 도구를 뿌리면 발자국과 공격면만 커진다(§2.3).

## 2. ★ 정찰이 배포를 정한다 (핵심 전략)

JVM 애드온을 **모든 노드에 미리 뿌리지 않는다.** 이 플랫폼은 이미 *관측해서 결정*하는 도구이므로,
배포도 같은 방식을 쓴다 — **추측 대신 정찰 결과로.**

```
① slim 번들만 전 노드에 배포 (Go 정적 3종, 가볍다)
        ↓
② pqcota-jvmscan 정찰(ScanJVMs, /proc) — "이 노드에 JVM이 있나? 어디에? attach 가능한가?"
        ↓
③ JVM이 있는 노드에만 collector.jar를 보낸다 (JVM 없으면 아무것도 더 안 보낸다 — 대부분의 노드)
```

**왜 이렇게** — JVM 없는 노드에 Java 애드온을 까는 건 낭비이고, 무엇보다 **누가 JVM인지 이미 관측으로
알 수 있는데 추측할 이유가 없다.** 에어갭 반입량도 최소가 된다.

### 런타임은 동봉하지 않는다 — JDK 없이 직접 붙는다

attach는 Java 기능이 아니라 **OS IPC**다. Go로 직접 구현하면 JDK 의존이 사라지고, 벤더 커버리지를
위해 JDK 클라이언트와 정적 폴백을 뒤에 둔다 — 3계층의 메커니즘·한계·실측은 [jvm collector README
§1–§2](collectors/jvm/README.md)가 SSOT다. 배포에 미치는 결론은 하나:

- **노드엔 Go 바이너리 + `collector.jar`만 올린다.** 미니 JDK(jlink) 동봉은 계획에 없다 — 순수 JRE·
  jlink 런타임·최소 컨테이너까지 ①(Go 네이티브)이 덮고, JDK가 있으면 그것을 재사용한다.
- 그래서 번들 크기가 런타임 유무와 무관하게 고정된다(§3 레이아웃).

### 정찰이 보고하는 것 (구현됨)

`ScanJVMs`가 PID·JavaHome·버전에 더해 **`AttachCapable`**(=`$JAVA_HOME/lib/libattach.so` 존재 →
`jdk.attach` 있는 JDK인가)을 보고한다. 프로세스를 띄우지 않는 파일 검사라 정찰이 가볍다.
이 값이 ③의 배포 결정과 위 클라이언트 선택을 모두 정하고, **attach 실패 사유를 미리 설명**해
갭 고지의 질도 높인다(§2.6).

## 3. 번들 레이아웃

```
pqcota-collector-<ver>-linux-<arch>.tar.gz     # arch: amd64 | arm64 (CGO_ENABLED=0 정적)
  bin/pqcota-nodescan
  bin/pqcota-netcap
  bin/pqcota-jvmscan
  README.txt                                   # 검증·실행 3줄

pqcota-collector-jvm-<ver>.tar.gz              # arch 무관(바이트코드)
  collector.jar

SHA256SUMS                                     # 위 전 아티팩트 해시 한 파일
SHA256SUMS.sig                                 # ed25519 detached 서명 1개
```

## 4. 무결성·진정성 — ed25519 detached + SHA256SUMS

- **해시 한 파일에 모으고 그 파일에만 서명한다** — 아티팩트가 늘어도 서명은 하나(배포판 관례).
- **왜 cosign/sigstore가 아닌가**: 투명성 로그·레지스트리 접근을 전제해 **에어갭과 상충**한다.
  ed25519는 이미 있는 인프라(`pkg/kernel/sign`·`pqcota-keygen`)와 결이 같고 오프라인에서 완결된다.
- **공개키 배포**: 리포와 릴리스 페이지 **두 곳**에 게시해 서로 대조 가능하게.

> ★ **결과 서명 키와 다른 키다.** 지금 있는 ed25519는 **사용자가 자기 관측 결과(`CollectionResult`)에
> 서명**하는 용도다(`PQCOTA_SIGN_KEY`/`PQCOTA_VERIFY_KEY`). 여기서 말하는 건 **릴리스에
> 서명**하는 공급망 키다. 두 키를 섞지 않는다.

**타깃에서의 검증(에어갭 포함, 네트워크 불요):**
```bash
sha256sum -c SHA256SUMS                       # 아티팩트 무결성
pqcota-verify-bundle SHA256SUMS SHA256SUMS.sig <pubkey>   # 진정성(로드맵)
```

## 4A. 지원 범위 — 어디까지 "그냥 도는가"

`CGO_ENABLED=0` 순수 Go 정적 바이너리라 **복사가 곧 설치**다. 다만 *"어느 리눅스나"*는 아니고 축이 셋이다.

### 검증된 것 — 배포판·libc 무관 ✅

같은 바이너리 하나로 실측했다(`ldd` = 동적 의존 0, `file` = statically linked):

| 환경 | 결과 |
|---|---|
| CentOS 7 (glibc 2.17, 2014) · Debian 8 (glibc 2.19, 2015) | ✅ 실행 |
| Alpine 3.19 (**musl**) | ✅ 실행 |
| Ubuntu 20.04 / 22.04 / 24.04 | ✅ (데모 상시) |

10년 된 배포판에서도 돈다 — glibc 버전도, musl/glibc 구분도 안 가린다.

### 배포를 가르는 축 셋

| 축 | 상태 |
|---|---|
| **아키텍처** | 빌드가 갈린다 — `linux/amd64`·`linux/arm64` 각각 배포(OS 매트릭스가 아니라 **arch 매트릭스**) |
| **커널 하한** | **3.2 — 실기로 확정했다.** Go 툴체인이 정하는 값이고 툴체인 버전이 오르면 함께 오른다. 컨테이너로는 검증할 수 없어(호스트 커널 공유) KVM VM에서 확인했다 — 아래 |
| **기능별 요구** | `/proc` 마운트(정찰) · **`CAP_NET_RAW`**(netcap) · 시그널 권한과 `/proc/<pid>/root`(네이티브 attach) · **SELinux/AppArmor/seccomp**가 이들을 막을 수 있음 |

> **커널 하한이 왜 중요했나** — 관측 대상이 **레거시 서버**다. RHEL 6(커널 2.6.32)급 장비가 정확히
> OpenSSL 1.0.x를 물고 있을 후보인데, 최신 툴체인으로 빌드한 바이너리가 거기서 안 뜰 수 있다.
> 가장 관측이 필요한 곳에서 못 도는 상황이라 릴리스 전에 확정해야 했다.
>
> **실측** — 커널 **3.2**(Ubuntu 12.04)와 **3.10**(CentOS 7.9) VM에서 세 collector가 모두 정상 종료했다.
> 하한 그 자체에서 돌고, 둘 다 `/proc/<pid>/status`에 `NSpid`가 없어 호스트 PID 폴백까지 실물로 확인됐다.
> 결과 표는 [discovery/cmd — 지원 범위](cmd/README.md#실행-요건--커널권한),
> 릴리스 기록은 [RELEASE_NOTES](../RELEASE_NOTES.md)의 「알아낸 것」이다.
>
> RHEL 6(2.6.32)은 하한 **아래**다 — 돌지 않는다. 그 사실을 아는 것이 확정의 값이다.

## 4B. 반입 방식 — Ansible 참조 플레이북 (구현됨)

**collector는 영구 설치가 아니라 일회성 반입이다.** 상주 에이전트가 아니라 실행 후 종료하는
CLI이므로, 관측이 끝나면 노드는 원래 상태로 돌아간다.

[`discovery/ansible/discover.yml`](ansible/discover.yml)이 **참조 구현**이고 데모가
매번 이 경로로 돈다(즉 상시 검증된다). 4단계다:

```
① 반입    ctl의 collector 3종 → 노드 /tmp/pqcota-collector/   (중앙 CLI는 보내지 않는다)
② 정찰    pqcota-jvmscan -recon → "JVM 있나?" → **있는 노드에만** collector.jar 추가 반입
③ 실행·회수  각 collector 실행 → stdout을 컨트롤러에 저장 (노드에 결과 파일도 남기지 않는다)
④ 정리    스테이징 디렉터리 삭제 — 노드 잔존물 0
```

- **②가 이 설계의 핵심**(§2)을 오케스트레이션에서 실현한다 — JVM 없는 노드엔 Java 애드온이 가지 않는다.
  `-recon`은 관측이 아니라 **배포 판정용**이라 JSON만 내고 끝난다.
- **`become: true`** — `netcap`의 `CAP_NET_RAW`와 `/proc` 전 프로세스 커버리지 때문.
- **실환경 이식**: `collector_bin_dir`를 자기 빌드 산출로 바꾸면 된다(arch별 `dist/linux-amd64` 등 —
  [루트 README · 빌드](../README.md#빌드)). 데모 전용은
  트래픽 생성 헬퍼뿐이고, 실환경은 진짜 트래픽을 관측만 하면 된다.

> **왜 참조 플레이북이 이 리포에 있나** — 아키텍처 §2.3의 *"collector를 사용자 자신의 substrate로 감싸
> 돌릴 수도 있다"*가 이것이다. 만들지 않는 것은 **fleet 규모로 플레이북·패키지를 생성하는 도구**(T2)다.

## 5. 놓이는 위치·권한

- 실행 파일: 일회성이면 `/tmp/pqcota-collector/`(참조 플레이북 기본), 상시 두려면 `/opt/pqcota/bin/` 등 사용자 정책대로.
- **`pqcota-netcap`은 `CAP_NET_RAW`** 필요(`setcap` 또는 root). 나머지는 일반 권한으로 동작하되,
  `/proc` 커버리지는 UID에 달렸다(root면 전 프로세스, 아니면 자기 것만 — §2.6로 고지).
- **상주하지 않는다.** 실행 후 종료하는 CLI이지 에이전트가 아니다(상주형은 만들지 않는다).

## 6. 버전·계약 호환

- collector 버전 = 리포 릴리스 버전. `Envelope`에 `collector_id`·`collector_version`이 이미
  실려, 중앙은 **어느 버전이 낸 관측인지** 항상 안다.
- 계약(`contracts/`)이 바뀌면 그 릴리스의 collector와 중앙이 함께 올라간다 — proto는 additive라
  구버전 collector 결과도 신버전 중앙이 읽을 수 있어야 한다(하위호환은 `buf breaking`이 지킨다).

## 7. 에어갭 흐름

반입: **번들 + SHA256SUMS + .sig + 공개키** → 오프라인 검증 → 실행 → 결과 JSON 반출 →
중앙에서 `pqcota-ingest`(사용자 서명 검증은 `PQCOTA_VERIFY_KEY`). 네트워크·레지스트리 접근이
어느 단계에도 없다.

## 8. 접은 선택지

~~jlink 미니 런타임 아카이브~~ — **불필요**로 결론. 머신의 JDK를 attach 클라이언트로 재사용하고,
없으면 Go 네이티브 attach가 덮는다. attach 가능 JDK가 전무한 환경이 실제로 확인되면 재검토한다.

