#include "holons/holons.h"
#include "holons/observability.h"

#include "holons/composite.hpp"
#include "holons/serve.hpp"

#if HOLONS_HAS_GRPCPP
#include <grpcpp/generic/callback_generic_service.h>
#endif

#include <chrono>
#include <condition_variable>
#include <cstdarg>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <map>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <vector>

namespace {

struct c_observability_options {
  std::string slug;
  std::vector<holons::serve::member_ref> members;
};

std::mutex &c_observability_options_mu() {
  static std::mutex mu;
  return mu;
}

c_observability_options &c_observability_options_state() {
  static c_observability_options state;
  return state;
}

std::map<std::string, std::string> c_kv_to_map(const char *const *items) {
  std::map<std::string, std::string> out;
  if (items == nullptr) {
    return out;
  }
  for (size_t i = 0; items[i] != nullptr; i += 2) {
    if (items[i + 1] == nullptr) {
      break;
    }
    if (items[i][0] == '\0') {
      continue;
    }
    out.emplace(items[i], items[i + 1]);
  }
  return out;
}

holons::observability::Level c_level_to_cpp(int level) {
  switch (level) {
    case HOLON_LEVEL_TRACE:
      return holons::observability::Level::Trace;
    case HOLON_LEVEL_DEBUG:
      return holons::observability::Level::Debug;
    case HOLON_LEVEL_INFO:
      return holons::observability::Level::Info;
    case HOLON_LEVEL_WARN:
      return holons::observability::Level::Warn;
    case HOLON_LEVEL_ERROR:
      return holons::observability::Level::Error;
    case HOLON_LEVEL_FATAL:
      return holons::observability::Level::Fatal;
    default:
      return holons::observability::Level::Unset;
  }
}

holons::observability::EventType c_event_to_cpp(int type) {
  switch (type) {
    case HOLON_EVENT_INSTANCE_SPAWNED:
      return holons::observability::EventType::InstanceSpawned;
    case HOLON_EVENT_INSTANCE_READY:
      return holons::observability::EventType::InstanceReady;
    case HOLON_EVENT_INSTANCE_EXITED:
      return holons::observability::EventType::InstanceExited;
    case HOLON_EVENT_INSTANCE_CRASHED:
      return holons::observability::EventType::InstanceCrashed;
    case HOLON_EVENT_SESSION_STARTED:
      return holons::observability::EventType::SessionStarted;
    case HOLON_EVENT_SESSION_ENDED:
      return holons::observability::EventType::SessionEnded;
    case HOLON_EVENT_HANDLER_PANIC:
      return holons::observability::EventType::HandlerPanic;
    case HOLON_EVENT_CONFIG_RELOADED:
      return holons::observability::EventType::ConfigReloaded;
    default:
      return holons::observability::EventType::Unspecified;
  }
}

void set_err(char *err, size_t err_len, const char *fmt, ...) {
  va_list ap;

  if (err == nullptr || err_len == 0) {
    return;
  }

  va_start(ap, fmt);
  std::vsnprintf(err, err_len, fmt, ap);
  va_end(ap);
}

void copy_cstr(char *out, size_t out_len, const std::string &value) {
  if (out == nullptr || out_len == 0) {
    return;
  }
  std::snprintf(out, out_len, "%s", value.c_str());
}

template <typename T>
class c_follow_queue {
 public:
  void push(T value) {
    {
      std::scoped_lock lk(mu_);
      queue_.push_back(std::move(value));
    }
    cv_.notify_one();
  }

  bool wait_pop(T *value) {
    std::unique_lock<std::mutex> lk(mu_);
    cv_.wait(lk, [this]() { return !queue_.empty() || closed_; });
    if (queue_.empty()) {
      return false;
    }
    *value = std::move(queue_.front());
    queue_.erase(queue_.begin());
    return true;
  }

  void close() {
    {
      std::scoped_lock lk(mu_);
      closed_ = true;
    }
    cv_.notify_all();
  }

