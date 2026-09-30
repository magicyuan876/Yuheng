"""Resolve the declared license of every crate a Cargo lockfile pins.

    scan_cargo.py --lockfile third_party/anydoc-go/Cargo.lock --cache-dir DIR out.json

The crate set is `Cargo.lock`, the lockfile scripts/build-anydoc-lib.sh builds
with `--locked`, so it is exactly what goes into libanydoc_go.a. Licenses are
the `license` expression each crate version was published with, read from the
crates.io API (https://crates.io/api/v1/crates/<name>/<version>). That is the
same answer on every machine and needs no Rust toolchain: `cargo metadata`
would give it too, but neither this machine nor the CI runners have cargo, and
installing one to read a field crates.io already serves is not worth a minute
of every run.

Which packages are listed:

* Every registry package in the lockfile, the same way scan_npm.py lists every
  package in package-lock.json, build tooling and other platforms' binaries
  included. Each row also says what the package is for the Linux images the
  app is published as (linux/amd64 and linux/arm64, both glibc -- the
  `lib_linux_gnu.go` link line): linked into the archive, compiled only to run
  at build time (cbindgen, build scripts and their dependencies), or not built
  for Linux at all (the windows-* family, wasm and Android shims). Cargo.lock
  records dependency edges but not their kind or target, so those come from
  each crate version's entry in the crates.io index (https://index.crates.io),
  evaluated against the targets' `cfg` values. Listing the non-shipped crates
  is deliberate: the list stays one complete, reviewable picture of the lock,
  and a copyleft crate arriving anywhere in it still fails the gate, as a
  copyleft build tool does in the npm trees.
* Proc-macro crates (serde_derive and the like) run inside the compiler and are
  not linked either, but registry metadata does not say which crates are proc
  macros, so they are classified by their edges like any other crate. The
  classification errs towards "linked", never away from it.
* The root package -- anydoc-go itself, the vendored crate this lockfile
  belongs to -- is not listed: it is this repository's vendored source, listed
  under Bundled assets and, as a Go module, in the Go section.
* A local package that `[patch.crates-io]` puts in place of a published crate
  (`anydoc`, which the build script copies from the published crate and makes
  one function public) is listed at its locked version with the license of the
  crate it patches, and marked as patched.
* A package from a git repository or another registry has no crates.io record
  to read, so it is reported as NOT-RECORDED for a human to classify (an entry
  in render_notices.py's annotation map records the answer). Any other local
  package stops the scan: its license lives in a Cargo.toml this scanner does
  not know how to find, and guessing would be worse than stopping.

Every registry answer is checked against the lockfile's checksum, so the
license shown is the one of the exact crate file Cargo downloads. Answers are
cached per name@version under --cache-dir: a published crate version never
changes, so a cached entry never goes stale, and a rerun makes no requests.
Uncached requests to the API are spaced a second apart, as the crates.io crawler
policy asks, and identify this tool in their User-Agent.

The JSON output is what render_notices.py turns into THIRD_PARTY_NOTICES.md;
the text on stdout is what tools/license_check.sh gates on. Exit status 3 means
the registry could not be reached and the cache does not cover the lockfile,
which license_check.sh treats as "cannot run here" rather than as a failure.
"""
import argparse
import json
import os
import re
import sys
import time
import tomllib
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter

API = "https://crates.io/api/v1/crates"
INDEX = "https://index.crates.io"
CRATES_IO_SOURCES = {
    "registry+https://github.com/rust-lang/crates.io-index",
    "sparse+https://index.crates.io/",
}
USER_AGENT = ("yuheng-licensescan/1 (tools/licensescan/scan_cargo.py; "
              "https://github.com/magicyuan876/yuheng)")
API_INTERVAL = 1.0  # seconds between uncached API requests (crates.io crawler policy)
UNREACHABLE = 3     # exit status when the registry cannot be reached

# The Rust targets the app image links the archive for, and the `cfg` values
# rustc sets for each (`rustc --print cfg --target <triple>`, release profile).
# Keep the triples in step with the `platforms:` of the image builds in
# .github/workflows/docker-image.yml and the link lines in
# third_party/anydoc-go/lib_linux_gnu.go.
COMMON_LINUX = {
    "unix": {""},
    "target_os": {"linux"},
    "target_family": {"unix"},
    "target_env": {"gnu"},
    "target_vendor": {"unknown"},
    "target_abi": {""},
    "target_pointer_width": {"64"},
    "target_endian": {"little"},
    "target_has_atomic": {"8", "16", "32", "64", "ptr"},
    "panic": {"unwind"},
}
TARGETS = [
    ("x86_64-unknown-linux-gnu", "linux/amd64",
     {**COMMON_LINUX, "target_arch": {"x86_64"}, "target_feature": {"fxsr", "sse", "sse2"}}),
    ("aarch64-unknown-linux-gnu", "linux/arm64",
     {**COMMON_LINUX, "target_arch": {"aarch64"}, "target_feature": {"neon"}}),
]

