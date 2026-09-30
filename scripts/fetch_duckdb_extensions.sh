#!/usr/bin/env bash
# Downloads the DuckDB extensions the app image needs into
# docker/duckdb-extensions, for the architecture of the Docker host, so the
# image build does not have to fetch them itself (see the README there).
#
#   scripts/fetch_duckdb_extensions.sh [linux_amd64|linux_arm64]
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
cd "$root"

platform="${1:-}"
if [ -z "$platform" ]; then
  case "$(docker info --format '{{.Architecture}}' 2>/dev/null || uname -m)" in
    aarch64 | arm64) platform=linux_arm64 ;;
    x86_64 | amd64) platform=linux_amd64 ;;
    *) echo "cannot tell the Docker architecture; pass linux_amd64 or linux_arm64" >&2; exit 2 ;;
  esac
fi

version="$(go run ./cmd/download/duckdb -print-version)"
dest="docker/duckdb-extensions/${version}/${platform}"
mkdir -p "$dest"
for ext in spatial excel; do
  out="${dest}/${ext}.duckdb_extension"
  if [ -s "$out" ]; then
    echo "have ${out}"
    continue
  fi
  url="https://extensions.duckdb.org/${version}/${platform}/${ext}.duckdb_extension.gz"
  echo "fetching ${url}"
  curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 -o "${out}.gz.part" "$url"
  gunzip -c "${out}.gz.part" >"${out}.part"
  mv "${out}.part" "$out"
  rm -f "${out}.gz.part"
done
echo "done: ${dest}"
