"""Classify the license of every Go module in the build's module cache.

Reads the module list from the repo's go.mod files, locates each module in
/go/pkg/mod (case-encoded the way the Go toolchain writes it), and matches the
license text against a small set of SPDX signatures. Anything that does not
match -- or that matches a copyleft/source-available family -- is reported so a
human decides, rather than being silently binned as "probably fine".
"""
import json
import os
import re
import sys

MOD_ROOT = os.environ.get("MOD_ROOT", "/go/pkg/mod")

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


def find_license(mod: str, ver: str):
    """Look in the module dir, then walk up the path.

    Submodules (".../v18", ".../lib/linux-arm64", ".../loader") usually carry no
    license file of their own -- the text sits in the repository root, which the
    cache stores as a separate directory. Walking up finds it instead of
    reporting a false NOT-FOUND.
    """
    d = os.path.join(MOD_ROOT, escape(mod) + "@" + escape(ver))
    present = os.path.isdir(d)
    name, text = read_license_dir(d)
    if text:
        return name, text, present

    parts = mod.split("/")
    while len(parts) > 2:
        parts = parts[:-1]
        parent = "/".join(parts)
        base = os.path.join(MOD_ROOT, escape(parent))
        root = os.path.dirname(base)
        prefix = os.path.basename(base) + "@"
        if os.path.isdir(root):
            for entry in sorted(os.listdir(root)):
                if entry.startswith(prefix):
                    name, text = read_license_dir(os.path.join(root, entry))
                    if text:
                        return parent + "/" + name, text, present
    return None, None, present


def classify(text: str):
    flat = re.sub(r"\s+", " ", text)
    for spdx, pattern in SIGNATURES:
        if re.search(pattern.replace(r"\s+", r"\s+"), flat, re.I):
            return spdx
    return None


def main():
    mods = json.load(open(sys.argv[1]))
    rows = []
    for mod, meta in mods.items():
        fname, text, present = find_license(mod, meta["version"])
        spdx = classify(text) if text else None
        if not spdx:
            spdx = "UNRECOGNISED" if text else ("NO-LICENSE-FILE" if present else "NOT-IN-CACHE")
        rows.append({
            "module": mod,
            "version": meta["version"],
            "indirect": meta["indirect"],
            "license_file": fname,
            "spdx": spdx,
        })
    rows.sort(key=lambda r: r["module"])
    json.dump(rows, open(sys.argv[2], "w"), indent=1)

    counts = {}
    for r in rows:
        counts[r["spdx"]] = counts.get(r["spdx"], 0) + 1
    for k in sorted(counts, key=lambda k: -counts[k]):
        flag = "" if k in ALLOWED else "   <-- REVIEW"
        print(f"{counts[k]:4d}  {k}{flag}")
    print()
    for r in rows:
        if r["spdx"] not in ALLOWED:
            print(f"  {r['module']} {r['version']} -> {r['spdx']}"
                  f" (file: {r['license_file']})")


main()
