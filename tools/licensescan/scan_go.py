"""Classify the license of every Go module the repository's go.mod files require.

    scan_go.py --modcache DIR --repo DIR gomods.json go.json

Reads the module list collect_gomods.py wrote, locates each module in the
module cache (case-encoded the way the Go toolchain writes it), and matches the
license text against a small set of SPDX signatures. A module replaced by a
local directory is read from the working tree instead, and one replaced by
another version is read at the replacement, because that is what gets built.
Anything that does not match -- or that matches a copyleft/source-available
family -- is reported so a human decides, rather than being silently binned as
"probably fine".

The JSON output is what render_notices.py turns into THIRD_PARTY_NOTICES.md;
the text on stdout is what tools/license_check.sh gates on.
"""
import argparse
import json
import os
import re

# Order matters. MPL-2.0 must be tested before the GNU family: its own text
# names GPL/LGPL/AGPL as "Secondary Licenses", so a naive AGPL signature matches
# every MPL-2.0 file (this mis-filed go-sql-driver/mysql on the first pass).
SIGNATURES = [
    ("MPL-2.0",        r"Mozilla Public License,?\s+[Vv]ersion 2\.0"),
    ("GPL-3.0",        r"GNU GENERAL PUBLIC LICENSE\s+Version 3"),
    ("GPL-2.0",        r"GNU GENERAL PUBLIC LICENSE\s+Version 2"),
    ("AGPL-3.0",       r"GNU AFFERO GENERAL PUBLIC LICENSE"),
    ("LGPL",           r"GNU LESSER GENERAL PUBLIC LICENSE"),
    ("SSPL",           r"Server Side Public License"),
    ("Elastic-2.0",    r"Elastic License"),
    ("BUSL-1.1",       r"Business Source License"),
    ("Apache-2.0",     r"Apache License\s+Version 2\.0"),
    ("ISC",            r"ISC License|Permission to use, copy, modify, and(/or)? distribute"),
    ("BSD-3-Clause",   r"Neither the name of .{0,120}(nor|or) the names of its\s+contributors"),
    ("BSD-2-Clause",   r"Redistribution and use in source and binary forms"),
    ("MIT",            r"Permission is hereby granted, free of charge"),
    ("Unlicense",      r"This is free and unencumbered software released into the public domain"),
    ("CC0-1.0",        r"CC0 1\.0 Universal"),
]

ALLOWED = {"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC",
           "MPL-2.0", "Unlicense", "CC0-1.0", "Python-2.0", "MIT-CMU"}


def escape(path: str) -> str:
    """Go encodes uppercase letters in cache paths as '!' + lowercase."""
    return re.sub(r"[A-Z]", lambda m: "!" + m.group(0).lower(), path)


LICENSE_NAME = re.compile(r"(LICEN[CS]E|COPYING|COPYRIGHT|NOTICE)([.-].*)?$", re.I)


def read_license_dir(d: str):
    if not os.path.isdir(d):
        return None, None
    for name in sorted(os.listdir(d)):
        if LICENSE_NAME.fullmatch(name):
            f = os.path.join(d, name)
            if os.path.isfile(f):
                with open(f, "rb") as fh:
                    return name, fh.read(20000).decode("utf-8", "replace")
    return None, None


def find_in_cache(modcache: str, mod: str, ver: str):
    """Look in the module dir, then walk up the path.

    Submodules (".../v18", ".../lib/linux-arm64", ".../loader") usually carry no
    license file of their own -- the text sits in the repository root, which the
    cache stores as a separate directory. Walking up finds it instead of
    reporting a false NOT-FOUND.
    """
    d = os.path.join(modcache, escape(mod) + "@" + escape(ver))
    present = os.path.isdir(d)
    name, text = read_license_dir(d)
    if text:
        return name, text, present

    parts = mod.split("/")
    while len(parts) > 2:
        parts = parts[:-1]
        parent = "/".join(parts)
        base = os.path.join(modcache, escape(parent))
        root = os.path.dirname(base)
        prefix = os.path.basename(base) + "@"
        if os.path.isdir(root):
            for entry in sorted(os.listdir(root)):
                if entry.startswith(prefix):
                    name, text = read_license_dir(os.path.join(root, entry))
                    if text:
                        return parent + "/" + name, text, present
    return None, None, present


def find_in_tree(repo: str, rel: str):
    """Find the license of a local-path replacement.

    A vendored copy carries its own LICENSE and is classified like any cached
    module. A sibling module of this repository usually carries none: it is
    Yuheng's own code, not a third-party component, so it takes the project's
    license as NOTICE declares it (`SPDX-License-Identifier:`). The root LICENSE
    is deliberately not classified -- it bundles upstream's third-party notices,
    and the first signature that matches in it is Apache-2.0, not the MIT grant
    it opens with. The walk never leaves the checkout.

    Returns (license file, license text or None, SPDX id or None, present).
    """
    parts = [p for p in rel.split("/") if p not in ("", ".")]
    present = os.path.isdir(os.path.join(repo, *parts))
    while parts:
        name, text = read_license_dir(os.path.join(repo, *parts))
        if text:
            return "/".join(parts + [name]), text, None, present
        parts = parts[:-1]
    with open(os.path.join(repo, "NOTICE"), encoding="utf-8") as fh:
        m = re.search(r"SPDX-License-Identifier:\s*(\S+)", fh.read())
    return "NOTICE", None, (m.group(1) if m else None), present


def classify(text: str):
    flat = re.sub(r"\s+", " ", text)
    for spdx, pattern in SIGNATURES:
        if re.search(pattern, flat, re.I):
            return spdx
    return None


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--modcache", required=True, help="`go env GOMODCACHE`")
    ap.add_argument("--repo", required=True, help="repository root, for local-path replacements")
    ap.add_argument("gomods", help="collect_gomods.py output")
    ap.add_argument("out", help="JSON to write")
    args = ap.parse_args()

    with open(args.gomods, encoding="utf-8") as fh:
        collected = json.load(fh)

    rows = []
    for mod in collected["rows"]:
        repl = mod["replace"] or {}
        own = None
        if "path" in repl:
            fname, text, own, present = find_in_tree(args.repo, repl["path"])
        else:
            fname, text, present = find_in_cache(
                args.modcache, repl.get("module", mod["module"]), repl.get("version", mod["version"]))
        spdx = own or (classify(text) if text else None)
        if not spdx:
            spdx = "UNRECOGNISED" if text else ("NO-LICENSE-FILE" if present else "NOT-IN-CACHE")
        rows.append({**mod, "license_file": fname, "spdx": spdx, "this_repository": own is not None})
    rows.sort(key=lambda r: (r["module"], r["version"]))
    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump({"meta": collected["meta"], "rows": rows}, fh, indent=1)

    counts = {}
    for r in rows:
        counts[r["spdx"]] = counts.get(r["spdx"], 0) + 1
    for k in sorted(counts, key=lambda k: (-counts[k], k)):
        flag = "" if k in ALLOWED else "   <-- REVIEW"
        print(f"{counts[k]:4d}  {k}{flag}")
    print()
    for r in rows:
        if r["spdx"] not in ALLOWED:
            print(f"  {r['module']} {r['version']} -> {r['spdx']}"
                  f" (file: {r['license_file']})")


main()
