#include "internal/server.h"

#include "api/public.h"

#include "holons/holons.h"

#include <errno.h>
#include <limits.h>
#include <signal.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#ifdef __APPLE__
#include <mach-o/dyld.h>
#endif

#ifndef PATH_MAX
#define PATH_MAX 4096
#endif

#ifndef GABRIEL_GREETING_C_BACKEND_BINARY
#error "GABRIEL_GREETING_C_BACKEND_BINARY must be defined"
#endif

#ifndef GABRIEL_GREETING_C_BRIDGE_BINARY
#error "GABRIEL_GREETING_C_BRIDGE_BINARY must be defined"
#endif

#ifndef GABRIEL_GREETING_C_PROTO_DIR
#error "GABRIEL_GREETING_C_PROTO_DIR must be defined"
#endif

#ifndef GABRIEL_GREETING_C_MANIFEST_PATH
#error "GABRIEL_GREETING_C_MANIFEST_PATH must be defined"
#endif

#ifndef GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH
#error "GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH must be defined"
#endif

const holons_describe_response_t *holons_generated_describe_response(void);

static int file_exists(const char *path) {
  return path != NULL && access(path, F_OK) == 0;
}

static int dir_exists(const char *path) {
  return path != NULL && access(path, F_OK) == 0;
}

static int copy_path(char *out, size_t out_len, const char *path) {
  if (out == NULL || out_len == 0 || path == NULL) {
    return -1;
  }
  if (snprintf(out, out_len, "%s", path) >= (int)out_len) {
    return -1;
  }
  return 0;
}

static int join_path(char *out, size_t out_len, const char *base, const char *name) {
  if (out == NULL || out_len == 0 || base == NULL || name == NULL) {
    return -1;
  }
  if (snprintf(out, out_len, "%s/%s", base, name) >= (int)out_len) {
    return -1;
  }
  return 0;
}

static int executable_dir(char *out, size_t out_len) {
#ifdef __APPLE__
  uint32_t size = (uint32_t)out_len;
  char resolved[PATH_MAX];
  char *dir;
  if (_NSGetExecutablePath(out, &size) != 0) {
    return -1;
  }
  if (realpath(out, resolved) != NULL) {
    if (copy_path(out, out_len, resolved) != 0) {
      return -1;
    }
  }
  dir = strrchr(out, '/');
  if (dir == NULL) {
    return -1;
  }
  *dir = '\0';
  return 0;
#else
  (void)out;
  (void)out_len;
  return -1;
#endif
}

static const char *resolve_backend_binary(char *buffer, size_t buffer_len) {
  char dir[PATH_MAX];
  if (executable_dir(dir, sizeof(dir)) == 0 &&
      join_path(buffer, buffer_len, dir, "gabriel-greeting-c-backend") == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, GABRIEL_GREETING_C_BACKEND_BINARY) == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  return GABRIEL_GREETING_C_BACKEND_BINARY;
}

static const char *resolve_bridge_binary(char *buffer, size_t buffer_len) {
  char dir[PATH_MAX];
  if (executable_dir(dir, sizeof(dir)) == 0 &&
      join_path(buffer, buffer_len, dir, "grpc-bridge") == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, GABRIEL_GREETING_C_BRIDGE_BINARY) == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  return GABRIEL_GREETING_C_BRIDGE_BINARY;
}

static const char *resolve_proto_dir(char *buffer, size_t buffer_len) {
  if (copy_path(buffer, buffer_len, "protos") == 0 && dir_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, "_protos") == 0 && dir_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, GABRIEL_GREETING_C_PROTO_DIR) == 0 && dir_exists(buffer)) {
    return buffer;
  }
  return GABRIEL_GREETING_C_PROTO_DIR;
}

