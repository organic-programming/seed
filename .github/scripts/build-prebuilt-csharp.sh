#!/usr/bin/env bash
set -euo pipefail
export SDK_LANG=csharp
export SDK_VERSION="${SDK_VERSION:-0.1.0}"
exec "$(dirname "$0")/build-prebuilt-codegen-light.sh"
