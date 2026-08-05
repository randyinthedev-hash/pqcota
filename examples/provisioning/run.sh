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
CASES='00-basic-two-actions|한 계획에 노드 둘(OpenSSL + JCA) — 노드별 play로 갈린다
openssl-3.5-config-only|OpenSSL 3.5+ 네이티브: config만, provider 모듈 없음
openssl-3.0-provider-inject|OpenSSL 3.0–3.4: 버전 유지 + provider 모듈 주입
openssl-1.1.1-fork-replace|OpenSSL 1.1.1: provider API 없음 → 생성 불가, 수동 단계로 표기
jca-native-config-only|JDK 네이티브 PQC: namedGroups만, provider 무등록
jca-provider-inject-bc|JCA: BouncyCastle 주입 — 클래스명 자동 확정
jca-fips-bcfips|규제 자산: BC-FJA(FIPS) 라우팅 — 등록 클래스가 달라진다
jca-eol-jdk-upgrade|EOL JDK: config로 불가 → 수동 단계로 표기
custom-openssl-provider|커스텀 OpenSSL provider: 절대 경로·provider별 소스 변수·sha256
custom-jca-provider|커스텀 JCA provider: providerClass로 FQCN 명시 → 계획만으로 완결
custom-jca-missing-class|같은 것에서 providerClass만 뺐을 때 — placeholder + 안내
signature-algorithm|서명 알고리즘(ML-DSA): KEM 그룹이 아니라 그룹 줄이 주석으로
l3-activation-hooks|L3: 계획의 훅을 의미 순서로 — pre → 배치 → activate → restart (롤백은 역순)|l3
l3-hooks-missing|L3인데 훅이 없을 때: 명령을 지어내지 않고 무엇이 안 일어나는지 고지|l3'

list() {
  echo "케이스 (plans/<이름>.json):"
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
    echo "그런 케이스 없음: $name" >&2
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
    echo "기본 케이스를 돌려본다 — 다른 케이스는 ./run.sh <이름>"
    echo
    run_case 00-basic-two-actions
    echo "✅ 전 케이스: ./run.sh --all · 역방향: ./run.sh <케이스> --rollback"
    echo "   • before 캡처 + 롤백 레코드 영속: --dsn <postgres> 추가(디스커버리가 먼저 적재돼 있어야 함 — demo/ 참고)"
    echo "   • status를 PLAN_STATUS_DRAFT로 바꾸면 §3.7 게이트가 거부한다."
    ;;
  *)
    run_case "$@"
    ;;
esac
