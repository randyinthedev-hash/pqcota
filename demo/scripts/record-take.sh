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
# 전제: `./demo/scripts/up.sh` 와 `DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh` 가 한 번 돌아
# 관측 결과와 생성물이 컨트롤러에 있어야 한다. 그것들을 **다시 만들지 않고 그대로 쓰므로**
# 촬영을 몇 번 다시 해도 같은 화면이 나온다. provision 컷은 끝에 롤백해 노드를 원래대로 둔다.
set -euo pipefail

PAUSE="${TAKE_PAUSE:-2}"        # 컷 사이 정지(초). 편집에서 자르기 좋게
TYPE="${TAKE_TYPE_DELAY:-0.05}" # 타이핑 한 글자 간격(초). 0이면 즉시 출력

usage() {
	cat <<'HELP'
사용법: record-take.sh [컷] [노드]

  컷    observe    정적(java.security) vs 런타임(attach) 대조 + 자산·엣지 표
        provision  ML-KEM 0개 → 14개 → 0개 (기본)
        gap        권한이 없어 관측하지 못한 계층을 따로 낸다

  노드  provision 컷의 대상 (기본: 인벤토리에서 OpenSSL 3.0–3.4 노드를 자동 선택)

환경변수:
  TAKE_PAUSE=<초>        컷 사이 정지 (기본 2)
  TAKE_TYPE_DELAY=<초>   타이핑 한 글자 간격 (기본 0.05, 0이면 타이핑 연출 없음)

먼저 ./demo/scripts/up.sh → DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh 를 한 번 돌려 둔다.
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
cut_mark() { printf '\n\033[2m%s\033[0m\n\n' "────────────────────────────────────────────────────────"; sleep "$PAUSE"; }
big() { printf '\n\033[1;33m%s\033[0m\n' "$1"; sleep "$PAUSE"; }

need_ctl() {
	docker inspect pqcota-ctl >/dev/null 2>&1 || { echo "pqcota-ctl이 없다 — up.sh를 먼저 돌린다." >&2; exit 1; }
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
	jnode=$(docker exec pqcota-ctl bash -lc 'ls -1 /work/results/*-jca.json 2>/dev/null | head -1' |
		xargs -r basename | sed 's/-jca\.json$//' | tr -d '[:space:]')
	[ -n "$jnode" ] || { echo "JCA 관측 결과가 없다 — demo.sh를 먼저 돌린다." >&2; exit 1; }
	jsec=$(docker exec "$jnode" sh -lc 'ls /opt/java/*/conf/security/java.security /usr/lib/jvm/*/conf/security/java.security 2>/dev/null | head -1' | tr -d '[:space:]')

	clear 2>/dev/null || printf '\033[2J\033[H'
	printf '\033[1m관측 대상: %s (JVM이 도는 노드)\033[0m\n' "$jnode"
	sleep "$PAUSE"

	say "정적으로 보면 — java.security에 등록된 provider 목록"
	type_cmd "grep '^security.provider' $jsec"
	docker exec "$jnode" sh -lc "grep '^security.provider' $jsec | head -12"
	printf '\n'
	type_cmd "grep -ci bouncycastle $jsec"
	docker exec "$jnode" sh -lc "grep -ci bouncycastle $jsec || true"
	note "   BouncyCastle은 이 파일 어디에도 없다."
	cut_mark

	say "실행 중인 JVM에 attach해서 물으면 — 같은 노드, 같은 시각"
	type_cmd "pqcota-discover-view /work/results"
	# 범위의 끝줄(다음 절 제목)은 빼고 낸다 — 남기면 엣지 절 제목이 두 번 나온다.
	docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results 2>/dev/null' |
		sed -n '/발견 자산/,/관측 통신 엣지/p' | sed '$d' | head -14
	note "   체인 마지막의 BC — 앱이 실행 중에 addProvider()로 등록한 것이다."
	note "   파일을 아무리 읽어도 안 나온다. 이것이 런타임 관측의 이유다."
	cut_mark

	say "회선에서 실제로 협상된 것 — 복호화 없이 핸드셰이크만 본다"
	docker exec pqcota-ctl bash -lc 'pqcota-discover-view /work/results 2>/dev/null' |
		sed -n '/관측 통신 엣지/,/posture 합계/p' | head -10
	note "   같은 게이트웨이인데 상대에 따라 갈린다 — 능력이 아니라 협상 결과다."
}

# ─────────────────────────────────────────────────────────────────────────────
# 컷 2 — 도구가 만든 config가 실제 암호 알고리즘으로 반영되고, 되돌리면 원상복귀한다
# ─────────────────────────────────────────────────────────────────────────────
take_provision() {
	need_ctl
	docker exec pqcota-ctl test -f /work/ansible/provision-real.yml 2>/dev/null ||
		{ echo "생성물이 없다 — DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh 를 한 번 돌린다." >&2; exit 1; }

	local NODE="${1:-}"
	if [ -z "$NODE" ]; then
		NODE=$(docker exec pqcota-demo-pg psql -U postgres -d pqcota -tAc \
			"select s.node_id from pqcota_snapshots s, jsonb_array_elements(s.findings) f
			 where f->'openssl'->>'version' ~ '^3\.[0-4]([.]|\$)' order by s.seq desc limit 1" 2>/dev/null | tr -d '[:space:]')
	fi
	[ -n "$NODE" ] || { echo "OpenSSL 3.0–3.4 노드를 찾지 못했다." >&2; exit 1; }
	docker inspect "$NODE" >/dev/null 2>&1 || { echo "컨테이너 '$NODE'가 없다." >&2; exit 1; }

	local SHA VER BEFORE AFTER BACK APPLY
	SHA=$(docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -d" " -f1' | tr -d '[:space:]')
	# 활성화 지점을 읽되 **없어도 죽지 않아야 한다** — 되돌린 뒤에는 이 파일이 사라지고,
	# 그때 물어보는 것이 이 촬영의 마지막 컷이다. `.`은 대상이 없으면 비대화형 sh를 종료시키므로
	# 반드시 존재 확인을 앞에 둔다(없으면 그냥 기본 설정으로 묻는다 — 그게 원상복귀의 정의다).
	local ACT='[ -f /etc/pqcota/service.env ] && . /etc/pqcota/service.env; export OPENSSL_CONF;'
	local KEMQ='openssl list -kem-algorithms 2>/dev/null | grep -ci mlkem || true'
	VER=$(docker exec "$NODE" sh -lc 'openssl version' 2>/dev/null)

	clear 2>/dev/null || printf '\033[2J\033[H'
	printf '\033[1m대상 노드: %s  (%s)\033[0m\n' "$NODE" "$VER"
	sleep "$PAUSE"

	say "조치 전 — 이 노드의 OpenSSL이 아는 ML-KEM 알고리즘"
	type_cmd "openssl list -kem-algorithms | grep -ci mlkem"
	BEFORE=$(docker exec "$NODE" sh -lc "$KEMQ" | tr -d '[:space:]')
	big "$BEFORE"
	cut_mark

	# ★ 이 세 컷이 "우리가 만든다"를 보인다. 없으면 영상이 앤서블 사용법으로 읽힌다 —
	#   플레이북만 돌리는 화면은 이 도구가 무엇을 했는지 말해 주지 않는다.
	say "사람이 쓰는 것은 계획 파일 하나뿐이다 — 어느 노드에 어떤 provider를 넣을지 적는다"
	type_cmd "cat plan-real.json"
	docker exec pqcota-ctl bash -lc "python3 -m json.tool --no-ensure-ascii /work/plan-real.json | head -18"
	cut_mark

	# 사람이 준비하는 것은 둘이다 — 계획과 **모듈 파일**. 이 컷이 없으면 .so가 어디선가
	# 튀어나온 것처럼 보이고, "도구가 provider도 준다"는 오해가 남는다(§4.2 — 선택·조달은 사용자).
	say "그리고 provider 모듈 — 도구가 주지 않는다. 사용자가 빌드하거나 벤더에서 받아 둔다"
	type_cmd "ls -l ansible/files/oqsprovider.so"
	docker exec pqcota-ctl bash -lc 'ls -l /work/ansible/files/oqsprovider.so | sed "s|/work/ansible/files/||"'
	type_cmd "sha256sum ansible/files/oqsprovider.so"
	docker exec pqcota-ctl bash -lc 'sha256sum /work/ansible/files/oqsprovider.so | cut -c1-24' | sed 's/$/…/'
	note "   이 해시를 플레이북에 넘긴다 — 배치된 것이 그 파일이 아니면 멈춘다."
	cut_mark

	say "도구가 그 계획에서 배포물을 만든다 — 적용용과 되돌림용이 함께"
	type_cmd "pqcota-provision --level l2 plan-real.json > provision-real.yml"
	docker exec pqcota-ctl bash -lc "pqcota-provision --level l2 /work/plan-real.json > /work/ansible/provision-real.yml" 2>&1 | sed 's/^/   /'
	docker exec pqcota-ctl bash -lc 'ls -1 /work/ansible/provision-real*.yml'
	cut_mark

	say "만들어진 것 — 사람이 쓴 적 없는 config 조각과 무결성 게이트가 들어 있다"
	type_cmd "grep -A3 'config 조각\|sha256' provision-real.yml"
	docker exec pqcota-ctl bash -lc "grep -E 'name:|module|sha256|state:' /work/ansible/provision-real.yml | head -10"
	cut_mark

	say "적용 — 생성된 플레이북을 사용자의 Ansible로 (L2 배치 + L3 활성화)"
	type_cmd "ansible-playbook -i targets.ini provision-real.yml"
	# 한 번만 돌리고 두 가지를 뽑는다 — 무엇을 했는지(TASK)와 결과 요약(recap).
	# 두 번 돌리면 두 번째는 changed=0이라 "아무것도 안 바뀐 것"처럼 보인다(멱등).
	APPLY=$(docker exec pqcota-ctl bash -lc "$ANS-playbook $INV -e pqcota_module_sha256_oqsprovider=$SHA provision-real.yml")
	printf '%s\n' "$APPLY" | grep -E "^TASK" | sed -E 's/ \*+$//' | tail -6
	printf '%s\n' "$APPLY" | recap
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3.yml" | recap
	cut_mark

	# L3까지 돌린 뒤라 활성화 지점(service.env)도 함께 놓인다 — "둘"이라고 적으면 화면과 어긋난다.
	say "노드에 놓인 것 — provider 모듈(.so) · 설정 조각(.cnf) · 활성화 지점(service.env) 셋뿐이다"
	type_cmd "ls /opt/pqcota /etc/pqcota"
	docker exec "$NODE" sh -lc 'ls -1 /opt/pqcota /etc/pqcota'
	note "   기존 /etc/ssl/openssl.cnf는 열지도 않았다 — 그래서 되돌림이 파일 제거로 끝난다."
	cut_mark

	# ★ 파일만 놓아서는 아무것도 안 바뀐다. 그 조각을 **읽게 만드는 것**이 L3 활성화이고,
	#   그 방법은 환경마다 달라 계획의 activation 훅에 사용자가 적는다. 이 컷이 없으면
	#   "파일을 떨구니 능력이 생겼다"로 읽혀 기전이 마술처럼 보인다.
	say "그런데 파일만 놓으면 아무것도 안 바뀐다 — 그 조각을 읽게 만들어야 한다"
	type_cmd "cat /etc/pqcota/service.env"
	docker exec "$NODE" sh -lc 'cat /etc/pqcota/service.env 2>/dev/null || echo "(L2까지만 — 활성화 안 됨)"'
	note "   L3 활성화가 이 한 줄을 쓰고 서비스를 재시작했다. 명령은 계획의 activation 훅에"
	note "   사용자가 적은 것이다 — 활성화 지점은 환경마다 달라 도구가 추측하지 않는다."
	cut_mark

	say "조치 후 — 그 설정을 가리킨 채로, 같은 노드에 같은 질문"
	type_cmd "OPENSSL_CONF=/etc/pqcota/openssl-pqc.cnf openssl list -providers | grep -A2 oqs"
	docker exec "$NODE" sh -lc "$ACT openssl list -providers 2>/dev/null | grep -A2 -i oqs | head -4"
	sleep 1
	type_cmd "OPENSSL_CONF=/etc/pqcota/openssl-pqc.cnf openssl list -kem-algorithms | grep -ci mlkem"
	AFTER=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
	big "$AFTER"
	note "   ${BEFORE}개 → ${AFTER}개.  바뀐 것은 도구가 만든 설정 조각 하나와, 그것을 가리킨 한 줄이다."
	cut_mark

	say "되돌림 — 원본을 덮어쓴 적이 없으므로 파일을 지우는 것이 곧 복원이다"
	type_cmd "ansible-playbook -i targets.ini provision-real-l3-rollback.yml provision-real-rollback.yml"
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-l3-rollback.yml" | recap
	docker exec pqcota-ctl bash -lc "$ANS-playbook $INV provision-real-rollback.yml" | recap
	sleep 1
	type_cmd "OPENSSL_CONF=… openssl list -kem-algorithms | grep -ci mlkem   # 조각이 사라졌다"
	BACK=$(docker exec "$NODE" sh -lc "$ACT $KEMQ" | tr -d '[:space:]')
	big "$BACK"
	note "   ${BEFORE} → ${AFTER} → ${BACK}.  적용과 되돌림이 같은 계획에서 대칭으로 나온다."
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
	printf '\033[1m관측 권한(CAP_NET_RAW)이 없는 상황에서\033[0m\n'
	sleep "$PAUSE"

	say "핸드셰이크 관측을 시도한다 — 권한 없이"
	type_cmd "pqcota-netcap demo-node eth0 2"
	# stdout(결과 JSON)은 버리고 **stderr만** 보인다 — 둘을 섞으면 JSON 조각이 사람에게
	# 하는 말 사이에 끼어 화면이 지저분해진다. 결과 본문은 다음 컷에서 따로 보인다.
	docker exec -u nobody pqcota-ctl bash -lc \
		'/work/dist/linux-amd64/pqcota-netcap demo-node eth0 2 >/dev/null' 2>&1 | head -5
	cut_mark

	say "그런데 수집은 실패가 아니라 정상 종료로 끝난다 — 결과에는 이것이 담긴다"
	type_cmd "echo \$?"
	docker exec -u nobody pqcota-ctl bash -lc \
		'/work/dist/linux-amd64/pqcota-netcap demo-node eth0 2 >/dev/null 2>&1; echo "$?"'
	printf '\n'
	type_cmd "pqcota-netcap … | jq .completeness"
	docker exec -u nobody pqcota-ctl bash -lc \
		'/work/dist/linux-amd64/pqcota-netcap demo-node eth0 2 2>/dev/null | python3 -c "
import json,sys
d=json.load(sys.stdin)
print(json.dumps(d.get(\"completeness\",{}), ensure_ascii=False, indent=1))"'
	printf '\n'
	note "   layersMissing = [NETWORK].  \"엣지 0개\"가 아니라 \"이 계층을 관측하지 못했다\"이다."
	note "   실패가 아니라 정상 종료로 보고하는 이유 — 오류로 끝내면 이 기록이 중앙까지 가지"
	note "   못하고, 인벤토리에는 '이 노드엔 링크가 없다'로 읽힌다. 없는 것과는 다른 사실이다."
}

case "${1:-provision}" in
-h | --help) usage; exit 0 ;;
observe) take_observe ;;
provision) take_provision "${2:-}" ;;
gap) take_gap ;;
*) echo "record-take.sh: 모르는 컷 '$1' — observe · provision · gap 중 하나다." >&2; usage >&2; exit 2 ;;
esac
