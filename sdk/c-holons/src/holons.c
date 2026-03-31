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
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>
#ifdef __APPLE__
#include <mach-o/dyld.h>
#endif

static volatile sig_atomic_t g_stop_requested = 0;

#define HOLONS_CONNECT_DEFAULT_TIMEOUT_MS 5000
#define HOLONS_CONNECT_STOP_TIMEOUT_MS 2000
#define HOLONS_CONNECT_POLL_MS 50

struct grpc_channel {
  char target[HOLONS_MAX_URI_LEN];
};

typedef struct holons_started_channel {
  grpc_channel *channel;
  pid_t pid;
  int ephemeral;
  int output_fd;
  struct holons_started_channel *next;
} holons_started_channel_t;

typedef struct {
  HolonsHolonRef ref;
  char *dir_path;
  char *relative_path;
} holons_discovered_ref_t;

typedef struct {
  holons_discovered_ref_t *items;
  size_t count;
  size_t capacity;
} holons_discovered_refs_t;

static holons_started_channel_t *g_started_channels = NULL;
static const holons_describe_response_t *g_static_describe_response = NULL;

static const char *holons_no_incode_description_error =
    "no Incode Description registered — run op build";

#define HOLONS_SOURCE_BRIDGE_COMMAND_ENV "HOLONS_SOURCE_BRIDGE_COMMAND"
#define HOLONS_DESCRIBE_PROBE_COMMAND_ENV "HOLONS_DESCRIBE_PROBE_COMMAND"

void holons_free_entries(holon_entry_t *entries);

static void set_err(char *err, size_t err_len, const char *fmt, ...) {
  va_list ap;

  if (err == NULL || err_len == 0) {
    return;
  }

  va_start(ap, fmt);
  (void)vsnprintf(err, err_len, fmt, ap);
  va_end(ap);
}

static int copy_string(char *dst, size_t dst_len, const char *src, char *err, size_t err_len) {
  size_t n;

  if (dst == NULL || dst_len == 0) {
    set_err(err, err_len, "invalid destination buffer");
    return -1;
  }
  if (src == NULL) {
    dst[0] = '\0';
    return 0;
  }

  n = strlen(src);
  if (n >= dst_len) {
    set_err(err, err_len, "string is too long");
    return -1;
  }

  (void)memcpy(dst, src, n + 1);
  return 0;
}

static char *ltrim(char *s) {
  while (*s != '\0' && isspace((unsigned char)*s)) {
    ++s;
  }
  return s;
}

static void rtrim(char *s) {
  size_t n = strlen(s);
  while (n > 0 && isspace((unsigned char)s[n - 1])) {
    s[n - 1] = '\0';
    --n;
  }
}

static char *trim(char *s) {
  char *start = ltrim(s);
  rtrim(start);
  return start;
}

static char *strip_quotes(char *value) {
  size_t len = strlen(value);
  if (len >= 2) {
    if ((value[0] == '"' && value[len - 1] == '"') ||
        (value[0] == '\'' && value[len - 1] == '\'')) {
      value[len - 1] = '\0';
      return value + 1;
    }
  }
  return value;
}

static int parse_port(const char *text, int *out_port, char *err, size_t err_len) {
  char *end = NULL;
  long value;

  if (text == NULL || *text == '\0') {
    set_err(err, err_len, "missing port");
    return -1;
  }

  errno = 0;
  value = strtol(text, &end, 10);
  if (errno != 0 || end == text || *end != '\0') {
    set_err(err, err_len, "invalid port: %s", text);
    return -1;
  }

  if (value < 0 || value > 65535) {
    set_err(err, err_len, "port out of range: %ld", value);
    return -1;
  }

  *out_port = (int)value;
  return 0;
}

static int path_depth(const char *rel) {
  char tmp[HOLONS_MAX_URI_LEN];
  char *token;
  int depth = 0;

  if (rel == NULL || rel[0] == '\0' || strcmp(rel, ".") == 0) {
    return 0;
  }

  if (copy_string(tmp, sizeof(tmp), rel, NULL, 0) != 0) {
    return 0;
  }

  token = strtok(tmp, "/");
  while (token != NULL) {
    ++depth;
    token = strtok(NULL, "/");
  }
  return depth;
}

static int relative_path_from_root(const char *root,
                                   const char *dir,
                                   char *out,
                                   size_t out_len,
                                   char *err,
                                   size_t err_len) {
  size_t root_len;

  if (root == NULL || dir == NULL) {
    return copy_string(out, out_len, ".", err, err_len);
  }

  root_len = strlen(root);
  if (strncmp(root, dir, root_len) == 0 &&
      (dir[root_len] == '\0' || dir[root_len] == '/' || root[root_len - 1] == '/')) {
    const char *rel = dir + root_len;
    while (*rel == '/') {
      ++rel;
    }
    if (*rel == '\0') {
      return copy_string(out, out_len, ".", err, err_len);
    }
    return copy_string(out, out_len, rel, err, err_len);
  }

  return copy_string(out, out_len, dir, err, err_len);
}

static void slug_for_identity(const holons_identity_t *id, char *out, size_t out_len) {
  char slug[HOLONS_MAX_FIELD_LEN];
  size_t i;
  size_t n = 0;
  const char *given = id != NULL ? id->given_name : "";
  const char *family = id != NULL ? id->family_name : "";

  if (given == NULL) {
    given = "";
  }
  if (family == NULL) {
    family = "";
  }

  while (*given != '\0' && isspace((unsigned char)*given)) {
    ++given;
  }
  while (*family != '\0' && isspace((unsigned char)*family)) {
    ++family;
  }

  if (*given == '\0' && *family == '\0') {
    if (out != NULL && out_len > 0) {
      out[0] = '\0';
    }
    return;
  }

  for (i = 0; given[i] != '\0' && n + 1 < sizeof(slug); ++i) {
    char c = given[i];
    if (c == ' ') {
      c = '-';
    }
    slug[n++] = (char)tolower((unsigned char)c);
  }
  if (n > 0 && n + 1 < sizeof(slug)) {
    slug[n++] = '-';
  }
  for (i = 0; family[i] != '\0' && n + 1 < sizeof(slug); ++i) {
    char c = family[i];
    if (c == '?') {
      continue;
    }
    if (c == ' ') {
      c = '-';
    }
    slug[n++] = (char)tolower((unsigned char)c);
  }
  while (n > 0 && slug[n - 1] == '-') {
    --n;
  }
  slug[n] = '\0';
  (void)copy_string(out, out_len, slug, NULL, 0);
}

static int is_api_version_dir(const char *name) {
  size_t i;

  if (name == NULL || name[0] != 'v' || !isdigit((unsigned char)name[1])) {
    return 0;
  }
  for (i = 2; name[i] != '\0'; ++i) {
    unsigned char ch = (unsigned char)name[i];
    if (!isalnum(ch) && ch != '.' && ch != '_' && ch != '-') {
      return 0;
    }
  }
  return 1;
}

static int parent_dir_path(const char *path, char *out, size_t out_len, char *err, size_t err_len) {
  char tmp[PATH_MAX];
  char *slash;

  if (copy_string(tmp, sizeof(tmp), path, err, err_len) != 0) {
    return -1;
  }
  slash = strrchr(tmp, '/');
  if (slash == NULL) {
    return copy_string(out, out_len, ".", err, err_len);
  }
  if (slash == tmp) {
    slash[1] = '\0';
  } else {
    *slash = '\0';
  }
  return copy_string(out, out_len, tmp, err, err_len);
}

static int manifest_root_from_path(const char *manifest_path,
                                   char *out,
                                   size_t out_len,
                                   char *err,
                                   size_t err_len) {
  char manifest_dir[PATH_MAX];
  char api_dir[PATH_MAX];
  const char *version_name;
  const char *api_name;

  if (parent_dir_path(manifest_path, manifest_dir, sizeof(manifest_dir), err, err_len) != 0) {
    return -1;
  }
  version_name = strrchr(manifest_dir, '/');
  version_name = version_name != NULL ? version_name + 1 : manifest_dir;
  if (is_api_version_dir(version_name)) {
    if (parent_dir_path(manifest_dir, api_dir, sizeof(api_dir), err, err_len) != 0) {
      return -1;
    }
    api_name = strrchr(api_dir, '/');
    api_name = api_name != NULL ? api_name + 1 : api_dir;
    if (strcmp(api_name, "api") == 0) {
      return parent_dir_path(api_dir, out, out_len, err, err_len);
    }
  }
  return copy_string(out, out_len, manifest_dir, err, err_len);
}

static int split_proto_field_line(char *line, char **key, char **value) {
  char *sep;

  sep = strchr(line, ':');
  if (sep == NULL) {
    return 0;
  }
  *sep = '\0';
  *key = trim(line);
  *value = trim(sep + 1);
  *value = strip_quotes(*value);
  if (strcmp(*value, "null") == 0) {
    *value = "";
  }
  return 1;
}

static int resolve_manifest_path(const char *root,
                                 char *out,
                                 size_t out_len,
                                 char *err,
                                 size_t err_len) {
  char candidate[PATH_MAX];
  char parent[PATH_MAX];
  struct stat st;
  const char *resolved_root = root != NULL && root[0] != '\0' ? root : ".";
  const char *base;

  base = strrchr(resolved_root, '/');
  base = base != NULL ? base + 1 : resolved_root;
  if (strcmp(base, "holon.proto") == 0 && stat(resolved_root, &st) == 0 && S_ISREG(st.st_mode)) {
    return copy_string(out, out_len, resolved_root, err, err_len);
  }

  if (snprintf(candidate, sizeof(candidate), "%s/holon.proto", resolved_root) < (int)sizeof(candidate) &&
      stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return copy_string(out, out_len, candidate, err, err_len);
  }
  if (snprintf(candidate, sizeof(candidate), "%s/api/v1/holon.proto", resolved_root) <
          (int)sizeof(candidate) &&
      stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return copy_string(out, out_len, candidate, err, err_len);
  }

  if (strcmp(base, "protos") == 0 || strchr(resolved_root, '/') != NULL) {
    if (parent_dir_path(resolved_root, parent, sizeof(parent), NULL, 0) == 0) {
      if (snprintf(candidate, sizeof(candidate), "%s/holon.proto", parent) < (int)sizeof(candidate) &&
          stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
        return copy_string(out, out_len, candidate, err, err_len);
      }
      if (snprintf(candidate, sizeof(candidate), "%s/api/v1/holon.proto", parent) <
              (int)sizeof(candidate) &&
          stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
        return copy_string(out, out_len, candidate, err, err_len);
      }
    }
  }

  set_err(err, err_len, "no holon.proto found near %s", resolved_root);
  return -1;
}

