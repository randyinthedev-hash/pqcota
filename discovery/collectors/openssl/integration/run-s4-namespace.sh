#!/usr/bin/env bash
# SD-4 컨테이너: collector가 별도 컨테이너의 프로세스를 PID 네임스페이스 공유로 탐지(사이드카/hostPID).
# 양성: --pid=container:target 공유 시 교차 탐지. 음성: 공유 없으면 관측하지 못함 → 갭.
# 전제: pqcota-test/openssl-collector-it 이미지 존재(discovery/collectors/openssl/integration/run.sh로 빌드).
set -u
IMG="pqcota-test/openssl-collector-it:latest"
TARGET="pqcota-s4-target"
RC=0

cleanup() { docker rm -f "$TARGET" >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "########## 대상 컨테이너 기동 (python3+libssl, PID 1) ##########"
docker rm -f "$TARGET" >/dev/null 2>&1 || true
docker run -d --label pqcota-test --name "$TARGET" "$IMG" \
  python3 -c "import ssl, time; time.sleep(999)" >/dev/null
sleep 2

echo "########## 양성: 네임스페이스 공유 사이드카 탐지 (TD-CONTAINER-1) ##########"
# 공유 네임스페이스에서 대상 python은 PID 1.
OUT=$(docker run --rm --label pqcota-test --pid="container:$TARGET" "$IMG" \
  /usr/local/bin/openssl-collector 1)
echo "$OUT"
if echo "$OUT" | grep -q "lib=libssl" && echo "$OUT" | grep -q "fork=OpenSSL"; then
  echo "PASS(TD-CONTAINER-1): 네임스페이스 공유로 타 컨테이너 프로세스의 libssl 교차 탐지"
else
  echo "FAIL(TD-CONTAINER-1)"; RC=1
fi

echo
echo "########## 음성: 네임스페이스 분리 → 관측하지 못함 = 갭 (TD-CONTAINER-2) ##########"
# 공유 없이 자기 PID 1(대상 아님)을 보면 libssl 없음 → '원리상 관측하지 못함'(부재 아님).
OUT2=$(docker run --rm --label pqcota-test "$IMG" /usr/local/bin/openssl-collector 1)
echo "$OUT2"
if echo "$OUT2" | grep -q "no OpenSSL"; then
  echo "PASS(TD-CONTAINER-2): 네임스페이스 분리 시 미탐지 → 완전성 맵 갭으로 처리(부재로 오판 금지)"
else
  echo "FAIL(TD-CONTAINER-2)"; RC=1
fi

exit $RC
