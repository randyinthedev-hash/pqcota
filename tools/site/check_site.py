#!/usr/bin/env python3
"""Spike: check a built static site without trusting the generator's own warnings.
usage: check_site.py <site-dir>
For every <a href> in every page: external links are counted; internal links must resolve to an existing page,
and a #fragment must be an id on that page. Also reports mermaid blocks per page."""
import os, re, sys, html.parser, collections, urllib.parse

site = sys.argv[1]
PREFIX = sys.argv[sys.argv.index("--prefix") + 1].rstrip("/") if "--prefix" in sys.argv else ""
pages = {}


class P(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids, self.hrefs, self.mermaid = set(), [], 0

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if "id" in a: self.ids.add(a["id"])
        if tag == "a" and "href" in a: self.hrefs.append(a["href"])
        if tag in ("pre", "div") and "mermaid" in (a.get("class") or "").split(): self.mermaid += 1


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
    if pg == "404.html": continue  # the 404 page uses site-root links by design
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
print("pages:", len(pages)); 
for k, v in sorted(cnt.items()): print("  %-28s %d" % (k, v))
print("mermaid blocks:", {k: v.mermaid for k, v in pages.items() if v.mermaid})
print("problems:", len(bad))
for b in bad[:30]: print("  ", b)
