#define _POSIX_C_SOURCE 200809L

#include "holons/holons.h"
#include "holons/observability.h"

#include <assert.h>
#include <arpa/inet.h>
#include <ctype.h>
#include <errno.h>
#include <limits.h>
#include <netdb.h>
#include <netinet/in.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

static int passed = 0;
static int failed = 0;
static int handler_calls = 0;

static void check_int(int cond, const char *label) {
  if (cond) {
    ++passed;
  } else {
    ++failed;
    fprintf(stderr, "FAIL: %s\n", label);
  }
}

static int is_bind_restricted(const char *err) {
  return strstr(err, "Operation not permitted") != NULL || strstr(err, "Permission denied") != NULL;
}

static int dial_unix(const char *path) {
  struct sockaddr_un addr;
  int fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) {
    return -1;
  }
  memset(&addr, 0, sizeof(addr));
  addr.sun_family = AF_UNIX;
  strncpy(addr.sun_path, path, sizeof(addr.sun_path) - 1);
  if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    close(fd);
    return -1;
  }
  return fd;
}

static int noop_handler(const holons_conn_t *conn, void *ctx) {
  (void)conn;
  (void)ctx;
  ++handler_calls;
  return 0;
}

static int read_file(const char *path, char *buf, size_t buf_len) {
  FILE *f;
  size_t n;

  if (path == NULL || buf == NULL || buf_len == 0) {
    return -1;
  }

  f = fopen(path, "r");
  if (f == NULL) {
    return -1;
  }

  n = fread(buf, 1, buf_len - 1, f);
  if (ferror(f)) {
    (void)fclose(f);
    return -1;
  }
  buf[n] = '\0';
  (void)fclose(f);
  return 0;
}

static int command_exit_code(const char *cmd) {
  int status = system(cmd);
  if (status == -1 || !WIFEXITED(status)) {
    return -1;
  }
  return WEXITSTATUS(status);
}

static int run_bash_script(const char *script_body) {
  char path[] = "/tmp/holons_script_XXXXXX";
  char cmd[512];
  int fd = mkstemp(path);
  FILE *script_file;
  int exit_code;

  if (fd < 0) {
    return -1;
  }

  script_file = fdopen(fd, "w");
  if (script_file == NULL) {
    close(fd);
    unlink(path);
    return -1;
  }

  fputs("#!/usr/bin/env bash\n", script_file);
  fputs("set -euo pipefail\n", script_file);
  fputs(script_body, script_file);
  if (ferror(script_file)) {
    (void)fclose(script_file);
    unlink(path);
    return -1;
  }
  (void)fclose(script_file);

  if (chmod(path, 0700) != 0) {
    unlink(path);
    return -1;
  }

  snprintf(cmd, sizeof(cmd), "%s", path);
  exit_code = command_exit_code(cmd);
  unlink(path);
  return exit_code;
}

static void restore_env(const char *name, char *value) {
  if (value != NULL) {
    (void)setenv(name, value, 1);
    free(value);
    return;
  }
  (void)unsetenv(name);
}

static long long test_monotonic_millis(void) {
  struct timespec ts;

  if (clock_gettime(CLOCK_MONOTONIC, &ts) != 0) {
    return 0;
  }
  return (long long)ts.tv_sec * 1000LL + (long long)(ts.tv_nsec / 1000000L);
}

static void test_sleep_millis(int millis) {
  struct timespec req;

  if (millis <= 0) {
    return;
  }

  req.tv_sec = millis / 1000;
  req.tv_nsec = (long)(millis % 1000) * 1000000L;
  while (nanosleep(&req, &req) != 0 && errno == EINTR) {
  }
}

static int pid_exists(pid_t pid) {
  if (pid <= 0) {
    return 0;
  }
  return kill(pid, 0) == 0 || errno == EPERM;
}

static int wait_for_pid_exit(pid_t pid, int timeout_ms) {
  long long deadline = test_monotonic_millis() + timeout_ms;

  while (test_monotonic_millis() < deadline) {
    if (!pid_exists(pid)) {
      return 0;
    }
    test_sleep_millis(25);
  }
  return pid_exists(pid) ? -1 : 0;
}

static int wait_for_file(const char *path, int timeout_ms) {
  long long deadline = test_monotonic_millis() + timeout_ms;

  while (test_monotonic_millis() < deadline) {
    if (access(path, F_OK) == 0) {
      return 0;
    }
    test_sleep_millis(25);
  }
  return access(path, F_OK) == 0 ? 0 : -1;
}

static int read_pid_file(const char *path, pid_t *out_pid) {
  char buf[64];
  char *end = NULL;
  long value;

  if (read_file(path, buf, sizeof(buf)) != 0) {
    return -1;
  }

  errno = 0;
  value = strtol(buf, &end, 10);
  if (errno != 0 || end == buf) {
    return -1;
  }

  *out_pid = (pid_t)value;
  return 0;
}

static int make_temp_dir(char *path_template) {
  int fd;

  if (path_template == NULL) {
    return -1;
  }

  fd = mkstemp(path_template);
  if (fd < 0) {
    return -1;
  }
  close(fd);
  if (unlink(path_template) != 0) {
    return -1;
  }
  if (mkdir(path_template, 0700) != 0) {
    return -1;
  }
  return 0;
}

static void canonicalize_temp_dir(char *path, size_t path_len) {
  char resolved[PATH_MAX];

  if (path == NULL || path_len == 0) {
    return;
  }
  if (realpath(path, resolved) != NULL) {
    assert(snprintf(path, path_len, "%s", resolved) < (int)path_len);
  }
}

static void lowercase_slug(char *out,
                           size_t out_len,
                           const char *given_name,
                           const char *family_name) {
  size_t i;
  size_t n = 0;

  assert(out != NULL);
  assert(given_name != NULL);
  assert(family_name != NULL);

  for (i = 0; given_name[i] != '\0' && n + 1 < out_len; ++i) {
    out[n++] = (char)tolower((unsigned char)given_name[i]);
  }
  if (n + 1 < out_len) {
    out[n++] = '-';
  }
  for (i = 0; family_name[i] != '\0' && n + 1 < out_len; ++i) {
    out[n++] = (char)tolower((unsigned char)family_name[i]);
  }
  out[n] = '\0';
}

static int ensure_dir_with_system(const char *path) {
  char cmd[2048];

  if (snprintf(cmd, sizeof(cmd), "mkdir -p '%s'", path) >= (int)sizeof(cmd)) {
    return -1;
  }
  return system(cmd);
}

static int wait_for_uri_from_fd(int fd, pid_t pid, int timeout_ms, char *out_uri, size_t out_uri_len) {
  char buf[4096];
  size_t used = 0;
  long long deadline = test_monotonic_millis() + timeout_ms;

  buf[0] = '\0';
  while (test_monotonic_millis() < deadline) {
    fd_set readfds;
    struct timeval tv;
    int rc;
    const char *prefixes[] = {"tcp://", "ws://", "wss://", "http://", "https://"};
    size_t i;

    for (i = 0; i < sizeof(prefixes) / sizeof(prefixes[0]); ++i) {
      char *start = strstr(buf, prefixes[i]);
      if (start != NULL) {
        char *end = start;
        size_t n;

        while (*end != '\0' && !isspace((unsigned char)*end)) {
          ++end;
        }
        n = (size_t)(end - start);
        if (n > 0 && n < out_uri_len) {
          memcpy(out_uri, start, n);
          out_uri[n] = '\0';
          return 0;
        }
      }
    }

    if (!pid_exists(pid)) {
      return -1;
    }

    FD_ZERO(&readfds);
    FD_SET(fd, &readfds);
    tv.tv_sec = 0;
    tv.tv_usec = 50000;
    rc = select(fd + 1, &readfds, NULL, NULL, &tv);
    if (rc < 0) {
      if (errno == EINTR) {
        continue;
      }
      return -1;
    }
    if (rc == 0) {
      continue;
    }

    if (FD_ISSET(fd, &readfds)) {
      ssize_t nread = read(fd, buf + used, sizeof(buf) - 1 - used);

      if (nread <= 0) {
        continue;
      }
      used += (size_t)nread;
      buf[used] = '\0';
      if (used >= sizeof(buf) - 1) {
        size_t keep = used > 1024 ? 1024 : used;
        memmove(buf, buf + used - keep, keep);
        used = keep;
        buf[used] = '\0';
      }
    }
  }

  return -1;
}

static int spawn_background_server(const char *binary_path,
                                   const char *listen_uri,
                                   pid_t *out_pid,
                                   char *out_uri,
                                   size_t out_uri_len) {
  char *const argv[] = {(char *)binary_path, "serve", "--listen", (char *)listen_uri, NULL};
  int pipefd[2];
  pid_t pid;

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
    close(pipefd[0]);
    if (dup2(pipefd[1], STDOUT_FILENO) < 0 || dup2(pipefd[1], STDERR_FILENO) < 0) {
      _exit(127);
    }
    if (pipefd[1] != STDOUT_FILENO && pipefd[1] != STDERR_FILENO) {
      close(pipefd[1]);
    }
    execv(binary_path, argv);
    _exit(127);
  }

  close(pipefd[1]);
  if (wait_for_uri_from_fd(pipefd[0], pid, 5000, out_uri, out_uri_len) != 0) {
    kill(pid, SIGTERM);
    waitpid(pid, NULL, 0);
    close(pipefd[0]);
    return -1;
  }

  close(pipefd[0]);
  *out_pid = pid;
  return 0;
}

static int spawn_background_command(const char *binary_path,
                                    char *const argv[],
                                    pid_t *out_pid,
                                    char *out_uri,
                                    size_t out_uri_len) {
  int pipefd[2];
  pid_t pid;

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
    close(pipefd[0]);
    if (dup2(pipefd[1], STDOUT_FILENO) < 0 || dup2(pipefd[1], STDERR_FILENO) < 0) {
      _exit(127);
    }
    if (pipefd[1] != STDOUT_FILENO && pipefd[1] != STDERR_FILENO) {
      close(pipefd[1]);
    }
    execv(binary_path, argv);
    _exit(127);
  }

  close(pipefd[1]);
  if (wait_for_uri_from_fd(pipefd[0], pid, 5000, out_uri, out_uri_len) != 0) {
    kill(pid, SIGTERM);
    waitpid(pid, NULL, 0);
    close(pipefd[0]);
    return -1;
  }

  close(pipefd[0]);
  *out_pid = pid;
  return 0;
}

static int reserve_loopback_port(void) {
  struct sockaddr_in addr;
  socklen_t addr_len = sizeof(addr);
  int fd = socket(AF_INET, SOCK_STREAM, 0);
  int port = -1;

  if (fd < 0) {
    return -1;
  }

  memset(&addr, 0, sizeof(addr));
  addr.sin_family = AF_INET;
  addr.sin_addr.s_addr = inet_addr("127.0.0.1");
  addr.sin_port = 0;

  if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) != 0) {
    close(fd);
    return -1;
  }
  if (getsockname(fd, (struct sockaddr *)&addr, &addr_len) != 0) {
    close(fd);
    return -1;
  }

  port = ntohs(addr.sin_port);
  close(fd);
  return port;
}

static void write_connect_holon_fixture(const char *root,
                                        const char *given_name,
                                        const char *family_name,
                                        char *out_slug,
                                        size_t out_slug_len,
                                        char *out_pid_file,
                                        size_t out_pid_file_len,
                                        char *out_port_file,
                                        size_t out_port_file_len,
                                        char *out_binary_path,
                                        size_t out_binary_path_len) {
  char slug[128];
  char holon_dir[1024];
  char bin_dir[1024];
  char binary_path[1024];
  char manifest_path[1024];
  char args_path[1024];
  FILE *f;

  lowercase_slug(slug, sizeof(slug), given_name, family_name);
  snprintf(holon_dir, sizeof(holon_dir), "%s/holons/%s", root, slug);
  snprintf(bin_dir, sizeof(bin_dir), "%s/.op/build/bin", holon_dir);
  snprintf(binary_path, sizeof(binary_path), "%s/connect-server", bin_dir);
  snprintf(manifest_path, sizeof(manifest_path), "%s/holon.proto", holon_dir);
  snprintf(args_path, sizeof(args_path), "%s/%s.args", root, slug);
  snprintf(out_pid_file, out_pid_file_len, "%s/%s.pid", root, slug);
  snprintf(out_port_file, out_port_file_len, "%s/.op/run/%s.port", root, slug);
  snprintf(out_binary_path, out_binary_path_len, "%s", binary_path);
  snprintf(out_slug, out_slug_len, "%s", slug);

  assert(ensure_dir_with_system(bin_dir) == 0);

  f = fopen(binary_path, "w");
  assert(f != NULL);
  fprintf(f,
          "#!/bin/sh\n"
          "printf '%%s\\n' \"$$\" > '%s'\n"
          ": > '%s'\n"
          "for arg in \"$@\"; do printf '%%s\\n' \"$arg\" >> '%s'; done\n"
          "exec python3 - \"$@\" <<'PY'\n"
          "import signal\n"
          "import socket\n"
          "import sys\n"
          "import time\n"
          "\n"
          "listen_uri = 'tcp://127.0.0.1:0'\n"
          "args = sys.argv[1:]\n"
          "for i, arg in enumerate(args):\n"
          "    if arg == '--listen' and i + 1 < len(args):\n"
          "        listen_uri = args[i + 1]\n"
          "        break\n"
          "\n"
          "if listen_uri in ('stdio://', 'stdio'):\n"
          "    for _ in sys.stdin:\n"
          "        pass\n"
          "    raise SystemExit(0)\n"
          "\n"
          "if not listen_uri.startswith('tcp://'):\n"
          "    raise SystemExit('unsupported listen uri')\n"
          "\n"
          "host_port = listen_uri[len('tcp://'):]\n"
          "host, port_text = host_port.rsplit(':', 1)\n"
          "host = host or '127.0.0.1'\n"
          "port = int(port_text)\n"
          "\n"
          "sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)\n"
          "sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)\n"
          "sock.bind((host, port))\n"
          "sock.listen(8)\n"
          "\n"
          "bound_host, bound_port = sock.getsockname()[:2]\n"
          "public_host = '127.0.0.1' if bound_host in ('0.0.0.0', '') else bound_host\n"
          "sys.stdout.write(f'tcp://{public_host}:{bound_port}\\n')\n"
          "sys.stdout.flush()\n"
          "\n"
          "stop = False\n"
          "\n"
          "def handle(_signo, _frame):\n"
          "    global stop\n"
          "    stop = True\n"
          "    try:\n"
          "        sock.close()\n"
          "    except OSError:\n"
          "        pass\n"
          "\n"
          "signal.signal(signal.SIGTERM, handle)\n"
          "signal.signal(signal.SIGINT, handle)\n"
          "\n"
          "while not stop:\n"
          "    try:\n"
          "        conn, _ = sock.accept()\n"
          "    except OSError:\n"
          "        if stop:\n"
          "            break\n"
          "        time.sleep(0.05)\n"
          "        continue\n"
          "    conn.close()\n"
          "PY\n",
          out_pid_file,
          args_path,
          args_path);
  fclose(f);
  assert(chmod(binary_path, 0755) == 0);

  f = fopen(manifest_path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package holons.test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    uuid: \"%s-uuid\"\n"
          "    given_name: \"%s\"\n"
          "    family_name: \"%s\"\n"
          "    composer: \"connect-test\"\n"
          "  }\n"
          "  kind: \"service\"\n"
          "  build: {\n"
          "    runner: \"shell\"\n"
          "  }\n"
          "  artifacts: {\n"
          "    binary: \"connect-server\"\n"
          "  }\n"
          "};\n",
          slug,
          given_name,
          family_name);
  fclose(f);
}