# License identifiers that need no review. MPL-2.0 is here for the reason
# scan_go.py allows it: file-level copyleft on unmodified files is compatible
# with shipping Yuheng under MIT (NOTICE_AUDIT.md §4). Unicode-3.0 and BSL-1.0
# (Boost) are permissive licenses whose one condition is that the notice travels
# with the code, which THIRD_PARTY_NOTICES.md is for. LLVM-exception only ever
# widens Apache-2.0.
ALLOWED = {
    "MIT", "MIT-0", "Apache-2.0", "LLVM-exception", "BSD-2-Clause", "BSD-3-Clause",
    "0BSD", "ISC", "Zlib", "Unlicense", "CC0-1.0", "BSL-1.0", "Unicode-3.0",
    "Unicode-DFS-2016", "MPL-2.0",
}


class ScanError(Exception):
    pass


class Unreachable(Exception):
    pass


# ---------------------------------------------------------------------------
# Registry access, cached per name@version.
# ---------------------------------------------------------------------------
class Registry:
    def __init__(self, cache_dir):
        self.cache_dir = cache_dir
        self.last_api = 0.0
        self.index = {}  # crate name -> {version: index entry}, fetched once per run

    def _get(self, url):
        req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT, "Accept": "application/json"})
        for attempt in range(4):
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:
                    return resp.read().decode("utf-8")
            except urllib.error.HTTPError as e:
                # 404 is an answer, not an outage; 429 and 5xx are worth waiting out.
                if e.code == 404:
                    raise ScanError(f"{url}: not found on crates.io")
                if e.code != 429 and e.code < 500:
                    raise ScanError(f"{url}: HTTP {e.code}")
                last = e
            except (urllib.error.URLError, TimeoutError, ConnectionError) as e:
                last = e
            time.sleep(2 ** attempt)
        raise Unreachable(f"{url}: {last}")

    def _index_entries(self, name):
        if name not in self.index:
            lower = name.lower()
            if len(lower) <= 2:
                path = f"{len(lower)}/{lower}"
            elif len(lower) == 3:
                path = f"3/{lower[0]}/{lower}"
            else:
                path = f"{lower[:2]}/{lower[2:4]}/{lower}"
            entries = {}
            for line in self._get(f"{INDEX}/{path}").splitlines():
                if line.strip():
                    entry = json.loads(line)
                    entries[entry["vers"]] = entry
            self.index[name] = entries
        return self.index[name]

    def _license(self, name, version):
        wait = self.last_api + API_INTERVAL - time.monotonic()
        if wait > 0:
            time.sleep(wait)
        try:
            body = self._get(f"{API}/{urllib.parse.quote(name)}/{urllib.parse.quote(version)}")
        finally:
            self.last_api = time.monotonic()
        return json.loads(body)["version"].get("license")

    def lookup(self, name, version):
        """{"license", "checksum", "deps"} for one published crate version."""
        path = os.path.join(self.cache_dir, name, version + ".json")
        try:
            with open(path, encoding="utf-8") as fh:
                return json.load(fh)
        except (OSError, ValueError):
            pass
        entry = self._index_entries(name).get(version)
        if entry is None:
            raise ScanError(f"{name} {version} is not in the crates.io index")
        record = {
            "license": self._license(name, version),
            "checksum": entry.get("cksum"),
            # Only what the classification needs: the real package name (an
            # entry's `name` is the rename when `package` is set), kind, target.
            "deps": [{"package": d.get("package") or d["name"], "kind": d.get("kind") or "normal",
                      "target": d.get("target")} for d in entry.get("deps", [])],
        }
        os.makedirs(os.path.dirname(path), exist_ok=True)
        tmp = path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as fh:
            json.dump(record, fh, indent=1, sort_keys=True)
        os.replace(tmp, path)
        return record


# ---------------------------------------------------------------------------
# `cfg(...)` evaluation, for target-specific dependencies.
# ---------------------------------------------------------------------------
CFG_TOKEN = re.compile(r'\s*(?:(?P<punct>[(),=])|"(?P<str>[^"]*)"|(?P<ident>[A-Za-z_][A-Za-z0-9_]*))')


