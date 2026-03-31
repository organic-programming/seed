#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif

#include "holons/holons.h"

#include <arpa/inet.h>
#include <ctype.h>
#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <netdb.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/types.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>
#ifdef __APPLE__
#include <mach-o/dyld.h>
#endif

#define HOLONS_CONNECT_DEFAULT_TIMEOUT_MS 5000
#define HOLONS_CONNECT_POLL_MS 50
#define HOLONS_CONNECT_STOP_TIMEOUT_MS 2000

#define HOLONS_SOURCE_BRIDGE_COMMAND_ENV "HOLONS_SOURCE_BRIDGE_COMMAND"
#define HOLONS_DESCRIBE_PROBE_COMMAND_ENV "HOLONS_DESCRIBE_PROBE_COMMAND"

struct grpc_channel {
  char target[HOLONS_MAX_URI_LEN];
};

typedef struct started_channel {
  grpc_channel *channel;
  pid_t pid;
  int output_fd;
  struct started_channel *next;
} started_channel_t;

typedef struct {
  HolonsHolonRef ref;
  char *dir_path;
  char *relative_path;
} discovered_entry_t;

typedef struct {
  discovered_entry_t *items;
  size_t count;
  size_t capacity;
} discovered_list_t;

static started_channel_t *g_started_channels = NULL;

static void set_err(char *err, size_t err_len, const char *fmt, ...) {
  va_list ap;

  if (err == NULL || err_len == 0) {
    return;
  }

  va_start(ap, fmt);
  (void)vsnprintf(err, err_len, fmt, ap);
  va_end(ap);
}

static char *dup_string_nullable(const char *src) {
  if (src == NULL) {
    return NULL;
  }
  return strdup(src);
}

static char *dup_stringf(const char *fmt, ...) {
  va_list ap;
  va_list copy;
  char *buf;
  int needed;

  va_start(ap, fmt);
  va_copy(copy, ap);
  needed = vsnprintf(NULL, 0, fmt, copy);
  va_end(copy);
  if (needed < 0) {
    va_end(ap);
    return NULL;
  }

  buf = calloc((size_t)needed + 1, 1);
  if (buf == NULL) {
    va_end(ap);
    return NULL;
  }
  (void)vsnprintf(buf, (size_t)needed + 1, fmt, ap);
  va_end(ap);
  return buf;
}

static char *trimmed_dup(const char *src) {
  const char *start;
  const char *end;
  size_t n;
  char *out;

  if (src == NULL) {
    return NULL;
  }

  start = src;
  while (*start != '\0' && isspace((unsigned char)*start)) {
    ++start;
  }

  end = src + strlen(src);
  while (end > start && isspace((unsigned char)end[-1])) {
    --end;
  }

  n = (size_t)(end - start);
  out = calloc(n + 1, 1);
  if (out == NULL) {
    return NULL;
  }
  if (n > 0) {
    (void)memcpy(out, start, n);
  }
  out[n] = '\0';
  return out;
}

static int is_blank_string(const char *text) {
  if (text == NULL) {
    return 1;
  }
  while (*text != '\0') {
    if (!isspace((unsigned char)*text)) {
      return 0;
    }
    ++text;
  }
  return 1;
}

static long long monotonic_millis(void) {
  struct timespec ts;

  if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
    return 0;
  }
  return (long long)ts.tv_sec * 1000LL + (long long)(ts.tv_nsec / 1000000L);
}

static void sleep_millis(int millis) {
  struct timespec req;

  if (millis <= 0) {
    return;
  }

  req.tv_sec = millis / 1000;
  req.tv_nsec = (long)(millis % 1000) * 1000000L;
  while (nanosleep(&req, &req) != 0 && errno == EINTR) {
  }
}

static int path_depth(const char *rel) {
  int depth = 0;
  int in_segment = 0;

  if (rel == NULL) {
    return 0;
  }

  while (*rel != '\0') {
    if (*rel == '/') {
      in_segment = 0;
    } else if (!in_segment) {
      ++depth;
      in_segment = 1;
    }
    ++rel;
  }

  if (depth == 1 && rel != NULL && strcmp(rel, ".") == 0) {
    return 0;
  }
  return depth;
}

static const char *path_basename(const char *path) {
  const char *base;

  if (path == NULL) {
    return "";
  }
  base = strrchr(path, '/');
  return base != NULL ? base + 1 : path;
}

static int has_suffix(const char *value, const char *suffix) {
  size_t value_len;
  size_t suffix_len;

  if (value == NULL || suffix == NULL) {
    return 0;
  }

  value_len = strlen(value);
  suffix_len = strlen(suffix);
  if (value_len < suffix_len) {
    return 0;
  }
  return strcmp(value + value_len - suffix_len, suffix) == 0;
}

static int ensure_parent_join(char *out, size_t out_len, const char *left, const char *right) {
  if (left == NULL || left[0] == '\0') {
    return snprintf(out, out_len, "%s", right != NULL ? right : "") < (int)out_len ? 0 : -1;
  }
  if (right == NULL || right[0] == '\0') {
    return snprintf(out, out_len, "%s", left) < (int)out_len ? 0 : -1;
  }
  return snprintf(out, out_len, "%s/%s", left, right) < (int)out_len ? 0 : -1;
}

static int absolute_path_allow_missing(const char *path,
                                       char *out,
                                       size_t out_len,
                                       char *err,
                                       size_t err_len) {
  char cwd[PATH_MAX];

  if (path == NULL || path[0] == '\0') {
    if (getcwd(out, out_len) == NULL) {
      set_err(err, err_len, "getcwd failed: %s", strerror(errno));
      return -1;
    }
    return 0;
  }

  if (realpath(path, out) != NULL) {
    return 0;
  }
  if (path[0] == '/') {
    if (snprintf(out, out_len, "%s", path) >= (int)out_len) {
      set_err(err, err_len, "path is too long");
      return -1;
    }
    return 0;
  }
  if (getcwd(cwd, sizeof(cwd)) == NULL) {
    set_err(err, err_len, "getcwd failed: %s", strerror(errno));
    return -1;
  }
  if (snprintf(out, out_len, "%s/%s", cwd, path) >= (int)out_len) {
    set_err(err, err_len, "path is too long");
    return -1;
  }
  return 0;
}

static int resolve_discovery_root(const char *root,
                                  char *out,
                                  size_t out_len,
                                  char *err,
                                  size_t err_len) {
  struct stat st;
  char candidate[PATH_MAX];
  char *trimmed = NULL;

  (void)out_len;

  if (root == NULL) {
    if (getcwd(candidate, sizeof(candidate)) == NULL) {
      set_err(err, err_len, "getcwd failed: %s", strerror(errno));
      return -1;
    }
  } else {
    trimmed = trimmed_dup(root);
    if (trimmed == NULL) {
      set_err(err, err_len, "out of memory");
      return -1;
    }
    if (trimmed[0] == '\0') {
      free(trimmed);
      set_err(err, err_len, "root cannot be empty");
      return -1;
    }
    if (absolute_path_allow_missing(trimmed, candidate, sizeof(candidate), err, err_len) != 0) {
      free(trimmed);
      return -1;
    }
    free(trimmed);
  }

  if (realpath(candidate, out) == NULL) {
    set_err(err, err_len, "root \"%s\" is not a directory", root != NULL ? root : candidate);
    return -1;
  }
  if (stat(out, &st) != 0 || !S_ISDIR(st.st_mode)) {
    set_err(err, err_len, "root \"%s\" is not a directory", root != NULL ? root : out);
    return -1;
  }
  return 0;
}

static int relative_path(const char *root,
                         const char *path,
                         char *out,
                         size_t out_len,
                         char *err,
                         size_t err_len) {
  size_t root_len;
  const char *rel;

  if (root == NULL || path == NULL) {
    return snprintf(out, out_len, "%s", path != NULL ? path : ".") < (int)out_len ? 0 : -1;
  }

  root_len = strlen(root);
  if (strncmp(root, path, root_len) == 0 &&
      (path[root_len] == '\0' || path[root_len] == '/' || root[root_len - 1] == '/')) {
    rel = path + root_len;
    while (*rel == '/') {
      ++rel;
    }
    if (*rel == '\0') {
      return snprintf(out, out_len, ".") < (int)out_len ? 0 : -1;
    }
    return snprintf(out, out_len, "%s", rel) < (int)out_len ? 0 : -1;
  }

  if (snprintf(out, out_len, "%s", path) >= (int)out_len) {
    set_err(err, err_len, "relative path is too long");
    return -1;
  }
  return 0;
}

static int file_url_from_path(const char *path, char *out, size_t out_len, char *err, size_t err_len) {
  char abs_path[PATH_MAX];

  if (absolute_path_allow_missing(path, abs_path, sizeof(abs_path), err, err_len) != 0) {
    return -1;
  }
  if (snprintf(out, out_len, "file://%s", abs_path) >= (int)out_len) {
    set_err(err, err_len, "file URL is too long");
    return -1;
  }
  return 0;
}

static char *file_url_dup(const char *path) {
  char url[PATH_MAX + 16];

  if (file_url_from_path(path, url, sizeof(url), NULL, 0) != 0) {
    return NULL;
  }
  return strdup(url);
}

static int path_from_file_url(const char *url,
                              char *out,
                              size_t out_len,
                              char *err,
                              size_t err_len) {
  const char *path;

  if (url == NULL || strncmp(url, "file://", 7) != 0) {
    set_err(err, err_len, "holon URL is not a local file target");
    return -1;
  }

  path = url + 7;
  if (strncmp(path, "localhost", 9) == 0) {
    path += 9;
  }
  if (*path == '\0') {
    set_err(err, err_len, "holon URL has no path");
    return -1;
  }
  if (*path != '/') {
    set_err(err, err_len, "unsupported file URL host");
    return -1;
  }
  if (snprintf(out, out_len, "%s", path) >= (int)out_len) {
    set_err(err, err_len, "file path is too long");
    return -1;
  }
  return 0;
}

static void free_identity_info(HolonsIdentityInfo *identity) {
  size_t i;

  if (identity == NULL) {
    return;
  }
  free(identity->given_name);
  free(identity->family_name);
  free(identity->motto);
  for (i = 0; i < identity->aliases_len; ++i) {
    free(identity->aliases[i]);
  }
  free(identity->aliases);
  (void)memset(identity, 0, sizeof(*identity));
}

