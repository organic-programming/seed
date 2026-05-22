#!/usr/bin/env bash
# scripts/stacked-release.sh - local pre-flight for a coherent stack release.
#
# Usage:
#   ./scripts/stacked-release.sh <version>
#   e.g. ./scripts/stacked-release.sh 1.0.0
#
# Updates version markers across the seed tree, rebuilds SDKs and holons, and
# runs the mandatory Ader op_invoke smoke. It does not commit, push, tag, create
# releases, or contact GitHub.

set -euo pipefail

VERSION=""
SEED_ROOT=""
FAILED_STEP="initialization"
declare -a MANIFEST_PROTOS=()
declare -a UPDATED_PROTOS=()
declare -a BUILT_HOLONS=()

usage() {
  echo "usage: $0 <version>" >&2
}

on_failure() {
  local code=$?
  if [[ $code -ne 0 ]]; then
    echo >&2
    echo "========================================================================" >&2
    echo "PRE-FLIGHT FAILED during: ${FAILED_STEP} (exit code ${code})" >&2
    echo "========================================================================" >&2
    echo "The working tree may contain partial modifications. Inspect with:" >&2
    echo >&2
    echo "  git status" >&2
    echo "  git diff" >&2
    echo >&2
    echo "To roll back after inspection:" >&2
    echo "  git restore ." >&2
    echo "  git clean -fd" >&2
    echo "========================================================================" >&2
  fi
}
trap on_failure EXIT

validate_version() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 2
  fi
  VERSION="$1"
  if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
    echo "error: '$VERSION' is not a valid semver string" >&2
    exit 2
  fi
}

enter_seed_root() {
  SEED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  cd "$SEED_ROOT"
  if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
    echo "error: scripts/stacked-release.sh must run inside a git checkout" >&2
    exit 2
  fi
}

require_clean_worktree() {
  FAILED_STEP="sanity checks"
  local status
  status="$(git status --short --untracked-files=all)"
  if [[ -n "$status" ]]; then
    echo "error: working tree is not clean; commit or stash before running this script" >&2
    echo "$status" >&2
    exit 3
  fi
}

discover_manifest_protos() {
  FAILED_STEP="discover manifest protos"
  MANIFEST_PROTOS=()
  while IFS= read -r -d '' path; do
    case "$path" in
      */.op/*)
        ;;
      holons/*/api/v1/holon.proto|examples/*/api/v1/holon.proto|organism_kits/*/api/v1/holon.proto)
        MANIFEST_PROTOS+=("$path")
        ;;
    esac
  done < <(git ls-files -z -- holons examples organism_kits 2>/dev/null || true)
}

update_proto_version() {
  local proto="$1"
  local tmp="${proto}.tmp.$$"
  local status=0
  awk -v version="$VERSION" '
    /option[[:space:]]+\(holons\.v1\.manifest\)[[:space:]]*=/ { in_manifest = 1 }
    in_manifest && /^[[:space:]]*version:[[:space:]]*"/ {
      sub(/"[^"]*"/, "\"" version "\"")
      changed = 1
      in_manifest = 0
    }
    { print }
    END {
      if (!changed) {
        exit 42
      }
    }
  ' "$proto" > "$tmp" || status=$?

  if [[ $status -eq 42 ]]; then
    rm -f "$tmp"
    return 1
  fi
  if [[ $status -ne 0 ]]; then
    rm -f "$tmp"
    echo "error: failed to update $proto" >&2
    exit "$status"
  fi
  if cmp -s "$proto" "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  mv "$tmp" "$proto"
  UPDATED_PROTOS+=("$proto")
  return 0
}

update_seed_toolchain() {
  FAILED_STEP="update seed-toolchain.yaml"
  if [[ ! -f seed-toolchain.yaml ]]; then
    echo "error: seed-toolchain.yaml not found" >&2
    exit 4
  fi
  sed -i.bak -E "s/^(seed_release:[[:space:]]*)\"?[^\"#[:space:]]+\"?(.*)$/\1\"${VERSION}\"\2/" seed-toolchain.yaml
  rm -f seed-toolchain.yaml.bak
  if ! grep -qE "^seed_release:[[:space:]]*\"${VERSION}\"" seed-toolchain.yaml; then
    echo "error: failed to set seed_release to ${VERSION}" >&2
    exit 4
  fi
}

