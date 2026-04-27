#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
OUT_DIR="$ROOT_DIR/gen/ruby/greeting"

mkdir -p "$OUT_DIR"

if [ -n "${OP_SDK_RUBY_PATH:-}" ] && [ -d "$OP_SDK_RUBY_PATH/vendor/bundle" ]; then
  export BUNDLE_GEMFILE="$ROOT_DIR/Gemfile"
  export BUNDLE_PATH="$OP_SDK_RUBY_PATH/vendor/bundle"
  export BUNDLE_DISABLE_SHARED_GEMS=true
  GRPC_TOOLS_PROTOC=$(bundle exec ruby -e 'print Gem.bin_path("grpc-tools", "grpc_tools_ruby_protoc")')
  "$GRPC_TOOLS_PROTOC" \
    --ruby_out="$OUT_DIR" \
    --grpc_out="$OUT_DIR" \
    -I "$ROOT_DIR/../../_protos" \
    "$ROOT_DIR/../../_protos/v1/greeting.proto"
  exit 0
else
  GRPC_RUBY_PLUGIN=$(command -v grpc_ruby_plugin)
fi

protoc \
  --ruby_out="$OUT_DIR" \
  --grpc_out="$OUT_DIR" \
  --plugin=protoc-gen-grpc="$GRPC_RUBY_PLUGIN" \
  -I "$ROOT_DIR/../../_protos" \
  "$ROOT_DIR/../../_protos/v1/greeting.proto"
