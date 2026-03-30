#pragma once

#include "holons.hpp"

namespace holons::identity {

using identity = HolonIdentity;
using manifest = HolonManifest;
using resolved_manifest = ResolvedHolonManifest;

inline constexpr std::string_view kProtoManifestFileName = "holon.proto";

inline resolved_manifest resolve(const std::filesystem::path &manifest_path) {
  return parse_resolved_manifest(manifest_path.string());
}

inline identity read(const std::filesystem::path &manifest_path) {
  return parse_holon(manifest_path.string());
}

inline std::optional<manifest> read_manifest(
    const std::filesystem::path &manifest_path) {
  return parse_manifest(manifest_path.string());
}

inline std::optional<std::filesystem::path> find_manifest(
    const std::filesystem::path &root) {
  return find_holon_proto(root);
}

inline std::filesystem::path resolve_manifest(
    const std::filesystem::path &root) {
  return resolve_manifest_path(root);
}

} // namespace holons::identity
