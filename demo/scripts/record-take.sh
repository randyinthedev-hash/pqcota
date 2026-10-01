#!/usr/bin/env bash
# 시연영상 촬영용 — 데모가 보이는 것 중 **근거가 되는 장면만** 떼어 또박또박 보인다.
#
# demo.sh와 무엇이 다른가: demo.sh는 6단계를 한 번에 흘려보내 화면이 빨리 지나가고, 어느 줄이
# 근거인지 영상에서 짚기 어렵다. 이 스크립트는 컷 하나만 남기고 **명령을 먼저 보인 뒤 잠깐
# 멈추고 결과를 낸다.** 편집에서 잘라 붙이기 좋도록 컷 경계마다 구분선을 찍는다.
#
# 컷 셋 — 각각이 영상에서 하나의 주장을 증명한다:
#   observe    정적 스캔으로는 관측되지 않는 provider를 런타임 관측이 잡는다
#   provision  도구가 만든 config가 실제 암호 알고리즘으로 반영되고, 되돌리면 원상복귀한다
#   gap        관측하지 못한 것을 "없음"으로 적지 않는다
#
# 전제: `./demo/scripts/up.sh`와 `DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh`가 한 번 돌아
# 관측 결과와 생성물이 컨트롤러에 있어야 한다. 그것들을 **다시 만들지 않고 그대로 쓰므로**
# 촬영을 몇 번 다시 해도 같은 화면이 나온다. provision 컷은 끝에 롤백해 노드를 원래대로 둔다.
set -euo pipefail

PAUSE="${TAKE_PAUSE:-2}"        # 컷 사이 정지(초). 편집에서 자르기 좋게
TYPE="${TAKE_TYPE_DELAY:-0.05}" # 타이핑 한 글자 간격(초). 0이면 즉시 출력

usage() {
	cat <<'HELP'
usage: record-take.sh [cut] [node]

  cut   observe    static (java.security) vs runtime (attach), plus the asset and edge tables
        provision  ML-KEM 0 → 14 → 0 (default)
        gap        shows a layer that could not be observed for lack of permission

  node  target of the provision cut (default: an OpenSSL 3.0-3.4 node picked from the inventory)

environment:
  TAKE_PAUSE=<sec>       pause between cuts (default 2)
  TAKE_TYPE_DELAY=<sec>  delay per typed character (default 0.05; 0 disables the typing effect)

Run ./demo/scripts/up.sh and then DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh once first.
HELP
}

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
note() { printf '\033[1m%s\033[0m\n' "$1"; }

# Ansible의 PLAY RECAP 한 줄은 100자를 넘어 화면에서 접힌다 — 접힌 줄은 영상에서 지저분하고,
# 정작 봐야 하는 것은 `changed`(무엇을 바꿨나)와 `failed=0`(깨지지 않았나) 둘뿐이다.
# 나머지 칸(unreachable·skipped·rescued·ignored)은 이 촬영에서 늘 0이라 정보가 없다.
recap() { sed -nE 's/^([^ ]+) +: +(ok=[0-9]+) +(changed=[0-9]+).* (failed=[0-9]+).*/  \1  \2 \3 \4/p'; }
# 구간 표지 — 영상 조립이 컷 경계를 찾는 근거다. 화면에는 보이지 않는 OSC 시퀀스를 찍어 녹화본(.cast)에
# 이름과 시각이 남게 한다. 표지 이름은 "그 구간이 끝나는 곳"이다(예: provision.plan = 계획 보이기가 끝난 곳).
# 이름은 recording/build.py의 편집표가 그대로 쓰므로 바꾸면 편집표도 함께 바꾼다.
mark() { printf '\033]777;pqcota-mark;%s\a' "$1"; }
cut_mark() { mark "$1"; printf '\n\033[2m%s\033[0m\n\n' "────────────────────────────────────────────────────────"; sleep "$PAUSE"; }
big() { printf '\n\033[1;33m%s\033[0m\n' "$1"; sleep "$PAUSE"; }

need_ctl() {
	docker inspect pqcota-ctl >/dev/null 2>&1 || { echo "pqcota-ctl is not running — run up.sh first." >&2; exit 1; }
}