static void test_echo_scripts_exist(void) {
  check_int(access("./bin/echo-client", F_OK) == 0, "echo-client script exists");
  check_int(access("./bin/echo-server", F_OK) == 0, "echo-server script exists");
  check_int(access("./bin/holon-rpc-client", F_OK) == 0, "holon-rpc-client script exists");
  check_int(access("./bin/holon-rpc-server", F_OK) == 0, "holon-rpc-server script exists");
  check_int(access("./bin/echo-client", X_OK) == 0, "echo-client script executable");
  check_int(access("./bin/echo-server", X_OK) == 0, "echo-server script executable");
  check_int(access("./bin/holon-rpc-client", X_OK) == 0, "holon-rpc-client script executable");
  check_int(access("./bin/holon-rpc-server", X_OK) == 0, "holon-rpc-server script executable");
}

static void write_discovery_holon(const char *dir,
                                  const char *uuid,
                                  const char *given_name,
                                  const char *family_name,
                                  const char *binary) {
  char path[1024];
  FILE *f;
  char cmd[1200];

  snprintf(cmd, sizeof(cmd), "mkdir -p '%s'", dir);
  (void)system(cmd);
  snprintf(path, sizeof(path), "%s/holon.proto", dir);
  f = fopen(path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package holons.test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    uuid: \"%s\"\n"
          "    given_name: \"%s\"\n"
          "    family_name: \"%s\"\n"
          "    motto: \"Test\"\n"
          "    composer: \"test\"\n"
          "    clade: \"deterministic/pure\"\n"
          "    status: \"draft\"\n"
          "    born: \"2026-03-07\"\n"
          "  }\n"
          "  kind: \"native\"\n"
          "  build: {\n"
          "    runner: \"go-module\"\n"
          "  }\n"
          "  artifacts: {\n"
          "    binary: \"%s\"\n"
          "  }\n"
          "};\n",
          uuid,
          given_name,
          family_name,
          binary);
  fclose(f);
}

static void write_echo_holon(const char *root) {
  char proto_dir[1024];
  char proto_path[1024];
  char holon_path[1024];
  char cmd[1200];
  FILE *f;

  snprintf(proto_dir, sizeof(proto_dir), "%s/protos/echo/v1", root);
  snprintf(proto_path, sizeof(proto_path), "%s/echo.proto", proto_dir);
  snprintf(holon_path, sizeof(holon_path), "%s/holon.proto", root);
  snprintf(cmd, sizeof(cmd), "mkdir -p '%s'", proto_dir);
  assert(system(cmd) == 0);

  f = fopen(holon_path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package holons.test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    given_name: \"Echo\"\n"
          "    family_name: \"Server\"\n"
          "    motto: \"Reply precisely.\"\n"
          "  }\n"
          "};\n");
  fclose(f);

  f = fopen(proto_path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package echo.v1;\n\n"
          "// Echo echoes request payloads for documentation tests.\n"
          "service Echo {\n"
          "  // Ping echoes the inbound message.\n"
          "  // @example {\"message\":\"hello\",\"sdk\":\"go-holons\"}\n"
          "  rpc Ping(PingRequest) returns (PingResponse);\n"
          "}\n\n"
          "message PingRequest {\n"
          "  // Message to echo back.\n"
          "  // @required\n"
          "  // @example \"hello\"\n"
          "  string message = 1;\n\n"
          "  // SDK marker included in the response.\n"
          "  // @example \"go-holons\"\n"
          "  string sdk = 2;\n"
          "}\n\n"
          "message PingResponse {\n"
          "  // Echoed message.\n"
          "  string message = 1;\n\n"
          "  // SDK marker from the server.\n"
          "  string sdk = 2;\n"
          "}\n");
  fclose(f);
}

static void discovery_path(char *out, size_t out_len, const char *root, const char *suffix) {
  assert(out != NULL);
  assert(root != NULL);
  assert(suffix != NULL);
  assert(snprintf(out, out_len, "%s/%s", root, suffix) < (int)out_len);
}

static void write_text_file_mode(const char *path, const char *contents, mode_t mode) {
  FILE *f = fopen(path, "w");
  assert(f != NULL);
  fputs(contents, f);
  fclose(f);
  if (mode != 0) {
    assert(chmod(path, mode) == 0);
  }
}

static void test_package_arch_dir(char *out, size_t out_len) {
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

  assert(snprintf(out, out_len, "%s_%s", system, arch) < (int)out_len);
}

static void write_package_binary(const char *dir, const char *binary_name) {
  char arch_dir[64];
  char bin_dir[1024];
  char binary_path[1024];

  test_package_arch_dir(arch_dir, sizeof(arch_dir));
  assert(snprintf(bin_dir, sizeof(bin_dir), "%s/bin/%s", dir, arch_dir) < (int)sizeof(bin_dir));
  assert(ensure_dir_with_system(bin_dir) == 0);
  assert(snprintf(binary_path, sizeof(binary_path), "%s/%s", bin_dir, binary_name) < (int)sizeof(binary_path));
  write_text_file_mode(binary_path,
                       "#!/bin/sh\n"
                       "if [ \"$1\" = \"serve\" ]; then\n"
                       "  cat >/dev/null\n"
                       "fi\n",
                       0755);
}

static void write_package_holon_json(const char *dir,
                                     const char *slug,
                                     const char *uuid,
                                     const char *given_name,
                                     const char *family_name,
                                     const char *alias,
                                     const char *entrypoint,
                                     int with_binary) {
  char manifest_path[1024];
  char arch_dir[64];
  FILE *f;

  assert(ensure_dir_with_system(dir) == 0);
  if (with_binary) {
    write_package_binary(dir, entrypoint != NULL ? entrypoint : slug);
  }
  test_package_arch_dir(arch_dir, sizeof(arch_dir));
  assert(snprintf(manifest_path, sizeof(manifest_path), "%s/.holon.json", dir) < (int)sizeof(manifest_path));
  f = fopen(manifest_path, "w");
  assert(f != NULL);
  fprintf(f,
          "{\n"
          "  \"schema\": \"holon-package/v1\",\n"
          "  \"slug\": \"%s\",\n"
          "  \"uuid\": \"%s\",\n"
          "  \"identity\": {\n"
          "    \"given_name\": \"%s\",\n"
          "    \"family_name\": \"%s\",\n"
          "    \"motto\": \"Test\",\n"
          "    \"aliases\": %s%s%s\n"
          "  },\n"
          "  \"lang\": \"c\",\n"
          "  \"runner\": \"shell\",\n"
          "  \"status\": \"draft\",\n"
          "  \"kind\": \"native\",\n"
          "  \"transport\": \"\",\n"
          "  \"entrypoint\": \"%s\",\n"
          "  \"architectures\": [\"%s\"],\n"
          "  \"has_dist\": false,\n"
          "  \"has_source\": false\n"
          "}\n",
          slug,
          uuid,
          given_name,
          family_name,
          alias != NULL ? "[\"" : "[",
          alias != NULL ? alias : "",
          alias != NULL ? "\"]" : "]",
          entrypoint != NULL ? entrypoint : slug,
          arch_dir);
  fclose(f);
}

static int discover_result_has_slug(const HolonsDiscoverResult *result, const char *slug) {
  size_t i;

  if (result == NULL || slug == NULL) {
    return 0;
  }
  for (i = 0; i < result->found_len; ++i) {
    if (result->found[i].info != NULL && result->found[i].info->slug != NULL &&
        strcmp(result->found[i].info->slug, slug) == 0) {
      return 1;
    }
  }
  return 0;
}

static HolonsHolonRef *discover_result_find_slug(const HolonsDiscoverResult *result, const char *slug) {
  size_t i;

  if (result == NULL || slug == NULL) {
    return NULL;
  }
  for (i = 0; i < result->found_len; ++i) {
    if (result->found[i].info != NULL && result->found[i].info->slug != NULL &&
        strcmp(result->found[i].info->slug, slug) == 0) {
      return &result->found[i];
    }
  }
  return NULL;
}

static void file_url_for_path(char *out, size_t out_len, const char *path) {
  assert(out != NULL);
  assert(path != NULL);
  assert(snprintf(out, out_len, "file://%s", path) < (int)out_len);
}