def cfg_active(target, triple, cfg):
    """Whether a dependency's `target` field selects this target.

    The field is either a target triple or a `cfg(...)` predicate. A name rustc
    does not set for the target (a custom `--cfg` such as `loom`, `docsrs`, or
    `debug_assertions` in a release build) is false, as it is for Cargo.
    """
    if target is None:
        return True
    target = target.strip()
    if not target.startswith("cfg("):
        return target == triple
    tokens = []
    pos = 0
    while pos < len(target):
        m = CFG_TOKEN.match(target, pos)
        if not m:
            raise ScanError(f"cannot parse the target predicate {target!r}")
        pos = m.end()
        if m.group("punct"):
            tokens.append(("punct", m.group("punct")))
        elif m.group("str") is not None:
            tokens.append(("str", m.group("str")))
        else:
            tokens.append(("ident", m.group("ident")))

    def expect(i, kind, value=None):
        if i >= len(tokens) or tokens[i][0] != kind or (value is not None and tokens[i][1] != value):
            raise ScanError(f"cannot parse the target predicate {target!r}")
        return tokens[i][1], i + 1

    def pred(i):
        ident, i = expect(i, "ident")
        if ident in ("all", "any", "not") and i < len(tokens) and tokens[i] == ("punct", "("):
            i += 1
            values = []
            while tokens[i] != ("punct", ")"):
                value, i = pred(i)
                values.append(value)
                if tokens[i] == ("punct", ","):
                    i += 1
            i += 1
            if ident == "all":
                return all(values), i
            if ident == "any":
                return any(values), i
            if len(values) != 1:
                raise ScanError(f"not() takes one predicate in {target!r}")
            return not values[0], i
        if i < len(tokens) and tokens[i] == ("punct", "="):
            value, i = expect(i + 1, "str")
            return value in cfg.get(ident, set()), i
        return "" in cfg.get(ident, set()), i

    try:
        _, i = expect(0, "ident", "cfg")
        _, i = expect(i, "punct", "(")
        value, i = pred(i)
        _, i = expect(i, "punct", ")")
    except IndexError:  # an unbalanced predicate runs off the end of the tokens
        raise ScanError(f"cannot parse the target predicate {target!r}") from None
    if i != len(tokens):
        raise ScanError(f"cannot parse the target predicate {target!r}")
    return value


# ---------------------------------------------------------------------------
# The lockfile and the root manifest.
# ---------------------------------------------------------------------------
def manifest_deps(manifest):
    """The root crate's own dependencies, in the shape the index uses."""
    deps = []

    def add(table, kind, target):
        for key, spec in (table or {}).items():
            package = spec.get("package", key) if isinstance(spec, dict) else key
            deps.append({"package": package, "kind": kind, "target": target})

    for section, kind in (("dependencies", "normal"), ("build-dependencies", "build"),
                          ("dev-dependencies", "dev")):
        add(manifest.get(section), kind, None)
        for target, tables in (manifest.get("target") or {}).items():
            add(tables.get(section), kind, target)
    return deps


