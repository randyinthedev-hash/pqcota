#!/usr/bin/env bash
# 관측 구간을 채울 핸드셰이크를 생성한다. 인자: 대상 목록(타입:host:port).
#   pqc:node-app:8443   → Go TLS X25519MLKEM768 (🟢)
#   ssl:node-db:4433    → OpenSSL 고전 TLS (🔴)
#   ssh:node-db:22      → SSH KEXINIT (🟢 sntrup761x25519)
# 여러 번 반복해 수집 구간 안에 확실히 들어가게 한다.
set -u
ROUNDS="${TRAFFIC_ROUNDS:-4}"

# 캡처(비동기 netcap)가 AF_PACKET 바인딩을 마칠 시간을 준다 — 트래픽이 관측 구간 안에 들도록.
sleep "${TRAFFIC_WARMUP:-3}"

gen_one() {
  local kind="$1" host="$2" port="$3"
  case "$kind" in
    pqc) pqc-echo client "${host}:${port}" 2 >/dev/null 2>&1 || true ;;
    ssl) echo Q | timeout 4 openssl s_client -connect "${host}:${port}" -quiet >/dev/null 2>&1 || true ;;
    ssh) timeout 4 ssh -o StrictHostKeyChecking=no -o BatchMode=yes -o ConnectTimeout=2 \
           "root@${host}" -p "${port}" true >/dev/null 2>&1 || true ;;
    *) echo "unknown traffic kind: $kind" >&2 ;;
  esac
}

for ((r = 0; r < ROUNDS; r++)); do
  for spec in "$@"; do
    IFS=: read -r kind host port <<<"$spec"
    gen_one "$kind" "$host" "${port:-443}"
  done
  sleep 1
done
echo "[traffic] 완료 (rounds=$ROUNDS, targets=$*)"
