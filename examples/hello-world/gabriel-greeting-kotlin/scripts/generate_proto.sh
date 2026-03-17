#!/bin/zsh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)

mkdir -p "$ROOT/gen/kotlin"
protoc \
  -I "$ROOT/../../_protos" \
  --java_out="$ROOT/gen/kotlin" \
  --kotlin_out="$ROOT/gen/kotlin" \
  --plugin=protoc-gen-grpc-java="$HOME/.gradle/caches/modules-2/files-2.1/io.grpc/protoc-gen-grpc-java/1.60.0/58b05363282f34f342fced67ae8eb6484545db07/protoc-gen-grpc-java-1.60.0-osx-aarch_64.exe" \
  --grpc-java_out="$ROOT/gen/kotlin" \
  --plugin=protoc-gen-grpc-kotlin="$ROOT/scripts/protoc-gen-grpc-kotlin.sh" \
  --grpc-kotlin_out="$ROOT/gen/kotlin" \
  "$ROOT/../../_protos/v1/greeting.proto"

descriptor_file=$(mktemp)
protoc \
  -I "$ROOT/api" \
  -I "$ROOT/../../_protos" \
  -I "$ROOT/../../../_protos" \
  --descriptor_set_out="$descriptor_file" \
  "v1/holon.proto"
rm -f "$descriptor_file"