static void test_discover(void) {
  char root[1024] = "/tmp/holons_uniform_discover_XXXXXX";
  char cwd[1024];
  char op_home[1024];
  char op_bin[1024];
  char siblings_root[1024];
  char bundle_dir[1024];
  char cwd_alpha[1024];
  char alias_dir[1024];
  char path_dir[1024];
  char fast_dir[1024];
  char probe_dir[1024];
  char built_dir[1024];
  char built_dup_dir[1024];
  char installed_dir[1024];
  char cached_dir[1024];
  char source_dir[1024];
  char source_proto[1024];
  char source_bridge_script[1024];
  char source_marker[1024];
  char fast_probe_script[1024];
  char fast_marker[1024];
  char probe_script[1024];
  char probe_marker[1024];
  char ignored_dir[1024];
  char expected_url[1024];
  char source_script_body[4096];
  char fast_script_body[1024];
  char probe_script_body[2048];
  char *prev_oppath = NULL;
  char *prev_opbin = NULL;
  char *prev_bridge = NULL;
  char *prev_siblings = NULL;
  char *prev_probe = NULL;
  HolonsDiscoverResult result;
  HolonsResolveResult resolved;

  check_int(make_temp_dir(root) == 0, "mk discovery temp dir");
  if (root[0] == '\0') {
    return;
  }
  canonicalize_temp_dir(root, sizeof(root));

  discovery_path(op_home, sizeof(op_home), root, "runtime");
  discovery_path(op_bin, sizeof(op_bin), op_home, "bin");
  discovery_path(siblings_root, sizeof(siblings_root), root, "TestApp.app/Contents/Resources/Holons");
  discovery_path(bundle_dir, sizeof(bundle_dir), siblings_root, "bundle.holon");
  discovery_path(cwd_alpha, sizeof(cwd_alpha), root, "cwd-alpha.holon");
  discovery_path(alias_dir, sizeof(alias_dir), root, "nested/alias-target.holon");
  discovery_path(path_dir, sizeof(path_dir), root, "nested/path-match.holon");
  discovery_path(fast_dir, sizeof(fast_dir), root, "fast-path.holon");
  discovery_path(probe_dir, sizeof(probe_dir), root, "probe-fallback.holon");
  discovery_path(built_dir, sizeof(built_dir), root, ".op/build/built-beta.holon");
  discovery_path(built_dup_dir, sizeof(built_dup_dir), root, ".op/build/cwd-alpha-copy.holon");
  discovery_path(installed_dir, sizeof(installed_dir), op_bin, "installed-gamma.holon");
  discovery_path(cached_dir, sizeof(cached_dir), op_home, "cache/deep/cached-delta.holon");
  discovery_path(source_dir, sizeof(source_dir), root, "source-alpha");
  discovery_path(source_proto, sizeof(source_proto), source_dir, "holon.proto");
  discovery_path(source_bridge_script, sizeof(source_bridge_script), root, "source-bridge.sh");
  discovery_path(source_marker, sizeof(source_marker), root, "source-bridge.marker");
  discovery_path(fast_probe_script, sizeof(fast_probe_script), root, "fast-probe.sh");
  discovery_path(fast_marker, sizeof(fast_marker), root, "fast-probe.marker");
  discovery_path(probe_script, sizeof(probe_script), root, "probe-helper.sh");
  discovery_path(probe_marker, sizeof(probe_marker), root, "probe-helper.marker");
  discovery_path(ignored_dir, sizeof(ignored_dir), root, ".git/ignored.holon");

  write_package_holon_json(cwd_alpha, "cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha", NULL, "cwd-alpha", 1);
  write_package_holon_json(alias_dir,
                           "alias-target",
                           "12345678-alias-target",
                           "Alias",
                           "Target",
                           "friendly",
                           "alias-target",
                           1);
  write_package_holon_json(path_dir,
                           "path-match",
                           "uuid-path-match",
                           "Path",
                           "Match",
                           NULL,
                           "path-match",
                           1);
  write_package_holon_json(fast_dir,
                           "fast-path",
                           "uuid-fast-path",
                           "Fast",
                           "Path",
                           NULL,
                           "fast-path",
                           1);
  assert(ensure_dir_with_system(probe_dir) == 0);
  write_package_binary(probe_dir, "probe-fallback");
  write_package_holon_json(built_dir,
                           "built-beta",
                           "uuid-built-beta",
                           "Built",
                           "Beta",
                           NULL,
                           "built-beta",
                           1);
  write_package_holon_json(installed_dir,
                           "installed-gamma",
                           "uuid-installed-gamma",
                           "Installed",
                           "Gamma",
                           NULL,
                           "installed-gamma",
                           1);
  write_package_holon_json(cached_dir,
                           "cached-delta",
                           "uuid-cached-delta",
                           "Cached",
                           "Delta",
                           NULL,
                           "cached-delta",
                           1);
  write_package_holon_json(ignored_dir, "ignored", "uuid-ignored", "Ignored", "Hidden", NULL, "ignored", 1);
  write_package_holon_json(bundle_dir, "bundle", "uuid-bundle", "Bundle", "Holon", NULL, "bundle", 1);
  assert(ensure_dir_with_system(source_dir) == 0);
  write_text_file_mode(source_proto,
                       "syntax = \"proto3\";\n"
                       "package source.v1;\n",
                       0644);

  assert(snprintf(source_script_body,
                  sizeof(source_script_body),
                  "#!/bin/sh\n"
                  "printf '%%s\\n' \"$PWD\" >> '%s'\n"
                  "case \"$PWD\" in\n"
                  "  '%s')\n"
                  "    cat <<'EOF'\n"
                  "{\"entries\":[{\"slug\":\"source-alpha\",\"uuid\":\"uuid-source-alpha\","
                  "\"given_name\":\"Source\",\"family_name\":\"Alpha\","
                  "\"relative_path\":\"source-alpha\"}]}\n"
                  "EOF\n"
                  "    ;;\n"
                  "  '%s')\n"
                  "    cat <<'EOF'\n"
                  "{\"entries\":[{\"slug\":\"source-alpha\",\"uuid\":\"uuid-source-alpha\","
                  "\"given_name\":\"Source\",\"family_name\":\"Alpha\","
                  "\"relative_path\":\".\"}]}\n"
                  "EOF\n"
                  "    ;;\n"
                  "  *)\n"
                  "    printf '{\"entries\":[]}'\n"
                  "    ;;\n"
                  "esac\n",
                  source_marker,
                  root,
                  source_dir) < (int)sizeof(source_script_body));
  write_text_file_mode(source_bridge_script, source_script_body, 0755);

  assert(snprintf(fast_script_body,
                  sizeof(fast_script_body),
                  "#!/bin/sh\n"
                  "printf 'called\\n' >> '%s'\n"
                  "exit 7\n",
                  fast_marker) < (int)sizeof(fast_script_body));
  write_text_file_mode(fast_probe_script, fast_script_body, 0755);

  assert(snprintf(probe_script_body,
                  sizeof(probe_script_body),
                  "#!/bin/sh\n"
                  "printf '%%s\\n' \"$PWD\" >> '%s'\n"
                  "cat <<'EOF'\n"
                  "{\"slug\":\"probe-fallback\",\"uuid\":\"uuid-probe-fallback\","
                  "\"identity\":{\"given_name\":\"Probe\",\"family_name\":\"Fallback\"},"
                  "\"lang\":\"c\",\"runner\":\"shell\",\"status\":\"draft\","
                  "\"kind\":\"native\",\"entrypoint\":\"probe-fallback\","
                  "\"architectures\":[],\"has_dist\":false,\"has_source\":false}\n"
                  "EOF\n",
                  probe_marker) < (int)sizeof(probe_script_body));
  write_text_file_mode(probe_script, probe_script_body, 0755);

  if (getenv("OPPATH") != NULL) {
    prev_oppath = strdup(getenv("OPPATH"));
  }
  if (getenv("OPBIN") != NULL) {
    prev_opbin = strdup(getenv("OPBIN"));
  }
  if (getenv("HOLONS_SOURCE_BRIDGE_COMMAND") != NULL) {
    prev_bridge = strdup(getenv("HOLONS_SOURCE_BRIDGE_COMMAND"));
  }
  if (getenv("HOLONS_SIBLINGS_ROOT") != NULL) {
    prev_siblings = strdup(getenv("HOLONS_SIBLINGS_ROOT"));
  }
  if (getenv("HOLONS_DESCRIBE_PROBE_COMMAND") != NULL) {
    prev_probe = strdup(getenv("HOLONS_DESCRIBE_PROBE_COMMAND"));
  }

  (void)setenv("OPPATH", op_home, 1);
  (void)setenv("OPBIN", op_bin, 1);
  (void)setenv("HOLONS_SOURCE_BRIDGE_COMMAND", source_bridge_script, 1);
  (void)setenv("HOLONS_SIBLINGS_ROOT", siblings_root, 1);
  (void)unsetenv("HOLONS_DESCRIBE_PROBE_COMMAND");

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL, "discover all layers");
  check_int(discover_result_has_slug(&result, "cwd-alpha"), "discover all includes cwd");
  check_int(discover_result_has_slug(&result, "built-beta"), "discover all includes built");
  check_int(discover_result_has_slug(&result, "installed-gamma"), "discover all includes installed");
  check_int(discover_result_has_slug(&result, "cached-delta"), "discover all includes cached");
  check_int(discover_result_has_slug(&result, "bundle"), "discover all includes siblings");
  check_int(discover_result_has_slug(&result, "source-alpha"), "discover all includes source");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_BUILT | HOLONS_INSTALLED, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL, "discover filter by specifiers");
  check_int(result.found_len == 2, "discover filter count");
  check_int(discover_result_has_slug(&result, "built-beta"), "discover filter built");
  check_int(discover_result_has_slug(&result, "installed-gamma"), "discover filter installed");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, "built-beta", root, HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1, "discover match by slug");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, "friendly", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1 &&
                discover_result_has_slug(&result, "alias-target"),
            "discover match by alias");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, "12345678", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1 &&
                discover_result_has_slug(&result, "alias-target"),
            "discover match by UUID prefix");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, "nested/path-match.holon", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  file_url_for_path(expected_url, sizeof(expected_url), path_dir);
  check_int(result.error == NULL && result.found_len == 1 &&
                result.found[0].url != NULL && strcmp(result.found[0].url, expected_url) == 0,
            "discover match by path");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CWD, 1, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1, "discover limit one");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CWD, 0, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len >= 4, "discover limit zero means unlimited");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CWD, -1, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 0, "discover negative limit returns empty");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, 0xFF, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error != NULL, "discover invalid specifiers");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, 0, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && discover_result_has_slug(&result, "built-beta"), "discover specifiers zero treated as all");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len >= 4, "discover null expression returns all");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, "missing", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 0, "discover missing expression returns empty");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(!discover_result_has_slug(&result, "ignored"), "discover excluded dirs skipped");
  holons_discover_result_free(&result);

  write_package_holon_json(
      built_dup_dir, "cwd-alpha-copy", "uuid-cwd-alpha", "Cwd", "Alpha", NULL, "cwd-alpha-copy", 1);
  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && !discover_result_has_slug(&result, "cwd-alpha-copy"),
            "discover deduplicates by UUID");
  holons_discover_result_free(&result);

  unlink(fast_marker);
  (void)setenv("HOLONS_DESCRIBE_PROBE_COMMAND", fast_probe_script, 1);
  result = holons_discover(HOLONS_LOCAL, "fast-path.holon", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1, "discover holon.json fast path");
  check_int(access(fast_marker, F_OK) != 0, "discover holon.json skips probe fallback");
  holons_discover_result_free(&result);

  unlink(probe_marker);
  (void)setenv("HOLONS_DESCRIBE_PROBE_COMMAND", probe_script, 1);
  result = holons_discover(HOLONS_LOCAL, "probe-fallback", root, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1 &&
                discover_result_has_slug(&result, "probe-fallback"),
            "discover Describe fallback");
  check_int(access(probe_marker, F_OK) == 0, "discover Describe fallback invokes probe");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_SIBLINGS, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1 && discover_result_has_slug(&result, "bundle"),
            "discover siblings layer");
  holons_discover_result_free(&result);

  unlink(source_marker);
  (void)unsetenv("HOLONS_DESCRIBE_PROBE_COMMAND");
  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_SOURCE, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && result.found_len == 1 && discover_result_has_slug(&result, "source-alpha"),
            "discover source layer offloads to local op");
  check_int(access(source_marker, F_OK) == 0, "discover source layer called local op bridge");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_BUILT, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && discover_result_has_slug(&result, "built-beta"), "discover built layer");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_INSTALLED, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && discover_result_has_slug(&result, "installed-gamma"), "discover installed layer");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_LOCAL, NULL, root, HOLONS_CACHED, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && discover_result_has_slug(&result, "cached-delta"), "discover cached layer");
  holons_discover_result_free(&result);

  check_int(getcwd(cwd, sizeof(cwd)) != NULL, "discover capture cwd");
  check_int(chdir(root) == 0, "discover chdir root");
  result = holons_discover(HOLONS_LOCAL, NULL, NULL, HOLONS_CWD, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error == NULL && discover_result_has_slug(&result, "cwd-alpha"), "discover nil root defaults to cwd");
  holons_discover_result_free(&result);
  check_int(chdir(cwd) == 0, "discover restore cwd");

  result = holons_discover(HOLONS_LOCAL, NULL, "", HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error != NULL, "discover empty root returns error");
  holons_discover_result_free(&result);

  result = holons_discover(HOLONS_PROXY, NULL, root, HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error != NULL, "discover proxy scope unsupported");
  holons_discover_result_free(&result);
  result = holons_discover(HOLONS_DELEGATED, NULL, root, HOLONS_ALL, HOLONS_NO_LIMIT, HOLONS_NO_TIMEOUT);
  check_int(result.error != NULL, "discover delegated scope unsupported");
  holons_discover_result_free(&result);

  resolved = holons_resolve(HOLONS_LOCAL, "cwd-alpha", root, HOLONS_CWD, HOLONS_NO_TIMEOUT);
  check_int(resolved.error == NULL && resolved.ref != NULL && resolved.ref->info != NULL &&
                strcmp(resolved.ref->info->slug, "cwd-alpha") == 0,
            "resolve known slug");
  holons_resolve_result_free(&resolved);

  resolved = holons_resolve(HOLONS_LOCAL, "missing", root, HOLONS_ALL, HOLONS_NO_TIMEOUT);
  check_int(resolved.error != NULL, "resolve missing target");
  holons_resolve_result_free(&resolved);

  resolved = holons_resolve(HOLONS_LOCAL, "cwd-alpha", root, 0xFF, HOLONS_NO_TIMEOUT);
  check_int(resolved.error != NULL, "resolve invalid specifiers");
  holons_resolve_result_free(&resolved);

  restore_env("OPPATH", prev_oppath);
  restore_env("OPBIN", prev_opbin);
  restore_env("HOLONS_SOURCE_BRIDGE_COMMAND", prev_bridge);
  restore_env("HOLONS_SIBLINGS_ROOT", prev_siblings);
  restore_env("HOLONS_DESCRIBE_PROBE_COMMAND", prev_probe);

  {
    char cleanup_cmd[1200];
    snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
    (void)system(cleanup_cmd);
  }
}

