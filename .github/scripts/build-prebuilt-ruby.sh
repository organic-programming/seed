#!/usr/bin/env bash
set -euo pipefail

sdk_target="${SDK_TARGET:?SDK_TARGET is required}"
sdk_version="${SDK_VERSION:-1.58.3}"
jobs="${RUBY_HOLONS_JOBS:-4}"
ruby_bin="${RUBY:-$(command -v ruby || true)}"
bundle_bin="${BUNDLE:-$(command -v bundle || true)}"

if [[ -z "$ruby_bin" || -z "$bundle_bin" ]]; then
  echo "ruby and bundle are required to build Ruby prebuilts" >&2
  exit 127
fi

if ! "$ruby_bin" -e 'v = Gem::Version.new(RUBY_VERSION); exit(v >= Gem::Version.new("3.1.0") && v < Gem::Version.new("3.2.0") ? 0 : 1)' >/dev/null 2>&1; then
  echo "Ruby SDK prebuilts require Ruby 3.1.x to match the grpc 1.58.3 release ABI" >&2
  echo "Set RUBY and BUNDLE or run via the GitHub workflow ruby/setup-ruby toolchain." >&2
  exit 2
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

bundler_platform_for_target() {
  case "$1" in
    aarch64-apple-darwin) echo "arm64-darwin" ;;
    x86_64-apple-darwin) echo "x86_64-darwin" ;;
    x86_64-unknown-linux-gnu) echo "x86_64-linux" ;;
    aarch64-unknown-linux-gnu) echo "aarch64-linux" ;;
    x86_64-unknown-linux-musl) echo "x86_64-linux-musl" ;;
    aarch64-unknown-linux-musl) echo "aarch64-linux-musl" ;;
    x86_64-windows-gnu) echo "x64-mingw-ucrt" ;;
    *) echo "unsupported Ruby SDK prebuilt target: $1" >&2; return 2 ;;
  esac
}

docker_platform_for_target() {
  case "$1" in
    x86_64-unknown-linux-musl) echo "linux/amd64" ;;
    aarch64-unknown-linux-musl) echo "linux/arm64" ;;
    *) return 1 ;;
  esac
}

if ! repo_root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
  repo_root="${GITHUB_WORKSPACE:-$PWD}"
fi
# shellcheck source=.github/scripts/lib-codegen-prebuilt.sh
source "${repo_root}/.github/scripts/lib-codegen-prebuilt.sh"
ruby_sdk_dir="${repo_root}/sdk/ruby-holons"
dist_dir="${repo_root}/dist/sdk-prebuilts/ruby/${sdk_target}"
work_dir="${ruby_sdk_dir}/.ruby-prebuilt/${sdk_target}"
bundle_platform="$(bundler_platform_for_target "$sdk_target")"

