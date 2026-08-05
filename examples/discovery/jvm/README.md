# examples/discovery/jvm — 실행 중 JVM 정찰→attach

```bash
./examples/discovery/jvm/run.sh
```

> **전제: Go 툴체인 + Docker.** 다른 discovery 예제는 Go만으로 도는데, 이건 **살아있는 JVM**이 필요해(정찰·attach 대상) 컨테이너로 격리했다.

## 무엇을 보이나

openssl collector가 `/proc`를 훑어 로드된 lib를 스스로 찾듯, **jvm-collector도 실행 중 JVM을 직접 정찰**한다. 이 예제는 그 종단을 최소로 보인다:

1. **정찰**(`ScanJVMs`) — `/proc`로 실행 중 JVM을 찾아 PID·JAVA_HOME·버전·앱을 뽑는다. 호출자가 PID·JDK를 미리 몰라도 된다.
2. **attach** — 발견한 PID에 붙어 `Security.getProviders()` **실체**를 관측한다.
3. **동적 등록 포착** — 예제 앱은 `java.security`에 BC를 **정적으로 안 심고** 런타임에 `addProvider(BouncyCastle)`만 한다. **정적 스캔(프로브)으론 이걸 못 본다 — attach만 잡는다**(`detection=runtime-introspection`).
4. **JSON Lines 적재** — attach 경로는 JVM별로 한 줄씩 낸다(다중 JVM 대비). `pqcota-ingest`가 `*.jsonl`을 읽어 적재한다.

## 핵심 — 프로브 vs attach

| | 정적 프로브 | **attach** |
|---|---|---|
| 보는 것 | `java.security` 정적 등록 체인 | 실행 중 JVM의 **실체**(동적 `addProvider` 포함) |
| 이 예제의 동적 BC | ❌ 못 봄 | ✅ 포착 |

`PQCOTA_JVM_AGENT`(collector JAR)가 있으면 attach, 없으면 프로브로 정직히 폴백한다.

## 다중 JVM

한 노드에 JVM이 여럿이면 각각 **구별되는 finding**이 된다 — 식별자는 **앱**(main 클래스·`-jar`)이라(PID 아님) 같은 JDK의 두 앱도 안 뭉개지고, 재스캔해도 이력이 안 깨진다. 설계·경계: [collectors/jvm/README](../../../discovery/collectors/jvm/README.md).

## 전체 종단

Ansible/SSH·다중 노드·Postgres 적재까지는 [demo/](../../../demo) 6단계(2번째 스텝이 이 정찰→attach를 실제 노드에서 돌린다).