static const char *resolve_describe_static_path(char *buffer, size_t buffer_len) {
  char dir[PATH_MAX];
  if (executable_dir(dir, sizeof(dir)) == 0 &&
      join_path(buffer, buffer_len, dir, "describe_generated.json") == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, "gen/describe_generated.json") == 0 && file_exists(buffer)) {
    return buffer;
  }
  if (copy_path(buffer, buffer_len, GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH) == 0 &&
      file_exists(buffer)) {
    return buffer;
  }
  return GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH;
}

static void register_static_describe_response(void) {
  holons_use_static_describe_response(holons_generated_describe_response());
}

static void handle_signal(int signo) {
  (void)signo;
  holons_request_stop();
}

static int install_backend_signal_handlers(struct sigaction *old_int,
                                           struct sigaction *old_term) {
  struct sigaction action;
  int saved_errno;

  memset(&action, 0, sizeof(action));
  action.sa_handler = handle_signal;
  (void)sigemptyset(&action.sa_mask);
  /* Keep accept() interruptible so SIGTERM breaks the serve loop immediately. */
  action.sa_flags = 0;

  if (sigaction(SIGINT, &action, old_int) != 0) {
    return -1;
  }
  if (sigaction(SIGTERM, &action, old_term) != 0) {
    saved_errno = errno;
    (void)sigaction(SIGINT, old_int, NULL);
    errno = saved_errno;
    return -1;
  }
  return 0;
}

static void restore_backend_signal_handlers(const struct sigaction *old_int,
                                            const struct sigaction *old_term) {
  (void)sigaction(SIGINT, old_int, NULL);
  (void)sigaction(SIGTERM, old_term, NULL);
}

static void write_json_string(FILE *output, const char *data, size_t size) {
  size_t i;
  fputc('"', output);
  for (i = 0; i < size; ++i) {
    unsigned char ch = (unsigned char)data[i];
    switch (ch) {
    case '\\':
      fputs("\\\\", output);
      break;
    case '"':
      fputs("\\\"", output);
      break;
    case '\b':
      fputs("\\b", output);
      break;
    case '\f':
      fputs("\\f", output);
      break;
    case '\n':
      fputs("\\n", output);
      break;
    case '\r':
      fputs("\\r", output);
      break;
    case '\t':
      fputs("\\t", output);
      break;
    default:
      fputc((int)ch, output);
      break;
    }
  }
  fputc('"', output);
}

static void write_json_string_view(FILE *output, upb_StringView value) {
  write_json_string(output, value.data, value.size);
}

static void write_http_response(const holons_conn_t *conn, int status_code,
                                const char *status_text, const char *body) {
  char header[256];
  const size_t body_len = strlen(body);
  snprintf(header, sizeof(header),
           "HTTP/1.1 %d %s\r\nContent-Type: application/json\r\n"
           "Content-Length: %zu\r\n\r\n",
           status_code, status_text, body_len);
  holons_conn_write(conn, header, strlen(header));
  holons_conn_write(conn, body, body_len);
}

static void write_not_found(const holons_conn_t *conn) {
  const char *body = "{\"error\":\"not found\"}";
  write_http_response(conn, 404, "Not Found", body);
}

static void write_method_not_allowed(const holons_conn_t *conn) {
  const char *body = "{\"error\":\"method not allowed\"}";
  write_http_response(conn, 405, "Method Not Allowed", body);
}

static void write_internal_error(const holons_conn_t *conn) {
  const char *body = "{\"error\":\"internal error\"}";
  write_http_response(conn, 500, "Internal Server Error", body);
}

static const char *find_body(char *request) {
  char *body = strstr(request, "\r\n\r\n");
  if (body == NULL) {
    return "";
  }
  return body + 4;
}

