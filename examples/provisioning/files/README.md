# files/: 여기에 당신의 provider 모듈을 둔다

**이 리포에는 provider 모듈 바이너리가 없다.** `.so`는 arch·libc별로 다르고 JAR은 10MB급이라, 커밋하면 리포가 **갱신되지 않는 사본**을 떠안는다. 더미를 넣어두는 것은 더 나쁘다. 로드되지도 않으면서 동작하는 것처럼 보인다.

대신 **받을 곳과 고정한 해시**를 둔다. 받은 것이 기대한 것인지 확인할 수 있으면 커밋한 것보다 낫다.

```bash
./examples/provisioning/files/fetch-example-provider.sh          # BC.jar을 여기 놓는다(sha256 확인)
./examples/provisioning/files/fetch-example-provider.sh --check  # 받지 않고 고정값만
```

BouncyCastle **1.85**를 고정한다. 1.80+부터 ML-KEM이 `BouncyCastleProvider`(생성되는 조각이 등록하는 그 클래스)에 있고, 1.78.x 이하는 그 클래스에 KEM이 0개이며 Kyber가 `BouncyCastlePQCProvider`에 따로 있다. 같은 조각을 써도 목표 알고리즘이 생기지 않는다.

OpenSSL provider(`oqsprovider.so`)는 받을 수 없다. 배포판·arch별 공식 바이너리가 없어 liboqs + oqs-provider를 빌드해야 한다. 없는 것을 있는 척 만들지 않는다.

## 조각이 정말 provider를 등록하는가

파일을 복사했다는 것과 **의도한 일이 일어났다**는 것은 다르다(조각이 적용조차 되지 않는 경우가 실제로 있다). 진짜 JAR과 진짜 JVM으로 끝까지 본다:

```bash
./examples/provisioning/files/verify-registration.sh                    # jca-provider-inject-bc
./examples/provisioning/files/verify-registration.sh jca-native-config-only
```

출력은 세 가지를 보인다. 조각 적용 후의 **provider 순서**, 목표 알고리즘(ML-KEM·ML-DSA)이 **실제로 어느 provider에서 나오는가**, 그리고 조각이 목록에 한 일의 **전후 대조**.

그 대조가 중요한 사실 하나를 드러낸다: `security.provider.2=`는 **끼워 넣지 않고 그 자리를 대체한다.** JDK 21에서 원래 2번이던 `SunRsaSign`이 목록에서 빠지고, RSA 서비스가 새 provider 구현으로 넘어간다. 밀어내지 않으려면 대상 노드의 `java.security`에서 뒤 번호를 한 칸씩 미룬 뒤 넣어야 한다. 그러려면 그 노드의 원본을 알아야 하므로 도구가 대신 하지 않는다.

`jca-native-config-only`로 돌려 보면 세 알고리즘이 모두 **없음**으로 나온다. provider를 등록하지 않는 케이스이고 JDK 21에는 네이티브 ML-KEM이 없기 때문이다. 이것도 정직한 결과다.

생성된 플레이북을 **실제로 실행**하려면 모듈이 컨트롤러에 있어야 한다. Ansible `copy`는 `src`를 플레이북 옆 `files/`에서도 찾으므로, 여기 두면 인자 없이 동작한다:

```
examples/provisioning/files/
  acme-pqc.so      ← custom-openssl-provider 케이스가 찾는 이름
  acme-jce.jar     ← custom-jca-provider 케이스가 찾는 이름
  oqsprovider.so   ← openssl-3.0-provider-inject 케이스
  BC.jar           ← jca-provider-inject-bc 케이스
```

파일명은 계획의 `providerChoice`에서 온다. `"providerChoice": "acme-pqc"` → `acme-pqc.so`.

다른 곳에 두려면 경로를 넘긴다:

```bash
ansible-playbook provision.yml \
  -e pqcota_module_src_acme_pqc=/srv/pqcota/modules/acme-pqc.so \
  -e pqcota_module_sha256_acme_pqc=$(sha256sum /srv/pqcota/modules/acme-pqc.so | cut -d' ' -f1)
```

## 플레이북 자체만 시험해 보고 싶다면

빈 파일이라도 두면 **배치·체크섬 task는 정상적으로 돈다**(당연히 실제 암호 기능은 없다). 호스트를 건드리지 않으려면 컨테이너 안에서 로컬 연결로 돌린다:

```bash
mkdir -p /tmp/try/files && : > /tmp/try/files/acme-pqc.so
go run ./provisioning/cmd/pqcota-provision --level l2 \
  examples/provisioning/plans/custom-openssl-provider.json \
  | sed 's/^  hosts: .*/  hosts: all/' > /tmp/try/provision.yml

docker run --rm -v /tmp/try:/work -w /work alpine/ansible:latest \
  ansible-playbook -i 'localhost,' -c local provision.yml
```

무결성 게이트까지 보려면 해시를 넘긴다. 맞으면 통과, 틀리면 **중단**한다:

```bash
... ansible-playbook ... -e "pqcota_module_sha256_acme_pqc=$(sha256sum /tmp/try/files/acme-pqc.so | cut -d' ' -f1)"
... ansible-playbook ... -e "pqcota_module_sha256_acme_pqc=deadbeef"   # → fail_msg로 중단
```

되돌림까지 보려면 **같은 컨테이너 안에서** 이어 돌려야 한다. `docker run`은 매번 새 컨테이너라 따로 돌리면 지울 파일이 애초에 없다(`changed=0`이 나오고 아무 일도 안 한 것처럼 보인다):

```bash
go run ./provisioning/cmd/pqcota-provision --level l2 --rollback \
  examples/provisioning/plans/custom-openssl-provider.json \
  | sed 's/^  hosts: .*/  hosts: all/' > /tmp/try/provision-rollback.yml

docker run --rm -v /tmp/try:/work -w /work alpine/ansible:latest sh -c '
  ansible-playbook -i localhost, -c local provision.yml
  ls /opt/pqcota /etc/pqcota                    # 놓였다
  ansible-playbook -i localhost, -c local provision-rollback.yml
  ls /opt/pqcota /etc/pqcota'                   # 비었다 (changed=2)
```

활성화·재시작(L3)까지 보려면 `--level l3`과 `activation` 훅이 있는 계획을 쓴다 → [`l3-activation-hooks`](../plans/l3-activation-hooks.json), [예제 README의 L3 절](../README.md#l3-활성화재시작).

> 이 폴더의 `.so`·`.jar`은 gitignore된다. 받은 것이든 당신 것이든 벤더 바이너리가 실수로 커밋되지 않게.
