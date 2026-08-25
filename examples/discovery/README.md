# examples/discovery: 접근 준비 + 두 수신 경로(① 직접 관측 · ② 위임 CBOM)

```bash
./examples/discovery/run.sh
```

> **§ 표기**: 별도 언급이 없으면 [규정서](../../docs/regulation.md)의 절 번호다.

## 무슨 일이 일어나나

### 1) `pqcota-hosts`: 사용자 hosts 파일 → Ansible 인벤토리 + 엔드포인트 (§1.5)
입력 [`hosts.csv`](hosts.csv)(사용자가 관리하는 파일):
```
node_id,name,ip,port,ssh_user,ssh_key,ssh_pass,os,connection
node-a,Web Frontend,10.0.0.2,22,deploy,/home/me/.ssh/id_ed25519,,,            ← SSH 키 방식(권장)
node-b,Payments App (Java),10.0.0.3,22,deploy,,example-password,,             ← 비밀번호 방식
node-c,Payments DB,10.0.0.9,22,deploy,/home/me/.ssh/id_ed25519,,,
node-d,Payments Gateway (Windows),10.0.0.11,,Administrator,,example-password,windows,winrm
```
→ 두 가지를 낸다:
- `--ansible-out targets.ini`: 런타임 전용 **Ansible 인벤토리**(접속 비밀이 실려 소유자만 읽을 수 있게 `0600`). 이걸로 각 노드에서 collector를 돌린다. **pqcota 인벤토리엔 영속하지 않는다.**
- stdout: **안전 엔드포인트**(node_id·이름·ip·port: **비밀 제외**). `--dsn`을 주면 이걸 인벤토리(Postgres)에 upsert(재사용·수정 대상).

> 접근 비밀(키·비밀번호·계정)은 **hosts.csv(사용자 파일)와 생성된 targets.ini(런타임)에만** 있고 pqcota 인벤토리엔 적재하지 않는다.

#### 인증 방식: SSH 키(권장) 또는 비밀번호
컬럼은 헤더 필수·순서 자유이며, **호스트마다 독립**이다. `node_id`만 필수다.

| 컬럼 | 뜻 |
|---|---|
| `ssh_user` | 로그인 계정(예: `deploy`, `root`) |
| `ssh_key` | **미리 만들어둔 SSH 개인키 경로** → `ansible_ssh_private_key_file` (권장) |
| `ssh_pass` | 비밀번호 → `ansible_ssh_pass` (지원하나 권장 안 함) |
| `os` | `linux`(기본) 또는 `windows`. 그 노드에서 어느 collector를 돌릴지 가른다 |
| `connection` | `ssh`(기본) 또는 `winrm`. 그 노드에 **어떻게 붙을지** |

- **키 방식**(node-a·node-c): `ssh_key`에 개인키 경로를 적고 `ssh_pass`는 비운다.
- **비밀번호 방식**(node-b): `ssh_pass`에 비밀번호, `ssh_key`는 비움. ⚠️ Ansible이 비밀번호로 접속하려면 컨트롤러에 **`sshpass`가 설치**돼 있어야 한다(`apt install sshpass`). 평문 비밀번호가 targets.ini에 실리니 키 방식을 권한다.
- 섞어 써도 된다(위 예시처럼 노드마다 다르게).

#### `os`: 어느 collector를 보낼지 가른다

`linux`면 `pqcota-nodescan`·`pqcota-netcap`·`pqcota-jvmscan`, `windows`면 `pqcota-cngscan`·`pqcota-jvmscan`이다. 비우면 `linux`이고, 둘 중 어느 것도 아닌 값은 **오류**다. 조용히 리눅스로 삼키면 Windows 노드에 리눅스 collector가 올라가고 실패는 한참 뒤에 드러난다.

**관측하지 않고 받아 적는다.** OS는 collector를 올리기 *전에* 알아야 하는데 알아내려면 이미 무언가를 올려야 한다. hosts.csv는 사용자가 관리하는 파일이라 대개 이미 알고 있다. (플레이북은 `gather_facts`로 한 번 더 확인한다. 적힌 것과 다르면 그 노드는 그냥 건너뛴다.)

