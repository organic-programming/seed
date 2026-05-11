#!/usr/bin/env bash
# scripts/stacked-release.sh — local pre-flight for a coherent stack release.
#
# Usage:
#   ./scripts/stacked-release.sh <version>
#   e.g. ./scripts/stacked-release.sh 1.0.0
#
# Updates version markers across the seed tree (proto manifests, seed-toolchain.yaml),
# then rebuilds every SDK and every holon locally to confirm the new state compiles.
# Does NOT commit, push, tag, or publish anything. The composer reviews `git diff`
# and pushes manually if satisfied.

set -euo pipefail

on_failure() {
  local code=$?
  if [[ $code -ne 0 ]]; then
    echo
    echo "========================================================================"
    echo "PRE-FLIGHT FAILED (exit code $code)"
    echo "========================================================================"
    echo "The working tree contains partial modifications. To roll back everything:"
    echo
    echo "  git checkout -- ."
    echo "  git clean -fd"
    echo
    echo "To inspect what was modified before rolling back:"
    echo "  git status"
    echo "  git diff"
    echo "========================================================================"
  fi
}
trap on_failure EXIT

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

VERSION="$1"

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+].+)?$ ]]; then
  echo "error: '$VERSION' is not a valid semver string" >&2
  exit 2
fi

SEED_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SEED_ROOT"

if [[ -n "$(git status --short)" ]]; then
  echo "error: working tree is not clean; commit or stash before running this script" >&2
  git status --short >&2
  exit 3
fi

echo "=== Stacked release pre-flight for v${VERSION} ==="
echo "Seed root: $SEED_ROOT"
echo

# -----------------------------------------------------------------------------
echo "=== [1/6] Rebuild op ==="
op build op --install
echo

# -----------------------------------------------------------------------------
echo "=== [2/6] Update version in holon.proto manifests ==="
updated_protos=()
while IFS= read -r -d '' proto; do
  awk -v version="$VERSION" '
    BEGIN { in_manifest = 0 }
    /option \(holons\.v1\.manifest\)/ { in_manifest = 1 }
    in_manifest && /version:/ {
      sub(/"[^"]*"/, "\"" version "\"")
      in_manifest = 0
    }
    { print }
  ' "$proto" > "${proto}.tmp"
  if ! cmp -s "$proto" "${proto}.tmp"; then
    mv "${proto}.tmp" "$proto"
    updated_protos+=("$proto")
    echo "  updated: $proto"
  else
    rm "${proto}.tmp"
  fi
done < <(find holons examples organism_kits -name 'holon.proto' -path '*/api/v1/*' -print0 2>/dev/null)
echo "  → ${#updated_protos[@]} proto manifest(s) updated"
echo

# -----------------------------------------------------------------------------
echo "=== [3/6] Bump seed_release in seed-toolchain.yaml ==="
if [[ -f seed-toolchain.yaml ]]; then
  sed -i.bak -E "s/^(seed_release:[[:space:]]*)\"?[^\"#[:space:]]+\"?(.*)$/\1\"${VERSION}\"\2/" seed-toolchain.yaml
  rm seed-toolchain.yaml.bak
  echo "  $(grep '^seed_release:' seed-toolchain.yaml)"
else
  echo "  seed-toolchain.yaml not found, skipping"
fi
echo

# -----------------------------------------------------------------------------
echo "=== [4/6] Rebuild all SDKs ==="
if op sdk build --help 2>&1 | grep -qE '(^|[[:space:]])all([[:space:]]|$)'; then
  op sdk build all --force
else
  echo "  'op sdk build all' not available, falling back to per-lang loop"
  for lang in c cpp csharp dart go java js js-web kotlin python ruby rust swift zig; do
    echo
    echo "  → op sdk build $lang --force"
    op sdk build "$lang" --force
  done
fi
echo

# -----------------------------------------------------------------------------
echo "=== [5/6] Rebuild every holon ==="
built_holons=()
while IFS= read -r -d '' proto; do
  holon_dir="$(dirname "$(dirname "$(dirname "$proto")")")"
  if [[ ! -d "$holon_dir" ]]; then continue; fi
  echo
  echo "  → op build $holon_dir"
  if op build "$holon_dir"; then
    built_holons+=("$holon_dir")
  else
    echo "error: op build $holon_dir failed" >&2
    exit 4
  fi
done < <(find holons examples organism_kits -name 'holon.proto' -path '*/api/v1/*' -print0 2>/dev/null)
echo
echo "  → ${#built_holons[@]} holon(s) built"
echo

# -----------------------------------------------------------------------------
echo "=== [6/7] Ader smoke (op_invoke triplet) ==="
ader test ader/catalogues/grace-op \
  --suite op_invoke \
  --profile smoke \
  --lane progression \
  --source workspace
echo

# -----------------------------------------------------------------------------
# Disarm the failure trap — if we got here, everything passed.
trap - EXIT

echo "========================================================================"
echo "PRE-FLIGHT GREEN for v${VERSION}"
echo "========================================================================"
echo "  Proto manifests updated: ${#updated_protos[@]}"
echo "  SDKs rebuilt:            ok"
echo "  Holons rebuilt:          ${#built_holons[@]}"
echo "  Ader op_invoke smoke:    ok"
echo
echo "The working tree is ready. Review and commit manually when satisfied:"
echo
echo "  git status"
echo "  git diff"
echo "  git add ."
echo "  git commit -m \"release: stack v${VERSION}\""
echo "  git push"
echo "========================================================================"
