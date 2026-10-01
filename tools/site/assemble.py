#!/usr/bin/env python3
"""Spike: assemble one documentation tree from the five pqcota repositories.

usage: assemble.py <workspace-root> <out-dir> [--ref main] [--hugo] [--home index.md] [--pages FILE | --write-pages FILE]

--pages fails when the discovered page list differs from FILE ("repo<TAB>path<TAB>destination" per line);
--write-pages writes that list.

Copies the selected Markdown files into <out-dir>/docs (or <out-dir>/content with --hugo) and rewrites links so
that a link between two copied pages becomes a relative link inside the new tree, and a link to anything else
becomes a GitHub URL at --ref. Writes <out-dir>/link-report.json. Nothing here touches the repositories.
"""
import json, os, re, subprocess, sys, shutil, collections

ORG = "randyinthedev-hash"
STAGE = {"pqcota-common": "common", "pqcota-inventory": "inventory", "pqcota-discovery": "discovery",
         "pqcota-provisioning": "provisioning"}
USERS = {"docs/pqc-migration-primer.md": "users/first-migration.md", "docs/faq.md": "users/faq.md",
         "docs/reporting-guide.md": "users/reporting-guide.md"}
DIRNAME = {"cmd": "commands", "ansible": "playbook", "contracts": "contracts"}
HOME = "index.md"   # --home overview.md keeps the site root free for a hand-written landing page
REPOS = ["pqcota", "pqcota-common", "pqcota-inventory", "pqcota-discovery", "pqcota-provisioning"]


def destination(repo, path):
    """Where a repository file goes in the site tree."""
    if repo == "pqcota":
        if path == "README.md": return HOME
        if path in USERS: return USERS[path]
        if path == "docs/developers.md": return "developers/index.md"
        if path.startswith("docs/"): return "developers/" + os.path.basename(path)
        if "/" not in path: return "project/" + path.lower()          # CONTRIBUTING, SECURITY, ...
        if path.endswith("/README.md"): return path[:-len("README.md")] + "index.md"   # demo/..., examples/...
        return path
    base = "stages/%s/" % STAGE[repo]
    if path == "README.md": return base + "index.md"
    if path.endswith("/README.md"):
        d = path[:-len("/README.md")]
        return base + (DIRNAME[d] + ".md" if d in DIRNAME else d + "/index.md")
    return base + os.path.basename(path)


def discover(root, with_release_notes=False):
    out = []
    for repo in REPOS:
        files = subprocess.check_output(["git", "-C", os.path.join(root, repo), "ls-files", "*.md"], text=True).split("\n")
        for f in files:
            if not f or (f == "RELEASE_NOTES.md" and not with_release_notes): continue
            text = open(os.path.join(root, repo, f), encoding="utf-8").read()
            m = re.search(r"^#\s+(.+?)\s*$", text, re.M)
            title = re.sub(r"[`*]", "", m.group(1)) if m else os.path.basename(f)
            out.append((repo, f, destination(repo, f), title))
    return out


PAGES, MAP, TITLE = [], {}, {}
IMAGES = {}  # (repo, path) -> destination under assets/
LINK = re.compile(r'(!?)\[((?:[^\[\]]|!\[[^\]]*\]\([^)]*\))*)\]\(([^)\s]+)((?:\s+"[^"]*")?)\)')
GH = re.compile(r'https://github\.com/%s/(pqcota[\w-]*)/(blob|tree)/(?:main|v[\w.]+)/([^#]*)(?:#(.*))?$' % ORG)


def github_url(repo, path, ref, kind="blob", anchor=""):
    u = "https://github.com/%s/%s/%s/%s/%s" % (ORG, repo, kind, ref, path)
    return u + ("#" + anchor if anchor else "")


