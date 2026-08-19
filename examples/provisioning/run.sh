#!/usr/bin/env bash
# examples/provisioning — 확정 계획(plans/*.json)에서 Ansible 플레이북을 생성한다.
#
#   ./run.sh                        케이스 목록 + 기본 케이스 실행
#   ./run.sh <케이스>                그 케이스만
#   ./run.sh <케이스> --rollback     역방향(제거) 플레이북
#   ./run.sh --all                  전 케이스 순서대로
#
# 전제: Go 툴체인만. Postgres 없으면 플레이북만(before 캡처·레코드 영속은 --dsn 필요).
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT"

# 케이스 = plans/<이름>.json | 무엇을 보여주는가 | (선택)배포 수준. 수준은 케이스의 성질이라
# 읽는 사람이 플래그를 맞혀야 하지 않게 여기 적는다(기본 l2).
CASES='00-basic-two-actions|two nodes in one plan (OpenSSL + JCA) — split into a play per node
openssl-3.5-config-only|OpenSSL 3.5+ native: config only, no provider module
openssl-3.0-provider-inject|OpenSSL 3.0-3.4: keep the version, inject a provider module
openssl-1.1.1-fork-replace|OpenSSL 1.1.1: no provider API → nothing is generated, recorded as a manual step
jca-native-config-only|JDK-native PQC: namedGroups only, no provider registered
jca-provider-inject-bc|JCA: BouncyCastle injection — the class name is resolved automatically
jca-fips-bcfips|regulated asset: routed to BC-FJA (FIPS) — a different registration class
jca-eol-jdk-upgrade|end-of-life JDK: impossible through config → recorded as a manual step
custom-openssl-provider|custom OpenSSL provider: absolute path, per-provider source variable, sha256
custom-jca-provider|custom JCA provider: the FQCN is named in providerClass → the plan alone is enough
custom-jca-missing-class|the same, with providerClass removed — a placeholder plus guidance
signature-algorithm|signature algorithm (ML-DSA): not a KEM group, so the group line is commented out
l3-activation-hooks|L3: the plan's hooks in meaningful order — pre → stage → activate → restart (rollback reverses it)|l3
l3-hooks-missing|L3 without hooks: no command is invented; it reports what will not happen|l3'

list() {
  echo "cases (plans/<name>.json):"
  printf '%s\n' "$CASES" | while IFS='|' read -r name desc lvl; do
    printf "  %-28s %s%s\n" "$name" "$desc" "${lvl:+  [--level $lvl]}"
  done
}

desc_of() { printf '%s\n' "$CASES" | awk -F'|' -v n="$1" '$1==n{print $2}'; }
# 케이스가 수준을 정한다 — 안 적혀 있으면 l2.
level_of() { printf '%s\n' "$CASES" | awk -F'|' -v n="$1" '$1==n{print ($3==""?"l2":$3)}'; }

run_case() {
  local name="$1"; shift
  local file="$HERE/plans/$name.json"
  if [ ! -f "$file" ]; then
    echo "no such case: $name" >&2
    list >&2
    exit 2
  fi
  echo "════════════════════════════════════════════════════════════════"
  echo "▶ $name"
  echo "  $(desc_of "$name")"
  echo "════════════════════════════════════════════════════════════════"
  go run ./provisioning/cmd/pqcota-provision --level "$(level_of "$name")" "$@" "$file"
  echo
}

case "${1:-}" in
  --all)
    printf '%s\n' "$CASES" | while IFS='|' read -r name _; do run_case "$name"; done
    ;;
  "")
    list
    echo
    echo "running the default case — for others: ./run.sh <name>"
    echo
    run_case 00-basic-two-actions
    echo "✅ all cases: ./run.sh --all · reverse: ./run.sh <case> --rollback"
    echo "   • before capture + persisted rollback record: add --dsn <postgres> (discovery must have ingested first — see demo/)"
    echo "   • change status to PLAN_STATUS_DRAFT and the §3.7 gate refuses it."
    ;;
  *)
    run_case "$@"
    ;;
esac
