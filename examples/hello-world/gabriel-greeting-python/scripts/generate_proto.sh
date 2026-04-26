#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)

mkdir -p "$ROOT/gen/python/greeting"
python3 -m grpc_tools.protoc \
  -I "$ROOT/../../_protos" \
  --python_out="$ROOT/gen/python/greeting" \
  --grpc_python_out="$ROOT/gen/python/greeting" \
  "$ROOT/../../_protos/v1/greeting.proto"
