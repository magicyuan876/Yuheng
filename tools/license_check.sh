#!/usr/bin/env bash
#
# License and attribution gate for Yuheng.
#
# Yuheng incorporates upstream code under the MIT License (see NOTICE). The MIT
# License's one condition is that the copyright and permission notices travel
# with the code, so the files that carry them are not ordinary files: deleting or hollowing one out is the single
# change that would make a release non-compliant. This script fails the build if
# that happens, and fails it again if a dependency arrives under a license we
# cannot ship under MIT.
#
#   tools/license_check.sh              # all checks
#   tools/license_check.sh attribution  # just the file/notice checks (no deps)
#   tools/license_check.sh deps         # just the dependency license scan
#   tools/license_check.sh notices      # regenerate THIRD_PARTY_NOTICES.md's lists
#
# deps and notices take optional section names (go, cargo-anydoc, npm-frontend,
# npm-collab, python-docreader, python-mcp-server) to limit the run to those
# ecosystems. A section whose scan cannot run here (no module cache, no
# node_modules, no uv, crates.io unreachable with a cold cache) is skipped with
# a warning. deps also fails when THIRD_PARTY_NOTICES.md no longer matches
# what it scanned; the fix is always `notices`, then commit.
#
# Exit codes: 0 clean, 1 a check failed, 2 the script could not run a check.

set -uo pipefail

cd "$(dirname "$0")/.."
ROOT=$(pwd)
MODE=${1:-all}
FAILED=0
PY=${PYTHON:-python3}

red()  { printf '\033[31m%s\033[0m\n' "$*"; }
grn()  { printf '\033[32m%s\033[0m\n' "$*"; }
ylw()  { printf '\033[33m%s\033[0m\n' "$*"; }
fail() { red "FAIL  $*"; FAILED=1; }
pass() { grn "ok    $*"; }
warn() { ylw "warn  $*"; }

# ---------------------------------------------------------------------------
# 1. The notice files must exist and still say what they are supposed to say.
# ---------------------------------------------------------------------------
check_attribution() {
  echo "== attribution =="

  local f
  for f in LICENSE NOTICE THIRD_PARTY_NOTICES.md licenses/NOTICE_AUDIT.md; do
    if [[ ! -s "$f" ]]; then
      fail "$f is missing or empty"
    else
      pass "$f present"
    fi
  done

  [[ -d licenses/upstream-weknora ]] \
    && pass "licenses/upstream-weknora/ present ($(find licenses/upstream-weknora -type f | wc -l) files)" \
    || fail "licenses/upstream-weknora/ is missing"

  # The upstream MIT grant. Without these exact facts in LICENSE the fork has no
  # permission to exist.
  grep -q "Copyright (C) 2025 Tencent" LICENSE \
    && pass "LICENSE carries Tencent's copyright line" \
    || fail "LICENSE no longer carries 'Copyright (C) 2025 Tencent'"

  grep -q "Permission is hereby granted, free of charge" LICENSE \
    && pass "LICENSE carries the MIT permission notice" \
    || fail "LICENSE no longer carries the MIT permission notice"

  # LICENSE also carries upstream's third-party notice bundle; a truncated file
  # usually means someone replaced it with a bare MIT template.
  local lines
  lines=$(wc -l < LICENSE)
  if (( lines < 3000 )); then
    fail "LICENSE is only ${lines} lines — the upstream third-party notices look truncated"
  else
    pass "LICENSE is ${lines} lines (third-party notices intact)"
  fi

  grep -q "Portions Copyright (c) 2025 Tencent" NOTICE \
    && pass "NOTICE carries the dual copyright lines" \
    || fail "NOTICE no longer carries 'Portions Copyright (c) 2025 Tencent'"

  grep -q "Copyright (c) 2024 WeKnora Team" mcp-server/LICENSE \
    && pass "mcp-server/LICENSE keeps its original copyright" \
    || fail "mcp-server/LICENSE lost 'Copyright (c) 2024 WeKnora Team'"

  # Every file that inherited a Tencent header must still say so. The list is
  # the chart as the initial import (978b666) brought it in, written out because
  # CI checks out shallow and cannot ask git; a template written here later is
  # ours alone and carries no Tencent line. A listed file that has since been
  # deleted owes nothing.
  local hdr missing=0
  for hdr in helm/values.yaml helm/templates/{NOTES.txt,_helpers.tpl,app.yaml,docreader.yaml,frontend.yaml} \
    helm/templates/{ingress.yaml,neo4j.yaml,postgres.yaml,pvc.yaml,redis.yaml,secrets.yaml,serviceaccount.yaml}; do
    [[ -f $hdr ]] || continue
    grep -q "Portions Copyright (c) 2025 Tencent" "$hdr" || { red "      $hdr"; missing=1; }
  done
  (( missing )) && fail "helm files above lost their Tencent attribution header" \
                || pass "helm files keep 'Portions Copyright (c) 2025 Tencent'"

  # Upstream issue references ('upstream#NNNN', defined in NOTICE) are
  # attribution facts, not brand strings.
  local refs
  refs=$(git grep -c 'upstream#[0-9]' -- ':!CHANGELOG.md' ':!licenses/' ':!NOTICE' 2>/dev/null | wc -l)
  if (( refs == 0 )); then
    warn "no 'upstream#NNNN' references left in source — if a rename stripped them, restore them"
  else
    pass "$refs file(s) still carry upstream issue references"
  fi
}

