#!/bin/sh
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
CANONICAL_ROOT="$ROOT/../../protos"
DESCRIBE_PROTO="$ROOT/protos/describe/greeting/v1/greeting.proto"
GEN_ROOT="$ROOT/gen/swift"

mkdir -p "$(dirname "$DESCRIBE_PROTO")" "$GEN_ROOT"
cp "$CANONICAL_ROOT/greeting/v1/greeting.proto" "$DESCRIBE_PROTO"

protoc \
  -I "$CANONICAL_ROOT" \
  --swift_opt=Visibility=Public \
  --swift_out="$GEN_ROOT" \
  "$CANONICAL_ROOT/greeting/v1/greeting.proto"