static void extract_json_field(const char *json, const char *field, char *out,
                               size_t out_len) {
  char needle[64];
  const char *cursor;
  size_t len = 0;

  if (out_len == 0) {
    return;
  }
  out[0] = '\0';

  snprintf(needle, sizeof(needle), "\"%s\":", field);
  cursor = strstr(json, needle);
  if (cursor == NULL) {
    return;
  }

  cursor += strlen(needle);
  while (*cursor == ' ' || *cursor == '\n' || *cursor == '\t' || *cursor == '\r') {
    ++cursor;
  }
  if (*cursor != '"') {
    return;
  }
  ++cursor;

  while (cursor[len] != '\0' && cursor[len] != '"' && len + 1 < out_len) {
    out[len] = cursor[len];
    ++len;
  }
  out[len] = '\0';
}

static int write_list_languages_response(const holons_conn_t *conn) {
  upb_Arena *arena = upb_Arena_New();
  greeting_v1_ListLanguagesResponse *response =
      gabriel_greeting_c_list_languages(arena);
  size_t count = 0;
  size_t i;
  const greeting_v1_Language *const *languages;
  FILE *stream;
  char *body = NULL;
  size_t body_len = 0;

  if (arena == NULL || response == NULL) {
    if (arena != NULL) {
      upb_Arena_Free(arena);
    }
    return -1;
  }

  languages = greeting_v1_ListLanguagesResponse_languages(response, &count);
  stream = open_memstream(&body, &body_len);
  if (stream == NULL) {
    upb_Arena_Free(arena);
    return -1;
  }

  fputs("{\"languages\":[", stream);
  for (i = 0; i < count; ++i) {
    if (i != 0) {
      fputc(',', stream);
    }
    fputs("{\"code\":", stream);
    write_json_string_view(stream, greeting_v1_Language_code(languages[i]));
    fputs(",\"name\":", stream);
    write_json_string_view(stream, greeting_v1_Language_name(languages[i]));
    fputs(",\"native\":", stream);
    write_json_string_view(stream, greeting_v1_Language_native(languages[i]));
    fputc('}', stream);
  }
  fputs("]}", stream);
  fclose(stream);

  write_http_response(conn, 200, "OK", body);
  free(body);
  upb_Arena_Free(arena);
  return 0;
}

static int write_say_hello_response(const holons_conn_t *conn, const char *body_json) {
  upb_Arena *arena = upb_Arena_New();
  greeting_v1_SayHelloRequest *request = greeting_v1_SayHelloRequest_new(arena);
  greeting_v1_SayHelloResponse *response;
  FILE *stream;
  char *body = NULL;
  size_t body_len = 0;
  char name[256];
  char lang_code[32];

  if (arena == NULL || request == NULL) {
    if (arena != NULL) {
      upb_Arena_Free(arena);
    }
    return -1;
  }

  extract_json_field(body_json, "name", name, sizeof(name));
  extract_json_field(body_json, "lang_code", lang_code, sizeof(lang_code));
  if (lang_code[0] == '\0') {
    strcpy(lang_code, "en");
  }

  greeting_v1_SayHelloRequest_set_name(request, upb_StringView_FromString(name));
  greeting_v1_SayHelloRequest_set_lang_code(request,
                                            upb_StringView_FromString(lang_code));

  response = gabriel_greeting_c_say_hello(request, arena);
  if (response == NULL) {
    upb_Arena_Free(arena);
    return -1;
  }

  stream = open_memstream(&body, &body_len);
  if (stream == NULL) {
    upb_Arena_Free(arena);
    return -1;
  }

  fputs("{\"greeting\":", stream);
  write_json_string_view(stream, greeting_v1_SayHelloResponse_greeting(response));
  fputs(",\"language\":", stream);
  write_json_string_view(stream, greeting_v1_SayHelloResponse_language(response));
  fputs(",\"lang_code\":", stream);
  write_json_string_view(stream, greeting_v1_SayHelloResponse_lang_code(response));
  fputs("}", stream);
  fclose(stream);

  write_http_response(conn, 200, "OK", body);
  free(body);
  upb_Arena_Free(arena);
  return 0;
}