static void free_holon_info(HolonsHolonInfo *info) {
  size_t i;

  if (info == NULL) {
    return;
  }
  free(info->slug);
  free(info->uuid);
  free_identity_info(&info->identity);
  free(info->lang);
  free(info->runner);
  free(info->status);
  free(info->kind);
  free(info->transport);
  free(info->entrypoint);
  for (i = 0; i < info->architectures_len; ++i) {
    free(info->architectures[i]);
  }
  free(info->architectures);
  free(info);
}

static void free_holon_ref_fields(HolonsHolonRef *ref) {
  if (ref == NULL) {
    return;
  }
  free(ref->url);
  free_holon_info(ref->info);
  free(ref->error);
  (void)memset(ref, 0, sizeof(*ref));
}

static void free_discovered_entry(discovered_entry_t *entry) {
  if (entry == NULL) {
    return;
  }
  free_holon_ref_fields(&entry->ref);
  free(entry->dir_path);
  free(entry->relative_path);
  (void)memset(entry, 0, sizeof(*entry));
}

static void free_discovered_list(discovered_list_t *list) {
  size_t i;

  if (list == NULL) {
    return;
  }
  for (i = 0; i < list->count; ++i) {
    free_discovered_entry(&list->items[i]);
  }
  free(list->items);
  (void)memset(list, 0, sizeof(*list));
}

static int ensure_discovered_capacity(discovered_list_t *list, size_t need) {
  discovered_entry_t *items;
  size_t capacity;

  if (need <= list->capacity) {
    return 0;
  }

  capacity = list->capacity == 0 ? 8 : list->capacity;
  while (capacity < need) {
    capacity *= 2;
  }

  items = realloc(list->items, capacity * sizeof(*items));
  if (items == NULL) {
    return -1;
  }
  list->items = items;
  list->capacity = capacity;
  return 0;
}

static const char *entry_key(const discovered_entry_t *entry) {
  if (entry == NULL) {
    return "";
  }
  if (entry->ref.info != NULL && entry->ref.info->uuid != NULL && entry->ref.info->uuid[0] != '\0') {
    return entry->ref.info->uuid;
  }
  if (entry->dir_path != NULL && entry->dir_path[0] != '\0') {
    return entry->dir_path;
  }
  return entry->ref.url != NULL ? entry->ref.url : "";
}

static int append_or_replace_entry(discovered_list_t *list, discovered_entry_t *entry) {
  size_t i;
  const char *key = entry_key(entry);

  for (i = 0; i < list->count; ++i) {
    if (strcmp(entry_key(&list->items[i]), key) != 0) {
      continue;
    }
    if (path_depth(entry->relative_path) < path_depth(list->items[i].relative_path)) {
      free_discovered_entry(&list->items[i]);
      list->items[i] = *entry;
      (void)memset(entry, 0, sizeof(*entry));
    } else {
      free_discovered_entry(entry);
    }
    return 0;
  }

  if (ensure_discovered_capacity(list, list->count + 1) != 0) {
    return -1;
  }
  list->items[list->count++] = *entry;
  (void)memset(entry, 0, sizeof(*entry));
  return 0;
}

static int compare_discovered_entries(const void *left, const void *right) {
  const discovered_entry_t *a = left;
  const discovered_entry_t *b = right;
  int rel_cmp = strcmp(a->relative_path != NULL ? a->relative_path : "",
                       b->relative_path != NULL ? b->relative_path : "");

  if (rel_cmp != 0) {
    return rel_cmp;
  }
  return strcmp(entry_key(a), entry_key(b));
}

static void sort_discovered_entries(discovered_list_t *list) {
  if (list != NULL && list->count > 1) {
    qsort(list->items, list->count, sizeof(*list->items), compare_discovered_entries);
  }
}

static char *slug_from_identity(const char *given_name, const char *family_name) {
  char buf[HOLONS_MAX_FIELD_LEN];
  size_t n = 0;
  size_t i;
  const char *given = given_name != NULL ? given_name : "";
  const char *family = family_name != NULL ? family_name : "";

  while (*given != '\0' && isspace((unsigned char)*given)) {
    ++given;
  }
  while (*family != '\0' && isspace((unsigned char)*family)) {
    ++family;
  }

  for (i = 0; given[i] != '\0' && n + 1 < sizeof(buf); ++i) {
    char c = given[i];
    if (c == ' ') {
      c = '-';
    }
    buf[n++] = (char)tolower((unsigned char)c);
  }
  if (n > 0 && n + 1 < sizeof(buf)) {
    buf[n++] = '-';
  }
  for (i = 0; family[i] != '\0' && n + 1 < sizeof(buf); ++i) {
    char c = family[i];
    if (c == '?') {
      continue;
    }
    if (c == ' ') {
      c = '-';
    }
    buf[n++] = (char)tolower((unsigned char)c);
  }
  while (n > 0 && buf[n - 1] == '-') {
    --n;
  }
  buf[n] = '\0';
  return strdup(buf);
}

static const char *json_skip_ws(const char *p) {
  while (p != NULL && *p != '\0' && isspace((unsigned char)*p)) {
    ++p;
  }
  return p;
}

static int json_skip_string(const char **pp) {
  const char *p = *pp;

  if (*p != '"') {
    return -1;
  }
  ++p;
  while (*p != '\0') {
    if (*p == '\\') {
      ++p;
      if (*p == '\0') {
        return -1;
      }
      ++p;
      continue;
    }
    if (*p == '"') {
      *pp = p + 1;
      return 0;
    }
    ++p;
  }
  return -1;
}

static int json_parse_string_dup(const char **pp, char **out) {
  const char *p = *pp;
  char *buf;
  size_t used = 0;
  size_t cap = 16;

  if (*p != '"') {
    return -1;
  }

  buf = calloc(cap, 1);
  if (buf == NULL) {
    return -1;
  }

  ++p;
  while (*p != '\0') {
    char ch = *p;
    if (ch == '\\') {
      ++p;
      if (*p == '\0') {
        free(buf);
        return -1;
      }
      switch (*p) {
        case '"':
        case '\\':
        case '/':
          ch = *p;
          break;
        case 'b':
          ch = '\b';
          break;
        case 'f':
          ch = '\f';
          break;
        case 'n':
          ch = '\n';
          break;
        case 'r':
          ch = '\r';
          break;
        case 't':
          ch = '\t';
          break;
        default:
          ch = *p;
          break;
      }
    } else if (ch == '"') {
      buf[used] = '\0';
      *out = buf;
      *pp = p + 1;
      return 0;
    }

    if (used + 2 > cap) {
      char *next;
      cap *= 2;
      next = realloc(buf, cap);
      if (next == NULL) {
        free(buf);
        return -1;
      }
      buf = next;
    }
    buf[used++] = ch;
    ++p;
  }

  free(buf);
  return -1;
}

static int json_skip_compound(const char **pp, char open_ch, char close_ch) {
  const char *p = *pp;
  int depth = 0;

  if (*p != open_ch) {
    return -1;
  }

  while (*p != '\0') {
    if (*p == '"') {
      if (json_skip_string(&p) != 0) {
        return -1;
      }
      continue;
    }
    if (*p == open_ch) {
      ++depth;
    } else if (*p == close_ch) {
      --depth;
      if (depth == 0) {
        *pp = p + 1;
        return 0;
      }
    }
    ++p;
  }
  return -1;
}

static int json_skip_value(const char **pp) {
  const char *p = json_skip_ws(*pp);

  if (*p == '"') {
    if (json_skip_string(&p) != 0) {
      return -1;
    }
  } else if (*p == '{') {
    if (json_skip_compound(&p, '{', '}') != 0) {
      return -1;
    }
  } else if (*p == '[') {
    if (json_skip_compound(&p, '[', ']') != 0) {
      return -1;
    }
  } else {
    while (*p != '\0' && *p != ',' && *p != '}' && *p != ']') {
      ++p;
    }
  }

  *pp = p;
  return 0;
}

static int json_object_get_member(const char *object, const char *key, const char **out_value) {
  const char *p = json_skip_ws(object);

  if (p == NULL || *p != '{') {
    return 0;
  }
  ++p;

  for (;;) {
    char *member_key = NULL;

    p = json_skip_ws(p);
    if (*p == '}') {
      return 0;
    }
    if (json_parse_string_dup(&p, &member_key) != 0) {
      return 0;
    }
    p = json_skip_ws(p);
    if (*p != ':') {
      free(member_key);
      return 0;
    }
    ++p;
    p = json_skip_ws(p);
    if (strcmp(member_key, key) == 0) {
      free(member_key);
      *out_value = p;
      return 1;
    }
    free(member_key);
    if (json_skip_value(&p) != 0) {
      return 0;
    }
    p = json_skip_ws(p);
    if (*p == ',') {
      ++p;
      continue;
    }
    if (*p == '}') {
      return 0;
    }
    return 0;
  }
}

static char *json_dup_string_member(const char *object, const char *key) {
  const char *value = NULL;
  char *out = NULL;

  if (!json_object_get_member(object, key, &value)) {
    return NULL;
  }
  if (json_parse_string_dup(&value, &out) != 0) {
    free(out);
    return NULL;
  }
  return out;
}

static int json_parse_bool_member(const char *object, const char *key, bool *out, int *found) {
  const char *value = NULL;

  if (found != NULL) {
    *found = 0;
  }
  if (!json_object_get_member(object, key, &value)) {
    return 0;
  }
  if (found != NULL) {
    *found = 1;
  }
  value = json_skip_ws(value);
  if (strncmp(value, "true", 4) == 0) {
    *out = true;
    return 0;
  }
  if (strncmp(value, "false", 5) == 0) {
    *out = false;
    return 0;
  }
  return -1;
}

static int json_parse_string_array_member(const char *object,
                                          const char *key,
                                          char ***out_items,
                                          size_t *out_len) {
  const char *value = NULL;
  const char *p;
  char **items = NULL;
  size_t count = 0;
  size_t capacity = 0;

  *out_items = NULL;
  *out_len = 0;

  if (!json_object_get_member(object, key, &value)) {
    return 0;
  }
  p = json_skip_ws(value);
  if (*p != '[') {
    return -1;
  }
  ++p;

  for (;;) {
    char *item = NULL;
    p = json_skip_ws(p);
    if (*p == ']') {
      *out_items = items;
      *out_len = count;
      return 0;
    }
    if (json_parse_string_dup(&p, &item) != 0) {
      size_t i;
      for (i = 0; i < count; ++i) {
        free(items[i]);
      }
      free(items);
      return -1;
    }
    if (count == capacity) {
      char **next;
      capacity = capacity == 0 ? 4 : capacity * 2;
      next = realloc(items, capacity * sizeof(*items));
      if (next == NULL) {
        size_t i;
        free(item);
        for (i = 0; i < count; ++i) {
          free(items[i]);
        }
        free(items);
        return -1;
      }
      items = next;
    }
    items[count++] = item;
    p = json_skip_ws(p);
    if (*p == ',') {
      ++p;
      continue;
    }
    if (*p == ']') {
      *out_items = items;
      *out_len = count;
      return 0;
    }
    break;
  }

  while (count > 0) {
    free(items[--count]);
  }
  free(items);
  return -1;
}

