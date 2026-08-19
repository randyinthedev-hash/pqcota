#!/usr/bin/env bash
# examples/discovery/jvm — 실행 중 JVM을 **정찰→attach**해 JCA provider 체인(런타임 동적 등록 포함)을 관측한다.
#
# 다른 discovery 예제와 달리 **살아있는 JVM**이 필요해 격리했다. openssl collector가 /proc를
# 훑어 로드된 lib를 스스로 찾듯, jvm도 /proc로 실행 중 JVM을 정찰해 그 PID에 attach한다.
#
# 전제: **Go 툴체인 + Docker**(JDK 컨테이너). bcprov를 컨테이너가 한 번 내려받는다(해시 확인).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cd "$ROOT"

echo "▶ 1) Go 바이너리 빌드 — pqcota-jvmscan, pqcota-ingest"
GOOS=linux go build -o "$TMP/pqcota-jvmscan" ./discovery/cmd/pqcota-jvmscan
GOOS=linux go build -o "$TMP/pqcota-ingest"  ./inventory/cmd/pqcota-ingest

echo "▶ 2) JDK 컨테이너: 순수 Java collector.jar 빌드 + 동적 BC JVM 기동 + 정찰→attach"
docker run --rm -i \
  -v "$TMP":/x:ro \
  -v "$ROOT/discovery/collectors/jvm/collector":/c:ro \
  eclipse-temurin:21-jdk bash -s <<'INNER'
set -e
BC=/tmp/bcprov.jar
# 버전·해시 고정 — 받은 것이 기대한 것인지 확인한다(examples/provisioning/files와 같은 핀).
BC_VERSION=1.85
BC_SHA256=20af26bf6060bb8005cc2389916812c1e0e998dc48d2ced7131b89461b54cff7
curl -fsSL -o "$BC" "https://repo1.maven.org/maven2/org/bouncycastle/bcprov-jdk18on/${BC_VERSION}/bcprov-jdk18on-${BC_VERSION}.jar"
echo "${BC_SHA256}  ${BC}" | sha256sum -c -

# 순수 Java attach 사이드카(Kotlin·Gradle 없음) — collector 소스를 그대로 빌드.
javac -d /tmp/cls $(find /c/src/main/java -name '*.java')
jar cfm /tmp/collector.jar /c/manifest.mf -C /tmp/cls .

# java.security엔 BC를 안 심는다 — 앱이 런타임에 addProvider로만 등록(정적 스캔으론 불가).
cat > /tmp/App.java <<'JAVA'
import java.security.Security;
import org.bouncycastle.jce.provider.BouncyCastleProvider;
public class App {
  public static void main(String[] a) throws Exception {
    Security.addProvider(new BouncyCastleProvider());     // ← 런타임 동적 등록
    System.out.println("[app] BC 동적 등록, providers=" + Security.getProviders().length);
    while (true) Thread.sleep(5000);
  }
}
JAVA
java -cp "$BC" /tmp/App.java & sleep 3

echo
echo "── 정찰(ScanJVMs) + attach → CollectionResult(JSON Lines) ──"
PQCOTA_JVM_AGENT=/tmp/collector.jar /x/pqcota-jvmscan host://demo-jvm > /tmp/jca.jsonl 2>/tmp/err
grep -E 'found JVM|attach:' /tmp/err | sed 's/^/   /'

echo "── 적재(pqcota-ingest, JSONL) — 동적 BC가 관측됐나 ──"
mkdir -p /tmp/res && cp /tmp/jca.jsonl /tmp/res/
/x/pqcota-ingest /tmp/res 2>&1 | grep -E 'ingest result|•' | sed 's/^/   /'
# CBOM은 base64로 실려 있다. 이 이미지엔 python도 jq도 없으므로 **있는 것**으로 꺼낸다 —
# 없는 도구로 확인하면 검사가 조용히 죽는다(실제로 죽어 있었다: python3가 없어 이 줄이 늘 실패했다).
if grep -oE '"cbomCyclonedx": *"[^"]*"' /tmp/jca.jsonl | head -1 | cut -d'"' -f4 \
     | base64 -d 2>/dev/null | grep -q '"BC"\|,BC'; then
  echo "   ✅ 정적 java.security엔 없는 동적 BC를 attach로 포착 — 정찰→attach의 가치"
else
  echo "   ⚠ 동적 BC를 CBOM에서 확인하지 못했다 — attach가 실패했거나 출력 형식이 바뀌었다"
fi
INNER

echo
echo "✅ 정찰→attach 예제 완료. 프로브(정적)로는 이 동적 BC를 관측하지 못한다 — attach만 잡는다."
echo "   전체 종단(Ansible/SSH·다중 노드)은 demo/ 6단계가 보인다."
