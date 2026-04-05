#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT="$ROOT/app/lib/src/gen"
TMP=$(mktemp -d)

cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is required" >&2
  exit 1
fi

if ! command -v protoc-gen-dart >/dev/null 2>&1; then
  dart pub global activate protoc_plugin >/dev/null
fi

PATH="$HOME/.pub-cache/bin:$PATH" protoc \
  -I "$ROOT/api" \
  -I "$ROOT/../../_protos" \
  -I "$ROOT/../../../holons/grace-op/_protos" \
  --dart_out=grpc:"$TMP" \
  "v1/holon.proto" \
  "v1/greeting.proto" \
  "holons/v1/manifest.proto"

rm -rf "$OUT"
mkdir -p "$OUT"
cp -R "$TMP"/. "$OUT"/
