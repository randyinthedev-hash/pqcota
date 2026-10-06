#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 randyinthedev
# SPDX-License-Identifier: Apache-2.0
# 카드(정지 화면)를 캡처하고 도입·마무리 클립을 만든다 — 헤드리스 Chrome과 ffmpeg가 있는 곳에서 돌린다.
#
# 쓰는 법:  VIDEO_VER=v0.10.1 [VIDEO_DATE="Oct 2026"] demo/recording/cards.sh [out-dir]
# 환경변수: VIDEO_LANG(en 또는 ko, 기본 en) · CHROME(기본: macOS Chrome 또는 google-chrome)
#           TOPOLOGY_SVG(기본: demo/.generated/topology.svg) · SITE_URL(마무리에 넣을 사이트 화면, 없으면 건너뛴다)
# 만드는 것: <out>/cards/<lang>/{title,s0..s8,c1..c3,topo,end1,end2}.png, intro.mp4, outro.mp4
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT=${1:-$HERE/out}
VIDEO_LANG=${VIDEO_LANG:-en}
[ -f "$HERE/lang/$VIDEO_LANG.json" ] || { echo "no language file lang/$VIDEO_LANG.json" >&2; exit 2; }
VER=${VIDEO_VER:?set VIDEO_VER to the release tag the video was recorded from (e.g. v0.10.1)}
DATE=${VIDEO_DATE:-$(date '+%b %Y')}
SVG=${TOPOLOGY_SVG:-$ROOT/demo/.generated/topology.svg}
CHROME=${CHROME:-}
if [ -z "$CHROME" ]; then
	for c in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" google-chrome chromium; do
		if command -v "$c" >/dev/null 2>&1 || [ -x "$c" ]; then CHROME=$c; break; fi
	done
fi
[ -n "$CHROME" ] || { echo "Chrome is required (set CHROME)" >&2; exit 1; }

D=$OUT/cards/$VIDEO_LANG
mkdir -p "$D" "$OUT/cards/.work"
cp "$SVG" "$OUT/cards/.work/topology.svg"
urlenc() { python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1]))' "$1"; }
cap() { # cap <png> <url> [WxH]
	"$CHROME" --headless --disable-gpu --hide-scrollbars --window-size="${3:-1920,1080}" \
		--screenshot="$1" --virtual-time-budget=4000 "$2" >/dev/null 2>&1
}
R="file://$HERE"; Q="lang=$VIDEO_LANG"

cap "$D/title.png" "$R/title-card.html?$Q&ver=$(urlenc "$VER")&date=$(urlenc "$DATE")"
for s in 0 1 2 3 4 5 6 7 8; do cap "$D/s$s.png" "$R/intro-slides.html?$Q&s=$s"; done
for s in 1 2 3; do cap "$D/c$s.png" "$R/section-cards.html?$Q&s=$s"; done
cap "$D/end1.png" "$R/outro-card.html?$Q&s=1&ver=$(urlenc "$VER")"
cap "$D/end2.png" "$R/outro-card.html?$Q&s=2"
# 토폴로지 프레임은 SVG를 같은 폴더에서 읽는다(file:// 에서 상대 경로)
cp "$HERE/topology-frame.html" "$OUT/cards/.work/"
cap "$D/topo.png" "file://$OUT/cards/.work/topology-frame.html?$Q&f=topology.svg"

# 도입: 상태 9장을 xfade로 잇는다(각 상태를 몇 초 보일지는 아래 표 — 1.6·2.4·2.4·1.2·3.4·2.2·3.2·3.6·3.4초, 0.4초 페이드).
ffmpeg -nostdin -v error -y \
	-loop 1 -t 1.6 -i "$D/s0.png" -loop 1 -t 2.4 -i "$D/s1.png" -loop 1 -t 2.4 -i "$D/s2.png" \
	-loop 1 -t 1.2 -i "$D/s3.png" -loop 1 -t 3.4 -i "$D/s4.png" -loop 1 -t 2.2 -i "$D/s5.png" \
	-loop 1 -t 3.2 -i "$D/s6.png" -loop 1 -t 3.6 -i "$D/s7.png" -loop 1 -t 3.4 -i "$D/s8.png" \
	-filter_complex "[0:v][1:v]xfade=transition=fade:duration=0.4:offset=1.2[v1];[v1][2:v]xfade=transition=fade:duration=0.4:offset=3.2[v2];[v2][3:v]xfade=transition=fade:duration=0.4:offset=5.2[v3];[v3][4:v]xfade=transition=fade:duration=0.4:offset=6.0[v4];[v4][5:v]xfade=transition=fade:duration=0.4:offset=9.0[v5];[v5][6:v]xfade=transition=fade:duration=0.4:offset=10.8[v6];[v6][7:v]xfade=transition=fade:duration=0.4:offset=13.6[v7];[v7][8:v]xfade=transition=fade:duration=0.4:offset=16.8[v8]" \
	-map "[v8]" -r 30 -pix_fmt yuv420p "$D/intro.mp4"

# 마무리: (사이트 화면 둘) + 명령 카드 + 인사 카드. 사이트 화면은 SITE_URL을 줄 때만 넣는다.
# browser-frame.html은 캡처 이미지를 같은 폴더에서 읽으므로 작업 폴더에 함께 둔다.
parts=()
if [ -n "${SITE_URL:-}" ]; then
	W=$OUT/cards/.work
	cp "$HERE/browser-frame.html" "$W/"
	cap "$W/site-raw.png" "$SITE_URL/" 1500,940
	cap "$W/docs-raw.png" "$SITE_URL/developers/" 1500,940
	cap "$D/site.png" "file://$W/browser-frame.html?i=site-raw.png&u=${SITE_URL#https://}"
	cap "$D/docs.png" "file://$W/browser-frame.html?i=docs-raw.png&u=${SITE_URL#https://}/developers/"
	parts+=(-loop 1 -t 5 -i "$D/site.png" -loop 1 -t 5 -i "$D/docs.png")
fi
n=$(( ${#parts[@]} / 6 ))
parts+=(-loop 1 -t 6 -i "$D/end1.png" -loop 1 -t 3.4 -i "$D/end2.png")
total=$(( n + 2 ))
ffmpeg -nostdin -v error -y "${parts[@]}" \
	-filter_complex "concat=n=$total:v=1:a=0,fps=30,format=yuv420p[v]" -map "[v]" "$D/outro.mp4"
echo "cards: $D"
