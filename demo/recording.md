# 데모 화면 녹화하기 — 스크린캐스트 만드는 법

데모를 **영상으로 보이려 할 때**의 절차. 발표·블로그·컨퍼런스 데모 어디에나 같다.

`demo.sh`를 그대로 녹화하면 6단계가 한 번에 흘러 화면이 빨리 지나가고, 어느 줄이 근거인지
영상에서 짚기 어렵다. [`scripts/record-take.sh`](scripts/record-take.sh)는 그중 **한 장면만** 떼어
명령을 먼저 보이고 → 잠깐 멈추고 → 결과를 낸다.

## 컷 셋 — 각각이 하나를 증명한다

| 컷 | 무엇을 보이나 | 길이(실측) |
|---|---|---|
| `observe` | `java.security`에 없는 provider가 실행 중 체인에는 있다 — 정적 스캔과 런타임 관측의 차이 | 약 16초 |
| `provision` | 도구가 만든 config가 실제 암호 능력을 만들고(ML-KEM 0→14), 되돌리면 원상복귀(→0) | 약 55초 |
| `gap` | 권한이 없어 관측 못 한 계층을 갭으로 낸다. 종료코드는 0 — 갭이 중앙까지 가야 한다 | 약 9초 |

```bash
./demo/scripts/up.sh && DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh   # 한 번만 (생성물이 남는다)
./demo/scripts/record-take.sh observe                                 # 촬영은 이걸 반복
```

산출물을 **다시 만들지 않고 그대로 쓰므로** 몇 번 다시 찍어도 같은 화면이 나온다.
`provision` 컷은 끝에 롤백해 노드를 원래대로 둔다.

페이스는 환경변수로 — `TAKE_PAUSE`(컷 사이 정지, 기본 2초) · `TAKE_TYPE_DELAY`(타이핑 한 글자
간격, 기본 0.05초). 화면이 빠르면 올린다.

## 영상 파일로 뽑기

터미널을 화면 녹화해도 되지만, `asciinema`로 뜨면 **찍은 뒤에도 폰트·테마·배속을 바꿀 수 있다.**

```bash
sudo apt install -y asciinema ffmpeg
curl -sL https://github.com/asciinema/agg/releases/download/v1.9.0/agg-x86_64-unknown-linux-gnu \
  -o /tmp/agg && sudo install -m755 /tmp/agg /usr/local/bin/agg
```

```bash
TERM=xterm-256color asciinema rec take.cast --cols 100 --rows 32 --overwrite \
  -c "./demo/scripts/record-take.sh provision"

agg take.cast take.gif \
  --font-family "DejaVu Sans Mono,Noto Sans Mono CJK KR" --font-size 20 \
  --theme 0e1420,e6ebf2,161f2e,e0736b,7fc08a,e2c08d,7aa2f7,c0a3e0,7fd1c5,c3cbd8 \
  --idle-time-limit 3 --fps-cap 30

ffmpeg -i take.gif -movflags faststart -pix_fmt yuv420p take.mp4
```

**폰트 순서가 중요하다.** CJK 폰트를 앞에 두면 `agg`가 셀 폭을 잘못 계산해 공백을 먹는다 —
`0 → 14 → 0`이 `0 →14 →0`으로 나온다. 진짜 고정폭(DejaVu Sans Mono)을 앞에 두고 한글만
폴백시킨다. 테마 값은 이 리포의 [구조도](../docs/architectures/platform-structure.html) 팔레트다.

## 편집할 때

- **숫자가 바뀌는 순간은 배속하지 않는다.** `provision` 컷에서 `0`이 `14`가 되는 구간이 근거다.
  나머지(파일 목록·플레이북 진행)는 1.5–2배로 줄여도 잃는 것이 없다.
- **전환은 하드컷.** 디졸브로 터미널이 흐려지면 읽히지 않는다.
- **강조는 확대가 아니라 밑줄·테두리.** 터미널 클립을 확대하면 글자가 뭉갠다.
- 컷 경계는 화면의 구분선(`────`)이다. `.cast`에서 그 시각을 뽑으면 자를 지점이 그대로 나온다.

## 함께 쓸 소재

- 관측 토폴로지 — `demo/.generated/topology.svg` (색이 posture)
- 플랫폼 구조도 — [`docs/architectures/platform-structure.html`](../docs/architectures/platform-structure.html)
- 예상 출력 — [`expected-output/`](expected-output/)
