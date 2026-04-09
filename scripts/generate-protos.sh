#!/usr/bin/env bash

set -euo pipefail

# ==============================================================================
# Canonical Proto Compiler for Core Holon SDKs
# ==============================================================================
#
# PURPOSE:
# This script is a maintainer tool. It compiles the foundational protobuf schemas
# (manifest, describe, coax) into native code bindings for all 13 supported SDKs 
# (Go, Swift, Dart, Python, etc.) and places them in their respective `sdk/*` directories.
#
# WHY IT'S NEEDED EVEN WITH `embed.go`:
# The `op` binary statically embeds the raw `.proto` definitions via `embed.go`.
# That allows `op` to read or serve the plain-text schemas at runtime (e.g., 
# when scaffolding a new holon via `op new`). However, the SDK codebases need 
# the actual *compiled code* (Go structs, Swift classes) to function. Embedding 
# the raw schemas does not magically translate them into Swift or Dart files 
# for the framework code.
#
# WHY NOT HAVE `OP` DO THIS?
# 1. The Bootstrap Chicken-and-Egg: `op` is written in Go and depends on the
#    `go-holons` SDK to even compile. If we relied on `op` to generate the 
#    base protos, we wouldn't be able to build `op` in the first place.
# 2. Maintainer Environment Load: Running this operation requires all 13 
#    `protoc` language plugins installed on the host machine. It is a heavy,
#    infrastructure-level maintenance chore, fundamentally different from `op`'s
#    mission which is to compile isolated user-domain applications and holons.
# ==============================================================================

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_ROOT="$ROOT/holons/grace-op/_protos"
CANONICAL_DESCRIBE="$PROTO_ROOT/holons/v1/describe.proto"
PROTO_FILES=(
  "$PROTO_ROOT/holons/v1/manifest.proto"
  "$PROTO_ROOT/holons/v1/describe.proto"
  "$PROTO_ROOT/holons/v1/coax.proto"
)
PROTO_INCLUDES=(-I "$PROTO_ROOT")
SUMMARY=()

for include_dir in /opt/homebrew/include /usr/local/include; do
  if [[ -d "$include_dir/google/protobuf" ]]; then
    PROTO_INCLUDES+=(-I "$include_dir")
  fi
done

record_summary() {
  SUMMARY+=("$1")
}

usage() {
  cat <<'EOF'
Usage: ./scripts/generate-protos.sh [--sdk=<name>]

SDK names:
  c, cpp, csharp, dart, go, java, js, js-web, kotlin, python, ruby, rust, swift
EOF
}

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "Missing required file: $path" >&2
    exit 1
  fi
}

require_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Missing required tool: $tool" >&2
    exit 1
  fi
}

run_in_root() {
  (
    cd "$ROOT"
    "$@"
  )
}

