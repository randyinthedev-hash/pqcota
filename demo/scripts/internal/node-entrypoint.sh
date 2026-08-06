#!/usr/bin/env bash
# 데모 노드 엔트리포인트: sshd + 역할별 서비스(암호 워크로드)를 띄우고 컨테이너를 유지한다.
# 역할은 환경변수로 지정: PQC_SERVER / SSL_SERVER / JAVA_APP (compose에서 설정).
set -e
NODE_NAME="${NODE_NAME:-node}"

# ── sshd (Ansible/관측 대상 SSH) ──
echo "PermitRootLogin yes" >>/etc/ssh/sshd_config
echo "PasswordAuthentication no" >>/etc/ssh/sshd_config
/usr/sbin/sshd
echo "[$NODE_NAME] sshd 시작"

# ── 🟢 PQC TLS 서버 (Go, X25519MLKEM768) ──
if [ "${PQC_SERVER:-0}" = "1" ]; then
  pqc-echo server ":8443" &
  echo "[$NODE_NAME] pqc-echo(🟢 X25519MLKEM768) :8443"
fi

# ── 🔴 고전 TLS 서버 (s_server) — SSL_APPS의 각 앱이 같은 libssl 로드 → 공유 .so 다중 귀속(#3) ──
# SSL_APPS: 콤마 목록. 기본은 payment-gw,api-gw(공유 .so 시연). fork 무관하게 openssl CLI를 찾는다.
if [ "${SSL_SERVER:-0}" = "1" ]; then
  # fork별 CLI 이름이 다르다 — LibreSSL은 libressl-openssl. 있는 것을 쓴다.
  OSSL="$(command -v openssl || command -v libressl-openssl || command -v libressl)"
  "$OSSL" req -x509 -newkey rsa:2048 -keyout /tmp/k.pem -out /tmp/c.pem \
    -days 3650 -nodes -subj "/CN=$NODE_NAME" >/dev/null 2>&1
  # openssl 바이너리를 앱 이름으로 복사 → exe 경로가 곧 app_key(§0.5). 같은 .so를 동적 링크하므로
  # ScanHost가 그 .so를 여러 앱으로 **합집합** 귀속한다(공유 라이브러리 교체 영향 반경).
  mkdir -p /opt/apps
  port=4433
  : > /run/pqcota-ssl-apps.map
  IFS=',' read -ra APPS <<< "${SSL_APPS:-payment-gw,api-gw}"
  for app in "${APPS[@]}"; do
    cp "$OSSL" "/opt/apps/$app"
    # 앱→포트 매핑을 기록한다 — 기동·재시작이 같은 매핑을 쓰게(엔드포인트↔앱 귀속이 흔들리지 않게).
    echo "$app $port" >> /run/pqcota-ssl-apps.map
    port=$((port + 1))
  done
  # 기동은 서비스 스크립트에 맡긴다 — 프로비저닝의 activation.restart 훅도 같은 스크립트를 부른다.
  /usr/local/bin/ssl-apps.sh start | sed "s/^/[$NODE_NAME] /"
fi

# ── 배포된 Java 크립토 워크로드 (BouncyCastle 로드, 실 JVM) ──
if [ "${JAVA_APP:-0}" = "1" ] && command -v java >/dev/null 2>&1; then
  if [ -f /opt/CryptoApp.java ]; then
    java --class-path "${JVMSCAN_CP:-/opt/bcprov.jar}" /opt/CryptoApp.java >/tmp/javaapp.log 2>&1 &
    echo "[$NODE_NAME] Java 앱(BouncyCastle) 기동 pid=$!"
  fi
fi

echo "[$NODE_NAME] 준비 완료 — 대기"
tail -f /dev/null
