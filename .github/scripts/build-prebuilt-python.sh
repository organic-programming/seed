#!/usr/bin/env bash
set -euo pipefail
export SDK_LANG=python
export SDK_VERSION="${SDK_VERSION:-0.1.0}"
export PROTOC_VERSION="${PROTOC_VERSION:-32.0}"
exec "$(dirname "$0")/build-prebuilt-codegen-light.sh"
