#!/usr/bin/env bash
set -euo pipefail

sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION:-1.80.0}"
jobs="${C_HOLONS_JOBS:-${CPP_HOLONS_JOBS:-${ZIG_HOLONS_JOBS:-8}}}"
zig_bin="${ZIG:-$(command -v zig || true)}"

if [[ -z "$zig_bin" ]]; then
  echo "zig is required to build cross-target C prebuilts" >&2
  exit 127
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

strip_archive() {
  local archive="$1"
  local host

  if ! command -v strip >/dev/null 2>&1; then
    return 0
  fi

  host="$(uname -s)"
  case "${host}:${sdk_target}" in
    Darwin:*apple-darwin|Linux:*linux-*|Linux:*windows-gnu)
      ;;
    *)
      return 0
      ;;
  esac

  strip -S "$archive" >/dev/null 2>&1 || true
}

repo_root="$(git rev-parse --show-toplevel)"
# shellcheck source=.github/scripts/lib-codegen-prebuilt.sh
source "${repo_root}/.github/scripts/lib-codegen-prebuilt.sh"
trap 'cleanup_grpc_third_party_pollution "$repo_root"' EXIT
c_sdk_dir="${repo_root}/sdk/c-holons"
cpp_sdk_dir="${repo_root}/sdk/cpp-holons"
protobuf_c_source="${PROTOBUF_C_SOURCE_DIR:-${repo_root}/sdk/zig-holons/third_party/protobuf-c}"
grpc_source="${GRPC_SOURCE_DIR:-${repo_root}/sdk/zig-holons/third_party/grpc}"
nlohmann_json_header="${cpp_sdk_dir}/third_party/nlohmann-json/single_include/nlohmann/json.hpp"
nlohmann_json_license="${cpp_sdk_dir}/third_party/nlohmann-json/LICENSE.MIT"
cpp_prefix="${cpp_sdk_dir}/.cpp-prebuilt/${sdk_target}/prefix"
cpp_toolchain_dir="${cpp_sdk_dir}/.cpp-prebuilt/${sdk_target}/toolchain"
dist_dir="${repo_root}/dist/sdk-prebuilts/c/${sdk_target}"
work_dir="${c_sdk_dir}/.c-prebuilt/${sdk_target}"
protobuf_c_build="${work_dir}/build/protobuf-c"
protobuf_c_prefix="${work_dir}/protobuf-c-prefix"
stage="${work_dir}/stage/c-holons-v${sdk_version}-${sdk_target}"

if [[ ! -f "$nlohmann_json_header" ]]; then
  echo "nlohmann/json single-header source not found at ${nlohmann_json_header}" >&2
  exit 1
fi

if [[ ! -f "${protobuf_c_source}/build-cmake/CMakeLists.txt" ]]; then
  echo "protobuf-c source checkout not found at ${protobuf_c_source}" >&2
  echo "Run: git submodule update --init sdk/zig-holons/third_party/protobuf-c" >&2
  exit 1
fi

if [[ ! -f "${cpp_prefix}/lib/cmake/grpc/gRPCConfig.cmake" ||
      ! -x "${cpp_prefix}/bin/protoc" ||
      ! -x "${cpp_prefix}/bin/protoc-gen-upb" ||
      ! -x "${cpp_prefix}/bin/protoc-gen-upbdefs" ||
      ! -x "${cpp_prefix}/bin/protoc-gen-upb_minitable" ]]; then
  GRPC_SOURCE_DIR="$grpc_source" SDK_TARGET="$sdk_target" SDK_VERSION="$sdk_version" CPP_HOLONS_JOBS="$jobs" \
    "${repo_root}/.github/scripts/build-prebuilt-cpp.sh"
fi

case "$sdk_target" in
  aarch64-apple-darwin|x86_64-apple-darwin)
    os_family="Darwin"
    ;;
  x86_64-unknown-linux-gnu|aarch64-unknown-linux-gnu|x86_64-unknown-linux-musl|aarch64-unknown-linux-musl)
    os_family="Linux"
    ;;
  x86_64-windows-gnu)
    os_family="Windows"
    ;;
  *)
    echo "unsupported C SDK prebuilt target: ${sdk_target}" >&2
    exit 2
    ;;
esac

mkdir -p "$protobuf_c_build" "$protobuf_c_prefix" "$dist_dir"

protobuf_c_flags=(
  -G Ninja
  -DCMAKE_TOOLCHAIN_FILE="${cpp_toolchain_dir}/${sdk_target}.cmake"
  -DCMAKE_BUILD_TYPE=Release
  -DCMAKE_INSTALL_PREFIX="${protobuf_c_prefix}"
  -DCMAKE_PREFIX_PATH="${cpp_prefix}"
  -DBUILD_SHARED_LIBS=OFF
  -DBUILD_PROTOC=ON
  -DBUILD_TESTS=OFF
)

if [[ "$os_family" == Darwin ]]; then
  case "$sdk_target" in
    aarch64-apple-darwin) cmake_arch="arm64" ;;
    x86_64-apple-darwin) cmake_arch="x86_64" ;;
  esac
  protobuf_c_flags+=(
    -DCMAKE_OSX_ARCHITECTURES="${cmake_arch}"
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  )
fi

PATH="${cpp_prefix}/bin:${PATH}" cmake -S "${protobuf_c_source}/build-cmake" -B "$protobuf_c_build" "${protobuf_c_flags[@]}"
PATH="${cpp_prefix}/bin:${PATH}" cmake --build "$protobuf_c_build" --target install --parallel "$jobs"