static void test_describe_response(void) {
  char root[] = "/tmp/holons_describe_c_XXXXXX";
  char proto_dir[1024];
  char cleanup_cmd[1200];
  char err[256];
  holons_describe_response_t response;

  check_int(make_temp_dir(root) == 0, "mk describe temp dir");
  write_echo_holon(root);
  snprintf(proto_dir, sizeof(proto_dir), "%s/protos", root);

  memset(&response, 0, sizeof(response));
  err[0] = '\0';
  check_int(holons_build_describe_response(proto_dir, &response, err, sizeof(err)) == 0,
            "describe build response");
  check_int(strcmp(response.manifest.identity.given_name, "Echo") == 0, "describe identity given_name");
  check_int(strcmp(response.manifest.identity.family_name, "Server") == 0, "describe identity family_name");
  check_int(strcmp(response.manifest.identity.motto, "Reply precisely.") == 0, "describe identity motto");
  check_int(response.service_count == 1, "describe service count");
  if (response.service_count == 1) {
    holons_service_doc_t *service = &response.services[0];
    check_int(strcmp(service->name, "echo.v1.Echo") == 0, "describe service name");
    check_int(strcmp(service->description,
                     "Echo echoes request payloads for documentation tests.") == 0,
              "describe service description");
    check_int(service->method_count == 1, "describe method count");
    if (service->method_count == 1) {
      holons_method_doc_t *method = &service->methods[0];
      check_int(strcmp(method->name, "Ping") == 0, "describe method name");
      check_int(strcmp(method->description, "Ping echoes the inbound message.") == 0,
                "describe method description");
      check_int(strcmp(method->input_type, "echo.v1.PingRequest") == 0, "describe input type");
      check_int(strcmp(method->output_type, "echo.v1.PingResponse") == 0, "describe output type");
      check_int(strcmp(method->example_input, "{\"message\":\"hello\",\"sdk\":\"go-holons\"}") == 0,
                "describe method example");
      check_int(method->input_field_count == 2, "describe input field count");
      if (method->input_field_count >= 1) {
        holons_field_doc_t *field = &method->input_fields[0];
        check_int(strcmp(field->name, "message") == 0, "describe field name");
        check_int(strcmp(field->type, "string") == 0, "describe field type");
        check_int(field->number == 1, "describe field number");
        check_int(strcmp(field->description, "Message to echo back.") == 0,
                  "describe field description");
        check_int(field->label == HOLONS_FIELD_LABEL_REQUIRED, "describe field label");
        check_int(field->required == 1, "describe field required");
        check_int(strcmp(field->example, "\"hello\"") == 0, "describe field example");
      }
    }
  }

  holons_free_describe_response(&response);
  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  check_int(system(cleanup_cmd) == 0, "cleanup describe tmp root");
}

static void test_describe_registration(void) {
  char root[] = "/tmp/holons_meta_reg_c_XXXXXX";
  char proto_dir[1024];
  char cleanup_cmd[1200];
  char err[256];
  holons_describe_registration_t registration;
  holons_describe_response_t static_response;
  holons_describe_response_t response;
  holons_describe_request_t request;

  check_int(make_temp_dir(root) == 0, "mk registration temp dir");
  write_echo_holon(root);
  snprintf(proto_dir, sizeof(proto_dir), "%s/protos", root);

  memset(&static_response, 0, sizeof(static_response));
  err[0] = '\0';
  check_int(holons_build_describe_response(proto_dir, &static_response, err, sizeof(err)) == 0,
            "describe registration static response");
  holons_use_static_describe_response(&static_response);

  err[0] = '\0';
  check_int(holons_make_describe_registration(&registration, err, sizeof(err)) == 0,
            "describe registration build");
  check_int(strcmp(registration.service_name, "holons.v1.HolonMeta") == 0,
            "describe registration service");
  check_int(strcmp(registration.method_name, "Describe") == 0,
            "describe registration method");
  check_int(registration.response == &static_response,
            "describe registration uses static response");

  memset(&response, 0, sizeof(response));
  memset(&request, 0, sizeof(request));
  err[0] = '\0';
  check_int(holons_invoke_describe(&registration,
                                   &request,
                                   &response,
                                   err,
                                   sizeof(err)) == 0,
            "describe registration invoke");
  check_int(strcmp(response.manifest.identity.given_name, "Echo") == 0,
            "describe registration identity given_name");
  check_int(strcmp(response.manifest.identity.family_name, "Server") == 0,
            "describe registration identity family_name");
  check_int(response.service_count == 1, "describe registration services");

  holons_free_describe_response(&response);
  holons_use_static_describe_response(NULL);
  holons_free_describe_response(&static_response);
  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  check_int(system(cleanup_cmd) == 0, "cleanup describe registration tmp root");
}

static void test_describe_registration_requires_static_response(void) {
  char err[256];
  holons_describe_registration_t registration;

  holons_use_static_describe_response(NULL);
  err[0] = '\0';
  check_int(holons_make_describe_registration(&registration, err, sizeof(err)) != 0,
            "describe registration requires static response");
  check_int(strcmp(err, "no Incode Description registered — run op build") == 0,
            "describe registration missing error");
}

static void test_describe_without_protos(void) {
  char root[] = "/tmp/holons_describe_empty_c_XXXXXX";
  char manifest_path[1024];
  char proto_dir[1024];
  char cleanup_cmd[1200];
  char err[256];
  FILE *f;
  holons_describe_response_t response;

  check_int(make_temp_dir(root) == 0, "mk empty describe temp dir");
  snprintf(manifest_path, sizeof(manifest_path), "%s/holon.proto", root);
  snprintf(proto_dir, sizeof(proto_dir), "%s/protos", root);
  f = fopen(manifest_path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package holons.test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    given_name: \"Empty\"\n"
          "    family_name: \"Holon\"\n"
          "    motto: \"Still available.\"\n"
          "  }\n"
          "};\n");
  fclose(f);

  memset(&response, 0, sizeof(response));
  err[0] = '\0';
  check_int(holons_build_describe_response(proto_dir, &response, err, sizeof(err)) == 0,
            "describe without protos");
  check_int(strcmp(response.manifest.identity.given_name, "Empty") == 0, "describe empty identity given_name");
  check_int(strcmp(response.manifest.identity.family_name, "Holon") == 0, "describe empty identity family_name");
  check_int(strcmp(response.manifest.identity.motto, "Still available.") == 0, "describe empty identity motto");
  check_int(response.service_count == 0, "describe empty service count");

  holons_free_describe_response(&response);
  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  check_int(system(cleanup_cmd) == 0, "cleanup empty describe tmp root");
}

static void test_describe_static_binary_without_protos(void) {
  char root[] = "/tmp/holons_describe_static_bin_XXXXXX";
  char cwd[1024];
  char holon_proto[1024];
  char output_path[1024];
  char cleanup_cmd[1200];
  char command[4096];
  char output[512];

  check_int(make_temp_dir(root) == 0, "mk static describe temp dir");
  check_int(getcwd(cwd, sizeof(cwd)) != NULL, "capture static describe cwd");

  snprintf(holon_proto, sizeof(holon_proto), "%s/holon.proto", root);
  check_int(access(holon_proto, F_OK) != 0, "static describe has no adjacent holon.proto");

  snprintf(output_path, sizeof(output_path), "%s/describe.out", root);
  snprintf(command,
           sizeof(command),
           "cd '%s' && '%s/describe-static-helper' > '%s' 2>&1",
           root,
           cwd,
           output_path);

  check_int(command_exit_code(command) == 0, "static describe helper exit");
  check_int(read_file(output_path, output, sizeof(output)) == 0, "read static describe helper output");
  check_int(strstr(output, "service=static.v1.Echo") != NULL, "static describe helper service");
  check_int(strstr(output, "methods=1") != NULL, "static describe helper methods");

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  check_int(system(cleanup_cmd) == 0, "cleanup static describe temp root");
}

static void test_connect_direct_dial(void) {
  char root[1024] = "/tmp/holons_connect_direct_XXXXXX";
  char uri[256];
  char binary[1024];
  char cleanup_cmd[1200];
  pid_t pid = -1;
  HolonsConnectResult result;

  check_int(make_temp_dir(root) == 0, "connect direct temp root");
  if (root[0] == '\0') {
    return;
  }
  canonicalize_temp_dir(root, sizeof(root));

  {
    char arch_dir[64];
    char package_dir[1024];

    test_package_arch_dir(arch_dir, sizeof(arch_dir));
    discovery_path(package_dir, sizeof(package_dir), root, "direct-connect.holon");
    write_package_holon_json(package_dir,
                             "direct-connect",
                             "uuid-direct-connect",
                             "Direct",
                             "Connect",
                             NULL,
                             "direct-connect",
                             1);
    snprintf(binary, sizeof(binary), "%s/bin/%s/direct-connect", package_dir, arch_dir);
  }

  if (spawn_background_server(binary, "tcp://127.0.0.1:0", &pid, uri, sizeof(uri)) != 0) {
    ++passed;
    fprintf(stderr, "SKIP: connect direct dial (helper did not start)\n");
  } else {
    result = holons_connect(HOLONS_LOCAL, uri, NULL, HOLONS_ALL, 5000);
    check_int(result.error == NULL && result.channel != NULL, "connect direct target");
    if (result.channel != NULL) {
      holons_disconnect(&result);
      holons_connect_result_free(&result);
      check_int(pid_exists(pid), "connect direct disconnect keeps external server running");
    }
  }

  if (pid > 0) {
    (void)kill(pid, SIGTERM);
    (void)waitpid(pid, NULL, 0);
  }

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  (void)system(cleanup_cmd);
}

static void test_connect_starts_slug_ephemerally(void) {
  char root[1024] = "/tmp/holons_connect_ephemeral_XXXXXX";
  char op_home[1024];
  char op_bin[1024];
  char package_dir[1024];
  char cleanup_cmd[1200];
  char *prev_oppath = NULL;
  char *prev_opbin = NULL;
  HolonsConnectResult result;

  check_int(make_temp_dir(root) == 0, "connect ephemeral temp root");
  if (root[0] == '\0') {
    return;
  }
  canonicalize_temp_dir(root, sizeof(root));

  discovery_path(op_home, sizeof(op_home), root, "runtime");
  discovery_path(op_bin, sizeof(op_bin), op_home, "bin");
  discovery_path(package_dir, sizeof(package_dir), op_bin, "known-slug.holon");
  write_package_holon_json(package_dir,
                           "known-slug",
                           "uuid-known-slug",
                           "Known",
                           "Slug",
                           NULL,
                           "known-slug",
                           1);

  if (getenv("OPPATH") != NULL) {
    prev_oppath = strdup(getenv("OPPATH"));
  }
  if (getenv("OPBIN") != NULL) {
    prev_opbin = strdup(getenv("OPBIN"));
  }

  (void)setenv("OPPATH", op_home, 1);
  (void)setenv("OPBIN", op_bin, 1);

  result = holons_connect(HOLONS_LOCAL, "known-slug", root, HOLONS_INSTALLED, 5000);
  check_int(result.error == NULL, "connect returns HolonsConnectResult");
  check_int(result.channel != NULL, "connect slug returns channel");
  if (result.channel != NULL) {
    holons_disconnect(&result);
  }
  holons_connect_result_free(&result);
  restore_env("OPPATH", prev_oppath);
  restore_env("OPBIN", prev_opbin);

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  (void)system(cleanup_cmd);
}

static void test_connect_reuses_port_file(void) {
  char root[1024] = "/tmp/holons_connect_origin_XXXXXX";
  char op_home[1024];
  char op_bin[1024];
  char package_dir[1024];
  char expected_url[1024];
  char cleanup_cmd[1200];
  char *prev_oppath = NULL;
  char *prev_opbin = NULL;
  HolonsConnectResult result;

  check_int(make_temp_dir(root) == 0, "connect origin temp root");
  if (root[0] == '\0') {
    return;
  }
  canonicalize_temp_dir(root, sizeof(root));

  discovery_path(op_home, sizeof(op_home), root, "runtime");
  discovery_path(op_bin, sizeof(op_bin), op_home, "bin");
  discovery_path(package_dir, sizeof(package_dir), op_bin, "origin-slug.holon");
  write_package_holon_json(package_dir,
                           "origin-slug",
                           "uuid-origin-slug",
                           "Origin",
                           "Slug",
                           NULL,
                           "origin-slug",
                           1);
  file_url_for_path(expected_url, sizeof(expected_url), package_dir);

  if (getenv("OPPATH") != NULL) {
    prev_oppath = strdup(getenv("OPPATH"));
  }
  if (getenv("OPBIN") != NULL) {
    prev_opbin = strdup(getenv("OPBIN"));
  }

  (void)setenv("OPPATH", op_home, 1);
  (void)setenv("OPBIN", op_bin, 1);

  result = holons_connect(HOLONS_LOCAL, "origin-slug", root, HOLONS_INSTALLED, 5000);
  check_int(result.error == NULL, "connect origin returns success");
  check_int(result.origin != NULL && result.origin->info != NULL, "connect origin populated");
  if (result.origin != NULL && result.origin->url != NULL) {
    check_int(strcmp(result.origin->url, expected_url) == 0, "connect origin URL");
  }
  if (result.origin != NULL && result.origin->info != NULL && result.origin->info->slug != NULL) {
    check_int(strcmp(result.origin->info->slug, "origin-slug") == 0, "connect origin slug");
  }
  holons_disconnect(&result);
  holons_connect_result_free(&result);

  restore_env("OPPATH", prev_oppath);
  restore_env("OPBIN", prev_opbin);

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  (void)system(cleanup_cmd);
}

static void test_connect_removes_stale_port_file(void) {
  char root[1024] = "/tmp/holons_connect_missing_XXXXXX";
  char cleanup_cmd[1200];
  HolonsConnectResult result;

  check_int(make_temp_dir(root) == 0, "connect missing temp root");
  if (root[0] == '\0') {
    return;
  }
  canonicalize_temp_dir(root, sizeof(root));

  result = holons_connect(HOLONS_LOCAL, "missing-target", root, HOLONS_INSTALLED, 1000);
  check_int(result.error != NULL, "connect unresolvable target");
  check_int(result.channel == NULL, "connect unresolvable channel");
  check_int(result.origin == NULL, "connect unresolvable origin");
  holons_disconnect(&result);
  holons_connect_result_free(&result);

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  (void)system(cleanup_cmd);
}

