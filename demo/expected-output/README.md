# 예상 결과 (샘플)

이 데모(`../scripts/up.sh` → `demo.sh`)를 실행하면 나오는 **대표 결과**입니다. 실행 전에 무엇을 보게 될지
미리 확인하세요. (완전 관측된 warm 실행을 캡처한 것.) 데모는 **접근준비→SSH 확인→디스커버리→뷰→토폴로지→인벤토리→프로비저닝** 6단계이며,
아래 샘플은 그 중 **디스커버리 뷰**(`3/6`)를 캡처한 것입니다.

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/플랫폼_규정.md)의 절 번호다.

| 파일 | 내용 |
|---|---|
| [discover-view.txt](discover-view.txt) | 콘솔 출력 — 발견 자산(OpenSSL·JCA/BouncyCastle) + 관측 통신 엣지 등급 |
| [topology.svg](topology.svg) | 관측 토폴로지 (색=등급: 🟢 PQC / 🔴 고전 / ⚪ 불명, 실선=관측) |

**핵심 서사 — 현대 스택과 레거시가 TLS·SSH 양쪽에서 갈린다:**

| 엣지 | 등급 | 왜 |
|---|---|---|
| `web-gw→pay-app` TLS | 🟢 X25519MLKEM768 | Go `crypto/tls` 하이브리드 |
| `web-gw→pay-app` SSH | 🟢 sntrup761 | 양쪽 다 OpenSSH 9+ |
| `web-gw→pay-db` TLS | 🔴 x25519 | **OpenSSL 1.1.1**엔 PQC 그룹이 없다 |
| `web-gw→pay-db` SSH | 🔴 curve25519 | 레거시 OS의 **OpenSSH 8.2**엔 PQC KEX가 없다 |

그리고 `pay-app`에서 **BouncyCastle 포함 JCA provider 체인**(런타임 `addProvider` — 정적 스캔으론 안 보이는 것)을
attach로 관측합니다. 구성은 [topology/topology.yaml](../topology/README.md)이 정의하며, 고치면 이 결과도 바뀝니다.

> **등급은 관측 결과이지 설정이 아닙니다.** SSH 등급은 **양쪽 KEXINIT의 교집합**(RFC 4253)으로 계산합니다 —
> 클라이언트가 sntrup761을 제안해도 서버가 지원하지 않으면 🔴입니다. 한쪽만 관측되면 협상을 지어내지 않고
> ⚪ 불명으로 둡니다(§2.6).

이후 단계에서 추가로 보게 되는 것:
- **접근준비(0)** — `pqcota-hosts`가 hosts.csv→Ansible 인벤토리(접속 키·런타임 전용) + 엔드포인트 upsert. 인벤토리엔 **비밀 0건**.
- **중앙 인벤토리(5)** — `▸ 결제 DB (ip:22) │ 결제 DB · production · db · owner=DBA팀` 처럼 **엔드포인트·프로필 헤더** + `@앱` 귀속. pay-db의 공유 `libssl.so.1.1`은 `@/opt/apps/api-gw,/opt/apps/payment-gw` **두 앱 동시 귀속**.
- **프로비저닝(6)** — 확정 계획→L2 플레이북 + 롤백 레코드: `영향앱=/opt/apps/api-gw,/opt/apps/payment-gw · before=["libssl.so.1.1@1.1.1f"]`.

## 실제 실행 시 달라질 수 있는 점 (그리고 이유)

결정론을 위해 두 장치를 넣어서 **첫 실행에서도 위 결과와 일치**합니다:
- **엣지 캡처 retry-until-complete**: `demo.sh`가 목표 엣지 수(토폴로지의 엣지 개수)에 도달할 때까지 재수집(최대 4회).
  netcap 창–트래픽 타이밍 경쟁을 흡수 → 콜드 스타트 첫 실행도 완전한 엣지를 냅니다.
  (`DEMO_TARGET_EDGES`/`DEMO_MAX_ATTEMPTS`로 조정. 극히 제약된 환경에서 최대 시도 내 미달 시에만 엣지가 줄어듦.)
- **base 이미지 고정** + BouncyCastle `1.85` 고정(해시 확인) → OpenSSL/Go/JDK 버전·`1.1.1f`/`3.0.13`/`3.5.5` 같은
  문자열이 태그 갱신에도 대체로 **동일**합니다(배포판 보안 업데이트 시 마이너 갱신 가능).

여전히 다를 수 있는 것:
- **컨테이너 IP**(172.18.0.x): 매 실행 동적 — 서사엔 무관(노드명 해소).
- **토폴로지를 고치면** 노드·엣지·등급이 그대로 달라집니다 — 이 샘플은 **기본 구성** 기준입니다.

## 결정론

동일 입력이면 동일 출력입니다: 등급 분류(🟢/🔴/⚪), 발견 자산, (별도 확장의) reconcile 3-상태 모두
결정론적 로직입니다. 위 "차이"는 **무엇이 관측되는가(캡처 타이밍)** 와 **base 이미지 버전**의 문제이지,
분류·판정 로직의 비결정성이 아닙니다.
