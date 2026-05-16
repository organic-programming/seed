#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/pre-validate-cascade.sh [options]

Runs the local pre-validation suite that mirrors the observability-cascade CI.

Options:
  --no-cascade          Skip observability-cascade builds and invocations.
  --no-sdk-tests        Skip SDK test suites.
  --no-apps             Skip Phase 3 app builds.
  --stress-runs N       Run RunMultiPattern N times per cascade composite. Default: 10.
  --dry-run             Print commands without executing them.
  --list                Print the default validation targets.
  -h, --help            Show this help.

Environment overrides:
  CASCADE_LANGS         Space-separated cascade languages to validate.
  SDK_TEST_LANGS        Space-separated SDK languages to test.
  PHASE3_APPS           Space-separated Phase 3 app slugs to build.
  CASCADE_STRESS_RUNS   Stress count, equivalent to --stress-runs.
EOF
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

# Committed successful cascade ports as of Phase 4. C is structurally blocked,
# Zig is intentionally excluded until its uncommitted validation work lands.
DEFAULT_CASCADE_LANGS="go dart csharp rust python cpp ruby java node kotlin swift"
DEFAULT_SDK_TEST_LANGS="go dart csharp rust python cpp ruby java node kotlin swift js-web"
DEFAULT_PHASE3_APPS="gabriel-greeting-swift gabriel-greeting-app-swiftui gabriel-greeting-app-flutter"

RUN_CASCADE=1
RUN_SDK_TESTS=1
RUN_APPS=1
DRY_RUN=0
LIST_ONLY=0
CASCADE_STRESS_RUNS="${CASCADE_STRESS_RUNS:-10}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-cascade)
      RUN_CASCADE=0
      ;;
    --no-sdk-tests)
      RUN_SDK_TESTS=0
      ;;
    --no-apps)
      RUN_APPS=0
      ;;
    --stress-runs)
      shift
      if [ "$#" -eq 0 ]; then
        echo "--stress-runs requires a value" >&2
        exit 2
      fi
      CASCADE_STRESS_RUNS="$1"
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    --list)
      LIST_ONLY=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

case "$CASCADE_STRESS_RUNS" in
  ''|*[!0-9]*)
    echo "CASCADE_STRESS_RUNS must be a non-negative integer" >&2
    exit 2
    ;;
esac

CASCADE_LANGS_TEXT="${CASCADE_LANGS:-$DEFAULT_CASCADE_LANGS}"
SDK_TEST_LANGS_TEXT="${SDK_TEST_LANGS:-$DEFAULT_SDK_TEST_LANGS}"
PHASE3_APPS_TEXT="${PHASE3_APPS:-$DEFAULT_PHASE3_APPS}"

read -r -a CASCADE_LANGS_ARR <<< "$CASCADE_LANGS_TEXT"
read -r -a SDK_TEST_LANGS_ARR <<< "$SDK_TEST_LANGS_TEXT"
read -r -a PHASE3_APPS_ARR <<< "$PHASE3_APPS_TEXT"

if [ "$LIST_ONLY" -eq 1 ]; then
  printf 'cascade_langs: %s\n' "$CASCADE_LANGS_TEXT"
  printf 'sdk_test_langs: %s\n' "$SDK_TEST_LANGS_TEXT"
  printf 'phase3_apps: %s\n' "$PHASE3_APPS_TEXT"
  printf 'stress_runs: %s\n' "$CASCADE_STRESS_RUNS"
  exit 0
fi

PASS_STEPS=()
FAIL_STEPS=()
SKIP_STEPS=()

record_pass() {
  PASS_STEPS+=("$1")
}

record_fail() {
  FAIL_STEPS+=("$1")
}

record_skip() {
  SKIP_STEPS+=("$1")
}

print_command() {
  local first=1
  for arg in "$@"; do
    if [ "$first" -eq 0 ]; then
      printf ' '
    fi
    first=0
    printf '%q' "$arg"
  done
  printf '\n'
}

run_cmd() {
  local label="$1"
  shift
  printf '\n==> %s\n' "$label"
  printf '+ '
  print_command "$@"
  if [ "$DRY_RUN" -eq 1 ]; then
    record_pass "$label (dry-run)"
    return 0
  fi

  set +e
  "$@"
  local rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    printf 'PASS: %s\n' "$label"
    record_pass "$label"
    return 0
  fi

  printf 'FAIL: %s (exit %s)\n' "$label" "$rc" >&2
  record_fail "$label"
  return "$rc"
}

