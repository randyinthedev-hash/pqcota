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
사용법: demo.sh            접근준비 → 디스커버리 → 인벤토리 → 프로비저닝(6단계)

인자는 받지 않는다. 조정은 환경변수로 한다:

  DEMO_REAL_PROVIDER=1   선택 단계를 켠다 — 실물 oqsprovider를 빌드해 OpenSSL 3.0–3.4 노드에
                         배치·활성화하고, 조치가 **실제 암호 알고리즘으로 반영되는지**까지 확인한다.
                         기본 데모는 빈 파일로 배포 경로만 보인다. 첫 실행은 빌드로 수 분 더 걸린다.
  OQS_BUILD_BASE=<이미지> 그 provider를 빌드할 베이스. 노드 이미지와 같아야 ABI가 맞는다
                         (기본 ubuntu:24.04 — 기본 토폴로지의 OpenSSL 3 노드와 같다).

  DEMO_TARGET_EDGES=<n>  이만큼 관측될 때까지 디스커버리를 다시 돈다 (기본: 토폴로지의 엣지 수)
  DEMO_MAX_ATTEMPTS=<n>  그 재수집의 상한 (기본 4)

예:
  ./demo/scripts/demo.sh
  DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh

먼저 ./demo/scripts/up.sh 로 환경을 세우고, 끝나면 ./demo/scripts/down.sh 로 지운다.
HELP
}
case "${1:-}" in
-h | --help) usage; exit 0 ;;
"") ;;
*) echo "demo.sh: 모르는 인자 '$1' — 이 스크립트는 인자를 받지 않는다." >&2; usage >&2; exit 2 ;;
esac

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

echo "▶ 5/6 중앙 인벤토리 적재·조회 (Postgres append-only · 엔드포인트·프로필·앱 표시 · 이력·변화)…"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' | sed 's/^/   /'
echo "   ── 조회(pqcota-inventory) — ▸머신 헤더(엔드포인트·프로필) · @앱 표시(공유 .so는 다중) ──"
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

