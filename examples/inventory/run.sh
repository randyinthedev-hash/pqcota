#!/usr/bin/env bash
# examples/inventory — 회수된 CollectionResult를 읽기전용 인벤토리 뷰로 조회한다(자산·앱 표시·등급).
# 전제: Go 툴체인만. 저장소·타깃 노드 불필요(파일 취합 모드).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cd "$ROOT"

echo "▶ pqcota-discover-view — ../data/results 취합 → 발견 자산 + 관측 엣지 등급 + 토폴로지 DOT"
echo "   (nodes.json으로 관측 IP→노드명 해소: 10.0.0.9 → node-c)"
echo
go run ./inventory/cmd/pqcota-discover-view \
    "$ROOT/examples/data/results" \
    "$ROOT/examples/data/nodes.json" \
    "$TMP/topology.dot"

echo
echo "생성된 토폴로지 DOT(색=등급) 앞부분:"
head -6 "$TMP/topology.dot" | sed 's/^/   /'
echo
echo "▶ pqcota-cbom-ingest — 외부 도구가 낸 CBOM을 검증·적재"
echo "   (collector가 관측하지 않는 소스·빌드 아티팩트를, 사용자 CI가 낸 표준 CycloneDX로 받는다)"
go run ./inventory/cmd/pqcota-cbom-ingest "$HERE/sample-cbom.json" node-b

echo
echo "✅ 파일 취합 뷰(휘발성)와 외부 CBOM 수신을 돌려봤다."
echo "   • 중앙 영속 조회(엔드포인트·프로필 헤더 포함): PQCOTA_DSN=<postgres> go run ./inventory/cmd/pqcota-inventory"
echo "     (먼저 pqcota-ingest로 같은 DSN에 적재해야 함 — demo/ 참고)"