run_shell() {
  local label="$1"
  local script="$2"
  run_cmd "$label" bash -lc "$script"
}

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    printf 'Missing required command: %s\n' "$name" >&2
    record_fail "required command: $name"
    return 1
  fi
  return 0
}

work_tmp=""
validator=""
if [ "$DRY_RUN" -eq 0 ]; then
  work_tmp="$(mktemp -d "${TMPDIR:-/tmp}/cascade-prevalidate.XXXXXX")"
  validator="$work_tmp/validate_report.py"
  cat > "$validator" <<'PY'
import json
import sys


def find_report(value, mode):
    if not isinstance(value, dict):
        return None
    keys = set(value.keys())
    if mode in ("default", "live") and "ticks" in keys and ("pass" in keys or "fail" in keys):
        return value
    if mode == "multi" and (
        "totalPass" in keys
        or "total_pass" in keys
        or "totalFail" in keys
        or "total_fail" in keys
    ):
        return value
    for key in ("report", "response", "result", "data"):
        nested = value.get(key)
        if isinstance(nested, dict):
            found = find_report(nested, mode)
            if found is not None:
                return found
    return None


def get_int(report, names, default=None):
    for name in names:
        if name in report and report[name] is not None:
            return int(report[name])
    if default is not None:
        return default
    raise AssertionError(f"missing integer field, expected one of {names}: {report}")


def get_list(report, names):
    for name in names:
        value = report.get(name)
        if isinstance(value, list):
            return value
    return None


def get_elapsed_us(phase):
    for name in ("elapsed_us", "elapsedUs", "elapsedMicros", "elapsed_microseconds"):
        if name in phase and phase[name] is not None:
            return int(phase[name])
    return None


def validate_default_or_live(report, name):
    ticks = get_int(report, ("ticks",))
    passed = get_int(report, ("pass", "passed"))
    failed = get_int(report, ("fail", "failed"), 0)
    if ticks != 30 or passed != 30 or failed != 0:
        raise AssertionError(f"{name} expected ticks=30 pass=30 fail=0, got {report}")
    phases = get_list(report, ("phases", "phaseReports", "phase_reports"))
    if phases is not None:
        if len(phases) != 10:
            raise AssertionError(f"{name} expected 10 phases, got {len(phases)}")
        for index, phase in enumerate(phases, start=1):
            elapsed = get_elapsed_us(phase)
            if elapsed is not None and elapsed <= 0:
                raise AssertionError(f"{name} phase {index} elapsed_us must be > 0, got {elapsed}")


