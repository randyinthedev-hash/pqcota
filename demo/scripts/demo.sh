#!/usr/bin/env bash
# pqcota 데모 — 접근 준비(hosts→Ansible·엔드포인트) → 디스커버리 → 중앙 인벤토리(엔드포인트·
# 프로필·앱 귀속) → 프로비저닝 생성(플레이북 + before/롤백 레코드). 전부 이 리포 범위.
set -euo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DSN="postgres://postgres:pqcota@pqcota-demo-pg:5432/pqcota"

# 데모 구성은 up.sh가 topology.yaml에서 생성한 산출물(compose·groups·profiles·manifest)이 정의한다.
GEN="$DEMO_DIR/.generated"
[ -f "$GEN/manifest.env" ] || { echo "먼저 ./demo/scripts/up.sh 를 실행하세요(생성물이 없습니다)." >&2; exit 1; }
source "$GEN/manifest.env"  # NODES · EDGE_COUNT · HUMAN
ANS="cd /work/ansible && ansible"       # ansible.cfg 적용 위해 그 디렉토리에서 실행
INV="-i /work/ansible/targets.ini -i /work/ansible/groups.ini"
pg() { docker exec pqcota-demo-pg psql -U postgres -d pqcota "$@"; }

echo "▶ 0/6 접근 준비 — 사용자 hosts 파일 → Ansible 인벤토리(비밀 미영속) + 엔드포인트 인벤토리 upsert…"
for i in $(seq 1 20); do docker exec pqcota-demo-pg pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
# pqcota-hosts: (a) targets.ini에 접속 비밀(키) — 런타임 전용·0600, (b) 엔드포인트(비밀 제외) Postgres upsert.
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
  "pqcota-hosts --ansible-out /work/ansible/targets.ini --dsn '$DSN' /work/hosts.csv" | sed 's/^/   /'
echo "   ── 생성된 targets.ini: 접속 키가 실림(런타임 전용·미영속) ──"
docker exec pqcota-ctl bash -lc 'grep -m1 ansible_ssh_private_key_file /work/ansible/targets.ini' | sed 's/^/   /'
echo "   ── 인벤토리(Postgres) 엔드포인트: 비밀 없음(node_id·이름·ip·port만) ──"
pg -tAc "select '   '||node_id||'  '||(endpoint->>'name')||'  '||(endpoint->>'ip')||':'||(endpoint->>'port') from pqcota_endpoint order by node_id"
secret_ct=$(pg -tAc "select count(*) from pqcota_endpoint where endpoint::text ~* 'ssh|key|root|id_demo'")
echo "   → 엔드포인트에 접근 비밀 흔적: ${secret_ct}건 (0이어야 정상)"
# CMDB 프로필 선언 임포트(pqcota-profile — CMDB/리뷰어 레인, 관측 아님). 인벤토리 뷰 시각 구분.
# 프로필·그룹은 토폴로지에서 생성된 것을 쓴다(노드가 가변이므로).
docker cp "$GEN/profiles.csv" pqcota-ctl:/work/profiles.csv
docker cp "$GEN/groups.ini"   pqcota-ctl:/work/ansible/groups.ini
docker exec pqcota-ctl bash -lc "pqcota-profile --dsn '$DSN' /work/profiles.csv" >/dev/null 2>&1

echo "▶ 1/6 컨트롤러 → 타깃 SSH 연결 확인 (Ansible ping, pqcota-hosts 생성 인벤토리로)…"
docker exec pqcota-ctl bash -lc "$ANS $INV -m ping targets"

echo "▶ 2/6 디스커버리 실행 (OpenSSL /proc · JCA provider · 네트워크 핸드셰이크)…"
# 목표 엣지 수에 못 미치면 재수집한다. 관측 창 안에 트래픽이 안 흐를 수 있는 것은 실환경에서도
# 참이라(유휴 링크) 이 backstop은 남긴다. 다만 예전에 이 루프가 자주 돌던 진짜 이유는 타이밍이
# 아니라 **collector 결함**이었다 — 원시 syscall의 EINTR을 치명적으로 다뤄 관측 창이 무작위로
# 잘렸다(25초 창이 0·0·14·25초에 끝남). 고친 뒤로는 첫 시도에 다 잡힌다.
TARGET_EDGES="${DEMO_TARGET_EDGES:-${EDGE_COUNT:-3}}"
MAX_ATTEMPTS="${DEMO_MAX_ATTEMPTS:-4}"
edge_count() {
  docker exec pqcota-ctl bash -lc 'grep -oh srcNodeId /work/results/*-net.json 2>/dev/null | wc -l' | tr -d '[:space:]'
}
for attempt in $(seq 1 "$MAX_ATTEMPTS"); do
  docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover.yml" >/dev/null
  cnt="$(edge_count)"; cnt="${cnt:-0}"
  echo "   시도 $attempt/$MAX_ATTEMPTS — 관측 엣지 ${cnt}개 (목표 ${TARGET_EDGES}+)"
  if [ "$cnt" -ge "$TARGET_EDGES" ]; then break; fi
  if [ "$attempt" -lt "$MAX_ATTEMPTS" ]; then echo "   엣지 부족 → 재수집(컨테이너 warm)…"; fi
