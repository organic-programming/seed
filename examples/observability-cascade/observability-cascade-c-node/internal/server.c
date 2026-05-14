#include "internal/server.h"

#include "holons/observability.h"
#include "relay.upb.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

const holons_describe_response_t *holons_generated_describe_response(void);

typedef struct observability_cascade_node_handlers {
  void *ctx;
  relay_v1_TickResponse *(*tick)(const relay_v1_TickRequest *request,
                                 upb_Arena *arena,
                                 void *ctx);
} observability_cascade_node_handlers_t;

int observability_cascade_node_generated_serve(
    const char *listen_uri,
    const observability_cascade_node_handlers_t *handlers,
    const holons_grpc_serve_options_t *options,
    char *err,
    size_t err_len);

static upb_StringView arena_copy_cstr(upb_Arena *arena, const char *value) {
  size_t len;
  char *copy;
  if (value == NULL) {
    value = "";
  }
  len = strlen(value);
  copy = (char *)upb_Arena_Malloc(arena, len + 1);
  if (copy == NULL) {
    return upb_StringView_FromDataAndSize("", 0);
  }
  memcpy(copy, value, len);
  copy[len] = '\0';
  return upb_StringView_FromDataAndSize(copy, len);
}

static void copy_string_view(upb_StringView view, char *out, size_t out_len) {
  size_t n;
  if (out == NULL || out_len == 0) {
    return;
  }
  n = view.size < out_len - 1 ? view.size : out_len - 1;
  if (view.data != NULL && n > 0) {
    memcpy(out, view.data, n);
  }
  out[n] = '\0';
}

static const char *instance_uid(void) {
  const char *uid = getenv("OP_INSTANCE_UID");
  return uid != NULL ? uid : "";
}

static relay_v1_TickResponse *cascade_node_c_handle_tick(
    const relay_v1_TickRequest *request, upb_Arena *arena, void *ctx) {
  relay_v1_TickResponse *response;
  char sender[256];
  char note[256];
  const char *uid;
  const char *fields[9];
  const char *labels[3];
  (void)ctx;

  if (request == NULL) {
    return NULL;
  }

  sender[0] = '\0';
  note[0] = '\0';
  copy_string_view(relay_v1_TickRequest_sender(request), sender, sizeof(sender));
  copy_string_view(relay_v1_TickRequest_note(request), note, sizeof(note));
  uid = instance_uid();

  fields[0] = "sender";
  fields[1] = sender;
  fields[2] = "note";
  fields[3] = note;
  fields[4] = "responder_slug";
  fields[5] = "observability-cascade-node-c";
  fields[6] = "responder_uid";
  fields[7] = uid;
  fields[8] = NULL;
  holon_obs_log_named("tick", HOLON_LEVEL_INFO, "tick received", fields);

  labels[0] = "responder_uid";
  labels[1] = uid;
  labels[2] = NULL;
  holon_obs_counter_inc_with_help(
      "cascade_ticks_total", "Ticks received by this cascade node.", labels);

  response = relay_v1_TickResponse_new(arena);
  if (response == NULL) {
    return NULL;
  }
  relay_v1_TickResponse_set_responder_slug(
      response, arena_copy_cstr(arena, "observability-cascade-node-c"));
  relay_v1_TickResponse_set_responder_instance_uid(
      response, arena_copy_cstr(arena, uid));
  return response;
}

int cascade_node_c_serve(const char *listen_uri,
                         const holons_grpc_member_ref_t *members,
                         size_t member_count,
                         FILE *stderr_stream) {
  observability_cascade_node_handlers_t handlers;
  holons_grpc_serve_options_t serve_options;
  holons_grpc_observability_options_t obs_options;
  holon_obs_config_t obs_config;
  char err[256];
  int rc;

  holons_use_static_describe_response(holons_generated_describe_response());

  memset(&handlers, 0, sizeof(handlers));
  handlers.tick = cascade_node_c_handle_tick;

  memset(&serve_options, 0, sizeof(serve_options));
  serve_options.announce = 1;
  serve_options.enable_reflection = 0;
  serve_options.graceful_shutdown_timeout_ms = 10000;

  memset(&obs_options, 0, sizeof(obs_options));
  obs_options.slug = "observability-cascade-node-c";
  obs_options.member_endpoints = members;
  obs_options.member_endpoint_count = member_count;
  if (holons_grpc_set_observability_options(&obs_options) != 0) {
    fprintf(stderr_stream, "serve: invalid member endpoint\n");
    return 1;
  }

  memset(&obs_config, 0, sizeof(obs_config));
  obs_config.slug = "observability-cascade-node-c";
  obs_config.instance_uid = getenv("OP_INSTANCE_UID");
  obs_config.organism_uid = getenv("OP_ORGANISM_UID");
  obs_config.organism_slug = getenv("OP_ORGANISM_SLUG");
  obs_config.run_dir = getenv("OP_RUN_DIR");
  obs_config.default_log_level = HOLON_LEVEL_INFO;
  holon_obs_configure(&obs_config);

  rc = observability_cascade_node_generated_serve(
      listen_uri, &handlers, &serve_options, err, sizeof(err));
  holons_grpc_clear_observability_options();
  if (rc != 0) {
    fprintf(stderr_stream, "serve: %s\n", err);
    return 1;
  }
  return 0;
}