# 엣지의 앱 — 관측은 캡처하는 순간 소켓이 살아 있어야 앱을 알아낸다. 짧게 붙었다 끊긴 연결은
# 그 창을 벗어나 `@?`로 남는다. **그건 "앱이 없다"가 아니라 "어느 앱인지 밝히지 못했다"이고**, 그 자리를
# 사람이 선언으로 메운다. 관측을 고치지 않고 자기 레인으로 들어가며, 메운 것은 (declared)로 표시된다.
# dst는 **엣지에 찍힌 그대로** 적는다(이 데모에서는 "ip:port" 모양이다) — 화면에서 보이는 값을
# 그대로 옮기면 되므로, 읽는 사람이 형식을 따로 배울 필요가 없다.
UNATTR_DST=$(pg -tAc "select e->>'dstAddr' from pqcota_snapshots s, jsonb_array_elements(s.edges) e
  where s.id='$PRE_SNAP' and coalesce(e->>'appKey','')='' limit 1" | tr -d '[:space:]')
if [ -n "$UNATTR_DST" ]; then
  echo "   ── 앱 선언(pqcota-declare-attribution): 관측이 못 잡은 엣지를 사람이 지정한다 ──"
  echo "      대상 $UNATTR_DST — 짧은 연결이라 캡처 창에서 소켓이 이미 닫혀 있었다"
  docker exec -i pqcota-ctl bash -lc "cat > /work/attribution.csv" <<CSV
node_id,dst,app_key
$HNODE,$UNATTR_DST,batch-runner.service
CSV
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc \
    'pqcota-declare-attribution --out /work/declared-attr /work/attribution.csv && pqcota-ingest /work/declared-attr >/dev/null' \
    | sed 's/^/      /'
  echo "   ── 다시 조회: 관측 엣지는 그대로고, 빈 자리만 메워진다 ──"
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -snapshot '$PRE_SNAP'" \
    | grep -E '관측 엣지|→|사람이 선언한 앱' | sed 's/^/   /'
else
  echo "   (이 창에서는 모든 엣지가 앱까지 잡혔다 — 메울 자리가 없으면 선언도 없다)"
fi

# 자산 스코프 — 노드는 등재됐어도(§1.4) 그 안에서 계속 관리할 자산만 남긴다. 시스템 기본
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
# 앱이 붙은 쪽 우선 = 영향 반경이 가장 또렷한 케이스). 없으면 이 단계는 정직히 생략한다(§2.5).
PNODE=$(pg -tAc "select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where f ? 'openssl' order by jsonb_array_length(coalesce(f->'appKeys','[]'::jsonb)) desc, s.node_id limit 1" | tr -d '[:space:]')
if [ -z "$PNODE" ]; then
  echo "   (openssl finding을 가진 노드가 없어 프로비저닝 시연은 생략 — 토폴로지에 openssl 서버를 두면 보인다)"
else
# 인벤토리에서 실제 finding을 골라 확정 계획을 만든다. 공유 libssl(여러 앱에 걸침) 우선, 없으면 아무 finding.
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
# 활성화·재시작 방법은 환경마다 다르므로 도구가 추측하지 않는다(§2.5): 계획의 activation 훅에
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
echo "     그건 config로 배포되지 않는다고 L2 플레이북이 이미 주석으로 말한다(프로비저닝 설계 §4.1)."
echo "   ── L3 되돌림(--rollback): 대칭 역순 — pre → 활성화 되돌림 → 파일 제거 → 재시작 ──"
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
echo "▶ (선택) 실물 provider 검증 — 조치 → 재관측 → 인벤토리 변화 (DEMO_REAL_PROVIDER=1)"
RNODE=$(pg -tAc "select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where $BAND order by s.seq desc limit 1" | tr -d '[:space:]')
if [ -z "$RNODE" ]; then
  echo "   (OpenSSL 3.0–3.4를 관측한 노드가 없어 생략 — provider 주입이 갈 자리가 그 대역이다)"
else
RFID=$(pg -tAc "select f->>'id' from pqcota_snapshots s, jsonb_array_elements(s.findings) f
  where s.node_id='$RNODE' and $BAND order by s.seq desc, f->>'id' limit 1" | tr -d '[:space:]')
RVER=$(pg -tAc "select distinct (f->'openssl'->>'lib')||' '||(f->'openssl'->>'version') from pqcota_snapshots s, jsonb_array_elements(s.findings) f where f->>'id'='$RFID'" | head -1 | sed 's/^ *//;s/ *$//')
echo "   대상: $RNODE ($RVER) · finding $RFID"
# 기본 토폴로지에서 이 finding은 5단계 스코프가 잡음으로 뺀 것이다(sshd·python이 로드한 libcrypto).
# 여기서 보려는 것은 "관리할 자산인가"가 아니라 "3.0 런타임에서 도구가 낸 조각이 먹는가"라서 그대로 쓴다.

# 능력 측정은 `list -kem-algorithms`로 한다. `-tls-groups`는 3.2+에만 있어서 이 대역(3.0–3.4)의
# 아래쪽 노드에서는 옵션 자체가 없다 — 없는 옵션의 빈 출력을 "능력 없음"으로 읽으면 오답이 된다.
echo "   ── 조치 전 능력: 이 노드의 OpenSSL이 아는 ML-KEM 계열 KEM ──"
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
   "providerChoice":"oqsprovider","rollbackNote":"cnf 한 줄 + 모듈 제거",
   "activation":{
     "pre":"/usr/local/bin/ssl-apps.sh stop",
     "activate":"printf 'OPENSSL_CONF=%s\\n' /etc/pqcota/openssl-pqc.cnf > /etc/pqcota/service.env",
     "deactivate":"rm -f /etc/pqcota/service.env",
     "restart":"/usr/local/bin/ssl-apps.sh start"}}]}
JSON
echo "   ── L2 배치(실물 .so · sha256 게이트) + L3 활성화 ──"
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --dsn '$DSN' /work/plan-real.json > /work/ansible/provision-real.yml" 2>&1 | sed 's/^/   /'
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$RSHA provision-real.yml" \
  | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 /work/plan-real.json > /work/ansible/provision-real-l3.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'

