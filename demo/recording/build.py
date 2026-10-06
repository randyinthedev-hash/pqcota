#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 randyinthedev
# SPDX-License-Identifier: Apache-2.0
"""영상 조립 — 장면 표(SCENES)와 언어별 파일(lang/<lang>.json)에서 영상과 자막을 함께 만든다.

장면 이름과 순서, 배속, 정지 프레임을 찍을 자리는 여기(공통)에 있다. 정지 화면과 정지 시간, 자막 문구와 자리는
언어마다 읽는 시간이 달라 lang/<lang>.json에 둔다. 컷 경계는 시각이 아니라 record-take.sh가 .cast에 남긴
이름 있는 표지(예: provision.approve)로 찾는다 — 다시 찍어 시각이 바뀌어도 이 표는 그대로다.

시각의 규칙: 조립본의 시각은 .cast의 시각과 같다. agg가 대기 시간을 줄이지 않았다는 전제이고(record.sh의
IDLE_LIMIT), 아래 검사가 이를 확인한다. 녹화 영상은 마지막 화면을 몇 초 더 잡으므로 길이는 .cast보다 약간 길다.

usage: build.py [--lang en|ko] [--out demo/recording/out] [--version v0.10.1]
필요: ffmpeg(ass 필터 포함 — libass), out/clips/<lang>/*.{cast,mp4}, out/cards/<lang>/*
만드는 것: out/pqcota-demo.<lang>.mp4(자막 포함) · .nocaps.mp4(자막 없는 업로드용) · .srt · .ass · .provenance.txt
"""
import argparse, hashlib, json, os, re, shutil, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ap = argparse.ArgumentParser()
ap.add_argument("--lang", default=os.environ.get("VIDEO_LANG", "en"))
ap.add_argument("--out", default=os.path.join(HERE, "out"))
args = ap.parse_args()
LANG, OUT = args.lang, args.out
CLIPS, CARDS = f"{OUT}/clips/{LANG}", f"{OUT}/cards/{LANG}"
LANGFILE = f"{HERE}/lang/{LANG}.json"
L = json.load(open(LANGFILE, encoding="utf-8"))

W, H, FPS = 1920, 1080, 30
BG = "0x0e1420"
TERM_H, TERM_Y = 880, 20            # 터미널은 위 880px, 아래 180px은 자막 띠
IDLE_LIMIT = 60                     # record.sh의 --idle-time-limit과 같아야 한다
FIT = f"scale={W}:{H}:force_original_aspect_ratio=decrease,pad={W}:{H}:(ow-iw)/2:(oh-ih)/2:{BG},setsar=1,fps={FPS}"
TERM_FIT = (f"scale={W}:{TERM_H}:force_original_aspect_ratio=decrease,"
            f"pad={W}:{H}:(ow-iw)/2:{TERM_Y}:{BG},setsar=1,fps={FPS}")
ENC = ["-c:v", "libx264", "-preset", "fast", "-crf", "12", "-pix_fmt", "yuv420p", "-r", str(FPS)]

# 장면 표 — (이름, 종류, 인자). 종류:
#   still(png)  whole(mp4)  clip(원본, 시작 표지|None, 끝 표지, 배속)  hold(원본, 표지, 표지에서의 초 차이)
# 규칙: 정지 프레임은 그 구간의 끝에서 뜬다(앞에서 뜨면 이미 찍힌 줄이 사라졌다 나타난다). 숫자가 바뀌는
# 구간(provision.activation → provision.after, 0 → 14)은 배속하지 않는다.
SCENES = [
    ("title", "still", "title.png"),
    ("intro", "whole", "intro.mp4"),
    ("card1", "still", "c1.png"),
    ("obs-static", "clip", "observe", None, "observe.static", 1),
    ("hold-0", "hold", "observe", "observe.static", -0.05),
    ("obs-chain", "clip", "observe", "observe.static", "observe.chain", 1),
    ("hold-chain", "hold", "observe", "observe.chain", 0.1),
    ("hold-edges", "hold", "observe", "observe.edges", -0.05),
    ("topo", "still", "topo.png"),
    ("card2", "still", "c2.png"),
    ("pv-before", "clip", "provision", None, "provision.before", 1),
    ("pv-plan", "clip", "provision", "provision.before", "provision.approve", 1),
    ("pv-module", "clip", "provision", "provision.approve", "provision.module", 1),
    ("pv-gen", "clip", "provision", "provision.module", "provision.output", 1),
    ("pv-apply", "clip", "provision", "provision.output", "provision.apply", 1.5),
    ("pv-landed", "clip", "provision", "provision.apply", "provision.activation", 1),
    ("pv-14", "clip", "provision", "provision.activation", "provision.after", 1),   # 0 → 14: 배속 금지
    ("hold-14", "hold", "provision", "provision.after", -0.05),
    ("pv-rollback", "clip", "provision", "provision.after", "provision.deactivated", 1),
    ("pv-back0", "clip", "provision", "provision.deactivated", "provision.rollback", 1),
    ("hold-final", "hold", "provision", "provision.rollback", -0.05),
    ("card3", "still", "c3.png"),
    ("gap", "clip", "gap", None, "gap.result", 1),
    ("hold-gap", "hold", "gap", "gap.result", -0.05),
    ("outro", "whole", "outro.mp4"),
]


