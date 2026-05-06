#!/usr/bin/env bash

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

repo_root_or_pwd() {
  git rev-parse --show-toplevel 2>/dev/null || printf '%s\n' "${GITHUB_WORKSPACE:-$PWD}"
}

cleanup_grpc_third_party_pollution() {
  local root="$1"
  git -C "$root" checkout -- sdk/zig-holons/third_party/grpc/third_party/zlib/ 2>/dev/null || true
  git -C "$root/sdk/zig-holons/third_party/grpc/third_party/zlib" checkout -- . 2>/dev/null || true
}

target_goos_goarch() {
  case "$1" in
    aarch64-apple-darwin) echo "darwin arm64" ;;
    x86_64-apple-darwin) echo "darwin amd64" ;;
    x86_64-unknown-linux-gnu|x86_64-unknown-linux-musl) echo "linux amd64" ;;
    aarch64-unknown-linux-gnu|aarch64-unknown-linux-musl) echo "linux arm64" ;;
    x86_64-windows-gnu|x86_64-pc-windows-msvc) echo "windows amd64" ;;
    *) echo "unsupported SDK prebuilt target: $1" >&2; return 2 ;;
  esac
}

target_exe_suffix() {
  case "$1" in
    x86_64-windows-gnu|x86_64-pc-windows-msvc) echo ".exe" ;;
    *) echo "" ;;
  esac
}

protoc_release_asset() {
  case "$1" in
    aarch64-apple-darwin) echo "osx-aarch_64" ;;
    x86_64-apple-darwin) echo "osx-x86_64" ;;
    x86_64-unknown-linux-gnu|x86_64-unknown-linux-musl) echo "linux-x86_64" ;;
    aarch64-unknown-linux-gnu|aarch64-unknown-linux-musl) echo "linux-aarch_64" ;;
    x86_64-windows-gnu|x86_64-pc-windows-msvc) echo "win64" ;;
    *) echo "unsupported SDK prebuilt target: $1" >&2; return 2 ;;
  esac
}

install_protoc_release() {
  local target="$1"
  local dest="$2"
  local version="${PROTOC_VERSION:-34.1}"
  local asset
  local tmp

  asset="$(protoc_release_asset "$target")"
  tmp="$(mktemp -d)"
  curl -fsSL "https://github.com/protocolbuffers/protobuf/releases/download/v${version}/protoc-${version}-${asset}.zip" -o "${tmp}/protoc.zip"
  unzip -q "${tmp}/protoc.zip" -d "${tmp}/protoc"
  mkdir -p "${dest}/bin" "${dest}/share/protoc"
  cp -R "${tmp}/protoc/include" "${dest}/share/protoc/"
  if [[ -f "${tmp}/protoc/bin/protoc.exe" ]]; then
    cp "${tmp}/protoc/bin/protoc.exe" "${dest}/bin/protoc.exe"
    chmod +x "${dest}/bin/protoc.exe"
  else
    cp "${tmp}/protoc/bin/protoc" "${dest}/bin/protoc"
    chmod +x "${dest}/bin/protoc"
  fi
  rm -rf "$tmp"
}

build_go_tool_for_target() {
  local repo_root="$1"
  local target="$2"
  local package="$3"
  local output="$4"
  local goos
  local goarch

  read -r goos goarch < <(target_goos_goarch "$target")
  mkdir -p "$(dirname "$output")"
  (
    cd "$repo_root/holons/grace-op"
    env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -mod=readonly -trimpath -ldflags="-s -w" -o "$output" "$package"
  )
}

build_adapter_family() {
  local repo_root="$1"
  local target="$2"
  local bin_dir="$3"
  shift 3
  local suffix
  local adapter
  local name

  suffix="$(target_exe_suffix "$target")"
  adapter="${bin_dir}/protoc-gen-op-adapter${suffix}"
  build_go_tool_for_target "$repo_root" "$target" ./cmd/protoc-gen-op-adapter "$adapter"
  chmod +x "$adapter"
  for name in "$@"; do
    cp "$adapter" "${bin_dir}/protoc-gen-${name}${suffix}"
    chmod +x "${bin_dir}/protoc-gen-${name}${suffix}"
  done
}

copy_grpc_sibling() {
  local stage="$1"
  local plugin_name="$2"
  local target="${SDK_TARGET:?SDK_TARGET is required}"
  local repo_root
  local suffix
  local source
  local dest

  repo_root="$(repo_root_or_pwd)"
  suffix="$(target_exe_suffix "$target")"
  source="${repo_root}/sdk/cpp-holons/.cpp-prebuilt/${target}/prefix/bin/${plugin_name}${suffix}"
  dest="${stage}/bin/${plugin_name}${suffix}"
  if [[ ! -x "$source" ]]; then
    echo "gRPC sibling plugin not found or not executable: ${source}" >&2
    return 1
  fi
  mkdir -p "${stage}/bin"
  cp "$source" "$dest"
  chmod +x "$dest"
}

install_grpc_java_plugin() {
  local stage="$1"
  local target="${SDK_TARGET:?SDK_TARGET is required}"
  local version="${GRPC_JAVA_PLUGIN_VERSION:-1.60.0}"
  local classifier
  local expected_sha
  local suffix
  local tmp
  local actual_sha

  case "$target" in
    aarch64-apple-darwin)
      classifier="osx-aarch_64"
      expected_sha="bb6c0c079998ee7080e66ea122dfb66a34a602482a7ed1760b30b7324fdf8ede"
      ;;
    *)
      echo "unsupported protoc-gen-grpc-java target: ${target}" >&2
      return 2
      ;;
  esac

  suffix="$(target_exe_suffix "$target")"
  tmp="$(mktemp)"
  curl -fsSL \
    "https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/${version}/protoc-gen-grpc-java-${version}-${classifier}.exe" \
    -o "$tmp"
  actual_sha="$(sha256_file "$tmp" | awk '{print $1}')"
  if [[ "$actual_sha" != "$expected_sha" ]]; then
    echo "protoc-gen-grpc-java sha256 mismatch: got ${actual_sha}, want ${expected_sha}" >&2
    rm -f "$tmp"
    return 1
  fi
  mkdir -p "${stage}/bin"
  cp "$tmp" "${stage}/bin/protoc-gen-grpc-java${suffix}"
  chmod +x "${stage}/bin/protoc-gen-grpc-java${suffix}"
  rm -f "$tmp"
}

archive_codegen_stage() {
  local stage="$1"
  local archive="$2"
  local name="$3"
  local namespace="$4"
  local dist_dir
  local entries=()
  local entry

  dist_dir="$(dirname "$archive")"
  mkdir -p "$dist_dir"
  for entry in bin include lib share vendor manifest.json; do
    if [[ -e "${stage}/${entry}" ]]; then
      entries+=("$entry")
    fi
  done
  if [[ ${#entries[@]} -eq 0 ]]; then
    echo "stage has no archive entries: ${stage}" >&2
    return 1
  fi
  tar -C "$stage" -czf "$archive" "${entries[@]}"

  if command -v syft >/dev/null 2>&1; then
    syft "dir:${stage}" -o "spdx-json=${archive}.spdx.json"
  else
    cat >"${archive}.spdx.json" <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "${name}",
  "documentNamespace": "${namespace}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: $(basename "$0")"]
  },
  "packages": []
}
EOF
  fi
  sha256_file "$archive" >"${archive}.sha256"
  sha256_file "${archive}.spdx.json" >"${archive}.spdx.json.sha256"
}
