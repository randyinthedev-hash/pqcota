#!/usr/bin/env bash
# 컨테이너 내부: libssl 로드한 장수 프로세스 기동 → collector가 /proc·ELF로 탐지 → 검증.
set -e

python3 -c "import ssl, time; print('[app] ssl loaded:', ssl.OPENSSL_VERSION); time.sleep(999)" &
PID=$!
sleep 2
echo "[test] target pid=$PID (python3 + libssl)"

echo "=== openssl-collector 출력 ==="
OUT=$(/usr/local/bin/openssl-collector "$PID")
echo "$OUT"

echo "=== assertion (SD-1+SD-3 실물) ==="
if echo "$OUT" | grep -q "lib=libssl" && echo "$OUT" | grep -q "fork=OpenSSL"; then
  echo "PASS: 실행 중 프로세스의 libssl 로드 탐지 + ELF 문자열로 fork=OpenSSL 판별"
  RC=0
else
  echo "FAIL: 탐지/판별 실패"
  RC=1
fi
kill "$PID" 2>/dev/null || true
exit $RC
