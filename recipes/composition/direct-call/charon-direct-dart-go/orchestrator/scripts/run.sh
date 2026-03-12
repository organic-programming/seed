#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
workspace_root=${CHARON_WORKSPACE_ROOT:-$(CDPATH= cd -- "$script_dir/../.." && pwd)}
cd "$workspace_root"

json_result() {
  printf '%s' "$1" | tr -d '\n\r' | sed -E 's/.*"result"[[:space:]]*:[[:space:]]*"?([^",}]*)"?.*/\1/'
}

result=$(json_result "$(op -f json grpc://charon-worker-compute compute.v1.ComputeService/Compute '{"value":42}')")
printf '{"pattern":"direct-call","input":42,"result":%s}
' "$result"
