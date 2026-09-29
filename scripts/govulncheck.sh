#!/usr/bin/env bash
# Runs govulncheck on the server and fails on any vulnerability that the code
# actually reaches and that .govulncheck-ignore does not list with a reason.
# govulncheck itself has no ignore mechanism, and a scan that is always red
# because of findings that do not apply is a scan nobody reads.
#
#   scripts/govulncheck.sh [packages...]    (default: ./cmd/... ./internal/...)
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"
command -v govulncheck >/dev/null || { echo "install: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2; exit 2; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 2; }

pkgs=("$@")
[ ${#pkgs[@]} -eq 0 ] && pkgs=(./cmd/... ./internal/...)

report="$(mktemp)"
trap 'rm -f "$report"' EXIT
govulncheck -format json "${pkgs[@]}" >"$report"

# A finding whose first trace frame names a function is one the code calls; the
# others are only imported or required and are not reported by govulncheck's
# default text output either.
reached="$(jq -r 'select(.finding) | select(.finding.trace[0].function) | .finding.osv' "$report" | sort -u)"
ignored="$(grep -Ev '^[[:space:]]*(#|$)' .govulncheck-ignore | awk '{print $1}' | sort -u)"

new="$(comm -23 <(printf '%s\n' "$reached" | sed '/^$/d') <(printf '%s\n' "$ignored" | sed '/^$/d'))"
stale="$(comm -13 <(printf '%s\n' "$reached" | sed '/^$/d') <(printf '%s\n' "$ignored" | sed '/^$/d'))"

if [ -n "$stale" ]; then
  echo "govulncheck: no longer reported, remove from .govulncheck-ignore:"
  printf '  %s\n' $stale
fi
if [ -n "$new" ]; then
  echo "govulncheck: reachable vulnerabilities not in .govulncheck-ignore:" >&2
  for id in $new; do
    summary="$(jq -r --arg id "$id" 'select(.osv and .osv.id == $id) | .osv.summary' "$report" | head -1)"
    echo "  $id  $summary  (https://pkg.go.dev/vuln/$id)" >&2
  done
  echo "Update the dependency, or add the id with a reason to .govulncheck-ignore." >&2
  exit 1
fi
echo "govulncheck: ok ($(printf '%s\n' "$reached" | sed '/^$/d' | wc -l | tr -d ' ') reported, all listed in .govulncheck-ignore)"