# ---------------------------------------------------------------------------
# 2. Residual brand strings outside the places they are allowed to appear.
# ---------------------------------------------------------------------------
check_residual_brand() {
  echo
  echo "== residual brand strings =="

  local hits
  # A line naming Tencent alongside WeKnora is attribution: the fork statement,
  # the trademark disclaimer, an upstream issue reference. A rename we actually
  # missed looks like 'weknora_embeddings' or 'WEKNORA_HOST' and never mentions
  # Tencent, so requiring that co-occurrence separates the two cheaply.
  # A line that must name the old brand for another reason -- an upgrade note
  # telling operators what the old default was, say -- opts out explicitly with
  # a trailing 'license-check: attribution' comment.
  hits=$(git grep -In -i -e 'weknora' -e 'genscript' -- \
           ':!LICENSE' ':!NOTICE' ':!THIRD_PARTY_NOTICES.md' ':!licenses/' \
           ':!CHANGELOG.md' ':!cli/CHANGELOG.md' ':!mcp-server/CHANGELOG.md' \
           ':!mcp-server/LICENSE' ':!tools/license_check.sh' \
         2>/dev/null | grep -v -e 'Tencent' -e '腾讯' -e 'license-check: attribution' || true)

  if [[ -n "$hits" ]]; then
    fail "'weknora' / 'genscript' appears outside the legal/attribution files:"
    printf '%s\n' "$hits" | head -20 | sed 's/^/      /'
  else
    pass "no stray 'weknora' / 'genscript' outside legal and attribution contexts"
  fi

  # Claims of affiliation are the actual impersonation risk, not the name. Only
  # positive claims count: the disclaimers in the READMEs say the opposite of
  # what these patterns describe, so lines carrying a negation are dropped.
  local claims
  claims=$(git grep -In -i -e 'official Tencent' -e 'endorsed by Tencent' \
             -e 'a Tencent product' -e 'Tencent open.source project' \
             -e '腾讯官方' -e '官方出品' -e '腾讯出品' -e '腾讯开源的' \
             -- ':!licenses/' ':!NOTICE' ':!CHANGELOG.md' ':!tools/license_check.sh' 2>/dev/null \
           | grep -v -i -e ' not ' -e '不是' -e '不隶属' -e 'ではありません' -e '아닙니다' || true)
  if [[ -n "$claims" ]]; then
    fail "text implying Tencent affiliation or endorsement:"
    printf '%s\n' "$claims" | head -10 | sed 's/^/      /'
  else
    pass "no affiliation/endorsement claims"
  fi
}

