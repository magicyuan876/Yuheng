#!/bin/bash
# Regenerate the Python and Go stubs from docreader/proto/docreader.proto.
# Run from the repository root inside the docreader uv environment, which is
# what `make -C docreader proto` does.
#
# Both languages are compiled by the protoc that grpcio-tools bundles, so the
# compiler version stamped into every stub header is the one uv.lock pins and
# no system protoc is needed. The Go plugins are not Python packages; install
# them at the versions the committed stubs name in their headers, so a
# regeneration changes only what the .proto changed:
#
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
set -euxo pipefail

PROTO_DIR="docreader/proto"
OUT_DIR="docreader/proto"

go_out=()
if command -v protoc-gen-go >/dev/null && command -v protoc-gen-go-grpc >/dev/null; then
    go_out=(
        --go_out="${OUT_DIR}" --go_opt=paths=source_relative
        --go-grpc_out="${OUT_DIR}" --go-grpc_opt=paths=source_relative
    )
else
    # Without the Go stubs the Go app would silently keep compiling against
    # the old schema, so say so loudly rather than skipping in passing.
    echo "WARNING: protoc-gen-go / protoc-gen-go-grpc not on PATH; Go stubs NOT regenerated" >&2
fi
# The ${a[@]+...} form expands an empty array to nothing; a plain "${a[@]}"
# trips `set -u` on the bash 3.2 macOS ships.

python3 -m grpc_tools.protoc -I"${PROTO_DIR}" \
    --python_out="${OUT_DIR}" \
    --pyi_out="${OUT_DIR}" \
    --grpc_python_out="${OUT_DIR}" \
    ${go_out[@]+"${go_out[@]}"} \
    "${PROTO_DIR}/docreader.proto"

# grpc_tools emits a top-level `import docreader_pb2`, which only resolves when
# docreader/proto is itself on sys.path. The service imports the stubs as the
# package docreader.proto, so rewrite the import to the package path. The
# in-place flag differs between BSD sed (macOS) and GNU sed.
if [ "$(uname)" == "Darwin" ]; then
    sed -i '' 's/^import docreader_pb2/from docreader.proto import docreader_pb2/' "${OUT_DIR}/docreader_pb2_grpc.py"
else
    sed -i 's/^import docreader_pb2/from docreader.proto import docreader_pb2/' "${OUT_DIR}/docreader_pb2_grpc.py"
fi

echo "Proto files generated successfully!"
