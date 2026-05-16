#include "api/cli.h"

#include "holons/holons.h"
#include "internal/server.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
  holons_composite_child_spec_t *items;
  size_t len;
  size_t cap;
} child_list_t;

typedef struct {
  char **items;
  size_t len;
  size_t cap;
} string_list_t;

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

static void child_list_free(child_list_t *list) {
  size_t i;
  if (list == NULL) {
    return;
  }
  for (i = 0; i < list->len; ++i) {
    free((void *)list->items[i].slug);
    free((void *)list->items[i].binary);
  }
  free(list->items);
  list->items = NULL;
  list->len = 0;
  list->cap = 0;
}

static void string_list_free(string_list_t *list) {
  size_t i;
  if (list == NULL) {
    return;
  }
  for (i = 0; i < list->len; ++i) {
    free(list->items[i]);
  }
  free(list->items);
  list->items = NULL;
  list->len = 0;
  list->cap = 0;
}

static int string_list_add(string_list_t *list, const char *value) {
  char **grown;
  char *copy;
  if (list == NULL || value == NULL) {
    return -1;
  }
  if (list->len == list->cap) {
    size_t next = list->cap == 0 ? 2 : list->cap * 2;
    grown = (char **)realloc(list->items, next * sizeof(*grown));
    if (grown == NULL) {
      return -1;
    }
    list->items = grown;
    list->cap = next;
  }
  copy = dup_cstr(value);
  if (copy == NULL) {
    return -1;
  }
  list->items[list->len++] = copy;
  return 0;
}

static int child_list_add(child_list_t *list, const char *spec) {
  const char *eq;
  holons_composite_child_spec_t *grown;
  size_t slug_len;
  char *slug;
  char *binary;

  if (list == NULL || spec == NULL) {
    return -1;
  }
  eq = strchr(spec, '=');
  if (eq == NULL || eq == spec || eq[1] == '\0') {
    return -1;
  }
  if (list->len == list->cap) {
    size_t next = list->cap == 0 ? 4 : list->cap * 2;
    grown = (holons_composite_child_spec_t *)realloc(
        list->items, next * sizeof(*grown));
    if (grown == NULL) {
      return -1;
    }
    list->items = grown;
    list->cap = next;
  }
  slug_len = (size_t)(eq - spec);
  slug = (char *)malloc(slug_len + 1);
  binary = dup_cstr(eq + 1);
  if (slug == NULL || binary == NULL) {
    free(slug);
    free(binary);
    return -1;
  }
  memcpy(slug, spec, slug_len);
  slug[slug_len] = '\0';
  list->items[list->len].slug = slug;
  list->items[list->len].binary = binary;
  ++list->len;
  return 0;
}

static int parse_serve_args(int argc,
                            char **argv,
                            string_list_t *listeners,
                            char *transport,
                            size_t transport_len,
                            child_list_t *children,
                            FILE *stderr_stream) {
  int i;
  if (listeners == NULL || transport == NULL ||
      transport_len == 0 || children == NULL) {
    return -1;
  }
  snprintf(transport, transport_len, "%s", "stdio");
  for (i = 0; i < argc; ++i) {
    if (strcmp(argv[i], "--listen") == 0 && i + 1 < argc) {
      if (string_list_add(listeners, argv[++i]) != 0) {
        fprintf(stderr_stream, "failed to record --listen value\n");
        return -1;
      }
      continue;
    }
    if (strcmp(argv[i], "--transport") == 0 && i + 1 < argc) {
      snprintf(transport, transport_len, "%s", argv[++i]);
      continue;
    }
    if (strcmp(argv[i], "--child") == 0 && i + 1 < argc) {
      if (child_list_add(children, argv[++i]) != 0) {
        fprintf(stderr_stream, "invalid --child, expected <slug>=<binary>\n");
        return -1;
      }
      continue;
    }
    if (strncmp(argv[i], "--child=", 8) == 0) {
      if (child_list_add(children, argv[i] + 8) != 0) {
        fprintf(stderr_stream, "invalid --child, expected <slug>=<binary>\n");
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
  if (listeners->len == 0 && string_list_add(listeners, HOLONS_DEFAULT_URI) != 0) {
    fprintf(stderr_stream, "failed to record default listener\n");
    return -1;
  }
  return 0;
}

int cascade_node_c_run_cli(int argc,
                           char **argv,
                           FILE *stdout_stream,
                           FILE *stderr_stream) {
  string_list_t listeners;
  char transport[32];
  child_list_t children;
  int rc;
  (void)stdout_stream;

  if (argc < 1 || strcmp(argv[0], "serve") != 0) {
    fprintf(stderr_stream,
            "usage: observability-cascade-c-node serve [--listen <uri>] "
            "[--transport <stdio|tcp|unix>] [--child <slug>=<binary>]\n");
    return 1;
  }

  memset(&listeners, 0, sizeof(listeners));
  memset(&children, 0, sizeof(children));
  rc = parse_serve_args(argc - 1, argv + 1, &listeners, transport,
                        sizeof(transport), &children,
                        stderr_stream);
  if (rc == 0) {
    rc = cascade_node_c_serve((const char *const *)listeners.items,
                              listeners.len, transport, children.items,
                              children.len, stderr_stream);
  } else {
    rc = 1;
  }
  child_list_free(&children);
  string_list_free(&listeners);
  return rc;
}
