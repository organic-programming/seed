#!/usr/bin/env bash
set -euo pipefail

artifact_root="${1:-${RUNNER_TEMP:-/tmp}/sdk-prebuilt-artifacts}"
repo="${GITHUB_REPOSITORY:-organic-programming/seed}"
target_commit="${GITHUB_SHA:-master}"
dry_run="${SDK_PREBUILTS_PROMOTE_DRY_RUN:-}"

if [[ ! -d "$artifact_root" ]]; then
  echo "artifact root not found: ${artifact_root}" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to promote SDK prebuilts" >&2
  exit 127
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

sha256_value() {
  local path="$1"
  if [[ -f "${path}.sha256" ]]; then
    awk '{print $1; exit}' "${path}.sha256"
  else
    sha256_file "$path" | awk '{print $1; exit}'
  fi
}

asset_url() {
  local tag="$1"
  local name="$2"
  printf 'https://github.com/%s/releases/download/%s/%s' "$repo" "$tag" "$name"
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
manifest_root="${tmp}/manifests"
mkdir -p "$manifest_root"
groups_file="${tmp}/groups.txt"
: >"$groups_file"

while IFS= read -r -d '' archive; do
  name="$(basename "$archive")"
  [[ "$name" == *-debug.tar.gz ]] && continue
  [[ "$name" =~ ^([a-z]+)-holons-v([0-9][0-9A-Za-z.+]*)-.+\.tar\.gz$ ]] || continue
  sdk="${BASH_REMATCH[1]}"
  version="${BASH_REMATCH[2]}"
  printf '%s|%s\n' "$sdk" "$version" >>"$groups_file"
done < <(find "$artifact_root" -type f -name '*-holons-v*.tar.gz' -print0)

if [[ ! -s "$groups_file" ]]; then
  echo "no SDK prebuilt archives found under ${artifact_root}" >&2
  exit 1
fi

while IFS='|' read -r sdk version; do
  tag="${sdk}-holons-v${version}"
  release_dir="${manifest_root}/${tag}"
  artifacts_jsonl="${tmp}/${tag}.artifacts.jsonl"
  mkdir -p "$release_dir"
  : >"$artifacts_jsonl"

  while IFS= read -r -d '' archive; do
    name="$(basename "$archive")"
    prefix="${sdk}-holons-v${version}-"
    [[ "$name" == "$prefix"* ]] || continue
    target="${name#"$prefix"}"
    target="${target%.tar.gz}"
    [[ "$target" == *-debug ]] && continue

    debug="${archive%.tar.gz}-debug.tar.gz"
    sbom="${archive}.spdx.json"
    cp "$archive" "$release_dir/"
    [[ -f "${archive}.sha256" ]] && cp "${archive}.sha256" "$release_dir/"
    [[ -f "$debug" ]] && cp "$debug" "$release_dir/"
    [[ -f "${debug}.sha256" ]] && cp "${debug}.sha256" "$release_dir/"
    [[ -f "$sbom" ]] && cp "$sbom" "$release_dir/"
    [[ -f "${sbom}.sha256" ]] && cp "${sbom}.sha256" "$release_dir/"

    archive_name="$(basename "$archive")"
    debug_name="$(basename "$debug")"
    sbom_name="$(basename "$sbom")"
    archive_sha="$(sha256_value "$archive")"
    debug_sha=""
    sbom_sha=""
    if [[ -f "$debug" ]]; then
      debug_sha="$(sha256_value "$debug")"
    fi
    if [[ -f "$sbom" ]]; then
      sbom_sha="$(sha256_value "$sbom")"
    fi

    jq -n \
      --arg target "$target" \
      --arg archive_name "$archive_name" \
      --arg archive_url "$(asset_url "$tag" "$archive_name")" \
      --arg archive_sha "$archive_sha" \
      --arg debug_name "$debug_name" \
      --arg debug_url "$(asset_url "$tag" "$debug_name")" \
      --arg debug_sha "$debug_sha" \
      --arg sbom_name "$sbom_name" \
      --arg sbom_url "$(asset_url "$tag" "$sbom_name")" \
      --arg sbom_sha "$sbom_sha" \
      '{
        target: $target,
        archive: {name: $archive_name, url: $archive_url, sha256: $archive_sha},
        debug: (if $debug_sha == "" then null else {name: $debug_name, url: $debug_url, sha256: $debug_sha} end),
        sbom: (if $sbom_sha == "" then null else {name: $sbom_name, url: $sbom_url, sha256: $sbom_sha} end)
      }' >>"$artifacts_jsonl"
  done < <(find "$artifact_root" -type f -name "${sdk}-holons-v${version}-*.tar.gz" ! -name '*-debug.tar.gz' -print0 | sort -z)

  generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  jq -s \
    --arg sdk "$sdk" \
    --arg version "$version" \
    --arg tag "$tag" \
    --arg generated_at "$generated_at" \
    '{
      schema: "sdk-prebuilts-release-manifest/v1",
      sdk: $sdk,
      version: $version,
      tag: $tag,
      generated_at: $generated_at,
      artifacts: .
    }' "$artifacts_jsonl" >"${release_dir}/release-manifest.json"

  echo "Prepared ${tag}:"
  ls -lh "$release_dir"

  if [[ -z "$dry_run" ]]; then
    if ! command -v gh >/dev/null 2>&1; then
      echo "gh is required when SDK_PREBUILTS_PROMOTE_DRY_RUN is not set" >&2
      exit 127
    fi
    if gh release view "$tag" >/dev/null 2>&1; then
      gh release upload "$tag" "${release_dir}"/* --clobber
    else
      gh release create "$tag" "${release_dir}"/* \
        --target "$target_commit" \
        --title "$tag" \
      --notes "SDK prebuilt release for ${sdk} ${version}."
    fi
  fi
done < <(sort -u "$groups_file")
