#!/usr/bin/env bash
# pqcota 데모 — 접근 준비(hosts→Ansible·엔드포인트) → 디스커버리 → 중앙 인벤토리(엔드포인트·
# 프로필·앱 표시) → 프로비저닝 생성(플레이북 + before/롤백 레코드). 전부 이 리포 범위.
set -euo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DSN="postgres://postgres:pqcota@pqcota-demo-pg:5432/pqcota"

# 조정 지점은 전부 환경변수다. 여기 적어 두는 건 **문서를 읽어야만 있는 줄 아는** 것을 없애기
# 위해서다 — 특히 선택 단계는 켜지 않으면 존재 자체가 출력에 안 나온다.
# up.sh 확인보다 먼저 온다: 아직 안 세운 사람도 무엇이 있는지는 볼 수 있어야 한다.
usage() {
	cat <<'HELP'
usage: demo.sh            access prep → discovery → inventory → provisioning (6 stages)

It takes no arguments; tune it with environment variables:

  DEMO_REAL_PROVIDER=1   enable the optional stage — build a real oqsprovider, stage and activate it on an
                         OpenSSL 3.0-3.4 node, and check the remediation **lands as real algorithms**.
                         The default demo ships an empty file and shows the delivery path only. The first
                         run takes a few minutes longer because of that build.
#   OQS_BUILD_BASE=<image> base image to build that provider on. It must match the node image for ABI
                         compatibility (default ubuntu:24.04 — same as the OpenSSL 3 node in the default topology).

  DEMO_TARGET_EDGES=<n>  re-run discovery until this many edges are observed (default: edges in the topology)
  DEMO_MAX_ATTEMPTS=<n>  cap on those retries (default 4)

examples:
  ./demo/scripts/demo.sh
  DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh

Bring the environment up with ./demo/scripts/up.sh first, and tear it down with ./demo/scripts/down.sh.
HELP
}
case "${1:-}" in
-h | --help) usage; exit 0 ;;
"") ;;
*) echo "demo.sh: unknown argument '$1' — this script takes none." >&2; usage >&2; exit 2 ;;
esac

# 데모 구성은 up.sh가 topology.yaml에서 생성한 산출물(compose·groups·profiles·manifest)이 정의한다.
GEN="$DEMO_DIR/.generated"
[ -f "$GEN/manifest.env" ] || { echo "run ./demo/scripts/up.sh first (nothing has been generated yet)." >&2; exit 1; }
source "$GEN/manifest.env"  # NODES · EDGE_COUNT · HUMAN
ANS="cd /work/ansible && ansible"       # ansible.cfg 적용 위해 그 디렉토리에서 실행
INV="-i /work/ansible/targets.ini -i /work/ansible/groups.ini"
pg() { docker exec pqcota-demo-pg psql -U postgres -d pqcota "$@"; }

echo "▶ 0/6 access prep — your hosts file → Ansible inventory (secrets not persisted) + endpoint upsert…"
for i in $(seq 1 20); do docker exec pqcota-demo-pg pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
# pqcota-hosts: (a) targets.ini에 접속 비밀(키) — 런타임 전용·0600, (b) 엔드포인트(비밀 제외) Postgres upsert.
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
  "pqcota-hosts --ansible-out /work/ansible/targets.ini --dsn '$DSN' /work/hosts.csv" | sed 's/^/   /'
echo "   ── generated targets.ini: carries the access key (runtime-only, not persisted) ──"
docker exec pqcota-ctl bash -lc 'grep -m1 ansible_ssh_private_key_file /work/ansible/targets.ini' | sed 's/^/   /'
echo "   ── inventory (Postgres) endpoints: no secrets (node_id, name, ip, port only) ──"
pg -tAc "select '   '||node_id||'  '||(endpoint->>'name')||'  '||(endpoint->>'ip')||':'||(endpoint->>'port') from pqcota_endpoint order by node_id"
secret_ct=$(pg -tAc "select count(*) from pqcota_endpoint where endpoint::text ~* 'ssh|key|root|id_demo'")
echo "   → traces of access secrets in the endpoints: ${secret_ct} (0 is correct)"
# CMDB 프로필 선언 임포트(pqcota-profile — CMDB/리뷰어 레인, 관측 아님). 인벤토리 뷰 시각 구분.
# 프로필·그룹은 토폴로지에서 생성된 것을 쓴다(노드가 가변이므로).
docker cp "$GEN/profiles.csv" pqcota-ctl:/work/profiles.csv
docker cp "$GEN/groups.ini"   pqcota-ctl:/work/ansible/groups.ini
docker exec pqcota-ctl bash -lc "pqcota-profile --dsn '$DSN' /work/profiles.csv" >/dev/null 2>&1