 private:
  std::mutex mu_;
  std::condition_variable cv_;
  std::vector<T> queue_;
  bool closed_ = false;
};

int64_t unix_nanos(std::chrono::system_clock::time_point tp) {
  return std::chrono::duration_cast<std::chrono::nanoseconds>(
             tp.time_since_epoch())
      .count();
}

holon_level_t cpp_level_to_c(holons::observability::Level level) {
  switch (level) {
    case holons::observability::Level::Trace:
      return HOLON_LEVEL_TRACE;
    case holons::observability::Level::Debug:
      return HOLON_LEVEL_DEBUG;
    case holons::observability::Level::Info:
      return HOLON_LEVEL_INFO;
    case holons::observability::Level::Warn:
      return HOLON_LEVEL_WARN;
    case holons::observability::Level::Error:
      return HOLON_LEVEL_ERROR;
    case holons::observability::Level::Fatal:
      return HOLON_LEVEL_FATAL;
    default:
      return HOLON_LEVEL_UNSET;
  }
}

bool is_resource_attribute(const std::string &key) {
  return key == "holons.slug" || key == "service.name" ||
         key == "holons.instance_uid" ||
         key == "service.instance.id" || key == "holons.session_id" ||
         key == "logger.name";
}

struct kv_snapshot {
  std::vector<std::string> keys;
  std::vector<std::string> values;
  std::vector<holon_obs_kv_t> items;
};

kv_snapshot snapshot_fields(
    const std::vector<holons::observability::Field> &fields,
    bool include_resource_attributes = false) {
  kv_snapshot out;
  out.keys.reserve(fields.size());
  out.values.reserve(fields.size());
  out.items.reserve(fields.size());
  for (const auto &field : fields) {
    if (field.key.empty()) {
      continue;
    }
    if (!include_resource_attributes && is_resource_attribute(field.key)) {
      continue;
    }
    out.keys.push_back(field.key);
    out.values.push_back(holons::observability::any_value_string(field.value));
    out.items.push_back(
        holon_obs_kv_t{out.keys.back().c_str(), out.values.back().c_str()});
  }
  return out;
}

struct chain_snapshot {
  std::vector<std::string> slugs;
  std::vector<std::string> instance_uids;
  std::vector<holon_obs_chain_hop_t> items;
};

chain_snapshot snapshot_chain(const std::vector<std::string> &chain) {
  chain_snapshot out;
  out.slugs.reserve(chain.size());
  out.instance_uids.reserve(chain.size());
  out.items.reserve(chain.size());
  for (const auto &hop : chain) {
    const auto slash = hop.find('/');
    if (slash == std::string::npos) {
      out.slugs.push_back(hop);
      out.instance_uids.emplace_back();
    } else {
      out.slugs.push_back(hop.substr(0, slash));
      out.instance_uids.push_back(hop.substr(slash + 1));
    }
    out.items.push_back(holon_obs_chain_hop_t{
        out.slugs.back().c_str(), out.instance_uids.back().c_str()});
  }
  return out;
}

holon_event_type_t cpp_event_name_to_c(const std::string &name) {
  if (name == HOLON_EVENT_NAME_INSTANCE_SPAWNED) {
    return HOLON_EVENT_INSTANCE_SPAWNED;
  }
  if (name == HOLON_EVENT_NAME_INSTANCE_READY) {
    return HOLON_EVENT_INSTANCE_READY;
  }
  if (name == HOLON_EVENT_NAME_INSTANCE_EXITED) {
    return HOLON_EVENT_INSTANCE_EXITED;
  }
  if (name == HOLON_EVENT_NAME_INSTANCE_CRASHED) {
    return HOLON_EVENT_INSTANCE_CRASHED;
  }
  if (name == HOLON_EVENT_NAME_SESSION_STARTED) {
    return HOLON_EVENT_SESSION_STARTED;
  }
  if (name == HOLON_EVENT_NAME_SESSION_ENDED) {
    return HOLON_EVENT_SESSION_ENDED;
  }
  if (name == HOLON_EVENT_NAME_HANDLER_PANIC) {
    return HOLON_EVENT_HANDLER_PANIC;
  }
  if (name == HOLON_EVENT_NAME_CONFIG_RELOADED) {
    return HOLON_EVENT_CONFIG_RELOADED;
  }
  return HOLON_EVENT_UNSPECIFIED;
}

holons::observability::Field c_field_to_cpp(const holons_field_t &field) {
  std::string key = field.key != nullptr ? field.key : "";
  switch (field.value.type) {
    case HOLONS_ANYVALUE_BOOL:
      return {std::move(key), field.value.value.bool_value != 0};
    case HOLONS_ANYVALUE_INT:
      return {std::move(key), static_cast<std::int64_t>(field.value.value.int_value)};
    case HOLONS_ANYVALUE_DOUBLE:
      return {std::move(key), field.value.value.double_value};
    case HOLONS_ANYVALUE_STRING:
    default:
      return {std::move(key), field.value.value.string_value != nullptr
                                  ? field.value.value.string_value
                                  : ""};
  }
}

std::vector<holons::observability::Field> c_fields_to_cpp(
    const holons_field_t *fields,
    size_t field_count,
    bool *private_entry) {
  std::vector<holons::observability::Field> out;
  out.reserve(field_count);
  const char *private_key = holon_obs_private();
  for (size_t i = 0; i < field_count; ++i) {
    const char *key = fields != nullptr ? fields[i].key : nullptr;
    if (key == nullptr || key[0] == '\0') {
      continue;
    }
    if (private_key != nullptr && std::strcmp(key, private_key) == 0) {
      if (private_entry != nullptr) {
        *private_entry = true;
      }
      continue;
    }
    out.push_back(c_field_to_cpp(fields[i]));
  }
  return out;
}

int deliver_log_snapshot(const holons::observability::LogRecord &entry,
                         holon_obs_log_snapshot_fn callback,
                         void *user_data) {
  auto fields = snapshot_fields(entry.attributes);
  auto chain = snapshot_chain(entry.chain);
  std::string slug =
      holons::observability::attribute_string(entry, "holons.slug");
  std::string instance_uid =
      holons::observability::attribute_string(entry, "holons.instance_uid");
  std::string logger_name =
      holons::observability::attribute_string(entry, "logger.name");
  std::string message = holons::observability::any_value_string(entry.body);
  holon_obs_log_snapshot_t snapshot;
  std::memset(&snapshot, 0, sizeof(snapshot));
  snapshot.unix_nanos = unix_nanos(entry.timestamp);
  snapshot.level = cpp_level_to_c(entry.level);
  snapshot.slug = slug.c_str();
  snapshot.instance_uid = instance_uid.c_str();
  snapshot.logger_name = logger_name.c_str();
  snapshot.message = message.c_str();
  snapshot.fields = fields.items.empty() ? nullptr : fields.items.data();
  snapshot.field_count = fields.items.size();
  snapshot.chain = chain.items.empty() ? nullptr : chain.items.data();
  snapshot.chain_count = chain.items.size();
  snapshot.private_entry = entry.private_entry ? 1 : 0;
  return callback(&snapshot, user_data);
}

int deliver_event_snapshot(const holons::observability::Event &event,
                           holon_obs_event_snapshot_fn callback,
                           void *user_data) {
  auto payload = snapshot_fields(event.attributes);
  auto chain = snapshot_chain(event.chain);
  std::string slug =
      holons::observability::attribute_string(event, "holons.slug");
  std::string instance_uid =
      holons::observability::attribute_string(event, "holons.instance_uid");
  holon_obs_event_snapshot_t snapshot;
  std::memset(&snapshot, 0, sizeof(snapshot));
  snapshot.unix_nanos = unix_nanos(event.timestamp);
  snapshot.type = cpp_event_name_to_c(event.event_name);
  snapshot.slug = slug.c_str();
  snapshot.instance_uid = instance_uid.c_str();
  snapshot.payload = payload.items.empty() ? nullptr : payload.items.data();
  snapshot.payload_count = payload.items.size();
  snapshot.chain = chain.items.empty() ? nullptr : chain.items.data();
  snapshot.chain_count = chain.items.size();
  snapshot.private_entry = event.private_entry ? 1 : 0;
  return callback(&snapshot, user_data);
}

#if HOLONS_HAS_GRPCPP

struct registration_entry {
  holons_grpc_unary_handler_t handler = nullptr;
  void *ctx = nullptr;
};

struct stream_registration_entry {
  holons_grpc_stream_handler_t handler = nullptr;
  void *ctx = nullptr;
};

class finish_reactor final : public grpc::ServerGenericBidiReactor {
 public:
  explicit finish_reactor(grpc::Status status) { Finish(std::move(status)); }

