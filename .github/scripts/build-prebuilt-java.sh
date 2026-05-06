#!/usr/bin/env bash
set -euo pipefail
export SDK_LANG=java
export SDK_VERSION="${SDK_VERSION:-0.1.0}"
export PROTOC_VERSION="${PROTOC_VERSION:-31.1}"
exec "$(dirname "$0")/build-prebuilt-codegen-light.sh"
