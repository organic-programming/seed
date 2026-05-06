#!/usr/bin/env bash
set -euo pipefail

sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION:-1.80.0}"
jobs="${CPP_HOLONS_JOBS:-${ZIG_HOLONS_JOBS:-8}}"
zig_bin="${ZIG:-$(command -v zig || true)}"

if [[ -z "$zig_bin" ]]; then
  echo "zig is required to build cross-target C++ prebuilts" >&2
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

copy_first_executable() {
  local root="$1"
  local name="$2"
  local dest="$3"
  local found

  found="$(find -L "$root" -type f \( -name "$name" -o -name "${name}.exe" \) -perm -111 -print 2>/dev/null | head -n 1 || true)"
  if [[ -z "$found" && "$name" == "protoc" ]]; then
    found="$(find -L "$root" -type f -name 'protoc-*' -perm -111 -print 2>/dev/null | head -n 1 || true)"
  fi
  if [[ -z "$found" ]]; then
    found="$(find -L "$root" -type f \( -name "$name" -o -name "${name}.exe" \) -print 2>/dev/null | head -n 1 || true)"
  fi
  if [[ -z "$found" && "$name" == "protoc" ]]; then
    found="$(find -L "$root" -type f -name 'protoc-*' -print 2>/dev/null | head -n 1 || true)"
  fi
  if [[ -z "$found" ]]; then
    echo "could not find ${name} under ${root}" >&2
    return 1
  fi

  mkdir -p "$dest"
  cp "$found" "$dest/"
  chmod +x "$dest/$(basename "$found")" 2>/dev/null || true
}

repo_root="$(git rev-parse --show-toplevel)"
# shellcheck source=.github/scripts/lib-codegen-prebuilt.sh
source "${repo_root}/.github/scripts/lib-codegen-prebuilt.sh"
sdk_dir="${repo_root}/sdk/cpp-holons"
grpc_source="${GRPC_SOURCE_DIR:-${repo_root}/sdk/zig-holons/third_party/grpc}"
nlohmann_json_header="${sdk_dir}/third_party/nlohmann-json/single_include/nlohmann/json.hpp"
nlohmann_json_license="${sdk_dir}/third_party/nlohmann-json/LICENSE.MIT"
dist_dir="${repo_root}/dist/sdk-prebuilts/cpp/${sdk_target}"
work_dir="${sdk_dir}/.cpp-prebuilt/${sdk_target}"
toolchain_dir="${work_dir}/toolchain"
host_build="${work_dir}/build/host-tools"
host_tools="${work_dir}/host-tools"
grpc_build="${work_dir}/build/grpc"
prefix="${work_dir}/prefix"
stage="${work_dir}/stage/cpp-holons-v${sdk_version}-${sdk_target}"

if [[ ! -f "${grpc_source}/CMakeLists.txt" ]]; then
  echo "gRPC source checkout not found at ${grpc_source}" >&2
  echo "Run: git submodule update --init --recursive sdk/zig-holons/third_party/grpc" >&2
  exit 1
fi

if [[ ! -f "$nlohmann_json_header" ]]; then
  echo "nlohmann/json single-header vendored source not found at ${nlohmann_json_header}" >&2
  exit 1
fi

for required in \
  third_party/abseil-cpp/CMakeLists.txt \
  third_party/boringssl-with-bazel/CMakeLists.txt \
  third_party/cares/cares/CMakeLists.txt \
  third_party/protobuf/CMakeLists.txt \
  third_party/re2/CMakeLists.txt \
  third_party/zlib/CMakeLists.txt
do
  if [[ ! -f "${grpc_source}/${required}" ]]; then
    echo "gRPC dependency submodule missing: ${grpc_source}/${required}" >&2
    echo "Run: git -C ${grpc_source} submodule update --init --depth 1 third_party/abseil-cpp third_party/boringssl-with-bazel third_party/cares/cares third_party/protobuf third_party/re2 third_party/zlib" >&2
    exit 1
  fi
done

macos_framework_flag=""
if [[ "$sdk_target" == *apple-darwin ]]; then
  if ! command -v xcrun >/dev/null 2>&1; then
    echo "xcrun not on PATH; required for macOS framework headers" >&2
    exit 2
  fi
  macos_sdk_path="$(xcrun --show-sdk-path)"
  # See sdk/PREBUILTS.md "macOS toolchain workarounds (zigcxx + zig build)".
  macos_framework_flag="-F${macos_sdk_path}/System/Library/Frameworks -iframework ${macos_sdk_path}/System/Library/Frameworks -isystem ${macos_sdk_path}/usr/include"