static int parse_manifest_file(const char *path, holons_manifest_t *out, char *err, size_t err_len) {
  FILE *f;
  char line[1024];
  int saw_manifest = 0;
  int in_identity = 0;
  int in_build = 0;
  int in_artifacts = 0;

  if (path == NULL || out == NULL) {
    set_err(err, err_len, "path and output are required");
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  f = fopen(path, "r");
  if (f == NULL) {
    set_err(err, err_len, "cannot open %s: %s", path, strerror(errno));
    return -1;
  }

  while (fgets(line, sizeof(line), f) != NULL) {
    char *raw = trim(line);
    char *key;
    char *value;

    rtrim(raw);
    if (strstr(raw, "holons.v1.manifest") != NULL) {
      saw_manifest = 1;
    }
    if (raw[0] == '\0' || raw[0] == '#' || (raw[0] == '/' && raw[1] == '/')) {
      continue;
    }
    if (!saw_manifest) {
      continue;
    }
    if (raw[0] == '}') {
      in_identity = 0;
      in_build = 0;
      in_artifacts = 0;
      continue;
    }
    if (!split_proto_field_line(raw, &key, &value)) {
      continue;
    }

    if (strcmp(key, "identity") == 0 && strchr(value, '{') != NULL) {
      in_identity = 1;
      in_build = 0;
      in_artifacts = 0;
      continue;
    }
    if (strcmp(key, "build") == 0 && strchr(value, '{') != NULL) {
      in_identity = 0;
      in_build = 1;
      in_artifacts = 0;
      continue;
    }
    if (strcmp(key, "artifacts") == 0 && strchr(value, '{') != NULL) {
      in_identity = 0;
      in_artifacts = 1;
      in_build = 0;
      continue;
    }

    if (in_identity) {
      if (strcmp(key, "uuid") == 0) {
        (void)copy_string(out->identity.uuid, sizeof(out->identity.uuid), value, NULL, 0);
      } else if (strcmp(key, "given_name") == 0) {
        (void)copy_string(out->identity.given_name, sizeof(out->identity.given_name), value, NULL, 0);
      } else if (strcmp(key, "family_name") == 0) {
        (void)copy_string(out->identity.family_name, sizeof(out->identity.family_name), value, NULL, 0);
      } else if (strcmp(key, "motto") == 0) {
        (void)copy_string(out->identity.motto, sizeof(out->identity.motto), value, NULL, 0);
      } else if (strcmp(key, "composer") == 0) {
        (void)copy_string(out->identity.composer, sizeof(out->identity.composer), value, NULL, 0);
      } else if (strcmp(key, "clade") == 0) {
        (void)copy_string(out->identity.clade, sizeof(out->identity.clade), value, NULL, 0);
      } else if (strcmp(key, "status") == 0) {
        (void)copy_string(out->identity.status, sizeof(out->identity.status), value, NULL, 0);
      } else if (strcmp(key, "born") == 0) {
        (void)copy_string(out->identity.born, sizeof(out->identity.born), value, NULL, 0);
      }
    } else if (in_build) {
      if (strcmp(key, "runner") == 0) {
        (void)copy_string(out->build.runner, sizeof(out->build.runner), value, NULL, 0);
      } else if (strcmp(key, "main") == 0) {
        (void)copy_string(out->build.main, sizeof(out->build.main), value, NULL, 0);
      }
    } else if (in_artifacts) {
      if (strcmp(key, "binary") == 0) {
        (void)copy_string(out->artifacts.binary, sizeof(out->artifacts.binary), value, NULL, 0);
      } else if (strcmp(key, "primary") == 0) {
        (void)copy_string(out->artifacts.primary, sizeof(out->artifacts.primary), value, NULL, 0);
      }
    } else if (strcmp(key, "kind") == 0) {
      (void)copy_string(out->kind, sizeof(out->kind), value, NULL, 0);
    } else if (strcmp(key, "lang") == 0) {
      (void)copy_string(out->lang, sizeof(out->lang), value, NULL, 0);
      (void)copy_string(out->identity.lang, sizeof(out->identity.lang), value, NULL, 0);
    }
  }

  (void)fclose(f);

  if (!saw_manifest) {
    set_err(err, err_len, "%s: missing holons.v1.manifest option in holon.proto", path);
    return -1;
  }
  return 0;
}

typedef struct {
  holon_entry_t *items;
  size_t count;
  size_t capacity;
} holon_entries_t;

static int ensure_entries_capacity(holon_entries_t *entries, size_t needed, char *err, size_t err_len) {
  holon_entry_t *next;
  size_t new_capacity;

  if (entries->capacity >= needed) {
    return 0;
  }

  new_capacity = entries->capacity == 0 ? 8 : entries->capacity * 2;
  while (new_capacity < needed) {
    new_capacity *= 2;
  }

  next = realloc(entries->items, new_capacity * sizeof(*next));
  if (next == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  entries->items = next;
  entries->capacity = new_capacity;
  return 0;
}

static int append_or_replace_entry(holon_entries_t *entries,
                                   const holon_entry_t *entry,
                                   char *err,
                                   size_t err_len) {
  size_t i;
  const char *key = entry->uuid[0] != '\0' ? entry->uuid : entry->dir;

  for (i = 0; i < entries->count; ++i) {
    const char *existing_key =
        entries->items[i].uuid[0] != '\0' ? entries->items[i].uuid : entries->items[i].dir;
    if (strcmp(existing_key, key) == 0) {
      if (path_depth(entry->relative_path) < path_depth(entries->items[i].relative_path)) {
        entries->items[i] = *entry;
      }
      return 0;
    }
  }

  if (ensure_entries_capacity(entries, entries->count + 1, err, err_len) != 0) {
    return -1;
  }
  entries->items[entries->count++] = *entry;
  return 0;
}

static int should_skip_discovery_dir(const char *root, const char *path, const char *name) {
  (void)path;
  if (root != NULL && path != NULL && strcmp(root, path) == 0) {
    return 0;
  }
  if (strcmp(name, ".git") == 0 || strcmp(name, ".op") == 0 || strcmp(name, "node_modules") == 0 ||
      strcmp(name, "vendor") == 0 || strcmp(name, "build") == 0) {
    return 1;
  }
  return name[0] == '.';
}

static int discover_scan_dir(const char *root,
                             const char *dir,
                             const char *origin,
                             holon_entries_t *entries,
                             char *err,
                             size_t err_len) {
  DIR *handle;
  struct dirent *item;

  handle = opendir(dir);
  if (handle == NULL) {
    return 0;
  }

  while ((item = readdir(handle)) != NULL) {
    char child[PATH_MAX];
    struct stat st;

    if (strcmp(item->d_name, ".") == 0 || strcmp(item->d_name, "..") == 0) {
      continue;
    }

    if (snprintf(child, sizeof(child), "%s/%s", dir, item->d_name) >= (int)sizeof(child)) {
      continue;
    }

    if (lstat(child, &st) != 0) {
      continue;
    }

    if (S_ISDIR(st.st_mode)) {
      if (should_skip_discovery_dir(root, child, item->d_name)) {
        continue;
      }
      if (discover_scan_dir(root, child, origin, entries, err, err_len) != 0) {
        (void)closedir(handle);
        return -1;
      }
      continue;
    }

    if (!S_ISREG(st.st_mode) || strcmp(item->d_name, "holon.proto") != 0) {
      continue;
    }

    {
      holon_entry_t entry;
      char abs_dir[PATH_MAX];
      char manifest_root[PATH_MAX];

      (void)memset(&entry, 0, sizeof(entry));
      if (holons_resolve_manifest(child, &entry.manifest, NULL, 0, NULL, 0) != 0) {
        continue;
      }
      entry.identity = entry.manifest.identity;
      entry.has_manifest = 1;
      if (manifest_root_from_path(child, manifest_root, sizeof(manifest_root), NULL, 0) != 0) {
        (void)copy_string(manifest_root, sizeof(manifest_root), dir, NULL, 0);
      }
      if (realpath(manifest_root, abs_dir) == NULL) {
        (void)copy_string(abs_dir, sizeof(abs_dir), manifest_root, NULL, 0);
      }

      slug_for_identity(&entry.identity, entry.slug, sizeof(entry.slug));
      (void)copy_string(entry.uuid, sizeof(entry.uuid), entry.identity.uuid, NULL, 0);
      (void)copy_string(entry.dir, sizeof(entry.dir), abs_dir, NULL, 0);
      (void)copy_string(entry.origin, sizeof(entry.origin), origin, NULL, 0);
      if (relative_path_from_root(root, abs_dir, entry.relative_path, sizeof(entry.relative_path), NULL, 0) != 0) {
        (void)copy_string(entry.relative_path, sizeof(entry.relative_path), abs_dir, NULL, 0);
      }

      if (append_or_replace_entry(entries, &entry, err, err_len) != 0) {
        (void)closedir(handle);
        return -1;
      }
    }
  }

  (void)closedir(handle);
  return 0;
}

static int compare_entries(const void *left, const void *right) {
  const holon_entry_t *a = left;
  const holon_entry_t *b = right;
  int rel_cmp = strcmp(a->relative_path, b->relative_path);
  if (rel_cmp != 0) {
    return rel_cmp;
  }
  return strcmp(a->uuid, b->uuid);
}

static int resolve_root(const char *root, char *out, size_t out_len, char *err, size_t err_len) {
  char cwd[PATH_MAX];
  const char *candidate = root;

  if (candidate == NULL || candidate[0] == '\0') {
    if (getcwd(cwd, sizeof(cwd)) == NULL) {
      set_err(err, err_len, "getcwd failed: %s", strerror(errno));
      return -1;
    }
    candidate = cwd;
  }

  if (realpath(candidate, out) != NULL) {
    return 0;
  }
  if (errno == ENOENT) {
    out[0] = '\0';
    return 0;
  }
  return copy_string(out, out_len, candidate, err, err_len);
}

static int oppath(char *out, size_t out_len, char *err, size_t err_len) {
  const char *configured = getenv("OPPATH");
  const char *home;
  char buf[PATH_MAX];

  if (configured != NULL && configured[0] != '\0') {
    return resolve_root(configured, out, out_len, err, err_len);
  }

  home = getenv("HOME");
  if (home == NULL || home[0] == '\0') {
    return copy_string(out, out_len, ".op", err, err_len);
  }

  if (snprintf(buf, sizeof(buf), "%s/.op", home) >= (int)sizeof(buf)) {
    set_err(err, err_len, "OPPATH is too long");
    return -1;
  }
  return resolve_root(buf, out, out_len, err, err_len);
}

static int opbin(char *out, size_t out_len, char *err, size_t err_len) {
  const char *configured = getenv("OPBIN");
  char op_path[PATH_MAX];

  if (configured != NULL && configured[0] != '\0') {
    return resolve_root(configured, out, out_len, err, err_len);
  }

  if (oppath(op_path, sizeof(op_path), err, err_len) != 0) {
    return -1;
  }
  if (snprintf(out, out_len, "%s/bin", op_path) >= (int)out_len) {
    set_err(err, err_len, "OPBIN is too long");
    return -1;
  }
  return 0;
}

static int cache_dir(char *out, size_t out_len, char *err, size_t err_len) {
  char op_path[PATH_MAX];

  if (oppath(op_path, sizeof(op_path), err, err_len) != 0) {
    return -1;
  }
  if (snprintf(out, out_len, "%s/cache", op_path) >= (int)out_len) {
    set_err(err, err_len, "cache path is too long");
    return -1;
  }
  return 0;
}

static int parse_host_port(const char *input,
                           char *host,
                           size_t host_len,
                           int *port,
                           char *err,
                           size_t err_len) {
  const char *host_begin = input;
  const char *host_end = NULL;
  const char *port_begin = NULL;
  size_t host_n;

  if (input == NULL || *input == '\0') {
    set_err(err, err_len, "empty address");
    return -1;
  }

  if (input[0] == '[') {
    host_begin = input + 1;
    host_end = strchr(host_begin, ']');
    if (host_end == NULL) {
      set_err(err, err_len, "invalid IPv6 address: missing ']'");
      return -1;
    }
    if (host_end[1] != ':') {
      set_err(err, err_len, "missing port in address: %s", input);
      return -1;
    }
    port_begin = host_end + 2;
  } else {
    const char *last_colon = strrchr(input, ':');
    if (last_colon == NULL) {
      set_err(err, err_len, "missing port in address: %s", input);
      return -1;
    }
    host_end = last_colon;
    port_begin = last_colon + 1;
  }

  host_n = (size_t)(host_end - host_begin);
  if (host_n >= host_len) {
    set_err(err, err_len, "host is too long");
    return -1;
  }
  (void)memcpy(host, host_begin, host_n);
  host[host_n] = '\0';

  return parse_port(port_begin, port, err, err_len);
}

static int parse_ws_uri(const char *rest,
                        holons_uri_t *out,
                        char *err,
                        size_t err_len) {
  const char *slash = strchr(rest, '/');
  char host_port[256];

  if (slash == NULL) {
    if (copy_string(host_port, sizeof(host_port), rest, err, err_len) != 0) {
      return -1;
    }
    if (copy_string(out->path, sizeof(out->path), "/grpc", err, err_len) != 0) {
      return -1;
    }
  } else {
    size_t host_port_len = (size_t)(slash - rest);
    if (host_port_len >= sizeof(host_port)) {
      set_err(err, err_len, "websocket host:port is too long");
      return -1;
    }
    (void)memcpy(host_port, rest, host_port_len);
    host_port[host_port_len] = '\0';

    if (copy_string(out->path, sizeof(out->path), slash, err, err_len) != 0) {
      return -1;
    }
    if (out->path[0] == '\0') {
      if (copy_string(out->path, sizeof(out->path), "/grpc", err, err_len) != 0) {
        return -1;
      }
    }
  }

  return parse_host_port(host_port, out->host, sizeof(out->host), &out->port, err, err_len);
}

static int parse_http_like_uri(const char *uri,
                               char *host,
                               size_t host_len,
                               int *port,
                               char *path,
                               size_t path_len,
                               char *err,
                               size_t err_len) {
  const char *rest = NULL;
  const char *slash;
  const char *default_path = "/api/v1/rpc";
  char host_port[256];

  if (strncmp(uri, "http://", 7) == 0) {
    rest = uri + 7;
  } else if (strncmp(uri, "https://", 8) == 0) {
    rest = uri + 8;
  } else if (strncmp(uri, "rest+sse://", 11) == 0) {
    rest = uri + 11;
  } else {
    set_err(err, err_len, "unsupported HTTP+SSE URI: %s", uri);
    return -1;
  }

  slash = strchr(rest, '/');
  if (slash == NULL) {
    if (copy_string(host_port, sizeof(host_port), rest, err, err_len) != 0) {
      return -1;
    }
    if (copy_string(path, path_len, default_path, err, err_len) != 0) {
      return -1;
    }
  } else {
    size_t host_port_len = (size_t)(slash - rest);
    if (host_port_len >= sizeof(host_port)) {
      set_err(err, err_len, "HTTP+SSE host:port is too long");
      return -1;
    }
    (void)memcpy(host_port, rest, host_port_len);
    host_port[host_port_len] = '\0';

    if (copy_string(path, path_len, slash, err, err_len) != 0) {
      return -1;
    }
    if (path[0] == '\0') {
      if (copy_string(path, path_len, default_path, err, err_len) != 0) {
        return -1;
      }
    }
  }

  return parse_host_port(host_port, host, host_len, port, err, err_len);
}

static int create_tcp_listener(const char *host, int port, int *out_fd, char *err, size_t err_len) {
  struct addrinfo hints;
  struct addrinfo *res = NULL;
  struct addrinfo *it;
  const char *bind_host = NULL;
  char service[16];
  int rc;
  int fd = -1;
  int last_errno = 0;

  (void)memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_flags = AI_PASSIVE;

  if (host != NULL && host[0] != '\0') {
    bind_host = host;
  }

  (void)snprintf(service, sizeof(service), "%d", port);
  rc = getaddrinfo(bind_host, service, &hints, &res);
  if (rc != 0) {
    set_err(err, err_len, "getaddrinfo failed: %s", gai_strerror(rc));
    return -1;
  }

  for (it = res; it != NULL; it = it->ai_next) {
    int one = 1;

    fd = socket(it->ai_family, it->ai_socktype, it->ai_protocol);
    if (fd < 0) {
      last_errno = errno;
      continue;
    }

    (void)setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

    if (bind(fd, it->ai_addr, it->ai_addrlen) == 0 && listen(fd, 128) == 0) {
      *out_fd = fd;
      freeaddrinfo(res);
      return 0;
    }

    last_errno = errno;
    (void)close(fd);
    fd = -1;
  }

  freeaddrinfo(res);
  set_err(err, err_len, "unable to bind/listen: %s", strerror(last_errno));
  return -1;
}

static int connect_tcp_socket(const char *host, int port, int *out_fd, char *err, size_t err_len) {
  struct addrinfo hints;
  struct addrinfo *res = NULL;
  struct addrinfo *it;
  const char *connect_host = host;
  char service[16];
  int rc;
  int fd = -1;
  int last_errno = 0;

  if (out_fd == NULL) {
    set_err(err, err_len, "output fd is required");
    return -1;
  }
  if (port < 0 || port > 65535) {
    set_err(err, err_len, "port out of range: %d", port);
    return -1;
  }

  if (connect_host == NULL || connect_host[0] == '\0' || strcmp(connect_host, "0.0.0.0") == 0) {
    connect_host = "127.0.0.1";
  }

  (void)memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;

  (void)snprintf(service, sizeof(service), "%d", port);
  rc = getaddrinfo(connect_host, service, &hints, &res);
  if (rc != 0) {
    set_err(err, err_len, "getaddrinfo failed: %s", gai_strerror(rc));
    return -1;
  }

  for (it = res; it != NULL; it = it->ai_next) {
    fd = socket(it->ai_family, it->ai_socktype, it->ai_protocol);
    if (fd < 0) {
      last_errno = errno;
      continue;
    }

    if (connect(fd, it->ai_addr, it->ai_addrlen) == 0) {
      *out_fd = fd;
      freeaddrinfo(res);
      return 0;
    }

    last_errno = errno;
    (void)close(fd);
    fd = -1;
  }

  freeaddrinfo(res);
  set_err(err, err_len, "unable to connect: %s", strerror(last_errno));
  return -1;
}

static int create_unix_listener(const char *path, int *out_fd, char *err, size_t err_len) {
  struct sockaddr_un addr;
  int fd;

  if (path == NULL || path[0] == '\0') {
    set_err(err, err_len, "unix path is empty");
    return -1;
  }
  if (strlen(path) >= sizeof(addr.sun_path)) {
    set_err(err, err_len, "unix path is too long");
    return -1;
  }

  fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) {
    set_err(err, err_len, "socket(AF_UNIX) failed: %s", strerror(errno));
    return -1;
  }

  (void)memset(&addr, 0, sizeof(addr));
  addr.sun_family = AF_UNIX;
  (void)strncpy(addr.sun_path, path, sizeof(addr.sun_path) - 1);

  (void)unlink(path);

  if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    set_err(err, err_len, "bind(%s) failed: %s", path, strerror(errno));
    (void)close(fd);
    return -1;
  }

  if (listen(fd, 128) != 0) {
    set_err(err, err_len, "listen(%s) failed: %s", path, strerror(errno));
    (void)close(fd);
    (void)unlink(path);
    return -1;
  }

  *out_fd = fd;
  return 0;
}

static int format_bound_uri(int fd,
                            holons_scheme_t scheme,
                            const char *path,
                            char *out_uri,
                            size_t out_uri_len,
                            char *err,
                            size_t err_len) {
  struct sockaddr_storage addr;
  socklen_t addr_len = sizeof(addr);
  char host[128];
  char host_fmt[130];
  char service[32];
  const char *scheme_name = holons_scheme_name(scheme);
  const char *final_host = host;
  int rc;

  if (getsockname(fd, (struct sockaddr *)&addr, &addr_len) != 0) {
    set_err(err, err_len, "getsockname failed: %s", strerror(errno));
    return -1;
  }

  rc = getnameinfo((struct sockaddr *)&addr,
                   addr_len,
                   host,
                   sizeof(host),
                   service,
                   sizeof(service),
                   NI_NUMERICHOST | NI_NUMERICSERV);
  if (rc != 0) {
    set_err(err, err_len, "getnameinfo failed: %s", gai_strerror(rc));
    return -1;
  }

  if (strchr(host, ':') != NULL) {
    (void)snprintf(host_fmt, sizeof(host_fmt), "[%s]", host);
    final_host = host_fmt;
  }

  if (scheme == HOLONS_SCHEME_TCP) {
    if (snprintf(out_uri, out_uri_len, "%s://%s:%s", scheme_name, final_host, service) >=
        (int)out_uri_len) {
      set_err(err, err_len, "bound URI too long");
      return -1;
    }
    return 0;
  }

  if (path == NULL || path[0] == '\0') {
    path = "/grpc";
  }

  if (snprintf(out_uri, out_uri_len, "%s://%s:%s%s", scheme_name, final_host, service, path) >=
      (int)out_uri_len) {
    set_err(err, err_len, "bound URI too long");
    return -1;
  }
  return 0;
}

static void install_stop_handler(int signo) {
  (void)signo;
  g_stop_requested = 1;
}

const char *holons_default_uri(void) { return HOLONS_DEFAULT_URI; }

holons_scheme_t holons_scheme_from_uri(const char *uri) {
  if (uri == NULL) {
    return HOLONS_SCHEME_INVALID;
  }
  if (strncmp(uri, "tcp://", 6) == 0) {
    return HOLONS_SCHEME_TCP;
  }
  if (strncmp(uri, "unix://", 7) == 0) {
    return HOLONS_SCHEME_UNIX;
  }
  if (strcmp(uri, "stdio://") == 0 || strcmp(uri, "stdio") == 0) {
    return HOLONS_SCHEME_STDIO;
  }
  if (strncmp(uri, "ws://", 5) == 0) {
    return HOLONS_SCHEME_WS;
  }
  if (strncmp(uri, "wss://", 6) == 0) {
    return HOLONS_SCHEME_WSS;
  }
  return HOLONS_SCHEME_INVALID;
}

const char *holons_scheme_name(holons_scheme_t scheme) {
  switch (scheme) {
  case HOLONS_SCHEME_TCP:
    return "tcp";
  case HOLONS_SCHEME_UNIX:
    return "unix";
  case HOLONS_SCHEME_STDIO:
    return "stdio";
  case HOLONS_SCHEME_WS:
    return "ws";
  case HOLONS_SCHEME_WSS:
    return "wss";
  default:
    return "invalid";
  }
}

