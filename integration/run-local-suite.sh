#!/usr/bin/env bash

set -u
set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INTEGRATION_DIR="$ROOT/integration"
ARTIFACTS_DIR="$INTEGRATION_DIR/.artifacts"
TOOL_CACHE_DIR="$ARTIFACTS_DIR/tool-cache"
REPORTS_DIR="$INTEGRATION_DIR/reports"
SHORT_TMP_DIR="$INTEGRATION_DIR/.t"

PROFILE="${1:-full}"
STEP_FILTER="${2:-}"
RUN_ID="$(date +%Y%m%dT%H%M%S)"

RUN_ARTIFACTS_DIR="$ARTIFACTS_DIR/local-suite/$RUN_ID"
WORKSPACE_ROOT="$RUN_ARTIFACTS_DIR/workspace"
SHORT_TMP_STORE="/tmp/seed-int-store-$RUN_ID"
RUN_DIR="$REPORTS_DIR/$RUN_ID"
LOG_DIR="$RUN_DIR/logs"
SUMMARY_MD="$RUN_DIR/summary.md"
SUMMARY_TSV="$RUN_DIR/summary.tsv"

have_tool() {
  command -v "$1" >/dev/null 2>&1
}

usage() {
  cat <<'EOF'
Usage:
  ./integration/run-local-suite.sh <profile> [step-regex]

Profiles:
  quick        Go-first smoke: grace-op, go SDK, go example, integration short
  unit         Native unit suites across grace-op, SDKs, and examples with tests
  integration  Black-box integration suite only
  full         Unit suites plus full integration suite
  list         Print the profiles and the execution matrix
  help         Print this help

Arguments:
  step-regex   Optional regex filter applied to step ids after profile expansion

Examples:
  ./integration/run-local-suite.sh quick
  ./integration/run-local-suite.sh unit 'sdk-|example-go'
  ./integration/run-local-suite.sh full 'integration-|grace-op'

Notes:
  - Reports are written under integration/reports/<timestamp>/
  - Shared caches live under integration/.artifacts/tool-cache/
  - Native unit suites run from a mirrored workspace under integration/.artifacts/
  - integration/tests remains its own isolated black-box package
EOF
}

list_profiles() {
  cat <<'EOF'
Profiles:
  quick
    - grace-op-unit
    - sdk-go-unit
    - example-go-unit
    - integration-short

  unit
    - grace-op-unit
    - sdk-go-unit
    - sdk-c-unit
    - sdk-cpp-unit
    - sdk-csharp-unit
    - sdk-dart-unit
    - sdk-java-unit
    - sdk-js-unit
    - sdk-js-web-unit
    - sdk-kotlin-unit
    - sdk-ruby-unit
    - sdk-rust-unit
    - sdk-swift-unit
    - example-c-unit
    - example-cpp-unit
    - example-csharp-unit
    - example-dart-unit
    - example-go-unit
    - example-java-unit
    - example-kotlin-unit
    - example-node-unit
    - example-python-unit
    - example-ruby-unit
    - example-rust-unit
    - example-swift-unit

  integration
    - integration-full

  full
    - unit
    - integration-full

Filter examples:
  ./integration/run-local-suite.sh unit 'sdk-go|sdk-rust'
  ./integration/run-local-suite.sh full 'integration-|example-go'
EOF
}

workspace_path() {
  printf '%s/%s' "$WORKSPACE_ROOT" "$1"
}

source_path() {
  printf '%s/%s' "$ROOT" "$1"
}

prepare_workspace_copy() {
  if [[ -d "$WORKSPACE_ROOT/sdk" ]] && [[ -d "$WORKSPACE_ROOT/examples" ]] && [[ -d "$WORKSPACE_ROOT/holons" ]]; then
    return
  fi

  mkdir -p "$WORKSPACE_ROOT"
  printf 'Preparing mirrored workspace under %s\n' "$WORKSPACE_ROOT"

  local rel
  for rel in examples holons protos sdk scripts; do
    printf '  - copying %s\n' "$rel"
    (
      cd "$ROOT" && tar \
        --exclude '.git' \
        --exclude 'integration/.artifacts' \
        --exclude 'integration/reports' \
        --exclude 'integration/.t' \
        --exclude '*/.gradle' \
        --exclude '*/.kotlin' \
        --exclude '*/.build' \
        --exclude '*/build' \
        --exclude '*/target' \
        --exclude '*/obj' \
        --exclude '*/__pycache__' \
        -cf - "$rel"
    ) | (
      cd "$WORKSPACE_ROOT" && tar -xf -
    )
  done
}

