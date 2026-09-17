"""Collect the union of every module required by the repo's go.mod files.

Output is the input format scan_go.py expects: {module: {version, indirect}}.
Kept separate from the scanner so the module list can be produced on a machine
without a populated module cache (CI resolves the cache in a later step).
"""
import io
import json
import os
import re
import sys

REQUIRE = re.compile(
    r'^\s*([a-z0-9.-]+\.[a-z]{2,}/[^\s]+)\s+(v[^\s]+?)(\s*//\s*indirect)?\s*$',
    re.I,
)

GOMODS = [
    "go.mod",
    "cli/go.mod",
    "client/go.mod",
    "docs/poc/docker-sandbox/go.mod",
]


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    out = sys.argv[2] if len(sys.argv) > 2 else "gomods.json"

    mods = {}
    for rel in GOMODS:
        p = os.path.join(root, rel)
        if not os.path.isfile(p):
            print(f"warning: {rel} not found, skipping", file=sys.stderr)
            continue
        text = io.open(p, encoding="utf-8").read()
        # Skip the "module" and "replace" lines; only require blocks matter.
        for line in text.splitlines():
            if line.startswith(("module ", "replace ", "go ", "toolchain ")):
                continue
            m = REQUIRE.match(line)
            if m:
                mods.setdefault(m.group(1), {
                    "version": m.group(2),
                    "indirect": bool(m.group(3)),
                })

    json.dump(dict(sorted(mods.items())), io.open(out, "w", encoding="utf-8"), indent=1)
    print(f"{len(mods)} unique Go modules -> {out}")


main()
