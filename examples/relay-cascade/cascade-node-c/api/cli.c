#include "api/cli.h"

#include "holons/holons.h"
#include "internal/server.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
  holons_grpc_member_ref_t *items;
  size_t len;
  size_t cap;
} member_list_t;

static char *dup_cstr(const char *value) {
  size_t len;
  char *copy;
  if (value == NULL) {
    return NULL;
  }
  len = strlen(value);
  copy = (char *)malloc(len + 1);
  if (copy == NULL) {
    return NULL;
  }
  memcpy(copy, value, len + 1);
  return copy;
}

static void member_list_free(member_list_t *list) {
  size_t i;
  if (list == NULL) {
    return;
  }
  for (i = 0; i < list->len; ++i) {
    free((void *)list->items[i].slug);
    free((void *)list->items[i].address);
  }
  free(list->items);
  list->items = NULL;
  list->len = 0;
  list->cap = 0;
}

static int member_list_add(member_list_t *list, const char *spec) {
  const char *eq;
  holons_grpc_member_ref_t *grown;
  size_t slug_len;
  char *slug;
  char *address;

  if (list == NULL || spec == NULL) {
    return -1;
  }
  eq = strchr(spec, '=');
  if (eq == NULL || eq == spec || eq[1] == '\0') {
    return -1;
  }
  if (list->len == list->cap) {
    size_t next = list->cap == 0 ? 4 : list->cap * 2;
    grown = (holons_grpc_member_ref_t *)realloc(list->items, next * sizeof(*grown));
    if (grown == NULL) {
      return -1;
    }
    list->items = grown;
    list->cap = next;
  }
  slug_len = (size_t)(eq - spec);
  slug = (char *)malloc(slug_len + 1);
  address = dup_cstr(eq + 1);
  if (slug == NULL || address == NULL) {
    free(slug);
    free(address);
    return -1;
  }
  memcpy(slug, spec, slug_len);
  slug[slug_len] = '\0';
  list->items[list->len].slug = slug;
  list->items[list->len].address = address;
  ++list->len;
  return 0;
}

static int parse_serve_args(int argc,
                            char **argv,
                            char *listen_uri,
                            size_t listen_uri_len,
                            member_list_t *members,
                            FILE *stderr_stream) {
  int i;
  if (listen_uri == NULL || listen_uri_len == 0 || members == NULL) {
    return -1;
  }
  snprintf(listen_uri, listen_uri_len, "%s", HOLONS_DEFAULT_URI);
  for (i = 0; i < argc; ++i) {
    if (strcmp(argv[i], "--listen") == 0 && i + 1 < argc) {
      snprintf(listen_uri, listen_uri_len, "%s", argv[++i]);
      continue;
    }
    if (strcmp(argv[i], "--port") == 0 && i + 1 < argc) {
      snprintf(listen_uri, listen_uri_len, "tcp://:%s", argv[++i]);
      continue;
    }
    if (strcmp(argv[i], "--member") == 0 && i + 1 < argc) {
      if (member_list_add(members, argv[++i]) != 0) {
        fprintf(stderr_stream, "invalid --member, expected <slug>=<address>\n");
        return -1;
      }
      continue;
    }
    if (strncmp(argv[i], "--member=", 9) == 0) {
      if (member_list_add(members, argv[i] + 9) != 0) {
        fprintf(stderr_stream, "invalid --member, expected <slug>=<address>\n");
        return -1;
      }
      continue;
    }
    if (strcmp(argv[i], "--reflect") == 0) {
      continue;
    }
    fprintf(stderr_stream, "unknown serve argument: %s\n", argv[i]);
    return -1;
  }
  return 0;
}

int cascade_node_c_run_cli(int argc, char **argv, FILE *stdout_stream, FILE *stderr_stream) {
  char listen_uri[HOLONS_MAX_URI_LEN];
  member_list_t members;
  int rc;
  (void)stdout_stream;

  if (argc < 1 || strcmp(argv[0], "serve") != 0) {
    fprintf(stderr_stream, "usage: cascade-node-c serve [--listen <uri>] [--member <slug>=<address>]\n");
    return 1;
  }

  memset(&members, 0, sizeof(members));
  rc = parse_serve_args(argc - 1, argv + 1, listen_uri, sizeof(listen_uri),
                        &members, stderr_stream);
  if (rc == 0) {
    rc = cascade_node_c_serve(listen_uri, members.items, members.len, stderr_stream);
  } else {
    rc = 1;
  }
  member_list_free(&members);
  return rc;
}

