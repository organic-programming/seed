#!/bin/sh
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
PROTO_ROOT="$ROOT/../../protos"

cd "$PROTO_ROOT"

python3 -m grpc_tools.protoc \
  -I . \
  --python_out="$ROOT/gen/python" \
  --grpc_python_out="$ROOT/gen/python" \
  greeting/v1/greeting.proto
