#!/usr/bin/env bash
# 노드에서 네트워크 관측(netcap)과 핸드셰이크 생성을 co-locate 한다 — 순서를 보장해
# "캡처 시작 ↔ 트래픽" 경쟁을 없앤다. netcap 결과(JSON)를 stdout으로 낸다.
# usage: pqcota-observe.sh <node> <window-sec> [traffic-target...]
set -u
NODE="$1"
WINDOW="$2"
shift 2

OUT="$(mktemp)"
pqcota-netcap "$NODE" "${NETCAP_IFACE:-eth0}" "$WINDOW" >"$OUT" 2>/tmp/netcap.err &
CAP=$!

sleep 2 # AF_PACKET 바인딩이 끝나길 기다린 뒤 트래픽 시작

if [ "$#" -gt 0 ]; then
  END=$((SECONDS + WINDOW - 3))
  while [ "$SECONDS" -lt "$END" ]; do
    TRAFFIC_WARMUP=0 TRAFFIC_ROUNDS=1 /usr/local/bin/pqcota-gen-traffic.sh "$@" >/dev/null 2>&1 || true
  done
fi

wait "$CAP"
cat "$OUT"
