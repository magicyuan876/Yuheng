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

  # Every file that inherited a Tencent header must still say so.
  local hdr missing=0
  while IFS= read -r hdr; do
    grep -q "Portions Copyright (c) 2025 Tencent" "$hdr" || { red "      $hdr"; missing=1; }
  done < <(git ls-files 'helm/templates/*' helm/values.yaml)
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
# 3. Dependency licenses.
# ---------------------------------------------------------------------------
# Anything in DENIED stops a release outright. Anything not in ALLOWED and not in
# DENIED is reported for a human to classify, rather than passed silently.
DENIED_RE='GPL|AGPL|LGPL|SSPL|Elastic|Business Source|BUSL|Commons Clause'

check_deps() {
  echo
  echo "== dependency licenses =="

  command -v "$PY" >/dev/null 2>&1 || { fail "python3 not found; set PYTHON=..."; return; }

  local tmp
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN

  # -- Go -------------------------------------------------------------------
  if [[ -d "${GOMODCACHE:-$(go env GOMODCACHE 2>/dev/null)}" ]]; then
    "$PY" tools/licensescan/collect_gomods.py "$ROOT" "$tmp/gomods.json" >/dev/null
    MOD_ROOT="$(go env GOMODCACHE)" "$PY" tools/licensescan/scan_go.py \
        "$tmp/gomods.json" "$tmp/go.json" > "$tmp/go.txt" 2>&1
    report "Go" "$tmp/go.txt"
  else
    warn "Go module cache not found; skipping the Go scan (run 'go mod download' first)"
  fi

  # -- npm ------------------------------------------------------------------
  if [[ -d frontend/node_modules ]]; then
    "$PY" tools/licensescan/scan_npm.py frontend/node_modules "$tmp/npm.json" > "$tmp/npm.txt" 2>&1
    report "npm (frontend)" "$tmp/npm.txt"
  else
    warn "frontend/node_modules not found; skipping the npm scan (run 'npm ci' first)"
  fi

  # -- Python ---------------------------------------------------------------
  # Must run against an interpreter that actually has this project's Python
  # dependencies installed -- a bare system python3 would report the base
  # image's Debian packages instead, which is both wrong and alarming.
  # In CI, call this inside the docreader / mcp-server image, or after 'uv sync'
  # with PYTHON pointing at the venv.
  if "$PY" -c 'import grpc_tools' 2>/dev/null || "$PY" -c 'import mcp' 2>/dev/null; then
    "$PY" tools/licensescan/scan_python.py "$tmp/py.json" > "$tmp/py.txt" 2>&1
    report "Python ($PY)" "$tmp/py.txt"
  else
    warn "$PY has no docreader/mcp-server dependencies installed; skipping the Python scan"
    warn "  (run it inside the docreader image, or set PYTHON=<venv>/bin/python)"
  fi
}

# A disjunction like '(MIT OR GPL-3.0-or-later)' is not a copyleft dependency:
# the licensee picks. Only fail when every option is copyleft, and record the
# elections in THIRD_PARTY_NOTICES.md so the choice is written down somewhere.
PERMISSIVE_RE='MIT|Apache-2\.0|BSD|ISC|Unlicense|CC0|Zlib|PSF|Python-2\.0'

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

report() {
  local eco=$1 out=$2
  if [[ ! -s "$out" ]]; then
    warn "$eco: scanner produced no output"
    return
  fi

  local bad='' known='' line
  while IFS= read -r line; do
    if [[ "$line" == *' OR '* ]] && [[ "$line" =~ $PERMISSIVE_RE ]]; then
      continue   # dual-licensed; a permissive option exists
    fi
    if is_known_exception "$line"; then
      known+="$line"$'
'
    else
      bad+="$line"$'
'
    fi
  done < <(grep -E "$DENIED_RE" "$out" | grep -- " -> " || true)

  if [[ -n "${bad//[$'
' ]/}" ]]; then
    fail "$eco: NEW copyleft / source-available dependency"
    printf '%s' "$bad" | sed 's/^/      /'
  else
    pass "$eco: no new denied licenses"
  fi

  if [[ -n "${known//[$'
' ]/}" ]]; then
    warn "$eco: known blockers still present — releases stay blocked (NOTICE_AUDIT.md §1)"
    printf '%s' "$known" | sed 's/^/      /'
  fi

  local review
  review=$(grep -E 'REVIEW|NOT-DECLARED|UNRECOGNISED|NO-LICENSE-FILE|NOT-IN-CACHE' "$out" || true)
  if [[ -n "$review" ]]; then
    warn "$eco: components needing manual classification"
    printf '%s
' "$review" | head -15 | sed 's/^/      /'
  fi
}

case "$MODE" in
  attribution) check_attribution; check_residual_brand ;;
  deps)        check_deps ;;
  all)         check_attribution; check_residual_brand; check_deps ;;
  *)           echo "usage: $0 [all|attribution|deps]" >&2; exit 2 ;;
esac

echo
if (( FAILED )); then
  red "license check FAILED — see licenses/NOTICE_AUDIT.md"
  exit 1
fi
grn "license check passed"
