#!/usr/bin/env bash
set -euo pipefail

sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION:-0.1.0}"
jobs="${ZIG_HOLONS_JOBS:-8}"
zig_bin="${ZIG:-$(command -v zig)}"

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
sdk_dir="${repo_root}/sdk/zig-holons"
dist_dir="${repo_root}/dist/sdk-prebuilts/zig/${sdk_target}"
work_dir="${sdk_dir}/.zig-prebuilt/${sdk_target}"
toolchain_dir="${work_dir}/toolchain"
grpc_build="${work_dir}/build/grpc"
protobuf_c_build="${work_dir}/build/protobuf-c"
prefix="${work_dir}/prefix"
sdk_out="${work_dir}/sdk-out"
stage="${work_dir}/stage/zig-holons-v${sdk_version}-${sdk_target}"

case "$sdk_target" in
  aarch64-apple-darwin)
    cmake_system="Darwin"
    cmake_processor="arm64"
    zig_target="aarch64-macos"
    extra_cflags="-mmacos-version-min=${MACOSX_DEPLOYMENT_TARGET:-14.0}"
    extra_ldflags="$extra_cflags"
    ;;
  x86_64-apple-darwin)
    cmake_system="Darwin"
    cmake_processor="x86_64"
    zig_target="x86_64-macos"
    extra_cflags="-mmacos-version-min=${MACOSX_DEPLOYMENT_TARGET:-13.0}"
    extra_ldflags="$extra_cflags"
    ;;
  x86_64-unknown-linux-gnu)
    cmake_system="Linux"
    cmake_processor="x86_64"
    zig_target="x86_64-linux-gnu"
    extra_cflags=""
    extra_ldflags=""
    ;;
  aarch64-unknown-linux-gnu)
    cmake_system="Linux"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-gnu"
    extra_cflags=""
    extra_ldflags=""
    ;;
  x86_64-unknown-linux-musl)
    cmake_system="Linux"
    cmake_processor="x86_64"
    zig_target="x86_64-linux-musl"
    extra_cflags=""
    extra_ldflags=""
    ;;
  aarch64-unknown-linux-musl)
    cmake_system="Linux"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-musl"
    extra_cflags=""
    extra_ldflags=""
    ;;
  x86_64-windows-gnu)
    cmake_system="Windows"
    cmake_processor="x86_64"
    zig_target="x86_64-windows-gnu"
    extra_cflags=""
    extra_ldflags=""
    ;;
  *)
    echo "unsupported Zig SDK prebuilt target: ${sdk_target}" >&2
    exit 2
    ;;
esac

mkdir -p "$toolchain_dir" "$grpc_build" "$protobuf_c_build" "$prefix" "$dist_dir"
cd "$sdk_dir"

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

grpc_flags=(
  -G Ninja
  -DCMAKE_TOOLCHAIN_FILE="${toolchain_dir}/${sdk_target}.cmake"
  -DCMAKE_BUILD_TYPE=Release
  -DCMAKE_INSTALL_PREFIX="${prefix}"
  -DBUILD_SHARED_LIBS=OFF
  -DgRPC_INSTALL=ON
  -DgRPC_BUILD_TESTS=OFF
  -DgRPC_BUILD_CODEGEN=OFF
  -DgRPC_BUILD_GRPC_CPP_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_CSHARP_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_NODE_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_OBJECTIVE_C_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_PHP_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_PYTHON_PLUGIN=OFF
  -DgRPC_BUILD_GRPC_RUBY_PLUGIN=OFF
  -DgRPC_ABSL_PROVIDER=module
  -DgRPC_CARES_PROVIDER=module
  -DgRPC_PROTOBUF_PROVIDER=module
  -DgRPC_RE2_PROVIDER=module
  -DgRPC_SSL_PROVIDER=module
  -DgRPC_ZLIB_PROVIDER=module
  -DOPENSSL_NO_ASM=ON
  -Dprotobuf_BUILD_TESTS=OFF
  -Dprotobuf_BUILD_PROTOC_BINARIES=OFF
  -Dprotobuf_BUILD_LIBPROTOC=OFF
  -Dprotobuf_BUILD_LIBUPB=OFF
  -DRE2_BUILD_TESTING=OFF
  -DCARES_BUILD_TOOLS=OFF
  -DZLIB_BUILD_EXAMPLES=OFF
)

