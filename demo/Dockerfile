# pqcota 디스커버리 데모 — OSS, 단일 리포로 완결(Docker만 있으면 됨).
# 컨트롤러·저장소·타깃 노드를 한 네트워크에 묶어 OpenSSL·JCA·통신 핸드셰이크를 실측 디스커버리한다.
# (노드 수·구성은 기본 데모가 고정, demo/topology는 사용자 명세로 생성 — 스테이지는 양쪽이 공유.)
# (선언 대비 3-상태 대조·토폴로지는 이 리포 범위 밖. 여기선 관측 등급까지.)

# node-openssl base = 그 배포판의 OpenSSL 버전. 글로벌 ARG는 첫 FROM 앞에 와야 한다.
# 기본은 24.04(digest 핀 — 기본 데모 결정론 유지). 커스텀 토폴로지가 20.04(1.1.1)·22.04(3.0)를 넘긴다.
ARG OPENSSL_BASE=ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90

# ─────────────────────────── builder (데모 소품 전용) ───────────────────────────
# ★ 여기서 **pqcota 소프트웨어를 빌드하지 않는다** — 그건 ctl 머신이 한다(실사용과 같게).
# 이 스테이지가 내는 둘은 도구가 아니라 데모의 소품이다:
#   pqc-echo — 관측 **대상** 워크로드(현실에선 고객의 앱). 노드 이미지에 미리 들어 있어야 한다.
#   topogen  — topology.yaml → compose 생성기. 컨테이너가 서기 전에 필요하다.
# 둘 다 contracts에 의존하지 않아 buf 없이 빌드된다.
FROM golang:1.26@sha256:b900de91b15b2e2953d930ece1d0ecff0a1590ab2006088d20dcf0f56f1e979f AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/ ./demo/workloads/pqc-echo ./demo/topology/topogen

# ─────────────────────────── node-openssl ───────────────────────────
FROM ${OPENSSL_BASE} AS node-openssl
RUN apt-get update && apt-get install -y --no-install-recommends \
      openssh-server openssl ca-certificates iproute2 iputils-ping curl python3 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /run/sshd /root/.ssh && chmod 700 /root/.ssh
# ★ collector는 여기 없다 — Ansible이 반입한다. 노드엔 관측 대상(워크로드)만.
COPY --from=builder /out/pqc-echo /usr/local/bin/pqc-echo
COPY demo/scripts/internal/node-entrypoint.sh /usr/local/bin/node-entrypoint.sh
COPY demo/scripts/internal/pqcota-gen-traffic.sh /usr/local/bin/pqcota-gen-traffic.sh
COPY demo/scripts/internal/pqcota-observe.sh /usr/local/bin/pqcota-observe.sh
COPY demo/scripts/internal/ssl-apps.sh /usr/local/bin/ssl-apps.sh
RUN chmod +x /usr/local/bin/node-entrypoint.sh /usr/local/bin/pqcota-gen-traffic.sh /usr/local/bin/pqcota-observe.sh
ENTRYPOINT ["/usr/local/bin/node-entrypoint.sh"]

# ─────────────────────────── node-libressl (다른 fork) ───────────────────────────
# LibreSSL을 로드하는 노드 — discovery의 fork 판별(§1.2, 같은 soname 다른 fork)을 보인다.
# Alpine의 libressl 패키지를 쓴다(openssl 호환 CLI 포함). 정적 Go 바이너리라 musl에서도 돈다.
FROM alpine:3.19 AS node-libressl
RUN apk add --no-cache bash libressl openssh openssh-server iproute2 iputils python3 curl \
 && mkdir -p /run/sshd /root/.ssh && chmod 700 /root/.ssh \
 && ssh-keygen -A \
 # LibreSSL CLI(libressl-openssl)는 /etc/ssl/libressl.cnf를 찾는다 — req -x509에 [req] 섹션 필요.
 && mkdir -p /etc/ssl && printf '[req]\ndistinguished_name=dn\n[dn]\n' > /etc/ssl/libressl.cnf
# ★ collector는 여기 없다 — Ansible이 반입한다. 노드엔 관측 대상(워크로드)만.
COPY --from=builder /out/pqc-echo /usr/local/bin/pqc-echo
COPY demo/scripts/internal/node-entrypoint.sh /usr/local/bin/node-entrypoint.sh
COPY demo/scripts/internal/pqcota-gen-traffic.sh /usr/local/bin/pqcota-gen-traffic.sh
COPY demo/scripts/internal/pqcota-observe.sh /usr/local/bin/pqcota-observe.sh
COPY demo/scripts/internal/ssl-apps.sh /usr/local/bin/ssl-apps.sh
RUN chmod +x /usr/local/bin/node-entrypoint.sh /usr/local/bin/pqcota-gen-traffic.sh /usr/local/bin/pqcota-observe.sh
ENTRYPOINT ["/usr/local/bin/node-entrypoint.sh"]

