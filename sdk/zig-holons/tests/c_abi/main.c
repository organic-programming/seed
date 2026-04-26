#include "holons_sdk.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int fail_status(const char *label, holons_status status) {
  fprintf(stderr, "%s failed: %s (%d)\n", label, holons_status_message(status), (int)status);
  return 1;
}

int main(int argc, char **argv) {
  const char *uri = argc > 1 ? argv[1] : getenv("HOLONS_C_ABI_TARGET");
  if (uri == NULL || uri[0] == '\0') {
    fprintf(stderr, "usage: %s tcp://127.0.0.1:<port>\n", argv[0]);
    return 2;
  }

  holons_sdk_context *ctx = NULL;
  holons_status status = holons_sdk_init(&ctx);
  if (status != HOLONS_STATUS_OK) {
    return fail_status("holons_sdk_init", status);
  }

  holons_connection *connection = NULL;
  status = holons_connect(ctx, uri, &connection);
  if (status != HOLONS_STATUS_OK) {
    holons_sdk_shutdown(ctx);
    return fail_status("holons_connect", status);
  }

  holons_string_result describe = holons_connection_describe_json(connection);
  if (describe.status != HOLONS_STATUS_OK) {
    fprintf(stderr, "describe failed: %s\n",
            describe.error_message != NULL ? describe.error_message : holons_status_message(describe.status));
    holons_string_result_free(&describe);
    holons_connection_close(connection);
    holons_sdk_shutdown(ctx);
    return 1;
  }

  if (describe.data == NULL || strstr(describe.data, "Greeting-Go") == NULL) {
    fprintf(stderr, "describe payload missing Greeting-Go: %s\n",
            describe.data != NULL ? describe.data : "(null)");
    holons_string_result_free(&describe);
    holons_connection_close(connection);
    holons_sdk_shutdown(ctx);
    return 1;
  }

  puts(describe.data);
  holons_string_result_free(&describe);
  holons_connection_close(connection);
  holons_sdk_shutdown(ctx);
  return 0;
}
