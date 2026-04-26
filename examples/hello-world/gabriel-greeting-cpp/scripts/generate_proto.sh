#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/gen/cpp/greeting/v1"
TMP=$(mktemp -d)

if [[ -n "${OP_SDK_CPP_PATH:-}" ]]; then
  PROTOC="$OP_SDK_CPP_PATH/bin/protoc"
  GRPC_PLUGIN="$OP_SDK_CPP_PATH/bin/grpc_cpp_plugin"
else
  PROTOC=$(command -v protoc)
  GRPC_PLUGIN=$(command -v grpc_cpp_plugin)
fi

mkdir -p "$OUT"
rm -f "$OUT"/greeting.pb.h "$OUT"/greeting.pb.cc "$OUT"/greeting.grpc.pb.h "$OUT"/greeting.grpc.pb.cc

"$PROTOC" \
  --cpp_out="$TMP" \
  --grpc_out="$TMP" \
  --plugin=protoc-gen-grpc="$GRPC_PLUGIN" \
  -I "$ROOT/../../_protos/v1" \
  "$ROOT/../../_protos/v1/greeting.proto"

mv "$TMP/greeting.pb.h" "$OUT/greeting.pb.h"
mv "$TMP/greeting.pb.cc" "$OUT/greeting.pb.cc"
mv "$TMP/greeting.grpc.pb.h" "$OUT/greeting.grpc.pb.h"
mv "$TMP/greeting.grpc.pb.cc" "$OUT/greeting.grpc.pb.cc"

rm -rf "$TMP"
