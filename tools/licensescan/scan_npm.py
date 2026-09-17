"""Read the declared license of every package under a node_modules tree.

npm lockfiles do not record licenses, so the installed tree is the only offline
source. Nested node_modules are walked too, because a hoisting conflict can put
a second copy of a package at a different version and license.
"""
import io
import json
import os
import sys
from collections import Counter

ALLOWED = {"MIT", "ISC", "BSD-3-Clause", "BSD-2-Clause", "Apache-2.0",
           "0BSD", "Unlicense", "BlueOak-1.0.0", "Python-2.0"}


def declared(meta):
    lic = meta.get("license")
    if isinstance(lic, dict):
        lic = lic.get("type")
    if not lic and meta.get("licenses"):
        licenses = meta["licenses"]
        if isinstance(licenses, list):
            lic = "/".join(x.get("type", "?") for x in licenses)
        else:
            lic = str(licenses)
    return lic or "NOT-DECLARED"


def walk(base, out):
    if not os.path.isdir(base):
        return
    for name in sorted(os.listdir(base)):
        if name == ".bin":
            continue
        d = os.path.join(base, name)
        if name.startswith("@") and os.path.isdir(d):
            walk(d, out)
            continue
        pj = os.path.join(d, "package.json")
        if not os.path.isfile(pj):
            continue
        try:
            meta = json.load(io.open(pj, encoding="utf-8"))
        except Exception:
            continue
        out[(meta.get("name", name), meta.get("version", ""))] = declared(meta)
        walk(os.path.join(d, "node_modules"), out)


def main():
    root = sys.argv[1]
    out_path = sys.argv[2]
    found = {}
    walk(root, found)

    rows = [{"name": n, "version": v, "license": lic}
            for (n, v), lic in sorted(found.items())]
    json.dump(rows, io.open(out_path, "w", encoding="utf-8"), indent=1)

    for lic, n in Counter(r["license"] for r in rows).most_common():
        flag = "" if lic in ALLOWED else "   <-- REVIEW"
        print(f"{n:4d}  {lic}{flag}")
    print()
    for r in rows:
        if r["license"] not in ALLOWED:
            print(f'  {r["name"]} {r["version"]} -> {r["license"]}')


main()
