#!/usr/bin/env bash
# examples/discovery — 접근 준비(hosts→Ansible·엔드포인트)와 결과 적재를 실제로 돌려본다.
# 전제: Go 툴체인만. Postgres·타깃 노드 불필요(적재는 인메모리 요약 모드).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cd "$ROOT"

echo "▶ 1) pqcota-hosts — 사용자 hosts.csv → 런타임 Ansible 인벤토리(비밀 포함) + 안전 엔드포인트(비밀 제외)"
go run ./discovery/cmd/pqcota-hosts --ansible-out "$TMP/targets.ini" "$HERE/hosts.csv"
echo
echo "   생성된 Ansible 인벤토리(런타임 전용·0600 — 접속 키가 실림, pqcota 인벤토리엔 미영속):"
sed 's/^/     /' "$TMP/targets.ini"

echo
echo "▶ 2) pqcota-ingest — [① 직접 관측] 회수된 CollectionResult(../data/results)를 스코프 게이트→정규화→적재"
echo "   (PQCOTA_DSN 없으면 인메모리 요약. 영속하려면 Postgres DSN을 export)"
go run ./inventory/cmd/pqcota-ingest "$ROOT/examples/data/results"

echo
echo "✅ 접근 준비 + collector 결과 적재를 돌려봤다."
echo "   • 실제 노드 스캔(리눅스): go run ./discovery/cmd/pqcota-nodescan <node-id>  (/proc의 OpenSSL 관측)"
echo "   • 외부 CBOM 수신·조회: examples/inventory"