# ─────────────────────────────────────────────────────────────────────────────
# 컷 1 — 정적 스캔에 잡히지 않는 것을 런타임 관측이 잡는다
#
# 이 대조가 이 도구의 첫 번째 근거다. java.security에는 없는 provider가 실행 중 체인에는
# 있다 — 앱이 `Security.addProvider()`로 등록했기 때문이고, 파일만 읽어서는 알 수 없다.
# ─────────────────────────────────────────────────────────────────────────────
take_observe() {
	need_ctl
	local jnode jsec
	jnode=$(docker exec pqcota-ctl bash -lc 'ls -1 /work/results/*-jca.jsonl 2>/dev/null | head -1' |
		xargs -r basename | sed 's/-jca\.jsonl$//' | tr -d '[:space:]')
	[ -n "$jnode" ] || { echo "no JCA observation yet — run demo.sh first." >&2; exit 1; }
	jsec=$(docker exec "$jnode" sh -lc 'ls /opt/java/*/conf/security/java.security /usr/lib/jvm/*/conf/security/java.security 2>/dev/null | head -1' | tr -d '[:space:]')

	clear 2>/dev/null || printf '\033[2J\033[H'
	printf '\033[1mnode: %s (runs a JVM) — the results below were collected earlier\033[0m\n' "$jnode"
	sleep "$PAUSE"

	say "statically — the provider list registered in java.security"
	type_cmd "grep '^security.provider' $jsec"
	docker exec "$jnode" sh -lc "grep '^security.provider' $jsec | head -12"
	printf '\n'
	type_cmd "grep -ci bouncycastle $jsec"
	docker exec "$jnode" sh -lc "grep -ci bouncycastle $jsec || true"
	note "   BouncyCastle appears nowhere in this file."
	cut_mark observe.static

	say "what the collector found earlier by attaching to the running JVM — read back from the stored results"
	type_cmd "pqcota-discover-view /work/results"
	# 범위의 끝줄(다음 절 제목)은 빼고 낸다 — 남기면 엣지 절 제목이 두 번 나온다.
	docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results 2>/dev/null' |
		sed -n '/discovered assets/,/observed edges/p' | sed '$d' | head -14
	note "   the BC at the end of the chain: the app registered it at runtime with addProvider()."
	note "   it is not in this java.security, so reading the file would not show it. Observing the running JVM does."
	cut_mark observe.chain

	say "the connections observed earlier — what was negotiated in the handshake, no decryption"
	docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results 2>/dev/null' |
		sed -n '/observed edges/,/grade totals/p' | head -10
	note "   the same gateway splits by peer — this is the negotiated result, not a capability."
	mark observe.edges
}

