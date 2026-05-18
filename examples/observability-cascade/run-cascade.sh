#!/usr/bin/env bash
set -u

# Local regression gate for observability-cascade composites.
# Env:
#   CASCADE_LANGS        Space-separated languages. Default: "go dart rust".
#   KEEP_CASCADE_TMP=1  Keep JSON responses and command logs.
# Exit codes:
#   0 all requested languages passed; 1 one or more failed; 2 invalid input.

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root" || exit 2

langs_text="${CASCADE_LANGS:-go dart rust}"
[ -n "$langs_text" ] || { echo "CASCADE_LANGS resolved empty" >&2; exit 2; }

for cmd in op python3; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 2; }
done

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/cascade-run.XXXXXX")"
cleanup() {
  if [ "${KEEP_CASCADE_TMP:-0}" = "1" ] || [ "${failures:-0}" -gt 0 ]; then
    echo "Kept cascade temp dir: $tmp_dir" >&2
  else
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

validate_report() {
  python3 - "$1" "$2" <<'PY'
import json, sys

def find_report(value):
    if not isinstance(value, dict):
        return None
    if "ticks" in value and ("pass" in value or "passed" in value):
        return value
    for key in ("report", "response", "result", "data"):
        found = find_report(value.get(key))
        if found is not None:
            return found
    return None

path, slug = sys.argv[1:3]
with open(path, encoding="utf-8") as handle:
    report = find_report(json.load(handle))
if report is None:
    raise SystemExit(f"{slug} RunDefault response has no relay tick/pass report")
ticks = int(report["ticks"])
passed = int(report.get("pass", report.get("passed", 0)))
failed = int(report.get("fail", report.get("failed", 0)))
if passed <= 0:
    raise SystemExit(f"{slug} pass count must be > 0, got {passed}")
if failed != 0:
    raise SystemExit(f"{slug} fail count must be 0, got {failed}")
print(f"ticks={ticks} pass={passed}")
PY
}

read -r -a langs <<< "$langs_text"
failures=0

for lang in "${langs[@]}"; do
  slug="observability-cascade-$lang"
  build_log="$tmp_dir/$slug.build.log"
  output_json="$tmp_dir/$slug.default.json"
  invoke_log="$tmp_dir/$slug.invoke.log"

  if ! op build "$slug" >"$build_log" 2>&1; then
    echo "FAIL $lang build failed; log: $build_log"
    failures=$((failures + 1))
    continue
  fi
  if ! op invoke "$slug" RunDefault '{}' -f json >"$output_json" 2>"$invoke_log"; then
    echo "FAIL $lang RunDefault failed; log: $invoke_log"
    failures=$((failures + 1))
    continue
  fi
  if ! summary="$(validate_report "$output_json" "$slug" 2>&1)"; then
    echo "FAIL $lang sanity failed: $summary"
    failures=$((failures + 1))
    continue
  fi
  echo "PASS $lang $summary"
done

[ "$failures" -eq 0 ] || exit 1
