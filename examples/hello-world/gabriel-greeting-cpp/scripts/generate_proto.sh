#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/gen/cpp/greeting/v1"
TMP=$(mktemp -d)
DESCRIPTOR_FILE=$(mktemp)
GRPC_PLUGIN=$(command -v grpc_cpp_plugin)

mkdir -p "$OUT"
rm -f "$OUT"/greeting.pb.h "$OUT"/greeting.pb.cc "$OUT"/greeting.grpc.pb.h "$OUT"/greeting.grpc.pb.cc

protoc \
  --cpp_out="$TMP" \
  --grpc_out="$TMP" \
  --plugin=protoc-gen-grpc="$GRPC_PLUGIN" \
  -I "$ROOT/../../_protos/v1" \
  "$ROOT/../../_protos/v1/greeting.proto"

mv "$TMP/greeting.pb.h" "$OUT/greeting.pb.h"
mv "$TMP/greeting.pb.cc" "$OUT/greeting.pb.cc"
mv "$TMP/greeting.grpc.pb.h" "$OUT/greeting.grpc.pb.h"
mv "$TMP/greeting.grpc.pb.cc" "$OUT/greeting.grpc.pb.cc"

protoc \
  -I "$ROOT/api" \
  -I "$ROOT/../../_protos" \
  -I "$ROOT/../../../_protos" \
  --descriptor_set_out="$DESCRIPTOR_FILE" \
  v1/holon.proto

rm -rf "$TMP"
rm -f "$DESCRIPTOR_FILE"
