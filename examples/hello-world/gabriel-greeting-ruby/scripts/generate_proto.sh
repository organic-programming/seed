#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
OUT_DIR="$ROOT_DIR/gen/ruby/greeting"

mkdir -p "$OUT_DIR"

protoc \
  --ruby_out="$OUT_DIR" \
  --grpc_out="$OUT_DIR" \
  --plugin=protoc-gen-grpc="$(command -v grpc_ruby_plugin)" \
  -I "$ROOT_DIR/../../_protos" \
  "$ROOT_DIR/../../_protos/v1/greeting.proto"
