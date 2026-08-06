# examples/discovery — 접근 준비 + 두 수신 경로(① 직접 관측 · ② 위임 CBOM)

```bash
./examples/discovery/run.sh
```

> **§ 표기**: 별도 언급이 없으면 [프로세스 규정서](../../docs/PQC플랫폼_단계별_프로세스규정.md)의 절 번호다.

## 무슨 일이 일어나나

### 1) `pqcota-hosts` — 사용자 hosts 파일 → Ansible 인벤토리 + 엔드포인트 (§4A.3)
입력 [`hosts.csv`](hosts.csv)(사용자가 관리하는 파일):
```
node_id,name,ip,port,ssh_user,ssh_key,ssh_pass
node-a,Web 프론트,10.0.0.2,22,deploy,/home/me/.ssh/id_ed25519,      ← SSH 키 방식(권장)
node-b,결제 앱(Java),10.0.0.3,22,deploy,,example-password           ← 비밀번호 방식
node-c,결제 DB,10.0.0.9,22,deploy,/home/me/.ssh/id_ed25519,
```
→ 두 가지를 낸다:
- `--ansible-out targets.ini`: 런타임 전용 **Ansible 인벤토리**(접속 비밀이 실려 소유자만 읽을 수 있게 `0600`). 이걸로 각 노드에서 collector를 돌린다. **pqcota 인벤토리엔 영속하지 않는다.**
- stdout: **안전 엔드포인트**(node_id·이름·ip·port — **비밀 제외**). `--dsn`을 주면 이걸 인벤토리(Postgres)에 upsert(재사용·수정 대상).

> 접근 비밀(키·비밀번호·계정)은 **hosts.csv(사용자 파일)와 생성된 targets.ini(런타임)에만** 있고 pqcota 인벤토리엔 적재하지 않는다.

#### 인증 방식 — SSH 키(권장) 또는 비밀번호
컬럼은 헤더 필수·순서 자유이며, **호스트마다 독립**이다. `node_id`만 필수.

| 컬럼 | 뜻 |
|---|---|
| `ssh_user` | 로그인 계정(예: `deploy`, `root`) |
| `ssh_key` | **미리 만들어둔 SSH 개인키 경로** → `ansible_ssh_private_key_file` (권장) |
| `ssh_pass` | 비밀번호 → `ansible_ssh_pass` (지원하나 권장 안 함) |

- **키 방식**(node-a·node-c): `ssh_key`에 개인키 경로, `ssh_pass`는 비움.
- **비밀번호 방식**(node-b): `ssh_pass`에 비밀번호, `ssh_key`는 비움. ⚠️ Ansible이 비밀번호로 접속하려면 컨트롤러에 **`sshpass`가 설치**돼 있어야 한다(`apt install sshpass`). 평문 비밀번호가 targets.ini에 실리니 키 방식을 권한다.
- 섞어 써도 된다(위 예시처럼 노드마다 다르게).

#### SSH 키 만들고 타깃에 등록하기
키 방식을 쓰려면 개인/공개키 쌍을 만들고 **공개키를 타깃의 `authorized_keys`에 등록**한다:
```bash
# 1) 키 쌍 생성 (한 번만) — 개인키 ~/.ssh/id_ed25519, 공개키 ~/.ssh/id_ed25519.pub
ssh-keygen -t ed25519 -C "pqcota-discovery" -f ~/.ssh/id_ed25519

# 2) 각 타깃에 공개키 등록 (타깃당 한 번) — 이후 비밀번호 없이 접속
ssh-copy-id -i ~/.ssh/id_ed25519.pub deploy@10.0.0.2
ssh-copy-id -i ~/.ssh/id_ed25519.pub deploy@10.0.0.3
ssh-copy-id -i ~/.ssh/id_ed25519.pub deploy@10.0.0.9

# 3) 접속 확인
ssh -i ~/.ssh/id_ed25519 deploy@10.0.0.2 true && echo OK
```
그 다음 `hosts.csv`의 `ssh_key`에 **개인키 경로**(`~/.ssh/id_ed25519`)를 적는다. `ssh-copy-id`가 없으면 공개키(`.pub`) 내용을 타깃의 `~/.ssh/authorized_keys`에 한 줄 추가하면 된다(퍼미션: `.ssh` 700, `authorized_keys` 600).

> 이 예제(`run.sh`)는 접속까지 하지 않으므로 **키 파일이 없어도 동작**한다 — `pqcota-hosts`는 경로 문자열을 targets.ini에 옮겨 적을 뿐이다. 키는 이후 실제 `ansible-playbook` 단계에서 쓰인다.

### 2) `pqcota-ingest` — 회수 결과 → 스코프 게이트 → 정규화 → 적재
[`../data/results`](../data)의 `CollectionResult` JSON들을 읽어 파생 `Finding`으로 정규화·적재한다. `PQCOTA_DSN`이 없으면 **인메모리 요약**(스냅샷·노드별 자산/엣지 수), 있으면 Postgres에 append-only 영속. 서명 검증은 `PQCOTA_VERIFY_KEY`가 있을 때만.

`node-a-openssl.json`의 CBOM(디코드)에는 공유 라이브러리의 다중 앱 귀속이 들어 있다:
```json
{"name":"pqcota:openssl.lib","value":"libssl.so.3"},
{"name":"pqcota:app_keys","value":"/opt/apps/api-gw,/opt/apps/payment-gw"}
```

## 실행 중 JVM 관측 (정찰→attach)
JCA provider 체인은 **살아있는 JVM에 attach**해야 실체(런타임 `addProvider` 포함)가 보인다. 별도 예제로 격리했다(Go만이 아니라 JDK+Docker 필요): **[jvm/](jvm/README.md)** — `/proc` 정찰 → attach → 동적 BC 포착 → JSONL 적재를 최소로 보인다.

## 실제 노드를 스캔하려면 (리눅스)
샘플 대신 진짜 관측을 내려면 관측 대상에서:
```bash
go run ./discovery/cmd/pqcota-nodescan <node-id>   # /proc의 로드된 OpenSSL(libssl/libcrypto)
```
결과 JSON을 모아 그 디렉터리를 `pqcota-ingest`에 준다. JVM은 [jvm/](jvm/README.md) 참고. 커맨드 전체 지도: [discovery/cmd/README](../../discovery/cmd/README.md).