fi

case "$sdk_target" in
  aarch64-apple-darwin)
    cmake_system="Darwin"
    cmake_processor="arm64"
    zig_target="aarch64-macos"
    extra_cflags="-mmacos-version-min=${MACOSX_DEPLOYMENT_TARGET:-14.0} ${macos_framework_flag}"
    ;;
  x86_64-apple-darwin)
    cmake_system="Darwin"
    cmake_processor="x86_64"
    zig_target="x86_64-macos"
    extra_cflags="-mmacos-version-min=${MACOSX_DEPLOYMENT_TARGET:-13.0} ${macos_framework_flag}"
    ;;
  x86_64-unknown-linux-gnu)
    cmake_system="Linux"
    cmake_processor="x86_64"
    zig_target="x86_64-linux-gnu"
    extra_cflags=""
    ;;
  aarch64-unknown-linux-gnu)
    cmake_system="Linux"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-gnu"
    extra_cflags=""
    ;;
  x86_64-unknown-linux-musl)
    cmake_system="Linux"
    cmake_processor="x86_64"
    zig_target="x86_64-linux-musl"
    extra_cflags=""
    ;;
  aarch64-unknown-linux-musl)
    cmake_system="Linux"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-musl"
    extra_cflags=""
    ;;
  x86_64-windows-gnu)
    cmake_system="Windows"
    cmake_processor="x86_64"
    zig_target="x86_64-windows-gnu"
    extra_cflags=""
    ;;
  *)
    echo "unsupported C++ SDK prebuilt target: ${sdk_target}" >&2
    exit 2
    ;;
esac

mkdir -p "$toolchain_dir" "$host_build" "$host_tools/bin" "$grpc_build" "$prefix" "$dist_dir"

cat >"${toolchain_dir}/zigcc" <<EOF
#!/usr/bin/env bash
exec "${zig_bin}" cc -target "${zig_target}" ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigcxx" <<EOF
#!/usr/bin/env bash
exec "${zig_bin}" c++ -target "${zig_target}" ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigar" <<EOF
#!/usr/bin/env bash
exec "${zig_bin}" ar "\$@"
EOF
cat >"${toolchain_dir}/zigranlib" <<EOF
#!/usr/bin/env bash
exec "${zig_bin}" ranlib "\$@"
EOF
cat >"${toolchain_dir}/zigrc" <<EOF
#!/usr/bin/env bash
args=()
input=""
output=""
while [[ \$# -gt 0 ]]; do
  case "\$1" in
    -D)
      shift
      args+=(/d "\$1")
      ;;
    -D*)
      args+=(/d "\${1#-D}")
      ;;
    -I)
      shift
      args+=(/i "\$1")
      ;;
    -I*)
      args+=(/i "\${1#-I}")
      ;;
    -i)
      shift
      input="\$1"
      ;;
    -o)
      shift
      output="\$1"
      ;;
    --)
      shift
      break
      ;;
    *)
      args+=("\$1")
      ;;
  esac
  shift
done