int holons_parse_flags(int argc, char **argv, char *out_uri, size_t out_uri_len) {
  int i;
  char uri[HOLONS_MAX_URI_LEN];

  if (copy_string(uri, sizeof(uri), HOLONS_DEFAULT_URI, NULL, 0) != 0) {
    return -1;
  }

  for (i = 0; i < argc; ++i) {
    if (strcmp(argv[i], "--listen") == 0 && i + 1 < argc) {
      if (copy_string(uri, sizeof(uri), argv[i + 1], NULL, 0) != 0) {
        return -1;
      }
      break;
    }
    if (strcmp(argv[i], "--port") == 0 && i + 1 < argc) {
      if (snprintf(uri, sizeof(uri), "tcp://:%s", argv[i + 1]) >= (int)sizeof(uri)) {
        return -1;
      }
      break;
    }
  }

  return copy_string(out_uri, out_uri_len, uri, NULL, 0);
}

int holons_parse_uri(const char *uri, holons_uri_t *out, char *err, size_t err_len) {
  const char *rest = NULL;

  if (uri == NULL || out == NULL) {
    set_err(err, err_len, "uri and out must be provided");
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  out->scheme = holons_scheme_from_uri(uri);

  switch (out->scheme) {
  case HOLONS_SCHEME_TCP:
    rest = uri + 6;
    return parse_host_port(rest, out->host, sizeof(out->host), &out->port, err, err_len);
  case HOLONS_SCHEME_UNIX:
    rest = uri + 7;
    if (rest[0] == '\0') {
      set_err(err, err_len, "unix URI requires a path");
      return -1;
    }
    return copy_string(out->path, sizeof(out->path), rest, err, err_len);
  case HOLONS_SCHEME_STDIO:
    return 0;
  case HOLONS_SCHEME_WS:
    rest = uri + 5;
    return parse_ws_uri(rest, out, err, err_len);
  case HOLONS_SCHEME_WSS:
    rest = uri + 6;
    return parse_ws_uri(rest, out, err, err_len);
  default:
    set_err(err, err_len, "unsupported transport URI: %s", uri);
    return -1;
  }
}

int holons_listen(const char *uri, holons_listener_t *out, char *err, size_t err_len) {
  if (out == NULL) {
    set_err(err, err_len, "listener output is required");
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  out->fd = -1;

  if (holons_parse_uri(uri, &out->uri, err, err_len) != 0) {
    return -1;
  }

  switch (out->uri.scheme) {
  case HOLONS_SCHEME_TCP:
    if (create_tcp_listener(out->uri.host, out->uri.port, &out->fd, err, err_len) != 0) {
      return -1;
    }
    return format_bound_uri(out->fd,
                            HOLONS_SCHEME_TCP,
                            NULL,
                            out->bound_uri,
                            sizeof(out->bound_uri),
                            err,
                            err_len);
  case HOLONS_SCHEME_UNIX:
    if (create_unix_listener(out->uri.path, &out->fd, err, err_len) != 0) {
      return -1;
    }
    if (copy_string(out->unix_path, sizeof(out->unix_path), out->uri.path, err, err_len) != 0) {
      (void)holons_close_listener(out);
      return -1;
    }
    if (snprintf(out->bound_uri, sizeof(out->bound_uri), "unix://%s", out->uri.path) >=
        (int)sizeof(out->bound_uri)) {
      set_err(err, err_len, "bound URI too long");
      (void)holons_close_listener(out);
      return -1;
    }
    return 0;
  case HOLONS_SCHEME_STDIO:
    return copy_string(out->bound_uri, sizeof(out->bound_uri), "stdio://", err, err_len);
  case HOLONS_SCHEME_WS:
  case HOLONS_SCHEME_WSS:
    if (create_tcp_listener(out->uri.host, out->uri.port, &out->fd, err, err_len) != 0) {
      return -1;
    }
    return format_bound_uri(out->fd,
                            out->uri.scheme,
                            out->uri.path,
                            out->bound_uri,
                            sizeof(out->bound_uri),
                            err,
                            err_len);
  default:
    set_err(err, err_len, "unsupported transport scheme");
    return -1;
  }
}

int holons_accept(holons_listener_t *listener, holons_conn_t *out, char *err, size_t err_len) {
  int fd = -1;

  if (listener == NULL || out == NULL) {
    set_err(err, err_len, "listener and out must be provided");
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  out->read_fd = -1;
  out->write_fd = -1;
  out->scheme = listener->uri.scheme;

  switch (listener->uri.scheme) {
  case HOLONS_SCHEME_STDIO:
    if (listener->consumed) {
      set_err(err, err_len, "stdio listener is single-use");
      return -1;
    }
    listener->consumed = 1;
    out->read_fd = STDIN_FILENO;
    out->write_fd = STDOUT_FILENO;
    out->owns_read_fd = 0;
    out->owns_write_fd = 0;
    return 0;
  case HOLONS_SCHEME_TCP:
  case HOLONS_SCHEME_UNIX:
  case HOLONS_SCHEME_WS:
  case HOLONS_SCHEME_WSS:
    do {
      fd = accept(listener->fd, NULL, NULL);
    } while (fd < 0 && errno == EINTR && !g_stop_requested);

    if (fd < 0) {
      set_err(err, err_len, "accept failed: %s", strerror(errno));
      return -1;
    }
    out->read_fd = fd;
    out->write_fd = fd;
    out->owns_read_fd = 1;
    out->owns_write_fd = 1;
    return 0;
  default:
    set_err(err, err_len, "listener scheme is invalid");
    return -1;
  }
}

int holons_dial_tcp(const char *host, int port, holons_conn_t *out, char *err, size_t err_len) {
  int fd;

  if (out == NULL) {
    set_err(err, err_len, "connection output is required");
    return -1;
  }
  if (connect_tcp_socket(host, port, &fd, err, err_len) != 0) {
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  out->read_fd = fd;
  out->write_fd = fd;
  out->scheme = HOLONS_SCHEME_TCP;
  out->owns_read_fd = 1;
  out->owns_write_fd = 1;
  return 0;
}

int holons_dial_stdio(holons_conn_t *out, char *err, size_t err_len) {
  if (out == NULL) {
    set_err(err, err_len, "connection output is required");
    return -1;
  }

  (void)memset(out, 0, sizeof(*out));
  out->read_fd = STDIN_FILENO;
  out->write_fd = STDOUT_FILENO;
  out->scheme = HOLONS_SCHEME_STDIO;
  out->owns_read_fd = 0;
  out->owns_write_fd = 0;
  return 0;
}

ssize_t holons_conn_read(const holons_conn_t *conn, void *buf, size_t n) {
  if (conn == NULL || conn->read_fd < 0) {
    errno = EBADF;
    return -1;
  }
  return read(conn->read_fd, buf, n);
}

ssize_t holons_conn_write(const holons_conn_t *conn, const void *buf, size_t n) {
  if (conn == NULL || conn->write_fd < 0) {
    errno = EBADF;
    return -1;
  }
  return write(conn->write_fd, buf, n);
}

int holons_conn_close(holons_conn_t *conn) {
  int rc = 0;
  int saved_errno = 0;

  if (conn == NULL) {
    return 0;
  }

  if (conn->owns_read_fd && conn->read_fd >= 0) {
    if (close(conn->read_fd) != 0) {
      rc = -1;
      saved_errno = errno;
    }
  }

  if (conn->owns_write_fd && conn->write_fd >= 0 && conn->write_fd != conn->read_fd) {
    if (close(conn->write_fd) != 0 && rc == 0) {
      rc = -1;
      saved_errno = errno;
    }
  }

  conn->read_fd = -1;
  conn->write_fd = -1;
  conn->owns_read_fd = 0;
  conn->owns_write_fd = 0;

  if (rc != 0) {
    errno = saved_errno;
  }
  return rc;
}

int holons_close_listener(holons_listener_t *listener) {
  int rc = 0;

  if (listener == NULL) {
    return 0;
  }

  if (listener->fd >= 0) {
    if (close(listener->fd) != 0) {
      rc = -1;
    }
    listener->fd = -1;
  }

  if (listener->uri.scheme == HOLONS_SCHEME_UNIX && listener->unix_path[0] != '\0') {
    (void)unlink(listener->unix_path);
  }

  listener->consumed = 0;
  listener->bound_uri[0] = '\0';
  listener->unix_path[0] = '\0';

  return rc;
}

int holons_serve(const char *listen_uri,
                 holons_conn_handler_t handler,
                 void *ctx,
                 int max_connections,
                 int install_signal_handlers,
                 char *err,
                 size_t err_len) {
  holons_listener_t listener;
  struct sigaction act;
  struct sigaction old_int;
  struct sigaction old_term;
  int previous_stop = g_stop_requested;
  int handled = 0;
  int rc = 0;

  if (handler == NULL) {
    set_err(err, err_len, "handler is required");
    return -1;
  }

  if (listen_uri == NULL || listen_uri[0] == '\0') {
    listen_uri = HOLONS_DEFAULT_URI;
  }

  if (holons_listen(listen_uri, &listener, err, err_len) != 0) {
    return -1;
  }

  if (install_signal_handlers) {
    (void)memset(&act, 0, sizeof(act));
    act.sa_handler = install_stop_handler;
    (void)sigemptyset(&act.sa_mask);
    act.sa_flags = 0;

    (void)sigaction(SIGINT, &act, &old_int);
    (void)sigaction(SIGTERM, &act, &old_term);
  }

  g_stop_requested = 0;

  for (;;) {
    holons_conn_t conn;
    int handler_rc;

    if (g_stop_requested) {
      break;
    }

    if (holons_accept(&listener, &conn, err, err_len) != 0) {
      if (g_stop_requested) {
        break;
      }
      rc = -1;
      break;
    }

    handler_rc = handler(&conn, ctx);
    (void)holons_conn_close(&conn);

    if (handler_rc != 0) {
      set_err(err, err_len, "connection handler returned %d", handler_rc);
      rc = -1;
      break;
    }

    ++handled;

    if (listener.uri.scheme == HOLONS_SCHEME_STDIO) {
      break;
    }

    if (max_connections > 0 && handled >= max_connections) {
      break;
    }
  }

  (void)holons_close_listener(&listener);

  if (install_signal_handlers) {
    (void)sigaction(SIGINT, &old_int, NULL);
    (void)sigaction(SIGTERM, &old_term, NULL);
  }

  g_stop_requested = previous_stop;
  return rc;
}

int holons_resolve_manifest(const char *path,
                            holons_manifest_t *out,
                            char *resolved_path,
                            size_t resolved_path_len,
                            char *err,
                            size_t err_len) {
  char manifest_path[PATH_MAX];
  struct stat st;

  if (out == NULL) {
    set_err(err, err_len, "manifest output is required");
    return -1;
  }
  if (path != NULL && path[0] != '\0' &&
      copy_string(manifest_path, sizeof(manifest_path), path, err, err_len) == 0 &&
      stat(manifest_path, &st) == 0 && S_ISREG(st.st_mode)) {
    if (parse_manifest_file(manifest_path, out, err, err_len) != 0) {
      return -1;
    }
    if (resolved_path != NULL && resolved_path_len > 0 &&
        copy_string(resolved_path, resolved_path_len, manifest_path, err, err_len) != 0) {
      return -1;
    }
    return 0;
  }
  if (resolve_manifest_path(path, manifest_path, sizeof(manifest_path), err, err_len) != 0) {
    return -1;
  }
  if (parse_manifest_file(manifest_path, out, err, err_len) != 0) {
    return -1;
  }
  if (resolved_path != NULL && resolved_path_len > 0 &&
      copy_string(resolved_path, resolved_path_len, manifest_path, err, err_len) != 0) {
    return -1;
  }
  return 0;
}

int holons_parse_holon(const char *path, holons_identity_t *out, char *err, size_t err_len) {
  holons_manifest_t manifest;

  if (path == NULL || out == NULL) {
    set_err(err, err_len, "path and output are required");
    return -1;
  }
  if (holons_resolve_manifest(path, &manifest, NULL, 0, err, err_len) != 0) {
    return -1;
  }
  *out = manifest.identity;
  return 0;
}

static int holons_discover_native_source(const char *root,
                                         holon_entry_t **entries,
                                         size_t *count,
                                         char *err,
                                         size_t err_len) {
  char resolved_root[PATH_MAX];
  holon_entries_t found;

  if (entries == NULL || count == NULL) {
    set_err(err, err_len, "entries and count are required");
    return -1;
  }

  *entries = NULL;
  *count = 0;
  (void)memset(&found, 0, sizeof(found));

  if (resolve_root(root, resolved_root, sizeof(resolved_root), err, err_len) != 0) {
    return -1;
  }
  if (resolved_root[0] == '\0') {
    return 0;
  }

  if (discover_scan_dir(resolved_root, resolved_root, "local", &found, err, err_len) != 0) {
    free(found.items);
    return -1;
  }

  if (found.count > 1) {
    qsort(found.items, found.count, sizeof(*found.items), compare_entries);
  }

  *entries = found.items;
  *count = found.count;
  return 0;
}

int holons_discover_local(holon_entry_t **entries, size_t *count, char *err, size_t err_len) {
  return holons_discover_native_source(NULL, entries, count, err, err_len);
}

int holons_discover_all(holon_entry_t **entries, size_t *count, char *err, size_t err_len) {
  holon_entries_t found;
  char roots[3][PATH_MAX];
  const char *origins[3] = {"local", "$OPBIN", "cache"};
  int i;

  if (entries == NULL || count == NULL) {
    set_err(err, err_len, "entries and count are required");
    return -1;
  }
  *entries = NULL;
  *count = 0;
  (void)memset(&found, 0, sizeof(found));

  if (resolve_root(NULL, roots[0], sizeof(roots[0]), err, err_len) != 0) {
    return -1;
  }
  if (opbin(roots[1], sizeof(roots[1]), err, err_len) != 0) {
    return -1;
  }
  if (cache_dir(roots[2], sizeof(roots[2]), err, err_len) != 0) {
    return -1;
  }

  for (i = 0; i < 3; ++i) {
    holon_entries_t local = {0};
    size_t j;

    if (roots[i][0] == '\0') {
      continue;
    }
    if (discover_scan_dir(roots[i], roots[i], origins[i], &local, err, err_len) != 0) {
      free(found.items);
      free(local.items);
      return -1;
    }
    for (j = 0; j < local.count; ++j) {
      if (append_or_replace_entry(&found, &local.items[j], err, err_len) != 0) {
        free(found.items);
        free(local.items);
        return -1;
      }
    }
    free(local.items);
  }

  if (found.count > 1) {
    qsort(found.items, found.count, sizeof(*found.items), compare_entries);
  }

  *entries = found.items;
  *count = found.count;
  return 0;
}

holon_entry_t *holons_find_by_slug(const char *slug, char *err, size_t err_len) {
  holon_entry_t *entries = NULL;
  holon_entry_t *match = NULL;
  size_t count = 0;
  size_t i;

  if (slug == NULL || slug[0] == '\0') {
    return NULL;
  }

  if (holons_discover_all(&entries, &count, err, err_len) != 0) {
    return NULL;
  }

  for (i = 0; i < count; ++i) {
    if (strcmp(entries[i].slug, slug) != 0) {
      continue;
    }
    if (match != NULL && strcmp(match->uuid, entries[i].uuid) != 0) {
      set_err(err, err_len, "ambiguous holon \"%s\"", slug);
      free(entries);
      return NULL;
    }
    match = &entries[i];
  }

  if (match == NULL) {
    free(entries);
    return NULL;
  }

  {
    holon_entry_t *result = malloc(sizeof(*result));
    if (result == NULL) {
      set_err(err, err_len, "out of memory");
      free(entries);
      return NULL;
    }
    *result = *match;
    free(entries);
    return result;
  }
}

holon_entry_t *holons_find_by_uuid(const char *prefix, char *err, size_t err_len) {
  holon_entry_t *entries = NULL;
  holon_entry_t *match = NULL;
  size_t count = 0;
  size_t i;

  if (prefix == NULL || prefix[0] == '\0') {
    return NULL;
  }

  if (holons_discover_all(&entries, &count, err, err_len) != 0) {
    return NULL;
  }

  for (i = 0; i < count; ++i) {
    if (strncmp(entries[i].uuid, prefix, strlen(prefix)) != 0) {
      continue;
    }
    if (match != NULL && strcmp(match->uuid, entries[i].uuid) != 0) {
      set_err(err, err_len, "ambiguous UUID prefix \"%s\"", prefix);
      free(entries);
      return NULL;
    }
    match = &entries[i];
  }

  if (match == NULL) {
    free(entries);
    return NULL;
  }

  {
    holon_entry_t *result = malloc(sizeof(*result));
    if (result == NULL) {
      set_err(err, err_len, "out of memory");
      free(entries);
      return NULL;
    }
    *result = *match;
    free(entries);
    return result;
  }
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

static int path_is_dir(const char *path) {
  struct stat st;

  if (path == NULL || stat(path, &st) != 0) {
    return 0;
  }
  return S_ISDIR(st.st_mode);
}

static const char *path_basename(const char *path) {
  const char *slash;

  if (path == NULL) {
    return "";
  }

  slash = strrchr(path, '/');
  if (slash == NULL) {
    return path;
  }
  return slash + 1;
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

static int ensure_dir_recursive(const char *path, mode_t mode, char *err, size_t err_len) {
  char tmp[PATH_MAX];
  char *p;

  if (path == NULL || path[0] == '\0') {
    return 0;
  }

  if (copy_string(tmp, sizeof(tmp), path, err, err_len) != 0) {
    return -1;
  }

  for (p = tmp + 1; *p != '\0'; ++p) {
    if (*p != '/') {
      continue;
    }
    *p = '\0';
    if (tmp[0] != '\0') {
      if (mkdir(tmp, mode) != 0) {
        if (errno != EEXIST) {
          set_err(err, err_len, "mkdir(%s) failed: %s", tmp, strerror(errno));
          return -1;
        }
        if (!path_is_dir(tmp)) {
          set_err(err, err_len, "%s exists and is not a directory", tmp);
          return -1;
        }
      }
    }
    *p = '/';
  }

  if (mkdir(tmp, mode) != 0) {
    if (errno != EEXIST) {
      set_err(err, err_len, "mkdir(%s) failed: %s", tmp, strerror(errno));
      return -1;
    }
    if (!path_is_dir(tmp)) {
      set_err(err, err_len, "%s exists and is not a directory", tmp);
      return -1;
    }
  }
  return 0;
}

static int ensure_parent_dir(const char *path, mode_t mode, char *err, size_t err_len) {
  char parent[PATH_MAX];
  char *slash;

  if (path == NULL || path[0] == '\0') {
    return 0;
  }

  if (copy_string(parent, sizeof(parent), path, err, err_len) != 0) {
    return -1;
  }

  slash = strrchr(parent, '/');
  if (slash == NULL) {
    return 0;
  }
  if (slash == parent) {
    slash[1] = '\0';
  } else {
    *slash = '\0';
  }
  return ensure_dir_recursive(parent, mode, err, err_len);
}

static int write_port_file(const char *path, const char *uri, char *err, size_t err_len) {
  char trimmed_uri[HOLONS_MAX_URI_LEN];
  char *value;
  FILE *f;

  if (path == NULL || uri == NULL) {
    set_err(err, err_len, "port file path and URI are required");
    return -1;
  }

  if (ensure_parent_dir(path, 0755, err, err_len) != 0) {
    return -1;
  }
  if (copy_string(trimmed_uri, sizeof(trimmed_uri), uri, err, err_len) != 0) {
    return -1;
  }

  value = trim(trimmed_uri);
  f = fopen(path, "w");
  if (f == NULL) {
    set_err(err, err_len, "cannot open %s: %s", path, strerror(errno));
    return -1;
  }

  if (fprintf(f, "%s\n", value) < 0 || fclose(f) != 0) {
    set_err(err, err_len, "cannot write %s: %s", path, strerror(errno));
    return -1;
  }
  return 0;
}

static int default_port_file_path(const char *slug, char *out, size_t out_len, char *err, size_t err_len) {
  char cwd[PATH_MAX];

  if (slug == NULL || slug[0] == '\0') {
    set_err(err, err_len, "slug is required");
    return -1;
  }
  if (getcwd(cwd, sizeof(cwd)) == NULL) {
    set_err(err, err_len, "getcwd failed: %s", strerror(errno));
    return -1;
  }
  if (snprintf(out, out_len, "%s/.op/run/%s.port", cwd, slug) >= (int)out_len) {
    set_err(err, err_len, "port file path is too long");
    return -1;
  }
  return 0;
}

static int is_direct_target(const char *target) {
  if (target == NULL) {
    return 0;
  }
  return strstr(target, "://") != NULL || strchr(target, ':') != NULL;
}

static int normalize_dial_target(const char *target,
                                 char *out,
                                 size_t out_len,
                                 char *err,
                                 size_t err_len) {
  holons_uri_t uri;
  const char *host;

  if (target == NULL || target[0] == '\0') {
    set_err(err, err_len, "target is required");
    return -1;
  }

  if (strstr(target, "://") == NULL) {
    return copy_string(out, out_len, target, err, err_len);
  }

  if (strncmp(target, "unix://", 7) == 0) {
    return copy_string(out, out_len, target, err, err_len);
  }
  if (strncmp(target, "http://", 7) == 0 || strncmp(target, "https://", 8) == 0 ||
      strncmp(target, "rest+sse://", 11) == 0) {
    return copy_string(out, out_len, target, err, err_len);
  }

  if (holons_parse_uri(target, &uri, err, err_len) != 0) {
    return -1;
  }
  if (uri.scheme != HOLONS_SCHEME_TCP) {
    return copy_string(out, out_len, target, err, err_len);
  }

  host = uri.host;
  if (host[0] == '\0' || strcmp(host, "0.0.0.0") == 0 || strcmp(host, "::") == 0) {
    host = "127.0.0.1";
  }

  if (strchr(host, ':') != NULL) {
    if (snprintf(out, out_len, "[%s]:%d", host, uri.port) >= (int)out_len) {
      set_err(err, err_len, "normalized dial target is too long");
      return -1;
    }
    return 0;
  }

  if (snprintf(out, out_len, "%s:%d", host, uri.port) >= (int)out_len) {
    set_err(err, err_len, "normalized dial target is too long");
    return -1;
  }
  return 0;
}

static int probe_unix_target(const char *path, char *err, size_t err_len) {
  struct sockaddr_un addr;
  int fd;

  if (path == NULL || path[0] == '\0') {
    set_err(err, err_len, "unix path is empty");
    return -1;
  }
  if (strlen(path) >= sizeof(addr.sun_path)) {
    set_err(err, err_len, "unix path is too long");
    return -1;
  }

  fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) {
    set_err(err, err_len, "socket(AF_UNIX) failed: %s", strerror(errno));
    return -1;
  }

  (void)memset(&addr, 0, sizeof(addr));
  addr.sun_family = AF_UNIX;
  (void)strncpy(addr.sun_path, path, sizeof(addr.sun_path) - 1);

  if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    set_err(err, err_len, "connect(%s) failed: %s", path, strerror(errno));
    (void)close(fd);
    return -1;
  }

  (void)close(fd);
  return 0;
}

static int probe_tcp_target(const char *host, int port, char *err, size_t err_len) {
  int fd = -1;

  if (connect_tcp_socket(host, port, &fd, err, err_len) != 0) {
    return -1;
  }

  if (fd >= 0) {
    (void)close(fd);
  }
  return 0;
}

static int wait_for_ready_target(const char *target, int timeout_ms, char *err, size_t err_len) {
  long long deadline;
  char probe_err[256] = "";

  if (target == NULL || target[0] == '\0') {
    set_err(err, err_len, "target is required");
    return -1;
  }

  if (timeout_ms <= 0) {
    timeout_ms = HOLONS_CONNECT_DEFAULT_TIMEOUT_MS;
  }
  deadline = monotonic_millis() + timeout_ms;

  for (;;) {
    if (strstr(target, "://") != NULL) {
      if (strncmp(target, "http://", 7) == 0 || strncmp(target, "https://", 8) == 0 ||
          strncmp(target, "rest+sse://", 11) == 0) {
        char host[128];
        char path[256];
        int port;

        if (parse_http_like_uri(target, host, sizeof(host), &port, path, sizeof(path), probe_err,
                                sizeof(probe_err)) != 0) {
          set_err(err, err_len, "%s", probe_err);
          return -1;
        }
        if (probe_tcp_target(host, port, probe_err, sizeof(probe_err)) == 0) {
          return 0;
        }
      } else {
        holons_uri_t uri;

        if (holons_parse_uri(target, &uri, probe_err, sizeof(probe_err)) != 0) {
          set_err(err, err_len, "%s", probe_err);
          return -1;
        }

        if (uri.scheme == HOLONS_SCHEME_TCP || uri.scheme == HOLONS_SCHEME_WS ||
            uri.scheme == HOLONS_SCHEME_WSS) {
          if (probe_tcp_target(uri.host, uri.port, probe_err, sizeof(probe_err)) == 0) {
            return 0;
          }
        } else if (uri.scheme == HOLONS_SCHEME_UNIX) {
          if (probe_unix_target(uri.path, probe_err, sizeof(probe_err)) == 0) {
            return 0;
          }
        } else {
          set_err(err, err_len, "unsupported connect target: %s", target);
          return -1;
        }
      }
    } else {
      char host[256];
      int port;

      if (parse_host_port(target, host, sizeof(host), &port, probe_err, sizeof(probe_err)) != 0) {
        set_err(err, err_len, "%s", probe_err);
        return -1;
      }
      if (probe_tcp_target(host, port, probe_err, sizeof(probe_err)) == 0) {
        return 0;
      }
    }

    if (monotonic_millis() >= deadline) {
      break;
    }
    sleep_millis(HOLONS_CONNECT_POLL_MS);
  }

  if (probe_err[0] == '\0') {
    set_err(err, err_len, "timed out waiting for connect target");
  } else {
    set_err(err, err_len, "timed out waiting for connect target: %s", probe_err);
  }
  return -1;
}

static int usable_port_file(const char *path,
                            int timeout_ms,
                            char *out_uri,
                            size_t out_uri_len,
                            char *err,
                            size_t err_len) {
  char buf[HOLONS_MAX_URI_LEN];
  char *target;
  FILE *f;
  size_t n;
  int probe_timeout;
  char probe_err[256] = "";

  if (path == NULL || path[0] == '\0' || out_uri == NULL || out_uri_len == 0) {
    return 0;
  }

  f = fopen(path, "r");
  if (f == NULL) {
    return 0;
  }

  n = fread(buf, 1, sizeof(buf) - 1, f);
  if (ferror(f)) {
    (void)fclose(f);
    return 0;
  }
  buf[n] = '\0';
  (void)fclose(f);

  target = trim(buf);
  if (target[0] == '\0') {
    (void)unlink(path);
    return 0;
  }

  probe_timeout = timeout_ms / 4;
  if (probe_timeout <= 0) {
    probe_timeout = 1000;
  }
  if (probe_timeout > 1000) {
    probe_timeout = 1000;
  }

  if (wait_for_ready_target(target, probe_timeout, probe_err, sizeof(probe_err)) == 0) {
    return copy_string(out_uri, out_uri_len, target, err, err_len) == 0 ? 1 : 0;
  }

  (void)unlink(path);
  return 0;
}

static int uri_prefix_at(const char *text) {
  return strncmp(text, "tcp://", 6) == 0 || strncmp(text, "unix://", 7) == 0 ||
         strncmp(text, "ws://", 5) == 0 || strncmp(text, "wss://", 6) == 0 ||
         strncmp(text, "http://", 7) == 0 || strncmp(text, "https://", 8) == 0 ||
         strncmp(text, "rest+sse://", 11) == 0 || strncmp(text, "stdio://", 8) == 0;
}

static int first_uri_in_text(const char *text, char *out, size_t out_len) {
  const char *trim_chars = "\"'()[]{}.,";
  size_t i;

  if (text == NULL || out == NULL || out_len == 0) {
    return 0;
  }

  for (i = 0; text[i] != '\0'; ++i) {
    size_t start = i;
    size_t end = i;
    size_t n;

    if (!uri_prefix_at(text + i)) {
      continue;
    }

    while (text[end] != '\0' && !isspace((unsigned char)text[end])) {
      ++end;
    }
    while (end > start && strchr(trim_chars, text[end - 1]) != NULL) {
      --end;
    }

    n = end - start;
    if (n == 0 || n >= out_len) {
      continue;
    }

    (void)memcpy(out, text + start, n);
    out[n] = '\0';
    return 1;
  }

  return 0;
}

static int read_advertised_uri(int fd,
                               pid_t pid,
                               int timeout_ms,
                               char *out_uri,
                               size_t out_uri_len,
                               char *err,
                               size_t err_len) {
  char buf[4096];
  size_t used = 0;
  long long deadline;

  if (fd < 0 || out_uri == NULL || out_uri_len == 0) {
    set_err(err, err_len, "startup pipe is invalid");
    return -1;
  }

  if (timeout_ms <= 0) {
    timeout_ms = HOLONS_CONNECT_DEFAULT_TIMEOUT_MS;
  }

  buf[0] = '\0';
  deadline = monotonic_millis() + timeout_ms;

  for (;;) {
    fd_set readfds;
    struct timeval tv;
    long long now = monotonic_millis();
    long long remaining = deadline - now;
    int select_ms;
    int status;
    int rc;

    if (first_uri_in_text(buf, out_uri, out_uri_len)) {
      return 0;
    }

    rc = (int)waitpid(pid, &status, WNOHANG);
    if (rc == (int)pid) {
      set_err(err, err_len, "holon exited before advertising an address");
      return -1;
    }
    if (rc < 0 && errno != EINTR && errno != ECHILD) {
      set_err(err, err_len, "waitpid(%ld) failed: %s", (long)pid, strerror(errno));
      return -1;
    }

    if (remaining <= 0) {
      break;
    }

    select_ms = (int)(remaining > HOLONS_CONNECT_POLL_MS ? HOLONS_CONNECT_POLL_MS : remaining);
    FD_ZERO(&readfds);
    FD_SET(fd, &readfds);
    tv.tv_sec = select_ms / 1000;
    tv.tv_usec = (select_ms % 1000) * 1000;

    rc = select(fd + 1, &readfds, NULL, NULL, &tv);
    if (rc < 0) {
      if (errno == EINTR) {
        continue;
      }
      set_err(err, err_len, "select() failed: %s", strerror(errno));
      return -1;
    }
    if (rc == 0) {
      continue;
    }

    if (FD_ISSET(fd, &readfds)) {
      ssize_t n = read(fd, buf + used, sizeof(buf) - 1 - used);

      if (n < 0) {
        if (errno == EINTR || errno == EAGAIN) {
          continue;
        }
        set_err(err, err_len, "read() failed: %s", strerror(errno));
        return -1;
      }
      if (n == 0) {
        continue;
      }

      used += (size_t)n;
      buf[used] = '\0';
      if (first_uri_in_text(buf, out_uri, out_uri_len)) {
        return 0;
      }

      if (used >= sizeof(buf) - 1) {
        size_t keep = used > 1024 ? 1024 : used;
        (void)memmove(buf, buf + used - keep, keep);
        used = keep;
        buf[used] = '\0';
      }
    }
  }

  set_err(err, err_len, "timed out waiting for holon startup");
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

  for (;;) {
    pid_t waited = waitpid(pid, &status, 0);
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
  }
}

static int start_tcp_holon(const char *binary_path,
                           int timeout_ms,
                           char *out_uri,
                           size_t out_uri_len,
                           pid_t *out_pid,
                           int *out_fd,
                           char *err,
                           size_t err_len) {
  int pipefd[2];
  pid_t pid;

  if (binary_path == NULL || binary_path[0] == '\0') {
    set_err(err, err_len, "binary path is required");
    return -1;
  }

  if (pipe(pipefd) != 0) {
    set_err(err, err_len, "pipe() failed: %s", strerror(errno));
    return -1;
  }

  pid = fork();
  if (pid < 0) {
    set_err(err, err_len, "fork() failed: %s", strerror(errno));
    (void)close(pipefd[0]);
    (void)close(pipefd[1]);
    return -1;
  }

  if (pid == 0) {
    (void)close(pipefd[0]);
    if (dup2(pipefd[1], STDOUT_FILENO) < 0 || dup2(pipefd[1], STDERR_FILENO) < 0) {
      _exit(127);
    }
    if (pipefd[1] != STDOUT_FILENO && pipefd[1] != STDERR_FILENO) {
      (void)close(pipefd[1]);
    }
    execl(binary_path, binary_path, "serve", "--listen", "tcp://127.0.0.1:0", (char *)NULL);
    _exit(127);
  }

  (void)close(pipefd[1]);
  if (read_advertised_uri(pipefd[0], pid, timeout_ms, out_uri, out_uri_len, err, err_len) != 0) {
    (void)stop_started_process(pid);
    (void)close(pipefd[0]);
    return -1;
  }

  if (out_pid != NULL) {
    *out_pid = pid;
  }
  if (out_fd != NULL) {
    *out_fd = pipefd[0];
  } else {
    (void)close(pipefd[0]);
  }

  return 0;
}

static int start_stdio_holon(const char *binary_path,
                             int timeout_ms,
                             pid_t *out_pid,
                             int *out_fd,
                             char *err,
                             size_t err_len) {
  int stdin_pipe[2] = {-1, -1};
  int devnull_fd = -1;
  pid_t pid;
  long long deadline;

  if (binary_path == NULL || binary_path[0] == '\0') {
    set_err(err, err_len, "binary path is required");
    return -1;
  }

  if (pipe(stdin_pipe) != 0) {
    set_err(err, err_len, "pipe() failed: %s", strerror(errno));
    return -1;
  }

  devnull_fd = open("/dev/null", O_WRONLY);
  if (devnull_fd < 0) {
    set_err(err, err_len, "open(/dev/null) failed: %s", strerror(errno));
    (void)close(stdin_pipe[0]);
    (void)close(stdin_pipe[1]);
    return -1;
  }

  pid = fork();
  if (pid < 0) {
    set_err(err, err_len, "fork() failed: %s", strerror(errno));
    (void)close(stdin_pipe[0]);
    (void)close(stdin_pipe[1]);
    (void)close(devnull_fd);
    return -1;
  }

  if (pid == 0) {
    (void)close(stdin_pipe[1]);
    if (dup2(stdin_pipe[0], STDIN_FILENO) < 0 || dup2(devnull_fd, STDOUT_FILENO) < 0 ||
        dup2(devnull_fd, STDERR_FILENO) < 0) {
      _exit(127);
    }
    if (stdin_pipe[0] != STDIN_FILENO) {
      (void)close(stdin_pipe[0]);
    }
    if (devnull_fd != STDOUT_FILENO && devnull_fd != STDERR_FILENO) {
      (void)close(devnull_fd);
    }
    execl(binary_path, binary_path, "serve", "--listen", "stdio://", (char *)NULL);
    _exit(127);
  }

  (void)close(stdin_pipe[0]);
  (void)close(devnull_fd);

  deadline = monotonic_millis() + (timeout_ms > 0 && timeout_ms < 200 ? timeout_ms : 200);
  for (;;) {
    int status;
    pid_t waited = waitpid(pid, &status, WNOHANG);

    if (waited == pid) {
      set_err(err, err_len, "holon exited before stdio startup");
      (void)close(stdin_pipe[1]);
      return -1;
    }
    if (waited < 0 && errno != EINTR && errno != ECHILD) {
      set_err(err, err_len, "waitpid(%ld) failed: %s", (long)pid, strerror(errno));
      (void)close(stdin_pipe[1]);
      return -1;
    }
    if (monotonic_millis() >= deadline) {
      break;
    }
    sleep_millis(10);
  }

  if (out_pid != NULL) {
    *out_pid = pid;
  }
  if (out_fd != NULL) {
    *out_fd = stdin_pipe[1];
  } else {
    (void)close(stdin_pipe[1]);
  }

  return 0;
}

static int resolve_binary_path(const holon_entry_t *entry,
                               char *out,
                               size_t out_len,
                               char *err,
                               size_t err_len) {
  char binary_buf[HOLONS_MAX_FIELD_LEN];
  char candidate[PATH_MAX];
  const char *binary_name;
  const char *base_name;
  struct stat st;
  char *path_copy = NULL;
  char *dir;

  if (entry == NULL) {
    set_err(err, err_len, "holon entry is required");
    return -1;
  }
  if (!entry->has_manifest) {
    set_err(err, err_len, "holon \"%s\" has no manifest", entry->slug);
    return -1;
  }
  if (copy_string(binary_buf, sizeof(binary_buf), entry->manifest.artifacts.binary, err, err_len) != 0) {
    return -1;
  }

  binary_name = trim(binary_buf);
  if (binary_name[0] == '\0') {
    set_err(err, err_len, "holon \"%s\" has no artifacts.binary", entry->slug);
    return -1;
  }

  if (binary_name[0] == '/' && access(binary_name, X_OK) == 0 && stat(binary_name, &st) == 0 &&
      S_ISREG(st.st_mode)) {
    return copy_string(out, out_len, binary_name, err, err_len);
  }

  base_name = path_basename(binary_name);
  if (snprintf(candidate, sizeof(candidate), "%s/.op/build/bin/%s", entry->dir, base_name) <
          (int)sizeof(candidate) &&
      access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
    return copy_string(out, out_len, candidate, err, err_len);
  }

  if (getenv("PATH") == NULL || getenv("PATH")[0] == '\0') {
    set_err(err, err_len, "built binary not found for holon \"%s\"", entry->slug);
    return -1;
  }

  path_copy = strdup(getenv("PATH"));
  if (path_copy == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  dir = strtok(path_copy, ":");
  while (dir != NULL) {
    const char *lookup_dir = dir[0] == '\0' ? "." : dir;

    if (snprintf(candidate, sizeof(candidate), "%s/%s", lookup_dir, base_name) < (int)sizeof(candidate) &&
        access(candidate, X_OK) == 0 && stat(candidate, &st) == 0 && S_ISREG(st.st_mode)) {
      int rc = copy_string(out, out_len, candidate, err, err_len);
      free(path_copy);
      return rc;
    }
    dir = strtok(NULL, ":");
  }

  free(path_copy);
  set_err(err, err_len, "built binary not found for holon \"%s\"", entry->slug);
  return -1;
}

static int remember_started_channel(grpc_channel *channel, pid_t pid, int ephemeral, int output_fd) {
  holons_started_channel_t *started = malloc(sizeof(*started));

  if (started == NULL) {
    return -1;
  }

  started->channel = channel;
  started->pid = pid;
  started->ephemeral = ephemeral;
  started->output_fd = output_fd;
  started->next = g_started_channels;
  g_started_channels = started;
  return 0;
}

static holons_started_channel_t *take_started_channel(grpc_channel *channel) {
  holons_started_channel_t **current = &g_started_channels;

  while (*current != NULL) {
    if ((*current)->channel == channel) {
      holons_started_channel_t *match = *current;
      *current = match->next;
      match->next = NULL;
      return match;
    }
    current = &(*current)->next;
  }

  return NULL;
}

static grpc_channel *grpc_insecure_channel_create(const char *target, const void *args, void *reserved) {
  grpc_channel *channel;

  (void)args;
  (void)reserved;

  if (target == NULL || target[0] == '\0') {
    errno = EINVAL;
    return NULL;
  }

  channel = calloc(1, sizeof(*channel));
  if (channel == NULL) {
    return NULL;
  }
  if (copy_string(channel->target, sizeof(channel->target), target, NULL, 0) != 0) {
    free(channel);
    errno = ENAMETOOLONG;
    return NULL;
  }
  return channel;
}

static void grpc_channel_destroy(grpc_channel *channel) { free(channel); }

static grpc_channel *connect_internal(const char *target, holons_connect_options opts, int ephemeral) {
  char target_buf[HOLONS_MAX_URI_LEN];
  char transport_buf[32];
  char port_file_buf[PATH_MAX];
  char port_path[PATH_MAX];
  char dial_target[HOLONS_MAX_URI_LEN];
  char started_uri[HOLONS_MAX_URI_LEN];
  char binary_path[PATH_MAX];
  char err[256] = "";
  const char *transport;
  const char *trimmed_target;
  holon_entry_t *entry = NULL;
  grpc_channel *channel = NULL;
  pid_t pid = -1;
  int output_fd = -1;
  int timeout_ms = opts.timeout_ms;
  int start = opts.start;
  int ephemeral_mode;
  int zero_opts_defaults;

  if (target == NULL) {
    errno = EINVAL;
    return NULL;
  }
  if (copy_string(target_buf, sizeof(target_buf), target, NULL, 0) != 0) {
    errno = ENAMETOOLONG;
    return NULL;
  }

  trimmed_target = trim(target_buf);
  if (trimmed_target[0] == '\0') {
    errno = EINVAL;
    return NULL;
  }

  if (timeout_ms <= 0) {
    timeout_ms = HOLONS_CONNECT_DEFAULT_TIMEOUT_MS;
  }

  transport = opts.transport;
  if (transport == NULL || transport[0] == '\0') {
    transport = "stdio";
  }
  if (copy_string(transport_buf, sizeof(transport_buf), transport, NULL, 0) != 0) {
    errno = EINVAL;
    return NULL;
  }
  transport = trim(transport_buf);
  if (transport[0] == '\0') {
    transport = "stdio";
  }
  for (size_t i = 0; transport[i] != '\0'; ++i) {
    transport_buf[i] = (char)tolower((unsigned char)transport[i]);
  }
  transport = transport_buf;

  zero_opts_defaults =
      opts.timeout_ms == 0 && is_blank_string(opts.transport) && is_blank_string(opts.port_file) && opts.start == 0;
  if (zero_opts_defaults) {
    start = 1;
  }
  ephemeral_mode = ephemeral || strcmp(transport, "stdio") == 0;

  if (strcmp(transport, "tcp") != 0 && strcmp(transport, "stdio") != 0) {
    errno = ENOTSUP;
    return NULL;
  }

  if (is_direct_target(trimmed_target)) {
    if (wait_for_ready_target(trimmed_target, timeout_ms, err, sizeof(err)) != 0) {
      errno = ETIMEDOUT;
      return NULL;
    }
    if (normalize_dial_target(trimmed_target, dial_target, sizeof(dial_target), err, sizeof(err)) != 0) {
      errno = EINVAL;
      return NULL;
    }
    return grpc_insecure_channel_create(dial_target, NULL, NULL);
  }

  entry = holons_find_by_slug(trimmed_target, err, sizeof(err));
  if (entry == NULL) {
    errno = ENOENT;
    return NULL;
  }

  if (!is_blank_string(opts.port_file)) {
    if (copy_string(port_file_buf, sizeof(port_file_buf), opts.port_file, NULL, 0) != 0) {
      errno = ENAMETOOLONG;
      holons_free_entries(entry);
      return NULL;
    }
    if (copy_string(port_path, sizeof(port_path), trim(port_file_buf), NULL, 0) != 0) {
      errno = ENAMETOOLONG;
      holons_free_entries(entry);
      return NULL;
    }
  } else if (default_port_file_path(entry->slug, port_path, sizeof(port_path), err, sizeof(err)) != 0) {
    errno = ENAMETOOLONG;
    holons_free_entries(entry);
    return NULL;
  }

  if (usable_port_file(port_path, timeout_ms, started_uri, sizeof(started_uri), err, sizeof(err))) {
    if (normalize_dial_target(started_uri, dial_target, sizeof(dial_target), err, sizeof(err)) != 0) {
      errno = EINVAL;
      holons_free_entries(entry);
      return NULL;
    }
    holons_free_entries(entry);
    return grpc_insecure_channel_create(dial_target, NULL, NULL);
  }

  if (!start) {
    errno = ENOENT;
    holons_free_entries(entry);
    return NULL;
  }

  if (resolve_binary_path(entry, binary_path, sizeof(binary_path), err, sizeof(err)) != 0) {
    errno = ENOENT;
    holons_free_entries(entry);
    return NULL;
  }
  if (strcmp(transport, "stdio") == 0) {
    if (start_stdio_holon(binary_path, timeout_ms, &pid, &output_fd, err, sizeof(err)) != 0) {
      errno = ETIMEDOUT;
      holons_free_entries(entry);
      return NULL;
    }
    channel = grpc_insecure_channel_create("stdio://", NULL, NULL);
  } else {
    if (start_tcp_holon(binary_path,
                        timeout_ms,
                        started_uri,
                        sizeof(started_uri),
                        &pid,
                        &output_fd,
                        err,
                        sizeof(err)) != 0) {
      errno = ETIMEDOUT;
      holons_free_entries(entry);
      return NULL;
    }
    if (wait_for_ready_target(started_uri, timeout_ms, err, sizeof(err)) != 0) {
      (void)stop_started_process(pid);
      if (output_fd >= 0) {
        (void)close(output_fd);
      }
      errno = ETIMEDOUT;
      holons_free_entries(entry);
      return NULL;
    }
    if (normalize_dial_target(started_uri, dial_target, sizeof(dial_target), err, sizeof(err)) != 0) {
      (void)stop_started_process(pid);
      if (output_fd >= 0) {
        (void)close(output_fd);
      }
      errno = EINVAL;
      holons_free_entries(entry);
      return NULL;
    }

    channel = grpc_insecure_channel_create(dial_target, NULL, NULL);
  }
  if (channel == NULL) {
    (void)stop_started_process(pid);
    if (output_fd >= 0) {
      (void)close(output_fd);
    }
    holons_free_entries(entry);
    return NULL;
  }

  if (!ephemeral_mode && strcmp(transport, "tcp") == 0 && write_port_file(port_path, started_uri, err, sizeof(err)) != 0) {
    grpc_channel_destroy(channel);
    (void)stop_started_process(pid);
    if (output_fd >= 0) {
      (void)close(output_fd);
    }
    errno = EIO;
    holons_free_entries(entry);
    return NULL;
  }

  if (remember_started_channel(channel, pid, ephemeral_mode, output_fd) != 0) {
    grpc_channel_destroy(channel);
    (void)stop_started_process(pid);
    if (output_fd >= 0) {
      (void)close(output_fd);
    }
    errno = ENOMEM;
    holons_free_entries(entry);
    return NULL;
  }

  holons_free_entries(entry);
  return channel;
}

static grpc_channel *holons_connect_legacy(const char *target) {
  holons_connect_options opts;

  opts.timeout_ms = HOLONS_CONNECT_DEFAULT_TIMEOUT_MS;
  opts.transport = "stdio";
  opts.start = 1;
  opts.port_file = NULL;
  return connect_internal(target, opts, 1);
}

grpc_channel *holons_connect_with_opts(const char *target, holons_connect_options opts) {
  return connect_internal(target, opts, 0);
}

static void holons_disconnect_channel(grpc_channel *channel) {
  holons_started_channel_t *started;

  if (channel == NULL) {
    return;
  }

  started = take_started_channel(channel);
  grpc_channel_destroy(channel);

  if (started == NULL) {
    return;
  }

  if (started->ephemeral) {
    (void)stop_started_process(started->pid);
  }
  if (started->output_fd >= 0) {
    (void)close(started->output_fd);
  }
  free(started);
}

void holons_free_entries(holon_entry_t *entries) { free(entries); }

volatile sig_atomic_t *holons_stop_token(void) { return &g_stop_requested; }

void holons_request_stop(void) { g_stop_requested = 1; }

#define HOLONS_DESCRIBE_SERVICE_NAME "holons.v1.HolonMeta"
#define HOLONS_MAX_COMMENT_LINES 32
#define HOLONS_MAX_SCOPE_LEN HOLONS_MAX_FIELD_LEN
#define HOLONS_SCALAR_TYPE_COUNT 15

typedef struct {
  char description[HOLONS_MAX_DOC_LEN];
  int required;
  char example[HOLONS_MAX_DOC_LEN];
} holons_comment_meta_t;

typedef enum {
  HOLONS_CARDINALITY_OPTIONAL = 0,
  HOLONS_CARDINALITY_REPEATED = 1,
  HOLONS_CARDINALITY_MAP = 2
} holons_cardinality_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  int number;
  holons_comment_meta_t comment;
} holons_enum_value_def_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char type_name[HOLONS_MAX_FIELD_LEN];
  char raw_type[HOLONS_MAX_FIELD_LEN];
  int number;
  holons_comment_meta_t comment;
  holons_cardinality_t cardinality;
  char package_name[HOLONS_MAX_FIELD_LEN];
  char scope[HOLONS_MAX_SCOPE_LEN];
  char map_key_type[HOLONS_MAX_FIELD_LEN];
  char map_value_type[HOLONS_MAX_FIELD_LEN];
} holons_field_def_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char full_name[HOLONS_MAX_FIELD_LEN];
  char package_name[HOLONS_MAX_FIELD_LEN];
  char scope[HOLONS_MAX_SCOPE_LEN];
  holons_comment_meta_t comment;
  holons_field_def_t *fields;
  size_t field_count;
  size_t field_capacity;
} holons_message_def_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char full_name[HOLONS_MAX_FIELD_LEN];
  char package_name[HOLONS_MAX_FIELD_LEN];
  char scope[HOLONS_MAX_SCOPE_LEN];
  holons_comment_meta_t comment;
  holons_enum_value_def_t *values;
  size_t value_count;
  size_t value_capacity;
} holons_enum_def_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char input_type[HOLONS_MAX_FIELD_LEN];
  char output_type[HOLONS_MAX_FIELD_LEN];
  int client_streaming;
  int server_streaming;
  holons_comment_meta_t comment;
} holons_method_def_t;