  void OnDone() override { delete this; }
};

class unary_reactor final : public grpc::ServerGenericBidiReactor {
 public:
  unary_reactor(const registration_entry *entry, std::string method)
      : entry_(entry), method_(std::move(method)) {
    if (entry_ == nullptr || entry_->handler == nullptr) {
      Finish(grpc::Status(grpc::StatusCode::UNIMPLEMENTED,
                          "unimplemented method: " + method_));
      return;
    }
    StartRead(&request_buffer_);
  }

  void OnReadDone(bool ok) override {
    if (!ok) {
      Finish(grpc::Status(grpc::StatusCode::INVALID_ARGUMENT,
                          "missing unary request payload"));
      return;
    }

    std::vector<unsigned char> request_data;
    grpc::Status status;
    if (!copy_request_payload(&request_data, &status)) {
      Finish(status);
      return;
    }

    unsigned char *response_data = nullptr;
    size_t response_len = 0;
    char err[256] = {0};
    int rc = entry_->handler(request_data.data(), request_data.size(),
                             entry_->ctx, &response_data, &response_len, err,
                             sizeof(err));
    if (rc != 0) {
      Finish(grpc::Status(grpc::StatusCode::INTERNAL,
                          err[0] != '\0' ? err : "native C handler failed"));
      return;
    }

    if (response_len > 0 && response_data == nullptr) {
      Finish(grpc::Status(grpc::StatusCode::INTERNAL,
                          "native C handler returned a null response buffer"));
      return;
    }

    if (response_len == 0) {
      std::free(response_data);
      static const char kEmpty = '\0';
      grpc::Slice slice(&kEmpty, 0, grpc::Slice::STATIC_SLICE);
      response_buffer_ = grpc::ByteBuffer(&slice, 1);
    } else {
      grpc::Slice slice(response_data, response_len, std::free);
      response_buffer_ = grpc::ByteBuffer(&slice, 1);
    }

    StartWriteAndFinish(&response_buffer_, grpc::WriteOptions(),
                        grpc::Status::OK);
  }

  void OnDone() override { delete this; }

 private:
  bool copy_request_payload(std::vector<unsigned char> *out,
                            grpc::Status *status) const {
    if (request_buffer_.Length() == 0) {
      out->clear();
      return true;
    }

    grpc::Slice slice;
    grpc::Status dump_status = request_buffer_.DumpToSingleSlice(&slice);
    if (!dump_status.ok()) {
      *status = grpc::Status(grpc::StatusCode::INTERNAL,
                             dump_status.error_message());
      return false;
    }

    out->assign(slice.begin(), slice.end());
    return true;
  }

  const registration_entry *entry_;
  std::string method_;
  grpc::ByteBuffer request_buffer_;
  grpc::ByteBuffer response_buffer_;
};

class stream_reactor;

struct stream_writer {
  stream_reactor *reactor = nullptr;
};

class stream_reactor final : public grpc::ServerGenericBidiReactor {
 public:
  stream_reactor(grpc::GenericCallbackServerContext *server_context,
                 const stream_registration_entry *entry,
                 std::string method)
      : server_context_(server_context), entry_(entry), method_(std::move(method)) {
    if (entry_ == nullptr || entry_->handler == nullptr) {
      Finish(grpc::Status(grpc::StatusCode::UNIMPLEMENTED,
                          "unimplemented method: " + method_));
      return;
    }
    StartRead(&request_buffer_);
  }

  void OnReadDone(bool ok) override {
    if (!ok) {
      Finish(grpc::Status(grpc::StatusCode::INVALID_ARGUMENT,
                          "missing streaming request payload"));
      return;
    }

    grpc::Status status;
    if (!copy_request_payload(&request_data_, &status)) {
      Finish(status);
      return;
    }

    // The C stream context is valid only while this worker runs. Keeping the
    // handler off the reactor callback lets holons_grpc_stream_write wait for
    // OnWriteDone without blocking the same gRPC reaction.
    worker_ = std::thread([this]() { run_handler(); });
  }

  void OnWriteDone(bool ok) override {
    {
      std::scoped_lock lk(mu_);
      write_ok_ = ok;
      write_in_flight_ = false;
      if (!ok) {
        cancelled_ = true;
      }
    }
    cv_.notify_all();
  }

