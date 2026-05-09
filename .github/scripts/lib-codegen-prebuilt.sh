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

seed_toolchain() {
  python3 "$(CDPATH= cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)/seed_toolchain.py" "$@"
}

toolchain_manifest_json() {
  local repo_root="$1"
  local lang="$2"
  local target="$3"
  seed_toolchain manifest-json "$repo_root" "$lang" "$target"
}

plugin_version() {
  local repo_root="$1"
  local lang="$2"
  local plugin="$3"
  seed_toolchain plugin-version "$repo_root" "$lang" "$plugin"
}

plugin_sha256() {
  local repo_root="$1"
  local lang="$2"
  local plugin="$3"
  local target="$4"
  seed_toolchain plugin-sha256 "$repo_root" "$lang" "$plugin" "$target"
}

seed_release() {
  local repo_root="$1"
  seed_toolchain seed-release "$repo_root"
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
  local repo_root
  local version
  local asset
  local tmp

  repo_root="$(repo_root_or_pwd)"
  version="$(seed_toolchain protoc-version "$repo_root")"
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
  local repo_root
  local version
  local classifier
  local expected_sha
  local suffix
  local tmp
  local actual_sha
  local url

  repo_root="$(repo_root_or_pwd)"
  version="$(plugin_version "$repo_root" java protoc-gen-grpc-java)"
  expected_sha="$(plugin_sha256 "$repo_root" java protoc-gen-grpc-java "$target")"
  if [[ -z "$expected_sha" ]]; then
    echo "seed-toolchain.yaml missing sha256 for java/protoc-gen-grpc-java/${target}" >&2
    return 1
  fi

  case "$target" in
    aarch64-apple-darwin)
      classifier="osx-aarch_64"
      ;;
    *)
      echo "unsupported protoc-gen-grpc-java target: ${target}" >&2
      return 2
      ;;
  esac

  suffix="$(target_exe_suffix "$target")"
  tmp="$(mktemp)"
  url="https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/${version}/protoc-gen-grpc-java-${version}-${classifier}.exe"
  curl -fsSL "$url" -o "$tmp"
  actual_sha="$(sha256_file "$tmp" | awk '{print $1}')"
  if [[ "$actual_sha" != "$expected_sha" ]]; then
    echo "protoc-gen-grpc-java sha256 mismatch: got ${actual_sha}, want ${expected_sha}" >&2
    rm -f "$tmp"
    return 1
  fi
  if command -v file >/dev/null 2>&1 && ! file "$tmp" | grep -q 'arm64'; then
    echo "protoc-gen-grpc-java ${version}/${classifier} is not arm64-capable: $(file "$tmp")" >&2
    rm -f "$tmp"
    return 1
  fi
  mkdir -p "${stage}/bin"
  cp "$tmp" "${stage}/bin/protoc-gen-grpc-java${suffix}"
  chmod +x "${stage}/bin/protoc-gen-grpc-java${suffix}"
  rm -f "$tmp"
}

install_grpc_kotlin_plugin() {
  local stage="$1"
  local repo_root
  local version
  local expected_sha
  local tmp
  local actual_sha
  local jar
  local wrapper

  repo_root="$(repo_root_or_pwd)"
  version="$(plugin_version "$repo_root" kotlin protoc-gen-grpc-kotlin)"
  expected_sha="$(plugin_sha256 "$repo_root" kotlin protoc-gen-grpc-kotlin "${SDK_TARGET:?SDK_TARGET is required}")"
  if [[ -z "$expected_sha" ]]; then
    echo "seed-toolchain.yaml missing sha256 for kotlin/protoc-gen-grpc-kotlin/${SDK_TARGET}" >&2
    return 1
  fi

  tmp="$(mktemp)"
  curl -fsSL \
    "https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-kotlin/${version}/protoc-gen-grpc-kotlin-${version}-jdk8.jar" \
    -o "$tmp"
  actual_sha="$(sha256_file "$tmp" | awk '{print $1}')"
  if [[ "$actual_sha" != "$expected_sha" ]]; then
    echo "protoc-gen-grpc-kotlin sha256 mismatch: got ${actual_sha}, want ${expected_sha}" >&2
    rm -f "$tmp"
    return 1
  fi
  jar="${stage}/share/kotlin/protoc-gen-grpc-kotlin-${version}-jdk8.jar"
  wrapper="${stage}/bin/protoc-gen-grpc-kotlin"
  mkdir -p "$(dirname "$jar")" "${stage}/bin"
  cp "$tmp" "$jar"
  cat >"$wrapper" <<EOF
#!/usr/bin/env sh
set -eu
exec java -jar "\$(CDPATH= cd -- "\$(dirname "\$0")" && pwd)/../share/kotlin/protoc-gen-grpc-kotlin-${version}-jdk8.jar" "\$@"
EOF
  chmod +x "$wrapper"
  rm -f "$tmp"
}