def sh(*a):
    subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-y", *a], check=True)


def probe(f):
    return float(subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                                         "-of", "csv=p=0", f], text=True))


# ── 녹화본에서 표지와 시각을 읽고, 시각 규칙을 확인한다 ──────────────────────────────
MARK = re.compile(r"\x1b\]777;pqcota-mark;([A-Za-z0-9._-]+)\x07")
marks, cast_end = {}, {}
for cut in ("observe", "provision", "gap"):
    ev = [json.loads(l) for l in open(f"{CLIPS}/{cut}.cast", encoding="utf-8") if l.startswith("[")]
    times = [e[0] for e in ev if e[1] == "o"]
    gap = max(b - a for a, b in zip(times, times[1:]))
    if gap >= IDLE_LIMIT:
        sys.exit(f"{cut}: a {gap:.1f}s silence reaches the idle limit ({IDLE_LIMIT}s); cast time would no longer equal video time")
    cast_end[cut] = ev[-1][0]
    vid = probe(f"{CLIPS}/{cut}.mp4")
    if not (cast_end[cut] - 0.5 <= vid <= cast_end[cut] + 6):
        sys.exit(f"{cut}: video is {vid:.1f}s but the cast ends at {cast_end[cut]:.1f}s; agg shortened or stretched it")
    for t, k, d in ev:
        for m in MARK.finditer(d if k == "o" else ""):
            if m.group(1) in marks:
                sys.exit(f"marker {m.group(1)} appears twice")
            marks[m.group(1)] = t


def at(name):
    if name is None:
        return 0.0
    if name not in marks:
        sys.exit(f"marker {name} is not in the recordings (record-take.sh and the scene table disagree)")
    return marks[name]


work = tempfile.mkdtemp()
start, dur, files = {}, {}, []
t = 0.0
for i, sc in enumerate(SCENES):
    name, kind, *a = sc
    f = f"{work}/{i:03d}.mp4"
    if kind == "still":
        d = L["stills"][name]
        sh("-loop", "1", "-t", str(d), "-i", f"{CARDS}/{a[0]}", "-vf", FIT, *ENC, f)
    elif kind == "whole":
        sh("-i", f"{CARDS}/{a[0]}", "-vf", FIT, *ENC, f)
    elif kind == "clip":
        src, m0, m1, sp = a
        s0, s1 = at(m0), at(m1)
        assert s1 > s0, (name, s0, s1)
        vf = TERM_FIT if sp == 1 else f"setpts=PTS/{sp},{TERM_FIT}"
        sh("-ss", f"{s0:.3f}", "-to", f"{s1:.3f}", "-i", f"{CLIPS}/{src}.mp4", "-vf", vf, *ENC, f)
    elif kind == "hold":
        src, m, dt = a
        png = f"{work}/h{i}.png"
        sh("-ss", f"{max(at(m) + dt, 0):.3f}", "-i", f"{CLIPS}/{src}.mp4", "-frames:v", "1", png)
        sh("-loop", "1", "-t", str(L["holds"][name]), "-i", png, "-vf", TERM_FIT, *ENC, f)
    start[name], dur[name] = t, probe(f)
    t += dur[name]
    files.append(f)