def parse_dep(spec):
    """A lockfile dependency string: "name", "name version" or "name version (source)"."""
    parts = spec.split(" ", 2)
    return parts[0], (parts[1] if len(parts) > 1 else None)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--lockfile", required=True, help="Cargo.lock; its Cargo.toml sits beside it")
    ap.add_argument("--cache-dir", required=True, help="where registry answers are cached")
    ap.add_argument("out", help="JSON to write")
    args = ap.parse_args()

    with open(args.lockfile, "rb") as fh:
        lock = tomllib.load(fh)
    with open(os.path.join(os.path.dirname(args.lockfile), "Cargo.toml"), "rb") as fh:
        manifest = tomllib.load(fh)
    root = manifest["package"]["name"]
    patched = {name for name, spec in (manifest.get("patch", {}).get("crates-io") or {}).items()
               if isinstance(spec, dict) and "path" in spec}

    packages = lock.get("package", [])
    by_name = {}
    for p in packages:
        by_name.setdefault(p["name"], []).append(p)

    def resolve(spec):
        name, version = parse_dep(spec)
        candidates = [p for p in by_name.get(name, []) if version is None or p["version"] == version]
        if len(candidates) != 1:
            raise ScanError(f"lockfile dependency {spec!r} matches {len(candidates)} packages")
        return candidates[0]

    registry = Registry(args.cache_dir)
    # (name, version) -> {"license", "source", "deps"}; deps None when unknown.
    info = {}
    for p in packages:
        key = (p["name"], p["version"])
        source = p.get("source")
        if source is None and p["name"] == root:
            info[key] = {"license": None, "source": "root", "deps": manifest_deps(manifest)}
        elif source is None and p["name"] in patched:
            record = registry.lookup(*key)
            info[key] = {"license": record["license"], "source": "patch", "deps": record["deps"]}
        elif source is None:
            raise ScanError(f"{p['name']} {p['version']} is a local package that is neither the root crate "
                            "nor a [patch.crates-io] of a published one; teach scan_cargo.py where its "
                            "license is declared")
        elif source in CRATES_IO_SOURCES:
            record = registry.lookup(*key)
            if record["checksum"] != p.get("checksum"):
                raise ScanError(f"{p['name']} {p['version']}: Cargo.lock's checksum does not match the "
                                "crates.io index, so the lockfile does not describe the published crate")
            info[key] = {"license": record["license"], "source": "crates.io", "deps": record["deps"]}
        else:
            # A git or alternative-registry source: nothing on crates.io
            # describes this build, so its license is left to a human, and its
            # edges are all followed as if they were normal and unconditional.
            info[key] = {"license": None, "source": source.split("+", 1)[0], "deps": None}

    def edge_kinds(parent, child, triple, cfg):
        """The kinds (normal / build) under which parent uses child on this target."""
        deps = info[(parent["name"], parent["version"])]["deps"]
        if deps is None:
            return {"normal"}
        matching = [d for d in deps if d["package"] == child["name"]]
        if not matching:
            raise ScanError(f"Cargo.lock says {parent['name']} {parent['version']} depends on {child['name']}, "
                            "but its registry metadata declares no such dependency")
        return {d["kind"] for d in matching if d["kind"] != "dev" and cfg_active(d["target"], triple, cfg)}

    # Per target: the crates linked into the archive (reachable over normal
    # edges), and the crates compiled at all (reachable over normal or build
    # edges -- a build dependency's own normal dependencies are build-time too).
    root_pkg = next(p for p in packages if p["name"] == root and p.get("source") is None)
    linked, built = {}, {}
    for triple, platform, cfg in TARGETS:
        for kinds, reach in (({"normal"}, linked), ({"normal", "build"}, built)):
            seen = set()
            stack = [root_pkg]
            while stack:
                pkg = stack.pop()
                for spec in pkg.get("dependencies", []):
                    child = resolve(spec)
                    key = (child["name"], child["version"])
                    if key not in seen and edge_kinds(pkg, child, triple, cfg) & kinds:
                        seen.add(key)
                        stack.append(child)
            reach[platform] = seen

    platforms = [platform for _, platform, _ in TARGETS]
    rows = []
    for key, entry in sorted(info.items(), key=lambda kv: (kv[0][0].lower(), kv[0][0], kv[0][1])):
        if entry["source"] == "root":
            continue
        linked_on = [pl for pl in platforms if key in linked[pl]]
        built_on = [pl for pl in platforms if key in built[pl]]
        if linked_on:
            role, on = "linked", linked_on
        elif built_on:
            role, on = "build", built_on
        else:
            role, on = "other-targets", []
        license_ = entry["license"] or ("NOT-DECLARED" if entry["source"] in ("crates.io", "patch")
                                        else "NOT-RECORDED")
        rows.append({
            "name": key[0],
            "version": key[1],
            "license": license_,
            "source": entry["source"],
            "role": role,
            # Empty when the role holds on every target.
            "platforms": [] if on == platforms or not on else on,
        })

    result = {
        "meta": {"lockfile": args.lockfile.replace(os.sep, "/"), "root": root, "platforms": platforms},
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


def allowed(lic):
    """True when every identifier in the expression is on the allowlist.

    A disjunction with one copyleft option is left to license_check.sh, which
    knows about elections; this only decides what is worth printing. The
    legacy "MIT/Apache-2.0" spelling crates.io still serves for old versions
    splits the same way.
    """
    names = re.split(r"\s+(?:AND|OR|WITH)\s+|/", lic)
    return all(n.strip("() ").removesuffix("+") in ALLOWED for n in names)


if __name__ == "__main__":
    try:
        main()
    except Unreachable as e:
        print(f"scan_cargo: crates.io is unreachable and the cache does not cover the lockfile: {e}",
              file=sys.stderr)
        sys.exit(UNREACHABLE)
    except ScanError as e:
        print(f"scan_cargo: {e}", file=sys.stderr)
        sys.exit(2)