typedef struct {
  char name[HOLONS_MAX_FIELD_LEN];
  char full_name[HOLONS_MAX_FIELD_LEN];
  holons_comment_meta_t comment;
  holons_method_def_t *methods;
  size_t method_count;
  size_t method_capacity;
} holons_service_def_t;

typedef struct {
  char key[HOLONS_MAX_FIELD_LEN];
  char full_name[HOLONS_MAX_FIELD_LEN];
} holons_simple_type_t;

typedef struct {
  holons_service_def_t *services;
  size_t service_count;
  size_t service_capacity;
  holons_message_def_t *messages;
  size_t message_count;
  size_t message_capacity;
  holons_enum_def_t *enums;
  size_t enum_count;
  size_t enum_capacity;
  holons_simple_type_t *simple_types;
  size_t simple_type_count;
  size_t simple_type_capacity;
} holons_proto_index_t;

typedef struct {
  int kind;
  size_t index;
  char name[HOLONS_MAX_FIELD_LEN];
} holons_block_t;

enum {
  HOLONS_BLOCK_SERVICE = 1,
  HOLONS_BLOCK_MESSAGE = 2,
  HOLONS_BLOCK_ENUM = 3
};

static const char *holons_scalar_types[HOLONS_SCALAR_TYPE_COUNT] = {
    "double",  "float",   "int64",   "uint64",  "int32",
    "fixed64", "fixed32", "bool",    "string",  "bytes",
    "uint32",  "sfixed32","sfixed64","sint32",  "sint64"};

