"""Collect every module required by the repository's go.mod files.

Output is the input scan_go.py expects:

    {"meta": {"gomods": [...]},
     "rows": [{module, version, indirect, gomods, replace}, ...]}

Kept separate from the scanner so the module list can be produced on a machine
without a populated module cache (CI resolves the cache in a later step).

The go.mod files are discovered with `git ls-files` rather than listed here: a
hand-kept list is exactly what drifted before (it still named a proof-of-concept
module that had been deleted, and a new module would have been silently left
out). Each file is parsed by `go mod edit -json`, the toolchain's own parser,
so block syntax, comments and `exclude` directives cannot fool a regex.

A module is keyed by (path, version, replacement). Two go.mod files requiring
different versions of the same module each produce a row, because each version
is linked into a different binary. `replace` directives are honoured, since the
replacement is what actually gets built: a version replacement is scanned at the
replacement version, and a local-path replacement (a vendored copy, or a sibling
module of this repository) is scanned from the working tree.
"""
import json
import os
import posixpath
import subprocess
import sys


def discover(root):
    out = subprocess.run(
        ["git", "-C", root, "ls-files", "--", ":(glob)**/go.mod"],
        check=True, capture_output=True, text=True,
    ).stdout
    return sorted(set(out.split()))


def parse(root, rel):
    out = subprocess.run(
        ["go", "mod", "edit", "-json", os.path.join(root, rel)],
        check=True, capture_output=True, text=True,
    ).stdout
    return json.loads(out)


def replacement_for(replaces, path, version, moddir):
    """Return the replacement that applies to path@version, as scan_go.py wants it.

    A replace whose left-hand side names a version applies to that version only,
    and wins over a version-less replace of the same path, as in cmd/go.
    """
    best = None
    for r in replaces:
        old = r["Old"]
        if old["Path"] != path:
            continue
        if old.get("Version") and old["Version"] != version:
            continue
        if best is None or old.get("Version"):
            best = r["New"]
    if best is None:
        return None
    if best.get("Version"):
        return {"module": best["Path"], "version": best["Version"]}
    # A local directory, relative to the go.mod that names it. Stored relative
    # to the repository root with forward slashes so the output is identical on
    # every machine.
    return {"path": posixpath.normpath(posixpath.join(moddir, best["Path"]))}


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    out = sys.argv[2] if len(sys.argv) > 2 else "gomods.json"

    rows = {}
    gomods = discover(root)
    for rel in gomods:
        mod = parse(root, rel)
        moddir = posixpath.dirname(rel)
        replaces = mod.get("Replace") or []
        for req in mod.get("Require") or []:
            path, version = req["Path"], req["Version"]
            repl = replacement_for(replaces, path, version, moddir)
            key = (path, version, json.dumps(repl, sort_keys=True))
            row = rows.setdefault(key, {
                "module": path,
                "version": version,
                # Indirect only if no go.mod requires it directly.
                "indirect": True,
                "gomods": [],
                "replace": repl,
            })
            row["indirect"] = row["indirect"] and bool(req.get("Indirect"))
            row["gomods"].append(rel)

    result = {
        "meta": {"gomods": gomods},
        "rows": [rows[k] for k in sorted(rows)],
    }
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=1)
    print(f"{len(rows)} Go module versions from {len(gomods)} go.mod files -> {out}")


main()
