# 데모가 검증하는 것 — 통합 케이스 명세

데모는 시연물이면서 **이 리포의 통합 테스트 하네스**다. 실물이 필요한 케이스(실 OpenSSL·실행 중 JVM·컨테이너·SSH·Postgres)는 Go 테스트로 자동화돼 있지 않고, `./demo/scripts/demo.sh` 6단계가 그 자리를 맡는다.

여기 적은 것은 **데모가 실제로 확인하는 것**이다. 커버하지 않는 것도 아래에 그대로 적는다 — 데모를 돌렸다고 다 검증된 것은 아니다.

## 커버하는 케이스

| 케이스 | 데모 단계 | 무엇을 확인하나 | 출력에서 보이는 것 |
|---|---|---|---|
| **TD-OPENSSL-3** 실 OpenSSL 호스트 수집 | 2/6 | 컨테이너 노드(`openssl 3` · `openssl 1.1.1`)에서 `pqcota-nodescan`이 `/proc`·ELF로 로드된 lib을 잡는다 | 3/6 뷰의 `openssl confirmed runtime_introspection` 행 |
| **TD-JVM-3 · TD-JVM-8** 실 JVM attach 종단 | 2/6 | `pqcota-jvmscan --recon`으로 JVM을 먼저 찾고, **있는 노드에만** `collector.jar`를 보내 실제 attach → `getProviders()` | 3/6 뷰의 `jca` 행에 BouncyCastle 포함 provider 체인 |
| **TD-CONTAINER-1** 컨테이너 — 동일 PID 네임스페이스 | 2/6 | collector가 대상 컨테이너 **안에서** 돌아 `/proc`이 보인다 | 노드별 자산이 잡힘 |
| **NET** 실 핸드셰이크 관측 | 2/6 | `pqcota-netcap`이 AF_PACKET으로 TLS·SSH 협상 그룹을 관측 | `관측 엣지 N개`, 4/6 토폴로지 SVG의 색 |
| **IC-H** 이력·스냅샷 diff | 5/6 | 같은 상태 재적재는 스냅샷을 늘리지 않고 관측 기록만 +1. `-history`·`-snapshot`·`-diff` | `스냅샷은 변화가 있을 때만, obs는 재확인 횟수` |
| **IC-T** 보존 절단 | 5/6 | `pqcota-prune -keep-last 1` | 절단 계획·결과 |
| **IC-S** 자산 스코프 | 5/6 | 제외 규칙 적용 후에도 **제외 건수를 고지**한다 | `제외 ≠ 부재 — 건수를 고지한다` |
| **접근 비밀 미영속** | 0/6 | 엔드포인트 테이블에 계정·키 흔적이 없다 — SQL로 직접 센다 | `엔드포인트에 접근 비밀 흔적: 0건 (0이어야 정상)` |
| **PV 종단** 생성→적용→되돌림 | 6/6 | L2 생성·적용 후 타깃에 파일이 실제로 놓였는지, L3 활성화 후 **서비스 PID가 바뀌었는지**, 롤백 후 사라지는지 | `ok= changed= failed=0`, `재시작 확인: pid A → B` |

| **PV 반영** 조치가 실제 암호 알고리즘으로 반영되는가 | 선택 | `DEMO_REAL_PROVIDER=1`일 때만. 실물 oqsprovider를 3.0–3.4 노드에 배치·활성화하고 `openssl list`로 전후를 잰다 | `ML-KEM KEM 0개 → 14개`, 되돌린 뒤 다시 `0` |

**6/6이 이 하네스의 핵심이다.** 생성만 보면 깨끗한 노드에서 깨지는 플레이북도 통과한다 — 실제로 그런 결함이 여럿 있었다.

**선택 단계는 그 다음 칸을 본다.** 6/6은 빈 파일로 도니 "놓였다·참조된다·되돌아간다"까지만 말할 수 있다. 조각이 **실제로 능력을 만드는지**는 실물이라야 갈린다 — 실제로 여기서 렌더 결함이 하나 나왔다(최상위 `openssl_conf` 누락: 배치도 게이트도 통과하는데 provider가 안 올라왔다).

## 커버하지 않는 것

| 케이스 | 왜 | 지금 무엇이 대신하나 |
|---|---|---|
| **TD-JVM-4** attach 차단(`DisableAttachMechanism`·JEP 451) 시 열화 | 데모 JVM은 attach를 허용한다 | 폴백 경로는 unit(`attach_test.go`·`staticfallback_test.go`) |
| **TD-CONTAINER-2** 네임스페이스 분리 | 데모는 대상 안에서 돌리므로 분리 상황이 생기지 않는다 | 없음 — 미검증 |
| **TD-SIGN-1** 서명된 결과 반입 | 데모는 `PQCOTA_SIGN_KEY`를 주지 않아 서명 검증을 건너뛴다(출력이 `서명 검증: 생략`이라 고지) | 서명·거부는 unit(`sign_test.go`·`central_test.go` `TestIngestSignatureReject`) |
| **오래된 커널** | 컨테이너는 호스트 커널을 공유한다 | 없음 — 실기 확인이 릴리스 블로커 |

## 돌리는 법

```bash
./demo/scripts/up.sh && ./demo/scripts/demo.sh
```

선택 단계까지 보려면 `DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh` — 실물 provider를 빌드하느라 첫 실행이 수 분 더 걸린다.

전제는 Docker뿐이다. 예상 결과는 [expected-output](expected-output/README.md), 구성은 [topology](topology/README.md).