static int json_array_for_each_object(const char *array,
                                      int (*callback)(const char *object, void *ctx),
                                      void *ctx) {
  const char *p = json_skip_ws(array);

  if (*p != '[') {
    return -1;
  }
  ++p;

  for (;;) {
    p = json_skip_ws(p);
    if (*p == ']') {
      return 0;
    }
    if (*p != '{') {
      return -1;
    }
    if (callback(p, ctx) != 0) {
      return -1;
    }
    if (json_skip_value(&p) != 0) {
      return -1;
    }
    p = json_skip_ws(p);
    if (*p == ',') {
      ++p;
      continue;
    }
    if (*p == ']') {
      return 0;
    }
    return -1;
  }
}

static int parse_holon_info_object(const char *object, HolonsHolonInfo **out_info) {
  HolonsHolonInfo *info;
  const char *identity_object = NULL;
  int found_bool = 0;

  info = calloc(1, sizeof(*info));
  if (info == NULL) {
    return -1;
  }

  if (json_object_get_member(object, "identity", &identity_object)) {
    info->identity.given_name = json_dup_string_member(identity_object, "given_name");
    if (info->identity.given_name == NULL) {
      info->identity.given_name = json_dup_string_member(identity_object, "givenName");
    }
    info->identity.family_name = json_dup_string_member(identity_object, "family_name");
    if (info->identity.family_name == NULL) {
      info->identity.family_name = json_dup_string_member(identity_object, "familyName");
    }
    info->identity.motto = json_dup_string_member(identity_object, "motto");
    (void)json_parse_string_array_member(identity_object,
                                         "aliases",
                                         &info->identity.aliases,
                                         &info->identity.aliases_len);
  }

  info->slug = json_dup_string_member(object, "slug");
  info->uuid = json_dup_string_member(object, "uuid");
  info->lang = json_dup_string_member(object, "lang");
  info->runner = json_dup_string_member(object, "runner");
  info->status = json_dup_string_member(object, "status");
  info->kind = json_dup_string_member(object, "kind");
  info->transport = json_dup_string_member(object, "transport");
  info->entrypoint = json_dup_string_member(object, "entrypoint");
  (void)json_parse_string_array_member(object,
                                       "architectures",
                                       &info->architectures,
                                       &info->architectures_len);
  (void)json_parse_bool_member(object, "has_dist", &info->has_dist, &found_bool);
  (void)json_parse_bool_member(object, "has_source", &info->has_source, &found_bool);

  if (info->slug == NULL) {
    info->slug = slug_from_identity(info->identity.given_name, info->identity.family_name);
  }

  *out_info = info;
  return 0;
}

static int parse_holon_info_from_describe_json(const char *json, HolonsHolonInfo **out_info) {
  const char *manifest = NULL;
  const char *identity = NULL;
  const char *build = NULL;
  const char *artifacts = NULL;
  HolonsHolonInfo *info;

  if (!json_object_get_member(json, "manifest", &manifest)) {
    return -1;
  }

  info = calloc(1, sizeof(*info));
  if (info == NULL) {
    return -1;
  }

  if (json_object_get_member(manifest, "identity", &identity)) {
    info->uuid = json_dup_string_member(identity, "uuid");
    info->identity.given_name = json_dup_string_member(identity, "given_name");
    if (info->identity.given_name == NULL) {
      info->identity.given_name = json_dup_string_member(identity, "givenName");
    }
    info->identity.family_name = json_dup_string_member(identity, "family_name");
    if (info->identity.family_name == NULL) {
      info->identity.family_name = json_dup_string_member(identity, "familyName");
    }
    info->identity.motto = json_dup_string_member(identity, "motto");
    info->status = json_dup_string_member(identity, "status");
    (void)json_parse_string_array_member(identity,
                                         "aliases",
                                         &info->identity.aliases,
                                         &info->identity.aliases_len);
  }

  info->slug = slug_from_identity(info->identity.given_name, info->identity.family_name);
  info->lang = json_dup_string_member(manifest, "lang");
  info->kind = json_dup_string_member(manifest, "kind");
  info->transport = json_dup_string_member(manifest, "transport");
  (void)json_parse_string_array_member(manifest,
                                       "platforms",
                                       &info->architectures,
                                       &info->architectures_len);
  if (json_object_get_member(manifest, "build", &build)) {
    info->runner = json_dup_string_member(build, "runner");
  }
  if (json_object_get_member(manifest, "artifacts", &artifacts)) {
    info->entrypoint = json_dup_string_member(artifacts, "binary");
  }

  *out_info = info;
  return 0;
}

static int parse_probe_info_json(const char *json, HolonsHolonInfo **out_info) {
  const char *trimmed = json_skip_ws(json);

  if (trimmed == NULL || *trimmed != '{') {
    return -1;
  }
  if (json_object_get_member(trimmed, "manifest", &trimmed)) {
    return parse_holon_info_from_describe_json(json, out_info);
  }
  return parse_holon_info_object(json, out_info);
}

static int read_text_file(const char *path, char **out_data, char *err, size_t err_len) {
  FILE *f;
  long size;
  char *data;
  size_t read_n;

  *out_data = NULL;

  f = fopen(path, "rb");
  if (f == NULL) {
    set_err(err, err_len, "cannot open %s: %s", path, strerror(errno));
    return -1;
  }
  if (fseek(f, 0, SEEK_END) != 0) {
    fclose(f);
    return -1;
  }
  size = ftell(f);
  if (size < 0) {
    fclose(f);
    return -1;
  }
  if (fseek(f, 0, SEEK_SET) != 0) {
    fclose(f);
    return -1;
  }

  data = calloc((size_t)size + 1, 1);
  if (data == NULL) {
    fclose(f);
    return -1;
  }
  read_n = fread(data, 1, (size_t)size, f);
  fclose(f);
  if (read_n != (size_t)size) {
    free(data);
    return -1;
  }
  data[size] = '\0';
  *out_data = data;
  return 0;
}

static int package_arch_dir(char *out, size_t out_len) {
#if defined(__APPLE__)
  const char *system = "darwin";
#elif defined(__linux__)
  const char *system = "linux";
#else
  const char *system = "unknown";
#endif

#if defined(__x86_64__) || defined(_M_X64)
  const char *arch = "amd64";
#elif defined(__aarch64__) || defined(__arm64__) || defined(_M_ARM64)
  const char *arch = "arm64";
#else
  const char *arch = "unknown";
#endif

  return snprintf(out, out_len, "%s_%s", system, arch) < (int)out_len ? 0 : -1;
}

static int package_binary_path(const char *package_dir,
                               const HolonsHolonInfo *info,
                               char *out,
                               size_t out_len) {
  char arch_dir[64];
  char candidate[PATH_MAX];
  const char *entrypoint;
  const char *base_name;
  struct stat st;

  if (package_arch_dir(arch_dir, sizeof(arch_dir)) != 0) {
    return -1;
  }

  entrypoint = info != NULL && info->entrypoint != NULL && info->entrypoint[0] != '\0'
                   ? info->entrypoint
                   : (info != NULL ? info->slug : NULL);
  if (entrypoint == NULL || entrypoint[0] == '\0') {
    return -1;
  }

  base_name = path_basename(entrypoint);
  if (snprintf(candidate, sizeof(candidate), "%s/bin/%s/%s", package_dir, arch_dir, base_name) >=
      (int)sizeof(candidate)) {
    return -1;
  }
  if (access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return snprintf(out, out_len, "%s", candidate) < (int)out_len ? 0 : -1;
  }
  return -1;
}

static int resolve_source_binary_path(const char *source_dir,
                                      const HolonsHolonInfo *info,
                                      char *out,
                                      size_t out_len) {
  char arch_dir[64];
  char candidate[PATH_MAX];
  const char *entrypoint;
  const char *base_name;
  struct stat st;

  if (info == NULL) {
    return -1;
  }

  entrypoint = info->entrypoint != NULL && info->entrypoint[0] != '\0' ? info->entrypoint : info->slug;
  if (entrypoint == NULL || entrypoint[0] == '\0') {
    return -1;
  }

  if (entrypoint[0] == '/' && access(entrypoint, X_OK) == 0) {
    return snprintf(out, out_len, "%s", entrypoint) < (int)out_len ? 0 : -1;
  }

  base_name = path_basename(entrypoint);
  if (package_arch_dir(arch_dir, sizeof(arch_dir)) == 0 &&
      snprintf(candidate,
               sizeof(candidate),
               "%s/.op/build/%s.holon/bin/%s/%s",
               source_dir,
               info->slug != NULL ? info->slug : base_name,
               arch_dir,
               base_name) < (int)sizeof(candidate) &&
      access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return snprintf(out, out_len, "%s", candidate) < (int)out_len ? 0 : -1;
  }

  if (snprintf(candidate, sizeof(candidate), "%s/.op/build/bin/%s", source_dir, base_name) < (int)sizeof(candidate) &&
      access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return snprintf(out, out_len, "%s", candidate) < (int)out_len ? 0 : -1;
  }

  if (snprintf(candidate, sizeof(candidate), "%s/%s", source_dir, entrypoint) < (int)sizeof(candidate) &&
      access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return snprintf(out, out_len, "%s", candidate) < (int)out_len ? 0 : -1;
  }

  return -1;
}

static int should_skip_dir(const char *root, const char *path, const char *name) {
  (void)path;

  if (root != NULL && path != NULL && strcmp(root, path) == 0) {
    return 0;
  }
  if (name == NULL) {
    return 0;
  }
  if (has_suffix(name, ".holon")) {
    return 0;
  }
  if (strcmp(name, ".git") == 0 || strcmp(name, ".op") == 0 || strcmp(name, "node_modules") == 0 ||
      strcmp(name, "vendor") == 0 || strcmp(name, "build") == 0 || strcmp(name, "testdata") == 0) {
    return 1;
  }
  return name[0] == '.';
}