install_js_protoc_plugin() {
  local stage="$1"
  local work_dir="$2"
  local target="${SDK_TARGET:?SDK_TARGET is required}"
  local repo_root
  local version
  local npm_prefix="${work_dir}/protoc-gen-js"
  local suffix
  local source
  local dest

  repo_root="$(repo_root_or_pwd)"
  version="$(plugin_version "$repo_root" js protoc-gen-js)"

  suffix="$(target_exe_suffix "$target")"
  rm -rf "$npm_prefix"
  mkdir -p "$npm_prefix" "${stage}/bin"
  npm install --prefix "$npm_prefix" --omit=dev --no-audit --no-fund \
    "protoc-gen-js@${version}"
  source="${npm_prefix}/node_modules/protoc-gen-js/bin/protoc-gen-js"
  dest="${stage}/bin/protoc-gen-js${suffix}"
  cp "$source" "$dest"
  chmod +x "$dest"
  if [[ "$target" == "aarch64-apple-darwin" ]] && command -v file >/dev/null 2>&1 && ! file "$dest" | grep -q 'arm64'; then
    echo "protoc-gen-js ${version} is not arm64-capable: $(file "$dest")" >&2
    return 1
  fi
}

install_node_codegen_plugins() {
  local stage="$1"
  local work_dir="$2"

  copy_grpc_sibling "$stage" grpc_node_plugin
  build_grpc_js_wrapper "$stage" "$work_dir"
}

build_grpc_js_wrapper() {
  local stage="$1"
  local work_dir="$2"
  local target="${SDK_TARGET:?SDK_TARGET is required}"
  local src_dir="${work_dir}/grpc-js-wrapper"
  local suffix
  local goos
  local goarch

  suffix="$(target_exe_suffix "$target")"
  read -r goos goarch < <(target_goos_goarch "$target")
  rm -rf "$src_dir"
  mkdir -p "$src_dir"
  cat >"${src_dir}/go.mod" <<'EOF'
module op.local/grpcjswrapper

go 1.22

require google.golang.org/protobuf v1.36.11
EOF
  cat >"${src_dir}/main.go" <<'EOF'
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		return err
	}
	req.Parameter = nil
	rewrittenInput, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	plugin := filepath.Join(filepath.Dir(exe), "grpc_node_plugin")
	cmd := exec.Command(plugin)
	cmd.Stdin = bytes.NewReader(rewrittenInput)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			_, _ = os.Stderr.Write(stderr.Bytes())
		}
		return fmt.Errorf("grpc_node_plugin failed: %w", err)
	}

	resp := &pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(stdout.Bytes(), resp); err != nil {
		return err
	}
	for _, file := range resp.File {
		if file.Content == nil || !strings.HasSuffix(file.GetName(), "_grpc_pb.js") {
			continue
		}
		content := file.GetContent()
		content = strings.ReplaceAll(content, "require('grpc')", "require('@grpc/grpc-js')")
		content = strings.ReplaceAll(content, "new Buffer(", "Buffer.from(")
		file.Content = &content
	}
	output, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(output)
	return err
}
EOF
  (
    cd "$src_dir"
    env GOWORK=off CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -mod=mod -trimpath -ldflags="-s -w" -o "${stage}/bin/grpc_tools_node_protoc_plugin${suffix}" .
  )
  chmod +x "${stage}/bin/grpc_node_plugin${suffix}" "${stage}/bin/grpc_tools_node_protoc_plugin${suffix}"
}

wrap_protoc_with_sibling_path() {
  local stage="$1"
  local target="${SDK_TARGET:?SDK_TARGET is required}"
  local suffix
  local protoc
  local real_protoc

  suffix="$(target_exe_suffix "$target")"
  protoc="${stage}/bin/protoc${suffix}"
  real_protoc="${stage}/bin/protoc-real${suffix}"
  mv "$protoc" "$real_protoc"
  cat >"$protoc" <<'EOF'
#!/usr/bin/env sh
set -eu
bin_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
PATH="${bin_dir}:${PATH:-}" exec "${bin_dir}/protoc-real" "$@"
EOF
  chmod +x "$protoc" "$real_protoc"
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