  void OnCancel() override {
    {
      std::scoped_lock lk(mu_);
      cancelled_ = true;
      write_in_flight_ = false;
    }
    cv_.notify_all();
  }

  void OnDone() override {
    if (worker_.joinable()) {
      if (worker_.get_id() == std::this_thread::get_id()) {
        worker_.detach();
      } else {
        worker_.join();
      }
    }
    delete this;
  }

  int write(const void *buf, size_t len) {
    if (len > 0 && buf == nullptr) {
      return -1;
    }

    std::unique_lock<std::mutex> lk(mu_);
    if (cancelled_ || finished_) {
      return 1;
    }
    cv_.wait(lk, [this]() { return !write_in_flight_ || cancelled_ || finished_; });
    if (cancelled_ || finished_) {
      return 1;
    }

    if (len == 0) {
      static const char kEmpty = '\0';
      grpc::Slice slice(&kEmpty, 0, grpc::Slice::STATIC_SLICE);
      write_buffer_ = grpc::ByteBuffer(&slice, 1);
    } else {
      std::string bytes(static_cast<const char *>(buf), len);
      grpc::Slice slice(bytes);
      write_buffer_ = grpc::ByteBuffer(&slice, 1);
    }
    write_in_flight_ = true;
    write_ok_ = false;
    StartWrite(&write_buffer_);
    cv_.wait(lk, [this]() { return !write_in_flight_ || cancelled_ || finished_; });
    return write_ok_ ? 0 : 1;
  }

  bool cancelled() const {
    std::scoped_lock lk(mu_);
    return cancelled_ || finished_ ||
           (server_context_ != nullptr && server_context_->IsCancelled());
  }

 private:
  bool copy_request_payload(std::vector<unsigned char> *out,
                            grpc::Status *status) const {
    if (request_buffer_.Length() == 0) {
      out->clear();
      return true;
    }

    grpc::Slice slice;
    grpc::Status dump_status = request_buffer_.DumpToSingleSlice(&slice);
    if (!dump_status.ok()) {
      *status = grpc::Status(grpc::StatusCode::INTERNAL,
                             dump_status.error_message());
      return false;
    }

    out->assign(slice.begin(), slice.end());
    return true;
  }

  void run_handler() {
    char err[256] = {0};
    stream_writer writer{this};
    holons_grpc_call_ctx_t call_ctx;
    std::memset(&call_ctx, 0, sizeof(call_ctx));
    call_ctx.request_data = request_data_.empty() ? nullptr : request_data_.data();
    call_ctx.request_len = request_data_.size();
    call_ctx.stream_writer = &writer;
    call_ctx.server_context = server_context_;
    call_ctx.err = err;
    call_ctx.err_len = sizeof(err);

    int rc = entry_->handler(&call_ctx, entry_->ctx);
    grpc::Status status = grpc::Status::OK;
    if (rc != 0 && !cancelled()) {
      status = grpc::Status(grpc::StatusCode::INTERNAL,
                            err[0] != '\0' ? err : "native C stream handler failed");
    }

    {
      std::unique_lock<std::mutex> lk(mu_);
      cv_.wait(lk, [this]() { return !write_in_flight_ || cancelled_; });
      finished_ = true;
    }
    cv_.notify_all();
    Finish(status);
  }

  grpc::GenericCallbackServerContext *server_context_ = nullptr;
  const stream_registration_entry *entry_ = nullptr;
  std::string method_;
  grpc::ByteBuffer request_buffer_;
  std::vector<unsigned char> request_data_;
  grpc::ByteBuffer write_buffer_;
  std::thread worker_;
  mutable std::mutex mu_;
  std::condition_variable cv_;
  bool write_in_flight_ = false;
  bool write_ok_ = false;
  bool cancelled_ = false;
  bool finished_ = false;
};

class native_generic_service final : public grpc::CallbackGenericService {
 public:
  explicit native_generic_service(
      const holons_grpc_unary_registration_t *registrations,
      size_t registration_count,
      const holons_grpc_stream_registration_t *stream_registrations,
      size_t stream_registration_count) {
    for (size_t i = 0; i < registration_count; ++i) {
      const auto &registration = registrations[i];
      if (registration.full_method == nullptr ||
          registration.full_method[0] == '\0' || registration.handler == nullptr) {
        continue;
      }
      registrations_.emplace(std::string(registration.full_method),
                             registration_entry{registration.handler,
                                                registration.ctx});
    }
    for (size_t i = 0; i < stream_registration_count; ++i) {
      const auto &registration = stream_registrations[i];
      if (registration.full_method == nullptr ||
          registration.full_method[0] == '\0' || registration.handler == nullptr) {
        continue;
      }
      stream_registrations_.emplace(
          std::string(registration.full_method),
          stream_registration_entry{registration.handler, registration.ctx});
    }
  }

  grpc::ServerGenericBidiReactor *CreateReactor(
      grpc::GenericCallbackServerContext *ctx) override {
    if (ctx == nullptr) {
      return new finish_reactor(grpc::Status(
          grpc::StatusCode::INTERNAL, "missing generic callback context"));
    }

    auto stream_it = stream_registrations_.find(ctx->method());
    if (stream_it != stream_registrations_.end()) {
      return new stream_reactor(ctx, &stream_it->second, ctx->method());
    }

    auto it = registrations_.find(ctx->method());
    if (it == registrations_.end()) {
      return new finish_reactor(grpc::Status(
          grpc::StatusCode::UNIMPLEMENTED,
          "unimplemented method: " + ctx->method()));
    }

    return new unary_reactor(&it->second, ctx->method());
  }

 private:
  std::unordered_map<std::string, registration_entry> registrations_;
  std::unordered_map<std::string, stream_registration_entry> stream_registrations_;
};

#endif

}  // namespace