static int probe_unix_target(const char *path) {
  struct sockaddr_un addr;
  int fd;

  if (path == NULL || path[0] == '\0' || strlen(path) >= sizeof(addr.sun_path)) {
    return -1;
  }

  fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) {
    return -1;
  }
  (void)memset(&addr, 0, sizeof(addr));
  addr.sun_family = AF_UNIX;
  (void)strncpy(addr.sun_path, path, sizeof(addr.sun_path) - 1);
  if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    close(fd);
    return -1;
  }
  close(fd);
  return 0;
}

static int parse_host_port(const char *text, char *host, size_t host_len, int *port) {
  const char *last_colon;
  char port_buf[32];
  long value;
  char *end = NULL;
  size_t host_n;

  if (text == NULL || text[0] == '\0') {
    return -1;
  }

  last_colon = strrchr(text, ':');
  if (last_colon == NULL) {
    return -1;
  }
  host_n = (size_t)(last_colon - text);
  if (host_n == 0 || host_n >= host_len || strlen(last_colon + 1) >= sizeof(port_buf)) {
    return -1;
  }
  (void)memcpy(host, text, host_n);
  host[host_n] = '\0';
  (void)snprintf(port_buf, sizeof(port_buf), "%s", last_colon + 1);
  errno = 0;
  value = strtol(port_buf, &end, 10);
  if (errno != 0 || end == port_buf || *end != '\0' || value < 0 || value > 65535) {
    return -1;
  }
  *port = (int)value;
  return 0;
}

static int connect_tcp_socket(const char *host, int port) {
  struct addrinfo hints;
  struct addrinfo *res = NULL;
  struct addrinfo *it;
  char service[32];
  int fd = -1;

  if (snprintf(service, sizeof(service), "%d", port) >= (int)sizeof(service)) {
    return -1;
  }

  (void)memset(&hints, 0, sizeof(hints));
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_family = AF_UNSPEC;
  if (getaddrinfo(host, service, &hints, &res) != 0) {
    return -1;
  }

  for (it = res; it != NULL; it = it->ai_next) {
    fd = socket(it->ai_family, it->ai_socktype, it->ai_protocol);
    if (fd < 0) {
      continue;
    }
    if (connect(fd, it->ai_addr, it->ai_addrlen) == 0) {
      freeaddrinfo(res);
      return fd;
    }
    close(fd);
    fd = -1;
  }

  freeaddrinfo(res);
  return -1;
}

static int parse_network_uri(const char *uri, char *host, size_t host_len, int *port, char *path, size_t path_len) {
  const char *scheme_end;
  const char *host_begin;
  const char *host_end;
  const char *path_begin;
  const char *port_begin;
  char port_buf[32];
  long value;
  char *end = NULL;
  size_t host_n;
  size_t path_n = 0;

  scheme_end = strstr(uri, "://");
  if (scheme_end == NULL) {
    return -1;
  }
  host_begin = scheme_end + 3;
  path_begin = strchr(host_begin, '/');
  host_end = path_begin != NULL ? path_begin : uri + strlen(uri);

  if (host_begin[0] == '[') {
    const char *close = strchr(host_begin, ']');
    if (close == NULL || close >= host_end || close[1] != ':') {
      return -1;
    }
    host_begin += 1;
    host_end = close;
    port_begin = close + 2;
  } else {
    const char *colon = NULL;
    const char *cursor;
    for (cursor = host_begin; cursor < host_end; ++cursor) {
      if (*cursor == ':') {
        colon = cursor;
      }
    }
    if (colon == NULL) {
      return -1;
    }
    port_begin = colon + 1;
    host_end = colon;
  }

  host_n = (size_t)(host_end - host_begin);
  if (host_n == 0 || host_n >= host_len || strlen(port_begin) >= sizeof(port_buf)) {
    return -1;
  }
  (void)memcpy(host, host_begin, host_n);
  host[host_n] = '\0';
  (void)snprintf(port_buf, sizeof(port_buf), "%s", port_begin);
  errno = 0;
  value = strtol(port_buf, &end, 10);
  if (errno != 0 || end == port_buf || (*end != '\0' && *end != '/') || value < 0 || value > 65535) {
    return -1;
  }
  *port = (int)value;

  if (path != NULL && path_len > 0) {
    if (path_begin != NULL) {
      path_n = strlen(path_begin);
    }
    if (path_n + 1 > path_len) {
      return -1;
    }
    if (path_begin != NULL) {
      (void)memcpy(path, path_begin, path_n);
    }
    path[path_n] = '\0';
  }
  return 0;
}

static int wait_for_ready_target(const char *target, int timeout_ms) {
  long long deadline;

  if (timeout_ms <= 0) {
    timeout_ms = HOLONS_CONNECT_DEFAULT_TIMEOUT_MS;
  }
  deadline = monotonic_millis() + timeout_ms;

  while (monotonic_millis() <= deadline) {
    if (strncmp(target, "unix://", 7) == 0) {
      if (probe_unix_target(target + 7) == 0) {
        return 0;
      }
    } else if (strstr(target, "://") != NULL) {
      char host[256];
      char path[256];
      int port = 0;
      int fd;

      if (parse_network_uri(target, host, sizeof(host), &port, path, sizeof(path)) == 0) {
        fd = connect_tcp_socket(host, port);
        if (fd >= 0) {
          close(fd);
          return 0;
        }
      }
    } else {
      char host[256];
      int port = 0;
      int fd;

      if (parse_host_port(target, host, sizeof(host), &port) == 0) {
        fd = connect_tcp_socket(host, port);
        if (fd >= 0) {
          close(fd);
          return 0;
        }
      }
    }
    sleep_millis(HOLONS_CONNECT_POLL_MS);
  }

  return -1;
}

static int stop_started_process(pid_t pid) {
  int status;
  long long deadline;

  if (pid <= 0) {
    return 0;
  }

  if (kill(pid, SIGTERM) != 0 && errno != ESRCH) {
    return -1;
  }
  deadline = monotonic_millis() + HOLONS_CONNECT_STOP_TIMEOUT_MS;
  for (;;) {
    pid_t waited = waitpid(pid, &status, WNOHANG);

    if (waited == pid) {
      return 0;
    }
    if (waited < 0) {
      if (errno == EINTR) {
        continue;
      }
      if (errno == ECHILD) {
        return 0;
      }
      return -1;
    }
    if (monotonic_millis() >= deadline) {
      break;
    }
    sleep_millis(25);
  }

  if (kill(pid, SIGKILL) != 0 && errno != ESRCH) {
    return -1;
  }
  while (waitpid(pid, &status, 0) < 0) {
    if (errno == EINTR) {
      continue;
    }
    if (errno == ECHILD) {
      return 0;
    }
    return -1;
  }
  return 0;
}

static int start_stdio_holon(const char *binary_path, int timeout_ms, pid_t *out_pid, int *out_fd) {
  int stdin_pipe[2] = {-1, -1};
  int devnull_fd = -1;
  pid_t pid;
  long long deadline;

  if (pipe(stdin_pipe) != 0) {
    return -1;
  }
  devnull_fd = open("/dev/null", O_WRONLY);
  if (devnull_fd < 0) {
    close(stdin_pipe[0]);
    close(stdin_pipe[1]);
    return -1;
  }

  pid = fork();
  if (pid < 0) {
    close(stdin_pipe[0]);
    close(stdin_pipe[1]);
    close(devnull_fd);
    return -1;
  }

  if (pid == 0) {
    close(stdin_pipe[1]);
    if (dup2(stdin_pipe[0], STDIN_FILENO) < 0 || dup2(devnull_fd, STDOUT_FILENO) < 0 ||
        dup2(devnull_fd, STDERR_FILENO) < 0) {
      _exit(127);
    }
    if (stdin_pipe[0] != STDIN_FILENO) {
      close(stdin_pipe[0]);
    }
    if (devnull_fd != STDOUT_FILENO && devnull_fd != STDERR_FILENO) {
      close(devnull_fd);
    }
    execl(binary_path, binary_path, "serve", "--listen", "stdio://", (char *)NULL);
    _exit(127);
  }

  close(stdin_pipe[0]);
  close(devnull_fd);

  deadline = monotonic_millis() + (timeout_ms > 0 && timeout_ms < 200 ? timeout_ms : 200);
  while (monotonic_millis() < deadline) {
    int status;
    pid_t waited = waitpid(pid, &status, WNOHANG);

    if (waited == pid) {
      close(stdin_pipe[1]);
      return -1;
    }
    if (waited < 0 && errno != EINTR && errno != ECHILD) {
      close(stdin_pipe[1]);
      return -1;
    }
    sleep_millis(10);
  }

  if (out_pid != NULL) {
    *out_pid = pid;
  }
  if (out_fd != NULL) {
    *out_fd = stdin_pipe[1];
  } else {
    close(stdin_pipe[1]);
  }
  return 0;
}

static grpc_channel *grpc_channel_create(const char *target) {
  grpc_channel *channel = calloc(1, sizeof(*channel));

  if (channel == NULL) {
    return NULL;
  }
  if (snprintf(channel->target, sizeof(channel->target), "%s", target != NULL ? target : "") >=
      (int)sizeof(channel->target)) {
    free(channel);
    return NULL;
  }
  return channel;
}

static void grpc_channel_destroy(grpc_channel *channel) { free(channel); }

static int remember_started_channel(grpc_channel *channel, pid_t pid, int output_fd) {
  started_channel_t *started = calloc(1, sizeof(*started));

  if (started == NULL) {
    return -1;
  }
  started->channel = channel;
  started->pid = pid;
  started->output_fd = output_fd;
  started->next = g_started_channels;
  g_started_channels = started;
  return 0;
}

static started_channel_t *take_started_channel(grpc_channel *channel) {
  started_channel_t **current = &g_started_channels;

  while (*current != NULL) {
    if ((*current)->channel == channel) {
      started_channel_t *match = *current;
      *current = match->next;
      match->next = NULL;
      return match;
    }
    current = &(*current)->next;
  }
  return NULL;
}