echo "▶ 1/6 controller → target SSH check (Ansible ping, using the inventory pqcota-hosts generated)…"
docker exec pqcota-ctl bash -lc "$ANS $INV -m ping targets"

echo "▶ 2/6 running discovery — assets once (discover.yml), then traffic (discover_traffic.yml)…"
# 목표 엣지 수에 못 미치면 재수집한다. 관측 구간 안에 트래픽이 안 흐를 수 있는 것은 실환경에서도
# 참이라(유휴 링크) 이 backstop은 남긴다. 다만 예전에 이 루프가 자주 돌던 진짜 이유는 타이밍이
# 아니라 **collector 결함**이었다 — 원시 syscall의 EINTR을 치명적으로 다뤄 관측 구간이 무작위로
# 잘렸다(25초 구간이 0·0·14·25초에 끝남). 고친 뒤로는 첫 시도에 다 잡힌다.
TARGET_EDGES="${DEMO_TARGET_EDGES:-${EDGE_COUNT:-3}}"
MAX_ATTEMPTS="${DEMO_MAX_ATTEMPTS:-4}"
edge_count() {
  docker exec pqcota-ctl bash -lc 'grep -oh srcNodeId /work/results/*-net-*.json 2>/dev/null | wc -l' | tr -d '[:space:]'
}
# 자산 스캔은 한 번이면 된다 — 로드된 lib과 provider 체인은 재수집으로 달라지지 않는다.
# 다시 도는 것은 통신 관측뿐이고, 그것이 두 플레이북을 가른 이유다.
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover.yml" >/dev/null
for attempt in $(seq 1 "$MAX_ATTEMPTS"); do
  docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover_traffic.yml" >/dev/null
  cnt="$(edge_count)"; cnt="${cnt:-0}"
  echo "   attempt $attempt/$MAX_ATTEMPTS — ${cnt} observed edges (target ${TARGET_EDGES}+)"
  if [ "$cnt" -ge "$TARGET_EDGES" ]; then break; fi
  if [ "$attempt" -lt "$MAX_ATTEMPTS" ]; then echo "   not enough edges → collecting again (containers are warm)…"; fi
done

echo "▶ 3/6 read-only discovery view (assets + grade)…"
docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results /work/nodes.json /work/topology.dot'

echo "▶ 4/6 rendering the observed topology as SVG and fetching it…"
# 리포로 꺼내는 결과물은 전부 demo/.generated/ 아래로(일관성 — down.sh가 통째로 지운다).
mkdir -p "$GEN"
if docker exec pqcota-ctl bash -lc 'command -v dot >/dev/null && dot -Tsvg /work/topology.dot -o /work/topology.svg'; then
  docker cp pqcota-ctl:/work/topology.svg "$GEN/topology.svg"
  echo "   → $GEN/topology.svg"
fi
docker cp pqcota-ctl:/work/topology.dot "$GEN/topology.dot" 2>/dev/null || true

echo "▶ 5/6 central inventory ingest and query (append-only Postgres · endpoints, profiles, app labels · history and changes)…"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' | sed 's/^/   /'
echo "   ── query (pqcota-inventory) — ▸machine header (endpoint, profile) · @app label (a shared .so has several) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-inventory' \
  | grep -E 'central inventory|▸|@|totals' | sed 's/^/   /'

