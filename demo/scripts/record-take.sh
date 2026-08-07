#!/usr/bin/env bash
# 시연영상 촬영용 — 데모의 한 장면(**조치가 실제로 암호 능력을 만드는가**)만 떼어 또박또박 보인다.
#
# demo.sh와 무엇이 다른가: demo.sh는 6단계를 한 번에 흘려보내 화면이 빨리 지나가고, 어느 줄이
# 근거인지 영상에서 짚기 어렵다. 이 스크립트는 그 한 장면만 남기고 **명령을 먼저 보인 뒤 잠깐
# 멈추고 결과를 낸다.** 편집에서 잘라 붙이기 좋도록 컷 경계마다 구분선을 찍는다.
#
# 전제: `./demo/scripts/up.sh` 와 `DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh` 가 한 번 돌아
# 컨트롤러에 산출물(plan-real.json · provision-real*.yml · files/oqsprovider.so)이 있어야 한다.
# 그것들을 **다시 만들지 않고 그대로 적용**하므로 촬영을 몇 번 다시 해도 같은 화면이 나온다.
#
# 끝나면 롤백까지 해서 노드를 원래대로 둔다 — 다시 찍으면 처음부터 같은 0에서 시작한다.
set -euo pipefail

PAUSE="${TAKE_PAUSE:-2}"        # 컷 사이 정지(초). 편집에서 자르기 좋게
TYPE="${TAKE_TYPE_DELAY:-0.05}" # 타이핑 한 글자 간격(초). 0이면 즉시 출력

usage() {
	cat <<'HELP'
사용법: record-take.sh [노드]

  노드            OpenSSL 3.0–3.4를 관측한 컨테이너 이름 (기본: 인벤토리에서 자동 선택)

환경변수:
  TAKE_PAUSE=<초>        컷 사이 정지 (기본 2)
  TAKE_TYPE_DELAY=<초>   타이핑 한 글자 간격 (기본 0.05, 0이면 타이핑 연출 없음)

먼저 ./demo/scripts/up.sh → DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh 를 한 번 돌려 둔다.
HELP
}
case "${1:-}" in -h | --help) usage; exit 0 ;; esac

ANS="cd /work/ansible && ansible"
INV="-i /work/ansible/targets.ini -i /work/ansible/groups.ini"

