#!/usr/bin/env bash
set -euo pipefail
export SDK_LANG=rust
script_dir="$(CDPATH= cd -- "$(dirname "$0")" && pwd)"
# shellcheck source=.github/scripts/lib-codegen-prebuilt.sh
source "${script_dir}/lib-codegen-prebuilt.sh"
repo_root="$(repo_root_or_pwd)"
export SDK_VERSION="${SDK_VERSION:-$(seed_release "$repo_root")}"
exec "${script_dir}/build-prebuilt-codegen-light.sh"
