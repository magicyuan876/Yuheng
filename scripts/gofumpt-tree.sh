#!/usr/bin/env bash
# Format-check (or format) every Go file of the root module with gofumpt.
#
#   scripts/gofumpt-tree.sh check   # list unformatted files, exit 1 if any
#   scripts/gofumpt-tree.sh write   # rewrite them in place
#
# One definition of "the tree" for `make fmt`, `make fmt-check` and CI. It
# skips the separately versioned modules (cli/, client/), vendored code
# (third_party/), node_modules and generated files.
#
# The gofumpt version is pinned to the one golangci-lint embeds (see
# GOLANGCI_LINT_VERSION in .github/workflows/go-lint.yml), so the lint job, CI
# and a developer's `make fmt` all format identically. gofumpt is a superset of
# gofmt, so this also covers plain gofmt. Bump both together.
set -euo pipefail

GOFUMPT_VERSION="v0.9.2"

cd "$(dirname "${BASH_SOURCE[0]}")/.."

mode="${1:-check}"
case "$mode" in
check | write) ;;
*)
	echo "usage: $0 [check|write]" >&2
	exit 2
	;;
esac

if ! command -v gofumpt >/dev/null 2>&1; then
	echo "gofumpt is not installed. Install the pinned version:" >&2
	echo "  go install mvdan.cc/gofumpt@${GOFUMPT_VERSION}" >&2
	exit 2
fi
if ! gofumpt --version 2>/dev/null | grep -q "^${GOFUMPT_VERSION} "; then
	echo "warning: gofumpt $(gofumpt --version 2>/dev/null | cut -d' ' -f1) found, CI uses ${GOFUMPT_VERSION};" >&2
	echo "  results may differ. go install mvdan.cc/gofumpt@${GOFUMPT_VERSION}" >&2
fi

files="$(
	find . -name '*.go' \
		-not -path './cli/*' -not -path './client/*' -not -path './third_party/*' \
		-not -path '*/node_modules/*' -not -path './.git/*' -print0 |
		xargs -0 grep -L -E '^// Code generated .* DO NOT EDIT\.$' || true
)"
[ -n "$files" ] || exit 0

if [ "$mode" = write ]; then
	echo "$files" | xargs gofumpt -w
	exit 0
fi

unformatted="$(echo "$files" | xargs gofumpt -l)"
if [ -n "$unformatted" ]; then
	echo "These Go files are not gofumpt-formatted (run: make fmt):" >&2
	echo "$unformatted" >&2
	exit 1
fi
