#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/gen/c/greeting/v1"
TMP=$(mktemp -d)
DESCRIPTOR_FILE=$(mktemp)

mkdir -p "$OUT"
rm -f "$OUT"/greeting.upb.h "$OUT"/greeting.upb.c "$OUT"/greeting.upb_minitable.h \
  "$OUT"/greeting.upb_minitable.c "$OUT"/greeting.upbdefs.h "$OUT"/greeting.upbdefs.c

protoc \
  --plugin=protoc-gen-upb="$(command -v protoc-gen-upb)" \
  --plugin=protoc-gen-upbdefs="$(command -v protoc-gen-upbdefs)" \
  --plugin=protoc-gen-upb_minitable="$(command -v protoc-gen-upb_minitable)" \
  --upb_out="$TMP" \
  --upbdefs_out="$TMP" \
  --upb_minitable_out="$TMP" \
  -I "$ROOT/../../_protos/v1" \
  "$ROOT/../../_protos/v1/greeting.proto"

mv "$TMP/greeting.upb.h" "$OUT/greeting.upb.h"
mv "$TMP/greeting.upb.c" "$OUT/greeting.upb.c"
mv "$TMP/greeting.upb_minitable.h" "$OUT/greeting.upb_minitable.h"
mv "$TMP/greeting.upb_minitable.c" "$OUT/greeting.upb_minitable.c"
mv "$TMP/greeting.upbdefs.h" "$OUT/greeting.upbdefs.h"
mv "$TMP/greeting.upbdefs.c" "$OUT/greeting.upbdefs.c"

protoc \
  -I "$ROOT/api" \
  -I "$ROOT/../../_protos" \
  -I "$ROOT/../../../_protos" \
  --descriptor_set_out="$DESCRIPTOR_FILE" \
  v1/holon.proto

rm -rf "$TMP"
rm -f "$DESCRIPTOR_FILE"
