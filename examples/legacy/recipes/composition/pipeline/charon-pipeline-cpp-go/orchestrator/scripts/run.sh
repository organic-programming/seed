#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
workspace_root=${CHARON_WORKSPACE_ROOT:-$(CDPATH= cd -- "$script_dir/../.." && pwd)}
cd "$workspace_root"

json_result() {
  printf '%s' "$1" | tr -d '\n\r' | sed -E 's/.*"result"[[:space:]]*:[[:space:]]*"?([^",}]*)"?.*/\1/'
}

computed=$(json_result "$(op -f json grpc://charon-worker-compute compute.v1.ComputeService/Compute '{"value":5}')")
transform_request=$(printf '{"text":"%s"}' "$computed")
transformed=$(json_result "$(op -f json grpc://charon-worker-transform transform.v1.TransformService/Transform "$transform_request")")
printf '{"pattern":"pipeline","input":5,"computed":%s,"transformed":"%s"}\n' "$computed" "$transformed"
