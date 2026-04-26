#!/usr/bin/env bash
set -euo pipefail

target="${1:?usage: tools/cross_smoke.sh <target>}"
jobs="${ZIG_HOLONS_JOBS:-8}"
zig_bin="${ZIG:-$(command -v zig)}"
cc_driver="zig"

sdk_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$sdk_dir"

case "$target" in
  aarch64-linux-musl)
    cmake_system="Linux"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-musl"
    extra_cflags=""
    extra_ldflags=""
    ;;
  x86_64-linux-musl)
    cmake_system="Linux"
    cmake_processor="x86_64"
    zig_target="x86_64-linux-musl"
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
  aarch64-ios)
    cmake_system="iOS"
    cmake_processor="arm64"
    zig_target="aarch64-ios"
    ios_sdk="$(xcrun --sdk iphoneos --show-sdk-path)"
    cc_driver="xcrun-ios"
    extra_cflags="-isysroot ${ios_sdk} -idirafter ${ios_sdk}/usr/include -L${ios_sdk}/usr/lib -F${ios_sdk}/System/Library/Frameworks -Wno-elaborated-enum-base -miphoneos-version-min=${IOS_DEPLOYMENT_TARGET:-17.0}"
    extra_ldflags="$extra_cflags"
    ;;
  aarch64-linux-android)
    cmake_system="Android"
    cmake_processor="aarch64"
    zig_target="aarch64-linux-android"
    android_api="${ANDROID_API_LEVEL:-26}"
    android_ndk="${ANDROID_NDK_HOME:-${ANDROID_NDK_ROOT:-${HOME}/Library/Android/sdk/ndk/28.0.12433566}}"
    if [[ ! -d "$android_ndk" ]]; then
      echo "Android NDK not found; set ANDROID_NDK_HOME or ANDROID_NDK_ROOT" >&2
      exit 1
    fi
    case "$(uname -m)" in
      arm64|aarch64) ndk_host="darwin-arm64" ;;
      x86_64|amd64) ndk_host="darwin-x86_64" ;;
      *) echo "unsupported Android NDK host architecture: $(uname -m)" >&2; exit 1 ;;
    esac
    if [[ ! -d "${android_ndk}/toolchains/llvm/prebuilt/${ndk_host}" && "$ndk_host" == "darwin-arm64" ]]; then
      ndk_host="darwin-x86_64"
    fi
    android_toolchain="${android_ndk}/toolchains/llvm/prebuilt/${ndk_host}/bin"
    if [[ ! -x "${android_toolchain}/aarch64-linux-android${android_api}-clang" ]]; then
      echo "Android NDK clang not found under ${android_toolchain}" >&2
      exit 1
    fi
    cc_driver="ndk-android"
    extra_cflags=""
    extra_ldflags=""
    ;;
  *)
    echo "unsupported target: $target" >&2
    exit 2
    ;;
esac

root=".zig-cross/${target}"
toolchain_dir="${root}/toolchain"
grpc_build="${root}/build/grpc"
protobuf_c_build="${root}/build/protobuf-c"
prefix="${root}/out"
mkdir -p "$toolchain_dir" "$grpc_build" "$protobuf_c_build" "$prefix"

if [[ "$cc_driver" == xcrun-ios ]]; then
cat >"${toolchain_dir}/zigcc" <<EOF
#!/usr/bin/env bash
exec xcrun --sdk iphoneos clang -arch arm64 ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigcxx" <<EOF
#!/usr/bin/env bash
exec xcrun --sdk iphoneos clang++ -arch arm64 ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigar" <<'EOF'
#!/usr/bin/env bash
exec xcrun --sdk iphoneos ar "$@"
EOF
cat >"${toolchain_dir}/zigranlib" <<'EOF'
#!/usr/bin/env bash
exec xcrun --sdk iphoneos ranlib "$@"
EOF
elif [[ "$cc_driver" == ndk-android ]]; then
cat >"${toolchain_dir}/zigcc" <<EOF
#!/usr/bin/env bash
exec "${android_toolchain}/aarch64-linux-android${android_api}-clang" ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigcxx" <<EOF
#!/usr/bin/env bash
exec "${android_toolchain}/aarch64-linux-android${android_api}-clang++" ${extra_cflags} "\$@"
EOF
cat >"${toolchain_dir}/zigar" <<EOF
#!/usr/bin/env bash
exec "${android_toolchain}/llvm-ar" "\$@"
EOF
cat >"${toolchain_dir}/zigranlib" <<EOF
#!/usr/bin/env bash
exec "${android_toolchain}/llvm-ranlib" "\$@"
EOF
else
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
fi
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

cat >"${toolchain_dir}/${target}.cmake" <<EOF
set(CMAKE_SYSTEM_NAME ${cmake_system})
set(CMAKE_SYSTEM_PROCESSOR ${cmake_processor})
set(CMAKE_C_COMPILER "${sdk_dir}/${toolchain_dir}/zigcc")
set(CMAKE_CXX_COMPILER "${sdk_dir}/${toolchain_dir}/zigcxx")
set(CMAKE_AR "${sdk_dir}/${toolchain_dir}/zigar")
set(CMAKE_RANLIB "${sdk_dir}/${toolchain_dir}/zigranlib")
set(CMAKE_RC_COMPILER "${sdk_dir}/${toolchain_dir}/zigrc")
set(CMAKE_TRY_COMPILE_TARGET_TYPE STATIC_LIBRARY)
set(CMAKE_CROSSCOMPILING TRUE)
set(CMAKE_FIND_ROOT_PATH "${sdk_dir}/${prefix}")
set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)
set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)
set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)
EOF

