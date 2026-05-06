#!/usr/bin/env bash
set -euo pipefail

sdk_lang="${SDK_LANG:?SDK_LANG is required}"
sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION:-0.1.0}"

script_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
# shellcheck source=.github/scripts/lib-codegen-prebuilt.sh
source "${script_dir}/lib-codegen-prebuilt.sh"

repo_root="$(repo_root_or_pwd)"
dist_dir="${repo_root}/dist/sdk-prebuilts/${sdk_lang}/${sdk_target}"
work_dir="${repo_root}/sdk/${sdk_lang}-holons/.codegen-prebuilt/${sdk_target}"
stage="${work_dir}/stage/${sdk_lang}-holons-v${sdk_version}-${sdk_target}"
suffix="$(target_exe_suffix "$sdk_target")"

rm -rf "$stage"
mkdir -p "$stage/bin" "$stage/share" "$dist_dir"

case "$sdk_lang" in
  go)
    read -r goos goarch < <(target_goos_goarch "$sdk_target")
    (
      cd "$repo_root"
      env GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOBIN="$stage/bin" \
        go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
      env GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOBIN="$stage/bin" \
        go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
    )
    plugins=$(cat <<EOF
      {"name": "go", "binary": "bin/protoc-gen-go${suffix}", "out_subdir": "go"},
      {"name": "go-grpc", "binary": "bin/protoc-gen-go-grpc${suffix}", "out_subdir": "go"}
EOF
)
    ;;
  rust)
    cargo install --locked --root "$stage" protoc-gen-prost@0.5.0 protoc-gen-tonic@0.5.0
    plugins=$(cat <<EOF
      {"name": "rust", "binary": "bin/protoc-gen-prost", "out_subdir": "rust"},
      {"name": "rust-tonic", "binary": "bin/protoc-gen-tonic", "out_subdir": "rust"}
EOF
)
    ;;
  dart)
    dart pub global activate protoc_plugin 25.0.0
    dart_snapshot="$(find "${PUB_CACHE:-$HOME/.pub-cache}/global_packages/protoc_plugin/bin" -maxdepth 1 -name 'protoc_plugin.dart-*.snapshot' | sort | tail -n 1)"
    if [[ -z "$dart_snapshot" || ! -f "$dart_snapshot" ]]; then
      echo "protoc_plugin snapshot not found after pub global activate" >&2
      exit 1
    fi
    mkdir -p "${stage}/share/dart"
    cp "$dart_snapshot" "${stage}/share/dart/protoc_plugin.snapshot"
    cat >"${stage}/bin/protoc-gen-dart" <<'EOF'
#!/usr/bin/env sh
set -eu
bin_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
exec dart "${bin_dir}/../share/dart/protoc_plugin.snapshot" "$@"
EOF
    chmod +x "${stage}/bin/protoc-gen-dart"
    plugins=$(cat <<EOF
      {"name": "dart", "binary": "bin/protoc-gen-dart", "out_subdir": "dart"}
EOF
)
    ;;
  swift)
    swift_version="${SWIFT_PROTOBUF_VERSION:-1.33.0}"
    swift_src="${work_dir}/swift-protobuf"
    if [[ ! -d "$swift_src/.git" ]]; then
      rm -rf "$swift_src"
      git clone --depth 1 --recurse-submodules --shallow-submodules --branch "$swift_version" https://github.com/apple/swift-protobuf.git "$swift_src"
    else
      git -C "$swift_src" fetch --depth 1 origin "refs/tags/${swift_version}:refs/tags/${swift_version}"
      git -C "$swift_src" checkout -f "$swift_version"
      git -C "$swift_src" submodule update --init --recursive --depth 1
    fi
    swift build --package-path "$swift_src" -c release --product protoc-gen-swift
    cp "$swift_src/.build/release/protoc-gen-swift" "${stage}/bin/protoc-gen-swift"
    chmod +x "${stage}/bin/protoc-gen-swift"
    plugins=$(cat <<EOF
      {"name": "swift", "binary": "bin/protoc-gen-swift", "out_subdir": "swift"}
EOF
)
    ;;
  js-web)
    build_go_tool_for_target "$repo_root" "$sdk_target" ./cmd/protoc-gen-op-noop "${stage}/bin/protoc-gen-js-web${suffix}"
    plugins=$(cat <<EOF
      {"name": "js-web", "binary": "bin/protoc-gen-js-web${suffix}", "out_subdir": "js-web"}
EOF
)
    ;;
  java|python|csharp|kotlin|js)
    install_protoc_release "$sdk_target" "$stage"
    build_adapter_family "$repo_root" "$sdk_target" "$stage/bin" "$sdk_lang"
    if [[ "$sdk_lang" == "python" ]]; then
      copy_grpc_sibling "$stage" grpc_python_plugin
    fi
    plugins=$(cat <<EOF
      {"name": "${sdk_lang}", "binary": "bin/protoc-gen-${sdk_lang}${suffix}", "out_subdir": "${sdk_lang}"}
EOF
)
    ;;
  *)
    echo "unsupported light codegen SDK: ${sdk_lang}" >&2
    exit 2
    ;;
esac

cat >"${stage}/manifest.json" <<EOF
{
  "lang": "${sdk_lang}",
  "version": "${sdk_version}",
  "target": "${sdk_target}",
  "codegen": {
    "plugins": [
${plugins}
    ]
  }
}
EOF

{
  echo "sdk=${sdk_lang}"
  echo "version=${sdk_version}"
  echo "target=${sdk_target}"
  echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"${stage}/share/prebuilt.env"

archive="${dist_dir}/${sdk_lang}-holons-v${sdk_version}-${sdk_target}.tar.gz"
archive_codegen_stage \
  "$stage" \
  "$archive" \
  "${sdk_lang}-holons-v${sdk_version}-${sdk_target}" \
  "https://github.com/organic-programming/seed/sdk-prebuilts/${sdk_lang}/${sdk_version}/${sdk_target}"

echo "${sdk_lang} codegen distribution staged:"
ls -lh "$dist_dir"