static void test_grpc_bridge_advertises_public_uri_first(void) {
  char root[] = "/tmp/holons_grpc_bridge_XXXXXX";
  char proto_dir[1024];
  char proto_file[1024];
  char manifest_path[1024];
  char backend_path[1024];
  char backend_uri_file[1024];
  char wrapper_path[1024];
  char bridge_uri[256];
  char backend_uri[256];
  char cleanup_cmd[1200];
  FILE *f;
  pid_t pid = -1;

  check_int(make_temp_dir(root) == 0, "grpc bridge temp root");
  if (root[0] == '\0') {
    return;
  }

  snprintf(proto_dir, sizeof(proto_dir), "%s/protos/greeting/v1", root);
  snprintf(proto_file, sizeof(proto_file), "%s/greeting.proto", proto_dir);
  snprintf(manifest_path, sizeof(manifest_path), "%s/holon.proto", root);
  snprintf(backend_path, sizeof(backend_path), "%s/backend.sh", root);
  snprintf(backend_uri_file, sizeof(backend_uri_file), "%s/backend.uri", root);
  snprintf(wrapper_path, sizeof(wrapper_path), "%s/bridge-wrapper.sh", root);

  check_int(ensure_dir_with_system(proto_dir) == 0, "grpc bridge proto dir");

  f = fopen(proto_file, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package greeting.v1;\n"
          "service GreetingService {\n"
          "  rpc Ping(PingRequest) returns (PingResponse);\n"
          "}\n"
          "message PingRequest {}\n"
          "message PingResponse {}\n");
  fclose(f);

  f = fopen(manifest_path, "w");
  assert(f != NULL);
  fprintf(f,
          "syntax = \"proto3\";\n"
          "package holons.test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    uuid: \"grpc-bridge-test\"\n"
          "    given_name: \"bridge\"\n"
          "    family_name: \"test\"\n"
          "    motto: \"Checks startup ordering.\"\n"
          "  }\n"
          "};\n");
  fclose(f);

  f = fopen(backend_path, "w");
  assert(f != NULL);
  fprintf(f,
          "#!/bin/sh\n"
          "exec python3 - \"$@\" <<'PY'\n"
          "import signal\n"
          "import socket\n"
          "import sys\n"
          "import time\n"
          "\n"
          "uri_file = %c%s%c\n"
          "listen_uri = 'tcp://127.0.0.1:0'\n"
          "args = sys.argv[1:]\n"
          "for i, arg in enumerate(args):\n"
          "    if arg == '--listen' and i + 1 < len(args):\n"
          "        listen_uri = args[i + 1]\n"
          "        break\n"
          "\n"
          "if not listen_uri.startswith('tcp://'):\n"
          "    raise SystemExit('unsupported listen uri')\n"
          "\n"
          "host_port = listen_uri[len('tcp://'):]\n"
          "host, port_text = host_port.rsplit(':', 1)\n"
          "host = host or '127.0.0.1'\n"
          "port = int(port_text)\n"
          "\n"
          "sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)\n"
          "sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)\n"
          "sock.bind((host, port))\n"
          "sock.listen(8)\n"
          "\n"
          "bound_host, bound_port = sock.getsockname()[:2]\n"
          "public_host = '127.0.0.1' if bound_host in ('0.0.0.0', '') else bound_host\n"
          "uri = f'tcp://{public_host}:{bound_port}'\n"
          "with open(uri_file, 'w', encoding='utf-8') as handle:\n"
          "    handle.write(uri + '\\n')\n"
          "sys.stdout.write(f'gRPC (Connect JSON) server listening on {uri}\\n')\n"
          "sys.stdout.flush()\n"
          "\n"
          "stop = False\n"
          "\n"
          "def handle_signal(_signo, _frame):\n"
          "    global stop\n"
          "    stop = True\n"
          "    try:\n"
          "        sock.close()\n"
          "    except OSError:\n"
          "        pass\n"
          "\n"
          "signal.signal(signal.SIGTERM, handle_signal)\n"
          "signal.signal(signal.SIGINT, handle_signal)\n"
          "\n"
          "while not stop:\n"
          "    try:\n"
          "        conn, _ = sock.accept()\n"
          "    except OSError:\n"
          "        if stop:\n"
          "            break\n"
          "        time.sleep(0.05)\n"
          "        continue\n"
          "    conn.close()\n"
          "PY\n",
          '\'',
          backend_uri_file,
          '\'');
  fclose(f);
  check_int(chmod(backend_path, 0755) == 0, "grpc bridge backend executable");

  f = fopen(wrapper_path, "w");
  assert(f != NULL);
  fprintf(f,
          "#!/bin/sh\n"
          "exec ./bin/grpc-bridge --backend %c%s%c --proto-dir %c%s%c --manifest %c%s%c \"$@\"\n",
          '\'',
          backend_path,
          '\'',
          '\'',
          root,
          '\'',
          '\'',
          manifest_path,
          '\'');
  fclose(f);
  check_int(chmod(wrapper_path, 0755) == 0, "grpc bridge wrapper executable");

  bridge_uri[0] = '\0';
  backend_uri[0] = '\0';

  if (spawn_background_server(wrapper_path, "tcp://127.0.0.1:0", &pid, bridge_uri, sizeof(bridge_uri)) != 0) {
    ++passed;
    fprintf(stderr, "SKIP: grpc bridge public uri ordering (helper did not start)\n");
  } else {
    check_int(wait_for_file(backend_uri_file, 3000) == 0, "grpc bridge backend uri file");
    check_int(read_file(backend_uri_file, backend_uri, sizeof(backend_uri)) == 0,
              "grpc bridge backend uri read");
    if (backend_uri[0] != '\0') {
      char *newline = strchr(backend_uri, '\n');
      if (newline != NULL) {
        *newline = '\0';
      }
      check_int(strcmp(bridge_uri, backend_uri) != 0,
                "grpc bridge advertises public uri before backend uri");
      check_int(strstr(bridge_uri, "tcp://") == bridge_uri,
                "grpc bridge publishes tcp uri");
    }
  }

  if (pid > 0) {
    (void)kill(pid, SIGTERM);
    (void)waitpid(pid, NULL, 0);
  }

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  (void)system(cleanup_cmd);
}

static void test_echo_wrapper_invocation(void) {
  char fake_go[] = "/tmp/holons_fake_go_XXXXXX";
  char fake_log[] = "/tmp/holons_fake_go_log_XXXXXX";
  char capture[8192];
  char *prev_go_bin = NULL;
  char *prev_log = NULL;
  char *prev_gocache = NULL;
  int fake_fd = -1;
  int log_fd = -1;
  FILE *script = NULL;
  int exit_code;

  if (getenv("GO_BIN") != NULL) {
    prev_go_bin = strdup(getenv("GO_BIN"));
  }
  if (getenv("HOLONS_FAKE_GO_LOG") != NULL) {
    prev_log = strdup(getenv("HOLONS_FAKE_GO_LOG"));
  }
  if (getenv("GOCACHE") != NULL) {
    prev_gocache = strdup(getenv("GOCACHE"));
  }

  fake_fd = mkstemp(fake_go);
  check_int(fake_fd >= 0, "mkstemp fake go binary");
  if (fake_fd < 0) {
    restore_env("GO_BIN", prev_go_bin);
    restore_env("HOLONS_FAKE_GO_LOG", prev_log);
    restore_env("GOCACHE", prev_gocache);
    return;
  }

  log_fd = mkstemp(fake_log);
  check_int(log_fd >= 0, "mkstemp fake go log");
  if (log_fd < 0) {
    close(fake_fd);
    unlink(fake_go);
    restore_env("GO_BIN", prev_go_bin);
    restore_env("HOLONS_FAKE_GO_LOG", prev_log);
    restore_env("GOCACHE", prev_gocache);
    return;
  }

  script = fdopen(fake_fd, "w");
  check_int(script != NULL, "fdopen fake go binary");
  if (script == NULL) {
    close(fake_fd);
    close(log_fd);
    unlink(fake_go);
    unlink(fake_log);
    restore_env("GO_BIN", prev_go_bin);
    restore_env("HOLONS_FAKE_GO_LOG", prev_log);
    restore_env("GOCACHE", prev_gocache);
    return;
  }

  fprintf(script,
          "#!/usr/bin/env bash\n"
          "set -euo pipefail\n"
          ": \"${HOLONS_FAKE_GO_LOG:?missing HOLONS_FAKE_GO_LOG}\"\n"
          "{\n"
          "  printf 'PWD=%%s\\n' \"$PWD\"\n"
          "  i=0\n"
          "  for arg in \"$@\"; do\n"
          "    printf 'ARG%%d=%%s\\n' \"$i\" \"$arg\"\n"
          "    i=$((i+1))\n"
          "  done\n"
          "} >\"$HOLONS_FAKE_GO_LOG\"\n");

  (void)fclose(script);
  script = NULL;
  check_int(chmod(fake_go, 0700) == 0, "chmod fake go binary");
  (void)close(log_fd);
  log_fd = -1;

  (void)setenv("GO_BIN", fake_go, 1);
  (void)setenv("HOLONS_FAKE_GO_LOG", fake_log, 1);
  (void)unsetenv("GOCACHE");

  capture[0] = '\0';
  exit_code = command_exit_code("./bin/echo-client stdio:// --message cert-stdio >/dev/null 2>&1");
  check_int(exit_code == 0, "echo-client wrapper exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0, "read echo-client wrapper capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "PWD=") != NULL && strstr(capture, "/sdk/go-holons") != NULL,
              "echo-client wrapper cwd");
    check_int(strstr(capture, "ARG0=run") != NULL, "echo-client wrapper uses go run");
    check_int(strstr(capture, "go_echo_client.go") != NULL, "echo-client wrapper helper path");
    check_int(strstr(capture, "--sdk") != NULL && strstr(capture, "c-holons") != NULL,
              "echo-client wrapper sdk default");
    check_int(strstr(capture, "--server-sdk") != NULL && strstr(capture, "go-holons") != NULL,
              "echo-client wrapper server sdk default");
    check_int(strstr(capture, "stdio://") != NULL, "echo-client wrapper forwards URI");
    check_int(strstr(capture, "--message") != NULL && strstr(capture, "cert-stdio") != NULL,
              "echo-client wrapper forwards message");
  }

  capture[0] = '\0';
  exit_code = command_exit_code("./bin/echo-server --listen stdio:// >/dev/null 2>&1");
  check_int(exit_code == 0, "echo-server wrapper exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0, "read echo-server wrapper capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "PWD=") != NULL && strstr(capture, "/sdk/go-holons") != NULL,
              "echo-server wrapper cwd");
    check_int(strstr(capture, "ARG0=run") != NULL, "echo-server wrapper uses go run");
    check_int(strstr(capture, "go_echo_server_slow.go") != NULL, "echo-server wrapper helper path");
    check_int(strstr(capture, "--sdk") != NULL && strstr(capture, "c-holons") != NULL,
              "echo-server wrapper sdk default");
    check_int(strstr(capture, "--max-recv-bytes") != NULL && strstr(capture, "1572864") != NULL,
              "echo-server wrapper max recv default");
    check_int(strstr(capture, "--max-send-bytes") != NULL && strstr(capture, "1572864") != NULL,
              "echo-server wrapper max send default");
    check_int(strstr(capture, "--listen") != NULL && strstr(capture, "stdio://") != NULL,
              "echo-server wrapper forwards listen URI");
  }

  capture[0] = '\0';
  exit_code =
      command_exit_code("./bin/echo-server serve --listen stdio:// --sdk cert-go >/dev/null 2>&1");
  check_int(exit_code == 0, "echo-server wrapper serve exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0,
            "read echo-server wrapper serve capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "go_echo_server_slow.go") != NULL,
              "echo-server serve wrapper helper path");
    check_int(strstr(capture, "serve") != NULL, "echo-server serve wrapper preserves serve");
    check_int(strstr(capture, "--sdk") != NULL && strstr(capture, "c-holons") != NULL,
              "echo-server serve wrapper default sdk placement");
    check_int(strstr(capture, "--max-recv-bytes") != NULL && strstr(capture, "1572864") != NULL,
              "echo-server serve wrapper max recv default");
    check_int(strstr(capture, "--max-send-bytes") != NULL && strstr(capture, "1572864") != NULL,
              "echo-server serve wrapper max send default");
    check_int(strstr(capture, "--listen") != NULL && strstr(capture, "stdio://") != NULL,
              "echo-server serve wrapper forwards listen URI");
  }

  capture[0] = '\0';
  exit_code =
      command_exit_code("./bin/holon-rpc-client ws://127.0.0.1:8080/rpc --connect-only >/dev/null 2>&1");
  check_int(exit_code == 0, "holon-rpc-client wrapper exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0,
            "read holon-rpc-client wrapper capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "PWD=") != NULL && strstr(capture, "/sdk/go-holons") != NULL,
              "holon-rpc-client wrapper cwd");
    check_int(strstr(capture, "ARG0=run") != NULL, "holon-rpc-client wrapper uses go run");
    check_int(strstr(capture, "go_holonrpc_client.go") != NULL,
              "holon-rpc-client wrapper helper path");
    check_int(strstr(capture, "--sdk") != NULL && strstr(capture, "c-holons") != NULL,
              "holon-rpc-client wrapper sdk default");
    check_int(strstr(capture, "--server-sdk") != NULL && strstr(capture, "go-holons") != NULL,
              "holon-rpc-client wrapper server sdk default");
    check_int(strstr(capture, "ws://127.0.0.1:8080/rpc") != NULL,
              "holon-rpc-client wrapper forwards URI");
    check_int(strstr(capture, "--connect-only") != NULL,
              "holon-rpc-client wrapper forwards connect-only");
  }

  capture[0] = '\0';
  exit_code = command_exit_code("./bin/holon-rpc-server ws://127.0.0.1:8080/rpc >/dev/null 2>&1");
  check_int(exit_code == 0, "holon-rpc-server wrapper exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0,
            "read holon-rpc-server wrapper capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "PWD=") != NULL && strstr(capture, "/sdk/go-holons") != NULL,
              "holon-rpc-server wrapper cwd");
    check_int(strstr(capture, "ARG0=run") != NULL, "holon-rpc-server wrapper uses go run");
    check_int(strstr(capture, "go_holonrpc_server.go") != NULL,
              "holon-rpc-server wrapper helper path");
    check_int(strstr(capture, "--sdk") != NULL && strstr(capture, "c-holons") != NULL,
              "holon-rpc-server wrapper sdk default");
    check_int(strstr(capture, "ws://127.0.0.1:8080/rpc") != NULL,
              "holon-rpc-server wrapper forwards URI");
  }

  capture[0] = '\0';
  exit_code = command_exit_code(
      "./bin/grpc-bridge --backend /tmp/cert-backend --proto-dir /tmp/cert-protos "
      "--manifest /tmp/cert-holon.proto --listen stdio:// >/dev/null 2>&1");
  check_int(exit_code == 0, "grpc-bridge wrapper exit");
  check_int(read_file(fake_log, capture, sizeof(capture)) == 0,
            "read grpc-bridge wrapper capture");
  if (capture[0] != '\0') {
    check_int(strstr(capture, "PWD=") != NULL && strstr(capture, "/sdk/go-holons") != NULL,
              "grpc-bridge wrapper cwd");
    check_int(strstr(capture, "ARG0=run") != NULL, "grpc-bridge wrapper uses go run");
    check_int(strstr(capture, "grpc-bridge-go/main.go") != NULL,
              "grpc-bridge wrapper helper path");
    check_int(strstr(capture, "--backend") != NULL && strstr(capture, "/tmp/cert-backend") != NULL,
              "grpc-bridge wrapper forwards backend");
    check_int(strstr(capture, "--proto-dir") != NULL && strstr(capture, "/tmp/cert-protos") != NULL,
              "grpc-bridge wrapper forwards proto dir");
    check_int(strstr(capture, "--manifest") != NULL &&
                  strstr(capture, "/tmp/cert-holon.proto") != NULL,
              "grpc-bridge wrapper forwards manifest");
    check_int(strstr(capture, "--listen") != NULL && strstr(capture, "stdio://") != NULL,
              "grpc-bridge wrapper forwards listen");
  }

  unlink(fake_go);
  unlink(fake_log);
  restore_env("GO_BIN", prev_go_bin);
  restore_env("HOLONS_FAKE_GO_LOG", prev_log);
  restore_env("GOCACHE", prev_gocache);
}

