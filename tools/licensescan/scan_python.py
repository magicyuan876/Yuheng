"""Read license metadata out of every installed distribution in this interpreter.

Uses importlib.metadata so it works against whatever site-packages the image
actually ships, rather than re-resolving the lock file offline.
"""
import json
import sys
from importlib import metadata

rows = []
for dist in metadata.distributions():
    md = dist.metadata
    name = md.get("Name")
    if not name:
        continue
    lic = (md.get("License-Expression") or "").strip()
    if not lic:
        classifiers = [c for c in md.get_all("Classifier") or []
                       if c.startswith("License ::")]
        lic = "; ".join(c.split(" :: ")[-1] for c in classifiers)
    if not lic:
        raw = (md.get("License") or "").strip()
        # Some projects paste the whole license text into this field.
        lic = raw.splitlines()[0][:80] if raw else ""
    rows.append({
        "name": name,
        "version": md.get("Version", ""),
        "license": lic or "NOT-DECLARED",
    })

rows.sort(key=lambda r: r["name"].lower())
json.dump(rows, open(sys.argv[1], "w"), indent=1)

from collections import Counter
for lic, n in Counter(r["license"] for r in rows).most_common():
    print(f"{n:4d}  {lic}")
