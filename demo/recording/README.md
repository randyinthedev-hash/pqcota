English · [한국어](README.ko.md)

# Making the demo video

This folder is the source of truth for the demo video. It holds the tools and the text, not the video: everything that is generated goes to `out/`, which git ignores.

The video is silent. Terminal scenes are real recordings of the demo, with captions in a band below them. It can be made in English or in Korean. The cards (title, intro, section titles, closing) are HTML pages captured as images.

## Three steps, three machines

Each step needs different software, so they are separate scripts.

| Step | Script | Run it where | Needs |
|---|---|---|---|
| 1. Record the terminal scenes | `record.sh` | Where the demo runs (Docker) | `asciinema`, `agg`, `ffmpeg`, the five repositories side by side |
| 2. Capture the cards | `cards.sh` | Anywhere with Chrome | headless Chrome, `ffmpeg` |
| 3. Assemble | `build.py` | Anywhere with `ffmpeg` built with `libass` | `ffmpeg` with the `ass` filter |

```bash
# 1. on the machine that runs the demo (the real-provider stage must have run once)
./demo/scripts/up.sh && DEMO_REAL_PROVIDER=1 ./demo/scripts/demo.sh
VIDEO_LANG=en VIDEO_VER=v0.10.1 demo/recording/record.sh   # → out/clips/en/*.cast, *.mp4, PROVENANCE.txt

# 2. cards (copy demo/.generated/topology.svg over if this is another machine)
VIDEO_LANG=en VIDEO_VER=v0.10.1 SITE_URL=https://randyinthedev-hash.github.io/pqcota demo/recording/cards.sh

# 3. assemble (put out/clips and out/cards in the same out/ first)
demo/recording/build.py --lang en             # → out/pqcota-demo.en.mp4 and friends
```

`VIDEO_LANG` (or `--lang` for `build.py`) picks the language: `en` or `ko`. Run the three steps once per language. Do not use `LANG`: it is the operating system's locale variable.

## What comes out

- `pqcota-demo.<lang>.mp4`: captions burned in, for direct sharing.
- `pqcota-demo.<lang>.nocaps.mp4` and `pqcota-demo.<lang>.srt`: the same video without captions, plus the captions as a file. Upload this pair to a site that shows captions itself, so they are not shown twice.
- `pqcota-demo.<lang>.provenance.txt`: how it was made (below).

## How the pieces fit

- **`record-take.sh`** (in `../scripts/`) runs one scene at a time against the running demo. It prints named markers, such as `provision.approve`, that do not show on screen but are kept in the recording. A marker name means "the section that ends here".
- **`build.py`** finds the cuts by those names, not by seconds. Its scene table holds what is common to every language: the order of scenes, the speed, and where a still frame is taken. If you re-record and the seconds change, the table does not.
- **`lang/<lang>.json`** holds what depends on the language: how long a still stays, the caption text with its position, the caption font and the reading-speed limit. A position is a scene and seconds inside it, or `@marker` and seconds around that marker in the recording. Reading time differs between languages, so each language sets its own stills, holds and limit. The scene order and the anchors are the same in every language.
- **`lang/take.<lang>.sh`** holds the commentary lines that `record-take.sh` prints between commands. English is the reference and the other languages are translations; the commands and the programs' own output stay English in every language.
- **The cards** (`*-card.html`, `intro-slides.html`, `section-cards.html`, `topology-frame.html`) hold both languages in one file, as `<span class="en">` and `<span class="ko">`. `?lang=` picks one, and English is the default.
- **Time rule.** Seconds in the assembled video equal seconds in the `.cast` as long as `agg` does not shorten waiting time. `record.sh` sets the idle limit to 60 seconds, and `build.py` stops if a recording has a silence that long or if the video length and the cast length disagree.
- **Two rules for editing.** A still frame is taken at the end of its scene. If it were taken at the start, lines that were already printed would disappear and come back, which looks edited. Where a number changes (0 → 14), the clip is never sped up.

## The record of how it was made

`record.sh` writes `out/clips/<lang>/PROVENANCE.txt`: the commit of each of the five repositories (and whether it is a release tag and whether the tree was clean), the tool versions, and the recording options. `build.py` adds the caption file's hash, the marker times and the scene times into `pqcota-demo.<lang>.provenance.txt`. The closing card shows the release tag the video was recorded from (`VIDEO_VER`); when the recorded commits are not that tag exactly, the provenance file shows how they differ.

## Checking a rebuild

A rebuilt video is the same video when the scenes come in the same order, each scene shows the command that was run, the key outputs match (the provider chain with `BC`, the `0 → 14 → 0` counts, `layersMissing`), and the captions say the same things. Bytes will differ. A last check is to watch the whole video at normal speed once.

## Adding a language

Add `lang/<lang>.json` and `lang/take.<lang>.sh`, and add the language's spans to each card. Then list the two new files in `tools/checkprose/files.txt`, so the prose check reads them (it reads `.json` as it is and `.sh` without comment lines).

Korean exists (`ko`). Its video differs from the 2026 August video in one way: the programs now print English, so the terminal output is English and only the commentary, the cards and the captions are Korean.