static int run_command_capture(const char *command,
                               const char *cwd,
                               int timeout_ms,
                               char **out_data,
                               int *out_exit_code) {
  int pipefd[2];
  pid_t pid;
  char *buffer = NULL;
  size_t used = 0;
  size_t capacity = 0;
  int exit_code = -1;
  int finished = 0;
  long long deadline = timeout_ms > 0 ? monotonic_millis() + timeout_ms : 0;

  *out_data = NULL;
  if (out_exit_code != NULL) {
    *out_exit_code = -1;
  }
  if (pipe(pipefd) != 0) {
    return -1;
  }

  pid = fork();
  if (pid < 0) {
    close(pipefd[0]);
    close(pipefd[1]);
    return -1;
  }

  if (pid == 0) {
    int devnull_fd = open("/dev/null", O_WRONLY);

    close(pipefd[0]);
    if (cwd != NULL && cwd[0] != '\0') {
      (void)chdir(cwd);
    }
    if (dup2(pipefd[1], STDOUT_FILENO) < 0) {
      _exit(127);
    }
    if (devnull_fd >= 0) {
      (void)dup2(devnull_fd, STDERR_FILENO);
    }
    if (pipefd[1] != STDOUT_FILENO) {
      close(pipefd[1]);
    }
    if (devnull_fd >= 0 && devnull_fd != STDERR_FILENO) {
      close(devnull_fd);
    }
    execl("/bin/sh", "sh", "-lc", command, (char *)NULL);
    _exit(127);
  }

  close(pipefd[1]);

  for (;;) {
    fd_set readfds;
    struct timeval tv;
    int rc;
    int status;

    if (deadline > 0 && monotonic_millis() > deadline) {
      (void)stop_started_process(pid);
      close(pipefd[0]);
      free(buffer);
      return -1;
    }

    FD_ZERO(&readfds);
    FD_SET(pipefd[0], &readfds);
    tv.tv_sec = 0;
    tv.tv_usec = 50000;
    rc = select(pipefd[0] + 1, &readfds, NULL, NULL, &tv);
    if (rc < 0) {
      if (errno == EINTR) {
        continue;
      }
      break;
    }
    if (rc > 0 && FD_ISSET(pipefd[0], &readfds)) {
      char chunk[1024];
      ssize_t nread = read(pipefd[0], chunk, sizeof(chunk));

      if (nread > 0) {
        if (used + (size_t)nread + 1 > capacity) {
          char *next;
          capacity = capacity == 0 ? 2048 : capacity * 2;
          while (capacity < used + (size_t)nread + 1) {
            capacity *= 2;
          }
          next = realloc(buffer, capacity);
          if (next == NULL) {
            break;
          }
          buffer = next;
        }
        (void)memcpy(buffer + used, chunk, (size_t)nread);
        used += (size_t)nread;
        buffer[used] = '\0';
        continue;
      }
      if (nread == 0) {
        finished = 1;
      }
    }

    rc = (int)waitpid(pid, &status, WNOHANG);
    if (rc == (int)pid) {
      exit_code = WIFEXITED(status) ? WEXITSTATUS(status) : -1;
      if (finished) {
        break;
      }
    } else if (rc < 0 && errno != EINTR && errno != ECHILD) {
      break;
    }
    if (finished && exit_code >= 0) {
      break;
    }
  }

  close(pipefd[0]);
  if (buffer == NULL) {
    buffer = calloc(1, 1);
    if (buffer == NULL) {
      return -1;
    }
  } else {
    buffer[used] = '\0';
  }
  *out_data = buffer;
  if (out_exit_code != NULL) {
    *out_exit_code = exit_code;
  }
  return 0;
}

static int load_package_info_from_json(const char *package_dir, HolonsHolonInfo **out_info) {
  char manifest_path[PATH_MAX];
  char *json = NULL;
  char *schema = NULL;
  int rc = -1;

  if (snprintf(manifest_path, sizeof(manifest_path), "%s/.holon.json", package_dir) >= (int)sizeof(manifest_path)) {
    return -1;
  }
  if (read_text_file(manifest_path, &json, NULL, 0) != 0) {
    return -1;
  }

  schema = json_dup_string_member(json, "schema");
  if (schema != NULL && schema[0] != '\0' && strcmp(schema, "holon-package/v1") != 0) {
    free(schema);
    free(json);
    return -1;
  }
  free(schema);

  rc = parse_holon_info_object(json, out_info);
  free(json);
  return rc;
}

static void apply_package_defaults(const char *package_dir, HolonsHolonInfo *info) {
  char dist_path[PATH_MAX];
  char git_path[PATH_MAX];
  char bin_root[PATH_MAX];
  struct stat st;

  if (snprintf(dist_path, sizeof(dist_path), "%s/dist", package_dir) < (int)sizeof(dist_path) &&
      stat(dist_path, &st) == 0 && S_ISDIR(st.st_mode)) {
    info->has_dist = true;
  }
  if (snprintf(git_path, sizeof(git_path), "%s/git", package_dir) < (int)sizeof(git_path) &&
      stat(git_path, &st) == 0 && S_ISDIR(st.st_mode)) {
    info->has_source = true;
  }
  if (info->architectures_len == 0 &&
      snprintf(bin_root, sizeof(bin_root), "%s/bin", package_dir) < (int)sizeof(bin_root) &&
      stat(bin_root, &st) == 0 && S_ISDIR(st.st_mode)) {
    struct dirent **entries = NULL;
    int count = scandir(bin_root, &entries, NULL, alphasort);
    if (count > 0) {
      size_t i;
      char **architectures = calloc((size_t)count, sizeof(*architectures));
      if (architectures != NULL) {
        size_t used = 0;
        for (i = 0; i < (size_t)count; ++i) {
          struct stat child_st;
          char child[PATH_MAX];

          if (entries[i] == NULL || strcmp(entries[i]->d_name, ".") == 0 ||
              strcmp(entries[i]->d_name, "..") == 0) {
            free(entries[i]);
            continue;
          }
          if (snprintf(child, sizeof(child), "%s/%s", bin_root, entries[i]->d_name) < (int)sizeof(child) &&
              stat(child, &child_st) == 0 && S_ISDIR(child_st.st_mode)) {
            architectures[used++] = strdup(entries[i]->d_name);
          }
          free(entries[i]);
        }
        free(entries);
        info->architectures = architectures;
        info->architectures_len = used;
      } else {
        for (i = 0; i < (size_t)count; ++i) {
          free(entries[i]);
        }
        free(entries);
      }
    }
  }
}

static int probe_package_info(const char *package_dir, int timeout, HolonsHolonInfo **out_info) {
  const char *command = getenv(HOLONS_DESCRIBE_PROBE_COMMAND_ENV);
  char *stdout_data = NULL;
  int exit_code = -1;
  int rc;

  if (is_blank_string(command)) {
    return -1;
  }
  if (run_command_capture(command, package_dir, timeout, &stdout_data, &exit_code) != 0) {
    return -1;
  }
  if (exit_code != 0 || stdout_data == NULL) {
    free(stdout_data);
    return -1;
  }
  rc = parse_probe_info_json(stdout_data, out_info);
  free(stdout_data);
  if (rc == 0 && *out_info != NULL) {
    apply_package_defaults(package_dir, *out_info);
  }
  return rc;
}

static int build_package_entry(const char *search_root,
                               const char *package_dir,
                               const char *origin,
                               int timeout,
                               discovered_entry_t *out_entry) {
  HolonsHolonInfo *info = NULL;
  char abs_dir[PATH_MAX];
  char rel[PATH_MAX];

  if (absolute_path_allow_missing(package_dir, abs_dir, sizeof(abs_dir), NULL, 0) != 0) {
    return -1;
  }
  if (load_package_info_from_json(abs_dir, &info) != 0 && probe_package_info(abs_dir, timeout, &info) != 0) {
    return -1;
  }
  if (info == NULL) {
    return -1;
  }
  if (relative_path(search_root, abs_dir, rel, sizeof(rel), NULL, 0) != 0) {
    free_holon_info(info);
    return -1;
  }

  (void)memset(out_entry, 0, sizeof(*out_entry));
  out_entry->dir_path = strdup(abs_dir);
  out_entry->relative_path = strdup(rel);
  out_entry->ref.url = file_url_dup(abs_dir);
  out_entry->ref.info = info;
  if (out_entry->dir_path == NULL || out_entry->relative_path == NULL || out_entry->ref.url == NULL) {
    free_discovered_entry(out_entry);
    return -1;
  }
  (void)origin;
  return 0;
}

static int scan_packages(const char *search_root,
                         const char *dir,
                         const char *origin,
                         int recursive,
                         int timeout,
                         discovered_list_t *out) {
  struct stat st;
  struct dirent **entries = NULL;
  int count;
  int i;

  if (dir == NULL || dir[0] == '\0') {
    return 0;
  }
  if (stat(dir, &st) != 0 || !S_ISDIR(st.st_mode)) {
    return 0;
  }

  count = scandir(dir, &entries, NULL, alphasort);
  if (count < 0) {
    return 0;
  }

  for (i = 0; i < count; ++i) {
    struct dirent *entry = entries[i];
    char child[PATH_MAX];
    struct stat child_st;

    if (entry == NULL) {
      continue;
    }
    if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) {
      free(entry);
      continue;
    }
    if (snprintf(child, sizeof(child), "%s/%s", dir, entry->d_name) >= (int)sizeof(child)) {
      free(entry);
      continue;
    }
    if (lstat(child, &child_st) != 0 || !S_ISDIR(child_st.st_mode)) {
      free(entry);
      continue;
    }

    if (has_suffix(entry->d_name, ".holon")) {
      discovered_entry_t found;
      (void)memset(&found, 0, sizeof(found));
      if (build_package_entry(search_root, child, origin, timeout, &found) == 0) {
        (void)append_or_replace_entry(out, &found);
      }
      free(entry);
      continue;
    }

    if (recursive && !should_skip_dir(search_root, child, entry->d_name)) {
      (void)scan_packages(search_root, child, origin, recursive, timeout, out);
    }
    free(entry);
  }
  free(entries);
  return 0;
}

static int bundle_holons_root(char *out, size_t out_len) {
  const char *configured = getenv("HOLONS_SIBLINGS_ROOT");
  char executable[PATH_MAX];
  char current[PATH_MAX];

  if (!is_blank_string(configured)) {
    return absolute_path_allow_missing(configured, out, out_len, NULL, 0);
  }

#ifdef __APPLE__
  uint32_t size = (uint32_t)sizeof(executable);
  if (_NSGetExecutablePath(executable, &size) != 0) {
    return -1;
  }
#elif defined(__linux__)
  ssize_t n = readlink("/proc/self/exe", executable, sizeof(executable) - 1);
  if (n < 0) {
    return -1;
  }
  executable[n] = '\0';
#else
  return -1;
#endif

  if (absolute_path_allow_missing(executable, current, sizeof(current), NULL, 0) != 0) {
    return -1;
  }

  for (;;) {
    char *base = strrchr(current, '/');
    char candidate[PATH_MAX];
    struct stat st;

    if (has_suffix(current, ".app")) {
      if (snprintf(candidate, sizeof(candidate), "%s/Contents/Resources/Holons", current) < (int)sizeof(candidate) &&
          stat(candidate, &st) == 0 && S_ISDIR(st.st_mode)) {
        return snprintf(out, out_len, "%s", candidate) < (int)out_len ? 0 : -1;
      }
    }

    if (base == NULL || base == current) {
      break;
    }
    *base = '\0';
  }

  return -1;
}

