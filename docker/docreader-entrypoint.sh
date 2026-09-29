#!/bin/sh
# Entrypoint of the docreader image: fix ownership of the shared image
# directory, then drop root and exec the service as the `docreader` user.
#
# The directory is a named volume shared with the app container. A volume that
# already exists was populated by earlier versions of this image, which ran as
# root, so its owner is root and the unprivileged service could not write to it.
set -e

if [ "$(id -u)" = "0" ]; then
    dir="${DOCREADER_IMAGE_OUTPUT_DIR:-/tmp/docreader}"
    mkdir -p "$dir"
    chown -R docreader:docreader "$dir" 2>/dev/null || true
    exec gosu docreader "$@"
fi

# Started with an explicit non-root user (compose `user:`): nothing to fix.
exec "$@"
