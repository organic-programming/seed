#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/gen/c/greeting/v1"
TMP=$(mktemp -d)

if [[ -n "${OP_SDK_C_PATH:-}" ]]; then
  PROTOC="$OP_SDK_C_PATH/bin/protoc"
  UPB_PLUGIN="$OP_SDK_C_PATH/bin/protoc-gen-upb"
  UPBDEFS_PLUGIN="$OP_SDK_C_PATH/bin/protoc-gen-upbdefs"
else
  PROTOC=$(command -v protoc)
  UPB_PLUGIN=$(command -v protoc-gen-upb)
  UPBDEFS_PLUGIN=$(command -v protoc-gen-upbdefs)
fi

mkdir -p "$OUT"
rm -f "$OUT"/greeting.upb.h "$OUT"/greeting.upb.c "$OUT"/greeting.upb_minitable.h \
  "$OUT"/greeting.upb_minitable.c "$OUT"/greeting.upbdefs.h "$OUT"/greeting.upbdefs.c

"$PROTOC" \
  --plugin=protoc-gen-upb="$UPB_PLUGIN" \
  --plugin=protoc-gen-upbdefs="$UPBDEFS_PLUGIN" \
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

rm -rf "$TMP"