static int oppath(char *out, size_t out_len) {
  const char *configured = getenv("OPPATH");
  const char *home;

  if (!is_blank_string(configured)) {
    return absolute_path_allow_missing(configured, out, out_len, NULL, 0);
  }

  home = getenv("HOME");
  if (is_blank_string(home)) {
    return snprintf(out, out_len, ".op") < (int)out_len ? 0 : -1;
  }
  return snprintf(out, out_len, "%s/.op", home) < (int)out_len ? 0 : -1;
}

static int opbin(char *out, size_t out_len) {
  const char *configured = getenv("OPBIN");
  char op_path[PATH_MAX];

  if (!is_blank_string(configured)) {
    return absolute_path_allow_missing(configured, out, out_len, NULL, 0);
  }
  if (oppath(op_path, sizeof(op_path)) != 0) {
    return -1;
  }
  return snprintf(out, out_len, "%s/bin", op_path) < (int)out_len ? 0 : -1;
}

static int cache_dir(char *out, size_t out_len) {
  char op_path[PATH_MAX];

  if (oppath(op_path, sizeof(op_path)) != 0) {
    return -1;
  }
  return snprintf(out, out_len, "%s/cache", op_path) < (int)out_len ? 0 : -1;
}

static int is_path_expression(const char *expression) {
  if (expression == NULL || expression[0] == '\0') {
    return 0;
  }
  if (strncmp(expression, "file://", 7) == 0) {
    return 1;
  }
  if (strstr(expression, "://") != NULL) {
    return 0;
  }
  if (expression[0] == '/' || expression[0] == '.') {
    return 1;
  }
  if (strchr(expression, '/') != NULL || strchr(expression, '\\') != NULL) {
    return 1;
  }
  return has_suffix(expression, ".holon");
}

static int expression_path_candidate(const char *expression,
                                     const char *resolved_root,
                                     char *out,
                                     size_t out_len,
                                     char *err,
                                     size_t err_len) {
  if (strncmp(expression, "file://", 7) == 0) {
    return path_from_file_url(expression, out, out_len, err, err_len);
  }
  if (expression[0] == '/') {
    return absolute_path_allow_missing(expression, out, out_len, err, err_len);
  }
  if (ensure_parent_join(out, out_len, resolved_root, expression) != 0) {
    set_err(err, err_len, "path is too long");
    return -1;
  }
  return 0;
}

static int entry_matches_expression(const discovered_entry_t *entry, const char *expression) {
  const char *base;
  char base_buf[PATH_MAX];

  if (expression == NULL) {
    return 1;
  }
  if (expression[0] == '\0') {
    return 0;
  }
  if (entry->ref.info != NULL) {
    size_t i;
    if (entry->ref.info->slug != NULL && strcmp(entry->ref.info->slug, expression) == 0) {
      return 1;
    }
    if (entry->ref.info->uuid != NULL &&
        strncmp(entry->ref.info->uuid, expression, strlen(expression)) == 0) {
      return 1;
    }
    for (i = 0; i < entry->ref.info->identity.aliases_len; ++i) {
      if (strcmp(entry->ref.info->identity.aliases[i], expression) == 0) {
        return 1;
      }
    }
  }

  base = path_basename(entry->dir_path != NULL ? entry->dir_path : "");
  if (snprintf(base_buf, sizeof(base_buf), "%s", base) >= (int)sizeof(base_buf)) {
    return 0;
  }
  if (has_suffix(base_buf, ".holon")) {
    base_buf[strlen(base_buf) - strlen(".holon")] = '\0';
  }
  return strcmp(base_buf, expression) == 0;
}

static HolonsHolonRef holon_ref_take(HolonsHolonRef *ref) {
  HolonsHolonRef moved = {0};

  if (ref == NULL) {
    return moved;
  }
  moved = *ref;
  (void)memset(ref, 0, sizeof(*ref));
  return moved;
}

typedef struct {
  const char *root;
  discovered_list_t *out;
} bridge_parse_ctx_t;

static int parse_bridge_found_object(const char *object, void *ctx_void) {
  bridge_parse_ctx_t *ctx = ctx_void;
  discovered_entry_t entry;
  const char *info_object = NULL;
  char path[PATH_MAX];
  char rel[PATH_MAX];

  (void)memset(&entry, 0, sizeof(entry));
  entry.ref.url = json_dup_string_member(object, "url");
  entry.ref.error = json_dup_string_member(object, "error");
  if (json_object_get_member(object, "info", &info_object)) {
    if (parse_holon_info_object(info_object, &entry.ref.info) != 0) {
      free_discovered_entry(&entry);
      return -1;
    }
  }
  if (entry.ref.url == NULL) {
    free_discovered_entry(&entry);
    return -1;
  }
  if (path_from_file_url(entry.ref.url, path, sizeof(path), NULL, 0) == 0) {
    entry.dir_path = strdup(path);
  } else {
    entry.dir_path = strdup(entry.ref.url);
  }
  if (entry.dir_path == NULL || relative_path(ctx->root, entry.dir_path, rel, sizeof(rel), NULL, 0) != 0) {
    free_discovered_entry(&entry);
    return -1;
  }
  entry.relative_path = strdup(rel);
  if (entry.relative_path == NULL) {
    free_discovered_entry(&entry);
    return -1;
  }
  return append_or_replace_entry(ctx->out, &entry);
}

static int parse_bridge_entry_object(const char *object, void *ctx_void) {
  bridge_parse_ctx_t *ctx = ctx_void;
  discovered_entry_t entry;
  const char *identity_object = NULL;
  char abs_path[PATH_MAX];
  char rel[PATH_MAX];
  char *relative_value = NULL;
  char *url = NULL;

  (void)memset(&entry, 0, sizeof(entry));
  entry.ref.info = calloc(1, sizeof(*entry.ref.info));
  if (entry.ref.info == NULL) {
    return -1;
  }

  if (json_object_get_member(object, "identity", &identity_object)) {
    entry.ref.info->uuid = json_dup_string_member(identity_object, "uuid");
    entry.ref.info->identity.given_name = json_dup_string_member(identity_object, "given_name");
    if (entry.ref.info->identity.given_name == NULL) {
      entry.ref.info->identity.given_name = json_dup_string_member(identity_object, "givenName");
    }
    entry.ref.info->identity.family_name = json_dup_string_member(identity_object, "family_name");
    if (entry.ref.info->identity.family_name == NULL) {
      entry.ref.info->identity.family_name = json_dup_string_member(identity_object, "familyName");
    }
    entry.ref.info->identity.motto = json_dup_string_member(identity_object, "motto");
    entry.ref.info->status = json_dup_string_member(identity_object, "status");
    (void)json_parse_string_array_member(identity_object,
                                         "aliases",
                                         &entry.ref.info->identity.aliases,
                                         &entry.ref.info->identity.aliases_len);
  } else {
    entry.ref.info->uuid = json_dup_string_member(object, "uuid");
    entry.ref.info->identity.given_name = json_dup_string_member(object, "given_name");
    entry.ref.info->identity.family_name = json_dup_string_member(object, "family_name");
    entry.ref.info->status = json_dup_string_member(object, "status");
  }
  entry.ref.info->slug = json_dup_string_member(object, "slug");
  if (entry.ref.info->slug == NULL) {
    entry.ref.info->slug = slug_from_identity(entry.ref.info->identity.given_name,
                                              entry.ref.info->identity.family_name);
  }
  entry.ref.info->lang = json_dup_string_member(object, "lang");
  entry.ref.info->runner = json_dup_string_member(object, "runner");
  entry.ref.info->kind = json_dup_string_member(object, "kind");
  entry.ref.info->transport = json_dup_string_member(object, "transport");
  entry.ref.info->entrypoint = json_dup_string_member(object, "entrypoint");
  entry.ref.info->has_source = true;
  (void)json_parse_string_array_member(object,
                                       "architectures",
                                       &entry.ref.info->architectures,
                                       &entry.ref.info->architectures_len);

  url = json_dup_string_member(object, "url");
  if (url != NULL) {
    entry.ref.url = url;
  } else {
    relative_value = json_dup_string_member(object, "relative_path");
    if (relative_value == NULL) {
      relative_value = json_dup_string_member(object, "relativePath");
    }
    if (relative_value == NULL) {
      relative_value = json_dup_string_member(object, "path");
    }
    if (relative_value == NULL) {
      relative_value = strdup(".");
    }
    if (relative_value == NULL) {
      free_discovered_entry(&entry);
      return -1;
    }
    if (relative_value[0] == '/') {
      if (absolute_path_allow_missing(relative_value, abs_path, sizeof(abs_path), NULL, 0) != 0) {
        free(relative_value);
        free_discovered_entry(&entry);
        return -1;
      }
    } else if (ensure_parent_join(abs_path, sizeof(abs_path), ctx->root, relative_value) != 0) {
      free(relative_value);
      free_discovered_entry(&entry);
      return -1;
    }
    entry.ref.url = file_url_dup(abs_path);
    free(relative_value);
  }
  if (entry.ref.url == NULL) {
    free_discovered_entry(&entry);
    return -1;
  }
  if (path_from_file_url(entry.ref.url, abs_path, sizeof(abs_path), NULL, 0) != 0) {
    free_discovered_entry(&entry);
    return -1;
  }
  if (relative_path(ctx->root, abs_path, rel, sizeof(rel), NULL, 0) != 0) {
    free_discovered_entry(&entry);
    return -1;
  }
  entry.dir_path = strdup(abs_path);
  entry.relative_path = strdup(rel);
  if (entry.dir_path == NULL || entry.relative_path == NULL) {
    free_discovered_entry(&entry);
    return -1;
  }
  return append_or_replace_entry(ctx->out, &entry);
}

