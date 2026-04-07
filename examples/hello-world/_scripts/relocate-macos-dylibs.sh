#!/usr/bin/env bash
set -euo pipefail

if [[ "${OSTYPE:-}" != darwin* ]]; then
  exit 0
fi

if [[ "$#" -lt 2 ]]; then
  echo "usage: relocate-macos-dylibs.sh <runtime-dir> <executable> [<executable>...]" >&2
  exit 64
fi

runtime_dir=$1
shift
frameworks_dir="${runtime_dir}/Frameworks"
mkdir -p "${frameworks_dir}"

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

resolve_token() {
  local token=$1
  local source_file=$2
  local source_dir
  source_dir=$(dirname "${source_file}")

  case "${token}" in
    @loader_path/*)
      realpath "${source_dir}/${token#@loader_path/}" 2>/dev/null || return 1
      ;;
    @executable_path/*)
      realpath "${runtime_dir}/${token#@executable_path/}" 2>/dev/null || return 1
      ;;
    /*)
      realpath "${token}" 2>/dev/null || return 1
      ;;
    *)
      return 1
      ;;
  esac
}

list_deps() {
  local file=$1
  otool -L "${file}" | tail -n +2 | awk '{print $1}'
}

list_rpaths() {
  local file=$1
  otool -l "${file}" | awk '
    $1 == "cmd" && $2 == "LC_RPATH" { want = 1; next }
    want && $1 == "path" { print $2; want = 0 }
  '
}

resolve_dependency() {
  local dep=$1
  local source_file=$2

  case "${dep}" in
    /System/Library/*|/usr/lib/*)
      return 1
      ;;
    @rpath/*)
      local suffix=${dep#@rpath/}
      while IFS= read -r raw_rpath; do
        [[ -n "${raw_rpath}" ]] || continue
        local resolved_rpath
        resolved_rpath=$(resolve_token "${raw_rpath}" "${source_file}" 2>/dev/null || true)
        [[ -n "${resolved_rpath}" ]] || continue
        if [[ -e "${resolved_rpath}/${suffix}" ]]; then
          realpath "${resolved_rpath}/${suffix}"
          return 0
        fi
      done < <(list_rpaths "${source_file}")
      return 1
      ;;
    @loader_path/*|@executable_path/*|/*)
      resolve_token "${dep}" "${source_file}" 2>/dev/null || return 1
      ;;
    *)
      return 1
      ;;
  esac
}

rewrite_file() {
  local file=$1
  local new_prefix=$2
  local dep

  chmod u+w "${file}" 2>/dev/null || true
  while IFS= read -r dep; do
    [[ -n "${dep}" ]] || continue
    case "${dep}" in
      /System/Library/*|/usr/lib/*)
        continue
        ;;
    esac

    local base
    base=$(basename "${dep}")
    local bundled="${frameworks_dir}/${base}"
    if [[ -e "${bundled}" ]]; then
      local new_ref="${new_prefix}${base}"
      if [[ "${dep}" != "${new_ref}" ]]; then
        install_name_tool -change "${dep}" "${new_ref}" "${file}"
      fi
    fi
  done < <(list_deps "${file}")
}

sign_if_macho() {
  local file=$1
  [[ -f "${file}" ]] || return 0
  if file -b "${file}" | grep -q "Mach-O"; then
    codesign --force --sign - "${file}" >/dev/null
  fi
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

  while IFS= read -r dep; do
    [[ -n "${dep}" ]] || continue
    resolved_dep=$(resolve_dependency "${dep}" "${source_file}" 2>/dev/null || true)
    [[ -n "${resolved_dep}" ]] || continue

    case "${resolved_dep}" in
      /System/Library/*|/usr/lib/*)
        continue
        ;;
    esac

    dep_base=$(basename "${dep}")
    dep_dest="${frameworks_dir}/${dep_base}"
    if [[ ! -e "${dep_dest}" ]]; then
      cp -f "${resolved_dep}" "${dep_dest}"
      chmod u+w "${dep_dest}" 2>/dev/null || true
    fi
    record_copied "${dep_dest}"
    queue_scan "${resolved_dep}"
  done < <(list_deps "${source_file}")
done

for entry in "$@"; do
  if [[ "${entry}" = /* ]]; then
    rewrite_file "${entry}" "@loader_path/Frameworks/"
  else
    rewrite_file "${runtime_dir}/${entry}" "@loader_path/Frameworks/"
  fi
done

while IFS= read -r dylib || [[ -n "${dylib}" ]]; do
  [[ -n "${dylib}" ]] || continue
  chmod u+w "${dylib}" 2>/dev/null || true
  install_name_tool -id "@loader_path/$(basename "${dylib}")" "${dylib}"
  rewrite_file "${dylib}" "@loader_path/"
  sign_if_macho "${dylib}"
done <"${copied_file}"

for entry in "$@"; do
  if [[ "${entry}" = /* ]]; then
    sign_if_macho "${entry}"
  else
    sign_if_macho "${runtime_dir}/${entry}"
  fi
done

while IFS= read -r runtime_entry || [[ -n "${runtime_entry}" ]]; do
  [[ -n "${runtime_entry}" ]] || continue
  sign_if_macho "${runtime_entry}"
done < <(find "${runtime_dir}" -maxdepth 1 -type f)
