#define _POSIX_C_SOURCE 200809L

#include "holons/holons.h"
#include "hello.h"

#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

static int passed = 0;
static int failed = 0;

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

static int dial_tcp(const holons_uri_t *uri) {
  struct addrinfo hints;
  struct addrinfo *res = NULL;
  struct addrinfo *it;
  char service[16];
  int fd = -1;
  int rc;

  memset(&hints, 0, sizeof(hints));
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;

  snprintf(service, sizeof(service), "%d", uri->port);
  rc = getaddrinfo(uri->host[0] ? uri->host : "127.0.0.1", service, &hints, &res);
  if (rc != 0) {
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

static void test_greeting(void) {
  char out[128];

  check_int(hello_greet("Bob", out, sizeof(out)) == 0, "hello_greet Bob");
  check_int(strcmp(out, "Hello, Bob!") == 0, "greet Bob text");

  check_int(hello_greet("", out, sizeof(out)) == 0, "hello_greet empty");
  check_int(strcmp(out, "Hello, World!") == 0, "greet empty -> World");
}

static void test_transports(void) {
  char err[256];
  holons_uri_t uri;
  holons_listener_t listener;
  holons_conn_t client_conn;
  holons_conn_t server_conn;
  char buffer[64];
  ssize_t n;

  check_int(holons_parse_uri("tcp://:9090", &uri, err, sizeof(err)) == 0, "parse tcp");
  check_int(holons_parse_uri("unix:///tmp/hello.sock", &uri, err, sizeof(err)) == 0, "parse unix");
  check_int(holons_parse_uri("stdio://", &uri, err, sizeof(err)) == 0, "parse stdio");
  check_int(holons_parse_uri("mem://", &uri, err, sizeof(err)) == 0, "parse mem");
  check_int(holons_parse_uri("ws://127.0.0.1:8080/grpc", &uri, err, sizeof(err)) == 0, "parse ws");
  check_int(holons_parse_uri("wss://127.0.0.1:8443/grpc", &uri, err, sizeof(err)) == 0, "parse wss");

  check_int(holons_listen("mem://", &listener, err, sizeof(err)) == 0, "listen mem");
  check_int(holons_mem_dial(&listener, &client_conn, err, sizeof(err)) == 0, "mem dial");
  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "mem accept");

  check_int(holons_conn_write(&client_conn, "Bob\n", 6) == 6, "mem write request");
  n = holons_conn_read(&server_conn, buffer, sizeof(buffer) - 1);
  check_int(n == 6, "mem read request");
  buffer[n] = '\0';
  if (n > 0 && buffer[n - 1] == '\n') {
    buffer[n - 1] = '\0';
  }

  check_int(hello_greet(buffer, buffer, sizeof(buffer)) == 0, "hello over mem");
  check_int(holons_conn_write(&server_conn, buffer, strlen(buffer)) == (ssize_t)strlen(buffer),
            "mem write response");
  n = holons_conn_read(&client_conn, buffer, sizeof(buffer) - 1);
  check_int(n > 0, "mem read response");

  holons_conn_close(&client_conn);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

static void test_tcp_roundtrip(void) {
  char err[256];
  holons_listener_t listener;
  holons_uri_t bound;
  holons_conn_t server_conn;
  int client_fd = -1;
  char input[32];
  char output[64];
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
  check_int(holons_parse_uri(listener.bound_uri, &bound, err, sizeof(err)) == 0, "parse bound tcp");
  client_fd = dial_tcp(&bound);
  check_int(client_fd >= 0, "dial tcp");
  if (client_fd < 0) {
    holons_close_listener(&listener);
    return;
  }

  check_int(holons_accept(&listener, &server_conn, err, sizeof(err)) == 0, "accept tcp");
  write(client_fd, "Bob\n", 4);
  n = holons_conn_read(&server_conn, input, sizeof(input) - 1);
  check_int(n == 4, "read tcp request");
  input[n] = '\0';
  if (n > 0 && input[n - 1] == '\n') {
    input[n - 1] = '\0';
  }

  check_int(hello_greet(input, output, sizeof(output)) == 0, "hello over tcp");
  holons_conn_write(&server_conn, output, strlen(output));
  n = read(client_fd, input, sizeof(input));
  check_int(n > 0, "read tcp response");

  close(client_fd);
  holons_conn_close(&server_conn);
  holons_close_listener(&listener);
}

int main(void) {
  test_greeting();
  test_transports();
  test_tcp_roundtrip();

  printf("%d passed, %d failed\n", passed, failed);
  return failed > 0 ? 1 : 0;
}