static int discover_source_with_local_op(const char *root,
                                         int timeout,
                                         discovered_list_t *out,
                                         char *err,
                                         size_t err_len) {
  const char *configured = getenv(HOLONS_SOURCE_BRIDGE_COMMAND_ENV);
  const char *commands[3];
  size_t i;

  commands[0] = !is_blank_string(configured) ? configured : NULL;
  commands[1] = "op --format json discover";
  commands[2] = "op discover --json";

  for (i = 0; i < sizeof(commands) / sizeof(commands[0]); ++i) {
    char *stdout_data = NULL;
    int exit_code = -1;
    const char *found_array = NULL;
    const char *entries_array = NULL;
    const char *top_error = NULL;
    bridge_parse_ctx_t ctx = {
        .root = root,
        .out = out,
    };

    if (commands[i] == NULL) {
      continue;
    }
    if (run_command_capture(commands[i], root, timeout, &stdout_data, &exit_code) != 0) {
      free(stdout_data);
      continue;
    }
    if (exit_code != 0 || stdout_data == NULL) {
      free(stdout_data);
      continue;
    }
    if (json_object_get_member(stdout_data, "error", &top_error) && top_error != NULL &&
        *json_skip_ws(top_error) == '"') {
      char *bridge_error = NULL;
      if (json_parse_string_dup(&top_error, &bridge_error) == 0 && bridge_error != NULL &&
          bridge_error[0] != '\0') {
        set_err(err, err_len, "%s", bridge_error);
        free(bridge_error);
        free(stdout_data);
        return -1;
      }
      free(bridge_error);
    }
    if (json_object_get_member(stdout_data, "found", &found_array)) {
      if (json_array_for_each_object(found_array, parse_bridge_found_object, &ctx) == 0) {
        free(stdout_data);
        return 0;
      }
    }
    if (json_object_get_member(stdout_data, "entries", &entries_array)) {
      if (json_array_for_each_object(entries_array, parse_bridge_entry_object, &ctx) == 0) {
        free(stdout_data);
        return 0;
      }
    }
    free(stdout_data);
  }

  return 0;
}

static int discover_path_expression(const char *expression,
                                    const char *resolved_root,
                                    int timeout,
                                    discovered_list_t *out,
                                    int *handled,
                                    char *err,
                                    size_t err_len) {
  char candidate[PATH_MAX];
  char abs_path[PATH_MAX];
  struct stat st;

  *handled = 0;
  if (!is_path_expression(expression)) {
    return 0;
  }
  *handled = 1;
  if (expression_path_candidate(expression, resolved_root, candidate, sizeof(candidate), err, err_len) != 0) {
    return -1;
  }
  if (absolute_path_allow_missing(candidate, abs_path, sizeof(abs_path), err, err_len) != 0) {
    return -1;
  }
  if (stat(abs_path, &st) != 0) {
    return 0;
  }

  if (S_ISDIR(st.st_mode)) {
    if (has_suffix(path_basename(abs_path), ".holon")) {
      discovered_entry_t entry;
      (void)memset(&entry, 0, sizeof(entry));
      if (build_package_entry(resolved_root, abs_path, "path", timeout, &entry) == 0) {
        return append_or_replace_entry(out, &entry);
      }
      return 0;
    }
    return discover_source_with_local_op(abs_path, timeout, out, err, err_len);
  }

  if (S_ISREG(st.st_mode) && strcmp(path_basename(abs_path), "holon.proto") == 0) {
    char parent[PATH_MAX];
    (void)snprintf(parent, sizeof(parent), "%s", abs_path);
    {
      char *slash = strrchr(parent, '/');
      if (slash != NULL) {
        *slash = '\0';
      }
    }
    return discover_source_with_local_op(parent, timeout, out, err, err_len);
  }

  return 0;
}

static int merge_unique_entries(discovered_list_t *dest, discovered_list_t *src) {
  size_t i;

  for (i = 0; i < src->count; ++i) {
    size_t j;
    int seen = 0;
    for (j = 0; j < dest->count; ++j) {
      if (strcmp(entry_key(&dest->items[j]), entry_key(&src->items[i])) == 0) {
        seen = 1;
        break;
      }
    }
    if (seen) {
      free_discovered_entry(&src->items[i]);
      continue;
    }
    if (ensure_discovered_capacity(dest, dest->count + 1) != 0) {
      return -1;
    }
    dest->items[dest->count++] = src->items[i];
    (void)memset(&src->items[i], 0, sizeof(src->items[i]));
  }
  free(src->items);
  src->items = NULL;
  src->count = 0;
  src->capacity = 0;
  return 0;
}

static HolonsDiscoverResult discover_result_with_error(const char *message) {
  HolonsDiscoverResult result;

  (void)memset(&result, 0, sizeof(result));
  result.error = dup_string_nullable(message);
  return result;
}

static HolonsDiscoverResult discover_result_take_error(char *message) {
  HolonsDiscoverResult result;

  (void)memset(&result, 0, sizeof(result));
  result.error = message;
  return result;
}

static HolonsResolveResult resolve_result_with_error(const char *message) {
  HolonsResolveResult result;

  (void)memset(&result, 0, sizeof(result));
  result.error = dup_string_nullable(message);
  return result;
}

static HolonsConnectResult connect_result_with_error(HolonsHolonRef *origin, const char *message) {
  HolonsConnectResult result;

  (void)memset(&result, 0, sizeof(result));
  result.origin = origin;
  result.error = dup_string_nullable(message);
  return result;
}

static HolonsConnectResult connect_result_take_error(HolonsHolonRef *origin, char *message) {
  HolonsConnectResult result;

  (void)memset(&result, 0, sizeof(result));
  result.origin = origin;
  result.error = message;
  return result;
}

