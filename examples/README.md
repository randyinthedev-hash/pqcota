# examples/: 단계별 실행 예제 (Discovery · Inventory · Provisioning)

cmd가 여럿이라 **실제로 어떻게 돌리는지** 감을 잡기 위한 최소 예제다. 각 단계 폴더에 **샘플 입력 + `run.sh`(복붙 실행) + README(무슨 일이 일어나나)**가 있다. 데모(`demo/`)가 컨테이너로 종단 전체를 보인다면, 여기선 **커맨드 하나하나를 최소 설정으로** 돌려본다.

## 전제
- **Go 툴체인**: `gen/`이 커밋돼 있어 클론 직후 바로 `go run`으로 소스에서 직접 실행할 수 있고 Postgres·타깃 노드는 필요 없다. proto를 고쳤다면 `make generate`를 먼저 돌린다(buf가 필요하다, [CONTRIBUTING](../CONTRIBUTING.md)).
- **예외가 하나 있다([discovery/jvm](discovery/jvm/README.md)).** 정찰→attach는 **살아있는 JVM**이 필요해 **Docker**를 쓴다(JDK는 컨테이너 안에 있다). 아래 표에 표시.

## 무엇이 있나
| 단계 | 폴더 | 돌려보는 것 | 전제 |
|---|---|---|---|
| **Discovery** | [`discovery/`](discovery) | `pqcota-hosts`(hosts→Ansible·엔드포인트) · `pqcota-ingest`(① 직접 관측 적재) · `pqcota-cbom-ingest`(② 위임 CBOM 수신) | Go만 |
| ↳ JVM | [`discovery/jvm/`](discovery/jvm) | `pqcota-jvmscan` 정찰→attach(실행 중 JVM의 동적 등록 실체) | **Go + Docker + JDK** |
| **Inventory** | [`inventory/`](inventory) | `pqcota-discover-view`(자산·앱 표시·등급) · `pqcota-declare-attribution`(관측이 못 짚은 엣지의 앱을 사람이 적는다) · `pqcota-cbom-ingest`(외부 CBOM 수신) | Go만 |
| **Provisioning** | [`provisioning/`](provisioning) | `pqcota-provision`(확정 계획→L2 플레이북) | Go만 |

```bash
./examples/discovery/run.sh
./examples/discovery/jvm/run.sh    # Docker + JDK 필요 (살아있는 JVM에 attach)
./examples/inventory/run.sh
./examples/provisioning/run.sh
```

## 공유 샘플 데이터: [`data/`](data)
Discovery·Inventory 예제가 함께 쓰는 **회수된 결과**(실물 collector가 낼 법한 `CollectionResult` JSON):
- `data/results/node-a-openssl.json`: OpenSSL 자산(공유 `libssl.so.3`이 두 앱 `api-gw`·`payment-gw`에 **걸쳐 있다**).
- `data/results/node-b-jca.json`: JCA provider 체인(SUN·SunJCE·**BC**).
- `data/results/node-a-net.json`: 관측 통신 엣지 3개(🟢 MLKEM · 🔴 x25519 · 🟢 SSH sntrup761).
- `data/results/node-d-cng.json`: Windows CNG provider 9개·알고리즘 50개. **실기에서 관측한 값**이다
  (Windows 11 Pro 25H2 · 빌드 26200). 머신 지문은 뺐고 노드 이름만 예제에 맞췄다.
- `data/nodes.json`: 관측 IP를 노드명으로 잇기(10.0.0.9 → node-c).
- [`inventory/attribution.csv`](inventory/attribution.csv): 관측이 앱을 못 짚은 엣지 하나를 사람이 지정하는 선언 한 줄.

> `CollectionResult`의 `cbomCyclonedx`는 **base64로 감싼 CycloneDX**(protobuf `bytes`)다. 각 단계 README에 디코드된 내용을 실어 뒀다.

## 저장소·타깃까지 포함한 종단 흐름은?
Postgres 영속(중앙 인벤토리)·Ansible/SSH 실접속·라이브 스캔까지 이어지는 전체는 **[demo/](../demo)**(Docker 6단계)를 본다. 여기 예제는 그 각 커맨드를 떼어내 최소로 돌리는 용도다.
