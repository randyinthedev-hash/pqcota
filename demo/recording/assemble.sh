#!/usr/bin/env bash
# 조립 — 촬영본(clips/)과 정지 카드(.cap/)를 편집 지시서대로 이어 붙인다.
#
# 이 스크립트가 있는 이유: 조립을 손으로 하면 그 명령이 어디에도 남지 않아,
# 클립 하나를 다시 찍을 때마다 전체 타임라인을 기억에서 복원해야 한다.
# 타임라인은 아래 SEG 표가 전부다 — 고칠 곳도 거기 하나다.
#
# 쓰는 법:  ./assemble.sh [작업폴더]     (기본: 현재 폴더)
# 필요한 것: ffmpeg. clips/{intro,observe,provision,gap,outro}.mp4 와
#            .cap/{title,c1,c2,c3,topo}.png
#
# 규칙 둘 (편집 지시서와 같다):
#  · 정지 프레임은 그 구간의 **마지막**에서 뜬다. 앞에서 뜨면 이미 인쇄된 줄이
#    사라졌다 다시 나타나 영상을 조작한 것처럼 보인다.
#  · 숫자가 바뀌는 구간(0 → 14)은 배속하지 않는다. 근거가 사라진다.
set -euo pipefail

cd "${1:-.}"
OUT=${OUT:-final.mp4}
W=1920 H=1080 FPS=30
WORK=$(mktemp -d); trap 'rm -rf "$WORK"' EXIT

# 카드·그림은 1920×1080 그대로. 터미널 클립은 높이 960으로 맞춰 위아래 60px씩 띄운다 —
# 화면 끝에 줄이 붙으면 마지막 출력을 읽기 힘들다.
FIT="scale=$W:$H:force_original_aspect_ratio=decrease,pad=$W:$H:(ow-iw)/2:(oh-ih)/2:0x0e1420,setsar=1,fps=$FPS"
TERM_FIT="scale=-2:960:force_original_aspect_ratio=decrease,pad=$W:$H:(ow-iw)/2:(oh-ih)/2:0x0e1420,setsar=1,fps=$FPS"

n=0
still() { # still <png> <초>
	local f; f=$(printf '%s/%03d.mp4' "$WORK" "$n"); n=$((n + 1))
	ffmpeg -nostdin -v error -y -loop 1 -t "$2" -i "$1" \
		-vf "$FIT" -c:v libx264 -pix_fmt yuv420p -r $FPS "$f"
}
clip() { # clip <mp4> <시작> <끝> [배속]
	local f sp=${4:-1}; f=$(printf '%s/%03d.mp4' "$WORK" "$n"); n=$((n + 1))
	local vf="$TERM_FIT"
	[ "$sp" = 1 ] || vf="setpts=PTS/$sp,$TERM_FIT"
	ffmpeg -nostdin -v error -y -ss "$2" -to "$3" -i "$1" \
		-vf "$vf" -c:v libx264 -pix_fmt yuv420p -r $FPS "$f"
}
hold() { # hold <mp4> <뜰 시각> <초> — 그 구간의 끝에서 뜬다
	local f png; f=$(printf '%s/%03d.mp4' "$WORK" "$n"); png="$WORK/h$n.png"; n=$((n + 1))
	ffmpeg -nostdin -v error -y -ss "$2" -i "$1" -frames:v 1 "$png"
	ffmpeg -nostdin -v error -y -loop 1 -t "$3" -i "$png" \
		-vf "$TERM_FIT" -c:v libx264 -pix_fmt yuv420p -r $FPS "$f"
}
whole() { # whole <mp4> — 이미 1920×1080으로 만든 조립본(도입·마무리)
	local f; f=$(printf '%s/%03d.mp4' "$WORK" "$n"); n=$((n + 1))
	ffmpeg -nostdin -v error -y -i "$1" -vf "$FIT" -c:v libx264 -pix_fmt yuv420p -r $FPS "$f"
}

O=clips/observe.mp4 P=clips/provision.mp4 G=clips/gap.mp4

# ── 타임라인 ─────────────────────────────────────────────────────────
still .cap/title.png 4.0                    # 제목
whole clips/intro.mp4                       # 도입 슬라이드
still .cap/c1.png 3.5                       # 카드 ① 관측
clip  "$O" 0     10.0                       #   정적 스캔 → 0건
hold  "$O" 9.95  1.5                        #   (정지 — 0건)
clip  "$O" 10.0  14.2                       #   attach로 조회한 체인
clip  "$O" 14.2  16.4                       #   등급 표
hold  "$O" 16.35 5.0                        #   (정지 — 등급)
still .cap/topo.png 8.0                     # 토폴로지
still .cap/c2.png 4.5                       # 카드 ② 전환물 생성
clip  "$P" 0     7.0                        #   조치 전 0
clip  "$P" 7.0   16.9                       #   계획 JSON
clip  "$P" 16.9  22.6                       #   모듈은 사용자가 준비
clip  "$P" 22.6  27.4                       #   여기서부터 도구가 만든다
clip  "$P" 27.4  41.7  1.5                  #   생성물 + 적용 (진행 화면만 배속)
clip  "$P" 41.7  45.5                       #   놓인 파일 셋
clip  "$P" 45.5  49.3                       #   활성화(service.env)
clip  "$P" 49.3  63.5                       #   0 → 14  ← 배속 금지
hold  "$P" 63.45 1.5                        #   (정지 — 14)
clip  "$P" 63.5  77.0  1.5                  #   되돌림
clip  "$P" 77.0  80.3                       #   되돌린 뒤 0
hold  "$P" 80.25 3.5                        #   (정지 — 0 → 14 → 0)
still .cap/c3.png 3.5                       # 카드 ③ 정직성
clip  "$G" 0     9.2                        #   관측하지 못한 것
hold  "$G" 9.15  3.5                        #   (정지 — layersMissing)
whole clips/outro.mp4                       # 마무리 + 감사 카드
# ─────────────────────────────────────────────────────────────────────

ls "$WORK"/[0-9]*.mp4 | sed "s/^/file '/;s/$/'/" > "$WORK/list.txt"
ffmpeg -nostdin -v error -y -f concat -safe 0 -i "$WORK/list.txt" \
	-c:v libx264 -preset slow -crf 20 -pix_fmt yuv420p -movflags faststart "$OUT"

printf '%s  %s초\n' "$OUT" \
	"$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$OUT")"
