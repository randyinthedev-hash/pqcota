#!/usr/bin/env bash
# 컨테이너 내부에서 실행: 대상 JVM 기동 → attach → getProviders() 포착 → 검증.
set -e
cd /poc

java -cp .:bcprov.jar ProviderApp &
APP_PID=$!
sleep 2
echo "[test] target JVM pid=$APP_PID"

# 별도 JVM에서 attach (jdk.attach 모듈). 동일 UID(root) 이므로 attach 허용.
java --add-modules jdk.attach -cp . Attacher "$APP_PID" /poc/agent.jar /tmp/pqcota-providers.txt
sleep 1

echo "=== 포착된 provider 체인 (실체) ==="
cat /tmp/pqcota-providers.txt

echo "=== assertion (SD-2 핵심) ==="
if grep -q '|BC|' /tmp/pqcota-providers.txt; then
  echo "PASS: 런타임 동적 등록된 BouncyCastle(BC)을 attach로 포착 — 정적 스캔으론 불가한 실체 확인"
  RC=0
else
  echo "FAIL: BC 미포착"
  RC=1
fi
kill "$APP_PID" 2>/dev/null || true
exit $RC
