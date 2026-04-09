#include "internal/server.h"

#include "api/public.h"
#include "gen/serve_generated.h"
#include "holons/holons.h"

#include <errno.h>
#include <stdio.h>
#include <string.h>

const holons_describe_response_t *holons_generated_describe_response(void);

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
  (void)ctx;
  return gabriel_greeting_c_say_hello(request, arena);
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
