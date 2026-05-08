#!/usr/bin/env bash
set -euo pipefail

export SDK_LANG=kotlin
export SDK_VERSION="${SDK_VERSION:-0.1.0}"
export PROTOC_VERSION="${PROTOC_VERSION:-31.1}"

sdk_lang="${SDK_LANG}"
sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION}"

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

install_protoc_release "$sdk_target" "$stage"
build_adapter_family "$repo_root" "$sdk_target" "$stage/bin" \
  kotlin-java kotlin kotlin-grpc
install_grpc_java_plugin "$stage"
install_grpc_kotlin_plugin "$stage"

test -x "${stage}/bin/protoc-gen-grpc-java${suffix}"

cat >"${stage}/manifest.json" <<EOF
{
  "lang": "${sdk_lang}",
  "version": "${sdk_version}",
  "target": "${sdk_target}",
  "codegen": {
    "plugins": [
      {"name": "kotlin-java", "binary": "bin/protoc-gen-kotlin-java${suffix}", "out_subdir": "kotlin"},
      {"name": "kotlin-java-grpc", "binary": "bin/protoc-gen-grpc-java${suffix}", "out_subdir": "kotlin"},
      {"name": "kotlin", "binary": "bin/protoc-gen-kotlin${suffix}", "out_subdir": "kotlin"},
      {"name": "kotlin-grpc", "binary": "bin/protoc-gen-kotlin-grpc${suffix}", "out_subdir": "kotlin"}
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