echo "   ── 조치 후 능력: 활성화된 그 설정으로 다시 묻는다 ──"
ACT='. /etc/pqcota/service.env 2>/dev/null; export OPENSSL_CONF;'
docker exec "$RNODE" sh -lc "$ACT openssl list -providers 2>/dev/null | grep -A2 -i oqs | head -4" | sed 's/^/   /'
docker exec "$RNODE" sh -lc "$ACT openssl list -kem-algorithms 2>/dev/null | grep -i mlkem | head -3" | sed 's/^/   /'
AFTER_G=$(docker exec "$RNODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
if [ "${AFTER_G:-0}" -gt "${BEFORE_G:-0}" ]; then
  echo "   → ML-KEM KEM ${BEFORE_G:-0}개 → ${AFTER_G:-0}개. 도구가 낸 config + 배치가 **실제 암호 알고리즘으로 반영**됐다."
else
  echo "   → ML-KEM KEM ${BEFORE_G:-0}개 → ${AFTER_G:-0}개 — 늘지 않았다. 모듈이 로드되지 않은 것이다."
  echo "     확인할 곳: 모듈의 미해결 의존(\`ldd\`)과 cnf의 module 경로. 안 된 것을 됐다고 적지 않는다."
fi

echo "   ── 재관측(디스커버리 재실행 → 적재) 후 인벤토리가 이 변화를 보는가 ──"
RPRE=$(pg -tAc "select id from pqcota_snapshots where node_id='$RNODE' order by seq desc limit 1" | tr -d '[:space:]')
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV discover.yml" >/dev/null
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc 'pqcota-ingest /work/results' | sed 's/^/   /'
RPOST=$(pg -tAc "select id from pqcota_snapshots where node_id='$RNODE' order by seq desc limit 1" | tr -d '[:space:]')
if [ -n "$RPRE" ] && [ -n "$RPOST" ] && [ "$RPRE" != "$RPOST" ]; then
  docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-inventory -diff '$RPRE','$RPOST'" | sed 's/^/   /'
else
  echo "   변화 없음 — 새 스냅샷이 생기지 않았다(내용이 같으면 스냅샷을 만들지 않는다)."
fi
echo "   ※ 능력은 분명히 늘었는데 인벤토리는 그대로다. 지어낸 결과가 아니라 **관측 범위의 사실**이다:"
echo "     · OpenSSL은 provider 층을 관측하는 경로가 아직 없다 — /proc/maps의 libssl·libcrypto와"
echo "       ELF 문자열(fork·버전)까지다. JCA는 attach로 provider 체인을 보지만 OpenSSL은 관측하지 못한다."
echo "     · 핸드셰이크도 안 바뀐다 — 협상은 양쪽이 알아야 하고, 이 토폴로지의 상대는 1.1.1이다."
echo "     근거는 discovery/design.md §2.1. 없는 것을 있는 척하지 않는 것이 이 도구의 전제다(§2.5)."

echo "   ── 되돌림(L3 → L2) — 노드를 원래대로 ──"
docker exec pqcota-ctl bash -lc "pqcota-provision --level l3 --rollback /work/plan-real.json > /work/ansible/provision-real-l3-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec -e PQCOTA_DSN="$DSN" pqcota-ctl bash -lc "pqcota-provision --level l2 --rollback /work/plan-real.json > /work/ansible/provision-real-rollback.yml" 2>/dev/null
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-rollback.yml" | grep -E "ok=|changed=|failed=" | sed 's/^/   /'
docker exec "$RNODE" sh -lc "$KEMQ" | sed 's/^/   되돌린 뒤 ML-KEM KEM: /'
fi  # RNODE 가드 끝
fi  # DEMO_REAL_PROVIDER 끝

echo
echo "✅ 데모 완료 (전 범위): 접근준비→디스커버리→인벤토리(엔드포인트·프로필·앱 표시·이력·스코프)→프로비저닝(L2 배치·L3 활성화·되돌림)."
echo "   산출물: demo/.generated/topology.svg (색=posture) · 컨트롤러 /work/{plan.json,plan-l3.json,ansible/playbook{,-l3}.yml,ansible/rollback{,-l3}.yml}."
echo "   ※ 생성물을 실제로 적용·활성화·되돌림까지 실행해 확인한 것 — 생성만 보면 깨끗한 노드에서 깨지는 플레이북도 통과한다."
echo "   접근 비밀은 targets.ini(런타임 전용)에만 — 인벤토리엔 미영속(§1.5)."
echo "   (선언 대비 3-상태 대조·거버넌스는 이 리포가 하지 않는다)"
echo "   정리: ./demo/scripts/down.sh"