def rewrite(text, repo, src, ref, root, stats, detail):
    dest_src = MAP[(repo, src)]
    out, fence = [], False
    for line in text.split("\n"):
        if line.lstrip().startswith("```"):
            fence = not fence
            out.append(line)
            continue
        if fence:
            out.append(line)
            continue

        def sub(m):
            bang, label, target, title = m.groups()
            if target.startswith("#"):
                stats["same-page anchor"] += 1
                return m.group(0)
            gm = GH.match(target)
            if gm:
                trepo, kind, tpath, anchor = gm.group(1), gm.group(2), gm.group(3).rstrip("/"), gm.group(4) or ""
                key = (trepo, tpath)
                if key in MAP:
                    rel = os.path.relpath(MAP[key], os.path.dirname(dest_src)) + ("#" + anchor if anchor else "")
                    stats["cross-repo github -> internal"] += 1
                    detail.append((dest_src, target, rel))
                    return "%s[%s](%s%s)" % (bang, label, rel, title)
                stats["cross-repo github -> github (ref)"] += 1
                new = github_url(trepo, tpath, ref, kind, anchor)
                detail.append((dest_src, target, new))
                return "%s[%s](%s%s)" % (bang, label, new, title)
            if re.match(r"(https?:|mailto:)", target):
                stats["external"] += 1
                return m.group(0)
            path, _, anchor = target.partition("#")
            tpath = os.path.normpath(os.path.join(os.path.dirname(src), path)) if path else src
            if tpath.startswith(".."):
                stats["leaves the repository (kept)"] += 1
                detail.append((dest_src, target, "KEPT"))
                return m.group(0)
            key = (repo, tpath)
            full0 = os.path.join(root, repo, tpath)
            if bang and os.path.isfile(full0) and not tpath.endswith(".md"):
                asset = "assets/%s/%s" % (repo, tpath)
                os.makedirs(os.path.dirname(os.path.join(OUT, asset)), exist_ok=True)
                shutil.copyfile(full0, os.path.join(OUT, asset))
                rel = os.path.relpath(asset, os.path.dirname(dest_src))
                stats["image copied into the site"] += 1
                detail.append((dest_src, target, rel))
                return "%s[%s](%s%s)" % (bang, label, rel, title)
            if key in MAP:
                rel = os.path.relpath(MAP[key], os.path.dirname(dest_src)) + ("#" + anchor if anchor else "")
                stats["relative -> internal"] += 1
                detail.append((dest_src, target, rel))
                return "%s[%s](%s%s)" % (bang, label, rel, title)
            full = os.path.join(root, repo, tpath)
            kind = "tree" if os.path.isdir(full) else "blob"
            if not os.path.exists(full):
                stats["relative -> MISSING in repo"] += 1
                detail.append((dest_src, target, "MISSING"))
                return m.group(0)
            new = github_url(repo, tpath, ref, kind, anchor)
            stats["relative -> github (ref)"] += 1
            detail.append((dest_src, target, new))
            return "%s[%s](%s%s)" % (bang, label, new, title)

        out.append(LINK.sub(sub, line))
    return "\n".join(out)


GROUPS = ["@home", "users/", "developers/", "project/", "demo/", "examples/",
          "stages/common/", "stages/discovery/", "stages/inventory/", "stages/provisioning/"]
DEV_ORDER = ["developers/index.md", "developers/build.md", "developers/environment-variables.md",
             "developers/change-a-contract.md", "developers/checks-and-gates.md",
             "developers/compatibility.md", "developers/licensing.md"]
USER_ORDER = ["users/first-migration.md", "users/faq.md", "users/reporting-guide.md"]


def order_key(dest):
    if dest == HOME: return (0, 0, dest)
    for gi, g in enumerate(GROUPS):
        if dest == g or dest.startswith(g):
            if g == "developers/" and dest in DEV_ORDER: return (gi, DEV_ORDER.index(dest), dest)
            if g == "users/" and dest in USER_ORDER: return (gi, USER_ORDER.index(dest), dest)
            return (gi, 0 if dest.endswith("index.md") and dest.count("/") == g.count("/") else 1, dest)
    return (len(GROUPS), 1, dest)


