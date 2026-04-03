#!/usr/bin/env bash
set -euo pipefail

if [[ -f gen/describe_generated.go ]]; then
	go test ./...
else
	go test -tags codexloops_stubs ./...
fi