grpc_common_flags=(
  -G Ninja
  -DCMAKE_TOOLCHAIN_FILE="${sdk_dir}/${toolchain_dir}/${target}.cmake"
  -DCMAKE_BUILD_TYPE=Release
  -DCMAKE_INSTALL_PREFIX="${sdk_dir}/${prefix}"
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

if [[ "$target" == aarch64-ios ]]; then
  grpc_common_flags+=(
    -DCMAKE_OSX_SYSROOT="${ios_sdk}"
    -DCMAKE_OSX_ARCHITECTURES=arm64
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${IOS_DEPLOYMENT_TARGET:-17.0}"
  )
elif [[ "$target" == aarch64-linux-android ]]; then
  grpc_common_flags+=(
    -DCMAKE_ANDROID_NDK="${android_ndk}"
    -DCMAKE_SYSTEM_VERSION="${android_api}"
  )
fi

cmake -S third_party/grpc -B "$grpc_build" "${grpc_common_flags[@]}"
if [[ "$target" == aarch64-ios ]]; then
  cmake --build "$grpc_build" --target grpc_unsecure --parallel "$jobs"
  mkdir -p "$prefix/include" "$prefix/lib/pkgconfig"
  rsync -a third_party/grpc/include/ "$prefix/include/"
  rsync -a third_party/grpc/third_party/protobuf/src/google/ "$prefix/include/google/"
  find "$grpc_build" -name '*.a' -exec cp {} "$prefix/lib/" \;
  find "$grpc_build" -name '*.pc' -exec cp {} "$prefix/lib/pkgconfig/" \;
else
  cmake --build "$grpc_build" --target install --parallel "$jobs"
fi

protobuf_c_flags=(
  -G Ninja
  -DCMAKE_TOOLCHAIN_FILE="${sdk_dir}/${toolchain_dir}/${target}.cmake"
  -DCMAKE_BUILD_TYPE=Release
  -DCMAKE_INSTALL_PREFIX="${sdk_dir}/${prefix}"
  -DCMAKE_PREFIX_PATH="${sdk_dir}/${prefix}"
  -DBUILD_SHARED_LIBS=OFF
  -DBUILD_PROTOC=OFF
  -DBUILD_TESTS=OFF
)

if [[ "$target" == aarch64-ios ]]; then
  protobuf_c_flags+=(
    -DCMAKE_OSX_SYSROOT="${ios_sdk}"
    -DCMAKE_OSX_ARCHITECTURES=arm64
    -DCMAKE_OSX_DEPLOYMENT_TARGET="${IOS_DEPLOYMENT_TARGET:-17.0}"
    -DProtobuf_INCLUDE_DIR="${sdk_dir}/${prefix}/include"
    -DProtobuf_LIBRARY="${sdk_dir}/${prefix}/lib/libprotobuf.a"
  )
elif [[ "$target" == aarch64-linux-android ]]; then
  protobuf_c_flags+=(
    -DCMAKE_ANDROID_NDK="${android_ndk}"
    -DCMAKE_SYSTEM_VERSION="${android_api}"
    -DProtobuf_INCLUDE_DIR="${sdk_dir}/${prefix}/include"
    -DProtobuf_LIBRARY="${sdk_dir}/${prefix}/lib/libprotobuf.a"
  )
fi

cmake -S third_party/protobuf-c/build-cmake -B "$protobuf_c_build" "${protobuf_c_flags[@]}"
cmake --build "$protobuf_c_build" --target install --parallel "$jobs"

cat >"${root}/native_grpc_smoke.c" <<'EOF'
#include <grpc/grpc.h>
#include <protobuf-c/protobuf-c.h>

int main(void) {
  grpc_init();
  grpc_shutdown();
  return protobuf_c_version_number() >= 1005002 ? 0 : 1;
}
EOF

export PKG_CONFIG_LIBDIR="${sdk_dir}/${prefix}/lib/pkgconfig:${sdk_dir}/${prefix}/share/pkgconfig"
libs="$(pkg-config --libs --static grpc_unsecure) -lprotobuf-c"
if [[ "$target" == x86_64-windows-gnu ]]; then
  libs="${libs//-lz/-lzlibstatic}"
fi
out="${root}/native-grpc-smoke"
case "$target" in
  x86_64-windows-gnu) out="${out}.exe" ;;
esac

if [[ "$target" == aarch64-ios ]]; then
  "${sdk_dir}/${toolchain_dir}/zigcxx" \
    "${root}/native_grpc_smoke.c" \
    -I"${prefix}/include" -L"${prefix}/lib" \
    $libs -framework CoreFoundation -framework CFNetwork \
    -o "$out"
elif [[ "$target" == aarch64-linux-android ]]; then
  "${sdk_dir}/${toolchain_dir}/zigcxx" \
    "${root}/native_grpc_smoke.c" \
    -I"${prefix}/include" -L"${prefix}/lib" \
    $libs \
    -o "$out"
else
  "$zig_bin" cc -target "$zig_target" ${extra_cflags} ${extra_ldflags} \
    "${root}/native_grpc_smoke.c" \
    -I"${prefix}/include" -L"${prefix}/lib" \
    $libs -lc++ \
    -o "$out"
fi

file "$out"
