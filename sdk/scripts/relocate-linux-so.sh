#!/usr/bin/env bash
#
# This script bundles non-system dynamic libraries (.so) into a Linux executable's folder.
# It makes the application portable by relinking the binary using patchelf to set RPATH/RUNPATH
# so that the dependency libraries are found in a local lib/ directory.
#
set -euo pipefail

if [[ "${OSTYPE:-}" != linux-gnu* ]] && [[ "${OSTYPE:-}" != linux* ]]; then
  exit 0
fi

if [[ "$#" -lt 2 ]]; then
  echo "usage: relocate-linux-so.sh <runtime-dir> <executable> [<executable>...]" >&2
  exit 64
fi

if ! command -v patchelf &> /dev/null; then
  echo "error: patchelf is required but not installed." >&2
  exit 1
fi

runtime_dir=$1
shift
lib_dir="${runtime_dir}/lib"
mkdir -p "${lib_dir}"

queue_file=$(mktemp)
seen_file=$(mktemp)
copied_file=$(mktemp)
trap 'rm -f "${queue_file}" "${seen_file}" "${copied_file}"' EXIT

contains_exact() {
  local needle=$1
  local haystack=$2
  grep -Fqx -- "${needle}" "${haystack}" 2>/dev/null
}

queue_scan() {
  local item=$1
  [[ -n "${item}" ]] || return 0
  if ! contains_exact "${item}" "${queue_file}" && ! contains_exact "${item}" "${seen_file}"; then
    printf '%s\n' "${item}" >>"${queue_file}"
  fi
}

record_copied() {
  local item=$1
  [[ -n "${item}" ]] || return 0
  if ! contains_exact "${item}" "${copied_file}"; then
    printf '%s\n' "${item}" >>"${copied_file}"
  fi
}

list_deps() {
  local file=$1
  ldd "${file}" 2>/dev/null | while read -r line; do
    if echo "$line" | grep -q "=>"; then
      local path=$(echo "$line" | awk '{print $3}')
      if [[ -f "$path" ]]; then
        echo "$path"
      fi
    else
      local path=$(echo "$line" | awk '{print $1}')
      if [[ -f "$path" ]]; then
        echo "$path"
      fi
    fi
  done
}

for entry in "$@"; do
  if [[ "${entry}" = /* ]]; then
    queue_scan "${entry}"
  else
    queue_scan "${runtime_dir}/${entry}"
  fi
done

while true; do
  source_file=$(sed -n '1p' "${queue_file}")
  if [[ -z "${source_file}" ]]; then
    break
  fi
  sed '1d' "${queue_file}" >"${queue_file}.next"
  mv "${queue_file}.next" "${queue_file}"

  if contains_exact "${source_file}" "${seen_file}"; then
    continue
  fi
  printf '%s\n' "${source_file}" >>"${seen_file}"

  while IFS= read -r resolved_dep; do
    [[ -n "${resolved_dep}" ]] || continue

    case "${resolved_dep}" in
      /lib/*|/usr/lib/*|/lib64/*|/usr/lib64/*|/usr/local/lib/*)
        continue
        ;;
    esac

    dep_base=$(basename "${resolved_dep}")
    dep_dest="${lib_dir}/${dep_base}"
    if [[ ! -e "${dep_dest}" ]]; then
      cp -f "${resolved_dep}" "${dep_dest}"
      chmod u+w "${dep_dest}" 2>/dev/null || true
    fi
    record_copied "${dep_dest}"
    queue_scan "${resolved_dep}"
  done < <(list_deps "${source_file}")
done

# Patch executables
for entry in "$@"; do
  local target
  if [[ "${entry}" = /* ]]; then
    target="${entry}"
  else
    target="${runtime_dir}/${entry}"
  fi
  patchelf --set-rpath '$ORIGIN/lib' "${target}" 2>/dev/null || true
done

# Patch copied libraries
while IFS= read -r so_file || [[ -n "${so_file}" ]]; do
  [[ -n "${so_file}" ]] || continue
  chmod u+w "${so_file}" 2>/dev/null || true
  patchelf --set-rpath '$ORIGIN' "${so_file}" 2>/dev/null || true
done <"${copied_file}"