static void holons_init_describe_response(holons_describe_response_t *out) {
  if (out != NULL) {
    (void)memset(out, 0, sizeof(*out));
  }
}

static void holons_free_field_docs(holons_field_doc_t *fields, size_t count) {
  size_t i;
  if (fields == NULL) {
    return;
  }
  for (i = 0; i < count; ++i) {
    holons_free_field_docs(fields[i].nested_fields, fields[i].nested_field_count);
    free(fields[i].enum_values);
  }
  free(fields);
}

void holons_free_describe_response(holons_describe_response_t *response) {
  size_t i;

  if (response == NULL) {
    return;
  }
  if (response->services != NULL) {
    for (i = 0; i < response->service_count; ++i) {
      size_t j;
      for (j = 0; j < response->services[i].method_count; ++j) {
        holons_free_field_docs(response->services[i].methods[j].input_fields,
                               response->services[i].methods[j].input_field_count);
        holons_free_field_docs(response->services[i].methods[j].output_fields,
                               response->services[i].methods[j].output_field_count);
      }
      free(response->services[i].methods);
    }
    free(response->services);
  }
  (void)memset(response, 0, sizeof(*response));
}

static int holons_clone_enum_value_docs(const holons_enum_value_doc_t *values,
                                        size_t count,
                                        holons_enum_value_doc_t **out,
                                        char *err,
                                        size_t err_len) {
  if (out == NULL) {
    set_err(err, err_len, "enum value output is required");
    return -1;
  }
  *out = NULL;
  if (count == 0 || values == NULL) {
    return 0;
  }

  *out = calloc(count, sizeof(**out));
  if (*out == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }
  memcpy(*out, values, count * sizeof(**out));
  return 0;
}