print_modified_files() {
  local files
  files="$(git diff --name-only)"
  if [[ -z "$files" ]]; then
    printf "  %-8s %s\n" "UNCHANGED" "(no tracked file changes)"
    return
  fi
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    printf "  %-8s %s\n" "MODIFIED" "$file"
  done <<< "$files"
}

print_success_summary() {
  echo "========================================================================"
  echo "PRE-FLIGHT GREEN for v${VERSION}"
  echo "========================================================================"
  echo
  echo "Modified files:"
  print_modified_files
  echo
  echo "Builds:"
  printf "  %-8s %s\n" "OK" "op build op --install   (bootstrap, against previous SDK)"
  printf "  %-8s %s\n" "OK" "op sdk build all --version ${VERSION} --force"
  for holon in "${BUILT_HOLONS[@]}"; do
    printf "  %-8s %s\n" "OK" "op build ${holon}"
  done
  printf "  %-8s %s\n" "OK" "ader op_invoke smoke"
  echo
  echo "The working tree is ready. Review and commit manually when satisfied:"
  echo
  echo "  git status"
  echo "  git diff"
  echo "  git add ."
  echo "  git commit -m \"release: stack v${VERSION}\""
  echo "  git push"
  echo "========================================================================"
}

validate_version "$@"
enter_seed_root
require_clean_worktree
discover_manifest_protos

echo "=== Stacked release pre-flight for v${VERSION} ==="
echo "Seed root: $SEED_ROOT"
echo

FAILED_STEP="rebuild op"
echo "=== [1/6] Rebuild op ==="
op build op --install
echo

FAILED_STEP="update proto manifest versions"
echo "=== [2/6] Update version in holon.proto manifests ==="
for proto in "${MANIFEST_PROTOS[@]}"; do
  if update_proto_version "$proto"; then
    echo "  updated: $proto"
  fi
done
echo "  ${#UPDATED_PROTOS[@]} proto manifest(s) updated"
echo

echo "=== [3/6] Bump seed_release in seed-toolchain.yaml ==="
update_seed_toolchain
echo "  $(grep '^seed_release:' seed-toolchain.yaml)"
echo

FAILED_STEP="rebuild all SDKs"
echo "=== [4/6] Rebuild all SDKs at v${VERSION} ==="
if ! op sdk build all --version "$VERSION" --force; then
  echo "error: op sdk build all --version ${VERSION} --force failed" >&2
  exit 5
fi
echo

FAILED_STEP="rebuild holons"
echo "=== [5/6] Rebuild every holon against the new SDK (install op + ader) ==="
for proto in "${MANIFEST_PROTOS[@]}"; do
  holon_dir="${proto%/api/v1/holon.proto}"
  install_flag=""
  case "$holon_dir" in
    holons/grace-op|holons/clem-ader)
      install_flag="--install"
      ;;
  esac
  echo "  op build $holon_dir $install_flag"
  if op build "$holon_dir" $install_flag; then
    BUILT_HOLONS+=("${holon_dir}${install_flag:+ ${install_flag}}")
  else
    echo "error: op build $holon_dir $install_flag failed" >&2
    exit 6
  fi
done
echo
echo "  ${#BUILT_HOLONS[@]} holon(s) built (op + ader installed)"
echo

FAILED_STEP="Ader op_invoke smoke"
echo "=== [6/6] Ader smoke (op_invoke triplet) ==="
if ! ader test ader/catalogues/grace-op \
  --suite op_invoke \
  --profile smoke \
  --lane progression \
  --source workspace; then
  echo "error: Ader op_invoke smoke failed" >&2
  exit 7
fi
echo

trap - EXIT
print_success_summary
