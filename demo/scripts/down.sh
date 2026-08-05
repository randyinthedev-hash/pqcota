#!/usr/bin/env bash
# pqcota 검증 데모 — 제거(down): 컨테이너·네트워크 정리. --rmi로 이미지까지 삭제.
set -euo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DEMO_DIR"

RMI=""
[ "${1:-}" = "--rmi" ] && RMI="--rmi local"
GEN="$DEMO_DIR/.generated"

echo "▶ 컨테이너·네트워크 제거…"
if [ -f "$GEN/docker-compose.yml" ]; then
  docker compose -f "$GEN/docker-compose.yml" down -v $RMI --remove-orphans
else
  # 생성물이 이미 지워졌으면 라벨로 남은 것을 정리한다(up.sh를 안 돌렸거나 수동 삭제한 경우).
  ids=$(docker ps -aq --filter label=pqcota-demo)
  [ -n "$ids" ] && docker rm -f $ids >/dev/null
  docker network ls -q --filter name=pqcota-topo- | xargs -r docker network rm >/dev/null 2>&1 || true
  echo "   (생성물이 없어 라벨로 정리했습니다)"
fi

# 리포 산출물 정리 — 전부 .generated/ 아래라 한 번에 지운다(그림·생성된 토폴로지).
rm -rf "$GEN"

echo "✅ 정리 완료. (이미지까지 지우려면: ./demo/scripts/down.sh --rmi)"
