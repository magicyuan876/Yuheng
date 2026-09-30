# Targets for working on docreader outside its image. Call from anywhere:
# `make -C docreader run`, or `make run` inside docreader/.
#
# The service imports itself as the package `docreader` (docreader.main,
# docreader.proto, ...), so Python has to start in the directory that
# *contains* docreader/ — the repository root. That is the image's layout too:
# the package sits at /app/docreader, WORKDIR is /app and the CMD is
# `uv run -m docreader.main`. Started inside docreader/ the import fails with
# "No module named 'docreader'", which is what the old `run` target (pointing
# at a src/server/server.py that no longer exists) never got far enough to hit.
# The uv project — pyproject.toml, uv.lock, .venv — lives here, hence
# `--project`; `--locked` refuses a uv.lock that has drifted from pyproject,
# as the image build does.
ROOT := $(abspath $(CURDIR)/..)
UV_RUN := cd "$(ROOT)" && uv run --locked --project "$(CURDIR)"

.PHONY: run proto clean

# Start the gRPC server on DOCREADER_GRPC_PORT (default 50051).
run:
	$(UV_RUN) python -m docreader.main

# Regenerate the Python (and, when protoc-gen-go is installed, Go) stubs from
# proto/docreader.proto. The script uses repository-relative paths and bash
# syntax, and needs grpc_tools from the project's environment.
proto:
	$(UV_RUN) bash docreader/scripts/generate_proto.sh

# Remove bytecode caches from the sources (the .venv is left alone).
clean:
	find . -path ./.venv -prune -o -name "__pycache__" -type d -prune -exec rm -rf {} +
	find . -path ./.venv -prune -o -name "*.pyc" -type f -exec rm -f {} +