struct holons_composite_channel {
  std::shared_ptr<grpc::Channel> channel;
  bool owned = false;
};

struct holons_composite_spawned_member {
  holons::composite::SpawnedMember member;
  holons_composite_channel channel;
};

struct holons_composite_cascade {
  holons::composite::Cascade cascade;
  holons_composite_channel channel;
};

namespace {

std::map<std::string, std::string> c_extra_env_to_map(const char *const *items) {
  return c_kv_to_map(items);
}

std::vector<holons::composite::ChildSpec> c_child_specs(
    const holons_composite_child_spec_t *items,
    size_t count) {
  std::vector<holons::composite::ChildSpec> out;
  out.reserve(count);
  for (size_t i = 0; i < count; ++i) {
    const char *slug = items != nullptr && items[i].slug != nullptr ? items[i].slug : "";
    const char *binary = items != nullptr && items[i].binary != nullptr ? items[i].binary : "";
    out.push_back({slug, binary});
  }
  return out;
}

std::vector<holons::composite::DialOptions> c_dial_options(
    const holons_composite_dial_options_t *items,
    size_t count) {
  std::vector<holons::composite::DialOptions> out;
  if (items == nullptr) {
    return out;
  }
  out.reserve(count);
  for (size_t i = 0; i < count; ++i) {
    if (items[i].has_transitive_observability) {
      out.push_back(holons::composite::WithTransitiveObservability(
          items[i].transitive_observability != 0));
    }
  }
  return out;
}

std::vector<holons::composite::ChainHop> c_chain(
    const holons_composite_child_spec_t *items,
    size_t count) {
  std::vector<holons::composite::ChainHop> out;
  out.reserve(count);
  for (size_t i = 0; i < count; ++i) {
    const char *slug = items != nullptr && items[i].slug != nullptr ? items[i].slug : "";
    const char *uid = items != nullptr && items[i].binary != nullptr ? items[i].binary : "";
    out.push_back({slug, uid});
  }
  return out;
}

void set_check_result(holons_composite_check_result_t *out,
                      const holons::composite::CheckOutcome &result) {
  if (out == nullptr) {
    return;
  }
  out->pass = result.pass ? 1 : 0;
  copy_cstr(out->evidence, sizeof(out->evidence), result.evidence);
}

}  // namespace

extern "C" int holons_grpc_set_observability_options(
    const holons_grpc_observability_options_t *options) {
  std::scoped_lock lk(c_observability_options_mu());
  auto &state = c_observability_options_state();
  state.slug.clear();
  state.members.clear();
  if (options == nullptr) {
    return 0;
  }
  if (options->slug != nullptr) {
    state.slug = options->slug;
  }
  if (options->member_endpoints != nullptr &&
      options->member_endpoint_count > 0) {
    state.members.reserve(options->member_endpoint_count);
    for (size_t i = 0; i < options->member_endpoint_count; ++i) {
      const auto &member = options->member_endpoints[i];
      if (member.slug == nullptr || member.slug[0] == '\0' ||
          member.address == nullptr || member.address[0] == '\0') {
        state.slug.clear();
        state.members.clear();
        return -1;
      }
      state.members.push_back(
          holons::serve::member_ref{member.slug, member.address});
    }
  }
  return 0;
}

extern "C" void holons_grpc_clear_observability_options(void) {
  std::scoped_lock lk(c_observability_options_mu());
  auto &state = c_observability_options_state();
  state.slug.clear();
  state.members.clear();
}

extern "C" void holons_cpp_obs_log_from_c(const char *logger_name,
                                           int level,
                                           const char *message,
                                           const char *const *fields) {
  (void)logger_name;
  (void)level;
  (void)message;
  (void)fields;
}

extern "C" void holons_cpp_obs_log_fields_from_c(
    const char *logger_name,
    int level,
    const char *message,
    const holons_field_t *fields,
    size_t field_count) {
  auto &obs = holons::observability::current();
  if (!obs.enabled(holons::observability::Family::Logs)) {
    return;
  }
  const char *name =
      (logger_name != nullptr && logger_name[0] != '\0') ? logger_name : "c";
  bool private_entry = false;
  auto mapped = c_fields_to_cpp(fields, field_count, &private_entry);
  obs.logger(name).log(c_level_to_cpp(level), message != nullptr ? message : "",
                       mapped, private_entry);
}

extern "C" void holons_cpp_obs_event_from_c(int type,
                                             const char *const *payload) {
  (void)type;
  (void)payload;
}

extern "C" void holons_cpp_obs_event_fields_from_c(
    int type,
    const holons_field_t *payload,
    size_t payload_count) {
  auto &obs = holons::observability::current();
  if (!obs.enabled(holons::observability::Family::Events)) {
    return;
  }
  bool private_entry = false;
  auto mapped = c_fields_to_cpp(payload, payload_count, &private_entry);
  std::unordered_set<std::string> redact(obs.cfg.redacted_fields.begin(),
                                         obs.cfg.redacted_fields.end());
  std::vector<holons::observability::Field> attrs;
  attrs.reserve(mapped.size() + 5);
  attrs.emplace_back("holons.slug", obs.cfg.slug);
  attrs.emplace_back("service.name", obs.cfg.slug);
  attrs.emplace_back("holons.instance_uid", obs.cfg.instance_uid);
  attrs.emplace_back("service.instance.id", obs.cfg.instance_uid);
  attrs.emplace_back("holons.session_id", obs.cfg.session_id);
  for (auto &field : mapped) {
    if (redact.count(field.key)) {
      field.value = std::string{"<redacted>"};
    }
    attrs.push_back(std::move(field));
  }
  holons::observability::LogRecord event;
  event.timestamp = std::chrono::system_clock::now();
  event.level = holons::observability::Level::Info;
  event.event_name = holons::observability::event_name(c_event_to_cpp(type));
  event.body = std::string(event.event_name);
  event.attributes = std::move(attrs);
  event.private_entry = private_entry;
  if (obs.event_bus) {
    obs.event_bus->emit(event);
  }
}

