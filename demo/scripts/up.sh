#!/usr/bin/env bash
# pqcota 디스커버리 데모 — 설치(up): 토폴로지 생성 → 이미지 빌드 → 컨테이너 → SSH 키 → 노드 IP 맵.
#
# 데모가 세우는 환경은 **demo/topology/topology.yaml 하나**가 정의한다. 그 파일이 없으면 추적되는
# 샘플(topology.example.yaml)을 복사해 쓴다 — 편집 없이 그대로 돌아가고, 고치면 그대로 반영된다.
set -euo pipefail
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT="$(cd "$DEMO_DIR/.." && pwd)"
cd "$DEMO_DIR"

TOPO_FILE="$DEMO_DIR/topology/topology.yaml"
if [ ! -f "$TOPO_FILE" ]; then
  cp "$DEMO_DIR/topology/topology.example.yaml" "$TOPO_FILE"
  echo "ℹ starting with the default setup — the sample was copied to demo/topology/topology.yaml (gitignored)."
  echo "   edit that file and run up.sh again to match your own environment."
fi

echo "▶ 0/6 generating the topology (topogen container — Docker is the only host dependency)…"
# 리포에 생기는 산출물은 전부 demo/.generated/ 아래로 모은다(down.sh가 통째로 지운다).
GEN="$DEMO_DIR/.generated"; rm -rf "$GEN"; mkdir -p "$GEN"
docker build -q --target topo-gen -t pqcota-demo/topo-gen -f "$DEMO_DIR/Dockerfile" "$ROOT" >/dev/null
docker run --rm -v "$TOPO_FILE:/in.yaml:ro" -v "$GEN:/out" pqcota-demo/topo-gen /in.yaml /out
DC=(docker compose -f "$GEN/docker-compose.yml")
source "$GEN/manifest.env"   # NODES · EDGE_COUNT · HUMAN

echo "▶ 1/6 building images (OS, toolchain and workloads only — pqcota is not built here)…"
echo "   the first run takes a few minutes to pull base images."
"${DC[@]}" build

echo "▶ 2/6 starting containers…"
"${DC[@]}" up -d

# ── 리포 빌드: **ctl 머신 안에서**. 사용자가 자기 빌드 머신에서 하는 것과 같은 명령·같은 순서다.
# 이미지에 결과를 구워 넣지 않는 이유: 어디서 무엇을 어떤 옵션으로 빌드하는지 보이게 하려고.
echo "▶ 3/6 building the repo — the source is compiled **on the ctl machine (pqcota-ctl)**"
docker exec pqcota-ctl bash -lc '
  set -e
  cd /src
  ARCH=$(go env GOARCH)
  echo "   [ctl] $(. /etc/os-release; echo $PRETTY_NAME) · $(uname -m) · $(go version | cut -d" " -f3)"
  echo "   [ctl] make generate                       # contracts/*.proto → gen/"
  make generate >/dev/null
  echo "   [ctl] go build -o /usr/local/bin/ …        # the central CLIs used on this machine"
  CGO_ENABLED=0 go build -o /usr/local/bin/     ./inventory/cmd/pqcota-ingest ./discovery/cmd/pqcota-hosts     ./inventory/cmd/pqcota-inventory ./inventory/cmd/pqcota-discover-view     ./inventory/cmd/pqcota-profile ./inventory/cmd/pqcota-declare ./inventory/cmd/pqcota-prune \
    ./inventory/cmd/pqcota-declare-attribution     ./provisioning/cmd/pqcota-provision ./provisioning/cmd/pqcota-records
  echo "   [ctl] CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -o dist/linux-$ARCH/ …   # collectors to carry onto the nodes"
  CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -o "/work/dist/linux-$ARCH/"     ./discovery/cmd/pqcota-nodescan ./discovery/cmd/pqcota-netcap ./discovery/cmd/pqcota-jvmscan
  echo "   [ctl] make build-jar                      # JVM attach sidecar"
  make build-jar >/dev/null 2>&1 && cp build/collector.jar /work/dist/collector.jar
  echo "   [ctl] artifacts: $(ls /work/dist/linux-$ARCH | tr "\n" " ")· collector.jar"