case "$PROFILE" in
  help|-h|--help)
    usage
    exit 0
    ;;
  list)
    list_profiles
    exit 0
    ;;
esac

mkdir -p \
  "$ARTIFACTS_DIR" \
  "$TOOL_CACHE_DIR" \
  "$REPORTS_DIR" \
  "$RUN_ARTIFACTS_DIR" \
  "$LOG_DIR" \
  "$TOOL_CACHE_DIR/go-build" \
  "$TOOL_CACHE_DIR/go-mod" \
  "$TOOL_CACHE_DIR/gradle" \
  "$TOOL_CACHE_DIR/npm" \
  "$TOOL_CACHE_DIR/bundle" \
  "$TOOL_CACHE_DIR/dart-pub" \
  "$TOOL_CACHE_DIR/nuget" \
  "$TOOL_CACHE_DIR/dotnet-home"

mkdir -p "$SHORT_TMP_STORE"
rm -rf "$SHORT_TMP_DIR"
ln -s "$SHORT_TMP_STORE" "$SHORT_TMP_DIR"
trap 'rm -rf "$SHORT_TMP_DIR"; rm -rf "$SHORT_TMP_STORE"' EXIT

export TMPDIR="$SHORT_TMP_STORE"
export TMP="$TMPDIR"
export TEMP="$TMPDIR"
export GOCACHE="$TOOL_CACHE_DIR/go-build"
export GOMODCACHE="$TOOL_CACHE_DIR/go-mod"
export GRADLE_USER_HOME="$TOOL_CACHE_DIR/gradle"
export npm_config_cache="$TOOL_CACHE_DIR/npm"
export BUNDLE_PATH="$TOOL_CACHE_DIR/bundle"
export PUB_CACHE="$TOOL_CACHE_DIR/dart-pub"
export DOTNET_CLI_HOME="$TOOL_CACHE_DIR/dotnet-home"
export NUGET_PACKAGES="$TOOL_CACHE_DIR/nuget"
export CARGO_TARGET_DIR="$ARTIFACTS_DIR/cargo/default"
export PYTHONDONTWRITEBYTECODE=1

declare -a STEP_SPECS=()

add_step() {
  STEP_SPECS+=("$1|$2|$3|$4|$5")
}

add_quick_steps() {
  add_step "grace-op-unit" "$(workspace_path holons/grace-op)" "go" "go test ./..." "Go unit tests for the op binary and runtime"
  add_step "sdk-go-unit" "$(workspace_path sdk/go-holons)" "go" "go test ./..." "Go SDK unit tests"
  add_step "example-go-unit" "$(workspace_path examples/hello-world/gabriel-greeting-go)" "go" "go test ./..." "Go hello-world holon unit tests"
  add_step "integration-short" "$(source_path integration/tests)" "go" "go test -short -count=1 -timeout 15m ./..." "Short black-box integration suite"
}