OUT = ""


def main():
    global OUT, PAGES, MAP, TITLE, HOME
    root, OUT = sys.argv[1], sys.argv[2]
    ref = sys.argv[sys.argv.index("--ref") + 1] if "--ref" in sys.argv else "main"
    hugo = "--hugo" in sys.argv
    if "--home" in sys.argv: HOME = sys.argv[sys.argv.index("--home") + 1]
    PAGES = sorted(discover(root, "--with-release-notes" in sys.argv), key=lambda p: order_key(p[2]))
    listing = "".join("%s\t%s\t%s\n" % (r, p, d) for r, p, d, _ in PAGES)
    if "--write-pages" in sys.argv: open(sys.argv[sys.argv.index("--write-pages") + 1], "w").write(listing)
    if "--pages" in sys.argv:
        want = open(sys.argv[sys.argv.index("--pages") + 1]).read()
        if want != listing:
            import difflib
            sys.exit("page list differs from the pinned list:\n" + "".join(difflib.unified_diff(want.splitlines(1), listing.splitlines(1), "pinned", "found")))
    MAP = {(r, p): d for r, p, d, _ in PAGES}
    TITLE = {d: t for _, _, d, t in PAGES}
    sub = "content" if hugo else "docs"
    stats, detail = collections.Counter(), []
    for n, (repo, src, dest, title) in enumerate(PAGES):
        text = open(os.path.join(root, repo, src), encoding="utf-8").read()
        text = rewrite(text, repo, src, ref, root, stats, detail)
        d = dest
        if hugo:
            d = dest[:-len("index.md")] + "_index.md" if dest.endswith("index.md") else dest
            text = '---\ntitle: "%s"\nweight: %d\n---\n\n' % (title.replace('"', "'"), n + 1) + text
        path = os.path.join(OUT, sub, d)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        open(path, "w", encoding="utf-8").write(text)
    if hugo and os.path.isdir(os.path.join(OUT, "assets")):   # images live under static/ in Hugo
        shutil.copytree(os.path.join(OUT, "assets"), os.path.join(OUT, "static", "assets"), dirs_exist_ok=True)
    json.dump({"ref": ref, "pages": len(PAGES), "stats": dict(stats), "detail": detail,
               "nav": [[d, t] for _, _, d, t in PAGES]}, open(os.path.join(OUT, "link-report.json"), "w"), indent=1)
    write_nav(OUT)
    print("pages:", len(PAGES))
    for k, v in sorted(stats.items()):
        print("  %-40s %d" % (k, v))


def write_nav(out):
    """mkdocs nav (YAML text) from the page list: groups by top-level folder, stages by name."""
    def item(d): return "- %s: %s" % (yq(TITLE[d]), d)
    def yq(t): return '"%s"' % t.replace('"', "'")
    lines = ["nav:"]
    lines.append("  - Overview: " + HOME)   # the site root (Home) is the hand-written landing page, not this page
    for label, prefix in [("For users", "users/"), ("For developers", "developers/"), ("Project", "project/"),
                          ("Demo", "demo/"), ("Examples", "examples/")]:
        ds = [d for _, _, d, _ in PAGES if d.startswith(prefix)]
        if not ds: continue
        lines.append("  - %s:" % label)
        for d in ds: lines.append("      " + item(d))
    lines.append("  - Stages:")
    for st in ["common", "discovery", "inventory", "provisioning"]:
        ds = [d for _, _, d, _ in PAGES if d.startswith("stages/%s/" % st)]
        lines.append("      - %s:" % st.capitalize())
        for d in ds: lines.append("          " + item(d))
    open(os.path.join(out, "nav.yml"), "w", encoding="utf-8").write("\n".join(lines) + "\n")


main()