' 2>&1 | sed "s/^/  /"

echo "▶ 4/6 waiting for sshd…"
for n in "${NODES[@]}"; do
  for i in $(seq 1 30); do
    if docker exec "$n" bash -lc 'ss -ltn 2>/dev/null | grep -q ":22 " || netstat -ltn 2>/dev/null | grep -q ":22 "'; then break; fi
    sleep 1
  done
done

# 크립토 서버(pqc-echo·s_server)는 노드·포트가 토폴로지마다 달라 여기서 일일이 기다리지 않는다.
# sshd 대기(위)면 디스커버리를 시작할 수 있고, 엣지가 덜 잡히면 demo.sh가 목표치까지 재수집한다.

echo "▶ 5/6 distributing the SSH key (controller → targets)…"
KEY="$(mktemp -d)/id_demo"
ssh-keygen -t ed25519 -N '' -f "$KEY" -q
for n in "${NODES[@]}"; do
  docker exec "$n" mkdir -p /root/.ssh
  docker cp "$KEY.pub" "$n:/root/.ssh/authorized_keys"
  docker exec "$n" chown -R root:root /root/.ssh
  docker exec "$n" chmod 700 /root/.ssh
  docker exec "$n" chmod 600 /root/.ssh/authorized_keys
done
docker cp "$KEY" pqcota-ctl:/work/id_demo
docker exec pqcota-ctl chmod 600 /work/id_demo
rm -rf "$(dirname "$KEY")"

# hosts.csv는 제품 모델에선 **사용자가 쓰는 파일**이다(자기 호스트의 IP·계정·키를 적는다).
# 데모에선 컨테이너가 떠야 IP가 정해지므로 up.sh가 그 역할을 대신 수행해 만든다 —
# 사용자가 편집하는 건 (커스텀이면) topology.yaml 하나뿐이고, 이 파일은 생성물이다.
echo "▶ 6/6 building the node IP map and hosts.csv (resolves observed IPs to node names; defines discovery access)…"
NODESJSON="$(mktemp)"
HOSTSCSV="$(mktemp)"
# HUMAN(사람이 읽는 이름)은 위에서 선언(기본) 또는 토폴로지 manifest에서 source됨 — pqcota-hosts가
# name 컬럼을 엔드포인트 인벤토리로 upsert(시각 구분).
echo "node_id,name,ip,port,ssh_user,ssh_key" > "$HOSTSCSV"
{
  echo -n '['
  first=1
  for n in "${NODES[@]}"; do
    # 노드가 여러 세그먼트에 걸치면 IP도 여럿 — 첫 IP를 접속용(hosts.csv)에, 전체를 관측 해소용
    # (nodes.json)에 담는다(관측된 IP가 어느 세그먼트의 것이든 노드로 해소되게).
    allips=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$n")
    ip=$(echo "$allips" | awk '{print $1}')
    ipsjson=$(echo "$allips" | tr ' ' '\n' | grep . | sed 's/.*/"&"/' | paste -sd, -)
    [ $first -eq 0 ] && echo -n ','
    echo -n "{\"name\":\"$n\",\"ips\":[$ipsjson]}"
    first=0
    echo "   $n = ${allips% }" >&2
    # hosts.csv: 접근 비밀(ssh_key)은 여기(사용자 파일)에만 — pqcota 인벤토리엔 적재 안 함(§1.5).
    echo "$n,${HUMAN[$n]:-$n},$ip,22,root,/work/id_demo" >> "$HOSTSCSV"
  done
  echo ']'
} > "$NODESJSON"
docker cp "$NODESJSON" pqcota-ctl:/work/nodes.json
docker cp "$HOSTSCSV" pqcota-ctl:/work/hosts.csv
rm -f "$NODESJSON" "$HOSTSCSV"

echo
echo "✅ setup complete. Next: ./demo/scripts/demo.sh (access prep → discovery → inventory → provisioning)"
