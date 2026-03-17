#!/bin/sh
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
CANONICAL_ROOT="$ROOT/../../protos"
DESCRIBE_PROTO="$ROOT/protos/describe/greeting/v1/greeting.proto"

mkdir -p "$(dirname "$DESCRIBE_PROTO")" "$ROOT/gen/dart"
cp "$CANONICAL_ROOT/greeting/v1/greeting.proto" "$DESCRIBE_PROTO"

PATH="$HOME/.pub-cache/bin:$PATH" protoc \
  -I "$CANONICAL_ROOT" \
  --dart_out=grpc:"$ROOT/gen/dart" \
  "$CANONICAL_ROOT/greeting/v1/greeting.proto"
