#!/usr/bin/env bash
# 실물 oqsprovider.so를 만들어 표준출력 경로로 놓는다 — 데모의 선택 단계(DEMO_REAL_PROVIDER=1)용.
#
# 기본 데모는 **빈 파일**을 배치한다. 배포 경로(스테이징·sha256 게이트·롤백)를 보이는 데는
# 그것으로 충분하고, 데모 전제가 "Docker만 있으면 된다"이기 때문이다. 다만 빈 파일로는
# **생성물이 실제 암호 알고리즘으로 반영되는지**를 보일 수 없다 — 그것을 보려면 실물이 필요하다.
#
# 노드 이미지와 **같은 베이스**에서 빌드해야 ABI가 맞는다(노드 = ubuntu:24.04).
#
# liboqs는 **정적으로 링크한다**. 프로비저닝이 노드에 놓는 것은 `.so` 하나뿐이라, 기본값대로
# 공유 liboqs에 링크하면 노드에서 `liboqs.so.9 => not found`로 조용히 안 뜬다(실제로 그랬다).
set -euo pipefail

OUT="${1:?사용법: build-oqsprovider.sh <출력 .so 경로>}"
BASE="${OQS_BUILD_BASE:-ubuntu:24.04}"
IMG="pqcota-demo/oqsprovider-build"

if ! docker image inspect "$IMG" >/dev/null 2>&1; then
	echo "   oqsprovider 빌드 중(liboqs + oqs-provider · 첫 실행은 수 분)…"
	# 빈 컨텍스트로 빌드한다 — COPY가 없는데 리포를 통째로 보낼 이유가 없다.
	CTX=$(mktemp -d)
	trap 'rm -rf "$CTX"' EXIT
	docker build -q -t "$IMG" -f - "$CTX" >/dev/null <<EOF
FROM ${BASE}
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends \
      build-essential cmake ninja-build git libssl-dev ca-certificates >/dev/null
WORKDIR /src
RUN git clone -q --depth 1 https://github.com/open-quantum-safe/liboqs.git && \
    cmake -S liboqs -B liboqs/build -GNinja -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF -DCMAKE_POSITION_INDEPENDENT_CODE=ON >/dev/null && \
    cmake --build liboqs/build >/dev/null && cmake --install liboqs/build >/dev/null && ldconfig
RUN git clone -q --depth 1 https://github.com/open-quantum-safe/oqs-provider.git && \
    cmake -S oqs-provider -B oqs-provider/build -GNinja -DCMAKE_BUILD_TYPE=Release >/dev/null && \
    cmake --build oqs-provider/build >/dev/null && \
    find oqs-provider/build -name oqsprovider.so -exec cp {} /oqsprovider.so \;
# 미해결 의존이 남은 모듈은 여기서 끊는다 — 노드에 가서야 조용히 안 뜨면 원인을 찾기 어렵다.
RUN ldd /oqsprovider.so | grep -q 'not found' \
      && { echo "✗ 미해결 의존이 남았다:"; ldd /oqsprovider.so | grep 'not found'; exit 1; } || true
EOF
fi

cid=$(docker create "$IMG" true)
docker cp "$cid:/oqsprovider.so" "$OUT" >/dev/null
docker rm -f "$cid" >/dev/null
echo "   실물 모듈: $OUT ($(stat -c%s "$OUT" 2>/dev/null || stat -f%z "$OUT") 바이트)"