static int holons_clone_field_docs(const holons_field_doc_t *fields,
                                   size_t count,
                                   holons_field_doc_t **out,
                                   char *err,
                                   size_t err_len) {
  size_t i;

  if (out == NULL) {
    set_err(err, err_len, "field output is required");
    return -1;
  }
  *out = NULL;
  if (count == 0 || fields == NULL) {
    return 0;
  }

  *out = calloc(count, sizeof(**out));
  if (*out == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  for (i = 0; i < count; ++i) {
    (*out)[i] = fields[i];
    (*out)[i].nested_fields = NULL;
    (*out)[i].nested_field_count = 0;
    (*out)[i].enum_values = NULL;
    (*out)[i].enum_value_count = 0;
    if (holons_clone_field_docs(fields[i].nested_fields,
                                fields[i].nested_field_count,
                                &(*out)[i].nested_fields,
                                err,
                                err_len) != 0) {
      holons_free_field_docs(*out, i + 1);
      *out = NULL;
      return -1;
    }
    (*out)[i].nested_field_count = fields[i].nested_field_count;
    if (holons_clone_enum_value_docs(fields[i].enum_values,
                                     fields[i].enum_value_count,
                                     &(*out)[i].enum_values,
                                     err,
                                     err_len) != 0) {
      holons_free_field_docs(*out, i + 1);
      *out = NULL;
      return -1;
    }
    (*out)[i].enum_value_count = fields[i].enum_value_count;
  }

  return 0;
}

static int holons_clone_method_docs(const holons_method_doc_t *methods,
                                    size_t count,
                                    holons_method_doc_t **out,
                                    char *err,
                                    size_t err_len) {
  size_t i;

  if (out == NULL) {
    set_err(err, err_len, "method output is required");
    return -1;
  }
  *out = NULL;
  if (count == 0 || methods == NULL) {
    return 0;
  }

  *out = calloc(count, sizeof(**out));
  if (*out == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  for (i = 0; i < count; ++i) {
    (*out)[i] = methods[i];
    (*out)[i].input_fields = NULL;
    (*out)[i].input_field_count = 0;
    (*out)[i].output_fields = NULL;
    (*out)[i].output_field_count = 0;
    if (holons_clone_field_docs(methods[i].input_fields,
                                methods[i].input_field_count,
                                &(*out)[i].input_fields,
                                err,
                                err_len) != 0) {
      size_t j;
      for (j = 0; j <= i; ++j) {
        holons_free_field_docs((*out)[j].input_fields, (*out)[j].input_field_count);
        holons_free_field_docs((*out)[j].output_fields, (*out)[j].output_field_count);
      }
      free(*out);
      *out = NULL;
      return -1;
    }
    (*out)[i].input_field_count = methods[i].input_field_count;
    if (holons_clone_field_docs(methods[i].output_fields,
                                methods[i].output_field_count,
                                &(*out)[i].output_fields,
                                err,
                                err_len) != 0) {
      size_t j;
      for (j = 0; j <= i; ++j) {
        holons_free_field_docs((*out)[j].input_fields, (*out)[j].input_field_count);
        holons_free_field_docs((*out)[j].output_fields, (*out)[j].output_field_count);
      }
      free(*out);
      *out = NULL;
      return -1;
    }
    (*out)[i].output_field_count = methods[i].output_field_count;
  }

  return 0;
}

static void holons_free_method_docs(holons_method_doc_t *methods, size_t count) {
  size_t i;
  if (methods == NULL) {
    return;
  }
  for (i = 0; i < count; ++i) {
    holons_free_field_docs(methods[i].input_fields, methods[i].input_field_count);
    holons_free_field_docs(methods[i].output_fields, methods[i].output_field_count);
  }
  free(methods);
}

static int holons_clone_service_docs(const holons_service_doc_t *services,
                                     size_t count,
                                     holons_service_doc_t **out,
                                     char *err,
                                     size_t err_len) {
  size_t i;

  if (out == NULL) {
    set_err(err, err_len, "service output is required");
    return -1;
  }
  *out = NULL;
  if (count == 0 || services == NULL) {
    return 0;
  }

  *out = calloc(count, sizeof(**out));
  if (*out == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  for (i = 0; i < count; ++i) {
    (*out)[i] = services[i];
    (*out)[i].methods = NULL;
    (*out)[i].method_count = 0;
    if (holons_clone_method_docs(services[i].methods,
                                 services[i].method_count,
                                 &(*out)[i].methods,
                                 err,
                                 err_len) != 0) {
      size_t j;
      for (j = 0; j < i; ++j) {
        holons_free_method_docs((*out)[j].methods, (*out)[j].method_count);
      }
      free(*out);
      *out = NULL;
      return -1;
    }
    (*out)[i].method_count = services[i].method_count;
  }

  return 0;
}

static int holons_clone_describe_response(const holons_describe_response_t *response,
                                          holons_describe_response_t *out,
                                          char *err,
                                          size_t err_len) {
  if (out == NULL) {
    set_err(err, err_len, "response output is required");
    return -1;
  }

  holons_init_describe_response(out);
  if (response == NULL) {
    set_err(err, err_len, "%s", holons_no_incode_description_error);
    return -1;
  }

  out->manifest = response->manifest;
  if (holons_clone_service_docs(response->services,
                                response->service_count,
                                &out->services,
                                err,
                                err_len) != 0) {
    holons_free_describe_response(out);
    return -1;
  }
  out->service_count = response->service_count;
  return 0;
}

static void holons_free_proto_index(holons_proto_index_t *index) {
  size_t i;
  if (index == NULL) {
    return;
  }
  for (i = 0; i < index->service_count; ++i) {
    free(index->services[i].methods);
  }
  for (i = 0; i < index->message_count; ++i) {
    free(index->messages[i].fields);
  }
  for (i = 0; i < index->enum_count; ++i) {
    free(index->enums[i].values);
  }
  free(index->services);
  free(index->messages);
  free(index->enums);
  free(index->simple_types);
  (void)memset(index, 0, sizeof(*index));
}

static int holons_is_scalar_type(const char *type_name) {
  size_t i;
  for (i = 0; i < HOLONS_SCALAR_TYPE_COUNT; ++i) {
    if (strcmp(type_name, holons_scalar_types[i]) == 0) {
      return 1;
    }
  }
  return 0;
}

static int holons_ensure_capacity(void **items,
                                  size_t *capacity,
                                  size_t count,
                                  size_t item_size,
                                  char *err,
                                  size_t err_len) {
  size_t next_capacity;
  void *grown;

  if (count < *capacity) {
    return 0;
  }

  next_capacity = (*capacity == 0) ? 4 : (*capacity * 2);
  grown = realloc(*items, next_capacity * item_size);
  if (grown == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }

  *items = grown;
  *capacity = next_capacity;
  return 0;
}

static void holons_clear_pending_comments(char comments[][HOLONS_MAX_DOC_LEN], size_t *count) {
  size_t i;
  if (count == NULL) {
    return;
  }
  for (i = 0; i < *count; ++i) {
    comments[i][0] = '\0';
  }
  *count = 0;
}

static void holons_append_comment_line(char comments[][HOLONS_MAX_DOC_LEN],
                                       size_t *count,
                                       const char *line) {
  if (count == NULL || line == NULL) {
    return;
  }
  if (*count >= HOLONS_MAX_COMMENT_LINES) {
    return;
  }
  (void)copy_string(comments[*count], HOLONS_MAX_DOC_LEN, line, NULL, 0);
  *count += 1;
}

static void holons_comment_meta_from_lines(const char comments[][HOLONS_MAX_DOC_LEN],
                                           size_t count,
                                           holons_comment_meta_t *out) {
  size_t i;

  (void)memset(out, 0, sizeof(*out));
  for (i = 0; i < count; ++i) {
    const char *line = comments[i];
    if (strcmp(line, "@required") == 0) {
      out->required = 1;
      continue;
    }
    if (strncmp(line, "@example ", 9) == 0) {
      (void)copy_string(out->example, sizeof(out->example), line + 9, NULL, 0);
      continue;
    }
    if (line[0] != '\0') {
      if (out->description[0] != '\0' &&
          strlen(out->description) + 1 < sizeof(out->description)) {
        strncat(out->description,
                " ",
                sizeof(out->description) - strlen(out->description) - 1);
      }
      if (strlen(out->description) + strlen(line) < sizeof(out->description)) {
        strncat(out->description,
                line,
                sizeof(out->description) - strlen(out->description) - 1);
      }
    }
  }
}

static int holons_parse_name_after_keyword(const char *line,
                                           const char *keyword,
                                           char *out,
                                           size_t out_len) {
  const char *p;
  size_t n = 0;

  if (line == NULL || keyword == NULL || out == NULL || out_len == 0) {
    return 0;
  }
  if (strncmp(line, keyword, strlen(keyword)) != 0) {
    return 0;
  }
  p = line + strlen(keyword);
  while (*p != '\0' && isspace((unsigned char)*p)) {
    ++p;
  }
  while (*p != '\0' &&
         (isalnum((unsigned char)*p) || *p == '_' || *p == '.')) {
    if (n + 1 >= out_len) {
      return 0;
    }
    out[n++] = *p++;
  }
  out[n] = '\0';
  return n > 0;
}

static int holons_parse_package_line(const char *line, char *out, size_t out_len) {
  char name[HOLONS_MAX_FIELD_LEN];
  if (!holons_parse_name_after_keyword(line, "package", name, sizeof(name))) {
    return 0;
  }
  return copy_string(out, out_len, name, NULL, 0) == 0;
}

static int holons_parse_rpc_line(const char *line,
                                 char *name,
                                 size_t name_len,
                                 char *input_type,
                                 size_t input_len,
                                 int *client_streaming,
                                 char *output_type,
                                 size_t output_len,
                                 int *server_streaming) {
  const char *p;
  const char *open_paren;
  const char *close_paren;
  const char *returns_kw;
  const char *out_open;
  const char *out_close;
  char in_buf[HOLONS_MAX_FIELD_LEN];
  char out_buf[HOLONS_MAX_FIELD_LEN];
  size_t n = 0;

  if (strncmp(line, "rpc ", 4) != 0) {
    return 0;
  }
  p = line + 4;
  while (*p != '\0' && isspace((unsigned char)*p)) {
    ++p;
  }
  while (*p != '\0' &&
         (isalnum((unsigned char)*p) || *p == '_' || *p == '.')) {
    if (n + 1 >= name_len) {
      return 0;
    }
    name[n++] = *p++;
  }
  name[n] = '\0';
  if (name[0] == '\0') {
    return 0;
  }

  open_paren = strchr(p, '(');
  close_paren = open_paren != NULL ? strchr(open_paren + 1, ')') : NULL;
  returns_kw = close_paren != NULL ? strstr(close_paren, "returns") : NULL;
  out_open = returns_kw != NULL ? strchr(returns_kw, '(') : NULL;
  out_close = out_open != NULL ? strchr(out_open + 1, ')') : NULL;
  if (open_paren == NULL || close_paren == NULL || out_open == NULL || out_close == NULL) {
    return 0;
  }

  n = (size_t)(close_paren - open_paren - 1);
  if (n >= sizeof(in_buf)) {
    return 0;
  }
  memcpy(in_buf, open_paren + 1, n);
  in_buf[n] = '\0';
  n = (size_t)(out_close - out_open - 1);
  if (n >= sizeof(out_buf)) {
    return 0;
  }
  memcpy(out_buf, out_open + 1, n);
  out_buf[n] = '\0';

  p = trim(in_buf);
  *client_streaming = 0;
  if (strncmp(p, "stream ", 7) == 0) {
    *client_streaming = 1;
    p = trim((char *)(p + 7));
  }
  if (copy_string(input_type, input_len, p, NULL, 0) != 0) {
    return 0;
  }

  p = trim(out_buf);
  *server_streaming = 0;
  if (strncmp(p, "stream ", 7) == 0) {
    *server_streaming = 1;
    p = trim((char *)(p + 7));
  }
  if (copy_string(output_type, output_len, p, NULL, 0) != 0) {
    return 0;
  }

  return 1;
}

static int holons_parse_map_field_line(const char *line,
                                       char *key_type,
                                       size_t key_len,
                                       char *value_type,
                                       size_t value_len,
                                       char *name,
                                       size_t name_len,
                                       int *number) {
  char buf[1024];
  char *p;
  char *lt;
  char *comma;
  char *gt;
  char *eq;
  char *value;

  if (copy_string(buf, sizeof(buf), line, NULL, 0) != 0) {
    return 0;
  }
  p = trim(buf);
  if (strncmp(p, "repeated ", 9) == 0) {
    p = trim(p + 9);
  }
  if (strncmp(p, "map", 3) != 0) {
    return 0;
  }
  lt = strchr(p, '<');
  comma = lt != NULL ? strchr(lt + 1, ',') : NULL;
  gt = comma != NULL ? strchr(comma + 1, '>') : NULL;
  eq = gt != NULL ? strchr(gt + 1, '=') : NULL;
  if (lt == NULL || comma == NULL || gt == NULL || eq == NULL) {
    return 0;
  }

  *comma = '\0';
  *gt = '\0';
  *eq = '\0';
  if (copy_string(key_type, key_len, trim(lt + 1), NULL, 0) != 0) {
    return 0;
  }
  if (copy_string(value_type, value_len, trim(comma + 1), NULL, 0) != 0) {
    return 0;
  }
  if (copy_string(name, name_len, trim(gt + 1), NULL, 0) != 0) {
    return 0;
  }
  value = trim(eq + 1);
  if (value[0] == '\0') {
    return 0;
  }
  if (sscanf(value, "%d", number) != 1) {
    return 0;
  }
  return 1;
}

static int holons_parse_field_line(const char *line,
                                   char *type_name,
                                   size_t type_len,
                                   char *name,
                                   size_t name_len,
                                   int *number,
                                   holons_cardinality_t *cardinality) {
  char buf[1024];
  char *p;
  char *space;
  char *eq;
  char *value;

  if (copy_string(buf, sizeof(buf), line, NULL, 0) != 0) {
    return 0;
  }
  p = trim(buf);
  if (strncmp(p, "repeated ", 9) == 0) {
    *cardinality = HOLONS_CARDINALITY_REPEATED;
    p = trim(p + 9);
  } else if (strncmp(p, "optional ", 9) == 0) {
    *cardinality = HOLONS_CARDINALITY_OPTIONAL;
    p = trim(p + 9);
  } else {
    *cardinality = HOLONS_CARDINALITY_OPTIONAL;
  }
  if (strncmp(p, "map", 3) == 0) {
    return 0;
  }
  space = strchr(p, ' ');
  eq = strchr(p, '=');
  if (space == NULL || eq == NULL || space > eq) {
    return 0;
  }
  *space = '\0';
  if (copy_string(type_name, type_len, trim(p), NULL, 0) != 0) {
    return 0;
  }
  *eq = '\0';
  if (copy_string(name, name_len, trim(space + 1), NULL, 0) != 0) {
    return 0;
  }
  value = trim(eq + 1);
  if (sscanf(value, "%d", number) != 1) {
    return 0;
  }
  return 1;
}

static int holons_parse_enum_value_line(const char *line,
                                        char *name,
                                        size_t name_len,
                                        int *number) {
  char buf[1024];
  char *eq;
  char *value;

  if (copy_string(buf, sizeof(buf), line, NULL, 0) != 0) {
    return 0;
  }
  eq = strchr(buf, '=');
  if (eq == NULL) {
    return 0;
  }
  *eq = '\0';
  if (copy_string(name, name_len, trim(buf), NULL, 0) != 0) {
    return 0;
  }
  value = trim(eq + 1);
  if (sscanf(value, "%d", number) != 1) {
    return 0;
  }
  return 1;
}

static void holons_scope_from_stack(const holons_block_t *stack,
                                    size_t stack_count,
                                    char *out,
                                    size_t out_len) {
  size_t i;
  out[0] = '\0';
  for (i = 0; i < stack_count; ++i) {
    if (stack[i].kind != HOLONS_BLOCK_MESSAGE) {
      continue;
    }
    if (out[0] != '\0' && strlen(out) + 1 < out_len) {
      strncat(out, ".", out_len - strlen(out) - 1);
    }
    if (strlen(out) + strlen(stack[i].name) < out_len) {
      strncat(out, stack[i].name, out_len - strlen(out) - 1);
    }
  }
}

static void holons_qualify_scope(const char *scope,
                                 const char *name,
                                 char *out,
                                 size_t out_len) {
  if (scope == NULL || scope[0] == '\0') {
    (void)copy_string(out, out_len, name, NULL, 0);
    return;
  }
  (void)snprintf(out, out_len, "%s.%s", scope, name);
}

static ssize_t holons_find_message_index(const holons_proto_index_t *index,
                                         const char *full_name) {
  size_t i;
  for (i = 0; i < index->message_count; ++i) {
    if (strcmp(index->messages[i].full_name, full_name) == 0) {
      return (ssize_t)i;
    }
  }
  return -1;
}

static ssize_t holons_find_enum_index(const holons_proto_index_t *index,
                                      const char *full_name) {
  size_t i;
  for (i = 0; i < index->enum_count; ++i) {
    if (strcmp(index->enums[i].full_name, full_name) == 0) {
      return (ssize_t)i;
    }
  }
  return -1;
}

static void holons_add_simple_type(holons_proto_index_t *index,
                                   const char *key,
                                   const char *full_name,
                                   char *err,
                                   size_t err_len) {
  size_t i;
  if (key == NULL || key[0] == '\0' || full_name == NULL || full_name[0] == '\0') {
    return;
  }
  for (i = 0; i < index->simple_type_count; ++i) {
    if (strcmp(index->simple_types[i].key, key) == 0) {
      return;
    }
  }
  if (holons_ensure_capacity((void **)&index->simple_types,
                             &index->simple_type_capacity,
                             index->simple_type_count,
                             sizeof(*index->simple_types),
                             err,
                             err_len) != 0) {
    return;
  }
  (void)memset(&index->simple_types[index->simple_type_count], 0,
               sizeof(index->simple_types[index->simple_type_count]));
  (void)copy_string(index->simple_types[index->simple_type_count].key,
                    sizeof(index->simple_types[index->simple_type_count].key),
                    key,
                    NULL,
                    0);
  (void)copy_string(index->simple_types[index->simple_type_count].full_name,
                    sizeof(index->simple_types[index->simple_type_count].full_name),
                    full_name,
                    NULL,
                    0);
  index->simple_type_count += 1;
}

static int holons_find_simple_type(const holons_proto_index_t *index,
                                   const char *key,
                                   char *out,
                                   size_t out_len) {
  size_t i;
  for (i = 0; i < index->simple_type_count; ++i) {
    if (strcmp(index->simple_types[i].key, key) == 0) {
      return copy_string(out, out_len, index->simple_types[i].full_name, NULL, 0);
    }
  }
  return -1;
}

static void holons_resolve_type(const char *type_name,
                                const char *package_name,
                                const char *scope,
                                const holons_proto_index_t *index,
                                char *out,
                                size_t out_len) {
  char candidate[HOLONS_MAX_FIELD_LEN];
  char scope_buf[HOLONS_MAX_SCOPE_LEN];
  char *last_dot;

  if (type_name == NULL || type_name[0] == '\0') {
    out[0] = '\0';
    return;
  }
  if (type_name[0] == '.') {
    (void)copy_string(out, out_len, type_name + 1, NULL, 0);
    return;
  }
  if (holons_is_scalar_type(type_name)) {
    (void)copy_string(out, out_len, type_name, NULL, 0);
    return;
  }

  (void)copy_string(scope_buf, sizeof(scope_buf), scope != NULL ? scope : "", NULL, 0);
  while (scope_buf[0] != '\0') {
    char qualified[HOLONS_MAX_FIELD_LEN];
    holons_qualify_scope(scope_buf, type_name, qualified, sizeof(qualified));
    (void)snprintf(candidate,
                   sizeof(candidate),
                   "%s%s%s",
                   package_name != NULL && package_name[0] != '\0' ? package_name : "",
                   package_name != NULL && package_name[0] != '\0' ? "." : "",
                   qualified);
    if (holons_find_message_index(index, candidate) >= 0 ||
        holons_find_enum_index(index, candidate) >= 0) {
      (void)copy_string(out, out_len, candidate, NULL, 0);
      return;
    }
    last_dot = strrchr(scope_buf, '.');
    if (last_dot == NULL) {
      scope_buf[0] = '\0';
    } else {
      *last_dot = '\0';
    }
  }

  if (package_name != NULL && package_name[0] != '\0') {
    (void)snprintf(candidate, sizeof(candidate), "%s.%s", package_name, type_name);
  } else {
    (void)copy_string(candidate, sizeof(candidate), type_name, NULL, 0);
  }
  if (holons_find_message_index(index, candidate) >= 0 ||
      holons_find_enum_index(index, candidate) >= 0) {
    (void)copy_string(out, out_len, candidate, NULL, 0);
    return;
  }
  if (holons_find_simple_type(index, type_name, out, out_len) == 0) {
    return;
  }
  (void)copy_string(out, out_len, candidate, NULL, 0);
}

static int holons_parse_proto_file(const char *path,
                                   holons_proto_index_t *index,
                                   char *err,
                                   size_t err_len) {
  FILE *f;
  char line[1024];
  char package_name[HOLONS_MAX_FIELD_LEN] = "";
  holons_block_t stack[32];
  size_t stack_count = 0;
  char pending_comments[HOLONS_MAX_COMMENT_LINES][HOLONS_MAX_DOC_LEN];
  size_t pending_count = 0;

  f = fopen(path, "r");
  if (f == NULL) {
    set_err(err, err_len, "cannot open %s: %s", path, strerror(errno));
    return -1;
  }

  while (fgets(line, sizeof(line), f) != NULL) {
    char raw[1024];
    char *value;
    char name[HOLONS_MAX_FIELD_LEN];
    char scope[HOLONS_MAX_SCOPE_LEN];
    holons_comment_meta_t comment;

    (void)copy_string(raw, sizeof(raw), line, NULL, 0);
    value = trim(raw);
    if (value[0] == '\0') {
      continue;
    }
    if (strncmp(value, "//", 2) == 0) {
      holons_append_comment_line(pending_comments, &pending_count, trim(value + 2));
      continue;
    }

    if (holons_parse_package_line(value, package_name, sizeof(package_name))) {
      holons_clear_pending_comments(pending_comments, &pending_count);
      continue;
    }
    if (holons_parse_name_after_keyword(value, "service", name, sizeof(name))) {
      holons_service_def_t *service;
      if (holons_ensure_capacity((void **)&index->services,
                                 &index->service_capacity,
                                 index->service_count,
                                 sizeof(*index->services),
                                 err,
                                 err_len) != 0) {
        (void)fclose(f);
        return -1;
      }
      service = &index->services[index->service_count];
      (void)memset(service, 0, sizeof(*service));
      holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
      (void)copy_string(service->name, sizeof(service->name), name, NULL, 0);
      if (package_name[0] != '\0') {
        (void)snprintf(service->full_name, sizeof(service->full_name), "%s.%s", package_name, name);
      } else {
        (void)copy_string(service->full_name, sizeof(service->full_name), name, NULL, 0);
      }
      service->comment = comment;
      if (stack_count < sizeof(stack) / sizeof(stack[0])) {
        (void)memset(&stack[stack_count], 0, sizeof(stack[stack_count]));
        stack[stack_count].kind = HOLONS_BLOCK_SERVICE;
        stack[stack_count].index = index->service_count;
        (void)copy_string(stack[stack_count].name, sizeof(stack[stack_count].name), name, NULL, 0);
        stack_count += 1;
      }
      index->service_count += 1;
      holons_clear_pending_comments(pending_comments, &pending_count);
      holons_trim_closed_blocks:
      {
        size_t closes = 0;
        size_t i;
        for (i = 0; value[i] != '\0'; ++i) {
          if (value[i] == '}') {
            closes += 1;
          }
        }
        while (closes > 0 && stack_count > 0) {
          stack_count -= 1;
          closes -= 1;
        }
      }
      continue;
    }
    if (holons_parse_name_after_keyword(value, "message", name, sizeof(name))) {
      holons_message_def_t *message;
      char qualified[HOLONS_MAX_FIELD_LEN];
      holons_scope_from_stack(stack, stack_count, scope, sizeof(scope));
      holons_qualify_scope(scope, name, qualified, sizeof(qualified));
      if (holons_ensure_capacity((void **)&index->messages,
                                 &index->message_capacity,
                                 index->message_count,
                                 sizeof(*index->messages),
                                 err,
                                 err_len) != 0) {
        (void)fclose(f);
        return -1;
      }
      message = &index->messages[index->message_count];
      (void)memset(message, 0, sizeof(*message));
      holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
      (void)copy_string(message->name, sizeof(message->name), name, NULL, 0);
      (void)copy_string(message->package_name, sizeof(message->package_name), package_name, NULL, 0);
      (void)copy_string(message->scope, sizeof(message->scope), scope, NULL, 0);
      if (package_name[0] != '\0') {
        (void)snprintf(message->full_name, sizeof(message->full_name), "%s.%s", package_name, qualified);
      } else {
        (void)copy_string(message->full_name, sizeof(message->full_name), qualified, NULL, 0);
      }
      message->comment = comment;
      holons_add_simple_type(index, message->name, message->full_name, err, err_len);
      holons_add_simple_type(index, qualified, message->full_name, err, err_len);
      if (stack_count < sizeof(stack) / sizeof(stack[0])) {
        (void)memset(&stack[stack_count], 0, sizeof(stack[stack_count]));
        stack[stack_count].kind = HOLONS_BLOCK_MESSAGE;
        stack[stack_count].index = index->message_count;
        (void)copy_string(stack[stack_count].name, sizeof(stack[stack_count].name), name, NULL, 0);
        stack_count += 1;
      }
      index->message_count += 1;
      holons_clear_pending_comments(pending_comments, &pending_count);
      goto holons_trim_closed_blocks;
    }
    if (holons_parse_name_after_keyword(value, "enum", name, sizeof(name))) {
      holons_enum_def_t *enum_def;
      char qualified[HOLONS_MAX_FIELD_LEN];
      holons_scope_from_stack(stack, stack_count, scope, sizeof(scope));
      holons_qualify_scope(scope, name, qualified, sizeof(qualified));
      if (holons_ensure_capacity((void **)&index->enums,
                                 &index->enum_capacity,
                                 index->enum_count,
                                 sizeof(*index->enums),
                                 err,
                                 err_len) != 0) {
        (void)fclose(f);
        return -1;
      }
      enum_def = &index->enums[index->enum_count];
      (void)memset(enum_def, 0, sizeof(*enum_def));
      holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
      (void)copy_string(enum_def->name, sizeof(enum_def->name), name, NULL, 0);
      (void)copy_string(enum_def->package_name, sizeof(enum_def->package_name), package_name, NULL, 0);
      (void)copy_string(enum_def->scope, sizeof(enum_def->scope), scope, NULL, 0);
      if (package_name[0] != '\0') {
        (void)snprintf(enum_def->full_name, sizeof(enum_def->full_name), "%s.%s", package_name, qualified);
      } else {
        (void)copy_string(enum_def->full_name, sizeof(enum_def->full_name), qualified, NULL, 0);
      }
      enum_def->comment = comment;
      holons_add_simple_type(index, enum_def->name, enum_def->full_name, err, err_len);
      holons_add_simple_type(index, qualified, enum_def->full_name, err, err_len);
      if (stack_count < sizeof(stack) / sizeof(stack[0])) {
        (void)memset(&stack[stack_count], 0, sizeof(stack[stack_count]));
        stack[stack_count].kind = HOLONS_BLOCK_ENUM;
        stack[stack_count].index = index->enum_count;
        (void)copy_string(stack[stack_count].name, sizeof(stack[stack_count].name), name, NULL, 0);
        stack_count += 1;
      }
      index->enum_count += 1;
      holons_clear_pending_comments(pending_comments, &pending_count);
      goto holons_trim_closed_blocks;
    }
    {
      char input_type[HOLONS_MAX_FIELD_LEN];
      char output_type[HOLONS_MAX_FIELD_LEN];
      int client_streaming;
      int server_streaming;
      if (holons_parse_rpc_line(value,
                                name,
                                sizeof(name),
                                input_type,
                                sizeof(input_type),
                                &client_streaming,
                                output_type,
                                sizeof(output_type),
                                &server_streaming)) {
        ssize_t i;
        holons_service_def_t *service = NULL;
        for (i = (ssize_t)stack_count - 1; i >= 0; --i) {
          if (stack[i].kind == HOLONS_BLOCK_SERVICE) {
            service = &index->services[stack[i].index];
            break;
          }
        }
        if (service != NULL) {
          holons_method_def_t *method;
          if (holons_ensure_capacity((void **)&service->methods,
                                     &service->method_capacity,
                                     service->method_count,
                                     sizeof(*service->methods),
                                     err,
                                     err_len) != 0) {
            (void)fclose(f);
            return -1;
          }
          method = &service->methods[service->method_count];
          (void)memset(method, 0, sizeof(*method));
          holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
          (void)copy_string(method->name, sizeof(method->name), name, NULL, 0);
          holons_resolve_type(input_type, package_name, "", index,
                              method->input_type, sizeof(method->input_type));
          holons_resolve_type(output_type, package_name, "", index,
                              method->output_type, sizeof(method->output_type));
          method->client_streaming = client_streaming;
          method->server_streaming = server_streaming;
          method->comment = comment;
          service->method_count += 1;
        }
        holons_clear_pending_comments(pending_comments, &pending_count);
        goto holons_trim_closed_blocks;
      }
    }
    {
      char key_type[HOLONS_MAX_FIELD_LEN];
      char value_type[HOLONS_MAX_FIELD_LEN];
      int number;
      if (holons_parse_map_field_line(value,
                                      key_type,
                                      sizeof(key_type),
                                      value_type,
                                      sizeof(value_type),
                                      name,
                                      sizeof(name),
                                      &number)) {
        ssize_t i;
        holons_message_def_t *message = NULL;
        for (i = (ssize_t)stack_count - 1; i >= 0; --i) {
          if (stack[i].kind == HOLONS_BLOCK_MESSAGE) {
            message = &index->messages[stack[i].index];
            break;
          }
        }
        if (message != NULL) {
          holons_field_def_t *field;
          if (holons_ensure_capacity((void **)&message->fields,
                                     &message->field_capacity,
                                     message->field_count,
                                     sizeof(*message->fields),
                                     err,
                                     err_len) != 0) {
            (void)fclose(f);
            return -1;
          }
          field = &message->fields[message->field_count];
          (void)memset(field, 0, sizeof(*field));
          holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
          (void)copy_string(field->name, sizeof(field->name), name, NULL, 0);
          (void)snprintf(field->type_name, sizeof(field->type_name), "map<%s,%s>", key_type, value_type);
          (void)copy_string(field->raw_type, sizeof(field->raw_type), "map", NULL, 0);
          field->number = number;
          field->comment = comment;
          field->cardinality = HOLONS_CARDINALITY_MAP;
          (void)copy_string(field->package_name, sizeof(field->package_name), package_name, NULL, 0);
          (void)copy_string(field->scope, sizeof(field->scope), message->scope, NULL, 0);
          if (field->scope[0] != '\0') {
            strncat(field->scope, ".", sizeof(field->scope) - strlen(field->scope) - 1);
          }
          strncat(field->scope, message->name, sizeof(field->scope) - strlen(field->scope) - 1);
          (void)copy_string(field->map_key_type, sizeof(field->map_key_type), key_type, NULL, 0);
          (void)copy_string(field->map_value_type, sizeof(field->map_value_type), value_type, NULL, 0);
          message->field_count += 1;
        }
        holons_clear_pending_comments(pending_comments, &pending_count);
        goto holons_trim_closed_blocks;
      }
    }
    {
      char type_name[HOLONS_MAX_FIELD_LEN];
      holons_cardinality_t cardinality;
      int number;
      if (holons_parse_field_line(value,
                                  type_name,
                                  sizeof(type_name),
                                  name,
                                  sizeof(name),
                                  &number,
                                  &cardinality)) {
        ssize_t i;
        holons_message_def_t *message = NULL;
        for (i = (ssize_t)stack_count - 1; i >= 0; --i) {
          if (stack[i].kind == HOLONS_BLOCK_MESSAGE) {
            message = &index->messages[stack[i].index];
            break;
          }
        }
        if (message != NULL) {
          holons_field_def_t *field;
          if (holons_ensure_capacity((void **)&message->fields,
                                     &message->field_capacity,
                                     message->field_count,
                                     sizeof(*message->fields),
                                     err,
                                     err_len) != 0) {
            (void)fclose(f);
            return -1;
          }
          field = &message->fields[message->field_count];
          (void)memset(field, 0, sizeof(*field));
          holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
          (void)copy_string(field->name, sizeof(field->name), name, NULL, 0);
          holons_resolve_type(type_name,
                              package_name,
                              message->scope[0] != '\0' ? message->scope : message->name,
                              index,
                              field->type_name,
                              sizeof(field->type_name));
          (void)copy_string(field->raw_type, sizeof(field->raw_type), type_name, NULL, 0);
          field->number = number;
          field->comment = comment;
          field->cardinality = cardinality;
          (void)copy_string(field->package_name, sizeof(field->package_name), package_name, NULL, 0);
          (void)copy_string(field->scope, sizeof(field->scope), message->scope, NULL, 0);
          if (field->scope[0] != '\0') {
            strncat(field->scope, ".", sizeof(field->scope) - strlen(field->scope) - 1);
          }
          strncat(field->scope, message->name, sizeof(field->scope) - strlen(field->scope) - 1);
          message->field_count += 1;
        }
        holons_clear_pending_comments(pending_comments, &pending_count);
        goto holons_trim_closed_blocks;
      }
    }
    {
      char value_name[HOLONS_MAX_FIELD_LEN];
      int number;
      if (holons_parse_enum_value_line(value, value_name, sizeof(value_name), &number)) {
        ssize_t i;
        holons_enum_def_t *enum_def = NULL;
        for (i = (ssize_t)stack_count - 1; i >= 0; --i) {
          if (stack[i].kind == HOLONS_BLOCK_ENUM) {
            enum_def = &index->enums[stack[i].index];
            break;
          }
        }
        if (enum_def != NULL) {
          holons_enum_value_def_t *enum_value;
          if (holons_ensure_capacity((void **)&enum_def->values,
                                     &enum_def->value_capacity,
                                     enum_def->value_count,
                                     sizeof(*enum_def->values),
                                     err,
                                     err_len) != 0) {
            (void)fclose(f);
            return -1;
          }
          enum_value = &enum_def->values[enum_def->value_count];
          (void)memset(enum_value, 0, sizeof(*enum_value));
          holons_comment_meta_from_lines(pending_comments, pending_count, &comment);
          (void)copy_string(enum_value->name, sizeof(enum_value->name), value_name, NULL, 0);
          enum_value->number = number;
          enum_value->comment = comment;
          enum_def->value_count += 1;
        }
        holons_clear_pending_comments(pending_comments, &pending_count);
        goto holons_trim_closed_blocks;
      }
    }

    if (strcmp(value, "}") != 0) {
      holons_clear_pending_comments(pending_comments, &pending_count);
    }
    goto holons_trim_closed_blocks;
  }

  (void)fclose(f);
  return 0;
}

static int holons_parse_proto_directory(const char *proto_dir,
                                        holons_proto_index_t *index,
                                        char *err,
                                        size_t err_len) {
  DIR *dir;
  struct dirent *entry;

  dir = opendir(proto_dir);
  if (dir == NULL) {
    if (errno == ENOENT) {
      return 0;
    }
    set_err(err, err_len, "cannot open proto directory %s: %s", proto_dir, strerror(errno));
    return -1;
  }

  while ((entry = readdir(dir)) != NULL) {
    char child[PATH_MAX];
    struct stat st;
    size_t len = strlen(entry->d_name);
    if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0) {
      continue;
    }
    if (snprintf(child, sizeof(child), "%s/%s", proto_dir, entry->d_name) >= (int)sizeof(child)) {
      (void)closedir(dir);
      set_err(err, err_len, "proto path is too long");
      return -1;
    }
    if (stat(child, &st) != 0) {
      continue;
    }
    if (S_ISDIR(st.st_mode)) {
      if (holons_parse_proto_directory(child, index, err, err_len) != 0) {
        (void)closedir(dir);
        return -1;
      }
      continue;
    }
    if (len > 6 && strcmp(entry->d_name + len - 6, ".proto") == 0) {
      if (holons_parse_proto_file(child, index, err, err_len) != 0) {
        (void)closedir(dir);
        return -1;
      }
    }
  }

  (void)closedir(dir);
  return 0;
}

static int holons_build_field_doc(const holons_field_def_t *field,
                                  const holons_proto_index_t *index,
                                  const ssize_t *seen_messages,
                                  size_t seen_count,
                                  holons_field_doc_t *out,
                                  char *err,
                                  size_t err_len);

static int holons_seen_contains(const ssize_t *seen_messages,
                                size_t seen_count,
                                ssize_t message_index) {
  size_t i;
  for (i = 0; i < seen_count; ++i) {
    if (seen_messages[i] == message_index) {
      return 1;
    }
  }
  return 0;
}

static int holons_build_field_docs_from_message(ssize_t message_index,
                                                const holons_proto_index_t *index,
                                                const ssize_t *seen_messages,
                                                size_t seen_count,
                                                holons_field_doc_t **out_fields,
                                                size_t *out_count,
                                                char *err,
                                                size_t err_len) {
  const holons_message_def_t *message;
  size_t i;
  ssize_t next_seen[32];

  if (message_index < 0 || (size_t)message_index >= index->message_count) {
    *out_fields = NULL;
    *out_count = 0;
    return 0;
  }
  if (holons_seen_contains(seen_messages, seen_count, message_index)) {
    *out_fields = NULL;
    *out_count = 0;
    return 0;
  }

  message = &index->messages[message_index];
  if (message->field_count == 0) {
    *out_fields = NULL;
    *out_count = 0;
    return 0;
  }
  if (seen_count >= sizeof(next_seen) / sizeof(next_seen[0])) {
    *out_fields = NULL;
    *out_count = 0;
    return 0;
  }
  memcpy(next_seen, seen_messages, seen_count * sizeof(next_seen[0]));
  next_seen[seen_count++] = message_index;

  *out_fields = calloc(message->field_count, sizeof(**out_fields));
  if (*out_fields == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }
  *out_count = message->field_count;
  for (i = 0; i < message->field_count; ++i) {
    if (holons_build_field_doc(&message->fields[i],
                               index,
                               next_seen,
                               seen_count,
                               &(*out_fields)[i],
                               err,
                               err_len) != 0) {
      holons_free_field_docs(*out_fields, i);
      *out_fields = NULL;
      *out_count = 0;
      return -1;
    }
  }
  return 0;
}

static int holons_build_field_doc(const holons_field_def_t *field,
                                  const holons_proto_index_t *index,
                                  const ssize_t *seen_messages,
                                  size_t seen_count,
                                  holons_field_doc_t *out,
                                  char *err,
                                  size_t err_len) {
  char resolved_type[HOLONS_MAX_FIELD_LEN];
  ssize_t message_index;
  ssize_t enum_index;
  size_t i;

  (void)memset(out, 0, sizeof(*out));
  (void)copy_string(out->name, sizeof(out->name), field->name, NULL, 0);
  (void)copy_string(out->type, sizeof(out->type), field->type_name, NULL, 0);
  out->number = field->number;
  (void)copy_string(out->description, sizeof(out->description), field->comment.description, NULL, 0);
  (void)copy_string(out->map_key_type, sizeof(out->map_key_type), field->map_key_type, NULL, 0);
  (void)copy_string(out->map_value_type, sizeof(out->map_value_type), field->map_value_type, NULL, 0);
  (void)copy_string(out->example, sizeof(out->example), field->comment.example, NULL, 0);
  out->required = field->comment.required;
  if (field->cardinality == HOLONS_CARDINALITY_MAP) {
    out->label = HOLONS_FIELD_LABEL_MAP;
    holons_resolve_type(field->map_value_type, field->package_name, field->scope,
                        index, resolved_type, sizeof(resolved_type));
  } else if (field->cardinality == HOLONS_CARDINALITY_REPEATED) {
    out->label = HOLONS_FIELD_LABEL_REPEATED;
    holons_resolve_type(field->raw_type, field->package_name, field->scope,
                        index, resolved_type, sizeof(resolved_type));
  } else if (field->comment.required) {
    out->label = HOLONS_FIELD_LABEL_REQUIRED;
    holons_resolve_type(field->raw_type, field->package_name, field->scope,
                        index, resolved_type, sizeof(resolved_type));
  } else {
    out->label = HOLONS_FIELD_LABEL_OPTIONAL;
    holons_resolve_type(field->raw_type, field->package_name, field->scope,
                        index, resolved_type, sizeof(resolved_type));
  }

  message_index = holons_find_message_index(index, resolved_type);
  if (holons_build_field_docs_from_message(message_index,
                                           index,
                                           seen_messages,
                                           seen_count,
                                           &out->nested_fields,
                                           &out->nested_field_count,
                                           err,
                                           err_len) != 0) {
    return -1;
  }

  enum_index = holons_find_enum_index(index, resolved_type);
  if (enum_index >= 0) {
    const holons_enum_def_t *enum_def = &index->enums[enum_index];
    out->enum_values = calloc(enum_def->value_count, sizeof(*out->enum_values));
    if (enum_def->value_count > 0 && out->enum_values == NULL) {
      holons_free_field_docs(out->nested_fields, out->nested_field_count);
      out->nested_fields = NULL;
      out->nested_field_count = 0;
      set_err(err, err_len, "out of memory");
      return -1;
    }
    out->enum_value_count = enum_def->value_count;
    for (i = 0; i < enum_def->value_count; ++i) {
      (void)copy_string(out->enum_values[i].name,
                        sizeof(out->enum_values[i].name),
                        enum_def->values[i].name,
                        NULL,
                        0);
      out->enum_values[i].number = enum_def->values[i].number;
      (void)copy_string(out->enum_values[i].description,
                        sizeof(out->enum_values[i].description),
                        enum_def->values[i].comment.description,
                        NULL,
                        0);
    }
  }
  return 0;
}

static int holons_build_method_doc(const holons_method_def_t *method,
                                   const holons_proto_index_t *index,
                                   holons_method_doc_t *out,
                                   char *err,
                                   size_t err_len) {
  ssize_t input_index;
  ssize_t output_index;

  (void)memset(out, 0, sizeof(*out));
  (void)copy_string(out->name, sizeof(out->name), method->name, NULL, 0);
  (void)copy_string(out->description, sizeof(out->description), method->comment.description, NULL, 0);
  (void)copy_string(out->input_type, sizeof(out->input_type), method->input_type, NULL, 0);
  (void)copy_string(out->output_type, sizeof(out->output_type), method->output_type, NULL, 0);
  (void)copy_string(out->example_input, sizeof(out->example_input), method->comment.example, NULL, 0);
  out->client_streaming = method->client_streaming;
  out->server_streaming = method->server_streaming;

  input_index = holons_find_message_index(index, method->input_type);
  if (holons_build_field_docs_from_message(input_index,
                                           index,
                                           NULL,
                                           0,
                                           &out->input_fields,
                                           &out->input_field_count,
                                           err,
                                           err_len) != 0) {
    return -1;
  }

  output_index = holons_find_message_index(index, method->output_type);
  if (holons_build_field_docs_from_message(output_index,
                                           index,
                                           NULL,
                                           0,
                                           &out->output_fields,
                                           &out->output_field_count,
                                           err,
                                           err_len) != 0) {
    holons_free_field_docs(out->input_fields, out->input_field_count);
    out->input_fields = NULL;
    out->input_field_count = 0;
    return -1;
  }
  return 0;
}

static int holons_build_service_doc(const holons_service_def_t *service,
                                    const holons_proto_index_t *index,
                                    holons_service_doc_t *out,
                                    char *err,
                                    size_t err_len) {
  size_t i;
  (void)memset(out, 0, sizeof(*out));
  (void)copy_string(out->name, sizeof(out->name), service->full_name, NULL, 0);
  (void)copy_string(out->description, sizeof(out->description), service->comment.description, NULL, 0);
  if (service->method_count == 0) {
    return 0;
  }
  out->methods = calloc(service->method_count, sizeof(*out->methods));
  if (out->methods == NULL) {
    set_err(err, err_len, "out of memory");
    return -1;
  }
  out->method_count = service->method_count;
  for (i = 0; i < service->method_count; ++i) {
    if (holons_build_method_doc(&service->methods[i], index, &out->methods[i], err, err_len) != 0) {
      size_t j;
      for (j = 0; j < i; ++j) {
        holons_free_field_docs(out->methods[j].input_fields, out->methods[j].input_field_count);
        holons_free_field_docs(out->methods[j].output_fields, out->methods[j].output_field_count);
      }
      free(out->methods);
      out->methods = NULL;
      out->method_count = 0;
      return -1;
    }
  }
  return 0;
}

int holons_build_describe_response(const char *proto_dir,
                                   holons_describe_response_t *out,
                                   char *err,
                                   size_t err_len) {
  holons_proto_index_t index;
  size_t visible_services = 0;
  size_t i;
  size_t out_index = 0;

  if (out == NULL) {
    set_err(err, err_len, "response output is required");
    return -1;
  }

  holons_init_describe_response(out);
  (void)memset(&index, 0, sizeof(index));
  if (holons_resolve_manifest(proto_dir, &out->manifest, NULL, 0, err, err_len) != 0) {
    return -1;
  }

  if (proto_dir == NULL || proto_dir[0] == '\0') {
    return 0;
  }
  if (holons_parse_proto_directory(proto_dir, &index, err, err_len) != 0) {
    holons_free_proto_index(&index);
    return -1;
  }

  for (i = 0; i < index.service_count; ++i) {
    if (strcmp(index.services[i].full_name, HOLONS_DESCRIBE_SERVICE_NAME) != 0) {
      visible_services += 1;
    }
  }
  if (visible_services == 0) {
    holons_free_proto_index(&index);
    return 0;
  }

  out->services = calloc(visible_services, sizeof(*out->services));
  if (out->services == NULL) {
    holons_free_proto_index(&index);
    set_err(err, err_len, "out of memory");
    return -1;
  }
  out->service_count = visible_services;
  for (i = 0; i < index.service_count; ++i) {
    if (strcmp(index.services[i].full_name, HOLONS_DESCRIBE_SERVICE_NAME) == 0) {
      continue;
    }
    if (holons_build_service_doc(&index.services[i],
                                 &index,
                                 &out->services[out_index],
                                 err,
                                 err_len) != 0) {
      holons_free_describe_response(out);
      holons_free_proto_index(&index);
      return -1;
    }
    out_index += 1;
  }

  holons_free_proto_index(&index);
  return 0;
}

void holons_use_static_describe_response(const holons_describe_response_t *response) {
  g_static_describe_response = response;
}

int holons_make_describe_registration(holons_describe_registration_t *out,
                                      char *err,
                                      size_t err_len) {
  if (out == NULL) {
    set_err(err, err_len, "registration output is required");
    return -1;
  }
  if (g_static_describe_response == NULL) {
    set_err(err, err_len, "%s", holons_no_incode_description_error);
    return -1;
  }
  (void)memset(out, 0, sizeof(*out));
  (void)copy_string(out->service_name, sizeof(out->service_name),
                    HOLONS_DESCRIBE_SERVICE_NAME, err, err_len);
  (void)copy_string(out->method_name, sizeof(out->method_name),
                    "Describe", err, err_len);
  out->response = g_static_describe_response;
  return 0;
}

int holons_invoke_describe(const holons_describe_registration_t *registration,
                           const holons_describe_request_t *request,
                           holons_describe_response_t *out,
                           char *err,
                           size_t err_len) {
  (void)request;
  if (registration == NULL) {
    set_err(err, err_len, "registration is required");
    return -1;
  }
  return holons_clone_describe_response(registration->response, out, err, err_len);
}