extern "C" void holons_cpp_obs_counter_add_from_c(
    const char *name,
    const char *help,
    const char *const *labels,
    int64_t n) {
  if (name == nullptr || name[0] == '\0' || n < 0) {
    return;
  }
  auto &obs = holons::observability::current();
  if (!obs.enabled(holons::observability::Family::Metrics)) {
    return;
  }
  auto counter =
      obs.counter(name, help != nullptr ? help : "", c_kv_to_map(labels));
  if (counter) {
    counter->add(n);
  }
}

extern "C" int holons_cpp_obs_replay_logs_from_c(
    int follow,
    holon_obs_log_snapshot_fn callback,
    void *user_data) {
  if (callback == nullptr) {
    return -1;
  }
  auto &obs = holons::observability::current();
  if (!obs.enabled(holons::observability::Family::Logs) || !obs.log_ring) {
    return -1;
  }
  if (!follow) {
    for (const auto &entry : obs.log_ring->drain()) {
      if (!deliver_log_snapshot(entry, callback, user_data)) {
        return 0;
      }
    }
    return 0;
  }
  auto queue = std::make_shared<c_follow_queue<holons::observability::LogRecord>>();
  auto replay = obs.log_ring->replay_and_subscribe(
      std::chrono::system_clock::time_point{},
      [queue](const holons::observability::LogRecord &entry) {
        queue->push(entry);
      });
  for (const auto &entry : replay.first) {
    if (!deliver_log_snapshot(entry, callback, user_data)) {
      replay.second();
      queue->close();
      return 0;
    }
  }
  holons::observability::LogRecord entry;
  while (queue->wait_pop(&entry)) {
    if (!deliver_log_snapshot(entry, callback, user_data)) {
      break;
    }
  }
  replay.second();
  queue->close();
  return 0;
}

extern "C" int holons_cpp_obs_replay_events_from_c(
    int follow,
    holon_obs_event_snapshot_fn callback,
    void *user_data) {
  if (callback == nullptr) {
    return -1;
  }
  auto &obs = holons::observability::current();
  if (!obs.enabled(holons::observability::Family::Events) || !obs.event_bus) {
    return -1;
  }
  if (!follow) {
    for (const auto &event : obs.event_bus->drain()) {
      if (!deliver_event_snapshot(event, callback, user_data)) {
        return 0;
      }
    }
    return 0;
  }
  auto queue = std::make_shared<c_follow_queue<holons::observability::Event>>();
  auto replay = obs.event_bus->replay_and_subscribe(
      std::chrono::system_clock::time_point{},
      [queue](const holons::observability::Event &event) {
        queue->push(event);
      });
  for (const auto &event : replay.first) {
    if (!deliver_event_snapshot(event, callback, user_data)) {
      replay.second();
      queue->close();
      return 0;
    }
  }
  holons::observability::Event event;
  while (queue->wait_pop(&event)) {
    if (!deliver_event_snapshot(event, callback, user_data)) {
      break;
    }
  }
  replay.second();
  queue->close();
  return 0;
}

extern "C" int holons_serve_grpc(
    const char *listen_uri,
    const holons_grpc_unary_registration_t *registrations,
    size_t registration_count,
    const holons_grpc_serve_options_t *options,
    char *err,
    size_t err_len) {
  return holons_serve_grpc_with_streams(listen_uri, registrations,
                                        registration_count, nullptr, 0,
                                        options, err, err_len);
}

extern "C" int holons_grpc_stream_write(const holons_grpc_call_ctx_t *ctx,
                                         const void *buf,
                                         size_t len) {
  if (ctx == nullptr || ctx->stream_writer == nullptr) {
    return -1;
  }
#if !HOLONS_HAS_GRPCPP
  (void)buf;
  (void)len;
  return -1;
#else
  auto *writer = static_cast<stream_writer *>(ctx->stream_writer);
  if (writer == nullptr || writer->reactor == nullptr) {
    return -1;
  }
  return writer->reactor->write(buf, len);
#endif
}

extern "C" void holons_composite_with_transitive_observability(
    holons_composite_dial_options_t *out,
    int enabled) {
  if (out == nullptr) {
    return;
  }
  out->has_transitive_observability = 1;
  out->transitive_observability = enabled != 0;
}

extern "C" int holons_composite_spawn_member(
    const holons_composite_spawn_options_t *options,
    holons_composite_spawned_member_t **out,
    char *err,
    size_t err_len) {
  if (options == nullptr || out == nullptr) {
    set_err(err, err_len, "spawn options and output are required");
    return -1;
  }
  *out = nullptr;
  try {
    holons::composite::SpawnOptions cpp_options;
    cpp_options.slug = options->slug != nullptr ? options->slug : "";
    cpp_options.binary_path =
        options->binary_path != nullptr ? options->binary_path : "";
    cpp_options.transport = options->transport != nullptr ? options->transport : "";
    cpp_options.instance_uid =
        options->instance_uid != nullptr ? options->instance_uid : "";
    cpp_options.downstream_chain =
        c_child_specs(options->downstream_chain, options->downstream_chain_count);
    cpp_options.extra_env = c_extra_env_to_map(options->extra_env);
    cpp_options.dial_options =
        c_dial_options(options->dial_options, options->dial_option_count);
    auto handle = std::make_unique<holons_composite_spawned_member>();
    handle->member = holons::composite::SpawnMember(std::move(cpp_options));
    handle->channel.channel = handle->member.channel;
    *out = handle.release();
    return 0;
  } catch (const std::exception &ex) {
    set_err(err, err_len, "%s", ex.what());
    return -1;
  }
}