done

echo "▶ 3/6 읽기전용 디스커버리 뷰 (자산 + posture)…"
docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results /work/nodes.json /work/topology.dot'

echo "▶ 4/6 관측 토폴로지 SVG 렌더 + 회수…"
# 리포로 꺼내는 결과물은 전부 demo/.generated/ 아래로(일관성 — down.sh가 통째로 지운다).
mkdir -p "$GEN"
if docker exec pqcota-ctl bash -lc 'command -v dot >/dev/null && dot -Tsvg /work/topology.dot -o /work/topology.svg'; then
  docker cp pqcota-ctl:/work/topology.svg "$GEN/topology.svg"
  echo "   → $GEN/topology.svg"
fi
docker cp pqcota-ctl:/work/topology.dot "$GEN/topology.dot" 2>/dev/null || true

echo "▶ 5/6 중앙 인벤토리 적재·조회 (Postgres append-only · 엔드포인트·프로필·앱 귀속 · 이력·변화)…"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' | sed 's/^/   /'
echo "   ── 조회(pqcota-inventory) — ▸머신 헤더(엔드포인트·프로필) · @앱 귀속(공유 .so는 다중) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-inventory' \
  | grep -E '중앙 인벤토리|▸|@|합계' | sed 's/^/   /'

# 이력 — 같은 회수 결과를 한 번 더 적재한다(실운용의 "다음 회차 스캔"). 내용이 같으니
# 스냅샷은 늘지 않고 관측 기록만 쌓인다 — 저장은 변화 횟수만큼만 자라되 "봤다"는 사실은 남는다.
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' >/dev/null
# 엣지를 실제로 관측한 노드로 보인다(-snapshot의 핵심이 엣지라, 엣지 0건 노드면 볼 게 없다).
HNODE=$(pg -tAc "select node_id from pqcota_snapshots
  order by jsonb_array_length(coalesce(edges,'[]'::jsonb)) desc, node_id limit 1" | tr -d '[:space:]')
[ -z "$HNODE" ] && HNODE=$(pg -tAc "select node_id from pqcota_endpoint order by node_id limit 1" | tr -d '[:space:]')
echo "   ── 이력(pqcota-inventory -history $HNODE): 스냅샷은 변화가 있을 때만, obs는 재확인 횟수 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -history '$HNODE'" | sed 's/^/   /'
PRE_SNAP=$(pg -tAc "select id from pqcota_snapshots where node_id='$HNODE' order by seq desc limit 1" | tr -d '[:space:]')
echo "   ── 스냅샷 상세(-snapshot): 자산 + 그 스냅샷의 관측 엣지(누적 뷰는 합계만 낸다) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -snapshot '$PRE_SNAP'" | sed 's/^/   /'

# 자산 스코프 — 노드는 등재됐어도(§0.4) 그 안에서 계속 관리할 자산만 남긴다. 시스템 기본
# 라이브러리·패키지 런타임이 섞이면 인벤토리가 잡음에 묻힌다.
# 내용이 바뀌므로 새 스냅샷이 생긴다 → 바로 이게 -diff로 보일 "실제 변화"다.
docker exec -i pqcota-ctl bash -lc "cat > /work/scope-assets.csv" <<'CSV'
action,runtime,lib,app_key,note
exclude,*,*,/usr/sbin/sshd*,sshd는 OS 패치로 관리 — 지속 관측 불필요
exclude,*,*,/usr/bin/python*,패키지가 딸려 넣은 python 런타임
CSV
echo "   ── 자산 스코프(-scope-assets): 잡음(sshd·python 런타임)을 관리 대상에서 뺀다 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
  'pqcota-ingest -scope-assets /work/scope-assets.csv /work/results' | grep -E '자산 스코프|•' | sed 's/^/   /'
echo "   ── 제외 후 인벤토리: 앱이 실제로 쓰는 자산만 남는다 (제외 ≠ 부재 — 건수를 고지한다) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-inventory' \
  | grep -E '▸|openssl|jca|스코프 제외|합계' | sed 's/^/   /'

POST_SNAP=$(pg -tAc "select id from pqcota_snapshots where node_id='$HNODE' order by seq desc limit 1" | tr -d '[:space:]')
echo "   ── 변화(-diff): 스코프 적용 전후. '사라짐'은 자산이 없어진 게 아니라 관리 대상에서 뺐다는 뜻 ──"
if [ -n "$PRE_SNAP" ] && [ -n "$POST_SNAP" ] && [ "$PRE_SNAP" != "$POST_SNAP" ]; then
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -diff '$PRE_SNAP','$POST_SNAP'" \
    | grep -vE '^\s*$' | sed 's/^/   /'
else
  echo "   (이 노드는 스코프 적용 후에도 내용이 같아 새 스냅샷이 없다 — 비교할 두 지점이 없으면 diff도 없다)"
fi

# 보존 정책 — 파괴적 동작이라 조회 커맨드와 분리했고 기본이 dry-run이다.
echo "   ── 보존 정책(pqcota-prune, 기본 dry-run): 최신은 남기고 그 이전만 — 실제 삭제는 -apply로만 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-prune -keep-last 1' | sed 's/^/   /'

echo "▶ 6/6 프로비저닝 — 확정 계획 → L2/L3 플레이북 생성 → 적용 → 되돌림…"
# 프로비저닝 대상 노드(PNODE) — openssl finding이 있는 노드를 인벤토리에서 고른다(공유 .so로 다중
# 귀속된 쪽 우선 = 영향 반경이 가장 또렷한 케이스). 없으면 이 단계는 정직히 생략한다(§2.6).
PNODE=$(pg -tAc "select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where f ? 'openssl' order by jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb)) desc, s.node_id limit 1" | tr -d '[:space:]')
if [ -z "$PNODE" ]; then
  echo "   (openssl finding을 가진 노드가 없어 프로비저닝 시연은 생략 — 토폴로지에 openssl 서버를 두면 보인다)"
else
# 인벤토리에서 실제 finding을 골라 확정 계획을 만든다. 공유 libssl(다중 귀속) 우선, 없으면 아무 finding.
pick() { pg -tAc "select f->>'id' from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where s.seq=(select max(seq) from pqcota_snapshots where node_id='$PNODE') and ($1)
  order by jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb)) desc, f->>'id' limit 1" | tr -d '[:space:]'; }
