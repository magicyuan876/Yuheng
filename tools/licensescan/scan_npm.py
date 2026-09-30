"""Read the declared license of every package an npm project's lockfile resolves.

    scan_npm.py <project dir> <out.json>

The package list comes from `package-lock.json`, not from walking node_modules.
The installed tree is platform-specific -- npm installs only the esbuild,
rollup or lightningcss binary that matches the machine -- and it can hold
leftovers from an earlier install, so a list built from it differs between a
developer's Mac and the Linux CI runner. The lockfile lists every package for
every platform and is the same everywhere.

Licenses are read from the installed package's own package.json, which is what
the package actually declares (the lockfile's `license` field is npm's copy,
and is missing for a few dozen packages). Platform-specific packages are the
exception: which of them are installed depends on the machine, so their license
is taken from the lockfile only, and reported as NOT-RECORDED when the lockfile
has none. Everything else must be installed at the locked version; if it is
not, node_modules is stale and the scan stops rather than guess.

The JSON output is what render_notices.py turns into THIRD_PARTY_NOTICES.md;
the text on stdout is what tools/license_check.sh gates on.
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


def main():
    project, out_path = sys.argv[1], sys.argv[2]
    with io.open(os.path.join(project, "package-lock.json"), encoding="utf-8") as fh:
        lock = json.load(fh)

    found = {}
    stale = []
    for path, entry in lock.get("packages", {}).items():
        # "" is the project itself; paths without node_modules/ are workspace
        # sources, and links point at one of those. None is a dependency.
        # "extraneous" entries are orphans npm sometimes leaves in the lockfile
        # after an uninstall; nothing depends on them and `npm ci` does not
        # install them, so they are not shipped either.
        if "node_modules/" not in path or entry.get("link") or entry.get("extraneous"):
            continue
        name = entry.get("name") or path.rsplit("node_modules/", 1)[1]
        version = entry.get("version", "")
        if entry.get("os") or entry.get("cpu"):
            lic = entry.get("license") or "NOT-RECORDED"
        else:
            pj = os.path.join(project, *path.split("/"), "package.json")
            try:
                with io.open(pj, encoding="utf-8") as fh:
                    meta = json.load(fh)
            except (OSError, ValueError):
                stale.append(f"{path} is not installed")
                continue
            if meta.get("version") != version:
                stale.append(f"{path} is {meta.get('version')}, the lockfile says {version}")
                continue
            lic = declared(meta)
        found.setdefault((name, version), set()).add(lic)

    if stale:
        print(f"{project}/node_modules does not match package-lock.json "
              f"(run 'npm ci' in {project}/):", file=sys.stderr)
        for s in stale[:20]:
            print(f"  {s}", file=sys.stderr)
        sys.exit(2)

    # The same name@version installed at two paths is one package; its license
    # cannot differ, but if it ever did both spellings are kept.
    rows = [{"name": n, "version": v, "license": " / ".join(sorted(lics))}
            for (n, v), lics in sorted(found.items())]
    result = {"meta": {"lockfile": f"{project}/package-lock.json"}, "rows": rows}
    with io.open(out_path, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=1)

    counts = Counter(r["license"] for r in rows)
    for lic in sorted(counts, key=lambda k: (-counts[k], k)):
        flag = "" if lic in ALLOWED else "   <-- REVIEW"
        print(f"{counts[lic]:4d}  {lic}{flag}")
    print()
    for r in rows:
        if r["license"] not in ALLOWED:
            print(f'  {r["name"]} {r["version"]} -> {r["license"]}')


main()
