#!/usr/bin/env python3
"""Spike: check a built static site without trusting the generator's own warnings.
usage: check_site.py <site-dir>
For every <a href> in every page: external links are counted; internal links must resolve to an existing page,
and a #fragment must be an id on that page. Also reports mermaid blocks per page."""
import os, re, sys, html.parser, collections, urllib.parse

site = sys.argv[1]
PREFIX = sys.argv[sys.argv.index("--prefix") + 1].rstrip("/") if "--prefix" in sys.argv else ""
pages = {}


BLOCKS = ("p", "li", "td", "th", "dd", "h1", "h2", "h3", "h4")
CHROME = ("nav", "header", "footer")        # 탐색·머리·바닥은 본문이 아니다


class P(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids, self.hrefs, self.mermaid = set(), [], 0
        self.chrome = 0                       # nav/header/footer 안의 깊이
        self.block = []                       # 열린 본문 블록: [텍스트 조각, 이 블록 안의 링크 대상 목록]
        self.body_links = []                  # (링크 대상, 그 링크가 든 블록의 전체 텍스트) — 본문 링크만

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if "id" in a: self.ids.add(a["id"])
        if tag in CHROME: self.chrome += 1
        if tag in BLOCKS and not self.chrome: self.block.append([[], []])
        if tag == "a" and "href" in a:
            self.hrefs.append(a["href"])
            if self.block and not self.chrome and "pq-lang" not in (a.get("class") or ""):
                self.block[-1][1].append(a["href"])
        if tag in ("pre", "div") and "mermaid" in (a.get("class") or "").split(): self.mermaid += 1

    def handle_endtag(self, tag):
        if tag in CHROME and self.chrome: self.chrome -= 1
        if tag in BLOCKS and self.block and not self.chrome:
            texts, links = self.block.pop()
            text = "".join(texts)
            if self.block:
                self.block[-1][0].append(text)       # 바깥 블록(예: li 안의 p)에도 글자를 남긴다
            for l in links: self.body_links.append((l, text))

    def handle_data(self, data):
        if self.block: self.block[-1][0].append(data)


for d, _, fs in os.walk(site):
    for f in fs:
        if f.endswith(".html"):
            path = os.path.join(d, f)
            p = P(); p.feed(open(path, encoding="utf-8").read())
            pages[os.path.relpath(path, site)] = p


def target_page(frm, href):
    u = urllib.parse.urlparse(href)
    path = urllib.parse.unquote(u.path)
    base = os.path.dirname(frm)
    if path.startswith('/'):  # site-root link
        if PREFIX and not (path == PREFIX or path.startswith(PREFIX + '/')):
            return 'OUTSIDE-PREFIX:' + path, u.fragment
        path = path[len(PREFIX):] if PREFIX else path
        full = os.path.normpath(path.lstrip('/')) if path.strip('/') else ''
    else:
        full = os.path.normpath(os.path.join(base, path)) if path else frm
    if full == '.': full = ''
    if full.endswith("/") or os.path.isdir(os.path.join(site, full)): full = os.path.join(full, "index.html")
    if not full.endswith(".html") and os.path.exists(os.path.join(site, full + ".html")): full += ".html"
    return full, u.fragment


cnt = collections.Counter(); bad = []
for pg, p in pages.items():
    if pg in ("404.html", "ko/404.html"): continue  # the 404 pages use site-root links by design
    for h in p.hrefs:
        if re.match(r"(https?:|mailto:|javascript:)", h):
            cnt["external"] += 1
            if h.startswith("https://github.com/randyinthedev-hash/") and "/blob/main/docs/" in h: bad.append((pg, h, "points at main/docs on GitHub although the page is in the site"))
            continue
        if h.startswith("#") and len(h) == 1: continue
        full, frag = target_page(pg, h)
        if full not in pages and not os.path.exists(os.path.join(site, full)):
            cnt["internal broken (page)"] += 1; bad.append((pg, h, "page missing")); continue
        cnt["internal ok (page)"] += 1
        if frag:
            tp = pages.get(full)
            if tp is not None and frag not in tp.ids:
                cnt["internal broken (anchor)"] += 1; bad.append((pg, h, "anchor missing"))
            else:
                cnt["internal ok (anchor)"] += 1
# 한국어 쪽 본문(문단·목록 항목·표 칸 단위)에 사이트 안 영문 쪽으로 가는 링크가 있으면 그 블록 어딘가에 「영문」(또는 English)이 있어야 한다.
# 보장하는 범위: 블록 단위다. 링크가 여럿이면 하나만 표시해도 통과하고, 사이트 밖(GitHub 등)으로 가는 링크는 보지 않는다. 링크마다의 표시는 글쓴이가 지킨다.
unmarked = 0
for pg, p in pages.items():
    if not pg.startswith("ko/"): continue
    for h, text in p.body_links:
        if re.match(r"(https?:|mailto:|javascript:|#)", h): continue
        full, _ = target_page(pg, h)
        if isinstance(full, str) and not full.startswith("ko/") and not full.startswith("OUTSIDE-PREFIX"):
            tp = full if full in pages else None
            if tp is None: continue
            cnt["Korean page -> English page"] += 1
            if "영문" not in text and "English" not in text:
                unmarked += 1; bad.append((pg, h, "link to an English page without 영문/English in the same block"))
n_ko = sum(1 for pg in pages if pg.startswith("ko/"))
print("pages:", len(pages), "(English %d, Korean %d)" % (len(pages) - n_ko, n_ko))

for k, v in sorted(cnt.items()): print("  %-28s %d" % (k, v))
print("mermaid blocks:", {k: v.mermaid for k, v in pages.items() if v.mermaid})
print("problems:", len(bad))
for b in bad[:30]: print("  ", b)