def validate_multi(report):
    total_pass = get_int(report, ("total_pass", "totalPass"))
    total_fail = get_int(report, ("total_fail", "totalFail"), 0)
    total = get_int(report, ("total", "total_ticks", "totalTicks"), total_pass + total_fail)
    if total != 240 or total_pass != 240 or total_fail != 0:
        raise AssertionError(f"RunMultiPattern expected total=240 total_pass=240 total_fail=0, got {report}")
    patterns = get_list(report, ("patterns", "patternReports", "pattern_reports"))
    if patterns is not None:
        if len(patterns) != 8:
            raise AssertionError(f"RunMultiPattern expected 8 patterns, got {len(patterns)}")
        for index, pattern in enumerate(patterns, start=1):
            passed = get_int(pattern, ("pass", "passed"))
            failed = get_int(pattern, ("fail", "failed"), 0)
            if passed != 30 or failed != 0:
                raise AssertionError(f"RunMultiPattern pattern {index} expected pass=30 fail=0, got {pattern}")


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: validate_report.py <default|live|multi> <json-path> <slug>")
    mode, path, slug = sys.argv[1:4]
    with open(path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    report = find_report(data, mode)
    if report is None:
        raise AssertionError(f"{slug} {mode} did not contain a recognized report: {data}")
    if mode in ("default", "live"):
        validate_default_or_live(report, "RunDefault" if mode == "default" else "RunLiveStream")
    elif mode == "multi":
        validate_multi(report)
    else:
        raise AssertionError(f"unknown mode: {mode}")
    print(f"{slug} {mode} report passed")


if __name__ == "__main__":
    main()
PY
else
  work_tmp="${TMPDIR:-/tmp}/cascade-prevalidate-dry-run"
  validator="$work_tmp/validate_report.py"
fi

cleanup() {
  if [ -n "$work_tmp" ] && [ "${KEEP_PREVALIDATE_TMP:-0}" != "1" ]; then
    rm -rf "$work_tmp"
  elif [ -n "$work_tmp" ]; then
    printf 'Kept validation temp dir: %s\n' "$work_tmp"
  fi
}
trap cleanup EXIT

print_header() {
  printf 'Observability cascade pre-validation\n'
  printf 'Repository: %s\n' "$repo_root"
  printf 'Cascade languages: %s\n' "$CASCADE_LANGS_TEXT"
  printf 'SDK test languages: %s\n' "$SDK_TEST_LANGS_TEXT"
  printf 'Phase 3 apps: %s\n' "$PHASE3_APPS_TEXT"
  printf 'Stress runs per cascade: %s\n' "$CASCADE_STRESS_RUNS"
}

warn_dirty_tree() {
  if [ "$DRY_RUN" -eq 1 ]; then
    return 0
  fi
  if ! git diff --quiet || ! git diff --cached --quiet || [ -n "$(git ls-files --others --exclude-standard)" ]; then
    printf '\nWARNING: working tree is not clean; validation will use current files.\n' >&2
  fi
}

validate_report_file() {
  local label="$1"
  local mode="$2"
  local path="$3"
  local slug="$4"
  run_cmd "$label" python3 "$validator" "$mode" "$path" "$slug"
}

invoke_and_validate() {
  local slug="$1"
  local method="$2"
  local mode="$3"
  local output="$4"
  if ! run_cmd "$slug $method" bash -lc "set -o pipefail; op invoke '$slug' '$method' '{}' -f json | tee '$output'"; then
    return 1
  fi
  validate_report_file "$slug $method structured assertions" "$mode" "$output" "$slug"
}

validate_cascade_lang() {
  local lang="$1"
  local slug="observability-cascade-$lang"
  local slug_tmp="$work_tmp/$slug"
  if [ "$DRY_RUN" -eq 0 ]; then
    mkdir -p "$slug_tmp"
  fi

  if ! run_cmd "$slug build" op build "$slug" --install; then
    return 0
  fi

  invoke_and_validate "$slug" RunDefault default "$slug_tmp/default.json" || true
  invoke_and_validate "$slug" RunLiveStream live "$slug_tmp/live.json" || true
  invoke_and_validate "$slug" RunMultiPattern multi "$slug_tmp/multi.json" || true

  local i
  for ((i = 1; i <= CASCADE_STRESS_RUNS; i++)); do
    local out="$slug_tmp/stress-$i.json"
    if ! run_cmd "$slug RunMultiPattern stress $i/$CASCADE_STRESS_RUNS" bash -lc "op invoke '$slug' RunMultiPattern '{}' -f json > '$out'"; then
      continue
    fi
    validate_report_file "$slug stress $i/$CASCADE_STRESS_RUNS structured assertions" multi "$out" "$slug" || true
  done
}

npm_install_and_test() {
  local dir="$1"
  if [ -f "$dir/package-lock.json" ]; then
    run_shell "$dir npm ci/test" "cd '$repo_root/$dir' && npm ci && npm test"
  else
    run_shell "$dir npm install/test" "cd '$repo_root/$dir' && npm install && npm test"
  fi
}

validate_sdk_lang() {
  local lang="$1"
  case "$lang" in
    go)
      run_shell "sdk/go-holons go test" "cd '$repo_root/sdk/go-holons' && go test ./..."
      ;;
    dart)
      run_shell "sdk/dart-holons dart test" "cd '$repo_root/sdk/dart-holons' && dart pub get && dart test"
      ;;
    csharp)
      run_cmd "sdk/csharp-holons dotnet test" dotnet test "$repo_root/sdk/csharp-holons/csharp-holons.sln"
      ;;
    rust)
      run_shell "sdk/rust-holons cargo test" "cd '$repo_root/sdk/rust-holons' && cargo test"
      ;;
    python)
      run_shell "sdk/python-holons pytest" "cd '$repo_root/sdk/python-holons' && python3 -m pip install -e . && python3 -m pytest tests -q"
      ;;
    cpp)
      local build_dir="$work_tmp/cpp-sdk-build"
      run_shell "sdk/cpp-holons cmake/ctest" "cmake -S '$repo_root/sdk/cpp-holons' -B '$build_dir' && cmake --build '$build_dir' && ctest --test-dir '$build_dir' --output-on-failure && if [ -x '$build_dir/test_runner' ]; then '$build_dir/test_runner'; fi"
      ;;
    ruby)
      run_shell "sdk/ruby-holons bundle test" "cd '$repo_root/sdk/ruby-holons' && bundle install && bundle exec rake test"
      ;;
    java)
      run_shell "sdk/java-holons gradle test" "cd '$repo_root/sdk/java-holons' && ./gradlew test"
      ;;
    node)
      npm_install_and_test "sdk/node-holons"
      ;;
    kotlin)
      run_shell "sdk/kotlin-holons gradle test" "cd '$repo_root/sdk/kotlin-holons' && ./gradlew test"
      ;;
    swift)
      run_shell "sdk/swift-holons swift test" "cd '$repo_root/sdk/swift-holons' && swift test"
      ;;
    js-web)
      npm_install_and_test "sdk/js-web-holons"
      ;;
    *)
      printf 'Unknown SDK test language: %s\n' "$lang" >&2
      record_fail "sdk/$lang unknown test command"
      return 1
      ;;
  esac
}

