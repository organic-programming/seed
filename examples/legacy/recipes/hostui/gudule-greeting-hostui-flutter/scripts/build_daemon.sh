#!/bin/bash
# Build one extracted greeting daemon into the local build/ directory.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAEMON_VARIANT="${1:-}"
if [ -z "${DAEMON_VARIANT}" ]; then
  echo "usage: ./scripts/build_daemon.sh <go|rust|swift|kotlin|dart|python|csharp|node> [output-dir]" >&2
  exit 1
fi

DAEMON_NAME="gudule-daemon-greeting-${DAEMON_VARIANT}"
DAEMON_SRC="$(cd "${SCRIPT_DIR}/../../../daemons/${DAEMON_NAME}" && pwd)"
OUTPUT_DIR="${2:-${SCRIPT_DIR}/../build}"
if [ $# -ge 2 ]; then
  OUTPUT_DIR="$2"
fi
BINARY_NAME="${DAEMON_NAME}"

GO="$(command -v go)"
mkdir -p "${OUTPUT_DIR}"

case "${DAEMON_VARIANT}" in
  go)
    echo "Building ${BINARY_NAME} for $(${GO} env GOOS)/$(${GO} env GOARCH)..."
    "${GO}" build -C "${DAEMON_SRC}" -o "$(cd "${OUTPUT_DIR}" && pwd)/${BINARY_NAME}" ./cmd/daemon
    ;;
  *)
    echo "This helper only supports the Go daemon today. Build ${DAEMON_NAME} with its native toolchain or let an assembly bundle it." >&2
    exit 1
    ;;
esac

echo "Built: ${OUTPUT_DIR}/${BINARY_NAME}"