find_compatible_protobuf33() {
  local candidate
  for candidate in /opt/homebrew/Cellar/protobuf/33.* /usr/local/Cellar/protobuf/33.*; do
    if [[ -d "$candidate/lib" && -d "$candidate/include" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

generate_go() {
  require_tool protoc
  require_tool protoc-gen-go
  require_tool protoc-gen-go-grpc
  local go_package="github.com/organic-programming/go-holons/gen/go/holons/v1;v1"
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --go_out=sdk/go-holons/gen/go \
    --go_opt=paths=source_relative \
    --go_opt=Mholons/v1/manifest.proto="$go_package" \
    --go_opt=Mholons/v1/describe.proto="$go_package" \
    --go_opt=Mholons/v1/coax.proto="$go_package" \
    --go-grpc_out=sdk/go-holons/gen/go \
    --go-grpc_opt=paths=source_relative \
    --go-grpc_opt=Mholons/v1/manifest.proto="$go_package" \
    --go-grpc_opt=Mholons/v1/describe.proto="$go_package" \
    --go-grpc_opt=Mholons/v1/coax.proto="$go_package" \
    "${PROTO_FILES[@]}"
  record_summary "go -> sdk/go-holons/gen/go/holons/v1"
}

generate_swift() {
  require_tool protoc
  require_tool protoc-gen-swift
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --swift_out=sdk/swift-holons/Sources/Holons/Gen \
    --swift_opt=Visibility=Public \
    "${PROTO_FILES[@]}"
  record_summary "swift -> sdk/swift-holons/Sources/Holons/Gen/holons/v1"
}

generate_python() {
  local python_bin="${ROOT}/sdk/python-holons/.venv-codex/bin/python"
  if [[ ! -x "$python_bin" ]]; then
    python_bin="$(command -v python3 || command -v python)"
  fi
  if [[ -z "${python_bin:-}" ]]; then
    echo "Missing required tool: python3 or python" >&2
    exit 1
  fi
  run_in_root "$python_bin" -m grpc_tools.protoc "${PROTO_INCLUDES[@]}" \
    --python_out=sdk/python-holons/gen/python \
    --grpc_python_out=sdk/python-holons/gen/python \
    "${PROTO_FILES[@]}"
  record_summary "python -> sdk/python-holons/gen/python/holons/v1"
}

generate_cpp() {
  require_tool protoc
  require_tool grpc_cpp_plugin
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --cpp_out=sdk/cpp-holons/gen/cpp \
    --grpc_out=sdk/cpp-holons/gen/cpp \
    --plugin=protoc-gen-grpc="$(command -v grpc_cpp_plugin)" \
    "${PROTO_FILES[@]}"
  record_summary "cpp -> sdk/cpp-holons/gen/cpp/holons/v1"
}

generate_c() {
  require_tool protoc-c
  local protoc_c_env=()
  local c_includes=("${PROTO_INCLUDES[@]}")
  local compat_dir=""
  compat_dir="$(find_compatible_protobuf33 || true)"
  if [[ -n "$compat_dir" ]]; then
    protoc_c_env=(env "DYLD_LIBRARY_PATH=${compat_dir}/lib")
    c_includes=(-I "$PROTO_ROOT" -I "${compat_dir}/include")
    for include_dir in /opt/homebrew/include /usr/local/include; do
      if [[ -d "$include_dir/google/protobuf" ]]; then
        c_includes+=(-I "$include_dir")
      fi
    done
  elif ! protoc-c --version >/dev/null 2>&1; then
    echo "protoc-c is installed but not runnable, and no protobuf 33.x runtime was found" >&2
    exit 1
  fi
  run_in_root "${protoc_c_env[@]}" protoc-c "${c_includes[@]}" \
    --c_out=sdk/c-holons/gen/c \
    "${PROTO_FILES[@]}"
  record_summary "c -> sdk/c-holons/gen/c/holons/v1"
}

generate_csharp() {
  require_tool protoc
  require_tool grpc_csharp_plugin
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --csharp_out=sdk/csharp-holons/Holons/Gen \
    --grpc_out=sdk/csharp-holons/Holons/Gen \
    --plugin=protoc-gen-grpc="$(command -v grpc_csharp_plugin)" \
    "${PROTO_FILES[@]}"
  record_summary "csharp -> sdk/csharp-holons/Holons/Gen"
}

generate_dart() {
  require_tool protoc
  local dart_plugin_dir="$HOME/.pub-cache/bin"
  if [[ ! -x "$dart_plugin_dir/protoc-gen-dart" ]]; then
    echo "Missing required tool: protoc-gen-dart (expected at $dart_plugin_dir/protoc-gen-dart)" >&2
    exit 1
  fi
  run_in_root env PATH="$PATH:$dart_plugin_dir" protoc "${PROTO_INCLUDES[@]}" \
    --dart_out=grpc:sdk/dart-holons/lib/gen \
    "${PROTO_FILES[@]}"
  record_summary "dart -> sdk/dart-holons/lib/gen/holons/v1"
}

generate_java() {
  require_tool protoc
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --java_out=sdk/java-holons/src/main/java/gen \
    "${PROTO_FILES[@]}"
  record_summary "java -> sdk/java-holons/src/main/java/gen/holons/v1"
}

generate_kotlin() {
  require_tool protoc
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --java_out=sdk/kotlin-holons/src/main/kotlin/gen \
    "${PROTO_FILES[@]}"
  record_summary "kotlin -> sdk/kotlin-holons/src/main/kotlin/gen/holons/v1"
}

generate_js() {
  local js_dir="$ROOT/sdk/js-holons/src/gen/holons/v1"
  require_file "$js_dir/root.js"
  require_file "$js_dir/manifest.js"
  require_file "$js_dir/describe.js"
  require_file "$js_dir/coax.js"
  record_summary "js -> runtime loader already canonical at sdk/js-holons/src/gen/holons/v1"
}

generate_js_web() {
  local js_dir="$ROOT/sdk/js-web-holons/src/gen/holons/v1"
  require_file "$js_dir/root.mjs"
  require_file "$js_dir/manifest.mjs"
  require_file "$js_dir/describe.mjs"
  require_file "$js_dir/coax.mjs"
  record_summary "js-web -> runtime loader already canonical at sdk/js-web-holons/src/gen/holons/v1"
}

generate_ruby() {
  require_tool protoc
  require_tool grpc_ruby_plugin
  run_in_root protoc "${PROTO_INCLUDES[@]}" \
    --ruby_out=sdk/ruby-holons/lib/gen \
    --grpc_out=sdk/ruby-holons/lib/gen \
    --plugin=protoc-gen-grpc="$(command -v grpc_ruby_plugin)" \
    "${PROTO_FILES[@]}"
  record_summary "ruby -> sdk/ruby-holons/lib/gen/holons/v1"
}

generate_rust() {
  require_tool cargo
  run_in_root cargo build --manifest-path sdk/rust-holons/Cargo.toml
  record_summary "rust -> sdk/rust-holons/src/gen/holons.v1.rs"
}

run_sdk() {
  local sdk="$1"
  case "$sdk" in
    c) generate_c ;;
    cpp) generate_cpp ;;
    csharp) generate_csharp ;;
    dart) generate_dart ;;
    go) generate_go ;;
    java) generate_java ;;
    js) generate_js ;;
    js-web|js_web) generate_js_web ;;
    kotlin) generate_kotlin ;;
    python) generate_python ;;
    ruby) generate_ruby ;;
    rust) generate_rust ;;
    swift) generate_swift ;;
    *)
      echo "Unknown SDK: $sdk" >&2
      usage >&2
      exit 1
      ;;
  esac
}

SDK_FILTER=""
for arg in "$@"; do
  case "$arg" in
    --sdk=*)
      SDK_FILTER="${arg#--sdk=}"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_file "$CANONICAL_DESCRIBE"

if [[ -n "$SDK_FILTER" ]]; then
  run_sdk "$SDK_FILTER"
else
  for sdk in go swift python cpp c csharp dart java kotlin js js-web ruby rust; do
    run_sdk "$sdk"
  done
fi

echo "Generated proto outputs:"
for line in "${SUMMARY[@]}"; do
  echo " - $line"
done
