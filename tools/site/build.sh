#!/bin/bash
# usage: build.sh <workspace-with-five-repos> <out-site-dir> <ref>
# 고정된 쪽 목록(pages.txt)으로 모으고, 빌드하고, 기존 랜딩·구조도를 얹은 뒤, 그 최종 트리를 검사한다.
set -euo pipefail
WS=$(cd "$1" && pwd); OUT=$2; REF=$3; ORG=randyinthedev-hash
S=$(cd "$(dirname "$0")" && pwd)
W=$(mktemp -d)
python "$S/assemble.py" "$WS" "$W" --home overview.md --pages "$S/pages.txt" --ref "$REF" > "$W/assemble.txt"
( cd "$W"
  cat "$S/mkdocs.full.base.yml" nav.yml > mkdocs.yml
  sed -i "s#^site_name:.*#site_name: pqcota\nsite_url: https://$ORG.github.io/pqcota/#" mkdocs.yml
  zensical build -f mkdocs.yml )
rm -rf "$OUT"; mv "$W/site" "$OUT"; cp "$W/assemble.txt" "$OUT/../assemble.txt" 2>/dev/null || true
# 루트 index.html과 구조도는 손으로 쓴 쪽이 맡는다. 생성물과 겹치면 멈춘다.
test ! -e "$OUT/index.html" || { echo "생성된 사이트가 루트 index.html을 만들었다. 충돌"; exit 1; }
test ! -e "$OUT/architectures" || { echo "생성된 사이트가 architectures/를 만들었다. 충돌"; exit 1; }
cp "$WS/pqcota/docs/index.html" "$OUT/index.html"
mkdir -p "$OUT/architectures" && cp "$WS/pqcota/docs/architectures/platform-structure.html" "$OUT/architectures/"
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
( cd "$OUT" && find . -type f -print0 | sort -z | xargs -0 sha256sum ) > "$OUT/../tree.sha256"