rm -rf "${work_dir}/stage"
mkdir -p "$stage/include" "$stage/lib" "$stage/bin" "$stage/share" "$stage/debug"
cp -R "$cpp_prefix/include/." "$stage/include/"
cp -R "$cpp_prefix/lib/." "$stage/lib/"
if [[ -d "$cpp_prefix/bin" ]]; then
  cp -R "$cpp_prefix/bin/." "$stage/bin/"
fi
if [[ -d "$protobuf_c_prefix/include" ]]; then
  cp -R "$protobuf_c_prefix/include/." "$stage/include/"
fi
if [[ -d "$protobuf_c_prefix/lib" ]]; then
  cp -R "$protobuf_c_prefix/lib/." "$stage/lib/"
fi
if [[ -d "$protobuf_c_prefix/bin" ]]; then
  cp -R "$protobuf_c_prefix/bin/." "$stage/bin/"
fi
if [[ -x "${stage}/bin/protoc-gen-c" && ! -e "${stage}/bin/protoc-c" ]]; then
  cp "${stage}/bin/protoc-gen-c" "${stage}/bin/protoc-c"
elif [[ -x "${stage}/bin/protoc-gen-c.exe" && ! -e "${stage}/bin/protoc-c.exe" ]]; then
  cp "${stage}/bin/protoc-gen-c.exe" "${stage}/bin/protoc-c.exe"
fi
mkdir -p "$stage/include/nlohmann" "$stage/share/licenses/nlohmann-json"
cp "$nlohmann_json_header" "$stage/include/nlohmann/json.hpp"
if [[ -f "$nlohmann_json_license" ]]; then
  cp "$nlohmann_json_license" "$stage/share/licenses/nlohmann-json/LICENSE.MIT"
fi

if ! find "$stage/lib" \( -name 'libprotobuf-c.a' -o -name 'protobuf-c.lib' \) -print -quit | grep -q .; then
  echo "protobuf-c static library was not installed under ${stage}/lib" >&2
  exit 1
fi
if [[ ! -x "${stage}/bin/protoc" && ! -x "${stage}/bin/protoc.exe" ]]; then
  echo "protoc was not staged under ${stage}/bin" >&2
  exit 1
fi
if ! find "$stage/bin" \( -name 'protoc-c' -o -name 'protoc-c.exe' \) -print -quit | grep -q .; then
  echo "protoc-c was not staged under ${stage}/bin" >&2
  exit 1
fi

if [[ -f "${grpc_source}/CMakeLists.txt" ]]; then
  grpc_commit="$(git -C "$grpc_source" rev-parse HEAD 2>/dev/null || echo unknown)"
else
  grpc_commit="$(git -C "$repo_root" rev-parse "HEAD:sdk/zig-holons/third_party/grpc" 2>/dev/null || echo unknown)"
fi
protobuf_c_commit="$(git -C "$protobuf_c_source" rev-parse HEAD 2>/dev/null || echo unknown)"
{
  echo "sdk=c"
  echo "version=${sdk_version}"
  echo "target=${sdk_target}"
  echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "grpc_commit=${grpc_commit}"
  echo "protobuf_c_commit=${protobuf_c_commit}"
  echo "nlohmann_json=3.11.3"
  echo "zig=$("$zig_bin" version)"
} >"$stage/share/prebuilt.env"

cat >"$stage/manifest.json" <<EOF
{
  "lang": "c",
  "version": "${sdk_version}",
  "target": "${sdk_target}",
  "codegen": {
    "plugins": [
      {"name": "c", "binary": "bin/protoc-gen-upb$(target_exe_suffix "$sdk_target")", "out_subdir": "c"},
      {"name": "c-upbdefs", "binary": "bin/protoc-gen-upbdefs$(target_exe_suffix "$sdk_target")", "out_subdir": "c"},
      {"name": "c-upb-minitable", "binary": "bin/protoc-gen-upb_minitable$(target_exe_suffix "$sdk_target")", "out_subdir": "c"}
    ]
  }
}
EOF

find "$stage/lib" -type f \( -name '*.a' -o -name '*.lib' \) -print0 | while IFS= read -r -d '' archive_file; do
  strip_archive "$archive_file"
done

printf 'No separate debug sidecar files were emitted for %s.\n' "$sdk_target" >"$stage/debug/README.txt"
tar -C "$stage" -czf "${dist_dir}/c-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" debug

archive="${dist_dir}/c-holons-v${sdk_version}-${sdk_target}.tar.gz"
tar -C "$stage" -czf "$archive" include lib bin share manifest.json

if command -v syft >/dev/null 2>&1; then
  syft "dir:${stage}" -o "spdx-json=${archive}.spdx.json"
else
  cat >"${archive}.spdx.json" <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "c-holons-v${sdk_version}-${sdk_target}",
  "documentNamespace": "https://github.com/organic-programming/seed/sdk-prebuilts/c/${sdk_version}/${sdk_target}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: build-prebuilt-c.sh"]
  },
  "packages": []
}
EOF
fi

sha256_file "$archive" >"${archive}.sha256"
sha256_file "${archive}.spdx.json" >"${archive}.spdx.json.sha256"
sha256_file "${dist_dir}/c-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" >"${dist_dir}/c-holons-v${sdk_version}-${sdk_target}-debug.tar.gz.sha256"

echo "C SDK prebuilt staged:"
ls -lh "$dist_dir"
