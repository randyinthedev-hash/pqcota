#!/usr/bin/env bash
# 터미널 장면을 녹화한다 — ubuntu-dev(docker·asciinema·agg·ffmpeg가 있는 곳)에서 돌린다.
#
# 전제: ./demo/scripts/up.sh 와 DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh 가 한 번 돌아 있어야 한다
# (record-take.sh가 그 결과를 그대로 읽는다).
# 쓰는 법:  [VIDEO_VER=v0.10.1] demo/recording/record.sh [out-dir]       (기본: demo/recording/out)
# 만드는 것: <out>/clips/{observe,provision,gap}.{cast,mp4} 와 PROVENANCE.txt
#
# 시각의 규칙 — build.py가 컷 경계를 .cast의 이름 있는 표지(record-take.sh의 mark)로 찾는다.
# 조립본의 시각이 .cast의 시각과 같으려면 agg가 대기 시간을 줄이면 안 된다. 그래서 --idle-time-limit을
# 어떤 녹화의 가장 긴 공백보다 크게 둔다(IDLE_LIMIT). build.py는 이를 다시 확인한다.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT=${1:-$HERE/out}
CLIPS=$OUT/clips
COLS=${COLS:-132} ROWS=${ROWS:-32}
FONT_SIZE=${FONT_SIZE:-24} LINE_HEIGHT=${LINE_HEIGHT:-1.2}
IDLE_LIMIT=${IDLE_LIMIT:-60}
THEME=0e1420,e6ebf2,161f2e,e0736b,7fc08a,e2c08d,7aa2f7,c0a3e0,7fd1c5,c3cbd8   # 구조도 팔레트
FONTS="DejaVu Sans Mono,Noto Color Emoji"                                      # 고정폭을 앞에: 셀 폭 계산

for c in asciinema agg ffmpeg; do command -v "$c" >/dev/null || { echo "$c is required" >&2; exit 1; }; done
mkdir -p "$CLIPS"

for cut in observe provision gap; do
	[ -z "${PROVENANCE_ONLY:-}" ] || break      # PROVENANCE_ONLY=1: 녹화는 건너뛰고 기록만 다시 쓴다
	echo "▶ recording $cut"
	(cd "$ROOT" && TERM=xterm-256color asciinema rec "$CLIPS/$cut.cast" --cols "$COLS" --rows "$ROWS" \
		--overwrite -q -c "./demo/scripts/record-take.sh $cut" </dev/null >/dev/null)
	agg "$CLIPS/$cut.cast" "$CLIPS/$cut.gif" --font-family "$FONTS" --font-size "$FONT_SIZE" \
		--line-height "$LINE_HEIGHT" --theme "$THEME" --idle-time-limit "$IDLE_LIMIT" --fps-cap 30 >/dev/null
	ffmpeg -nostdin -v error -y -i "$CLIPS/$cut.gif" -movflags faststart -pix_fmt yuv420p \
		-vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" "$CLIPS/$cut.mp4"
	rm -f "$CLIPS/$cut.gif"
done

# 제작 기록 — 어떤 코드·도구·옵션으로 찍었는지. 영상은 이 기록으로 다시 만들 수 있다.
{
	echo "recorded: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	for r in pqcota pqcota-common pqcota-inventory pqcota-discovery pqcota-provisioning; do
		d=$ROOT/../$r; [ "$r" = pqcota ] && d=$ROOT
		printf '%s %s tag=%s dirty=%s\n' "$r" "$(git -C "$d" rev-parse HEAD)" \
			"$(git -C "$d" describe --tags --exact-match 2>/dev/null || echo none)" "$(git -C "$d" status --short | wc -l | tr -d ' ')"
		# VIDEO_VER를 주면, 찍은 커밋이 그 릴리스 태그와 어떻게 다른지(문서가 아닌 파일만) 적는다.
		if [ -n "${VIDEO_VER:-}" ]; then
			printf '  vs %s: %s\n' "$VIDEO_VER" "$(git -C "$d" diff --name-only "$VIDEO_VER" HEAD 2>/dev/null |
				grep -v -E '\.md$|^docs/|RELEASE_NOTES' | tr '\n' ' ' | sed 's/ $//' | sed 's/^$/(no non-doc file differs)/')"
		fi
	done
	echo "asciinema: $(asciinema --version 2>&1 | head -1)"
	echo "agg: $(agg --version 2>&1 | head -1)"
	echo "ffmpeg: $(ffmpeg -version | head -1)"
	echo "options: cols=$COLS rows=$ROWS font=\"$FONTS\" size=$FONT_SIZE line-height=$LINE_HEIGHT idle-limit=$IDLE_LIMIT fps-cap=30"
	echo "demo: DEMO_REAL_PROVIDER=1 (real provider stage)"
} > "$CLIPS/PROVENANCE.txt"
cat "$CLIPS/PROVENANCE.txt"
