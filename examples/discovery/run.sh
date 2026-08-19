#!/usr/bin/env bash
# examples/discovery — 접근 준비(hosts→Ansible·엔드포인트)와 결과 적재를 실제로 돌려본다.
# 전제: Go 툴체인만. Postgres·타깃 노드 불필요(적재는 인메모리 요약 모드).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cd "$ROOT"

echo "▶ 1) pqcota-hosts — your hosts.csv → runtime Ansible inventory (with secrets) + safe endpoints (without)"
go run ./discovery/cmd/pqcota-hosts --ansible-out "$TMP/targets.ini" "$HERE/hosts.csv"
echo
echo "   the generated Ansible inventory (runtime-only, 0600 — it carries the access key; never persisted in the pqcota inventory):"
sed 's/^/     /' "$TMP/targets.ini"

echo
echo "▶ 2) pqcota-ingest — [① direct observation] fetched CollectionResults (../data/results) through the scope gate → normalize → ingest"
echo "   (without PQCOTA_DSN it is an in-memory summary; export a Postgres DSN to persist)"
go run ./inventory/cmd/pqcota-ingest "$ROOT/examples/data/results"

echo
echo "✅ ran access prep and the ingest of collector results."
echo "   • scan a real node (Linux): go run ./discovery/cmd/pqcota-nodescan <node-id>  (observes OpenSSL through /proc)"
echo "   • external CBOM intake and query: examples/inventory"