extern "C" void holons_composite_spawned_member_stop(
    holons_composite_spawned_member_t *member) {
  if (member != nullptr) {
    member->member.stop();
  }
}

extern "C" void holons_composite_spawned_member_free(
    holons_composite_spawned_member_t *member) {
  delete member;
}

extern "C" holons_composite_channel_t *
holons_composite_spawned_member_channel(
    holons_composite_spawned_member_t *member) {
  return member != nullptr ? &member->channel : nullptr;
}

extern "C" const char *holons_composite_spawned_member_uid(
    const holons_composite_spawned_member_t *member) {
  return member != nullptr ? member->member.uid.c_str() : "";
}

extern "C" const char *holons_composite_spawned_member_listen_uri(
    const holons_composite_spawned_member_t *member) {
  return member != nullptr ? member->member.listen_uri.c_str() : "";
}

extern "C" int holons_composite_build_cascade(
    const holons_composite_cascade_options_t *options,
    holons_composite_cascade_t **out,
    char *err,
    size_t err_len) {
  if (options == nullptr || out == nullptr) {
    set_err(err, err_len, "cascade options and output are required");
    return -1;
  }
  *out = nullptr;
  try {
    holons::composite::CascadeOptions cpp_options;
    cpp_options.transport =
        options->transport != nullptr ? options->transport : "";
    cpp_options.members = c_child_specs(options->members, options->member_count);
    cpp_options.extra_env = c_extra_env_to_map(options->extra_env);
    auto handle = std::make_unique<holons_composite_cascade>();
    handle->cascade = holons::composite::BuildCascade(std::move(cpp_options));
    if (handle->cascade.top) {
      handle->channel.channel = handle->cascade.top->channel;
    }
    *out = handle.release();
    return 0;
  } catch (const std::exception &ex) {
    set_err(err, err_len, "%s", ex.what());
    return -1;
  }
}

extern "C" void holons_composite_cascade_stop(
    holons_composite_cascade_t *cascade) {
  if (cascade != nullptr) {
    cascade->cascade.stop();
  }
}

extern "C" void holons_composite_cascade_free(
    holons_composite_cascade_t *cascade) {
  delete cascade;
}

extern "C" holons_composite_channel_t *
holons_composite_cascade_top_channel(holons_composite_cascade_t *cascade) {
  return cascade != nullptr ? &cascade->channel : nullptr;
}

extern "C" const char *holons_composite_cascade_top_uid(
    const holons_composite_cascade_t *cascade) {
  if (cascade == nullptr || !cascade->cascade.top) {
    return "";
  }
  return cascade->cascade.top->uid.c_str();
}

extern "C" int holons_composite_dial(
    const char *address,
    const holons_composite_dial_options_t *options,
    size_t option_count,
    holons_composite_channel_t **out,
    char *err,
    size_t err_len) {
  if (address == nullptr || out == nullptr) {
    set_err(err, err_len, "address and output are required");
    return -1;
  }
  *out = nullptr;
  try {
    auto handle = std::make_unique<holons_composite_channel>();
    handle->owned = true;
    handle->channel = holons::composite::Dial(
        address, c_dial_options(options, option_count));
    *out = handle.release();
    return 0;
  } catch (const std::exception &ex) {
    set_err(err, err_len, "%s", ex.what());
    return -1;
  }
}

extern "C" void holons_composite_channel_free(
    holons_composite_channel_t *channel) {
  if (channel != nullptr && channel->owned) {
    delete channel;
  }
}

extern "C" int holons_composite_check_relayed_log(
    const holons_composite_log_check_options_t *options,
    holons_composite_check_result_t *out) {
  if (options == nullptr || out == nullptr) {
    return -1;
  }
  auto result = holons::composite::CheckRelayedLog(
      holons::composite::LogCheckOptions{
          options->channel != nullptr ? options->channel->channel : nullptr,
          options->sender != nullptr ? options->sender : "",
          options->leaf_uid != nullptr ? options->leaf_uid : "",
          c_chain(options->expected_chain, options->expected_chain_count),
          std::chrono::milliseconds(options->timeout_ms),
          std::chrono::milliseconds(options->poll_interval_ms),
          options->live != 0,
      });
  set_check_result(out, result);
  return result.pass ? 0 : 1;
}

extern "C" int holons_composite_check_relayed_event(
    const holons_composite_event_check_options_t *options,
    holons_composite_check_result_t *out) {
  if (options == nullptr || out == nullptr) {
    return -1;
  }
  auto result = holons::composite::CheckRelayedEvent(
      holons::composite::EventCheckOptions{
          options->channel != nullptr ? options->channel->channel : nullptr,
          c_event_to_cpp(options->event_type),
          options->leaf_uid != nullptr ? options->leaf_uid : "",
          c_chain(options->expected_chain, options->expected_chain_count),
          std::chrono::milliseconds(options->timeout_ms),
          std::chrono::milliseconds(options->poll_interval_ms),
          options->live != 0,
      });
  set_check_result(out, result);
  return result.pass ? 0 : 1;
}