if [[ "$cmake_system" == Darwin ]]; then
  grpc_flags+=(
    -DCMAKE_OSX_ARCHITECTURES="${cmake_processor}"
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  )
fi

cmake -S third_party/grpc -B "$grpc_build" "${grpc_flags[@]}"
cmake --build "$grpc_build" --target install --parallel "$jobs"

protobuf_c_flags=(
  -G Ninja
  -DCMAKE_TOOLCHAIN_FILE="${toolchain_dir}/${sdk_target}.cmake"
  -DCMAKE_BUILD_TYPE=Release
  -DCMAKE_INSTALL_PREFIX="${prefix}"
  -DCMAKE_PREFIX_PATH="${prefix}"
  -DBUILD_SHARED_LIBS=OFF
  -DBUILD_PROTOC=OFF
  -DBUILD_TESTS=OFF
)

if [[ "$cmake_system" == Darwin ]]; then
  protobuf_c_flags+=(
    -DCMAKE_OSX_ARCHITECTURES="${cmake_processor}"
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
  )
fi

cmake -S third_party/protobuf-c/build-cmake -B "$protobuf_c_build" "${protobuf_c_flags[@]}"
cmake --build "$protobuf_c_build" --target install --parallel "$jobs"

rm -rf "$sdk_out"
zig_build_env=(OP_SDK_ZIG_PATH="$prefix")
if [[ "$cmake_system" == Darwin ]]; then
  zig_build_env+=(SDKROOT="$darwin_sdkroot")
fi
env "${zig_build_env[@]}" "$zig_bin" build \
  -Dtarget="$zig_target" \
  -Doptimize=ReleaseFast \
  --prefix "$sdk_out"

rm -rf "${work_dir}/stage"
mkdir -p "$stage/include" "$stage/lib" "$stage/share" "$stage/debug"
cp -R "$prefix/include/." "$stage/include/"
cp -R include/. "$stage/include/"
cp -R "$prefix/lib/." "$stage/lib/"
if [[ -f "$sdk_out/lib/libholons_zig.a" ]]; then
  cp "$sdk_out/lib/libholons_zig.a" "$stage/lib/"
elif [[ -f "$sdk_out/lib/holons_zig.lib" ]]; then
  cp "$sdk_out/lib/holons_zig.lib" "$stage/lib/"
else
  echo "could not find installed holons_zig static library under $sdk_out/lib" >&2
  find "$sdk_out/lib" -maxdepth 1 -type f -print >&2 || true
  exit 1
fi

{
  echo "sdk=zig"
  echo "version=${sdk_version}"
  echo "target=${sdk_target}"
  echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "zig=$("$zig_bin" version)"
} >"$stage/share/prebuilt.env"

find "$stage/lib" -type f \( -name '*.a' -o -name '*.lib' \) -print0 | while IFS= read -r -d '' archive; do
  strip_archive "$archive"
done

printf 'No separate debug sidecar files were emitted for %s.\n' "$sdk_target" >"$stage/debug/README.txt"
tar -C "$stage" -czf "${dist_dir}/zig-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" debug

archive="${dist_dir}/zig-holons-v${sdk_version}-${sdk_target}.tar.gz"
tar -C "$stage" -czf "$archive" include lib share

if command -v syft >/dev/null 2>&1; then
  syft "dir:${stage}" -o "spdx-json=${archive}.spdx.json"
else
  cat >"${archive}.spdx.json" <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "zig-holons-v${sdk_version}-${sdk_target}",
  "documentNamespace": "https://github.com/organic-programming/seed/sdk-prebuilts/zig/${sdk_version}/${sdk_target}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: build-prebuilt-zig.sh"]
  },
  "packages": []
}
EOF
fi

sha256_file "$archive" >"${archive}.sha256"
sha256_file "${archive}.spdx.json" >"${archive}.spdx.json.sha256"
sha256_file "${dist_dir}/zig-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" >"${dist_dir}/zig-holons-v${sdk_version}-${sdk_target}-debug.tar.gz.sha256"

echo "Zig SDK prebuilt staged:"
ls -lh "$dist_dir"
