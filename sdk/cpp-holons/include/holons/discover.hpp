#pragma once

#include "holons.hpp"

namespace holons {

using discovered_holon = HolonEntry;

inline std::optional<HolonEntry> find_nearby_by_slug(
    const std::filesystem::path &root, const std::string &slug) {
  for (const auto &entry : discover(root)) {
    if (entry.slug == slug) {
      return entry;
    }
  }
  return std::nullopt;
}

} // namespace holons
