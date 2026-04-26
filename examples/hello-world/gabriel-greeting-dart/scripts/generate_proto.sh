#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TMP="$ROOT/gen/dart/_tmp"
OUT="$ROOT/gen/dart/greeting/v1"

rm -rf "$TMP"
mkdir -p "$TMP" "$OUT"

PATH="$HOME/.pub-cache/bin:$PATH" protoc \
  -I "$ROOT/../../_protos" \
  --dart_out=grpc:"$TMP" \
  "$ROOT/../../_protos/v1/greeting.proto"

mv "$TMP/v1/greeting.pb.dart" "$OUT/greeting.pb.dart"
mv "$TMP/v1/greeting.pbenum.dart" "$OUT/greeting.pbenum.dart"
mv "$TMP/v1/greeting.pbjson.dart" "$OUT/greeting.pbjson.dart"
mv "$TMP/v1/greeting.pbgrpc.dart" "$OUT/greeting.pbgrpc.dart"
rm -rf "$TMP"
