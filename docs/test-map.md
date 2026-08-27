# 테스트 명세: 무엇이 얼마나 검증돼 있나

케이스는 단계별 명세에 있고 **케이스 번호가 곧 테스트 파일 링크**다. 이 문서는 그 셋을 합쳐 얼마나 덮여 있는지만 본다.

> **§ 표기**: 별도 언급이 없으면 [규정서](regulation.md)의 절 번호다.

찾는 것이 정해져 있다면 바로 간다. [커널](kernel-testcases.md) · [디스커버리](../discovery/testcases.md) · [인벤토리](../inventory/testcases.md) · [프로비저닝](../provisioning/testcases.md) · [데모가 검증하는 것](../demo/integration-verification.md).

## 레벨 분포

| 레벨 | 수 | 어디서 도나 |
|---|---|---|
| **unit** | 222 | 어디서나. 입력은 테스트 내부 상수 |
| **integration** | 11 | **리눅스** 필요(`//go:build linux`): 실 crypto/tls·실물 sshd·`CAP_NET_RAW`·실 JVM·Postgres |
| **e2e** | 1 | 리눅스 + Docker: [데모](../demo/integration-verification.md). Go 테스트가 아니라 데모 6단계다 |

케이스는 **테스트 함수 단위**다. 한 항목이 한 테스트(또는 한 테이블 테스트)에 대응한다. 그래서 테스트가 없는 항목은 감출 곳이 없다.

위 수는 리포에서 센 값이고, **`check-docs`가 그 말이 참인지 대신 센다.** unit과 integration이 어긋나면 게이트가 막는다. 다만 **태그로 세는 근사치라 케이스 표의 등급 라벨과 한 건씩 맞지는 않는다.** 어느 쪽으로 어긋나는지는 [디스커버리 테스트케이스 §0](../discovery/testcases.md#0-테스트-레벨)에 적어 두었다. e2e만은 세지 않는데, Go 테스트가 아니라 데모 6단계라서 셀 대상이 없기 때문이다. 직접 확인하려면 아래를 돌린다:

```bash
grep -rh '^func Test' --include='*_test.go' . | wc -l          # 전체
grep -rl '//go:build linux' --include='*_test.go' . | xargs grep -c '^func Test'   # integration
```

일부는 환경이 갖춰졌을 때만 돈다. `PQCOTA_TEST_DSN`이 없으면 Postgres 케이스(TV-RETENTION-8·**TV-ORG-4**·**TV-ATTR-7·8**·TP-RECORD-3)가, JDK가 없으면 TD-JVM-9가, `CAP_NET_RAW`가 없으면 TD-NETWORK-15가 스킵된다. **스킵은 통과가 아니다.**

**TD-NETWORK-15와 TD-NETWORK-16은 서로 배타적이다.** 15는 소켓이 열려야 「구간이 끝까지 채워지는가」를 재고, 16은 소켓이 안 열려야 「권한이 없을 때 갭으로 강등하는가」를 잰다. 한 번 돌려서 둘 다 재는 환경은 없다. 그래서 CI는 권한 없는 러너로 전체를 한 번 돌려 16을 재고, 15만 `sudo`로 다시 돌린다. 그 두 번째 실행은 스킵을 통과로 세지 않도록 **PASS가 찍혔는지까지 확인한다.**

CI는 Postgres 서비스와 JDK도 붙여 나머지 케이스를 실제로 돌리고, 그러고도 건너뛴 것이 있으면 이름을 로그에 남긴다. 손에서 `go test`만 돌리면 DSN이 없어 Postgres 케이스가 스킵된다.

## 코드는 어디에

**`_test.go`는 대상 코드와 같은 디렉터리에 있다**. Go에서 그래야 그 패키지의 테스트가 된다. 별도 테스트 트리를 두지 않는 이유다.

대부분은 **외부 테스트 패키지**(`package foo_test`)라 공개 API로만 돈다. 공개 표면만으로 실제로 쓸 수 있는지가 함께 확인된다. 비공개 로직을 직접 봐야 하는 여섯 파일만 내부(`package foo`)로 남았다: `parseDetectionMethod` · `mergeByPath` · `machineKey` · `edgeFor` 메서드 · JVM 정찰·attach 파서 둘.

케이스와 테스트는 **한 방향으로** 이어져 있다. 케이스 표의 ID가 테스트 파일을 링크하고 테스트 함수 이름까지
적는다(예: [TD-OPENSSL-1](../discovery/testcases.md)이 `procmaps_test.go`의 `TestParseProcMaps`를 가리킨다).
역방향(테스트 파일이 자기 케이스 번호를 다는 것)은 **아직 없고, 자동 검사도 없다.** 즉 이 리포가 커버리지를
주장하는 근거 중 이 대응만 사람 손에 남아 있다. [검토 중](under-review.md)에 올려 둔 과제다.

## 돌리는 법

```bash
make test
```

`integration`은 리눅스에서만 컴파일된다(collector 핵심이 `//go:build linux`). 한 케이스만 보려면 이름으로 좁힌다.

```bash
go test ./pkg/discovery/normalize/ -run TestEvidenceStrength -v
```

종단(생성 → 적용 → 되돌림)은 Docker로 돈다.

```bash
./demo/scripts/up.sh && ./demo/scripts/demo.sh
```