validate_phase3_app() {
  local app="$1"
  case "$app" in
    gabriel-greeting-swift)
      run_cmd "$app op build" op build "$app" --install
      ;;
    gabriel-greeting-app-swiftui)
      run_cmd "$app hardened op build" op build "$app" --hardened
      ;;
    gabriel-greeting-app-flutter)
      run_cmd "$app hardened op build" op build "$app" --hardened
      ;;
    *)
      printf 'Unknown Phase 3 app: %s\n' "$app" >&2
      record_fail "phase3 app $app unknown build command"
      return 1
      ;;
  esac
}

print_summary() {
  printf '\nValidation summary\n'
  printf '  PASS: %s\n' "${#PASS_STEPS[@]}"
  printf '  FAIL: %s\n' "${#FAIL_STEPS[@]}"
  printf '  SKIP: %s\n' "${#SKIP_STEPS[@]}"

  if [ "${#FAIL_STEPS[@]}" -gt 0 ]; then
    printf '\nFailed steps:\n'
    local step
    for step in "${FAIL_STEPS[@]}"; do
      printf '  - %s\n' "$step"
    done
  fi

  if [ "${#SKIP_STEPS[@]}" -gt 0 ]; then
    printf '\nSkipped steps:\n'
    local skipped
    for skipped in "${SKIP_STEPS[@]}"; do
      printf '  - %s\n' "$skipped"
    done
  fi
}

main() {
  print_header
  warn_dirty_tree

  require_command bash || true
  require_command python3 || true
  if [ "$RUN_CASCADE" -eq 1 ] || [ "$RUN_APPS" -eq 1 ]; then
    require_command op || true
  fi

  if [ "$RUN_CASCADE" -eq 1 ]; then
    local lang
    for lang in "${CASCADE_LANGS_ARR[@]}"; do
      validate_cascade_lang "$lang"
    done
  else
    record_skip "observability-cascade builds/invocations"
  fi

  if [ "$RUN_SDK_TESTS" -eq 1 ]; then
    local sdk_lang
    for sdk_lang in "${SDK_TEST_LANGS_ARR[@]}"; do
      validate_sdk_lang "$sdk_lang" || true
    done
  else
    record_skip "SDK test suites"
  fi

  if [ "$RUN_APPS" -eq 1 ]; then
    local app
    for app in "${PHASE3_APPS_ARR[@]}"; do
      validate_phase3_app "$app" || true
    done
  else
    record_skip "Phase 3 app builds"
  fi

  print_summary
  if [ "${#FAIL_STEPS[@]}" -gt 0 ]; then
    exit 1
  fi
}

main "$@"