# 프롬프트를 흉내 내 한 글자씩 찍는다 — 사람이 친 것처럼 보이게.
type_cmd() {
	printf '\033[1;32m$\033[0m '
	if [ "$TYPE" = "0" ]; then printf '%s' "$1"; else
		local i
		for ((i = 0; i < ${#1}; i++)); do printf '%s' "${1:i:1}"; sleep "$TYPE"; done
	fi
	printf '\n'
	sleep 0.4
}
say() { printf '\n\033[1;36m# %s\033[0m\n' "$1"; }

# Ansible의 PLAY RECAP 한 줄은 100자를 넘어 화면에서 접힌다 — 접힌 줄은 영상에서 지저분하고,
# 정작 봐야 하는 것은 `changed`(무엇을 바꿨나)와 `failed=0`(깨지지 않았나) 둘뿐이다.
# 나머지 칸(unreachable·skipped·rescued·ignored)은 이 촬영에서 늘 0이라 정보가 없다.
recap() { sed -nE 's/^([^ ]+) +: +(ok=[0-9]+) +(changed=[0-9]+).* (failed=[0-9]+).*/  \1  \2 \3 \4/p'; }
cut_mark() { printf '\n\033[2m%s\033[0m\n\n' "────────────────────────────────────────────────────────"; sleep "$PAUSE"; }
big() { printf '\n\033[1;33m%s\033[0m\n' "$1"; sleep "$PAUSE"; }

docker inspect pqcota-ctl >/dev/null 2>&1 || { echo "pqcota-ctl이 없다 — up.sh를 먼저 돌린다." >&2; exit 1; }
docker exec pqcota-ctl test -f /work/ansible/provision-real.yml 2>/dev/null ||
	{ echo "생성물이 없다 — DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh 를 한 번 돌린다." >&2; exit 1; }

NODE="${1:-}"
if [ -z "$NODE" ]; then
	NODE=$(docker exec pqcota-demo-pg psql -U postgres -d pqcota -tAc \
		"select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
		 where f->'openssl'->>'version' ~ '^3\.[0-4]([.]|\$)' order by s.seq desc limit 1" 2>/dev/null | tr -d '[:space:]')
fi
[ -n "$NODE" ] || { echo "OpenSSL 3.0–3.4 노드를 찾지 못했다." >&2; exit 1; }
docker inspect "$NODE" >/dev/null 2>&1 || { echo "컨테이너 '$NODE'가 없다." >&2; exit 1; }

SHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')
# 활성화 지점을 읽되 **없어도 죽지 않아야 한다** — 되돌린 뒤에는 이 파일이 사라지고,
# 그때 물어보는 것이 이 촬영의 마지막 컷이다. `.`은 대상이 없으면 비대화형 sh를 종료시키므로
# 반드시 존재 확인을 앞에 둔다(없으면 그냥 기본 설정으로 묻는다 — 그게 원상복귀의 정의다).
ACT='[ -f /etc/pqcota/service.env ] && . /etc/pqcota/service.env; export OPENSSL_CONF;'
KEMQ='openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem || true'
VER=$(docker exec "$NODE" sh -lc 'openssl version' 2>/dev/null)

clear 2>/dev/null || printf '\033[2J\033[H' # TERM이 없는 환경(CI·비대화형)에서도 화면은 지운다
printf '\033[1m대상 노드: %s  (%s)\033[0m\n' "$NODE" "$VER"
sleep "$PAUSE"

# ── 컷 A — 조치 전. 이 노드가 아는 ML-KEM은 몇 개인가 ────────────────────────────
say "조치 전 — 이 노드의 OpenSSL이 아는 ML-KEM 알고리즘"
type_cmd "openssl list -kem-algorithms | grep -ci mlkem"
BEFORE=$(docker exec "$NODE" sh -lc "$KEMQ" | tr -d '[:space:]')
big "$BEFORE"
cut_mark

# ── 컷 B — 도구가 만든 것. 계획 한 장에서 나온 플레이북과 설정 조각 ──────────────
say "도구가 계획에서 만든 것 — 적용용과 되돌림용이 함께 나온다"
type_cmd "ls /work/ansible/provision-real*.yml"
docker exec pqcota-ctl bash -lc 'ls -1 /work/ansible/provision-real*.yml'
cut_mark

# ── 컷 C — 적용. 사용자의 Ansible로 돌린다 ────────────────────────────────────
say "적용 — 생성된 플레이북을 사용자의 Ansible로 (L2 배치 + L3 활성화)"
type_cmd "ansible-playbook -i targets.ini provision-real.yml"
# 한 번만 돌리고 두 가지를 뽑는다 — 무엇을 했는지(TASK)와 결과 요약(recap).
# 임시 파일을 두지 않는 것은 촬영 중 남는 것을 만들지 않기 위해서다.
APPLY=$(docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$SHA provision-real.yml")
printf '%s\n' "$APPLY" | grep -E "^TASK" | sed -E 's/ \*+$//' | tail -6
printf '%s\n' "$APPLY" | recap
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3.yml" |
	recap
cut_mark

# ── 컷 D — 놓인 것은 새 파일 둘뿐. 원본은 건드리지 않았다 ────────────────────────
say "노드에 놓인 것 — 새 파일 둘뿐이다. 기존 openssl.cnf는 건드리지 않았다"
type_cmd "ls /opt/pqcota /etc/pqcota"
docker exec "$NODE" sh -lc 'ls -1 /opt/pqcota /etc/pqcota'
cut_mark

# ── 컷 E — 같은 명령, 다른 답 ────────────────────────────────────────────────
say "조치 후 — 같은 노드에 같은 질문"
type_cmd "openssl list -providers | grep -A2 oqs"
docker exec "$NODE" sh -lc "$ACT openssl list -providers 2>/dev/null | grep -A2 -i oqs | head -4"
sleep 1
type_cmd "openssl list -kem-algorithms | grep -ci mlkem"
AFTER=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
big "$AFTER"
printf '\033[1m   %s개 → %s개.  바뀐 것은 도구가 만든 설정 조각 하나다.\033[0m\n' "$BEFORE" "$AFTER"
cut_mark

# ── 컷 F — 되돌림. 원본을 덮은 적이 없으니 제거가 곧 복원이다 ────────────────────
say "되돌림 — 원본을 덮어쓴 적이 없으므로 파일을 지우는 것이 곧 복원이다"
type_cmd "ansible-playbook -i targets.ini provision-real-l3-rollback.yml provision-real-rollback.yml"
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3-rollback.yml" |
	recap
docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-rollback.yml" |
	recap
sleep 1
type_cmd "openssl list -kem-algorithms | grep -ci mlkem"
BACK=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
big "$BACK"
printf '\033[1m   %s → %s → %s.  적용과 되돌림이 같은 계획에서 대칭으로 나온다.\033[0m\n\n' "$BEFORE" "$AFTER" "$BACK"
