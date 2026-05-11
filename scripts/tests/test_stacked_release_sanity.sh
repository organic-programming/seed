#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/scripts/stacked-release.sh"

write_proto() {
  local path="$1"
  local version="$2"
  mkdir -p "$(dirname "$path")"
  cat > "$path" <<EOF
syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    given_name: "Test"
    family_name: "Holon"
    version: "${version}"
  }
};
EOF
}

make_repo() {
  local repo="$1"
  mkdir -p "$repo/scripts" "$repo/bin"
  cp "$SCRIPT" "$repo/scripts/stacked-release.sh"
  chmod +x "$repo/scripts/stacked-release.sh"
  cat > "$repo/.gitignore" <<'EOF'
.op/
EOF
  cat > "$repo/seed-toolchain.yaml" <<'EOF'
seed_release: "0.1.0"
EOF
  write_proto "$repo/holons/good/api/v1/holon.proto" "0.1.0"
  write_proto "$repo/holons/good/.op/protos/api/v1/holon.proto" "0.1.0"
  cat > "$repo/bin/op" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'op %s\n' "$*" >> "${STACKED_RELEASE_LOG:?}"
if [[ "${1:-}" == "build" && "${2:-}" == "${STACKED_RELEASE_FAIL_HOLON:-__none__}" ]]; then
  echo "stub failure for op build ${2}" >&2
  exit 44
fi
exit 0
EOF
  chmod +x "$repo/bin/op"
  cat > "$repo/bin/ader" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'ader %s\n' "$*" >> "${STACKED_RELEASE_LOG:?}"
exit 0
EOF
  chmod +x "$repo/bin/ader"
  git -C "$repo" init -q
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name "Test User"
  git -C "$repo" add .
  git -C "$repo" add -f holons/good/.op/protos/api/v1/holon.proto
  git -C "$repo" commit -q -m initial
}

assert_contains() {
  local file="$1"
  local needle="$2"
  if ! grep -Fq "$needle" "$file"; then
    echo "expected $file to contain: $needle" >&2
    echo "--- $file ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

test_successful_subset_release() {
  local tmp repo out log
  tmp="$(mktemp -d)"
  repo="$tmp/repo"
  out="$tmp/out.txt"
  log="$tmp/commands.log"
  make_repo "$repo"

  STACKED_RELEASE_LOG="$log" PATH="$repo/bin:$PATH" "$repo/scripts/stacked-release.sh" 99.99.99 >"$out" 2>&1

  assert_contains "$repo/holons/good/api/v1/holon.proto" 'version: "99.99.99"'
  assert_contains "$repo/seed-toolchain.yaml" 'seed_release: "99.99.99"'
  assert_contains "$repo/holons/good/.op/protos/api/v1/holon.proto" 'version: "0.1.0"'
  assert_contains "$log" "op build op --install"
  assert_contains "$log" "op sdk build all --force"
  assert_contains "$log" "op build holons/good"
  assert_contains "$log" "ader test ader/catalogues/grace-op --suite op_invoke --profile smoke --lane progression --source workspace"
  assert_contains "$out" "PRE-FLIGHT GREEN for v99.99.99"
  assert_contains "$out" "MODIFIED holons/good/api/v1/holon.proto"
  assert_contains "$out" "OK       ader op_invoke smoke"
}

test_broken_holon_fails_loudly() {
  local tmp repo out log
  tmp="$(mktemp -d)"
  repo="$tmp/repo"
  out="$tmp/out.txt"
  log="$tmp/commands.log"
  make_repo "$repo"
  write_proto "$repo/examples/broken/api/v1/holon.proto" "0.1.0"
  git -C "$repo" add examples/broken/api/v1/holon.proto
  git -C "$repo" commit -q -m broken-fixture

  if STACKED_RELEASE_LOG="$log" STACKED_RELEASE_FAIL_HOLON="examples/broken" PATH="$repo/bin:$PATH" \
    "$repo/scripts/stacked-release.sh" 99.99.99 >"$out" 2>&1; then
    echo "expected stacked-release.sh to fail for broken holon" >&2
    exit 1
  fi

  assert_contains "$out" "error: op build examples/broken failed"
  assert_contains "$out" "PRE-FLIGHT FAILED during: rebuild holons"
}

test_successful_subset_release
test_broken_holon_fails_loudly
