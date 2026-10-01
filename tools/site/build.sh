#!/bin/bash
# usage: build.sh <workspace-with-five-repos> <out-site-dir> <ref>
# 환경변수: PAGES = 고정 쪽 목록 파일(기본은 이 스크립트 옆 pages.txt. 도구와 문서를 다른 커밋에서 받을 때 문서 쪽 것을 준다)
#          TOOLS_REF = 이 도구를 받은 커밋(산출물의 build-info.txt에 적는다)
#          LANDING_ROOT = 손으로 쓴 랜딩·구조도(docs/index.html 등)를 가져올 pqcota 작업 트리(기본은 문서와 같은 $WS/pqcota).
#                         랜딩은 릴리스 문서와 따로 고칠 일이 있어(영상 링크 등) 도구 커밋에서 받게 할 수 있다
# 고정된 쪽 목록(pages.txt)으로 모으고, 빌드하고, 기존 랜딩·구조도를 얹은 뒤, 그 최종 트리를 검사한다.
set -euo pipefail
WS=$(cd "$1" && pwd); OUT=$2; REF=$3; ORG=randyinthedev-hash
S=$(cd "$(dirname "$0")" && pwd)
W=$(mktemp -d)
PAGES=${PAGES:-$S/pages.txt}
# 어느 릴리스 기준인지 사이트에 적는다. 태그(v로 시작)가 아니면 릴리스가 아닌 빌드로 적는다.
if [[ "$REF" =~ ^v[0-9] ]]; then LABEL="pqcota $REF"; NOTE="This documentation matches release $REF. The development version is on GitHub (main)."
else LABEL="pqcota (unreleased: $REF)"; NOTE="This documentation was built from $REF, not from a release."; fi
python "$S/assemble.py" "$WS" "$W" --home overview.md --pages "$PAGES" --ref "$REF" > "$W/assemble.txt"
( cd "$W"
  cat "$S/mkdocs.full.base.yml" nav.yml > mkdocs.yml
  sed -i "s#^site_name:.*#site_name: \"$LABEL\"\nsite_url: https://$ORG.github.io/pqcota/\ncopyright: \"$NOTE\"#" mkdocs.yml
  zensical build -f mkdocs.yml )
rm -rf "$OUT"; mv "$W/site" "$OUT"; cp "$W/assemble.txt" "$OUT/../assemble.txt" 2>/dev/null || true
# 루트 index.html과 구조도는 손으로 쓴 쪽이 맡는다. 생성물과 겹치면 멈춘다.
test ! -e "$OUT/index.html" || { echo "생성된 사이트가 루트 index.html을 만들었다. 충돌"; exit 1; }
test ! -e "$OUT/architectures" || { echo "생성된 사이트가 architectures/를 만들었다. 충돌"; exit 1; }
LAND=${LANDING_ROOT:-$WS/pqcota}
cp "$LAND/docs/index.html" "$OUT/index.html"
mkdir -p "$OUT/architectures" && cp "$LAND/docs/architectures/platform-structure.html" "$OUT/architectures/"
# 게시 소스가 main:/docs였던 때의 원문 주소(/pqcota/<이름>.md)를 같은 경로로 보존한다. 생성물과 겹치면 멈춘다.
N=0
for f in "$WS"/pqcota/docs/*.md; do
  b=$(basename "$f")
  test ! -e "$OUT/$b" || { echo "원문 $b 가 생성물과 겹친다"; exit 1; }
  cp "$f" "$OUT/$b"; cmp -s "$f" "$OUT/$b" || { echo "$b 복사가 원본과 다르다"; exit 1; }
  N=$((N+1))
done
test "$N" -eq 10 || { echo "원문이 10편이 아니다($N). 보존 범위를 다시 정한다"; exit 1; }
# 랜딩의 개발자 링크는 이 빌드 결과 안의 주소로만 바꾼다(리포의 docs/index.html은 그대로).
OLD="https://github.com/$ORG/pqcota/blob/main/docs/developers.md"
grep -q "$OLD" "$OUT/index.html" || { echo "랜딩에 기대한 개발자 링크가 없다"; exit 1; }
sed -i "s#$OLD#developers/#" "$OUT/index.html"
python "$S/check_site.py" "$OUT" --prefix /pqcota | tee "$OUT/../check.txt"
grep -q "^problems: 0" "$OUT/../check.txt" || { echo "링크 검사에서 문제가 나왔다"; exit 1; }
# 다시 만들 수 있도록 입력을 적는다: 문서 입력(다섯 리포의 같은 ref와 각 커밋), 도구 커밋, 도구 버전.
{ echo "docs ref: $REF"
  echo "landing from: $(git -C "$LAND" rev-parse HEAD 2>/dev/null || echo unknown) ($([ "$LAND" = "$WS/pqcota" ] && echo "same as docs" || echo "tools commit"))"
  for r in pqcota pqcota-common pqcota-inventory pqcota-discovery pqcota-provisioning; do echo "$r $(git -C "$WS/$r" rev-parse HEAD)"; done
  echo "tools commit: ${TOOLS_REF:-$(git -C "$S" rev-parse HEAD 2>/dev/null || echo unknown)}"
  echo "zensical: $(zensical --version 2>/dev/null)"; } > "$OUT/build-info.txt"
( cd "$OUT" && find . -type f -print0 | sort -z | xargs -0 sha256sum ) > "$OUT/../tree.sha256"