# ─────────────────────────── node-java (JCA/BouncyCastle) ───────────────────────────
FROM eclipse-temurin:21-jdk@sha256:1eeacc8c295ed4805f6ffead2417b1936aad296b02ea9e56b457230befc9e98d AS node-java
RUN apt-get update && apt-get install -y --no-install-recommends \
      openssh-server openssl ca-certificates iproute2 iputils-ping curl python3 \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir -p /run/sshd /root/.ssh && chmod 700 /root/.ssh
# BC는 java.security에 **정적으로 심지 않는다** — CryptoApp이 런타임에 addProvider로만 등록한다.
# 그래야 정적 스캔(프로브)으론 안 보이고 **attach로만** 잡히는 실체가 되어, 정찰→attach의 가치를 보인다.
# 버전·해시를 고정하고 **받은 것을 확인한다** — 생성 플레이북에는 sha256 게이트를 넣으라고 하면서
# 정작 데모가 무검증으로 받으면 앞뒤가 안 맞는다. 1.80+를 쓰는 이유: 이 세대부터 ML-KEM이
# BouncyCastleProvider(우리 조각이 등록하는 그 클래스)에 있다.
ARG BC_VERSION=1.85
ARG BC_SHA256=20af26bf6060bb8005cc2389916812c1e0e998dc48d2ced7131b89461b54cff7
RUN curl -fsSL -o /opt/bcprov.jar \
      "https://repo1.maven.org/maven2/org/bouncycastle/bcprov-jdk18on/${BC_VERSION}/bcprov-jdk18on-${BC_VERSION}.jar" \
 && echo "${BC_SHA256}  /opt/bcprov.jar" | sha256sum -c -
COPY demo/workloads/CryptoApp.java /opt/CryptoApp.java
# ★ collector는 여기 없다 — Ansible이 실행 시점에 반입한다(배포 경로 실증). 노드엔 **관측 대상**인
# 워크로드만 둔다(pqc-echo = PQC TLS 서버).
COPY --from=builder /out/pqc-echo /usr/local/bin/pqc-echo
COPY demo/scripts/internal/node-entrypoint.sh /usr/local/bin/node-entrypoint.sh
COPY demo/scripts/internal/pqcota-gen-traffic.sh /usr/local/bin/pqcota-gen-traffic.sh
COPY demo/scripts/internal/pqcota-observe.sh /usr/local/bin/pqcota-observe.sh
COPY demo/scripts/internal/ssl-apps.sh /usr/local/bin/ssl-apps.sh
RUN chmod +x /usr/local/bin/node-entrypoint.sh /usr/local/bin/pqcota-gen-traffic.sh /usr/local/bin/pqcota-observe.sh
ENTRYPOINT ["/usr/local/bin/node-entrypoint.sh"]

# ─────────────────────────── controller (Discovery: Ansible + 읽기전용 뷰) ───────────────────────────
# ★ 이 이미지가 곧 **사용자의 빌드 머신**이다 — 리포를 클론해 두고, 여기서 빌드해서, 여기서 실행한다.
# 그래서 툴체인(Go·JDK·buf)과 소스가 함께 들어간다. 빌드 자체는 up.sh가 **컨테이너 안에서** 돌린다
# (이미지에 결과를 굽지 않는다 — 사용자가 자기 머신에서 하는 일과 같은 순서로 보이게).
FROM ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90 AS ctl
ARG TARGETARCH
ARG GO_VERSION=1.26.4
RUN apt-get update && apt-get install -y --no-install-recommends \
      ansible openssh-client ca-certificates iproute2 iputils-ping graphviz \
      curl git make openjdk-21-jdk-headless \
 && rm -rf /var/lib/apt/lists/*
# Go 툴체인 — apt판은 go.mod가 요구하는 버전보다 낮아 공식 tarball을 쓴다(호스트 arch에 맞춰).
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
# proto 코드 생성 도구 — 버전 고정(리포의 CONTRIBUTING이 안내하는 그것).
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 \
 && go install github.com/bufbuild/buf/cmd/buf@v1.71.0
COPY . /src
COPY demo/scripts/ansible/ /work/ansible/
# 참조 플레이북은 데모 것이 아니라 제품 산출물이다 — 실환경에 그대로 이식하라고 만든 것이라
# discovery/ 아래 두고, 데모는 그걸 가져다 쓴다.
COPY discovery/ansible/discover.yml /work/ansible/discover.yml
WORKDIR /work
RUN mkdir -p /work/results
CMD ["sleep", "infinity"]

# ─────────────────────────── topo-gen (커스텀 토폴로지 생성기) ───────────────────────────
# topology.yaml → compose·manifest·groups·profiles. builder가 낸 정적 Go 바이너리 하나라
# base 이미지조차 없다(scratch) — 별도 런타임을 끌어오지 않는다. up.sh --topology가 쓴다.
FROM scratch AS topo-gen
COPY --from=builder /out/topogen /topogen
ENTRYPOINT ["/topogen"]
