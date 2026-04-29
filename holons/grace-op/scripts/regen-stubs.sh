#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
op_root="$(CDPATH= cd -- "${script_dir}/.." && pwd)"

for tool in protoc protoc-gen-go protoc-gen-go-grpc; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "missing ${tool} on PATH" >&2
    exit 1
  fi
done

cd "$op_root"
mkdir -p gen/go

protoc \
  -I . \
  -I _protos \
  --go_out=gen/go \
  --go_opt=module=github.com/organic-programming/grace-op/gen/go \
  --go_opt=Mapi/v1/holon.proto=github.com/organic-programming/grace-op/gen/go/op/v1 \
  --go_opt=Mholons/v1/manifest.proto=github.com/organic-programming/go-holons/gen/go/holons/v1 \
  --go-grpc_out=gen/go \
  --go-grpc_opt=module=github.com/organic-programming/grace-op/gen/go \
  --go-grpc_opt=Mapi/v1/holon.proto=github.com/organic-programming/grace-op/gen/go/op/v1 \
  --go-grpc_opt=Mholons/v1/manifest.proto=github.com/organic-programming/go-holons/gen/go/holons/v1 \
  api/v1/holon.proto