if [[ -z "\$input" && \$# -gt 0 ]]; then
  input="\$1"
  shift
fi
if [[ -z "\$output" && \$# -gt 0 ]]; then
  output="\$1"
  shift
fi

if [[ -z "\$input" || -z "\$output" ]]; then
  exec "${zig_bin}" rc "\${args[@]}" "\$@"
fi

exec "${zig_bin}" rc /:target "${cmake_processor}" "\${args[@]}" -- "\$input" "\$output"
EOF
chmod +x "${toolchain_dir}/zigcc" "${toolchain_dir}/zigcxx" "${toolchain_dir}/zigar" "${toolchain_dir}/zigranlib" "${toolchain_dir}/zigrc"

if [[ "$cmake_system" == Darwin ]]; then
  darwin_sdkroot="$(xcrun --sdk macosx --show-sdk-path)"
  darwin_cc="$(xcrun --sdk macosx --find clang)"
  darwin_cxx="$(xcrun --sdk macosx --find clang++)"
  darwin_ar="$(xcrun --sdk macosx --find ar)"
  darwin_ranlib="$(xcrun --sdk macosx --find ranlib)"
  cat >"${toolchain_dir}/${sdk_target}.cmake" <<EOF
set(CMAKE_SYSTEM_NAME Darwin)
set(CMAKE_SYSTEM_PROCESSOR ${cmake_processor})
set(CMAKE_C_COMPILER "${darwin_cc}")
set(CMAKE_CXX_COMPILER "${darwin_cxx}")
set(CMAKE_AR "${darwin_ar}")
set(CMAKE_RANLIB "${darwin_ranlib}")
set(CMAKE_OSX_SYSROOT "${darwin_sdkroot}")
set(CMAKE_OSX_ARCHITECTURES "${cmake_processor}")
set(CMAKE_OSX_DEPLOYMENT_TARGET "${MACOSX_DEPLOYMENT_TARGET:-13.0}")
set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)
set(CMAKE_CROSSCOMPILING TRUE)
set(CMAKE_FIND_ROOT_PATH "${prefix}")
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
EOF
else
  cat >"${toolchain_dir}/${sdk_target}.cmake" <<EOF
set(CMAKE_SYSTEM_NAME ${cmake_system})
set(CMAKE_SYSTEM_PROCESSOR ${cmake_processor})
set(CMAKE_C_COMPILER "${toolchain_dir}/zigcc")
set(CMAKE_CXX_COMPILER "${toolchain_dir}/zigcxx")
set(CMAKE_AR "${toolchain_dir}/zigar")
set(CMAKE_RANLIB "${toolchain_dir}/zigranlib")
set(CMAKE_RC_COMPILER "${toolchain_dir}/zigrc")
set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)
set(CMAKE_CROSSCOMPILING TRUE)
set(CMAKE_FIND_ROOT_PATH "${prefix}")
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
EOF
fi

common_grpc_flags=(
  -G Ninja
  -DCMAKE_BUILD_TYPE=Release
  -DBUILD_SHARED_LIBS=OFF
  -DgRPC_INSTALL=ON
  -DgRPC_BUILD_TESTS=OFF
  -DgRPC_BUILD_CODEGEN=ON
  -DgRPC_BUILD_GRPC_CPP_PLUGIN=ON
  -DgRPC_BUILD_GRPC_CSHARP_PLUGIN=ON
  -DgRPC_BUILD_GRPC_NODE_PLUGIN=ON
  -DgRPC_BUILD_GRPC_OBJECTIVE_C_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_PHP_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_PYTHON_PLUGIN=ON
  -DgRPC_BUILD_GRPC_RUBY_PLUGIN=ON
  -DgRPC_ABSL_PROVIDER=module
  -DgRPC_CARES_PROVIDER=module
  -DgRPC_PROTOBUF_PROVIDER=module
  -DgRPC_RE2_PROVIDER=module
  -DgRPC_SSL_PROVIDER=module
  -DgRPC_ZLIB_PROVIDER=module
  -DOPENSSL_NO_ASM=ON
  -Dprotobuf_BUILD_TESTS=OFF
  -Dprotobuf_BUILD_PROTOC_BINARIES=ON
  -Dprotobuf_BUILD_LIBPROTOC=ON
  -Dprotobuf_BUILD_LIBUPB=ON
  -DRE2_BUILD_TESTING=OFF
  -DCARES_BUILD_TOOLS=OFF
  -DZLIB_BUILD_EXAMPLES=OFF
)

if [[ ! -x "${host_tools}/bin/protoc" || ! -x "${host_tools}/bin/grpc_cpp_plugin" ]]; then
  cmake -S "$grpc_source" -B "$host_build" \
    "${common_grpc_flags[@]}" \
    -DCMAKE_INSTALL_PREFIX="${host_tools}"
  cmake --build "$host_build" --target protoc grpc_cpp_plugin --parallel "$jobs"
  rm -rf "${host_tools}/bin"
  mkdir -p "${host_tools}/bin"
  copy_first_executable "$host_build" protoc "${host_tools}/bin"
  copy_first_executable "$host_build" grpc_cpp_plugin "${host_tools}/bin"
fi

grpc_flags=(
  "${common_grpc_flags[@]}"
  -DCMAKE_TOOLCHAIN_FILE="${toolchain_dir}/${sdk_target}.cmake"
  -DCMAKE_INSTALL_PREFIX="${prefix}"
  -DCMAKE_PREFIX_PATH="${prefix}"
)

if [[ "$cmake_system" == Darwin ]]; then
  grpc_flags+=(
    -DCMAKE_OSX_ARCHITECTURES="${cmake_processor}"
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  )
fi

PATH="${host_tools}/bin:${PATH}" cmake -S "$grpc_source" -B "$grpc_build" "${grpc_flags[@]}"
PATH="${host_tools}/bin:${PATH}" cmake --build "$grpc_build" --target install --parallel "$jobs"

mkdir -p "${prefix}/bin"
if [[ ! -e "${prefix}/bin/protoc" && ! -e "${prefix}/bin/protoc.exe" ]]; then
  copy_first_executable "$grpc_build" protoc "${prefix}/bin"
fi
if [[ ! -e "${prefix}/bin/grpc_cpp_plugin" && ! -e "${prefix}/bin/grpc_cpp_plugin.exe" ]]; then
  copy_first_executable "$grpc_build" grpc_cpp_plugin "${prefix}/bin"
fi

if ! find "$prefix" -path '*/gRPCConfig.cmake' -print -quit | grep -q .; then
  echo "gRPC CMake package was not installed under ${prefix}" >&2
  exit 1
fi
if ! find "$prefix" \( -name 'protobuf-config.cmake' -o -name 'ProtobufConfig.cmake' \) -print -quit | grep -q .; then
  echo "Protobuf CMake package was not installed under ${prefix}" >&2
  exit 1
fi

rm -rf "${work_dir}/stage"
mkdir -p "$stage/include" "$stage/lib" "$stage/bin" "$stage/share" "$stage/debug"
cp -R "$prefix/include/." "$stage/include/"
cp -R "$prefix/lib/." "$stage/lib/"
if [[ -d "$prefix/bin" ]]; then
  cp -R "$prefix/bin/." "$stage/bin/"
fi
build_adapter_family "$repo_root" "$sdk_target" "$stage/bin" cpp
mkdir -p "$stage/include/nlohmann" "$stage/share/licenses/nlohmann-json"
cp "$nlohmann_json_header" "$stage/include/nlohmann/json.hpp"
if [[ -f "$nlohmann_json_license" ]]; then
  cp "$nlohmann_json_license" "$stage/share/licenses/nlohmann-json/LICENSE.MIT"
fi

grpc_commit="$(git -C "$grpc_source" rev-parse HEAD 2>/dev/null || echo unknown)"
{
  echo "sdk=cpp"
  echo "version=${sdk_version}"
  echo "target=${sdk_target}"
  echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "grpc_commit=${grpc_commit}"
  echo "nlohmann_json=3.11.3"
  echo "zig=$("$zig_bin" version)"
} >"$stage/share/prebuilt.env"

cat >"$stage/manifest.json" <<EOF
{
  "lang": "cpp",
  "version": "${sdk_version}",
  "target": "${sdk_target}",
  "codegen": {
    "plugins": [
      {"name": "cpp", "binary": "bin/protoc-gen-cpp$(target_exe_suffix "$sdk_target")", "out_subdir": "cpp"}
    ]
  }
}
EOF

find "$stage/lib" -type f \( -name '*.a' -o -name '*.lib' \) -print0 | while IFS= read -r -d '' archive; do
  strip_archive "$archive"
done

printf 'No separate debug sidecar files were emitted for %s.\n' "$sdk_target" >"$stage/debug/README.txt"
tar -C "$stage" -czf "${dist_dir}/cpp-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" debug

archive="${dist_dir}/cpp-holons-v${sdk_version}-${sdk_target}.tar.gz"
tar -C "$stage" -czf "$archive" include lib bin share manifest.json

if command -v syft >/dev/null 2>&1; then
  syft "dir:${stage}" -o "spdx-json=${archive}.spdx.json"
else
  cat >"${archive}.spdx.json" <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "cpp-holons-v${sdk_version}-${sdk_target}",
  "documentNamespace": "https://github.com/organic-programming/seed/sdk-prebuilts/cpp/${sdk_version}/${sdk_target}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: build-prebuilt-cpp.sh"]
  },
  "packages": []
}
EOF
fi

sha256_file "$archive" >"${archive}.sha256"
sha256_file "${archive}.spdx.json" >"${archive}.spdx.json.sha256"
sha256_file "${dist_dir}/cpp-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" >"${dist_dir}/cpp-holons-v${sdk_version}-${sdk_target}-debug.tar.gz.sha256"

echo "C++ SDK prebuilt staged:"
ls -lh "$dist_dir"
