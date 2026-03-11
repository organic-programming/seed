#!/bin/bash
# Build the extracted Go daemon into the local build/ directory.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAEMON_SRC="$(cd "${SCRIPT_DIR}/../../../daemons/gudule-daemon-greeting-go" && pwd)"
OUTPUT_DIR="${1:-${SCRIPT_DIR}/../build}"
BINARY_NAME="gudule-daemon-greeting-go"

GO="$(command -v go)"
mkdir -p "$OUTPUT_DIR"

echo "Building ${BINARY_NAME} for $(${GO} env GOOS)/$(${GO} env GOARCH)..."
"${GO}" build -C "$DAEMON_SRC" -o "$(cd "$OUTPUT_DIR" && pwd)/${BINARY_NAME}" ./cmd/daemon

echo "Built: ${OUTPUT_DIR}/${BINARY_NAME}"