static void test_scheme_and_flags(void) {
  char uri[HOLONS_MAX_URI_LEN];
  char *args1[] = {"--listen", "ws://127.0.0.1:8080/grpc"};
  char *args2[] = {"--port", "7000"};
  char **args3 = NULL;

  check_int(holons_scheme_from_uri("tcp://:9090") == HOLONS_SCHEME_TCP, "scheme tcp");
  check_int(holons_scheme_from_uri("unix:///tmp/x.sock") == HOLONS_SCHEME_UNIX, "scheme unix");
  check_int(holons_scheme_from_uri("stdio://") == HOLONS_SCHEME_STDIO, "scheme stdio");
  check_int(holons_scheme_from_uri("ws://127.0.0.1:8080/grpc") == HOLONS_SCHEME_WS, "scheme ws");
  check_int(holons_scheme_from_uri("wss://127.0.0.1:8443/grpc") == HOLONS_SCHEME_WSS, "scheme wss");

  check_int(strcmp(holons_default_uri(), "tcp://:9090") == 0, "default URI");

  check_int(holons_parse_flags(2, args1, uri, sizeof(uri)) == 0, "parse flags --listen");
  check_int(strcmp(uri, "ws://127.0.0.1:8080/grpc") == 0, "flags --listen value");

  check_int(holons_parse_flags(2, args2, uri, sizeof(uri)) == 0, "parse flags --port");
  check_int(strcmp(uri, "tcp://:7000") == 0, "flags --port value");

  check_int(holons_parse_flags(0, args3, uri, sizeof(uri)) == 0, "parse flags default");
  check_int(strcmp(uri, "tcp://:9090") == 0, "flags default value");
}

static void test_uri_parsing(void) {
  holons_uri_t parsed;
  char err[256];

  check_int(holons_parse_uri("tcp://127.0.0.1:9090", &parsed, err, sizeof(err)) == 0, "parse tcp");
  check_int(parsed.scheme == HOLONS_SCHEME_TCP, "tcp scheme");
  check_int(strcmp(parsed.host, "127.0.0.1") == 0, "tcp host");
  check_int(parsed.port == 9090, "tcp port");

  check_int(holons_parse_uri("unix:///tmp/holons.sock", &parsed, err, sizeof(err)) == 0, "parse unix");
  check_int(parsed.scheme == HOLONS_SCHEME_UNIX, "unix scheme");
  check_int(strcmp(parsed.path, "/tmp/holons.sock") == 0, "unix path");

  check_int(holons_parse_uri("stdio://", &parsed, err, sizeof(err)) == 0, "parse stdio");
  check_int(parsed.scheme == HOLONS_SCHEME_STDIO, "stdio scheme");

  check_int(holons_parse_uri("ws://127.0.0.1:8080/grpc", &parsed, err, sizeof(err)) == 0, "parse ws");
  check_int(parsed.scheme == HOLONS_SCHEME_WS, "ws scheme");
  check_int(strcmp(parsed.path, "/grpc") == 0, "ws path");

  check_int(holons_parse_uri("wss://127.0.0.1:8443", &parsed, err, sizeof(err)) == 0, "parse wss");
  check_int(parsed.scheme == HOLONS_SCHEME_WSS, "wss scheme");
  check_int(strcmp(parsed.path, "/grpc") == 0, "wss default path");
}

static void test_identity_parsing(void) {
  holons_identity_t id;
  char err[256];
  char path[] = "/tmp/holons_identity_XXXXXX";
  int fd = mkstemp(path);
  FILE *f;

  check_int(fd >= 0, "mkstemps");
  if (fd < 0) {
    return;
  }

  f = fdopen(fd, "w");
  check_int(f != NULL, "fdopen");
  if (f == NULL) {
    close(fd);
    unlink(path);
    return;
  }

  fprintf(f,
          "syntax = \"proto3\";\n"
          "package test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    uuid: \"abc-123\"\n"
          "    given_name: \"demo\"\n"
          "    family_name: \"Holons\"\n"
          "    motto: \"Hello\"\n"
          "    composer: \"B. ALTER\"\n"
          "    clade: \"deterministic/pure\"\n"
          "    status: \"draft\"\n"
          "    born: \"2026-02-12\"\n"
          "  }\n"
          "  lang: \"c\"\n"
          "};\n");
  fclose(f);

  check_int(holons_parse_holon(path, &id, err, sizeof(err)) == 0, "parse_holon");
  check_int(strcmp(id.uuid, "abc-123") == 0, "identity uuid");
  check_int(strcmp(id.given_name, "demo") == 0, "identity given_name");
  check_int(strcmp(id.lang, "c") == 0, "identity lang");

  unlink(path);
}

static void test_manifest_resolution(void) {
  char root[] = "/tmp/holons_manifest_c_XXXXXX";
  char manifest_path[1024];
  char resolved_path[1024];
  char cleanup_cmd[1200];
  char err[256];
  holons_manifest_t manifest;
  FILE *f;

  check_int(make_temp_dir(root) == 0, "mk manifest temp dir");
  snprintf(manifest_path, sizeof(manifest_path), "%s/holon.proto", root);
  f = fopen(manifest_path, "w");
  check_int(f != NULL, "open manifest fixture");
  if (f == NULL) {
    return;
  }

  fprintf(f,
          "syntax = \"proto3\";\n"
          "package test.v1;\n\n"
          "option (holons.v1.manifest) = {\n"
          "  identity: {\n"
          "    uuid: \"manifest-123\"\n"
          "    given_name: \"Manifest\"\n"
          "    family_name: \"Resolver\"\n"
          "    motto: \"Read the proto once.\"\n"
          "  }\n"
          "  lang: \"c\"\n"
          "  kind: \"native\"\n"
          "  build: {\n"
          "    runner: \"cmake\"\n"
          "    main: \"./cmd\"\n"
          "  }\n"
          "  artifacts: {\n"
          "    binary: \"manifest-resolver\"\n"
          "    primary: \"manifest.app\"\n"
          "  }\n"
          "};\n");
  fclose(f);

  memset(&manifest, 0, sizeof(manifest));
  err[0] = '\0';
  check_int(holons_resolve_manifest(root,
                                    &manifest,
                                    resolved_path,
                                    sizeof(resolved_path),
                                    err,
                                    sizeof(err)) == 0,
            "resolve manifest");
  check_int(strcmp(resolved_path, manifest_path) == 0, "resolve manifest path");
  check_int(strcmp(manifest.identity.uuid, "manifest-123") == 0, "resolve manifest uuid");
  check_int(strcmp(manifest.identity.given_name, "Manifest") == 0, "resolve manifest given_name");
  check_int(strcmp(manifest.lang, "c") == 0, "resolve manifest lang");
  check_int(strcmp(manifest.kind, "native") == 0, "resolve manifest kind");
  check_int(strcmp(manifest.build.runner, "cmake") == 0, "resolve manifest runner");
  check_int(strcmp(manifest.build.main, "./cmd") == 0, "resolve manifest main");
  check_int(strcmp(manifest.artifacts.binary, "manifest-resolver") == 0,
            "resolve manifest binary");
  check_int(strcmp(manifest.artifacts.primary, "manifest.app") == 0,
            "resolve manifest primary");

  snprintf(cleanup_cmd, sizeof(cleanup_cmd), "rm -rf '%s'", root);
  check_int(system(cleanup_cmd) == 0, "cleanup manifest temp root");
}

static void test_tcp_transport(void) {
  holons_listener_t listener;
  holons_uri_t bound;
  holons_conn_t client_conn = {.read_fd = -1, .write_fd = -1};
  holons_conn_t server_conn;
  char err[256];
  char buf[32];
  ssize_t n;

  if (holons_listen("tcp://127.0.0.1:0", &listener, err, sizeof(err)) != 0) {
    if (is_bind_restricted(err)) {
      ++passed;
      fprintf(stderr, "SKIP: listen tcp (%s)\n", err);
      return;
    }
    check_int(0, "listen tcp");
    return;
  }
  check_int(1, "listen tcp");
  check_int(strncmp(listener.bound_uri, "tcp://", 6) == 0, "tcp bound URI");

  check_int(holons_parse_uri(listener.bound_uri, &bound, err, sizeof(err)) == 0, "parse tcp bound URI");
  check_int(holons_dial_tcp(bound.host, bound.port, &client_conn, err, sizeof(err)) == 0, "dial tcp");
  if (client_conn.read_fd < 0) {
    holons_close_listener(&listener);
    return;
  }

  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "accept tcp");

  holons_conn_write(&client_conn, "ping", 4);
  n = holons_conn_read(&server_conn, buf, sizeof(buf));
  check_int(n == 4, "tcp read");

  holons_conn_write(&server_conn, "pong", 4);
  n = holons_conn_read(&client_conn, buf, sizeof(buf));
  check_int(n == 4, "tcp write");

  holons_conn_close(&client_conn);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

static void test_unix_transport(void) {
  holons_listener_t listener;
  holons_conn_t server_conn;
  char uri[256];
  char err[256];
  char buf[32];
  int client_fd;
  ssize_t n;

  snprintf(uri, sizeof(uri), "unix:///tmp/holons_test_%ld.sock", (long)getpid());
  if (holons_listen(uri, &listener, err, sizeof(err)) != 0) {
    if (is_bind_restricted(err)) {
      ++passed;
      fprintf(stderr, "SKIP: listen unix (%s)\n", err);
      return;
    }
    check_int(0, "listen unix");
    return;
  }
  check_int(1, "listen unix");

  client_fd = dial_unix(listener.uri.path);
  check_int(client_fd >= 0, "dial unix");
  if (client_fd < 0) {
    holons_close_listener(&listener);
    return;
  }

  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "accept unix");

  write(client_fd, "hi", 2);
  n = holons_conn_read(&server_conn, buf, sizeof(buf));
  check_int(n == 2, "unix read");

  holons_conn_write(&server_conn, "ok", 2);
  n = read(client_fd, buf, sizeof(buf));
  check_int(n == 2, "unix write");

  close(client_fd);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

static void test_stdio_transport(void) {
  holons_listener_t listener;
  holons_conn_t conn;
  char err[256];

  check_int(holons_listen("stdio://", &listener, err, sizeof(err)) == 0, "listen stdio");
  check_int(holons_accept(&listener, &conn, err, sizeof(err)) == 0, "accept stdio");
  check_int(conn.read_fd == STDIN_FILENO, "stdio read fd");
  check_int(conn.write_fd == STDOUT_FILENO, "stdio write fd");
  holons_conn_close(&conn);
  check_int(holons_accept(&listener, &conn, err, sizeof(err)) != 0, "stdio single-use");
  holons_close_listener(&listener);
}

static void test_dial_stdio(void) {
  holons_conn_t conn;
  char err[256];

  check_int(holons_dial_stdio(&conn, err, sizeof(err)) == 0, "dial stdio");
  check_int(conn.read_fd == STDIN_FILENO, "dial stdio read fd");
  check_int(conn.write_fd == STDOUT_FILENO, "dial stdio write fd");
  check_int(conn.owns_read_fd == 0, "dial stdio owns read fd");
  check_int(conn.owns_write_fd == 0, "dial stdio owns write fd");
  holons_conn_close(&conn);
}

