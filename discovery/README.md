# Discovery — 관측·발견 (1단계)

실행 중인 시스템이 **실제로 어떤 암호 알고리즘을 쓰는지** 관측한다 — 암호 자체(암호문·키)가 아니라 **어떤 라이브러리·provider·알고리즘이 로드·등록·협상되는지**다. 정적 문서·소스 스캔이 못 보는 **런타임 실체**(동적 등록된 provider, 로드된 라이브러리, 협상된 통신 그룹)를 잡아, 각 자산에 양자내성 등급(🟢 PQC/하이브리드 · 🔴 고전=양자취약 · ⚪ 불명)를 부착한다.

**왜 런타임인가** — 설정(`openssl.cnf`·`java.security`·nginx `ssl_ciphers`)은 *허용 목록*이라 실제와
어긋난다. 거기 PQC 그룹을 적어둬도 상대가 지원하지 않으면 고전으로 떨어지고, `java.security`에 없는
provider를 앱이 실행 중에 등록하기도 한다. 그래서 근거는 **실행 중 로드된 라이브러리**와 **핸드셰이크에서
협상된 알고리즘**에서 얻는다. 런타임에 닿지 못하면 설정으로 물러서되, 그때는 `evidence_strength`를 낮추고
갭으로 표시한다.

## 한눈에

```mermaid
flowchart LR
    H["hosts.csv<br/>접속 정보"] --> T["targets.ini"] --> C["collector 셋<br/>각 노드에서 실행"]
    C --> J["CollectionResult<br/>JSON"] --> I["인벤토리에 적재"]
```

## 무엇으로 이루어지나

| 요소 | 무엇 |
|---|---|
| **접근 준비** — `pqcota-hosts` | 사용자가 쓴 `hosts.csv`에서 Ansible 인벤토리를 만든다 |
| **collector 셋** | 아래 셋. 관측 대상 머신에서 돌며 `CollectionResult`를 낸다 |
| **참조 플레이북** — [`ansible/`](ansible) | 준비된 노드들에 collector를 반입·실행·회수·정리한다 |

| collector | 관측 대상 | 방법 |
|---|---|---|
| **[openssl](collectors/openssl/README.md)** | 로드된 libcrypto/libssl, fork, 앱 귀속 | `/proc`·ELF **자체 파싱**(Linux) — `ldd`·`readelf` 비의존 |
| **[jvm](collectors/jvm/README.md)** ★ | 살아있는 JCA provider 체인의 **실체**(등록 순서 포함) | JVM attach → `getProviders()` (순수 Java 사이드카) |
| **[network](collectors/network/README.md)** | TLS/SSH 핸드셰이크 협상 그룹 → 통신 엣지 | AF_PACKET 수동 캡처(Linux), 복호화 없음 |

★ 정적 스캔으론 불가능한 **동적 등록 provider**(런타임 `addProvider`된 BouncyCastle 등)를 jvm attach가 잡는 게 이 단계의 **킬러 기능**이다 — 전용 OSS가 없던 공백이다.

## 간단히 써보기

**한 노드를 그 자리에서** — 설치할 것도, Ansible도 필요 없다.

```bash
pqcota-nodescan --output table            # 화면에 표로 (쌓이지 않는다)
pqcota-nodescan node-01 > result.json     # JSON으로 (중앙에 쌓을 때)
```

**여러 노드를** — 접속 정보를 적고, 참조 플레이북으로 한꺼번에 돌린다.

```bash
pqcota-hosts --ansible-out targets.ini hosts.csv
ansible-playbook -i targets.ini discovery/ansible/discover.yml
pqcota-ingest ./results                   # 회수한 결과를 인벤토리에 적재
```

커맨드별 인자·권한·환경변수는 [discovery/cmd](cmd/README.md).

## 잘 안 될 때 — 증상과 원인

| 증상 | 원인 |
|---|---|
| `/proc를 열 수 없어 관측하지 못했다` | 리눅스가 아니다. 결과는 **빈 것이 아니라 갭**으로 나간다 |
| 자산이 자기 프로세스 것만 잡힌다 | root가 아니다 — 다른 사용자의 `/proc`은 못 본다(못 본 만큼 갭으로 고지) |
| `CAP_NET_RAW 없음 — 관측하지 못했다` | `setcap cap_net_raw+ep` 또는 root. 종료코드는 0이다 — 갭을 중앙까지 보내기 위해서다 |
| JVM provider가 **정적 체인만** 보인다 | attach가 막혀 폴백했다(`DisableAttachMechanism`·JEP 451·권한). 실행 중 등록된 provider는 사각이라 갭으로 고지된다 |
| 관측 엣지가 0이다 | 관측 창 동안 핸드셰이크가 흐르지 않았다. 유휴 링크는 실환경에서도 안 보인다 — **부재가 아니라 미관측**이다 |

## 더 알아야 한다면

collector가 어떻게 파싱하고 어디까지 열화되는지, 정규화 6단계·완전성 맵·스코프 게이트가 무엇인지 → **[디스커버리 설계](디스커버리_설계.md)**.

## 이 폴더

- [`collectors/`](collectors) — collector 구현 (openssl·jvm·network)
- [`cmd/`](cmd) — 접근 준비·collector 실행 진입점 → [커맨드 지도](cmd/README.md)
- [`ansible/`](ansible) — 준비된 노드들에서 collector를 한꺼번에 돌리는 **참조 플레이북**
- **설계 문서**: [디스커버리 설계](디스커버리_설계.md) · [collector 배포](collector_배포_설계.md) · [테스트케이스](디스커버리_테스트케이스.md)

## 더 보기

프로세스 규정서 §2 · [아키텍처 설계](../docs/PQC플랫폼_아키텍처_및_OSS경계_설계.md) · 정규화·히스토리 라이브러리 [`pkg/discovery/`](../pkg/discovery) · 실행 예제 [`examples/discovery/`](../examples/discovery)
