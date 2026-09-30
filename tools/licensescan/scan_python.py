"""Read the license metadata of every distribution a uv project's image ships.

    scan_python.py --project docreader --dockerfile docker/Dockerfile.docreader out.json

The distribution set is the project's `uv.lock` (runtime dependencies only: no
dev group, not the project itself), resolved for the Linux images the project
is published as -- linux/amd64 and linux/arm64, the platforms
.github/workflows/docker-image.yml builds -- and for the Python the image's
Dockerfile starts `FROM`. It is deliberately not "whatever is installed in the
local venv": uv installs a different onnxruntime (and so a different set of
its dependencies) on macOS than on x86-64 Linux, and a numpy per Python minor
version, so a venv-based list changes with the machine that produced it.

To read real wheel metadata for those targets, the locked set is exported and
installed into throwaway target directories with `uv pip install --target
--python-platform`, which works from any host and reuses uv's cache. Licenses
come from `importlib.metadata`, preferring `License-Expression`, then the
`License ::` classifiers, then the free-text `License` field.

The JSON output is what render_notices.py turns into THIRD_PARTY_NOTICES.md;
the text on stdout is what tools/license_check.sh gates on. A distribution
shipped on only one of the platforms records which.
"""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from collections import Counter
from importlib import metadata

# (uv target triple, Docker platform). Keep in step with the `platforms:` of
# the image builds in .github/workflows/docker-image.yml.
TARGETS = [
    ("x86_64-unknown-linux-gnu", "linux/amd64"),
    ("aarch64-unknown-linux-gnu", "linux/arm64"),
]

# License names that need no review. Python metadata mixes SPDX identifiers with
# trove-classifier names, so both spellings are listed. MPL is here for the same
# reason scan_go.py allows it: file-level copyleft on unmodified files is
# compatible with shipping Yuheng under MIT (NOTICE_AUDIT.md §4).
ALLOWED = {
    "MIT", "MIT License", "MIT-0", "MIT-CMU",
    "BSD License", "BSD-2-Clause", "BSD-3-Clause", "3-Clause BSD License",
    "Apache-2.0", "Apache Software License",
    "ISC", "ISC License (ISCL)",
    "PSF-2.0", "Python-2.0", "Python Software Foundation License", "CNRI-Python",
    "MPL-2.0", "Mozilla Public License 2.0 (MPL 2.0)",
    "Unlicense", "The Unlicense (Unlicense)", "CC0-1.0",
}


def image_python(dockerfile):
    """The X.Y of the first `FROM python:X.Y...` in the Dockerfile."""
    with open(dockerfile, encoding="utf-8") as fh:
        m = re.search(r"^FROM\s+(?:\S+/)?python:(\d+\.\d+)", fh.read(), re.M | re.I)
    if not m:
        sys.exit(f"{dockerfile}: no `FROM python:X.Y` line to take the image's Python version from")
    return m.group(1)


def license_of(md):
    lic = (md.get("License-Expression") or "").strip()
    if not lic:
        classifiers = [c for c in md.get_all("Classifier") or []
                       if c.startswith("License ::")]
        lic = "; ".join(c.split(" :: ")[-1] for c in classifiers)
    if not lic:
        raw = (md.get("License") or "").strip()
        # Some projects paste the whole license text into this field.
        lic = raw.splitlines()[0][:80] if raw else ""
    return lic or "NOT-DECLARED"


def allowed(lic):
    """True when every name in the expression is on the allowlist.

    A disjunction with one copyleft option is left to license_check.sh, which
    knows about elections; this only decides what is worth printing.
    """
    if lic in ALLOWED:
        return True  # a classifier name may itself contain parentheses
    names = re.split(r"\s+(?:AND|OR|WITH)\s+|;\s*|,\s*", lic)
    return all(n.strip("() ") in ALLOWED or n.strip() in ALLOWED for n in names)


def run(cmd):
    proc = subprocess.run(cmd, capture_output=True, text=True)
    if proc.returncode:
        sys.stderr.write(proc.stderr)
        sys.exit(f"command failed: {' '.join(cmd)}")
    return proc.stdout


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--project", required=True, help="directory holding pyproject.toml and uv.lock")
    ap.add_argument("--dockerfile", required=True, help="the image's Dockerfile, for its Python version")
    ap.add_argument("out", help="JSON to write")
    args = ap.parse_args()

    python = image_python(args.dockerfile)
    found = {}
    with tempfile.TemporaryDirectory(prefix="licensescan-") as tmp:
        req = os.path.join(tmp, "requirements.txt")
        run(["uv", "export", "--project", args.project, "--locked", "--no-dev",
             "--no-emit-project", "--no-hashes", "--format", "requirements-txt",
             "--quiet", "--output-file", req])
        for triple, platform in TARGETS:
            site = os.path.join(tmp, platform.replace("/", "-"))
            # --no-deps: the export is already the complete, locked set, and
            # its environment markers are evaluated for the target platform.
            run(["uv", "pip", "install", "--quiet", "--no-deps", "--target", site,
                 "--python-platform", triple, "--python-version", python, "-r", req])
            for dist in metadata.distributions(path=[site]):
                md = dist.metadata
                name = md.get("Name")
                if not name:
                    continue
                key = (name, md.get("Version", ""))
                entry = found.setdefault(key, {"licenses": set(), "platforms": set()})
                entry["licenses"].add(license_of(md))
                entry["platforms"].add(platform)

    everywhere = {p for _, p in TARGETS}
    rows = []
    for (name, version), entry in sorted(found.items(), key=lambda kv: (kv[0][0].lower(), kv[0][1])):
        rows.append({
            "name": name,
            "version": version,
            "license": " / ".join(sorted(entry["licenses"])),
            # Empty when the distribution ships on every platform.
            "platforms": [] if entry["platforms"] == everywhere else sorted(entry["platforms"]),
        })
    result = {
        "meta": {
            "lockfile": f"{args.project}/uv.lock",
            "dockerfile": args.dockerfile,
            "python": python,
            "platforms": [p for _, p in TARGETS],
        },
        "rows": rows,
    }
    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=1)

    counts = Counter(r["license"] for r in rows)
    for lic in sorted(counts, key=lambda k: (-counts[k], k)):
        flag = "" if allowed(lic) else "   <-- REVIEW"
        print(f"{counts[lic]:4d}  {lic}{flag}")
    print()
    for r in rows:
        if not allowed(r["license"]):
            print(f'  {r["name"]} {r["version"]} -> {r["license"]}')


main()
