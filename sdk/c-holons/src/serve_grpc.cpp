#include "holons/holons.h"

#include "holons/serve.hpp"

#if HOLONS_HAS_GRPCPP
#include <grpcpp/generic/callback_generic_service.h>
#endif

#include <cstdarg>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

namespace {

void set_err(char *err, size_t err_len, const char *fmt, ...) {
  va_list ap;

  if (err == nullptr || err_len == 0) {
    return;
  }

  va_start(ap, fmt);
  std::vsnprintf(err, err_len, fmt, ap);
  va_end(ap);
}

#if HOLONS_HAS_GRPCPP

struct registration_entry {
  holons_grpc_unary_handler_t handler = nullptr;
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

class native_generic_service final : public grpc::CallbackGenericService {
 public:
  explicit native_generic_service(
      const holons_grpc_unary_registration_t *registrations,
      size_t registration_count) {
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
  }

  grpc::ServerGenericBidiReactor *CreateReactor(
      grpc::GenericCallbackServerContext *ctx) override {
    if (ctx == nullptr) {
      return new finish_reactor(grpc::Status(
          grpc::StatusCode::INTERNAL, "missing generic callback context"));
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
};

#endif

}  // namespace

extern "C" int holons_serve_grpc(
    const char *listen_uri,
    const holons_grpc_unary_registration_t *registrations,
    size_t registration_count,
    const holons_grpc_serve_options_t *options,
    char *err,
    size_t err_len) {
#if !HOLONS_HAS_GRPCPP
  (void)listen_uri;
  (void)registrations;
  (void)registration_count;
  (void)options;
  set_err(err, err_len, "grpc++ headers are required for native C serve");
  return -1;
#else
  if (registrations == nullptr || registration_count == 0) {
    set_err(err, err_len, "at least one gRPC registration is required");
    return -1;
  }

  const char *effective_listen_uri =
      (listen_uri != nullptr && listen_uri[0] != '\0') ? listen_uri
                                                       : HOLONS_DEFAULT_URI;

  holons::serve::options serve_options;
  serve_options.auto_register_holon_meta = false;
  serve_options.announce = true;
  if (options != nullptr) {
    serve_options.enable_reflection = options->enable_reflection != 0;
    serve_options.announce = options->announce != 0;
    if (options->graceful_shutdown_timeout_ms > 0) {
      serve_options.graceful_shutdown_timeout_ms =
          options->graceful_shutdown_timeout_ms;
    }
  }

  try {
    auto service = std::make_shared<native_generic_service>(registrations,
                                                            registration_count);
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