# 이력 — 같은 회수 결과를 한 번 더 적재한다(실운용의 "다음 회차 스캔"). 내용이 같으니
# 스냅샷은 늘지 않고 관측 기록만 쌓인다 — 저장은 변화 횟수만큼만 자라되 "봤다"는 사실은 남는다.
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' >/dev/null
# 엣지를 실제로 관측한 노드로 보인다(-snapshot의 핵심이 엣지라, 엣지 0건 노드면 볼 게 없다).
HNODE=$(pg -tAc "select node_id from pqcota_snapshots
  order by jsonb_array_length(coalesce(edges,'[]'::jsonb)) desc, node_id limit 1" | tr -d '[:space:]')
[ -z "$HNODE" ] && HNODE=$(pg -tAc "select node_id from pqcota_endpoint order by node_id limit 1" | tr -d '[:space:]')
echo "   ── history (pqcota-inventory -history $HNODE): a snapshot only on change; obs counts re-confirmations ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -history '$HNODE'" | sed 's/^/   /'
PRE_SNAP=$(pg -tAc "select id from pqcota_snapshots where node_id='$HNODE' order by seq desc limit 1" | tr -d '[:space:]')
echo "   ── snapshot detail (-snapshot): assets + that snapshot's observed edges (the cumulative view shows totals only) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -snapshot '$PRE_SNAP'" | sed 's/^/   /'

# 엣지의 앱 — 관측은 캡처하는 순간 소켓이 살아 있어야 앱을 알아낸다. 짧게 붙었다 끊긴 연결은
# 그 구간을 벗어나 `@?`로 남는다. **그건 "앱이 없다"가 아니라 "어느 앱인지 밝히지 못했다"이고**, 그 자리를
# 사람이 선언으로 메운다. 관측을 고치지 않고 자기 레인으로 들어가며, 메운 것은 (declared)로 표시된다.
# dst는 **엣지에 찍힌 그대로** 적는다(이 데모에서는 "ip:port" 모양이다) — 화면에서 보이는 값을
# 그대로 옮기면 되므로, 읽는 사람이 형식을 따로 배울 필요가 없다.
UNATTR_DST=$(pg -tAc "select e->>'dstAddr' from pqcota_snapshots s, jsonb_array_elements(s.edges) e
  where s.id='$PRE_SNAP' and coalesce(e->>'appKey','')='' limit 1" | tr -d '[:space:]')
if [ -n "$UNATTR_DST" ]; then
  echo "   ── app declaration (pqcota-declare-attribution): a person names the app for edges observation missed ──"
  echo "      target $UNATTR_DST — a short-lived connection; the socket was already closed during the window"
  docker exec -i pqcota-ctl bash -lc "cat > /work/attribution.csv" <<CSV
node_id,dst,app_key
$HNODE,$UNATTR_DST,batch-runner.service
CSV
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
    'pqcota-declare-attribution --out /work/declared-attr /work/attribution.csv && pqcota-ingest /work/declared-attr >/dev/null' \
    | sed 's/^/      /'
  echo "   ── query again: the observed edges are untouched; only the blanks are filled ──"
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -snapshot '$PRE_SNAP'" \
    | grep -E 'observed edges|→|declared by a person' | sed 's/^/   /'
else
  echo "   (every edge got an app this time — with no blanks there is nothing to declare)"
fi

# 자산 스코프 — 노드는 등재됐어도(§1.4) 그 안에서 계속 관리할 자산만 남긴다. 시스템 기본
# 라이브러리·패키지 런타임이 섞이면 인벤토리가 잡음에 묻힌다.
# 내용이 바뀌므로 새 스냅샷이 생긴다 → 바로 이게 -diff로 보일 "실제 변화"다.
docker exec -i pqcota-ctl bash -lc "cat > /work/scope-assets.csv" <<'CSV'
action,runtime,lib,app_key,note
exclude,*,*,/usr/sbin/sshd*,sshd is managed by OS patching — no need to keep observing it
exclude,*,*,/usr/bin/python*,a python runtime pulled in by a package
CSV
echo "   ── asset scope (-scope-assets): drop the noise (sshd, python runtime) out of management ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
  'pqcota-ingest -scope-assets /work/scope-assets.csv /work/results' | grep -E 'asset scope|•' | sed 's/^/   /'
echo "   ── inventory after exclusion: only what the apps actually use (excluded != absent — the count is reported) ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-inventory' \
  | grep -E '▸|openssl|jca|excluded by asset scope|totals' | sed 's/^/   /'

POST_SNAP=$(pg -tAc "select id from pqcota_snapshots where node_id='$HNODE' order by seq desc limit 1" | tr -d '[:space:]')
echo "   ── changes (-diff): before and after the scope. 'removed' means dropped from management, not gone ──"
if [ -n "$PRE_SNAP" ] && [ -n "$POST_SNAP" ] && [ "$PRE_SNAP" != "$POST_SNAP" ]; then
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -diff '$PRE_SNAP','$POST_SNAP'" \
    | grep -vE '^\s*$' | sed 's/^/   /'
else
  echo "   (this node's content is unchanged after the scope, so there is no new snapshot — without two points there is no diff)"
fi

# 보존 정책 — 파괴적 동작이라 조회 커맨드와 분리했고 기본이 dry-run이다.
echo "   ── retention (pqcota-prune, dry-run by default): keeps the newest, prunes before it — deletion only with -apply ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-prune -keep-last 1' | sed 's/^/   /'

echo "▶ 6/6 provisioning — finalized plan → generate L2/L3 playbooks → apply → roll back…"
# 프로비저닝 대상 노드(PNODE) — openssl finding이 있는 노드를 인벤토리에서 고른다(공유 .so로 다중
# 앱이 붙은 쪽 우선 = 영향 반경이 가장 또렷한 케이스). 없으면 이 단계는 정직히 생략한다(§2.5).
PNODE=$(pg -tAc "select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where f ? 'openssl' order by jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb)) desc, s.node_id limit 1" | tr -d '[:space:]')
if [ -z "$PNODE" ]; then
  echo "   (no node has an openssl finding, so the provisioning walk-through is skipped — add an openssl server to the topology to see it)"
else
# 인벤토리에서 실제 finding을 골라 확정 계획을 만든다. 공유 libssl(여러 앱에 걸침) 우선, 없으면 아무 finding.
pick() { pg -tAc "select f->>'id' from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where s.seq=(select max(seq) from pqcota_snapshots where node_id='$PNODE') and ($1)
  order by jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb)) desc, f->>'id' limit 1" | tr -d '[:space:]'; }
FID=$(pick "f->'openssl'->>'lib'='libssl.so.3' and jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb))>=2")
[ -z "$FID" ] && FID=$(pick "jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb))>=2")
[ -z "$FID" ] && FID=$(pick "f ? 'openssl'")
echo "   target finding: $FID ($PNODE)"
docker exec -i pqcota-ctl bash -lc "cat > /work/plan.json" <<JSON
{"id":"plan-demo","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
 "approvalSignatures":["reviewer:demo"],
 "actions":[{"id":"a1","targetNodeId":"$PNODE","findingId":"$FID",
   "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL",
   "kind":"REMEDIATION_KIND_PROVIDER_INJECT","targetAlgorithm":"ML-KEM (FIPS 203)",
   "providerChoice":"oqsprovider","rollbackNote":"one cnf line + remove the module"}]}
JSON
echo "   ── pqcota-provision: finalized plan (§3.7 gate) → generate the L2 playbook + capture before, persist a record ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --dsn '$DSN' /work/plan.json > /work/ansible/provision.yml" 2>&1 | sed 's/^/   /'
docker exec pqcota-ctl bash -lc 'grep -E "module = |dest:" /work/ansible/provision.yml' | sed 's/^/   │ /'
echo "   ── read back the persisted rollback record (pqcota-records): affected apps (several for a shared .so) + the before state ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-records $PNODE" | sed 's/^/   /'

# ── 생성물이 실제로 도는지까지 봐야 "생성했다"가 말이 된다. 생성만 하고 안 돌리면
# 깨끗한 노드에서 깨지는 플레이북도 통과해 버린다(실제로 그런 결함이 여럿 있었다).
echo
echo "   ══ actually apply the generated playbook (ansible-playbook) ══"
# provider 모듈은 도구가 주지 않는다. 데모는 배포 경로만 보이려 빈 파일을 쓴다(암호 기능 없음).
docker exec pqcota-ctl bash -lc 'mkdir -p /work/ansible/files && : > /work/ansible/files/oqsprovider.so'
MOD_SHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')
echo "   ── apply (ansible-playbook) — with the module sha256 gate ──"
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$MOD_SHA provision.yml" \
  | grep -E "TASK|ok=|changed=|failed=" | sed 's/^/   /'
echo "   ── did it actually land on the target node ──"
docker exec "$PNODE" sh -lc 'ls -l /opt/pqcota/oqsprovider.so /etc/pqcota/openssl-pqc.cnf' | sed 's/^/   /'
docker exec "$PNODE" sh -lc 'grep "^module" /etc/pqcota/openssl-pqc.cnf' | sed 's/^/   path referenced by the config: /'

echo "   ── roll back (--rollback) — nothing was overwritten, so removal is enough ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --rollback /work/plan.json > /work/ansible/provision-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc 'ls /opt/pqcota/oqsprovider.so /etc/pqcota/openssl-pqc.cnf 2>&1 || true' | sed 's/^/   /'

# ── L3(활성화·재시작) — L2가 놓기만 한 조각을 실제로 **참조되게** 만들고 서비스를 다시 띄운다.
# 활성화·재시작 방법은 환경마다 다르므로 도구가 추측하지 않는다(§2.5): 계획의 activation 훅에
# **사용자가 적은 명령**을 도구가 의미 순서로 배치할 뿐이다. 이 노드는 ssl-apps.sh로 서비스를
# 관리하므로 훅이 그것을 가리킨다 — 현실의 systemd unit·사내 기동 스크립트에 해당한다.
echo
echo "   ══ L3 (activate and restart) — the plan's hooks in meaningful order: pre → stage → activate → restart ══"
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   before │ /'
PID_BEFORE=$(docker exec "$PNODE" sh -lc "pgrep -f 's_server -accept' | head -1" | tr -d '[:space:]')
docker exec -i pqcota-ctl bash -lc "cat > /work/plan-l3.json" <<JSON
{"id":"plan-demo-l3","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
 "approvalSignatures":["reviewer:demo"],
 "actions":[{"id":"a1","targetNodeId":"$PNODE","findingId":"$FID",
   "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL",
   "kind":"REMEDIATION_KIND_CONFIG_ONLY","targetAlgorithm":"ML-KEM (FIPS 203)",
   "rollbackNote":"remove the activation point + restart",
   "activation":{
     "pre":"/usr/local/bin/ssl-apps.sh stop",
     "activate":"printf 'OPENSSL_CONF=%s\\n' /etc/pqcota/openssl-pqc.cnf > /etc/pqcota/service.env",
     "deactivate":"rm -f /etc/pqcota/service.env",
     "restart":"/usr/local/bin/ssl-apps.sh start"}}]}
JSON
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 /work/plan-l3.json > /work/ansible/provision-l3.yml" 2>&1 | sed 's/^/   /'
echo "   ── the generated hook tasks (the order is the safety: stop → change → enable → restart) ──"
docker exec pqcota-ctl bash -lc 'grep -A2 -E "name: \"[①②③]" /work/ansible/provision-l3.yml | grep -vE "^--$"' | sed 's/^/   │ /'
echo "   ── apply ──"
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-l3.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   after  │ /'
PID_AFTER=$(docker exec "$PNODE" sh -lc "pgrep -f 's_server -accept' | head -1" | tr -d '[:space:]')
echo "   restart check: service pid $PID_BEFORE → $PID_AFTER $([ "$PID_BEFORE" != "$PID_AFTER" ] && echo '(new process = loaded with the new config)' || echo '(same pid — it did not restart)')"
echo "   ※ this node's OpenSSL does not know the PQC group in this fragment — so **we do not claim its capability changed**."
echo "     what L3 shows is the activation wiring, the restart and the reversibility. The real remediation here is a fork"
echo "     replacement, and the L2 playbook already says in a comment that this is not delivered through config (provisioning design §4.1)."
echo "   ── L3 rollback (--rollback): the symmetric reverse — pre → undo activation → remove files → restart ──"
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 --rollback /work/plan-l3.json > /work/ansible/provision-l3-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-l3-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$PNODE" sh -lc '/usr/local/bin/ssl-apps.sh status' | sed 's/^/   rolled │ /'
fi  # PNODE 가드 끝

# ── (선택) 실물 provider — 생성물이 정말 능력을 만드는가 ──────────────────────────
# 6/6까지는 **빈 파일**을 배치한다. 배포 경로(스테이징·sha256 게이트·활성화·되돌림)를 보이는 데는
# 그것으로 충분하고, "Docker만 있으면 된다"는 데모 전제도 지킨다. 다만 빈 파일로는 마지막 한 칸을
# 못 보인다: **도구가 낸 조각이 그 노드의 OpenSSL에 실제로 PQC 능력을 만드는가.**
# DEMO_REAL_PROVIDER=1이면 실물 oqsprovider를 빌드해 그 한 칸까지 확인한다(빌드가 수 분 걸려 기본 꺼짐).
#
# 대상은 6/6의 PNODE가 아니다 — 넣을 자리가 있는 노드는 **OpenSSL 3.0–3.4**뿐이다.
# 1.1.1엔 provider라는 개념이 없고, 3.5+는 ML-KEM이 네이티브라 조치가 CONFIG_ONLY로 갈린다
# (pkg/provisioning/openssl.go). 인벤토리에서 그 대역을 관측한 노드를 고르고, 없으면 생략한다(§2.5).
BAND="f->'openssl'->>'version' ~ '^3\.[0-4]([.]|$)'"
if [ "${DEMO_REAL_PROVIDER:-0}" = "1" ]; then
echo
echo "▶ (optional) real provider check — remediate → re-observe → inventory change (DEMO_REAL_PROVIDER=1)"
RNODE=$(pg -tAc "select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where $BAND order by s.seq desc limit 1" | tr -d '[:space:]')
if [ -z "$RNODE" ]; then
  echo "   (no node observed with OpenSSL 3.0-3.4, so this is skipped — that band is where provider injection belongs)"
else
RFID=$(pg -tAc "select f->>'id' from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where s.node_id='$RNODE' and $BAND order by s.seq desc, f->>'id' limit 1" | tr -d '[:space:]')
RVER=$(pg -tAc "select distinct (f->'openssl'->>'lib')||' '||(f->'openssl'->>'version') from pqcota_snapshots s, jsonb_array_elements(s.findings) f where f->>'id'='$RFID'" | head -1 | sed 's/^ *//;s/ *$//')
echo "   target: $RNODE ($RVER) · finding $RFID"
# 기본 토폴로지에서 이 finding은 5단계 스코프가 잡음으로 뺀 것이다(sshd·python이 로드한 libcrypto).
# 여기서 보려는 것은 "관리할 자산인가"가 아니라 "3.0 런타임에서 도구가 낸 조각이 먹는가"라서 그대로 쓴다.

# 능력 측정은 `list -kem-algorithms`로 한다. `-tls-groups`는 3.2+에만 있어서 이 대역(3.0–3.4)의
# 아래쪽 노드에서는 옵션 자체가 없다 — 없는 옵션의 빈 출력을 "능력 없음"으로 읽으면 오답이 된다.
echo "   ── capability before: the ML-KEM family KEMs this node's OpenSSL knows ──"
KEMQ='openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem || true'
BEFORE_G=$(docker exec "$RNODE" sh -lc "$KEMQ" | tr -d '[:space:]')
echo "   openssl list -kem-algorithms | grep -ci mlkem  →  ${BEFORE_G:-0}"

# 실물 모듈. 노드 이미지와 같은 베이스에서 빌드해야 ABI가 맞는다.
"$DEMO_DIR/scripts/internal/build-oqsprovider.sh" "$GEN/oqsprovider.so"
docker exec pqcota-ctl bash -lc 'mkdir -p /work/ansible/files'
docker cp "$GEN/oqsprovider.so" pqcota-ctl:/work/ansible/files/oqsprovider.so >/dev/null
RSHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')

docker exec -i pqcota-ctl bash -lc "cat > /work/plan-real.json" <<JSON
{"id":"plan-demo-real","status":"PLAN_STATUS_FINALIZED","scope":"ring-0",
 "approvalSignatures":["reviewer:demo"],
 "actions":[{"id":"a1","targetNodeId":"$RNODE","findingId":"$RFID",
   "cryptoRuntime":"CRYPTO_RUNTIME_OPENSSL",
   "kind":"REMEDIATION_KIND_PROVIDER_INJECT","targetAlgorithm":"ML-KEM (FIPS 203)",
   "providerChoice":"oqsprovider","rollbackNote":"one cnf line + remove the module",
   "activation":{
     "pre":"/usr/local/bin/ssl-apps.sh stop",
     "activate":"printf 'OPENSSL_CONF=%s\\n' /etc/pqcota/openssl-pqc.cnf > /etc/pqcota/service.env",
     "deactivate":"rm -f /etc/pqcota/service.env",
     "restart":"/usr/local/bin/ssl-apps.sh start"}}]}
JSON
echo "   ── L2 staging (a real .so, sha256 gate) + L3 activation ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --dsn '$DSN' /work/plan-real.json > /work/ansible/provision-real.yml" 2>&1 | sed 's/^/   /'
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$RSHA provision-real.yml" \
  | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 /work/plan-real.json > /work/ansible/provision-real-l3.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'

echo "   ── capability after: ask again, with that configuration active ──"
ACT='. /etc/pqcota/service.env 2>/dev/null; export OPENSSL_CONF;'
docker exec "$RNODE" sh -lc "$ACT openssl list -providers 2>/dev/null | grep -A2 -i oqs | head -4" | sed 's/^/   /'
docker exec "$RNODE" sh -lc "$ACT openssl list -kem-algorithms 2>/dev/null | grep -i mlkem | head -3" | sed 's/^/   /'
AFTER_G=$(docker exec "$RNODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
if [ "${AFTER_G:-0}" -gt "${BEFORE_G:-0}" ]; then
  echo "   → ML-KEM KEMs ${BEFORE_G:-0} → ${AFTER_G:-0}. The config and staging this tool produced **landed as real algorithms**."
else
  echo "   → ML-KEM KEMs ${BEFORE_G:-0} → ${AFTER_G:-0} — no increase. The module was not loaded."
  echo "     where to look: unresolved dependencies of the module (\`ldd\`) and the module path in the cnf. We do not record a failure as a success."
fi

echo "   ── after re-observing (discovery again → ingest), does the inventory see this change ──"
RPRE=$(pg -tAc "select id from pqcota_snapshots where node_id='$RNODE' order by seq desc limit 1" | tr -d '[:space:]')
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover.yml" >/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover_traffic.yml" >/dev/null
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' | sed 's/^/   /'
RPOST=$(pg -tAc "select id from pqcota_snapshots where node_id='$RNODE' order by seq desc limit 1" | tr -d '[:space:]')
if [ -n "$RPRE" ] && [ -n "$RPOST" ] && [ "$RPRE" != "$RPOST" ]; then
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -diff '$RPRE','$RPOST'" | sed 's/^/   /'
else
  echo "   no change — no new snapshot was created (identical content creates none)."
fi
echo "   ※ the capability clearly grew, yet the inventory did not move. That is not a fabricated result but **a fact about what is observed**:"
echo "     · there is still no path that observes the OpenSSL provider layer — it goes as far as libssl/libcrypto in"
echo "       /proc/maps and ELF strings (fork, version). JCA sees its provider chain via attach; OpenSSL cannot be seen that way."
echo "     · the handshake does not change either — negotiation needs both ends, and the peer in this topology is 1.1.1."
echo "     the reasoning is in discovery/design.md §2.1. Not pretending to have what it does not is this tool's premise (§2.5)."

echo "   ── roll back (L3 → L2) — return the node to its original state ──"
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 --rollback /work/plan-real.json > /work/ansible/provision-real-l3-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --rollback /work/plan-real.json > /work/ansible/provision-real-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$RNODE" sh -lc "$KEMQ" | sed 's/^/   ML-KEM KEMs after rollback: /'
fi  # RNODE 가드 끝
fi  # DEMO_REAL_PROVIDER 끝

echo
echo "✅ demo complete (full scope): access prep → discovery → inventory (endpoints, profiles, app labels, history, scope) → provisioning (L2 staging, L3 activation, rollback)."
echo "   artifacts: demo/.generated/topology.svg (colour = grade) · on the controller /work/{plan.json,plan-l3.json,ansible/playbook{,-l3}.yml,ansible/rollback{,-l3}.yml}."
echo "   ※ the artifacts were actually applied, activated and rolled back — checking generation alone lets a playbook pass that breaks on a clean node."
echo "   access secrets live only in targets.ini (runtime-only) — never persisted in the inventory (§1.5)."
echo "   (three-state reconciliation against declarations, and governance, are not done by this repo)"
echo "   tear down: ./demo/scripts/down.sh"