HolonsDiscoverResult holons_discover(int scope,
                                     const char *expression,
                                     const char *root,
                                     int specifiers,
                                     int limit,
                                     int timeout) {
  HolonsDiscoverResult result;
  char resolved_root[PATH_MAX];
  char err[256] = "";
  char *expr = NULL;
  discovered_list_t found;
  int handled = 0;

  (void)memset(&result, 0, sizeof(result));
  (void)memset(&found, 0, sizeof(found));

  if (scope != HOLONS_LOCAL) {
    return discover_result_take_error(dup_stringf("scope %d not supported", scope));
  }
  if (specifiers < 0 || (specifiers & ~HOLONS_ALL) != 0) {
    return discover_result_take_error(dup_stringf("invalid specifiers 0x%02X: valid range is 0x00-0x3F",
                                                  specifiers & 0xFF));
  }
  if (specifiers == 0) {
    specifiers = HOLONS_ALL;
  }
  if (limit < 0) {
    return result;
  }

  if (expression != NULL) {
    expr = trimmed_dup(expression);
    if (expr == NULL) {
      return discover_result_with_error("out of memory");
    }
    if (expr[0] == '\0') {
      free(expr);
      return result;
    }
    if (strstr(expr, "://") != NULL && strncmp(expr, "file://", 7) != 0) {
      HolonsDiscoverResult direct = discover_result_with_error("direct transport expressions are not supported");
      free(expr);
      return direct;
    }
  }

  if (resolve_discovery_root(root, resolved_root, sizeof(resolved_root), err, sizeof(err)) != 0) {
    free(expr);
    return discover_result_with_error(err);
  }

  if (expr != NULL) {
    if (discover_path_expression(expr, resolved_root, timeout, &found, &handled, err, sizeof(err)) != 0) {
      free_discovered_list(&found);
      free(expr);
      return discover_result_with_error(err);
    }
    if (handled) {
      if (found.count > 0) {
        sort_discovered_entries(&found);
      }
    } else {
      discovered_list_t merged = {0};

      if ((specifiers & HOLONS_SIBLINGS) != 0) {
        char siblings_root[PATH_MAX];
        discovered_list_t layer = {0};
        if (bundle_holons_root(siblings_root, sizeof(siblings_root)) == 0) {
          (void)scan_packages(siblings_root, siblings_root, "siblings", 0, timeout, &layer);
          sort_discovered_entries(&layer);
          (void)merge_unique_entries(&merged, &layer);
        }
      }
      if ((specifiers & HOLONS_CWD) != 0) {
        discovered_list_t layer = {0};
        (void)scan_packages(resolved_root, resolved_root, "cwd", 1, timeout, &layer);
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
      if ((specifiers & HOLONS_SOURCE) != 0) {
        discovered_list_t layer = {0};
        if (discover_source_with_local_op(resolved_root, timeout, &layer, err, sizeof(err)) != 0) {
          free_discovered_list(&merged);
          free(expr);
          return discover_result_with_error(err);
        }
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
      if ((specifiers & HOLONS_BUILT) != 0) {
        char built_root[PATH_MAX];
        discovered_list_t layer = {0};
        if (snprintf(built_root, sizeof(built_root), "%s/.op/build", resolved_root) < (int)sizeof(built_root)) {
          (void)scan_packages(built_root, built_root, "built", 0, timeout, &layer);
          sort_discovered_entries(&layer);
          (void)merge_unique_entries(&merged, &layer);
        }
      }
      if ((specifiers & HOLONS_INSTALLED) != 0) {
        char installed_root[PATH_MAX];
        discovered_list_t layer = {0};
        if (opbin(installed_root, sizeof(installed_root)) == 0) {
          (void)scan_packages(installed_root, installed_root, "installed", 0, timeout, &layer);
          sort_discovered_entries(&layer);
          (void)merge_unique_entries(&merged, &layer);
        }
      }
      if ((specifiers & HOLONS_CACHED) != 0) {
        char cached_root[PATH_MAX];
        discovered_list_t layer = {0};
        if (cache_dir(cached_root, sizeof(cached_root)) == 0) {
          (void)scan_packages(cached_root, cached_root, "cached", 1, timeout, &layer);
          sort_discovered_entries(&layer);
          (void)merge_unique_entries(&merged, &layer);
        }
      }
      found = merged;
    }
  } else {
    discovered_list_t merged = {0};

    if ((specifiers & HOLONS_SIBLINGS) != 0) {
      char siblings_root[PATH_MAX];
      discovered_list_t layer = {0};
      if (bundle_holons_root(siblings_root, sizeof(siblings_root)) == 0) {
        (void)scan_packages(siblings_root, siblings_root, "siblings", 0, timeout, &layer);
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
    }
    if ((specifiers & HOLONS_CWD) != 0) {
      discovered_list_t layer = {0};
      (void)scan_packages(resolved_root, resolved_root, "cwd", 1, timeout, &layer);
      sort_discovered_entries(&layer);
      (void)merge_unique_entries(&merged, &layer);
    }
    if ((specifiers & HOLONS_SOURCE) != 0) {
      discovered_list_t layer = {0};
      if (discover_source_with_local_op(resolved_root, timeout, &layer, err, sizeof(err)) != 0) {
        free_discovered_list(&merged);
        free(expr);
        return discover_result_with_error(err);
      }
      sort_discovered_entries(&layer);
      (void)merge_unique_entries(&merged, &layer);
    }
    if ((specifiers & HOLONS_BUILT) != 0) {
      char built_root[PATH_MAX];
      discovered_list_t layer = {0};
      if (snprintf(built_root, sizeof(built_root), "%s/.op/build", resolved_root) < (int)sizeof(built_root)) {
        (void)scan_packages(built_root, built_root, "built", 0, timeout, &layer);
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
    }
    if ((specifiers & HOLONS_INSTALLED) != 0) {
      char installed_root[PATH_MAX];
      discovered_list_t layer = {0};
      if (opbin(installed_root, sizeof(installed_root)) == 0) {
        (void)scan_packages(installed_root, installed_root, "installed", 0, timeout, &layer);
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
    }
    if ((specifiers & HOLONS_CACHED) != 0) {
      char cached_root[PATH_MAX];
      discovered_list_t layer = {0};
      if (cache_dir(cached_root, sizeof(cached_root)) == 0) {
        (void)scan_packages(cached_root, cached_root, "cached", 1, timeout, &layer);
        sort_discovered_entries(&layer);
        (void)merge_unique_entries(&merged, &layer);
      }
    }
    found = merged;
  }

  if (found.count > 0) {
    size_t i;
    size_t used = 0;
    result.found = calloc(found.count, sizeof(*result.found));
    if (result.found == NULL) {
      free_discovered_list(&found);
      free(expr);
      return discover_result_with_error("out of memory");
    }
    for (i = 0; i < found.count; ++i) {
      if (expr != NULL && !handled && !entry_matches_expression(&found.items[i], expr)) {
        continue;
      }
      result.found[used++] = holon_ref_take(&found.items[i].ref);
      if (limit > 0 && (int)used >= limit) {
        break;
      }
    }
    result.found_len = used;
  }

  free_discovered_list(&found);
  free(expr);
  return result;
}

HolonsResolveResult holons_resolve(int scope,
                                   const char *expression,
                                   const char *root,
                                   int specifiers,
                                   int timeout) {
  HolonsResolveResult result;
  HolonsDiscoverResult discovered;

  (void)memset(&result, 0, sizeof(result));
  discovered = holons_discover(scope, expression, root, specifiers, 1, timeout);
  if (discovered.error != NULL) {
    result.error = discovered.error;
    discovered.error = NULL;
    holons_discover_result_free(&discovered);
    return result;
  }
  if (discovered.found_len == 0) {
    result.error = dup_stringf("holon \"%s\" not found", expression != NULL ? expression : "");
    holons_discover_result_free(&discovered);
    return result;
  }

  result.ref = calloc(1, sizeof(*result.ref));
  if (result.ref == NULL) {
    holons_discover_result_free(&discovered);
    return resolve_result_with_error("out of memory");
  }
  *result.ref = holon_ref_take(&discovered.found[0]);
  if (result.ref->error != NULL && result.error == NULL) {
    result.error = dup_string_nullable(result.ref->error);
  }
  holons_discover_result_free(&discovered);
  return result;
}

static int is_direct_connect_target(const char *expression) {
  if (expression == NULL) {
    return 0;
  }
  if (strncmp(expression, "tcp://", 6) == 0 || strncmp(expression, "unix://", 7) == 0 ||
      strncmp(expression, "ws://", 5) == 0 || strncmp(expression, "wss://", 6) == 0 ||
      strncmp(expression, "http://", 7) == 0 || strncmp(expression, "https://", 8) == 0 ||
      strncmp(expression, "rest+sse://", 11) == 0) {
    return 1;
  }
  return strstr(expression, "://") == NULL && strchr(expression, ':') != NULL;
}

static int resolve_launch_binary_from_ref(const HolonsHolonRef *ref,
                                          char *out,
                                          size_t out_len,
                                          char *err,
                                          size_t err_len) {
  char path[PATH_MAX];
  struct stat st;

  if (ref == NULL || ref->url == NULL) {
    set_err(err, err_len, "target has no URL");
    return -1;
  }
  if (path_from_file_url(ref->url, path, sizeof(path), err, err_len) != 0) {
    return -1;
  }
  if (stat(path, &st) != 0) {
    set_err(err, err_len, "target path is not launchable");
    return -1;
  }

  if (S_ISREG(st.st_mode) && access(path, X_OK) == 0) {
    return snprintf(out, out_len, "%s", path) < (int)out_len ? 0 : -1;
  }
  if (!S_ISDIR(st.st_mode)) {
    set_err(err, err_len, "target path is not launchable");
    return -1;
  }

  if (has_suffix(path, ".holon")) {
    if (package_binary_path(path, ref->info, out, out_len) == 0) {
      return 0;
    }
    set_err(err, err_len, "target unreachable");
    return -1;
  }

  if (resolve_source_binary_path(path, ref->info, out, out_len) == 0) {
    return 0;
  }
  set_err(err, err_len, "target unreachable");
  return -1;
}

HolonsConnectResult holons_connect(int scope,
                                   const char *expression,
                                   const char *root,
                                   int specifiers,
                                   int timeout) {
  HolonsConnectResult result;
  char *trimmed_expression;
  char err[256] = "";

  (void)memset(&result, 0, sizeof(result));

  if (scope != HOLONS_LOCAL) {
    return connect_result_take_error(NULL, dup_stringf("scope %d not supported", scope));
  }

  trimmed_expression = trimmed_dup(expression);
  if (trimmed_expression == NULL || trimmed_expression[0] == '\0') {
    free(trimmed_expression);
    return connect_result_with_error(NULL, "expression is required");
  }

  if (is_direct_connect_target(trimmed_expression)) {
    grpc_channel *channel;

    if (wait_for_ready_target(trimmed_expression, timeout) != 0) {
      HolonsConnectResult direct_error;
      HolonsHolonRef *origin = calloc(1, sizeof(*origin));
      if (origin != NULL) {
        origin->url = strdup(trimmed_expression);
      }
      direct_error = connect_result_with_error(origin, "target unreachable");
      free(trimmed_expression);
      return direct_error;
    }
    channel = grpc_channel_create(trimmed_expression);
    if (channel == NULL) {
      free(trimmed_expression);
      return connect_result_with_error(NULL, "out of memory");
    }
    result.channel = channel;
    result.origin = calloc(1, sizeof(*result.origin));
    if (result.origin != NULL) {
      result.origin->url = strdup(trimmed_expression);
    }
    free(trimmed_expression);
    return result;
  }

  {
    HolonsResolveResult resolved = holons_resolve(scope, trimmed_expression, root, specifiers, timeout);
    if (resolved.error != NULL) {
      result.origin = resolved.ref;
      resolved.ref = NULL;
      result.error = resolved.error;
      resolved.error = NULL;
      holons_resolve_result_free(&resolved);
      free(trimmed_expression);
      return result;
    }
    result.origin = resolved.ref;
    resolved.ref = NULL;
    holons_resolve_result_free(&resolved);
  }

  if (result.origin == NULL) {
    free(trimmed_expression);
    return connect_result_with_error(NULL, "target unreachable");
  }
  if (result.origin->error != NULL) {
    result.error = dup_string_nullable(result.origin->error);
    free(trimmed_expression);
    return result;
  }
  if (result.origin->url != NULL && strncmp(result.origin->url, "file://", 7) != 0) {
    if (wait_for_ready_target(result.origin->url, timeout) == 0) {
      result.channel = grpc_channel_create(result.origin->url);
      free(trimmed_expression);
      return result;
    }
  }

  {
    char binary_path[PATH_MAX];
    pid_t pid = -1;
    int output_fd = -1;
    grpc_channel *channel;

    if (resolve_launch_binary_from_ref(result.origin, binary_path, sizeof(binary_path), err, sizeof(err)) != 0) {
      result.error = dup_string_nullable(err);
      free(trimmed_expression);
      return result;
    }
    if (start_stdio_holon(binary_path, timeout, &pid, &output_fd) != 0) {
      result.error = dup_stringf("failed to launch %s", binary_path);
      free(trimmed_expression);
      return result;
    }
    channel = grpc_channel_create("stdio://");
    if (channel == NULL) {
      (void)stop_started_process(pid);
      if (output_fd >= 0) {
        close(output_fd);
      }
      free(trimmed_expression);
      return connect_result_with_error(result.origin, "out of memory");
    }
    if (remember_started_channel(channel, pid, output_fd) != 0) {
      grpc_channel_destroy(channel);
      (void)stop_started_process(pid);
      if (output_fd >= 0) {
        close(output_fd);
      }
      free(trimmed_expression);
      return connect_result_with_error(result.origin, "out of memory");
    }
    result.channel = channel;
  }

  free(trimmed_expression);
  return result;
}

void holons_disconnect(HolonsConnectResult *result) {
  started_channel_t *started;
  grpc_channel *channel;

  if (result == NULL || result->channel == NULL) {
    return;
  }

  channel = (grpc_channel *)result->channel;
  result->channel = NULL;
  started = take_started_channel(channel);
  grpc_channel_destroy(channel);
  if (started == NULL) {
    return;
  }
  (void)stop_started_process(started->pid);
  if (started->output_fd >= 0) {
    close(started->output_fd);
  }
  free(started);
}

void holons_discover_result_free(HolonsDiscoverResult *result) {
  size_t i;

  if (result == NULL) {
    return;
  }
  for (i = 0; i < result->found_len; ++i) {
    free_holon_ref_fields(&result->found[i]);
  }
  free(result->found);
  free(result->error);
  (void)memset(result, 0, sizeof(*result));
}

void holons_resolve_result_free(HolonsResolveResult *result) {
  if (result == NULL) {
    return;
  }
  if (result->ref != NULL) {
    free_holon_ref_fields(result->ref);
    free(result->ref);
  }
  free(result->error);
  (void)memset(result, 0, sizeof(*result));
}

void holons_connect_result_free(HolonsConnectResult *result) {
  if (result == NULL) {
    return;
  }
  free(result->uid);
  if (result->origin != NULL) {
    free_holon_ref_fields(result->origin);
    free(result->origin);
  }
  free(result->error);
  (void)memset(result, 0, sizeof(*result));
}