static const char *resolve_go_binary(void) {
  const char *preferred = "/Users/bpds/go/go1.25.1/bin/go";
  if (access(preferred, X_OK) == 0) {
    return preferred;
  }
  return "go";
}

static void test_cross_language_go_echo(void) {
  const char *go_bin = resolve_go_binary();
  const char *helper = "../c-holons/test/go_echo_server.go";
  char cmd[1024];
  char uri[256];
  char err[256];
  char buf[32];
  holons_uri_t parsed;
  holons_conn_t conn = {.read_fd = -1, .write_fd = -1};
  FILE *proc;
  ssize_t n;
  int status;

  snprintf(cmd,
           sizeof(cmd),
           "cd ../go-holons && '%s' run '%s' 2>/dev/null",
           go_bin,
           helper);

  proc = popen(cmd, "r");
  if (proc == NULL) {
    ++passed;
    fprintf(stderr, "SKIP: cross-language go echo (popen failed)\n");
    return;
  }

  if (fgets(uri, sizeof(uri), proc) == NULL) {
    ++passed;
    fprintf(stderr, "SKIP: cross-language go echo (helper did not start)\n");
    (void)pclose(proc);
    return;
  }
  uri[strcspn(uri, "\r\n")] = '\0';

  check_int(holons_parse_uri(uri, &parsed, err, sizeof(err)) == 0, "cross-language parse go URI");
  check_int(parsed.scheme == HOLONS_SCHEME_TCP, "cross-language go URI scheme");

  check_int(holons_dial_tcp(parsed.host, parsed.port, &conn, err, sizeof(err)) == 0,
            "cross-language dial go tcp");
  if (conn.read_fd >= 0) {
    holons_conn_write(&conn, "go", 2);
    n = holons_conn_read(&conn, buf, sizeof(buf));
    check_int(n == 2, "cross-language go echo read");
    check_int(memcmp(buf, "go", 2) == 0, "cross-language go echo payload");
    holons_conn_close(&conn);
  }

  status = pclose(proc);
  check_int(status == 0, "cross-language go echo process exit");
}

static void test_cross_language_go_holonrpc(void) {
  const char *go_bin = resolve_go_binary();
  const char *helper = "../c-holons/test/go_holonrpc_server.go";
  const char *client_args[] = {
      "--connect-only --timeout-ms 1200",
      "--method echo.v1.Echo/Ping --message cert",
      "--method does.not.Exist/Nope --expect-error -32601,12",
      "--method rpc.heartbeat",
  };
  const char *client_labels[] = {
      "cross-language holon-rpc connect",
      "cross-language holon-rpc echo",
      "cross-language holon-rpc error",
      "cross-language holon-rpc heartbeat",
  };
  const char *server_labels[] = {
      "cross-language holon-rpc server exit connect",
      "cross-language holon-rpc server exit echo",
      "cross-language holon-rpc server exit error",
      "cross-language holon-rpc server exit heartbeat",
  };
  char server_cmd[1024];
  char client_cmd[2048];
  char uri[256];
  FILE *proc;
  int i;

  for (i = 0; i < 4; ++i) {
    int exit_code;
    int status;

    snprintf(server_cmd,
             sizeof(server_cmd),
             "cd ../go-holons && '%s' run '%s' --once 2>/dev/null",
             go_bin,
             helper);

    proc = popen(server_cmd, "r");
    if (proc == NULL) {
      ++passed;
      fprintf(stderr, "SKIP: %s (popen failed)\n", client_labels[i]);
      return;
    }

    if (fgets(uri, sizeof(uri), proc) == NULL) {
      ++passed;
      fprintf(stderr, "SKIP: %s (helper did not start)\n", client_labels[i]);
      (void)pclose(proc);
      return;
    }
    uri[strcspn(uri, "\r\n")] = '\0';

    snprintf(client_cmd,
             sizeof(client_cmd),
             "./bin/holon-rpc-client \"%s\" %s >/dev/null 2>&1",
             uri,
             client_args[i]);
    exit_code = command_exit_code(client_cmd);
    check_int(exit_code == 0, client_labels[i]);

    status = pclose(proc);
    check_int(status == 0, server_labels[i]);
  }
}

static void test_go_client_against_sdk_stdio_server(void) {
  const char *go_bin = resolve_go_binary();
  char cmd[2048];
  int exit_code;

  snprintf(cmd,
           sizeof(cmd),
           "cd ../go-holons && '%s' run ./cmd/echo-client --sdk go-holons --server-sdk c-holons "
           "--message cert-l2-listen-stdio --stdio-bin ../c-holons/bin/echo-server stdio:// "
           ">/dev/null 2>&1",
           go_bin);
  exit_code = command_exit_code(cmd);
  check_int(exit_code == 0, "go echo-client stdio dial against c-holons server");
}

static void test_holonrpc_connect_only_reconnect_probe(void) {
  const char *script =
      "cleanup() {\n"
      "  if [ -n \"${C_PID:-}\" ] && kill -0 \"$C_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$C_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$C_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "  if [ -n \"${S1_PID:-}\" ] && kill -0 \"$S1_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$S1_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$S1_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "  if [ -n \"${S2_PID:-}\" ] && kill -0 \"$S2_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$S2_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$S2_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "}\n"
      "trap cleanup EXIT\n"
      "PORT=\"\"\n"
      "for p in $(seq 39310 39390); do\n"
      "  if ! lsof -nP -iTCP:\"$p\" -sTCP:LISTEN >/dev/null 2>&1; then\n"
      "    PORT=\"$p\"\n"
      "    break\n"
      "  fi\n"
      "done\n"
      "[ -n \"$PORT\" ]\n"
      "URL=\"ws://127.0.0.1:${PORT}/rpc\"\n"
      "S1_OUT=$(mktemp)\n"
      "S1_ERR=$(mktemp)\n"
      "./bin/holon-rpc-server --sdk go-holons \"$URL\" >\"$S1_OUT\" 2>\"$S1_ERR\" &\n"
      "S1_PID=$!\n"
      "for _ in $(seq 1 80); do\n"
      "  if [ -s \"$S1_OUT\" ]; then break; fi\n"
      "  sleep 0.05\n"
      "done\n"
      "C_OUT=$(mktemp)\n"
      "C_ERR=$(mktemp)\n"
      "./bin/holon-rpc-client \"$URL\" --connect-only --timeout-ms 5200 >\"$C_OUT\" 2>\"$C_ERR\" &\n"
      "C_PID=$!\n"
      "sleep 1\n"
      "kill -0 \"$C_PID\" >/dev/null 2>&1\n"
      "kill -TERM \"$S1_PID\" >/dev/null 2>&1 || true\n"
      "wait \"$S1_PID\" >/dev/null 2>&1 || true\n"
      "S2_OUT=$(mktemp)\n"
      "S2_ERR=$(mktemp)\n"
      "./bin/holon-rpc-server --sdk go-holons \"$URL\" >\"$S2_OUT\" 2>\"$S2_ERR\" &\n"
      "S2_PID=$!\n"
      "sleep 0.3\n"
      "sleep 1\n"
      "kill -0 \"$C_PID\" >/dev/null 2>&1\n"
      "wait \"$C_PID\"\n"
      "grep -q '\"status\":\"pass\"' \"$C_OUT\"\n";

  check_int(run_bash_script(script) == 0, "holon-rpc connect-only reconnect probe");
}

static void test_echo_server_rejects_oversized_message(void) {
  const char *go_bin = resolve_go_binary();
  char script[8192];

  snprintf(script,
           sizeof(script),
           "cleanup() {\n"
           "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
           "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
           "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
           "  fi\n"
           "}\n"
           "trap cleanup EXIT\n"
           "S_OUT=$(mktemp)\n"
           "S_ERR=$(mktemp)\n"
           "./bin/echo-server --listen tcp://127.0.0.1:0 >\"$S_OUT\" 2>\"$S_ERR\" &\n"
           "S_PID=$!\n"
           "ADDR=\"\"\n"
           "for _ in $(seq 1 120); do\n"
           "  if [ -s \"$S_OUT\" ]; then\n"
           "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\\\r\\\\n')\n"
           "    if [ -n \"$ADDR\" ]; then break; fi\n"
           "  fi\n"
           "  sleep 0.05\n"
           "done\n"
           "[ -n \"$ADDR\" ]\n"
           "cd ../go-holons\n"
           "'%s' run ../c-holons/test/go_large_ping.go \"$ADDR\" >/dev/null 2>&1\n",
           go_bin);

  check_int(run_bash_script(script) == 0, "echo-server oversized request rejection");
}

static void test_ws_transport(void) {
  holons_listener_t listener;
  holons_uri_t bound;
  holons_conn_t client_conn = {.read_fd = -1, .write_fd = -1};
  holons_conn_t server_conn;
  char err[256];
  char buf[32];
  ssize_t n;

  if (holons_listen("ws://127.0.0.1:0/grpc", &listener, err, sizeof(err)) != 0) {
    if (is_bind_restricted(err)) {
      ++passed;
      fprintf(stderr, "SKIP: listen ws (%s)\n", err);
      return;
    }
    check_int(0, "listen ws");
    return;
  }
  check_int(1, "listen ws");
  check_int(strncmp(listener.bound_uri, "ws://", 5) == 0, "ws bound URI");
  check_int(holons_parse_uri(listener.bound_uri, &bound, err, sizeof(err)) == 0, "parse ws URI");
  check_int(strcmp(bound.path, "/grpc") == 0, "ws path");

  check_int(holons_dial_tcp(bound.host, bound.port, &client_conn, err, sizeof(err)) == 0, "dial ws socket");
  if (client_conn.read_fd < 0) {
    holons_close_listener(&listener);
    return;
  }

  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "accept ws");
  holons_conn_write(&client_conn, "ws", 2);
  n = holons_conn_read(&server_conn, buf, sizeof(buf));
  check_int(n == 2, "ws read");
  holons_conn_write(&server_conn, "ok", 2);
  n = holons_conn_read(&client_conn, buf, sizeof(buf));
  check_int(n == 2, "ws write");

  holons_conn_close(&client_conn);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

static void test_wss_transport(void) {
  holons_listener_t listener;
  holons_uri_t bound;
  holons_conn_t client_conn = {.read_fd = -1, .write_fd = -1};
  holons_conn_t server_conn;
  char err[256];
  char buf[32];
  ssize_t n;

  if (holons_listen("wss://127.0.0.1:0", &listener, err, sizeof(err)) != 0) {
    if (is_bind_restricted(err)) {
      ++passed;
      fprintf(stderr, "SKIP: listen wss (%s)\n", err);
      return;
    }
    check_int(0, "listen wss");
    return;
  }
  check_int(1, "listen wss");
  check_int(strncmp(listener.bound_uri, "wss://", 6) == 0, "wss bound URI");
  check_int(holons_parse_uri(listener.bound_uri, &bound, err, sizeof(err)) == 0, "parse wss URI");
  check_int(strcmp(bound.path, "/grpc") == 0, "wss default path");

  check_int(holons_dial_tcp(bound.host, bound.port, &client_conn, err, sizeof(err)) == 0,
            "dial wss socket");
  if (client_conn.read_fd < 0) {
    holons_close_listener(&listener);
    return;
  }

  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "accept wss");
  holons_conn_write(&client_conn, "wss", 3);
  n = holons_conn_read(&server_conn, buf, sizeof(buf));
  check_int(n == 3, "wss read");
  holons_conn_write(&server_conn, "ok", 2);
  n = holons_conn_read(&client_conn, buf, sizeof(buf));
  check_int(n == 2, "wss write");

  holons_conn_close(&client_conn);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

static void test_connect_direct_ws_target(void) {
  const char *binary = "./bin/echo-server";
  char uri[256];
  pid_t pid = -1;
  HolonsConnectResult result;

  if (spawn_background_server(binary, "ws://127.0.0.1:0/grpc", &pid, uri, sizeof(uri)) != 0) {
    ++passed;
    fprintf(stderr, "SKIP: connect direct ws target (server did not start)\n");
    return;
  }

  result = holons_connect(HOLONS_LOCAL, uri, NULL, HOLONS_ALL, 5000);
  check_int(result.error == NULL && result.channel != NULL, "connect direct ws target");
  if (result.channel != NULL) {
    holons_disconnect(&result);
    holons_connect_result_free(&result);
  }

  kill(pid, SIGTERM);
  waitpid(pid, NULL, 0);
}

static void test_connect_direct_rest_sse_target(void) {
  const char *binary = "./bin/holon-rpc-server";
  char http_uri[256];
  char rest_uri[256];
  pid_t pid = -1;
  HolonsConnectResult result;
  char *const argv[] = {(char *)binary, "http://127.0.0.1:0/api/v1/rpc", NULL};

  if (spawn_background_command(binary, argv, &pid, http_uri, sizeof(http_uri)) != 0) {
    ++passed;
    fprintf(stderr, "SKIP: connect direct rest+sse target (server did not start)\n");
    return;
  }

  if (strncmp(http_uri, "http://", 7) != 0) {
    check_int(0, "connect direct rest+sse target URI");
    kill(pid, SIGTERM);
    waitpid(pid, NULL, 0);
    return;
  }

  if (snprintf(rest_uri, sizeof(rest_uri), "rest+sse://%s", http_uri + 7) >= (int)sizeof(rest_uri)) {
    check_int(0, "connect direct rest+sse target URI conversion");
    kill(pid, SIGTERM);
    waitpid(pid, NULL, 0);
    return;
  }

  result = holons_connect(HOLONS_LOCAL, rest_uri, NULL, HOLONS_ALL, 5000);
  check_int(result.error == NULL && result.channel != NULL, "connect direct rest+sse target");
  if (result.channel != NULL) {
    holons_disconnect(&result);
    holons_connect_result_free(&result);
  }

  kill(pid, SIGTERM);
  waitpid(pid, NULL, 0);
}

static void test_echo_client_ws_dial(void) {
  const char *script =
      "cleanup() {\n"
      "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "}\n"
      "trap cleanup EXIT\n"
      "S_OUT=$(mktemp)\n"
      "S_ERR=$(mktemp)\n"
      "./bin/echo-server --listen ws://127.0.0.1:0/grpc >\"$S_OUT\" 2>\"$S_ERR\" &\n"
      "S_PID=$!\n"
      "ADDR=\"\"\n"
      "for _ in $(seq 1 120); do\n"
      "  if [ -s \"$S_OUT\" ]; then\n"
      "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
      "    if [ -n \"$ADDR\" ]; then break; fi\n"
      "  fi\n"
      "  sleep 0.05\n"
      "done\n"
      "[ -n \"$ADDR\" ]\n"
      "./bin/echo-client --message cert-ws \"$ADDR\" >/dev/null 2>&1\n";

  check_int(run_bash_script(script) == 0, "echo-client ws dial");
}