add_unit_steps() {
  add_step "grace-op-unit" "$(workspace_path holons/grace-op)" "go" "go test ./..." "Go unit tests for the op binary and runtime"
  add_step "sdk-go-unit" "$(workspace_path sdk/go-holons)" "go" "go test ./..." "Go SDK unit tests"
  add_step "sdk-c-unit" "$(workspace_path sdk/c-holons)" "make" "make clean && make test && make clean" "C SDK unit tests"
  add_step "sdk-cpp-unit" "$(workspace_path sdk/cpp-holons)" "make,cmake" "rm -rf \"$ARTIFACTS_DIR/cpp-holons-build\" && BUILD_DIR=\"$ARTIFACTS_DIR/cpp-holons-build\" make test && rm -rf \"$ARTIFACTS_DIR/cpp-holons-build\"" "C++ SDK unit tests"
  add_step "sdk-csharp-unit" "$(workspace_path .)" "dotnet" "dotnet test sdk/csharp-holons/csharp-holons.sln --artifacts-path \"$ARTIFACTS_DIR/dotnet/sdk-csharp\"" "C# SDK unit tests"
  add_step "sdk-dart-unit" "$(workspace_path sdk/dart-holons)" "dart" "dart test" "Dart SDK unit tests"
  add_step "sdk-java-unit" "$(workspace_path sdk/java-holons)" "java,gradle" "gradle test" "Java SDK unit tests"
  add_step "sdk-js-unit" "$(workspace_path sdk/js-holons)" "node,npm" "npm test" "Node.js SDK unit tests"
  add_step "sdk-js-web-unit" "$(workspace_path sdk/js-web-holons)" "node,npm" "npm test" "Browser/JS web SDK unit tests"
  add_step "sdk-kotlin-unit" "$(workspace_path sdk/kotlin-holons)" "java,gradle" "gradle test" "Kotlin SDK unit tests"
  add_step "sdk-ruby-unit" "$(workspace_path sdk/ruby-holons)" "ruby,bundle" "bundle exec rake test" "Ruby SDK unit tests"
  add_step "sdk-rust-unit" "$(workspace_path sdk/rust-holons)" "cargo" "cargo test --target-dir \"$ARTIFACTS_DIR/cargo/sdk-rust\"" "Rust SDK unit tests"
  add_step "sdk-swift-unit" "$(workspace_path sdk/swift-holons)" "swift" "swift test --scratch-path \"$ARTIFACTS_DIR/swift/sdk-swift\"" "Swift SDK unit tests"

  add_step "example-c-unit" "$(workspace_path examples/hello-world/gabriel-greeting-c)" "cmake" "rm -rf \"$ARTIFACTS_DIR/cmake/example-c\" && cmake -S . -B \"$ARTIFACTS_DIR/cmake/example-c\" && cmake --build \"$ARTIFACTS_DIR/cmake/example-c\" && ctest --test-dir \"$ARTIFACTS_DIR/cmake/example-c\" --output-on-failure" "C hello-world holon unit tests"
  add_step "example-cpp-unit" "$(workspace_path examples/hello-world/gabriel-greeting-cpp)" "cmake" "rm -rf \"$ARTIFACTS_DIR/cmake/example-cpp\" && cmake -S . -B \"$ARTIFACTS_DIR/cmake/example-cpp\" && cmake --build \"$ARTIFACTS_DIR/cmake/example-cpp\" && ctest --test-dir \"$ARTIFACTS_DIR/cmake/example-cpp\" --output-on-failure" "C++ hello-world holon unit tests"
  add_step "example-csharp-unit" "$(workspace_path examples/hello-world/gabriel-greeting-csharp)" "dotnet" "dotnet test tests/Gabriel.Greeting.Csharp.Tests.csproj --artifacts-path \"$ARTIFACTS_DIR/dotnet/example-csharp\"" "C# hello-world holon unit tests"
  add_step "example-dart-unit" "$(workspace_path examples/hello-world/gabriel-greeting-dart)" "dart" "dart test" "Dart hello-world holon unit tests"
  add_step "example-go-unit" "$(workspace_path examples/hello-world/gabriel-greeting-go)" "go" "go test ./..." "Go hello-world holon unit tests"
  add_step "example-java-unit" "$(workspace_path examples/hello-world/gabriel-greeting-java)" "java,gradle" "gradle test" "Java hello-world holon unit tests"
  add_step "example-kotlin-unit" "$(workspace_path examples/hello-world/gabriel-greeting-kotlin)" "java,gradle" "gradle test" "Kotlin hello-world holon unit tests"
  add_step "example-node-unit" "$(workspace_path examples/hello-world/gabriel-greeting-node)" "node,npm" "npm test" "Node.js hello-world holon unit tests"
  add_step "example-python-unit" "$(workspace_path examples/hello-world/gabriel-greeting-python)" "python3" "python3 -m unittest api.public_test api.cli_test _internal.server_test" "Python hello-world holon unit tests"
  add_step "example-ruby-unit" "$(workspace_path examples/hello-world/gabriel-greeting-ruby)" "ruby,bundle" "bundle exec rake test" "Ruby hello-world holon unit tests"
  add_step "example-rust-unit" "$(workspace_path examples/hello-world/gabriel-greeting-rust)" "cargo" "cargo test --target-dir \"$ARTIFACTS_DIR/cargo/example-rust\"" "Rust hello-world holon unit tests"
  add_step "example-swift-unit" "$(workspace_path examples/hello-world/gabriel-greeting-swift)" "swift" "swift test --scratch-path \"$ARTIFACTS_DIR/swift/example-swift\"" "Swift hello-world holon unit tests"
}

add_integration_steps() {
  add_step "integration-full" "$(source_path integration/tests)" "go" "go test -count=1 -timeout 30m ./..." "Full black-box integration suite"
}

case "$PROFILE" in
  quick)
    prepare_workspace_copy
    add_quick_steps
    ;;
  unit)
    prepare_workspace_copy
    add_unit_steps
    ;;
  integration)
    add_integration_steps
    ;;
  full|global)
    prepare_workspace_copy
    add_unit_steps
    add_integration_steps
    ;;
  *)
    usage
    echo
    echo "Unknown profile: $PROFILE" >&2
    exit 2
    ;;
esac

if [[ -n "$STEP_FILTER" ]]; then
  declare -a FILTERED_STEPS=()
  for spec in "${STEP_SPECS[@]}"; do
    IFS='|' read -r step _ <<<"$spec"
    if [[ "$step" =~ $STEP_FILTER ]]; then
      FILTERED_STEPS+=("$spec")
    fi
  done
  STEP_SPECS=("${FILTERED_STEPS[@]}")