FID=$(pick "f->'openssl'->>'lib'='libssl.so.3' and jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb))>=2")
[ -z "$FID" ] && FID=$(pick "jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb))>=2")
[ -z "$FID" ] && FID=$(pick "f ? 'openssl'")
echo "   대상 finding: $FID ($PNODE)"
docker exec -i pqcota-ctl bash -lc "cat > /work/plan.json" <<JSON
{"id":"plan-demo","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
 "approvalSignatures":["reviewer:demo"],
 "actions":[{"id":"a1","targetNodeId":"$PNODE","findingId":"$FID",
   "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL",
   "kind":"REMEDIATION_KIND_PROVIDER_INJECT","targetAlgorithm":"ML-KEM (FIPS 203)",
   "providerChoice":"oqsprovider","rollbackNote":"cnf 한 줄 + 모듈 제거"}]}
JSON
echo "   ── pqcota-provision: 확정 계획(§3.7 게이트) → L2 플레이북 생성 + before 캡처·레코드 영속 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --dsn '$DSN' /work/plan.json > /work/ansible/provision.yml" 2>&1 | sed 's/^/   /'
docker exec pqcota-ctl bash -lc 'grep -E "module = |dest:" /work/ansible/provision.yml' | sed 's/^/   │ /'
echo "   ── 영속된 롤백 레코드 조회(pqcota-records): 영향 앱(공유 .so면 다중) + before 상태 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-records $PNODE" | sed 's/^/   /'

# ── 생성물이 실제로 도는지까지 봐야 "생성했다"가 말이 된다. 생성만 하고 안 돌리면
# 깨끗한 노드에서 깨지는 플레이북도 통과해 버린다(실제로 그런 결함이 여럿 있었다).
echo
echo "   ══ 생성된 플레이북을 실제로 적용한다(ansible-playbook) ══"
# provider 모듈은 도구가 주지 않는다. 데모는 배포 경로만 보이려 빈 파일을 쓴다(암호 기능 없음).
docker exec pqcota-ctl bash -lc 'mkdir -p /work/ansible/files && : > /work/ansible/files/oqsprovider.so'
MOD_SHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')
echo "   ── 적용(ansible-playbook) — 모듈 sha256 게이트도 함께 ──"
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$MOD_SHA provision.yml" \
  | grep -E "TASK|ok=|changed=|failed=" | sed 's/^/   /'
echo "   ── 타깃 노드에 실제로 놓였나 ──"
docker exec "$PNODE" sh -lc 'ls -l /opt/pqcota/oqsprovider.so /etc/pqcota/openssl-pqc.cnf' | sed 's/^/   /'
docker exec "$PNODE" sh -lc 'grep "^module" /etc/pqcota/openssl-pqc.cnf' | sed 's/^/   config가 참조하는 경로: /'

