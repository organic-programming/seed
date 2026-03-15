#include "holons/holons.h"
#include "hello.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static void print_usage(void) {
  fprintf(stderr,
          "c-hello-world\n"
          "\n"
          "Usage:\n"
          "  hello greet [name]\n"
          "  hello serve [--listen <URI>] [--port <N>] [--once | --max <N>]\n"
          "  hello version\n"
          "\n"
          "Supported transports:\n"
          "  tcp://<host>:<port>\n"
          "  unix://<path>\n"
          "  stdio://\n"
          "  mem://\n"
          "  ws://<host>:<port>[/path]\n"
          "  wss://<host>:<port>[/path]\n");
}

static void trim_line_end(char *s) {
  size_t n = strlen(s);
  while (n > 0 && isspace((unsigned char)s[n - 1])) {
    s[n - 1] = '\0';
    --n;
  }
}

static int greet_connection(const holons_conn_t *conn, void *ctx) {
  char input[128];
  char output[256];
  ssize_t n;

  (void)ctx;

  n = holons_conn_read(conn, input, sizeof(input) - 1);
  if (n <= 0) {
    return 0;
  }
  input[n] = '\0';
  trim_line_end(input);

  if (hello_greet(input, output, sizeof(output)) != 0) {
    return 1;
  }

  if (holons_conn_write(conn, output, strlen(output)) < 0) {
    return 1;
  }
  if (holons_conn_write(conn, "\n", 1) < 0) {
    return 1;
  }
  return 0;
}

static int parse_max_connections(int argc, char **argv, int *out_max) {
  int i;
  int max_connections = 0;

  for (i = 0; i < argc; ++i) {
    if (strcmp(argv[i], "--once") == 0) {
      max_connections = 1;
      continue;
    }
    if (strcmp(argv[i], "--max") == 0 && i + 1 < argc) {
      char *end = NULL;
      long value = strtol(argv[i + 1], &end, 10);
      if (end == argv[i + 1] || *end != '\0' || value <= 0 || value > 1000000) {
        return -1;
      }
      max_connections = (int)value;
      ++i;
    }
  }

  *out_max = max_connections;
  return 0;
}

int main(int argc, char **argv) {
  char response[256];
  char listen_uri[HOLONS_MAX_URI_LEN];
  char err[256];
  int max_connections = 0;
  const char *cmd;

  if (argc < 2) {
    print_usage();
    return 1;
  }

  cmd = argv[1];

  if (strcmp(cmd, "greet") == 0) {
    const char *name = argc >= 3 ? argv[2] : "World";
    if (hello_greet(name, response, sizeof(response)) != 0) {
      fprintf(stderr, "hello: failed to build greeting\n");
      return 1;
    }
    printf("%s\n", response);
    return 0;
  }

  if (strcmp(cmd, "serve") == 0) {
    if (holons_parse_flags(argc - 2, argv + 2, listen_uri, sizeof(listen_uri)) != 0) {
      fprintf(stderr, "hello serve: invalid --listen/--port flags\n");
      return 1;
    }
    if (parse_max_connections(argc - 2, argv + 2, &max_connections) != 0) {
      fprintf(stderr, "hello serve: invalid --max value\n");
      return 1;
    }

    fprintf(stderr, "hello serve listening on %s\n", listen_uri);
    if (holons_serve(listen_uri, greet_connection, NULL, max_connections, 1, err, sizeof(err)) != 0) {
      fprintf(stderr, "hello serve: %s\n", err);
      return 1;
    }
    return 0;
  }

  if (strcmp(cmd, "version") == 0) {
    printf("c-hello-world 0.1.0\n");
    return 0;
  }

  print_usage();
  return 1;
}