fi

if [[ "${#STEP_SPECS[@]}" -eq 0 ]]; then
  echo "No steps matched profile '$PROFILE' and filter '$STEP_FILTER'." >&2
  exit 2
fi

cat >"$SUMMARY_MD" <<EOF
# Local Regression Report

- Profile: \`$PROFILE\`
- Step Filter: \`${STEP_FILTER:-<none>}\`
- Started: \`$(date -u +"%Y-%m-%dT%H:%M:%SZ")\`
- Repo Root: \`$ROOT\`
- Reports Dir: \`$RUN_DIR\`
- Mirrored Workspace: \`${WORKSPACE_ROOT}\`

This report is generated by \`integration/run-local-suite.sh\`. The runner is
intended for heavy local regression loops and is not designed for free CI.

| Status | Duration | Step | Description | Workdir | Command | Log |
| --- | --- | --- | --- | --- | --- | --- |
EOF

printf 'status\tduration_sec\tstep\tdescription\tworkdir\tcommand\tlog\n' >"$SUMMARY_TSV"

append_report_row() {
  local status="$1"
  local duration="$2"
  local step="$3"
  local description="$4"
  local workdir="$5"
  local command="$6"
  local logfile="$7"

  local rel_log="${logfile#$RUN_DIR/}"
  printf '| %s | %ss | `%s` | %s | `%s` | `%s` | [%s](%s) |\n' \
    "$status" "$duration" "$step" "$description" "$workdir" "$command" "$rel_log" "$rel_log" >>"$SUMMARY_MD"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$status" "$duration" "$step" "$description" "$workdir" "$command" "$logfile" >>"$SUMMARY_TSV"
}

missing_prereqs() {
  local prereq_csv="$1"
  local missing=()
  local item
  IFS=',' read -r -a items <<<"$prereq_csv"
  for item in "${items[@]}"; do
    if [[ -n "$item" ]] && ! have_tool "$item"; then
      missing+=("$item")
    fi
  done
  if ((${#missing[@]} > 0)); then
    printf '%s' "${missing[*]}"
    return 0
  fi
  return 1
}

pass_count=0
fail_count=0
skip_count=0
total_count="${#STEP_SPECS[@]}"
index=0

for spec in "${STEP_SPECS[@]}"; do
  IFS='|' read -r step workdir prereqs command description <<<"$spec"
  index=$((index + 1))
  logfile="$LOG_DIR/$step.log"

  if missing="$(missing_prereqs "$prereqs")"; then
    printf '[%02d/%02d] SKIP %s (missing: %s)\n' "$index" "$total_count" "$step" "$missing"
    {
      echo "status: SKIP"
      echo "reason: missing prerequisites: $missing"
      echo "workdir: $workdir"
      echo "command: $command"
    } >"$logfile"
    append_report_row "SKIP" "0" "$step" "$description" "$workdir" "$command" "$logfile"
    skip_count=$((skip_count + 1))
    continue
  fi

  printf '[%02d/%02d] RUN  %s\n' "$index" "$total_count" "$step"
  start_ts="$(date +%s)"
  (
    cd "$workdir"
    bash -lc "$command"
  ) 2>&1 | tee "$logfile"
  status_code="${PIPESTATUS[0]}"
  end_ts="$(date +%s)"
  duration=$((end_ts - start_ts))

  if [[ "$status_code" -eq 0 ]]; then
    printf '[%02d/%02d] PASS %s (%ss)\n' "$index" "$total_count" "$step" "$duration"
    append_report_row "PASS" "$duration" "$step" "$description" "$workdir" "$command" "$logfile"
    pass_count=$((pass_count + 1))
  else
    printf '[%02d/%02d] FAIL %s (%ss)\n' "$index" "$total_count" "$step" "$duration"
    append_report_row "FAIL" "$duration" "$step" "$description" "$workdir" "$command" "$logfile"
    fail_count=$((fail_count + 1))
  fi
done

{
  echo
  echo "## Totals"
  echo
  echo "- Pass: $pass_count"
  echo "- Fail: $fail_count"
  echo "- Skip: $skip_count"
  echo "- Report: \`$SUMMARY_MD\`"
} >>"$SUMMARY_MD"

printf '\nSummary: pass=%d fail=%d skip=%d\n' "$pass_count" "$fail_count" "$skip_count"
printf 'Report: %s\n' "$SUMMARY_MD"

if [[ "$fail_count" -ne 0 ]]; then
  exit 1
fi