# ── 자막: (장면, 장면 안 초)로 적혀 있어 장면 길이가 바뀌어도 밀리지 않는다 ───────────────
def stamp(x):
    ms = round(x * 1000)
    return "%02d:%02d:%02d,%03d" % (ms // 3600000, ms // 60000 % 60, ms // 1000 % 60, ms % 1000)


def ass_time(x):
    cs = round(x * 100)
    return "%d:%02d:%02d.%02d" % (cs // 360000, cs // 6000 % 60, cs // 100 % 60, cs % 100)


CLIP_SCENES = [(sc[0], sc[2], at(sc[3]), at(sc[4]), sc[5]) for sc in SCENES if sc[1] == "clip"]


def T(ref, d, is_end=False):
    """자막의 한 지점을 조립본 시각으로 바꾼다.
    ref가 장면 이름이면 '그 장면 시작 + d초'(조립본 초). '@표지'면 '그 표지의 .cast 시각 + d초'(녹화 초)이고,
    그 시각이 속한 클립 장면을 찾아 배속을 반영해 옮긴다 — 녹화 속도가 달라져 장면 길이가 변해도 자막은 화면 내용을
    따라간다. 장면 안에서 시작하는 자막은 시작 쪽 장면을, 끝나는 자막은 끝 쪽 장면을 고른다(경계에서 모호하지 않게)."""
    if not ref.startswith("@"):
        return start[ref] + d
    name = ref[1:]
    cut, tc = name.split(".")[0], at(name) + d
    for sc, src, s0, s1, sp in CLIP_SCENES:
        if src == cut and ((s0 < tc <= s1) if is_end else (s0 <= tc < s1)):
            return start[sc] + (tc - s0) / sp
    sys.exit(f"caption anchor {ref}{d:+} falls outside every clip scene of {cut}")


caps, prev_end = [], 0.0
for r0, o0, r1, o1, text in L["captions"]:
    a, b = T(r0, o0), T(r1, o1, True)
    assert b > a, ("caption has no length", text, a, b)
    if not r0.startswith("@"): assert o0 <= dur[r0] + 0.05, ("caption starts after its scene", text)
    if not r1.startswith("@"): assert o1 <= dur[r1] + 0.05, ("caption ends after its scene", text)
    assert a >= prev_end - 1e-6, ("caption overlaps the previous one", text, a, prev_end)
    cps = len(text) / (b - a)
    assert cps <= L.get("max_cps", 16.5), ("caption too fast", round(cps, 1), text)
    caps.append((a, b, text)); prev_end = b

base = f"{OUT}/pqcota-demo.{LANG}"
with open(base + ".srt", "w", encoding="utf-8") as f:
    for n, (a, b, text) in enumerate(caps, 1):
        f.write(f"{n}\n{stamp(a)} --> {stamp(b)}\n{text}\n\n")
ASS = f"""[Script Info]
ScriptType: v4.00+
PlayResX: {W}
PlayResY: {H}
WrapStyle: 0

[V4+ Styles]
Format: Name,Fontname,Fontsize,PrimaryColour,SecondaryColour,OutlineColour,BackColour,Bold,Italic,Underline,StrikeOut,ScaleX,ScaleY,Spacing,Angle,BorderStyle,Outline,Shadow,Alignment,MarginL,MarginR,MarginV,Encoding
Style: Cap,{L.get("font", "DejaVu Sans")},40,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,2,180,180,60,1

[Events]
Format: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text
"""
with open(base + ".ass", "w", encoding="utf-8") as f:
    f.write(ASS)
    for a, b, text in caps:
        f.write("Dialogue: 0,%s,%s,Cap,,0,0,0,,%s\n" % (ass_time(a), ass_time(b), text))

# ── 이어 붙이기: 구간은 거의 무손실로 만들었고 여기서 한 번만 다시 인코딩한다 ─────────────────────
lst = f"{work}/list.txt"
open(lst, "w").write("".join(f"file '{f}'\n" for f in files))
final = ["-c:v", "libx264", "-preset", "slow", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "faststart"]
subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "concat", "-safe", "0", "-i", lst, *final,
                base + ".nocaps.mp4"], check=True)                       # 업로드용: 자막 없는 영상 + 선택형 SRT
subprocess.run(["ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "concat", "-safe", "0", "-i", lst,
                "-vf", f"ass={base}.ass", *final, base + ".mp4"], check=True)   # 직접 배포용: 자막 포함
shutil.rmtree(work)

# ── 제작 기록 ───────────────────────────────────────────────────────────────────────
def sha(p):
    return hashlib.sha256(open(p, "rb").read()).hexdigest()[:16]


prov = open(f"{CLIPS}/PROVENANCE.txt", encoding="utf-8").read() if os.path.exists(f"{CLIPS}/PROVENANCE.txt") else "(no recording provenance)\n"
with open(base + ".provenance.txt", "w", encoding="utf-8") as f:
    f.write(f"language: {LANG}\nlang file: lang/{LANG}.json sha256:{sha(LANGFILE)}\n")
    f.write(f"build.py: sha256:{sha(os.path.abspath(__file__))}\n")
    f.write(subprocess.check_output(["ffmpeg", "-version"], text=True).split("\n")[0] + "\n")
    f.write("--- recording ---\n" + prov)
    f.write("--- markers (cast seconds) ---\n" + "".join(f"{k} {v:.2f}\n" for k, v in sorted(marks.items(), key=lambda x: x[1])))
    f.write("--- scenes (assembled seconds) ---\n" + "".join(f"{n} {start[n]:.2f} {dur[n]:.2f}\n" for n, *_ in SCENES))
print(base + ".mp4", "%.1f s" % probe(base + ".mp4"), "|", len(caps), "captions | nocaps + srt + provenance written")
