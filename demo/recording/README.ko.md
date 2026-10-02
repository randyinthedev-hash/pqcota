[English](README.md) · 한국어

# 데모 영상 만들기

이 폴더가 데모 영상의 정본입니다. 영상 자체가 아니라 도구와 텍스트가 들어 있고, 생성되는 것은 모두 `out/`으로 가며 git은 이를 무시합니다.

영상에는 소리가 없습니다. 터미널 장면은 데모를 실제로 녹화한 것이고, 그 아래 띠에 자막이 붙습니다. 영어판과 한국어판을 만들 수 있습니다. 카드(제목, 소개, 절 제목, 마무리)는 HTML 페이지를 이미지로 캡처한 것입니다.

## 세 단계, 세 머신

단계마다 필요한 소프트웨어가 달라서 스크립트가 따로 있습니다.

| 단계 | 스크립트 | 실행할 곳 | 필요한 것 |
|---|---|---|---|
| 1. 터미널 장면 녹화 | `record.sh` | 데모가 도는 곳(Docker) | `asciinema`, `agg`, `ffmpeg`, 나란히 둔 다섯 리포지터리 |
| 2. 카드 캡처 | `cards.sh` | Chrome이 있는 곳 어디서나 | 헤드리스 Chrome, `ffmpeg` |
| 3. 조립 | `build.py` | `libass`로 빌드한 `ffmpeg`가 있는 곳 어디서나 | `ass` 필터가 있는 `ffmpeg` |

```bash
# 1. on the machine that runs the demo (the real-provider stage must have run once)
./demo/scripts/up.sh && DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
VIDEO_LANG=en VIDEO_VER=v0.10.1 demo/recording/record.sh   # → out/clips/en/*.cast, *.mp4, PROVENANCE.txt

# 2. cards (copy demo/.generated/topology.svg over if this is another machine)
VIDEO_LANG=en VIDEO_VER=v0.10.1 SITE_URL=https://randyinthedev-hash.github.io/pqcota demo/recording/cards.sh

# 3. assemble (put out/clips and out/cards in the same out/ first)
demo/recording/build.py --lang en             # → out/pqcota-demo.en.mp4 and friends
```

`VIDEO_LANG`(`build.py`는 `--lang`)가 언어를 고릅니다. `en` 또는 `ko`입니다. 세 단계를 언어마다 한 번씩 돌리세요. `LANG`은 쓰지 마세요. 운영체제의 로케일 환경변수입니다.

## 무엇이 나오나

- `pqcota-demo.<lang>.mp4`: 자막을 영상에 입힌 것으로, 바로 공유하는 용도입니다.
- `pqcota-demo.<lang>.nocaps.mp4`와 `pqcota-demo.<lang>.srt`: 자막 없는 같은 영상과 파일로 된 자막입니다. 자막을 스스로 보여 주는 사이트에는 이 한 쌍을 올려, 자막이 두 번 보이지 않게 하세요.
- `pqcota-demo.<lang>.provenance.txt`: 만든 방법의 기록입니다(아래).

## 조각이 맞물리는 방식

- **`record-take.sh`**(`../scripts/`에 있음)는 장면을 하나씩 실행 중인 데모에 대해 돌립니다. `provision.approve` 같은 이름 붙은 마커를 출력하는데, 화면에는 보이지 않지만 녹화에는 남습니다. 마커 이름은 「여기서 끝나는 구간」이라는 뜻입니다.
- **`build.py`**는 초가 아니라 그 이름으로 자를 지점을 찾습니다. 장면 표에는 모든 언어에 공통인 것이 들어 있습니다. 장면의 순서, 속도, 정지 화면을 따는 위치입니다. 다시 녹화해서 초가 달라져도 이 표는 그대로입니다.
- **`lang/<lang>.json`**에는 언어에 따라 달라지는 것이 들어 있습니다. 정지 화면이 머무는 시간, 위치가 붙은 자막 텍스트, 자막 글꼴, 읽기 속도 한도입니다. 위치는 장면과 그 안의 초, 또는 `@marker`와 녹화에서 그 마커 앞뒤의 초입니다. 읽는 시간은 언어마다 다르므로 언어별로 정지 화면, 머무는 시간, 한도를 정합니다. 장면 순서와 기준점은 모든 언어에서 같습니다.
- **`lang/take.<lang>.sh`**에는 `record-take.sh`가 명령 사이에 출력하는 해설 줄이 들어 있습니다. 영어가 기준이고 다른 언어는 번역입니다. 명령과 프로그램 자체의 출력은 어느 언어에서나 영어입니다.
- **카드**(`*-card.html`, `intro-slides.html`, `section-cards.html`, `topology-frame.html`)는 두 언어를 한 파일에 `<span class="en">`과 `<span class="ko">`로 담습니다. `?lang=`이 하나를 고르고, 기본은 영어입니다.
- **시간 규칙.** `agg`가 기다리는 시간을 줄이지 않는 한, 조립한 영상의 초는 `.cast`의 초와 같습니다. `record.sh`는 유휴 한도를 60초로 두고, `build.py`는 녹화에 그만큼 긴 정적이 있거나 영상 길이와 cast 길이가 어긋나면 멈춥니다.
- **편집 규칙 둘.** 정지 화면은 그 장면의 끝에서 땁니다. 시작에서 따면 이미 출력된 줄이 사라졌다가 되돌아와서 편집한 것처럼 보입니다. 숫자가 바뀌는 곳(0 → 14)에서는 클립을 절대 빠르게 하지 않습니다.

## 만든 방법의 기록

`record.sh`는 `out/clips/<lang>/PROVENANCE.txt`를 씁니다. 다섯 리포지터리 각각의 커밋(릴리스 태그인지, 트리가 깨끗했는지 포함), 도구 버전, 녹화 옵션입니다. `build.py`는 자막 파일의 해시, 마커 시각, 장면 시각을 `pqcota-demo.<lang>.provenance.txt`에 더합니다. 마무리 카드는 영상을 녹화한 릴리스 태그(`VIDEO_VER`)를 보여 주며, 녹화한 커밋이 그 태그와 정확히 같지 않으면 provenance 파일이 어떻게 다른지 보여 줍니다.

## 다시 빌드한 것을 확인하기

장면이 같은 순서로 나오고, 장면마다 실행한 명령이 보이고, 핵심 출력(`BC`가 있는 provider 체인, `0 → 14 → 0` 개수, `layersMissing`)이 맞고, 자막이 같은 내용을 말하면 다시 빌드한 영상은 같은 영상입니다. 바이트는 다릅니다. 마지막 확인은 영상 전체를 보통 속도로 한 번 보는 것입니다.

## 언어 추가하기

`lang/<lang>.json`과 `lang/take.<lang>.sh`를 더하고, 카드마다 그 언어의 span을 더합니다. 그다음 새 파일 둘을 `tools/checkprose/files.txt`에 올려 문체 점검이 읽게 합니다(`.json`은 그대로, `.sh`는 주석 줄을 빼고 읽습니다).

한국어(`ko`)는 이미 있습니다. 한국어판 영상이 2026년 8월 영상과 다른 점은 하나입니다. 프로그램이 이제 영어로 출력하므로 터미널 출력은 영어이고, 해설, 카드, 자막만 한국어입니다.