echo "   ── 되돌리기(--rollback) — 원본을 덮은 적이 없으니 제거로 끝난다 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --rollback /work/plan.json > /work/ansible/provision-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc 'ls /opt/pqcota/oqsprovider.so /etc/pqcota/openssl-pqc.cnf 2>&1 || true' | sed 's/^/   /'

# ── L3(활성화·재시작) — L2가 놓기만 한 조각을 실제로 **참조되게** 만들고 서비스를 다시 띄운다.
# 활성화·재시작 방법은 환경마다 다르므로 도구가 추측하지 않는다(§2.6): 계획의 activation 훅에
# **사용자가 적은 명령**을 도구가 의미 순서로 배치할 뿐이다. 이 노드는 ssl-apps.sh로 서비스를
# 관리하므로 훅이 그것을 가리킨다 — 현실의 systemd unit·사내 기동 스크립트에 해당한다.
echo
echo "   ══ L3(활성화·재시작) — 계획의 훅을 의미 순서로: pre → 배치 → activate → restart ══"
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   before │ /'
PID_BEFORE=$(docker exec "$PNODE" sh -lc "pgrep -f 's_server -accept' | head -1" | tr -d '[:space:]')
docker exec -i pqcota-ctl bash -lc "cat > /work/plan-l3.json" <<JSON
{"id":"plan-demo-l3","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
 "approvalSignatures":["reviewer:demo"],
 "actions":[{"id":"a1","targetNodeId":"$PNODE","findingId":"$FID",
   "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL",
   "kind":"REMEDIATION_KIND_CONFIG_ONLY","targetAlgorithm":"ML-KEM (FIPS 203)",
   "rollbackNote":"활성화 지점 제거 + 재시작",
   "activation":{
     "pre":"/usr/local/bin/ssl-apps.sh stop",
     "activate":"printf 'OPENSSL_CONF=%s\\n' /etc/pqcota/openssl-pqc.cnf > /etc/pqcota/service.env",
     "deactivate":"rm -f /etc/pqcota/service.env",
     "restart":"/usr/local/bin/ssl-apps.sh start"}}]}
JSON
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 /work/plan-l3.json > /work/ansible/provision-l3.yml" 2>&1 | sed 's/^/   /'
echo "   ── 생성된 훅 태스크(순서가 곧 안전성: 내리고 → 바꾸고 → 켜고 → 재시작) ──"
docker exec pqcota-ctl bash -lc 'grep -A2 -E "name: \"[①②③]" /work/ansible/provision-l3.yml | grep -vE "^--$"' | sed 's/^/   │ /'
echo "   ── 적용 ──"
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-l3.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   after  │ /'
PID_AFTER=$(docker exec "$PNODE" sh -lc "pgrep -f 's_server -accept' | head -1" | tr -d '[:space:]')
echo "   재시작 확인: 서비스 pid $PID_BEFORE → $PID_AFTER $([ "$PID_BEFORE" != "$PID_AFTER" ] && echo '(새 프로세스 = 새 설정으로 로드됨)' || echo '(pid 동일 — 재시작되지 않았다)')"
echo "   ※ 이 노드의 OpenSSL은 이 조각의 PQC 그룹을 모른다 — 그래서 **능력이 바뀌었다고 말하지 않는다**."
echo "     L3가 보이는 것은 활성화 지점 연결·재시작·가역성이다. 이 노드의 실제 조치는 fork 교체이며,"
echo "     그건 config로 배포되지 않는다고 L2 플레이북이 이미 주석으로 말한다(§4.3)."
echo "   ── L3 되돌림(--rollback): 대칭 역순 — pre → 활성화 되돌림 → 파일 제거 → 재시작 ──"
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 --rollback /work/plan-l3.json > /work/ansible/provision-l3-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-l3-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   rolled │ /'
fi  # PNODE 가드 끝

echo
echo "✅ 데모 완료 (전 범위): 접근준비→디스커버리→인벤토리(엔드포인트·프로필·앱귀속·이력·스코프)→프로비저닝(L2 배치·L3 활성화·되돌림)."
echo "   산출물: demo/.generated/topology.svg (색=posture) · 컨트롤러 /work/{plan.json,plan-l3.json,ansible/playbook{,-l3}.yml,ansible/rollback{,-l3}.yml}."
echo "   ※ 생성물을 실제로 적용·활성화·되돌림까지 실행해 확인한 것 — 생성만 보면 깨끗한 노드에서 깨지는 플레이북도 통과한다."
echo "   접근 비밀은 targets.ini(런타임 전용)에만 — 인벤토리엔 미영속(§4A.3)."
echo "   (선언 대비 3-상태 대조·거버넌스는 이 리포가 하지 않는다)"
echo "   정리: ./demo/scripts/down.sh"
