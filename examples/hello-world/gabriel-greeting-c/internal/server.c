#include "internal/server.h"

#include "api/public.h"
#include "gen/serve_generated.h"
#include "holons/holons.h"
#include "holons/observability.h"
#include "internal/greetings.h"

#include <ctype.h>
#include <errno.h>
#include <inttypes.h>
#include <stdio.h>
#include <string.h>
#include <time.h>

const holons_describe_response_t *holons_generated_describe_response(void);

static int64_t monotonic_nanos(void) {
  struct timespec ts;
  clock_gettime(CLOCK_MONOTONIC, &ts);
  return (int64_t)ts.tv_sec * 1000000000LL + (int64_t)ts.tv_nsec;
}

static upb_StringView trim_view(upb_StringView value) {
  size_t start = 0;
  size_t end = value.size;

  while (start < end && isspace((unsigned char)value.data[start])) {
    ++start;
  }
  while (end > start && isspace((unsigned char)value.data[end - 1])) {
    --end;
  }
  return upb_StringView_FromDataAndSize(value.data + start, end - start);
}

static const char *arena_copy_view_z(upb_Arena *arena, upb_StringView value) {
  char *copy = (char *)upb_Arena_Malloc(arena, value.size + 1);
  if (copy == NULL) {
    return "";
  }
  memcpy(copy, value.data, value.size);
  copy[value.size] = '\0';
  return copy;
}

static const char *arena_format_greeting_message(upb_Arena *arena,
                                                 const char *name,
                                                 const char *language,
                                                 const char *lang_code) {
  const int len =
      snprintf(NULL, 0, "Greeted %s in %s (%s)", name, language, lang_code);
  char *message;
  if (len < 0) {
    return "Greeted";
  }
  message = (char *)upb_Arena_Malloc(arena, (size_t)len + 1);
  if (message == NULL) {
    return "Greeted";
  }
  snprintf(message, (size_t)len + 1, "Greeted %s in %s (%s)", name, language,
           lang_code);
  return message;
}

static const char *resolved_name(const greeting_v1_SayHelloRequest *request,
                                 upb_Arena *arena) {
  const gabriel_greeting_c_greeting *entry =
      gabriel_greeting_c_lookup(greeting_v1_SayHelloRequest_lang_code(request));
  upb_StringView name = trim_view(greeting_v1_SayHelloRequest_name(request));
  if (name.size == 0) {
    return entry->default_name;
  }
  return arena_copy_view_z(arena, name);
}

static void emit_greeting_observability(
    const greeting_v1_SayHelloRequest *request,
    const greeting_v1_SayHelloResponse *response, upb_Arena *arena,
    int64_t start_ns) {
  /* C Serve does not yet expose a handler-visible current transport. */
  const char *transport = "unknown";
  const char *lang_code =
      arena_copy_view_z(arena, greeting_v1_SayHelloResponse_lang_code(response));
  const char *language =
      arena_copy_view_z(arena, greeting_v1_SayHelloResponse_language(response));
  const char *greeting =
      arena_copy_view_z(arena, greeting_v1_SayHelloResponse_greeting(response));
  const char *name = resolved_name(request, arena);
  const int64_t duration_ns = monotonic_nanos() - start_ns;
  char duration_text[32];
  const char *message =
      arena_format_greeting_message(arena, name, language, lang_code);
  const char *fields[13];
  const char *labels[7];

  snprintf(duration_text, sizeof(duration_text), "%" PRId64, duration_ns);

  fields[0] = "lang_code";
  fields[1] = lang_code;
  fields[2] = "language";
  fields[3] = language;
  fields[4] = "name";
  fields[5] = name;
  fields[6] = "greeting";
  fields[7] = greeting;
  fields[8] = "transport";
  fields[9] = transport;
  fields[10] = "duration_ns";
  fields[11] = duration_text;
  fields[12] = NULL;
  holon_obs_log_named("greeting", HOLON_LEVEL_INFO, message, fields);

  labels[0] = "lang_code";
  labels[1] = lang_code;
  labels[2] = "language";
  labels[3] = language;
  labels[4] = "transport";
  labels[5] = transport;
  labels[6] = NULL;
  holon_obs_counter_inc_with_help(
      "greeting_emitted_total",
      "Greetings emitted, partitioned by language and transport.", labels);
}

static greeting_v1_ListLanguagesResponse *
gabriel_greeting_c_handle_list_languages(
    const greeting_v1_ListLanguagesRequest *request, upb_Arena *arena,
    void *ctx) {
  (void)request;
  (void)ctx;
  return gabriel_greeting_c_list_languages(arena);
}

static greeting_v1_SayHelloResponse *gabriel_greeting_c_handle_say_hello(
    const greeting_v1_SayHelloRequest *request, upb_Arena *arena, void *ctx) {
  int64_t start_ns = monotonic_nanos();
  greeting_v1_SayHelloResponse *response;
  (void)ctx;
  response = gabriel_greeting_c_say_hello(request, arena);
  if (response != NULL) {
    emit_greeting_observability(request, response, arena, start_ns);
  }
  return response;
}

int gabriel_greeting_c_serve(const char *listen_uri, FILE *stderr_stream) {
  gabriel_greeting_c_handlers_t handlers;
  holons_grpc_serve_options_t options;
  char err[256];
  int rc;

  holons_use_static_describe_response(holons_generated_describe_response());

  handlers.ctx = NULL;
  handlers.listLanguages = gabriel_greeting_c_handle_list_languages;
  handlers.sayHello = gabriel_greeting_c_handle_say_hello;

  options.announce = 1;
  options.enable_reflection = 0;
  options.graceful_shutdown_timeout_ms = 10000;

  rc = gabriel_greeting_c_generated_serve(listen_uri, &handlers, &options, err,
                                          sizeof(err));

  if (rc != 0) {
    fprintf(stderr_stream, "serve: %s\n", err);
    return 1;
  }
  return 0;
}
