#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
workspace_root=${CHARON_WORKSPACE_ROOT:-$(CDPATH= cd -- "$script_dir/../.." && pwd)}
cd "$workspace_root"

json_result() {
  printf '%s' "$1" | tr -d '\n\r' | sed -E 's/.*"result"[[:space:]]*:[[:space:]]*"?([^",}]*)"?.*/\1/'
}

compute_file=$(mktemp)
transform_file=$(mktemp)
cleanup() {
  rm -f "$compute_file" "$transform_file"
}
trap cleanup EXIT
(op -f json grpc://charon-worker-compute compute.v1.ComputeService/Compute '{"value":5}' >"$compute_file") &
compute_pid=$!
(op -f json grpc://charon-worker-transform transform.v1.TransformService/Transform '{"text":"hello"}' >"$transform_file") &
transform_pid=$!
wait "$compute_pid"
wait "$transform_pid"
computed=$(json_result "$(cat "$compute_file")")
transformed=$(json_result "$(cat "$transform_file")")
printf '{"pattern":"fan-out","compute_input":5,"compute_result":%s,"transform_input":"hello","transform_result":"%s"}
' "$computed" "$transformed"