# ---------------------------------------------------------------------------
# 3. Dependency licenses, and the generated half of THIRD_PARTY_NOTICES.md.
# ---------------------------------------------------------------------------
# Anything in DENIED stops a release outright. Anything not in ALLOWED and not in
# DENIED is reported for a human to classify, rather than passed silently.
DENIED_RE='GPL|AGPL|LGPL|SSPL|Elastic|Business Source|BUSL|Commons Clause'

# One section of THIRD_PARTY_NOTICES.md per scanned ecosystem; the names are the
# ones in its GENERATED markers and in tools/licensescan/render_notices.py.
ALL_SECTIONS=(go cargo-anydoc npm-frontend npm-collab python-docreader python-mcp-server)

label() {
  case "$1" in
    go)                echo "Go" ;;
    cargo-anydoc)      echo "Rust crates (anydoc-go)" ;;
    npm-frontend)      echo "npm (frontend)" ;;
    npm-collab)        echo "npm (collab)" ;;
    python-docreader)  echo "Python (docreader)" ;;
    python-mcp-server) echo "Python (mcp-server)" ;;
  esac
}

# The image each Python project ships as. scan_python.py resolves the lockfile
# for that image's Python, read from its FROM line, not for whatever Python the
# machine running the scan happens to have.
python_dockerfile() {
  case "$1" in
    docreader)  echo docker/Dockerfile.docreader ;;
    mcp-server) echo mcp-server/Dockerfile ;;
  esac
}

# The lockfile each Cargo section reads. Its crates are statically linked into
# the app binary through the vendored Go bindings (the `anydoc` build tag).
cargo_lockfile() {
  case "$1" in
    anydoc) echo third_party/anydoc-go/Cargo.lock ;;
  esac
}

# Where scan_cargo.py keeps crates.io's answers. They describe published crate
# versions, which never change, so the cache is safe to keep indefinitely; CI
# restores it with actions/cache.
CARGO_CACHE=${LICENSESCAN_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/yuheng-licensescan}/cargo

# scan_section NAME DIR — run one ecosystem's scanner, leaving DIR/NAME.json
# (for render_notices.py) and DIR/NAME.txt (for report). Returns 0 when the scan
# ran, 3 when it cannot run in this environment (with a warning saying what is
# missing), and 1 when it ran and failed.
scan_section() {
  local name=$1 dir=$2
  local json="$dir/$name.json" txt="$dir/$name.txt" status=0
  case "$name" in
    go)
      local cache
      cache=$(go env GOMODCACHE 2>/dev/null)
      if [[ -z $cache || ! -d $cache ]]; then
        warn "go: no Go toolchain or module cache; skipping (run 'go mod download' in ., cli/ and client/)"
        return 3
      fi
      { "$PY" tools/licensescan/collect_gomods.py "$ROOT" "$dir/gomods.json" \
          && "$PY" tools/licensescan/scan_go.py --modcache "$cache" --repo "$ROOT" \
               "$dir/gomods.json" "$json"; } > "$txt" 2>&1 || status=$?
      ;;
    cargo-*)
      # No Rust toolchain needed: scan_cargo.py reads the lockfile and asks
      # crates.io, caching every answer. It needs tomllib (Python 3.11+).
      if ! "$PY" -c 'import tomllib' 2>/dev/null; then
        warn "$name: $PY has no tomllib (Python 3.11+ needed); skipping"
        return 3
      fi
      "$PY" tools/licensescan/scan_cargo.py --lockfile "$(cargo_lockfile "${name#cargo-}")" \
          --cache-dir "$CARGO_CACHE" "$json" > "$txt" 2>&1 || status=$?
      # Unreachable registry, cold cache: on a developer's machine that is "cannot
      # run here", like a missing node_modules. In CI it is a failure, or an
      # outage would pass the gate without checking anything.
      if (( status == 3 )) && [[ -z ${CI:-} ]]; then
        warn "$name: crates.io unreachable and $CARGO_CACHE is cold; skipping"
        sed 's/^/      /' "$txt"
        return 3
      fi
      ;;
    npm-*)
      local project=${name#npm-}
      if [[ ! -d $project/node_modules ]]; then
        warn "$name: $project/node_modules not found; skipping (run 'npm ci --ignore-scripts' in $project/)"
        return 3
      fi
      "$PY" tools/licensescan/scan_npm.py "$project" "$json" > "$txt" 2>&1 || status=$?
      ;;
    python-*)
      # The Python scans resolve the lockfile for the published images with uv
      # (see scan_python.py), so they need uv, not an activated venv.
      local project=${name#python-}
      if ! command -v uv >/dev/null 2>&1; then
        warn "$name: uv not found; skipping (CI checks this section in its own job)"
        return 3
      fi
      "$PY" tools/licensescan/scan_python.py --project "$project" \
          --dockerfile "$(python_dockerfile "$project")" "$json" > "$txt" 2>&1 || status=$?
      ;;
  esac
  if (( status != 0 )); then
    fail "$name: the scan did not complete"
    sed 's/^/      /' "$txt"
    return 1
  fi
  return 0
}

