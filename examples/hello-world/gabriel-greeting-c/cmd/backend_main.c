#include "internal/server.h"

#include "holons/holons.h"

#include <stdio.h>
#include <string.h>

int main(int argc, char **argv) {
  char listen_uri[HOLONS_MAX_URI_LEN];

  if (argc < 2 || strcmp(argv[1], "serve") != 0) {
    fprintf(stderr, "usage: gabriel-greeting-c-backend serve [--listen <uri>]\n");
    return 1;
  }

  if (holons_parse_flags(argc - 1, argv + 1, listen_uri, sizeof(listen_uri)) != 0) {
    fprintf(stderr, "backend: failed to parse serve flags\n");
    return 1;
  }

  return gabriel_greeting_c_backend_serve(listen_uri, stdout, stderr);
}