# ─────────────────────────────────────────────────────────────────────────────
# 컷 2 — 도구가 만든 config가 실제 암호 알고리즘으로 반영되고, 되돌리면 원상복귀한다
# ─────────────────────────────────────────────────────────────────────────────
take_provision() {
	need_ctl
	docker exec pqcota-ctl test -f /work/ansible/provision-real.yml 2>/dev/null ||
		{ echo "nothing has been generated — run DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh once." >&2; exit 1; }

	local NODE="${1:-}"
	if [ -z "$NODE" ]; then
		NODE=$(docker exec pqcota-demo-pg psql -U postgres -d pqcota -tAc \
			"select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
			 where f->'openssl'->>'version' ~ '^3\.[0-4]([.]|\$)' order by s.seq desc limit 1" 2>/dev/null | tr -d '[:space:]')
	fi
	[ -n "$NODE" ] || { echo "no OpenSSL 3.0-3.4 node was found." >&2; exit 1; }
	docker inspect "$NODE" >/dev/null 2>&1 || { echo "there is no container '$NODE'." >&2; exit 1; }

	local SHA VER BEFORE AFTER BACK APPLY
	SHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')
	# 활성화 지점을 읽되 **없어도 죽지 않아야 한다** — 되돌린 뒤에는 이 파일이 사라지고,
	# 그때 물어보는 것이 이 촬영의 마지막 컷이다. `.`은 대상이 없으면 비대화형 sh를 종료시키므로
	# 반드시 존재 확인을 앞에 둔다(없으면 그냥 기본 설정으로 묻는다 — 그게 원상복귀의 정의다).
	local ACT='[ -f /etc/pqcota/service.env ] && . /etc/pqcota/service.env; export OPENSSL_CONF;'
	local KEMQ='openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem || true'
	VER=$(docker exec "$NODE" sh -lc 'openssl version' 2>/dev/null)

	clear 2>/dev/null || printf '\033[2J\033[H'
	printf '\033[1mtarget node: %s  (%s)\033[0m\n' "$NODE" "$VER"
	sleep "$PAUSE"

	say "before — how many ML-KEM-related entries this node's OpenSSL lists"
	type_cmd "openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem"
	BEFORE=$(docker exec "$NODE" sh -lc "$KEMQ" | tr -d '[:space:]')
	big "$BEFORE"
	cut_mark provision.before

	# ★ 이 세 컷이 "우리가 만든다"를 보인다. 없으면 영상이 앤서블 사용법으로 읽힌다 —
	#   플레이북만 돌리는 화면은 이 도구가 무엇을 했는지 말해 주지 않는다.
	say "a person writes one thing: the plan — which provider goes on which node"
	type_cmd "cat plan-real.json"
	docker exec pqcota-ctl bash -lc "python3 -m json.tool --no-ensure-ascii /work/plan-real.json | head -18"
	cut_mark provision.plan

	# 승인은 데모가 pqcota-approve로 서명해 둔 것이다. 생성기는 FINALIZED가 아니거나 서명이 없는
	# 계획에서는 아무것도 만들지 않는다(Executable 관문). 그 사실을 계획의 필드로 보인다.
	say "the plan is finalized and carries an approval signature — the generator refuses a plan without both"
	type_cmd "python3 -m json.tool plan-real.json | grep -E 'status|reviewer-1' | cut -c1-72"
	docker exec pqcota-ctl bash -lc "python3 -m json.tool /work/plan-real.json | grep -E 'status|reviewer-1' | cut -c1-72"
	note "   the demo signs the plan with a local approver key (pqcota-approve). A real deployment checks it"
	note "   against the approver's registered public key."
	cut_mark provision.approve

	# 사람이 준비하는 것은 둘이다 — 계획과 **모듈 파일**. 이 컷이 없으면 .so가 어디선가
	# 튀어나온 것처럼 보이고, "도구가 provider도 준다"는 오해가 남는다(§4.2 — 선택·조달은 사용자).
	say "and the provider module — the tool does not supply it; you build it or get it from a vendor"
	type_cmd "ls -l ansible/files/oqsprovider.so"
	docker exec pqcota-ctl bash -lc 'ls -l /work/ansible/files/oqsprovider.so | sed "s|/work/ansible/files/||"'
	type_cmd "sha256sum ansible/files/oqsprovider.so"
	docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -c1-24' | sed 's/$/…/'
	note "   this hash goes to the playbook — if what lands is not that file, it stops."
	cut_mark provision.module

	say "the tool generates the artifacts from that plan — one to apply, one to roll back"
	type_cmd "pqcota-provision --level l2 --allow-incomplete plan-real.json > provision-real.yml"
	docker exec pqcota-ctl bash -lc "pqcota-provision --level l2 --allow-incomplete /work/plan-real.json > /work/ansible/provision-real.yml" 2>&1 | sed 's/^/   /'
	docker exec pqcota-ctl bash -lc 'ls -1 /work/ansible/provision-real*.yml'
	cut_mark provision.generate

	say "what came out — a config fragment nobody wrote by hand, and an integrity gate"
	type_cmd "grep -A3 'config fragment\|sha256' provision-real.yml"
	docker exec pqcota-ctl bash -lc "grep -E 'name:|module|sha256|state:' /work/ansible/provision-real.yml | head -10"
	cut_mark provision.output

	say "apply — your own Ansible runs the generated playbooks: L2 staging first, then L3 activation"
	type_cmd "ansible-playbook -i targets.ini -i groups.ini -e pqcota_module_sha256_oqsprovider=\$SHA provision-real.yml"
	# 한 번만 돌리고 두 가지를 뽑는다 — 무엇을 했는지(TASK)와 결과 요약(recap).
	# 두 번 돌리면 두 번째는 changed=0이라 "아무것도 안 바뀐 것"처럼 보인다(멱등).
	APPLY=$(docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$SHA provision-real.yml")
	printf '%s\n' "$APPLY" | grep -E "^TASK" | sed -E 's/ \*+$//' | tail -6
	printf '%s\n' "$APPLY" | recap
	note "   \$SHA = the sha256 shown above"
	type_cmd "ansible-playbook -i targets.ini -i groups.ini provision-real-l3.yml"
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3.yml" | recap
	cut_mark provision.apply

	# L3까지 돌린 뒤라 활성화 지점(service.env)도 함께 놓인다 — "둘"이라고 적으면 화면과 어긋난다.
	say "what landed on the node — three things: the provider module (.so), the config fragment (.cnf), the activation point (service.env)"
	type_cmd "ls /opt/pqcota /etc/pqcota"
	docker exec "$NODE" sh -lc 'ls -1 /opt/pqcota /etc/pqcota'
	note "   the existing /etc/ssl/openssl.cnf was never opened: nothing that was already there was overwritten."
	cut_mark provision.landed

	# ★ 파일만 놓아서는 아무것도 안 바뀐다. 그 조각을 **읽게 만드는 것**이 L3 활성화이고,
	#   그 방법은 환경마다 달라 계획의 activation 훅에 사용자가 적는다. 이 컷이 없으면
	#   "파일을 떨구니 능력이 생겼다"로 읽혀 기전이 마술처럼 보인다.
	say "but staging files alone changes nothing — something has to read that fragment"
	type_cmd "cat /etc/pqcota/service.env"
	docker exec "$NODE" sh -lc 'cat /etc/pqcota/service.env 2>/dev/null || echo "(L2 only — not activated)"'
	note "   L3 activation wrote this one line and restarted the service. The commands come from the"
	note "   plan's activation hooks, written by you — activation points differ per environment, so the tool does not guess."
	cut_mark provision.activation

	say "after — the same question to the same node, now pointing at that configuration"
	type_cmd "OPENSSL_CONF=/etc/pqcota/openssl-pqc.cnf openssl list -providers | grep -A2 -i oqs"
	docker exec "$NODE" sh -lc "$ACT openssl list -providers 2>/dev/null | grep -A2 -i oqs | head -4"
	sleep 1
	type_cmd "OPENSSL_CONF=/etc/pqcota/openssl-pqc.cnf openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem"
	AFTER=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
	big "$AFTER"
	note "   ${BEFORE} → ${AFTER}: ML-KEM-related entries in OpenSSL's list when started with that configuration."
	note "   this does not show that any connection negotiated post-quantum cryptography."
	cut_mark provision.after

	say "rollback — undo the activation first, then remove what was staged"
	type_cmd "ansible-playbook -i targets.ini -i groups.ini provision-real-l3-rollback.yml"
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3-rollback.yml" | recap
	type_cmd "ansible-playbook -i targets.ini -i groups.ini provision-real-rollback.yml"
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-rollback.yml" | recap
	note "   the activation line is removed and the service restarted, then the staged files are removed."
	note "   nothing original was overwritten, so there is nothing to restore."
	mark provision.deactivated
	sleep 1
	type_cmd "openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem   # default configuration again"
	BACK=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
	big "$BACK"
	note "   ${BEFORE} → ${AFTER} → ${BACK}: the same plan produced both the apply and the rollback."
	mark provision.rollback
}

# ─────────────────────────────────────────────────────────────────────────────
# 컷 3 — 관측하지 못한 것을 "없음"으로 적지 않는다
#
# 권한이 없어 관측하지 못한 상황을 **실제로** 만든다(비특권 사용자로 실행). 종료코드가 0인
# 것이 요점이다 — 1이면 오케스트레이터가 실패로 처리해 결과를 회수하지 않고, 중앙에는 그
# 노드 기록이 아무것도 남지 않아 "링크가 없다"로 읽힌다.
# ─────────────────────────────────────────────────────────────────────────────
take_gap() {
	need_ctl
	clear 2>/dev/null || printf '\033[2J\033[H'
	printf '\033[1mwith no observation privilege (CAP_NET_RAW)\033[0m\n'
	sleep "$PAUSE"

	NETCAP='/work/dist/linux-amd64/pqcota-netcap'
	say "attempting to observe handshakes without the privilege — one run; stderr, the result and the exit code are kept"
	type_cmd "pqcota-netcap demo-node eth0 2 > result.json 2> stderr.txt; echo \$?"
	# 한 번만 실행한다. 아래 세 화면(stderr·종료코드·결과)은 모두 이 한 번의 실행에서 나온 것이다.
	docker exec -u nobody pqcota-ctl bash -lc \
		"cd /tmp && $NETCAP demo-node eth0 2 > result.json 2> stderr.txt; echo \"\$?\""
	printf '\n'
	type_cmd "head -5 stderr.txt"
	docker exec -u nobody pqcota-ctl bash -lc 'head -5 /tmp/stderr.txt'
	cut_mark gap.attempt

	say "the exit status above is 0 — the collection ends normally — and this is what the result carries"
	type_cmd "python3 -m json.tool --no-ensure-ascii result.json | grep -A6 completeness"
	docker exec -u nobody pqcota-ctl bash -lc 'python3 -m json.tool --no-ensure-ascii /tmp/result.json | grep -A6 completeness'
	docker exec -u nobody pqcota-ctl bash -lc 'rm -f /tmp/result.json /tmp/stderr.txt'
	printf '\n'
	note "   layersMissing = [NETWORK].  Not \"zero edges\" but \"this layer was not observed\"."
	note "   why it reports success rather than failure — an error exit would keep this record from reaching"
	note "   the centre, and the inventory would read it as 'this node has no links'. That is a different fact."
	mark gap.result
}

case "${1:-provision}" in
-h | --help) usage; exit 0 ;;
observe) take_observe ;;
provision) take_provision "${2:-}" ;;
gap) take_gap ;;
*) echo "record-take.sh: unknown cut '$1' — use observe, provision or gap." >&2; usage >&2; exit 2 ;;
esac