if [[ "$sdk_target" == *-linux-musl && "${RUBY_PREBUILT_INSIDE_MUSL:-}" != "1" ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    echo "docker is required for musl Ruby prebuilts" >&2
    exit 127
  fi
  docker_platform="$(docker_platform_for_target "$sdk_target")"
  exec docker run --rm \
    --platform "$docker_platform" \
    -e SDK_TARGET="$sdk_target" \
    -e SDK_VERSION="$sdk_version" \
    -e RUBY_HOLONS_JOBS="$jobs" \
    -e RUBY_PREBUILT_INSIDE_MUSL=1 \
    -v "${repo_root}:${repo_root}" \
    -w "$repo_root" \
    ruby:3.1-alpine \
    sh -lc 'apk add --no-cache bash build-base git linux-headers tar gzip && .github/scripts/build-prebuilt-ruby.sh'
fi

case "$sdk_target" in
  aarch64-apple-darwin|x86_64-apple-darwin|x86_64-unknown-linux-gnu|aarch64-unknown-linux-gnu|x86_64-unknown-linux-musl|aarch64-unknown-linux-musl|x86_64-windows-gnu)
    ;;
esac

rm -rf "$work_dir"
mkdir -p "$work_dir/work" "$dist_dir"
stage="${work_dir}/stage/ruby-holons-v${sdk_version}-${sdk_target}"
mkdir -p "$stage/vendor" "$stage/share" "$stage/debug"

cp "${ruby_sdk_dir}/Gemfile" "$work_dir/work/Gemfile"
if [[ -f "${ruby_sdk_dir}/Gemfile.lock" ]]; then
  cp "${ruby_sdk_dir}/Gemfile.lock" "$work_dir/work/Gemfile.lock"
fi

(
  cd "$work_dir/work"
  "$bundle_bin" lock --add-platform "$bundle_platform"
  "$bundle_bin" config set --local path "${stage}/vendor/bundle"
  "$bundle_bin" config set --local clean true
  "$bundle_bin" config set --local disable_shared_gems true
  if [[ "$sdk_target" == *-linux-musl ]]; then
    BUNDLE_FORCE_RUBY_PLATFORM=true "$bundle_bin" install --jobs "$jobs" --retry 3
  else
    "$bundle_bin" install --jobs "$jobs" --retry 3
  fi
  BUNDLE_GEMFILE="$PWD/Gemfile" BUNDLE_PATH="${stage}/vendor/bundle" "$bundle_bin" exec "$ruby_bin" -e 'require "grpc"; require "google/protobuf"; puts "grpc=#{Gem.loaded_specs["grpc"].version} protobuf=#{Gem.loaded_specs["google-protobuf"].version}"'
  BUNDLE_GEMFILE="$PWD/Gemfile" BUNDLE_PATH="${stage}/vendor/bundle" "$bundle_bin" exec "$ruby_bin" -e 'abort "missing grpc_tools_ruby_protoc" unless File.executable?(Gem.bin_path("grpc-tools", "grpc_tools_ruby_protoc"))'
)

ruby_version="$("$ruby_bin" -e 'print RUBY_VERSION')"
ruby_platform="$("$ruby_bin" -e 'print RUBY_PLATFORM')"
{
  echo "sdk=ruby"
  echo "version=${sdk_version}"
  echo "target=${sdk_target}"
  echo "bundler_platform=${bundle_platform}"
  echo "ruby_version=${ruby_version}"
  echo "ruby_platform=${ruby_platform}"
  echo "built_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >"$stage/share/prebuilt.env"

install_protoc_release "$sdk_target" "$stage"
build_adapter_family "$repo_root" "$sdk_target" "$stage/bin" ruby
(
  cd "$work_dir/work"
  grpc_plugin="$(BUNDLE_GEMFILE="$PWD/Gemfile" BUNDLE_PATH="${stage}/vendor/bundle" "$bundle_bin" exec "$ruby_bin" -e 'begin; path = Gem.bin_path("grpc-tools", "grpc_ruby_plugin"); print path if File.executable?(path); rescue Gem::Exception; end')"
  if [[ -n "$grpc_plugin" ]]; then
    cp "$grpc_plugin" "$stage/bin/grpc_ruby_plugin"
    chmod +x "$stage/bin/grpc_ruby_plugin"
  else
    echo "grpc_ruby_plugin not present in grpc-tools for ${sdk_target}; ruby adapter will emit message stubs only" >&2
  fi
)

cat >"$stage/manifest.json" <<EOF
{
  "lang": "ruby",
  "version": "${sdk_version}",
  "target": "${sdk_target}",
  "codegen": {
    "plugins": [
      {"name": "ruby", "binary": "bin/protoc-gen-ruby$(target_exe_suffix "$sdk_target")", "out_subdir": "ruby"}
    ]
  }
}
EOF

printf 'No separate debug sidecar files were emitted for %s.\n' "$sdk_target" >"$stage/debug/README.txt"
tar -C "$stage" -czf "${dist_dir}/ruby-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" debug

archive="${dist_dir}/ruby-holons-v${sdk_version}-${sdk_target}.tar.gz"
tar -C "$stage" -czf "$archive" vendor bin share manifest.json

if command -v syft >/dev/null 2>&1; then
  syft "dir:${stage}" -o "spdx-json=${archive}.spdx.json"
else
  cat >"${archive}.spdx.json" <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "ruby-holons-v${sdk_version}-${sdk_target}",
  "documentNamespace": "https://github.com/organic-programming/seed/sdk-prebuilts/ruby/${sdk_version}/${sdk_target}",
  "creationInfo": {
    "created": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "creators": ["Tool: build-prebuilt-ruby.sh"]
  },
  "packages": []
}
EOF
fi

sha256_file "$archive" >"${archive}.sha256"
sha256_file "${archive}.spdx.json" >"${archive}.spdx.json.sha256"
sha256_file "${dist_dir}/ruby-holons-v${sdk_version}-${sdk_target}-debug.tar.gz" >"${dist_dir}/ruby-holons-v${sdk_version}-${sdk_target}-debug.tar.gz.sha256"

echo "Ruby SDK prebuilt staged:"
ls -lh "$dist_dir"
