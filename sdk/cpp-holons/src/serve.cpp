#include "holons/serve.hpp"

#include <mutex>

namespace holons::serve {
namespace {
std::mutex &transport_mu() {
  static std::mutex mu;
  return mu;
}

std::string &active_transport() {
  static std::string value;
  return value;
}
} // namespace

namespace detail {
thread_local std::string current_transport;

void set_current_transport(std::string transport) {
  current_transport = transport;
  std::scoped_lock lk(transport_mu());
  active_transport() = std::move(transport);
}

void clear_current_transport() {
  current_transport.clear();
  std::scoped_lock lk(transport_mu());
  active_transport().clear();
}
} // namespace detail

std::string CurrentTransport() {
  if (!detail::current_transport.empty()) return detail::current_transport;
  std::scoped_lock lk(transport_mu());
  return active_transport();
}

} // namespace holons::serve