`os`가 만드는 것은 인벤토리의 **그룹**이다: `[targets_linux]`·`[targets_windows]`, 그리고 둘의 부모인 `[targets]`. `hosts: targets`로 쓰던 플레이북은 그대로 돈다.

#### `connection`: 어떻게 붙을지

**여기 적는 이유는 `targets.ini`가 매 실행 덮어써지기 때문이다.** 손으로 더한 연결 설정은 다음 실행에 지워진다. 접속 방법은 지워지지 않는 곳, 즉 이 파일에 있어야 한다.

| 값 | 인벤토리에 나가는 것 | 계정·비밀 |
|---|---|---|
| `ssh` (기본) | 리눅스면 그대로. **Windows면 `ansible_shell_type=powershell`**: 셸이 sh가 아니다 | `ssh_key`(권장) 또는 `ssh_pass` |
| `winrm` | `ansible_connection=winrm`, 포트 기본 **5985** | `ssh_pass` → `ansible_password`. **키로는 붙지 않는다** |

`port`를 적었으면 그것이 이긴다(HTTPS면 `5986`). `connection=winrm`인데 `os`가 `windows`가 아니거나 `ssh_key`가 있으면 **오류**다. 접속 시점에야 드러날 어긋남을 파일 읽는 자리에서 끊는다.

> **사이트마다 갈리는 값 둘은 지어내지 않는다**. SSH의 `ansible_shell_type=cmd`(sshd 기본 셸이 cmd일 때만)와 WinRM의 `ansible_winrm_transport`다. 생성된 ini의 주석이 그 자리를 알려 주고, 값은 `group_vars/targets_windows.yml`에 둔다.
>
> 인증서 검증처럼 winrm 플러그인이 **선언하지 않은** 설정은 `ansible_winrm_<option>` 꼴로 적으면 pywinrm까지 그대로 넘어간다(`ansible_winrm_server_cert_validation` 등). 앤서블 옵션이 아니라 pywinrm의 이름이라 `ansible-doc`에서는 찾을 수 없다.

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

> 이 예제(`run.sh`)는 접속까지 하지 않으므로 **키 파일이 없어도 동작**한다. `pqcota-hosts`는 경로 문자열을 targets.ini에 옮겨 적을 뿐이다. 키는 이후 실제 `ansible-playbook` 단계에서 쓰인다.

### 2) `pqcota-ingest`: 회수 결과 → 스코프 게이트 → 정규화 → 적재
[`../data/results`](../data)의 `CollectionResult` JSON들을 읽어 파생 `Finding`으로 정규화·적재한다. `PQCOTA_DSN`이 없으면 **인메모리 요약**(스냅샷·노드별 자산/엣지 수), 있으면 Postgres에 append-only 영속. 서명 검증은 `PQCOTA_VERIFY_KEY`가 있을 때만.

`node-a-openssl.json`의 CBOM(디코드)에는 한 공유 라이브러리에 앱이 여럿 붙은 것이 들어 있다:
```json
{"name":"pqcota:openssl.lib","value":"libssl.so.3"},
{"name":"pqcota:app_keys","value":"/opt/apps/api-gw,/opt/apps/payment-gw"}
```

## 실행 중 JVM 관측 (정찰→attach)
JCA provider 체인은 **살아있는 JVM에 attach**해야 실체(런타임 `addProvider` 포함)가 보인다. 별도 예제로 격리했다(Go만이 아니라 JDK+Docker 필요): **[jvm/](jvm/README.md)**: `/proc` 정찰 → attach → 동적 BC 포착 → JSONL 적재를 최소로 보인다.

## 실제 노드를 스캔하려면 (리눅스)
샘플 대신 진짜 관측을 내려면 관측 대상에서:
```bash
go run ./discovery/cmd/pqcota-nodescan <node-id>   # /proc의 로드된 OpenSSL(libssl/libcrypto) — 리눅스 전용
```
결과 JSON을 모아 그 디렉터리를 `pqcota-ingest`에 준다. JVM은 [jvm/](jvm/README.md) 참고. 커맨드 전체 지도: [discovery/cmd/README](../../discovery/cmd/README.md).