# The dependency gate: scan each section, fail on a license we cannot ship, and
# fail when THIRD_PARTY_NOTICES.md no longer says what the scan found. The
# second check is what keeps the notices from drifting again: a dependency
# change that lands without `tools/license_check.sh notices` breaks the build.
check_deps() {
  echo
  echo "== dependency licenses =="

  command -v "$PY" >/dev/null 2>&1 || { fail "python3 not found; set PYTHON=..."; return; }

  local tmp name
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN

  for name in "${SECTIONS[@]}"; do
    scan_section "$name" "$tmp" || continue
    report "$(label "$name")" "$tmp/$name.txt"
    verify_notices "$name" "$tmp/$name.json"
  done
}

verify_notices() {
  local name=$1 json=$2 out status=0
  out=$("$PY" tools/licensescan/render_notices.py --check "$name=$json" 2>&1) || status=$?
  case $status in
    0) pass "$(label "$name"): THIRD_PARTY_NOTICES.md matches the scan" ;;
    1) fail "$(label "$name"): THIRD_PARTY_NOTICES.md is out of date — run 'tools/license_check.sh notices $name' and commit the result"
       printf '%s\n' "$out" | sed 's/^/      /' ;;
    *) fail "$(label "$name"): could not render THIRD_PARTY_NOTICES.md"
       printf '%s\n' "$out" | sed 's/^/      /' ;;
  esac
}

