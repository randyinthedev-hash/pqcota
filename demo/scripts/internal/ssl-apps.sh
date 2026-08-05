#!/usr/bin/env bash
# 이 노드의 고전 TLS 서비스(s_server) 기동·정지 — **환경이 가진 운영 지식**을 한 곳에 둔다.
# 엔트리포인트도, 프로비저닝 계획의 activation 훅도 같은 이 스크립트를 호출한다: 도구는 재시작
# 방법을 추측하지 않고(§2.6), 계획의 activation.restart가 이 스크립트를 가리킨다.
#
# 앱→포트는 엔트리포인트가 기록한 맵에서 읽는다. 재시작이 순서를 다시 정하면 엔드포인트↔앱 귀속이
# 조용히 바뀌므로(관측 결과가 흔들린다) 처음 정한 매핑을 그대로 쓴다.
set -u
MAP=/run/pqcota-ssl-apps.map
ENVF=/etc/pqcota/service.env # 활성화 지점 — 있으면 서비스가 이 설정으로 뜬다(없으면 기본)

stop() {
	pgrep -f 's_server -accept' >/dev/null 2>&1 || return 0
	pkill -f 's_server -accept' 2>/dev/null || true
	for _ in 1 2 3 4 5 6 7 8 9 10; do
		pgrep -f 's_server -accept' >/dev/null 2>&1 || return 0
		sleep 0.2
	done
}

start() {
	[ -f "$MAP" ] || { echo "ssl-apps: $MAP 없음 — 이 노드엔 고전 TLS 앱이 없다"; return 0; }
	# 활성화 지점이 있으면 그 설정으로 띄운다. 프로비저닝이 켠 것이 실제로 서비스에 닿는 경로다.
	if [ -f "$ENVF" ]; then
		set -a
		# shellcheck disable=SC1090
		. "$ENVF"
		set +a
	fi
	while read -r app port; do
		[ -n "${app:-}" ] || continue
		"/opt/apps/$app" s_server -accept "$port" -cert /tmp/c.pem -key /tmp/k.pem -www -quiet \
			>/dev/null 2>&1 &
		echo "ssl-apps: $app :$port 기동 pid=$!${OPENSSL_CONF:+ (OPENSSL_CONF=$OPENSSL_CONF)}"
	done <"$MAP"
}

status() {
	printf 'ssl-apps: 활성화 지점 %s' "$ENVF"
	[ -f "$ENVF" ] && printf ' = %s\n' "$(tr '\n' ' ' <"$ENVF")" || printf ' (없음 — 기본 설정)\n'
	pgrep -af 's_server -accept' 2>/dev/null | sed 's/^/  실행중: /' || echo "  실행중: (없음)"
}

case "${1:-status}" in
stop) stop ;;
start) start ;;
restart)
	stop
	start
	;;
status) status ;;
*)
	echo "usage: ssl-apps.sh {start|stop|restart|status}" >&2
	exit 2
	;;
esac