static int handle_connection(const holons_conn_t *conn, void *ctx) {
  char request[8192];
  char method[16];
  char path[256];
  char protocol[16];
  ssize_t read_size;
  (void)ctx;

  read_size = holons_conn_read(conn, request, sizeof(request) - 1);
  if (read_size <= 0) {
    return 0;
  }
  request[read_size] = '\0';

  if (sscanf(request, "%15s %255s %15s", method, path, protocol) != 3) {
    write_not_found(conn);
    return 0;
  }

  if (strcmp(method, "POST") != 0) {
    write_method_not_allowed(conn);
    return 0;
  }

  if (strcmp(path, "/greeting.v1.GreetingService/ListLanguages") == 0) {
    if (write_list_languages_response(conn) != 0) {
      write_internal_error(conn);
    }
    return 0;
  }

  if (strcmp(path, "/greeting.v1.GreetingService/SayHello") == 0) {
    if (write_say_hello_response(conn, find_body(request)) != 0) {
      write_internal_error(conn);
    }
    return 0;
  }

  write_not_found(conn);
  return 0;
}

int gabriel_greeting_c_exec_bridge(const char *listen_uri, FILE *stderr_stream) {
  char bridge_binary[PATH_MAX];
  char backend_binary[PATH_MAX];
  char proto_dir[PATH_MAX];
  char describe_static_path[PATH_MAX];
  char *const argv[] = {
      (char *)resolve_bridge_binary(bridge_binary, sizeof(bridge_binary)),
      (char *)"--backend",
      (char *)resolve_backend_binary(backend_binary, sizeof(backend_binary)),
      (char *)"--proto-dir",
      (char *)resolve_proto_dir(proto_dir, sizeof(proto_dir)),
      (char *)"--describe-static",
      (char *)resolve_describe_static_path(describe_static_path, sizeof(describe_static_path)),
      (char *)"--listen",
      (char *)listen_uri,
      NULL,
  };

  register_static_describe_response();
  execv(argv[0], argv);
  fprintf(stderr_stream, "serve: exec %s failed: %s\n", argv[0], strerror(errno));
  return 1;
}

int gabriel_greeting_c_backend_serve(const char *listen_uri, FILE *stdout_stream,
                                     FILE *stderr_stream) {
  holons_listener_t listener;
  holons_conn_t conn;
  char err[256];
  struct sigaction old_int;
  struct sigaction old_term;
  int handlers_installed = 0;

  register_static_describe_response();
  if (holons_listen(listen_uri, &listener, err, sizeof(err)) != 0) {
    fprintf(stderr_stream, "backend listen error: %s\n", err);
    return 1;
  }

  fprintf(stdout_stream, "HTTP backend listening on %s\n",
          listener.bound_uri[0] != '\0' ? listener.bound_uri : listen_uri);
  fflush(stdout_stream);

  *holons_stop_token() = 0;
  if (install_backend_signal_handlers(&old_int, &old_term) != 0) {
    fprintf(stderr_stream, "backend signal install error: %s\n", strerror(errno));
    holons_close_listener(&listener);
    return 1;
  }
  handlers_installed = 1;

  for (;;) {
    if (*holons_stop_token()) {
      break;
    }
    if (holons_accept(&listener, &conn, err, sizeof(err)) != 0) {
      if (*holons_stop_token()) {
        break;
      }
      fprintf(stderr_stream, "backend accept error: %s\n", err);
      if (handlers_installed) {
        restore_backend_signal_handlers(&old_int, &old_term);
      }
      holons_close_listener(&listener);
      return 1;
    }
    if (handle_connection(&conn, NULL) != 0) {
      holons_conn_close(&conn);
      if (handlers_installed) {
        restore_backend_signal_handlers(&old_int, &old_term);
      }
      holons_close_listener(&listener);
      return 1;
    }
    holons_conn_close(&conn);
  }

  if (handlers_installed) {
    restore_backend_signal_handlers(&old_int, &old_term);
  }
  holons_close_listener(&listener);
  return 0;
}