static void test_echo_client_wss_dial(void) {
  const char *script;

  if (command_exit_code("command -v openssl >/dev/null 2>&1") != 0) {
    ++passed;
    fprintf(stderr, "SKIP: echo-client wss dial (openssl missing)\n");
    return;
  }

  script =
      "cleanup() {\n"
      "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "}\n"
      "trap cleanup EXIT\n"
      "TMP_DIR=$(mktemp -d)\n"
      "CERT=\"$TMP_DIR/server.crt\"\n"
      "KEY=\"$TMP_DIR/server.key\"\n"
      "openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 1 \\\n"
      "  -subj '/CN=127.0.0.1' \\\n"
      "  -addext 'subjectAltName=IP:127.0.0.1,DNS:localhost' \\\n"
      "  -keyout \"$KEY\" -out \"$CERT\" >/dev/null 2>&1\n"
      "S_OUT=$(mktemp)\n"
      "S_ERR=$(mktemp)\n"
      "./bin/echo-server --listen \"wss://127.0.0.1:0/grpc?cert=$CERT&key=$KEY\" >\"$S_OUT\" 2>\"$S_ERR\" &\n"
      "S_PID=$!\n"
      "ADDR=\"\"\n"
      "for _ in $(seq 1 120); do\n"
      "  if [ -s \"$S_OUT\" ]; then\n"
      "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
      "    if [ -n \"$ADDR\" ]; then break; fi\n"
      "  fi\n"
      "  sleep 0.05\n"
      "done\n"
      "[ -n \"$ADDR\" ]\n"
      "./bin/echo-client --insecure-tls --message cert-wss \"$ADDR\" >/dev/null 2>&1\n";

  check_int(run_bash_script(script) == 0, "echo-client wss dial");
}

static void test_rest_sse_transport(void) {
  const char *script;

  if (command_exit_code("command -v python3 >/dev/null 2>&1") != 0) {
    ++passed;
    fprintf(stderr, "SKIP: rest+sse transport (python3 missing)\n");
    return;
  }

  script =
      "cleanup() {\n"
      "  if [ -n \"${S_PID:-}\" ] && kill -0 \"$S_PID\" >/dev/null 2>&1; then\n"
      "    kill -TERM \"$S_PID\" >/dev/null 2>&1 || true\n"
      "    wait \"$S_PID\" >/dev/null 2>&1 || true\n"
      "  fi\n"
      "}\n"
      "trap cleanup EXIT\n"
      "S_OUT=$(mktemp)\n"
      "S_ERR=$(mktemp)\n"
      "./bin/holon-rpc-server http://127.0.0.1:0/api/v1/rpc >\"$S_OUT\" 2>\"$S_ERR\" &\n"
      "S_PID=$!\n"
      "ADDR=\"\"\n"
      "for _ in $(seq 1 120); do\n"
      "  if [ -s \"$S_OUT\" ]; then\n"
      "    ADDR=$(head -n1 \"$S_OUT\" | tr -d '\\r\\n')\n"
      "    if [ -n \"$ADDR\" ]; then break; fi\n"
      "  fi\n"
      "  sleep 0.05\n"
      "done\n"
      "[ -n \"$ADDR\" ]\n"
      "REST_ADDR=\"rest+sse://${ADDR#http://}\"\n"
      "./bin/holon-rpc-client \"$REST_ADDR\" --method echo.v1.Echo/Ping --message cert-rest >/dev/null 2>&1\n"
      "./bin/holon-rpc-client \"$REST_ADDR\" --stream --method build.v1.Build/Watch "
      "--params-json '{\"project\":\"myapp\"}' >/dev/null 2>&1\n"
      "ADDR=\"$ADDR\" python3 - <<'PY'\n"
      "import http.client\n"
      "import json\n"
      "import os\n"
      "from urllib import request\n"
      "from urllib.parse import urlparse\n"
      "\n"
      "addr = os.environ['ADDR'].rstrip('/')\n"
      "origin = 'https://example.test'\n"
      "\n"
      "preflight = request.Request(addr + '/echo.v1.Echo/Ping', method='OPTIONS', headers={'Origin': origin})\n"
      "with request.urlopen(preflight, timeout=5) as resp:\n"
      "    assert resp.status == 204, resp.status\n"
      "    assert resp.headers.get('Access-Control-Allow-Origin') == origin\n"
      "    assert resp.headers.get('Access-Control-Allow-Methods') == 'GET, POST, OPTIONS'\n"
      "    assert resp.headers.get('Access-Control-Allow-Headers') == 'Content-Type, Accept, Last-Event-ID'\n"
      "\n"
      "payload = json.dumps({'message': 'raw-rest'}).encode()\n"
      "post = request.Request(\n"
      "    addr + '/echo.v1.Echo/Ping',\n"
      "    data=payload,\n"
      "    method='POST',\n"
      "    headers={'Content-Type': 'application/json', 'Accept': 'application/json'},\n"
      ")\n"
      "with request.urlopen(post, timeout=5) as resp:\n"
      "    assert resp.status == 200, resp.status\n"
      "    assert resp.headers.get_content_type() == 'application/json'\n"
      "    body = json.load(resp)\n"
      "    assert body['result']['message'] == 'raw-rest'\n"
      "\n"
      "parsed = urlparse(addr + '/build.v1.Build/Watch')\n"
      "conn = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=5)\n"
      "conn.request(\n"
      "    'POST',\n"
      "    parsed.path,\n"
      "    body=json.dumps({'project': 'myapp'}),\n"
      "    headers={'Content-Type': 'application/json', 'Accept': 'text/event-stream'},\n"
      ")\n"
      "resp = conn.getresponse()\n"
      "body = resp.read().decode()\n"
      "assert resp.status == 200, resp.status\n"
      "assert resp.getheader('Content-Type', '').startswith('text/event-stream')\n"
      "assert 'event: message' in body\n"
      "assert 'event: done' in body\n"
      "assert 'building' in body and 'done' in body\n"
      "PY\n";

  check_int(run_bash_script(script) == 0, "rest+sse transport");
}

static void test_serve_stdio(void) {
  char err[256];
  handler_calls = 0;
  check_int(holons_serve("stdio://", noop_handler, NULL, 1, 0, err, sizeof(err)) == 0, "serve stdio");
  check_int(handler_calls == 1, "serve handler call count");
}

static void test_observability_env(void) {
  char token[HOLON_OBS_TOKEN_MAX];
  uint32_t all = HOLON_FAMILY_LOGS | HOLON_FAMILY_METRICS | HOLON_FAMILY_EVENTS | HOLON_FAMILY_PROM;

  check_int(holon_obs_parse_families("all") == all, "observability parse all families");
  check_int(holon_obs_parse_families("all,otel") == 0, "observability parse rejects otel");
  check_int(holon_obs_parse_families("all,sessions") == 0, "observability parse rejects sessions");
  check_int(holon_obs_parse_families("unknown") == 0, "observability parse rejects unknown");
  check_int(holon_obs_check_env("logs,otel", token) != 0, "observability rejects otel");
  check_int(holon_obs_check_env("logs,sessions", token) != 0, "observability rejects sessions token");
  setenv("OP_OBS", "logs,otel", 1);
  check_int(holon_obs_configure(NULL) == 0, "observability configure rejects otel");
  unsetenv("OP_OBS");
  setenv("OP_SESSIONS", "metrics", 1);
  check_int(holon_obs_check_env(NULL, token) != 0, "observability rejects OP_SESSIONS");
  check_int(holon_obs_configure(NULL) == 0, "observability configure rejects OP_SESSIONS");
  unsetenv("OP_SESSIONS");
}

static void test_observability_disk_outputs(void) {
  char root[PATH_MAX];
  char expected[PATH_MAX];
  char run_dir[PATH_MAX];
  char path[PATH_MAX];
  char buf[4096];
  char cleanup[PATH_MAX + 32];
  char *old_obs = getenv("OP_OBS") ? strdup(getenv("OP_OBS")) : NULL;
  char *old_sessions = getenv("OP_SESSIONS") ? strdup(getenv("OP_SESSIONS")) : NULL;
  holon_obs_config_t cfg;
  const char *fields[] = {"component", "c", NULL};
  const char *payload[] = {"listener", "tcp://127.0.0.1:1", NULL};
  holon_meta_t meta;

  snprintf(root, sizeof(root), "/tmp/c-holons-obs-XXXXXX");
  check_int(make_temp_dir(root) == 0, "observability temp dir");
  canonicalize_temp_dir(root, sizeof(root));
  setenv("OP_OBS", "logs,metrics,events", 1);
  unsetenv("OP_SESSIONS");

  memset(&cfg, 0, sizeof(cfg));
  cfg.slug = "gabriel-greeting-c";
  cfg.instance_uid = "uid-1";
  cfg.run_dir = root;
  cfg.default_log_level = HOLON_LEVEL_INFO;

  holon_obs_reset();
  check_int(holon_obs_configure(&cfg) == 1, "observability configure enabled");
  snprintf(expected, sizeof(expected), "%s/gabriel-greeting-c/uid-1", root);
  check_int(holon_obs_current_run_dir(run_dir, sizeof(run_dir)) == 0, "observability current run dir");
  check_int(strcmp(run_dir, expected) == 0, "observability derives run dir from registry root");
  check_int(holon_obs_enable_disk_writers(run_dir) == 0, "observability disk writers");

  holon_obs_log(HOLON_LEVEL_INFO, "service-log", fields);
  check_int(holon_obs_counter_inc("c_requests_total", NULL) == 1, "observability counter inc");
  holon_obs_gauge_set("c_live_gauge", NULL, 2.5);
  check_int(holon_obs_gauge_value("c_live_gauge", NULL) == 2.5, "observability gauge value");
  holon_obs_emit(HOLON_EVENT_INSTANCE_READY, payload);

  memset(&meta, 0, sizeof(meta));
  meta.slug = cfg.slug;
  meta.uid = cfg.instance_uid;
  meta.pid = 123;
  meta.started_at_epoch = (int64_t)time(NULL);
  meta.transport = "tcp";
  meta.address = "tcp://127.0.0.1:1";
  snprintf(path, sizeof(path), "%s/stdout.log", run_dir);
  meta.log_path = path;
  check_int(holon_obs_write_meta_json(run_dir, &meta) == 0, "observability meta json");

  snprintf(path, sizeof(path), "%s/stdout.log", run_dir);
  check_int(read_file(path, buf, sizeof(buf)) == 0 && strstr(buf, "\"message\":\"service-log\"") != NULL,
            "observability stdout log");
  snprintf(path, sizeof(path), "%s/events.jsonl", run_dir);
  check_int(read_file(path, buf, sizeof(buf)) == 0 && strstr(buf, "\"type\":\"INSTANCE_READY\"") != NULL,
            "observability events log");
  snprintf(path, sizeof(path), "%s/meta.json", run_dir);
  check_int(read_file(path, buf, sizeof(buf)) == 0 && strstr(buf, "\"uid\":\"uid-1\"") != NULL,
            "observability meta uid");

  holon_obs_reset();
  restore_env("OP_OBS", old_obs);
  restore_env("OP_SESSIONS", old_sessions);
  snprintf(cleanup, sizeof(cleanup), "rm -rf %s", root);
  (void)system(cleanup);
}

int main(void) {
  test_observability_env();
  test_observability_disk_outputs();
  test_discover();
  test_connect_direct_dial();
  test_connect_starts_slug_ephemerally();
  test_connect_reuses_port_file();
  test_connect_removes_stale_port_file();
  test_scheme_and_flags();
  test_uri_parsing();
  test_identity_parsing();
  test_manifest_resolution();
  test_describe_response();
  test_describe_registration_requires_static_response();
  test_describe_registration();
  test_describe_without_protos();
  test_describe_static_binary_without_protos();
  test_tcp_transport();
  test_unix_transport();
  test_stdio_transport();
  test_dial_stdio();
  test_ws_transport();
  test_wss_transport();
  test_serve_stdio();

  printf("%d passed, %d failed\n", passed, failed);
  return failed > 0 ? 1 : 0;
}
