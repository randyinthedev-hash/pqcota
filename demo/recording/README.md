# demo/recording/ — 데모를 영상으로 보이기

데모를 **스크린캐스트로 만들 때**의 절차와 뼈대. 발표·블로그·대회 출품 어디에나 같다.

- **어떻게 찍나** — 이 문서 아래.
- **찍은 클립을 무슨 순서로 붙이나** — 이 폴더의 [템플릿](#템플릿--복사해서-고쳐-쓴다)을 복사해 쓴다.

---

## 촬영 — 컷 하나씩

`demo.sh`를 그대로 녹화하면 6단계가 한 번에 흘러 화면이 빨리 지나가고, 어느 줄이 근거인지
영상에서 짚기 어렵다. [`../scripts/record-take.sh`](../scripts/record-take.sh)는 그중 **한 장면만** 떼어
명령을 먼저 보이고 → 잠깐 멈추고 → 결과를 낸다.

| 컷 | 무엇을 보이나 | 길이(실측) |
|---|---|---|
| `observe` | `java.security`에 없는 provider가 실행 중 체인에는 있다 — 정적 스캔과 런타임 관측의 차이 | 약 16초 |
| `provision` | 도구가 만든 config가 실제 암호 알고리즘으로 반영되고(ML-KEM 0→14), 되돌리면 원상복귀(→0) | 약 55초 |
| `gap` | 권한이 없어 관측하지 못한 계층을 따로 낸다. 수집은 실패가 아니라 정상 종료 — 그래야 그 기록이 중앙까지 간다 | 약 9초 |

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
폴백시킨다. 테마 값은 이 리포의 [구조도](../../docs/architectures/platform-structure.html) 팔레트다.

## 편집할 때

- **숫자가 바뀌는 순간은 배속하지 않는다.** `provision` 컷에서 `0`이 `14`가 되는 구간이 근거다.
  나머지(파일 목록·플레이북 진행)는 1.5–2배로 줄여도 잃는 것이 없다.
- **전환은 하드컷.** 디졸브로 터미널이 흐려지면 읽히지 않는다.
- **정지 프레임은 그 구간의 마지막에서 뜬다.** 앞에서 뜨면 이미 인쇄된 줄이 사라졌다
  다시 나타나 — 영상을 조작한 것처럼 보인다.
- **제작 시기를 넣는다.** 화면의 버전·provider는 시간이 지나면 달라진다. 언제의 관측인지가
  근거의 일부다.
- **강조는 확대가 아니라 밑줄·테두리.** 터미널 클립을 확대하면 글자가 뭉갠다.
- 컷 경계는 화면의 구분선(`────`)이다. `.cast`에서 그 시각을 뽑으면 자를 지점이 그대로 나온다.

## 함께 쓸 소재

- 관측 토폴로지 — `demo/.generated/topology.svg` (색이 posture)
- 플랫폼 구조도 — [`docs/architectures/platform-structure.html`](../../docs/architectures/platform-structure.html)
- 예상 출력 — [`expected-output/`](../expected-output/)

---

## 템플릿 — 복사해서 고쳐 쓴다

이 폴더의 나머지 파일은 **뼈대**다. 그대로 쓰지 말고 자기 작업 폴더로 복사한 뒤 내용을 바꾼다 —
다음 사람도 처음부터 시작하지 않게 하려고 남긴 것이다.

```bash
cp -r demo/recording ~/my-pqcota-video && cd ~/my-pqcota-video
```

| 파일 | 무엇 | 무엇을 고치나 |
|---|---|---|
| [`script.md`](script.md) | 대본·컷 시트 — 컷별 화면·내레이션·자막 | 분량, 강조할 것, 내레이션 문장 |
| [`edit-sheet.md`](edit-sheet.md) | 편집 지시서 — 타임라인·배속·자막 타이밍 | 클립 실측 길이(다시 찍으면 바뀐다) |
| [`title-card.html`](title-card.html) | 맨 앞 — 무엇을 보는지와 **언제 찍었는지** | 제목·제작 시기 |
| [`intro-slides.html`](intro-slides.html) | 도입 — 질문을 던지고 문제를 세운다 | 슬라이드 문구 |
| [`section-cards.html`](section-cards.html) | 각 실연 앞에 붙이는 예고 카드 셋 | 단계 이름·한 줄 설명 |
| [`outro-card.html`](outro-card.html) | 마무리 — 명령 한 줄과 리포 주소 | 주소·라이선스 |
| [`browser-frame.html`](browser-frame.html) | 웹 화면을 브라우저 창으로 감싼다 | 주소·캡처 파일 |
| [`topology-frame.html`](topology-frame.html) | demo가 만든 `topology.svg`를 화면 폭에 맞춰 키운다 | 제목 문구 |
| [`assemble.sh`](assemble.sh) | 조립 — 클립·카드를 타임라인대로 잇는다 | **타임라인 표** |

카드 HTML들은 **브라우저로 열어 화면을 잡는 것이 아니라 캡처해서 붙인다.** 정지 화면이 단계별로
켜지는 구조라 녹화할 이유가 없다 — 헤드리스로 각 상태를 뜨면 커서도 흔들림도 없고, 다시 뽑아도
같은 파일이 나온다. `?s=N`으로 그 상태만 바로 연다:

```bash
CH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"   # 리눅스면 google-chrome
for s in 0 1 2 3 4 5 6 7 8; do
  "$CH" --headless --disable-gpu --hide-scrollbars --window-size=1920,1080 \
    --screenshot="s$s.png" --virtual-time-budget=1500 "file://$PWD/intro-slides.html?s=$s"
done
```

뜬 PNG를 `ffmpeg`의 `xfade`로 이으면 도입 클립이 된다(각 상태를 몇 초 보일지는 대본에서 정한다).

전체 조립은 [`assemble.sh`](assemble.sh)가 한다. **타임라인을 고치는 곳은 그 스크립트의 표
하나뿐이다** — 손으로 이어 붙이면 그 명령이 어디에도 남지 않아, 클립 하나를 다시 찍을 때마다
전체를 기억에서 복원해야 한다.

```bash
./assemble.sh .        # clips/{intro,observe,provision,gap,outro}.mp4 + .cap/*.png → final.mp4
```

### 왜 템플릿인가

시연영상은 만들 때마다 목적이 다르다. 대회에 내는 것, 발표 자리에서 트는 것, 리포를 소개하는
것은 분량도 강조할 곳도 다르다. 그래서 완성본 대신 뼈대만 둔다.

다만 목적이 달라져도 잘 바뀌지 않는 것이 셋 있다.

1. **시간은 숫자가 바뀌는 화면에 몰아준다.** 이 리포라면 조치 전후의 ML-KEM 개수
   (0개 → 14개 → 0개)다. 기능을 하나씩 소개하다 보면 3분이 금방 지나간다.
2. **말로 주장하지 말고 화면으로 보인다.** "정적 스캔이 관측되지 않는 것을 본다"고 쓰는 대신,
   `java.security`에는 없는 provider가 관측된 체인에는 있는 두 출력을 나란히 놓는다.
3. **관측하지 못한 것을 없다고 하지 않는 장면도 넣는다.** 그 화면은 몇 초 안 되지만, 이 도구가
   무엇을 하지 않는지 보여 주는 유일한 컷이다.

### 피할 것

- **과장.** "양자컴퓨터가 곧 모든 암호를 깬다" 같은 말은 하지 않는다. 이 리포는 무엇이
  쓰이는지 관측할 뿐 앞일을 점치지 않는다.
- **슬라이드로만 채우기.** 강점은 실제로 돌아가는 화면이다. 슬라이드는 도입까지만 쓴다.
- **기능 나열.** 커맨드를 하나씩 소개하다 보면 정작 근거가 되는 화면을 보일 시간이 없다.
