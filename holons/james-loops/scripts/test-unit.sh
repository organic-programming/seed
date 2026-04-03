#!/usr/bin/env bash
set -euo pipefail

if [[ -f gen/describe_generated.go ]]; then
	go test ./...
else
	go test -tags jamesloops_stubs ./...
fi