# Regenerate every section whose scan can run here. A section that cannot be
# scanned (no uv, no node_modules) is left exactly as it is, so running this on
# a machine with only part of the toolchain never deletes what another machine
# generated.
generate_notices() {
  echo "== THIRD_PARTY_NOTICES.md =="

  command -v "$PY" >/dev/null 2>&1 || { fail "python3 not found; set PYTHON=..."; return; }

  local tmp name out status=0 specs=()
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN

  for name in "${SECTIONS[@]}"; do
    scan_section "$name" "$tmp" && specs+=("$name=$tmp/$name.json")
  done
  if (( ${#specs[@]} == 0 )); then
    warn "no section could be scanned here; THIRD_PARTY_NOTICES.md left untouched"
    return
  fi
  out=$("$PY" tools/licensescan/render_notices.py "${specs[@]}" 2>&1) || status=$?
  if (( status != 0 )); then
    fail "could not render THIRD_PARTY_NOTICES.md"
    printf '%s\n' "$out" | sed 's/^/      /'
  else
    printf '%s\n' "$out" | sed 's/^/ok    /'
  fi
}

# A disjunction like '(MIT OR GPL-3.0-or-later)' is not a copyleft dependency:
# the licensee picks. Only fail when every option is copyleft, and record the
# elections in THIRD_PARTY_NOTICES.md so the choice is written down somewhere.
# An expression that also has an AND is not read as a choice at all: in
# '(MIT OR Apache-2.0) AND LGPL-2.1' the LGPL term applies whatever is elected,
# and crates.io serves compound expressions like that routinely. Such a line
# fails, and a human either proves it harmless or lists it as an exception.
# MPL counts as an acceptable option: its copyleft is per file, and a covered
# file used unmodified obliges nothing an MIT distribution cannot meet (this is
# how `tld`'s MPL-1.1 OR GPL OR LGPL is elected; NOTICE_AUDIT.md §2).
PERMISSIVE_RE='MIT|Apache-2\.0|BSD|ISC|Unlicense|CC0|Zlib|PSF|Python-2\.0|MPL'

EXCEPTIONS=licenses/known-exceptions.txt

# True when the line names a dependency already on the blocking list. Those are
# warnings, not failures: they are inherited, tracked in NOTICE_AUDIT.md, and
# gate releases there. Failing the build on them would leave the pipeline
# permanently red, which is exactly how a NEW copyleft dependency slips in
# unnoticed.
is_known_exception() {
  local line=$1 pattern
  [[ -f $EXCEPTIONS ]] || return 1
  while IFS= read -r pattern; do
    pattern=${pattern%%#*}
    pattern=$(printf '%s' "$pattern" | tr -d '[:space:]')
    [[ -z $pattern ]] && continue
    [[ $line == *"$pattern"* ]] && return 0
  done < "$EXCEPTIONS"
  return 1
}

# Every scanner prints one '  <name> <version> -> <license>' line per component
# that is not plainly permissive. Only the license part is matched, so a package
# whose *name* happens to contain 'gpl' or 'mit' is not misread.
report() {
  local eco=$1 out=$2
  if [[ ! -s "$out" ]]; then
    warn "$eco: scanner produced no output"
    return
  fi

  local bad='' known='' line lic
  while IFS= read -r line; do
    lic=${line##* -> }
    lic=${lic%% (file: *}
    [[ $lic =~ $DENIED_RE ]] || continue
    if [[ $lic == *' OR '* && $lic != *' AND '* ]] && [[ $lic =~ $PERMISSIVE_RE ]]; then
      continue   # dual-licensed; a permissive option exists
    fi
    if is_known_exception "$line"; then
      known+="$line"$'\n'
    else
      bad+="$line"$'\n'
    fi
  done < <(grep -- " -> " "$out" || true)

  if [[ -n "${bad//[$'\n' ]/}" ]]; then
    fail "$eco: NEW copyleft / source-available dependency"
    printf '%s' "$bad" | sed 's/^/      /'
  else
    pass "$eco: no new denied licenses"
  fi

  if [[ -n "${known//[$'\n' ]/}" ]]; then
    warn "$eco: known blockers still present — releases stay blocked (NOTICE_AUDIT.md §1)"
    printf '%s' "$known" | sed 's/^/      /'
  fi

  local review
  review=$(grep -E 'REVIEW|NOT-DECLARED|NOT-RECORDED|UNRECOGNISED|NO-LICENSE-FILE|NOT-IN-CACHE' "$out" || true)
  if [[ -n "$review" ]]; then
    warn "$eco: components needing manual classification"
    printf '%s\n' "$review" | head -15 | sed 's/^/      /'
  fi
}

# Sections named after the mode narrow the dependency scan to those ecosystems;
# the Python CI jobs use this to check only their own section.
shift $(( $# > 0 ? 1 : 0 ))
SECTIONS=("$@")
(( ${#SECTIONS[@]} )) || SECTIONS=("${ALL_SECTIONS[@]}")
for name in "${SECTIONS[@]}"; do
  [[ " ${ALL_SECTIONS[*]} " == *" $name "* ]] || {
    echo "unknown section '$name' (expected one of: ${ALL_SECTIONS[*]})" >&2; exit 2; }
done

case "$MODE" in
  attribution) check_attribution; check_residual_brand ;;
  deps)        check_deps ;;
  notices)     generate_notices ;;
  all)         check_attribution; check_residual_brand; check_deps ;;
  *)           echo "usage: $0 [all|attribution|deps|notices] [section...]" >&2; exit 2 ;;
esac

echo
if (( FAILED )); then
  red "license check FAILED — see licenses/NOTICE_AUDIT.md"
  exit 1
fi
grn "license check passed"