extern "C" int holons_composite_parse_child_flags(
    int argc,
    char **argv,
    holons_composite_child_spec_t **out_children,
    size_t *out_child_count,
    char ***out_remaining,
    size_t *out_remaining_count,
    char *err,
    size_t err_len) {
  if (out_children == nullptr || out_child_count == nullptr ||
      out_remaining == nullptr || out_remaining_count == nullptr) {
    set_err(err, err_len, "parse outputs are required");
    return -1;
  }
  *out_children = nullptr;
  *out_child_count = 0;
  *out_remaining = nullptr;
  *out_remaining_count = 0;
  try {
    std::vector<std::string> args;
    args.reserve(argc > 0 ? static_cast<size_t>(argc) : 0);
    for (int i = 0; i < argc; ++i) {
      args.emplace_back(argv != nullptr && argv[i] != nullptr ? argv[i] : "");
    }
    auto parsed = holons::composite::ParseChildFlags(args);

    auto children = static_cast<holons_composite_child_spec_t *>(
        std::calloc(parsed.first.size(), sizeof(holons_composite_child_spec_t)));
    auto remaining = static_cast<char **>(
        std::calloc(parsed.second.size(), sizeof(char *)));
    if ((parsed.first.size() > 0 && children == nullptr) ||
        (parsed.second.size() > 0 && remaining == nullptr)) {
      std::free(children);
      std::free(remaining);
      set_err(err, err_len, "out of memory");
      return -1;
    }
    for (size_t i = 0; i < parsed.first.size(); ++i) {
      children[i].slug = ::strdup(parsed.first[i].slug.c_str());
      children[i].binary = ::strdup(parsed.first[i].binary.c_str());
      if (children[i].slug == nullptr || children[i].binary == nullptr) {
        holons_composite_free_child_flags(children, parsed.first.size(),
                                         remaining, parsed.second.size());
        set_err(err, err_len, "out of memory");
        return -1;
      }
    }
    for (size_t i = 0; i < parsed.second.size(); ++i) {
      remaining[i] = ::strdup(parsed.second[i].c_str());
      if (remaining[i] == nullptr) {
        holons_composite_free_child_flags(children, parsed.first.size(),
                                         remaining, parsed.second.size());
        set_err(err, err_len, "out of memory");
        return -1;
      }
    }
    *out_children = children;
    *out_child_count = parsed.first.size();
    *out_remaining = remaining;
    *out_remaining_count = parsed.second.size();
    return 0;
  } catch (const std::exception &ex) {
    set_err(err, err_len, "%s", ex.what());
    return -1;
  }
}

extern "C" void holons_composite_free_child_flags(
    holons_composite_child_spec_t *children,
    size_t child_count,
    char **remaining,
    size_t remaining_count) {
  for (size_t i = 0; i < child_count; ++i) {
    std::free(const_cast<char *>(children[i].slug));
    std::free(const_cast<char *>(children[i].binary));
  }
  std::free(children);
  for (size_t i = 0; i < remaining_count; ++i) {
    std::free(remaining[i]);
  }
  std::free(remaining);
}

extern "C" int holons_grpc_stream_check_cancel(
    const holons_grpc_call_ctx_t *ctx) {
  if (ctx == nullptr || ctx->stream_writer == nullptr) {
    return 1;
  }
#if !HOLONS_HAS_GRPCPP
  return 1;
#else
  auto *writer = static_cast<stream_writer *>(ctx->stream_writer);
  if (writer == nullptr || writer->reactor == nullptr) {
    return 1;
  }
  return writer->reactor->cancelled() ? 1 : 0;
#endif
}

extern "C" int holons_serve_grpc_with_streams(
    const char *listen_uri,
    const holons_grpc_unary_registration_t *registrations,
    size_t registration_count,
    const holons_grpc_stream_registration_t *stream_registrations,
    size_t stream_registration_count,
    const holons_grpc_serve_options_t *options,
    char *err,
    size_t err_len) {
#if !HOLONS_HAS_GRPCPP
  (void)listen_uri;
  (void)registrations;
  (void)registration_count;
  (void)stream_registrations;
  (void)stream_registration_count;
  (void)options;
  set_err(err, err_len, "grpc++ headers are required for native C serve");
  return -1;
#else
  if ((registrations == nullptr || registration_count == 0) &&
      (stream_registrations == nullptr || stream_registration_count == 0)) {
    set_err(err, err_len, "at least one gRPC registration is required");
    return -1;
  }

  const char *effective_listen_uri =
      (listen_uri != nullptr && listen_uri[0] != '\0') ? listen_uri
                                                       : HOLONS_DEFAULT_URI;

  holons::serve::options serve_options;
  serve_options.auto_register_holon_meta = false;
  serve_options.announce = true;
  {
    std::scoped_lock lk(c_observability_options_mu());
    const auto &state = c_observability_options_state();
    serve_options.slug = state.slug;
    serve_options.member_endpoints = state.members;
  }
  if (options != nullptr) {
    serve_options.enable_reflection = options->enable_reflection != 0;
    serve_options.announce = options->announce != 0;
    if (options->graceful_shutdown_timeout_ms > 0) {
      serve_options.graceful_shutdown_timeout_ms =
          options->graceful_shutdown_timeout_ms;
    }
  }

  try {
    auto service = std::make_shared<native_generic_service>(
        registrations, registration_count, stream_registrations,
        stream_registration_count);
    holons::serve::serve(
        std::string(effective_listen_uri),
        [service](grpc::ServerBuilder &builder) {
          builder.RegisterCallbackGenericService(service.get());
        },
        serve_options,
        {std::static_pointer_cast<void>(service)});
    return 0;
  } catch (const std::exception &ex) {
    set_err(err, err_len, "%s", ex.what());
    return -1;
  } catch (...) {
    set_err(err, err_len, "unknown native C serve failure");
    return -1;
  }
#endif
}
